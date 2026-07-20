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
	where, args := appendResourceGroupAccessScope([]string{
		"tel.created_at >= ?", "tel.created_at < ?",
		`tel.event_type IN ('task.created','task.assigned','task.reassigned','task.design.submitted','task.audit.approved','task.audit.rejected','task.closed')`,
	}, []interface{}{from, to}, access)
	args = append(args, normalizeAIAnalysisLimit(limit, 80))
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT tel.id, t.id, COALESCE(t.task_no,''), COALESCE(t.sku_code,''),
		       COALESCE(t.product_name_snapshot,''), tel.event_type,
		       COALESCE(NULLIF(u.display_name,''),u.username,''), tel.created_at,
		       COALESCE(CAST(t.workflow_revision AS CHAR),'0')
		FROM task_event_logs tel
		JOIN tasks t ON t.id=tel.task_id
		LEFT JOIN users u ON u.id=tel.operator_id
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY tel.created_at DESC, tel.id DESC LIMIT ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("list scoped KPI evidence: %w", err)
	}
	defer rows.Close()
	hits := make([]domain.AIRetrievalHit, 0, limit)
	for rows.Next() {
		var eventID, eventType, taskNo, sku, product, operator, version string
		var taskID int64
		var createdAt time.Time
		if err := rows.Scan(&eventID, &taskID, &taskNo, &sku, &product, &eventType, &operator, &createdAt, &version); err != nil {
			return nil, fmt.Errorf("scan scoped KPI evidence: %w", err)
		}
		title := strings.TrimSpace(strings.Join([]string{taskNo, sku, product}, " "))
		excerpt := fmt.Sprintf("%s，事件：%s，操作人：%s，发生时间：%s", title, eventType, operator, createdAt.Format("2006-01-02 15:04"))
		hits = append(hits, analysisHit("kpi:event:"+eventID, "task_kpi", fmt.Sprint(taskID), title, fmt.Sprintf("/tasks/%d", taskID), excerpt, version))
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
