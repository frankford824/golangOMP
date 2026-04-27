package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

const (
	taskCostOverrideAuditHistorySource    = "cost_override_events"
	taskCostOverrideFallbackHistorySource = "task_event_logs.business_info_updates"
	taskGovernanceGeneralEventLayer       = "task_event_logs"
	taskCostOverrideReviewBoundarySource  = "cost_override_reviews"
	taskCostFinanceBoundarySource         = "cost_override_finance_flags"
	taskCostDerivedBoundarySource         = "derived_placeholder_boundary"
)

type costRuleLineageLoader struct {
	repo  repo.CostRuleRepo
	asOf  time.Time
	cache map[int64]*domain.CostRule
}

func newCostRuleLineageLoader(costRuleRepo repo.CostRuleRepo, asOf time.Time) *costRuleLineageLoader {
	return &costRuleLineageLoader{
		repo:  costRuleRepo,
		asOf:  asOf,
		cache: map[int64]*domain.CostRule{},
	}
}

func (l *costRuleLineageLoader) get(ctx context.Context, id int64) (*domain.CostRule, error) {
	if l == nil || l.repo == nil || id == 0 {
		return nil, nil
	}
	if cached, ok := l.cache[id]; ok {
		return cached, nil
	}
	rule, err := l.repo.GetByID(ctx, id)
	if err != nil || rule == nil {
		return rule, err
	}
	hydrateCostRuleGovernanceStatus(rule, l.asOf)
	l.cache[id] = rule
	return rule, nil
}

func decorateCostRuleLineage(ctx context.Context, loader *costRuleLineageLoader, rule *domain.CostRule) (*domain.CostRuleHistoryReadModel, error) {
	if rule == nil {
		return nil, nil
	}
	if loader == nil {
		loader = newCostRuleLineageLoader(nil, time.Now().UTC())
	}
	hydrateCostRuleGovernanceStatus(rule, loader.asOf)
	if rule.RuleID != 0 && loader.cache != nil {
		loader.cache[rule.RuleID] = rule
	}

	chain, currentIndex, err := loadCostRuleLineageChain(ctx, loader, rule)
	if err != nil {
		return nil, err
	}
	rule.SupersessionDepth = currentIndex
	rule.VersionChainSummary = buildCostRuleVersionChainSummary(chain, currentIndex)
	if currentIndex > 0 {
		rule.PreviousVersion = costRuleVersionRef(chain[currentIndex-1])
	}
	if currentIndex+1 < len(chain) {
		rule.NextVersion = costRuleVersionRef(chain[currentIndex+1])
	}

	return &domain.CostRuleHistoryReadModel{
		Rule:         rule,
		VersionChain: costRuleVersionRefs(chain),
		CurrentRule:  costRuleVersionRef(chain[len(chain)-1]),
	}, nil
}

func loadCostRuleLineageChain(ctx context.Context, loader *costRuleLineageLoader, rule *domain.CostRule) ([]*domain.CostRule, int, error) {
	if rule == nil {
		return nil, 0, nil
	}

	visited := map[int64]struct{}{}
	if rule.RuleID != 0 {
		visited[rule.RuleID] = struct{}{}
	}

	previous := make([]*domain.CostRule, 0, 4)
	cursor := rule
	for cursor.SupersedesRuleID != nil && *cursor.SupersedesRuleID != 0 {
		priorRule, err := loader.get(ctx, *cursor.SupersedesRuleID)
		if err != nil {
			return nil, 0, err
		}
		if priorRule == nil {
			break
		}
		if _, seen := visited[priorRule.RuleID]; seen {
			break
		}
		visited[priorRule.RuleID] = struct{}{}
		previous = append(previous, priorRule)
		cursor = priorRule
	}
	reverseCostRuleSlice(previous)

	chain := append(previous, rule)
	currentIndex := len(chain) - 1

	cursor = rule
	for cursor.SupersededByRuleID != nil && *cursor.SupersededByRuleID != 0 {
		nextRule, err := loader.get(ctx, *cursor.SupersededByRuleID)
		if err != nil {
			return nil, 0, err
		}
		if nextRule == nil {
			break
		}
		if _, seen := visited[nextRule.RuleID]; seen {
			break
		}
		visited[nextRule.RuleID] = struct{}{}
		chain = append(chain, nextRule)
		cursor = nextRule
	}

	return chain, currentIndex, nil
}

func reverseCostRuleSlice(items []*domain.CostRule) {
	for left, right := 0, len(items)-1; left < right; left, right = left+1, right-1 {
		items[left], items[right] = items[right], items[left]
	}
}

func costRuleVersionRefs(items []*domain.CostRule) []*domain.CostRuleVersionRef {
	if len(items) == 0 {
		return []*domain.CostRuleVersionRef{}
	}
	refs := make([]*domain.CostRuleVersionRef, 0, len(items))
	for _, item := range items {
		refs = append(refs, costRuleVersionRef(item))
	}
	return refs
}

func costRuleVersionRef(rule *domain.CostRule) *domain.CostRuleVersionRef {
	if rule == nil {
		return nil
	}
	return &domain.CostRuleVersionRef{
		RuleID:           rule.RuleID,
		RuleName:         rule.RuleName,
		RuleVersion:      rule.RuleVersion,
		GovernanceStatus: rule.GovernanceStatus,
		EffectiveFrom:    cloneTimePtr(rule.EffectiveFrom),
		EffectiveTo:      cloneTimePtr(rule.EffectiveTo),
		Source:           rule.Source,
	}
}

func buildCostRuleVersionChainSummary(chain []*domain.CostRule, currentIndex int) *domain.CostRuleVersionChainSummary {
	if len(chain) == 0 || currentIndex < 0 || currentIndex >= len(chain) {
		return nil
	}
	rootRule := chain[0]
	latestRule := chain[len(chain)-1]
	return &domain.CostRuleVersionChainSummary{
		RootRuleID:        rootRule.RuleID,
		RootRuleVersion:   rootRule.RuleVersion,
		LatestRuleID:      latestRule.RuleID,
		LatestRuleVersion: latestRule.RuleVersion,
		TotalVersions:     len(chain),
		SupersessionDepth: currentIndex,
		IsLatestVersion:   currentIndex == len(chain)-1,
	}
}

func buildTaskGovernanceReadModels(
	ctx context.Context,
	costRuleRepo repo.CostRuleRepo,
	detail *domain.TaskDetail,
	events []*domain.TaskEvent,
	overrideEvents []*domain.TaskCostOverrideAuditEvent,
	reviewRecords []*domain.TaskCostOverrideReviewRecord,
	financeFlags []*domain.TaskCostFinanceFlag,
) (*domain.TaskMatchedRuleGovernance, *domain.TaskCostOverrideSummary, *domain.TaskGovernanceAuditSummary, *domain.TaskCostOverrideGovernanceBoundary, *domain.AppError) {
	overrideSummary, governanceAuditSummary, overrideBoundary := buildTaskCostOverrideReadModels(detail, events, overrideEvents, reviewRecords, financeFlags)
	if detail == nil {
		return nil, overrideSummary, governanceAuditSummary, overrideBoundary, nil
	}

	snapshot := &domain.TaskMatchedRuleSnapshot{
		RuleName:             detail.CostRuleName,
		RuleSource:           detail.CostRuleSource,
		GovernanceStatus:     domain.CostRuleGovernanceStatusNoMatch,
		PrefillSource:        detail.PrefillSource,
		PrefillAt:            cloneTimePtr(detail.PrefillAt),
		RequiresManualReview: detail.RequiresManualReview,
	}
	if detail.CostRuleID != nil {
		snapshot.RuleID = *detail.CostRuleID
	}
	if detail.MatchedRuleVersion != nil {
		snapshot.RuleVersion = *detail.MatchedRuleVersion
	}

	if costRuleRepo == nil || detail.CostRuleID == nil || *detail.CostRuleID == 0 {
		if snapshot.RuleID == 0 && snapshot.RuleName == "" && snapshot.RuleVersion == 0 && snapshot.RuleSource == "" {
			return nil, overrideSummary, governanceAuditSummary, overrideBoundary, nil
		}
		return &domain.TaskMatchedRuleGovernance{MatchedRule: snapshot}, overrideSummary, governanceAuditSummary, overrideBoundary, nil
	}

	loader := newCostRuleLineageLoader(costRuleRepo, time.Now().UTC())
	matchedRule, err := loader.get(ctx, *detail.CostRuleID)
	if err != nil {
		return nil, overrideSummary, governanceAuditSummary, overrideBoundary, infraError("get matched cost rule governance", err)
	}
	if matchedRule == nil {
		return &domain.TaskMatchedRuleGovernance{MatchedRule: snapshot}, overrideSummary, governanceAuditSummary, overrideBoundary, nil
	}

	history, err := decorateCostRuleLineage(ctx, loader, matchedRule)
	if err != nil {
		return nil, overrideSummary, governanceAuditSummary, overrideBoundary, infraError("decorate matched cost rule governance", err)
	}
	if snapshot.RuleID == 0 {
		snapshot.RuleID = matchedRule.RuleID
	}
	if snapshot.RuleName == "" {
		snapshot.RuleName = matchedRule.RuleName
	}
	if snapshot.RuleVersion == 0 {
		snapshot.RuleVersion = matchedRule.RuleVersion
	}
	if snapshot.RuleSource == "" {
		snapshot.RuleSource = matchedRule.Source
	}
	snapshot.GovernanceStatus = matchedRule.GovernanceStatus

	governance := &domain.TaskMatchedRuleGovernance{
		MatchedRule:         snapshot,
		CurrentRule:         history.CurrentRule,
		VersionChainSummary: matchedRule.VersionChainSummary,
		IsRuleOutdated:      history.CurrentRule != nil && history.CurrentRule.RuleID != snapshot.RuleID,
	}
	if history.CurrentRule != nil {
		snapshot.IsCurrentRule = history.CurrentRule.RuleID == snapshot.RuleID
		governance.CurrentRuleVersionHint = cloneIntPtr(&history.CurrentRule.RuleVersion)
	}

	return governance, overrideSummary, governanceAuditSummary, overrideBoundary, nil
}

type taskBusinessInfoOverridePayload struct {
	ManualCostOverride       bool       `json:"manual_cost_override"`
	ManualCostOverrideReason string     `json:"manual_cost_override_reason"`
	OverrideActor            string     `json:"override_actor"`
	OverrideAt               *time.Time `json:"override_at"`
	CostPrice                *float64   `json:"cost_price"`
}

func buildTaskCostOverrideReadModels(
	detail *domain.TaskDetail,
	events []*domain.TaskEvent,
	overrideEvents []*domain.TaskCostOverrideAuditEvent,
	reviewRecords []*domain.TaskCostOverrideReviewRecord,
	financeFlags []*domain.TaskCostFinanceFlag,
) (*domain.TaskCostOverrideSummary, *domain.TaskGovernanceAuditSummary, *domain.TaskCostOverrideGovernanceBoundary) {
	if len(overrideEvents) > 0 {
		return buildTaskCostOverrideSummaryFromAuditEvents(detail, overrideEvents, reviewRecords, financeFlags)
	}
	return buildTaskCostOverrideSummaryFromTaskEvents(detail, events)
}

func buildTaskCostOverrideSummaryFromAuditEvents(
	detail *domain.TaskDetail,
	events []*domain.TaskCostOverrideAuditEvent,
	reviewRecords []*domain.TaskCostOverrideReviewRecord,
	financeFlags []*domain.TaskCostFinanceFlag,
) (*domain.TaskCostOverrideSummary, *domain.TaskGovernanceAuditSummary, *domain.TaskCostOverrideGovernanceBoundary) {
	if detail == nil {
		return nil, nil, nil
	}

	reviewMap := taskCostOverrideReviewMap(reviewRecords)
	financeMap := taskCostFinanceFlagMap(financeFlags)
	summary := &domain.TaskCostOverrideSummary{
		CurrentOverrideActive: detail.ManualCostOverride,
		CurrentOverrideReason: detail.ManualCostOverrideReason,
		CurrentOverrideActor:  detail.OverrideActor,
		CurrentOverrideAt:     cloneTimePtr(detail.OverrideAt),
		CurrentCostPrice:      cloneFloat64Ptr(detail.CostPrice),
		HistorySource:         taskCostOverrideAuditHistorySource,
	}
	var overrideBoundary *domain.TaskCostOverrideGovernanceBoundary
	for _, event := range events {
		if event == nil {
			continue
		}
		event.OverrideBoundary = buildTaskCostOverrideGovernanceBoundary(event, reviewMap[event.EventID], financeMap[event.EventID])
		overrideBoundary = cloneTaskCostOverrideGovernanceBoundary(event.OverrideBoundary)
		snapshot := summarizeTaskCostOverrideAuditEvent(event)
		summary.LatestAuditEvent = snapshot
		switch event.EventType {
		case domain.TaskCostOverrideAuditEventApplied, domain.TaskCostOverrideAuditEventUpdated:
			summary.OverrideEventCount++
			summary.LatestOverrideEvent = snapshot
		case domain.TaskCostOverrideAuditEventReleased:
			summary.LatestReleaseEvent = snapshot
		}
	}

	return summary, buildTaskGovernanceAuditSummary(detail, summary, len(events)), overrideBoundary
}

func buildTaskCostOverrideSummaryFromTaskEvents(detail *domain.TaskDetail, events []*domain.TaskEvent) (*domain.TaskCostOverrideSummary, *domain.TaskGovernanceAuditSummary, *domain.TaskCostOverrideGovernanceBoundary) {
	if detail == nil {
		return nil, nil, nil
	}

	summary := &domain.TaskCostOverrideSummary{
		CurrentOverrideActive: detail.ManualCostOverride,
		CurrentOverrideReason: detail.ManualCostOverrideReason,
		CurrentOverrideActor:  detail.OverrideActor,
		CurrentOverrideAt:     cloneTimePtr(detail.OverrideAt),
		CurrentCostPrice:      cloneFloat64Ptr(detail.CostPrice),
		HistorySource:         taskCostOverrideFallbackHistorySource,
	}

	overrideActive := false
	auditEventCount := 0
	for _, event := range events {
		if event == nil || event.EventType != domain.TaskEventBusinessInfoUpdated {
			continue
		}
		payload, ok := parseTaskBusinessInfoOverridePayload(event.Payload)
		if !ok {
			continue
		}
		occurredAt := event.CreatedAt
		if payload.OverrideAt != nil {
			occurredAt = *payload.OverrideAt
		}

		eventType := domain.TaskCostOverrideAuditEventUpdated
		if payload.ManualCostOverride {
			if !overrideActive {
				eventType = domain.TaskCostOverrideAuditEventApplied
			}
		} else {
			if !overrideActive {
				continue
			}
			eventType = domain.TaskCostOverrideAuditEventReleased
		}

		snapshot := &domain.TaskCostOverrideEventSummary{
			EventID:          event.ID,
			Sequence:         event.Sequence,
			EventType:        eventType,
			CostPrice:        cloneFloat64Ptr(payload.CostPrice),
			OverrideCost:     cloneFloat64Ptr(payload.CostPrice),
			ResultCostPrice:  cloneFloat64Ptr(payload.CostPrice),
			GovernanceStatus: domain.CostRuleGovernanceStatusNoMatch,
			Reason:           payload.ManualCostOverrideReason,
			Actor:            payload.OverrideActor,
			Source:           taskCostOverrideFallbackHistorySource,
			OccurredAt:       occurredAt,
		}
		auditEventCount++
		if !payload.ManualCostOverride {
			snapshot.OverrideCost = nil
		}
		summary.LatestAuditEvent = snapshot
		if payload.ManualCostOverride {
			summary.OverrideEventCount++
			summary.LatestOverrideEvent = snapshot
			overrideActive = true
			continue
		}
		if overrideActive {
			summary.LatestReleaseEvent = snapshot
		}
		overrideActive = false
	}

	return summary, buildTaskGovernanceAuditSummary(detail, summary, auditEventCount), buildFallbackTaskCostOverrideGovernanceBoundary(detail, summary)
}

func buildTaskCostOverrideGovernanceBoundary(
	event *domain.TaskCostOverrideAuditEvent,
	review *domain.TaskCostOverrideReviewRecord,
	finance *domain.TaskCostFinanceFlag,
) *domain.TaskCostOverrideGovernanceBoundary {
	if event == nil {
		return nil
	}

	reviewRequired := event.EventType != domain.TaskCostOverrideAuditEventReleased
	if review != nil {
		reviewRequired = review.ReviewRequired
	}
	reviewStatus := normalizeTaskCostOverrideReviewStatus("", reviewRequired)
	if review != nil {
		reviewStatus = normalizeTaskCostOverrideReviewStatus(review.ReviewStatus, reviewRequired)
	}

	financeRequired := event.EventType != domain.TaskCostOverrideAuditEventReleased
	if finance != nil {
		financeRequired = finance.FinanceRequired
	}
	financeStatus := normalizeTaskCostOverrideFinanceStatus("", financeRequired, reviewRequired, reviewStatus)
	if finance != nil {
		financeStatus = normalizeTaskCostOverrideFinanceStatus(finance.FinanceStatus, financeRequired, reviewRequired, reviewStatus)
	}

	return composeTaskCostOverrideGovernanceBoundary(event.EventID, event.TaskID, reviewRequired, reviewStatus, review, financeRequired, financeStatus, finance)
}

func buildFallbackTaskCostOverrideGovernanceBoundary(detail *domain.TaskDetail, summary *domain.TaskCostOverrideSummary) *domain.TaskCostOverrideGovernanceBoundary {
	if detail == nil || summary == nil || summary.LatestAuditEvent == nil {
		return nil
	}

	reviewRequired := summary.LatestAuditEvent.EventType != domain.TaskCostOverrideAuditEventReleased
	reviewStatus := normalizeTaskCostOverrideReviewStatus("", reviewRequired)
	financeRequired := reviewRequired
	financeStatus := normalizeTaskCostOverrideFinanceStatus("", financeRequired, reviewRequired, reviewStatus)

	return composeTaskCostOverrideGovernanceBoundary(summary.LatestAuditEvent.EventID, detail.TaskID, reviewRequired, reviewStatus, nil, financeRequired, financeStatus, nil)
}

func composeTaskCostOverrideGovernanceBoundary(
	overrideEventID string,
	taskID int64,
	reviewRequired bool,
	reviewStatus domain.TaskCostOverrideReviewStatus,
	review *domain.TaskCostOverrideReviewRecord,
	financeRequired bool,
	financeStatus domain.TaskCostOverrideFinanceStatus,
	finance *domain.TaskCostFinanceFlag,
) *domain.TaskCostOverrideGovernanceBoundary {
	financeViewReady := computeTaskCostOverrideFinanceViewReady(financeRequired, financeStatus, reviewRequired, reviewStatus)
	reviewAction := buildTaskCostOverrideReviewActionSummary(reviewRequired, reviewStatus, review)
	financeAction := buildTaskCostOverrideFinanceActionSummary(financeRequired, financeStatus, finance)
	latestAction := latestTaskCostOverrideBoundaryAction(reviewAction, financeAction)

	boundary := &domain.TaskCostOverrideGovernanceBoundary{
		OverrideEventID:           overrideEventID,
		TaskID:                    taskID,
		ReviewRequired:            reviewRequired,
		ReviewStatus:              reviewStatus,
		ApprovalPlaceholderStatus: reviewStatus,
		FinanceRequired:           financeRequired,
		FinanceStatus:             financeStatus,
		FinancePlaceholderStatus:  financeStatus,
		FinanceViewReady:          financeViewReady,
		LatestReviewAction:        cloneTaskCostOverrideBoundaryActionSummary(reviewAction),
		LatestFinanceAction:       cloneTaskCostOverrideBoundaryActionSummary(financeAction),
		IsPlaceholderBoundaryOnly: true,
	}
	if latestAction != nil {
		boundary.LatestBoundaryActor = latestAction.Actor
		boundary.LatestBoundaryAt = cloneTimePtr(latestAction.ActedAt)
	}
	if review != nil {
		boundary.ReviewRecordID = cloneInt64Ptr(&review.RecordID)
		boundary.ReviewNote = review.ReviewNote
		boundary.ReviewActor = review.ReviewActor
		boundary.ReviewedAt = cloneTimePtr(review.ReviewedAt)
	}
	if finance != nil {
		boundary.FinanceRecordID = cloneInt64Ptr(&finance.RecordID)
		boundary.FinanceNote = finance.FinanceNote
		boundary.FinanceMarkedBy = finance.FinanceMarkedBy
		boundary.FinanceMarkedAt = cloneTimePtr(finance.FinanceMarkedAt)
	}

	boundary.ApprovalPlaceholderSummary = &domain.TaskCostOverrideApprovalPlaceholderSummary{
		OverrideEventID:           overrideEventID,
		TaskID:                    taskID,
		ReviewRecordID:            cloneInt64Ptr(boundary.ReviewRecordID),
		ReviewRequired:            reviewRequired,
		ReviewStatus:              reviewStatus,
		ApprovalPlaceholderStatus: reviewStatus,
		ReviewNote:                boundary.ReviewNote,
		ReviewActor:               boundary.ReviewActor,
		ReviewedAt:                cloneTimePtr(boundary.ReviewedAt),
		LatestReviewAction:        cloneTaskCostOverrideBoundaryActionSummary(reviewAction),
		Source:                    taskCostOverrideBoundarySource(review != nil, taskCostOverrideReviewBoundarySource),
		IsPlaceholderBoundaryOnly: true,
	}
	boundary.FinancePlaceholderSummary = &domain.TaskCostOverrideFinancePlaceholderSummary{
		OverrideEventID:           overrideEventID,
		TaskID:                    taskID,
		FinanceRecordID:           cloneInt64Ptr(boundary.FinanceRecordID),
		FinanceRequired:           financeRequired,
		FinanceStatus:             financeStatus,
		FinancePlaceholderStatus:  financeStatus,
		FinanceNote:               boundary.FinanceNote,
		FinanceMarkedBy:           boundary.FinanceMarkedBy,
		FinanceMarkedAt:           cloneTimePtr(boundary.FinanceMarkedAt),
		FinanceViewReady:          financeViewReady,
		LatestFinanceAction:       cloneTaskCostOverrideBoundaryActionSummary(financeAction),
		Source:                    taskCostOverrideBoundarySource(finance != nil, taskCostFinanceBoundarySource),
		IsPlaceholderBoundaryOnly: true,
	}
	boundary.GovernanceBoundarySummary = &domain.TaskCostOverrideGovernanceBoundarySummary{
		ReviewRequired:            reviewRequired,
		ReviewStatus:              reviewStatus,
		FinanceRequired:           financeRequired,
		FinanceStatus:             financeStatus,
		FinanceViewReady:          financeViewReady,
		LatestReviewAction:        cloneTaskCostOverrideBoundaryActionSummary(reviewAction),
		LatestFinanceAction:       cloneTaskCostOverrideBoundaryActionSummary(financeAction),
		LatestBoundaryActor:       boundary.LatestBoundaryActor,
		LatestBoundaryAt:          cloneTimePtr(boundary.LatestBoundaryAt),
		IsPlaceholderBoundaryOnly: true,
	}

	domain.HydrateTaskCostOverrideBoundaryPolicy(boundary)
	return boundary
}

func taskCostOverrideReviewMap(records []*domain.TaskCostOverrideReviewRecord) map[string]*domain.TaskCostOverrideReviewRecord {
	items := map[string]*domain.TaskCostOverrideReviewRecord{}
	for _, record := range records {
		if record == nil || strings.TrimSpace(record.OverrideEventID) == "" {
			continue
		}
		items[record.OverrideEventID] = record
	}
	return items
}

func taskCostFinanceFlagMap(flags []*domain.TaskCostFinanceFlag) map[string]*domain.TaskCostFinanceFlag {
	items := map[string]*domain.TaskCostFinanceFlag{}
	for _, flag := range flags {
		if flag == nil || strings.TrimSpace(flag.OverrideEventID) == "" {
			continue
		}
		items[flag.OverrideEventID] = flag
	}
	return items
}

func normalizeTaskCostOverrideReviewStatus(status domain.TaskCostOverrideReviewStatus, required bool) domain.TaskCostOverrideReviewStatus {
	if !required {
		return domain.TaskCostOverrideReviewStatusNotRequired
	}
	switch status {
	case domain.TaskCostOverrideReviewStatusApproved, domain.TaskCostOverrideReviewStatusRejected, domain.TaskCostOverrideReviewStatusPending:
		return status
	default:
		return domain.TaskCostOverrideReviewStatusPending
	}
}

func normalizeTaskCostOverrideFinanceStatus(
	status domain.TaskCostOverrideFinanceStatus,
	required bool,
	reviewRequired bool,
	reviewStatus domain.TaskCostOverrideReviewStatus,
) domain.TaskCostOverrideFinanceStatus {
	if !required {
		return domain.TaskCostOverrideFinanceStatusNotRequired
	}
	switch status {
	case domain.TaskCostOverrideFinanceStatusPending, domain.TaskCostOverrideFinanceStatusReadyForView, domain.TaskCostOverrideFinanceStatusMarkedForView:
		return status
	}
	if !reviewRequired || reviewStatus == domain.TaskCostOverrideReviewStatusApproved {
		return domain.TaskCostOverrideFinanceStatusReadyForView
	}
	return domain.TaskCostOverrideFinanceStatusPending
}

func computeTaskCostOverrideFinanceViewReady(
	financeRequired bool,
	financeStatus domain.TaskCostOverrideFinanceStatus,
	reviewRequired bool,
	reviewStatus domain.TaskCostOverrideReviewStatus,
) bool {
	if !financeRequired {
		return false
	}
	if reviewRequired && reviewStatus != domain.TaskCostOverrideReviewStatusApproved {
		return false
	}
	switch financeStatus {
	case domain.TaskCostOverrideFinanceStatusReadyForView, domain.TaskCostOverrideFinanceStatusMarkedForView:
		return true
	default:
		return false
	}
}

func buildTaskCostOverrideReviewActionSummary(
	reviewRequired bool,
	reviewStatus domain.TaskCostOverrideReviewStatus,
	review *domain.TaskCostOverrideReviewRecord,
) *domain.TaskCostOverrideBoundaryActionSummary {
	action := &domain.TaskCostOverrideBoundaryActionSummary{
		ActionType: taskCostOverrideReviewActionType(reviewRequired, reviewStatus),
		Status:     string(reviewStatus),
		Source:     taskCostOverrideBoundarySource(review != nil, taskCostOverrideReviewBoundarySource),
	}
	if review != nil {
		action.Actor = review.ReviewActor
		action.ActedAt = cloneTimePtr(review.ReviewedAt)
		action.Note = review.ReviewNote
	}
	return action
}

func buildTaskCostOverrideFinanceActionSummary(
	financeRequired bool,
	financeStatus domain.TaskCostOverrideFinanceStatus,
	finance *domain.TaskCostFinanceFlag,
) *domain.TaskCostOverrideBoundaryActionSummary {
	action := &domain.TaskCostOverrideBoundaryActionSummary{
		ActionType: taskCostOverrideFinanceActionType(financeRequired, financeStatus),
		Status:     string(financeStatus),
		Source:     taskCostOverrideBoundarySource(finance != nil, taskCostFinanceBoundarySource),
	}
	if finance != nil {
		action.Actor = finance.FinanceMarkedBy
		action.ActedAt = cloneTimePtr(finance.FinanceMarkedAt)
		action.Note = finance.FinanceNote
	}
	return action
}

func taskCostOverrideReviewActionType(required bool, status domain.TaskCostOverrideReviewStatus) string {
	switch {
	case !required:
		return "review_not_required"
	case status == domain.TaskCostOverrideReviewStatusApproved:
		return "review_approved"
	case status == domain.TaskCostOverrideReviewStatusRejected:
		return "review_rejected"
	default:
		return "review_pending"
	}
}

func taskCostOverrideFinanceActionType(required bool, status domain.TaskCostOverrideFinanceStatus) string {
	switch {
	case !required:
		return "finance_not_required"
	case status == domain.TaskCostOverrideFinanceStatusMarkedForView:
		return "finance_marked_for_view"
	case status == domain.TaskCostOverrideFinanceStatusReadyForView:
		return "finance_ready_for_view"
	default:
		return "finance_pending"
	}
}

func taskCostOverrideBoundarySource(hasPersistedRecord bool, persistedSource string) string {
	if hasPersistedRecord {
		return persistedSource
	}
	return taskCostDerivedBoundarySource
}

func latestTaskCostOverrideBoundaryAction(
	reviewAction *domain.TaskCostOverrideBoundaryActionSummary,
	financeAction *domain.TaskCostOverrideBoundaryActionSummary,
) *domain.TaskCostOverrideBoundaryActionSummary {
	switch {
	case reviewAction == nil:
		return cloneTaskCostOverrideBoundaryActionSummary(financeAction)
	case financeAction == nil:
		return cloneTaskCostOverrideBoundaryActionSummary(reviewAction)
	case reviewAction.ActedAt == nil && financeAction.ActedAt == nil:
		if strings.TrimSpace(financeAction.Actor) != "" {
			return cloneTaskCostOverrideBoundaryActionSummary(financeAction)
		}
		return cloneTaskCostOverrideBoundaryActionSummary(reviewAction)
	case reviewAction.ActedAt == nil:
		return cloneTaskCostOverrideBoundaryActionSummary(financeAction)
	case financeAction.ActedAt == nil:
		return cloneTaskCostOverrideBoundaryActionSummary(reviewAction)
	case !financeAction.ActedAt.Before(*reviewAction.ActedAt):
		return cloneTaskCostOverrideBoundaryActionSummary(financeAction)
	default:
		return cloneTaskCostOverrideBoundaryActionSummary(reviewAction)
	}
}

func cloneTaskCostOverrideBoundaryActionSummary(action *domain.TaskCostOverrideBoundaryActionSummary) *domain.TaskCostOverrideBoundaryActionSummary {
	if action == nil {
		return nil
	}
	copyAction := *action
	copyAction.ActedAt = cloneTimePtr(action.ActedAt)
	return &copyAction
}

func cloneTaskCostOverrideGovernanceBoundary(boundary *domain.TaskCostOverrideGovernanceBoundary) *domain.TaskCostOverrideGovernanceBoundary {
	if boundary == nil {
		return nil
	}
	copyBoundary := *boundary
	copyBoundary.PlatformEntryBoundary = domain.ClonePlatformEntryBoundary(boundary.PlatformEntryBoundary)
	copyBoundary.ReviewRecordID = cloneInt64Ptr(boundary.ReviewRecordID)
	copyBoundary.FinanceRecordID = cloneInt64Ptr(boundary.FinanceRecordID)
	copyBoundary.ReviewedAt = cloneTimePtr(boundary.ReviewedAt)
	copyBoundary.FinanceMarkedAt = cloneTimePtr(boundary.FinanceMarkedAt)
	copyBoundary.LatestReviewAction = cloneTaskCostOverrideBoundaryActionSummary(boundary.LatestReviewAction)
	copyBoundary.LatestFinanceAction = cloneTaskCostOverrideBoundaryActionSummary(boundary.LatestFinanceAction)
	copyBoundary.LatestBoundaryAt = cloneTimePtr(boundary.LatestBoundaryAt)
	if boundary.GovernanceBoundarySummary != nil {
		summary := *boundary.GovernanceBoundarySummary
		summary.LatestReviewAction = cloneTaskCostOverrideBoundaryActionSummary(boundary.GovernanceBoundarySummary.LatestReviewAction)
		summary.LatestFinanceAction = cloneTaskCostOverrideBoundaryActionSummary(boundary.GovernanceBoundarySummary.LatestFinanceAction)
		summary.LatestBoundaryAt = cloneTimePtr(boundary.GovernanceBoundarySummary.LatestBoundaryAt)
		copyBoundary.GovernanceBoundarySummary = &summary
	}
	if boundary.ApprovalPlaceholderSummary != nil {
		summary := *boundary.ApprovalPlaceholderSummary
		summary.ReviewRecordID = cloneInt64Ptr(boundary.ApprovalPlaceholderSummary.ReviewRecordID)
		summary.ReviewedAt = cloneTimePtr(boundary.ApprovalPlaceholderSummary.ReviewedAt)
		summary.LatestReviewAction = cloneTaskCostOverrideBoundaryActionSummary(boundary.ApprovalPlaceholderSummary.LatestReviewAction)
		copyBoundary.ApprovalPlaceholderSummary = &summary
	}
	if boundary.FinancePlaceholderSummary != nil {
		summary := *boundary.FinancePlaceholderSummary
		summary.FinanceRecordID = cloneInt64Ptr(boundary.FinancePlaceholderSummary.FinanceRecordID)
		summary.FinanceMarkedAt = cloneTimePtr(boundary.FinancePlaceholderSummary.FinanceMarkedAt)
		summary.LatestFinanceAction = cloneTaskCostOverrideBoundaryActionSummary(boundary.FinancePlaceholderSummary.LatestFinanceAction)
		copyBoundary.FinancePlaceholderSummary = &summary
	}
	copyBoundary.VisibleToRoles = append([]domain.Role(nil), boundary.VisibleToRoles...)
	if len(boundary.ActionRoles) > 0 {
		copyBoundary.ActionRoles = make([]domain.ActionPolicySummary, 0, len(boundary.ActionRoles))
		for _, action := range boundary.ActionRoles {
			copyAction := action
			copyAction.AllowedRoles = append([]domain.Role(nil), action.AllowedRoles...)
			copyBoundary.ActionRoles = append(copyBoundary.ActionRoles, copyAction)
		}
	}
	if boundary.PolicyScopeSummary != nil {
		summary := *boundary.PolicyScopeSummary
		summary.ResourceAccessPolicy.VisibleToRoles = append([]domain.Role(nil), boundary.PolicyScopeSummary.ResourceAccessPolicy.VisibleToRoles...)
		if len(boundary.PolicyScopeSummary.ResourceAccessPolicy.ActionRoles) > 0 {
			summary.ResourceAccessPolicy.ActionRoles = make([]domain.ActionPolicySummary, 0, len(boundary.PolicyScopeSummary.ResourceAccessPolicy.ActionRoles))
			for _, action := range boundary.PolicyScopeSummary.ResourceAccessPolicy.ActionRoles {
				copyAction := action
				copyAction.AllowedRoles = append([]domain.Role(nil), action.AllowedRoles...)
				summary.ResourceAccessPolicy.ActionRoles = append(summary.ResourceAccessPolicy.ActionRoles, copyAction)
			}
		}
		copyBoundary.PolicyScopeSummary = &summary
	}
	return &copyBoundary
}

func summarizeTaskCostOverrideAuditEvent(event *domain.TaskCostOverrideAuditEvent) *domain.TaskCostOverrideEventSummary {
	if event == nil {
		return nil
	}
	return &domain.TaskCostOverrideEventSummary{
		EventID:               event.EventID,
		Sequence:              event.Sequence,
		EventType:             event.EventType,
		CostPrice:             cloneFloat64Ptr(event.ResultCostPrice),
		PreviousEstimatedCost: cloneFloat64Ptr(event.PreviousEstimatedCost),
		PreviousCostPrice:     cloneFloat64Ptr(event.PreviousCostPrice),
		OverrideCost:          cloneFloat64Ptr(event.OverrideCost),
		ResultCostPrice:       cloneFloat64Ptr(event.ResultCostPrice),
		CategoryCode:          event.CategoryCode,
		MatchedRuleID:         cloneInt64Ptr(event.MatchedRuleID),
		MatchedRuleVersion:    cloneIntPtr(event.MatchedRuleVersion),
		MatchedRuleSource:     event.MatchedRuleSource,
		GovernanceStatus:      event.GovernanceStatus,
		Reason:                event.OverrideReason,
		Actor:                 event.OverrideActor,
		Source:                event.Source,
		Note:                  event.Note,
		OccurredAt:            event.OverrideAt,
	}
}

func buildTaskGovernanceAuditSummary(detail *domain.TaskDetail, summary *domain.TaskCostOverrideSummary, eventCount int) *domain.TaskGovernanceAuditSummary {
	if detail == nil || summary == nil {
		return nil
	}
	if !detail.ManualCostOverride && summary.OverrideEventCount == 0 && summary.LatestReleaseEvent == nil {
		return nil
	}

	auditSummary := &domain.TaskGovernanceAuditSummary{
		AuditLayer:            summary.HistorySource,
		GeneralEventLayer:     taskGovernanceGeneralEventLayer,
		EventCount:            eventCount,
		CurrentOverrideActive: detail.ManualCostOverride,
	}
	if summary.LatestAuditEvent != nil {
		auditSummary.LatestEventID = summary.LatestAuditEvent.EventID
		auditSummary.LatestEventType = summary.LatestAuditEvent.EventType
		auditSummary.LatestEventAt = cloneTimePtr(&summary.LatestAuditEvent.OccurredAt)
	}
	return auditSummary
}

func parseTaskBusinessInfoOverridePayload(raw json.RawMessage) (*taskBusinessInfoOverridePayload, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var payload taskBusinessInfoOverridePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, false
	}
	return &payload, true
}
