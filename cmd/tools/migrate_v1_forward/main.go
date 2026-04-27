package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"time"

	"workflow/cmd/tools/internal/v1migrate"
)

func main() {
	var dsn string
	var sqlDir string
	var r35Mode bool
	flag.StringVar(&dsn, "dsn", "", "MySQL DSN")
	flag.StringVar(&sqlDir, "sql-dir", "db/migrations", "directory containing 059~068 migration SQL files")
	flag.BoolVar(&r35Mode, "r35-mode", false, "enable R3.5 *_r3_test DSN guard")
	flag.Parse()

	if err := run(dsn, sqlDir, r35Mode); err != nil {
		fmt.Fprintf(os.Stderr, "[R2-FORWARD] abort: %v\n", err)
		os.Exit(v1migrate.ExitCode(err))
	}
}

func run(dsn, sqlDir string, r35Mode bool) error {
	if r35Mode {
		if err := v1migrate.GuardR35DSN(dsn); err != nil {
			return v1migrate.NewHardAbort(v1migrate.ExitCodeR35SafetyViolation, "%w", err)
		}
	}
	db, err := v1migrate.OpenDB(dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fmt.Printf("[R2-FORWARD] db=%s\n", v1migrate.SanitizeDSNForLog(dsn))
	for _, name := range v1migrate.MigrationFiles {
		start := time.Now()
		fmt.Printf("[R2-FORWARD] applying %s\n", name)
		if err := applyMigration(ctx, db, sqlDir, name); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		fmt.Printf("[R2-FORWARD] applied %s duration=%s\n", name, time.Since(start).Round(time.Millisecond))
	}
	return nil
}

func applyMigration(ctx context.Context, db *sql.DB, sqlDir, name string) error {
	switch name {
	case "061_v1_0_task_assets_source_module_key.sql":
		return apply061(ctx, db)
	case "066_v1_0_task_assets_lifecycle.sql":
		return apply066(ctx, db)
	case "067_v1_0_tasks_priority_constraint.sql":
		return apply067(ctx, db)
	default:
		raw, err := v1migrate.ReadForwardSQL(sqlDir, name)
		if err != nil {
			return err
		}
		return v1migrate.ExecStatements(ctx, db, raw)
	}
}

func apply061(ctx context.Context, db *sql.DB) error {
	steps := []struct {
		column string
		sql    string
	}{
		{"source_module_key", "ALTER TABLE task_assets ADD COLUMN source_module_key VARCHAR(32) NOT NULL DEFAULT 'design'"},
		{"source_task_module_id", "ALTER TABLE task_assets ADD COLUMN source_task_module_id BIGINT NULL"},
		{"is_archived", "ALTER TABLE task_assets ADD COLUMN is_archived TINYINT(1) NOT NULL DEFAULT 0"},
		{"archived_at", "ALTER TABLE task_assets ADD COLUMN archived_at DATETIME NULL"},
		{"archived_by", "ALTER TABLE task_assets ADD COLUMN archived_by BIGINT NULL"},
	}
	for _, step := range steps {
		if err := v1migrate.ExecIfMissingColumn(ctx, db, "task_assets", step.column, step.sql); err != nil {
			return err
		}
	}
	if err := v1migrate.ExecIfMissingIndex(ctx, db, "task_assets", "idx_task_assets_source_task_module_id", "ALTER TABLE task_assets ADD KEY idx_task_assets_source_task_module_id (source_task_module_id)"); err != nil {
		return err
	}
	return v1migrate.ExecIfMissingConstraint(ctx, db, "task_assets", "fk_task_assets_source_task_module", "ALTER TABLE task_assets ADD CONSTRAINT fk_task_assets_source_task_module FOREIGN KEY (source_task_module_id) REFERENCES task_modules (id)")
}

func apply066(ctx context.Context, db *sql.DB) error {
	if err := v1migrate.ExecIfMissingColumn(ctx, db, "task_assets", "cleaned_at", "ALTER TABLE task_assets ADD COLUMN cleaned_at DATETIME NULL"); err != nil {
		return err
	}
	if err := v1migrate.ExecIfMissingColumn(ctx, db, "task_assets", "deleted_at", "ALTER TABLE task_assets ADD COLUMN deleted_at DATETIME NULL"); err != nil {
		return err
	}
	return v1migrate.ExecIfMissingIndex(ctx, db, "task_assets", "idx_task_assets_archived_deleted", "ALTER TABLE task_assets ADD KEY idx_task_assets_archived_deleted (is_archived, deleted_at)")
}

func apply067(ctx context.Context, db *sql.DB) error {
	var badCount int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tasks WHERE priority NOT IN ('low','normal','high','critical')").Scan(&badCount); err != nil {
		return err
	}
	if badCount > 0 {
		return v1migrate.NewHardAbort(v1migrate.ExitCodeHardAbort, "priority out of range before CHECK: count=%d", badCount)
	}
	if err := v1migrate.ExecIfMissingConstraint(ctx, db, "tasks", "chk_tasks_priority_v1", "ALTER TABLE tasks ADD CONSTRAINT chk_tasks_priority_v1 CHECK (priority IN ('low', 'normal', 'high', 'critical'))"); err != nil {
		return err
	}
	return v1migrate.ExecIfMissingIndex(ctx, db, "tasks", "idx_tasks_priority_created", "ALTER TABLE tasks ADD KEY idx_tasks_priority_created (priority, created_at)")
}
