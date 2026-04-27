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
	var dryRun bool
	var cleanupPartial bool
	var r35Mode bool
	var batchSize int
	flag.StringVar(&dsn, "dsn", "", "MySQL DSN")
	flag.BoolVar(&dryRun, "dry-run", false, "plan backfill without writing")
	flag.IntVar(&batchSize, "batch-size", 1000, "scan batch size")
	flag.BoolVar(&cleanupPartial, "cleanup-partial", false, "clear partial R2 backfill rows and exit")
	flag.BoolVar(&r35Mode, "r35-mode", false, "enable R3.5 *_r3_test DSN guard")
	flag.Parse()

	if r35Mode {
		if err := v1migrate.GuardR35DSN(dsn); err != nil {
			fmt.Fprintf(os.Stderr, "[R2-BACKFILL] abort: %v\n", err)
			os.Exit(v1migrate.ExitCodeR35SafetyViolation)
		}
	}
	db, err := v1migrate.OpenDB(dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[R2-BACKFILL] abort: %v\n", err)
		os.Exit(v1migrate.ExitCode(err))
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if cleanupPartial {
		err = cleanupPartialBackfill(ctx, db, dryRun)
	} else {
		_, err = runBackfill(ctx, db, BackfillOptions{DryRun: dryRun, BatchSize: batchSize})
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "[R2-BACKFILL] abort: %v\n", err)
		os.Exit(v1migrate.ExitCode(err))
	}
}
