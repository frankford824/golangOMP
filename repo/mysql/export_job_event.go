package mysqlrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"workflow/domain"
	"workflow/repo"
)

type exportJobEventRepo struct{ db *DB }

func NewExportJobEventRepo(db *DB) repo.ExportJobEventRepo { return &exportJobEventRepo{db: db} }

func (r *exportJobEventRepo) Append(ctx context.Context, tx repo.Tx, event *domain.ExportJobEvent) (*domain.ExportJobEvent, error) {
	if event == nil {
		return nil, fmt.Errorf("append export job event: event is nil")
	}
	sqlTx := Unwrap(tx)

	seq, err := nextExportJobEventSequence(ctx, sqlTx, event.ExportJobID)
	if err != nil {
		return nil, err
	}

	payload := event.Payload
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	if !json.Valid(payload) {
		return nil, fmt.Errorf("append export job event: payload is not valid json")
	}

	eventID := strings.TrimSpace(event.EventID)
	if eventID == "" {
		eventID = uuid.NewString()
	}
	createdAt := event.CreatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	_, err = sqlTx.ExecContext(ctx, `
		INSERT INTO export_job_events (
			event_id, export_job_id, sequence, event_type, from_status, to_status,
			actor_id, actor_type, note, payload, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		eventID,
		event.ExportJobID,
		seq,
		event.EventType,
		exportJobStatusToNullString(event.FromStatus),
		exportJobStatusToNullString(event.ToStatus),
		event.ActorID,
		strings.TrimSpace(event.ActorType),
		strings.TrimSpace(event.Note),
		[]byte(payload),
		createdAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert export_job_event: %w", err)
	}

	copyEvent := *event
	copyEvent.EventID = eventID
	copyEvent.Sequence = seq
	copyEvent.Payload = append(json.RawMessage(nil), payload...)
	copyEvent.CreatedAt = createdAt
	return &copyEvent, nil
}

func (r *exportJobEventRepo) ListByExportJobID(ctx context.Context, exportJobID int64) ([]*domain.ExportJobEvent, error) {
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT event_id, export_job_id, sequence, event_type, from_status, to_status,
		       actor_id, actor_type, note, payload, created_at
		FROM export_job_events
		WHERE export_job_id = ?
		ORDER BY sequence ASC`, exportJobID)
	if err != nil {
		return nil, fmt.Errorf("list export_job_events: %w", err)
	}
	defer rows.Close()

	events := make([]*domain.ExportJobEvent, 0)
	for rows.Next() {
		event, err := scanExportJobEvent(rows, true)
		if err != nil {
			return nil, fmt.Errorf("scan export_job_event: %w", err)
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (r *exportJobEventRepo) ListRecent(ctx context.Context, filter repo.ExportJobEventListFilter) ([]*domain.ExportJobEvent, int64, error) {
	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	where := []string{"1=1"}
	args := make([]interface{}, 0, 4)
	if filter.ExportJobID != nil && *filter.ExportJobID > 0 {
		where = append(where, "export_job_id = ?")
		args = append(args, *filter.ExportJobID)
	}
	if eventType := strings.TrimSpace(filter.EventType); eventType != "" {
		where = append(where, "event_type = ?")
		args = append(args, eventType)
	}

	countQuery := `SELECT COUNT(*) FROM export_job_events WHERE ` + strings.Join(where, " AND ")
	var total int64
	if err := r.db.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count export_job_events: %w", err)
	}

	queryArgs := append([]interface{}{}, args...)
	queryArgs = append(queryArgs, pageSize, (page-1)*pageSize)
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT event_id, export_job_id, sequence, event_type, from_status, to_status,
		       actor_id, actor_type, note, payload, created_at
		FROM export_job_events
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY created_at DESC, sequence DESC
		LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list recent export_job_events: %w", err)
	}
	defer rows.Close()

	events := make([]*domain.ExportJobEvent, 0)
	for rows.Next() {
		event, err := scanExportJobEvent(rows, true)
		if err != nil {
			return nil, 0, fmt.Errorf("scan recent export_job_event: %w", err)
		}
		events = append(events, event)
	}
	return events, total, rows.Err()
}

func (r *exportJobEventRepo) SummariesByExportJobIDs(ctx context.Context, exportJobIDs []int64) (map[int64]repo.ExportJobEventAggregate, error) {
	out := make(map[int64]repo.ExportJobEventAggregate, len(exportJobIDs))
	if len(exportJobIDs) == 0 {
		return out, nil
	}

	inClause, args := buildInt64InClause("export_job_id", exportJobIDs)
	countRows, err := r.db.db.QueryContext(ctx, `
		SELECT export_job_id, COUNT(*)
		FROM export_job_events
		WHERE `+inClause+`
		GROUP BY export_job_id`, args...)
	if err != nil {
		return nil, fmt.Errorf("count export_job_events: %w", err)
	}
	defer countRows.Close()

	for countRows.Next() {
		var exportJobID int64
		var count int64
		if err := countRows.Scan(&exportJobID, &count); err != nil {
			return nil, fmt.Errorf("scan export_job_event count: %w", err)
		}
		aggregate := out[exportJobID]
		aggregate.EventCount = count
		out[exportJobID] = aggregate
	}
	if err := countRows.Err(); err != nil {
		return nil, err
	}

	latestRows, err := r.db.db.QueryContext(ctx, `
		SELECT e.event_id, e.export_job_id, e.sequence, e.event_type, e.from_status, e.to_status,
		       e.actor_id, e.actor_type, e.note, e.created_at
		FROM export_job_events e
		INNER JOIN (
			SELECT export_job_id, MAX(sequence) AS max_sequence
			FROM export_job_events
			WHERE `+inClause+`
			GROUP BY export_job_id
		) latest
		  ON latest.export_job_id = e.export_job_id AND latest.max_sequence = e.sequence`, args...)
	if err != nil {
		return nil, fmt.Errorf("list latest export_job_events: %w", err)
	}
	defer latestRows.Close()

	for latestRows.Next() {
		event, err := scanExportJobEvent(latestRows, false)
		if err != nil {
			return nil, fmt.Errorf("scan latest export_job_event: %w", err)
		}
		aggregate := out[event.ExportJobID]
		aggregate.LatestEvent = domain.SummarizeExportJobEvent(event)
		out[event.ExportJobID] = aggregate
	}
	if err := latestRows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func (r *exportJobEventRepo) LatestSummariesByExportJobIDsAndTypes(ctx context.Context, exportJobIDs []int64, eventTypes []string) (map[int64]*domain.ExportJobEventSummary, error) {
	out := make(map[int64]*domain.ExportJobEventSummary, len(exportJobIDs))
	if len(exportJobIDs) == 0 || len(eventTypes) == 0 {
		return out, nil
	}

	jobInClause, jobArgs := buildInt64InClause("export_job_id", exportJobIDs)
	typeInClause, typeArgs := buildStringInClause("event_type", eventTypes)
	args := append(jobArgs, typeArgs...)

	rows, err := r.db.db.QueryContext(ctx, `
		SELECT e.event_id, e.export_job_id, e.sequence, e.event_type, e.from_status, e.to_status,
		       e.actor_id, e.actor_type, e.note, e.created_at
		FROM export_job_events e
		INNER JOIN (
			SELECT export_job_id, MAX(sequence) AS max_sequence
			FROM export_job_events
			WHERE `+jobInClause+` AND `+typeInClause+`
			GROUP BY export_job_id
		) latest
		  ON latest.export_job_id = e.export_job_id AND latest.max_sequence = e.sequence`, args...)
	if err != nil {
		return nil, fmt.Errorf("list latest filtered export_job_events: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		event, err := scanExportJobEvent(rows, false)
		if err != nil {
			return nil, fmt.Errorf("scan latest filtered export_job_event: %w", err)
		}
		out[event.ExportJobID] = domain.SummarizeExportJobEvent(event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func scanExportJobEvent(scanner interface {
	Scan(...interface{}) error
}, includePayload bool) (*domain.ExportJobEvent, error) {
	event := &domain.ExportJobEvent{}
	var fromStatus sql.NullString
	var toStatus sql.NullString
	if includePayload {
		if err := scanner.Scan(
			&event.EventID,
			&event.ExportJobID,
			&event.Sequence,
			&event.EventType,
			&fromStatus,
			&toStatus,
			&event.ActorID,
			&event.ActorType,
			&event.Note,
			&event.Payload,
			&event.CreatedAt,
		); err != nil {
			return nil, err
		}
	} else {
		if err := scanner.Scan(
			&event.EventID,
			&event.ExportJobID,
			&event.Sequence,
			&event.EventType,
			&fromStatus,
			&toStatus,
			&event.ActorID,
			&event.ActorType,
			&event.Note,
			&event.CreatedAt,
		); err != nil {
			return nil, err
		}
	}
	event.FromStatus = exportJobStatusPtrFromNullString(fromStatus)
	event.ToStatus = exportJobStatusPtrFromNullString(toStatus)
	return event, nil
}

func nextExportJobEventSequence(ctx context.Context, sqlTx *sql.Tx, exportJobID int64) (int64, error) {
	var current int64
	err := sqlTx.QueryRowContext(ctx,
		`SELECT last_sequence FROM export_job_event_sequences WHERE export_job_id = ? FOR UPDATE`,
		exportJobID,
	).Scan(&current)

	if err == sql.ErrNoRows {
		if _, err = sqlTx.ExecContext(ctx,
			`INSERT INTO export_job_event_sequences (export_job_id, last_sequence) VALUES (?, 1)`,
			exportJobID,
		); err != nil {
			return 0, fmt.Errorf("export_job_event nextSequence insert: %w", err)
		}
		return 1, nil
	}
	if err != nil {
		return 0, fmt.Errorf("export_job_event nextSequence select: %w", err)
	}

	next := current + 1
	if _, err = sqlTx.ExecContext(ctx,
		`UPDATE export_job_event_sequences SET last_sequence = ? WHERE export_job_id = ?`,
		next, exportJobID,
	); err != nil {
		return 0, fmt.Errorf("export_job_event nextSequence update: %w", err)
	}
	return next, nil
}

func buildInt64InClause(column string, values []int64) (string, []interface{}) {
	placeholders := make([]string, 0, len(values))
	args := make([]interface{}, 0, len(values))
	for _, value := range values {
		placeholders = append(placeholders, "?")
		args = append(args, value)
	}
	return column + " IN (" + strings.Join(placeholders, ", ") + ")", args
}

func buildStringInClause(column string, values []string) (string, []interface{}) {
	placeholders := make([]string, 0, len(values))
	args := make([]interface{}, 0, len(values))
	for _, value := range values {
		placeholders = append(placeholders, "?")
		args = append(args, value)
	}
	return column + " IN (" + strings.Join(placeholders, ", ") + ")", args
}

func exportJobStatusToNullString(value *domain.ExportJobStatus) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	status := string(*value)
	return sql.NullString{String: status, Valid: true}
}

func exportJobStatusPtrFromNullString(value sql.NullString) *domain.ExportJobStatus {
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return nil
	}
	status := domain.ExportJobStatus(value.String)
	return &status
}
