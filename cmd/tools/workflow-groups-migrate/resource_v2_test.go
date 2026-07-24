package main

import (
	"context"
	"database/sql"
	"reflect"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestRevisionActorIDsForPreflightDefersCandidateReviewerSentinel(t *testing.T) {
	candidate := resourceRevisionMapping{CreatedBy: 21, ConfirmedBy: 0, Confidence: "proposed_review"}
	if got, want := revisionActorIDsForPreflight(candidate), []int64{21}; !reflect.DeepEqual(got, want) {
		t.Fatalf("candidate actor ids = %v, want %v", got, want)
	}

	confirmed := resourceRevisionMapping{CreatedBy: 21, ConfirmedBy: 34, Confidence: "confirmed_auto"}
	if got, want := revisionActorIDsForPreflight(confirmed), []int64{21, 34}; !reflect.DeepEqual(got, want) {
		t.Fatalf("confirmed actor ids = %v, want %v", got, want)
	}

	missingCreator := resourceRevisionMapping{CreatedBy: 0, ConfirmedBy: 0, Confidence: "hard_blocked"}
	if got, want := revisionActorIDsForPreflight(missingCreator), []int64{0}; !reflect.DeepEqual(got, want) {
		t.Fatalf("missing creator actor ids = %v, want %v", got, want)
	}

	missingReviewer := resourceRevisionMapping{CreatedBy: 21, ConfirmedBy: 0, Confidence: "confirmed_auto"}
	if got, want := revisionActorIDsForPreflight(missingReviewer), []int64{21, 0}; !reflect.DeepEqual(got, want) {
		t.Fatalf("missing reviewer actor ids = %v, want %v", got, want)
	}
}

func TestValidateRevisionEventSemanticsAllowsOnlyPolicyBoundRetouchTerminalSubmit(t *testing.T) {
	metadata := []evidenceEventMetadata{{EventType: "task.design.submitted", Payload: `{}`}}
	revision := resourceRevisionMapping{
		Status: "finalized", SourceStage: "retouch",
		ReviewPolicyIDs: []string{reviewPolicyLegacyRetouchTerminalSubmit},
	}
	if err := validateRevisionEventSemantics(7, revision, metadata); err != nil {
		t.Fatalf("policy-bound retouch terminal submit: %v", err)
	}

	revision.SourceStage = "reopen"
	if err := validateRevisionEventSemantics(7, revision, metadata); err != nil {
		t.Fatalf("policy-bound reopened retouch terminal submit: %v", err)
	}

	revision.ReviewPolicyIDs = nil
	if err := validateRevisionEventSemantics(7, revision, metadata); err == nil {
		t.Fatal("expected finalized retouch submit without policy to fail")
	}
}

func TestValidateRevisionEventSemanticsAllowsPolicyBoundPostCloseReplacement(t *testing.T) {
	metadata := []evidenceEventMetadata{{
		EventType: "task.asset.upload_session.completed",
		Payload:   `{"asset_version_id":42}`,
	}}
	revision := resourceRevisionMapping{
		Status: "finalized", SourceStage: "reopen",
		ReviewPolicyIDs: []string{reviewPolicyReopen, reviewPolicyLegacyPostCloseReplacement},
		Reason:          "policy legacy_post_close_replacement_v1: post-close same-root replacement proven by an immutable edge",
	}
	if err := validateRevisionEventSemantics(7, revision, metadata); err != nil {
		t.Fatalf("policy-bound post-close replacement: %v", err)
	}
	revision.Status = "draft"
	if err := validateRevisionEventSemantics(7, revision, metadata); err != nil {
		t.Fatalf("policy-bound post-close draft: %v", err)
	}
	revision.Status = "finalized"
	revision.Reason = "unbound reopen"
	if err := validateRevisionEventSemantics(7, revision, metadata); err == nil {
		t.Fatal("expected unbound finalized reopen without approval to fail")
	}
}

func TestValidateRevisionEventSemanticsAllowsApprovedLegacyRetouchExceptions(t *testing.T) {
	submit := []evidenceEventMetadata{{EventType: "task.design.submitted", Payload: `{}`}}
	unscoped := resourceRevisionMapping{
		Status:          "finalized",
		SourceStage:     "retouch",
		ReviewPolicyIDs: []string{reviewPolicyLegacyRetouchUnscopedAtomicBatch},
		Reason:          "policy legacy_retouch_unscoped_atomic_batch_v1: reviewed atomic membership",
	}
	if err := validateRevisionEventSemantics(2672, unscoped, submit); err != nil {
		t.Fatalf("approved unscoped atomic batch: %v", err)
	}
	unscoped.ReviewPolicyIDs = nil
	if err := validateRevisionEventSemantics(2672, unscoped, submit); err == nil {
		t.Fatal("expected unscoped atomic batch without policy to fail")
	}
	unscoped.ReviewPolicyIDs = []string{reviewPolicyLegacyRetouchUnscopedAtomicBatch}
	unscoped.Reason = "unbound atomic membership"
	if err := validateRevisionEventSemantics(2672, unscoped, submit); err == nil {
		t.Fatal("expected unscoped atomic batch without policy reason to fail")
	}

	partial := resourceRevisionMapping{
		Status:          "finalized",
		SourceStage:     "retouch",
		ReviewPolicyIDs: []string{reviewPolicyLegacyRetouchPrematurePartial},
		Reason:          "policy legacy_retouch_premature_terminal_partial_v1: reviewed partial terminal",
	}
	if err := validateRevisionEventSemantics(981, partial, submit); err != nil {
		t.Fatalf("approved premature partial final: %v", err)
	}
	partial.Status = "draft"
	partial.SourceStage = "reopen"
	if err := validateRevisionEventSemantics(981, partial, submit); err != nil {
		t.Fatalf("approved premature partial draft: %v", err)
	}
	if err := validateRevisionEventSemantics(982, partial, submit); err == nil {
		t.Fatal("expected premature partial policy outside frozen task allowlist to fail")
	}
}

func TestAllowsLegacyUnscopedRetouchFinalRequiresFrozenPolicyBoundary(t *testing.T) {
	partialReason := "policy legacy_retouch_premature_terminal_partial_v1: reviewed partial membership"
	partialPolicy := []string{reviewPolicyLegacyRetouchPrematurePartial}
	approved := []struct {
		mapping  resourceMapping
		revision resourceRevisionMapping
		assetID  int64
	}{
		{
			mapping: resourceMapping{TaskID: 981, ScopeKind: "retouch_requirement", ScopeRefID: 8},
			revision: resourceRevisionMapping{
				RevisionNo: 1, Status: "finalized", SourceStage: "retouch",
				ReviewPolicyIDs: partialPolicy, Reason: partialReason,
			},
			assetID: 2763,
		},
		{
			mapping: resourceMapping{TaskID: 981, ScopeKind: "retouch_requirement", ScopeRefID: 8},
			revision: resourceRevisionMapping{
				RevisionNo: 2, Status: "draft", SourceStage: "reopen",
				ReviewPolicyIDs: partialPolicy, Reason: partialReason,
			},
			assetID: 2763,
		},
		{
			mapping: resourceMapping{TaskID: 1035, ScopeKind: "retouch_requirement", ScopeRefID: 21},
			revision: resourceRevisionMapping{
				RevisionNo: 1, Status: "finalized", SourceStage: "retouch",
				ReviewPolicyIDs: partialPolicy, Reason: partialReason,
			},
			assetID: 3859,
		},
		{
			mapping: resourceMapping{TaskID: 1035, ScopeKind: "retouch_requirement", ScopeRefID: 21},
			revision: resourceRevisionMapping{
				RevisionNo: 2, Status: "draft", SourceStage: "reopen",
				ReviewPolicyIDs: partialPolicy, Reason: partialReason,
			},
			assetID: 3859,
		},
		{
			mapping: resourceMapping{TaskID: 1214, ScopeKind: "retouch_requirement", ScopeRefID: 43},
			revision: resourceRevisionMapping{
				RevisionNo: 1, Status: "draft", SourceStage: "reopen",
				ReviewPolicyIDs: partialPolicy, Reason: partialReason,
			},
			assetID: 5769,
		},
	}
	for _, test := range approved {
		if !allowsLegacyUnscopedRetouchFinal(test.mapping, test.revision, test.assetID) {
			t.Fatalf("expected exact approved tuple to allow task=%d scope=%d revision=%d asset=%d",
				test.mapping.TaskID, test.mapping.ScopeRefID, test.revision.RevisionNo, test.assetID)
		}
	}

	base := approved[0]
	base.revision.ReviewPolicyIDs = nil
	if allowsLegacyUnscopedRetouchFinal(base.mapping, base.revision, base.assetID) {
		t.Fatal("unexpected scope override without approved policy")
	}
	base = approved[0]
	base.revision.Reason = "unbound partial membership"
	if allowsLegacyUnscopedRetouchFinal(base.mapping, base.revision, base.assetID) {
		t.Fatal("unexpected scope override without policy-bound reason")
	}

	negativeTuples := []struct {
		name     string
		mapping  resourceMapping
		revision resourceRevisionMapping
		assetID  int64
	}{
		{"wrong requirement", resourceMapping{TaskID: 981, ScopeKind: "retouch_requirement", ScopeRefID: 9}, approved[0].revision, 2763},
		{"wrong scope kind", resourceMapping{TaskID: 981, ScopeKind: "task", ScopeRefID: 8}, approved[0].revision, 2763},
		{"wrong task", resourceMapping{TaskID: 982, ScopeKind: "retouch_requirement", ScopeRefID: 8}, approved[0].revision, 2763},
		{"wrong asset", approved[0].mapping, approved[0].revision, 2764},
		{"wrong revision", approved[0].mapping, resourceRevisionMapping{
			RevisionNo: 3, Status: "draft", SourceStage: "reopen",
			ReviewPolicyIDs: partialPolicy, Reason: partialReason,
		}, 2763},
		{"wrong status", approved[0].mapping, resourceRevisionMapping{
			RevisionNo: 1, Status: "draft", SourceStage: "retouch",
			ReviewPolicyIDs: partialPolicy, Reason: partialReason,
		}, 2763},
		{"wrong stage", approved[0].mapping, resourceRevisionMapping{
			RevisionNo: 1, Status: "finalized", SourceStage: "reopen",
			ReviewPolicyIDs: partialPolicy, Reason: partialReason,
		}, 2763},
		{"task 1045 unassigned asset", resourceMapping{TaskID: 1045, ScopeKind: "retouch_requirement", ScopeRefID: 26}, resourceRevisionMapping{
			RevisionNo: 1, Status: "finalized", SourceStage: "retouch",
			ReviewPolicyIDs: partialPolicy, Reason: partialReason,
		}, 3956},
		{"task 1052 unassigned asset", resourceMapping{TaskID: 1052, ScopeKind: "retouch_requirement", ScopeRefID: 30}, resourceRevisionMapping{
			RevisionNo: 1, Status: "finalized", SourceStage: "retouch",
			ReviewPolicyIDs: partialPolicy, Reason: partialReason,
		}, 8359},
	}
	for _, test := range negativeTuples {
		if allowsLegacyUnscopedRetouchFinal(test.mapping, test.revision, test.assetID) {
			t.Fatalf("unexpected scope override for %s", test.name)
		}
	}

	atomic := resourceRevisionMapping{
		ReviewPolicyIDs: []string{reviewPolicyLegacyRetouchUnscopedAtomicBatch},
		Reason:          "policy legacy_retouch_unscoped_atomic_batch_v1: reviewed atomic membership",
	}
	if !allowsLegacyUnscopedRetouchFinal(resourceMapping{TaskID: 2672}, atomic, 23109) {
		t.Fatal("expected exact approved atomic policy and reason to allow its unscoped final")
	}
	atomic.Reason = "unbound atomic membership"
	if allowsLegacyUnscopedRetouchFinal(resourceMapping{TaskID: 2672}, atomic, 23109) {
		t.Fatal("unexpected atomic scope override without policy-bound reason")
	}
}

func TestRetouchScopeOverrideCannotReplaceExplicitDifferentRequirement(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mapping := resourceMapping{TaskID: 981, ScopeKind: "retouch_requirement", ScopeRefID: 8}
	state := mappedAssetState{
		ID:                   2763,
		RetouchRequirementID: sql.NullInt64{Int64: 9, Valid: true},
	}
	if err := validateMappedAssetScope(context.Background(), db, mapping, state, false, true); err == nil {
		t.Fatal("expected explicit different requirement binding to reject even with a reviewed override")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLoadEvidenceEventMetadataCanonicalizesDatabaseSequence(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Date(2026, 7, 18, 10, 12, 4, 0, time.UTC)
	mock.ExpectQuery("SELECT task_id,sequence,event_type").
		WithArgs("event-later").
		WillReturnRows(sqlmock.NewRows([]string{"task_id", "sequence", "event_type", "payload", "created_at"}).
			AddRow(int64(2600), int64(19), "task.design.submitted", `{}`, now))
	mock.ExpectQuery("SELECT task_id,sequence,event_type").
		WithArgs("event-earlier").
		WillReturnRows(sqlmock.NewRows([]string{"task_id", "sequence", "event_type", "payload", "created_at"}).
			AddRow(int64(2600), int64(16), "task.asset.upload_session.completed", `{}`, now.Add(-time.Hour)))

	got, err := loadEvidenceEventMetadata(context.Background(), db, 2600, []string{
		"task_event_log:event-later",
		"task_event_log:event-earlier",
	})
	if err != nil {
		t.Fatalf("loadEvidenceEventMetadata() error = %v", err)
	}
	if got[0].EventType != "task.asset.upload_session.completed" || got[1].EventType != "task.design.submitted" {
		t.Fatalf("metadata order = %#v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestValidateRevisionLifecycleStateAllowsOnlyHistoricalLaterTransition(t *testing.T) {
	created := time.Date(2026, 7, 1, 1, 0, 0, 0, time.UTC)
	superseded := created.Add(time.Hour)
	revision := resourceRevisionMapping{
		RevisionNo: 1, Status: "superseded", CreatedAt: created,
		FinalizedAt: &created,
	}
	state := mappedAssetState{
		ID: 42, FlowReviewStatus: "superseded",
		SupersededBy: sql.NullInt64{Int64: 43, Valid: true},
		SupersededAt: sql.NullTime{Time: superseded, Valid: true},
	}
	historical := resourceMapping{}
	if err := validateRevisionLifecycleState(historical, revision, state); err != nil {
		t.Fatalf("historical later supersession: %v", err)
	}

	currentRevisionNo := 1
	current := resourceMapping{FinalizedRevisionNo: &currentRevisionNo}
	if err := validateRevisionLifecycleState(current, revision, state); err == nil {
		t.Fatal("expected current pointer to superseded asset to fail")
	}

	state.SupersededAt.Time = created.Add(-time.Second)
	if err := validateRevisionLifecycleState(historical, revision, state); err == nil {
		t.Fatal("expected pre-boundary supersession to fail")
	}
}

func TestValidateRevisionLifecycleStateAllowsRejectedSnapshotInheritanceIntoReopenDraft(t *testing.T) {
	created := time.Date(2026, 7, 22, 5, 39, 38, 0, time.UTC)
	assetID := int64(23916)
	previous := resourceRevisionMapping{
		RevisionNo:      1,
		Status:          "rejected",
		SourceStage:     "design",
		SourceAliasFrom: &assetID,
		FinalAssetIDs:   []int64{assetID},
		CreatedAt:       created,
	}
	current := resourceRevisionMapping{
		RevisionNo:      2,
		Status:          "draft",
		SourceStage:     "reopen",
		SourceAliasFrom: &assetID,
		FinalAssetIDs:   []int64{assetID},
		ReviewPolicyIDs: []string{reviewPolicyExplicitEventReplay, reviewPolicyDeliverySourceAlias, reviewPolicyReopen},
		CreatedAt:       created.Add(time.Hour),
	}
	working := 2
	mapping := resourceMapping{
		History:           []resourceRevisionMapping{previous, current},
		WorkingRevisionNo: &working,
	}
	state := mappedAssetState{
		ID:               assetID,
		FlowReviewStatus: "superseded",
		SupersededBy:     sql.NullInt64{Int64: 23917, Valid: true},
		SupersededAt:     sql.NullTime{Time: created.Add(2 * time.Hour), Valid: true},
	}
	if err := validateRevisionLifecycleState(mapping, current, state); err != nil {
		t.Fatalf("rejected snapshot inheritance: %v", err)
	}

	current.ReviewPolicyIDs = nil
	if err := validateRevisionLifecycleState(mapping, current, state); err == nil {
		t.Fatal("expected inherited superseded asset without reopen policy to fail")
	}
	current.ReviewPolicyIDs = []string{reviewPolicyReopen}
	mapping.History[0].Status = "finalized"
	if err := validateRevisionLifecycleState(mapping, current, state); err == nil {
		t.Fatal("expected inheritance from a non-rejected revision to fail")
	}
	mapping.History[0].Status = "rejected"
	mapping.History[0].FinalAssetIDs = []int64{23917}
	mapping.History[0].SourceAliasFrom = nil
	if err := validateRevisionLifecycleState(mapping, current, state); err == nil {
		t.Fatal("expected inheritance of a different asset to fail")
	}
}
