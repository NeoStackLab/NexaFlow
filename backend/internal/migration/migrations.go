package migration

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func defaultMigrations() []migration {
	return []migration{
		migrationFromSpec(1, "baseline_schema", "specs/000001_baseline_schema.txt", migrateBaseline),
		migrationFromSpec(2, "form_submissions_and_notification_outbox", "specs/000002_form_submissions_and_notification_outbox.txt", migrateFormSubmissions),
		migrationFromSpec(3, "user_lifecycle_status_management", "specs/000003_user_lifecycle_status_management.txt", ensureUserLifecyclePermission),
		migrationFromSpec(4, "tenant_audit_events", "specs/000004_tenant_audit_events.txt", migrateAuditEvents),
		migrationFromSpec(5, "durable_knowledge_ingestion_jobs", "specs/000005_durable_knowledge_ingestion_jobs.txt", migrateKnowledgeJobs),
		migrationFromSpec(6, "enterprise_tenant_context_settings_and_rls", "specs/000006_enterprise_tenant_context_settings_and_rls.txt", migrateEnterpriseTenantFoundation),
	}
}

func migrateBaseline(db *gorm.DB) error {
	if err := db.Exec("CREATE EXTENSION IF NOT EXISTS vector").Error; err != nil {
		return fmt.Errorf("enable pgvector extension: %w", err)
	}
	return db.AutoMigrate(
		&model.Tenant{}, &model.TenantSettings{}, &model.BootstrapCompany{}, &model.BootstrapUser{}, &model.TenantMembership{},
		&model.BootstrapRole{}, &model.BootstrapPermission{}, &model.BootstrapRolePermission{}, &model.TenantRolePermission{},
		&model.BootstrapUserRole{}, &model.TenantUserRole{}, &model.BootstrapSetting{}, &model.RefreshSession{}, &model.Entity{},
		&model.EntityField{}, &model.DynamicRecord{}, &model.Form{}, &model.FormSubmission{}, &model.Workflow{}, &model.WorkflowInstance{},
		&model.WorkflowAction{}, &model.Notification{}, &model.KnowledgeDocument{}, &model.KnowledgeChunk{}, &model.KnowledgeIngestionJob{},
		&model.AIConversation{}, &model.AIMessage{}, &model.Plan{}, &model.Subscription{}, &model.UsageCounter{}, &model.BillingEvent{},
		&model.FileAsset{}, &model.Dashboard{}, &model.AuditEvent{},
	)
}

func migrateFormSubmissions(db *gorm.DB) error {
	return db.AutoMigrate(&model.FormSubmission{}, &model.Notification{})
}

func migrateAuditEvents(db *gorm.DB) error {
	if err := db.AutoMigrate(&model.AuditEvent{}); err != nil {
		return err
	}
	return ensureAuditPermission(db)
}

func migrateKnowledgeJobs(db *gorm.DB) error {
	if err := db.AutoMigrate(&model.KnowledgeIngestionJob{}); err != nil {
		return err
	}
	return db.Exec("CREATE INDEX IF NOT EXISTS idx_knowledge_jobs_claim ON knowledge_ingestion_jobs (status, next_attempt_at, created_at)").Error
}

func migrateEnterpriseTenantFoundation(db *gorm.DB) error {
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_knowledge_chunks_embedding_hnsw ON knowledge_chunks USING hnsw (embedding vector_cosine_ops)").Error; err != nil {
		return fmt.Errorf("create knowledge embedding index: %w", err)
	}
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_notifications_outbox_claim ON notifications (status, next_attempt_at, created_at)").Error; err != nil {
		return fmt.Errorf("create notification outbox index: %w", err)
	}
	if err := seedBillingPlans(db); err != nil {
		return err
	}
	if err := backfillPrimaryTenant(db); err != nil {
		return err
	}
	if err := ensureDynamicModelPermissions(db); err != nil {
		return err
	}
	return ensureEnterpriseTenantSettingsAndRLS(db)
}

func ensureEnterpriseTenantSettingsAndRLS(db *gorm.DB) error {
	if err := db.AutoMigrate(&model.Tenant{}, &model.TenantSettings{}); err != nil {
		return err
	}
	now := time.Now().UTC()
	var tenants []model.Tenant
	if err := db.Find(&tenants).Error; err != nil {
		return err
	}
	for _, tenant := range tenants {
		settings := model.NewTenantSettings(tenant.ID, "UTC", "en", now)
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&settings).Error; err != nil {
			return err
		}
	}
	permissions := []model.BootstrapPermission{
		{ID: newUUID(), Name: "tenant.view", Description: "View tenant profile and settings", CreatedAt: now},
		{ID: newUUID(), Name: "tenant.manage", Description: "Manage tenant profile and settings", CreatedAt: now},
		{ID: newUUID(), Name: "tenant.lifecycle", Description: "Suspend, archive, or delete a tenant", CreatedAt: now},
	}
	for _, candidate := range permissions {
		if err := db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "name"}}, DoNothing: true}).Create(&candidate).Error; err != nil {
			return err
		}
		var permission model.BootstrapPermission
		if err := db.Where("name = ?", candidate.Name).First(&permission).Error; err != nil {
			return err
		}
		roleNames := []string{"super_admin", "admin"}
		if candidate.Name == "tenant.lifecycle" {
			roleNames = []string{"super_admin"}
		}
		var roles []model.BootstrapRole
		if err := db.Where("name IN ?", roleNames).Find(&roles).Error; err != nil {
			return err
		}
		for _, role := range roles {
			if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.BootstrapRolePermission{RoleID: role.ID, PermissionID: permission.ID, CreatedAt: now}).Error; err != nil {
				return err
			}
			for _, tenant := range tenants {
				if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.TenantRolePermission{TenantID: tenant.ID, RoleID: role.ID, PermissionID: permission.ID, CreatedAt: now}).Error; err != nil {
					return err
				}
			}
		}
	}
	return enableTenantRowLevelSecurity(db)
}

func ensureUserLifecyclePermission(db *gorm.DB) error {
	now := time.Now().UTC()
	permission := model.BootstrapPermission{ID: newUUID(), Name: "user.delete", Description: "Delete users", CreatedAt: now}
	if err := db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "name"}}, DoNothing: true}).Create(&permission).Error; err != nil {
		return err
	}
	if err := db.Where("name = ?", permission.Name).First(&permission).Error; err != nil {
		return err
	}
	var tenants []model.Tenant
	if err := db.Where("status = ?", "active").Find(&tenants).Error; err != nil {
		return err
	}
	var roles []model.BootstrapRole
	if err := db.Where("name IN ?", []string{"admin", "super_admin"}).Find(&roles).Error; err != nil {
		return err
	}
	for _, tenant := range tenants {
		for _, role := range roles {
			if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.TenantRolePermission{TenantID: tenant.ID, RoleID: role.ID, PermissionID: permission.ID, CreatedAt: now}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func ensureAuditPermission(db *gorm.DB) error {
	now := time.Now().UTC()
	permission := model.BootstrapPermission{ID: newUUID(), Name: "audit.view", Description: "View tenant audit events", CreatedAt: now}
	if err := db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "name"}}, DoNothing: true}).Create(&permission).Error; err != nil {
		return err
	}
	if err := db.Where("name = ?", permission.Name).First(&permission).Error; err != nil {
		return err
	}
	var tenants []model.Tenant
	if err := db.Where("status = ?", "active").Find(&tenants).Error; err != nil {
		return err
	}
	var roles []model.BootstrapRole
	if err := db.Where("name IN ?", []string{"admin", "super_admin"}).Find(&roles).Error; err != nil {
		return err
	}
	for _, role := range roles {
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.BootstrapRolePermission{RoleID: role.ID, PermissionID: permission.ID, CreatedAt: now}).Error; err != nil {
			return err
		}
	}
	for _, tenant := range tenants {
		for _, role := range roles {
			if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.TenantRolePermission{TenantID: tenant.ID, RoleID: role.ID, PermissionID: permission.ID, CreatedAt: now}).Error; err != nil {
				return err
			}
		}
	}
	return nil
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
	now := time.Now().UTC()
	definitions := []struct{ Name, Description string }{
		{"entity.view", "View dynamic business models"}, {"entity.manage", "Create and modify dynamic business models"},
		{"record.view", "View dynamic business records"}, {"record.manage", "Create and modify dynamic business records"},
		{"form.view", "View low-code forms"}, {"form.manage", "Create and modify low-code forms"}, {"form.submit", "Submit low-code forms"},
		{"workflow.view", "View workflows and instances"}, {"workflow.manage", "Create and modify workflows"},
		{"workflow.submit", "Submit records to workflows"}, {"workflow.approve", "Approve or reject workflow tasks"},
		{"knowledge.view", "View knowledge documents"}, {"knowledge.manage", "Upload and delete knowledge documents"},
		{"knowledge.search", "Search tenant knowledge"}, {"ai.chat", "Use the tenant AI assistant"},
		{"billing.manage", "Manage tenant subscription and billing"}, {"dashboard.manage", "Configure the enterprise dashboard"},
		{"file.view", "View and download tenant files"}, {"file.manage", "Upload and delete tenant files"},
	}
	createdPermissions := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		permission := model.BootstrapPermission{ID: newUUID(), Name: definition.Name, Description: definition.Description, CreatedAt: now}
		result := db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "name"}}, DoNothing: true}).Create(&permission)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 1 {
			createdPermissions = append(createdPermissions, definition.Name)
		}
	}
	if len(createdPermissions) == 0 {
		return nil
	}
	var tenants []model.Tenant
	if err := db.Where("status = ?", "active").Find(&tenants).Error; err != nil {
		return err
	}
	grants := map[string][]string{
		"super_admin": {"entity.view", "entity.manage", "record.view", "record.manage", "form.view", "form.manage", "form.submit", "workflow.view", "workflow.manage", "workflow.submit", "workflow.approve", "knowledge.view", "knowledge.manage", "knowledge.search", "ai.chat", "billing.manage", "dashboard.manage", "file.view", "file.manage"},
		"admin":       {"entity.view", "entity.manage", "record.view", "record.manage", "form.view", "form.manage", "form.submit", "workflow.view", "workflow.manage", "workflow.submit", "workflow.approve", "knowledge.view", "knowledge.manage", "knowledge.search", "ai.chat", "billing.manage", "dashboard.manage", "file.view", "file.manage"},
		"employee":    {"entity.view", "record.view", "form.view", "form.submit", "workflow.view", "workflow.submit", "knowledge.view", "knowledge.search", "ai.chat", "file.view"},
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
			if err := db.Where("name = ?", roleName).First(&role).Error; err != nil {
				return err
			}
			var permissions []model.BootstrapPermission
			if err := db.Where("name IN ?", names).Find(&permissions).Error; err != nil {
				return err
			}
			for _, permission := range permissions {
				if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.TenantRolePermission{TenantID: tenant.ID, RoleID: role.ID, PermissionID: permission.ID, CreatedAt: now}).Error; err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func backfillPrimaryTenant(db *gorm.DB) error {
	var tenant model.Tenant
	if err := db.Order("created_at").First(&tenant).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		var company model.BootstrapCompany
		if err := db.Order("created_at").First(&company).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		now := time.Now().UTC()
		tenant = model.Tenant{ID: newUUID(), Slug: tenantSlug(company.Name), Name: company.Name, Status: "active", Version: 1, CreatedAt: now, UpdatedAt: now}
		if err := db.Create(&tenant).Error; err != nil {
			return err
		}
		if err := db.Model(&company).Update("tenant_id", tenant.ID).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	var legacy []model.BootstrapUserRole
	if err := db.Find(&legacy).Error; err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, binding := range legacy {
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.TenantMembership{TenantID: tenant.ID, UserID: binding.UserID, Status: "active", CreatedAt: now}).Error; err != nil {
			return err
		}
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.TenantUserRole{TenantID: tenant.ID, UserID: binding.UserID, RoleID: binding.RoleID, CreatedAt: binding.CreatedAt}).Error; err != nil {
			return err
		}
	}
	var legacyPermissions []model.BootstrapRolePermission
	if err := db.Find(&legacyPermissions).Error; err != nil {
		return err
	}
	for _, binding := range legacyPermissions {
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.TenantRolePermission{TenantID: tenant.ID, RoleID: binding.RoleID, PermissionID: binding.PermissionID, CreatedAt: binding.CreatedAt}).Error; err != nil {
			return err
		}
	}
	return db.Model(&model.RefreshSession{}).Where("tenant_id = ?", "00000000-0000-0000-0000-000000000000").Update("tenant_id", tenant.ID).Error
}

func enableTenantRowLevelSecurity(db *gorm.DB) error {
	tables := []string{
		"companies", "tenant_settings", "tenant_memberships", "tenant_user_roles", "tenant_role_permissions", "refresh_sessions",
		"entities", "entity_fields", "dynamic_records", "forms", "form_submissions", "workflows", "workflow_instances", "workflow_actions", "notifications",
		"knowledge_documents", "knowledge_chunks", "knowledge_ingestion_jobs", "ai_conversations", "ai_messages", "subscriptions", "usage_counters", "file_assets", "dashboards", "audit_events",
	}
	for _, table := range tables {
		if err := db.Exec(fmt.Sprintf("ALTER TABLE %s ENABLE ROW LEVEL SECURITY", table)).Error; err != nil {
			return fmt.Errorf("enable tenant RLS on %s: %w", table, err)
		}
		if err := db.Exec(fmt.Sprintf("ALTER TABLE %s FORCE ROW LEVEL SECURITY", table)).Error; err != nil {
			return fmt.Errorf("force tenant RLS on %s: %w", table, err)
		}
		if err := db.Exec(fmt.Sprintf("DROP POLICY IF EXISTS tenant_isolation ON %s", table)).Error; err != nil {
			return err
		}
		policy := fmt.Sprintf("CREATE POLICY tenant_isolation ON %s USING (current_setting('app.system_access', true) = 'on' OR tenant_id::text = current_setting('app.current_tenant_id', true)) WITH CHECK (current_setting('app.system_access', true) = 'on' OR tenant_id::text = current_setting('app.current_tenant_id', true))", table)
		if err := db.Exec(policy).Error; err != nil {
			return fmt.Errorf("create tenant RLS policy on %s: %w", table, err)
		}
	}
	return nil
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
