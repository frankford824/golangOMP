//go:build linux

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"workflow/domain"
)

func TestScanWatermarkExcludesPendingAndUnacknowledgedEvents(t *testing.T) {
	w := &nasWatcher{state: watcherState{Sequence: 30, Outbox: []mediaJournalItem{{Event: domain.ExternalAssetFilesystemEvent{Sequence: 25}}}}, pending: map[string]pendingEvent{"a": {Revision: 19}, "b": {Revision: 30}}}
	if got := w.mediaScanWatermark(); got != 18 {
		t.Fatalf("watermark=%d want18", got)
	}
	w.pending = nil
	if got := w.mediaScanWatermark(); got != 24 {
		t.Fatalf("watermark=%d want24", got)
	}
	w.state.Outbox = nil
	if got := w.mediaScanWatermark(); got != 30 {
		t.Fatalf("watermark=%d want30", got)
	}
}

func TestMediaJournalRecoversCommittedLinesAndDiscardsCrashTail(t *testing.T) {
	dir := t.TempDir()
	w := &nasWatcher{cfg: watcherConfig{Root: dir, StateFile: filepath.Join(dir, "state.json")}, state: watcherState{AppliedSequence: map[string]int64{}}, pending: map[string]pendingEvent{}}
	if err := w.appendJournal("设计.psd", "upsert", 11); err != nil {
		t.Fatal(err)
	}
	good, err := os.ReadFile(w.cfg.StateFile + ".journal")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(w.cfg.StateFile+".journal", append(good, []byte(`{"relative":"unfinished`)...), 0600); err != nil {
		t.Fatal(err)
	}
	if err = w.restoreJournal(); err != nil {
		t.Fatal(err)
	}
	if w.state.Sequence != 11 || w.pending["设计.psd"].Revision != 11 {
		t.Fatal("lost complete journal record")
	}
	after, err := os.ReadFile(w.cfg.StateFile + ".journal")
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(good) {
		t.Fatal("crash tail retained")
	}
	if err = w.appendJournal("设计.psd", "delete", 12); err != nil {
		t.Fatal(err)
	}
	if err = w.restoreJournal(); err != nil {
		t.Fatal(err)
	}
	if w.pending["设计.psd"].Revision != 12 {
		t.Fatal("subsequent append not recoverable")
	}
}

func TestMediaStatePersistsOutboxBeforeAcknowledgement(t *testing.T) {
	file := filepath.Join(t.TempDir(), "state.json")
	state := watcherState{Version: watcherStateVersion, Sequence: 7, Epoch: "epoch", RootIdentity: "root", Files: map[string]fileSnapshot{}, AppliedSequence: map[string]int64{}, Outbox: []mediaJournalItem{{Relative: "a.jpg"}}}
	if err := saveWatcherState(file, state); err != nil {
		t.Fatal(err)
	}
	restored, err := loadWatcherState(file)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(state)
	b, _ := json.Marshal(restored)
	if string(a) != string(b) {
		t.Fatalf("state changed after restart: %s", b)
	}
}
