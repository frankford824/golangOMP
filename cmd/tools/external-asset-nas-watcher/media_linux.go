//go:build linux

package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"time"

	"github.com/google/uuid"
	"workflow/domain"
	"workflow/internal/assetmedia"
)

type mediaJournalItem struct {
	Relative string                              `json:"relative"`
	Snapshot fileSnapshot                        `json:"snapshot"`
	Event    domain.ExternalAssetFilesystemEvent `json:"event"`
}
type mediaJournalWrite struct {
	Relative  string `json:"relative"`
	Operation string `json:"operation"`
	Sequence  int64  `json:"sequence"`
}
type mediaScanReady struct {
	Scan  domain.ExternalMediaScan
	Files map[string]fileSnapshot
	Err   error
}

func (w *nasWatcher) mediaScanWatermark() int64 {
	watermark := w.state.Sequence
	for _, pending := range w.pending {
		if pending.Revision > 0 && pending.Revision <= watermark {
			watermark = pending.Revision - 1
		}
	}
	for _, item := range w.state.Outbox {
		if item.Event.Sequence > 0 && item.Event.Sequence <= watermark {
			watermark = item.Event.Sequence - 1
		}
	}
	return watermark
}

func (w *nasWatcher) appendJournal(rel, op string, seq int64) error {
	if err := os.MkdirAll(filepath.Dir(w.cfg.StateFile), 0750); err != nil {
		return err
	}
	f, err := os.OpenFile(w.cfg.StateFile+".journal", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err = json.NewEncoder(f).Encode(mediaJournalWrite{rel, op, seq}); err != nil {
		return err
	}
	return f.Sync()
}

func (w *nasWatcher) restoreJournal() error {
	f, err := os.OpenFile(w.cfg.StateFile+".journal", os.O_RDWR, 0600)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	var offset int64
	scan := bufio.NewScanner(f)
	scan.Buffer(make([]byte, 4096), 64<<10)
	for scan.Scan() {
		// Encoder writes a newline only after the JSON record is complete.
		// A crash-tail without it is discarded; startup performs a full scan.
		if offset+int64(len(scan.Bytes())) == st.Size() {
			if err = f.Truncate(offset); err != nil {
				return err
			}
			if err = f.Sync(); err != nil {
				return err
			}
			log.Printf("discarded incomplete NAS journal tail; full scan required")
			return nil
		}
		offset += int64(len(scan.Bytes()) + 1)
		var item mediaJournalWrite
		if err = json.Unmarshal(scan.Bytes(), &item); err != nil {
			return fmt.Errorf("invalid durable NAS journal: %w", err)
		}
		if _, ok := w.relative(filepath.Join(w.cfg.Root, item.Relative)); !ok || !w.ownsRelative(item.Relative) {
			return fmt.Errorf("journal path out of scope")
		}
		if item.Sequence > w.state.Sequence {
			w.state.Sequence = item.Sequence
		}
		if item.Sequence > w.state.AppliedSequence[item.Relative] {
			w.pending[item.Relative] = pendingEvent{Operation: item.Operation, Revision: item.Sequence, DueAt: time.Now().Add(w.cfg.Stability)}
		}
	}
	return scan.Err()
}

func (w *nasWatcher) runMedia(ctx context.Context) error {
	if w.cfg.OriginRoot != "/p3" || w.cfg.ShardCount != 2 {
		return fmt.Errorf("media scanner requires /p3 and two stable shards")
	}
	info, err := os.Stat(w.cfg.Root)
	if err != nil {
		return err
	}
	rootID, _, err := assetmedia.FileIdentity(info)
	if err != nil {
		return err
	}
	state, err := loadWatcherState(w.cfg.StateFile)
	if err != nil {
		return err
	}
	if state != nil && state.RootIdentity != "" && state.RootIdentity != rootID {
		return fmt.Errorf("NAS root identity changed; refuse absence reconciliation")
	}
	if state != nil && state.RootIdentity != "" {
		w.state = *state
	} else {
		w.state = watcherState{Version: watcherStateVersion, Files: map[string]fileSnapshot{}, Epoch: uuid.NewString(), RootIdentity: rootID}
	}
	if w.state.Epoch == "" {
		w.state.Epoch = uuid.NewString()
	}
	if w.state.AppliedSequence == nil {
		w.state.AppliedSequence = map[string]int64{}
	}
	if err = w.restoreJournal(); err != nil {
		return err
	}
	if err = saveWatcherState(w.cfg.StateFile, w.state); err != nil {
		return err
	}
	if err = w.addRecursive(w.cfg.Root); err != nil {
		return err
	}
	raw := make(chan rawWatchEvent, 8192)
	readErr := make(chan error, 1)
	go func() { readErr <- w.readEvents(ctx, raw) }()
	scanResults := make(chan mediaScanReady, 1)
	scanRunning := false
	scanInvalid := false
	var retryAt time.Time
	var ready *mediaScanReady
	startScan := func() {
		if scanRunning {
			return
		}
		scanRunning = true
		scanInvalid = false
		retryAt = time.Time{}
		identity := domain.ExternalMediaScan{ScanID: uuid.NewString(), AgentID: w.cfg.AgentID, AgentEpoch: w.state.Epoch, RootIdentity: rootID, OriginRoot: "/p3", StartSequence: w.mediaScanWatermark(), ShardIndex: w.cfg.ShardIndex, ShardCount: 2}
		go func() {
			files, e := w.uploadMediaSnapshot(ctx, identity)
			scanResults <- mediaScanReady{Scan: identity, Files: files, Err: e}
		}()
	}
	startScan()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	reconcile := time.NewTicker(w.cfg.ReconcileInterval)
	defer reconcile.Stop()
	for {
		if w.journalErr != nil {
			return w.journalErr
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-readErr:
			return err
		case event := <-raw:
			if event.QueueFull {
				scanInvalid = true
				if ready != nil {
					ready.Err = fmt.Errorf("event queue overflow during scan")
				}
				if err = w.reconcile(); err != nil {
					log.Printf("NAS reconciliation failed: %v", err)
				}
				continue
			}
			w.handleRawEvent(event)
		case result := <-scanResults:
			if scanInvalid && result.Err == nil {
				result.Err = fmt.Errorf("event queue overflow during scan")
			}
			if result.Err != nil {
				log.Printf("NAS scan failed without absence changes: %v", result.Err)
				scanRunning = false
				retryAt = time.Now().Add(time.Minute)
			} else {
				ready = &result
			}
		case <-tick.C:
			if !retryAt.IsZero() && time.Now().After(retryAt) {
				startScan()
			}
			if err = w.processMediaDue(ctx); err != nil {
				log.Printf("NAS durable event retry: %v", err)
				continue
			}
			if ready != nil && len(w.pending) == 0 && len(w.state.Outbox) == 0 && len(raw) == 0 {
				if ready.Err != nil {
					scanRunning = false
					retryAt = time.Now().Add(time.Minute)
					ready = nil
					continue
				}
				complete := ready.Scan
				complete.EndSequence = w.state.Sequence
				complete.Parts = (len(ready.Files) + w.cfg.BatchSize - 1) / w.cfg.BatchSize
				complete.Files = int64(len(ready.Files))
				complete.ManifestSHA256 = w.mediaSnapshotDigest(ready.Scan, ready.Files)
				var response struct {
					Data struct {
						JobID string `json:"job_id"`
					} `json:"data"`
				}
				if err = w.mediaAPI(ctx, "/scans/complete", complete, &response); err != nil {
					log.Printf("NAS scan completion rejected: %v", err)
					scanRunning = false
					retryAt = time.Now().Add(time.Minute)
					ready = nil
					continue
				}
				for rel, snapshot := range ready.Files {
					if w.state.AppliedSequence[rel] <= ready.Scan.StartSequence {
						w.state.Files[rel] = snapshot
					}
				}
				if err = saveWatcherState(w.cfg.StateFile, w.state); err != nil {
					return err
				}
				log.Printf("NAS snapshot queued scan_id=%s job_id=%s files=%d", complete.ScanID, response.Data.JobID, complete.Files)
				ready = nil
				scanRunning = false
			}
		case <-reconcile.C:
			startScan()
		}
	}
}

func (w *nasWatcher) mediaEvent(rel, operation string, snapshot fileSnapshot, sequence int64, scanID string) domain.ExternalAssetFilesystemEvent {
	e := w.buildEvent(rel, operation, snapshot)
	e.AgentEpoch = w.state.Epoch
	e.RootIdentity = w.state.RootIdentity
	e.Sequence = sequence
	e.ScanID = scanID
	e.EventID = domain.AssetMediaIdentity(e.EventID, e.AgentEpoch, fmt.Sprint(sequence), scanID)
	if operation == "upsert" {
		e.ModifiedNS = snapshot.ModifiedUnixNano
		e.ChangedNS = snapshot.ChangedUnixNano
		e.FileIdentity = snapshot.FileIdentity
	}
	return e
}

func (w *nasWatcher) processMediaDue(ctx context.Context) error {
	if len(w.state.Outbox) > 0 {
		events := make([]domain.ExternalAssetFilesystemEvent, 0, len(w.state.Outbox))
		for _, item := range w.state.Outbox {
			events = append(events, item.Event)
		}
		if err := w.postEvents(ctx, events); err != nil {
			return err
		}
		for _, item := range w.state.Outbox {
			if item.Event.Type == domain.ExternalAssetFilesystemEventDelete {
				delete(w.state.Files, item.Relative)
			} else {
				w.state.Files[item.Relative] = item.Snapshot
			}
			w.state.AppliedSequence[item.Relative] = item.Event.Sequence
		}
		w.state.Outbox = nil
		if err := saveWatcherState(w.cfg.StateFile, w.state); err != nil {
			return err
		}
		if len(w.pending) == 0 {
			if err := os.WriteFile(w.cfg.StateFile+".journal", nil, 0600); err != nil {
				return err
			}
		}
	}
	now := time.Now()
	items := []mediaJournalItem{}
	for rel, p := range w.pending {
		if len(items) >= w.cfg.BatchSize || p.DueAt.After(now) {
			continue
		}
		file, err := assetmedia.OpenBeneath(w.cfg.Root, filepath.ToSlash(rel))
		if os.IsNotExist(err) {
			items = append(items, mediaJournalItem{Relative: rel, Event: w.mediaEvent(rel, "delete", fileSnapshot{}, p.Revision, "")})
			continue
		}
		if err != nil {
			p.DueAt = now.Add(w.cfg.RetryInterval)
			w.pending[rel] = p
			continue
		}
		st, err := file.Stat()
		file.Close()
		if err != nil {
			continue
		}
		snapshot := snapshotFromInfo(st)
		if p.Sample == nil || *p.Sample != snapshot {
			p.Sample = &snapshot
			p.DueAt = now.Add(w.cfg.Stability)
			w.pending[rel] = p
			continue
		}
		items = append(items, mediaJournalItem{Relative: rel, Snapshot: snapshot, Event: w.mediaEvent(rel, "upsert", snapshot, p.Revision, "")})
	}
	if len(items) == 0 {
		return nil
	}
	w.state.Outbox = items
	if err := saveWatcherState(w.cfg.StateFile, w.state); err != nil {
		return err
	}
	for _, item := range items {
		delete(w.pending, item.Relative)
	}
	return w.processMediaDue(ctx)
}

func (w *nasWatcher) mediaAPI(ctx context.Context, suffix string, body, result any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.cfg.BackendURL+"/v1/integration/external-assets"+suffix, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-External-Asset-Event-Token", w.cfg.EventToken)
	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("NAS scan API transport failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("NAS scan API rejected HTTP %d", resp.StatusCode)
	}
	if result != nil {
		return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(result)
	}
	return nil
}

// Snapshot construction reads only immutable configuration. Live journal
// processing continues while the scanner walks directories and sends parts.
func (w *nasWatcher) snapshotParts(scan domain.ExternalMediaScan, files map[string]fileSnapshot) []domain.ExternalMediaScanPart {
	paths := make([]string, 0, len(files))
	for rel := range files {
		paths = append(paths, rel)
	}
	sort.Strings(paths)
	parts := []domain.ExternalMediaScanPart{}
	for start := 0; start < len(paths); start += w.cfg.BatchSize {
		end := start + w.cfg.BatchSize
		if end > len(paths) {
			end = len(paths)
		}
		items := []domain.ExternalAssetFilesystemEvent{}
		for _, rel := range paths[start:end] {
			snapshot := files[rel]
			modified := time.Unix(0, snapshot.ModifiedUnixNano).UTC()
			origin := path.Join(w.cfg.OriginRoot, filepath.ToSlash(rel))
			items = append(items, domain.ExternalAssetFilesystemEvent{EventID: domain.AssetMediaIdentity(scan.ScanID, origin), Type: domain.ExternalAssetFilesystemEventUpsert, MountPath: "/p3", OriginPath: origin,
				FileSize: snapshot.Size, ModifiedAt: &modified, ObservedAt: time.Unix(0, snapshot.ModifiedUnixNano).UTC(), RootIdentity: scan.RootIdentity, FileIdentity: snapshot.FileIdentity,
				ModifiedNS: snapshot.ModifiedUnixNano, ChangedNS: snapshot.ChangedUnixNano, AgentEpoch: scan.AgentEpoch, Sequence: scan.StartSequence, ScanID: scan.ScanID})
		}
		raw, _ := json.Marshal(items)
		sum := sha256.Sum256(raw)
		parts = append(parts, domain.ExternalMediaScanPart{ScanID: scan.ScanID, Part: len(parts), SHA256: hex.EncodeToString(sum[:]), Items: items})
	}
	return parts
}

func (w *nasWatcher) mediaSnapshotDigest(scan domain.ExternalMediaScan, files map[string]fileSnapshot) string {
	h := sha256.New()
	for _, p := range w.snapshotParts(scan, files) {
		_, _ = h.Write([]byte(p.SHA256 + "\n"))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (w *nasWatcher) uploadMediaSnapshot(ctx context.Context, scan domain.ExternalMediaScan) (map[string]fileSnapshot, error) {
	if err := w.mediaAPI(ctx, "/scans/start", scan, nil); err != nil {
		return nil, err
	}
	files, err := w.scanSnapshots(w.cfg.Root)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("empty NAS shard requires operator review; no absence changes applied")
	}
	for _, part := range w.snapshotParts(scan, files) {
		if err = w.mediaAPI(ctx, "/scans/parts", part, nil); err != nil {
			return nil, err
		}
	}
	return files, nil
}
