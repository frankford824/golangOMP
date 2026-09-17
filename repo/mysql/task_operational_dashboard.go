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

const taskOperationalCompletionCTE = `
	WITH exact_completion AS (
		SELECT task_id, MIN(created_at) AS completed_at
		  FROM task_event_logs
		 WHERE event_type = 'task.closed'
		    OR (
		      event_type = 'task.design.submitted'
		      AND JSON_UNQUOTE(JSON_EXTRACT(payload, '$.to_task_status')) = 'Completed'
		    )
		 GROUP BY task_id
	), task_facts AS (
		SELECT t.*,
		       ec.completed_at AS exact_completed_at,
		       COALESCE(ec.completed_at, CASE WHEN t.task_status = 'Completed' THEN t.updated_at END) AS completed_at
		  FROM tasks t
	  LEFT JOIN exact_completion ec ON ec.task_id = t.id
	)`

type taskOperationalDashboardRepo struct{ db *DB }

func NewTaskOperationalDashboardRepo(db *DB) repo.TaskOperationalDashboardRepo {
	return &taskOperationalDashboardRepo{db: db}
}

func (r *taskOperationalDashboardRepo) ListDesignDepartmentMembers(ctx context.Context, departmentID int64) ([]domain.DesignDepartmentMember, error) {
	queryCtx, cancel := mysqlReadQueryContext(ctx)
	rows, err := r.db.db.QueryContext(queryCtx, `
		SELECT id, employee_no, display_name, username, team_id, team, status
		  FROM users
		 WHERE department_id = ?
		 ORDER BY status = 'active' DESC, COALESCE(employee_no, 65535), display_name, id`, departmentID)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("list design department members: %w", err)
	}
	defer cancel()
	defer rows.Close()
	items := make([]domain.DesignDepartmentMember, 0)
	for rows.Next() {
		var item domain.DesignDepartmentMember
		var employeeNo, teamID sql.NullInt64
		if err := rows.Scan(&item.UserID, &employeeNo, &item.DisplayName, &item.Username, &teamID, &item.TeamName, &item.Status); err != nil {
			return nil, fmt.Errorf("scan design department member: %w", err)
		}
		item.EmployeeNo = fromNullInt64(employeeNo)
		item.TeamID = fromNullInt64(teamID)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *taskOperationalDashboardRepo) ListDesignDepartmentTaskFacts(ctx context.Context, filter domain.DesignDepartmentDashboardFilter) ([]domain.DesignDepartmentTaskFact, error) {
	dateColumn := "t.created_at"
	primaryColumn := "tf.created_at"
	switch filter.DateBasis {
	case domain.DesignDashboardDateCompleted:
		dateColumn = "COALESCE(cs.completed_at, CASE WHEN t.task_status = 'Completed' THEN t.updated_at END)"
		primaryColumn = "tf.completed_at"
	case domain.DesignDashboardDateDeadline:
		dateColumn = "t.deadline_at"
		primaryColumn = "tf.deadline_at"
	}
	where := []string{"d.department_id = ?", dateColumn + " >= ?", dateColumn + " < ?"}
	args := []interface{}{filter.DepartmentID, filter.StartAt, filter.EndAt}
	appendInt64List := func(column string, values []int64) {
		if len(values) == 0 {
			return
		}
		placeholders := make([]string, 0, len(values))
		for _, value := range values {
			if value > 0 {
				placeholders = append(placeholders, "?")
				args = append(args, value)
			}
		}
		if len(placeholders) > 0 {
			where = append(where, column+" IN ("+strings.Join(placeholders, ",")+")")
		}
	}
	appendStringList := func(column string, values []string) {
		if len(values) == 0 {
			return
		}
		placeholders := make([]string, 0, len(values))
		for _, value := range values {
			if strings.TrimSpace(value) != "" {
				placeholders = append(placeholders, "?")
				args = append(args, strings.TrimSpace(value))
			}
		}
		if len(placeholders) > 0 {
			where = append(where, column+" IN ("+strings.Join(placeholders, ",")+")")
		}
	}
	appendInt64List("d.id", filter.DesignerIDs)
	appendInt64List("d.team_id", filter.TeamIDs)
	taskTypes := make([]string, 0, len(filter.TaskTypes))
	for _, value := range filter.TaskTypes {
		taskTypes = append(taskTypes, string(value))
	}
	statuses := make([]string, 0, len(filter.Statuses))
	for _, value := range filter.Statuses {
		statuses = append(statuses, string(value))
	}
	priorities := make([]string, 0, len(filter.Priorities))
	for _, value := range filter.Priorities {
		priorities = append(priorities, string(value))
	}
	lanes := make([]string, 0, len(filter.BusinessLanes))
	for _, value := range filter.BusinessLanes {
		lanes = append(lanes, string(value))
	}
	appendStringList("t.task_type", taskTypes)
	appendStringList("t.task_status", statuses)
	appendStringList("t.priority", priorities)
	appendStringList("COALESCE(t.business_lane, '')", lanes)

	query := `
	WITH completion_stats AS (
		SELECT tel.task_id,
		       MIN(tel.created_at) completed_at
		  FROM task_event_logs tel
		 WHERE tel.event_type = 'task.closed'
		    OR (tel.event_type IN ('task.design.submitted', 'task.design_submitted') AND JSON_UNQUOTE(JSON_EXTRACT(tel.payload, '$.to_task_status')) = 'Completed')
		 GROUP BY tel.task_id
	), target_tasks AS (
		SELECT t.*, d.id designer_user_id,
		       COALESCE(NULLIF(d.display_name,''), d.username) designer_name,
		       d.status designer_status, d.department_id designer_department_id,
		       d.team_id designer_team_id, COALESCE(d.team,'') designer_team_name,
		       COALESCE(cs.completed_at, CASE WHEN t.task_status = 'Completed' THEN t.updated_at END) completed_at
		  FROM tasks t
		  JOIN users d ON d.id = t.designer_id
		  LEFT JOIN completion_stats cs ON cs.task_id = t.id
		 WHERE ` + strings.Join(where, " AND ") + `
	), event_stats AS (
		SELECT tel.task_id,
		       SUM(event_type IN ('task.design.submitted', 'task.design_submitted')) design_submission_count,
		       SUM(event_type IN ('task.audit.rejected', 'task.audit.returned_to_design')) audit_reject_count,
		       SUM(event_type = 'task.audit.approved') audit_approve_count
		  FROM task_event_logs tel JOIN target_tasks tt ON tt.id = tel.task_id GROUP BY tel.task_id
	), asset_stats AS (
		SELECT ta.task_id,
		       SUM(asset_type = 'source') source_file_count,
		       SUM(asset_type = 'delivery') final_file_count
		  FROM task_assets ta JOIN target_tasks tt ON tt.id = ta.task_id
		 WHERE ta.deleted_at IS NULL AND ta.cleaned_at IS NULL AND ta.upload_status = 'uploaded'
		 GROUP BY ta.task_id
	), sku_stats AS (
		SELECT tsi.task_id, COUNT(*) sku_count FROM task_sku_items tsi JOIN target_tasks tt ON tt.id = tsi.task_id GROUP BY tsi.task_id
	), group_stats AS (
		SELECT tag.task_id, COUNT(*) resource_unit_count FROM task_asset_groups tag JOIN target_tasks tt ON tt.id = tag.task_id GROUP BY tag.task_id
	), task_facts AS (
		SELECT t.id task_id, t.task_no, COALESCE(t.product_name_snapshot, '') product_name,
		       t.task_type, t.task_status, t.priority, COALESCE(t.business_lane, '') business_lane,
		       t.designer_user_id designer_id, t.designer_name,
		       t.designer_status, t.designer_department_id,
		       t.designer_team_id, t.designer_team_name,
		       t.created_at, t.updated_at, t.completed_at,
		       t.deadline_at,
		       COALESCE(ss.sku_count,0) sku_count,
		       COALESCE(gs.resource_unit_count,0) resource_unit_count,
		       COALESCE(ast.source_file_count,0) source_file_count,
		       COALESCE(ast.final_file_count,0) final_file_count,
		       COALESCE(es.design_submission_count,0) design_submission_count,
		       COALESCE(es.audit_reject_count,0) audit_reject_count,
		       COALESCE(es.audit_approve_count,0) audit_approve_count
		  FROM target_tasks t
		  LEFT JOIN event_stats es ON es.task_id = t.id
		  LEFT JOIN asset_stats ast ON ast.task_id = t.id
		  LEFT JOIN sku_stats ss ON ss.task_id = t.id
		  LEFT JOIN group_stats gs ON gs.task_id = t.id
	)
	SELECT tf.task_id, tf.task_no, tf.product_name, tf.task_type, tf.task_status, tf.priority, tf.business_lane,
	       tf.designer_id, tf.designer_name, tf.designer_status, tf.designer_team_id, tf.designer_team_name,
	       tf.created_at, tf.updated_at, tf.completed_at, tf.deadline_at, ` + primaryColumn + ` primary_at,
	       tf.sku_count, tf.resource_unit_count, tf.source_file_count, tf.final_file_count,
	       tf.design_submission_count, tf.audit_reject_count, tf.audit_approve_count
	  FROM task_facts tf
	 ORDER BY primary_at DESC, tf.task_id DESC`
	queryCtx, cancel := mysqlReadQueryContext(ctx)
	rows, err := r.db.db.QueryContext(queryCtx, query, args...)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("list design department dashboard facts: %w", err)
	}
	defer cancel()
	defer rows.Close()
	items := make([]domain.DesignDepartmentTaskFact, 0)
	for rows.Next() {
		var item domain.DesignDepartmentTaskFact
		var businessLane string
		var teamID sql.NullInt64
		var completedAt, deadlineAt sql.NullTime
		if err := rows.Scan(
			&item.TaskID, &item.TaskNo, &item.ProductName, &item.TaskType, &item.TaskStatus, &item.Priority, &businessLane,
			&item.DesignerID, &item.DesignerName, &item.DesignerStatus, &teamID, &item.TeamName,
			&item.CreatedAt, &item.UpdatedAt, &completedAt, &deadlineAt, &item.PrimaryAt,
			&item.SKUCount, &item.ResourceUnitCount, &item.SourceFileCount, &item.FinalFileCount,
			&item.DesignSubmissionCount, &item.AuditRejectCount, &item.AuditApproveCount,
		); err != nil {
			return nil, fmt.Errorf("scan design department dashboard fact: %w", err)
		}
		item.BusinessLane = domain.TaskBusinessLane(businessLane)
		item.TeamID = fromNullInt64(teamID)
		item.CompletedAt = fromNullTime(completedAt)
		item.DeadlineAt = fromNullTime(deadlineAt)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *taskOperationalDashboardRepo) GetTaskOperationalOverview(ctx context.Context, now time.Time) (*domain.TaskOperationalOverview, error) {
	generatedAt := now.UTC()
	location := taskOperationalLocation()
	todayStart, tomorrowStart, weekStart, trendStart := taskOperationalBoundaries(generatedAt, location)

	overview := &domain.TaskOperationalOverview{
		GeneratedAt:  generatedAt,
		TimeZone:     location.String(),
		PeriodStart:  weekStart,
		PeriodEnd:    tomorrowStart,
		HealthStatus: "ok",
		Trend:        make([]domain.TaskOperationalTrendPoint, 0, 7),
		RecentTasks:  make([]domain.TaskOperationalRecentTask, 0, 8),
		RecentEvents: make([]domain.TaskOperationalEvent, 0, 20),
	}

	if err := r.loadTaskOperationalCounts(ctx, overview, todayStart, tomorrowStart, weekStart); err != nil {
		return nil, err
	}
	if err := r.loadTaskOperationalTrend(ctx, overview, trendStart, tomorrowStart, location); err != nil {
		return nil, err
	}
	if err := r.loadTaskOperationalDistribution(ctx, overview); err != nil {
		return nil, err
	}
	if err := r.loadTaskOperationalRecentTasks(ctx, overview); err != nil {
		return nil, err
	}
	if err := r.loadTaskOperationalRecentEvents(ctx, overview); err != nil {
		return nil, err
	}
	return overview, nil
}

func (r *taskOperationalDashboardRepo) loadTaskOperationalRecentTasks(ctx context.Context, overview *domain.TaskOperationalOverview) error {
	query := `
	SELECT t.id, t.task_no, COALESCE(t.product_name_snapshot, ''),
	       COALESCE(NULLIF(designer.display_name, ''), NULLIF(designer.username, ''),
	                NULLIF(handler.display_name, ''), NULLIF(handler.username, ''),
	                NULLIF(requester.display_name, ''), NULLIF(requester.username, ''),
	                NULLIF(creator.display_name, ''), NULLIF(creator.username, ''), '—') AS owner_name,
	       t.task_status, t.deadline_at
	  FROM tasks t
	  LEFT JOIN users designer ON designer.id = t.designer_id
	  LEFT JOIN users handler ON handler.id = t.current_handler_id
	  LEFT JOIN users requester ON requester.id = t.requester_id
	  LEFT JOIN users creator ON creator.id = t.creator_id
	 ORDER BY t.updated_at DESC, t.id DESC
	 LIMIT 8`
	queryCtx, cancel := mysqlReadQueryContext(ctx)
	rows, err := r.db.db.QueryContext(queryCtx, query)
	if err != nil {
		cancel()
		return fmt.Errorf("task operational overview recent tasks: %w", err)
	}
	defer cancel()
	defer rows.Close()
	for rows.Next() {
		var task domain.TaskOperationalRecentTask
		if err := rows.Scan(&task.TaskID, &task.TaskNo, &task.ProductName, &task.OwnerName, &task.TaskStatus, &task.DeadlineAt); err != nil {
			return fmt.Errorf("scan task operational overview recent task: %w", err)
		}
		overview.RecentTasks = append(overview.RecentTasks, task)
	}
	return rows.Err()
}

func taskOperationalLocation() *time.Location {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err == nil {
		return location
	}
	return time.FixedZone("Asia/Shanghai", 8*60*60)
}

func taskOperationalBoundaries(now time.Time, location *time.Location) (todayStart, tomorrowStart, weekStart, trendStart time.Time) {
	localNow := now.In(location)
	localToday := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, location)
	daysSinceMonday := (int(localToday.Weekday()) + 6) % 7
	return localToday.UTC(), localToday.AddDate(0, 0, 1).UTC(), localToday.AddDate(0, 0, -daysSinceMonday).UTC(), localToday.AddDate(0, 0, -6).UTC()
}

func (r *taskOperationalDashboardRepo) loadTaskOperationalCounts(
	ctx context.Context,
	overview *domain.TaskOperationalOverview,
	todayStart, tomorrowStart, weekStart time.Time,
) error {
	query := taskOperationalCompletionCTE + `
	SELECT
		COUNT(*) AS total_tasks,
		COALESCE(SUM(task_status NOT IN ('Completed', 'Archived', 'Cancelled')), 0) AS active_tasks,
		COALESCE(SUM(task_status IN ('Draft', 'PendingAssign', 'Assigned', 'InProgress')), 0) AS design_pending,
		COALESCE(SUM(task_status = 'PendingAudit' AND task_type <> 'retouch_task'), 0) AS pending_audit,
		COALESCE(SUM(task_status = 'PendingAudit' AND current_handler_id IS NULL AND task_type <> 'retouch_task'), 0) AS handover,
		COALESCE(SUM(task_status NOT IN ('Completed', 'Archived', 'Cancelled') AND (COALESCE(business_lane, '') = 'customization' OR customization_required = 1)), 0) AS customization_in_progress,
		COALESCE(SUM(task_status NOT IN ('Completed', 'Archived', 'Cancelled') AND deadline_at IS NOT NULL AND deadline_at < ?), 0) AS overdue,
		COALESCE(SUM(deadline_at >= ? AND deadline_at < ?), 0) AS due_today,
		COALESCE(SUM(created_at >= ? AND created_at < ?), 0) AS today_created,
		COALESCE(SUM(completed_at >= ? AND completed_at < ?), 0) AS today_completed,
		COALESCE(SUM(created_at >= ? AND created_at < ?), 0) AS week_created,
		COALESCE(SUM(created_at >= ? AND created_at < ? AND task_status = 'Completed'), 0) AS week_created_completed,
		COALESCE(SUM(completed_at >= ? AND completed_at < ?), 0) AS week_completed,
		COALESCE(AVG(CASE WHEN completed_at >= ? AND completed_at < ? AND completed_at > created_at THEN TIMESTAMPDIFF(SECOND, created_at, completed_at) / 3600 END), 0) AS average_processing_hours,
		COALESCE(SUM(completed_at >= ? AND completed_at < ?), 0) AS average_processing_sample_count,
		COALESCE(SUM(completed_at >= ? AND completed_at < ? AND exact_completed_at IS NOT NULL), 0) AS exact_completion_sample_count,
		COALESCE(SUM(completed_at >= ? AND completed_at < ? AND exact_completed_at IS NULL), 0) AS fallback_completion_sample_count,
		(SELECT COUNT(*) FROM audit_records WHERE created_at >= ? AND created_at < ? AND action IN ('approve', 'reject')) AS week_audit_decisions,
		(SELECT COUNT(*) FROM audit_records WHERE created_at >= ? AND created_at < ? AND action = 'reject') AS week_audit_rejected
	  FROM task_facts`
	args := []interface{}{
		todayStart,
		todayStart, tomorrowStart,
		todayStart, tomorrowStart,
		todayStart, tomorrowStart,
		weekStart, tomorrowStart,
		weekStart, tomorrowStart,
		weekStart, tomorrowStart,
		weekStart, tomorrowStart,
		weekStart, tomorrowStart,
		weekStart, tomorrowStart,
		weekStart, tomorrowStart,
		weekStart, tomorrowStart,
		weekStart, tomorrowStart,
	}
	queryCtx, cancel := mysqlReadQueryContext(ctx)
	defer cancel()
	row := r.db.db.QueryRowContext(queryCtx, query, args...)
	counts := &overview.Counts
	kpis := &overview.KPIs
	if err := row.Scan(
		&counts.TotalTasks,
		&counts.ActiveTasks,
		&counts.DesignPending,
		&counts.PendingAudit,
		&counts.Handover,
		&counts.CustomizationInProgress,
		&counts.Overdue,
		&counts.DueToday,
		&counts.TodayCreated,
		&counts.TodayCompleted,
		&kpis.WeekCreated,
		&kpis.WeekCreatedCompleted,
		&kpis.WeekCompleted,
		&kpis.AverageProcessingHours,
		&kpis.AverageProcessingSampleCount,
		&kpis.ExactCompletionSampleCount,
		&kpis.FallbackCompletionSampleCount,
		&kpis.WeekAuditDecisions,
		&kpis.WeekAuditRejected,
	); err != nil {
		return fmt.Errorf("task operational overview counts: %w", err)
	}
	if kpis.WeekCreated > 0 {
		kpis.WeekCompletionRate = float64(kpis.WeekCreatedCompleted) * 100 / float64(kpis.WeekCreated)
	}
	if kpis.WeekAuditDecisions > 0 {
		kpis.WeekRejectRate = float64(kpis.WeekAuditRejected) * 100 / float64(kpis.WeekAuditDecisions)
	}
	if kpis.AverageProcessingSampleCount > 0 {
		kpis.CompletionEventCoverageRate = float64(kpis.ExactCompletionSampleCount) * 100 / float64(kpis.AverageProcessingSampleCount)
	}
	return nil
}

func (r *taskOperationalDashboardRepo) loadTaskOperationalTrend(
	ctx context.Context,
	overview *domain.TaskOperationalOverview,
	trendStart, tomorrowStart time.Time,
	location *time.Location,
) error {
	query := taskOperationalCompletionCTE + `
	SELECT day,
	       SUM(kind = 'created') AS created_count,
	       SUM(kind = 'completed') AS completed_count,
	       SUM(kind = 'due') AS due_count
	  FROM (
		SELECT DATE_FORMAT(DATE_ADD(created_at, INTERVAL 8 HOUR), '%Y-%m-%d') AS day, 'created' AS kind
		  FROM task_facts WHERE created_at >= ? AND created_at < ?
		UNION ALL
		SELECT DATE_FORMAT(DATE_ADD(completed_at, INTERVAL 8 HOUR), '%Y-%m-%d') AS day, 'completed' AS kind
		  FROM task_facts WHERE completed_at >= ? AND completed_at < ?
		UNION ALL
		SELECT DATE_FORMAT(DATE_ADD(deadline_at, INTERVAL 8 HOUR), '%Y-%m-%d') AS day, 'due' AS kind
		  FROM task_facts WHERE deadline_at >= ? AND deadline_at < ?
	  ) facts
	 GROUP BY day
	 ORDER BY day ASC`
	queryCtx, cancel := mysqlReadQueryContext(ctx)
	rows, err := r.db.db.QueryContext(queryCtx, query,
		trendStart, tomorrowStart,
		trendStart, tomorrowStart,
		trendStart, tomorrowStart,
	)
	if err != nil {
		cancel()
		return fmt.Errorf("task operational overview trend: %w", err)
	}
	defer cancel()
	defer rows.Close()
	byDate := make(map[string]domain.TaskOperationalTrendPoint, 7)
	for rows.Next() {
		var point domain.TaskOperationalTrendPoint
		if err := rows.Scan(&point.Date, &point.Created, &point.Completed, &point.Due); err != nil {
			return fmt.Errorf("scan task operational overview trend: %w", err)
		}
		byDate[point.Date] = point
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("task operational overview trend rows: %w", err)
	}
	localStart := trendStart.In(location)
	for day := 0; day < 7; day++ {
		date := localStart.AddDate(0, 0, day).Format("2006-01-02")
		point := byDate[date]
		point.Date = date
		overview.Trend = append(overview.Trend, point)
	}
	return nil
}

func (r *taskOperationalDashboardRepo) loadTaskOperationalDistribution(ctx context.Context, overview *domain.TaskOperationalOverview) error {
	query := `
	SELECT bucket, COUNT(*)
	  FROM (
		SELECT CASE
			WHEN task_status IN ('Completed', 'Archived', 'Cancelled') THEN 'completed'
			WHEN task_status = 'PendingAudit' AND task_type <> 'retouch_task' THEN 'audit'
			WHEN task_status = 'Blocked' THEN 'blocked'
			WHEN COALESCE(business_lane, '') = 'customization' OR customization_required = 1 THEN 'customization'
			ELSE 'design_ops'
		END AS bucket
		  FROM tasks
	  ) classified
	 GROUP BY bucket`
	queryCtx, cancel := mysqlReadQueryContext(ctx)
	rows, err := r.db.db.QueryContext(queryCtx, query)
	if err != nil {
		cancel()
		return fmt.Errorf("task operational overview distribution: %w", err)
	}
	defer cancel()
	defer rows.Close()
	counts := make(map[string]int64, 5)
	for rows.Next() {
		var key string
		var count int64
		if err := rows.Scan(&key, &count); err != nil {
			return fmt.Errorf("scan task operational overview distribution: %w", err)
		}
		counts[key] = count
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("task operational overview distribution rows: %w", err)
	}
	definitions := []struct{ key, name string }{
		{key: "design_ops", name: "设计/运营待推进"},
		{key: "audit", name: "待审核"},
		{key: "customization", name: "定制协同"},
		{key: "blocked", name: "异常待处理"},
		{key: "completed", name: "已完成/终止"},
	}
	overview.StatusDistribution = make([]domain.TaskOperationalStatusBucket, 0, len(definitions))
	for _, definition := range definitions {
		overview.StatusDistribution = append(overview.StatusDistribution, domain.TaskOperationalStatusBucket{
			Key: definition.key, Name: definition.name, Count: counts[definition.key],
		})
	}
	return nil
}

func (r *taskOperationalDashboardRepo) loadTaskOperationalRecentEvents(ctx context.Context, overview *domain.TaskOperationalOverview) error {
	query := `
	SELECT tel.id, tel.event_type, tel.task_id, t.task_no,
	       COALESCE(NULLIF(operator.display_name, ''), NULLIF(operator.username, ''), NULLIF(creator.display_name, ''), NULLIF(creator.username, ''), '系统') AS actor_name,
	       tel.created_at
	  FROM task_event_logs tel
	  JOIN tasks t ON t.id = tel.task_id
	  LEFT JOIN users operator ON operator.id = tel.operator_id
	  LEFT JOIN users creator ON creator.id = t.creator_id
	 WHERE tel.event_type IN (
		'task.created', 'task.assigned', 'task.reassigned', 'task.design.submitted',
		'task.audit.approved', 'task.audit.rejected', 'task.audit.handed_over',
		'task.customization.reviewed', 'task.closed'
	 )
	 ORDER BY tel.created_at DESC, tel.sequence DESC
	 LIMIT 20`
	queryCtx, cancel := mysqlReadQueryContext(ctx)
	rows, err := r.db.db.QueryContext(queryCtx, query)
	if err != nil {
		cancel()
		return fmt.Errorf("task operational overview recent events: %w", err)
	}
	defer cancel()
	defer rows.Close()
	for rows.Next() {
		var event domain.TaskOperationalEvent
		if err := rows.Scan(&event.ID, &event.EventType, &event.TaskID, &event.TaskNo, &event.ActorName, &event.CreatedAt); err != nil {
			return fmt.Errorf("scan task operational overview recent event: %w", err)
		}
		event.Title = taskOperationalEventTitle(event.EventType)
		overview.RecentEvents = append(overview.RecentEvents, event)
	}
	return rows.Err()
}

func taskOperationalEventTitle(eventType string) string {
	switch eventType {
	case domain.TaskEventCreated:
		return "新建任务"
	case domain.TaskEventAssigned:
		return "任务指派"
	case domain.TaskEventReassigned:
		return "重新指派"
	case domain.TaskEventDesignSubmitted:
		return "提交设计"
	case domain.TaskEventAuditApproved:
		return "审核通过"
	case domain.TaskEventAuditReturnedToDesign:
		return "审核打回"
	case domain.TaskEventAuditHandedOver:
		return "审核交班"
	case "task.customization.reviewed":
		return "定制复核"
	case domain.TaskEventClosed:
		return "任务结单"
	default:
		return "任务动态"
	}
}
