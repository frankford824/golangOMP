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
}
