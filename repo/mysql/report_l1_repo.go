package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type reportL1Repo struct{ db *DB }

func NewReportL1Repo(db *DB) repo.ReportL1Repo { return &reportL1Repo{db: db} }

const reportDailyAggregateFreshnessTTL = 2 * time.Hour

func (r *reportL1Repo) RefreshDailyAggregates(ctx context.Context, from, to time.Time) error {
	if to.Before(from) {
		return fmt.Errorf("report daily aggregate date range is invalid")
	}
	if !mysqlTableExists(ctx, r.db.db, "report_task_daily") {
		return fmt.Errorf("report_task_daily table does not exist")
	}
	rangeEnd := to.AddDate(0, 0, 1)
	tx, err := r.db.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin report daily aggregate refresh: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM report_task_daily
		 WHERE day >= DATE(?) AND day < DATE(?)`, from, rangeEnd); err != nil {
		return fmt.Errorf("delete report daily aggregate range: %w", err)
	}
	if _, err := tx.ExecContext(ctx, reportTaskDailyRefreshSQL, from, rangeEnd, from, rangeEnd); err != nil {
		return fmt.Errorf("insert report daily aggregate range: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit report daily aggregate refresh: %w", err)
	}
	return nil
}

func (r *reportL1Repo) GetCards(ctx context.Context) ([]domain.L1Card, error) {
	var inProgress, completedToday, archivedTotal int64
	inProgressCtx, cancelInProgress := mysqlReadQueryContext(ctx)
	err := r.db.db.QueryRowContext(inProgressCtx, `
		SELECT COUNT(*)
		  FROM tasks
		 WHERE task_status NOT IN ('Draft', 'Completed', 'Archived', 'Cancelled')`).Scan(&inProgress)
	cancelInProgress()
	if err != nil {
		return nil, fmt.Errorf("card in progress: %w", err)
	}
	completedCtx, cancelCompleted := mysqlReadQueryContext(ctx)
	err = r.db.db.QueryRowContext(completedCtx, `
		SELECT COUNT(DISTINCT t.id)
		  FROM tasks t
		 WHERE t.task_status IN ('Completed', 'Archived')
		   AND (
		     (t.updated_at >= UTC_DATE() AND t.updated_at < DATE_ADD(UTC_DATE(), INTERVAL 1 DAY))
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
		          AND tel.created_at >= UTC_DATE()
		          AND tel.created_at < DATE_ADD(UTC_DATE(), INTERVAL 1 DAY)
		     )
		   )`).Scan(&completedToday)
	cancelCompleted()
	if err != nil {
		return nil, fmt.Errorf("card completed today: %w", err)
	}
	archivedCtx, cancelArchived := mysqlReadQueryContext(ctx)
	err = r.db.db.QueryRowContext(archivedCtx, `
		SELECT COUNT(*)
		  FROM tasks
		 WHERE task_status IN ('Completed', 'Archived')`).Scan(&archivedTotal)
	cancelArchived()
	if err != nil {
		return nil, fmt.Errorf("card archived total: %w", err)
	}
	return []domain.L1Card{
		{Key: "tasks_in_progress", Title: "Tasks in progress", Value: float64(inProgress)},
		{Key: "tasks_completed_today", Title: "Tasks completed today", Value: float64(completedToday)},
		{Key: "archived_total", Title: "Archived total", Value: float64(archivedTotal)},
	}, nil
}

func (r *reportL1Repo) GetThroughput(ctx context.Context, filter repo.ReportL1Filter) ([]domain.L1ThroughputPoint, error) {
	if mysqlTableExists(ctx, r.db.db, "report_task_daily") && r.reportDailyAggregateFresh(ctx) {
		return r.getThroughputFromDailyWithRealtimeTail(ctx, filter)
	}
	return r.getThroughputRealtime(ctx, filter)
}

func (r *reportL1Repo) reportDailyAggregateFresh(ctx context.Context) bool {
	queryCtx, cancelQuery := mysqlReadQueryContext(ctx)
	defer cancelQuery()
	var refreshedAt sql.NullTime
	if err := r.db.db.QueryRowContext(queryCtx, `SELECT MAX(updated_at) FROM report_task_daily`).Scan(&refreshedAt); err != nil {
		return false
	}
	if !refreshedAt.Valid {
		return false
	}
	return time.Since(refreshedAt.Time.UTC()) <= reportDailyAggregateFreshnessTTL
}

func (r *reportL1Repo) getThroughputFromDailyWithRealtimeTail(ctx context.Context, filter repo.ReportL1Filter) ([]domain.L1ThroughputPoint, error) {
	today := startOfUTCDay(time.Now())
	fromDay := startOfUTCDay(filter.From)
	toDay := startOfUTCDay(filter.To)
	if toDay.Before(today) {
		return r.getThroughputFromDaily(ctx, filter)
	}
	if !fromDay.Before(today) {
		return r.getThroughputRealtime(ctx, filter)
	}
	historyFilter := filter
	historyFilter.To = today.AddDate(0, 0, -1)
	currentFilter := filter
	currentFilter.From = today
	points, err := r.getThroughputFromDaily(ctx, historyFilter)
	if err != nil {
		return nil, err
	}
	current, err := r.getThroughputRealtime(ctx, currentFilter)
	if err != nil {
		return nil, err
	}
	return append(points, current...), nil
}

func (r *reportL1Repo) getThroughputFromDaily(ctx context.Context, filter repo.ReportL1Filter) ([]domain.L1ThroughputPoint, error) {
	where, args := reportDailyAggregateWhere(filter, "r")
	query := `
		SELECT DATE_FORMAT(r.day, '%Y-%m-%d') AS day,
		       SUM(r.created_count) AS created_count,
		       SUM(r.completed_count) AS completed_count,
		       SUM(r.completed_count) AS archived_count
		  FROM report_task_daily r
		 WHERE r.day >= DATE(?) AND r.day < DATE(?)` + where + `
		 GROUP BY r.day
		 ORDER BY r.day ASC`
	queryArgs := append([]interface{}{filter.From, filter.To.AddDate(0, 0, 1)}, args...)
	queryCtx, cancelQuery := mysqlReadQueryContext(ctx)
	rows, err := r.db.db.QueryContext(queryCtx, query, queryArgs...)
	if err != nil {
		cancelQuery()
		return nil, fmt.Errorf("throughput daily aggregate: %w", err)
	}
	defer cancelQuery()
	defer rows.Close()

	var out []domain.L1ThroughputPoint
	for rows.Next() {
		var item domain.L1ThroughputPoint
		if err := rows.Scan(&item.Date, &item.Created, &item.Completed, &item.Archived); err != nil {
			return nil, fmt.Errorf("scan throughput daily aggregate: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *reportL1Repo) getThroughputRealtime(ctx context.Context, filter repo.ReportL1Filter) ([]domain.L1ThroughputPoint, error) {
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
	queryCtx, cancelQuery := mysqlReadQueryContext(ctx)
	rows, err := r.db.db.QueryContext(queryCtx, query, queryArgs...)
	if err != nil {
		cancelQuery()
		return nil, fmt.Errorf("throughput: %w", err)
	}
	defer cancelQuery()
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
		WITH event_sequence AS (
			SELECT t.id AS task_id,
			       t.customization_required,
			       tel.event_type,
			       tel.created_at AS event_at,
			       MIN(CASE WHEN tel.event_type IN ('task.design.submitted', 'task.reassigned', 'task.audit.approved', 'task.audit.rejected') THEN tel.created_at END)
			         OVER (PARTITION BY t.id ORDER BY tel.created_at, tel.id ROWS BETWEEN 1 FOLLOWING AND UNBOUNDED FOLLOWING) AS design_end_at,
			       MIN(CASE WHEN tel.event_type IN ('task.audit.approved', 'task.audit.rejected') THEN tel.created_at END)
			         OVER (PARTITION BY t.id ORDER BY tel.created_at, tel.id ROWS BETWEEN 1 FOLLOWING AND UNBOUNDED FOLLOWING) AS audit_end_at,
			       MIN(CASE WHEN tel.event_type IN ('task.customization.reviewed', 'task.design.submitted', 'task.audit.approved', 'task.audit.rejected') THEN tel.created_at END)
			         OVER (PARTITION BY t.id ORDER BY tel.created_at, tel.id ROWS BETWEEN 1 FOLLOWING AND UNBOUNDED FOLLOWING) AS customization_end_at,
			       MIN(CASE WHEN tel.event_type IN ('task.warehouse.received', 'task.warehouse.completed', 'task.closed') THEN tel.created_at END)
			         OVER (PARTITION BY t.id ORDER BY tel.created_at, tel.id ROWS BETWEEN 1 FOLLOWING AND UNBOUNDED FOLLOWING) AS warehouse_end_at
			  FROM task_event_logs tel
			  JOIN tasks t ON t.id = tel.task_id
			 WHERE tel.created_at >= ? AND tel.created_at < ?` + where + `
		),
		normalized_events AS (
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
			SELECT task_id, 'design' AS module_key, event_at AS start_at, design_end_at AS end_at
			  FROM event_sequence
			 WHERE event_type IN ('task.assigned', 'task.reassigned', 'task.batch_assigned')
			   AND event_at < ?
			UNION ALL
			SELECT task_id, 'audit' AS module_key, event_at AS start_at, audit_end_at AS end_at
			  FROM event_sequence
			 WHERE event_type = 'task.design.submitted'
			   AND event_at < ?
			UNION ALL
			SELECT task_id, 'customization' AS module_key, event_at AS start_at, customization_end_at AS end_at
			  FROM event_sequence
			 WHERE customization_required = 1
			   AND event_type = 'task.design.submitted'
			   AND event_at < ?
			UNION ALL
			SELECT task_id, 'warehouse' AS module_key, event_at AS start_at, warehouse_end_at AS end_at
			  FROM event_sequence
			 WHERE event_type IN ('task.audit.approved', 'task.customization.reviewed')
			   AND event_at < ?
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
	queryArgs := reportModuleDwellWindowArgs(filter, args)
	queryCtx, cancelQuery := mysqlReadQueryContext(ctx)
	rows, err := r.db.db.QueryContext(queryCtx, query, queryArgs...)
	if err != nil {
		cancelQuery()
		return nil, fmt.Errorf("module dwell: %w", err)
	}
	defer cancelQuery()
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

func reportDailyAggregateWhere(filter repo.ReportL1Filter, alias string) (string, []interface{}) {
	var where []string
	var args []interface{}
	if filter.DepartmentID != nil {
		where = append(where, alias+".owner_department = CAST(? AS CHAR)")
		args = append(args, *filter.DepartmentID)
	}
	if filter.TaskType != nil && strings.TrimSpace(*filter.TaskType) != "" {
		where = append(where, alias+".task_type = ?")
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

func reportModuleDwellWindowArgs(filter repo.ReportL1Filter, whereArgs []interface{}) []interface{} {
	rangeEnd := filter.To.AddDate(0, 0, 1)
	out := make([]interface{}, 0, 8+len(whereArgs)*2)
	out = append(out, filter.From, rangeEnd)
	out = append(out, whereArgs...)
	out = append(out, filter.From, rangeEnd)
	out = append(out, whereArgs...)
	for i := 0; i < 4; i++ {
		out = append(out, rangeEnd)
	}
	return out
}

func startOfUTCDay(value time.Time) time.Time {
	year, month, day := value.UTC().Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

const reportTaskDailyRefreshSQL = `
	INSERT INTO report_task_daily (day, owner_department, task_type, created_count, completed_count)
	SELECT day, owner_department, task_type, SUM(created_count) AS created_count, SUM(completed_count) AS completed_count
	  FROM (
	        SELECT DATE(t.created_at) AS day,
	               COALESCE(t.owner_department, '') AS owner_department,
	               COALESCE(t.task_type, '') AS task_type,
	               COUNT(*) AS created_count,
	               0 AS completed_count
	          FROM tasks t
	         WHERE t.created_at >= ? AND t.created_at < ?
	         GROUP BY DATE(t.created_at), COALESCE(t.owner_department, ''), COALESCE(t.task_type, '')
	        UNION ALL
	        SELECT DATE(tel.created_at) AS day,
	               COALESCE(t.owner_department, '') AS owner_department,
	               COALESCE(t.task_type, '') AS task_type,
	               0 AS created_count,
	               COUNT(DISTINCT tel.task_id) AS completed_count
	          FROM task_event_logs tel
	          JOIN tasks t ON t.id = tel.task_id
	         WHERE tel.created_at >= ? AND tel.created_at < ?
	           AND tel.event_type IN (
	               'task.audit.approved',
	               'task.customization.reviewed',
	               'task.warehouse.completed',
	               'task.closed'
	           )
	         GROUP BY DATE(tel.created_at), COALESCE(t.owner_department, ''), COALESCE(t.task_type, '')
	       ) daily
	 GROUP BY day, owner_department, task_type
	ON DUPLICATE KEY UPDATE
	  created_count = VALUES(created_count),
	  completed_count = VALUES(completed_count)`
