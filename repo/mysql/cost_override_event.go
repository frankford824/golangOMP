package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"workflow/domain"
	"workflow/repo"
)

type taskCostOverrideEventRepo struct{ db *DB }

func NewTaskCostOverrideEventRepo(db *DB) repo.TaskCostOverrideEventRepo {
	return &taskCostOverrideEventRepo{db: db}
}

func (r *taskCostOverrideEventRepo) Append(ctx context.Context, tx repo.Tx, event *domain.TaskCostOverrideAuditEvent) (*domain.TaskCostOverrideAuditEvent, error) {
	if event == nil {
		return nil, fmt.Errorf("append cost override event: event is nil")
	}
	sqlTx := Unwrap(tx)

	seq, err := nextTaskCostOverrideEventSequence(ctx, sqlTx, event.TaskID)
	if err != nil {
		return nil, err
	}

	eventID := strings.TrimSpace(event.EventID)
	if eventID == "" {
		eventID = uuid.NewString()
	}
	overrideAt := event.OverrideAt.UTC()
	if overrideAt.IsZero() {
		overrideAt = time.Now().UTC()
	}
	createdAt := event.CreatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = overrideAt
	}

	_, err = sqlTx.ExecContext(ctx, `
		INSERT INTO cost_override_events (
			event_id, task_id, task_detail_id, sequence, event_type, category_code,
			matched_rule_id, matched_rule_version, matched_rule_source, governance_status,
			previous_estimated_cost, previous_cost_price, override_cost, result_cost_price,
			override_reason, override_actor, override_at, source, note, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		eventID,
		event.TaskID,
		toNullInt64(event.TaskDetailID),
		seq,
		string(event.EventType),
		strings.TrimSpace(event.CategoryCode),
		toNullInt64(event.MatchedRuleID),
		toNullInt(event.MatchedRuleVersion),
		strings.TrimSpace(event.MatchedRuleSource),
		string(event.GovernanceStatus),
		toNullFloat64(event.PreviousEstimatedCost),
		toNullFloat64(event.PreviousCostPrice),
		toNullFloat64(event.OverrideCost),
		toNullFloat64(event.ResultCostPrice),
		strings.TrimSpace(event.OverrideReason),
		strings.TrimSpace(event.OverrideActor),
		overrideAt,
		strings.TrimSpace(event.Source),
		strings.TrimSpace(event.Note),
		createdAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert cost_override_event: %w", err)
	}

	copyEvent := *event
	copyEvent.EventID = eventID
	copyEvent.Sequence = seq
	copyEvent.OverrideAt = overrideAt
	copyEvent.CreatedAt = createdAt
	return &copyEvent, nil
}

func (r *taskCostOverrideEventRepo) ListByTaskID(ctx context.Context, taskID int64) ([]*domain.TaskCostOverrideAuditEvent, error) {
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT event_id, task_id, task_detail_id, sequence, event_type, category_code,
		       matched_rule_id, matched_rule_version, matched_rule_source, governance_status,
		       previous_estimated_cost, previous_cost_price, override_cost, result_cost_price,
		       override_reason, override_actor, override_at, source, note, created_at
		FROM cost_override_events
		WHERE task_id = ?
		ORDER BY sequence ASC`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list cost_override_events: %w", err)
	}
	defer rows.Close()

	events := make([]*domain.TaskCostOverrideAuditEvent, 0)
	for rows.Next() {
		event, err := scanTaskCostOverrideEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("scan cost_override_event: %w", err)
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (r *taskCostOverrideEventRepo) GetByEventID(ctx context.Context, eventID string) (*domain.TaskCostOverrideAuditEvent, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT event_id, task_id, task_detail_id, sequence, event_type, category_code,
		       matched_rule_id, matched_rule_version, matched_rule_source, governance_status,
		       previous_estimated_cost, previous_cost_price, override_cost, result_cost_price,
		       override_reason, override_actor, override_at, source, note, created_at
		FROM cost_override_events
		WHERE event_id = ?`, strings.TrimSpace(eventID))

	event, err := scanTaskCostOverrideEvent(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get cost_override_event by event_id: %w", err)
	}
	return event, nil
}

func scanTaskCostOverrideEvent(scanner interface {
	Scan(...interface{}) error
}) (*domain.TaskCostOverrideAuditEvent, error) {
	event := &domain.TaskCostOverrideAuditEvent{}
	var taskDetailID sql.NullInt64
	var matchedRuleID sql.NullInt64
	var matchedRuleVersion sql.NullInt64
	var previousEstimatedCost sql.NullFloat64
	var previousCostPrice sql.NullFloat64
	var overrideCost sql.NullFloat64
	var resultCostPrice sql.NullFloat64
	var governanceStatus string
	var eventType string
	if err := scanner.Scan(
		&event.EventID,
		&event.TaskID,
		&taskDetailID,
		&event.Sequence,
		&eventType,
		&event.CategoryCode,
		&matchedRuleID,
		&matchedRuleVersion,
		&event.MatchedRuleSource,
		&governanceStatus,
		&previousEstimatedCost,
		&previousCostPrice,
		&overrideCost,
		&resultCostPrice,
		&event.OverrideReason,
		&event.OverrideActor,
		&event.OverrideAt,
		&event.Source,
		&event.Note,
		&event.CreatedAt,
	); err != nil {
		return nil, err
	}
	event.TaskDetailID = fromNullInt64(taskDetailID)
	event.EventType = domain.TaskCostOverrideAuditEventType(eventType)
	event.MatchedRuleID = fromNullInt64(matchedRuleID)
	event.MatchedRuleVersion = fromNullInt(matchedRuleVersion)
	event.GovernanceStatus = domain.CostRuleGovernanceStatus(governanceStatus)
	event.PreviousEstimatedCost = fromNullFloat64(previousEstimatedCost)
	event.PreviousCostPrice = fromNullFloat64(previousCostPrice)
	event.OverrideCost = fromNullFloat64(overrideCost)
	event.ResultCostPrice = fromNullFloat64(resultCostPrice)
	return event, nil
}

func nextTaskCostOverrideEventSequence(ctx context.Context, sqlTx *sql.Tx, taskID int64) (int64, error) {
	var current int64
	err := sqlTx.QueryRowContext(ctx,
		`SELECT last_sequence FROM cost_override_event_sequences WHERE task_id = ? FOR UPDATE`,
		taskID,
	).Scan(&current)

	if err == sql.ErrNoRows {
		if _, err = sqlTx.ExecContext(ctx,
			`INSERT INTO cost_override_event_sequences (task_id, last_sequence) VALUES (?, 1)`,
			taskID,
		); err != nil {
			return 0, fmt.Errorf("cost_override_event nextSequence insert: %w", err)
		}
		return 1, nil
	}
	if err != nil {
		return 0, fmt.Errorf("cost_override_event nextSequence select: %w", err)
	}

	next := current + 1
	if _, err = sqlTx.ExecContext(ctx,
		`UPDATE cost_override_event_sequences SET last_sequence = ? WHERE task_id = ?`,
		next, taskID,
	); err != nil {
		return 0, fmt.Errorf("cost_override_event nextSequence update: %w", err)
	}
	return next, nil
}
