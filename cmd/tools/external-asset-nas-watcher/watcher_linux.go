//go:build linux

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"

	"workflow/domain"
)

const watcherStateVersion = 1

type watcherConfig struct {
	Root              string
	MountPath         string
	OriginRoot        string
	StateFile         string
	BackendURL        string
	EventToken        string
	AgentID           string
	Debounce          time.Duration
	Stability         time.Duration
	ReconcileInterval time.Duration
	RetryInterval     time.Duration
	HTTPTimeout       time.Duration
	BatchSize         int
	BootstrapEmit     bool
}

type fileSnapshot struct {
	Size             int64 `json:"size"`
	ModifiedUnixNano int64 `json:"modified_unix_nano"`
}

type watcherState struct {
	Version int                     `json:"version"`
	Files   map[string]fileSnapshot `json:"files"`
}

type pendingEvent struct {
	Operation string
	DueAt     time.Time
	Sample    *fileSnapshot
	Attempts  int
}

type rawWatchEvent struct {
	Path      string
	Mask      uint32
	IsDir     bool
	QueueFull bool
}

type nasWatcher struct {
	cfg       watcherConfig
	fd        int
	client    *http.Client
	watchMu   sync.Mutex
	watchDirs map[int]string
	state     watcherState
	pending   map[string]pendingEvent
}

func loadWatcherConfig() (watcherConfig, error) {
	cfg := watcherConfig{
		Root:              envOrDefault("WATCH_ROOT", "/data/image_lib/仓库素材区/徐凯"),
		MountPath:         cleanOriginPath(envOrDefault("WATCH_MOUNT_PATH", "/p3")),
		OriginRoot:        cleanOriginPath(envOrDefault("WATCH_ORIGIN_ROOT", "/p3/仓库素材区/徐凯")),
		StateFile:         envOrDefault("WATCH_STATE_FILE", "/state/snapshot.json"),
		BackendURL:        strings.TrimRight(strings.TrimSpace(os.Getenv("WATCH_BACKEND_URL")), "/"),
		EventToken:        strings.TrimSpace(os.Getenv("WATCH_EVENT_TOKEN")),
		AgentID:           envOrDefault("WATCH_AGENT_ID", "synology-p3-xukai"),
		Debounce:          durationEnv("WATCH_DEBOUNCE", 3*time.Second),
		Stability:         durationEnv("WATCH_STABILITY", 2*time.Second),
		ReconcileInterval: durationEnv("WATCH_RECONCILE_INTERVAL", 6*time.Hour),
		RetryInterval:     durationEnv("WATCH_RETRY_INTERVAL", 10*time.Second),
		HTTPTimeout:       durationEnv("WATCH_HTTP_TIMEOUT", 30*time.Second),
		BatchSize:         intEnv("WATCH_BATCH_SIZE", 200),
		BootstrapEmit:     boolEnv("WATCH_BOOTSTRAP_EMIT", false),
	}
	if cfg.BackendURL == "" || cfg.EventToken == "" {
		return watcherConfig{}, fmt.Errorf("WATCH_BACKEND_URL and WATCH_EVENT_TOKEN are required")
	}
	if cfg.MountPath == "/" || (cfg.OriginRoot != cfg.MountPath && !strings.HasPrefix(cfg.OriginRoot, cfg.MountPath+"/")) {
		return watcherConfig{}, fmt.Errorf("WATCH_ORIGIN_ROOT must be inside WATCH_MOUNT_PATH")
	}
	if cfg.BatchSize < 1 || cfg.BatchSize > 500 {
		return watcherConfig{}, fmt.Errorf("WATCH_BATCH_SIZE must be 1-500")
	}
	return cfg, nil
}

func newNASWatcher(cfg watcherConfig) (*nasWatcher, error) {
	root, err := filepath.Abs(cfg.Root)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("watch root is not a directory: %s", root)
	}
	fd, err := unix.InotifyInit1(unix.IN_NONBLOCK | unix.IN_CLOEXEC)
	if err != nil {
		return nil, fmt.Errorf("inotify init: %w", err)
	}
	cfg.Root = root
	return &nasWatcher{
		cfg:       cfg,
		fd:        fd,
		client:    &http.Client{Timeout: cfg.HTTPTimeout},
		watchDirs: map[int]string{},
		state:     watcherState{Version: watcherStateVersion, Files: map[string]fileSnapshot{}},
		pending:   map[string]pendingEvent{},
	}, nil
}

func (w *nasWatcher) Close() error {
	if w == nil || w.fd < 0 {
		return nil
	}
	err := unix.Close(w.fd)
	w.fd = -1
	return err
}

func (w *nasWatcher) Run(ctx context.Context) error {
	if err := w.addRecursive(w.cfg.Root); err != nil {
		return err
	}
	loaded, err := loadWatcherState(w.cfg.StateFile)
	if err != nil {
		return err
	}
	current, err := scanSnapshots(w.cfg.Root)
	if err != nil {
		return err
	}
	if loaded == nil {
		if w.cfg.BootstrapEmit {
			w.scheduleDiff(current, map[string]fileSnapshot{})
		} else {
			w.state.Files = current
			if err := saveWatcherState(w.cfg.StateFile, w.state); err != nil {
				return err
			}
		}
	} else {
		w.state = *loaded
		w.scheduleDiff(current, w.state.Files)
	}
	log.Printf("nas watcher started root=%q origin_root=%q watches=%d tracked=%d pending=%d reconcile=%s", w.cfg.Root, w.cfg.OriginRoot, len(w.watchDirs), len(w.state.Files), len(w.pending), w.cfg.ReconcileInterval)

	rawEvents := make(chan rawWatchEvent, 8192)
	errCh := make(chan error, 1)
	go func() { errCh <- w.readEvents(ctx, rawEvents) }()
	processTicker := time.NewTicker(time.Second)
	defer processTicker.Stop()
	reconcileTicker := time.NewTicker(w.cfg.ReconcileInterval)
	defer reconcileTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errCh:
			return err
		case event := <-rawEvents:
			if event.QueueFull {
				log.Printf("inotify queue overflow; scheduling full reconciliation")
				if err := w.reconcile(); err != nil {
					log.Printf("reconcile after overflow failed: %v", err)
				}
				continue
			}
			w.handleRawEvent(event)
		case <-processTicker.C:
			if err := w.processDue(ctx); err != nil {
				log.Printf("send filesystem events failed: %v", err)
			}
		case <-reconcileTicker.C:
			if err := w.reconcile(); err != nil {
				log.Printf("periodic reconciliation failed: %v", err)
			}
		}
	}
}

func (w *nasWatcher) addRecursive(root string) error {
	return filepath.WalkDir(root, func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(w.cfg.Root, current)
		if rel != "." && shouldIgnoreRelative(rel) {
			return filepath.SkipDir
		}
		mask := uint32(unix.IN_CREATE | unix.IN_CLOSE_WRITE | unix.IN_MOVED_TO | unix.IN_DELETE | unix.IN_MOVED_FROM | unix.IN_ATTRIB | unix.IN_DELETE_SELF | unix.IN_MOVE_SELF)
		wd, err := unix.InotifyAddWatch(w.fd, current, mask)
		if err != nil {
			return fmt.Errorf("watch %s: %w", current, err)
		}
		w.watchMu.Lock()
		w.watchDirs[wd] = current
		w.watchMu.Unlock()
		return nil
	})
}

func (w *nasWatcher) readEvents(ctx context.Context, out chan<- rawWatchEvent) error {
	buf := make([]byte, 64*1024)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		n, err := unix.Read(w.fd, buf)
		if err != nil {
			if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EINTR) {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			return fmt.Errorf("read inotify: %w", err)
		}
		for offset := 0; offset+unix.SizeofInotifyEvent <= n; {
			event := (*unix.InotifyEvent)(unsafe.Pointer(&buf[offset]))
			offset += unix.SizeofInotifyEvent
			nameBytes := buf[offset : offset+int(event.Len)]
			offset += int(event.Len)
			if event.Mask&unix.IN_Q_OVERFLOW != 0 {
				out <- rawWatchEvent{QueueFull: true}
				continue
			}
			w.watchMu.Lock()
			dir := w.watchDirs[int(event.Wd)]
			if event.Mask&unix.IN_IGNORED != 0 {
				delete(w.watchDirs, int(event.Wd))
			}
			w.watchMu.Unlock()
			if dir == "" {
				continue
			}
			name := string(bytes.TrimRight(nameBytes, "\x00"))
			fullPath := dir
			if name != "" {
				fullPath = filepath.Join(dir, name)
			}
			select {
			case out <- rawWatchEvent{Path: fullPath, Mask: event.Mask, IsDir: event.Mask&unix.IN_ISDIR != 0}:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}

func (w *nasWatcher) handleRawEvent(event rawWatchEvent) {
	rel, ok := w.relative(event.Path)
	if !ok || shouldIgnoreRelative(rel) {
		return
	}
	if event.IsDir {
		if event.Mask&(unix.IN_CREATE|unix.IN_MOVED_TO) != 0 {
			if err := w.addRecursive(event.Path); err != nil {
				log.Printf("add directory watches failed path=%q err=%v", event.Path, err)
			}
			current, err := scanSnapshots(event.Path)
			if err == nil {
				for childRel := range current {
					rootRel, _ := filepath.Rel(w.cfg.Root, filepath.Join(event.Path, childRel))
					w.schedule(rootRel, "upsert")
				}
			}
		}
		if event.Mask&(unix.IN_DELETE|unix.IN_MOVED_FROM|unix.IN_DELETE_SELF|unix.IN_MOVE_SELF) != 0 {
			prefix := filepath.ToSlash(rel) + "/"
			for tracked := range w.state.Files {
				if tracked == filepath.ToSlash(rel) || strings.HasPrefix(filepath.ToSlash(tracked), prefix) {
					w.schedule(tracked, "delete")
				}
			}
		}
		return
	}
	if event.Mask&(unix.IN_DELETE|unix.IN_MOVED_FROM) != 0 {
		w.schedule(rel, "delete")
		return
	}
	if event.Mask&(unix.IN_CREATE|unix.IN_CLOSE_WRITE|unix.IN_MOVED_TO|unix.IN_ATTRIB) != 0 {
		w.schedule(rel, "upsert")
	}
}

func (w *nasWatcher) schedule(rel, operation string) {
	rel = filepath.Clean(rel)
	if rel == "." || shouldIgnoreRelative(rel) {
		return
	}
	w.pending[rel] = pendingEvent{Operation: operation, DueAt: time.Now().Add(w.cfg.Debounce)}
}

func (w *nasWatcher) processDue(ctx context.Context) error {
	now := time.Now()
	type readyEvent struct {
		rel      string
		snapshot fileSnapshot
		event    domain.ExternalAssetFilesystemEvent
	}
	ready := make([]readyEvent, 0, w.cfg.BatchSize)
	for rel, pending := range w.pending {
		if len(ready) >= w.cfg.BatchSize || now.Before(pending.DueAt) {
			continue
		}
		fullPath := filepath.Join(w.cfg.Root, rel)
		info, err := os.Stat(fullPath)
		if pending.Operation == "delete" || errors.Is(err, os.ErrNotExist) {
			if err == nil {
				w.schedule(rel, "upsert")
				continue
			}
			ready = append(ready, readyEvent{rel: rel, event: w.buildEvent(rel, "delete", fileSnapshot{})})
			continue
		}
		if err != nil {
			pending.Attempts++
			pending.DueAt = now.Add(w.cfg.RetryInterval)
			w.pending[rel] = pending
			continue
		}
		if info.IsDir() {
			delete(w.pending, rel)
			continue
		}
		snapshot := snapshotFromInfo(info)
		if pending.Sample == nil || *pending.Sample != snapshot {
			pending.Sample = &snapshot
			pending.DueAt = now.Add(w.cfg.Stability)
			w.pending[rel] = pending
			continue
		}
		ready = append(ready, readyEvent{rel: rel, snapshot: snapshot, event: w.buildEvent(rel, "upsert", snapshot)})
	}
	if len(ready) == 0 {
		return nil
	}
	events := make([]domain.ExternalAssetFilesystemEvent, 0, len(ready))
	for _, item := range ready {
		events = append(events, item.event)
	}
	if err := w.postEvents(ctx, events); err != nil {
		for _, item := range ready {
			pending := w.pending[item.rel]
			pending.Attempts++
			pending.DueAt = now.Add(w.cfg.RetryInterval)
			w.pending[item.rel] = pending
		}
		return err
	}
	for _, item := range ready {
		delete(w.pending, item.rel)
		if item.event.Type == domain.ExternalAssetFilesystemEventDelete {
			delete(w.state.Files, item.rel)
		} else {
			w.state.Files[item.rel] = item.snapshot
		}
	}
	if err := saveWatcherState(w.cfg.StateFile, w.state); err != nil {
		return err
	}
	log.Printf("filesystem events accepted count=%d tracked=%d pending=%d", len(ready), len(w.state.Files), len(w.pending))
	return nil
}

func (w *nasWatcher) reconcile() error {
	current, err := scanSnapshots(w.cfg.Root)
	if err != nil {
		return err
	}
	upserts, deletes := w.scheduleDiff(current, w.state.Files)
	log.Printf("filesystem reconciliation scheduled upserts=%d deletes=%d tracked=%d current=%d", upserts, deletes, len(w.state.Files), len(current))
	return nil
}

func (w *nasWatcher) scheduleDiff(current, previous map[string]fileSnapshot) (int, int) {
	upserts, deletes := diffSnapshots(current, previous)
	for _, rel := range upserts {
		w.schedule(rel, "upsert")
	}
	for _, rel := range deletes {
		w.schedule(rel, "delete")
	}
	return len(upserts), len(deletes)
}

func (w *nasWatcher) buildEvent(rel, operation string, snapshot fileSnapshot) domain.ExternalAssetFilesystemEvent {
	originPath := path.Join(w.cfg.OriginRoot, filepath.ToSlash(rel))
	observedAt := time.Now().UTC().Truncate(time.Second)
	event := domain.ExternalAssetFilesystemEvent{
		EventID:    filesystemEventID(operation, originPath, snapshot),
		Type:       domain.ExternalAssetFilesystemEventType(operation),
		MountPath:  w.cfg.MountPath,
		OriginPath: originPath,
		ObservedAt: observedAt,
	}
	if operation == "upsert" {
		modified := time.Unix(0, snapshot.ModifiedUnixNano).UTC()
		event.FileSize = snapshot.Size
		event.ModifiedAt = &modified
	}
	return event
}

func (w *nasWatcher) postEvents(ctx context.Context, events []domain.ExternalAssetFilesystemEvent) error {
	body, err := json.Marshal(domain.ExternalAssetFilesystemEventBatch{AgentID: w.cfg.AgentID, Events: events})
	if err != nil {
		return err
	}
	endpoint := w.cfg.BackendURL + "/v1/integration/external-assets/events"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-External-Asset-Event-Token", w.cfg.EventToken)
	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("backend event response %d: %s", resp.StatusCode, strings.TrimSpace(string(message)))
	}
	return nil
}

func (w *nasWatcher) relative(fullPath string) (string, bool) {
	rel, err := filepath.Rel(w.cfg.Root, fullPath)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return rel, true
}

func scanSnapshots(root string) (map[string]fileSnapshot, error) {
	result := map[string]fileSnapshot{}
	err := filepath.WalkDir(root, func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, current)
		if relErr != nil {
			return relErr
		}
		if rel != "." && shouldIgnoreRelative(rel) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		result[rel] = snapshotFromInfo(info)
		return nil
	})
	return result, err
}

func snapshotFromInfo(info fs.FileInfo) fileSnapshot {
	return fileSnapshot{Size: info.Size(), ModifiedUnixNano: info.ModTime().UnixNano()}
}

func diffSnapshots(current, previous map[string]fileSnapshot) (upserts, deletes []string) {
	for rel, snapshot := range current {
		if before, ok := previous[rel]; !ok || before != snapshot {
			upserts = append(upserts, rel)
		}
	}
	for rel := range previous {
		if _, ok := current[rel]; !ok {
			deletes = append(deletes, rel)
		}
	}
	return upserts, deletes
}

func loadWatcherState(filename string) (*watcherState, error) {
	raw, err := os.ReadFile(filename)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read watcher state: %w", err)
	}
	var state watcherState
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, fmt.Errorf("decode watcher state: %w", err)
	}
	if state.Version != watcherStateVersion || state.Files == nil {
		return nil, fmt.Errorf("unsupported watcher state version %d", state.Version)
	}
	return &state, nil
}

func saveWatcherState(filename string, state watcherState) error {
	if err := os.MkdirAll(filepath.Dir(filename), 0o750); err != nil {
		return err
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return err
	}
	tmp := filename + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, filename); err != nil {
		return err
	}
	return nil
}

func filesystemEventID(operation, originPath string, snapshot fileSnapshot) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{operation, cleanOriginPath(originPath), strconv.FormatInt(snapshot.Size, 10), strconv.FormatInt(snapshot.ModifiedUnixNano, 10)}, "|")))
	return hex.EncodeToString(sum[:])
}

func shouldIgnoreRelative(rel string) bool {
	for _, part := range strings.FieldsFunc(filepath.ToSlash(rel), func(r rune) bool { return r == '/' }) {
		lower := strings.ToLower(strings.TrimSpace(part))
		if lower == "@eadir" || lower == "#recycle" || strings.HasPrefix(lower, "@syno") || lower == ".ds_store" {
			return true
		}
	}
	return false
}

func cleanOriginPath(value string) string {
	value = strings.ReplaceAll(strings.TrimSpace(value), "\\", "/")
	return path.Clean("/" + strings.TrimLeft(value, "/"))
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func intEnv(key string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(key)))
	if err != nil {
		return fallback
	}
	return value
}

func boolEnv(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
