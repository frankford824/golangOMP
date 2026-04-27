package mysqlrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

func NewServerLogRepo(db *DB) repo.ServerLogRepo {
	return &serverLogRepo{db: db}
}

type serverLogRepo struct {
	db *DB
}

func (r *serverLogRepo) Create(ctx context.Context, log *domain.ServerLog) (int64, error) {
	if log == nil {
		return 0, fmt.Errorf("server log is nil")
	}
	detailsJSON := ""
	if len(log.Details) > 0 {
		raw, err := json.Marshal(log.Details)
		if err != nil {
			return 0, fmt.Errorf("marshal server log details: %w", err)
		}
		detailsJSON = string(raw)
	}
	result, err := r.db.db.ExecContext(ctx, `
		INSERT INTO server_logs (level, msg, details_json, created_at)
		VALUES (?, ?, ?, ?)`,
		log.Level,
		log.Msg,
		nullString(detailsJSON),
		log.CreatedAt,
	)
	if err != nil {
		return 0, fmt.Errorf("insert server log: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("server log last insert id: %w", err)
	}
	return id, nil
}

func (r *serverLogRepo) List(ctx context.Context, filter repo.ServerLogListFilter) ([]*domain.ServerLog, int64, error) {
	where := []string{"1=1"}
	args := []interface{}{}
	if filter.Level != "" {
		where = append(where, "level = ?")
		args = append(args, filter.Level)
	}
	if filter.Since != nil {
		where = append(where, "created_at >= ?")
		args = append(args, *filter.Since)
	}
	if filter.Until != nil {
		where = append(where, "created_at <= ?")
		args = append(args, *filter.Until)
	}
	if filter.Keyword != "" {
		where = append(where, "(msg LIKE ? OR details_json LIKE ?)")
		kw := "%" + filter.Keyword + "%"
		args = append(args, kw, kw)
	}

	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	page := filter.Page
	if page < 1 {
		page = 1
	}

	countQuery := `SELECT COUNT(*) FROM server_logs WHERE ` + strings.Join(where, " AND ")
	var total int64
	if err := r.db.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count server logs: %w", err)
	}

	queryArgs := append([]interface{}{}, args...)
	queryArgs = append(queryArgs, pageSize, (page-1)*pageSize)
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT id, level, msg, details_json, created_at
		FROM server_logs
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY id DESC
		LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list server logs: %w", err)
	}
	defer rows.Close()

	logs := make([]*domain.ServerLog, 0)
	for rows.Next() {
		var entry domain.ServerLog
		var detailsJSON sql.NullString
		if err := rows.Scan(&entry.ID, &entry.Level, &entry.Msg, &detailsJSON, &entry.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan server log: %w", err)
		}
		if detailsJSON.Valid && detailsJSON.String != "" {
			_ = json.Unmarshal([]byte(detailsJSON.String), &entry.Details)
		}
		logs = append(logs, &entry)
	}
	return logs, total, rows.Err()
}

func (r *serverLogRepo) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	result, err := r.db.db.ExecContext(ctx, `DELETE FROM server_logs WHERE created_at < ?`, before)
	if err != nil {
		return 0, fmt.Errorf("delete server logs older than: %w", err)
	}
	return result.RowsAffected()
}
