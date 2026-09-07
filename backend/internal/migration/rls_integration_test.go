//go:build integration

package migration

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestMigrationsEnforceTenantRLS(t *testing.T) {
	adminDSN := strings.TrimSpace(os.Getenv("NEXAFLOW_TEST_ADMIN_DSN"))
	if adminDSN == "" {
		t.Skip("NEXAFLOW_TEST_ADMIN_DSN is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	adminConfig, err := pgx.ParseConfig(adminDSN)
	if err != nil {
		t.Fatalf("parse admin DSN: %v", err)
	}
	suffix := strings.ReplaceAll(newUUID(), "-", "")[:12]
	databaseName := "nexaflow_migration_" + suffix
	roleName := "nexaflow_app_" + suffix
	rolePassword := "integration-only-password"

	maintenance, err := sql.Open("pgx", adminConfig.ConnString())
	if err != nil {
		t.Fatalf("open maintenance database: %v", err)
	}
	defer maintenance.Close()
	if err := maintenance.PingContext(ctx); err != nil {
		t.Fatalf("ping maintenance database: %v", err)
	}
	if _, err := maintenance.ExecContext(ctx, fmt.Sprintf(`CREATE ROLE "%s" LOGIN PASSWORD '%s' NOSUPERUSER NOBYPASSRLS`, roleName, rolePassword)); err != nil {
		t.Fatalf("create application role: %v", err)
	}
	defer func() {
		_, _ = maintenance.ExecContext(context.Background(), fmt.Sprintf(`DROP ROLE IF EXISTS "%s"`, roleName))
	}()
	if _, err := maintenance.ExecContext(ctx, fmt.Sprintf(`CREATE DATABASE "%s" OWNER "%s"`, databaseName, roleName)); err != nil {
		t.Fatalf("create migration database: %v", err)
	}
	defer func() {
		_, _ = maintenance.ExecContext(context.Background(), "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1 AND pid <> pg_backend_pid()", databaseName)
		_, _ = maintenance.ExecContext(context.Background(), fmt.Sprintf(`DROP DATABASE IF EXISTS "%s"`, databaseName))
	}()

	targetAdminConfig := adminConfig.Copy()
	targetAdminConfig.Database = databaseName
	targetAdmin, err := sql.Open("pgx", targetAdminConfig.ConnString())
	if err != nil {
		t.Fatalf("open target admin connection: %v", err)
	}
	if _, err := targetAdmin.ExecContext(ctx, "CREATE EXTENSION IF NOT EXISTS vector"); err != nil {
		targetAdmin.Close()
		t.Fatalf("install vector extension: %v", err)
	}
	if err := targetAdmin.Close(); err != nil {
		t.Fatalf("close target admin connection: %v", err)
	}

	appConfig := targetAdminConfig.Copy()
	appConfig.User = roleName
	appConfig.Password = rolePassword
	db, err := gorm.Open(postgres.Open(appConfig.ConnString()), &gorm.Config{})
	if err != nil {
		t.Fatalf("open application database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get application connection pool: %v", err)
	}
	defer sqlDB.Close()

	report, err := DefaultRunner().Run(ctx, db)
	if err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	if report.CurrentVersion != int64(len(defaultMigrations())) {
		t.Fatalf("current migration version = %d, want %d", report.CurrentVersion, len(defaultMigrations()))
	}

	tenantA := "10000000-0000-0000-0000-000000000001"
	tenantB := "20000000-0000-0000-0000-000000000002"
	entityA := integrationEntity("30000000-0000-0000-0000-000000000003", tenantA, "tenant-a")
	entityB := integrationEntity("40000000-0000-0000-0000-000000000004", tenantB, "tenant-b")
	if err := withSystemContext(db, func(tx *gorm.DB) error {
		now := time.Now().UTC()
		if err := tx.Create(&[]model.Tenant{
			{ID: tenantA, Slug: "tenant-a", Name: "Tenant A", Status: "active", Version: 1, CreatedAt: now, UpdatedAt: now},
			{ID: tenantB, Slug: "tenant-b", Name: "Tenant B", Status: "active", Version: 1, CreatedAt: now, UpdatedAt: now},
		}).Error; err != nil {
			return err
		}
		return tx.Create(&[]model.Entity{entityA, entityB}).Error
	}); err != nil {
		t.Fatalf("seed isolated records: %v", err)
	}

	if err := withTenantContext(db, tenantA, func(tx *gorm.DB) error {
		var gormCount int64
		if err := tx.Model(&model.Entity{}).Count(&gormCount).Error; err != nil {
			return err
		}
		if gormCount != 1 {
			return fmt.Errorf("tenant A GORM entity count = %d, want 1", gormCount)
		}
		var rawCount int64
		if err := tx.Raw("SELECT COUNT(*) FROM entities").Scan(&rawCount).Error; err != nil {
			return err
		}
		if rawCount != 1 {
			return fmt.Errorf("tenant A raw entity count = %d, want 1", rawCount)
		}
		result := tx.Model(&model.Entity{}).Where("id = ?", entityB.ID).Update("description", "cross-tenant")
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 0 {
			return fmt.Errorf("cross-tenant update affected %d rows, want 0", result.RowsAffected)
		}
		rawUpdate := tx.Exec("UPDATE entities SET description = ? WHERE id = ?", "raw-cross-tenant", entityB.ID)
		if rawUpdate.Error != nil || rawUpdate.RowsAffected != 0 {
			return fmt.Errorf("raw cross-tenant update affected %d rows, error = %v", rawUpdate.RowsAffected, rawUpdate.Error)
		}
		gormDelete := tx.Delete(&model.Entity{}, "id = ?", entityB.ID)
		if gormDelete.Error != nil || gormDelete.RowsAffected != 0 {
			return fmt.Errorf("GORM cross-tenant delete affected %d rows, error = %v", gormDelete.RowsAffected, gormDelete.Error)
		}
		rawDelete := tx.Exec("DELETE FROM entities WHERE id = ?", entityB.ID)
		if rawDelete.Error != nil || rawDelete.RowsAffected != 0 {
			return fmt.Errorf("raw cross-tenant delete affected %d rows, error = %v", rawDelete.RowsAffected, rawDelete.Error)
		}
		return nil
	}); err != nil {
		t.Fatalf("verify tenant-scoped reads and writes: %v", err)
	}

	wrongTenantEntity := integrationEntity("50000000-0000-0000-0000-000000000005", tenantB, "wrong-tenant")
	err = withTenantContext(db, tenantA, func(tx *gorm.DB) error { return tx.Create(&wrongTenantEntity).Error })
	if err == nil {
		t.Fatal("cross-tenant insert succeeded; expected RLS WITH CHECK rejection")
	}
	rawWrongTenantEntity := integrationEntity("60000000-0000-0000-0000-000000000006", tenantB, "raw-wrong-tenant")
	err = withTenantContext(db, tenantA, func(tx *gorm.DB) error {
		return tx.Exec(
			"INSERT INTO entities (id, tenant_id, name, slug, description, version, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
			rawWrongTenantEntity.ID, rawWrongTenantEntity.TenantID, rawWrongTenantEntity.Name, rawWrongTenantEntity.Slug,
			rawWrongTenantEntity.Description, rawWrongTenantEntity.Version, rawWrongTenantEntity.Status, rawWrongTenantEntity.CreatedAt, rawWrongTenantEntity.UpdatedAt,
		).Error
	})
	if err == nil {
		t.Fatal("raw cross-tenant insert succeeded; expected RLS WITH CHECK rejection")
	}

	if err := withSystemContext(db, func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&model.Entity{}).Count(&count).Error; err != nil {
			return err
		}
		if count != 2 {
			return fmt.Errorf("system entity count = %d, want 2", count)
		}
		return nil
	}); err != nil {
		t.Fatalf("verify system context: %v", err)
	}
}

func integrationEntity(id, tenantID, slug string) model.Entity {
	now := time.Now().UTC()
	return model.Entity{ID: id, TenantID: tenantID, Name: slug, Slug: slug, Description: slug, Version: 1, Status: "active", CreatedAt: now, UpdatedAt: now}
}

func withSystemContext(db *gorm.DB, operation func(*gorm.DB) error) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT set_config('app.system_access', 'on', true)").Error; err != nil {
			return err
		}
		return operation(tx)
	})
}

func withTenantContext(db *gorm.DB, tenantID string, operation func(*gorm.DB) error) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT set_config('app.current_tenant_id', ?, true)", tenantID).Error; err != nil {
			return err
		}
		return operation(tx)
	})
}
