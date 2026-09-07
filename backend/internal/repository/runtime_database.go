package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/config"
	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/database"
	"github.com/NeoStackLab/NexaFlow/backend/internal/tenantcontext"
	"gorm.io/gorm"
)

// RuntimeDBProvider is implemented by the installed repository so business
// repositories can share the application's database pool. The fallback path
// keeps lightweight repository doubles and bootstrap-only callers compatible.
type RuntimeDBProvider interface {
	RuntimeDB(ctx context.Context) (*gorm.DB, error)
	CloseRuntimeDB() error
}

func withInstallDB(ctx context.Context, install InstallRepository, fn func(*gorm.DB) error) error {
	if provider, ok := install.(RuntimeDBProvider); ok {
		db, err := provider.RuntimeDB(ctx)
		if err != nil {
			return err
		}
		return withTenantTransaction(ctx, db, fn)
	}

	runtimeConfig, err := install.RuntimeConfig()
	if err != nil {
		return fmt.Errorf("load installed database configuration: %w", err)
	}
	db, err := database.Open(ctx, config.DatabaseConfig{
		Host: runtimeConfig.Database.Host, Port: runtimeConfig.Database.Port,
		User: runtimeConfig.Database.User, Password: runtimeConfig.Database.Password,
		Name: runtimeConfig.Database.Name, SSLMode: runtimeConfig.Database.SSLMode,
		MaxOpenConnections: 5, MaxIdleConnections: 1,
		ConnectionMaxLifetime: 5 * time.Minute,
	})
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	return withTenantTransaction(ctx, db, fn)
}

func withTenantTransaction(ctx context.Context, db *gorm.DB, fn func(*gorm.DB) error) error {
	scope, ok := tenantcontext.From(ctx)
	if !ok || (!scope.System && (scope.TenantID == "" || scope.UserID == "")) {
		return tenantcontext.ErrRequired
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if scope.System {
			if err := tx.Exec("SELECT set_config('app.system_access', 'on', true)").Error; err != nil {
				return fmt.Errorf("set database system context: %w", err)
			}
		} else {
			if err := tx.Exec("SELECT set_config('app.current_tenant_id', ?, true)", scope.TenantID).Error; err != nil {
				return fmt.Errorf("set database tenant context: %w", err)
			}
			if err := tx.Exec("SELECT set_config('app.current_user_id', ?, true)", scope.UserID).Error; err != nil {
				return fmt.Errorf("set database user context: %w", err)
			}
		}
		return fn(tx)
	})
}
