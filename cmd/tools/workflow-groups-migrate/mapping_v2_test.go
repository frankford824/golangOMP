package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
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
	if err := validateRevisionEventSemantics(revision, metadata); err != nil {
		t.Fatalf("valid finalized design semantics: %v", err)
	}
	if err := validateRevisionEventSemantics(revision, metadata[:1]); err == nil || !strings.Contains(err.Error(), "approval") {
		t.Fatalf("missing approval semantics error = %v", err)
	}
	revision.SourceStage = "reopen"
	if err := validateRevisionEventSemantics(revision, []evidenceEventMetadata{{EventType: "task.audit.supplement_uploaded"}}); err != nil {
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
		ConfirmedAt: created, ConfirmationNote: "warehouse rejection reviewed; reopen design",
	}
	decision.ManifestRowHash, _ = taskStateDecisionManifestHash(decision)
	if err := validateTaskStateDecisions(mappingFile{Version: 2, TaskDecisions: []taskStateDecisionMapping{decision}}); err != nil {
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
	mock.ExpectQuery("SELECT task_status FROM tasks").WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"task_status"}).AddRow("RejectedByWarehouse"))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM users").WithArgs(int64(21)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("SELECT task_id,sequence,event_type").WithArgs("warehouse-1").WillReturnRows(sqlmock.NewRows([]string{"task_id", "sequence", "event_type", "payload", "created_at"}).AddRow(7, 1, "task.status.changed", `{"to":"RejectedByWarehouse"}`, created))
	if issue := validateTaskStateDecisionPreflight(context.Background(), db, decision, []resourceMapping{resource}); issue != nil {
		t.Fatalf("validateTaskStateDecisionPreflight() issue = %+v", issue)
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

func TestValidateFormalApplyMappingRejectsV1(t *testing.T) {
	if err := validateFormalApplyMapping(mappingFile{Version: workflowGroupsMappingV1}); err == nil || !strings.Contains(err.Error(), "version 2") {
		t.Fatalf("validateFormalApplyMapping() error = %v", err)
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
			err = validateMappedAsset(context.Background(), db, resourceMapping{TaskID: 7, ScopeKind: "task"}, 12, "final", "")
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("validateMappedAsset() error = %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
