package mysqlrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"workflow/domain"
	"workflow/repo"
)

// EventRepoImpl implements repo.EventRepo.
// Append MUST always be called inside an active transaction so that the
// event_log write is atomic with the state change that triggered it (spec §8.2).
type EventRepoImpl struct{ db *sql.DB }

func NewEventRepo(db *DB) repo.EventRepo { return &EventRepoImpl{db: db.db} }

// Append generates the next sequence number for the SKU, inserts the event_log row,
// and returns the persisted record — all inside the caller's transaction.
func (r *EventRepoImpl) Append(
	ctx context.Context,
	tx repo.Tx,
	skuID int64,
	eventType string,
	payload interface{},
) (*domain.EventLog, error) {
	sqlTx := Unwrap(tx)

	seq, err := nextSequence(ctx, sqlTx, skuID)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal event payload: %w", err)
	}

	id := uuid.New().String()
	now := time.Now()

	_, err = sqlTx.ExecContext(ctx, `
		INSERT INTO event_logs (id, sku_id, sequence, event_type, payload, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		id, skuID, seq, eventType, raw, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert event_log: %w", err)
	}

	return &domain.EventLog{
		ID:        id,
		SKUID:     skuID,
		Sequence:  seq,
		EventType: eventType,
		Payload:   raw,
		CreatedAt: now,
	}, nil
}

// ListSince returns all events for a SKU with sequence > sinceSequence, ordered ascending.
// Used by the frontend sync_status endpoint for sequence-gap recovery (spec §5.2 invariant 9).
func (r *EventRepoImpl) ListSince(ctx context.Context, skuID, sinceSequence int64) ([]*domain.EventLog, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, sku_id, sequence, event_type, payload, created_at
		FROM event_logs
		WHERE sku_id = ? AND sequence > ?
		ORDER BY sequence ASC`,
		skuID, sinceSequence,
	)
	if err != nil {
		return nil, fmt.Errorf("query event_logs: %w", err)
	}
	defer rows.Close()

	var events []*domain.EventLog
	for rows.Next() {
		e := &domain.EventLog{}
		if err := rows.Scan(&e.ID, &e.SKUID, &e.Sequence, &e.EventType, &e.Payload, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan event_log: %w", err)
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// GetLatestSequence returns the highest sequence number recorded for a SKU (0 if none).
func (r *EventRepoImpl) GetLatestSequence(ctx context.Context, skuID int64) (int64, error) {
	var seq int64
	err := r.db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(sequence), 0) FROM event_logs WHERE sku_id = ?`,
		skuID,
	).Scan(&seq)
	if err != nil {
		return 0, fmt.Errorf("get latest sequence: %w", err)
	}
	return seq, nil
}
