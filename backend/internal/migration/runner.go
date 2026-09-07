package migration

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"
)

const advisoryLockKey int64 = 0x4e657861466c6f77 // "NexaFlow"

var (
	ErrChecksumMismatch = errors.New("migration checksum mismatch")
	ErrDatabaseAhead    = errors.New("database migration version is newer than this binary")
	ErrMigrationHistory = errors.New("migration history is not contiguous")
	ErrInvalidRegistry  = errors.New("invalid migration registry")
	migrationName       = regexp.MustCompile(`^[a-z0-9]+(?:_[a-z0-9]+)*$`)
	migrationChecksum   = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

//go:embed specs/*.txt
var migrationSpecs embed.FS

type migration struct {
	version  int64
	name     string
	checksum string
	apply    func(*gorm.DB) error
}

type AppliedMigration struct {
	Version   int64         `json:"version"`
	Name      string        `json:"name"`
	Checksum  string        `json:"checksum"`
	Duration  time.Duration `json:"duration"`
	AppliedAt time.Time     `json:"applied_at"`
}

type Report struct {
	CurrentVersion int64              `json:"current_version"`
	Applied        []AppliedMigration `json:"applied"`
}

// Runner owns ordering, locking, checksum verification, and transactional
// application. Callers only provide a connected database.
type Runner struct {
	migrations []migration
	now        func() time.Time
}

func DefaultRunner() *Runner {
	return &Runner{migrations: defaultMigrations(), now: func() time.Time { return time.Now().UTC() }}
}

func (r *Runner) Run(ctx context.Context, db *gorm.DB) (Report, error) {
	if r == nil || r.now == nil {
		return Report{}, fmt.Errorf("%w: runner clock is required", ErrInvalidRegistry)
	}
	if ctx == nil {
		return Report{}, errors.New("migration context is required")
	}
	if db == nil {
		return Report{}, errors.New("migration database is required")
	}
	if err := validateRegistry(r.migrations); err != nil {
		return Report{}, err
	}
	var report Report
	err := db.WithContext(ctx).Connection(func(connection *gorm.DB) error {
		if err := connection.Exec("SELECT pg_advisory_lock(?)", advisoryLockKey).Error; err != nil {
			return fmt.Errorf("acquire migration advisory lock: %w", err)
		}
		runErr := r.runLocked(connection, &report)
		unlockContext, cancelUnlock := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		unlockErr := connection.WithContext(unlockContext).Exec("SELECT pg_advisory_unlock(?)", advisoryLockKey).Error
		cancelUnlock()
		if runErr != nil {
			return runErr
		}
		if unlockErr != nil {
			return fmt.Errorf("release migration advisory lock: %w", unlockErr)
		}
		return nil
	})
	return report, err
}

func (r *Runner) runLocked(db *gorm.DB, report *Report) error {
	if err := ensureTenantRLSSafeRole(db); err != nil {
		return err
	}
	if err := ensureLedger(db); err != nil {
		return err
	}
	applied, err := loadApplied(db)
	if err != nil {
		return err
	}
	known := make(map[int64]migration, len(r.migrations))
	for _, item := range r.migrations {
		known[item.version] = item
	}
	for index, row := range applied {
		expectedVersion := int64(index + 1)
		if row.Version != expectedVersion {
			return fmt.Errorf("%w: expected version %d, found %d", ErrMigrationHistory, expectedVersion, row.Version)
		}
		expected, ok := known[row.Version]
		if !ok {
			return fmt.Errorf("%w: version %d", ErrDatabaseAhead, row.Version)
		}
		if row.Name != expected.name {
			return fmt.Errorf("%w: version %d name is %q, expected %q", ErrChecksumMismatch, row.Version, row.Name, expected.name)
		}
		if !row.Checksum.Valid || row.Checksum.String == "" {
			if err := db.Exec("UPDATE schema_migrations SET checksum = ? WHERE version = ? AND checksum IS NULL", expected.checksum, row.Version).Error; err != nil {
				return fmt.Errorf("backfill migration %d checksum: %w", row.Version, err)
			}
			row.Checksum = sql.NullString{String: expected.checksum, Valid: true}
		}
		if row.Checksum.String != expected.checksum {
			return fmt.Errorf("%w: version %d", ErrChecksumMismatch, row.Version)
		}
		if row.Version > report.CurrentVersion {
			report.CurrentVersion = row.Version
		}
	}

	for _, item := range r.migrations {
		if item.version <= report.CurrentVersion {
			continue
		}
		started := r.now()
		var appliedAt time.Time
		err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec("SELECT set_config('app.system_access', 'on', true)").Error; err != nil {
				return fmt.Errorf("set migration system context: %w", err)
			}
			if err := item.apply(tx); err != nil {
				return fmt.Errorf("apply migration %d_%s: %w", item.version, item.name, err)
			}
			appliedAt = r.now()
			duration := appliedAt.Sub(started).Milliseconds()
			return tx.Exec("INSERT INTO schema_migrations (version, name, checksum, duration_ms, applied_at) VALUES (?, ?, ?, ?, ?)", item.version, item.name, item.checksum, duration, appliedAt).Error
		})
		if err != nil {
			return err
		}
		duration := appliedAt.Sub(started)
		report.Applied = append(report.Applied, AppliedMigration{Version: item.version, Name: item.name, Checksum: item.checksum, Duration: duration, AppliedAt: appliedAt})
		report.CurrentVersion = item.version
	}
	return nil
}

type ledgerRow struct {
	Version  int64
	Name     string
	Checksum sql.NullString
}

func ensureLedger(db *gorm.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS schema_migrations (version bigint PRIMARY KEY, name varchar(160) NOT NULL, checksum varchar(64), duration_ms bigint NOT NULL DEFAULT 0, applied_at timestamptz NOT NULL)`,
		`ALTER TABLE schema_migrations ADD COLUMN IF NOT EXISTS checksum varchar(64)`,
		`ALTER TABLE schema_migrations ADD COLUMN IF NOT EXISTS duration_ms bigint NOT NULL DEFAULT 0`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			return fmt.Errorf("prepare migration ledger: %w", err)
		}
	}
	return nil
}

func loadApplied(db *gorm.DB) ([]ledgerRow, error) {
	var rows []ledgerRow
	if err := db.Raw("SELECT version, name, checksum FROM schema_migrations ORDER BY version").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("load migration ledger: %w", err)
	}
	return rows, nil
}

func validateRegistry(items []migration) error {
	if len(items) == 0 {
		return fmt.Errorf("%w: registry is empty", ErrInvalidRegistry)
	}
	for index, item := range items {
		expectedVersion := int64(index + 1)
		if item.version != expectedVersion || !migrationName.MatchString(item.name) || !migrationChecksum.MatchString(item.checksum) || item.apply == nil {
			return fmt.Errorf("%w: migration at index %d", ErrInvalidRegistry, index)
		}
	}
	return nil
}

func migrationFromSpec(version int64, name, path string, apply func(*gorm.DB) error) migration {
	definition, err := migrationSpecs.ReadFile(path)
	if err != nil {
		panic(err)
	}
	contract := string(definition)
	required := []string{fmt.Sprintf("version=%d", version), "name=" + name, "contract=", "risk=", "rollback="}
	for _, field := range required {
		if !strings.Contains(contract, field) {
			panic(fmt.Sprintf("migration specification %s is missing %q", path, field))
		}
	}
	digest := sha256.Sum256(definition)
	return migration{version: version, name: name, checksum: hex.EncodeToString(digest[:]), apply: apply}
}

func ensureTenantRLSSafeRole(db *gorm.DB) error {
	var superuser, bypassRLS bool
	if err := db.Raw("SELECT rolsuper, rolbypassrls FROM pg_roles WHERE rolname = current_user").Row().Scan(&superuser, &bypassRLS); err != nil {
		return fmt.Errorf("inspect application database role: %w", err)
	}
	if superuser || bypassRLS {
		return errors.New("application database role must be NOSUPERUSER and NOBYPASSRLS for tenant isolation")
	}
	return nil
}
