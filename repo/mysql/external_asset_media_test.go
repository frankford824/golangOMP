package mysqlrepo

import (
	"testing"
	"workflow/domain"
)

func TestMediaSequenceTombstoneFencesOlderScan(t *testing.T) {
	old := &domain.ExternalMediaFingerprint{AgentID: "nas-0", AgentEpoch: "epoch", Sequence: 12}
	for _, seq := range []int64{0, 11, 12} {
		next := &domain.ExternalMediaFingerprint{AgentID: "nas-0", AgentEpoch: "epoch", Sequence: seq, SourceVersion: "scanned-before-delete"}
		if !mediaEventIsStale(old, next) {
			t.Fatalf("tombstone allowed stale sequence %d", seq)
		}
	}
	if mediaEventIsStale(old, &domain.ExternalMediaFingerprint{AgentID: "nas-0", AgentEpoch: "epoch", Sequence: 13}) {
		t.Fatal("new creation must be accepted")
	}
}

func TestFullScanAtCommittedWatermarkCanRepairOfflineChanges(t *testing.T) {
	old := &domain.ExternalMediaFingerprint{AgentID: "nas", AgentEpoch: "epoch", Sequence: 12, SourceVersion: "before-offline-edit"}
	next := &domain.ExternalMediaFingerprint{AgentID: "nas", AgentEpoch: "epoch", Sequence: 12, ScanID: "new-full-scan", SourceVersion: "after-offline-edit"}
	if mediaEventIsStale(old, next) {
		t.Fatal("same-watermark full scan must repair changes missed while watcher was offline")
	}
	old.Sequence = 13
	if !mediaEventIsStale(old, next) {
		t.Fatal("newer event must win over scan")
	}
}
