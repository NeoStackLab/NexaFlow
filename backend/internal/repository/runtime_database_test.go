package repository

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/NeoStackLab/NexaFlow/backend/internal/tenantcontext"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestWithTenantTransactionSetsLocalDatabaseContext(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("SELECT set_config('app.current_tenant_id', $1, true)")).WithArgs("tenant-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("SELECT set_config('app.current_user_id', $1, true)")).WithArgs("user-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE dynamic_records SET status = 'archived'")).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	ctx := tenantcontext.With(context.Background(), tenantcontext.Scope{TenantID: "tenant-1", UserID: "user-1"})
	err = withTenantTransaction(ctx, db, func(tx *gorm.DB) error {
		return tx.Exec("UPDATE dynamic_records SET status = 'archived'").Error
	})
	if err != nil {
		t.Fatalf("tenant transaction: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestWithTenantTransactionFailsClosedWithoutContext(t *testing.T) {
	if err := withTenantTransaction(context.Background(), &gorm.DB{}, func(*gorm.DB) error { return nil }); !errors.Is(err, tenantcontext.ErrRequired) {
		t.Fatalf("expected tenant context error, got %v", err)
	}
}
