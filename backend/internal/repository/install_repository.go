package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/config"
	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/database"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var databaseNamePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,62}$`)

type InstallRepository interface {
	Status(version string) model.InstallStatus
	Environment(ctx context.Context) []model.EnvironmentCheck
	Complete(ctx context.Context, input model.CompleteInstallationInput, passwordHash string) (model.InstallationResult, error)
	RuntimeConfig() (model.InstallRuntimeConfig, error)
	MigrateInstalled(ctx context.Context) error
}

func (r *installRepository) MigrateInstalled(ctx context.Context) error {
	runtimeConfig, err := r.RuntimeConfig()
	if err != nil {
		return err
	}
	db, err := openTargetDatabase(ctx, runtimeConfig.Database)
	if err != nil {
		return err
	}
	return migrateAndClose(db)
}

type installRepository struct {
	baseDatabase config.DatabaseConfig
	baseRedis    config.RedisConfig
	dataDir      string
	mode         string
	mu           sync.Mutex
}

func NewInstallRepository(cfg *config.Config) InstallRepository {
	return &installRepository{baseDatabase: cfg.Database, baseRedis: cfg.Redis, dataDir: cfg.Install.DataDir, mode: cfg.Install.Mode}
}

func (r *installRepository) lockPath() string { return filepath.Join(r.dataDir, ".install.lock") }
func (r *installRepository) Status(version string) model.InstallStatus {
	installed := false
	databaseReachable := false
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if db, err := openTargetDatabase(ctx, databaseInputFromConfig(r.baseDatabase)); err == nil {
		databaseReachable = true
		var count int64
		if db.Raw("SELECT COUNT(*) FROM system_settings WHERE key = 'installation.completed_at'").Scan(&count).Error == nil {
			installed = count > 0
		}
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}
	if !installed && !databaseReachable {
		_, err := os.Stat(r.lockPath())
		installed = err == nil
	}
	return model.InstallStatus{Installed: installed, Version: version, Mode: r.mode, LockPath: r.lockPath()}
}

func (r *installRepository) Environment(ctx context.Context) []model.EnvironmentCheck {
	databaseInput := databaseInputFromConfig(r.baseDatabase)
	redisInput := redisInputFromConfig(r.baseRedis)
	checks := []model.EnvironmentCheck{
		r.filePermissionCheck(),
		environmentVariablesCheck(),
	}
	dbContext, cancelDB := context.WithTimeout(ctx, 3*time.Second)
	dbVersion, dbErr := r.databaseReadiness(dbContext, databaseInput)
	cancelDB()
	checks = append(checks, versionedConnectionCheck("postgres", "PostgreSQL 18 + pgvector", dbVersion, dbErr, true, "Use the bundled pgvector PostgreSQL 18 image and inspect its Compose health check."))

	redisContext, cancelRedis := context.WithTimeout(ctx, 3*time.Second)
	redisVersion, redisErr := redisReadiness(redisContext, redisInput)
	cancelRedis()
	checks = append(checks, versionedConnectionCheck("redis", "Redis 8", redisVersion, redisErr, true, "Use the bundled Redis 8 image and inspect its Compose health check."))
	return checks
}

func (r *installRepository) filePermissionCheck() model.EnvironmentCheck {
	if err := os.MkdirAll(r.dataDir, 0o750); err != nil {
		return model.EnvironmentCheck{ID: "file_permissions", Name: "File permissions", Status: "fail", Message: "Installation data directory is not writable.", Remediation: "Grant the NexaFlow process write access to " + r.dataDir, Required: true}
	}
	file, err := os.CreateTemp(r.dataDir, ".write-check-*")
	if err != nil {
		return model.EnvironmentCheck{ID: "file_permissions", Name: "File permissions", Status: "fail", Message: "Installation data directory is not writable.", Remediation: "Grant the NexaFlow process write access to " + r.dataDir, Required: true}
	}
	name := file.Name()
	_ = file.Close()
	_ = os.Remove(name)
	return model.EnvironmentCheck{ID: "file_permissions", Name: "File permissions", Status: "pass", Message: "Installation data directory is writable.", Required: true}
}

func (r *installRepository) prepareDatabase(ctx context.Context, input model.DatabaseInput) error {
	if !databaseNamePattern.MatchString(input.Name) {
		return errors.New("database name must begin with a letter and contain only letters, numbers, underscores, or hyphens")
	}
	if input.SSLMode == "" {
		input.SSLMode = "disable"
	}
	if db, err := openTargetDatabase(ctx, input); err == nil {
		return migrateAndClose(db)
	}

	maintenanceDSN := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=postgres sslmode=%s connect_timeout=5", input.Host, input.Port, input.User, input.Password, input.SSLMode)
	maintenance, err := sql.Open("pgx", maintenanceDSN)
	if err != nil {
		return fmt.Errorf("open postgres maintenance connection: %w", err)
	}
	defer maintenance.Close()
	if err := maintenance.PingContext(ctx); err != nil {
		return fmt.Errorf("connect to postgres: %w", err)
	}
	var exists bool
	if err := maintenance.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", input.Name).Scan(&exists); err != nil {
		return fmt.Errorf("check target database: %w", err)
	}
	if !exists {
		if _, err := maintenance.ExecContext(ctx, `CREATE DATABASE "`+input.Name+`"`); err != nil {
			return fmt.Errorf("create target database: %w", err)
		}
	}
	db, err := openTargetDatabase(ctx, input)
	if err != nil {
		return err
	}
	return migrateAndClose(db)
}

func (r *installRepository) Complete(ctx context.Context, input model.CompleteInstallationInput, passwordHash string) (model.InstallationResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Status("").Installed {
		return model.InstallationResult{}, errors.New("NexaFlow is already installed")
	}
	databaseInput := databaseInputFromConfig(r.baseDatabase)
	redisInput := redisInputFromConfig(r.baseRedis)
	if err := r.prepareDatabase(ctx, databaseInput); err != nil {
		return model.InstallationResult{}, err
	}
	if err := pingRedis(ctx, redisInput); err != nil {
		return model.InstallationResult{}, fmt.Errorf("connect to redis: %w", err)
	}
	db, err := openTargetDatabase(ctx, databaseInput)
	if err != nil {
		return model.InstallationResult{}, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return model.InstallationResult{}, fmt.Errorf("get installation database pool: %w", err)
	}
	defer sqlDB.Close()
	now := time.Now().UTC()
	userID, roleID, companyID, tenantID, err := newUUID(), newUUID(), newUUID(), newUUID(), error(nil)
	if userID == "" || roleID == "" || companyID == "" || tenantID == "" {
		return model.InstallationResult{}, errors.New("generate installation identifiers")
	}
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		tenant := model.Tenant{ID: tenantID, Slug: tenantSlug(input.Company.Name), Name: strings.TrimSpace(input.Company.Name), Status: "active", CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(&tenant).Error; err != nil {
			return fmt.Errorf("create tenant: %w", err)
		}
		company := model.BootstrapCompany{ID: companyID, TenantID: &tenant.ID, Name: strings.TrimSpace(input.Company.Name), Industry: input.Company.Industry, DefaultLanguage: input.Company.DefaultLanguage, Timezone: input.Company.Timezone, CreatedAt: now, UpdatedAt: now}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&company).Error; err != nil {
			return fmt.Errorf("create company: %w", err)
		}
		role := model.BootstrapRole{ID: roleID, Name: "super_admin", DisplayName: "超级管理员", CreatedAt: now}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "name"}}, DoNothing: true}).Create(&role).Error; err != nil {
			return fmt.Errorf("create super admin role: %w", err)
		}
		if err := tx.Where("name = ?", "super_admin").First(&role).Error; err != nil {
			return fmt.Errorf("load super admin role: %w", err)
		}
		roleNames := []struct{ Name, Display string }{{"admin", "管理员"}, {"employee", "普通员工"}, {"guest", "访客"}}
		for _, item := range roleNames {
			candidate := model.BootstrapRole{ID: newUUID(), Name: item.Name, DisplayName: item.Display, CreatedAt: now}
			if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "name"}}, DoNothing: true}).Create(&candidate).Error; err != nil {
				return fmt.Errorf("create role %s: %w", item.Name, err)
			}
		}
		permissions := []struct{ Name, Description string }{
			{"dashboard.view", "View the enterprise dashboard"}, {"dashboard.manage", "Configure the enterprise dashboard"}, {"user.view", "View users"},
			{"user.create", "Create users"}, {"user.delete", "Delete users"},
			{"role.manage", "Manage roles and permissions"}, {"order.view", "View orders"},
			{"finance.manage", "Manage finance data"}, {"system.manage", "Manage platform settings"},
			{"entity.view", "View dynamic business models"}, {"entity.manage", "Create and modify dynamic business models"},
			{"record.view", "View dynamic business records"}, {"record.manage", "Create and modify dynamic business records"},
			{"form.view", "View low-code forms"}, {"form.manage", "Create and modify low-code forms"},
			{"workflow.view", "View workflows and instances"}, {"workflow.manage", "Create and modify workflows"},
			{"workflow.submit", "Submit records to workflows"}, {"workflow.approve", "Approve or reject workflow tasks"},
			{"knowledge.view", "View knowledge documents"}, {"knowledge.manage", "Upload and delete knowledge documents"},
			{"knowledge.search", "Search tenant knowledge"}, {"ai.chat", "Use the tenant AI assistant"},
			{"billing.manage", "Manage tenant subscription and billing"},
			{"file.view", "View and download tenant files"}, {"file.manage", "Upload and delete tenant files"},
		}
		roleGrants := map[string]map[string]bool{
			"super_admin": {"*": true},
			"admin":       {"dashboard.view": true, "dashboard.manage": true, "user.view": true, "user.create": true, "order.view": true, "entity.view": true, "entity.manage": true, "record.view": true, "record.manage": true, "form.view": true, "form.manage": true, "workflow.view": true, "workflow.manage": true, "workflow.submit": true, "workflow.approve": true, "knowledge.view": true, "knowledge.manage": true, "knowledge.search": true, "ai.chat": true, "billing.manage": true, "file.view": true, "file.manage": true},
			"employee":    {"dashboard.view": true, "order.view": true, "entity.view": true, "record.view": true, "form.view": true, "workflow.view": true, "workflow.submit": true, "knowledge.view": true, "knowledge.search": true, "ai.chat": true, "file.view": true},
			"guest":       {"dashboard.view": true},
		}
		for _, item := range permissions {
			permission := model.BootstrapPermission{ID: newUUID(), Name: item.Name, Description: item.Description, CreatedAt: now}
			if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "name"}}, DoNothing: true}).Create(&permission).Error; err != nil {
				return fmt.Errorf("create permission %s: %w", item.Name, err)
			}
			if err := tx.Where("name = ?", item.Name).First(&permission).Error; err != nil {
				return fmt.Errorf("load permission %s: %w", item.Name, err)
			}
			for roleName, grants := range roleGrants {
				if !grants["*"] && !grants[item.Name] {
					continue
				}
				var targetRole model.BootstrapRole
				if err := tx.Where("name = ?", roleName).First(&targetRole).Error; err != nil {
					return fmt.Errorf("load role %s: %w", roleName, err)
				}
				binding := model.BootstrapRolePermission{RoleID: targetRole.ID, PermissionID: permission.ID, CreatedAt: now}
				if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&binding).Error; err != nil {
					return fmt.Errorf("grant permission %s to %s: %w", item.Name, roleName, err)
				}
				tenantBinding := model.TenantRolePermission{TenantID: tenant.ID, RoleID: targetRole.ID, PermissionID: permission.ID, CreatedAt: now}
				if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&tenantBinding).Error; err != nil {
					return fmt.Errorf("grant tenant permission %s to %s: %w", item.Name, roleName, err)
				}
			}
		}
		user := model.BootstrapUser{ID: userID, Username: strings.TrimSpace(input.Admin.Username), Email: strings.ToLower(strings.TrimSpace(input.Admin.Email)), PasswordHash: passwordHash, Status: "active", CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(&user).Error; err != nil {
			return fmt.Errorf("create administrator: %w", err)
		}
		binding := model.BootstrapUserRole{UserID: user.ID, RoleID: role.ID, CreatedAt: now}
		if err := tx.Create(&binding).Error; err != nil {
			return fmt.Errorf("assign super admin role: %w", err)
		}
		if err := tx.Create(&model.TenantMembership{TenantID: tenant.ID, UserID: user.ID, Status: "active", CreatedAt: now}).Error; err != nil {
			return fmt.Errorf("create tenant membership: %w", err)
		}
		if err := tx.Create(&model.TenantUserRole{TenantID: tenant.ID, UserID: user.ID, RoleID: role.ID, CreatedAt: now}).Error; err != nil {
			return fmt.Errorf("assign tenant super admin role: %w", err)
		}
		settings := []model.BootstrapSetting{
			{Key: "installation.completed_at", Value: now.Format(time.RFC3339), CreatedAt: now, UpdatedAt: now},
			{Key: "company.primary_id", Value: company.ID, CreatedAt: now, UpdatedAt: now},
			{Key: "tenant.primary_id", Value: tenant.ID, CreatedAt: now, UpdatedAt: now},
		}
		return tx.Create(&settings).Error
	})
	if err != nil {
		return model.InstallationResult{}, err
	}

	lockContents := []byte("installed_at=" + now.Format(time.RFC3339) + "\n")
	if err := os.WriteFile(r.lockPath(), lockContents, 0o600); err != nil {
		return model.InstallationResult{}, fmt.Errorf("write installation lock: %w", err)
	}
	return model.InstallationResult{AdminURL: "/admin", Username: input.Admin.Username, LockPath: r.lockPath()}, nil
}

func (r *installRepository) RuntimeConfig() (model.InstallRuntimeConfig, error) {
	return model.InstallRuntimeConfig{Database: databaseInputFromConfig(r.baseDatabase), Redis: redisInputFromConfig(r.baseRedis), Written: time.Now().UTC()}, nil
}

func openTargetDatabase(ctx context.Context, input model.DatabaseInput) (*gorm.DB, error) {
	cfg := config.DatabaseConfig{Host: input.Host, Port: input.Port, User: input.User, Password: input.Password, Name: input.Name, SSLMode: input.SSLMode, MaxOpenConnections: 10, MaxIdleConnections: 2, ConnectionMaxLifetime: 30 * time.Minute}
	return database.Open(ctx, cfg)
}

func migrateBootstrapSchema(db *gorm.DB) error {
	if err := db.Exec("CREATE EXTENSION IF NOT EXISTS vector").Error; err != nil {
		return fmt.Errorf("enable pgvector extension: %w", err)
	}
	if err := db.AutoMigrate(&model.Tenant{}, &model.BootstrapCompany{}, &model.BootstrapUser{}, &model.TenantMembership{}, &model.BootstrapRole{}, &model.BootstrapPermission{}, &model.BootstrapRolePermission{}, &model.TenantRolePermission{}, &model.BootstrapUserRole{}, &model.TenantUserRole{}, &model.BootstrapSetting{}, &model.RefreshSession{}, &model.Entity{}, &model.EntityField{}, &model.DynamicRecord{}, &model.Form{}, &model.Workflow{}, &model.WorkflowInstance{}, &model.WorkflowAction{}, &model.Notification{}, &model.KnowledgeDocument{}, &model.KnowledgeChunk{}, &model.AIConversation{}, &model.AIMessage{}, &model.Plan{}, &model.Subscription{}, &model.UsageCounter{}, &model.BillingEvent{}, &model.FileAsset{}, &model.Dashboard{}); err != nil {
		return err
	}
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_knowledge_chunks_embedding_hnsw ON knowledge_chunks USING hnsw (embedding vector_cosine_ops)").Error; err != nil {
		return fmt.Errorf("create knowledge embedding index: %w", err)
	}
	if err := seedBillingPlans(db); err != nil {
		return err
	}
	if err := backfillPrimaryTenant(db); err != nil {
		return err
	}
	return ensureDynamicModelPermissions(db)
}

func seedBillingPlans(db *gorm.DB) error {
	now := time.Now().UTC()
	plans := []model.Plan{
		{ID: newUUID(), Code: "free", Name: "Community", PriceCents: 0, Currency: "usd", MaxUsers: 10, MaxRecords: 10000, MaxKnowledgeBytes: 100 << 20, MaxAITokens: 200000, Status: "active", CreatedAt: now, UpdatedAt: now},
		{ID: newUUID(), Code: "pro", Name: "Professional", PriceCents: 9900, Currency: "usd", MaxUsers: 100, MaxRecords: 1000000, MaxKnowledgeBytes: 10 << 30, MaxAITokens: 5000000, Status: "active", CreatedAt: now, UpdatedAt: now},
		{ID: newUUID(), Code: "enterprise", Name: "Enterprise", PriceCents: 49900, Currency: "usd", MaxUsers: 1000, MaxRecords: 10000000, MaxKnowledgeBytes: 100 << 30, MaxAITokens: 50000000, Status: "active", CreatedAt: now, UpdatedAt: now},
	}
	for _, plan := range plans {
		if err := db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "code"}}, DoUpdates: clause.AssignmentColumns([]string{"name", "price_cents", "currency", "max_users", "max_records", "max_knowledge_bytes", "max_ai_tokens", "status", "updated_at"})}).Create(&plan).Error; err != nil {
			return err
		}
	}
	return nil
}

func ensureDynamicModelPermissions(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		definitions := []struct{ Name, Description string }{
			{"entity.view", "View dynamic business models"}, {"entity.manage", "Create and modify dynamic business models"},
			{"record.view", "View dynamic business records"}, {"record.manage", "Create and modify dynamic business records"},
			{"form.view", "View low-code forms"}, {"form.manage", "Create and modify low-code forms"},
			{"workflow.view", "View workflows and instances"}, {"workflow.manage", "Create and modify workflows"},
			{"workflow.submit", "Submit records to workflows"}, {"workflow.approve", "Approve or reject workflow tasks"},
			{"knowledge.view", "View knowledge documents"}, {"knowledge.manage", "Upload and delete knowledge documents"},
			{"knowledge.search", "Search tenant knowledge"}, {"ai.chat", "Use the tenant AI assistant"},
			{"billing.manage", "Manage tenant subscription and billing"},
			{"dashboard.manage", "Configure the enterprise dashboard"},
			{"file.view", "View and download tenant files"}, {"file.manage", "Upload and delete tenant files"},
		}
		createdPermissions := make([]string, 0, len(definitions))
		for _, definition := range definitions {
			permission := model.BootstrapPermission{ID: newUUID(), Name: definition.Name, Description: definition.Description, CreatedAt: now}
			result := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "name"}}, DoNothing: true}).Create(&permission)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 1 {
				createdPermissions = append(createdPermissions, definition.Name)
			}
		}
		// Default grants are an upgrade action, not a startup reconciliation rule.
		// Once a permission exists, later tenant-specific revocations must survive restarts.
		if len(createdPermissions) == 0 {
			return nil
		}
		var tenants []model.Tenant
		if err := tx.Where("status = ?", "active").Find(&tenants).Error; err != nil {
			return err
		}
		grants := map[string][]string{
			"super_admin": {"entity.view", "entity.manage", "record.view", "record.manage", "form.view", "form.manage", "workflow.view", "workflow.manage", "workflow.submit", "workflow.approve", "knowledge.view", "knowledge.manage", "knowledge.search", "ai.chat", "billing.manage", "dashboard.manage", "file.view", "file.manage"},
			"admin":       {"entity.view", "entity.manage", "record.view", "record.manage", "form.view", "form.manage", "workflow.view", "workflow.manage", "workflow.submit", "workflow.approve", "knowledge.view", "knowledge.manage", "knowledge.search", "ai.chat", "billing.manage", "dashboard.manage", "file.view", "file.manage"},
			"employee":    {"entity.view", "record.view", "form.view", "workflow.view", "workflow.submit", "knowledge.view", "knowledge.search", "ai.chat", "file.view"},
		}
		for roleName, names := range grants {
			filtered := names[:0]
			for _, name := range names {
				for _, created := range createdPermissions {
					if name == created {
						filtered = append(filtered, name)
					}
				}
			}
			grants[roleName] = filtered
		}
		for _, tenant := range tenants {
			for roleName, names := range grants {
				if len(names) == 0 {
					continue
				}
				var role model.BootstrapRole
				if err := tx.Where("name = ?", roleName).First(&role).Error; err != nil {
					return err
				}
				var permissions []model.BootstrapPermission
				if err := tx.Where("name IN ?", names).Find(&permissions).Error; err != nil {
					return err
				}
				for _, permission := range permissions {
					binding := model.TenantRolePermission{TenantID: tenant.ID, RoleID: role.ID, PermissionID: permission.ID, CreatedAt: now}
					if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&binding).Error; err != nil {
						return err
					}
				}
			}
		}
		return nil
	})
}

func backfillPrimaryTenant(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var tenant model.Tenant
		if err := tx.Order("created_at").First(&tenant).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			var company model.BootstrapCompany
			if err := tx.Order("created_at").First(&company).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return nil
				}
				return err
			}
			now := time.Now().UTC()
			tenant = model.Tenant{ID: newUUID(), Slug: tenantSlug(company.Name), Name: company.Name, Status: "active", CreatedAt: now, UpdatedAt: now}
			if err := tx.Create(&tenant).Error; err != nil {
				return err
			}
			if err := tx.Model(&company).Update("tenant_id", tenant.ID).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		var legacy []model.BootstrapUserRole
		if err := tx.Find(&legacy).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		for _, binding := range legacy {
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.TenantMembership{TenantID: tenant.ID, UserID: binding.UserID, Status: "active", CreatedAt: now}).Error; err != nil {
				return err
			}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.TenantUserRole{TenantID: tenant.ID, UserID: binding.UserID, RoleID: binding.RoleID, CreatedAt: binding.CreatedAt}).Error; err != nil {
				return err
			}
		}
		var legacyPermissions []model.BootstrapRolePermission
		if err := tx.Find(&legacyPermissions).Error; err != nil {
			return err
		}
		for _, binding := range legacyPermissions {
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.TenantRolePermission{TenantID: tenant.ID, RoleID: binding.RoleID, PermissionID: binding.PermissionID, CreatedAt: binding.CreatedAt}).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&model.RefreshSession{}).Where("tenant_id = ?", "00000000-0000-0000-0000-000000000000").Update("tenant_id", tenant.ID).Error; err != nil {
			return err
		}
		return nil
	})
}

func migrateAndClose(db *gorm.DB) error {
	err := migrateBootstrapSchema(db)
	sqlDB, closeErr := db.DB()
	if closeErr == nil {
		closeErr = sqlDB.Close()
	}
	if err != nil {
		return err
	}
	return closeErr
}

func (r *installRepository) pingDatabase(ctx context.Context, input model.DatabaseInput) error {
	db, err := openTargetDatabase(ctx, input)
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err == nil {
		_ = sqlDB.Close()
	}
	return err
}

func (r *installRepository) databaseReadiness(ctx context.Context, input model.DatabaseInput) (string, error) {
	db, err := openTargetDatabase(ctx, input)
	if err != nil {
		return "", err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return "", err
	}
	defer sqlDB.Close()
	var versionNumber int
	var version string
	var vectorAvailable bool
	if err = db.Raw("SELECT current_setting('server_version_num')::int, current_setting('server_version'), EXISTS(SELECT 1 FROM pg_available_extensions WHERE name = 'vector')").Row().Scan(&versionNumber, &version, &vectorAvailable); err != nil {
		return "", err
	}
	if versionNumber < 180000 {
		return version, fmt.Errorf("PostgreSQL 18 or newer is required")
	}
	if !vectorAvailable {
		return version, fmt.Errorf("pgvector extension is not available")
	}
	return version + " / pgvector available", nil
}

func environmentVariablesCheck() model.EnvironmentCheck {
	configured := 0
	for _, key := range []string{"DATABASE_HOST", "DATABASE_USER", "DATABASE_NAME", "REDIS_HOST"} {
		if os.Getenv(key) != "" {
			configured++
		}
	}
	if configured == 4 {
		return model.EnvironmentCheck{ID: "environment", Name: "Environment variables", Status: "pass", Message: "Runtime service variables are configured.", Required: false}
	}
	return model.EnvironmentCheck{ID: "environment", Name: "Environment variables", Status: "warn", Message: "Runtime defaults are active.", Remediation: "Set DATABASE_* and REDIS_* values in .env before production deployment, then recreate the backend container.", Required: false}
}

func connectionCheck(id, name string, err error, required bool, remediation string) model.EnvironmentCheck {
	if err == nil {
		return model.EnvironmentCheck{ID: id, Name: name, Status: "pass", Message: name + " is reachable.", Required: required}
	}
	return model.EnvironmentCheck{ID: id, Name: name, Status: "fail", Message: name + " connection failed.", Remediation: remediation, Required: required}
}

func versionedConnectionCheck(id, name, version string, err error, required bool, remediation string) model.EnvironmentCheck {
	check := connectionCheck(id, name, err, required, remediation)
	check.Version = version
	return check
}

func pingRedis(ctx context.Context, input model.RedisInput) error {
	client := redis.NewClient(&redis.Options{
		Addr:     net.JoinHostPort(input.Host, fmt.Sprint(input.Port)),
		Password: input.Password,
		DB:       input.Database,
	})
	defer client.Close()
	return client.Ping(ctx).Err()
}

func redisReadiness(ctx context.Context, input model.RedisInput) (string, error) {
	client := redis.NewClient(&redis.Options{Addr: net.JoinHostPort(input.Host, fmt.Sprint(input.Port)), Password: input.Password, DB: input.Database})
	defer client.Close()
	info, err := client.Info(ctx, "server").Result()
	if err != nil {
		return "", err
	}
	match := regexp.MustCompile(`(?m)^redis_version:([^\r\n]+)`).FindStringSubmatch(info)
	if len(match) != 2 {
		return "", errors.New("Redis version was not reported")
	}
	major, err := strconv.Atoi(strings.Split(match[1], ".")[0])
	if err != nil || major < 8 {
		return match[1], errors.New("Redis 8 or newer is required")
	}
	return match[1], nil
}

func newUUID() string {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return ""
	}
	buffer[6] = (buffer[6] & 0x0f) | 0x40
	buffer[8] = (buffer[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(buffer)
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32]
}

func databaseInputFromConfig(cfg config.DatabaseConfig) model.DatabaseInput {
	return model.DatabaseInput{Host: cfg.Host, Port: cfg.Port, Name: cfg.Name, User: cfg.User, Password: cfg.Password, SSLMode: cfg.SSLMode}
}
func redisInputFromConfig(cfg config.RedisConfig) model.RedisInput {
	return model.RedisInput{Host: cfg.Host, Port: cfg.Port, Password: cfg.Password, Database: cfg.Database}
}

func tenantSlug(name string) string {
	value := strings.ToLower(strings.TrimSpace(name))
	value = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	if value == "" {
		value = "tenant-" + newUUID()[:8]
	}
	if len(value) > 72 {
		value = value[:72]
	}
	return value
}
