package mysqlrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type experienceRepo struct{ db *DB }

func NewExperienceRepo(db *DB) repo.ExperienceRepo {
	return &experienceRepo{db: db}
}

func (r *experienceRepo) ListReasonTags(ctx context.Context, scene string) ([]*domain.ExperienceReasonTag, error) {
	where := []string{"deleted_at IS NULL"}
	args := make([]interface{}, 0, 1)
	if value := strings.TrimSpace(scene); value != "" {
		where = append(where, "scene = ?")
		args = append(args, value)
	}
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT id, scene, code, name, tag_group, severity, version, enabled, deleted_at, sort_order, created_at, updated_at
		FROM experience_reason_tags
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY scene ASC, sort_order ASC, id ASC`, args...)
	if err != nil {
		return nil, fmt.Errorf("list experience reason tags: %w", err)
	}
	defer rows.Close()

	tags := make([]*domain.ExperienceReasonTag, 0)
	for rows.Next() {
		var tag domain.ExperienceReasonTag
		var deletedAt sql.NullTime
		if err := rows.Scan(
			&tag.ID,
			&tag.Scene,
			&tag.Code,
			&tag.Name,
			&tag.Group,
			&tag.Severity,
			&tag.Version,
			&tag.Enabled,
			&deletedAt,
			&tag.SortOrder,
			&tag.CreatedAt,
			&tag.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan experience reason tag: %w", err)
		}
		tag.DeletedAt = fromNullTime(deletedAt)
		tags = append(tags, &tag)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate experience reason tags: %w", err)
	}
	return tags, nil
}

func (r *experienceRepo) ListExperienceEvents(ctx context.Context, filter repo.ExperienceEventListFilter) ([]*domain.ExperienceEvent, int64, error) {
	querySQL, args := buildExperienceSampleUnion(filter)
	if querySQL == "" {
		return []*domain.ExperienceEvent{}, 0, nil
	}

	var total int64
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM (`+querySQL+`) experience_sample_count`, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count experience events: %w", err)
	}

	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	offset := (page - 1) * pageSize
	listArgs := append(append([]interface{}{}, args...), pageSize, offset)
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT id, event_key, schema_version, event_time, source_type, source_id, task_id, action, outcome,
		       actor_snapshot_json, business_snapshot_json, payload_json, data_classification, ground_truth_status, created_at
		FROM (`+querySQL+`) experience_samples
		ORDER BY event_time DESC, id DESC
		LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list experience events: %w", err)
	}
	defer rows.Close()

	events := make([]*domain.ExperienceEvent, 0)
	for rows.Next() {
		event, err := scanExperienceEvent(rows)
		if err != nil {
			return nil, 0, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate experience events: %w", err)
	}
	return events, total, nil
}

func buildExperienceSampleUnion(filter repo.ExperienceEventListFilter) (string, []interface{}) {
	queries := make([]string, 0, 2)
	args := make([]interface{}, 0, 12)

	if shouldIncludeBusinessExperienceSamples(filter) {
		whereSQL, whereArgs := buildBusinessExperienceSampleWhere(filter)
		queries = append(queries, `
		SELECT id, event_key, schema_version, event_time, source_type, source_id, task_id, action, outcome,
		       actor_snapshot_json, business_snapshot_json, payload_json, data_classification, ground_truth_status, created_at
		FROM experience_events
		WHERE `+whereSQL)
		args = append(args, whereArgs...)
	}

	if shouldIncludeAISuggestionSamples(filter) {
		whereSQL, whereArgs := buildAISuggestionSampleWhere(filter)
		queries = append(queries, `
		SELECT -id AS id,
		       suggestion_event_id AS event_key,
		       1 AS schema_version,
		       displayed_at AS event_time,
		       'ai_suggestion' AS source_type,
		       source AS source_id,
		       CASE WHEN target_type = 'task' AND target_id REGEXP '^[0-9]+$' THEN CAST(target_id AS SIGNED) ELSE NULL END AS task_id,
		       suggestion_type AS action,
		       'displayed' AS outcome,
		       CASE
		         WHEN actor_id IS NULL THEN NULL
		         ELSE JSON_OBJECT('actor_id', actor_id, 'actor_type', 'user', 'surface', 'ai_suggestion')
		       END AS actor_snapshot_json,
		       JSON_OBJECT('target_type', target_type, 'target_id', target_id) AS business_snapshot_json,
		       JSON_OBJECT(
		         'suggestion_id', suggestion_id,
		         'source', source,
		         'confidence', confidence,
		         'model', model,
		         'provider', provider,
		         'model_version', model_version,
		         'target_type', target_type,
		         'target_id', target_id
		       ) AS payload_json,
		       'ai_suggestion' AS data_classification,
		       'displayed' AS ground_truth_status,
		       created_at
		FROM ai_suggestion_events
		WHERE `+whereSQL)
		args = append(args, whereArgs...)
	}

	if len(queries) == 0 {
		return "", nil
	}
	return strings.Join(queries, "\n\t\tUNION ALL\n"), args
}

func shouldIncludeBusinessExperienceSamples(filter repo.ExperienceEventListFilter) bool {
	sourceType := strings.TrimSpace(filter.SourceType)
	return sourceType == "" || sourceType != "ai_suggestion"
}

func shouldIncludeAISuggestionSamples(filter repo.ExperienceEventListFilter) bool {
	if sourceType := strings.TrimSpace(filter.SourceType); sourceType != "" && sourceType != "ai_suggestion" {
		return false
	}
	if outcome := strings.TrimSpace(filter.Outcome); outcome != "" && outcome != "displayed" {
		return false
	}
	return true
}

func buildBusinessExperienceSampleWhere(filter repo.ExperienceEventListFilter) (string, []interface{}) {
	where := []string{"1=1"}
	args := make([]interface{}, 0, 8)
	if value := strings.TrimSpace(filter.SourceType); value != "" {
		where = append(where, "source_type = ?")
		args = append(args, value)
	}
	if value := strings.TrimSpace(filter.SourceID); value != "" {
		where = append(where, "source_id = ?")
		args = append(args, value)
	}
	if filter.TaskID != nil {
		where = append(where, "task_id = ?")
		args = append(args, *filter.TaskID)
	}
	if value := strings.TrimSpace(filter.Action); value != "" {
		where = append(where, "action = ?")
		args = append(args, value)
	}
	if value := strings.TrimSpace(filter.Outcome); value != "" {
		where = append(where, "outcome = ?")
		args = append(args, value)
	}
	if filter.From != nil {
		where = append(where, "event_time >= ?")
		args = append(args, *filter.From)
	}
	if filter.To != nil {
		where = append(where, "event_time <= ?")
		args = append(args, *filter.To)
	}
	return strings.Join(where, " AND "), args
}

func buildAISuggestionSampleWhere(filter repo.ExperienceEventListFilter) (string, []interface{}) {
	where := []string{"1=1"}
	args := make([]interface{}, 0, 8)
	if value := strings.TrimSpace(filter.SourceID); value != "" {
		where = append(where, "source = ?")
		args = append(args, value)
	}
	if filter.TaskID != nil {
		where = append(where, "target_type = 'task'")
		where = append(where, "target_id = ?")
		args = append(args, strconv.FormatInt(*filter.TaskID, 10))
	}
	if value := strings.TrimSpace(filter.Action); value != "" {
		where = append(where, "suggestion_type = ?")
		args = append(args, value)
	}
	if filter.From != nil {
		where = append(where, "displayed_at >= ?")
		args = append(args, *filter.From)
	}
	if filter.To != nil {
		where = append(where, "displayed_at <= ?")
		args = append(args, *filter.To)
	}
	return strings.Join(where, " AND "), args
}

func (r *experienceRepo) ExperienceStats(ctx context.Context) (*domain.ExperienceStats, error) {
	now := time.Now().UTC()
	since := now.Add(-24 * time.Hour)
	stats := &domain.ExperienceStats{GeneratedAt: now}
	var err error
	if stats.TotalEvents, err = r.scalarCount(ctx, `SELECT COUNT(*) FROM experience_events`); err != nil {
		return nil, err
	}
	if stats.OutboxQueued, err = r.scalarCount(ctx, `SELECT COUNT(*) FROM experience_outbox WHERE status = 'queued'`); err != nil {
		return nil, err
	}
	if stats.OutboxProcessing, err = r.scalarCount(ctx, `SELECT COUNT(*) FROM experience_outbox WHERE status = 'processing'`); err != nil {
		return nil, err
	}
	if stats.OutboxDeadLetter, err = r.scalarCount(ctx, `SELECT COUNT(*) FROM experience_outbox WHERE status = 'dead_letter'`); err != nil {
		return nil, err
	}
	if stats.OutboxProcessed24h, err = r.scalarCount(ctx, `SELECT COUNT(*) FROM experience_outbox WHERE status = 'processed' AND updated_at >= ?`, since); err != nil {
		return nil, err
	}
	if stats.OutboxFailed24h, err = r.scalarCount(ctx, `
		SELECT COUNT(*) FROM experience_outbox
		WHERE updated_at >= ?
		  AND (status = 'dead_letter' OR (status = 'queued' AND attempt_count > 0))`, since); err != nil {
		return nil, err
	}
	if stats.TagTotal, err = r.scalarCount(ctx, `SELECT COUNT(*) FROM experience_reason_tags WHERE deleted_at IS NULL`); err != nil {
		return nil, err
	}
	if stats.TagEnabled, err = r.scalarCount(ctx, `SELECT COUNT(*) FROM experience_reason_tags WHERE enabled = 1 AND deleted_at IS NULL`); err != nil {
		return nil, err
	}
	if stats.AISuggestionEvents, err = r.scalarCount(ctx, `SELECT COUNT(*) FROM ai_suggestion_events`); err != nil {
		return nil, err
	}
	if stats.AIFeedbackEvents, err = r.scalarCount(ctx, `SELECT COUNT(*) FROM ai_suggestion_feedback`); err != nil {
		return nil, err
	}
	if stats.TaskProfiles, err = r.scalarCount(ctx, `SELECT COUNT(*) FROM task_experience_profiles`); err != nil {
		return nil, err
	}
	if stats.AssetQualityLabels, err = r.scalarCount(ctx, `SELECT COUNT(*) FROM asset_quality_labels`); err != nil {
		return nil, err
	}
	taggedEvents, err := r.scalarCount(ctx, `
		SELECT COUNT(*) FROM experience_events
		WHERE payload_json IS NOT NULL
		  AND (
		    JSON_EXTRACT(payload_json, '$.reason_code') IS NOT NULL
		    OR JSON_EXTRACT(payload_json, '$.reason_codes') IS NOT NULL
		    OR JSON_EXTRACT(payload_json, '$.reason_tags') IS NOT NULL
		  )`)
	if err != nil {
		return nil, err
	}
	if stats.TotalEvents > 0 {
		stats.TagCoverageRate = float64(taggedEvents) / float64(stats.TotalEvents)
	}
	outbox24h := stats.OutboxProcessed24h + stats.OutboxFailed24h
	if outbox24h > 0 {
		stats.CaptureSuccessRate24h = float64(stats.OutboxProcessed24h) / float64(outbox24h)
		stats.CaptureFailureRate24h = float64(stats.OutboxFailed24h) / float64(outbox24h)
	}
	if stats.AISuggestionEvents > 0 {
		stats.AIFeedbackRate = float64(stats.AIFeedbackEvents) / float64(stats.AISuggestionEvents)
	}
	var rebuiltAt sql.NullTime
	if err := r.db.db.QueryRowContext(ctx, `SELECT MAX(rebuilt_at) FROM task_experience_profiles`).Scan(&rebuiltAt); err != nil {
		return nil, fmt.Errorf("query latest experience profile rebuild: %w", err)
	}
	stats.LatestProfileRebuiltAt = fromNullTime(rebuiltAt)
	return stats, nil
}

func (r *experienceRepo) EnqueueExperienceEvent(ctx context.Context, event *domain.ExperienceOutboxEvent) error {
	if event == nil {
		return fmt.Errorf("experience outbox event is nil")
	}
	status := strings.TrimSpace(event.Status)
	if status == "" {
		status = domain.ExperienceOutboxStatusQueued
	}
	_, err := r.db.db.ExecContext(ctx, `
		INSERT INTO experience_outbox (
			event_key, schema_version, source_type, source_id, task_id, action, outcome, event_time,
			actor_snapshot_json, business_snapshot_json, payload_json, data_classification, ground_truth_status, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE updated_at = updated_at`,
		event.EventKey,
		event.SchemaVersion,
		event.SourceType,
		event.SourceID,
		toNullInt64(event.TaskID),
		event.Action,
		event.Outcome,
		event.EventTime,
		toNullJSONString(event.ActorSnapshot),
		toNullJSONString(event.BusinessSnapshot),
		toNullJSONString(event.Payload),
		event.DataClassification,
		event.GroundTruthStatus,
		status,
	)
	if err != nil {
		return fmt.Errorf("enqueue experience event: %w", err)
	}
	return nil
}

func (r *experienceRepo) ClaimExperienceOutbox(ctx context.Context, limit int, claimToken string, now time.Time, leaseTTL time.Duration) ([]*domain.ExperienceOutboxEvent, error) {
	if limit <= 0 {
		limit = 50
	}
	if leaseTTL <= 0 {
		leaseTTL = 5 * time.Minute
	}
	staleBefore := now.Add(-leaseTTL)
	_, err := r.db.db.ExecContext(ctx, `
		UPDATE experience_outbox
		SET status = 'processing',
		    claimed_by = ?,
		    claimed_at = ?,
		    attempt_count = attempt_count + 1,
		    updated_at = ?
		WHERE id IN (
		  SELECT id FROM (
		    SELECT id
		    FROM experience_outbox
		    WHERE (
		      (status = 'queued' AND (next_retry_at IS NULL OR next_retry_at <= ?))
		      OR (status = 'processing' AND claimed_at < ?)
		    )
		    ORDER BY id ASC
		    LIMIT ?
		  ) pending
		)`,
		claimToken,
		now,
		now,
		now,
		staleBefore,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("claim experience outbox: %w", err)
	}
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT id, event_key, schema_version, source_type, source_id, task_id, action, outcome, event_time,
		       actor_snapshot_json, business_snapshot_json, payload_json, data_classification, ground_truth_status,
		       status, attempt_count, last_error, next_retry_at, claimed_by, claimed_at, processed_at, created_at, updated_at
		FROM experience_outbox
		WHERE status = 'processing' AND claimed_by = ?
		ORDER BY id ASC
		LIMIT ?`, claimToken, limit)
	if err != nil {
		return nil, fmt.Errorf("select claimed experience outbox: %w", err)
	}
	defer rows.Close()

	events := make([]*domain.ExperienceOutboxEvent, 0)
	for rows.Next() {
		event, err := scanExperienceOutboxEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate claimed experience outbox: %w", err)
	}
	return events, nil
}

func (r *experienceRepo) CreateExperienceEventFromOutbox(ctx context.Context, outbox *domain.ExperienceOutboxEvent) error {
	if outbox == nil {
		return fmt.Errorf("experience outbox event is nil")
	}
	_, err := r.db.db.ExecContext(ctx, `
		INSERT IGNORE INTO experience_events (
			event_key, schema_version, event_time, source_type, source_id, task_id, action, outcome,
			actor_snapshot_json, business_snapshot_json, payload_json, data_classification, ground_truth_status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		outbox.EventKey,
		outbox.SchemaVersion,
		outbox.EventTime,
		outbox.SourceType,
		outbox.SourceID,
		toNullInt64(outbox.TaskID),
		outbox.Action,
		outbox.Outcome,
		toNullJSONString(outbox.ActorSnapshot),
		toNullJSONString(outbox.BusinessSnapshot),
		toNullJSONString(outbox.Payload),
		outbox.DataClassification,
		outbox.GroundTruthStatus,
	)
	if err != nil {
		return fmt.Errorf("create experience event from outbox: %w", err)
	}
	return nil
}

func (r *experienceRepo) MarkExperienceOutboxProcessed(ctx context.Context, id int64, now time.Time) error {
	_, err := r.db.db.ExecContext(ctx, `
		UPDATE experience_outbox
		SET status = 'processed',
		    processed_at = ?,
		    last_error = '',
		    next_retry_at = NULL,
		    updated_at = ?
		WHERE id = ?`, now, now, id)
	if err != nil {
		return fmt.Errorf("mark experience outbox processed: %w", err)
	}
	return nil
}

func (r *experienceRepo) MarkExperienceOutboxFailed(ctx context.Context, id int64, attempts int, maxAttempts int, message string, now time.Time) (bool, error) {
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	message = strings.TrimSpace(message)
	if len(message) > 1024 {
		message = message[:1024]
	}
	dead := attempts >= maxAttempts
	status := domain.ExperienceOutboxStatusQueued
	var nextRetry interface{} = now.Add(time.Duration(attempts*attempts*10) * time.Second)
	if dead {
		status = domain.ExperienceOutboxStatusDeadLetter
		nextRetry = nil
	}
	_, err := r.db.db.ExecContext(ctx, `
		UPDATE experience_outbox
		SET status = ?,
		    last_error = ?,
		    next_retry_at = ?,
		    updated_at = ?
		WHERE id = ?`, status, message, nextRetry, now, id)
	if err != nil {
		return false, fmt.Errorf("mark experience outbox failed: %w", err)
	}
	return dead, nil
}

func (r *experienceRepo) CreateAISuggestionEvent(ctx context.Context, event *domain.AISuggestionEvent) error {
	if event == nil {
		return fmt.Errorf("ai suggestion event is nil")
	}
	_, err := r.db.db.ExecContext(ctx, `
		INSERT IGNORE INTO ai_suggestion_events (
			suggestion_event_id, suggestion_type, suggestion_id, source, confidence, model, provider, model_version,
			input_summary_json, suggestion_json, target_type, target_id, actor_id, displayed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.SuggestionEventID,
		event.SuggestionType,
		event.SuggestionID,
		event.Source,
		toNullFloat64(event.Confidence),
		event.Model,
		event.Provider,
		event.ModelVersion,
		toNullJSONString(event.InputSummary),
		toNullJSONString(event.Suggestion),
		event.TargetType,
		event.TargetID,
		toNullInt64(event.ActorID),
		event.DisplayedAt,
	)
	if err != nil {
		return fmt.Errorf("create ai suggestion event: %w", err)
	}
	return nil
}

func (r *experienceRepo) CreateAISuggestionFeedback(ctx context.Context, feedback *domain.AISuggestionFeedback) (int64, error) {
	if feedback == nil {
		return 0, fmt.Errorf("ai suggestion feedback is nil")
	}
	result, err := r.db.db.ExecContext(ctx, `
		INSERT INTO ai_suggestion_feedback (
			suggestion_event_id, feedback_value, reason_code, reason_note, outcome_source_type, outcome_source_id, actor_id, payload_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		feedback.SuggestionEventID,
		feedback.FeedbackValue,
		feedback.ReasonCode,
		feedback.ReasonNote,
		feedback.OutcomeSourceType,
		feedback.OutcomeSourceID,
		toNullInt64(feedback.ActorID),
		toNullJSONString(feedback.Payload),
	)
	if err != nil {
		return 0, fmt.Errorf("create ai suggestion feedback: %w", err)
	}
	return result.LastInsertId()
}

func (r *experienceRepo) scalarCount(ctx context.Context, query string, args ...interface{}) (int64, error) {
	var count int64
	if err := r.db.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("query experience scalar count: %w", err)
	}
	return count, nil
}

func scanExperienceEvent(scanner interface {
	Scan(...interface{}) error
}) (*domain.ExperienceEvent, error) {
	var event domain.ExperienceEvent
	var taskID sql.NullInt64
	var actorSnapshot, businessSnapshot, payload sql.NullString
	if err := scanner.Scan(
		&event.ID,
		&event.EventKey,
		&event.SchemaVersion,
		&event.EventTime,
		&event.SourceType,
		&event.SourceID,
		&taskID,
		&event.Action,
		&event.Outcome,
		&actorSnapshot,
		&businessSnapshot,
		&payload,
		&event.DataClassification,
		&event.GroundTruthStatus,
		&event.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan experience event: %w", err)
	}
	event.TaskID = fromNullInt64(taskID)
	event.ActorSnapshot = rawJSONFromNull(actorSnapshot)
	event.BusinessSnapshot = rawJSONFromNull(businessSnapshot)
	event.Payload = rawJSONFromNull(payload)
	return &event, nil
}

func scanExperienceOutboxEvent(scanner interface {
	Scan(...interface{}) error
}) (*domain.ExperienceOutboxEvent, error) {
	var event domain.ExperienceOutboxEvent
	var taskID sql.NullInt64
	var actorSnapshot, businessSnapshot, payload sql.NullString
	var nextRetryAt, claimedAt, processedAt sql.NullTime
	if err := scanner.Scan(
		&event.ID,
		&event.EventKey,
		&event.SchemaVersion,
		&event.SourceType,
		&event.SourceID,
		&taskID,
		&event.Action,
		&event.Outcome,
		&event.EventTime,
		&actorSnapshot,
		&businessSnapshot,
		&payload,
		&event.DataClassification,
		&event.GroundTruthStatus,
		&event.Status,
		&event.AttemptCount,
		&event.LastError,
		&nextRetryAt,
		&event.ClaimedBy,
		&claimedAt,
		&processedAt,
		&event.CreatedAt,
		&event.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan experience outbox event: %w", err)
	}
	event.TaskID = fromNullInt64(taskID)
	event.ActorSnapshot = rawJSONFromNull(actorSnapshot)
	event.BusinessSnapshot = rawJSONFromNull(businessSnapshot)
	event.Payload = rawJSONFromNull(payload)
	event.NextRetryAt = fromNullTime(nextRetryAt)
	event.ClaimedAt = fromNullTime(claimedAt)
	event.ProcessedAt = fromNullTime(processedAt)
	return &event, nil
}

func rawJSONFromNull(value sql.NullString) json.RawMessage {
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return nil
	}
	return json.RawMessage(value.String)
}
