package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"workflow/config"
)

type summary struct {
	DryRun             bool      `json:"dry_run"`
	MissingBefore      int64     `json:"missing_before"`
	Inserted           int64     `json:"inserted"`
	MissingAfter       int64     `json:"missing_after"`
	GeneratedAtUTC     time.Time `json:"generated_at_utc"`
	ProductionSafeNote string    `json:"production_safe_note"`
}

func main() {
	var dsn string
	var apply bool
	var confirmDatabase string
	var timeout time.Duration
	flag.StringVar(&dsn, "dsn", "", "MySQL DSN; defaults to config MySQL DSN")
	flag.BoolVar(&apply, "apply", false, "insert missing sku_planning resource-group shells")
	flag.StringVar(&confirmDatabase, "confirm-database", "", "required exact database name guard when --apply is used")
	flag.DurationVar(&timeout, "timeout", 2*time.Minute, "whole run timeout")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		exitError("load config", err)
	}
	if strings.TrimSpace(dsn) == "" {
		dsn = cfg.MySQL.DSN
	}
	if strings.TrimSpace(dsn) == "" {
		exitError("validate config", fmt.Errorf("mysql dsn is required"))
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		exitError("open mysql", err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		exitError("ping mysql", err)
	}
	var database string
	if err := db.QueryRowContext(ctx, `SELECT DATABASE()`).Scan(&database); err != nil {
		exitError("read database", err)
	}
	if apply && database != strings.TrimSpace(confirmDatabase) {
		exitError("validate database guard", fmt.Errorf("connected database %q does not match confirmation %q", database, confirmDatabase))
	}

	out := summary{
		DryRun:             !apply,
		GeneratedAtUTC:     time.Now().UTC(),
		ProductionSafeNote: "Only missing sku_planning SKU-scoped shells are inserted; tasks, revisions, files and ERP data are not modified.",
	}
	out.MissingBefore, err = countMissing(ctx, db)
	if err != nil {
		exitError("count missing groups", err)
	}
	if apply && out.MissingBefore > 0 {
		tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
		if err != nil {
			exitError("begin transaction", err)
		}
		result, execErr := tx.ExecContext(ctx, `
			INSERT INTO task_asset_groups (task_id, scope_kind, task_sku_item_id)
			SELECT tsi.task_id, 'sku', tsi.id
			FROM task_sku_items tsi
			JOIN tasks t ON t.id = tsi.task_id AND t.task_type = 'sku_planning'
			LEFT JOIN task_asset_groups g
			  ON g.task_id = tsi.task_id
			 AND g.scope_kind = 'sku'
			 AND g.task_sku_item_id = tsi.id
			WHERE g.id IS NULL
			ON DUPLICATE KEY UPDATE updated_at = task_asset_groups.updated_at`)
		if execErr != nil {
			_ = tx.Rollback()
			exitError("insert missing groups", execErr)
		}
		out.Inserted, err = result.RowsAffected()
		if err != nil {
			_ = tx.Rollback()
			exitError("read inserted count", err)
		}
		if err := tx.Commit(); err != nil {
			exitError("commit groups", err)
		}
	}
	out.MissingAfter, err = countMissing(ctx, db)
	if err != nil {
		exitError("verify missing groups", err)
	}
	if !apply {
		out.MissingAfter = out.MissingBefore
	}
	if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
		exitError("encode summary", err)
	}
}

func countMissing(ctx context.Context, queryer interface {
	QueryRowContext(context.Context, string, ...interface{}) *sql.Row
}) (int64, error) {
	var count int64
	err := queryer.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM task_sku_items tsi
		JOIN tasks t ON t.id = tsi.task_id AND t.task_type = 'sku_planning'
		LEFT JOIN task_asset_groups g
		  ON g.task_id = tsi.task_id
		 AND g.scope_kind = 'sku'
		 AND g.task_sku_item_id = tsi.id
		WHERE g.id IS NULL`).Scan(&count)
	return count, err
}

func exitError(operation string, err error) {
	_ = json.NewEncoder(os.Stderr).Encode(map[string]string{"error": operation + ": " + err.Error()})
	os.Exit(1)
}
