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

type exportJobRepo struct{ db *DB }

func NewExportJobRepo(db *DB) repo.ExportJobRepo {
	return &exportJobRepo{db: db}
}

const exportJobSelectCols = `
	id, template_key, export_type, source_query_type,
	source_filters_json, normalized_filters_json, query_template_json,
	requested_by_actor_id, requested_by_roles_json, requested_by_source, requested_by_auth_mode,
	status, result_ref_json, remark, created_at, status_updated_at, finished_at, updated_at`

func (r *exportJobRepo) Create(ctx context.Context, tx repo.Tx, job *domain.ExportJob) (int64, error) {
	if job == nil {
		return 0, fmt.Errorf("create export job: job is nil")
	}
	sqlTx := Unwrap(tx)

	sourceFiltersJSON, err := json.Marshal(job.SourceFilters)
	if err != nil {
		return 0, fmt.Errorf("marshal export source filters: %w", err)
	}
	normalizedFiltersJSON, err := marshalOptionalExportJSON(job.NormalizedFilters)
	if err != nil {
		return 0, fmt.Errorf("marshal export normalized filters: %w", err)
	}
	queryTemplateJSON, err := marshalOptionalExportJSON(job.QueryTemplate)
	if err != nil {
		return 0, fmt.Errorf("marshal export query template: %w", err)
	}
	rolesJSON, err := json.Marshal(job.RequestedBy.Roles)
	if err != nil {
		return 0, fmt.Errorf("marshal export requested_by roles: %w", err)
	}
	resultRefJSON, err := marshalOptionalExportJSON(job.ResultRef)
	if err != nil {
		return 0, fmt.Errorf("marshal export result ref: %w", err)
	}

	res, err := sqlTx.ExecContext(ctx, `
		INSERT INTO export_jobs (
			template_key, export_type, source_query_type,
			source_filters_json, normalized_filters_json, query_template_json,
			requested_by_actor_id, requested_by_roles_json, requested_by_source, requested_by_auth_mode,
			status, result_ref_json, remark, status_updated_at, finished_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		job.TemplateKey,
		string(job.ExportType),
		string(job.SourceQueryType),
		string(sourceFiltersJSON),
		string(normalizedFiltersJSON),
		string(queryTemplateJSON),
		job.RequestedBy.ID,
		string(rolesJSON),
		job.RequestedBy.Source,
		string(job.RequestedBy.AuthMode),
		string(job.Status),
		string(resultRefJSON),
		job.Remark,
		job.LatestStatusAt,
		toNullTime(job.FinishedAt),
	)
	if err != nil {
		return 0, fmt.Errorf("insert export job: %w", err)
	}
	return res.LastInsertId()
}

func (r *exportJobRepo) GetByID(ctx context.Context, id int64) (*domain.ExportJob, error) {
	row := r.db.db.QueryRowContext(ctx, `SELECT `+exportJobSelectCols+` FROM export_jobs WHERE id = ?`, id)
	job, err := scanExportJob(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get export job by id: %w", err)
	}
	return job, nil
}

func (r *exportJobRepo) List(ctx context.Context, filter repo.ExportJobListFilter) ([]*domain.ExportJob, int64, error) {
	where := []string{"1=1"}
	args := make([]interface{}, 0, 4)
	if filter.Status != nil {
		where = append(where, "status = ?")
		args = append(args, string(*filter.Status))
	}
	if filter.SourceQueryType != nil {
		where = append(where, "source_query_type = ?")
		args = append(args, string(*filter.SourceQueryType))
	}
	if filter.RequestedByID != nil {
		where = append(where, "requested_by_actor_id = ?")
		args = append(args, *filter.RequestedByID)
	}
	whereSQL := strings.Join(where, " AND ")

	var total int64
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM export_jobs WHERE `+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count export jobs: %w", err)
	}

	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	offset := (page - 1) * pageSize
	listArgs := append(append([]interface{}{}, args...), pageSize, offset)
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT `+exportJobSelectCols+`
		FROM export_jobs
		WHERE `+whereSQL+`
		ORDER BY created_at DESC, id DESC
		LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list export jobs: %w", err)
	}
	defer rows.Close()

	jobs := make([]*domain.ExportJob, 0)
	for rows.Next() {
		job, err := scanExportJob(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan export job: %w", err)
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate export jobs: %w", err)
	}
	return jobs, total, nil
}

func (r *exportJobRepo) UpdateLifecycle(ctx context.Context, tx repo.Tx, update repo.ExportJobLifecycleUpdate) error {
	sqlTx := Unwrap(tx)

	resultRefJSON, err := marshalOptionalExportJSON(update.ResultRef)
	if err != nil {
		return fmt.Errorf("marshal export result ref: %w", err)
	}

	res, err := sqlTx.ExecContext(ctx, `
		UPDATE export_jobs
		SET status = ?,
			result_ref_json = ?,
			remark = ?,
			status_updated_at = ?,
			finished_at = ?
		WHERE id = ?`,
		string(update.Status),
		string(resultRefJSON),
		update.Remark,
		update.LatestStatusAt,
		toNullTime(update.FinishedAt),
		update.ExportJobID,
	)
	if err != nil {
		return fmt.Errorf("update export job lifecycle: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update export job lifecycle rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func marshalOptionalExportJSON(value interface{}) ([]byte, error) {
	if value == nil {
		return []byte("null"), nil
	}
	return json.Marshal(value)
}

func scanExportJob(scanner interface {
	Scan(...interface{}) error
}) (*domain.ExportJob, error) {
	var job domain.ExportJob
	var sourceFiltersJSON string
	var normalizedFiltersJSON string
	var queryTemplateJSON string
	var rolesJSON string
	var requestedBySource string
	var requestedByAuthMode string
	var resultRefJSON string
	var finishedAt sql.NullTime
	var latestStatusAt sql.NullTime
	if err := scanner.Scan(
		&job.ExportJobID,
		&job.TemplateKey,
		&job.ExportType,
		&job.SourceQueryType,
		&sourceFiltersJSON,
		&normalizedFiltersJSON,
		&queryTemplateJSON,
		&job.RequestedBy.ID,
		&rolesJSON,
		&requestedBySource,
		&requestedByAuthMode,
		&job.Status,
		&resultRefJSON,
		&job.Remark,
		&job.CreatedAt,
		&latestStatusAt,
		&finishedAt,
		&job.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if err := unmarshalExportSourceFilters(sourceFiltersJSON, &job.SourceFilters); err != nil {
		return nil, err
	}
	normalizedFilters, err := unmarshalOptionalTaskQueryFilters(normalizedFiltersJSON)
	if err != nil {
		return nil, err
	}
	queryTemplate, err := unmarshalOptionalTaskQueryTemplate(queryTemplateJSON)
	if err != nil {
		return nil, err
	}
	roles, err := unmarshalOptionalRoles(rolesJSON)
	if err != nil {
		return nil, err
	}
	resultRef, err := unmarshalOptionalResultRef(resultRefJSON)
	if err != nil {
		return nil, err
	}
	job.NormalizedFilters = normalizedFilters
	job.QueryTemplate = queryTemplate
	job.RequestedBy.Roles = domain.NormalizeRoleValues(roles)
	job.RequestedBy.Source = requestedBySource
	job.RequestedBy.AuthMode = domain.AuthMode(requestedByAuthMode)
	job.ResultRef = resultRef
	if value := fromNullTime(latestStatusAt); value != nil {
		job.LatestStatusAt = *value
	}
	job.FinishedAt = fromNullTime(finishedAt)
	domain.HydrateExportJobDerived(&job)
	return &job, nil
}

func unmarshalExportSourceFilters(raw string, out *domain.ExportSourceFilters) error {
	if strings.TrimSpace(raw) == "" || strings.TrimSpace(raw) == "null" {
		*out = domain.ExportSourceFilters{}
		return nil
	}
	if err := json.Unmarshal([]byte(raw), out); err != nil {
		return fmt.Errorf("unmarshal export source filters: %w", err)
	}
	return nil
}

func unmarshalOptionalTaskQueryFilters(raw string) (*domain.TaskQueryFilterDefinition, error) {
	if strings.TrimSpace(raw) == "" || strings.TrimSpace(raw) == "null" {
		return nil, nil
	}
	var value domain.TaskQueryFilterDefinition
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return nil, fmt.Errorf("unmarshal export normalized filters: %w", err)
	}
	return &value, nil
}

func unmarshalOptionalTaskQueryTemplate(raw string) (*domain.TaskQueryTemplate, error) {
	if strings.TrimSpace(raw) == "" || strings.TrimSpace(raw) == "null" {
		return nil, nil
	}
	var value domain.TaskQueryTemplate
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return nil, fmt.Errorf("unmarshal export query template: %w", err)
	}
	return &value, nil
}

func unmarshalOptionalRoles(raw string) ([]domain.Role, error) {
	if strings.TrimSpace(raw) == "" || strings.TrimSpace(raw) == "null" {
		return nil, nil
	}
	var roles []domain.Role
	if err := json.Unmarshal([]byte(raw), &roles); err != nil {
		return nil, fmt.Errorf("unmarshal export requested_by roles: %w", err)
	}
	return roles, nil
}

func unmarshalOptionalResultRef(raw string) (*domain.ExportResultRef, error) {
	if strings.TrimSpace(raw) == "" || strings.TrimSpace(raw) == "null" {
		return nil, nil
	}
	var value domain.ExportResultRef
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return nil, fmt.Errorf("unmarshal export result ref: %w", err)
	}
	return &value, nil
}
