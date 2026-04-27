package mysqlrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

type integrationCallLogRepo struct{ db *DB }

func NewIntegrationCallLogRepo(db *DB) repo.IntegrationCallLogRepo {
	return &integrationCallLogRepo{db: db}
}

const integrationCallLogSelectCols = `
	id, connector_key, operation_key, direction, resource_type, resource_id,
	status, requested_by_actor_id, requested_by_roles_json, requested_by_source, requested_by_auth_mode,
	request_payload_json, response_payload_json, error_message, status_updated_at, started_at, finished_at,
	remark, created_at, updated_at`

func (r *integrationCallLogRepo) Create(ctx context.Context, tx repo.Tx, log *domain.IntegrationCallLog) (int64, error) {
	if log == nil {
		return 0, fmt.Errorf("create integration call log: log is nil")
	}
	sqlTx := Unwrap(tx)

	rolesJSON, err := json.Marshal(log.RequestedBy.Roles)
	if err != nil {
		return 0, fmt.Errorf("marshal integration call roles: %w", err)
	}
	requestPayloadJSON, err := marshalOptionalExportJSON(log.RequestPayload)
	if err != nil {
		return 0, fmt.Errorf("marshal integration request payload: %w", err)
	}
	responsePayloadJSON, err := marshalOptionalExportJSON(log.ResponsePayload)
	if err != nil {
		return 0, fmt.Errorf("marshal integration response payload: %w", err)
	}

	res, err := sqlTx.ExecContext(ctx, `
		INSERT INTO integration_call_logs (
			connector_key, operation_key, direction, resource_type, resource_id,
			status, requested_by_actor_id, requested_by_roles_json, requested_by_source, requested_by_auth_mode,
			request_payload_json, response_payload_json, error_message, status_updated_at, started_at, finished_at, remark
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		string(log.ConnectorKey),
		log.OperationKey,
		string(log.Direction),
		log.ResourceType,
		toNullInt64(log.ResourceID),
		string(log.Status),
		log.RequestedBy.ID,
		string(rolesJSON),
		log.RequestedBy.Source,
		string(log.RequestedBy.AuthMode),
		string(requestPayloadJSON),
		string(responsePayloadJSON),
		log.ErrorMessage,
		log.LatestStatusAt,
		toNullTime(log.StartedAt),
		toNullTime(log.FinishedAt),
		log.Remark,
	)
	if err != nil {
		return 0, fmt.Errorf("insert integration call log: %w", err)
	}
	return res.LastInsertId()
}

func (r *integrationCallLogRepo) GetByID(ctx context.Context, id int64) (*domain.IntegrationCallLog, error) {
	row := r.db.db.QueryRowContext(ctx, `SELECT `+integrationCallLogSelectCols+` FROM integration_call_logs WHERE id = ?`, id)
	log, err := scanIntegrationCallLog(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get integration call log: %w", err)
	}
	return log, nil
}

func (r *integrationCallLogRepo) List(ctx context.Context, filter repo.IntegrationCallLogListFilter) ([]*domain.IntegrationCallLog, int64, error) {
	where := []string{"1=1"}
	args := make([]interface{}, 0, 6)
	if filter.ConnectorKey != nil {
		where = append(where, "connector_key = ?")
		args = append(args, string(*filter.ConnectorKey))
	}
	if filter.Status != nil {
		where = append(where, "status = ?")
		args = append(args, string(*filter.Status))
	}
	if strings.TrimSpace(filter.ResourceType) != "" {
		where = append(where, "resource_type = ?")
		args = append(args, strings.TrimSpace(filter.ResourceType))
	}
	if filter.ResourceID != nil {
		where = append(where, "resource_id = ?")
		args = append(args, *filter.ResourceID)
	}
	whereSQL := strings.Join(where, " AND ")

	var total int64
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM integration_call_logs WHERE `+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count integration call logs: %w", err)
	}

	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	offset := (page - 1) * pageSize
	listArgs := append(append([]interface{}{}, args...), pageSize, offset)
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT `+integrationCallLogSelectCols+`
		FROM integration_call_logs
		WHERE `+whereSQL+`
		ORDER BY created_at DESC, id DESC
		LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list integration call logs: %w", err)
	}
	defer rows.Close()

	logs := make([]*domain.IntegrationCallLog, 0)
	for rows.Next() {
		log, err := scanIntegrationCallLog(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan integration call log: %w", err)
		}
		logs = append(logs, log)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate integration call logs: %w", err)
	}
	return logs, total, nil
}

func (r *integrationCallLogRepo) Update(ctx context.Context, tx repo.Tx, update repo.IntegrationCallLogUpdate) error {
	sqlTx := Unwrap(tx)
	responsePayloadJSON, err := marshalOptionalExportJSON(update.ResponsePayload)
	if err != nil {
		return fmt.Errorf("marshal integration response payload: %w", err)
	}
	res, err := sqlTx.ExecContext(ctx, `
		UPDATE integration_call_logs
		SET status = ?, response_payload_json = ?, error_message = ?, status_updated_at = ?, started_at = ?, finished_at = ?, remark = ?
		WHERE id = ?`,
		string(update.Status),
		string(responsePayloadJSON),
		update.ErrorMessage,
		update.LatestStatusAt,
		toNullTime(update.StartedAt),
		toNullTime(update.FinishedAt),
		update.Remark,
		update.CallLogID,
	)
	if err != nil {
		return fmt.Errorf("update integration call log: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update integration call log rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func scanIntegrationCallLog(scanner interface {
	Scan(...interface{}) error
}) (*domain.IntegrationCallLog, error) {
	var log domain.IntegrationCallLog
	var connectorKey string
	var direction string
	var status string
	var rolesJSON string
	var requestPayloadJSON string
	var responsePayloadJSON string
	var requestedBySource string
	var requestedByAuthMode string
	var resourceID sql.NullInt64
	var startedAt sql.NullTime
	var finishedAt sql.NullTime
	var latestStatusAt sql.NullTime
	if err := scanner.Scan(
		&log.CallLogID,
		&connectorKey,
		&log.OperationKey,
		&direction,
		&log.ResourceType,
		&resourceID,
		&status,
		&log.RequestedBy.ID,
		&rolesJSON,
		&requestedBySource,
		&requestedByAuthMode,
		&requestPayloadJSON,
		&responsePayloadJSON,
		&log.ErrorMessage,
		&latestStatusAt,
		&startedAt,
		&finishedAt,
		&log.Remark,
		&log.CreatedAt,
		&log.UpdatedAt,
	); err != nil {
		return nil, err
	}
	roles, err := unmarshalOptionalRoles(rolesJSON)
	if err != nil {
		return nil, err
	}
	requestPayload, err := unmarshalOptionalRawJSON(requestPayloadJSON, "integration request payload")
	if err != nil {
		return nil, err
	}
	responsePayload, err := unmarshalOptionalRawJSON(responsePayloadJSON, "integration response payload")
	if err != nil {
		return nil, err
	}
	log.ConnectorKey = domain.IntegrationConnectorKey(connectorKey)
	log.Direction = domain.IntegrationCallDirection(direction)
	log.Status = domain.IntegrationCallStatus(status)
	log.RequestedBy.Roles = domain.NormalizeRoleValues(roles)
	log.RequestedBy.Source = requestedBySource
	log.RequestedBy.AuthMode = domain.AuthMode(requestedByAuthMode)
	if resourceID.Valid {
		value := resourceID.Int64
		log.ResourceID = &value
	}
	log.RequestPayload = requestPayload
	log.ResponsePayload = responsePayload
	log.StartedAt = fromNullTime(startedAt)
	log.FinishedAt = fromNullTime(finishedAt)
	if value := fromNullTime(latestStatusAt); value != nil {
		log.LatestStatusAt = *value
	}
	domain.HydrateIntegrationCallLogDerived(&log)
	return &log, nil
}

func unmarshalOptionalRawJSON(raw string, label string) ([]byte, error) {
	if strings.TrimSpace(raw) == "" || strings.TrimSpace(raw) == "null" {
		return nil, nil
	}
	value := []byte(raw)
	if !json.Valid(value) {
		return nil, fmt.Errorf("unmarshal %s: invalid json", label)
	}
	return value, nil
}
