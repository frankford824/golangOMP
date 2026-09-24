package assetmedia

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/crc64"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type cacheReceipt struct {
	ContentID     string `json:"content_id"`
	SourceVersion string `json:"source_version"`
	Size          int64  `json:"size"`
	SHA256        string `json:"sha256"`
	CRC64         string `json:"crc64,omitempty"`
	ModifiedNS    int64  `json:"modified_ns"`
}

type CacheEntry struct {
	version    string
	sha256     string
	crc64      string
	mu         sync.Mutex
	id         string
	file       *os.File
	path       string
	size       int64
	received   int64
	complete   bool
	err        error
	changed    chan struct{}
	refs       int
	lastAccess time.Time
}

func (e *CacheEntry) signal() { close(e.changed); e.changed = make(chan struct{}) }

func (e *CacheEntry) Wait(ctx context.Context, offset int64) error {
	for {
		e.mu.Lock()
		if e.err != nil {
			err := e.err
			e.mu.Unlock()
			return err
		}
		if e.received > offset || e.complete {
			e.mu.Unlock()
			return nil
		}
		ch := e.changed
		e.mu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ch:
		}
	}
}

func (e *CacheEntry) CopyRange(ctx context.Context, w io.Writer, start, length int64) error {
	buffer := make([]byte, 64<<10)
	for offset, remaining := start, length; remaining > 0; {
		if err := e.Wait(ctx, offset); err != nil {
			return err
		}
		e.mu.Lock()
		available := e.received - offset
		finished := e.complete
		file := e.file
		e.mu.Unlock()
		if start == 0 && length == e.size && !finished && available >= remaining {
			available = remaining - 1
			if available <= 0 {
				if err := e.Wait(ctx, e.size); err != nil {
					return err
				}
				continue
			}
		}
		if available <= 0 {
			if finished {
				return io.ErrUnexpectedEOF
			}
			continue
		}
		next := int64(len(buffer))
		if next > available {
			next = available
		}
		if next > remaining {
			next = remaining
		}
		n, err := file.ReadAt(buffer[:int(next)], offset)
		if n > 0 {
			written, writeErr := w.Write(buffer[:n])
			if writeErr != nil {
				return writeErr
			}
			if written != n {
				return io.ErrShortWrite
			}
			offset += int64(n)
			remaining -= int64(n)
		}
		if err != nil && err != io.EOF {
			return err
		}
	}
	// Full responses finish only after integrity validation, so truncated or
	// invalid fills are never reported as successful complete objects.
	if start == 0 && length == e.size {
		for {
			e.mu.Lock()
			done, err, ch := e.complete, e.err, e.changed
			e.mu.Unlock()
			if err != nil {
				return err
			}
			if done {
				return nil
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ch:
			}
		}
	}
	return nil
}

type MediaCache struct {
	metrics   mediaMetrics
	cfg       GatewayConfig
	mu        sync.Mutex
	entries   map[string]*CacheEntry
	fillSlots chan struct{}
	limiter   *DirectionLimiter
	client    *http.Client
}

func NewMediaCache(cfg GatewayConfig) (*MediaCache, error) {
	if cfg.CacheDir == "" || !filepath.IsAbs(cfg.CacheDir) {
		return nil, fmt.Errorf("absolute dedicated cache directory required")
	}
	if err := os.MkdirAll(cfg.CacheDir, 0700); err != nil {
		return nil, err
	}
	if info, err := os.Lstat(cfg.CacheDir); err != nil || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("cache root must not be a symlink")
	}
	resolved, err := filepath.EvalSymlinks(cfg.CacheDir)
	if err != nil || resolved == string(filepath.Separator) {
		return nil, fmt.Errorf("unsafe cache root")
	}
	if cfg.Root != "" {
		root, e := filepath.EvalSymlinks(cfg.Root)
		if e != nil {
			return nil, e
		}
		rel, e := filepath.Rel(root, resolved)
		if e != nil || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
			return nil, fmt.Errorf("cache must be outside original source root")
		}
	}
	marker := filepath.Join(cfg.CacheDir, ".asset-media-cache-v1")
	if _, err := os.Stat(marker); os.IsNotExist(err) {
		entries, e := os.ReadDir(cfg.CacheDir)
		if e != nil {
			return nil, e
		}
		if len(entries) > 0 {
			return nil, fmt.Errorf("refusing to adopt nonempty unmanaged cache directory")
		}
		if e = os.WriteFile(marker, []byte("asset-media-cache-v1\n"), 0600); e != nil {
			return nil, e
		}
	} else if err != nil {
		return nil, err
	}
	if cfg.QuotaBytes <= 0 {
		cfg.QuotaBytes = 2 * 1024 * 1024 * 1024 * 1024
	}
	if cfg.TemporaryBytes <= 0 {
		cfg.TemporaryBytes = 20 * 1024 * 1024 * 1024
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = 20 * time.Second
	return &MediaCache{cfg: cfg, entries: map[string]*CacheEntry{}, fillSlots: make(chan struct{}, 2),
		limiter: &DirectionLimiter{DayMbps: cfg.DayMbps, NightMbps: cfg.NightMbps},
		client:  &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return fmt.Errorf("origin redirects are forbidden") }}}, nil
}

func (c *MediaCache) originAllowed(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Fragment != "" {
		return false
	}
	for _, host := range c.cfg.AllowedOSSHosts {
		if strings.EqualFold(u.Host, host) {
			return true
		}
	}
	return false
}

func cacheIDValid(id string) bool {
	if len(id) != 64 {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil && strings.ToLower(id) == id
}

func (c *MediaCache) Open(ctx context.Context, t ReadTarget) (*CacheEntry, string, error) {
	if !cacheIDValid(t.ContentID) || t.Size < 0 {
		return nil, "", fmt.Errorf("invalid cache identity")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if existing := c.entries[t.ContentID]; existing != nil {
		existing.mu.Lock()
		if existing.version != t.SourceVersion || existing.size != t.Size || (t.SHA256 != "" && existing.sha256 != "" && t.SHA256 != existing.sha256) || (t.CRC64 != "" && existing.crc64 != "" && t.CRC64 != existing.crc64) {
			existing.mu.Unlock()
			return nil, "", fmt.Errorf("source_version_changed: active cache fingerprint differs")
		}
		if existing.err == nil {
			existing.refs++
			existing.lastAccess = time.Now()
			hit := "coalesced"
			if existing.complete {
				hit = "hit"
			}
			existing.mu.Unlock()
			return existing, hit, nil
		}
		if existing.refs > 0 {
			existing.mu.Unlock()
			return nil, "", fmt.Errorf("previous transfer is finishing")
		}
		if existing.file != nil {
			existing.file.Close()
		}
		existing.mu.Unlock()
		delete(c.entries, t.ContentID)
	}
	dataPath := filepath.Join(c.cfg.CacheDir, t.ContentID+".data")
	metaPath := filepath.Join(c.cfg.CacheDir, t.ContentID+".json")
	if raw, err := os.ReadFile(metaPath); err == nil {
		var receipt cacheReceipt
		if json.Unmarshal(raw, &receipt) == nil && receipt.ContentID == t.ContentID && receipt.SourceVersion == t.SourceVersion && receipt.Size == t.Size && (t.SHA256 == "" || receipt.SHA256 == t.SHA256) && (t.CRC64 == "" || receipt.CRC64 == t.CRC64) {
			if f, e := os.Open(dataPath); e == nil {
				if st, e := f.Stat(); e == nil && st.Size() == t.Size && st.ModTime().UnixNano() == receipt.ModifiedNS {
					entry := &CacheEntry{version: t.SourceVersion, sha256: t.SHA256, crc64: t.CRC64, id: t.ContentID, file: f, path: dataPath, size: t.Size, received: t.Size, complete: true, changed: make(chan struct{}), refs: 1, lastAccess: time.Now()}
					c.entries[t.ContentID] = entry
					return entry, "hit", nil
				}
				f.Close()
			}
		}
	}
	free, err := AvailableBytes(c.cfg.CacheDir)
	if err != nil {
		return nil, "", err
	}
	if free < c.cfg.FreeFloorBytes || uint64(t.Size) > free-c.cfg.FreeFloorBytes {
		return nil, "", fmt.Errorf("cache free-space floor reached")
	}
	if err = c.reserveLocked(t.Size); err != nil {
		return nil, "", err
	}
	var pending int64
	for _, e := range c.entries {
		e.mu.Lock()
		if !e.complete && e.err == nil {
			pending += e.size
		}
		e.mu.Unlock()
	}
	files, err := os.ReadDir(c.cfg.CacheDir)
	if err != nil {
		return nil, "", err
	}
	for _, file := range files {
		id := strings.TrimSuffix(file.Name(), ".part")
		if id == file.Name() {
			id = strings.TrimSuffix(file.Name(), ".snapshot-part")
		}
		if !cacheIDValid(id) || id == t.ContentID || c.entries[id] != nil {
			continue
		}
		st, e := file.Info()
		if e != nil {
			return nil, "", e
		}
		if !st.Mode().IsRegular() {
			return nil, "", fmt.Errorf("invalid cache temporary file")
		}
		pending += st.Size()
	}
	if pending+t.Size > c.cfg.TemporaryBytes {
		return nil, "", fmt.Errorf("temporary cache capacity reached")
	}
	if t.Source == "nas" {
		entry := &CacheEntry{version: t.SourceVersion, sha256: t.SHA256, crc64: t.CRC64, id: t.ContentID, path: dataPath, size: t.Size, changed: make(chan struct{}), refs: 1, lastAccess: time.Now()}
		c.entries[t.ContentID] = entry
		go c.prepareNAS(entry, t, dataPath)
		return entry, "local", nil
	}
	if t.Source == "artifact" {
		return nil, "", fmt.Errorf("prepared artifact absent")
	}
	if !c.originAllowed(t.OriginURL) {
		return nil, "", fmt.Errorf("origin denied")
	}
	partPath := filepath.Join(c.cfg.CacheDir, t.ContentID+".part")
	resumePath := filepath.Join(c.cfg.CacheDir, t.ContentID+".resume")
	flags := os.O_CREATE | os.O_TRUNC | os.O_RDWR
	if raw, e := os.ReadFile(resumePath); e == nil && (t.SHA256 != "" || t.CRC64 != "") {
		var previous cacheReceipt
		if json.Unmarshal(raw, &previous) == nil && previous.ContentID == t.ContentID && previous.SourceVersion == t.SourceVersion && previous.Size == t.Size && previous.SHA256 == t.SHA256 && previous.CRC64 == t.CRC64 {
			if st, e := os.Lstat(partPath); e == nil && st.Mode().IsRegular() && st.Size() > 0 && st.Size() < t.Size {
				flags = os.O_RDWR
			}
		}
	}
	f, err := os.OpenFile(partPath, flags, 0600)
	if err != nil {
		return nil, "", err
	}
	resume, _ := json.Marshal(cacheReceipt{ContentID: t.ContentID, SourceVersion: t.SourceVersion, Size: t.Size, SHA256: t.SHA256, CRC64: t.CRC64})
	if err = os.WriteFile(resumePath, resume, 0600); err != nil {
		f.Close()
		return nil, "", err
	}
	entry := &CacheEntry{version: t.SourceVersion, sha256: t.SHA256, crc64: t.CRC64, id: t.ContentID, file: f, path: partPath, size: t.Size, changed: make(chan struct{}), refs: 1, lastAccess: time.Now()}
	c.entries[t.ContentID] = entry
	go c.fill(entry, t, dataPath)
	return entry, "miss", nil
}

func (c *MediaCache) Release(e *CacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e.mu.Lock()
	defer e.mu.Unlock()
	e.refs--
	e.lastAccess = time.Now()
	if e.refs == 0 && (e.complete || e.err != nil) {
		if e.file != nil {
			e.file.Close()
		}
		delete(c.entries, e.id)
		if e.complete {
			_ = os.Chtimes(filepath.Join(c.cfg.CacheDir, e.id+".json"), e.lastAccess, e.lastAccess)
		}
	}
}

func (c *MediaCache) finishEntry(e *CacheEntry, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e.mu.Lock()
	defer e.mu.Unlock()
	if err != nil {
		e.err = err
	}
	e.signal()
	if e.refs == 0 && (e.complete || e.err != nil) {
		if e.file != nil {
			e.file.Close()
		}
		delete(c.entries, e.id)
	}
}

func (c *MediaCache) prepareNAS(entry *CacheEntry, t ReadTarget, dataPath string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	var failure error
	defer func() { c.finishEntry(entry, failure) }()
	partPath := filepath.Join(c.cfg.CacheDir, t.ContentID+".snapshot-part")
	_ = os.Remove(partPath)
	if err := CloneStableSource(c.cfg.Root, t.RelativePath, t, partPath); err != nil {
		failure = err
		return
	}
	if err := os.Rename(partPath, dataPath); err != nil {
		failure = err
		return
	}
	f, err := os.Open(dataPath)
	if err != nil {
		failure = err
		return
	}
	sha, crc, err := hashFile(ctx, f)
	if err != nil {
		f.Close()
		failure = err
		return
	}
	if (t.SHA256 != "" && t.SHA256 != sha) || (t.CRC64 != "" && t.CRC64 != crc) {
		f.Close()
		failure = fmt.Errorf("snapshot checksum mismatch")
		return
	}
	if err = c.writeReceipt(t, dataPath, sha, crc); err != nil {
		f.Close()
		failure = err
		return
	}
	entry.mu.Lock()
	entry.file = f
	entry.received = t.Size
	entry.complete = true
	entry.mu.Unlock()
}

func (c *MediaCache) reserveLocked(incoming int64) error {
	files, err := os.ReadDir(c.cfg.CacheDir)
	if err != nil {
		return err
	}
	type candidate struct {
		id, path string
		size     int64
		used     time.Time
	}
	var total int64
	list := []candidate{}
	for _, f := range files {
		name := f.Name()
		for _, suffix := range []string{".part", ".snapshot-part", ".resume", ".json.tmp"} {
			id := strings.TrimSuffix(name, suffix)
			if id == name || !cacheIDValid(id) || c.entries[id] != nil {
				continue
			}
			st, e := f.Info()
			if e != nil {
				return e
			}
			if st.Mode().IsRegular() && time.Since(st.ModTime()) > 24*time.Hour {
				if e = os.Remove(filepath.Join(c.cfg.CacheDir, name)); e != nil && !os.IsNotExist(e) {
					return e
				}
			}
		}
		if !strings.HasSuffix(name, ".data") || !cacheIDValid(strings.TrimSuffix(name, ".data")) {
			continue
		}
		st, e := f.Info()
		if e != nil {
			return e
		}
		total += st.Size()
		id := strings.TrimSuffix(name, ".data")
		used := st.ModTime()
		busy := false
		if receipt, err := os.Stat(filepath.Join(c.cfg.CacheDir, id+".json")); err == nil {
			used = receipt.ModTime()
		}
		if active := c.entries[id]; active != nil {
			active.mu.Lock()
			busy = active.refs > 0 || !active.complete
			used = active.lastAccess
			active.mu.Unlock()
		}
		if !busy {
			list = append(list, candidate{id: id, path: filepath.Join(c.cfg.CacheDir, name), size: st.Size(), used: used})
		}
	}
	if total+incoming < c.cfg.QuotaBytes*85/100 {
		return nil
	}
	sort.Slice(list, func(i, j int) bool { return list[i].used.Before(list[j].used) })
	for _, item := range list {
		if total+incoming <= c.cfg.QuotaBytes*75/100 {
			break
		}
		if entry := c.entries[item.id]; entry != nil {
			if entry.file != nil {
				entry.file.Close()
			}
			delete(c.entries, item.id)
		}
		if err = os.Remove(item.path); err != nil {
			return err
		}
		_ = os.Remove(filepath.Join(c.cfg.CacheDir, item.id+".json"))
		total -= item.size
	}
	if total+incoming > c.cfg.QuotaBytes {
		return fmt.Errorf("cache quota exceeded")
	}
	return nil
}

func (c *MediaCache) fill(entry *CacheEntry, t ReadTarget, dataPath string) {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Hour)
	defer cancel()
	var failure error
	defer func() { c.finishEntry(entry, failure) }()
	select {
	case c.fillSlots <- struct{}{}:
		defer func() { <-c.fillSlots }()
	case <-ctx.Done():
		failure = ctx.Err()
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.OriginURL, nil)
	if err != nil {
		failure = fmt.Errorf("invalid origin request")
		return
	}
	sha := sha256.New()
	crc := crc64.New(crc64.MakeTable(crc64.ECMA))
	var count int64
	if st, e := entry.file.Stat(); e == nil {
		count = st.Size()
	} else {
		failure = e
		return
	}
	if count > 0 {
		if _, err = io.Copy(io.MultiWriter(sha, crc), io.NewSectionReader(entry.file, 0, count)); err != nil {
			failure = err
			return
		}
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", count))
	}
	resp, err := c.client.Do(req)
	c.metrics.OriginRequests.Add(1)
	if err != nil {
		failure = fmt.Errorf("origin transport failed")
		return
	}
	defer resp.Body.Close()
	expectedStatus := http.StatusOK
	if count > 0 {
		expectedStatus = http.StatusPartialContent
	}
	if resp.StatusCode != expectedStatus {
		failure = fmt.Errorf("origin status %d", resp.StatusCode)
		return
	}
	if count > 0 && resp.Header.Get("Content-Range") != fmt.Sprintf("bytes %d-%d/%d", count, t.Size-1, t.Size) {
		failure = fmt.Errorf("origin resume range mismatch")
		return
	}
	if resp.ContentLength >= 0 && resp.ContentLength != t.Size-count {
		failure = fmt.Errorf("origin length mismatch")
		return
	}
	buffer := make([]byte, 64<<10)
	entry.mu.Lock()
	entry.received = count
	entry.signal()
	entry.mu.Unlock()
	stall := time.AfterFunc(90*time.Second, cancel)
	defer stall.Stop()
	for {
		n, e := resp.Body.Read(buffer)
		if n > 0 {
			c.metrics.OriginBytes.Add(uint64(n))
			stall.Reset(90 * time.Second)
			if count+int64(n) > t.Size {
				failure = fmt.Errorf("origin exceeds expected length")
				return
			}
			if err = c.limiter.Wait(ctx, n); err != nil {
				failure = err
				return
			}
			if _, err = entry.file.WriteAt(buffer[:n], count); err != nil {
				failure = err
				return
			}
			_, _ = sha.Write(buffer[:n])
			_, _ = crc.Write(buffer[:n])
			count += int64(n)
			entry.mu.Lock()
			entry.received = count
			entry.signal()
			entry.mu.Unlock()
		}
		if e == io.EOF {
			break
		}
		if e != nil {
			failure = fmt.Errorf("origin stream interrupted")
			return
		}
	}
	actualSHA := hex.EncodeToString(sha.Sum(nil))
	actualCRC := strconv.FormatUint(crc.Sum64(), 10)
	expectedCRC := t.CRC64
	if expectedCRC == "" {
		expectedCRC = resp.Header.Get("x-oss-hash-crc64ecma")
	}
	if count != t.Size || (t.SHA256 != "" && t.SHA256 != actualSHA) || (expectedCRC != "" && expectedCRC != actualCRC) {
		_ = os.Remove(filepath.Join(c.cfg.CacheDir, t.ContentID+".resume"))
		failure = fmt.Errorf("origin integrity verification failed")
		return
	}
	if t.SHA256 == "" && expectedCRC == "" {
		failure = fmt.Errorf("origin checksum unavailable")
		return
	}
	if err = entry.file.Sync(); err != nil {
		failure = err
		return
	}
	if err = os.Rename(entry.path, dataPath); err != nil {
		failure = err
		return
	}
	if err = c.writeReceipt(t, dataPath, actualSHA, actualCRC); err != nil {
		failure = err
		return
	}
	entry.mu.Lock()
	entry.path = dataPath
	entry.complete = true
	entry.signal()
	entry.mu.Unlock()
	_ = os.Remove(filepath.Join(c.cfg.CacheDir, t.ContentID+".resume"))
}

func hashFile(ctx context.Context, f *os.File) (string, string, error) {
	sha := sha256.New()
	crc := crc64.New(crc64.MakeTable(crc64.ECMA))
	buf := make([]byte, 256<<10)
	var off int64
	for {
		if err := ctx.Err(); err != nil {
			return "", "", err
		}
		n, e := f.ReadAt(buf, off)
		if n > 0 {
			_, _ = sha.Write(buf[:n])
			_, _ = crc.Write(buf[:n])
			off += int64(n)
		}
		if e == io.EOF {
			break
		}
		if e != nil {
			return "", "", e
		}
	}
	return hex.EncodeToString(sha.Sum(nil)), strconv.FormatUint(crc.Sum64(), 10), nil
}

func (c *MediaCache) writeReceipt(t ReadTarget, path, sha, crc string) error {
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(cacheReceipt{ContentID: t.ContentID, SourceVersion: t.SourceVersion, Size: t.Size, SHA256: sha, CRC64: crc, ModifiedNS: st.ModTime().UnixNano()})
	if err != nil {
		return err
	}
	tmp := filepath.Join(c.cfg.CacheDir, t.ContentID+".json.tmp")
	if err = os.WriteFile(tmp, raw, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(c.cfg.CacheDir, t.ContentID+".json"))
}
