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
	if err := r.db.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		  FROM tasks
		 WHERE task_status NOT IN ('Draft', 'Completed', 'Archived', 'Cancelled')`).Scan(&inProgress); err != nil {
		return nil, fmt.Errorf("card in progress: %w", err)
	}
	if err := r.db.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT t.id)
		  FROM tasks t
		 WHERE t.task_status IN ('Completed', 'Archived')
		   AND (
		     DATE(t.updated_at) = UTC_DATE()
		     OR EXISTS (
		       SELECT 1
		         FROM task_event_logs tel
		        WHERE tel.task_id = t.id
		          AND tel.event_type IN (
		            'task.closed',
		            'task.warehouse.completed',
		            'task.audit.approved',
		            'task.customization.reviewed'
		          )
		          AND DATE(tel.created_at) = UTC_DATE()
		     )
		   )`).Scan(&completedToday); err != nil {
		return nil, fmt.Errorf("card completed today: %w", err)
	}
	if err := r.db.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		  FROM tasks
		 WHERE task_status IN ('Completed', 'Archived')`).Scan(&archivedTotal); err != nil {
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
		WITH dates AS (
			SELECT DATE(t.created_at) AS day
			  FROM tasks t
			 WHERE t.created_at >= ? AND t.created_at < ?` + where + `
			 GROUP BY DATE(t.created_at)
			UNION
			SELECT DATE(tel.created_at) AS day
			  FROM task_event_logs tel
			  JOIN tasks t ON t.id = tel.task_id
			 WHERE tel.created_at >= ? AND tel.created_at < ?
			   AND tel.event_type IN (
			     'task.audit.approved',
			     'task.customization.reviewed',
			     'task.warehouse.completed',
			     'task.closed'
			   )` + where + `
			 GROUP BY DATE(tel.created_at)
		)
		SELECT DATE_FORMAT(d.day, '%Y-%m-%d') AS day,
		       COALESCE(created.created_count, 0) AS created_count,
		       COALESCE(done.completed_count, 0) AS completed_count,
		       COALESCE(done.completed_count, 0) AS archived_count
		  FROM dates d
		  LEFT JOIN (
		    SELECT DATE(t.created_at) AS day, COUNT(*) AS created_count
		      FROM tasks t
		     WHERE t.created_at >= ? AND t.created_at < ?` + where + `
		     GROUP BY DATE(t.created_at)
		  ) created ON created.day = d.day
		  LEFT JOIN (
		    SELECT DATE(tel.created_at) AS day, COUNT(DISTINCT tel.task_id) AS completed_count
		      FROM task_event_logs tel
		      JOIN tasks t ON t.id = tel.task_id
		     WHERE tel.created_at >= ? AND tel.created_at < ?
		       AND tel.event_type IN (
		         'task.audit.approved',
		         'task.customization.reviewed',
		         'task.warehouse.completed',
		         'task.closed'
		       )` + where + `
		     GROUP BY DATE(tel.created_at)
		  ) done ON done.day = d.day
		 ORDER BY day ASC`
	queryArgs := reportRepeatedRangeArgs(filter, args, 4)
	rows, err := r.db.db.QueryContext(ctx, query, queryArgs...)
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
		WITH normalized_events AS (
			SELECT t.id AS task_id, 'task_detail' AS module_key, t.created_at AS start_at,
			       COALESCE(
			         MIN(CASE WHEN tel.event_type IN ('task.assigned', 'task.reassigned', 'task.design.submitted') THEN tel.created_at END),
			         t.updated_at
			       ) AS end_at
			  FROM tasks t
			  LEFT JOIN task_event_logs tel ON tel.task_id = t.id AND tel.created_at >= t.created_at
			 WHERE t.created_at >= ? AND t.created_at < ?` + where + `
			 GROUP BY t.id, t.created_at, t.updated_at
			UNION ALL
			SELECT t.id AS task_id, 'design' AS module_key, start_e.created_at AS start_at,
			       MIN(end_e.created_at) AS end_at
			  FROM task_event_logs start_e
			  JOIN tasks t ON t.id = start_e.task_id
			  JOIN task_event_logs end_e
			    ON end_e.task_id = start_e.task_id
			   AND end_e.created_at > start_e.created_at
			   AND end_e.event_type IN ('task.design.submitted', 'task.reassigned', 'task.audit.approved', 'task.audit.rejected')
			 WHERE start_e.event_type IN ('task.assigned', 'task.reassigned', 'task.batch_assigned')
			   AND start_e.created_at >= ? AND start_e.created_at < ?` + where + `
			 GROUP BY t.id, start_e.created_at
			UNION ALL
			SELECT t.id AS task_id, 'audit' AS module_key, start_e.created_at AS start_at,
			       MIN(end_e.created_at) AS end_at
			  FROM task_event_logs start_e
			  JOIN tasks t ON t.id = start_e.task_id
			  JOIN task_event_logs end_e
			    ON end_e.task_id = start_e.task_id
			   AND end_e.created_at > start_e.created_at
			   AND end_e.event_type IN ('task.audit.approved', 'task.audit.rejected')
			 WHERE start_e.event_type = 'task.design.submitted'
			   AND start_e.created_at >= ? AND start_e.created_at < ?` + where + `
			 GROUP BY t.id, start_e.created_at
			UNION ALL
			SELECT t.id AS task_id, 'customization' AS module_key, start_e.created_at AS start_at,
			       MIN(end_e.created_at) AS end_at
			  FROM task_event_logs start_e
			  JOIN tasks t ON t.id = start_e.task_id
			  JOIN task_event_logs end_e
			    ON end_e.task_id = start_e.task_id
			   AND end_e.created_at > start_e.created_at
			   AND end_e.event_type IN ('task.customization.reviewed', 'task.design.submitted', 'task.audit.approved', 'task.audit.rejected')
			 WHERE t.customization_required = 1
			   AND start_e.event_type = 'task.design.submitted'
			   AND start_e.created_at >= ? AND start_e.created_at < ?` + where + `
			 GROUP BY t.id, start_e.created_at
			UNION ALL
			SELECT t.id AS task_id, 'warehouse' AS module_key, start_e.created_at AS start_at,
			       MIN(end_e.created_at) AS end_at
			  FROM task_event_logs start_e
			  JOIN tasks t ON t.id = start_e.task_id
			  JOIN task_event_logs end_e
			    ON end_e.task_id = start_e.task_id
			   AND end_e.created_at > start_e.created_at
			   AND end_e.event_type IN ('task.warehouse.received', 'task.warehouse.completed', 'task.closed')
			 WHERE start_e.event_type IN ('task.audit.approved', 'task.customization.reviewed')
			   AND start_e.created_at >= ? AND start_e.created_at < ?` + where + `
			 GROUP BY t.id, start_e.created_at
		),
		ranked AS (
			SELECT module_key, dwell_seconds,
			       ROW_NUMBER() OVER (PARTITION BY module_key ORDER BY dwell_seconds) AS rn,
			       COUNT(*) OVER (PARTITION BY module_key) AS cnt
			  FROM (
			    SELECT module_key, TIMESTAMPDIFF(SECOND, start_at, end_at) AS dwell_seconds
			      FROM normalized_events
			     WHERE end_at IS NOT NULL AND end_at > start_at
			  ) samples
			 WHERE dwell_seconds IS NOT NULL AND dwell_seconds >= 0
		)
		SELECT module_key,
		       AVG(dwell_seconds) AS avg_dwell,
		       MAX(CASE WHEN rn >= CEIL(cnt * 0.95) THEN dwell_seconds END) AS p95_dwell,
		       COUNT(*) AS samples
		  FROM ranked
		 GROUP BY module_key`
	queryArgs := reportRepeatedRangeArgs(filter, args, 5)
	rows, err := r.db.db.QueryContext(ctx, query, queryArgs...)
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

func reportRepeatedRangeArgs(filter repo.ReportL1Filter, whereArgs []interface{}, count int) []interface{} {
	if count <= 0 {
		return nil
	}
	rangeEnd := filter.To.AddDate(0, 0, 1)
	out := make([]interface{}, 0, count*(2+len(whereArgs)))
	for i := 0; i < count; i++ {
		out = append(out, filter.From, rangeEnd)
		out = append(out, whereArgs...)
	}
	return out
}
