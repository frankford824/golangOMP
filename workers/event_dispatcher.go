package workers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const eventDispatcherCursorKey = "event_dispatcher:cursor"

// EventDispatcher tails event_logs and fans out WebSocket pushes via Redis pub/sub.
//
// This is the ONLY component allowed to send WebSocket messages.
// Business code MUST NOT push WS messages directly (spec §4.1, §5.2 invariant 8).
// The frontend sequence-gap detection relies on events arriving in order from this dispatcher.
type EventDispatcher struct {
	db       *sql.DB
	rdb      *redis.Client
	logger   *zap.Logger
	interval time.Duration
}

func NewEventDispatcher(db *sql.DB, rdb *redis.Client, logger *zap.Logger) *EventDispatcher {
	return &EventDispatcher{
		db:       db,
		rdb:      rdb,
		logger:   logger,
		interval: 500 * time.Millisecond,
	}
}

func (w *EventDispatcher) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	w.logger.Info("EventDispatcher started", zap.Duration("interval", w.interval))
	for {
		select {
		case <-ctx.Done():
			w.logger.Info("EventDispatcher stopped")
			return
		case <-ticker.C:
			if err := w.dispatch(ctx); err != nil {
				w.logger.Error("EventDispatcher error", zap.Error(err))
			}
		}
	}
}

func (w *EventDispatcher) DispatchOnce(ctx context.Context) error {
	return w.dispatch(ctx)
}

func (w *EventDispatcher) dispatch(ctx context.Context) error {
	cursorTime, cursorID, err := w.loadCursor(ctx)
	if err != nil {
		return err
	}

	type eventRow struct {
		ID        string
		SKUID     int64
		Sequence  int64
		EventType string
		Payload   []byte
		CreatedAt time.Time
	}

	rows, err := w.db.QueryContext(ctx, `
		SELECT id, sku_id, sequence, event_type, payload, created_at
		FROM event_logs
		WHERE (created_at > ?) OR (created_at = ? AND id > ?)
		ORDER BY created_at ASC, id ASC
		LIMIT 100`,
		cursorTime,
		cursorTime,
		cursorID,
	)
	if err != nil {
		return fmt.Errorf("query dispatch events: %w", err)
	}
	defer rows.Close()

	var events []eventRow
	for rows.Next() {
		var e eventRow
		if err = rows.Scan(&e.ID, &e.SKUID, &e.Sequence, &e.EventType, &e.Payload, &e.CreatedAt); err != nil {
			return fmt.Errorf("scan dispatch event: %w", err)
		}
		events = append(events, e)
	}
	if err = rows.Err(); err != nil {
		return fmt.Errorf("iterate dispatch events: %w", err)
	}
	if len(events) == 0 {
		return nil
	}

	for _, event := range events {
		wirePayload, marshalErr := json.Marshal(map[string]interface{}{
			"sku_id":     event.SKUID,
			"sequence":   event.Sequence,
			"event_type": event.EventType,
			"payload":    json.RawMessage(event.Payload),
			"created_at": event.CreatedAt,
		})
		if marshalErr != nil {
			return fmt.Errorf("marshal ws payload: %w", marshalErr)
		}
		if pubErr := w.rdb.Publish(ctx, fmt.Sprintf("ws:sku:%d", event.SKUID), wirePayload).Err(); pubErr != nil {
			return fmt.Errorf("publish ws payload: %w", pubErr)
		}
	}

	last := events[len(events)-1]
	cursorStr := fmt.Sprintf("%s|%s", last.CreatedAt.UTC().Format(time.RFC3339Nano), last.ID)
	if err = w.rdb.Set(ctx, eventDispatcherCursorKey, cursorStr, 0).Err(); err != nil {
		return fmt.Errorf("persist dispatch cursor: %w", err)
	}

	if len(events) == 100 {
		return w.dispatch(ctx)
	}
	return nil
}

func (w *EventDispatcher) loadCursor(ctx context.Context) (time.Time, string, error) {
	cursorStr, err := w.rdb.Get(ctx, eventDispatcherCursorKey).Result()
	if err == redis.Nil {
		return time.Time{}, "", nil
	}
	if err != nil {
		return time.Time{}, "", fmt.Errorf("load dispatch cursor: %w", err)
	}

	parts := strings.SplitN(cursorStr, "|", 2)
	if len(parts) != 2 {
		return time.Time{}, "", nil
	}
	t, parseErr := time.Parse(time.RFC3339Nano, parts[0])
	if parseErr != nil {
		return time.Time{}, "", nil
	}
	return t, parts[1], nil
}
