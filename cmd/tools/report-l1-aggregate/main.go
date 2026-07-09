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

type aggregateSummary struct {
	DryRun        bool   `json:"dry_run"`
	From          string `json:"from"`
	To            string `json:"to"`
	SourceRows    int64  `json:"source_rows"`
	AggregateRows int64  `json:"aggregate_rows"`
	Changed       bool   `json:"changed"`
	ElapsedMS     int64  `json:"elapsed_ms"`
	Message       string `json:"message,omitempty"`
}

func main() {
	var dsn string
	var fromRaw string
	var toRaw string
	var days int
	var dryRun bool
	var timeout time.Duration
	flag.StringVar(&dsn, "dsn", "", "MySQL DSN; defaults to config MySQL DSN")
	flag.StringVar(&fromRaw, "from", "", "inclusive UTC day YYYY-MM-DD; defaults to --days window")
	flag.StringVar(&toRaw, "to", "", "inclusive UTC day YYYY-MM-DD; defaults to today")
	flag.IntVar(&days, "days", 3, "UTC days to refresh when --from is omitted")
	flag.BoolVar(&dryRun, "dry-run", false, "count affected rows without rebuilding aggregates")
	flag.DurationVar(&timeout, "timeout", 5*time.Minute, "whole run timeout")
	flag.Parse()

	code := run(context.Background(), aggregateOptions{
		DSN:     dsn,
		FromRaw: fromRaw,
		ToRaw:   toRaw,
		Days:    days,
		DryRun:  dryRun,
		Timeout: timeout,
	})
	os.Exit(code)
}

type aggregateOptions struct {
	DSN     string
	FromRaw string
	ToRaw   string
	Days    int
	DryRun  bool
	Timeout time.Duration
}

func run(parent context.Context, opts aggregateOptions) int {
	start := time.Now()
	ctx, cancel := context.WithTimeout(parent, positiveDuration(opts.Timeout, 5*time.Minute))
	defer cancel()
	from, to, err := parseAggregateWindow(opts.FromRaw, opts.ToRaw, opts.Days, time.Now().UTC())
	if err != nil {
		writeAggregateError(err.Error())
		return 2
	}
	cfg, err := config.Load()
	if err != nil {
		writeAggregateError(fmt.Sprintf("load config: %v", err))
		return 1
	}
	dsn := strings.TrimSpace(opts.DSN)
	if dsn == "" {
		dsn = cfg.MySQL.DSN
	}
	if dsn == "" {
		writeAggregateError("mysql dsn is required")
		return 2
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		writeAggregateError(fmt.Sprintf("open mysql: %v", err))
		return 1
	}
	defer db.Close()
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(cfg.MySQL.ConnMaxLifetime)
	if err := db.PingContext(ctx); err != nil {
		writeAggregateError(fmt.Sprintf("ping mysql: %v", err))
		return 1
	}

	sourceRows, err := countAggregateSourceRows(ctx, db, from, to)
	if err != nil {
		writeAggregateError(fmt.Sprintf("count source rows: %v", err))
		return 1
	}
	if !opts.DryRun {
		repo := mysqlrepo.NewReportL1Repo(mysqlrepo.New(db))
		if err := repo.RefreshDailyAggregates(ctx, from, to); err != nil {
			writeAggregateError(fmt.Sprintf("refresh aggregates: %v", err))
			return 1
		}
	}
	aggregateRows, err := countAggregateRows(ctx, db, from, to)
	if err != nil {
		writeAggregateError(fmt.Sprintf("count aggregate rows: %v", err))
		return 1
	}
	out := aggregateSummary{
		DryRun:        opts.DryRun,
		From:          from.Format("2006-01-02"),
		To:            to.Format("2006-01-02"),
		SourceRows:    sourceRows,
		AggregateRows: aggregateRows,
		Changed:       !opts.DryRun,
		ElapsedMS:     time.Since(start).Milliseconds(),
	}
	if opts.DryRun {
		out.Message = "dry run only; no aggregate rows changed"
	}
	writeAggregateJSON(out)
	return 0
}

func parseAggregateWindow(fromRaw, toRaw string, days int, now time.Time) (time.Time, time.Time, error) {
	to, err := parseAggregateDay(toRaw, now.Truncate(24*time.Hour))
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if strings.TrimSpace(fromRaw) != "" {
		from, err := parseAggregateDay(fromRaw, time.Time{})
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		if to.Before(from) {
			return time.Time{}, time.Time{}, fmt.Errorf("--to must be on or after --from")
		}
		return from, to, nil
	}
	if days < 1 {
		days = 1
	}
	return to.AddDate(0, 0, -(days - 1)), to, nil
}

func parseAggregateDay(raw string, fallback time.Time) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		if fallback.IsZero() {
			return time.Time{}, fmt.Errorf("date is required")
		}
		return fallback.UTC().Truncate(24 * time.Hour), nil
	}
	day, err := time.ParseInLocation("2006-01-02", raw, time.UTC)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q, expected YYYY-MM-DD", raw)
	}
	return day.UTC(), nil
}

func positiveDuration(value, fallback time.Duration) time.Duration {
	if value > 0 {
		return value
	}
	return fallback
}

func countAggregateSourceRows(ctx context.Context, db *sql.DB, from, to time.Time) (int64, error) {
	rangeEnd := to.AddDate(0, 0, 1)
	var count int64
	if err := db.QueryRowContext(ctx, `
		SELECT SUM(count_value)
		  FROM (
		        SELECT COUNT(*) AS count_value
		          FROM tasks t
		         WHERE t.created_at >= ? AND t.created_at < ?
		        UNION ALL
		        SELECT COUNT(DISTINCT tel.task_id) AS count_value
		          FROM task_event_logs tel
		         WHERE tel.created_at >= ? AND tel.created_at < ?
		           AND tel.event_type IN (
		               'task.audit.approved',
		               'task.customization.reviewed',
		               'task.warehouse.completed',
		               'task.closed'
		           )
		       ) x`, from, rangeEnd, from, rangeEnd).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func countAggregateRows(ctx context.Context, db *sql.DB, from, to time.Time) (int64, error) {
	rangeEnd := to.AddDate(0, 0, 1)
	var count int64
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		  FROM report_task_daily
		 WHERE day >= DATE(?) AND day < DATE(?)`, from, rangeEnd).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func writeAggregateJSON(out aggregateSummary) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fmt.Fprintf(os.Stderr, "encode output: %v\n", err)
		os.Exit(1)
	}
}

func writeAggregateError(message string) {
	writeAggregateJSON(aggregateSummary{Message: message})
}
