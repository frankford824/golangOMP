package aichat

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"workflow/domain"
	"workflow/repo"
	"workflow/service/aiagent"
)

const maxAnalysisTools = 3

type AnalysisPlan struct {
	Tools []AnalysisToolCall `json:"tools"`
}

type AnalysisToolCall struct {
	Name     string `json:"name"`
	Query    string `json:"query,omitempty"`
	EntityID string `json:"entity_id,omitempty"`
	From     string `json:"from,omitempty"`
	To       string `json:"to,omitempty"`
}

type ToolOrchestrator struct {
	provider  aiagent.ChatProvider
	retriever EvidenceRetriever
	analytics repo.AIAnalysisRepo
}

func NewToolOrchestrator(provider aiagent.ChatProvider, retriever EvidenceRetriever, analytics ...repo.AIAnalysisRepo) *ToolOrchestrator {
	o := &ToolOrchestrator{provider: provider, retriever: retriever}
	if len(analytics) > 0 {
		o.analytics = analytics[0]
	}
	return o
}

func (o *ToolOrchestrator) Gather(ctx context.Context, actor domain.RequestActor, question string, limit int) ([]domain.AIRetrievalHit, domain.AIRetrievalMeta, error) {
	if o == nil || o.retriever == nil {
		return nil, domain.AIRetrievalMeta{}, fmt.Errorf("analysis retrieval is unavailable")
	}
	plan, err := o.plan(ctx, question)
	if err != nil || len(plan.Tools) == 0 {
		hits, meta, searchErr := o.retriever.Search(ctx, actor, question, limit)
		if searchErr == nil {
			meta.Reason = "planner_fallback"
		}
		return hits, meta, searchErr
	}
	type result struct {
		hits []domain.AIRetrievalHit
		meta domain.AIRetrievalMeta
		err  error
	}
	results := make([]result, len(plan.Tools))
	var wg sync.WaitGroup
	for index := range plan.Tools {
		index := index
		wg.Add(1)
		go func() {
			defer wg.Done()
			call := plan.Tools[index]
			query := strings.TrimSpace(call.Query)
			if query == "" {
				query = question
			}
			hits, meta, searchErr := o.execute(ctx, actor, call, query, max(limit, 20))
			results[index] = result{hits: hits, meta: meta, err: searchErr}
		}()
	}
	wg.Wait()
	merged := make(map[string]domain.AIRetrievalHit)
	meta := domain.AIRetrievalMeta{Mode: "hybrid"}
	succeeded := 0
	for _, item := range results {
		if item.err != nil {
			meta.Degraded = true
			meta.Reason = "tool_partial_failure"
			continue
		}
		succeeded++
		meta.Candidates += item.meta.Candidates
		meta.Degraded = meta.Degraded || item.meta.Degraded
		for _, hit := range item.hits {
			if existing, exists := merged[hit.DocumentID]; !exists || hit.Score > existing.Score {
				merged[hit.DocumentID] = hit
			}
		}
	}
	if succeeded == 0 {
		hits, fallbackMeta, fallbackErr := o.retriever.Search(ctx, actor, question, limit)
		if fallbackErr != nil {
			return nil, meta, fallbackErr
		}
		fallbackMeta.Degraded = true
		fallbackMeta.Reason = "tool_fallback"
		return hits, fallbackMeta, nil
	}
	hits := make([]domain.AIRetrievalHit, 0, len(merged))
	for _, hit := range merged {
		hits = append(hits, hit)
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits, meta, nil
}

func (o *ToolOrchestrator) execute(ctx context.Context, actor domain.RequestActor, call AnalysisToolCall, query string, limit int) ([]domain.AIRetrievalHit, domain.AIRetrievalMeta, error) {
	if call.Name == "task_detail" || call.Name == "resource_group_detail" {
		if strings.TrimSpace(call.EntityID) == "" {
			hits, meta, err := o.retriever.Search(ctx, actor, query, limit)
			if err == nil {
				hits = filterToolHits(call.Name, "", hits)
			}
			return hits, meta, err
		}
		if o.analytics == nil {
			return nil, domain.AIRetrievalMeta{}, fmt.Errorf("analysis data source is unavailable")
		}
		entityID, err := strconv.ParseInt(strings.TrimSpace(call.EntityID), 10, 64)
		if err != nil || entityID <= 0 {
			return nil, domain.AIRetrievalMeta{}, fmt.Errorf("analysis tool %q entity_id is invalid", call.Name)
		}
		permission := domain.PermissionTaskView
		if call.Name == "resource_group_detail" {
			permission = domain.PermissionAssetView
		}
		if !domain.ActorHasPermission(actor, permission) {
			return []domain.AIRetrievalHit{}, domain.AIRetrievalMeta{Mode: "exact", Reason: "scope_denied"}, nil
		}
		access := domain.ResourceGroupAccessFilterForActor(actor, permission)
		var hits []domain.AIRetrievalHit
		if call.Name == "task_detail" {
			hits, err = o.analytics.GetTaskDetailEvidence(ctx, access, entityID)
		} else {
			hits, err = o.analytics.GetResourceGroupDetailEvidence(ctx, access, entityID)
		}
		return hits, domain.AIRetrievalMeta{Mode: "exact", Candidates: len(hits)}, err
	}
	if call.Name == "task_kpi" || call.Name == "business_trends" || call.Name == "experience_summary" {
		if o.analytics == nil {
			return nil, domain.AIRetrievalMeta{}, fmt.Errorf("analysis data source is unavailable")
		}
		access := domain.ResourceGroupAccessFilterForActor(actor, domain.PermissionTaskView)
		if !domain.ActorHasPermission(actor, domain.PermissionTaskView) {
			return []domain.AIRetrievalHit{}, domain.AIRetrievalMeta{Mode: "exact", Reason: "task_scope_denied"}, nil
		}
		from, to := analysisDateRange(call, time.Now().UTC())
		var hits []domain.AIRetrievalHit
		var err error
		switch call.Name {
		case "task_kpi":
			hits, err = o.analytics.ListKPIEvidence(ctx, access, from, to, limit)
		case "business_trends":
			hits, err = o.analytics.ListBusinessTrendEvidence(ctx, access, from, to, limit)
		case "experience_summary":
			hits, err = o.analytics.ListExperienceEvidence(ctx, access, from, to, limit)
		}
		return hits, domain.AIRetrievalMeta{Mode: "exact", Candidates: len(hits)}, err
	}
	hits, meta, err := o.retriever.Search(ctx, actor, query, limit)
	if err == nil {
		hits = filterToolHits(call.Name, call.EntityID, hits)
	}
	return hits, meta, err
}

func analysisDateRange(call AnalysisToolCall, now time.Time) (time.Time, time.Time) {
	if call.From != "" && call.To != "" {
		from, _ := time.Parse("2006-01-02", call.From)
		to, _ := time.Parse("2006-01-02", call.To)
		return from.UTC(), to.AddDate(0, 0, 1).UTC()
	}
	return now.AddDate(0, 0, -30), now.AddDate(0, 0, 1)
}

func (o *ToolOrchestrator) plan(ctx context.Context, question string) (AnalysisPlan, error) {
	if o.provider == nil || !o.provider.Ready() {
		return AnalysisPlan{}, fmt.Errorf("analysis planner is unavailable")
	}
	text, _, err := o.provider.CompleteText(ctx, aiagent.ChatRequest{
		Scene: "data_center_tool_plan",
		System: `你是只读数据分析规划器。仅返回 JSON，不要解释。格式：{"tools":[{"name":"global_search","query":"..."}]}。
最多 3 个工具。允许：global_search、task_detail、resource_group_detail、task_kpi、business_trends、experience_summary。
禁止 SQL、写入、上传、发布或改变状态。`,
		Messages:  []aiagent.ChatMessage{{Role: "user", Content: truncateRunes(question, 4000)}},
		MaxTokens: 500, Temperature: 0,
	})
	if err != nil {
		return AnalysisPlan{}, err
	}
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	var plan AnalysisPlan
	if err := json.Unmarshal([]byte(strings.TrimSpace(text)), &plan); err != nil {
		return AnalysisPlan{}, fmt.Errorf("decode analysis plan: %w", err)
	}
	if err := validateAnalysisPlan(&plan); err != nil {
		return AnalysisPlan{}, err
	}
	return plan, nil
}

func validateAnalysisPlan(plan *AnalysisPlan) error {
	if plan == nil || len(plan.Tools) == 0 || len(plan.Tools) > maxAnalysisTools {
		return fmt.Errorf("analysis plan must contain 1-%d tools", maxAnalysisTools)
	}
	allowed := map[string]bool{
		"global_search": true, "task_detail": true, "resource_group_detail": true,
		"task_kpi": true, "business_trends": true, "experience_summary": true,
	}
	for index := range plan.Tools {
		call := &plan.Tools[index]
		call.Name = strings.TrimSpace(call.Name)
		call.Query = truncateRunes(strings.TrimSpace(call.Query), 1000)
		call.EntityID = truncateRunes(strings.TrimSpace(call.EntityID), 128)
		if !allowed[call.Name] {
			return fmt.Errorf("analysis tool %q is not allowed", call.Name)
		}
		if call.Name == "task_detail" || call.Name == "resource_group_detail" {
			if call.EntityID == "" && call.Query == "" {
				return fmt.Errorf("analysis tool %q requires entity_id or query", call.Name)
			}
		}
		if call.From != "" || call.To != "" {
			from, fromErr := time.Parse("2006-01-02", call.From)
			to, toErr := time.Parse("2006-01-02", call.To)
			if fromErr != nil || toErr != nil || to.Before(from) || to.Sub(from) > 366*24*time.Hour {
				return fmt.Errorf("analysis tool date range is invalid")
			}
		}
	}
	return nil
}

func filterToolHits(tool, entityID string, hits []domain.AIRetrievalHit) []domain.AIRetrievalHit {
	wanted := ""
	switch tool {
	case "task_detail":
		wanted = "task"
	case "resource_group_detail":
		wanted = "task_resource_group"
	case "business_trends":
		wanted = "business_trend"
	case "experience_summary":
		wanted = "experience_summary"
	}
	out := make([]domain.AIRetrievalHit, 0, len(hits))
	for _, hit := range hits {
		if wanted != "" && hit.EntityType != wanted {
			continue
		}
		if strings.TrimSpace(entityID) != "" && hit.EntityID != strings.TrimSpace(entityID) {
			continue
		}
		out = append(out, hit)
	}
	return out
}
