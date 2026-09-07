package migration

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestValidateRegistryRequiresContiguousOrderedMigrations(t *testing.T) {
	valid := func(version int64, name string) migration {
		return migration{version: version, name: name, checksum: strings.Repeat("a", 64), apply: func(*gorm.DB) error { return nil }}
	}
	tests := []struct {
		name  string
		items []migration
	}{
		{name: "empty"},
		{name: "starts after one", items: []migration{valid(2, "second")}},
		{name: "out of order", items: []migration{valid(2, "second"), valid(1, "first")}},
		{name: "invalid name", items: []migration{valid(1, "Invalid-Name")}},
		{name: "invalid checksum", items: []migration{{version: 1, name: "first", checksum: "short", apply: func(*gorm.DB) error { return nil }}}},
		{name: "non hexadecimal checksum", items: []migration{{version: 1, name: "first", checksum: strings.Repeat("z", 64), apply: func(*gorm.DB) error { return nil }}}},
		{name: "missing apply", items: []migration{{version: 1, name: "first", checksum: strings.Repeat("a", 64)}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := validateRegistry(test.items); !errors.Is(err, ErrInvalidRegistry) {
				t.Fatalf("validateRegistry() error = %v, want ErrInvalidRegistry", err)
			}
		})
	}
	if err := validateRegistry([]migration{valid(1, "first"), valid(2, "second")}); err != nil {
		t.Fatalf("validateRegistry(valid) error = %v", err)
	}
}

func TestEnsureTenantRLSSafeRoleRejectsBypassPrivileges(t *testing.T) {
	for _, test := range []struct {
		name      string
		superuser bool
		bypassRLS bool
		wantError bool
	}{
		{name: "application role"},
		{name: "superuser", superuser: true, wantError: true},
		{name: "bypass rls", bypassRLS: true, wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, mock, closeDB := mockDatabase(t)
			defer closeDB()
			query := "SELECT rolsuper, rolbypassrls FROM pg_roles WHERE rolname = current_user"
			mock.ExpectQuery(regexp.QuoteMeta(query)).WillReturnRows(sqlmock.NewRows([]string{"rolsuper", "rolbypassrls"}).AddRow(test.superuser, test.bypassRLS))
			err := ensureTenantRLSSafeRole(db)
			if test.wantError != (err != nil) {
				t.Fatalf("error = %v, wantError = %v", err, test.wantError)
			}
			assertExpectations(t, mock)
		})
	}
}

func TestRunnerAppliesPendingMigrationTransactionally(t *testing.T) {
	db, mock, closeDB := mockDatabase(t)
	defer closeDB()
	checksum := strings.Repeat("a", 64)
	started := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)
	appliedAt := started.Add(125 * time.Millisecond)
	times := []time.Time{started, appliedAt}
	runner := &Runner{
		migrations: []migration{{version: 1, name: "test_migration", checksum: checksum, apply: func(tx *gorm.DB) error {
			return tx.Exec("CREATE TABLE test_migration (id bigint PRIMARY KEY)").Error
		}}},
		now: func() time.Time {
			value := times[0]
			times = times[1:]
			return value
		},
	}

	mock.ExpectExec(regexp.QuoteMeta("SELECT pg_advisory_lock($1)")).WithArgs(advisoryLockKey).WillReturnResult(sqlmock.NewResult(0, 1))
	expectSafeRole(mock)
	expectLedger(mock)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT version, name, checksum FROM schema_migrations ORDER BY version")).WillReturnRows(sqlmock.NewRows([]string{"version", "name", "checksum"}))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("SELECT set_config('app.system_access', 'on', true)")).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE test_migration (id bigint PRIMARY KEY)")).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO schema_migrations (version, name, checksum, duration_ms, applied_at) VALUES ($1, $2, $3, $4, $5)")).
		WithArgs(int64(1), "test_migration", checksum, int64(125), appliedAt).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectExec(regexp.QuoteMeta("SELECT pg_advisory_unlock($1)")).WithArgs(advisoryLockKey).WillReturnResult(sqlmock.NewResult(0, 1))

	report, err := runner.Run(context.Background(), db)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.CurrentVersion != 1 || len(report.Applied) != 1 || report.Applied[0].Duration != 125*time.Millisecond {
		t.Fatalf("Run() report = %#v", report)
	}
	assertExpectations(t, mock)
}

func TestRunnerRejectsChangedAppliedMigration(t *testing.T) {
	db, mock, closeDB := mockDatabase(t)
	defer closeDB()
	runner := &Runner{migrations: []migration{{version: 1, name: "test_migration", checksum: strings.Repeat("a", 64), apply: func(*gorm.DB) error { return nil }}}, now: time.Now}

	expectSafeRole(mock)
	expectLedger(mock)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT version, name, checksum FROM schema_migrations ORDER BY version")).
		WillReturnRows(sqlmock.NewRows([]string{"version", "name", "checksum"}).AddRow(1, "test_migration", strings.Repeat("b", 64)))

	err := runner.runLocked(db, &Report{})
	if !errors.Is(err, ErrChecksumMismatch) {
		t.Fatalf("runLocked() error = %v, want ErrChecksumMismatch", err)
	}
	assertExpectations(t, mock)
}

func TestRunnerRollsBackFailedMigrationAndReleasesLock(t *testing.T) {
	db, mock, closeDB := mockDatabase(t)
	defer closeDB()
	applyError := errors.New("schema statement failed")
	runner := &Runner{
		migrations: []migration{{version: 1, name: "failed_migration", checksum: strings.Repeat("a", 64), apply: func(*gorm.DB) error { return applyError }}},
		now:        func() time.Time { return time.Date(2026, 8, 23, 11, 0, 0, 0, time.UTC) },
	}

	mock.ExpectExec(regexp.QuoteMeta("SELECT pg_advisory_lock($1)")).WithArgs(advisoryLockKey).WillReturnResult(sqlmock.NewResult(0, 1))
	expectSafeRole(mock)
	expectLedger(mock)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT version, name, checksum FROM schema_migrations ORDER BY version")).WillReturnRows(sqlmock.NewRows([]string{"version", "name", "checksum"}))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("SELECT set_config('app.system_access', 'on', true)")).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectRollback()
	mock.ExpectExec(regexp.QuoteMeta("SELECT pg_advisory_unlock($1)")).WithArgs(advisoryLockKey).WillReturnResult(sqlmock.NewResult(0, 1))

	_, err := runner.Run(context.Background(), db)
	if !errors.Is(err, applyError) {
		t.Fatalf("Run() error = %v, want wrapped apply error", err)
	}
	assertExpectations(t, mock)
}

func TestRunnerRejectsNonContiguousHistory(t *testing.T) {
	db, mock, closeDB := mockDatabase(t)
	defer closeDB()
	runner := &Runner{
		migrations: []migration{
			{version: 1, name: "first", checksum: strings.Repeat("a", 64), apply: func(*gorm.DB) error { return nil }},
			{version: 2, name: "second", checksum: strings.Repeat("b", 64), apply: func(*gorm.DB) error { return nil }},
		},
		now: time.Now,
	}

	expectSafeRole(mock)
	expectLedger(mock)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT version, name, checksum FROM schema_migrations ORDER BY version")).
		WillReturnRows(sqlmock.NewRows([]string{"version", "name", "checksum"}).AddRow(2, "second", strings.Repeat("b", 64)))

	err := runner.runLocked(db, &Report{})
	if !errors.Is(err, ErrMigrationHistory) {
		t.Fatalf("runLocked() error = %v, want ErrMigrationHistory", err)
	}
	assertExpectations(t, mock)
}

func mockDatabase(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, func()) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB, PreferSimpleProtocol: true}), &gorm.Config{})
	if err != nil {
		sqlDB.Close()
		t.Fatal(err)
	}
	return db, mock, func() { _ = sqlDB.Close() }
}

func expectSafeRole(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(regexp.QuoteMeta("SELECT rolsuper, rolbypassrls FROM pg_roles WHERE rolname = current_user")).
		WillReturnRows(sqlmock.NewRows([]string{"rolsuper", "rolbypassrls"}).AddRow(false, false))
}

func expectLedger(mock sqlmock.Sqlmock) {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS schema_migrations (version bigint PRIMARY KEY, name varchar(160) NOT NULL, checksum varchar(64), duration_ms bigint NOT NULL DEFAULT 0, applied_at timestamptz NOT NULL)`,
		`ALTER TABLE schema_migrations ADD COLUMN IF NOT EXISTS checksum varchar(64)`,
		`ALTER TABLE schema_migrations ADD COLUMN IF NOT EXISTS duration_ms bigint NOT NULL DEFAULT 0`,
	}
	for _, statement := range statements {
		mock.ExpectExec(regexp.QuoteMeta(statement)).WillReturnResult(sqlmock.NewResult(0, 0))
	}
}

func assertExpectations(t *testing.T, mock sqlmock.Sqlmock) {
	t.Helper()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("database expectations = %v", err)
	}
}
