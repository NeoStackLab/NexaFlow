package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/config"
	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/database"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrUserNotFound    = errors.New("user not found")
	ErrUserExists      = errors.New("user already exists")
	ErrSessionNotFound = errors.New("session not found")
	ErrTenantAccess    = errors.New("tenant membership not found")
)

type AuthRepository interface {
	CreateUser(ctx context.Context, username, email, passwordHash string) (model.AuthUser, error)
	FindCredentials(ctx context.Context, email string) (model.BootstrapUser, error)
	ResolveTenant(ctx context.Context, userID, tenantID string) (model.TenantSummary, error)
	AuthUser(ctx context.Context, userID, tenantID string) (model.AuthUser, error)
	CreateSession(ctx context.Context, session model.RefreshSession) error
	RotateSession(ctx context.Context, oldHash, targetTenantID string, next model.RefreshSession) (string, string, error)
	RevokeSession(ctx context.Context, tokenHash string) error
	ListSessions(ctx context.Context, userID, tenantID string) ([]model.RefreshSession, error)
	RevokeSessionByID(ctx context.Context, userID, tenantID, sessionID string) error
	ListRoles(ctx context.Context, tenantID string) ([]model.RoleView, error)
	ListPermissions(ctx context.Context) ([]model.PermissionView, error)
	SetRolePermissions(ctx context.Context, tenantID, roleID string, permissions []string) error
	ListUsers(ctx context.Context, tenantID string) ([]model.UserSummary, error)
	SetUserRoles(ctx context.Context, tenantID, userID string, roles []string) error
	ListTenants(ctx context.Context, userID string) ([]model.TenantSummary, error)
	CreateTenant(ctx context.Context, ownerUserID string, input model.CreateTenantInput) (model.TenantSummary, error)
}

type authRepository struct{ install InstallRepository }

func NewAuthRepository(install InstallRepository) AuthRepository {
	return &authRepository{install: install}
}

func (r *authRepository) withDB(ctx context.Context, fn func(*gorm.DB) error) error {
	runtimeConfig, err := r.install.RuntimeConfig()
	if err != nil {
		return fmt.Errorf("load installed database configuration: %w", err)
	}
	db, err := database.Open(ctx, config.DatabaseConfig{Host: runtimeConfig.Database.Host, Port: runtimeConfig.Database.Port, User: runtimeConfig.Database.User, Password: runtimeConfig.Database.Password, Name: runtimeConfig.Database.Name, SSLMode: runtimeConfig.Database.SSLMode, MaxOpenConnections: 5, MaxIdleConnections: 1, ConnectionMaxLifetime: 5 * time.Minute})
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	return fn(db.WithContext(ctx))
}

func (r *authRepository) CreateUser(ctx context.Context, username, email, passwordHash string) (model.AuthUser, error) {
	var result model.AuthUser
	err := r.withDB(ctx, func(db *gorm.DB) error {
		return db.Transaction(func(tx *gorm.DB) error {
			var count int64
			if err := tx.Model(&model.BootstrapUser{}).Where("email = ? OR username = ?", email, username).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return ErrUserExists
			}
			var tenant model.Tenant
			if err := tx.Joins("JOIN system_settings s ON s.value = tenants.id::text AND s.key = 'tenant.primary_id'").Where("tenants.status = ?", "active").First(&tenant).Error; err != nil {
				return fmt.Errorf("load primary tenant: %w", err)
			}
			var role model.BootstrapRole
			if err := tx.Where("name = ?", "employee").First(&role).Error; err != nil {
				return fmt.Errorf("load employee role: %w", err)
			}
			now := time.Now().UTC()
			user := model.BootstrapUser{ID: newUUID(), Username: username, Email: strings.ToLower(email), PasswordHash: passwordHash, Status: "active", CreatedAt: now, UpdatedAt: now}
			if err := tx.Create(&user).Error; err != nil {
				return err
			}
			if err := tx.Create(&model.TenantMembership{TenantID: tenant.ID, UserID: user.ID, Status: "active", CreatedAt: now}).Error; err != nil {
				return err
			}
			if err := tx.Create(&model.TenantUserRole{TenantID: tenant.ID, UserID: user.ID, RoleID: role.ID, CreatedAt: now}).Error; err != nil {
				return err
			}
			result = model.AuthUser{ID: user.ID, Username: user.Username, Email: user.Email, Status: user.Status, ActiveTenantID: tenant.ID, Tenants: []model.TenantSummary{{ID: tenant.ID, Slug: tenant.Slug, Name: tenant.Name}}, Roles: []string{"employee"}, Permissions: []string{}}
			return nil
		})
	})
	return result, err
}

func (r *authRepository) FindCredentials(ctx context.Context, email string) (model.BootstrapUser, error) {
	var user model.BootstrapUser
	err := r.withDB(ctx, func(db *gorm.DB) error {
		err := db.Where("email = ?", strings.ToLower(email)).First(&user).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return err
	})
	return user, err
}

func (r *authRepository) ResolveTenant(ctx context.Context, userID, tenantID string) (model.TenantSummary, error) {
	var result model.TenantSummary
	err := r.withDB(ctx, func(db *gorm.DB) error {
		query := db.Table("tenants t").Select("t.id, t.slug, t.name").Joins("JOIN tenant_memberships tm ON tm.tenant_id = t.id").Where("tm.user_id = ? AND tm.status = ? AND t.status = ?", userID, "active", "active")
		if tenantID != "" {
			query = query.Where("t.id = ?", tenantID)
		}
		if err := query.Order("tm.created_at").First(&result).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrTenantAccess
			}
			return err
		}
		return nil
	})
	return result, err
}

func (r *authRepository) AuthUser(ctx context.Context, userID, tenantID string) (model.AuthUser, error) {
	var result model.AuthUser
	err := r.withDB(ctx, func(db *gorm.DB) error {
		var user model.BootstrapUser
		if err := db.First(&user, "id = ?", userID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrUserNotFound
			}
			return err
		}
		var membership int64
		if err := db.Model(&model.TenantMembership{}).Where("tenant_id = ? AND user_id = ? AND status = ?", tenantID, userID, "active").Count(&membership).Error; err != nil {
			return err
		}
		if membership != 1 {
			return ErrTenantAccess
		}
		result = model.AuthUser{ID: user.ID, Username: user.Username, Email: user.Email, Status: user.Status, ActiveTenantID: tenantID, Roles: []string{}, Permissions: []string{}, Tenants: []model.TenantSummary{}}
		if err := db.Table("roles r").Select("r.name").Joins("JOIN tenant_user_roles tur ON tur.role_id = r.id").Where("tur.tenant_id = ? AND tur.user_id = ?", tenantID, userID).Order("r.name").Scan(&result.Roles).Error; err != nil {
			return err
		}
		if err := db.Table("permissions p").Distinct("p.name").Joins("JOIN tenant_role_permissions trp ON trp.permission_id = p.id").Joins("JOIN tenant_user_roles tur ON tur.role_id = trp.role_id AND tur.tenant_id = trp.tenant_id").Where("tur.tenant_id = ? AND tur.user_id = ?", tenantID, userID).Order("p.name").Scan(&result.Permissions).Error; err != nil {
			return err
		}
		return db.Table("tenants t").Select("t.id, t.slug, t.name").Joins("JOIN tenant_memberships tm ON tm.tenant_id = t.id").Where("tm.user_id = ? AND tm.status = ? AND t.status = ?", userID, "active", "active").Order("tm.created_at").Scan(&result.Tenants).Error
	})
	return result, err
}

func (r *authRepository) CreateSession(ctx context.Context, session model.RefreshSession) error {
	return r.withDB(ctx, func(db *gorm.DB) error { return db.Create(&session).Error })
}

func (r *authRepository) RotateSession(ctx context.Context, oldHash, targetTenantID string, next model.RefreshSession) (string, string, error) {
	var userID, tenantID string
	err := r.withDB(ctx, func(db *gorm.DB) error {
		return db.Transaction(func(tx *gorm.DB) error {
			var current model.RefreshSession
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("token_hash = ? AND revoked_at IS NULL AND expires_at > ?", oldHash, time.Now().UTC()).First(&current).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrSessionNotFound
				}
				return err
			}
			tenantID = current.TenantID
			if targetTenantID != "" {
				tenantID = targetTenantID
			}
			var membership int64
			if err := tx.Model(&model.TenantMembership{}).Where("tenant_id = ? AND user_id = ? AND status = ?", tenantID, current.UserID, "active").Count(&membership).Error; err != nil {
				return err
			}
			if membership != 1 {
				return ErrTenantAccess
			}
			now := time.Now().UTC()
			if err := tx.Model(&current).Update("revoked_at", now).Error; err != nil {
				return err
			}
			next.UserID, next.TenantID, userID = current.UserID, tenantID, current.UserID
			return tx.Create(&next).Error
		})
	})
	return userID, tenantID, err
}

func (r *authRepository) RevokeSession(ctx context.Context, tokenHash string) error {
	return r.withDB(ctx, func(db *gorm.DB) error {
		now := time.Now().UTC()
		return db.Model(&model.RefreshSession{}).Where("token_hash = ? AND revoked_at IS NULL", tokenHash).Update("revoked_at", now).Error
	})
}
func (r *authRepository) ListSessions(ctx context.Context, userID, tenantID string) ([]model.RefreshSession, error) {
	var sessions []model.RefreshSession
	err := r.withDB(ctx, func(db *gorm.DB) error {
		return db.Where("user_id = ? AND tenant_id = ? AND revoked_at IS NULL AND expires_at > ?", userID, tenantID, time.Now().UTC()).Order("created_at DESC").Find(&sessions).Error
	})
	return sessions, err
}
func (r *authRepository) RevokeSessionByID(ctx context.Context, userID, tenantID, sessionID string) error {
	return r.withDB(ctx, func(db *gorm.DB) error {
		now := time.Now().UTC()
		result := db.Model(&model.RefreshSession{}).Where("id = ? AND user_id = ? AND tenant_id = ? AND revoked_at IS NULL", sessionID, userID, tenantID).Update("revoked_at", now)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrSessionNotFound
		}
		return nil
	})
}

func (r *authRepository) ListRoles(ctx context.Context, tenantID string) ([]model.RoleView, error) {
	var roles []model.RoleView
	err := r.withDB(ctx, func(db *gorm.DB) error {
		if err := db.Model(&model.BootstrapRole{}).Select("id, name, display_name").Order("name").Scan(&roles).Error; err != nil {
			return err
		}
		for index := range roles {
			roles[index].Permissions = []string{}
			if err := db.Table("permissions p").Select("p.name").Joins("JOIN tenant_role_permissions trp ON trp.permission_id = p.id").Where("trp.tenant_id = ? AND trp.role_id = ?", tenantID, roles[index].ID).Order("p.name").Scan(&roles[index].Permissions).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return roles, err
}
func (r *authRepository) ListPermissions(ctx context.Context) ([]model.PermissionView, error) {
	var permissions []model.PermissionView
	err := r.withDB(ctx, func(db *gorm.DB) error {
		return db.Model(&model.BootstrapPermission{}).Select("id, name, description").Order("name").Scan(&permissions).Error
	})
	return permissions, err
}
func (r *authRepository) SetRolePermissions(ctx context.Context, tenantID, roleID string, names []string) error {
	return r.withDB(ctx, func(db *gorm.DB) error {
		return db.Transaction(func(tx *gorm.DB) error {
			var role model.BootstrapRole
			if err := tx.First(&role, "id = ?", roleID).Error; err != nil {
				return err
			}
			if role.Name == "super_admin" {
				return errors.New("super_admin permissions cannot be reduced")
			}
			var permissions []model.BootstrapPermission
			if len(names) > 0 {
				if err := tx.Where("name IN ?", names).Find(&permissions).Error; err != nil {
					return err
				}
				if len(permissions) != len(names) {
					return errors.New("one or more permissions do not exist")
				}
			}
			if err := tx.Where("tenant_id = ? AND role_id = ?", tenantID, roleID).Delete(&model.TenantRolePermission{}).Error; err != nil {
				return err
			}
			now := time.Now().UTC()
			for _, permission := range permissions {
				if err := tx.Create(&model.TenantRolePermission{TenantID: tenantID, RoleID: roleID, PermissionID: permission.ID, CreatedAt: now}).Error; err != nil {
					return err
				}
			}
			return nil
		})
	})
}
func (r *authRepository) ListUsers(ctx context.Context, tenantID string) ([]model.UserSummary, error) {
	var users []model.UserSummary
	err := r.withDB(ctx, func(db *gorm.DB) error {
		if err := db.Table("users u").Select("u.id, u.username, u.email, u.status").Joins("JOIN tenant_memberships tm ON tm.user_id = u.id").Where("tm.tenant_id = ? AND tm.status = ?", tenantID, "active").Order("tm.created_at").Scan(&users).Error; err != nil {
			return err
		}
		for index := range users {
			users[index].Roles = []string{}
			if err := db.Table("roles r").Select("r.name").Joins("JOIN tenant_user_roles tur ON tur.role_id = r.id").Where("tur.tenant_id = ? AND tur.user_id = ?", tenantID, users[index].ID).Order("r.name").Scan(&users[index].Roles).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return users, err
}
func (r *authRepository) SetUserRoles(ctx context.Context, tenantID, userID string, roleNames []string) error {
	if len(roleNames) == 0 {
		return errors.New("at least one role is required")
	}
	return r.withDB(ctx, func(db *gorm.DB) error {
		return db.Transaction(func(tx *gorm.DB) error {
			var member int64
			if err := tx.Model(&model.TenantMembership{}).Where("tenant_id = ? AND user_id = ? AND status = ?", tenantID, userID, "active").Count(&member).Error; err != nil {
				return err
			}
			if member != 1 {
				return ErrTenantAccess
			}
			var roles []model.BootstrapRole
			if err := tx.Where("name IN ?", roleNames).Find(&roles).Error; err != nil {
				return err
			}
			if len(roles) != len(roleNames) {
				return errors.New("one or more roles do not exist")
			}
			if err := tx.Where("tenant_id = ? AND user_id = ?", tenantID, userID).Delete(&model.TenantUserRole{}).Error; err != nil {
				return err
			}
			now := time.Now().UTC()
			for _, role := range roles {
				if err := tx.Create(&model.TenantUserRole{TenantID: tenantID, UserID: userID, RoleID: role.ID, CreatedAt: now}).Error; err != nil {
					return err
				}
			}
			return nil
		})
	})
}

func (r *authRepository) ListTenants(ctx context.Context, userID string) ([]model.TenantSummary, error) {
	var tenants []model.TenantSummary
	err := r.withDB(ctx, func(db *gorm.DB) error {
		return db.Table("tenants t").Select("t.id, t.slug, t.name").Joins("JOIN tenant_memberships tm ON tm.tenant_id = t.id").Where("tm.user_id = ? AND tm.status = ? AND t.status = ?", userID, "active", "active").Order("tm.created_at").Scan(&tenants).Error
	})
	return tenants, err
}
func (r *authRepository) CreateTenant(ctx context.Context, ownerUserID string, input model.CreateTenantInput) (model.TenantSummary, error) {
	var result model.TenantSummary
	err := r.withDB(ctx, func(db *gorm.DB) error {
		return db.Transaction(func(tx *gorm.DB) error {
			now := time.Now().UTC()
			tenant := model.Tenant{ID: newUUID(), Slug: input.Slug, Name: input.Name, Status: "active", CreatedAt: now, UpdatedAt: now}
			if err := tx.Create(&tenant).Error; err != nil {
				return err
			}
			if err := tx.Create(&model.TenantMembership{TenantID: tenant.ID, UserID: ownerUserID, Status: "active", CreatedAt: now}).Error; err != nil {
				return err
			}
			var super model.BootstrapRole
			if err := tx.Where("name = ?", "super_admin").First(&super).Error; err != nil {
				return err
			}
			if err := tx.Create(&model.TenantUserRole{TenantID: tenant.ID, UserID: ownerUserID, RoleID: super.ID, CreatedAt: now}).Error; err != nil {
				return err
			}
			defaultGrants := map[string][]string{
				"super_admin": {"dashboard.view", "dashboard.manage", "user.view", "user.create", "user.delete", "role.manage", "order.view", "finance.manage", "system.manage", "entity.view", "entity.manage", "record.view", "record.manage", "form.view", "form.manage", "workflow.view", "workflow.manage", "workflow.submit", "workflow.approve", "knowledge.view", "knowledge.manage", "knowledge.search", "ai.chat", "billing.manage", "file.view", "file.manage"},
				"admin":       {"dashboard.view", "dashboard.manage", "user.view", "user.create", "order.view", "entity.view", "entity.manage", "record.view", "record.manage", "form.view", "form.manage", "workflow.view", "workflow.manage", "workflow.submit", "workflow.approve", "knowledge.view", "knowledge.manage", "knowledge.search", "ai.chat", "billing.manage", "file.view", "file.manage"},
				"employee":    {"dashboard.view", "order.view", "entity.view", "record.view", "form.view", "workflow.view", "workflow.submit", "knowledge.view", "knowledge.search", "ai.chat", "file.view"},
				"guest":       {"dashboard.view"},
			}
			for roleName, permissionNames := range defaultGrants {
				var role model.BootstrapRole
				if err := tx.Where("name = ?", roleName).First(&role).Error; err != nil {
					return err
				}
				var permissions []model.BootstrapPermission
				if err := tx.Where("name IN ?", permissionNames).Find(&permissions).Error; err != nil {
					return err
				}
				if len(permissions) != len(permissionNames) {
					return errors.New("default tenant permission vocabulary is incomplete")
				}
				for _, permission := range permissions {
					if err := tx.Create(&model.TenantRolePermission{TenantID: tenant.ID, RoleID: role.ID, PermissionID: permission.ID, CreatedAt: now}).Error; err != nil {
						return err
					}
				}
			}
			result = model.TenantSummary{ID: tenant.ID, Slug: tenant.Slug, Name: tenant.Name}
			return nil
		})
	})
	return result, err
}
