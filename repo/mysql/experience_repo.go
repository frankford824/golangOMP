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

func (r *experienceRepo) ListClientReasonTags(ctx context.Context, scene string, allowedScenes []string) ([]*domain.ExperienceClientReasonTag, error) {
	allowed := make([]string, 0, len(allowedScenes))
	seen := make(map[string]struct{}, len(allowedScenes))
	for _, raw := range allowedScenes {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		allowed = append(allowed, value)
	}
	if len(allowed) == 0 {
		return []*domain.ExperienceClientReasonTag{}, nil
	}

	where := []string{"enabled = 1", "deleted_at IS NULL"}
	args := make([]interface{}, 0, len(allowed)+1)
	requestedScene := strings.TrimSpace(scene)
	if requestedScene != "" {
		if _, ok := seen[requestedScene]; !ok {
			return []*domain.ExperienceClientReasonTag{}, nil
		}
		where = append(where, "scene = ?")
		args = append(args, requestedScene)
	} else {
		placeholders := make([]string, 0, len(allowed))
		for _, value := range allowed {
			placeholders = append(placeholders, "?")
			args = append(args, value)
		}
		where = append(where, "scene IN ("+strings.Join(placeholders, ", ")+")")
	}

	rows, err := r.db.db.QueryContext(ctx, `
		SELECT scene, code, name, tag_group, sort_order
		FROM experience_reason_tags
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY scene ASC, sort_order ASC, id ASC`, args...)
	if err != nil {
		return nil, fmt.Errorf("list client experience reason tags: %w", err)
	}
	defer rows.Close()

	tags := make([]*domain.ExperienceClientReasonTag, 0)
	for rows.Next() {
		var tag domain.ExperienceClientReasonTag
		if err := rows.Scan(&tag.Scene, &tag.Code, &tag.Name, &tag.Group, &tag.SortOrder); err != nil {
			return nil, fmt.Errorf("scan client experience reason tag: %w", err)
		}
		tags = append(tags, &tag)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate client experience reason tags: %w", err)
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
		SELECT id, event_key, schema_version, event_time, source_type, source_id, task_id,
		       target_type, target_id, source_watermark, observed_from, observed_id, action, outcome,
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
		SELECT e.id, e.event_key, e.schema_version, e.event_time, e.source_type, e.source_id, e.task_id,
		       e.target_type, e.target_id, e.source_watermark, e.observed_from, e.observed_id, e.action, e.outcome,
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
		       a.target_type AS target_type,
		       a.target_id AS target_id,
		       a.suggestion_stable_key AS source_watermark,
		       'ai_suggestion_events' AS observed_from,
		       a.suggestion_event_id AS observed_id,
		       a.suggestion_type AS action,
		       'displayed' AS outcome,
		       CASE
		         WHEN a.actor_id IS NULL THEN NULL
		         ELSE JSON_OBJECT('actor_id', a.actor_id, 'actor_type', 'user', 'surface', 'ai_suggestion')
		       END AS actor_snapshot_json,
		       JSON_OBJECT('target_type', a.target_type, 'target_id', a.target_id) AS business_snapshot_json,
		       JSON_OBJECT(
		         'suggestion_id', a.suggestion_id,
		         'suggestion_stable_key', a.suggestion_stable_key,
		         'attribution_eligible', CAST(a.attribution_eligible AS UNSIGNED),
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
	businessLocatable, err := r.scalarCount(ctx, `SELECT COUNT(*) FROM experience_events WHERE task_id IS NOT NULL OR source_id <> '' OR (target_type <> '' AND target_id <> '')`)
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
	if runs, err := r.ListRecentExperienceWorkerRuns(ctx, 12); err != nil {
		return nil, err
	} else {
		stats.WorkerLastRuns = runs
	}
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
			event_key, schema_version, source_type, source_id, task_id, target_type, target_id,
			source_watermark, observed_from, observed_id, action, outcome, event_time,
			actor_snapshot_json, business_snapshot_json, payload_json, data_classification, ground_truth_status, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE updated_at = updated_at`,
		event.EventKey,
		event.SchemaVersion,
		event.SourceType,
		event.SourceID,
		toNullInt64(event.TaskID),
		event.TargetType,
		event.TargetID,
		event.SourceWatermark,
		event.ObservedFrom,
		event.ObservedID,
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
		SELECT id, event_key, schema_version, source_type, source_id, task_id,
		       target_type, target_id, source_watermark, observed_from, observed_id, action, outcome, event_time,
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
			event_key, schema_version, event_time, source_type, source_id, task_id, target_type, target_id,
			source_watermark, observed_from, observed_id, action, outcome,
			actor_snapshot_json, business_snapshot_json, payload_json, data_classification, ground_truth_status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		outbox.EventKey,
		outbox.SchemaVersion,
		outbox.EventTime,
		outbox.SourceType,
		outbox.SourceID,
		toNullInt64(outbox.TaskID),
		outbox.TargetType,
		outbox.TargetID,
		outbox.SourceWatermark,
		outbox.ObservedFrom,
		outbox.ObservedID,
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
			suggestion_event_id, suggestion_stable_key, attribution_eligible,
			suggestion_type, suggestion_id, source, confidence, model, provider, model_version,
			input_summary_json, suggestion_json, target_type, target_id, actor_id, displayed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.SuggestionEventID,
		event.SuggestionStableKey,
		event.AttributionEligible,
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

func (r *experienceRepo) GetAISuggestionEventByEventID(ctx context.Context, suggestionEventID string) (*domain.AISuggestionEvent, error) {
	var event domain.AISuggestionEvent
	var confidence sql.NullFloat64
	var inputSummary, suggestion sql.NullString
	var actorID sql.NullInt64
	err := r.db.db.QueryRowContext(ctx, `
		SELECT id, suggestion_event_id, suggestion_stable_key, attribution_eligible,
		       suggestion_type, suggestion_id, source, confidence, model, provider, model_version,
		       input_summary_json, suggestion_json, target_type, target_id, actor_id, displayed_at, created_at
		FROM ai_suggestion_events
		WHERE suggestion_event_id = ?`,
		strings.TrimSpace(suggestionEventID),
	).Scan(
		&event.ID,
		&event.SuggestionEventID,
		&event.SuggestionStableKey,
		&event.AttributionEligible,
		&event.SuggestionType,
		&event.SuggestionID,
		&event.Source,
		&confidence,
		&event.Model,
		&event.Provider,
		&event.ModelVersion,
		&inputSummary,
		&suggestion,
		&event.TargetType,
		&event.TargetID,
		&actorID,
		&event.DisplayedAt,
		&event.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get ai suggestion event by event id: %w", err)
	}
	event.Confidence = fromNullFloat64(confidence)
	event.InputSummary = rawJSONFromNull(inputSummary)
	event.Suggestion = rawJSONFromNull(suggestion)
	event.ActorID = fromNullInt64(actorID)
	return &event, nil
}

func (r *experienceRepo) CreateExperienceBehaviorEvents(ctx context.Context, events []*domain.ExperienceBehaviorEvent) (int, error) {
	if len(events) == 0 {
		return 0, nil
	}
	tx, err := r.db.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin experience behavior batch: %w", err)
	}
	stmt, err := tx.PrepareContext(ctx, `
		INSERT IGNORE INTO experience_behavior_events (
			event_key, client_event_id, page_instance_id, actor_id, surface, action, target_type, target_id, task_id,
			suggestion_event_id, suggestion_stable_key, occurred_at, received_at, route_name, component, dwell_ms,
			payload_json, data_classification
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		_ = tx.Rollback()
		return 0, fmt.Errorf("prepare experience behavior batch: %w", err)
	}
	defer stmt.Close()

	inserted := 0
	for _, event := range events {
		if event == nil {
			continue
		}
		result, err := stmt.ExecContext(ctx,
			event.EventKey,
			event.ClientEventID,
			event.PageInstanceID,
			toNullInt64(event.ActorID),
			event.Surface,
			event.Action,
			event.TargetType,
			event.TargetID,
			toNullInt64(event.TaskID),
			event.SuggestionEventID,
			event.SuggestionStableKey,
			event.OccurredAt,
			event.ReceivedAt,
			event.RouteName,
			event.Component,
			event.DwellMS,
			toNullJSONString(event.Payload),
			event.DataClassification,
		)
		if err != nil {
			_ = tx.Rollback()
			return 0, fmt.Errorf("insert experience behavior event: %w", err)
		}
		if rows, _ := result.RowsAffected(); rows > 0 {
			inserted += int(rows)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit experience behavior batch: %w", err)
	}
	return inserted, nil
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

func (r *experienceRepo) GetExperienceWorkerWatermark(ctx context.Context, workerName, sourceName string) (*domain.ExperienceWorkerWatermark, error) {
	var watermark domain.ExperienceWorkerWatermark
	var lastSeenAt sql.NullTime
	var metadata sql.NullString
	err := r.db.db.QueryRowContext(ctx, `
		SELECT worker_name, source_name, last_seen_at, last_seen_id, source_watermark, status, metadata_json, created_at, updated_at
		FROM experience_worker_watermarks
		WHERE worker_name = ? AND source_name = ?`,
		strings.TrimSpace(workerName),
		strings.TrimSpace(sourceName),
	).Scan(
		&watermark.WorkerName,
		&watermark.SourceName,
		&lastSeenAt,
		&watermark.LastSeenID,
		&watermark.SourceWatermark,
		&watermark.Status,
		&metadata,
		&watermark.CreatedAt,
		&watermark.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get experience worker watermark: %w", err)
	}
	watermark.LastSeenAt = fromNullTime(lastSeenAt)
	watermark.Metadata = rawJSONFromNull(metadata)
	return &watermark, nil
}

func (r *experienceRepo) SaveExperienceWorkerWatermark(ctx context.Context, watermark *domain.ExperienceWorkerWatermark) error {
	if watermark == nil {
		return fmt.Errorf("experience worker watermark is nil")
	}
	status := strings.TrimSpace(watermark.Status)
	if status == "" {
		status = "active"
	}
	_, err := r.db.db.ExecContext(ctx, `
		INSERT INTO experience_worker_watermarks (
			worker_name, source_name, last_seen_at, last_seen_id, source_watermark, status, metadata_json
		) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			last_seen_at = VALUES(last_seen_at),
			last_seen_id = VALUES(last_seen_id),
			source_watermark = VALUES(source_watermark),
			status = VALUES(status),
			metadata_json = VALUES(metadata_json),
			updated_at = CURRENT_TIMESTAMP`,
		strings.TrimSpace(watermark.WorkerName),
		strings.TrimSpace(watermark.SourceName),
		toNullTime(watermark.LastSeenAt),
		watermark.LastSeenID,
		strings.TrimSpace(watermark.SourceWatermark),
		status,
		toNullJSONString(watermark.Metadata),
	)
	if err != nil {
		return fmt.Errorf("save experience worker watermark: %w", err)
	}
	return nil
}

func (r *experienceRepo) ListExperienceAuditOutcomeRows(ctx context.Context, cursor repo.ExperienceSourceCursor, limit int) ([]*domain.ExperienceOutcomeEventRow, error) {
	if limit <= 0 {
		limit = 50
	}
	lastSeenAt := experienceCursorTime(cursor.LastSeenAt)
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT ar.id, ar.task_id, COALESCE(t.task_no, ''), ar.stage, ar.action, ar.auditor_id,
		       ar.issue_types_json, ar.affects_launch, ar.need_outsource, ar.created_at
		FROM audit_records ar
		LEFT JOIN tasks t ON t.id = ar.task_id
		WHERE (ar.created_at > ?) OR (ar.created_at = ? AND ar.id > ?)
		ORDER BY ar.created_at ASC, ar.id ASC
		LIMIT ?`,
		lastSeenAt,
		lastSeenAt,
		cursor.LastSeenID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list experience audit outcome rows: %w", err)
	}
	defer rows.Close()

	out := make([]*domain.ExperienceOutcomeEventRow, 0)
	for rows.Next() {
		var id, taskID, auditorID int64
		var taskNo, stage, action string
		var issueTypes sql.NullString
		var affectsLaunch, needOutsource bool
		var createdAt time.Time
		if err := rows.Scan(&id, &taskID, &taskNo, &stage, &action, &auditorID, &issueTypes, &affectsLaunch, &needOutsource, &createdAt); err != nil {
			return nil, fmt.Errorf("scan experience audit outcome row: %w", err)
		}
		taskIDCopy := taskID
		actorSnapshot := mustExperienceJSON(map[string]interface{}{
			"actor_type": "user",
			"actor_id":   auditorID,
			"source":     "audit_records",
		})
		businessSnapshot := mustExperienceJSON(map[string]interface{}{
			"target_type": "task",
			"target_id":   strconv.FormatInt(taskID, 10),
			"task_no":     taskNo,
			"stage":       stage,
		})
		payload := mustExperienceJSON(map[string]interface{}{
			"stage":          stage,
			"action":         action,
			"issue_types":    rawJSONOrNil(issueTypes),
			"affects_launch": affectsLaunch,
			"need_outsource": needOutsource,
			"observer":       "append_only",
		})
		out = append(out, &domain.ExperienceOutcomeEventRow{
			ID:               id,
			EventKey:         fmt.Sprintf("outcome:audit_records:%d", id),
			SourceName:       "audit_records",
			SourceID:         fmt.Sprintf("audit_record:%d", id),
			TaskID:           &taskIDCopy,
			TargetType:       "task",
			TargetID:         strconv.FormatInt(taskID, 10),
			Action:           auditOutcomeAction(action),
			Outcome:          strings.TrimSpace(action),
			EventTime:        createdAt.UTC(),
			ActorSnapshot:    actorSnapshot,
			BusinessSnapshot: businessSnapshot,
			Payload:          payload,
			SourceWatermark:  experienceSourceWatermark(createdAt, id),
			ObservedFrom:     "audit_records",
			ObservedID:       strconv.FormatInt(id, 10),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate experience audit outcome rows: %w", err)
	}
	return out, nil
}

func (r *experienceRepo) ListExperienceModuleOutcomeRows(ctx context.Context, cursor repo.ExperienceSourceCursor, limit int) ([]*domain.ExperienceOutcomeEventRow, error) {
	if limit <= 0 {
		limit = 50
	}
	lastSeenAt := experienceCursorTime(cursor.LastSeenAt)
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT e.id, e.task_module_id, tm.task_id, tm.module_key, e.event_type,
		       COALESCE(e.from_state, ''), COALESCE(e.to_state, ''), e.actor_id, e.actor_snapshot, e.payload, e.created_at
		FROM task_module_events e
		INNER JOIN task_modules tm ON tm.id = e.task_module_id
		WHERE (e.created_at > ?) OR (e.created_at = ? AND e.id > ?)
		ORDER BY e.created_at ASC, e.id ASC
		LIMIT ?`,
		lastSeenAt,
		lastSeenAt,
		cursor.LastSeenID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list experience module outcome rows: %w", err)
	}
	defer rows.Close()

	out := make([]*domain.ExperienceOutcomeEventRow, 0)
	for rows.Next() {
		var id, taskModuleID, taskID int64
		var moduleKey, eventType, fromState, toState string
		var actorID sql.NullInt64
		var actorSnapshot, payload sql.NullString
		var createdAt time.Time
		if err := rows.Scan(&id, &taskModuleID, &taskID, &moduleKey, &eventType, &fromState, &toState, &actorID, &actorSnapshot, &payload, &createdAt); err != nil {
			return nil, fmt.Errorf("scan experience module outcome row: %w", err)
		}
		taskIDCopy := taskID
		action := "module_event"
		if strings.TrimSpace(fromState) != "" || strings.TrimSpace(toState) != "" {
			action = "module_state_changed"
		}
		outcome := strings.TrimSpace(toState)
		if outcome == "" {
			outcome = strings.TrimSpace(eventType)
		}
		businessSnapshot := mustExperienceJSON(map[string]interface{}{
			"target_type":    "task",
			"target_id":      strconv.FormatInt(taskID, 10),
			"task_module_id": taskModuleID,
			"module_key":     moduleKey,
		})
		eventPayload := mustExperienceJSON(map[string]interface{}{
			"task_module_id": taskModuleID,
			"module_key":     moduleKey,
			"event_type":     eventType,
			"from_state":     fromState,
			"to_state":       toState,
			"changed_fields": []map[string]interface{}{
				{"field": "module_state", "from": fromState, "to": toState},
			},
			"source_payload": rawJSONOrNil(payload),
			"observer":       "append_only",
		})
		out = append(out, &domain.ExperienceOutcomeEventRow{
			ID:               id,
			EventKey:         fmt.Sprintf("outcome:task_module_events:%d", id),
			SourceName:       "task_module_events",
			SourceID:         fmt.Sprintf("task_module_event:%d", id),
			TaskID:           &taskIDCopy,
			TargetType:       "task",
			TargetID:         strconv.FormatInt(taskID, 10),
			Action:           action,
			Outcome:          outcome,
			EventTime:        createdAt.UTC(),
			ActorSnapshot:    mergeModuleActorSnapshot(actorID, actorSnapshot),
			BusinessSnapshot: businessSnapshot,
			Payload:          eventPayload,
			SourceWatermark:  experienceSourceWatermark(createdAt, id),
			ObservedFrom:     "task_module_events",
			ObservedID:       strconv.FormatInt(id, 10),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate experience module outcome rows: %w", err)
	}
	return out, nil
}

func (r *experienceRepo) ListExperienceTaskStatusSnapshots(ctx context.Context, cursor repo.ExperienceSourceCursor, limit int) ([]*domain.ExperienceOutcomeSnapshotRow, error) {
	if limit <= 0 {
		limit = 50
	}
	lastSeenAt := experienceCursorTime(cursor.LastSeenAt)
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT id, task_status, updated_at
		FROM tasks
		WHERE (updated_at > ?) OR (updated_at = ? AND id > ?)
		ORDER BY updated_at ASC, id ASC
		LIMIT ?`,
		lastSeenAt,
		lastSeenAt,
		cursor.LastSeenID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list experience task status snapshots: %w", err)
	}
	defer rows.Close()

	out := make([]*domain.ExperienceOutcomeSnapshotRow, 0)
	for rows.Next() {
		var id int64
		var status string
		var updatedAt time.Time
		if err := rows.Scan(&id, &status, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan experience task status snapshot: %w", err)
		}
		taskID := id
		out = append(out, &domain.ExperienceOutcomeSnapshotRow{
			SourceName:      "tasks_status_snapshot",
			EntityType:      "task",
			EntityID:        strconv.FormatInt(id, 10),
			TaskID:          &taskID,
			TargetType:      "task",
			TargetID:        strconv.FormatInt(id, 10),
			SourceUpdatedAt: updatedAt.UTC(),
			ObservedValue:   mustExperienceJSON(map[string]interface{}{"task_status": status}),
			TerminalState:   taskTerminalState(status),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate experience task status snapshots: %w", err)
	}
	return out, nil
}

func (r *experienceRepo) ListExperienceTaskAssetReviewSnapshots(ctx context.Context, cursor repo.ExperienceSourceCursor, limit int) ([]*domain.ExperienceOutcomeSnapshotRow, error) {
	if limit <= 0 {
		limit = 50
	}
	lastSeenAt := experienceCursorTime(cursor.LastSeenAt)
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT id, task_id, COALESCE(flow_review_status, ''), approved_at, rejected_at,
		       COALESCE(is_archived, 0), archived_at,
		       GREATEST(
		         created_at,
		         COALESCE(approved_at, created_at),
		         COALESCE(rejected_at, created_at),
		         COALESCE(archived_at, created_at),
		         COALESCE(cleaned_at, created_at)
		       ) AS source_updated_at
		FROM task_assets
		WHERE (created_at > ?) OR (created_at = ? AND id > ?)
		   OR (approved_at > ?) OR (approved_at = ? AND id > ?)
		   OR (rejected_at > ?) OR (rejected_at = ? AND id > ?)
		   OR (archived_at > ?) OR (archived_at = ? AND id > ?)
		   OR (cleaned_at > ?) OR (cleaned_at = ? AND id > ?)
		ORDER BY source_updated_at ASC, id ASC
		LIMIT ?`,
		lastSeenAt, lastSeenAt, cursor.LastSeenID,
		lastSeenAt, lastSeenAt, cursor.LastSeenID,
		lastSeenAt, lastSeenAt, cursor.LastSeenID,
		lastSeenAt, lastSeenAt, cursor.LastSeenID,
		lastSeenAt, lastSeenAt, cursor.LastSeenID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list experience task asset review snapshots: %w", err)
	}
	defer rows.Close()

	out := make([]*domain.ExperienceOutcomeSnapshotRow, 0)
	for rows.Next() {
		var id, taskID int64
		var status string
		var approvedAt, rejectedAt, archivedAt sql.NullTime
		var archived bool
		var sourceUpdatedAt time.Time
		if err := rows.Scan(&id, &taskID, &status, &approvedAt, &rejectedAt, &archived, &archivedAt, &sourceUpdatedAt); err != nil {
			return nil, fmt.Errorf("scan experience task asset review snapshot: %w", err)
		}
		taskIDCopy := taskID
		out = append(out, &domain.ExperienceOutcomeSnapshotRow{
			SourceName:      "task_assets_review_snapshot",
			EntityType:      "task_asset",
			EntityID:        strconv.FormatInt(id, 10),
			TaskID:          &taskIDCopy,
			TargetType:      "task_asset",
			TargetID:        strconv.FormatInt(id, 10),
			SourceUpdatedAt: sourceUpdatedAt.UTC(),
			ObservedValue: mustExperienceJSON(map[string]interface{}{
				"flow_review_status": status,
				"approved_at":        experienceNullableTimeValue(approvedAt),
				"rejected_at":        experienceNullableTimeValue(rejectedAt),
			}),
			TerminalState: taskAssetReviewTerminalState(status, archived, archivedAt),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate experience task asset review snapshots: %w", err)
	}
	return out, nil
}

func (r *experienceRepo) ListExperienceTaskDetailFilingSnapshots(ctx context.Context, cursor repo.ExperienceSourceCursor, limit int) ([]*domain.ExperienceOutcomeSnapshotRow, error) {
	if limit <= 0 {
		limit = 50
	}
	lastSeenAt := experienceCursorTime(cursor.LastSeenAt)
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT id, task_id, COALESCE(filing_status, ''), COALESCE(erp_sync_required, 0), last_filed_at, updated_at
		FROM task_details
		WHERE (updated_at > ?) OR (updated_at = ? AND id > ?)
		ORDER BY updated_at ASC, id ASC
		LIMIT ?`,
		lastSeenAt,
		lastSeenAt,
		cursor.LastSeenID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list experience task detail filing snapshots: %w", err)
	}
	defer rows.Close()

	out := make([]*domain.ExperienceOutcomeSnapshotRow, 0)
	for rows.Next() {
		var id, taskID int64
		var status string
		var syncRequired bool
		var lastFiledAt sql.NullTime
		var updatedAt time.Time
		if err := rows.Scan(&id, &taskID, &status, &syncRequired, &lastFiledAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan experience task detail filing snapshot: %w", err)
		}
		taskIDCopy := taskID
		out = append(out, &domain.ExperienceOutcomeSnapshotRow{
			SourceName:      "task_details_filing_snapshot",
			EntityType:      "task_detail",
			EntityID:        strconv.FormatInt(id, 10),
			TaskID:          &taskIDCopy,
			TargetType:      "task",
			TargetID:        strconv.FormatInt(taskID, 10),
			SourceUpdatedAt: updatedAt.UTC(),
			ObservedValue: mustExperienceJSON(map[string]interface{}{
				"filing_status":     status,
				"erp_sync_required": syncRequired,
				"last_filed_at":     experienceNullableTimeValue(lastFiledAt),
			}),
			TerminalState: erpFilingTerminalState(status, syncRequired),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate experience task detail filing snapshots: %w", err)
	}
	return out, nil
}

func (r *experienceRepo) ListExperienceTaskSKUItemFilingSnapshots(ctx context.Context, cursor repo.ExperienceSourceCursor, limit int) ([]*domain.ExperienceOutcomeSnapshotRow, error) {
	if limit <= 0 {
		limit = 50
	}
	lastSeenAt := experienceCursorTime(cursor.LastSeenAt)
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT id, task_id, COALESCE(filing_status, ''), COALESCE(erp_sync_status, ''),
		       COALESCE(erp_sync_required, 0), last_filed_at, updated_at
		FROM task_sku_items
		WHERE (updated_at > ?) OR (updated_at = ? AND id > ?)
		ORDER BY updated_at ASC, id ASC
		LIMIT ?`,
		lastSeenAt,
		lastSeenAt,
		cursor.LastSeenID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list experience task sku item filing snapshots: %w", err)
	}
	defer rows.Close()

	out := make([]*domain.ExperienceOutcomeSnapshotRow, 0)
	for rows.Next() {
		var id, taskID int64
		var filingStatus, syncStatus string
		var syncRequired bool
		var lastFiledAt sql.NullTime
		var updatedAt time.Time
		if err := rows.Scan(&id, &taskID, &filingStatus, &syncStatus, &syncRequired, &lastFiledAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan experience task sku item filing snapshot: %w", err)
		}
		taskIDCopy := taskID
		out = append(out, &domain.ExperienceOutcomeSnapshotRow{
			SourceName:      "task_sku_items_filing_snapshot",
			EntityType:      "task_sku_item",
			EntityID:        strconv.FormatInt(id, 10),
			TaskID:          &taskIDCopy,
			TargetType:      "task_sku_item",
			TargetID:        strconv.FormatInt(id, 10),
			SourceUpdatedAt: updatedAt.UTC(),
			ObservedValue: mustExperienceJSON(map[string]interface{}{
				"filing_status":     filingStatus,
				"erp_sync_status":   syncStatus,
				"erp_sync_required": syncRequired,
				"last_filed_at":     experienceNullableTimeValue(lastFiledAt),
			}),
			TerminalState: erpFilingTerminalState(filingStatus, syncRequired),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate experience task sku item filing snapshots: %w", err)
	}
	return out, nil
}

func (r *experienceRepo) GetExperienceObservedEntityState(ctx context.Context, sourceName, entityType, entityID string) (*domain.ExperienceObservedEntityState, error) {
	var state domain.ExperienceObservedEntityState
	var observedValue, tombstonePayload sql.NullString
	var terminalObservedAt, sourceUpdatedAt sql.NullTime
	err := r.db.db.QueryRowContext(ctx, `
		SELECT id, source_name, entity_type, entity_id, observed_value_json, observed_hash,
		       terminal_state, terminal_observed_at, source_updated_at, last_seen_at, tombstoned,
		       tombstone_payload_json, created_at, updated_at
		FROM experience_observed_entity_states
		WHERE source_name = ? AND entity_type = ? AND entity_id = ?`,
		strings.TrimSpace(sourceName),
		strings.TrimSpace(entityType),
		strings.TrimSpace(entityID),
	).Scan(
		&state.ID,
		&state.SourceName,
		&state.EntityType,
		&state.EntityID,
		&observedValue,
		&state.ObservedHash,
		&state.TerminalState,
		&terminalObservedAt,
		&sourceUpdatedAt,
		&state.LastSeenAt,
		&state.Tombstoned,
		&tombstonePayload,
		&state.CreatedAt,
		&state.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get experience observed entity state: %w", err)
	}
	state.ObservedValue = rawJSONFromNull(observedValue)
	state.TerminalObservedAt = fromNullTime(terminalObservedAt)
	state.SourceUpdatedAt = fromNullTime(sourceUpdatedAt)
	state.TombstonePayload = rawJSONFromNull(tombstonePayload)
	return &state, nil
}

func (r *experienceRepo) UpsertExperienceObservedEntityState(ctx context.Context, state *domain.ExperienceObservedEntityState) error {
	if state == nil {
		return fmt.Errorf("experience observed entity state is nil")
	}
	_, err := r.db.db.ExecContext(ctx, `
		INSERT INTO experience_observed_entity_states (
			source_name, entity_type, entity_id, observed_value_json, observed_hash,
			terminal_state, terminal_observed_at, source_updated_at, last_seen_at, tombstoned, tombstone_payload_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			observed_value_json = VALUES(observed_value_json),
			observed_hash = VALUES(observed_hash),
			terminal_state = VALUES(terminal_state),
			terminal_observed_at = VALUES(terminal_observed_at),
			source_updated_at = VALUES(source_updated_at),
			last_seen_at = VALUES(last_seen_at),
			tombstoned = VALUES(tombstoned),
			tombstone_payload_json = VALUES(tombstone_payload_json),
			updated_at = CURRENT_TIMESTAMP`,
		strings.TrimSpace(state.SourceName),
		strings.TrimSpace(state.EntityType),
		strings.TrimSpace(state.EntityID),
		toNullJSONString(state.ObservedValue),
		strings.TrimSpace(state.ObservedHash),
		strings.TrimSpace(state.TerminalState),
		toNullTime(state.TerminalObservedAt),
		toNullTime(state.SourceUpdatedAt),
		state.LastSeenAt,
		state.Tombstoned,
		toNullJSONString(state.TombstonePayload),
	)
	if err != nil {
		return fmt.Errorf("upsert experience observed entity state: %w", err)
	}
	return nil
}

func (r *experienceRepo) RunExperienceRetention(ctx context.Context, policy repo.ExperienceRetentionPolicy) (*domain.ExperienceRetentionRun, error) {
	limit := policy.Limit
	if limit <= 0 {
		limit = 1000
	}
	result := &domain.ExperienceRetentionRun{}
	behaviorDeleted, err := r.execRowsAffected(ctx, `
		DELETE FROM experience_behavior_events
		WHERE occurred_at < ?
		LIMIT ?`, policy.BehaviorBefore, limit)
	if err != nil {
		return nil, fmt.Errorf("delete expired experience behavior events: %w", err)
	}
	result.BehaviorDeleted = behaviorDeleted

	minuteDeleted, err := r.execRowsAffected(ctx, `
		DELETE FROM experience_rate_limits
		WHERE bucket_name LIKE '%minute%' AND period_end < ?
		LIMIT ?`, policy.MinuteRateLimitBefore, limit)
	if err != nil {
		return nil, fmt.Errorf("delete expired minute experience rate limits: %w", err)
	}
	dailyDeleted, err := r.execRowsAffected(ctx, `
		DELETE FROM experience_rate_limits
		WHERE bucket_name NOT LIKE '%minute%' AND period_end < ?
		LIMIT ?`, policy.DailyRateLimitBefore, limit)
	if err != nil {
		return nil, fmt.Errorf("delete expired daily experience rate limits: %w", err)
	}
	result.RateLimitDeleted = minuteDeleted + dailyDeleted

	workerRunDeleted, err := r.execRowsAffected(ctx, `
		DELETE FROM experience_worker_runs
		WHERE created_at < ?
		LIMIT ?`, policy.WorkerRunBefore, limit)
	if err != nil {
		return nil, fmt.Errorf("delete expired experience worker runs: %w", err)
	}
	result.WorkerRunDeleted = workerRunDeleted

	tombstoned, err := r.execRowsAffected(ctx, `
		UPDATE experience_observed_entity_states
		SET tombstoned = 1,
		    tombstone_payload_json = JSON_OBJECT(
		      'source_name', source_name,
		      'entity_type', entity_type,
		      'entity_id', entity_id,
		      'terminal_state', terminal_state,
		      'terminal_observed_at', terminal_observed_at
		    ),
		    updated_at = CURRENT_TIMESTAMP
		WHERE tombstoned = 0
		  AND terminal_state <> ''
		  AND terminal_observed_at IS NOT NULL
		  AND terminal_observed_at < ?
		LIMIT ?`, policy.ObservedTerminalBefore, limit)
	if err != nil {
		return nil, fmt.Errorf("tombstone terminal experience observed states: %w", err)
	}
	result.ObservedTombstoned = tombstoned
	return result, nil
}

func (r *experienceRepo) CreateExperienceWorkerRun(ctx context.Context, run *domain.ExperienceWorkerRunRecord) error {
	if run == nil {
		return fmt.Errorf("experience worker run is nil")
	}
	_, err := r.db.db.ExecContext(ctx, `
		INSERT INTO experience_worker_runs (
			worker_name, source_name, started_at, finished_at, status,
			scanned_count, enqueued_count, skipped_count, failed_count, last_error, metadata_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		strings.TrimSpace(run.WorkerName),
		strings.TrimSpace(run.SourceName),
		run.StartedAt.UTC(),
		toNullTime(run.FinishedAt),
		strings.TrimSpace(run.Status),
		run.ScannedCount,
		run.EnqueuedCount,
		run.SkippedCount,
		run.FailedCount,
		trimExperienceWorkerError(run.LastError),
		toNullJSONString(run.Metadata),
	)
	if err != nil {
		return fmt.Errorf("create experience worker run: %w", err)
	}
	return nil
}

func (r *experienceRepo) ListRecentExperienceWorkerRuns(ctx context.Context, limit int) ([]*domain.ExperienceWorkerRunRecord, error) {
	if limit <= 0 || limit > 50 {
		limit = 12
	}
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT id, worker_name, source_name, started_at, finished_at, status,
		       scanned_count, enqueued_count, skipped_count, failed_count, last_error, metadata_json, created_at
		FROM experience_worker_runs
		ORDER BY started_at DESC, id DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list recent experience worker runs: %w", err)
	}
	defer rows.Close()
	out := make([]*domain.ExperienceWorkerRunRecord, 0)
	for rows.Next() {
		run, err := scanExperienceWorkerRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recent experience worker runs: %w", err)
	}
	return out, nil
}

func (r *experienceRepo) ListExperienceAttributionOutcomes(ctx context.Context, cursor repo.ExperienceSourceCursor, limit int) ([]*domain.ExperienceAttributionOutcome, error) {
	if limit <= 0 {
		limit = 50
	}
	lastSeenAt := experienceCursorTime(cursor.LastSeenAt)
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT id, event_key, event_time, source_type, action, outcome, task_id,
		       target_type, target_id, payload_json
		FROM experience_events
		WHERE ((event_time > ?) OR (event_time = ? AND id > ?))
		  AND (target_type <> '' OR task_id IS NOT NULL)
		ORDER BY event_time ASC, id ASC
		LIMIT ?`,
		lastSeenAt,
		lastSeenAt,
		cursor.LastSeenID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list experience attribution outcomes: %w", err)
	}
	defer rows.Close()
	out := make([]*domain.ExperienceAttributionOutcome, 0)
	for rows.Next() {
		var item domain.ExperienceAttributionOutcome
		var taskID sql.NullInt64
		var targetType, targetID, payload sql.NullString
		if err := rows.Scan(
			&item.ID,
			&item.EventKey,
			&item.EventTime,
			&item.SourceType,
			&item.Action,
			&item.Outcome,
			&taskID,
			&targetType,
			&targetID,
			&payload,
		); err != nil {
			return nil, fmt.Errorf("scan experience attribution outcome: %w", err)
		}
		item.TaskID = fromNullInt64(taskID)
		item.TargetType = stringFromNull(targetType)
		item.TargetID = stringFromNull(targetID)
		item.Payload = rawJSONFromNull(payload)
		out = append(out, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate experience attribution outcomes: %w", err)
	}
	return out, nil
}

func (r *experienceRepo) ListExperienceAttributionCandidates(ctx context.Context, outcome *domain.ExperienceAttributionOutcome, lookback time.Duration, limit int) ([]*domain.ExperienceAttributionCandidate, error) {
	if outcome == nil {
		return []*domain.ExperienceAttributionCandidate{}, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if lookback <= 0 {
		lookback = 7 * 24 * time.Hour
	}
	outcomeAt := outcome.EventTime.UTC()
	from := outcomeAt.Add(-lookback)
	targetType := strings.TrimSpace(outcome.TargetType)
	targetID := strings.TrimSpace(outcome.TargetID)
	taskTargetID := ""
	if outcome.TaskID != nil {
		taskTargetID = strconv.FormatInt(*outcome.TaskID, 10)
	}
	if targetType == "" && taskTargetID == "" {
		return []*domain.ExperienceAttributionCandidate{}, nil
	}

	rows, err := r.db.db.QueryContext(ctx, `
		SELECT a.suggestion_event_id, a.suggestion_stable_key, a.suggestion_type, a.suggestion_id,
		       a.source, a.target_type, a.target_id, a.displayed_at,
		       COUNT(b.id) AS behavior_count,
		       COALESCE(MAX(CASE b.action
		         WHEN 'related_action_done' THEN 5
		         WHEN 'jump' THEN 5
		         WHEN 'click' THEN 4
		         WHEN 'copy' THEN 3
		         WHEN 'expand' THEN 2
		         WHEN 'visible' THEN 1
		         WHEN 'dismiss' THEN -2
		         WHEN 'ignored_after_timeout' THEN -2
		         ELSE 0
		       END), 0) AS behavior_score,
		       MAX(b.occurred_at) AS latest_behavior_at,
		       lf.feedback_value, lf.reason_code, lf.created_at
		FROM ai_suggestion_events a
		LEFT JOIN experience_behavior_events b
		  ON (b.suggestion_event_id = a.suggestion_event_id
		      OR (b.suggestion_event_id = '' AND b.suggestion_stable_key = a.suggestion_stable_key))
		 AND a.actor_id IS NOT NULL
		 AND b.actor_id = a.actor_id
		 AND b.occurred_at >= a.displayed_at
		 AND b.occurred_at <= ?
		LEFT JOIN (`+latestAISuggestionFeedbackSQL()+`) lf ON lf.suggestion_event_id = a.suggestion_event_id
		WHERE a.attribution_eligible = 1
		  AND a.displayed_at >= ?
		  AND a.displayed_at <= ?
		  AND (
		    (a.target_type = ? AND a.target_id = ?)
		    OR (? <> '' AND a.target_type = 'task' AND a.target_id = ?)
		  )
		GROUP BY a.suggestion_event_id, a.suggestion_stable_key, a.suggestion_type, a.suggestion_id,
		         a.source, a.target_type, a.target_id, a.displayed_at,
		         lf.feedback_value, lf.reason_code, lf.created_at
		HAVING behavior_count > 0 OR COALESCE(lf.feedback_value, '') <> ''
		ORDER BY a.displayed_at DESC
		LIMIT ?`,
		outcomeAt,
		from,
		outcomeAt,
		targetType,
		targetID,
		taskTargetID,
		taskTargetID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list experience attribution candidates: %w", err)
	}
	defer rows.Close()
	out := make([]*domain.ExperienceAttributionCandidate, 0)
	for rows.Next() {
		var item domain.ExperienceAttributionCandidate
		var latestBehaviorAt, feedbackCreatedAt sql.NullTime
		var feedbackValue, feedbackReasonCode sql.NullString
		if err := rows.Scan(
			&item.SuggestionEventID,
			&item.SuggestionStableKey,
			&item.SuggestionType,
			&item.SuggestionID,
			&item.Source,
			&item.TargetType,
			&item.TargetID,
			&item.DisplayedAt,
			&item.BehaviorCount,
			&item.BehaviorScore,
			&latestBehaviorAt,
			&feedbackValue,
			&feedbackReasonCode,
			&feedbackCreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan experience attribution candidate: %w", err)
		}
		item.LatestBehaviorAt = fromNullTime(latestBehaviorAt)
		item.FeedbackCreatedAt = fromNullTime(feedbackCreatedAt)
		item.FeedbackValue = stringFromNull(feedbackValue)
		item.FeedbackReasonCode = stringFromNull(feedbackReasonCode)
		out = append(out, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate experience attribution candidates: %w", err)
	}
	return out, nil
}

func (r *experienceRepo) CreateExperienceAttribution(ctx context.Context, attribution *domain.ExperienceAttribution) error {
	if attribution == nil {
		return fmt.Errorf("experience attribution is nil")
	}
	_, err := r.db.db.ExecContext(ctx, `
		INSERT INTO experience_attributions (
			suggestion_event_id, suggestion_stable_key, candidate_event_key, outcome_event_key,
			status, confidence, score, computed_at, evidence_summary_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			status = VALUES(status),
			confidence = VALUES(confidence),
			score = VALUES(score),
			computed_at = VALUES(computed_at),
			evidence_summary_json = VALUES(evidence_summary_json),
			updated_at = CURRENT_TIMESTAMP`,
		strings.TrimSpace(attribution.SuggestionEventID),
		strings.TrimSpace(attribution.SuggestionStableKey),
		strings.TrimSpace(attribution.CandidateEventKey),
		strings.TrimSpace(attribution.OutcomeEventKey),
		strings.TrimSpace(attribution.Status),
		strings.TrimSpace(attribution.Confidence),
		attribution.Score,
		attribution.ComputedAt.UTC(),
		toNullJSONString(attribution.EvidenceSummary),
	)
	if err != nil {
		return fmt.Errorf("create experience attribution: %w", err)
	}
	return nil
}

func (r *experienceRepo) ReserveExperienceRateLimit(ctx context.Context, req repo.ExperienceRateLimitRequest) (*domain.ExperienceRateLimitReservation, error) {
	limitKey := strings.TrimSpace(req.LimitKey)
	bucketName := strings.TrimSpace(req.BucketName)
	if limitKey == "" || bucketName == "" {
		return nil, fmt.Errorf("experience rate limit key and bucket are required")
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 1
	}
	hardCap := req.HardCap
	if hardCap < limit {
		hardCap = limit * 10
	}
	if hardCap < limit {
		hardCap = limit
	}
	_, err := r.db.db.ExecContext(ctx, `
		INSERT INTO experience_rate_limits (
			limit_key, actor_id, bucket_name, period_start, period_end, count, hard_cap
		) VALUES (?, ?, ?, ?, ?, 1, ?)
		ON DUPLICATE KEY UPDATE
			count = LEAST(count + 1, GREATEST(hard_cap, VALUES(hard_cap))),
			hard_cap = GREATEST(hard_cap, VALUES(hard_cap)),
			period_end = VALUES(period_end),
			updated_at = CURRENT_TIMESTAMP`,
		limitKey,
		toNullInt64(req.ActorID),
		bucketName,
		req.PeriodStart.UTC(),
		req.PeriodEnd.UTC(),
		hardCap,
	)
	if err != nil {
		return nil, fmt.Errorf("reserve experience rate limit: %w", err)
	}

	var reservation domain.ExperienceRateLimitReservation
	var actorID sql.NullInt64
	if err := r.db.db.QueryRowContext(ctx, `
		SELECT limit_key, actor_id, bucket_name, period_start, period_end, count, hard_cap
		FROM experience_rate_limits
		WHERE limit_key = ?`, limitKey).Scan(
		&reservation.LimitKey,
		&actorID,
		&reservation.BucketName,
		&reservation.PeriodStart,
		&reservation.PeriodEnd,
		&reservation.Count,
		&reservation.HardCap,
	); err != nil {
		return nil, fmt.Errorf("load experience rate limit reservation: %w", err)
	}
	reservation.ActorID = fromNullInt64(actorID)
	reservation.Limit = limit
	reservation.Allowed = reservation.Count <= limit
	return &reservation, nil
}

func (r *experienceRepo) RefundExperienceRateLimit(ctx context.Context, limitKey string) error {
	key := strings.TrimSpace(limitKey)
	if key == "" {
		return nil
	}
	_, err := r.db.db.ExecContext(ctx, `
		UPDATE experience_rate_limits
		SET count = GREATEST(count - 1, 0),
		    updated_at = CURRENT_TIMESTAMP
		WHERE limit_key = ?`, key)
	if err != nil {
		return fmt.Errorf("refund experience rate limit: %w", err)
	}
	return nil
}

func (r *experienceRepo) GetExperienceRateLimit(ctx context.Context, limitKey string, limit int) (*domain.ExperienceRateLimitReservation, error) {
	var reservation domain.ExperienceRateLimitReservation
	var actorID sql.NullInt64
	err := r.db.db.QueryRowContext(ctx, `
		SELECT limit_key, actor_id, bucket_name, period_start, period_end, count, hard_cap
		FROM experience_rate_limits
		WHERE limit_key = ?`,
		strings.TrimSpace(limitKey),
	).Scan(
		&reservation.LimitKey,
		&actorID,
		&reservation.BucketName,
		&reservation.PeriodStart,
		&reservation.PeriodEnd,
		&reservation.Count,
		&reservation.HardCap,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get experience rate limit: %w", err)
	}
	reservation.ActorID = fromNullInt64(actorID)
	reservation.Limit = limit
	reservation.Allowed = limit <= 0 || reservation.Count < limit
	return &reservation, nil
}

func (r *experienceRepo) CreateExperienceMicroQuestionAnswer(ctx context.Context, answer *domain.ExperienceMicroQuestionAnswer) (bool, error) {
	if answer == nil {
		return false, fmt.Errorf("experience micro question answer is nil")
	}
	result, err := r.db.db.ExecContext(ctx, `
		INSERT IGNORE INTO experience_micro_question_answers (
			answer_event_key, suggestion_event_id, suggestion_stable_key, actor_id,
			surface, target_type, target_id, answer_value, reason_code, payload_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		strings.TrimSpace(answer.AnswerEventKey),
		strings.TrimSpace(answer.SuggestionEventID),
		strings.TrimSpace(answer.SuggestionStableKey),
		toNullInt64(answer.ActorID),
		strings.TrimSpace(answer.Surface),
		strings.TrimSpace(answer.TargetType),
		strings.TrimSpace(answer.TargetID),
		strings.TrimSpace(answer.AnswerValue),
		strings.TrimSpace(answer.ReasonCode),
		toNullJSONString(answer.Payload),
	)
	if err != nil {
		return false, fmt.Errorf("create experience micro question answer: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("read experience micro question insert result: %w", err)
	}
	return rows > 0, nil
}

func (r *experienceRepo) HasExperienceMicroQuestionAnswer(ctx context.Context, answerEventKey string) (bool, error) {
	var exists int
	if err := r.db.db.QueryRowContext(ctx, `
		SELECT 1
		FROM experience_micro_question_answers
		WHERE answer_event_key = ?
		LIMIT 1`,
		strings.TrimSpace(answerEventKey),
	).Scan(&exists); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("check experience micro question answer: %w", err)
	}
	return exists == 1, nil
}

func (r *experienceRepo) CreateExperienceReviewItem(ctx context.Context, item *domain.ExperienceReviewItem) error {
	if item == nil {
		return fmt.Errorf("experience review item is nil")
	}
	status := strings.TrimSpace(item.Status)
	if status == "" {
		status = domain.ExperienceReviewItemStatusOpen
	}
	_, err := r.db.db.ExecContext(ctx, `
		INSERT INTO experience_review_items (
			item_key, item_type, status, priority, evidence_summary_json
		) VALUES (?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			priority = IF(status = 'open', VALUES(priority), priority),
			evidence_summary_json = IF(status = 'open', VALUES(evidence_summary_json), evidence_summary_json),
			updated_at = CURRENT_TIMESTAMP`,
		strings.TrimSpace(item.ItemKey),
		strings.TrimSpace(item.ItemType),
		status,
		strings.TrimSpace(item.Priority),
		toNullJSONString(item.EvidenceSummary),
	)
	if err != nil {
		return fmt.Errorf("create experience review item: %w", err)
	}
	return nil
}

func (r *experienceRepo) ListExperienceReviewItems(ctx context.Context, filter repo.ExperienceReviewItemFilter) ([]*domain.ExperienceReviewItem, int64, error) {
	where := []string{"1=1"}
	args := make([]interface{}, 0, 4)
	if value := strings.TrimSpace(filter.Status); value != "" {
		where = append(where, "status = ?")
		args = append(args, value)
	}
	if value := strings.TrimSpace(filter.ItemType); value != "" {
		where = append(where, "item_type = ?")
		args = append(args, value)
	}
	whereSQL := strings.Join(where, " AND ")
	var total int64
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM experience_review_items WHERE `+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count experience review items: %w", err)
	}
	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	listArgs := append([]interface{}{}, args...)
	listArgs = append(listArgs, pageSize, (page-1)*pageSize)
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT id, item_key, item_type, status, priority, evidence_summary_json, created_at, updated_at
		FROM experience_review_items
		WHERE `+whereSQL+`
		ORDER BY
		  CASE priority WHEN 'high' THEN 1 WHEN 'medium' THEN 2 WHEN 'low' THEN 3 ELSE 4 END,
		  updated_at DESC,
		  id DESC
		LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list experience review items: %w", err)
	}
	defer rows.Close()
	items := make([]*domain.ExperienceReviewItem, 0)
	for rows.Next() {
		item, err := scanExperienceReviewItem(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate experience review items: %w", err)
	}
	return items, total, nil
}

func (r *experienceRepo) CreateExperienceReviewDecision(ctx context.Context, decision *domain.ExperienceReviewDecision, nextStatus string) error {
	if decision == nil {
		return fmt.Errorf("experience review decision is nil")
	}
	tx, err := r.db.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin experience review decision: %w", err)
	}
	var itemType, currentStatus string
	var evidenceSummary sql.NullString
	if err := tx.QueryRowContext(ctx, `
		SELECT item_type, status, evidence_summary_json
		FROM experience_review_items
		WHERE item_key = ?
		FOR UPDATE`,
		strings.TrimSpace(decision.ReviewItemKey),
	).Scan(&itemType, &currentStatus, &evidenceSummary); err != nil {
		_ = tx.Rollback()
		if err == sql.ErrNoRows {
			return fmt.Errorf("experience review item not found")
		}
		return fmt.Errorf("load experience review item: %w", err)
	}
	if strings.TrimSpace(currentStatus) != domain.ExperienceReviewItemStatusOpen {
		_ = tx.Rollback()
		return fmt.Errorf("experience review item is not open")
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE experience_review_items
		SET status = ?,
		    updated_at = CURRENT_TIMESTAMP
		WHERE item_key = ?`,
		strings.TrimSpace(nextStatus),
		strings.TrimSpace(decision.ReviewItemKey),
	); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("update experience review item status: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO experience_review_decisions (
			review_item_key, decision, reason_code, actor_id, payload_json
		) VALUES (?, ?, ?, ?, ?)`,
		strings.TrimSpace(decision.ReviewItemKey),
		strings.TrimSpace(decision.Decision),
		strings.TrimSpace(decision.ReasonCode),
		toNullInt64(decision.ActorID),
		toNullJSONString(decision.Payload),
	); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("create experience review decision: %w", err)
	}
	if strings.TrimSpace(decision.Decision) == domain.ExperienceReviewDecisionApprove {
		if err := materializeExperienceApprovedReview(ctx, tx, decision, itemType, rawJSONFromNull(evidenceSummary)); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit experience review decision: %w", err)
	}
	return nil
}

func materializeExperienceApprovedReview(ctx context.Context, tx *sql.Tx, decision *domain.ExperienceReviewDecision, itemType string, evidence json.RawMessage) error {
	if tx == nil || decision == nil || len(evidence) == 0 {
		return fmt.Errorf("experience review item evidence is required for approval")
	}
	var summary map[string]interface{}
	if err := json.Unmarshal(evidence, &summary); err != nil {
		return fmt.Errorf("decode experience review evidence: %w", err)
	}
	suggestion := experienceMapValue(summary, "suggestion")
	outcome := experienceMapValue(summary, "outcome")
	targetType := firstNonEmptyString(
		experienceStringValue(suggestion, "target_type"),
		experienceStringValue(outcome, "target_type"),
	)
	targetID := firstNonEmptyString(
		experienceStringValue(suggestion, "target_id"),
		experienceStringValue(outcome, "target_id"),
	)
	payload := mustJSONRaw(map[string]interface{}{
		"source":           "experience_review",
		"review_item_key":  decision.ReviewItemKey,
		"item_type":        itemType,
		"decision":         decision.Decision,
		"reason_code":      decision.ReasonCode,
		"evidence_summary": summary,
	})
	switch strings.TrimSpace(targetType) {
	case "task":
		taskID, err := strconv.ParseInt(strings.TrimSpace(targetID), 10, 64)
		if err != nil || taskID <= 0 {
			return fmt.Errorf("experience review task target is invalid")
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO task_experience_profiles (
				task_id, profile_version, source_event_watermark, task_type, category_code,
				category_name, task_status, outcome, profile_json, rebuilt_at
			) VALUES (?, 1, 0, ?, '', '', ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE
				profile_version = VALUES(profile_version),
				task_type = VALUES(task_type),
				task_status = VALUES(task_status),
				outcome = VALUES(outcome),
				profile_json = VALUES(profile_json),
				rebuilt_at = VALUES(rebuilt_at),
				updated_at = CURRENT_TIMESTAMP`,
			taskID,
			truncateSQLString(experienceStringValue(suggestion, "type"), 64),
			truncateSQLString(firstNonEmptyString(experienceStringValue(outcome, "outcome"), experienceStringValue(outcome, "action")), 64),
			truncateSQLString(experienceStringValue(outcome, "action"), 64),
			toNullJSONString(payload),
			time.Now().UTC(),
		); err != nil {
			return fmt.Errorf("materialize task experience profile: %w", err)
		}
	case "asset":
		assetID, err := strconv.ParseInt(strings.TrimSpace(targetID), 10, 64)
		if err != nil || assetID <= 0 {
			return fmt.Errorf("experience review asset target is invalid")
		}
		reasonCode := truncateSQLString(firstNonEmptyString(decision.ReasonCode, experienceStringValue(experienceMapValue(summary, "feedback"), "reason_code")), 96)
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO asset_quality_labels (
				asset_id, quality_label, reason_code, source_type, source_id, actor_id, payload_json
			)
			SELECT ?, 'reusable_candidate', ?, 'experience_review', ?, ?, ?
			FROM DUAL
			WHERE NOT EXISTS (
				SELECT 1 FROM asset_quality_labels
				WHERE source_type = 'experience_review' AND source_id = ?
			)`,
			assetID,
			reasonCode,
			strings.TrimSpace(decision.ReviewItemKey),
			toNullInt64(decision.ActorID),
			toNullJSONString(payload),
			strings.TrimSpace(decision.ReviewItemKey),
		); err != nil {
			return fmt.Errorf("materialize asset quality label: %w", err)
		}
	default:
		return fmt.Errorf("experience review target is not materializable")
	}
	return nil
}

func experienceMapValue(values map[string]interface{}, key string) map[string]interface{} {
	if values == nil {
		return map[string]interface{}{}
	}
	if nested, ok := values[key].(map[string]interface{}); ok {
		return nested
	}
	return map[string]interface{}{}
}

func experienceStringValue(values map[string]interface{}, key string) string {
	if values == nil {
		return ""
	}
	switch value := values[key].(type) {
	case string:
		return value
	case fmt.Stringer:
		return value.String()
	case float64:
		if value == float64(int64(value)) {
			return strconv.FormatInt(int64(value), 10)
		}
	}
	return ""
}

func mustJSONRaw(value interface{}) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return raw
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
	var targetType, targetID, sourceWatermark, observedFrom, observedID sql.NullString
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
		&targetType,
		&targetID,
		&sourceWatermark,
		&observedFrom,
		&observedID,
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
	event.TargetType = stringFromNull(targetType)
	event.TargetID = stringFromNull(targetID)
	event.SourceWatermark = stringFromNull(sourceWatermark)
	event.ObservedFrom = stringFromNull(observedFrom)
	event.ObservedID = stringFromNull(observedID)
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
	if strings.TrimSpace(event.TargetType) != "" || strings.TrimSpace(event.TargetID) != "" {
		return strings.TrimSpace(event.TargetType), strings.TrimSpace(event.TargetID)
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
	var targetType, targetID, sourceWatermark, observedFrom, observedID sql.NullString
	var nextRetryAt, claimedAt, processedAt sql.NullTime
	if err := scanner.Scan(
		&event.ID,
		&event.EventKey,
		&event.SchemaVersion,
		&event.SourceType,
		&event.SourceID,
		&taskID,
		&targetType,
		&targetID,
		&sourceWatermark,
		&observedFrom,
		&observedID,
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
	event.TargetType = stringFromNull(targetType)
	event.TargetID = stringFromNull(targetID)
	event.SourceWatermark = stringFromNull(sourceWatermark)
	event.ObservedFrom = stringFromNull(observedFrom)
	event.ObservedID = stringFromNull(observedID)
	event.ActorSnapshot = rawJSONFromNull(actorSnapshot)
	event.BusinessSnapshot = rawJSONFromNull(businessSnapshot)
	event.Payload = rawJSONFromNull(payload)
	event.NextRetryAt = fromNullTime(nextRetryAt)
	event.ClaimedAt = fromNullTime(claimedAt)
	event.ProcessedAt = fromNullTime(processedAt)
	return &event, nil
}

func scanExperienceWorkerRun(scanner interface {
	Scan(...interface{}) error
}) (*domain.ExperienceWorkerRunRecord, error) {
	var run domain.ExperienceWorkerRunRecord
	var finishedAt sql.NullTime
	var metadata sql.NullString
	if err := scanner.Scan(
		&run.ID,
		&run.WorkerName,
		&run.SourceName,
		&run.StartedAt,
		&finishedAt,
		&run.Status,
		&run.ScannedCount,
		&run.EnqueuedCount,
		&run.SkippedCount,
		&run.FailedCount,
		&run.LastError,
		&metadata,
		&run.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan experience worker run: %w", err)
	}
	run.FinishedAt = fromNullTime(finishedAt)
	run.Metadata = rawJSONFromNull(metadata)
	return &run, nil
}

func scanExperienceReviewItem(scanner interface {
	Scan(...interface{}) error
}) (*domain.ExperienceReviewItem, error) {
	var item domain.ExperienceReviewItem
	var evidence sql.NullString
	if err := scanner.Scan(
		&item.ID,
		&item.ItemKey,
		&item.ItemType,
		&item.Status,
		&item.Priority,
		&evidence,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan experience review item: %w", err)
	}
	item.EvidenceSummary = rawJSONFromNull(evidence)
	return &item, nil
}

func trimExperienceWorkerError(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 1024 {
		return value[:1024]
	}
	return value
}

func rawJSONFromNull(value sql.NullString) json.RawMessage {
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return nil
	}
	return json.RawMessage(value.String)
}

func stringFromNull(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func experienceCursorTime(value *time.Time) time.Time {
	if value == nil || value.IsZero() {
		return time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	return value.UTC()
}

func experienceSourceWatermark(at time.Time, id int64) string {
	return fmt.Sprintf("%s#%d", at.UTC().Format(time.RFC3339), id)
}

func mustExperienceJSON(value interface{}) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return raw
}

func rawJSONOrNil(value sql.NullString) json.RawMessage {
	if !value.Valid || strings.TrimSpace(value.String) == "" || !json.Valid([]byte(value.String)) {
		return nil
	}
	return json.RawMessage(value.String)
}

func experienceNullableTimeValue(value sql.NullTime) interface{} {
	if !value.Valid || value.Time.IsZero() {
		return nil
	}
	return value.Time.UTC().Format(time.RFC3339)
}

func auditOutcomeAction(action string) string {
	switch strings.TrimSpace(action) {
	case "approve":
		return "audit_approved"
	case "reject":
		return "audit_rejected"
	default:
		value := strings.TrimSpace(action)
		if value == "" {
			return "audit_action"
		}
		return "audit_" + value
	}
}

func taskTerminalState(status string) string {
	switch domain.TaskStatus(strings.TrimSpace(status)) {
	case domain.TaskStatusCompleted, domain.TaskStatusCancelled, domain.TaskStatusArchived:
		return strings.TrimSpace(status)
	default:
		return ""
	}
}

func taskAssetReviewTerminalState(status string, archived bool, archivedAt sql.NullTime) string {
	if archived || archivedAt.Valid {
		return "archived"
	}
	switch domain.TaskAssetFlowReviewStatus(strings.TrimSpace(status)) {
	case domain.TaskAssetFlowReviewStatusApproved,
		domain.TaskAssetFlowReviewStatusSuperseded,
		domain.TaskAssetFlowReviewStatusCleaned,
		domain.TaskAssetFlowReviewStatusNotApplicable:
		return strings.TrimSpace(status)
	default:
		return ""
	}
}

func erpFilingTerminalState(status string, syncRequired bool) string {
	if domain.FilingStatus(strings.TrimSpace(status)) == domain.FilingStatusFiled && !syncRequired {
		return string(domain.FilingStatusFiled)
	}
	return ""
}

func mergeModuleActorSnapshot(actorID sql.NullInt64, snapshot sql.NullString) json.RawMessage {
	if snapshot.Valid && strings.TrimSpace(snapshot.String) != "" && json.Valid([]byte(snapshot.String)) {
		return json.RawMessage(snapshot.String)
	}
	if actorID.Valid {
		return mustExperienceJSON(map[string]interface{}{
			"actor_type": "user",
			"actor_id":   actorID.Int64,
			"source":     "task_module_events",
		})
	}
	return nil
}

func (r *experienceRepo) execRowsAffected(ctx context.Context, query string, args ...interface{}) (int64, error) {
	result, err := r.db.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return rows, nil
}
