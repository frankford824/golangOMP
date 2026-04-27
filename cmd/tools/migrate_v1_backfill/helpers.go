package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"workflow/cmd/tools/internal/v1migrate"
)

type BackfillOptions struct {
	DryRun    bool
	BatchSize int
}

type PhaseStat struct {
	Name      string
	Processed int64
	Generated int64
	Warnings  int64
	Errors    int64
	Duration  time.Duration
}

type BackfillResult struct {
	Stats         []PhaseStat
	TotalDuration time.Duration
}

func logf(format string, args ...any) {
	fmt.Printf("[R2-BACKFILL] "+format+"\n", args...)
}

func jsonString(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func nullIntValue(v sql.NullInt64) any {
	if v.Valid {
		return v.Int64
	}
	return nil
}

func nullStringValue(v sql.NullString) string {
	if v.Valid {
		return strings.TrimSpace(v.String)
	}
	return ""
}

func insertEvent(ctx context.Context, db *sql.DB, dryRun bool, moduleID int64, eventType, fromState, toState string, payload any) error {
	if dryRun {
		return nil
	}
	_, err := db.ExecContext(ctx, `
		INSERT INTO task_module_events (task_module_id, event_type, from_state, to_state, payload, created_at)
		VALUES (?, ?, NULLIF(?, ''), NULLIF(?, ''), CAST(? AS JSON), NOW())`,
		moduleID, eventType, fromState, toState, jsonString(payload))
	return err
}

func findModuleID(ctx context.Context, db *sql.DB, taskID int64, moduleKey string) (int64, bool, error) {
	var id int64
	err := db.QueryRowContext(ctx, `SELECT id FROM task_modules WHERE task_id=? AND module_key=?`, taskID, moduleKey).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return id, true, nil
}

func ensureModule(ctx context.Context, db *sql.DB, dryRun bool, spec moduleInstance) (int64, bool, error) {
	if dryRun {
		id, ok, err := findModuleID(ctx, db, spec.TaskID, spec.ModuleKey)
		return id, !ok, err
	}
	res, err := db.ExecContext(ctx, `
		INSERT IGNORE INTO task_modules
		  (task_id, module_key, state, pool_team_code, claimed_by, claimed_team_code, claimed_at, entered_at, terminal_at, data, updated_at)
		VALUES (?, ?, ?, NULLIF(?, ''), ?, NULLIF(?, ''), ?, NOW(), ?, CAST(? AS JSON), NOW())`,
		spec.TaskID, spec.ModuleKey, spec.State, spec.PoolTeamCode, spec.ClaimedBy, spec.ClaimedTeamCode, spec.ClaimedAt, spec.TerminalAt, spec.Data)
	if err != nil {
		return 0, false, err
	}
	affected, _ := res.RowsAffected()
	id, ok, err := findModuleID(ctx, db, spec.TaskID, spec.ModuleKey)
	if err != nil {
		return 0, false, err
	}
	if !ok {
		return 0, false, fmt.Errorf("module not found after insert: task_id=%d module=%s", spec.TaskID, spec.ModuleKey)
	}
	return id, affected > 0, nil
}

func cleanupPartialBackfill(ctx context.Context, db *sql.DB, dryRun bool) error {
	logf("cleanup-partial dry_run=%t", dryRun)
	statements := []string{
		"UPDATE task_assets SET source_task_module_id=NULL, source_module_key='design' WHERE source_task_module_id IS NOT NULL OR source_module_key <> 'design'",
		"DELETE FROM reference_file_refs",
		"DELETE FROM task_customization_orders",
		"DELETE FROM task_module_events",
		"DELETE FROM task_modules",
	}
	for _, stmt := range statements {
		logf("cleanup sql: %s", stmt)
		if dryRun {
			continue
		}
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func finishPhase(stat PhaseStat, start time.Time) PhaseStat {
	stat.Duration = time.Since(start)
	logf("%s processed=%d generated=%d warnings=%d errors=%d duration=%s",
		stat.Name, stat.Processed, stat.Generated, stat.Warnings, stat.Errors, stat.Duration.Round(time.Millisecond))
	return stat
}

func consistencyError(format string, args ...any) error {
	return v1migrate.NewHardAbort(v1migrate.ExitCodeConsistency, format, args...)
}
