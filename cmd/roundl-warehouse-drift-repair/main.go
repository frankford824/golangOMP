package main

import (
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type driftRow struct {
	TaskID        int64
	TaskNo        string
	TaskStatus    string
	ReceiptStatus string
	ReceiptID     int64
	ReceivedAt    sql.NullTime
}

type repairedRow struct {
	TaskID        int64
	TaskNo        string
	TaskStatus    string
	ReceiptStatus string
	ReceiptID     int64
	ReceivedAt    sql.NullTime
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		return errors.New("MYSQL_DSN is required")
	}

	beforePath := envOrDefault("ROUNDL_DRIFT_BEFORE_CSV", `/tmp/roundl_drift_before.csv`)
	afterPath := envOrDefault("ROUNDL_DRIFT_AFTER_CSV", `/tmp/roundl_drift_after.csv`)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("open mysql: %w", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping mysql: %w", err)
	}

	rows, err := discoverDrift(ctx, db)
	if err != nil {
		return err
	}
	if err := writeBeforeCSV(beforePath, rows); err != nil {
		return err
	}

	affectedIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		affectedIDs = append(affectedIDs, row.TaskID)
	}

	if len(rows) > 0 {
		if err := repair(ctx, db, rows); err != nil {
			return err
		}
	}

	postRows, err := loadPostRepairRows(ctx, db, affectedIDs)
	if err != nil {
		return err
	}
	if err := writeAfterCSV(afterPath, postRows); err != nil {
		return err
	}

	remaining, err := discoverDrift(ctx, db)
	if err != nil {
		return err
	}

	fmt.Printf("roundl warehouse drift before=%d after=%d\n", len(rows), len(remaining))
	fmt.Printf("before_csv=%s\n", beforePath)
	fmt.Printf("after_csv=%s\n", afterPath)
	return nil
}

func discoverDrift(ctx context.Context, db *sql.DB) ([]driftRow, error) {
	const query = `
SELECT t.id, t.task_no, t.task_status, wr.status AS receipt_status, wr.id AS receipt_id, wr.received_at
FROM tasks t
JOIN warehouse_receipts wr ON wr.task_id = t.id
WHERE (t.task_status = 'PendingWarehouseReceive' AND wr.status = 'received')
   OR (t.task_status = 'PendingWarehouseReceive' AND wr.status = 'rejected')
ORDER BY t.id ASC`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("discover drift: %w", err)
	}
	defer rows.Close()

	var result []driftRow
	for rows.Next() {
		var row driftRow
		if err := rows.Scan(&row.TaskID, &row.TaskNo, &row.TaskStatus, &row.ReceiptStatus, &row.ReceiptID, &row.ReceivedAt); err != nil {
			return nil, fmt.Errorf("scan drift row: %w", err)
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate drift rows: %w", err)
	}
	return result, nil
}

func repair(ctx context.Context, db *sql.DB, rows []driftRow) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin repair tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `UPDATE tasks SET task_status = ? WHERE id = ?`)
	if err != nil {
		return fmt.Errorf("prepare repair statement: %w", err)
	}
	defer stmt.Close()

	for _, row := range rows {
		nextStatus := repairedTaskStatus(row.ReceiptStatus)
		if nextStatus == "" {
			return fmt.Errorf("unsupported receipt status %q for task %d", row.ReceiptStatus, row.TaskID)
		}
		if _, err := stmt.ExecContext(ctx, nextStatus, row.TaskID); err != nil {
			return fmt.Errorf("repair task %d: %w", row.TaskID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit repair tx: %w", err)
	}
	return nil
}

func loadPostRepairRows(ctx context.Context, db *sql.DB, taskIDs []int64) ([]repairedRow, error) {
	if len(taskIDs) == 0 {
		return []repairedRow{}, nil
	}
	sort.Slice(taskIDs, func(i, j int) bool { return taskIDs[i] < taskIDs[j] })

	query := `SELECT t.id, t.task_no, t.task_status, wr.status AS receipt_status, wr.id AS receipt_id, wr.received_at
FROM tasks t
JOIN warehouse_receipts wr ON wr.task_id = t.id
WHERE t.id IN (?` + stringsRepeat(",?", len(taskIDs)-1) + `)
ORDER BY t.id ASC`

	args := make([]any, 0, len(taskIDs))
	for _, taskID := range taskIDs {
		args = append(args, taskID)
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("load post-repair rows: %w", err)
	}
	defer rows.Close()

	result := make([]repairedRow, 0, len(taskIDs))
	for rows.Next() {
		var row repairedRow
		if err := rows.Scan(&row.TaskID, &row.TaskNo, &row.TaskStatus, &row.ReceiptStatus, &row.ReceiptID, &row.ReceivedAt); err != nil {
			return nil, fmt.Errorf("scan post-repair row: %w", err)
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate post-repair rows: %w", err)
	}
	return result, nil
}

func writeBeforeCSV(path string, rows []driftRow) error {
	if err := ensureParentDir(path); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create before csv: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	if err := writer.Write([]string{"task_id", "task_no", "task_status", "receipt_status", "receipt_id", "received_at"}); err != nil {
		return fmt.Errorf("write before csv header: %w", err)
	}
	for _, row := range rows {
		if err := writer.Write([]string{
			fmt.Sprint(row.TaskID),
			row.TaskNo,
			row.TaskStatus,
			row.ReceiptStatus,
			fmt.Sprint(row.ReceiptID),
			formatNullTime(row.ReceivedAt),
		}); err != nil {
			return fmt.Errorf("write before csv row: %w", err)
		}
	}
	return writer.Error()
}

func writeAfterCSV(path string, rows []repairedRow) error {
	if err := ensureParentDir(path); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create after csv: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	if err := writer.Write([]string{"task_id", "task_no", "task_status", "receipt_status", "receipt_id", "received_at"}); err != nil {
		return fmt.Errorf("write after csv header: %w", err)
	}
	for _, row := range rows {
		if err := writer.Write([]string{
			fmt.Sprint(row.TaskID),
			row.TaskNo,
			row.TaskStatus,
			row.ReceiptStatus,
			fmt.Sprint(row.ReceiptID),
			formatNullTime(row.ReceivedAt),
		}); err != nil {
			return fmt.Errorf("write after csv row: %w", err)
		}
	}
	return writer.Error()
}

func repairedTaskStatus(receiptStatus string) string {
	switch receiptStatus {
	case "received":
		return "PendingProductionTransfer"
	case "rejected":
		return "RejectedByWarehouse"
	default:
		return ""
	}
}

func formatNullTime(value sql.NullTime) string {
	if !value.Valid {
		return ""
	}
	return value.Time.UTC().Format(time.RFC3339)
}

func ensureParentDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	return nil
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func stringsRepeat(token string, count int) string {
	if count <= 0 {
		return ""
	}
	out := ""
	for i := 0; i < count; i++ {
		out += token
	}
	return out
}
