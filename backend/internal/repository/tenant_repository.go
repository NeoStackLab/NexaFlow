package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
	"github.com/NeoStackLab/NexaFlow/backend/internal/tenantcontext"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

var (
	ErrTenantNotFound         = errors.New("tenant not found")
	ErrTenantVersionConflict  = errors.New("tenant version conflict")
	ErrTenantLifecycle        = errors.New("invalid tenant lifecycle transition")
	ErrPrimaryTenantProtected = errors.New("primary tenant lifecycle is protected")
	ErrCustomDomainConflict   = errors.New("custom domain is already assigned")
)

type TenantRepository interface {
	Get(context.Context) (model.TenantView, error)
	Update(context.Context, model.UpdateTenantInput) (model.TenantView, error)
	Transition(context.Context, string) (model.TenantView, error)
	Settings(context.Context) (model.TenantSettingsView, error)
	UpdateSettings(context.Context, model.UpdateTenantSettingsInput) (model.TenantSettingsView, error)
}

type tenantRepository struct{ install InstallRepository }

func NewTenantRepository(install InstallRepository) TenantRepository {
	return &tenantRepository{install: install}
}

func (r *tenantRepository) Get(ctx context.Context) (model.TenantView, error) {
	scope, err := tenantcontext.Require(ctx)
	if err != nil {
		return model.TenantView{}, err
	}
	var tenant model.Tenant
	var settings model.TenantSettings
	err = withInstallDB(ctx, r.install, func(db *gorm.DB) error {
		if err := db.Where("id = ? AND status <> ?", scope.TenantID, "deleted").First(&tenant).Error; err != nil {
			return mapTenantRecordError(err)
		}
		if err := db.Where("tenant_id = ?", scope.TenantID).First(&settings).Error; err != nil {
			return mapTenantRecordError(err)
		}
		return nil
	})
	if err != nil {
		return model.TenantView{}, err
	}
	settingsView, err := tenantSettingsView(settings)
	if err != nil {
		return model.TenantView{}, err
	}
	return tenantView(tenant, settingsView), nil
}

func (r *tenantRepository) Update(ctx context.Context, input model.UpdateTenantInput) (model.TenantView, error) {
	scope, err := tenantcontext.Require(ctx)
	if err != nil {
		return model.TenantView{}, err
	}
	err = withInstallDB(ctx, r.install, func(db *gorm.DB) error {
		result := db.Model(&model.Tenant{}).Where("id = ? AND status = ? AND version = ?", scope.TenantID, "active", input.ExpectedVersion).Updates(map[string]any{"name": input.Name, "version": gorm.Expr("version + 1"), "updated_at": time.Now().UTC()})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrTenantVersionConflict
		}
		return nil
	})
	if err != nil {
		return model.TenantView{}, err
	}
	return r.Get(ctx)
}

func (r *tenantRepository) Transition(ctx context.Context, target string) (model.TenantView, error) {
	scope, err := tenantcontext.Require(ctx)
	if err != nil {
		return model.TenantView{}, err
	}
	err = withInstallDB(ctx, r.install, func(db *gorm.DB) error {
		var primaryID string
		if err := db.Table("system_settings").Select("value").Where("key = ?", "tenant.primary_id").Scan(&primaryID).Error; err != nil {
			return err
		}
		if primaryID == scope.TenantID {
			return ErrPrimaryTenantProtected
		}
		var tenant model.Tenant
		if err := db.Where("id = ?", scope.TenantID).First(&tenant).Error; err != nil {
			return mapTenantRecordError(err)
		}
		allowed := (tenant.Status == "active" && target == "suspended") || ((tenant.Status == "active" || tenant.Status == "suspended") && target == "archived") || ((tenant.Status == "active" || tenant.Status == "suspended" || tenant.Status == "archived") && target == "deleted")
		if !allowed {
			return ErrTenantLifecycle
		}
		now := time.Now().UTC()
		updates := map[string]any{"status": target, "version": gorm.Expr("version + 1"), "updated_at": now}
		if target == "deleted" {
			updates["deleted_at"] = &now
		}
		if err := db.Model(&tenant).Updates(updates).Error; err != nil {
			return err
		}
		return db.Model(&model.RefreshSession{}).Where("tenant_id = ? AND revoked_at IS NULL", scope.TenantID).Update("revoked_at", now).Error
	})
	if err != nil {
		return model.TenantView{}, err
	}
	if target == "deleted" {
		return model.TenantView{ID: scope.TenantID, Status: target}, nil
	}
	return r.Get(ctx)
}

func (r *tenantRepository) Settings(ctx context.Context) (model.TenantSettingsView, error) {
	scope, err := tenantcontext.Require(ctx)
	if err != nil {
		return model.TenantSettingsView{}, err
	}
	var settings model.TenantSettings
	err = withInstallDB(ctx, r.install, func(db *gorm.DB) error {
		return db.Where("tenant_id = ?", scope.TenantID).First(&settings).Error
	})
	if err != nil {
		return model.TenantSettingsView{}, mapTenantRecordError(err)
	}
	return tenantSettingsView(settings)
}

func (r *tenantRepository) UpdateSettings(ctx context.Context, input model.UpdateTenantSettingsInput) (model.TenantSettingsView, error) {
	scope, err := tenantcontext.Require(ctx)
	if err != nil {
		return model.TenantSettingsView{}, err
	}
	branding, err := json.Marshal(input.Branding)
	if err != nil {
		return model.TenantSettingsView{}, err
	}
	security, err := json.Marshal(input.Security)
	if err != nil {
		return model.TenantSettingsView{}, err
	}
	var domain *string
	if input.CustomDomain != "" {
		domain = &input.CustomDomain
	}
	err = withInstallDB(ctx, r.install, func(db *gorm.DB) error {
		result := db.Model(&model.TenantSettings{}).Where("tenant_id = ? AND version = ?", scope.TenantID, input.ExpectedVersion).Updates(map[string]any{"timezone": input.Timezone, "locale": input.Locale, "branding": branding, "custom_domain": domain, "security": security, "version": gorm.Expr("version + 1"), "updated_at": time.Now().UTC()})
		if result.Error != nil {
			var postgresError *pgconn.PgError
			if errors.As(result.Error, &postgresError) && postgresError.Code == "23505" && postgresError.ConstraintName == "idx_tenant_settings_custom_domain" {
				return ErrCustomDomainConflict
			}
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrTenantVersionConflict
		}
		return nil
	})
	if err != nil {
		return model.TenantSettingsView{}, err
	}
	return r.Settings(ctx)
}

func tenantView(tenant model.Tenant, settings model.TenantSettingsView) model.TenantView {
	return model.TenantView{ID: tenant.ID, Slug: tenant.Slug, Name: tenant.Name, Status: tenant.Status, Version: tenant.Version, CreatedAt: tenant.CreatedAt, UpdatedAt: tenant.UpdatedAt, Settings: settings}
}

func tenantSettingsView(settings model.TenantSettings) (model.TenantSettingsView, error) {
	view := model.TenantSettingsView{Timezone: settings.Timezone, Locale: settings.Locale, Version: settings.Version, UpdatedAt: settings.UpdatedAt}
	if settings.CustomDomain != nil {
		view.CustomDomain = *settings.CustomDomain
	}
	if err := json.Unmarshal(settings.Branding, &view.Branding); err != nil {
		return model.TenantSettingsView{}, err
	}
	if err := json.Unmarshal(settings.Security, &view.Security); err != nil {
		return model.TenantSettingsView{}, err
	}
	return view, nil
}

func mapTenantRecordError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrTenantNotFound
	}
	return err
}
