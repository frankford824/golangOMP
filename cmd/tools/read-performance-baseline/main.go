package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	mysql "github.com/go-sql-driver/mysql"

	"workflow/domain"
	"workflow/repo"
	mysqlrepo "workflow/repo/mysql"
)

type surfaceResult struct {
	Name                string  `json:"name"`
	Iterations          int     `json:"iterations"`
	DatabaseRoundTrips  int64   `json:"database_round_trips"`
	RoundTripsPerCall   float64 `json:"round_trips_per_call"`
	P50MS               float64 `json:"p50_ms"`
	P95MS               float64 `json:"p95_ms"`
	P99MS               float64 `json:"p99_ms"`
	AllocatedBytesPerOp uint64  `json:"allocated_bytes_per_op"`
	GCCount             uint32  `json:"gc_count"`
}

type explainResult struct {
	Name string   `json:"name"`
	Plan []string `json:"plan"`
}

type output struct {
	Database   string          `json:"database"`
	TaskID     int64           `json:"task_id"`
	MeasuredAt time.Time       `json:"measured_at"`
	Surfaces   []surfaceResult `json:"surfaces"`
	Explains   []explainResult `json:"explain_analyze"`
}

func main() {
	var dsn string
	var taskID int64
	var iterations int
	var timeout time.Duration
	flag.StringVar(&dsn, "dsn", strings.TrimSpace(os.Getenv("MYSQL_DSN")), "read-only MySQL DSN")
	flag.Int64Var(&taskID, "task-id", 0, "representative task ID; latest task when omitted")
	flag.IntVar(&iterations, "iterations", 100, "iterations per read surface")
	flag.DurationVar(&timeout, "timeout", 3*time.Minute, "whole-run timeout")
	flag.Parse()
	if strings.TrimSpace(dsn) == "" || iterations < 2 {
		fatalf("--dsn and --iterations >= 2 are required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	counter := &roundTripCounter{}
	db, err := openCountingMySQL(dsn, counter)
	if err != nil {
		fatalf("open mysql: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.PingContext(ctx); err != nil {
		fatalf("ping mysql: %v", err)
	}
	var database string
	if err := db.QueryRowContext(ctx, `SELECT DATABASE()`).Scan(&database); err != nil {
		fatalf("read database: %v", err)
	}
	if taskID <= 0 {
		if err := db.QueryRowContext(ctx, `SELECT id FROM tasks ORDER BY id DESC LIMIT 1`).Scan(&taskID); err != nil {
			fatalf("select representative task: %v", err)
		}
	}

	mdb := mysqlrepo.New(db)
	taskRepository := mysqlrepo.NewTaskRepo(mdb)
	detailReader, ok := taskRepository.(interface {
		GetTaskDetailReadBundle(context.Context, int64, int) (*domain.TaskDetailReadBundle, error)
	})
	if !ok {
		fatalf("task repository does not expose the optimized detail bundle")
	}
	groups := mysqlrepo.NewTaskResourceGroupRepo(mdb)
	assets := mysqlrepo.NewTaskAssetSearchRepo(mdb)

	result := output{Database: database, TaskID: taskID, MeasuredAt: time.Now().UTC()}
	result.Surfaces = append(result.Surfaces,
		measure(ctx, counter, "task_detail", iterations, func() error {
			_, err := detailReader.GetTaskDetailReadBundle(ctx, taskID, 50)
			return err
		}),
		measure(ctx, counter, "task_list", iterations, func() error {
			_, _, err := taskRepository.List(ctx, repo.TaskListFilter{ScopeViewAll: true, Page: 1, PageSize: 50})
			return err
		}),
		measure(ctx, counter, "resource_groups", iterations, func() error {
			_, _, err := groups.ListResourceGroups(ctx, domain.ResourceGroupListParams{Access: domain.ResourceGroupAccessFilter{Global: true}, Page: 1, PageSize: 50})
			return err
		}),
		measure(ctx, counter, "asset_exact_search", iterations, func() error {
			_, _, err := assets.Search(ctx, domain.AssetSearchQuery{Source: domain.AssetResourceSourceSystem, Page: 1, Size: 50})
			return err
		}),
	)
	result.Explains = append(result.Explains,
		explain(ctx, db, "task_detail_lookup", `SELECT * FROM tasks WHERE id = ?`, taskID),
		explain(ctx, db, "task_list_page", `SELECT t.id FROM tasks t ORDER BY t.updated_at DESC, t.id DESC LIMIT 50`),
		explain(ctx, db, "resource_group_page", `SELECT g.id FROM task_asset_groups g JOIN tasks t ON t.id=g.task_id WHERE g.finalized_revision_id IS NOT NULL ORDER BY g.updated_at DESC, g.id DESC LIMIT 50`),
		explain(ctx, db, "asset_current_page", `SELECT ta.id FROM task_assets ta WHERE ta.deleted_at IS NULL ORDER BY ta.created_at DESC LIMIT 50`),
	)
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		fatalf("encode result: %v", err)
	}
}

func measure(ctx context.Context, counter *roundTripCounter, name string, iterations int, operation func() error) surfaceResult {
	for range 3 {
		if err := operation(); err != nil {
			fatalf("warm %s: %v", name, err)
		}
	}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	counter.reset()
	durations := make([]time.Duration, 0, iterations)
	for range iterations {
		started := time.Now()
		if err := operation(); err != nil {
			fatalf("measure %s: %v", name, err)
		}
		durations = append(durations, time.Since(started))
		if err := ctx.Err(); err != nil {
			fatalf("measure %s: %v", name, err)
		}
	}
	runtime.ReadMemStats(&after)
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	roundTrips := counter.value()
	return surfaceResult{
		Name: name, Iterations: iterations, DatabaseRoundTrips: roundTrips,
		RoundTripsPerCall: float64(roundTrips) / float64(iterations),
		P50MS:             durationMS(percentile(durations, 0.50)), P95MS: durationMS(percentile(durations, 0.95)), P99MS: durationMS(percentile(durations, 0.99)),
		AllocatedBytesPerOp: (after.TotalAlloc - before.TotalAlloc) / uint64(iterations),
		GCCount:             after.NumGC - before.NumGC,
	}
}

func percentile(values []time.Duration, quantile float64) time.Duration {
	if len(values) == 0 {
		return 0
	}
	index := int(float64(len(values)-1) * quantile)
	return values[index]
}

func durationMS(value time.Duration) float64 { return float64(value.Microseconds()) / 1000 }

func explain(ctx context.Context, db *sql.DB, name, query string, args ...any) explainResult {
	rows, err := db.QueryContext(ctx, "EXPLAIN ANALYZE "+query, args...)
	if err != nil {
		return explainResult{Name: name, Plan: []string{"ERROR: " + err.Error()}}
	}
	defer rows.Close()
	result := explainResult{Name: name, Plan: []string{}}
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			result.Plan = append(result.Plan, "ERROR: "+err.Error())
			break
		}
		result.Plan = append(result.Plan, line)
	}
	return result
}

type roundTripCounter struct{ calls atomic.Int64 }

func (c *roundTripCounter) reset()       { c.calls.Store(0) }
func (c *roundTripCounter) value() int64 { return c.calls.Load() }

type countingConnector struct {
	base    driver.Connector
	counter *roundTripCounter
}

func openCountingMySQL(dsn string, counter *roundTripCounter) (*sql.DB, error) {
	base, err := (&mysql.MySQLDriver{}).OpenConnector(dsn)
	if err != nil {
		return nil, err
	}
	return sql.OpenDB(&countingConnector{base: base, counter: counter}), nil
}

func (c *countingConnector) Connect(ctx context.Context) (driver.Conn, error) {
	connection, err := c.base.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &countingConn{Conn: connection, counter: c.counter}, nil
}

func (c *countingConnector) Driver() driver.Driver { return c.base.Driver() }

type countingConn struct {
	driver.Conn
	counter *roundTripCounter
}

func (c *countingConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	queryer, ok := c.Conn.(driver.QueryerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	c.counter.calls.Add(1)
	return queryer.QueryContext(ctx, query, args)
}

func (c *countingConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	execer, ok := c.Conn.(driver.ExecerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	c.counter.calls.Add(1)
	return execer.ExecContext(ctx, query, args)
}

func fatalf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
