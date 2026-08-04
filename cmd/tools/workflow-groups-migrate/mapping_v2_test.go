package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"workflow/service"
)

func validV2Revision(t *testing.T) resourceRevisionMapping {
	t.Helper()
	created := time.Date(2026, 7, 1, 1, 2, 3, 0, time.UTC)
	submitted := created.Add(time.Hour)
	finalized := submitted.Add(time.Hour)
	sourceID := int64(11)
	revision := resourceRevisionMapping{
		RevisionNo:       1,
		Status:           "finalized",
		Mode:             "single",
		SourceStage:      "migration",
		SourceAssetID:    &sourceID,
		FinalAssetIDs:    []int64{12},
		ReferenceIDs:     []int64{13},
		EvidenceEventIDs: []string{"task_event_log:event-1"},
		Confidence:       "confirmed_auto",
		ReviewPolicyIDs:  []string{reviewPolicyExplicitEventReplay},
		ConfirmedBy:      21,
		ConfirmedAt:      created,
		ConfirmationNote: "reviewed against the legacy event and stored objects",
		Reason:           "legacy finalized revision",
		CreatedBy:        21,
		CreatedAt:        created,
		SubmittedAt:      &submitted,
		FinalizedAt:      &finalized,
	}
	hash, err := revisionManifestRowHash(revision)
	if err != nil {
		t.Fatal(err)
	}
	revision.ManifestRowHash = hash
	return revision
}

func validV2Planning(t *testing.T) planningMapping {
	t.Helper()
	confirmedAt := time.Date(2026, 7, 1, 1, 2, 3, 0, time.UTC)
	planning := planningMapping{
		TaskID:             70,
		TargetTaskStatus:   "Completed",
		CodeRuleRevisionID: 9,
		CreatedBy:          21,
		Confidence:         "confirmed_auto",
		ReviewPolicyIDs: []string{
			reviewPolicyLegacyPurchaseToSKUPlanning,
			reviewPolicyFrozenSKUPlanningRuleRevision9,
		},
		ConfirmedBy:      21,
		ConfirmedAt:      confirmedAt,
		ConfirmationNote: "reviewed against the frozen planning truth",
		Items: []planningItemMapping{{
			TaskSKUItemID:   701,
			DescriptionSpec: "Blue / XL",
			Quantity:        2,
		}},
	}
	hash, err := planningManifestRowHash(planning)
	if err != nil {
		t.Fatal(err)
	}
	planning.ManifestRowHash = hash
	return planning
}

func validIncompleteUATPlanningTombstone(t *testing.T) planningMapping {
	t.Helper()
	planning := planningMapping{
		TaskID:             497,
		TargetTaskStatus:   "Cancelled",
		CodeRuleRevisionID: 9,
		CreatedBy:          1,
		Confidence:         "confirmed_auto",
		ReviewPolicyIDs: []string{
			reviewPolicyLegacyPurchaseToSKUPlanning,
			reviewPolicyLegacyIncompleteUATPlanningTombstone,
			reviewPolicyFrozenSKUPlanningRuleRevision9,
		},
		ConfirmedBy:      1,
		ConfirmedAt:      time.Date(2026, 7, 23, 1, 2, 3, 0, time.UTC),
		ConfirmationNote: "reviewed as the exact incomplete UAT planning tombstone",
		Items: []planningItemMapping{{
			TaskSKUItemID: 380,
		}},
	}
	hash, err := planningManifestRowHash(planning)
	if err != nil {
		t.Fatal(err)
	}
	planning.ManifestRowHash = hash
	return planning
}

func validV2Resource(t *testing.T) resourceMapping {
	t.Helper()
	pointer := 1
	return resourceMapping{
		TaskID:              7,
		ScopeKind:           "task",
		History:             []resourceRevisionMapping{validV2Revision(t)},
		WorkingRevisionNo:   &pointer,
		FinalizedRevisionNo: &pointer,
		V2Declared:          true,
	}
}

func TestValidateResourceMappingV2AcceptsConfirmedHistory(t *testing.T) {
	if err := validateResourceMappingV2(0, validV2Resource(t)); err != nil {
		t.Fatalf("validateResourceMappingV2() error = %v", err)
	}
}

func TestV2ReviewPolicyIDsFailClosed(t *testing.T) {
	tests := []struct {
		name     string
		policies []string
		reason   string
		want     string
	}{
		{name: "missing", policies: nil, want: "at least one"},
		{name: "unknown", policies: []string{"unknown_policy_v1"}, want: "unknown review policy"},
		{
			name:     "duplicate",
			policies: []string{reviewPolicyExplicitEventReplay, reviewPolicyExplicitEventReplay},
			want:     "unique and in canonical order",
		},
		{
			name:     "batch reason omits batch policy",
			policies: []string{reviewPolicyExplicitEventReplay},
			reason:   "policy legacy_multi_sku_atomic_batch_submit_v1: exact batch",
			want:     "requires review_policy_ids",
		},
		{
			name:     "retouch terminal reason omits terminal policy",
			policies: []string{reviewPolicyExplicitEventReplay, reviewPolicyRetouchSourceOptional},
			reason:   "policy legacy_retouch_terminal_submit_v1: exact terminal submit",
			want:     "requires review_policy_ids",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapping := validV2Resource(t)
			mapping.History[0].ReviewPolicyIDs = tt.policies
			if tt.reason != "" {
				mapping.History[0].Reason = tt.reason
			}
			mapping.History[0].ManifestRowHash, _ = revisionManifestRowHash(mapping.History[0])
			err := validateResourceMappingV2(0, mapping)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("validateResourceMappingV2() error = %v", err)
			}
		})
	}
}

func TestRetouchTerminalPolicyIsCanonicalAndCoveredByManifestHash(t *testing.T) {
	revision := validV2Revision(t)
	baselineHash := revision.ManifestRowHash
	revision.SourceStage = "retouch"
	revision.SourceAssetID = nil
	revision.ReviewPolicyIDs = []string{
		reviewPolicyExplicitEventReplay,
		reviewPolicyRetouchSourceOptional,
		reviewPolicyLegacyRetouchTerminalSubmit,
	}
	revision.Reason = "policy legacy_retouch_terminal_submit_v1: exact terminal submit"
	revision.FinalizedAt = revision.SubmittedAt
	hash, err := revisionManifestRowHash(revision)
	if err != nil {
		t.Fatal(err)
	}
	if hash == baselineHash {
		t.Fatal("review_policy_ids change did not affect manifest row hash")
	}
	revision.ManifestRowHash = hash
	mapping := validV2Resource(t)
	mapping.ScopeKind = "retouch_requirement"
	mapping.ScopeRefID = 31
	mapping.History = []resourceRevisionMapping{revision}
	if err := validateResourceMappingV2(0, mapping); err != nil {
		t.Fatalf("retouch terminal policy fixture rejected: %v", err)
	}
}

func TestV2PlanningPoliciesAndManifestHashAreRequired(t *testing.T) {
	valid := validV2Planning(t)
	if err := validateMapping(mappingFile{Version: 2, Planning: []planningMapping{valid}}); err != nil {
		t.Fatalf("valid planning mapping rejected: %v", err)
	}
	tests := []struct {
		name   string
		mutate func(*planningMapping)
		want   string
	}{
		{
			name: "missing purchase policy",
			mutate: func(planning *planningMapping) {
				planning.ReviewPolicyIDs = []string{reviewPolicyFrozenSKUPlanningRuleRevision9}
			},
			want: reviewPolicyLegacyPurchaseToSKUPlanning,
		},
		{
			name: "unknown policy",
			mutate: func(planning *planningMapping) {
				planning.ReviewPolicyIDs = []string{"unknown_policy_v1"}
			},
			want: "unknown review policy",
		},
		{
			name: "wrong frozen rule",
			mutate: func(planning *planningMapping) {
				planning.CodeRuleRevisionID = 8
			},
			want: "requires code_rule_revision_id=9",
		},
		{
			name: "stale hash",
			mutate: func(planning *planningMapping) {
				planning.Items[0].Quantity = 3
			},
			want: "manifest_row_hash does not match",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			planning := valid
			planning.ReviewPolicyIDs = append([]string(nil), valid.ReviewPolicyIDs...)
			planning.Items = append([]planningItemMapping(nil), valid.Items...)
			tt.mutate(&planning)
			if tt.name != "stale hash" {
				planning.ManifestRowHash, _ = planningManifestRowHash(planning)
			}
			err := validateMapping(mappingFile{Version: 2, Planning: []planningMapping{planning}})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("validateMapping() error = %v", err)
			}
		})
	}
}

func TestV2PlanningTombstoneIsRestrictedToExactUATRecord(t *testing.T) {
	valid := validIncompleteUATPlanningTombstone(t)
	if err := validateMapping(mappingFile{Version: 2, Planning: []planningMapping{valid}}); err != nil {
		t.Fatalf("valid UAT planning tombstone rejected: %v", err)
	}
	tests := []struct {
		name   string
		mutate func(*planningMapping)
	}{
		{name: "other task", mutate: func(planning *planningMapping) { planning.TaskID = 498 }},
		{name: "active target", mutate: func(planning *planningMapping) { planning.TargetTaskStatus = "InProgress" }},
		{name: "other SKU", mutate: func(planning *planningMapping) { planning.Items[0].TaskSKUItemID = 381 }},
		{name: "fabricated description", mutate: func(planning *planningMapping) { planning.Items[0].DescriptionSpec = "guessed" }},
		{name: "fabricated quantity", mutate: func(planning *planningMapping) { planning.Items[0].Quantity = 1 }},
		{name: "extra policy", mutate: func(planning *planningMapping) {
			planning.ReviewPolicyIDs = append(planning.ReviewPolicyIDs, reviewPolicyProductNameDescriptionFallback)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			planning := valid
			planning.ReviewPolicyIDs = append([]string(nil), valid.ReviewPolicyIDs...)
			planning.Items = append([]planningItemMapping(nil), valid.Items...)
			tt.mutate(&planning)
			planning.ManifestRowHash, _ = planningManifestRowHash(planning)
			err := validateMapping(mappingFile{Version: 2, Planning: []planningMapping{planning}})
			if err == nil || !strings.Contains(err.Error(), reviewPolicyLegacyIncompleteUATPlanningTombstone) {
				t.Fatalf("validateMapping() error = %v", err)
			}
		})
	}
}

func TestValidateResourceMappingV2FailsClosedOnUnresolvedConfidence(t *testing.T) {
	for _, confidence := range []string{"proposed_review", "hard_blocked"} {
		t.Run(confidence, func(t *testing.T) {
			mapping := validV2Resource(t)
			mapping.History[0].Confidence = confidence
			hash, err := revisionManifestRowHash(mapping.History[0])
			if err != nil {
				t.Fatal(err)
			}
			mapping.History[0].ManifestRowHash = hash
			err = validateResourceMappingV2(0, mapping)
			if err == nil || !strings.Contains(err.Error(), "cannot be applied") {
				t.Fatalf("validateResourceMappingV2() error = %v", err)
			}
		})
	}
}

func TestCandidateValidationReportsButFormalValidationRejectsUnresolvedConfidence(t *testing.T) {
	revision := validV2Revision(t)
	revision.Confidence = "proposed_review"
	revision.ConfirmedBy = 0
	revision.ConfirmedAt = time.Time{}
	revision.ConfirmationNote = ""
	revision.ManifestRowHash, _ = revisionManifestRowHash(revision)
	working, finalized := 1, 1
	mapping := mappingFile{Version: 2, Resources: []resourceMapping{{
		TaskID: 7, ScopeKind: "task", V2Declared: true, History: []resourceRevisionMapping{revision},
		WorkingRevisionNo: &working, FinalizedRevisionNo: &finalized,
	}}}
	if err := validateCandidateMapping(mapping); err != nil {
		t.Fatalf("candidate validation should accept structurally valid proposed revision: %v", err)
	}
	if err := validateMapping(mapping); err == nil || !strings.Contains(err.Error(), "cannot be applied") {
		t.Fatalf("formal validation error = %v", err)
	}
	issues := collectMappingCandidateIssues(mapping)
	if len(issues) != 1 || issues[0].Confidence != "proposed_review" || issues[0].TaskID != 7 {
		t.Fatalf("candidate issues = %+v", issues)
	}
}

func TestCandidateValidationKeepsHardBlockedIncompleteRevisionReportable(t *testing.T) {
	revision := validV2Revision(t)
	revision.Status = "draft"
	revision.SourceStage = "reopen"
	revision.FinalAssetIDs = nil
	revision.SourceAssetID = nil
	revision.Confidence = "hard_blocked"
	revision.Blockers = []string{"missing reviewed source membership"}
	revision.ConfirmedBy = 0
	revision.ConfirmedAt = time.Time{}
	revision.ConfirmationNote = ""
	revision.CreatedBy = 0
	revision.ManifestRowHash, _ = revisionManifestRowHash(revision)
	working := 1
	mapping := mappingFile{Version: 2, Resources: []resourceMapping{{
		TaskID: 7, ScopeKind: "task", V2Declared: true, History: []resourceRevisionMapping{revision}, WorkingRevisionNo: &working,
	}}}
	if err := validateCandidateMapping(mapping); err != nil {
		t.Fatalf("hard-blocked candidate must remain reportable: %v", err)
	}
	if err := validateMapping(mapping); err == nil {
		t.Fatal("formal apply validation accepted an incomplete hard-blocked revision")
	}
	issues := collectMappingCandidateIssues(mapping)
	if len(issues) != 1 || len(issues[0].Blockers) != 1 || issues[0].Blockers[0] != "missing reviewed source membership" {
		t.Fatalf("candidate blockers were not preserved in report: %+v", issues)
	}
}

func TestValidateResourceMappingV2RequiresNamespacedEvidence(t *testing.T) {
	mapping := validV2Resource(t)
	mapping.History[0].EvidenceEventIDs = []string{"event-1"}
	hash, err := revisionManifestRowHash(mapping.History[0])
	if err != nil {
		t.Fatal(err)
	}
	mapping.History[0].ManifestRowHash = hash
	err = validateResourceMappingV2(0, mapping)
	if err == nil || !strings.Contains(err.Error(), "evidence ids must use") {
		t.Fatalf("validateResourceMappingV2() error = %v", err)
	}
}

func TestValidateResourceMappingV2AllowsPolicyBoundInheritedPostCloseEvidence(t *testing.T) {
	first := validV2Revision(t)
	first.Status = "superseded"
	first.ManifestRowHash, _ = revisionManifestRowHash(first)
	second := validV2Revision(t)
	second.RevisionNo = 2
	second.SourceStage = "reopen"
	second.CreatedAt = first.CreatedAt.Add(3 * time.Hour)
	second.SubmittedAt = timePointer(second.CreatedAt)
	second.FinalizedAt = timePointer(second.CreatedAt)
	second.EvidenceEventIDs = []string{
		first.EvidenceEventIDs[0],
		"task_event_log:post-close-completion",
	}
	second.ReviewPolicyIDs = []string{
		reviewPolicyExplicitEventReplay,
		reviewPolicyReopen,
		reviewPolicyLegacyPostCloseReplacement,
	}
	second.Reason = "policy legacy_post_close_replacement_v1: exact same-root successor"
	second.ManifestRowHash, _ = revisionManifestRowHash(second)
	working, finalized := 2, 2
	mapping := resourceMapping{
		TaskID: 7, ScopeKind: "task", V2Declared: true,
		History:           []resourceRevisionMapping{first, second},
		WorkingRevisionNo: &working, FinalizedRevisionNo: &finalized,
	}
	if err := validateResourceMappingV2(0, mapping); err != nil {
		t.Fatalf("policy-bound inherited post-close evidence: %v", err)
	}
}

func TestValidateResourceMappingV2AllowsInheritedAuditSnapshotEvidence(t *testing.T) {
	first := validV2Revision(t)
	first.Status = "superseded"
	first.ManifestRowHash, _ = revisionManifestRowHash(first)
	second := validV2Revision(t)
	second.RevisionNo = 2
	second.SourceStage = "audit"
	second.CreatedAt = first.CreatedAt.Add(3 * time.Hour)
	second.SubmittedAt = timePointer(second.CreatedAt)
	second.FinalizedAt = timePointer(second.CreatedAt)
	second.EvidenceEventIDs = []string{
		first.EvidenceEventIDs[0],
		"task_event_log:audit-decision",
	}
	second.ManifestRowHash, _ = revisionManifestRowHash(second)
	working, finalized := 2, 2
	mapping := resourceMapping{
		TaskID: 7, ScopeKind: "task", V2Declared: true,
		History:           []resourceRevisionMapping{first, second},
		WorkingRevisionNo: &working, FinalizedRevisionNo: &finalized,
	}
	if err := validateResourceMappingV2(0, mapping); err != nil {
		t.Fatalf("inherited audit snapshot evidence: %v", err)
	}
}

func TestValidateResourceMappingV2RejectsIllegalHistoryTransitions(t *testing.T) {
	tests := []struct {
		name       string
		prior      string
		priorStage string
		next       string
		nextStage  string
		want       string
	}{
		{name: "finalized_to_design_submitted", prior: "finalized", priorStage: "migration", next: "submitted", nextStage: "design", want: "retained finalized"},
		{name: "rejected_to_audit", prior: "rejected", priorStage: "audit", next: "finalized", nextStage: "audit", want: "reopen/design/retouch"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			first := validV2Revision(t)
			first.Status, first.SourceStage = tt.prior, tt.priorStage
			first.ManifestRowHash, _ = revisionManifestRowHash(first)
			second := validV2Revision(t)
			second.RevisionNo, second.Status, second.SourceStage = 2, tt.next, tt.nextStage
			second.CreatedAt = first.CreatedAt.Add(time.Hour)
			second.SubmittedAt = timePointer(second.CreatedAt.Add(time.Hour))
			second.FinalizedAt = timePointer(second.CreatedAt.Add(2 * time.Hour))
			second.EvidenceEventIDs = []string{"task_event_log:event-2"}
			second.ManifestRowHash, _ = revisionManifestRowHash(second)
			working, finalized := 2, 1
			if second.Status == "finalized" {
				finalized = 2
			}
			mapping := resourceMapping{TaskID: 7, ScopeKind: "task", History: []resourceRevisionMapping{first, second}, WorkingRevisionNo: &working, FinalizedRevisionNo: &finalized, V2Declared: true}
			err := validateResourceMappingV2(0, mapping)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("validateResourceMappingV2() error = %v", err)
			}
		})
	}
}

func TestValidateRevisionMappingRejectsDraftDesignStage(t *testing.T) {
	revision := validV2Revision(t)
	revision.Status, revision.SourceStage = "draft", "design"
	revision.SubmittedAt, revision.FinalizedAt = nil, nil
	revision.ManifestRowHash, _ = revisionManifestRowHash(revision)
	if err := validateRevisionMapping("history[0]", &revision); err == nil || !strings.Contains(err.Error(), "cannot persist") {
		t.Fatalf("validateRevisionMapping() error = %v", err)
	}
}

func TestValidateRevisionEventSemantics(t *testing.T) {
	revision := validV2Revision(t)
	revision.SourceStage = "design"
	metadata := []evidenceEventMetadata{{EventType: "task.design.submitted"}, {EventType: "task.audit.approved"}}
	if err := validateRevisionEventSemantics(7, revision, metadata); err != nil {
		t.Fatalf("valid finalized design semantics: %v", err)
	}
	if err := validateRevisionEventSemantics(7, revision, metadata[:1]); err == nil || !strings.Contains(err.Error(), "approval") {
		t.Fatalf("missing approval semantics error = %v", err)
	}
	revision.SourceStage = "reopen"
	if err := validateRevisionEventSemantics(7, revision, []evidenceEventMetadata{{EventType: "task.audit.supplement_uploaded"}}); err != nil {
		t.Fatalf("audit supplement should finalize a reopen revision: %v", err)
	}
}

func TestPayloadContainsAssetVersionIDIsExactAndFailClosed(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    bool
	}{
		{name: "singular version", payload: `{"asset_version_id":11}`, want: true},
		{name: "plural version", payload: `{"asset_version_ids":[10,11]}`, want: true},
		{name: "task asset id", payload: `{"task_asset_id":11}`, want: true},
		{name: "legacy root asset id is not a version", payload: `{"asset_id":11}`, want: false},
		{name: "unrelated numeric value", payload: `{"asset_version_id":10,"count":11}`, want: false},
		{name: "string value is not accepted", payload: `{"asset_version_id":"11"}`, want: false},
		{name: "invalid json", payload: `{`, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := payloadContainsAssetVersionID(tt.payload, 11); got != tt.want {
				t.Fatalf("payloadContainsAssetVersionID(%s, 11) = %v, want %v", tt.payload, got, tt.want)
			}
		})
	}
}

func TestReviewedWarehouseDecisionIsExplicitAndEvidenceBound(t *testing.T) {
	created := time.Date(2026, 7, 1, 1, 0, 0, 0, time.UTC)
	decision := taskStateDecisionMapping{
		TaskID: 7, FromStatus: "RejectedByWarehouse", TargetStatus: "InProgress",
		EvidenceEventIDs: []string{"task_event_log:warehouse-1"}, ConfirmedBy: 21,
		Confidence:      "confirmed_auto",
		ReviewPolicyIDs: []string{reviewPolicyLegacyWarehouseReopenState},
		ConfirmedAt:     created, ConfirmationNote: "warehouse rejection reviewed; reopen design",
	}
	decision.ManifestRowHash, _ = taskStateDecisionManifestHash(decision)
	if err := validateTaskStateDecisions(mappingFile{Version: 2, TaskDecisions: []taskStateDecisionMapping{decision}}, false); err != nil {
		t.Fatalf("validateTaskStateDecisions() error = %v", err)
	}

	finalized := validV2Revision(t)
	draft := validV2Revision(t)
	draft.RevisionNo, draft.Status, draft.SourceStage = 2, "draft", "reopen"
	draft.CreatedAt = finalized.CreatedAt.Add(3 * time.Hour)
	draft.SubmittedAt, draft.FinalizedAt = nil, nil
	draft.EvidenceEventIDs = []string{"task_event_log:warehouse-1"}
	draft.ManifestRowHash, _ = revisionManifestRowHash(draft)
	working, finalizedNo := 2, 1
	resource := resourceMapping{TaskID: 7, ScopeKind: "task", History: []resourceRevisionMapping{finalized, draft}, WorkingRevisionNo: &working, FinalizedRevisionNo: &finalizedNo, V2Declared: true}

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT task_type,task_status FROM tasks").WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"task_type", "task_status"}).AddRow("design_task", "RejectedByWarehouse"))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM users").WithArgs(int64(21)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("SELECT task_id,sequence,event_type").WithArgs("warehouse-1").WillReturnRows(sqlmock.NewRows([]string{"task_id", "sequence", "event_type", "payload", "created_at"}).AddRow(7, 1, "task.status.changed", `{"to":"RejectedByWarehouse"}`, created))
	if issue := validateTaskStateDecisionPreflight(context.Background(), db, decision, []resourceMapping{resource}); issue != nil {
		t.Fatalf("validateTaskStateDecisionPreflight() issue = %+v", issue)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func customizationTerminalDraft(t *testing.T, taskID, sourceID int64) resourceMapping {
	t.Helper()
	revision := validV2Revision(t)
	revision.Status = "draft"
	revision.SourceStage = "reopen"
	revision.SourceAssetID = nil
	if sourceID > 0 {
		revision.SourceAssetID = &sourceID
	}
	revision.FinalAssetIDs = nil
	revision.ReferenceIDs = nil
	revision.EvidenceEventIDs = []string{"task_event_log:customization-approved"}
	revision.Confidence = "proposed_review"
	revision.ReviewPolicyIDs = []string{
		reviewPolicyExplicitEventReplay,
		reviewPolicyReopen,
		reviewPolicyLegacyCustomizationTerminalNoAssets,
	}
	revision.ConfirmedBy = 0
	revision.ConfirmedAt = time.Time{}
	revision.ConfirmationNote = ""
	revision.Reason = "policy " + reviewPolicyLegacyCustomizationTerminalNoAssets + ": exact incomplete customization terminal"
	revision.SubmittedAt = nil
	revision.FinalizedAt = nil
	revision.ManifestRowHash, _ = revisionManifestRowHash(revision)
	working := 1
	return resourceMapping{
		TaskID:            taskID,
		ScopeKind:         "task",
		History:           []resourceRevisionMapping{revision},
		WorkingRevisionNo: &working,
		V2Declared:        true,
	}
}

func TestCustomizationTerminalWithoutAssetsPolicyIsExactAndEvidenceBound(t *testing.T) {
	resource := customizationTerminalDraft(t, 452, 207)
	if err := validateCandidateResourceMappingV2(0, resource); err != nil {
		t.Fatalf("validate exact customization draft: %v", err)
	}
	drifted := resource
	drifted.TaskID = 451
	if err := validateCandidateResourceMappingV2(0, drifted); err == nil || !strings.Contains(err.Error(), "source/final contract") {
		t.Fatalf("source drift error = %v", err)
	}

	confirmedAt := time.Date(2026, 7, 23, 1, 2, 3, 0, time.UTC)
	decision := taskStateDecisionMapping{
		TaskID:           452,
		FromStatus:       "PendingWarehouseReceive",
		TargetStatus:     "InProgress",
		EvidenceEventIDs: []string{"task_event_log:customization-approved"},
		Confidence:       "confirmed_auto",
		ReviewPolicyIDs:  []string{reviewPolicyLegacyCustomizationTerminalNoAssets},
		ConfirmedBy:      1,
		ConfirmedAt:      confirmedAt,
		ConfirmationNote: "reviewed exact incomplete customization terminal",
	}
	decision.ManifestRowHash, _ = taskStateDecisionManifestHash(decision)
	if err := validateTaskStateDecisions(
		mappingFile{Version: 2, TaskDecisions: []taskStateDecisionMapping{decision}},
		false,
	); err != nil {
		t.Fatalf("validate task decision: %v", err)
	}

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT task_type,task_status FROM tasks").
		WithArgs(int64(452)).
		WillReturnRows(sqlmock.NewRows([]string{"task_type", "task_status"}).
			AddRow("original_product_development", "PendingWarehouseReceive"))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM users").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("SELECT task_id,sequence,event_type").
		WithArgs("customization-approved").
		WillReturnRows(sqlmock.NewRows(
			[]string{"task_id", "sequence", "event_type", "payload", "created_at"},
		).AddRow(
			452,
			7,
			"task.customization.reviewed",
			`{"customization_review_decision":"approved","from_task_status":"PendingCustomizationReview","to_task_status":"PendingWarehouseReceive"}`,
			confirmedAt,
		))
	if issue := validateTaskStateDecisionPreflight(
		context.Background(),
		db,
		decision,
		[]resourceMapping{resource},
	); issue != nil {
		t.Fatalf("validateTaskStateDecisionPreflight() issue = %+v", issue)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func retouchVisualTask2533Resource(t *testing.T, scopeID int64) resourceMapping {
	t.Helper()
	expected, ok := legacyRetouchVisualTask2533[scopeID]
	if !ok {
		t.Fatalf("unknown visual scope %d", scopeID)
	}
	created := time.Date(2026, 7, 17, 8, 27, 53, 0, time.UTC)
	sourceID := expected.sourceID
	revision := resourceRevisionMapping{
		RevisionNo:       1,
		Status:           "finalized",
		Mode:             "single",
		SourceStage:      "retouch",
		SourceAssetID:    &sourceID,
		FinalAssetIDs:    []int64{expected.finalID},
		ReferenceIDs:     append([]int64(nil), expected.referenceIDs...),
		EvidenceEventIDs: []string{"task_event_log:completion", "task_event_log:submit"},
		Confidence:       "proposed_review",
		ReviewPolicyIDs: []string{
			reviewPolicyExplicitEventReplay,
			reviewPolicyLegacyRetouchVisualScopeTask2533,
		},
		Reason:      "policy " + reviewPolicyLegacyRetouchVisualScopeTask2533 + ": exact visually reviewed membership",
		CreatedBy:   228,
		CreatedAt:   created,
		SubmittedAt: &created,
		FinalizedAt: &created,
	}
	revision.ManifestRowHash, _ = revisionManifestRowHash(revision)
	pointer := 1
	return resourceMapping{
		TaskID:              2533,
		ScopeKind:           "retouch_requirement",
		ScopeRefID:          scopeID,
		History:             []resourceRevisionMapping{revision},
		WorkingRevisionNo:   &pointer,
		FinalizedRevisionNo: &pointer,
		V2Declared:          true,
	}
}

func TestRetouchVisualTask2533PolicyIsExactAndComplete(t *testing.T) {
	resources := make([]resourceMapping, 0, len(legacyRetouchVisualTask2533))
	for _, scopeID := range []int64{183, 184, 185, 186, 187} {
		resource := retouchVisualTask2533Resource(t, scopeID)
		if err := validateCandidateResourceMappingV2(0, resource); err != nil {
			t.Fatalf("validate exact visual resource %d: %v", scopeID, err)
		}
		resources = append(resources, resource)
	}
	if err := validateCandidateMapping(mappingFile{Version: 2, Resources: resources}); err != nil {
		t.Fatalf("validate complete visual mapping: %v", err)
	}

	drifted := retouchVisualTask2533Resource(t, 183)
	drifted.History[0].FinalAssetIDs = []int64{19803}
	drifted.History[0].ManifestRowHash, _ = revisionManifestRowHash(drifted.History[0])
	if err := validateCandidateResourceMappingV2(0, drifted); err == nil || !strings.Contains(err.Error(), "source/final/reference contract") {
		t.Fatalf("final drift error = %v", err)
	}

	if err := validateCandidateMapping(mappingFile{Version: 2, Resources: resources[:4]}); err == nil || !strings.Contains(err.Error(), "all five exact") {
		t.Fatalf("incomplete task 2533 policy error = %v", err)
	}
}

func TestRetouchVisualTask2533ScopeExceptionAllowsOnlyExactFinal(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mapping := resourceMapping{
		TaskID:     2533,
		ScopeKind:  "retouch_requirement",
		ScopeRefID: 183,
	}
	exact := mappedAssetState{ID: 19789}
	if err := validateMappedAssetScope(context.Background(), db, mapping, exact, true, false); err != nil {
		t.Fatalf("exact visually reviewed final scope: %v", err)
	}

	mock.ExpectQuery("SELECT COUNT\\(\\*\\),MAX\\(id\\) FROM task_retouch_requirements").
		WithArgs(int64(2533)).
		WillReturnRows(sqlmock.NewRows([]string{"count", "max"}).AddRow(5, 187))
	if err := validateMappedAssetScope(
		context.Background(),
		db,
		mapping,
		mappedAssetState{ID: 19803},
		true,
		false,
	); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("unassigned visual asset scope error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRetouchUnscopedAtomicBatchScopeRequiresExplicitOverride(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mapping := resourceMapping{
		TaskID:     2672,
		ScopeKind:  "retouch_requirement",
		ScopeRefID: 209,
	}
	state := mappedAssetState{ID: 23109}
	if err := validateMappedAssetScope(context.Background(), db, mapping, state, false, true); err != nil {
		t.Fatalf("approved unscoped atomic final: %v", err)
	}

	mock.ExpectQuery("SELECT COUNT\\(\\*\\),MAX\\(id\\) FROM task_retouch_requirements").
		WithArgs(int64(2672)).
		WillReturnRows(sqlmock.NewRows([]string{"count", "max"}).AddRow(2, 210))
	if err := validateMappedAssetScope(context.Background(), db, mapping, state, false, false); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("unapproved unscoped atomic final error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestValidateSourceBundleRequiresDeterministicReviewedManifest(t *testing.T) {
	confirmedAt := time.Date(2026, 7, 1, 1, 2, 3, 0, time.UTC)
	bundle := sourceBundleMapping{
		TaskAssetID:  30,
		Format:       "zip",
		BundleSHA256: strings.Repeat("a", 64),
		Members: []sourceBundleMember{
			{TaskAssetID: 31, SHA256: strings.Repeat("b", 64), Confirmed: true},
			{TaskAssetID: 32, SHA256: strings.Repeat("c", 64), Confirmed: true},
		},
		ConfirmedBy:  21,
		ConfirmedAt:  confirmedAt,
		Confirmation: "byte hashes reviewed before materializing the ZIP",
	}
	manifestHash, err := sourceBundleManifestHash(bundle)
	if err != nil {
		t.Fatal(err)
	}
	bundle.ManifestHash = manifestHash
	if err := validateSourceBundle("bundle", &bundle); err != nil {
		t.Fatalf("validateSourceBundle() error = %v", err)
	}
	bundle.Members[1].Confirmed = false
	if err := validateSourceBundle("bundle", &bundle); err == nil || !strings.Contains(err.Error(), "confirmed=true") {
		t.Fatalf("validateSourceBundle() error = %v", err)
	}
}

func TestValidateAssetRecoveriesAreFrozenAndFailClosed(t *testing.T) {
	proposed := assetRecoveryMapping{
		TaskID: 2807, MissingTaskAssetID: 23989,
		RecoverySourceTaskAssetID:  24034,
		Strategy:                   "clone_b_prematerialized_storage_ref_v1",
		OriginalStorageRefID:       "f511c5d4-507f-4a69-bf10-70bae369429d",
		RecoverySourceStorageRefID: "983a746c-c674-4f5c-8812-073be989b194",
		ExpectedFileSize:           683001,
		PreviewWholeHash:           "471739776f4c230a80ae5514e83e92fd3f1e104d203ced3ac793c65c25a525e4",
		DesignThumbWholeHash:       "3442c0ac91eb61371d4057d6c0de232f8ba4f3c25cb6b68cff63142aa155e6ef",
		Confidence:                 "proposed_review",
		ReviewPolicyIDs:            []string{reviewPolicyLegacyDeletedAssetRecovery},
	}
	hash, err := assetRecoveryManifestRowHash(proposed)
	if err != nil {
		t.Fatal(err)
	}
	proposed.ManifestRowHash = hash
	mapping := mappingFile{Version: workflowGroupsMappingV2, AssetRecoveries: []assetRecoveryMapping{proposed}}
	if err := validateCandidateMapping(mapping); err != nil {
		t.Fatalf("validateCandidateMapping() error = %v", err)
	}
	if err := validateMapping(mapping); err == nil || !strings.Contains(err.Error(), "cannot be applied") {
		t.Fatalf("validateMapping() error = %v", err)
	}
	prematerializedConfirmed := proposed
	prematerializedConfirmed.Confidence = "confirmed_auto"
	prematerializedConfirmed.ControlledReadProtocol = "controlled-asset-read-v1"
	prematerializedConfirmed.ControlledReadEvidenceHash = "b39e0d9d26e6fdd35941b195bdc413eb12dd6795e23276a48c9b9bd49f829b08"
	prematerializedConfirmed.RecoverySourceSHA256 = "d0558b1a9d4a7afed5a03b6b97d4a765d34050866686e396ab0acf9f08f0dec5"
	prematerializedConfirmed.ConfirmedBy = 1
	prematerializedConfirmed.ConfirmedAt = time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	prematerializedConfirmed.ConfirmationNote = "reviewed controlled read receipt and Clone B recovery executor after-state"
	prematerializedConfirmed.ManifestRowHash, err = assetRecoveryManifestRowHash(prematerializedConfirmed)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateMapping(mappingFile{Version: workflowGroupsMappingV2, AssetRecoveries: []assetRecoveryMapping{prematerializedConfirmed}}); err != nil {
		t.Fatalf("confirmed prematerialized recovery error = %v", err)
	}
	unboundRead := prematerializedConfirmed
	unboundRead.ControlledReadEvidenceHash = strings.Repeat("0", 64)
	unboundRead.ManifestRowHash, err = assetRecoveryManifestRowHash(unboundRead)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateMapping(mappingFile{Version: workflowGroupsMappingV2, AssetRecoveries: []assetRecoveryMapping{unboundRead}}); err == nil || !strings.Contains(err.Error(), "controlled-read evidence") {
		t.Fatalf("unbound controlled read validation error = %v", err)
	}

	swapped := proposed
	swapped.RecoverySourceTaskAssetID = 24033
	swapped.ManifestRowHash, err = assetRecoveryManifestRowHash(swapped)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateCandidateMapping(mappingFile{Version: workflowGroupsMappingV2, AssetRecoveries: []assetRecoveryMapping{swapped}}); err == nil || !strings.Contains(err.Error(), "frozen size/derivative-hash contract") {
		t.Fatalf("swapped recovery validate error = %v", err)
	}

	unresolved := assetRecoveryMapping{
		TaskID: 2199, MissingTaskAssetID: 12323,
		RejectedSourceTaskAssetIDs: []int64{14510, 14514},
		Strategy:                   "historical_unavailable_tombstone_v1",
		OriginalStorageRefID:       "c0a135a1-080f-46a0-a41a-461aef0ea0fb",
		ExpectedFileSize:           17755216,
		PreviewWholeHash:           "82b35a045540d27f9656d6d02c99eb2814a62e9d048d33b20823fb8c0017aa4c",
		DesignThumbWholeHash:       "54dbf569874243a212c11c3e83e80f19944c2581f12c9473a793bc273ec666a3",
		ObjectProbeResult:          "not_found",
		ObjectProbeInputSHA256:     "3f17b37296d2670235ca9bfcfd4388823b81adecf8fbac0826e6f241923579c7",
		ObjectProbeEvidenceHash:    "f1c78819e1f3d5f4e7a4b25ff3d173368574a5639f4c6df45c8aae5482d047b8",
		ObjectProbeObjectKeySHA256: "e732f6cd269a93d6bac168b0852dbcf8480af8966847278cb073cd6905b0efdd",
		ObjectProbeReadOnlyGETs:    1,
		Confidence:                 "proposed_review",
		ReviewPolicyIDs:            []string{reviewPolicyLegacyHistoricalAssetUnavailable},
	}
	unresolved.ManifestRowHash, err = assetRecoveryManifestRowHash(unresolved)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateCandidateMapping(mappingFile{Version: workflowGroupsMappingV2, AssetRecoveries: []assetRecoveryMapping{unresolved}}); err != nil {
		t.Fatalf("historical unavailable candidate error = %v", err)
	}
	unboundProbe := unresolved
	unboundProbe.ObjectProbeEvidenceHash = ""
	unboundProbe.ManifestRowHash, err = assetRecoveryManifestRowHash(unboundProbe)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateCandidateMapping(mappingFile{Version: workflowGroupsMappingV2, AssetRecoveries: []assetRecoveryMapping{unboundProbe}}); err == nil || !strings.Contains(err.Error(), "object-absence probe binding") {
		t.Fatalf("unbound historical unavailable probe error = %v", err)
	}

	confirmed := unresolved
	confirmed.Confidence = "confirmed_auto"
	confirmed.ConfirmedBy = 1
	confirmed.ConfirmedAt = time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	confirmed.ConfirmationNote = "original bytes are proven unavailable; preserve immutable history without a current pointer"
	confirmed.ManifestRowHash, err = assetRecoveryManifestRowHash(confirmed)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateMapping(mappingFile{Version: workflowGroupsMappingV2, AssetRecoveries: []assetRecoveryMapping{confirmed}}); err != nil {
		t.Fatalf("confirmed historical unavailable mapping error = %v", err)
	}
	confirmed.ConfirmationNote = ""
	confirmed.ManifestRowHash, err = assetRecoveryManifestRowHash(confirmed)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateMapping(mappingFile{Version: workflowGroupsMappingV2, AssetRecoveries: []assetRecoveryMapping{confirmed}}); err == nil || !strings.Contains(err.Error(), "complete human confirmation metadata") {
		t.Fatalf("unconfirmed historical unavailable mapping error = %v", err)
	}
}

func TestValidateFormalApplyMappingRejectsV1(t *testing.T) {
	if err := validateFormalApplyMapping(mappingFile{Version: workflowGroupsMappingV1}); err == nil || !strings.Contains(err.Error(), "version 2") {
		t.Fatalf("validateFormalApplyMapping() error = %v", err)
	}
}

func TestPersistedRevisionReasonBindsFullEvidenceWithoutInliningEveryEvent(t *testing.T) {
	revision := resourceRevisionMapping{
		ManifestRowHash: strings.Repeat("a", 64),
		Confidence:      "confirmed_auto",
		ConfirmedBy:     1,
		ConfirmedAt:     time.Date(2026, 7, 23, 6, 4, 15, 0, time.UTC),
		Reason:          "candidate reconstructed from explicit legacy workflow boundaries; human confirmation remains required",
		EvidenceEventIDs: []string{
			"task_event_log:00000000-0000-0000-0000-000000000007",
			"task_event_log:00000000-0000-0000-0000-000000000006",
			"task_event_log:00000000-0000-0000-0000-000000000005",
			"task_event_log:00000000-0000-0000-0000-000000000004",
			"task_event_log:00000000-0000-0000-0000-000000000003",
			"task_event_log:00000000-0000-0000-0000-000000000002",
			"task_event_log:00000000-0000-0000-0000-000000000001",
		},
	}

	got, err := persistedRevisionReason(revision)
	if err != nil {
		t.Fatalf("persistedRevisionReason() error = %v", err)
	}
	if len([]rune(got)) > revisionReasonMaxRunes {
		t.Fatalf("persistedRevisionReason() length = %d, want <= %d", len([]rune(got)), revisionReasonMaxRunes)
	}
	for _, want := range []string{
		"manifest=" + strings.Repeat("a", 64),
		"evidence_count=7",
		"first_evidence=task_event_log:00000000-0000-0000-0000-000000000001",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("persistedRevisionReason() = %q, want %q", got, want)
		}
	}
	if strings.Contains(got, "00000000-0000-0000-0000-000000000002") {
		t.Fatalf("persistedRevisionReason() unexpectedly inlined the full evidence list: %q", got)
	}
	summary, marked := service.ParseLegacyMigrationEvidence(got)
	if !marked || summary == nil {
		t.Fatalf("persistedRevisionReason() was not readable: summary=%+v marked=%v reason=%q", summary, marked, got)
	}
	if summary.Confidence != "confirmed_auto" || summary.EvidenceEventCount != 7 ||
		summary.EvidenceEventIDsComplete ||
		len(summary.EvidenceEventIDs) != 1 ||
		summary.EvidenceEventIDs[0] != "task_event_log:00000000-0000-0000-0000-000000000001" {
		t.Fatalf("persisted compact evidence = %+v", summary)
	}
}

func TestPersistedRevisionReasonPreservesHardBlockedConfidence(t *testing.T) {
	revision := resourceRevisionMapping{
		ManifestRowHash: strings.Repeat("c", 64),
		Confidence:      "hard_blocked",
		ConfirmedBy:     1,
		ConfirmedAt:     time.Date(2026, 7, 23, 6, 4, 15, 0, time.UTC),
		Reason:          "candidate remains blocked",
		EvidenceEventIDs: []string{
			"task_module_event:42",
		},
	}

	got, err := persistedRevisionReason(revision)
	if err != nil {
		t.Fatalf("persistedRevisionReason() error = %v", err)
	}
	summary, marked := service.ParseLegacyMigrationEvidence(got)
	if !marked || summary == nil || summary.Confidence != "hard_blocked" ||
		summary.EvidenceEventCount != 1 || !summary.EvidenceEventIDsComplete {
		t.Fatalf("hard-blocked persisted evidence = %+v marked=%v reason=%q", summary, marked, got)
	}
}

func TestPersistedRevisionReasonHashBindsOversizedReasonWithoutTruncation(t *testing.T) {
	originalReason := strings.Repeat("reviewed historical evidence ", 40)
	revision := resourceRevisionMapping{
		ManifestRowHash: strings.Repeat("b", 64),
		Confidence:      "confirmed_auto",
		ConfirmedBy:     1,
		ConfirmedAt:     time.Date(2026, 7, 23, 6, 4, 15, 0, time.UTC),
		Reason:          originalReason,
		EvidenceEventIDs: []string{
			"task_event_log:00000000-0000-0000-0000-000000000001",
		},
	}

	got, err := persistedRevisionReason(revision)
	if err != nil {
		t.Fatalf("persistedRevisionReason() error = %v", err)
	}
	if len([]rune(got)) > revisionReasonMaxRunes {
		t.Fatalf("persistedRevisionReason() length = %d, want <= %d", len([]rune(got)), revisionReasonMaxRunes)
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(originalReason)))
	for _, want := range []string{
		"manifest=" + strings.Repeat("b", 64),
		"reason_sha256=" + hex.EncodeToString(sum[:]),
		"evidence_count=1",
		"first_evidence=task_event_log:00000000-0000-0000-0000-000000000001",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("persistedRevisionReason() = %q, want %q", got, want)
		}
	}
	if strings.Contains(got, strings.TrimSpace(originalReason)) {
		t.Fatalf("persistedRevisionReason() unexpectedly persisted oversized prose: %q", got)
	}
	summary, marked := service.ParseLegacyMigrationEvidence(got)
	if !marked || summary == nil {
		t.Fatalf("oversized persistedRevisionReason() was not readable: summary=%+v marked=%v reason=%q", summary, marked, got)
	}
	if summary.BusinessReason != "" ||
		summary.BusinessReasonSHA256 != hex.EncodeToString(sum[:]) ||
		summary.EvidenceEventCount != 1 ||
		!summary.EvidenceEventIDsComplete {
		t.Fatalf("oversized persisted evidence = %+v", summary)
	}
}

func TestValidateMappedAssetRejectsRoleScopeAndLifecycleDrift(t *testing.T) {
	tests := []struct {
		name         string
		assetType    string
		uploadStatus string
		scopeSKU     string
		want         string
	}{
		{name: "role", assetType: "source", uploadStatus: "uploaded", want: "cannot bind as final"},
		{name: "lifecycle", assetType: "delivery", uploadStatus: "pending", want: "lifecycle is not active/uploaded"},
		{name: "scope", assetType: "delivery", uploadStatus: "uploaded", scopeSKU: "SKU-1", want: "scoped asset cannot bind to task scope"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			mock.ExpectQuery("SELECT id,task_id,asset_type").WithArgs(int64(12)).WillReturnRows(sqlmock.NewRows([]string{
				"id", "task_id", "asset_type", "scope_sku_code", "retouch_requirement_id", "mime_type", "whole_hash", "upload_status",
				"flow_review_status", "rejected_at", "superseded_by_version_id", "superseded_at",
				"deleted_at", "cleaned_at", "access_revoked_at", "object_deleted_at",
			}).AddRow(12, 7, tt.assetType, tt.scopeSKU, nil, "image/png", strings.Repeat("d", 64), tt.uploadStatus,
				"not_applicable", nil, nil, nil, nil, nil, nil, nil))
			err = validateMappedAsset(context.Background(), db, resourceMapping{TaskID: 7, ScopeKind: "task"}, 12, "final", "", false, false)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("validateMappedAsset() error = %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
