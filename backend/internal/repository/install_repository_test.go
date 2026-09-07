package repository

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestLoadBootstrapPermissionByNameIgnoresUnpersistedCandidateID(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer sqlDB.Close()

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB, PreferSimpleProtocol: true}), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	query := `SELECT * FROM "permissions" WHERE name = $1 ORDER BY "permissions"."id" LIMIT $2`
	mock.ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs("dashboard.manage", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "description", "created_at"}).
			AddRow("persisted-id", "dashboard.manage", "Configure the enterprise dashboard", time.Now()))

	permission, err := loadBootstrapPermissionByName(db, model.BootstrapPermission{
		ID:   "unpersisted-candidate-id",
		Name: "dashboard.manage",
	})
	if err != nil {
		t.Fatalf("loadBootstrapPermissionByName() error = %v", err)
	}
	if permission.ID != "persisted-id" {
		t.Fatalf("permission ID = %q, want persisted-id", permission.ID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("database expectations = %v", err)
	}
}
