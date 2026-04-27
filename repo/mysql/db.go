package mysqlrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"workflow/repo"
)

// MySQLTx wraps *sql.Tx and satisfies repo.Tx.
type MySQLTx struct {
	tx    *sql.Tx
	mu    sync.Mutex
	after []func()
}

func (t *MySQLTx) IsTx() {}

func (t *MySQLTx) AfterCommit(fn func()) {
	if fn == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.after = append(t.after, fn)
}

func (t *MySQLTx) runAfterCommit() {
	t.mu.Lock()
	hooks := append([]func(){}, t.after...)
	t.mu.Unlock()
	for _, fn := range hooks {
		fn()
	}
}

// DB wraps *sql.DB with helpers shared by all repo implementations.
type DB struct{ db *sql.DB }

func New(db *sql.DB) *DB { return &DB{db: db} }

func RawDB(db *DB) *sql.DB {
	if db == nil {
		return nil
	}
	return db.db
}

// BeginTx starts a new transaction. The caller MUST call Commit or Rollback.
func (d *DB) BeginTx(ctx context.Context) (repo.Tx, *sql.Tx, error) {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("begin tx: %w", err)
	}
	return &MySQLTx{tx: tx}, tx, nil
}

// Unwrap extracts the *sql.Tx from a repo.Tx.
// Panics if t was not produced by BeginTx — this is a programming error.
func Unwrap(t repo.Tx) *sql.Tx {
	return t.(*MySQLTx).tx
}

// nextSequence atomically returns the next event sequence number for a given SKU.
//
// It issues a SELECT … FOR UPDATE on the sku_sequences counter row to serialize
// concurrent callers within the same transaction scope. The UNIQUE(sku_id,sequence)
// constraint on event_logs is the ultimate safety net.
//
// MUST be called inside an active transaction that already holds (or will acquire)
// a row-level lock on the relevant business entity.
func nextSequence(ctx context.Context, sqlTx *sql.Tx, skuID int64) (int64, error) {
	var current int64
	err := sqlTx.QueryRowContext(ctx,
		`SELECT last_sequence FROM sku_sequences WHERE sku_id = ? FOR UPDATE`,
		skuID,
	).Scan(&current)

	if err == sql.ErrNoRows {
		// First event for this SKU: initialise the counter row.
		if _, err = sqlTx.ExecContext(ctx,
			`INSERT INTO sku_sequences (sku_id, last_sequence) VALUES (?, 1)`,
			skuID,
		); err != nil {
			return 0, fmt.Errorf("nextSequence insert: %w", err)
		}
		return 1, nil
	}
	if err != nil {
		return 0, fmt.Errorf("nextSequence select: %w", err)
	}

	next := current + 1
	if _, err = sqlTx.ExecContext(ctx,
		`UPDATE sku_sequences SET last_sequence = ? WHERE sku_id = ?`,
		next, skuID,
	); err != nil {
		return 0, fmt.Errorf("nextSequence update: %w", err)
	}
	return next, nil
}

// RunInTx implements repo.TxRunner.
// It starts a transaction, calls fn with a repo.Tx, then commits or rolls back.
// If fn returns a non-nil error the transaction is silently rolled back and the
// error is propagated unchanged so callers can use errors.Is/As normally.
func (d *DB) RunInTx(ctx context.Context, fn func(tx repo.Tx) error) error {
	sqlTx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer rollback(sqlTx)

	wrapped := &MySQLTx{tx: sqlTx}
	if err = fn(wrapped); err != nil {
		return err
	}
	if err = sqlTx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	wrapped.runAfterCommit()
	return nil
}

// rollback silently calls Rollback; safe to call after a successful Commit.
func rollback(tx *sql.Tx) { _ = tx.Rollback() }

// ── Null conversion helpers ───────────────────────────────────────────────────

func toNullString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

func toNullStringPtr(s *string) sql.NullString {
	return toNullString(s)
}

func toNullInt64(i *int64) sql.NullInt64 {
	if i == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *i, Valid: true}
}

func toNullInt(i *int) sql.NullInt64 {
	if i == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*i), Valid: true}
}

func toNullTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *t, Valid: true}
}

func toNullFloat64(f *float64) sql.NullFloat64 {
	if f == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: *f, Valid: true}
}

func toNullJSONString(raw json.RawMessage) sql.NullString {
	if len(raw) == 0 {
		return sql.NullString{}
	}
	return sql.NullString{String: string(raw), Valid: true}
}

func fromNullString(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	return &ns.String
}

func fromNullInt64(ni sql.NullInt64) *int64 {
	if !ni.Valid {
		return nil
	}
	return &ni.Int64
}

func fromNullInt(ni sql.NullInt64) *int {
	if !ni.Valid {
		return nil
	}
	value := int(ni.Int64)
	return &value
}

func fromNullTime(nt sql.NullTime) *time.Time {
	if !nt.Valid {
		return nil
	}
	return &nt.Time
}

func fromNullFloat64(nf sql.NullFloat64) *float64 {
	if !nf.Valid {
		return nil
	}
	return &nf.Float64
}

func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return page, pageSize
}
