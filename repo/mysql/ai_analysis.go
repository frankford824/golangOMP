package mysqlrepo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type aiAnalysisRepo struct{ db *DB }

func NewAIAnalysisRepo(db *DB) repo.AIAnalysisRepo { return &aiAnalysisRepo{db: db} }

func (r *aiAnalysisRepo) GetTaskDetailEvidence(ctx context.Context, access domain.ResourceGroupAccessFilter, taskID int64) ([]domain.AIRetrievalHit, error) {
	where, args := appendResourceGroupAccessScope([]string{"t.id = ?"}, []interface{}{taskID}, access)
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT t.id, COALESCE(d.task_no,''), COALESCE(d.sku_code,''), COALESCE(d.product_name_snapshot,''),
		       COALESCE(d.search_text,''), COALESCE(CAST(t.workflow_revision AS CHAR),'0')
		FROM tasks t
		JOIN task_search_documents d ON d.task_id=t.id
		WHERE `+strings.Join(where, " AND ")+`
		LIMIT 1`, args...)
	if err != nil {
		return nil, fmt.Errorf("get scoped task detail evidence: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		return []domain.AIRetrievalHit{}, rows.Err()
	}
	var id int64
	var taskNo, sku, product, excerpt, version string
	if err := rows.Scan(&id, &taskNo, &sku, &product, &excerpt, &version); err != nil {
		return nil, fmt.Errorf("scan scoped task detail evidence: %w", err)
	}
	title := compactAnalysisText([]string{taskNo, sku, product}, 255)
	return []domain.AIRetrievalHit{analysisHit(fmt.Sprintf("task:%d", id), "task", fmt.Sprint(id), title, fmt.Sprintf("/tasks/%d", id), compactAnalysisText([]string{excerpt}, 1800), version)}, nil
}

func (r *aiAnalysisRepo) GetResourceGroupDetailEvidence(ctx context.Context, access domain.ResourceGroupAccessFilter, groupID int64) ([]domain.AIRetrievalHit, error) {
	where, args := appendResourceGroupAccessScope([]string{"g.id = ?", "g.finalized_revision_id IS NOT NULL"}, []interface{}{groupID}, access)
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT g.id, COALESCE(t.task_no,''), COALESCE(tsi.sku_code,''), COALESCE(d.internal_text,''),
		       COALESCE(CAST(g.finalized_revision_id AS CHAR),'0')
		FROM task_asset_groups g
		JOIN tasks t ON t.id=g.task_id
		JOIN task_asset_group_search_documents d ON d.group_id=g.id AND d.finalized_revision_id=g.finalized_revision_id
		LEFT JOIN task_sku_items tsi ON tsi.id=g.task_sku_item_id
		WHERE `+strings.Join(where, " AND ")+`
		LIMIT 1`, args...)
	if err != nil {
		return nil, fmt.Errorf("get scoped resource group evidence: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		return []domain.AIRetrievalHit{}, rows.Err()
	}
	var id int64
	var taskNo, sku, excerpt, version string
	if err := rows.Scan(&id, &taskNo, &sku, &excerpt, &version); err != nil {
		return nil, fmt.Errorf("scan scoped resource group evidence: %w", err)
	}
	title := compactAnalysisText([]string{taskNo, sku, "资源组"}, 255)
	return []domain.AIRetrievalHit{analysisHit(fmt.Sprintf("task_resource_group:%d", id), "task_resource_group", fmt.Sprint(id), title, fmt.Sprintf("/asset-center/%d", id), compactAnalysisText([]string{excerpt}, 1800), version)}, nil
}

func (r *aiAnalysisRepo) ListKPIEvidence(ctx context.Context, access domain.ResourceGroupAccessFilter, from, to time.Time, limit int) ([]domain.AIRetrievalHit, error) {
	limit = normalizeAIAnalysisLimit(limit, 20)
	designWhere, designArgs := appendResourceGroupAccessScope([]string{
		"tel.created_at >= ?", "tel.created_at < ?",
		`tel.event_type IN ('task.design_submitted','task.design.submitted')`,
	}, []interface{}{from, to}, access)
	retouchWhere, retouchArgs := appendResourceGroupAccessScope([]string{
		"revision.source_stage = 'retouch'", "revision.finalized_at >= ?", "revision.finalized_at < ?",
	}, []interface{}{from, to}, access)
	linkedCTE := `
		WITH design_events AS (
			SELECT tel.id AS event_id, DATE(tel.created_at) AS day, tel.task_id,
			       COALESCE(tel.operator_id, t.designer_id) AS person_id, tel.created_at
			  FROM task_event_logs tel
			  JOIN tasks t ON t.id = tel.task_id
			 WHERE ` + strings.Join(designWhere, " AND ") + `
		), linked AS (
			SELECT event.event_id, event.day, event.task_id, event.person_id,
			       g.id AS group_id, revision.id AS revision_id, revision.mode,
			       (SELECT COUNT(*) FROM task_asset_group_revision_items finalized_item
			         WHERE finalized_item.revision_id = g.finalized_revision_id) AS final_items
			  FROM design_events event
			  JOIN task_asset_groups g ON g.task_id = event.task_id
			  JOIN task_asset_group_revisions revision
			    ON revision.group_id = g.id AND revision.source_stage = 'design'
			   AND ABS(TIMESTAMPDIFF(SECOND, revision.submitted_at, event.created_at)) <= 5
		), set_average AS (
			SELECT COALESCE(AVG(CASE WHEN mode = 'set' AND final_items > 0 THEN final_items END), 2) AS value
			  FROM linked
		)`

	type kpiSummary struct {
		uniqueTasks, regularTasks, retouchTasks, submissions, designUnits int64
		exactGroups, fallbackSingles, fallbackSets                        int64
		averageSetImages                                                  float64
		minimumImages, estimatedImages, linkedEvents                      int64
	}
	var summary kpiSummary
	summaryQuery := linkedCTE + `,
		retouch AS (
			SELECT DISTINCT g.task_id, revision.id AS revision_id, item.id AS item_id
			  FROM task_asset_group_revisions revision
			  JOIN task_asset_groups g ON g.id = revision.group_id
			  JOIN tasks t ON t.id = g.task_id
			  LEFT JOIN task_asset_group_revision_items item ON item.revision_id = revision.id
			 WHERE ` + strings.Join(retouchWhere, " AND ") + `
		), all_tasks AS (
			SELECT DISTINCT task_id FROM design_events UNION SELECT DISTINCT task_id FROM retouch
		)
		SELECT (SELECT COUNT(*) FROM all_tasks),
		       (SELECT COUNT(DISTINCT task_id) FROM design_events),
		       (SELECT COUNT(DISTINCT task_id) FROM retouch),
		       (SELECT COUNT(DISTINCT event_id) FROM design_events),
		       COUNT(linked.revision_id) + (SELECT COUNT(DISTINCT revision_id) FROM retouch),
		       COALESCE(SUM(linked.final_items > 0), 0),
		       COALESCE(SUM(linked.final_items = 0 AND linked.mode = 'single'), 0),
		       COALESCE(SUM(linked.final_items = 0 AND linked.mode = 'set'), 0),
		       COALESCE((SELECT value FROM set_average), 2),
		       COALESCE(SUM(CASE WHEN linked.final_items > 0 THEN linked.final_items WHEN linked.mode = 'single' THEN 1 ELSE 2 END), 0)
		         + (SELECT COUNT(item_id) FROM retouch),
		       ROUND(COALESCE(SUM(CASE WHEN linked.final_items > 0 THEN linked.final_items WHEN linked.mode = 'single' THEN 1 ELSE (SELECT value FROM set_average) END), 0)
		         + (SELECT COUNT(item_id) FROM retouch)),
		       COUNT(DISTINCT linked.event_id)
		  FROM linked`
	summaryArgs := append(append([]interface{}{}, designArgs...), retouchArgs...)
	queryCtx, cancel := mysqlReadQueryContext(ctx)
	err := r.db.db.QueryRowContext(queryCtx, summaryQuery, summaryArgs...).Scan(
		&summary.uniqueTasks, &summary.regularTasks, &summary.retouchTasks, &summary.submissions,
		&summary.designUnits, &summary.exactGroups, &summary.fallbackSingles, &summary.fallbackSets,
		&summary.averageSetImages, &summary.minimumImages, &summary.estimatedImages, &summary.linkedEvents,
	)
	cancel()
	if err != nil {
		return nil, fmt.Errorf("get scoped KPI summary evidence: %w", err)
	}

	type dailyPoint struct {
		day                                            string
		submissions, regularTasks, retouchTasks, tasks int64
		designUnits, minimumImages, estimatedImages    int64
	}
	dailyQuery := linkedCTE + `,
		regular_daily AS (
			SELECT event.day, COUNT(DISTINCT event.event_id) AS submissions,
			       COUNT(DISTINCT event.task_id) AS task_count, COUNT(linked.revision_id) AS unit_count,
			       SUM(CASE WHEN linked.final_items > 0 THEN linked.final_items WHEN linked.mode = 'single' THEN 1 ELSE 2 END) AS minimum_images,
			       SUM(CASE WHEN linked.final_items > 0 THEN linked.final_items WHEN linked.mode = 'single' THEN 1 ELSE (SELECT value FROM set_average) END) AS estimated_images
			  FROM design_events event LEFT JOIN linked ON linked.event_id = event.event_id
			 GROUP BY event.day
		), retouch_daily AS (
			SELECT DATE(revision.finalized_at) AS day, COUNT(DISTINCT g.task_id) AS task_count,
			       COUNT(DISTINCT revision.id) AS unit_count, COUNT(item.id) AS image_count
			  FROM task_asset_group_revisions revision
			  JOIN task_asset_groups g ON g.id = revision.group_id
			  JOIN tasks t ON t.id = g.task_id
			  LEFT JOIN task_asset_group_revision_items item ON item.revision_id = revision.id
			 WHERE ` + strings.Join(retouchWhere, " AND ") + ` GROUP BY DATE(revision.finalized_at)
		), days AS (SELECT day FROM regular_daily UNION SELECT day FROM retouch_daily)
		SELECT DATE_FORMAT(days.day, '%Y-%m-%d'), COALESCE(regular_daily.submissions, 0),
		       COALESCE(regular_daily.task_count, 0), COALESCE(retouch_daily.task_count, 0),
		       COALESCE(regular_daily.task_count, 0) + COALESCE(retouch_daily.task_count, 0),
		       COALESCE(regular_daily.unit_count, 0) + COALESCE(retouch_daily.unit_count, 0),
		       COALESCE(regular_daily.minimum_images, 0) + COALESCE(retouch_daily.image_count, 0),
		       ROUND(COALESCE(regular_daily.estimated_images, 0) + COALESCE(retouch_daily.image_count, 0))
		  FROM days LEFT JOIN regular_daily ON regular_daily.day = days.day
		  LEFT JOIN retouch_daily ON retouch_daily.day = days.day ORDER BY days.day`
	dailyArgs := append(append([]interface{}{}, designArgs...), retouchArgs...)
	queryCtx, cancel = mysqlReadQueryContext(ctx)
	rows, err := r.db.db.QueryContext(queryCtx, dailyQuery, dailyArgs...)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("list scoped KPI daily evidence: %w", err)
	}
	daily := make([]dailyPoint, 0, 31)
	for rows.Next() {
		var point dailyPoint
		if err := rows.Scan(&point.day, &point.submissions, &point.regularTasks, &point.retouchTasks,
			&point.tasks, &point.designUnits, &point.minimumImages, &point.estimatedImages); err != nil {
			rows.Close()
			cancel()
			return nil, fmt.Errorf("scan scoped KPI daily evidence: %w", err)
		}
		daily = append(daily, point)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		cancel()
		return nil, fmt.Errorf("list scoped KPI daily rows: %w", err)
	}
	rows.Close()
	cancel()
	byDate := make(map[string]dailyPoint, len(daily))
	for _, point := range daily {
		byDate[point.day] = point
	}
	daily = daily[:0]
	for day := from; day.Before(to); day = day.AddDate(0, 0, 1) {
		key := day.Format("2006-01-02")
		point := byDate[key]
		point.day = key
		daily = append(daily, point)
	}

	coverage := float64(0)
	if summary.submissions > 0 {
		coverage = float64(summary.linkedEvents) * 100 / float64(summary.submissions)
	}
	dailyTasks := int64(0)
	dailyLines := make([]string, 0, len(daily))
	for _, point := range daily {
		dailyTasks += point.tasks
		dailyLines = append(dailyLines, fmt.Sprintf("%s：任务%d（常规%d/精修%d），约图%d，下限%d",
			point.day, point.tasks, point.regularTasks, point.retouchTasks, point.estimatedImages, point.minimumImages))
	}
	days := max(1, int(to.Sub(from).Hours()/24))
	period := from.Format("2006-01-02") + " 至 " + to.Add(-time.Nanosecond).Format("2006-01-02")
	summaryExcerpt := fmt.Sprintf(
		"口径：常规/定制按 task.design_submitted（兼容旧 task.design.submitted）及5秒内对应设计资源修订；精修按最终修订。区间：%s。不重复任务%d，按日工作量%d，常规%d，精修%d，设计提交%d，设计单元%d，约图%d（保守下限%d），日均任务%.1f，日均图%.1f。提交资源关联覆盖率%.1f%%；实际成品组%d，单图规则补足%d，待审核套装估算%d（已完成套装平均%.1f张）。",
		period, summary.uniqueTasks, dailyTasks, summary.regularTasks, summary.retouchTasks,
		summary.submissions, summary.designUnits, summary.estimatedImages, summary.minimumImages,
		float64(dailyTasks)/float64(days), float64(summary.estimatedImages)/float64(days), coverage,
		summary.exactGroups, summary.fallbackSingles, summary.fallbackSets, summary.averageSetImages,
	)
	if len(dailyLines) > 0 {
		summaryExcerpt += "每日：" + strings.Join(dailyLines, "；")
	}
	hits := []domain.AIRetrievalHit{analysisHit(
		"kpi:design-productivity:"+from.Format("20060102")+":"+to.Format("20060102"),
		"task_kpi", period, "设计产能统计 "+period, "/data-center",
		compactAnalysisText([]string{summaryExcerpt}, 3600), fmt.Sprintf("%d:%d:%d", summary.submissions, summary.designUnits, summary.estimatedImages),
	)}
	if limit <= 1 {
		return hits, nil
	}

	personQuery := linkedCTE + `,
		regular_person AS (
			SELECT person_id, COUNT(DISTINCT event_id) AS submissions, COUNT(DISTINCT task_id) AS task_count,
			       COUNT(revision_id) AS unit_count,
			       SUM(CASE WHEN final_items > 0 THEN final_items WHEN mode = 'single' THEN 1 ELSE 2 END) AS minimum_images,
			       SUM(CASE WHEN final_items > 0 THEN final_items WHEN mode = 'single' THEN 1 ELSE (SELECT value FROM set_average) END) AS estimated_images
			  FROM linked GROUP BY person_id
		), retouch_person AS (
			SELECT revision.created_by AS person_id, COUNT(DISTINCT g.task_id) AS task_count,
			       COUNT(DISTINCT revision.id) AS unit_count, COUNT(item.id) AS image_count
			  FROM task_asset_group_revisions revision
			  JOIN task_asset_groups g ON g.id = revision.group_id
			  JOIN tasks t ON t.id = g.task_id
			  LEFT JOIN task_asset_group_revision_items item ON item.revision_id = revision.id
			 WHERE ` + strings.Join(retouchWhere, " AND ") + ` GROUP BY revision.created_by
		), people AS (SELECT person_id FROM regular_person UNION SELECT person_id FROM retouch_person)
		SELECT COALESCE(people.person_id, 0),
		       COALESCE(NULLIF(users.display_name, ''), NULLIF(users.username, ''), CONCAT('人员#', people.person_id), '未识别人员'),
		       COALESCE(users.department, ''), COALESCE(users.team, ''),
		       COALESCE(regular_person.task_count, 0), COALESCE(retouch_person.task_count, 0),
		       COALESCE(regular_person.task_count, 0) + COALESCE(retouch_person.task_count, 0),
		       COALESCE(regular_person.submissions, 0),
		       COALESCE(regular_person.unit_count, 0) + COALESCE(retouch_person.unit_count, 0),
		       COALESCE(regular_person.minimum_images, 0) + COALESCE(retouch_person.image_count, 0),
		       ROUND(COALESCE(regular_person.estimated_images, 0) + COALESCE(retouch_person.image_count, 0))
		  FROM people LEFT JOIN users ON users.id = people.person_id
		  LEFT JOIN regular_person ON regular_person.person_id = people.person_id
		  LEFT JOIN retouch_person ON retouch_person.person_id = people.person_id
		 ORDER BY 11 DESC, 7 DESC, 2 ASC LIMIT ?`
	personArgs := append(append([]interface{}{}, designArgs...), retouchArgs...)
	personArgs = append(personArgs, limit-1)
	queryCtx, cancel = mysqlReadQueryContext(ctx)
	rows, err = r.db.db.QueryContext(queryCtx, personQuery, personArgs...)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("list scoped KPI people evidence: %w", err)
	}
	defer cancel()
	defer rows.Close()
	for rows.Next() {
		var userID, regularTasks, retouchTasks, taskCount, submissions, units, minimumImages, estimatedImages int64
		var name, department, team string
		if err := rows.Scan(&userID, &name, &department, &team, &regularTasks, &retouchTasks,
			&taskCount, &submissions, &units, &minimumImages, &estimatedImages); err != nil {
			return nil, fmt.Errorf("scan scoped KPI people evidence: %w", err)
		}
		share := float64(0)
		if summary.estimatedImages > 0 {
			share = float64(estimatedImages) * 100 / float64(summary.estimatedImages)
		}
		title := fmt.Sprintf("设计人员 %s", name)
		excerpt := fmt.Sprintf("%s（%s）：任务贡献%d（常规%d/精修%d），设计提交%d，设计单元%d，约图%d（下限%d），图量占比%.1f%%",
			name, compactAnalysisText([]string{department, team}, 120), taskCount, regularTasks, retouchTasks,
			submissions, units, estimatedImages, minimumImages, share)
		hits = append(hits, analysisHit(fmt.Sprintf("kpi:designer:%d:%s:%s", userID, from.Format("20060102"), to.Format("20060102")),
			"task_kpi", fmt.Sprint(userID), title, "/data-center", excerpt, fmt.Sprintf("%d:%d:%d", taskCount, submissions, estimatedImages)))
	}
	return hits, rows.Err()
}

func (r *aiAnalysisRepo) ListBusinessTrendEvidence(ctx context.Context, access domain.ResourceGroupAccessFilter, from, to time.Time, limit int) ([]domain.AIRetrievalHit, error) {
	where, args := appendResourceGroupAccessScope([]string{"t.created_at >= ?", "t.created_at < ?"}, []interface{}{from, to}, access)
	args = append(args, normalizeAIAnalysisLimit(limit, 80))
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT t.id, COALESCE(t.task_no,''), COALESCE(t.sku_code,''), COALESCE(t.product_name_snapshot,''),
		       COALESCE(td.category_name,''), COALESCE(td.demand_text,''), COALESCE(td.copy_text,''),
		       COALESCE(td.design_requirement,''), COALESCE(td.material,''), COALESCE(td.size_text,''),
		       t.updated_at, COALESCE(CAST(t.workflow_revision AS CHAR),'0')
		FROM tasks t LEFT JOIN task_details td ON td.task_id=t.id
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY t.updated_at DESC, t.id DESC LIMIT ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("list scoped business trend evidence: %w", err)
	}
	defer rows.Close()
	hits := make([]domain.AIRetrievalHit, 0, limit)
	for rows.Next() {
		var taskID int64
		var taskNo, sku, product, category, demand, copyText, requirement, material, sizeText, version string
		var updatedAt time.Time
		if err := rows.Scan(&taskID, &taskNo, &sku, &product, &category, &demand, &copyText, &requirement, &material, &sizeText, &updatedAt, &version); err != nil {
			return nil, fmt.Errorf("scan scoped business trend evidence: %w", err)
		}
		title := strings.TrimSpace(strings.Join([]string{taskNo, sku, product}, " "))
		excerpt := compactAnalysisText([]string{"品类：" + category, "需求：" + demand, "文案：" + copyText, "设计要求：" + requirement, "材质：" + material, "尺寸：" + sizeText}, 1400)
		hits = append(hits, analysisHit(fmt.Sprintf("trend:task:%d", taskID), "business_trend", fmt.Sprint(taskID), title, fmt.Sprintf("/tasks/%d", taskID), excerpt, version))
	}
	return hits, rows.Err()
}

func (r *aiAnalysisRepo) ListExperienceEvidence(ctx context.Context, access domain.ResourceGroupAccessFilter, from, to time.Time, limit int) ([]domain.AIRetrievalHit, error) {
	where, args := appendResourceGroupAccessScope([]string{
		"e.event_time >= ?", "e.event_time < ?", "e.task_id IS NOT NULL",
	}, []interface{}{from, to}, access)
	args = append(args, normalizeAIAnalysisLimit(limit, 80))
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT e.id, e.task_id, COALESCE(t.task_no,''), e.action, e.outcome,
		       COALESCE(e.feedback_value,''), COALESCE(e.feedback_reason_code,''),
		       COALESCE(CAST(e.payload_json AS CHAR),'{}'), e.event_time,
		       COALESCE(e.source_watermark, CAST(e.id AS CHAR))
		FROM experience_events e JOIN tasks t ON t.id=e.task_id
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY e.event_time DESC, e.id DESC LIMIT ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("list scoped experience evidence: %w", err)
	}
	defer rows.Close()
	hits := make([]domain.AIRetrievalHit, 0, limit)
	for rows.Next() {
		var id, taskID int64
		var taskNo, action, outcome, feedback, reason, payload, version string
		var eventTime time.Time
		if err := rows.Scan(&id, &taskID, &taskNo, &action, &outcome, &feedback, &reason, &payload, &eventTime, &version); err != nil {
			return nil, fmt.Errorf("scan scoped experience evidence: %w", err)
		}
		excerpt := compactAnalysisText([]string{"动作：" + action, "结果：" + outcome, "反馈：" + feedback, "原因：" + reason, "证据：" + payload}, 1400)
		hits = append(hits, analysisHit(fmt.Sprintf("experience:event:%d", id), "experience_summary", fmt.Sprint(taskID), "任务 "+taskNo+" 的经验反馈", fmt.Sprintf("/tasks/%d", taskID), excerpt, version))
	}
	return hits, rows.Err()
}

func normalizeAIAnalysisLimit(limit, fallback int) int {
	if limit <= 0 {
		limit = fallback
	}
	if limit > 200 {
		limit = 200
	}
	return limit
}

func analysisHit(documentID, entityType, entityID, title, route, excerpt, version string) domain.AIRetrievalHit {
	return domain.AIRetrievalHit{DocumentID: documentID, EntityType: entityType, EntityID: entityID, Title: title,
		InternalRoute: route, Excerpt: excerpt, SourceVersion: version, Score: 1, Source: "mysql_analysis"}
}

func compactAnalysisText(parts []string, max int) string {
	kept := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || strings.HasSuffix(part, "：") {
			continue
		}
		kept = append(kept, part)
	}
	value := strings.Join(kept, "；")
	if len([]rune(value)) > max {
		value = string([]rune(value)[:max])
	}
	return value
}
