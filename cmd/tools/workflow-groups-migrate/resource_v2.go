package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"workflow/domain"
)

type mappedAssetState struct {
	ID                   int64
	TaskID               int64
	AssetType            string
	ScopeSKUCode         string
	RetouchRequirementID sql.NullInt64
	MimeType             string
	WholeHash            string
	UploadStatus         string
	FlowReviewStatus     string
	RejectedAt           sql.NullTime
	SupersededBy         sql.NullInt64
	SupersededAt         sql.NullTime
	DeletedAt            sql.NullTime
	CleanedAt            sql.NullTime
	AccessRevokedAt      sql.NullTime
	ObjectDeletedAt      sql.NullTime
}

func validateResourceMappingV2Preflight(ctx context.Context, q snapshotQueryer, mapping resourceMapping) (*resourceMigrationIssue, error) {
	return validateResourceMappingV2PreflightForStatus(ctx, q, mapping, "")
}

func validateResourceMappingV2PreflightForStatus(ctx context.Context, q snapshotQueryer, mapping resourceMapping, reviewedTargetStatus string) (*resourceMigrationIssue, error) {
	issue := func(groupID int64, format string, args ...interface{}) *resourceMigrationIssue {
		return &resourceMigrationIssue{GroupID: groupID, TaskID: mapping.TaskID, Reason: fmt.Sprintf(format, args...)}
	}
	var taskType, taskStatus string
	if err := q.QueryRowContext(ctx, `SELECT task_type,task_status FROM tasks WHERE id=?`, mapping.TaskID).Scan(&taskType, &taskStatus); errors.Is(err, sql.ErrNoRows) {
		return issue(0, "resource mapping references a missing task"), nil
	} else if err != nil {
		return nil, err
	}
	if mapping.ScopeKind == "sku" || mapping.ScopeKind == "retouch_requirement" {
		table := "task_sku_items"
		if mapping.ScopeKind == "retouch_requirement" {
			table = "task_retouch_requirements"
		}
		var scopeCount int
		if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table+` WHERE id=? AND task_id=?`, mapping.ScopeRefID, mapping.TaskID).Scan(&scopeCount); err != nil {
			return nil, err
		}
		if scopeCount != 1 {
			return issue(0, "%s scope %d does not belong to task", mapping.ScopeKind, mapping.ScopeRefID), nil
		}
	}
	var groupID int64
	err := q.QueryRowContext(ctx, `SELECT id FROM task_asset_groups WHERE task_id=? AND scope_kind=? AND scope_ref_id=?`, mapping.TaskID, mapping.ScopeKind, mapping.ScopeRefID).Scan(&groupID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if err == nil {
		var revisionCount int
		var incomplete bool
		if err := q.QueryRowContext(ctx, `SELECT COUNT(r.id),migration_incomplete FROM task_asset_group_revisions r RIGHT JOIN task_asset_groups g ON g.id=r.group_id WHERE g.id=? GROUP BY g.id,migration_incomplete`, groupID).Scan(&revisionCount, &incomplete); err != nil {
			return nil, err
		}
		if revisionCount > 0 {
			if incomplete {
				return issue(groupID, "resource group has revisions but remains migration_incomplete"), nil
			}
			if err := verifyResourceMappingV2Query(ctx, q, groupID, mapping); err != nil {
				return issue(groupID, "existing resource history differs from reviewed mapping: %v", err), nil
			}
			return nil, nil
		}
	}
	effectiveStatus := migratedTaskStatus(taskStatus)
	if reviewedTargetStatus != "" {
		effectiveStatus = reviewedTargetStatus
	}
	if len(mapping.History) == 0 {
		if effectiveStatus == "PendingAudit" || effectiveStatus == "Completed" {
			return issue(groupID, "task status %s cannot be migrated as an empty shell", effectiveStatus), nil
		}
		return nil, nil
	}
	if effectiveStatus == "Completed" && mapping.FinalizedRevisionNo == nil {
		return issue(groupID, "completed task requires a finalized resource history pointer"), nil
	}
	if effectiveStatus == "PendingAudit" {
		if mapping.WorkingRevisionNo == nil || mapping.revisionByNo(*mapping.WorkingRevisionNo).Status != "submitted" {
			return issue(groupID, "pending-audit task requires a submitted working revision"), nil
		}
	}
	for _, revision := range mapping.History {
		for _, actorID := range revisionActorIDsForPreflight(revision) {
			var count int
			if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE id=?`, actorID).Scan(&count); err != nil {
				return nil, err
			}
			if count != 1 {
				return issue(groupID, "revision %d actor %d does not exist", revision.RevisionNo, actorID), nil
			}
		}
		if err := validateRevisionEvidence(ctx, q, mapping.TaskID, revision); err != nil {
			return issue(groupID, "revision %d evidence invalid: %v", revision.RevisionNo, err), nil
		}
		if err := validateRevisionAssets(ctx, q, mapping, revision); err != nil {
			return issue(groupID, "revision %d asset mapping invalid: %v", revision.RevisionNo, err), nil
		}
	}
	return nil, nil
}

func revisionActorIDsForPreflight(revision resourceRevisionMapping) []int64 {
	actorIDs := []int64{revision.CreatedBy}
	if revision.Confidence == "confirmed_auto" {
		actorIDs = append(actorIDs, revision.ConfirmedBy)
	}
	return actorIDs
}

type evidenceEventMetadata struct {
	EventType string
	Payload   string
}

func isCustomizationTerminalEvidence(event evidenceEventMetadata) bool {
	if !strings.EqualFold(event.EventType, "task.customization.reviewed") {
		return false
	}
	var payload map[string]any
	if json.Unmarshal([]byte(event.Payload), &payload) != nil {
		return false
	}
	return strings.EqualFold(fmt.Sprint(payload["customization_review_decision"]), "approved") &&
		fmt.Sprint(payload["from_task_status"]) == "PendingCustomizationReview" &&
		fmt.Sprint(payload["to_task_status"]) == "PendingWarehouseReceive"
}

func validateRevisionEvidence(ctx context.Context, q snapshotQueryer, taskID int64, revision resourceRevisionMapping) error {
	metadata, err := loadEvidenceEventMetadata(ctx, q, taskID, revision.EvidenceEventIDs)
	if err != nil {
		return err
	}
	if err := validateRevisionEventSemantics(taskID, revision, metadata); err != nil {
		return err
	}
	if revision.SourceAliasFrom != nil {
		linkedCompletion := false
		for _, event := range metadata {
			if strings.Contains(strings.ToLower(event.EventType), "upload_session.completed") && payloadContainsAssetVersionID(event.Payload, *revision.SourceAliasFrom) {
				linkedCompletion = true
				break
			}
		}
		if !linkedCompletion {
			return fmt.Errorf("source alias origin %d lacks exact upload-session completion evidence", *revision.SourceAliasFrom)
		}
	}
	if strings.HasPrefix(revision.Reason, "policy legacy_post_close_replacement_v1:") &&
		containsString(revision.ReviewPolicyIDs, reviewPolicyLegacyPostCloseReplacement) {
		memberIDs := append([]int64(nil), revision.FinalAssetIDs...)
		if revision.SourceAssetID != nil {
			memberIDs = append(memberIDs, *revision.SourceAssetID)
		}
		if revision.SourceAliasFrom != nil {
			memberIDs = append(memberIDs, *revision.SourceAliasFrom)
		}
		linkedCompletion := false
		for _, event := range metadata {
			if !strings.Contains(strings.ToLower(event.EventType), "upload_session.completed") {
				continue
			}
			for _, assetID := range memberIDs {
				if payloadContainsAssetVersionID(event.Payload, assetID) {
					linkedCompletion = true
					break
				}
			}
		}
		if !linkedCompletion {
			return fmt.Errorf("post-close replacement lacks exact successor upload-session completion evidence")
		}
	}
	return nil
}

func payloadContainsAssetVersionID(raw string, target int64) bool {
	var payload any
	if json.Unmarshal([]byte(raw), &payload) != nil {
		return false
	}
	var walk func(any, string) bool
	walk = func(value any, key string) bool {
		switch typed := value.(type) {
		case map[string]any:
			for childKey, child := range typed {
				if walk(child, childKey) {
					return true
				}
			}
		case []any:
			for _, child := range typed {
				if walk(child, key) {
					return true
				}
			}
		case float64:
			allowed := key == "asset_version_id" || key == "asset_version_ids" || key == "task_asset_id" || key == "task_asset_ids"
			return allowed && int64(typed) == target && typed == float64(int64(typed))
		}
		return false
	}
	return walk(payload, "")
}

func loadEvidenceEventMetadata(ctx context.Context, q snapshotQueryer, taskID int64, evidenceIDs []string) ([]evidenceEventMetadata, error) {
	type orderedEvidence struct {
		namespace string
		sequence  int64
		metadata  evidenceEventMetadata
	}
	ordered := make([]orderedEvidence, 0, len(evidenceIDs))
	for _, stableID := range evidenceIDs {
		parts := strings.SplitN(stableID, ":", 2)
		namespace, rawID := parts[0], parts[1]
		var sequence int64
		var eventType, payload string
		switch namespace {
		case "task_event_log":
			var ownerTaskID int64
			var createdAt time.Time
			if err := q.QueryRowContext(ctx, `SELECT task_id,sequence,event_type,CAST(payload AS CHAR),created_at FROM task_event_logs WHERE id=?`, rawID).Scan(&ownerTaskID, &sequence, &eventType, &payload, &createdAt); errors.Is(err, sql.ErrNoRows) {
				return nil, fmt.Errorf("evidence %s is missing", stableID)
			} else if err != nil {
				return nil, err
			}
			if ownerTaskID != taskID {
				return nil, fmt.Errorf("evidence %s belongs to task %d", stableID, ownerTaskID)
			}
		case "task_module_event":
			eventID, err := strconv.ParseInt(rawID, 10, 64)
			if err != nil || eventID <= 0 {
				return nil, fmt.Errorf("evidence %s has an invalid numeric module-event id", stableID)
			}
			var ownerTaskID int64
			var createdAt time.Time
			if err := q.QueryRowContext(ctx, `
				SELECT tm.task_id,e.id,e.event_type,CAST(e.payload AS CHAR),e.created_at
				FROM task_module_events e JOIN task_modules tm ON tm.id=e.task_module_id
				WHERE e.id=?`, eventID).Scan(&ownerTaskID, &sequence, &eventType, &payload, &createdAt); errors.Is(err, sql.ErrNoRows) {
				return nil, fmt.Errorf("evidence %s is missing", stableID)
			} else if err != nil {
				return nil, err
			}
			if ownerTaskID != taskID {
				return nil, fmt.Errorf("evidence %s belongs to task %d", stableID, ownerTaskID)
			}
		default:
			return nil, fmt.Errorf("unsupported evidence namespace %q", namespace)
		}
		ordered = append(ordered, orderedEvidence{
			namespace: namespace,
			sequence:  sequence,
			metadata:  evidenceEventMetadata{EventType: eventType, Payload: payload},
		})
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].namespace != ordered[j].namespace {
			return ordered[i].namespace < ordered[j].namespace
		}
		return ordered[i].sequence < ordered[j].sequence
	})
	metadata := make([]evidenceEventMetadata, 0, len(ordered))
	for _, item := range ordered {
		metadata = append(metadata, item.metadata)
	}
	return metadata, nil
}

func validateRevisionEventSemantics(taskID int64, revision resourceRevisionMapping, metadata []evidenceEventMetadata) error {
	if revision.SourceStage == "migration" {
		return nil
	}
	hasSubmit, hasApprove, hasReject, hasReopen, hasUploadCompletion := false, false, false, false, false
	for _, event := range metadata {
		eventType := strings.ToLower(event.EventType)
		payload := strings.ToLower(event.Payload)
		hasSubmit = hasSubmit || strings.Contains(eventType, "design.submitted") || strings.Contains(eventType, "design_submitted") || eventType == "submitted"
		hasApprove = hasApprove || strings.Contains(eventType, "audit.approved") || strings.Contains(eventType, "audit.supplement_uploaded") || eventType == "approved" || strings.Contains(eventType, "task.closed") || eventType == "closed" || (strings.Contains(eventType, "customization.reviewed") && strings.Contains(payload, "approv"))
		hasReject = hasReject || strings.Contains(eventType, "audit.rejected") || strings.Contains(eventType, "returned_to_design") || eventType == "rejected" || (strings.Contains(eventType, "customization.reviewed") && (strings.Contains(payload, "reject") || strings.Contains(payload, "return")))
		hasReopen = hasReopen || strings.Contains(eventType, "reopen") || strings.Contains(eventType, "warehouse") || strings.Contains(eventType, "audit.supplement_uploaded") || strings.Contains(payload, "rejectedbywarehouse") || strings.Contains(payload, "warehouse")
		hasUploadCompletion = hasUploadCompletion || strings.Contains(eventType, "upload_session.completed")
	}
	switch revision.Status {
	case "submitted":
		if !hasSubmit {
			return fmt.Errorf("submitted revision lacks design-submission evidence")
		}
	case "finalized":
		legacyRetouchTerminal := (revision.SourceStage == "retouch" || revision.SourceStage == "reopen") && hasSubmit &&
			(containsString(revision.ReviewPolicyIDs, reviewPolicyLegacyRetouchTerminalSubmit) ||
				containsString(revision.ReviewPolicyIDs, reviewPolicyLegacyRetouchVisualScopeTask2533) ||
				hasBoundPolicyReason(revision, reviewPolicyLegacyRetouchUnscopedAtomicBatch) ||
				isLegacyRetouchPrematurePartialRevision(taskID, revision))
		postCloseReplacement := revision.SourceStage == "reopen" && hasUploadCompletion &&
			containsString(revision.ReviewPolicyIDs, reviewPolicyLegacyPostCloseReplacement) &&
			strings.HasPrefix(revision.Reason, "policy legacy_post_close_replacement_v1:")
		if !hasApprove && !legacyRetouchTerminal && !postCloseReplacement {
			return fmt.Errorf("finalized revision lacks approval/close evidence")
		}
		if (revision.SourceStage == "design" || revision.SourceStage == "retouch") && !hasSubmit {
			return fmt.Errorf("finalized %s revision lacks original submission evidence", revision.SourceStage)
		}
	case "rejected":
		if !hasReject {
			return fmt.Errorf("rejected revision lacks rejection evidence")
		}
	case "draft":
		postCloseDraft := revision.SourceStage == "reopen" && hasUploadCompletion &&
			containsString(revision.ReviewPolicyIDs, reviewPolicyLegacyPostCloseReplacement) &&
			strings.HasPrefix(revision.Reason, "policy legacy_post_close_replacement_v1:")
		prematurePartialDraft := revision.SourceStage == "reopen" &&
			isLegacyRetouchPrematurePartialRevision(taskID, revision)
		if revision.SourceStage != "reopen" || (!hasReject && !hasReopen && !postCloseDraft && !prematurePartialDraft) {
			return fmt.Errorf("draft revision lacks reopen/rejection evidence")
		}
	}
	if revision.SourceStage == "audit" && !hasApprove && !hasReject {
		return fmt.Errorf("audit revision lacks audit decision evidence")
	}
	if (revision.SourceStage == "design" || revision.SourceStage == "retouch") && revision.Status != "draft" && !hasSubmit {
		return fmt.Errorf("%s revision lacks submission evidence", revision.SourceStage)
	}
	return nil
}

func hasBoundPolicyReason(revision resourceRevisionMapping, policy string) bool {
	return containsString(revision.ReviewPolicyIDs, policy) &&
		strings.HasPrefix(revision.Reason, "policy "+policy+":")
}

func isLegacyRetouchPrematurePartialRevision(taskID int64, revision resourceRevisionMapping) bool {
	return containsInt64([]int64{981, 1035, 1045, 1052, 1214}, taskID) &&
		hasBoundPolicyReason(revision, reviewPolicyLegacyRetouchPrematurePartial)
}

func validateTaskStateDecisionPreflight(ctx context.Context, q snapshotQueryer, decision taskStateDecisionMapping, resources []resourceMapping) *taskMigrationIssue {
	issue := func(format string, args ...interface{}) *taskMigrationIssue {
		return &taskMigrationIssue{TaskID: decision.TaskID, Reason: fmt.Sprintf(format, args...)}
	}
	var taskType, currentStatus string
	if err := q.QueryRowContext(ctx, `SELECT task_type,task_status FROM tasks WHERE id=?`, decision.TaskID).Scan(&taskType, &currentStatus); errors.Is(err, sql.ErrNoRows) {
		return issue("reviewed task state decision references a missing task")
	} else if err != nil {
		return issue("validate reviewed task state decision task: %v", err)
	}
	if currentStatus != decision.FromStatus && currentStatus != decision.TargetStatus {
		return issue("reviewed task state decision expects status %s or idempotent target %s but database has %s", decision.FromStatus, decision.TargetStatus, currentStatus)
	}
	retouchDecision := containsString(decision.ReviewPolicyIDs, reviewPolicyLegacyRetouchPrematurePartial)
	customizationTerminalDecision := containsString(
		decision.ReviewPolicyIDs,
		reviewPolicyLegacyCustomizationTerminalNoAssets,
	)
	if retouchDecision && taskType != "retouch_task" {
		return issue("premature terminal policy requires task_type=retouch_task, got %s", taskType)
	}
	if customizationTerminalDecision {
		expectedTaskType := "original_product_development"
		if decision.TaskID == 756 || decision.TaskID == 757 {
			expectedTaskType = "new_product_development"
		}
		if taskType != expectedTaskType {
			return issue("customization terminal policy requires task_type=%s, got %s", expectedTaskType, taskType)
		}
	}
	var actorCount int
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE id=?`, decision.ConfirmedBy).Scan(&actorCount); err != nil {
		return issue("validate reviewed task state decision actor: %v", err)
	}
	if actorCount != 1 {
		return issue("reviewed task state decision actor %d does not exist", decision.ConfirmedBy)
	}
	metadata, err := loadEvidenceEventMetadata(ctx, q, decision.TaskID, decision.EvidenceEventIDs)
	if err != nil {
		return issue("reviewed task state decision evidence invalid: %v", err)
	}
	policyEvidence := false
	for _, event := range metadata {
		combined := strings.ToLower(event.EventType + " " + event.Payload)
		if (customizationTerminalDecision && isCustomizationTerminalEvidence(event)) ||
			(!retouchDecision && !customizationTerminalDecision && (strings.Contains(combined, "warehouse") || strings.Contains(combined, "rejectedbywarehouse"))) ||
			(retouchDecision && (strings.Contains(combined, "upload_session.completed") || strings.Contains(combined, "design.submitted"))) {
			policyEvidence = true
			break
		}
	}
	if !policyEvidence {
		if customizationTerminalDecision {
			return issue("reviewed customization terminal decision lacks the exact approved PendingCustomizationReview -> PendingWarehouseReceive evidence")
		} else if retouchDecision {
			return issue("reviewed retouch state decision lacks submit or completed-upload evidence")
		}
		return issue("reviewed warehouse decision lacks warehouse-rejection evidence")
	}
	taskResources := []resourceMapping{}
	for _, resource := range resources {
		if resource.TaskID == decision.TaskID {
			taskResources = append(taskResources, resource)
		}
	}
	if len(taskResources) == 0 {
		return issue("reviewed task state decision has no reviewed resource mappings")
	}
	if customizationTerminalDecision {
		expectedScopeCount := 1
		if decision.TaskID == 757 {
			expectedScopeCount = 2
		}
		if len(taskResources) != expectedScopeCount {
			return issue("customization terminal policy requires exactly %d allowlisted resource scopes", expectedScopeCount)
		}
		for _, resource := range taskResources {
			if _, allowed := legacyCustomizationTerminalExpectedSource(resource.TaskID, resource.ScopeKind, resource.ScopeRefID); !allowed {
				return issue("customization terminal policy includes unexpected resource scope %s/%d", resource.ScopeKind, resource.ScopeRefID)
			}
		}
	}
	for _, resource := range taskResources {
		switch decision.TargetStatus {
		case "InProgress":
			if resource.WorkingRevisionNo == nil {
				return issue("InProgress task state decision requires a working reopen draft for every resource scope")
			}
			working := resource.revisionByNo(*resource.WorkingRevisionNo)
			if working == nil || working.Status != "draft" || working.SourceStage != "reopen" {
				return issue("InProgress task state decision scope %s/%d does not point to a reopen draft", resource.ScopeKind, resource.ScopeRefID)
			}
		case "Completed":
			if resource.FinalizedRevisionNo == nil {
				return issue("Completed warehouse decision requires a finalized revision for every resource scope")
			}
			finalized := resource.revisionByNo(*resource.FinalizedRevisionNo)
			if finalized == nil || finalized.Status != "finalized" {
				return issue("Completed warehouse decision scope %s/%d lacks a valid finalized pointer", resource.ScopeKind, resource.ScopeRefID)
			}
		}
	}
	return nil
}

func validateRevisionAssets(ctx context.Context, q snapshotQueryer, mapping resourceMapping, revision resourceRevisionMapping) error {
	if revision.SourceAssetID != nil {
		if err := validateMappedAsset(ctx, q, mapping, *revision.SourceAssetID, "source", "", false, false); err != nil {
			return err
		}
		if err := validateMappedAssetRevisionLifecycle(ctx, q, mapping, revision, *revision.SourceAssetID); err != nil {
			return err
		}
	}
	if revision.SourceBundle != nil {
		bundle := revision.SourceBundle
		if err := validateMappedAsset(ctx, q, mapping, bundle.TaskAssetID, "source", bundle.BundleSHA256, false, false); err != nil {
			return fmt.Errorf("source bundle: %w", err)
		}
		if err := validateMappedAssetRevisionLifecycle(ctx, q, mapping, revision, bundle.TaskAssetID); err != nil {
			return fmt.Errorf("source bundle: %w", err)
		}
		var mimeType string
		if err := q.QueryRowContext(ctx, `SELECT COALESCE(mime_type,'') FROM task_assets WHERE id=?`, bundle.TaskAssetID).Scan(&mimeType); err != nil {
			return err
		}
		if normalized := strings.ToLower(strings.TrimSpace(strings.Split(mimeType, ";")[0])); normalized != "application/zip" && normalized != "application/x-zip-compressed" {
			return fmt.Errorf("source bundle task asset %d is not a ZIP", bundle.TaskAssetID)
		}
		for _, member := range bundle.Members {
			if err := validateMappedAsset(ctx, q, mapping, member.TaskAssetID, "source", member.SHA256, false, false); err != nil {
				return fmt.Errorf("source bundle member %d: %w", member.TaskAssetID, err)
			}
			if err := validateMappedAssetRevisionLifecycle(ctx, q, mapping, revision, member.TaskAssetID); err != nil {
				return fmt.Errorf("source bundle member %d: %w", member.TaskAssetID, err)
			}
		}
	}
	if revision.SourceAliasFrom != nil {
		if !containsInt64(revision.FinalAssetIDs, *revision.SourceAliasFrom) {
			return fmt.Errorf("source alias origin %d must remain a final asset", *revision.SourceAliasFrom)
		}
		if err := validateMappedAsset(ctx, q, mapping, *revision.SourceAliasFrom, "final", "", false, false); err != nil {
			return fmt.Errorf("source alias origin: %w", err)
		}
		if err := validateMappedAssetRevisionLifecycle(ctx, q, mapping, revision, *revision.SourceAliasFrom); err != nil {
			return fmt.Errorf("source alias origin %d: %w", *revision.SourceAliasFrom, err)
		}
	}
	for _, assetID := range revision.FinalAssetIDs {
		allowVisualScope := containsString(
			revision.ReviewPolicyIDs,
			reviewPolicyLegacyRetouchVisualScopeTask2533,
		)
		allowUnscopedRetouch := allowsLegacyUnscopedRetouchFinal(mapping, revision)
		if err := validateMappedAsset(ctx, q, mapping, assetID, "final", "", allowVisualScope, allowUnscopedRetouch); err != nil {
			return err
		}
		if err := validateMappedAssetRevisionLifecycle(ctx, q, mapping, revision, assetID); err != nil {
			return err
		}
	}
	for _, referenceID := range revision.ReferenceIDs {
		var ownerTaskID int64
		var skuItemID, retouchRequirementID sql.NullInt64
		if err := q.QueryRowContext(ctx, `SELECT task_id,sku_item_id,retouch_requirement_id FROM reference_file_refs WHERE id=?`, referenceID).Scan(&ownerTaskID, &skuItemID, &retouchRequirementID); errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("reference file %d is missing", referenceID)
		} else if err != nil {
			return err
		}
		if ownerTaskID != mapping.TaskID || !referenceMatchesResourceScope(mapping.ScopeKind, mapping.ScopeRefID, skuItemID, retouchRequirementID) {
			return fmt.Errorf("reference file %d does not belong to the mapped task scope", referenceID)
		}
	}
	return nil
}

func validateMappedAssetRevisionLifecycle(ctx context.Context, q snapshotQueryer, mapping resourceMapping, revision resourceRevisionMapping, assetID int64) error {
	state, err := loadMappedAssetState(ctx, q, assetID)
	if err != nil {
		return err
	}
	return validateRevisionLifecycleState(mapping, revision, state)
}

func validateRevisionLifecycleState(mapping resourceMapping, revision resourceRevisionMapping, state mappedAssetState) error {
	currentPointer := (mapping.WorkingRevisionNo != nil && *mapping.WorkingRevisionNo == revision.RevisionNo) ||
		(mapping.FinalizedRevisionNo != nil && *mapping.FinalizedRevisionNo == revision.RevisionNo)
	boundary := revision.CreatedAt
	if revision.SubmittedAt != nil && revision.SubmittedAt.After(boundary) {
		boundary = *revision.SubmittedAt
	}
	if revision.FinalizedAt != nil && revision.FinalizedAt.After(boundary) {
		boundary = *revision.FinalizedAt
	}

	flowStatus := strings.ToLower(strings.TrimSpace(state.FlowReviewStatus))
	if flowStatus == "cleaned" {
		return fmt.Errorf("task asset %d lifecycle is cleaned", state.ID)
	}
	if state.SupersededBy.Valid || flowStatus == "superseded" {
		if currentPointer && !inheritsRejectedSnapshotIntoReopenDraft(mapping, revision, state.ID) {
			return fmt.Errorf("task asset %d is superseded but remains on a current revision pointer", state.ID)
		}
		if !state.SupersededAt.Valid {
			return fmt.Errorf("task asset %d superseded lifecycle lacks a timestamp", state.ID)
		}
		if state.SupersededAt.Time.Before(boundary) {
			return fmt.Errorf("task asset %d was superseded before the revision boundary", state.ID)
		}
	}
	if flowStatus == "rejected" {
		if currentPointer {
			return fmt.Errorf("task asset %d is rejected but remains on a current revision pointer", state.ID)
		}
		if !state.RejectedAt.Valid {
			return fmt.Errorf("task asset %d rejected lifecycle lacks a timestamp", state.ID)
		}
		if state.RejectedAt.Time.Before(boundary) {
			return fmt.Errorf("task asset %d was rejected before the revision boundary", state.ID)
		}
	}
	return nil
}

func inheritsRejectedSnapshotIntoReopenDraft(mapping resourceMapping, revision resourceRevisionMapping, assetID int64) bool {
	if revision.Status != "draft" ||
		revision.SourceStage != "reopen" ||
		revision.RevisionNo <= 1 ||
		!containsString(revision.ReviewPolicyIDs, reviewPolicyReopen) {
		return false
	}
	previous := mapping.revisionByNo(revision.RevisionNo - 1)
	if previous == nil || previous.Status != "rejected" {
		return false
	}
	return revisionContainsAsset(revision, assetID) &&
		revisionContainsAsset(*previous, assetID)
}

func revisionContainsAsset(revision resourceRevisionMapping, assetID int64) bool {
	return (revision.SourceAssetID != nil && *revision.SourceAssetID == assetID) ||
		(revision.SourceAliasFrom != nil && *revision.SourceAliasFrom == assetID) ||
		(revision.SourceBundle != nil && revision.SourceBundle.TaskAssetID == assetID) ||
		containsInt64(revision.FinalAssetIDs, assetID)
}

func validateMappedAsset(ctx context.Context, q snapshotQueryer, mapping resourceMapping, assetID int64, role, expectedHash string, allowVisualScope, allowUnscopedRetouch bool) error {
	state, err := loadMappedAssetState(ctx, q, assetID)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("task asset %d is missing", assetID)
	}
	if err != nil {
		return err
	}
	if state.TaskID != mapping.TaskID {
		return fmt.Errorf("task asset %d belongs to task %d", assetID, state.TaskID)
	}
	assetType := domain.TaskAssetType(state.AssetType)
	if (role == "source" && !assetType.IsSource()) || (role == "final" && !assetType.IsDelivery()) {
		return fmt.Errorf("task asset %d asset_type=%s cannot bind as %s", assetID, state.AssetType, role)
	}
	if state.UploadStatus != "uploaded" || state.DeletedAt.Valid || state.CleanedAt.Valid || state.AccessRevokedAt.Valid || state.ObjectDeletedAt.Valid {
		return fmt.Errorf("task asset %d lifecycle is not active/uploaded", assetID)
	}
	if expectedHash != "" && !strings.EqualFold(state.WholeHash, expectedHash) {
		return fmt.Errorf("task asset %d whole_hash does not match reviewed sha256", assetID)
	}
	if err := validateMappedAssetScope(ctx, q, mapping, state, allowVisualScope, allowUnscopedRetouch); err != nil {
		return fmt.Errorf("task asset %d: %w", assetID, err)
	}
	return nil
}

func loadMappedAssetState(ctx context.Context, q snapshotQueryer, assetID int64) (mappedAssetState, error) {
	var state mappedAssetState
	err := q.QueryRowContext(ctx, `
		SELECT id,task_id,asset_type,COALESCE(scope_sku_code,''),retouch_requirement_id,
		       COALESCE(mime_type,''),COALESCE(whole_hash,''),COALESCE(upload_status,''),
		       COALESCE(flow_review_status,''),DATE_SUB(rejected_at,INTERVAL 8 HOUR),
		       superseded_by_version_id,DATE_SUB(superseded_at,INTERVAL 8 HOUR),
		       deleted_at,cleaned_at,access_revoked_at,object_deleted_at
		FROM task_assets WHERE id=?`, assetID).
		Scan(&state.ID, &state.TaskID, &state.AssetType, &state.ScopeSKUCode, &state.RetouchRequirementID,
			&state.MimeType, &state.WholeHash, &state.UploadStatus,
			&state.FlowReviewStatus, &state.RejectedAt, &state.SupersededBy, &state.SupersededAt,
			&state.DeletedAt, &state.CleanedAt, &state.AccessRevokedAt, &state.ObjectDeletedAt)
	return state, err
}

func validateMappedAssetScope(ctx context.Context, q snapshotQueryer, mapping resourceMapping, state mappedAssetState, allowVisualScope, allowUnscopedRetouch bool) error {
	switch mapping.ScopeKind {
	case "task":
		if strings.TrimSpace(state.ScopeSKUCode) != "" || state.RetouchRequirementID.Valid {
			return fmt.Errorf("scoped asset cannot bind to task scope")
		}
	case "sku":
		var skuCode string
		if err := q.QueryRowContext(ctx, `SELECT sku_code FROM task_sku_items WHERE id=? AND task_id=?`, mapping.ScopeRefID, mapping.TaskID).Scan(&skuCode); err != nil {
			return err
		}
		if state.RetouchRequirementID.Valid {
			return fmt.Errorf("asset SKU scope %q does not match mapped SKU %q", state.ScopeSKUCode, skuCode)
		}
		if strings.TrimSpace(state.ScopeSKUCode) == "" {
			var count int
			var soleID sql.NullInt64
			if err := q.QueryRowContext(ctx, `SELECT COUNT(*),MAX(id) FROM task_sku_items WHERE task_id=?`, mapping.TaskID).Scan(&count, &soleID); err != nil {
				return err
			}
			if count == 1 && soleID.Valid && soleID.Int64 == mapping.ScopeRefID {
				return nil
			}
		}
		if strings.TrimSpace(state.ScopeSKUCode) != strings.TrimSpace(skuCode) {
			return fmt.Errorf("asset SKU scope %q does not match mapped SKU %q", state.ScopeSKUCode, skuCode)
		}
	case "retouch_requirement":
		if strings.TrimSpace(state.ScopeSKUCode) != "" {
			return fmt.Errorf("asset retouch scope does not match requirement %d", mapping.ScopeRefID)
		}
		if !state.RetouchRequirementID.Valid {
			if expected, allowed := legacyRetouchVisualExpected(mapping.TaskID, mapping.ScopeKind, mapping.ScopeRefID); allowVisualScope && allowed && state.ID == expected.finalID {
				return nil
			}
			if allowUnscopedRetouch {
				return nil
			}
			var count int
			var soleID sql.NullInt64
			if err := q.QueryRowContext(ctx, `SELECT COUNT(*),MAX(id) FROM task_retouch_requirements WHERE task_id=?`, mapping.TaskID).Scan(&count, &soleID); err != nil {
				return err
			}
			if count == 1 && soleID.Valid && soleID.Int64 == mapping.ScopeRefID {
				return nil
			}
		}
		if !state.RetouchRequirementID.Valid || state.RetouchRequirementID.Int64 != mapping.ScopeRefID {
			return fmt.Errorf("asset retouch scope does not match requirement %d", mapping.ScopeRefID)
		}
	default:
		return fmt.Errorf("invalid resource scope")
	}
	return nil
}

func allowsLegacyUnscopedRetouchFinal(mapping resourceMapping, revision resourceRevisionMapping) bool {
	return hasBoundPolicyReason(revision, reviewPolicyLegacyRetouchUnscopedAtomicBatch) ||
		isLegacyRetouchPrematurePartialRevision(mapping.TaskID, revision)
}

func mappedAssetScopeValues(ctx context.Context, q snapshotQueryer, mapping resourceMapping) (interface{}, interface{}, error) {
	switch mapping.ScopeKind {
	case "task":
		return nil, nil, nil
	case "sku":
		var skuCode string
		if err := q.QueryRowContext(ctx, `SELECT sku_code FROM task_sku_items WHERE id=? AND task_id=?`, mapping.ScopeRefID, mapping.TaskID).Scan(&skuCode); err != nil {
			return nil, nil, err
		}
		return skuCode, nil, nil
	case "retouch_requirement":
		var exists int
		if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_retouch_requirements WHERE id=? AND task_id=?`, mapping.ScopeRefID, mapping.TaskID).Scan(&exists); err != nil {
			return nil, nil, err
		}
		if exists != 1 {
			return nil, nil, fmt.Errorf("retouch requirement %d does not belong to task %d", mapping.ScopeRefID, mapping.TaskID)
		}
		return nil, mapping.ScopeRefID, nil
	default:
		return nil, nil, fmt.Errorf("invalid resource scope")
	}
}

func uniqueSortedRevisionIDs(values []int64) []int64 {
	result := append([]int64(nil), values...)
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return uniqueSortedInt64(result)
}

func loadResourceRevisionGraph(ctx context.Context, q snapshotQueryer, groupID int64) ([]resourceRevisionSnapshot, error) {
	revisions := []resourceRevisionSnapshot{}
	rows, err := q.QueryContext(ctx, `
		SELECT id,group_id,revision_no,status,mode,source_task_asset_id,source_stage,created_by,reason,submitted_at,finalized_at,created_at
		FROM task_asset_group_revisions WHERE group_id=? ORDER BY revision_no,id`, groupID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var item resourceRevisionSnapshot
		var sourceID sql.NullInt64
		var submittedAt, finalizedAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.GroupID, &item.RevisionNo, &item.Status, &item.Mode, &sourceID, &item.SourceStage,
			&item.CreatedBy, &item.Reason, &submittedAt, &finalizedAt, &item.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		item.SourceAssetID = nullInt64Pointer(sourceID)
		item.SubmittedAt = nullTimePointer(submittedAt)
		item.FinalizedAt = nullTimePointer(finalizedAt)
		item.Items = []resourceRevisionItemSnapshot{}
		item.References = []resourceRevisionReferenceSnapshot{}
		revisions = append(revisions, item)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for index := range revisions {
		rows, err := q.QueryContext(ctx, `
			SELECT id,revision_id,task_asset_id,sort_order,item_name,created_at
			FROM task_asset_group_revision_items WHERE revision_id=? ORDER BY sort_order,id`, revisions[index].ID)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var child resourceRevisionItemSnapshot
			if err := rows.Scan(&child.ID, &child.RevisionID, &child.TaskAssetID, &child.SortOrder, &child.ItemName, &child.CreatedAt); err != nil {
				rows.Close()
				return nil, err
			}
			revisions[index].Items = append(revisions[index].Items, child)
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
		rows, err = q.QueryContext(ctx, `
			SELECT id,revision_id,reference_file_ref_id,formal_task_asset_id,sort_order,
			       ref_id_snapshot,file_name_snapshot,scope_snapshot,created_at
			FROM task_asset_group_revision_references WHERE revision_id=? ORDER BY sort_order,id`, revisions[index].ID)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var child resourceRevisionReferenceSnapshot
			var formalID sql.NullInt64
			if err := rows.Scan(&child.ID, &child.RevisionID, &child.ReferenceFileRefID, &formalID, &child.SortOrder,
				&child.RefIDSnapshot, &child.FileNameSnapshot, &child.ScopeSnapshot, &child.CreatedAt); err != nil {
				rows.Close()
				return nil, err
			}
			child.FormalTaskAssetID = nullInt64Pointer(formalID)
			revisions[index].References = append(revisions[index].References, child)
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
	}
	return revisions, nil
}

func applyResourceV2(ctx context.Context, tx *sql.Tx, mapping resourceMapping) (int64, []int64, []int64, bool, bool, error) {
	var groupID int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM task_asset_groups WHERE task_id=? AND scope_kind=? AND scope_ref_id=? FOR UPDATE`, mapping.TaskID, mapping.ScopeKind, mapping.ScopeRefID).Scan(&groupID)
	inserted := false
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, nil, nil, false, false, err
	}
	if errors.Is(err, sql.ErrNoRows) {
		var skuID, retouchID any
		if mapping.ScopeKind == "sku" {
			skuID = mapping.ScopeRefID
		}
		if mapping.ScopeKind == "retouch_requirement" {
			retouchID = mapping.ScopeRefID
		}
		res, err := tx.ExecContext(ctx, `
			INSERT INTO task_asset_groups
			  (task_id,scope_kind,task_sku_item_id,retouch_requirement_id,migration_incomplete,migration_issue)
			VALUES (?,?,?,?,1,'legacy resource revision pending cutover mapping')`, mapping.TaskID, mapping.ScopeKind, skuID, retouchID)
		if err != nil {
			return 0, nil, nil, false, false, err
		}
		groupID, _ = res.LastInsertId()
		inserted = true
	}
	var revisionCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_asset_group_revisions WHERE group_id=?`, groupID).Scan(&revisionCount); err != nil {
		return 0, nil, nil, inserted, false, err
	}
	if revisionCount > 0 {
		var incomplete bool
		if err := tx.QueryRowContext(ctx, `SELECT migration_incomplete FROM task_asset_groups WHERE id=?`, groupID).Scan(&incomplete); err != nil {
			return 0, nil, nil, inserted, false, err
		}
		if incomplete {
			return 0, nil, nil, inserted, false, fmt.Errorf("group %d has revisions but is still migration_incomplete; manual reconciliation required", groupID)
		}
		if err := verifyResourceMappingV2Query(ctx, tx, groupID, mapping); err != nil {
			return 0, nil, nil, inserted, false, err
		}
		return groupID, nil, nil, inserted, false, nil
	}
	if len(mapping.History) == 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE task_asset_groups SET migration_incomplete=0,migration_issue='' WHERE id=?`, groupID); err != nil {
			return 0, nil, nil, inserted, false, err
		}
		return groupID, nil, nil, inserted, false, nil
	}
	for _, revision := range mapping.History {
		if err := validateRevisionAssets(ctx, tx, mapping, revision); err != nil {
			return 0, nil, nil, inserted, false, err
		}
		if err := validateRevisionEvidence(ctx, tx, mapping.TaskID, revision); err != nil {
			return 0, nil, nil, inserted, false, err
		}
	}
	aliasIDs := []int64{}
	sourceIDByRevision := map[int]*int64{}
	for _, revision := range mapping.History {
		sourceID := revision.SourceAssetID
		if revision.SourceBundle != nil {
			sourceID = &revision.SourceBundle.TaskAssetID
		}
		if revision.SourceAliasFrom != nil {
			aliasID, created, err := ensureSourceAlias(ctx, tx, mapping, groupID, *revision.SourceAliasFrom)
			if err != nil {
				return 0, nil, aliasIDs, inserted, false, err
			}
			sourceID = &aliasID
			if created {
				aliasIDs = append(aliasIDs, aliasID)
			}
		}
		sourceIDByRevision[revision.RevisionNo] = sourceID
	}
	revisionIDs := make([]int64, 0, len(mapping.History))
	revisionIDByNo := map[int]int64{}
	for _, revision := range mapping.History {
		reason, err := persistedRevisionReason(revision)
		if err != nil {
			return 0, nil, aliasIDs, inserted, false, err
		}
		sourceID := sourceIDByRevision[revision.RevisionNo]
		res, err := tx.ExecContext(ctx, `
			INSERT INTO task_asset_group_revisions
			  (group_id,revision_no,status,mode,source_task_asset_id,source_stage,created_by,reason,submitted_at,finalized_at,created_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
			groupID, revision.RevisionNo, revision.Status, revision.Mode, sourceID, revision.SourceStage,
			revision.CreatedBy, reason, nullableTimePointer(revision.SubmittedAt), nullableTimePointer(revision.FinalizedAt), revision.CreatedAt.UTC())
		if err != nil {
			return 0, nil, aliasIDs, inserted, false, err
		}
		revisionID, _ := res.LastInsertId()
		revisionIDs = append(revisionIDs, revisionID)
		revisionIDByNo[revision.RevisionNo] = revisionID
		for order, assetID := range revision.FinalAssetIDs {
			if _, err := tx.ExecContext(ctx, `INSERT INTO task_asset_group_revision_items (revision_id,task_asset_id,sort_order) VALUES (?,?,?)`, revisionID, assetID, order); err != nil {
				return 0, nil, aliasIDs, inserted, false, err
			}
		}
		for order, referenceID := range revision.ReferenceIDs {
			if err := insertMigratedReferenceSnapshot(ctx, tx, revisionID, referenceID, order); err != nil {
				return 0, nil, aliasIDs, inserted, false, err
			}
		}
	}
	var workingID, finalizedID any
	if mapping.WorkingRevisionNo != nil {
		workingID = revisionIDByNo[*mapping.WorkingRevisionNo]
	}
	if mapping.FinalizedRevisionNo != nil {
		finalizedID = revisionIDByNo[*mapping.FinalizedRevisionNo]
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE task_asset_groups
		SET working_revision_id=?,finalized_revision_id=?,lock_version=lock_version+1,migration_incomplete=0,migration_issue=''
		WHERE id=?`, workingID, finalizedID, groupID); err != nil {
		return 0, nil, aliasIDs, inserted, false, err
	}
	// Historical revisions are served by the same controlled asset access path
	// as current revisions. Every referenced asset therefore remains bound to
	// the group, including assets unique to rejected/superseded rows. Mapping
	// validation already forbids role changes across history.
	scopeSKUCode, retouchRequirementID, err := mappedAssetScopeValues(ctx, tx, mapping)
	if err != nil {
		return 0, nil, aliasIDs, inserted, false, err
	}
	for _, revision := range mapping.History {
		sourceID := sourceIDByRevision[revision.RevisionNo]
		if sourceID != nil {
			if _, err := tx.ExecContext(ctx, `UPDATE task_assets SET binding_state='bound',bound_group_id=?,bound_role='source',scope_sku_code=?,retouch_requirement_id=? WHERE id=?`, groupID, scopeSKUCode, retouchRequirementID, *sourceID); err != nil {
				return 0, nil, aliasIDs, inserted, false, err
			}
		}
		for _, assetID := range revision.FinalAssetIDs {
			if _, err := tx.ExecContext(ctx, `UPDATE task_assets SET binding_state='bound',bound_group_id=?,bound_role='final',scope_sku_code=?,retouch_requirement_id=? WHERE id=?`, groupID, scopeSKUCode, retouchRequirementID, assetID); err != nil {
				return 0, nil, aliasIDs, inserted, false, err
			}
		}
	}
	if err := verifyResourceMappingV2Query(ctx, tx, groupID, mapping); err != nil {
		return 0, nil, aliasIDs, inserted, false, err
	}
	return groupID, revisionIDs, aliasIDs, inserted, true, nil
}

func sourceAliasRemark(groupID, originID int64) string {
	return fmt.Sprintf("v8-source-alias:group=%d:origin=%d", groupID, originID)
}

func findSourceAlias(ctx context.Context, q snapshotQueryer, taskID, groupID, originID int64) (int64, error) {
	var aliasID int64
	err := q.QueryRowContext(ctx, `
		SELECT id FROM task_assets
		WHERE task_id=? AND asset_type='source' AND source_module_key='migration'
		  AND bound_group_id=? AND bound_role='source' AND remark=?
		ORDER BY id`, taskID, groupID, sourceAliasRemark(groupID, originID)).Scan(&aliasID)
	return aliasID, err
}

func ensureSourceAlias(ctx context.Context, tx *sql.Tx, mapping resourceMapping, groupID, originID int64) (int64, bool, error) {
	if aliasID, err := findSourceAlias(ctx, tx, mapping.TaskID, groupID, originID); err == nil {
		return aliasID, false, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return 0, false, err
	}
	var nextVersion int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(version_no),0)+1 FROM task_assets WHERE task_id=?`, mapping.TaskID).Scan(&nextVersion); err != nil {
		return 0, false, err
	}
	res, err := tx.ExecContext(ctx, `
		INSERT INTO task_assets
		  (task_id,asset_id,scope_sku_code,retouch_requirement_id,asset_type,binding_state,bound_group_id,bound_role,
		   version_no,asset_version_no,upload_mode,upload_request_id,storage_ref_id,file_name,original_filename,remote_file_id,
		   mime_type,file_size,file_path,storage_key,whole_hash,upload_status,preview_status,uploaded_by,uploaded_at,remark,
		   source_module_key,source_task_module_id,is_archived,flow_review_status,created_at)
		SELECT task_id,asset_id,scope_sku_code,retouch_requirement_id,'source','bound',?,'source',
		       ?,asset_version_no,upload_mode,upload_request_id,storage_ref_id,file_name,original_filename,remote_file_id,
		       mime_type,file_size,file_path,storage_key,whole_hash,upload_status,preview_status,uploaded_by,uploaded_at,?,
		       'migration',NULL,0,'not_applicable',UTC_TIMESTAMP()
		FROM task_assets WHERE id=? AND task_id=? AND asset_type IN ('delivery','draft','revised','final','outsource_return')`,
		groupID, nextVersion, sourceAliasRemark(groupID, originID), originID, mapping.TaskID)
	if err != nil {
		return 0, false, err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return 0, false, err
	}
	if rows != 1 {
		return 0, false, fmt.Errorf("source alias origin %d was not an eligible delivery row", originID)
	}
	aliasID, err := res.LastInsertId()
	return aliasID, true, err
}

func verifyResourceMappingV2Query(ctx context.Context, q snapshotQueryer, groupID int64, mapping resourceMapping) error {
	graph, err := loadResourceRevisionGraph(ctx, q, groupID)
	if err != nil {
		return err
	}
	if len(graph) != len(mapping.History) {
		return fmt.Errorf("group %d has %d revisions; mapping requires %d", groupID, len(graph), len(mapping.History))
	}
	var workingID, finalizedID sql.NullInt64
	if err := q.QueryRowContext(ctx, `SELECT working_revision_id,finalized_revision_id FROM task_asset_groups WHERE id=?`, groupID).Scan(&workingID, &finalizedID); err != nil {
		return err
	}
	idByRevisionNo := map[int]int64{}
	for index, actual := range graph {
		expected := mapping.History[index]
		idByRevisionNo[actual.RevisionNo] = actual.ID
		reason, err := persistedRevisionReason(expected)
		if err != nil {
			return err
		}
		expectedSourceID := expected.SourceAssetID
		if expected.SourceBundle != nil {
			expectedSourceID = &expected.SourceBundle.TaskAssetID
		}
		if expected.SourceAliasFrom != nil {
			aliasID, err := findSourceAlias(ctx, q, mapping.TaskID, groupID, *expected.SourceAliasFrom)
			if err != nil {
				return fmt.Errorf("group %d revision %d source alias: %w", groupID, expected.RevisionNo, err)
			}
			expectedSourceID = &aliasID
		}
		if actual.RevisionNo != expected.RevisionNo || actual.Status != expected.Status || actual.Mode != expected.Mode ||
			pointerInt64Value(actual.SourceAssetID) != pointerInt64Value(expectedSourceID) || actual.SourceStage != expected.SourceStage ||
			actual.CreatedBy != expected.CreatedBy || actual.Reason != reason || !sameSecond(actual.CreatedAt, expected.CreatedAt) ||
			!sameOptionalSecond(actual.SubmittedAt, expected.SubmittedAt) || !sameOptionalSecond(actual.FinalizedAt, expected.FinalizedAt) {
			return fmt.Errorf("group %d revision %d metadata differs from mapping", groupID, expected.RevisionNo)
		}
		actualFinals := make([]int64, 0, len(actual.Items))
		for _, item := range actual.Items {
			actualFinals = append(actualFinals, item.TaskAssetID)
		}
		if !equalInt64Slices(actualFinals, expected.FinalAssetIDs) {
			return fmt.Errorf("group %d revision %d final-file order differs from mapping", groupID, expected.RevisionNo)
		}
		actualReferences := make([]int64, 0, len(actual.References))
		for _, reference := range actual.References {
			actualReferences = append(actualReferences, reference.ReferenceFileRefID)
		}
		if !equalInt64Slices(actualReferences, expected.ReferenceIDs) {
			return fmt.Errorf("group %d revision %d references differ from mapping", groupID, expected.RevisionNo)
		}
		if err := verifyFrozenReferenceSnapshots(ctx, q, groupID, actual); err != nil {
			return err
		}
		if err := validateRevisionAssets(ctx, q, mapping, expected); err != nil {
			return err
		}
		if err := validateRevisionEvidence(ctx, q, mapping.TaskID, expected); err != nil {
			return err
		}
	}
	expectedWorkingID := int64(0)
	if mapping.WorkingRevisionNo != nil {
		expectedWorkingID = idByRevisionNo[*mapping.WorkingRevisionNo]
	}
	expectedFinalizedID := int64(0)
	if mapping.FinalizedRevisionNo != nil {
		expectedFinalizedID = idByRevisionNo[*mapping.FinalizedRevisionNo]
	}
	if nullableInt64Value(workingID) != expectedWorkingID || nullableInt64Value(finalizedID) != expectedFinalizedID {
		return fmt.Errorf("group %d working/finalized pointers differ from mapping", groupID)
	}
	return nil
}

func verifyFrozenReferenceSnapshots(ctx context.Context, q snapshotQueryer, groupID int64, revision resourceRevisionSnapshot) error {
	for _, reference := range revision.References {
		var expectedRefID, expectedFileName string
		var skuID, retouchID sql.NullInt64
		if err := q.QueryRowContext(ctx, `
			SELECT rfr.ref_id,COALESCE(asr.file_name,''),rfr.sku_item_id,rfr.retouch_requirement_id
			FROM reference_file_refs rfr LEFT JOIN asset_storage_refs asr ON asr.ref_id=rfr.ref_id
			WHERE rfr.id=?`, reference.ReferenceFileRefID).Scan(&expectedRefID, &expectedFileName, &skuID, &retouchID); err != nil {
			return err
		}
		expectedScope := "task"
		if retouchID.Valid {
			expectedScope = fmt.Sprintf("retouch_requirement:%d", retouchID.Int64)
		} else if skuID.Valid {
			expectedScope = fmt.Sprintf("sku:%d", skuID.Int64)
		}
		if reference.RefIDSnapshot != expectedRefID || reference.FileNameSnapshot != expectedFileName || reference.ScopeSnapshot != expectedScope {
			return fmt.Errorf("group %d revision %d reference %d has an invalid frozen snapshot", groupID, revision.RevisionNo, reference.ReferenceFileRefID)
		}
	}
	return nil
}

func sameSecond(left, right time.Time) bool {
	return left.UTC().Truncate(time.Second).Equal(right.UTC().Truncate(time.Second))
}

func sameOptionalSecond(left, right *time.Time) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return sameSecond(*left, *right)
}
