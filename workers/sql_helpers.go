package workers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

func workerNextSequence(ctx context.Context, tx *sql.Tx, skuID int64) (int64, error) {
	var current int64
	err := tx.QueryRowContext(
		ctx,
		`SELECT last_sequence FROM sku_sequences WHERE sku_id = ? FOR UPDATE`,
		skuID,
	).Scan(&current)
	if err == sql.ErrNoRows {
		if _, err = tx.ExecContext(
			ctx,
			`INSERT INTO sku_sequences (sku_id, last_sequence) VALUES (?, 1)`,
			skuID,
		); err != nil {
			return 0, fmt.Errorf("insert sku_sequence: %w", err)
		}
		return 1, nil
	}
	if err != nil {
		return 0, fmt.Errorf("select sku_sequence: %w", err)
	}

	next := current + 1
	if _, err = tx.ExecContext(
		ctx,
		`UPDATE sku_sequences SET last_sequence = ? WHERE sku_id = ?`,
		next,
		skuID,
	); err != nil {
		return 0, fmt.Errorf("update sku_sequence: %w", err)
	}
	return next, nil
}

func workerAppendEvent(
	ctx context.Context,
	tx *sql.Tx,
	skuID int64,
	eventType string,
	payload interface{},
) error {
	seq, err := workerNextSequence(ctx, tx, skuID)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal event payload: %w", err)
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO event_logs (id, sku_id, sequence, event_type, payload, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		uuid.New().String(),
		skuID,
		seq,
		eventType,
		raw,
		time.Now(),
	)
	if err != nil {
		return fmt.Errorf("insert event_log: %w", err)
	}
	return nil
}
