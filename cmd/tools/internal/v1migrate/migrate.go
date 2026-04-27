package v1migrate

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

const (
	ExitCodeConsistency = 2
	ExitCodeHardAbort   = 3
)

var MigrationFiles = []string{
	"059_v1_0_task_modules.sql",
	"060_v1_0_task_module_events.sql",
	"061_v1_0_task_assets_source_module_key.sql",
	"062_v1_0_reference_file_refs_flat.sql",
	"063_v1_0_task_drafts.sql",
	"064_v1_0_notifications.sql",
	"065_v1_0_org_move_requests.sql",
	"066_v1_0_task_assets_lifecycle.sql",
	"067_v1_0_tasks_priority_constraint.sql",
	"068_v1_0_task_customization_orders.sql",
}

type HardAbortError struct {
	Code int
	Err  error
}

func (e *HardAbortError) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *HardAbortError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func NewHardAbort(code int, format string, args ...any) *HardAbortError {
	return &HardAbortError{Code: code, Err: fmt.Errorf(format, args...)}
}

func ExitCode(err error) int {
	var hard *HardAbortError
	if errors.As(err, &hard) && hard.Code != 0 {
		return hard.Code
	}
	return 1
}

func OpenDB(dsn string) (*sql.DB, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, fmt.Errorf("--dsn is required")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(5 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func ExecStatements(ctx context.Context, db *sql.DB, sqlText string) error {
	for _, stmt := range SplitSQLStatements(sqlText) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("exec statement failed: %w\nstatement: %s", err, stmt)
		}
	}
	return nil
}

func SplitSQLStatements(sqlText string) []string {
	var out []string
	var b strings.Builder
	inSingle := false
	inDouble := false
	escaped := false
	for i := 0; i < len(sqlText); i++ {
		ch := sqlText[i]
		if escaped {
			b.WriteByte(ch)
			escaped = false
			continue
		}
		if ch == '\\' && (inSingle || inDouble) {
			b.WriteByte(ch)
			escaped = true
			continue
		}
		if ch == '\'' && !inDouble {
			inSingle = !inSingle
		}
		if ch == '"' && !inSingle {
			inDouble = !inDouble
		}
		if ch == ';' && !inSingle && !inDouble {
			appendStatement(&out, b.String())
			b.Reset()
			continue
		}
		b.WriteByte(ch)
	}
	appendStatement(&out, b.String())
	return out
}

func appendStatement(out *[]string, stmt string) {
	var lines []string
	scanner := bufio.NewScanner(strings.NewReader(stmt))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "--") {
			continue
		}
		lines = append(lines, scanner.Text())
	}
	trimmed := strings.TrimSpace(strings.Join(lines, "\n"))
	if trimmed != "" {
		*out = append(*out, trimmed)
	}
}

func ExtractRollback(sqlText string) []string {
	start := strings.Index(sqlText, "-- ROLLBACK-BEGIN")
	end := strings.Index(sqlText, "-- ROLLBACK-END")
	if start < 0 || end < 0 || end <= start {
		return nil
	}
	block := sqlText[start+len("-- ROLLBACK-BEGIN") : end]
	return SplitSQLStatements(block)
}

func ReadFile(sqlDir, name string) (string, error) {
	path := filepath.Join(sqlDir, name)
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func ReadForwardSQL(sqlDir, name string) (string, error) {
	raw, err := ReadFile(sqlDir, name)
	if err != nil {
		return "", err
	}
	if idx := strings.Index(raw, "-- ROLLBACK-BEGIN"); idx >= 0 {
		raw = raw[:idx]
	}
	return raw, nil
}

func HasTable(ctx context.Context, db *sql.DB, table string) (bool, error) {
	var n int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=?`, table).Scan(&n)
	return n > 0, err
}

func HasColumn(ctx context.Context, db *sql.DB, table, column string) (bool, error) {
	var n int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=? AND COLUMN_NAME=?`, table, column).Scan(&n)
	return n > 0, err
}

func HasIndex(ctx context.Context, db *sql.DB, table, index string) (bool, error) {
	var n int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=? AND INDEX_NAME=?`, table, index).Scan(&n)
	return n > 0, err
}

func HasConstraint(ctx context.Context, db *sql.DB, table, constraint string) (bool, error) {
	var n int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME=? AND CONSTRAINT_NAME=?`, table, constraint).Scan(&n)
	return n > 0, err
}

func ExecIfMissingColumn(ctx context.Context, db *sql.DB, table, column, statement string) error {
	ok, err := HasColumn(ctx, db, table, column)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	_, err = db.ExecContext(ctx, statement)
	return err
}

func ExecIfMissingIndex(ctx context.Context, db *sql.DB, table, index, statement string) error {
	ok, err := HasIndex(ctx, db, table, index)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	_, err = db.ExecContext(ctx, statement)
	return err
}

func ExecIfMissingConstraint(ctx context.Context, db *sql.DB, table, constraint, statement string) error {
	ok, err := HasConstraint(ctx, db, table, constraint)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	_, err = db.ExecContext(ctx, statement)
	return err
}

func SanitizeDSNForLog(dsn string) string {
	if at := strings.LastIndex(dsn, "@"); at > 0 {
		prefix := dsn[:at]
		if colon := strings.LastIndex(prefix, ":"); colon >= 0 {
			return prefix[:colon+1] + "****" + dsn[at:]
		}
	}
	return dsn
}

var checkExprSpace = regexp.MustCompile(`\s+`)

func NormalizeCheckExpression(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = checkExprSpace.ReplaceAllString(s, " ")
	return s
}
