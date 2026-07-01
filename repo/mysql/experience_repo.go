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

	evidenceRank := experienceEvidenceRank(filter.MinEvidenceLevel)
	outerWhere := ""
	countArgs := append([]interface{}{}, args...)
	if evidenceRank > 0 {
		outerWhere = " WHERE evidence_rank >= ?"
		countArgs = append(countArgs, evidenceRank)
	}

	var total int64
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM (`+querySQL+`) experience_sample_count`+outerWhere, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count experience events: %w", err)
	}

	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	offset := (page - 1) * pageSize
	listArgs := append([]interface{}{}, args...)
	if evidenceRank > 0 {
		listArgs = append(listArgs, evidenceRank)
	}
	listArgs = append(listArgs, pageSize, offset)
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT id, event_key, schema_version, event_time, source_type, source_id, task_id, action, outcome,
		       actor_snapshot_json, business_snapshot_json, payload_json, data_classification, ground_truth_status,
		       feedback_value, feedback_reason_code, feedback_created_at, evidence_rank, created_at
		FROM (`+querySQL+`) experience_samples
		`+outerWhere+`
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
		SELECT e.id, e.event_key, e.schema_version, e.event_time, e.source_type, e.source_id, e.task_id, e.action, e.outcome,
		       e.actor_snapshot_json, e.business_snapshot_json, e.payload_json, e.data_classification, e.ground_truth_status,
		       NULL AS feedback_value,
		       NULL AS feedback_reason_code,
		       NULL AS feedback_created_at,
		       CASE
		         WHEN e.task_id IS NOT NULL AND EXISTS (
		           SELECT 1 FROM task_experience_profiles p WHERE p.task_id = e.task_id
		         ) THEN 4
		         WHEN e.payload_json IS NOT NULL AND (
		           JSON_EXTRACT(e.payload_json, '$.reason_code') IS NOT NULL
		           OR JSON_EXTRACT(e.payload_json, '$.reason_codes') IS NOT NULL
		           OR JSON_EXTRACT(e.payload_json, '$.reason_tags') IS NOT NULL
		         ) THEN 3
		         WHEN e.task_id IS NOT NULL OR e.source_id <> '' THEN 1
		         ELSE 0
		       END AS evidence_rank,
		       e.created_at
		FROM experience_events e
		WHERE `+whereSQL)
		args = append(args, whereArgs...)
	}

	if shouldIncludeAISuggestionSamples(filter) {
		whereSQL, whereArgs := buildAISuggestionSampleWhere(filter)
		queries = append(queries, `
		SELECT -a.id AS id,
		       a.suggestion_event_id AS event_key,
		       1 AS schema_version,
		       a.displayed_at AS event_time,
		       'ai_suggestion' AS source_type,
		       a.source AS source_id,
		       CASE WHEN a.target_type = 'task' AND a.target_id REGEXP '^[0-9]+$' THEN CAST(a.target_id AS SIGNED) ELSE NULL END AS task_id,
		       a.suggestion_type AS action,
		       'displayed' AS outcome,
		       CASE
		         WHEN a.actor_id IS NULL THEN NULL
		         ELSE JSON_OBJECT('actor_id', a.actor_id, 'actor_type', 'user', 'surface', 'ai_suggestion')
		       END AS actor_snapshot_json,
		       JSON_OBJECT('target_type', a.target_type, 'target_id', a.target_id) AS business_snapshot_json,
		       JSON_OBJECT(
		         'suggestion_id', a.suggestion_id,
		         'source', a.source,
		         'confidence', a.confidence,
		         'model', a.model,
		         'provider', a.provider,
		         'model_version', a.model_version,
		         'target_type', a.target_type,
		         'target_id', a.target_id
		       ) AS payload_json,
		       'ai_suggestion' AS data_classification,
		       'displayed' AS ground_truth_status,
		       lf.feedback_value AS feedback_value,
		       lf.reason_code AS feedback_reason_code,
		       lf.created_at AS feedback_created_at,
		       CASE
		         WHEN a.target_type = 'task' AND a.target_id REGEXP '^[0-9]+$' AND EXISTS (
		           SELECT 1 FROM task_experience_profiles p WHERE p.task_id = CAST(a.target_id AS SIGNED)
		         ) THEN 4
		         WHEN a.target_type = 'asset' AND a.target_id REGEXP '^[0-9]+$' AND EXISTS (
		           SELECT 1 FROM asset_quality_labels q WHERE q.asset_id = CAST(a.target_id AS SIGNED)
		         ) THEN 4
		         WHEN lf.reason_code <> '' THEN 3
		         WHEN lf.feedback_value <> '' THEN 2
		         WHEN a.target_type <> '' AND a.target_id <> '' THEN 1
		         ELSE 0
		       END AS evidence_rank,
		       a.created_at
		FROM ai_suggestion_events a
		LEFT JOIN (`+latestAISuggestionFeedbackSQL()+`) lf ON lf.suggestion_event_id = a.suggestion_event_id
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

func latestAISuggestionFeedbackSQL() string {
	return `
		SELECT f.suggestion_event_id, f.feedback_value, f.reason_code, f.created_at
		FROM ai_suggestion_feedback f
		INNER JOIN (
		  SELECT suggestion_event_id, MAX(id) AS id
		  FROM ai_suggestion_feedback
		  GROUP BY suggestion_event_id
		) latest ON latest.id = f.id`
}

func experienceEvidenceRank(level string) int {
	switch strings.ToUpper(strings.TrimSpace(level)) {
	case domain.ExperienceEvidenceLocatable:
		return 1
	case domain.ExperienceEvidenceFeedback:
		return 2
	case domain.ExperienceEvidenceTagged:
		return 3
	case domain.ExperienceEvidenceReusable:
		return 4
	default:
		return 0
	}
}

func buildBusinessExperienceSampleWhere(filter repo.ExperienceEventListFilter) (string, []interface{}) {
	where := []string{"1=1"}
	args := make([]interface{}, 0, 8)
	if value := strings.TrimSpace(filter.SourceType); value != "" {
		where = append(where, "e.source_type = ?")
		args = append(args, value)
	}
	if value := strings.TrimSpace(filter.SourceID); value != "" {
		where = append(where, "e.source_id = ?")
		args = append(args, value)
	}
	if filter.TaskID != nil {
		where = append(where, "e.task_id = ?")
		args = append(args, *filter.TaskID)
	}
	if value := strings.TrimSpace(filter.Action); value != "" {
		where = append(where, "e.action = ?")
		args = append(args, value)
	}
	if value := strings.TrimSpace(filter.Outcome); value != "" {
		where = append(where, "e.outcome = ?")
		args = append(args, value)
	}
	if filter.From != nil {
		where = append(where, "e.event_time >= ?")
		args = append(args, *filter.From)
	}
	if filter.To != nil {
		where = append(where, "e.event_time <= ?")
		args = append(args, *filter.To)
	}
	return strings.Join(where, " AND "), args
}

func buildAISuggestionSampleWhere(filter repo.ExperienceEventListFilter) (string, []interface{}) {
	where := []string{"1=1"}
	args := make([]interface{}, 0, 8)
	if value := strings.TrimSpace(filter.SourceID); value != "" {
		where = append(where, "a.source = ?")
		args = append(args, value)
	}
	if filter.TaskID != nil {
		where = append(where, "a.target_type = 'task'")
		where = append(where, "a.target_id = ?")
		args = append(args, strconv.FormatInt(*filter.TaskID, 10))
	}
	if value := strings.TrimSpace(filter.Action); value != "" {
		where = append(where, "a.suggestion_type = ?")
		args = append(args, value)
	}
	if filter.From != nil {
		where = append(where, "a.displayed_at >= ?")
		args = append(args, *filter.From)
	}
	if filter.To != nil {
		where = append(where, "a.displayed_at <= ?")
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
	stats.DisplayedEvents = stats.AISuggestionEvents
	stats.SampleTotal = stats.TotalEvents + stats.AISuggestionEvents
	if stats.AIFeedbackEvents, err = r.scalarCount(ctx, `SELECT COUNT(*) FROM (`+latestAISuggestionFeedbackSQL()+`) latest_feedback`); err != nil {
		return nil, err
	}
	stats.FeedbackSamples = stats.AIFeedbackEvents
	if stats.FeedbackAccepted, err = r.scalarCount(ctx, `SELECT COUNT(*) FROM (`+latestAISuggestionFeedbackSQL()+`) latest_feedback WHERE feedback_value = ?`, domain.ExperienceFeedbackAccepted); err != nil {
		return nil, err
	}
	if stats.FeedbackPartiallyAccepted, err = r.scalarCount(ctx, `SELECT COUNT(*) FROM (`+latestAISuggestionFeedbackSQL()+`) latest_feedback WHERE feedback_value = ?`, domain.ExperienceFeedbackPartiallyAccepted); err != nil {
		return nil, err
	}
	if stats.FeedbackRejected, err = r.scalarCount(ctx, `SELECT COUNT(*) FROM (`+latestAISuggestionFeedbackSQL()+`) latest_feedback WHERE feedback_value = ?`, domain.ExperienceFeedbackRejected); err != nil {
		return nil, err
	}
	if stats.ReasonedFeedbackSamples, err = r.scalarCount(ctx, `SELECT COUNT(*) FROM (`+latestAISuggestionFeedbackSQL()+`) latest_feedback WHERE reason_code <> ''`); err != nil {
		return nil, err
	}
	if stats.TaskProfiles, err = r.scalarCount(ctx, `SELECT COUNT(*) FROM task_experience_profiles`); err != nil {
		return nil, err
	}
	if stats.AssetQualityLabels, err = r.scalarCount(ctx, `SELECT COUNT(*) FROM asset_quality_labels`); err != nil {
		return nil, err
	}
	stats.ReusableSamples = stats.TaskProfiles + stats.AssetQualityLabels
	businessLocatable, err := r.scalarCount(ctx, `SELECT COUNT(*) FROM experience_events WHERE task_id IS NOT NULL OR source_id <> ''`)
	if err != nil {
		return nil, err
	}
	aiLocatable, err := r.scalarCount(ctx, `SELECT COUNT(*) FROM ai_suggestion_events WHERE target_type <> '' AND target_id <> ''`)
	if err != nil {
		return nil, err
	}
	stats.LocatableSamples = businessLocatable + aiLocatable
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
	if stats.FeedbackSamples > 0 {
		stats.ReasonCoverageRate = float64(stats.ReasonedFeedbackSamples) / float64(stats.FeedbackSamples)
	}
	if stats.SampleTotal > 0 {
		stats.ReusableRate = float64(stats.ReusableSamples) / float64(stats.SampleTotal)
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
	var feedbackValue, feedbackReasonCode sql.NullString
	var feedbackCreatedAt sql.NullTime
	var evidenceRank int
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
		&feedbackValue,
		&feedbackReasonCode,
		&feedbackCreatedAt,
		&evidenceRank,
		&event.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan experience event: %w", err)
	}
	event.TaskID = fromNullInt64(taskID)
	event.ActorSnapshot = rawJSONFromNull(actorSnapshot)
	event.BusinessSnapshot = rawJSONFromNull(businessSnapshot)
	event.Payload = rawJSONFromNull(payload)
	if feedbackValue.Valid {
		event.FeedbackValue = feedbackValue.String
	}
	if feedbackReasonCode.Valid {
		event.FeedbackReasonCode = feedbackReasonCode.String
	}
	event.FeedbackCreatedAt = fromNullTime(feedbackCreatedAt)
	decorateExperienceEvent(&event, evidenceRank)
	return &event, nil
}

func decorateExperienceEvent(event *domain.ExperienceEvent, evidenceRank int) {
	if event == nil {
		return
	}
	event.EvidenceLevel = experienceEvidenceLevel(evidenceRank)
	missing := make([]string, 0, 4)
	targetType, targetID := experienceEventTarget(event)
	if targetType == "" || targetID == "" {
		missing = append(missing, "target")
	}
	if event.SourceType == "ai_suggestion" {
		if strings.TrimSpace(event.FeedbackValue) == "" {
			missing = append(missing, "feedback")
		}
		if requiresExperienceReason(event.FeedbackValue) && strings.TrimSpace(event.FeedbackReasonCode) == "" {
			missing = append(missing, "reason")
		}
		switch targetType {
		case "task":
			if evidenceRank < 4 {
				missing = append(missing, "profile")
			}
		case "asset":
			if evidenceRank < 4 {
				missing = append(missing, "asset_quality")
			}
		}
	} else {
		if !experiencePayloadHasReason(event.Payload) {
			missing = append(missing, "reason")
		}
		if evidenceRank < 4 {
			missing = append(missing, "profile")
		}
	}
	event.MissingSignals = missing
}

func experienceEvidenceLevel(rank int) string {
	switch {
	case rank >= 4:
		return domain.ExperienceEvidenceReusable
	case rank == 3:
		return domain.ExperienceEvidenceTagged
	case rank == 2:
		return domain.ExperienceEvidenceFeedback
	case rank == 1:
		return domain.ExperienceEvidenceLocatable
	default:
		return domain.ExperienceEvidenceDisplayed
	}
}

func requiresExperienceReason(feedbackValue string) bool {
	switch strings.TrimSpace(feedbackValue) {
	case domain.ExperienceFeedbackRejected, domain.ExperienceFeedbackPartiallyAccepted:
		return true
	default:
		return false
	}
}

func experienceEventTarget(event *domain.ExperienceEvent) (string, string) {
	if event == nil {
		return "", ""
	}
	var business map[string]interface{}
	if len(event.BusinessSnapshot) > 0 && json.Unmarshal(event.BusinessSnapshot, &business) == nil {
		targetType, _ := business["target_type"].(string)
		targetID, _ := business["target_id"].(string)
		if strings.TrimSpace(targetType) != "" || strings.TrimSpace(targetID) != "" {
			return strings.TrimSpace(targetType), strings.TrimSpace(targetID)
		}
	}
	if event.TaskID != nil {
		return "task", strconv.FormatInt(*event.TaskID, 10)
	}
	if sourceID := strings.TrimSpace(event.SourceID); sourceID != "" {
		return event.SourceType, sourceID
	}
	return "", ""
}

func experiencePayloadHasReason(payload json.RawMessage) bool {
	if len(payload) == 0 {
		return false
	}
	var value map[string]interface{}
	if err := json.Unmarshal(payload, &value); err != nil {
		return false
	}
	for _, key := range []string{"reason_code", "reason_codes", "reason_tags"} {
		if raw, ok := value[key]; ok && raw != nil {
			return true
		}
	}
	return false
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
