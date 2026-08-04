package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	workflowGroupsMappingV1 = 1
	workflowGroupsMappingV2 = 2
	revisionReasonMaxRunes  = 512
)

var sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
var reviewPolicyIDPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{2,127}$`)
var recoveryRunIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{2,80}$`)

const (
	reviewPolicyExplicitEventReplay                  = "explicit_event_replay"
	reviewPolicyDeliverySourceAlias                  = "delivery_source_alias"
	reviewPolicyRejectedHistory                      = "rejected_history"
	reviewPolicyReopen                               = "reopen"
	reviewPolicyLegacyPostCloseReplacement           = "legacy_post_close_replacement_v1"
	reviewPolicyRetouchSourceOptional                = "retouch_source_optional"
	reviewPolicyLegacyRetouchTerminalSubmit          = "legacy_retouch_terminal_submit_v1"
	reviewPolicyLegacyRetouchUnscopedAtomicBatch     = "legacy_retouch_unscoped_atomic_batch_v1"
	reviewPolicyLegacyRetouchPrematurePartial        = "legacy_retouch_premature_terminal_partial_v1"
	reviewPolicyLegacyRetouchVisualScopeTask2533     = "legacy_retouch_visual_scope_task2533_v1"
	reviewPolicyLegacyMultiSKUAtomicBatchSubmit      = "legacy_multi_sku_atomic_batch_submit_v1"
	reviewPolicyLegacyAtomicUploadBatchSubmit        = "legacy_atomic_upload_batch_submit_v1"
	reviewPolicyLegacyAuditStageFinalSnapshot        = "legacy_audit_stage_final_snapshot_v1"
	reviewPolicyLegacyPurchaseToSKUPlanning          = "legacy_purchase_to_sku_planning_v1"
	reviewPolicyLegacyIncompleteUATPlanningTombstone = "legacy_incomplete_uat_planning_tombstone_v1"
	reviewPolicyFrozenSKUPlanningRuleRevision9       = "frozen_sku_planning_rule_revision_9_v1"
	reviewPolicyProductNameDescriptionFallback       = "product_name_snapshot_description_fallback_v1"
	reviewPolicyRetiredPlanningStatusToCompleted     = "retired_planning_status_to_completed_v1"
	reviewPolicyLegacyOrgUniqueStableMatch           = "legacy_org_unique_stable_match_v1"
	reviewPolicyLegacyOrgAliasLineage                = "legacy_org_alias_lineage_v1"
	reviewPolicyLegacyOrgManualTarget                = "legacy_org_manual_target_required_v1"
	reviewPolicyRetiredWarehouseNoGrant              = "retired_warehouse_no_new_grant_v1"
	reviewPolicyExistingAccessPreserved              = "existing_access_assignment_preserved_v1"
	reviewPolicyLegacyOutsourceAccessDecision        = "legacy_outsource_access_decision_v1"
	reviewPolicyLegacyOrgAdminAccessDecision         = "legacy_org_admin_access_decision_v1"
	reviewPolicyLegacyUATOrphanOrgToUnassigned       = "legacy_uat_orphan_org_to_unassigned_v1"
	reviewPolicyLegacyWarehouseReopenState           = "legacy_warehouse_reopen_state_v1"
	reviewPolicyLegacyCustomizationTerminalNoAssets  = "legacy_customization_terminal_without_assets_to_inprogress_v1"
	reviewPolicyLegacyDeletedAssetRecovery           = "legacy_deleted_asset_recovery_v1"
	reviewPolicyLegacyHistoricalAssetUnavailable     = "legacy_historical_asset_unavailable_v1"
)

var knownReviewPolicyIDs = []string{
	reviewPolicyExplicitEventReplay,
	reviewPolicyDeliverySourceAlias,
	reviewPolicyRejectedHistory,
	reviewPolicyReopen,
	reviewPolicyLegacyPostCloseReplacement,
	reviewPolicyRetouchSourceOptional,
	reviewPolicyLegacyRetouchTerminalSubmit,
	reviewPolicyLegacyRetouchUnscopedAtomicBatch,
	reviewPolicyLegacyRetouchPrematurePartial,
	reviewPolicyLegacyRetouchVisualScopeTask2533,
	reviewPolicyLegacyMultiSKUAtomicBatchSubmit,
	reviewPolicyLegacyAtomicUploadBatchSubmit,
	reviewPolicyLegacyAuditStageFinalSnapshot,
	reviewPolicyLegacyPurchaseToSKUPlanning,
	reviewPolicyLegacyIncompleteUATPlanningTombstone,
	reviewPolicyFrozenSKUPlanningRuleRevision9,
	reviewPolicyProductNameDescriptionFallback,
	reviewPolicyRetiredPlanningStatusToCompleted,
	reviewPolicyLegacyOrgUniqueStableMatch,
	reviewPolicyLegacyOrgAliasLineage,
	reviewPolicyLegacyOrgManualTarget,
	reviewPolicyRetiredWarehouseNoGrant,
	reviewPolicyExistingAccessPreserved,
	reviewPolicyLegacyOutsourceAccessDecision,
	reviewPolicyLegacyOrgAdminAccessDecision,
	reviewPolicyLegacyUATOrphanOrgToUnassigned,
	reviewPolicyLegacyWarehouseReopenState,
	reviewPolicyLegacyCustomizationTerminalNoAssets,
	reviewPolicyLegacyDeletedAssetRecovery,
	reviewPolicyLegacyHistoricalAssetUnavailable,
}

type legacyRetouchVisualMembership struct {
	sourceID     int64
	finalID      int64
	referenceIDs []int64
}

var legacyRetouchVisualTask2533 = map[int64]legacyRetouchVisualMembership{
	183: {sourceID: 19299, finalID: 19789, referenceIDs: []int64{3211, 3212}},
	184: {sourceID: 19301, finalID: 19790, referenceIDs: []int64{3213}},
	185: {sourceID: 19304, finalID: 19791, referenceIDs: []int64{3214, 3215}},
	186: {sourceID: 19306, finalID: 19800, referenceIDs: []int64{3216}},
	187: {sourceID: 19308, finalID: 19802, referenceIDs: []int64{3217}},
}

func legacyRetouchVisualExpected(taskID int64, scopeKind string, scopeRefID int64) (legacyRetouchVisualMembership, bool) {
	if taskID != 2533 || scopeKind != "retouch_requirement" {
		return legacyRetouchVisualMembership{}, false
	}
	membership, ok := legacyRetouchVisualTask2533[scopeRefID]
	return membership, ok
}

func legacyCustomizationTerminalExpectedSource(taskID int64, scopeKind string, scopeRefID int64) (*int64, bool) {
	key := fmt.Sprintf("%d/%s/%d", taskID, scopeKind, scopeRefID)
	switch key {
	case "449/task/0", "450/task/0", "451/task/0", "756/sku/578", "757/sku/579", "757/sku/580", "3091/sku/3311":
		return nil, true
	case "452/task/0":
		sourceID := int64(207)
		return &sourceID, true
	default:
		return nil, false
	}
}

func isLegacyCustomizationTerminalTask(taskID int64) bool {
	return containsInt64([]int64{449, 450, 451, 452, 756, 757, 3091}, taskID)
}

func isLegacyCompletedCustomizationMissingFinalResource(r resourceMapping) bool {
	if r.TaskID != 3091 ||
		r.ScopeKind != "sku" ||
		r.ScopeRefID != 3311 ||
		len(r.History) != 4 ||
		r.WorkingRevisionNo == nil ||
		*r.WorkingRevisionNo != 4 ||
		r.FinalizedRevisionNo != nil {
		return false
	}
	expectedFinals := []int64{28966, 29023, 29144}
	expectedStages := []string{"design", "audit", "reopen"}
	for index := 0; index < 3; index++ {
		revision := r.History[index]
		expectedID := expectedFinals[index]
		if revision.RevisionNo != index+1 ||
			revision.Status != "superseded" ||
			revision.SourceStage != expectedStages[index] ||
			revision.SourceAssetID != nil ||
			revision.SourceAliasFrom == nil ||
			*revision.SourceAliasFrom != expectedID ||
			revision.SourceBundle != nil ||
			!equalInt64Slices(revision.FinalAssetIDs, []int64{expectedID}) ||
			!equalInt64Slices(revision.ReferenceIDs, []int64{4009}) {
			return false
		}
	}
	historical := r.History[2]
	if !equalStringSlices(
		historical.ReviewPolicyIDs,
		[]string{
			reviewPolicyExplicitEventReplay,
			reviewPolicyDeliverySourceAlias,
			reviewPolicyReopen,
			reviewPolicyLegacyHistoricalAssetUnavailable,
		},
	) || !strings.HasPrefix(
		historical.Reason,
		"policy "+reviewPolicyLegacyHistoricalAssetUnavailable+":",
	) {
		return false
	}
	draft := r.History[3]
	return draft.RevisionNo == 4 &&
		draft.Status == "draft" &&
		draft.SourceStage == "reopen" &&
		draft.SourceAssetID == nil &&
		draft.SourceAliasFrom == nil &&
		draft.SourceBundle == nil &&
		len(draft.FinalAssetIDs) == 0 &&
		equalInt64Slices(draft.ReferenceIDs, []int64{4009}) &&
		equalStringSlices(
			draft.ReviewPolicyIDs,
			[]string{
				reviewPolicyExplicitEventReplay,
				reviewPolicyReopen,
				reviewPolicyLegacyCustomizationTerminalNoAssets,
			},
		) &&
		strings.HasPrefix(
			draft.Reason,
			"policy "+reviewPolicyLegacyCustomizationTerminalNoAssets+":",
		)
}

func isIncompleteUATPlanningTombstone(planning planningMapping) bool {
	if planning.TaskID != 497 ||
		planning.TargetTaskStatus != "Cancelled" ||
		planning.CodeRuleRevisionID != 9 ||
		len(planning.ReviewPolicyIDs) != 3 ||
		planning.ReviewPolicyIDs[0] != reviewPolicyLegacyPurchaseToSKUPlanning ||
		planning.ReviewPolicyIDs[1] != reviewPolicyLegacyIncompleteUATPlanningTombstone ||
		planning.ReviewPolicyIDs[2] != reviewPolicyFrozenSKUPlanningRuleRevision9 ||
		len(planning.Items) != 1 {
		return false
	}
	item := planning.Items[0]
	return item.TaskSKUItemID == 380 &&
		strings.TrimSpace(item.DescriptionSpec) == "" &&
		item.Quantity == 0 &&
		item.TargetPrice == nil &&
		strings.TrimSpace(item.Note) == "" &&
		strings.TrimSpace(item.ReferenceURL) == "" &&
		strings.TrimSpace(item.ERPProductIID) == "" &&
		strings.TrimSpace(item.ERPProductName) == "" &&
		strings.TrimSpace(item.ImageStorageRef) == ""
}

type resourceRevisionMapping struct {
	RevisionNo       int                  `json:"revision_no"`
	Status           string               `json:"status"`
	Mode             string               `json:"mode"`
	SourceStage      string               `json:"source_stage"`
	SourceAssetID    *int64               `json:"source_task_asset_id,omitempty"`
	SourceAliasFrom  *int64               `json:"source_alias_from_task_asset_id,omitempty"`
	SourceBundle     *sourceBundleMapping `json:"source_bundle,omitempty"`
	FinalAssetIDs    []int64              `json:"final_task_asset_ids"`
	ReferenceIDs     []int64              `json:"reference_file_ref_ids"`
	EvidenceEventIDs []string             `json:"evidence_event_ids"`
	Confidence       string               `json:"confidence"`
	ReviewPolicyIDs  []string             `json:"review_policy_ids"`
	Blockers         []string             `json:"blockers,omitempty"`
	ConfirmedBy      int64                `json:"confirmed_by"`
	ConfirmedAt      time.Time            `json:"confirmed_at"`
	ConfirmationNote string               `json:"confirmation_note"`
	ManifestRowHash  string               `json:"manifest_row_hash,omitempty"`
	Reason           string               `json:"reason"`
	CreatedBy        int64                `json:"created_by"`
	CreatedAt        time.Time            `json:"created_at"`
	SubmittedAt      *time.Time           `json:"submitted_at,omitempty"`
	FinalizedAt      *time.Time           `json:"finalized_at,omitempty"`
}

type sourceBundleMapping struct {
	TaskAssetID  int64                `json:"task_asset_id"`
	Format       string               `json:"format"`
	BundleSHA256 string               `json:"bundle_sha256"`
	ManifestHash string               `json:"manifest_sha256"`
	Members      []sourceBundleMember `json:"members"`
	ConfirmedBy  int64                `json:"confirmed_by"`
	ConfirmedAt  time.Time            `json:"confirmed_at"`
	Confirmation string               `json:"confirmation_note"`
}

type sourceBundleMember struct {
	TaskAssetID int64  `json:"task_asset_id"`
	SHA256      string `json:"sha256"`
	Confirmed   bool   `json:"confirmed"`
}

type taskStateDecisionMapping struct {
	TaskID           int64     `json:"task_id"`
	FromStatus       string    `json:"from_status"`
	TargetStatus     string    `json:"target_status"`
	EvidenceEventIDs []string  `json:"evidence_event_ids"`
	Confidence       string    `json:"confidence"`
	ReviewPolicyIDs  []string  `json:"review_policy_ids"`
	Blockers         []string  `json:"blockers,omitempty"`
	ConfirmedBy      int64     `json:"confirmed_by"`
	ConfirmedAt      time.Time `json:"confirmed_at"`
	ConfirmationNote string    `json:"confirmation_note"`
	ManifestRowHash  string    `json:"manifest_row_hash,omitempty"`
}

func isReviewedRetouchReopenDecision(decision taskStateDecisionMapping) bool {
	return decision.FromStatus == "Completed" &&
		decision.TargetStatus == "InProgress" &&
		equalStringSlices(
			decision.ReviewPolicyIDs,
			[]string{reviewPolicyLegacyRetouchPrematurePartial},
		)
}

type assetRecoveryMapping struct {
	TaskID                     int64     `json:"task_id"`
	MissingTaskAssetID         int64     `json:"missing_task_asset_id"`
	RecoverySourceTaskAssetID  int64     `json:"recovery_source_task_asset_id"`
	RejectedSourceTaskAssetIDs []int64   `json:"rejected_source_task_asset_ids,omitempty"`
	Strategy                   string    `json:"strategy"`
	OriginalStorageRefID       string    `json:"original_storage_ref_id"`
	RecoverySourceStorageRefID string    `json:"recovery_source_storage_ref_id,omitempty"`
	ExpectedFileSize           int64     `json:"expected_file_size"`
	PreviewWholeHash           string    `json:"preview_whole_hash"`
	DesignThumbWholeHash       string    `json:"design_thumb_whole_hash"`
	ObjectProbeResult          string    `json:"object_probe_result,omitempty"`
	ObjectProbeInputSHA256     string    `json:"object_probe_input_manifest_sha256,omitempty"`
	ObjectProbeEvidenceHash    string    `json:"object_probe_evidence_hash,omitempty"`
	ObjectProbeObjectKeySHA256 string    `json:"object_probe_object_key_sha256,omitempty"`
	ObjectProbeReadOnlyGETs    int       `json:"object_probe_read_only_get_count,omitempty"`
	ControlledReadProtocol     string    `json:"controlled_read_protocol,omitempty"`
	ControlledReadEvidenceHash string    `json:"controlled_read_evidence_sha256,omitempty"`
	RecoverySourceSHA256       string    `json:"recovery_source_sha256,omitempty"`
	Confidence                 string    `json:"confidence"`
	ReviewPolicyIDs            []string  `json:"review_policy_ids"`
	Blockers                   []string  `json:"blockers,omitempty"`
	ConfirmedBy                int64     `json:"confirmed_by"`
	ConfirmedAt                time.Time `json:"confirmed_at"`
	ConfirmationNote           string    `json:"confirmation_note"`
	ManifestRowHash            string    `json:"manifest_row_hash,omitempty"`
}

type organizationMapping struct {
	SubjectType        string    `json:"subject_type"`
	SubjectID          int64     `json:"subject_id"`
	LegacyDepartment   string    `json:"legacy_department"`
	LegacyTeam         string    `json:"legacy_team"`
	FromDepartmentID   *int64    `json:"from_department_id"`
	FromTeamID         *int64    `json:"from_team_id"`
	TargetDepartmentID int64     `json:"target_department_id"`
	TargetTeamID       int64     `json:"target_team_id"`
	Confidence         string    `json:"confidence"`
	ReviewPolicyIDs    []string  `json:"review_policy_ids"`
	Blockers           []string  `json:"blockers,omitempty"`
	ConfirmedBy        int64     `json:"confirmed_by"`
	ConfirmedAt        time.Time `json:"confirmed_at"`
	ConfirmationNote   string    `json:"confirmation_note"`
	ManifestRowHash    string    `json:"manifest_row_hash,omitempty"`
}

type accessAssignmentEvidence struct {
	RoleCode    string `json:"role_code"`
	ScopeMode   string `json:"scope_mode"`
	SourceType  string `json:"source_type"`
	SourceRefID int64  `json:"source_ref_id"`
}

type accessDecisionMapping struct {
	UserID                      int64                      `json:"user_id"`
	LegacyRole                  string                     `json:"legacy_role"`
	Action                      string                     `json:"action"`
	RequiredExistingAssignments []accessAssignmentEvidence `json:"required_existing_assignments"`
	Confidence                  string                     `json:"confidence"`
	ReviewPolicyIDs             []string                   `json:"review_policy_ids"`
	Blockers                    []string                   `json:"blockers,omitempty"`
	ConfirmedBy                 int64                      `json:"confirmed_by"`
	ConfirmedAt                 time.Time                  `json:"confirmed_at"`
	ConfirmationNote            string                     `json:"confirmation_note"`
	ManifestRowHash             string                     `json:"manifest_row_hash,omitempty"`
}

func mappingVersion(m mappingFile) int {
	if m.Version == 0 {
		return workflowGroupsMappingV1
	}
	return m.Version
}

func normalizeMapping(m mappingFile) mappingFile {
	if mappingVersion(m) != workflowGroupsMappingV2 {
		return m
	}
	m.Resources = append([]resourceMapping(nil), m.Resources...)
	for i := range m.Resources {
		m.Resources[i].V2Declared = true
	}
	return m
}

func validateFormalApplyMapping(m mappingFile) error {
	if mappingVersion(m) != workflowGroupsMappingV2 {
		return fmt.Errorf("--apply requires mapping version 2; version 1 is accepted only for dry-run compatibility")
	}
	return nil
}

func validateReviewPolicyIDs(path string, policyIDs []string) error {
	if len(policyIDs) == 0 {
		return fmt.Errorf("%s: review_policy_ids must contain at least one explicit policy", path)
	}
	knownOrder := make(map[string]int, len(knownReviewPolicyIDs))
	for index, policyID := range knownReviewPolicyIDs {
		knownOrder[policyID] = index
	}
	previous := -1
	for index, policyID := range policyIDs {
		if !reviewPolicyIDPattern.MatchString(policyID) {
			return fmt.Errorf("%s.review_policy_ids[%d]: policy id must use safe lowercase snake_case", path, index)
		}
		order, known := knownOrder[policyID]
		if !known {
			return fmt.Errorf("%s.review_policy_ids[%d]: unknown review policy %q", path, index, policyID)
		}
		if order <= previous {
			return fmt.Errorf("%s.review_policy_ids: policies must be unique and in canonical order", path)
		}
		previous = order
	}
	return nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func (m resourceMapping) isV2() bool { return len(m.History) > 0 || m.V2Declared }

func (m resourceMapping) effectiveTargetStatus() string {
	if !m.isV2() {
		return m.TargetStatus
	}
	if m.FinalizedRevisionNo != nil {
		if revision := m.revisionByNo(*m.FinalizedRevisionNo); revision != nil {
			return revision.Status
		}
	}
	if m.WorkingRevisionNo != nil {
		if revision := m.revisionByNo(*m.WorkingRevisionNo); revision != nil {
			return revision.Status
		}
	}
	return "shell"
}

func (m resourceMapping) revisionByNo(revisionNo int) *resourceRevisionMapping {
	for i := range m.History {
		if m.History[i].RevisionNo == revisionNo {
			return &m.History[i]
		}
	}
	return nil
}

func (m resourceMapping) mappedAssetIDs() []int64 {
	if !m.isV2() {
		result := append([]int64(nil), m.FinalAssetIDs...)
		if m.SourceAssetID != nil {
			result = append(result, *m.SourceAssetID)
		}
		return uniqueSortedInt64(result)
	}
	result := []int64{}
	for _, revision := range m.History {
		result = append(result, revision.FinalAssetIDs...)
		if revision.SourceAssetID != nil {
			result = append(result, *revision.SourceAssetID)
		}
		if revision.SourceBundle != nil {
			result = append(result, revision.SourceBundle.TaskAssetID)
			for _, member := range revision.SourceBundle.Members {
				result = append(result, member.TaskAssetID)
			}
		}
	}
	return uniqueSortedInt64(result)
}

func (m resourceMapping) mappedReferenceIDs() []int64 {
	if !m.isV2() {
		return uniqueSortedInt64(m.ReferenceIDs)
	}
	result := []int64{}
	for _, revision := range m.History {
		result = append(result, revision.ReferenceIDs...)
	}
	return uniqueSortedInt64(result)
}

func validateResourceMappingV2(index int, r resourceMapping) error {
	return validateResourceMappingV2Mode(index, r, false)
}

func validateCandidateResourceMappingV2(index int, r resourceMapping) error {
	return validateResourceMappingV2Mode(index, r, true)
}

func validateResourceMappingV2Mode(index int, r resourceMapping, allowCandidateConfidence bool) error {
	if r.TaskID <= 0 {
		return fmt.Errorf("resources[%d]: task_id is required", index)
	}
	if r.ScopeKind != "task" && r.ScopeKind != "sku" && r.ScopeKind != "retouch_requirement" {
		return fmt.Errorf("resources[%d]: invalid scope_kind", index)
	}
	if (r.ScopeKind == "task" && r.ScopeRefID != 0) || (r.ScopeKind != "task" && r.ScopeRefID <= 0) {
		return fmt.Errorf("resources[%d]: invalid scope_ref_id", index)
	}
	if r.Mode != "" || r.SourceAssetID != nil || len(r.FinalAssetIDs) != 0 || len(r.ReferenceIDs) != 0 || r.CreatedBy != 0 || r.TargetStatus != "" || r.Reason != "" {
		return fmt.Errorf("resources[%d]: version 2 resources must use history[] instead of version 1 revision fields", index)
	}
	if len(r.History) == 0 {
		if r.WorkingRevisionNo != nil || r.FinalizedRevisionNo != nil {
			return fmt.Errorf("resources[%d]: shell history cannot declare working/finalized pointers", index)
		}
		return nil
	}
	customizationTerminalPolicy := false
	for _, revision := range r.History {
		customizationTerminalPolicy = customizationTerminalPolicy ||
			containsString(revision.ReviewPolicyIDs, reviewPolicyLegacyCustomizationTerminalNoAssets)
	}
	if customizationTerminalPolicy {
		if r.TaskID == 3091 {
			if !isLegacyCompletedCustomizationMissingFinalResource(r) {
				return fmt.Errorf("resources[%d]: completed customization missing-final policy does not match the frozen four-revision contract", index)
			}
		} else {
			expectedSource, allowed := legacyCustomizationTerminalExpectedSource(r.TaskID, r.ScopeKind, r.ScopeRefID)
			if !allowed || len(r.History) != 1 || r.WorkingRevisionNo == nil || *r.WorkingRevisionNo != 1 || r.FinalizedRevisionNo != nil {
				return fmt.Errorf("resources[%d]: customization terminal policy requires one exact allowlisted working draft and no finalized pointer", index)
			}
			revision := r.History[0]
			expectedPolicies := []string{
				reviewPolicyExplicitEventReplay,
				reviewPolicyReopen,
				reviewPolicyLegacyCustomizationTerminalNoAssets,
			}
			if revision.Status != "draft" ||
				revision.SourceStage != "reopen" ||
				!equalInt64Pointers(revision.SourceAssetID, expectedSource) ||
				revision.SourceAliasFrom != nil ||
				revision.SourceBundle != nil ||
				len(revision.FinalAssetIDs) != 0 ||
				!equalStringSlices(revision.ReviewPolicyIDs, expectedPolicies) ||
				!strings.HasPrefix(revision.Reason, "policy "+reviewPolicyLegacyCustomizationTerminalNoAssets+":") {
				return fmt.Errorf("resources[%d]: customization terminal policy draft does not match the frozen source/final contract", index)
			}
		}
	}
	retouchVisualPolicy := false
	for _, revision := range r.History {
		retouchVisualPolicy = retouchVisualPolicy ||
			containsString(revision.ReviewPolicyIDs, reviewPolicyLegacyRetouchVisualScopeTask2533)
	}
	if retouchVisualPolicy {
		expected, allowed := legacyRetouchVisualExpected(r.TaskID, r.ScopeKind, r.ScopeRefID)
		if !allowed ||
			len(r.History) != 1 ||
			r.WorkingRevisionNo == nil || *r.WorkingRevisionNo != 1 ||
			r.FinalizedRevisionNo == nil || *r.FinalizedRevisionNo != 1 {
			return fmt.Errorf("resources[%d]: retouch visual-scope policy requires one exact allowlisted finalized task 2533 revision", index)
		}
		revision := r.History[0]
		expectedPolicies := []string{
			reviewPolicyExplicitEventReplay,
			reviewPolicyLegacyRetouchVisualScopeTask2533,
		}
		if revision.Status != "finalized" ||
			revision.SourceStage != "retouch" ||
			revision.Mode != "single" ||
			revision.SourceAssetID == nil ||
			*revision.SourceAssetID != expected.sourceID ||
			revision.SourceAliasFrom != nil ||
			revision.SourceBundle != nil ||
			!equalInt64Slices(revision.FinalAssetIDs, []int64{expected.finalID}) ||
			!equalInt64Slices(revision.ReferenceIDs, expected.referenceIDs) ||
			!equalStringSlices(revision.ReviewPolicyIDs, expectedPolicies) ||
			!strings.HasPrefix(revision.Reason, "policy "+reviewPolicyLegacyRetouchVisualScopeTask2533+":") {
			return fmt.Errorf("resources[%d]: retouch visual-scope policy does not match the exact task 2533 source/final/reference contract", index)
		}
	}
	seenAssets := map[int64]string{}
	seenEvidence := map[string]int{}
	for revisionIndex := range r.History {
		revision := &r.History[revisionIndex]
		path := fmt.Sprintf("resources[%d].history[%d]", index, revisionIndex)
		if revision.RevisionNo != revisionIndex+1 {
			return fmt.Errorf("%s: revision_no must be contiguous and ordered from 1", path)
		}
		if err := validateRevisionMappingMode(path, revision, allowCandidateConfidence); err != nil {
			return err
		}
		assetIDs := append([]int64(nil), revision.FinalAssetIDs...)
		if revision.SourceAssetID != nil {
			assetIDs = append(assetIDs, *revision.SourceAssetID)
		}
		if revision.SourceBundle != nil {
			assetIDs = append(assetIDs, revision.SourceBundle.TaskAssetID)
			for _, member := range revision.SourceBundle.Members {
				assetIDs = append(assetIDs, member.TaskAssetID)
			}
		}
		for _, assetID := range assetIDs {
			role := "final"
			if (revision.SourceAssetID != nil && assetID == *revision.SourceAssetID) || (revision.SourceBundle != nil && assetID == revision.SourceBundle.TaskAssetID) {
				role = "source"
			}
			if revision.SourceBundle != nil {
				for _, member := range revision.SourceBundle.Members {
					if assetID == member.TaskAssetID {
						role = "source"
					}
				}
			}
			if priorRole, exists := seenAssets[assetID]; exists && priorRole != role {
				return fmt.Errorf("%s: task asset %d changes role across history (%s to %s)", path, assetID, priorRole, role)
			}
			seenAssets[assetID] = role
		}
		for _, eventID := range revision.EvidenceEventIDs {
			if prior, exists := seenEvidence[eventID]; exists {
				previous := r.History[prior-1]
				sharedRejectionBoundary := revision.RevisionNo == prior+1 && previous.Status == "rejected" && revision.SourceStage == "reopen"
				inheritedPostCloseEvidence := revision.SourceStage == "reopen" &&
					containsString(revision.ReviewPolicyIDs, reviewPolicyLegacyPostCloseReplacement) &&
					strings.HasPrefix(revision.Reason, "policy legacy_post_close_replacement_v1:")
				inheritedSnapshotEvidence := (revision.SourceStage == "audit" || revision.SourceStage == "reopen") &&
					containsString(revision.ReviewPolicyIDs, reviewPolicyExplicitEventReplay)
				if !sharedRejectionBoundary && !inheritedPostCloseEvidence && !inheritedSnapshotEvidence {
					return fmt.Errorf("%s: evidence event %q is already assigned to revision %d", path, eventID, prior)
				}
			}
			seenEvidence[eventID] = revision.RevisionNo
		}
		if revisionIndex > 0 {
			previous := r.History[revisionIndex-1]
			if err := validateRevisionTransition(path, previous, *revision); err != nil {
				return err
			}
		}
	}
	if r.WorkingRevisionNo == nil && r.FinalizedRevisionNo == nil {
		return fmt.Errorf("resources[%d]: non-empty history requires a working or finalized revision pointer", index)
	}
	if r.WorkingRevisionNo != nil {
		revision := r.revisionByNo(*r.WorkingRevisionNo)
		if revision == nil || (revision.Status != "draft" && revision.Status != "submitted" && revision.Status != "finalized") {
			return fmt.Errorf("resources[%d]: working_revision_no must reference a draft, submitted, or finalized history row", index)
		}
	}
	if r.FinalizedRevisionNo != nil {
		revision := r.revisionByNo(*r.FinalizedRevisionNo)
		if revision == nil || revision.Status != "finalized" {
			return fmt.Errorf("resources[%d]: finalized_revision_no must reference a finalized history row", index)
		}
	}
	lastRevisionNo := len(r.History)
	if r.WorkingRevisionNo != nil && *r.WorkingRevisionNo != lastRevisionNo {
		return fmt.Errorf("resources[%d]: working_revision_no must reference the latest history row", index)
	}
	for _, revision := range r.History {
		isWorking := r.WorkingRevisionNo != nil && revision.RevisionNo == *r.WorkingRevisionNo
		isFinalized := r.FinalizedRevisionNo != nil && revision.RevisionNo == *r.FinalizedRevisionNo
		if !isWorking && !isFinalized && revision.Status != "rejected" && revision.Status != "superseded" {
			return fmt.Errorf("resources[%d]: non-current revision %d must be rejected or superseded", index, revision.RevisionNo)
		}
	}
	return nil
}

func validateRevisionMapping(path string, revision *resourceRevisionMapping) error {
	return validateRevisionMappingMode(path, revision, false)
}

func validateRevisionMappingMode(path string, revision *resourceRevisionMapping, allowCandidateConfidence bool) error {
	if revision == nil || revision.RevisionNo <= 0 {
		return fmt.Errorf("%s: revision_no is required", path)
	}
	switch revision.Status {
	case "draft", "submitted", "finalized", "rejected", "superseded":
	default:
		return fmt.Errorf("%s: invalid status", path)
	}
	if revision.Mode != "single" && revision.Mode != "set" {
		return fmt.Errorf("%s: mode must be single or set", path)
	}
	hardCandidate := allowCandidateConfidence && revision.Confidence == "hard_blocked"
	switch revision.SourceStage {
	case "design", "audit", "retouch", "migration", "reopen":
	default:
		return fmt.Errorf("%s: invalid source_stage", path)
	}
	if err := validateReviewPolicyIDs(path, revision.ReviewPolicyIDs); err != nil {
		return err
	}
	if strings.HasPrefix(revision.Reason, "policy "+reviewPolicyLegacyMultiSKUAtomicBatchSubmit+":") &&
		!containsString(revision.ReviewPolicyIDs, reviewPolicyLegacyMultiSKUAtomicBatchSubmit) {
		return fmt.Errorf("%s: batch-submit policy reason requires review_policy_ids to include %s", path, reviewPolicyLegacyMultiSKUAtomicBatchSubmit)
	}
	if strings.HasPrefix(revision.Reason, "policy "+reviewPolicyLegacyRetouchTerminalSubmit+":") &&
		!containsString(revision.ReviewPolicyIDs, reviewPolicyLegacyRetouchTerminalSubmit) {
		return fmt.Errorf("%s: retouch-terminal policy reason requires review_policy_ids to include %s", path, reviewPolicyLegacyRetouchTerminalSubmit)
	}
	if err := validateRevisionStageStatus(path, revision.SourceStage, revision.Status); err != nil {
		return err
	}
	sourceForms := 0
	if revision.SourceAssetID != nil {
		sourceForms++
	}
	if revision.SourceAliasFrom != nil {
		sourceForms++
	}
	if revision.SourceBundle != nil {
		sourceForms++
	}
	if sourceForms > 1 {
		return fmt.Errorf("%s: source_task_asset_id, source_alias_from_task_asset_id and source_bundle are mutually exclusive", path)
	}
	if revision.SourceAliasFrom != nil {
		if *revision.SourceAliasFrom <= 0 {
			return fmt.Errorf("%s: source_alias_from_task_asset_id must be positive", path)
		}
		if revision.Status == "finalized" && !hardCandidate && !containsInt64(revision.FinalAssetIDs, *revision.SourceAliasFrom) {
			return fmt.Errorf("%s: source alias origin must remain in final_task_asset_ids", path)
		}
	}
	if revision.SourceBundle != nil {
		if err := validateSourceBundle(path+".source_bundle", revision.SourceBundle); err != nil {
			return err
		}
	}
	if !hardCandidate && (revision.CreatedBy <= 0 || revision.CreatedAt.IsZero()) {
		return fmt.Errorf("%s: created_by and created_at are required", path)
	}
	switch revision.Confidence {
	case "confirmed_auto":
		if len(revision.Blockers) != 0 {
			return fmt.Errorf("%s: confirmed_auto cannot retain candidate blockers", path)
		}
		if revision.ConfirmedBy <= 0 || revision.ConfirmedAt.IsZero() || strings.TrimSpace(revision.ConfirmationNote) == "" {
			return fmt.Errorf("%s: confirmed_auto requires complete human confirmation metadata", path)
		}
	case "proposed_review", "hard_blocked":
		if !allowCandidateConfidence {
			return fmt.Errorf("%s: confidence=%s cannot be applied", path, revision.Confidence)
		}
	default:
		return fmt.Errorf("%s: confidence must be confirmed_auto, proposed_review, or hard_blocked", path)
	}
	if len(revision.EvidenceEventIDs) == 0 {
		return fmt.Errorf("%s: evidence_event_ids must contain at least one reviewed legacy event", path)
	}
	if err := validateEvidenceIDs(path, revision.EvidenceEventIDs); err != nil {
		return err
	}
	if !hardCandidate && revision.Status == "submitted" && revision.SubmittedAt == nil {
		return fmt.Errorf("%s: submitted revision requires submitted_at", path)
	}
	if !hardCandidate && revision.Status == "finalized" && (revision.SubmittedAt == nil || revision.FinalizedAt == nil) {
		return fmt.Errorf("%s: finalized revision requires submitted_at and finalized_at", path)
	}
	if revision.SubmittedAt != nil && revision.SubmittedAt.Before(revision.CreatedAt) {
		return fmt.Errorf("%s: submitted_at cannot precede created_at", path)
	}
	if revision.FinalizedAt != nil && (revision.SubmittedAt == nil || revision.FinalizedAt.Before(*revision.SubmittedAt)) {
		return fmt.Errorf("%s: finalized_at cannot precede submitted_at", path)
	}
	seenFinals := map[int64]struct{}{}
	for _, assetID := range revision.FinalAssetIDs {
		if assetID <= 0 {
			return fmt.Errorf("%s: final asset ids must be positive", path)
		}
		if _, duplicate := seenFinals[assetID]; duplicate {
			return fmt.Errorf("%s: final asset ids must be unique and ordered", path)
		}
		seenFinals[assetID] = struct{}{}
		if revision.SourceAssetID != nil && assetID == *revision.SourceAssetID {
			return fmt.Errorf("%s: source asset cannot also be a final asset", path)
		}
		if revision.SourceBundle != nil && assetID == revision.SourceBundle.TaskAssetID {
			return fmt.Errorf("%s: source bundle asset cannot also be a final asset", path)
		}
		if revision.SourceBundle != nil {
			for _, member := range revision.SourceBundle.Members {
				if assetID == member.TaskAssetID {
					return fmt.Errorf("%s: source bundle member cannot also be a final asset", path)
				}
			}
		}
	}
	// Design-only historical submissions legitimately have no final files. All
	// other submitted/finalized rows must preserve the mode cardinality.
	if !hardCandidate && revision.Status != "draft" && !(revision.SourceStage == "design" && len(revision.FinalAssetIDs) == 0) {
		if (revision.Mode == "single" && len(revision.FinalAssetIDs) != 1) || (revision.Mode == "set" && len(revision.FinalAssetIDs) < 2) {
			return fmt.Errorf("%s: single requires one final and set requires at least two", path)
		}
	}
	expectedHash, err := revisionManifestRowHash(*revision)
	if err != nil {
		return fmt.Errorf("%s: compute manifest_row_hash: %w", path, err)
	}
	if !sha256Pattern.MatchString(revision.ManifestRowHash) || revision.ManifestRowHash != expectedHash {
		return fmt.Errorf("%s: manifest_row_hash does not match canonical revision content", path)
	}
	if !(allowCandidateConfidence && revision.Confidence != "confirmed_auto") {
		if _, err := persistedRevisionReason(*revision); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
	return nil
}

func validateRevisionStageStatus(path, stage, status string) error {
	allowed := false
	switch stage {
	case "design", "retouch":
		allowed = status == "submitted" || status == "finalized" || status == "rejected" || status == "superseded"
	case "audit":
		allowed = status == "finalized" || status == "rejected" || status == "superseded"
	case "reopen":
		allowed = status == "draft" || status == "submitted" || status == "finalized" || status == "rejected" || status == "superseded"
	case "migration":
		allowed = true
	}
	if !allowed {
		return fmt.Errorf("%s: source_stage=%s cannot persist with status=%s", path, stage, status)
	}
	return nil
}

func containsInt64(values []int64, target int64) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func equalInt64Pointers(left, right *int64) bool {
	return (left == nil && right == nil) ||
		(left != nil && right != nil && *left == *right)
}

func equalStringSlices(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func validateRevisionTransition(path string, previous, current resourceRevisionMapping) error {
	if current.CreatedAt.Before(previous.CreatedAt) {
		return fmt.Errorf("%s: created_at cannot precede revision %d", path, previous.RevisionNo)
	}
	switch previous.Status {
	case "rejected":
		if current.SourceStage != "reopen" && current.SourceStage != "design" && current.SourceStage != "retouch" {
			return fmt.Errorf("%s: a rejected revision must be followed by a reopen/design/retouch revision", path)
		}
	case "finalized":
		if current.SourceStage != "reopen" || current.Status == "finalized" {
			return fmt.Errorf("%s: a retained finalized revision may only be followed by a non-final reopen revision", path)
		}
	case "draft", "submitted":
		return fmt.Errorf("%s: non-terminal revision %d cannot be followed by another history row", path, previous.RevisionNo)
	}
	return nil
}

func validateEvidenceIDs(path string, evidenceIDs []string) error {
	seenEvidence := map[string]struct{}{}
	for _, eventID := range evidenceIDs {
		parts := strings.SplitN(strings.TrimSpace(eventID), ":", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" || (parts[0] != "task_event_log" && parts[0] != "task_module_event") {
			return fmt.Errorf("%s: evidence ids must use task_event_log:<id> or task_module_event:<id>", path)
		}
		if _, duplicate := seenEvidence[eventID]; duplicate {
			return fmt.Errorf("%s: evidence_event_ids must be unique and ordered", path)
		}
		seenEvidence[eventID] = struct{}{}
	}
	return nil
}

func validateTaskStateDecisions(m mappingFile, allowCandidateConfidence bool) error {
	if len(m.TaskDecisions) == 0 {
		return nil
	}
	if mappingVersion(m) != workflowGroupsMappingV2 {
		return fmt.Errorf("task_state_decisions require mapping version 2")
	}
	seen := map[int64]struct{}{}
	for index, decision := range m.TaskDecisions {
		path := fmt.Sprintf("task_state_decisions[%d]", index)
		if decision.TaskID <= 0 {
			return fmt.Errorf("%s: task_id is required", path)
		}
		if err := validateReviewPolicyIDs(path, decision.ReviewPolicyIDs); err != nil {
			return err
		}
		warehouseDecision := decision.FromStatus == "RejectedByWarehouse" &&
			(decision.TargetStatus == "InProgress" || decision.TargetStatus == "Completed") &&
			containsString(decision.ReviewPolicyIDs, reviewPolicyLegacyWarehouseReopenState)
		retouchDecision := isReviewedRetouchReopenDecision(decision)
		customizationTerminalFromStatus := decision.FromStatus == "PendingWarehouseReceive" ||
			(decision.TaskID == 3091 && decision.FromStatus == "Completed")
		customizationTerminalPolicies := []string{
			reviewPolicyLegacyCustomizationTerminalNoAssets,
		}
		if decision.TaskID == 3091 && decision.FromStatus == "Completed" {
			customizationTerminalPolicies = append(
				customizationTerminalPolicies,
				reviewPolicyLegacyHistoricalAssetUnavailable,
			)
		}
		customizationTerminalDecision := customizationTerminalFromStatus &&
			decision.TargetStatus == "InProgress" &&
			isLegacyCustomizationTerminalTask(decision.TaskID) &&
			equalStringSlices(
				decision.ReviewPolicyIDs,
				customizationTerminalPolicies,
			)
		if !warehouseDecision && !retouchDecision && !customizationTerminalDecision {
			return fmt.Errorf("%s: unsupported or insufficiently policy-bound task state transition", path)
		}
		if _, duplicate := seen[decision.TaskID]; duplicate {
			return fmt.Errorf("%s: duplicate task_id", path)
		}
		seen[decision.TaskID] = struct{}{}
		switch decision.Confidence {
		case "confirmed_auto":
			if len(decision.Blockers) != 0 || decision.ConfirmedBy <= 0 || decision.ConfirmedAt.IsZero() || strings.TrimSpace(decision.ConfirmationNote) == "" {
				return fmt.Errorf("%s: confirmed decision requires no blockers and complete human confirmation metadata", path)
			}
		case "proposed_review", "hard_blocked":
			if !allowCandidateConfidence {
				return fmt.Errorf("%s: confidence=%s cannot be applied", path, decision.Confidence)
			}
			if decision.Confidence == "hard_blocked" && len(decision.Blockers) == 0 {
				return fmt.Errorf("%s: hard_blocked decision requires blockers", path)
			}
		default:
			return fmt.Errorf("%s: confidence must be confirmed_auto, proposed_review, or hard_blocked", path)
		}
		if len(decision.EvidenceEventIDs) == 0 {
			return fmt.Errorf("%s: evidence_event_ids are required", path)
		}
		if err := validateEvidenceIDs(path, decision.EvidenceEventIDs); err != nil {
			return err
		}
		expected, err := taskStateDecisionManifestHash(decision)
		if err != nil {
			return err
		}
		if !sha256Pattern.MatchString(decision.ManifestRowHash) || decision.ManifestRowHash != expected {
			return fmt.Errorf("%s: manifest_row_hash does not match canonical decision content", path)
		}
	}
	return nil
}

type frozenAssetRecoveryEvidence struct {
	TaskID                     int64
	SourceTaskAssetID          int64
	OriginalStorageRefID       string
	SourceStorageRefID         string
	FileSize                   int64
	PreviewWholeHash           string
	DesignThumbWholeHash       string
	RejectedSourceTaskAssetIDs []int64
	RootAssetID                int64
	OriginalStorageRefKey      string
	ObjectProbeResult          string
	ObjectProbeInputSHA256     string
	ObjectProbeEvidenceHash    string
	ObjectProbeObjectKeySHA256 string
	ObjectProbeReadOnlyGETs    int
	RecoverySourceSHA256       string
}

var frozenAssetRecoveryEvidenceByMissingID = map[int64]frozenAssetRecoveryEvidence{
	23989: {
		TaskID: 2807, SourceTaskAssetID: 24034,
		OriginalStorageRefID: "f511c5d4-507f-4a69-bf10-70bae369429d",
		SourceStorageRefID:   "983a746c-c674-4f5c-8812-073be989b194",
		FileSize:             683001,
		PreviewWholeHash:     "471739776f4c230a80ae5514e83e92fd3f1e104d203ced3ac793c65c25a525e4",
		DesignThumbWholeHash: "3442c0ac91eb61371d4057d6c0de232f8ba4f3c25cb6b68cff63142aa155e6ef",
		RecoverySourceSHA256: "d0558b1a9d4a7afed5a03b6b97d4a765d34050866686e396ab0acf9f08f0dec5",
	},
	23990: {
		TaskID: 2807, SourceTaskAssetID: 24033,
		OriginalStorageRefID: "ca292dff-6824-4fe9-89cf-e439254f4383",
		SourceStorageRefID:   "85c01c4c-0e27-4df4-a851-4b888f54a837",
		FileSize:             689291,
		PreviewWholeHash:     "311d508fde06f4b7ae73ebfb915abda67c316f02d6356f052731d818d5e0ca47",
		DesignThumbWholeHash: "7d38a5ff3cc65aa89aa15476e479a5eb0af611c4c60f145bbec40497a00cb62c",
		RecoverySourceSHA256: "64cdfed11adc778fb6ede7f03c49f7c70e8655870236bdcd92a8207e41a8dfb8",
	},
	23991: {
		TaskID: 2807, SourceTaskAssetID: 24040,
		OriginalStorageRefID: "107bbca3-b716-4043-b036-54dab1d52b0d",
		SourceStorageRefID:   "769e687f-fd71-4f37-930c-fd3f566350e6",
		FileSize:             686447,
		PreviewWholeHash:     "e4d8c77d270fb03cbcce3b8285b3373779a231605a09af515d3e2697118370a3",
		DesignThumbWholeHash: "fd4a43d2b1e8cf2013c84a37a948538cc102f28a1a886f6662c50bdc08c5234d",
		RecoverySourceSHA256: "ebfecf3407e05c576bcddf74673d2e7568207ecc27855aa0e08c453d5a0d119a",
	},
	12323: {
		TaskID: 2199, SourceTaskAssetID: 0,
		OriginalStorageRefID:       "c0a135a1-080f-46a0-a41a-461aef0ea0fb",
		FileSize:                   17755216,
		PreviewWholeHash:           "82b35a045540d27f9656d6d02c99eb2814a62e9d048d33b20823fb8c0017aa4c",
		DesignThumbWholeHash:       "54dbf569874243a212c11c3e83e80f19944c2581f12c9473a793bc273ec666a3",
		RejectedSourceTaskAssetIDs: []int64{14510, 14514},
		RootAssetID:                12401,
		OriginalStorageRefKey:      "tasks/RW-20260709-A-002196/assets/AST-0002/v1/delivery/1783575756672661314_d97ed925.psd",
		ObjectProbeResult:          "not_found",
		ObjectProbeInputSHA256:     "3f17b37296d2670235ca9bfcfd4388823b81adecf8fbac0826e6f241923579c7",
		ObjectProbeEvidenceHash:    "f1c78819e1f3d5f4e7a4b25ff3d173368574a5639f4c6df45c8aae5482d047b8",
		ObjectProbeObjectKeySHA256: "e732f6cd269a93d6bac168b0852dbcf8480af8966847278cb073cd6905b0efdd",
		ObjectProbeReadOnlyGETs:    1,
	},
}

func validateAssetRecoveries(m mappingFile, allowCandidateConfidence bool) error {
	if len(m.AssetRecoveries) == 0 {
		return nil
	}
	if mappingVersion(m) != workflowGroupsMappingV2 {
		return fmt.Errorf("asset_recoveries require mapping version 2")
	}
	seen := map[int64]struct{}{}
	for index, recovery := range m.AssetRecoveries {
		path := fmt.Sprintf("asset_recoveries[%d]", index)
		if _, duplicate := seen[recovery.MissingTaskAssetID]; duplicate {
			return fmt.Errorf("%s: duplicate missing_task_asset_id", path)
		}
		seen[recovery.MissingTaskAssetID] = struct{}{}
		expected, known := frozenAssetRecoveryEvidenceByMissingID[recovery.MissingTaskAssetID]
		if !known {
			return fmt.Errorf("%s: missing task asset is outside the frozen recovery evidence set", path)
		}
		expectedStrategy := "verified_oss_recovery_v1"
		expectedPolicy := reviewPolicyLegacyDeletedAssetRecovery
		if recovery.MissingTaskAssetID == 12323 {
			expectedStrategy = "historical_unavailable_tombstone_v1"
			expectedPolicy = reviewPolicyLegacyHistoricalAssetUnavailable
		}
		if recovery.MissingTaskAssetID != 12323 && recovery.Confidence == "confirmed_auto" {
			if recovery.ControlledReadProtocol != "controlled-asset-read-v1" ||
				recovery.ControlledReadEvidenceHash != "b39e0d9d26e6fdd35941b195bdc413eb12dd6795e23276a48c9b9bd49f829b08" ||
				recovery.RecoverySourceSHA256 != expected.RecoverySourceSHA256 {
				return fmt.Errorf("%s: controlled-read evidence is incomplete or differs from the frozen recovery contract", path)
			}
		}
		if recovery.TaskID != expected.TaskID ||
			recovery.RecoverySourceTaskAssetID != expected.SourceTaskAssetID ||
			recovery.Strategy != expectedStrategy ||
			recovery.OriginalStorageRefID != expected.OriginalStorageRefID ||
			recovery.RecoverySourceStorageRefID != expected.SourceStorageRefID ||
			recovery.ExpectedFileSize != expected.FileSize ||
			recovery.PreviewWholeHash != expected.PreviewWholeHash ||
			recovery.DesignThumbWholeHash != expected.DesignThumbWholeHash ||
			!equalInt64Slices(recovery.RejectedSourceTaskAssetIDs, expected.RejectedSourceTaskAssetIDs) {
			return fmt.Errorf("%s: recovery evidence differs from the frozen size/derivative-hash contract", path)
		}
		if recovery.MissingTaskAssetID == 12323 &&
			(recovery.ObjectProbeResult != expected.ObjectProbeResult ||
				recovery.ObjectProbeInputSHA256 != expected.ObjectProbeInputSHA256 ||
				recovery.ObjectProbeEvidenceHash != expected.ObjectProbeEvidenceHash ||
				recovery.ObjectProbeObjectKeySHA256 != expected.ObjectProbeObjectKeySHA256 ||
				recovery.ObjectProbeReadOnlyGETs != expected.ObjectProbeReadOnlyGETs) {
			return fmt.Errorf("%s: historical-unavailable tombstone lacks the frozen read-only object-absence probe binding", path)
		}
		if recovery.MissingTaskAssetID == 12323 {
			objectKeyHash := sha256.Sum256([]byte(expected.OriginalStorageRefKey))
			if hex.EncodeToString(objectKeyHash[:]) != recovery.ObjectProbeObjectKeySHA256 {
				return fmt.Errorf("%s: object probe key hash does not identify the frozen original storage ref key", path)
			}
		}
		if !equalStringSlices(recovery.ReviewPolicyIDs, []string{expectedPolicy}) {
			return fmt.Errorf("%s: review_policy_ids must contain only %s", path, expectedPolicy)
		}
		if err := validateReviewPolicyIDs(path, recovery.ReviewPolicyIDs); err != nil {
			return err
		}
		if recovery.MissingTaskAssetID == 12323 {
			switch recovery.Confidence {
			case "proposed_review":
				if !allowCandidateConfidence || len(recovery.Blockers) != 0 {
					return fmt.Errorf("%s: proposed historical-unavailable tombstone requires candidate mode and no synthetic blockers", path)
				}
			case "confirmed_auto":
				if len(recovery.Blockers) != 0 ||
					recovery.ConfirmedBy <= 0 ||
					recovery.ConfirmedAt.IsZero() ||
					strings.TrimSpace(recovery.ConfirmationNote) == "" {
					return fmt.Errorf("%s: confirmed historical-unavailable tombstone requires no blockers and complete human confirmation metadata", path)
				}
			default:
				return fmt.Errorf("%s: task asset 12323 must be proposed_review or confirmed_auto; size-mismatched successors 14510/14514 are evidence, not recovery sources", path)
			}
		} else if recovery.Confidence == "hard_blocked" && len(recovery.Blockers) == 0 {
			return fmt.Errorf("%s: hard_blocked recovery requires blockers", path)
		}
		switch recovery.Confidence {
		case "proposed_review", "hard_blocked":
			if !allowCandidateConfidence {
				return fmt.Errorf("%s: confidence=%s cannot be applied; pre-materialize bytes under a run-scoped Clone B object root, register rollback-complete storage state, then extend this fail-closed tool", path, recovery.Confidence)
			}
		case "confirmed_auto":
			if len(recovery.Blockers) != 0 ||
				recovery.ConfirmedBy <= 0 ||
				recovery.ConfirmedAt.IsZero() ||
				strings.TrimSpace(recovery.ConfirmationNote) == "" {
				return fmt.Errorf("%s: confirmed recovery requires no blockers and complete human confirmation metadata", path)
			}
		default:
			return fmt.Errorf("%s: confidence must be confirmed_auto, proposed_review, or hard_blocked", path)
		}
		expectedHash, err := assetRecoveryManifestRowHash(recovery)
		if err != nil {
			return fmt.Errorf("%s: compute manifest_row_hash: %w", path, err)
		}
		if !sha256Pattern.MatchString(recovery.ManifestRowHash) || recovery.ManifestRowHash != expectedHash {
			return fmt.Errorf("%s: manifest_row_hash does not match canonical recovery content", path)
		}
	}
	return nil
}

func validateOrganizationMappings(m mappingFile, allowCandidateConfidence bool) error {
	if len(m.OrganizationMappings) == 0 {
		return nil
	}
	if mappingVersion(m) != workflowGroupsMappingV2 {
		return fmt.Errorf("organization_mappings require mapping version 2")
	}
	seen := map[string]struct{}{}
	for index, item := range m.OrganizationMappings {
		path := fmt.Sprintf("organization_mappings[%d]", index)
		if item.SubjectType != "task" && item.SubjectType != "user" {
			return fmt.Errorf("%s: subject_type must be task or user", path)
		}
		if item.SubjectID <= 0 {
			return fmt.Errorf("%s: subject_id must be positive", path)
		}
		key := fmt.Sprintf("%s/%d", item.SubjectType, item.SubjectID)
		if _, duplicate := seen[key]; duplicate {
			return fmt.Errorf("%s: duplicate subject %s", path, key)
		}
		seen[key] = struct{}{}
		if err := validateReviewPolicyIDs(path, item.ReviewPolicyIDs); err != nil {
			return err
		}
		if containsString(item.ReviewPolicyIDs, reviewPolicyLegacyUATOrphanOrgToUnassigned) &&
			(item.SubjectType != "task" ||
				(item.SubjectID != 463 && item.SubjectID != 464) ||
				item.TargetDepartmentID != 3 ||
				item.TargetTeamID != 14) {
			return fmt.Errorf("%s: %s is restricted to tasks 463/464 and target department/team 3/14", path, reviewPolicyLegacyUATOrphanOrgToUnassigned)
		}
		hardCandidate := allowCandidateConfidence && item.Confidence == "hard_blocked"
		if !hardCandidate && (item.TargetDepartmentID <= 0 || item.TargetTeamID <= 0) {
			return fmt.Errorf("%s: target department/team ids must be positive", path)
		}
		switch item.Confidence {
		case "confirmed_auto":
			if len(item.Blockers) != 0 {
				return fmt.Errorf("%s: confirmed mapping cannot retain blockers", path)
			}
			if item.ConfirmedBy <= 0 || item.ConfirmedAt.IsZero() || strings.TrimSpace(item.ConfirmationNote) == "" {
				return fmt.Errorf("%s: confirmed mapping requires complete confirmation metadata", path)
			}
		case "proposed_review", "hard_blocked":
			if !allowCandidateConfidence {
				return fmt.Errorf("%s: confidence=%s cannot be applied", path, item.Confidence)
			}
			if item.Confidence == "hard_blocked" && len(item.Blockers) == 0 {
				return fmt.Errorf("%s: hard_blocked requires blockers", path)
			}
		default:
			return fmt.Errorf("%s: invalid confidence", path)
		}
		expected, err := organizationManifestRowHash(item)
		if err != nil {
			return fmt.Errorf("%s: compute manifest_row_hash: %w", path, err)
		}
		if !sha256Pattern.MatchString(item.ManifestRowHash) || item.ManifestRowHash != expected {
			return fmt.Errorf("%s: manifest_row_hash does not match canonical organization content", path)
		}
	}
	return nil
}

func validateAccessDecisions(m mappingFile, allowCandidateConfidence bool) error {
	if len(m.AccessDecisions) == 0 {
		return nil
	}
	if mappingVersion(m) != workflowGroupsMappingV2 {
		return fmt.Errorf("access_decisions require mapping version 2")
	}
	seen := map[string]struct{}{}
	for index, item := range m.AccessDecisions {
		path := fmt.Sprintf("access_decisions[%d]", index)
		if item.UserID <= 0 || strings.TrimSpace(item.LegacyRole) == "" {
			return fmt.Errorf("%s: user_id and legacy_role are required", path)
		}
		key := fmt.Sprintf("%d/%s", item.UserID, item.LegacyRole)
		if _, duplicate := seen[key]; duplicate {
			return fmt.Errorf("%s: duplicate user/legacy role %s", path, key)
		}
		seen[key] = struct{}{}
		if err := validateReviewPolicyIDs(path, item.ReviewPolicyIDs); err != nil {
			return err
		}
		hardCandidate := allowCandidateConfidence && item.Confidence == "hard_blocked"
		if !hardCandidate && item.Action != "no_new_grant" && item.Action != "preserve_existing" {
			return fmt.Errorf("%s: action must be no_new_grant or preserve_existing", path)
		}
		seenEvidence := map[string]struct{}{}
		previous := ""
		for evidenceIndex, evidence := range item.RequiredExistingAssignments {
			if strings.TrimSpace(evidence.RoleCode) == "" ||
				!validAccessScopeMode(evidence.ScopeMode) ||
				!validAccessSourceType(evidence.SourceType) ||
				evidence.SourceRefID < 0 {
				return fmt.Errorf("%s.required_existing_assignments[%d]: invalid assignment evidence", path, evidenceIndex)
			}
			evidenceKey := accessAssignmentEvidenceKey(evidence)
			if _, duplicate := seenEvidence[evidenceKey]; duplicate {
				return fmt.Errorf("%s.required_existing_assignments[%d]: duplicate assignment evidence", path, evidenceIndex)
			}
			if evidenceKey <= previous {
				return fmt.Errorf("%s.required_existing_assignments: evidence must be unique and canonically sorted", path)
			}
			seenEvidence[evidenceKey] = struct{}{}
			previous = evidenceKey
		}
		if !hardCandidate && item.Action == "preserve_existing" &&
			len(item.RequiredExistingAssignments) == 0 {
			return fmt.Errorf("%s: preserve_existing access decisions require assignment evidence", path)
		}
		switch item.Confidence {
		case "confirmed_auto":
			if len(item.Blockers) != 0 {
				return fmt.Errorf("%s: confirmed decision cannot retain blockers", path)
			}
			if item.ConfirmedBy <= 0 || item.ConfirmedAt.IsZero() || strings.TrimSpace(item.ConfirmationNote) == "" {
				return fmt.Errorf("%s: confirmed decision requires complete confirmation metadata", path)
			}
		case "proposed_review", "hard_blocked":
			if !allowCandidateConfidence {
				return fmt.Errorf("%s: confidence=%s cannot be applied", path, item.Confidence)
			}
			if item.Confidence == "hard_blocked" && len(item.Blockers) == 0 {
				return fmt.Errorf("%s: hard_blocked requires blockers", path)
			}
		default:
			return fmt.Errorf("%s: invalid confidence", path)
		}
		expected, err := accessDecisionManifestRowHash(item)
		if err != nil {
			return fmt.Errorf("%s: compute manifest_row_hash: %w", path, err)
		}
		if !sha256Pattern.MatchString(item.ManifestRowHash) || item.ManifestRowHash != expected {
			return fmt.Errorf("%s: manifest_row_hash does not match canonical access content", path)
		}
	}
	return nil
}

func validAccessScopeMode(value string) bool {
	switch value {
	case "self", "own_department", "own_team", "selected_org", "global":
		return true
	default:
		return false
	}
}

func validAccessSourceType(value string) bool {
	return value == "direct" || value == "org_policy" || value == "migration"
}

func accessAssignmentEvidenceKey(item accessAssignmentEvidence) string {
	return fmt.Sprintf("%s\x00%s\x00%s\x00%020d", item.RoleCode, item.ScopeMode, item.SourceType, item.SourceRefID)
}

func organizationManifestRowHash(item organizationMapping) (string, error) {
	item.ManifestRowHash = ""
	raw, err := canonicalMappingJSON(item)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func accessDecisionManifestRowHash(item accessDecisionMapping) (string, error) {
	item.ManifestRowHash = ""
	raw, err := canonicalMappingJSON(item)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func taskStateDecisionManifestHash(decision taskStateDecisionMapping) (string, error) {
	decision.ManifestRowHash = ""
	raw, err := canonicalMappingJSON(decision)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func assetRecoveryManifestRowHash(recovery assetRecoveryMapping) (string, error) {
	recovery.ManifestRowHash = ""
	raw, err := canonicalMappingJSON(recovery)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func validateSourceBundle(path string, bundle *sourceBundleMapping) error {
	if bundle == nil || bundle.TaskAssetID <= 0 || bundle.Format != "zip" || !sha256Pattern.MatchString(bundle.BundleSHA256) || bundle.ConfirmedBy <= 0 || bundle.ConfirmedAt.IsZero() || strings.TrimSpace(bundle.Confirmation) == "" {
		return fmt.Errorf("%s: task_asset_id, format=zip, and confirmation metadata are required", path)
	}
	if len(bundle.Members) < 2 {
		return fmt.Errorf("%s: deterministic source bundle requires at least two ordered members", path)
	}
	seen := map[int64]struct{}{}
	for i, member := range bundle.Members {
		if member.TaskAssetID <= 0 || !member.Confirmed || !sha256Pattern.MatchString(member.SHA256) {
			return fmt.Errorf("%s.members[%d]: positive task_asset_id, confirmed=true, and lowercase sha256 are required", path, i)
		}
		if _, duplicate := seen[member.TaskAssetID]; duplicate {
			return fmt.Errorf("%s.members[%d]: duplicate task_asset_id", path, i)
		}
		if member.TaskAssetID == bundle.TaskAssetID {
			return fmt.Errorf("%s.members[%d]: bundle output cannot also be a source member", path, i)
		}
		seen[member.TaskAssetID] = struct{}{}
	}
	expected, err := sourceBundleManifestHash(*bundle)
	if err != nil {
		return err
	}
	if !sha256Pattern.MatchString(bundle.ManifestHash) || bundle.ManifestHash != expected {
		return fmt.Errorf("%s: manifest_sha256 does not match the ordered member manifest", path)
	}
	return nil
}

func sourceBundleManifestHash(bundle sourceBundleMapping) (string, error) {
	payload := struct {
		Format  string               `json:"format"`
		Members []sourceBundleMember `json:"members"`
	}{Format: bundle.Format, Members: bundle.Members}
	raw, err := canonicalMappingJSON(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func revisionManifestRowHash(revision resourceRevisionMapping) (string, error) {
	revision.ManifestRowHash = ""
	raw, err := canonicalMappingJSON(revision)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func planningManifestRowHash(planning planningMapping) (string, error) {
	planning.ManifestRowHash = ""
	raw, err := canonicalMappingJSON(planning)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// canonicalMappingJSON normalizes structs through a JSON value so object keys
// are sorted recursively by encoding/json. This keeps the Go validator byte-
// compatible with the Python manifest generator's sort_keys canonical form.
// UseNumber preserves integer identifiers without float64 coercion.
func canonicalMappingJSON(value interface{}) ([]byte, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var normalized interface{}
	if err := decoder.Decode(&normalized); err != nil {
		return nil, err
	}
	return json.Marshal(normalized)
}

func persistedRevisionReason(revision resourceRevisionMapping) (string, error) {
	evidence := append([]string(nil), revision.EvidenceEventIDs...)
	sort.Strings(evidence)
	metadata := fmt.Sprintf(
		"[migration_v2 manifest=%s confidence=%s confirmed_by=%d confirmed_at=%s evidence_count=%d",
		revision.ManifestRowHash,
		revision.Confidence,
		revision.ConfirmedBy,
		revision.ConfirmedAt.UTC().Format(time.RFC3339),
		len(evidence),
	)
	if len(evidence) > 0 {
		metadata += " first_evidence=" + evidence[0]
	}
	metadata += "]"
	originalReason := strings.TrimSpace(revision.Reason)
	reason := strings.TrimSpace(originalReason + " " + metadata)
	if utf8.RuneCountInString(reason) <= revisionReasonMaxRunes {
		return reason, nil
	}

	reasonSum := sha256.Sum256([]byte(originalReason))
	compactMetadata := fmt.Sprintf(
		"[migration_v2 manifest=%s reason_sha256=%s confidence=%s confirmed_by=%d confirmed_at=%s evidence_count=%d",
		revision.ManifestRowHash,
		hex.EncodeToString(reasonSum[:]),
		revision.Confidence,
		revision.ConfirmedBy,
		revision.ConfirmedAt.UTC().Format(time.RFC3339),
		len(evidence),
	)
	if len(evidence) > 0 {
		compactMetadata += " first_evidence=" + evidence[0]
	}
	compactMetadata += "]"
	if utf8.RuneCountInString(compactMetadata) > revisionReasonMaxRunes {
		return "", fmt.Errorf("revision evidence cannot fit task_asset_group_revisions.reason; retain the mapping artifact and resolve this blocker without truncation")
	}
	return compactMetadata, nil
}
