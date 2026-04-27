package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

type reportL1Repo struct{ db *DB }

func NewReportL1Repo(db *DB) repo.ReportL1Repo { return &reportL1Repo{db: db} }

func (r *reportL1Repo) GetCards(ctx context.Context) ([]domain.L1Card, error) {
	var inProgress, completedToday, archivedTotal int64
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tasks WHERE task_status NOT IN ('closed', 'archived', 'cancelled', 'Draft')`).Scan(&inProgress); err != nil {
		return nil, fmt.Errorf("card in progress: %w", err)
	}
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT t.id) FROM tasks t JOIN task_modules tm ON tm.task_id=t.id JOIN task_module_events e ON e.task_module_id=tm.id WHERE e.event_type IN ('closed','archived','approved') AND DATE(e.created_at)=UTC_DATE()`).Scan(&completedToday); err != nil {
		return nil, fmt.Errorf("card completed today: %w", err)
	}
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tasks WHERE task_status IN ('archived', 'closed')`).Scan(&archivedTotal); err != nil {
		return nil, fmt.Errorf("card archived total: %w", err)
	}
	return []domain.L1Card{
		{Key: "tasks_in_progress", Title: "Tasks in progress", Value: float64(inProgress)},
		{Key: "tasks_completed_today", Title: "Tasks completed today", Value: float64(completedToday)},
		{Key: "archived_total", Title: "Archived total", Value: float64(archivedTotal)},
	}, nil
}

func (r *reportL1Repo) GetThroughput(ctx context.Context, filter repo.ReportL1Filter) ([]domain.L1ThroughputPoint, error) {
	where, args := reportFilterWhere(filter, "t")
	query := `
		SELECT DATE_FORMAT(e.created_at, '%Y-%m-%d') AS day,
		       SUM(CASE WHEN e.event_type = 'created' THEN 1 ELSE 0 END) AS created_count,
		       COUNT(DISTINCT CASE WHEN e.event_type IN ('closed','archived','approved') OR t.task_status IN ('closed','archived') THEN t.id END) AS completed_count,
		       COUNT(DISTINCT CASE WHEN e.event_type IN ('closed','archived','approved') OR t.task_status IN ('closed','archived') THEN t.id END) AS archived_count
		  FROM task_module_events e
		  JOIN task_modules tm ON tm.id = e.task_module_id
		  JOIN tasks t ON t.id = tm.task_id
		 WHERE e.created_at >= ? AND e.created_at < ?
		   AND e.event_type <> 'backfill_placeholder'` + where + `
		 GROUP BY DATE_FORMAT(e.created_at, '%Y-%m-%d')
		 ORDER BY day ASC`
	args = append([]interface{}{filter.From, filter.To.AddDate(0, 0, 1)}, args...)
	rows, err := r.db.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("throughput: %w", err)
	}
	defer rows.Close()

	var out []domain.L1ThroughputPoint
	for rows.Next() {
		var item domain.L1ThroughputPoint
		if err := rows.Scan(&item.Date, &item.Created, &item.Completed, &item.Archived); err != nil {
			return nil, fmt.Errorf("scan throughput: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *reportL1Repo) GetModuleDwell(ctx context.Context, filter repo.ReportL1Filter) ([]domain.L1ModuleDwellPoint, error) {
	where, args := reportFilterWhere(filter, "t")
	query := `
		WITH samples AS (
			SELECT tm.module_key,
			       TIMESTAMPDIFF(SECOND, e.created_at, exit_e.created_at) AS dwell_seconds
			  FROM task_module_events e
			  JOIN task_modules tm ON tm.id = e.task_module_id
			  JOIN tasks t ON t.id = tm.task_id
			  JOIN task_module_events exit_e
			    ON exit_e.task_module_id = e.task_module_id
			   AND exit_e.created_at > e.created_at
			   AND exit_e.event_type IN ('submitted','approved','rejected','closed','archived')
			 WHERE e.event_type IN ('entered','claimed','created')
			   AND e.created_at >= ? AND e.created_at < ?
			   AND e.event_type <> 'backfill_placeholder'` + where + `
		),
		ranked AS (
			SELECT module_key, dwell_seconds,
			       ROW_NUMBER() OVER (PARTITION BY module_key ORDER BY dwell_seconds) AS rn,
			       COUNT(*) OVER (PARTITION BY module_key) AS cnt
			  FROM samples
			 WHERE dwell_seconds IS NOT NULL AND dwell_seconds >= 0
		)
		SELECT module_key,
		       AVG(dwell_seconds) AS avg_dwell,
		       MAX(CASE WHEN rn >= CEIL(cnt * 0.95) THEN dwell_seconds END) AS p95_dwell,
		       COUNT(*) AS samples
		  FROM ranked
		 GROUP BY module_key`
	args = append([]interface{}{filter.From, filter.To.AddDate(0, 0, 1)}, args...)
	rows, err := r.db.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("module dwell: %w", err)
	}
	defer rows.Close()

	byKey := map[string]domain.L1ModuleDwellPoint{}
	for rows.Next() {
		var key string
		var avg, p95 sql.NullFloat64
		var samples int64
		if err := rows.Scan(&key, &avg, &p95, &samples); err != nil {
			return nil, fmt.Errorf("scan module dwell: %w", err)
		}
		point := domain.L1ModuleDwellPoint{ModuleKey: key, Samples: samples}
		if avg.Valid {
			point.AvgDwellSeconds = avg.Float64
		}
		if p95.Valid {
			point.P95DwellSeconds = p95.Float64
		}
		byKey[key] = point
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	keys := []string{"task_detail", "design", "audit", "customization", "warehouse"}
	out := make([]domain.L1ModuleDwellPoint, 0, len(keys))
	for _, key := range keys {
		if point, ok := byKey[key]; ok {
			out = append(out, point)
		} else {
			out = append(out, domain.L1ModuleDwellPoint{ModuleKey: key})
		}
	}
	return out, nil
}

func reportFilterWhere(filter repo.ReportL1Filter, taskAlias string) (string, []interface{}) {
	var where []string
	var args []interface{}
	if filter.DepartmentID != nil {
		where = append(where, taskAlias+".owner_department = CAST(? AS CHAR)")
		args = append(args, *filter.DepartmentID)
	}
	if filter.TaskType != nil && strings.TrimSpace(*filter.TaskType) != "" {
		where = append(where, taskAlias+".task_type = ?")
		args = append(args, strings.TrimSpace(*filter.TaskType))
	}
	if len(where) == 0 {
		return "", nil
	}
	return " AND " + strings.Join(where, " AND "), args
}
