package main

import (
	"database/sql"
	"reflect"
	"testing"
	"time"
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
	if err := validateRevisionEventSemantics(revision, metadata); err != nil {
		t.Fatalf("policy-bound retouch terminal submit: %v", err)
	}

	revision.SourceStage = "reopen"
	if err := validateRevisionEventSemantics(revision, metadata); err != nil {
		t.Fatalf("policy-bound reopened retouch terminal submit: %v", err)
	}

	revision.ReviewPolicyIDs = nil
	if err := validateRevisionEventSemantics(revision, metadata); err == nil {
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
	if err := validateRevisionEventSemantics(revision, metadata); err != nil {
		t.Fatalf("policy-bound post-close replacement: %v", err)
	}
	revision.Status = "draft"
	if err := validateRevisionEventSemantics(revision, metadata); err != nil {
		t.Fatalf("policy-bound post-close draft: %v", err)
	}
	revision.Status = "finalized"
	revision.Reason = "unbound reopen"
	if err := validateRevisionEventSemantics(revision, metadata); err == nil {
		t.Fatal("expected unbound finalized reopen without approval to fail")
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
