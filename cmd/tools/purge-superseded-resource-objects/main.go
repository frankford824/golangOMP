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
	mysqlrepo "workflow/repo/mysql"
)

const applyConfirmation = "resource_revision_superseded"

func main() {
	var dsn string
	var apply bool
	var confirm string
	var timeout time.Duration
	flag.StringVar(&dsn, "dsn", "", "MySQL DSN; defaults to config MySQL DSN")
	flag.BoolVar(&apply, "apply", false, "revoke obsolete assets and queue exact object deletions")
	flag.StringVar(&confirm, "confirm", "", "required with --apply: "+applyConfirmation)
	flag.DurationVar(&timeout, "timeout", 5*time.Minute, "whole run timeout")
	flag.Parse()

	if apply && strings.TrimSpace(confirm) != applyConfirmation {
		writeError("--apply requires --confirm=" + applyConfirmation)
		os.Exit(2)
	}
	if timeout <= 0 {
		writeError("--timeout must be positive")
		os.Exit(2)
	}

	cfg, err := config.Load()
	if err != nil {
		writeError(fmt.Sprintf("load config: %v", err))
		os.Exit(1)
	}
	if strings.TrimSpace(dsn) == "" {
		dsn = cfg.MySQL.DSN
	}
	if strings.TrimSpace(dsn) == "" {
		writeError("mysql dsn is required")
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		writeError(fmt.Sprintf("open mysql: %v", err))
		os.Exit(1)
	}
	defer db.Close()
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(cfg.MySQL.ConnMaxLifetime)
	if err := db.PingContext(ctx); err != nil {
		writeError(fmt.Sprintf("ping mysql: %v", err))
		os.Exit(1)
	}

	startedAt := time.Now()
	summary, err := mysqlrepo.PurgeSupersededResourceObjects(ctx, db, apply)
	if err != nil {
		writeError(fmt.Sprintf("purge superseded resource objects: %v", err))
		os.Exit(1)
	}
	output := struct {
		mysqlrepo.SupersededResourceCleanupSummary
		ElapsedMS      int64  `json:"elapsed_ms"`
		GeneratedAtUTC string `json:"generated_at_utc"`
	}{
		SupersededResourceCleanupSummary: summary,
		ElapsedMS:                        time.Since(startedAt).Milliseconds(),
		GeneratedAtUTC:                   time.Now().UTC().Format(time.RFC3339),
	}
	if err := json.NewEncoder(os.Stdout).Encode(output); err != nil {
		writeError(fmt.Sprintf("encode result: %v", err))
		os.Exit(1)
	}
}

func writeError(message string) {
	_ = json.NewEncoder(os.Stderr).Encode(map[string]string{"error": message})
}
