package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"workflow/cmd/tools/internal/v1migrate"
)

func main() {
	var dsn string
	var sqlDir string
	var dryRun bool
	var r35Mode bool
	flag.StringVar(&dsn, "dsn", "", "MySQL DSN")
	flag.StringVar(&sqlDir, "sql-dir", "db/migrations", "directory containing 059~068 migration SQL files")
	flag.BoolVar(&dryRun, "dry-run", true, "print rollback SQL without executing")
	flag.BoolVar(&r35Mode, "r35-mode", false, "enable R3.5 *_r3_test DSN guard")
	flag.Parse()

	if err := run(dsn, sqlDir, dryRun, r35Mode); err != nil {
		fmt.Fprintf(os.Stderr, "[R2-ROLLBACK] abort: %v\n", err)
		os.Exit(v1migrate.ExitCode(err))
	}
}

func run(dsn, sqlDir string, dryRun bool, r35Mode bool) error {
	if r35Mode {
		if err := v1migrate.GuardR35DSN(dsn); err != nil {
			return v1migrate.NewHardAbort(v1migrate.ExitCodeR35SafetyViolation, "%w", err)
		}
	}
	var dbExec func(context.Context, string) error
	if !dryRun {
		db, err := v1migrate.OpenDB(dsn)
		if err != nil {
			return err
		}
		defer db.Close()
		dbExec = func(ctx context.Context, stmt string) error {
			_, err := db.ExecContext(ctx, stmt)
			return err
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fmt.Printf("[R2-ROLLBACK] dry_run=%t\n", dryRun)
	for i := len(v1migrate.MigrationFiles) - 1; i >= 0; i-- {
		name := v1migrate.MigrationFiles[i]
		raw, err := v1migrate.ReadFile(sqlDir, name)
		if err != nil {
			return err
		}
		stmts := v1migrate.ExtractRollback(raw)
		if len(stmts) == 0 {
			return fmt.Errorf("%s has empty rollback block", name)
		}
		fmt.Printf("[R2-ROLLBACK] %s\n", name)
		for _, stmt := range stmts {
			fmt.Printf("%s;\n", stmt)
			if !dryRun {
				if err := dbExec(ctx, stmt); err != nil {
					return fmt.Errorf("%s rollback failed: %w", name, err)
				}
			}
		}
	}
	return nil
}
