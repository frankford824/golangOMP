package assetmedia

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestGatewayResumesVerifiedPartialWithoutFullRefetch(t *testing.T) {
	body := bytes.Repeat([]byte("immutable-version"), 8192)
	const offset = 32000
	var reads atomic.Int32
	origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reads.Add(1)
		if r.Header.Get("Range") != fmt.Sprintf("bytes=%d-", offset) {
			t.Errorf("unexpected range %q", r.Header.Get("Range"))
			w.WriteHeader(400)
			return
		}
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", offset, len(body)-1, len(body)))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(body[offset:])
	}))
	defer origin.Close()
	g, endpoint, _ := gatewayFixture(t, body, origin)
	id := strings.Repeat("a", 64)
	sum := sha256.Sum256(body)
	raw, _ := json.Marshal(cacheReceipt{ContentID: id, SourceVersion: "ta:1", Size: int64(len(body)), SHA256: hex.EncodeToString(sum[:])})
	if err := os.WriteFile(filepath.Join(g.cfg.CacheDir, id+".resume"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(g.cfg.CacheDir, id+".part"), body[:offset], 0600); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	g.ServeHTTP(w, httptest.NewRequest("GET", endpoint, nil))
	if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), body) || reads.Load() != 1 {
		t.Fatalf("resume failed status=%d reads=%d bytes=%d", w.Code, reads.Load(), w.Body.Len())
	}
}

func TestGatewayPreservesOnlineRevocationStatus(t *testing.T) {
	origin := httptest.NewTLSServer(http.NotFoundHandler())
	defer origin.Close()
	g, endpoint, _ := gatewayFixture(t, []byte("source"), origin)
	for _, status := range []int{401, 403, 404, 409, 410, 503} {
		g.cfg.Authorize = func(context.Context, string) (ReadTarget, error) {
			return ReadTarget{}, &AuthorizationError{Status: status}
		}
		w := httptest.NewRecorder()
		g.ServeHTTP(w, httptest.NewRequest("HEAD", endpoint, nil))
		if w.Code != status {
			t.Errorf("status=%d want=%d", w.Code, status)
		}
	}
}

func TestGatewayIfRangeMismatchReturnsWholeBoundRepresentation(t *testing.T) {
	body := []byte("version-bound-body")
	origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(body) }))
	defer origin.Close()
	g, endpoint, _ := gatewayFixture(t, body, origin)
	req := httptest.NewRequest("GET", endpoint, nil)
	req.Header.Set("Range", "bytes=2-5")
	req.Header.Set("If-Range", `"old-fingerprint"`)
	w := httptest.NewRecorder()
	g.ServeHTTP(w, req)
	if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), body) {
		t.Fatalf("must restart instead of splice: status=%d body=%q", w.Code, w.Body.String())
	}
	etag := w.Header().Get("ETag")
	sum := sha256.Sum256(body)
	if !strings.Contains(etag, hex.EncodeToString(sum[:])) {
		t.Fatal("strong validator not bound to representation fingerprint")
	}
	req = httptest.NewRequest("GET", endpoint, nil)
	req.Header.Set("Range", "bytes=2-5")
	req.Header.Set("If-Range", etag)
	w = httptest.NewRecorder()
	g.ServeHTTP(w, req)
	if w.Code != 206 || w.Body.String() != string(body[2:6]) {
		t.Fatal("matching fingerprint did not resume")
	}
}

func gatewayFixture(t *testing.T, body []byte, origin *httptest.Server) (*Gateway, string, *atomic.Int32) {
	t.Helper()
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	sum := sha256.Sum256(body)
	content := strings.Repeat("a", 64)
	var authCalls atomic.Int32
	u, _ := url.Parse(origin.URL)
	g, err := NewGateway(GatewayConfig{ID: "office", CacheDir: t.TempDir(), Keys: map[string]ed25519.PublicKey{"v1": pub}, AllowedOSSHosts: []string{u.Host}, AllowedOrigins: []string{"https://yongbo.cloud"},
		Authorize: func(context.Context, string) (ReadTarget, error) {
			authCalls.Add(1)
			return ReadTarget{Source: "oss", ContentID: content, SourceVersion: "ta:1", Size: int64(len(body)), SHA256: hex.EncodeToString(sum[:]), OriginURL: origin.URL, Filename: "设计原件.psd", MimeType: "application/octet-stream"}, nil
		}})
	if err != nil {
		t.Fatal(err)
	}
	g.cache.client = origin.Client()
	now := time.Now()
	token, err := SignTicket(Ticket{Version: 1, KeyID: "v1", GatewayID: "office", ActorID: 1, ResourceKind: "task_asset", ResourceID: "1", SourceVersion: "ta:1", ContentID: content, Purpose: "download", Rendition: "original", IssuedAt: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix()}, priv)
	if err != nil {
		t.Fatal(err)
	}
	return g, "/edge/v1/content/" + content + "?ticket=" + url.QueryEscape(token), &authCalls
}

func TestGatewayTwentyReadersShareOneOriginAndReauthorizeHits(t *testing.T) {
	body := bytes.Repeat([]byte("immutable-media"), 8192)
	var reads atomic.Int32
	origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reads.Add(1)
		time.Sleep(25 * time.Millisecond)
		_, _ = w.Write(body)
	}))
	defer origin.Close()
	g, path, auth := gatewayFixture(t, body, origin)
	server := httptest.NewServer(g)
	defer server.Close()
	var wg sync.WaitGroup
	errors := make(chan string, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := http.Get(server.URL + path)
			if err != nil {
				errors <- err.Error()
				return
			}
			defer resp.Body.Close()
			got, err := io.ReadAll(resp.Body)
			if err != nil || resp.StatusCode != 200 || !bytes.Equal(got, body) {
				errors <- "incorrect response"
			}
		}()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
	if reads.Load() != 1 {
		t.Fatalf("origin requests=%d, want one", reads.Load())
	}
	if auth.Load() != 20 {
		t.Fatalf("authorization calls=%d, want every request", auth.Load())
	}
	req, _ := http.NewRequest(http.MethodGet, server.URL+path, nil)
	req.Header.Set("Range", "bytes=23-78")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	got, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 206 || !bytes.Equal(got, body[23:79]) {
		t.Fatalf("range status=%d body size=%d", resp.StatusCode, len(got))
	}
	if reads.Load() != 1 || auth.Load() != 21 {
		t.Fatal("cache hit bypassed authorization or fetched again")
	}
}

func TestGatewayColdFileStreamsBeforeOriginCompletes(t *testing.T) {
	body := bytes.Repeat([]byte("a"), 128<<10)
	release := make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(release) })
	origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body[:64<<10])
		w.(http.Flusher).Flush()
		<-release
		_, _ = w.Write(body[64<<10:])
	}))
	defer origin.Close()
	g, path, _ := gatewayFixture(t, body, origin)
	server := httptest.NewServer(g)
	defer server.Close()
	ready := make(chan error, 1)
	go func() {
		resp, err := http.Get(server.URL + path)
		if err != nil {
			ready <- err
			return
		}
		defer resp.Body.Close()
		b := make([]byte, 1024)
		_, err = io.ReadFull(resp.Body, b)
		ready <- err
		_, _ = io.Copy(io.Discard, resp.Body)
	}()
	select {
	case err := <-ready:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cold file waited for complete buffering")
	}
	once.Do(func() { close(release) })
}

func TestGatewayBadChecksumNeverCommitsCacheOrCompletesFile(t *testing.T) {
	expected := bytes.Repeat([]byte("a"), 128<<10)
	bad := bytes.Repeat([]byte("b"), len(expected))
	origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(bad) }))
	defer origin.Close()
	g, path, _ := gatewayFixture(t, expected, origin)
	server := httptest.NewServer(g)
	defer server.Close()
	resp, err := http.Get(server.URL + path)
	if err == nil {
		defer resp.Body.Close()
		got, readErr := io.ReadAll(resp.Body)
		if resp.StatusCode == 200 && readErr == nil && len(got) == len(expected) {
			t.Fatal("corrupt file reported complete")
		}
	}
	if _, err = os.Stat(filepath.Join(g.cfg.CacheDir, strings.Repeat("a", 64)+".json")); !os.IsNotExist(err) {
		t.Fatal("corrupt fill committed a receipt")
	}
}

func TestSingleRangeRejectsMultipleAndInvalidOffsets(t *testing.T) {
	for _, raw := range []string{"bytes=0-1,4-5", "bytes=100-", "bytes=-0", "bytes=-", "items=0-1"} {
		if _, _, err := parseSingleRange(raw, 100); err == nil {
			t.Errorf("accepted %q", raw)
		}
	}
	start, end, err := parseSingleRange("bytes=-10", 100)
	if err != nil || start != 90 || end != 99 {
		t.Fatalf("suffix=%d,%d,%v", start, end, err)
	}
}
