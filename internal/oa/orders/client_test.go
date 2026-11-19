package orders

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"
)

func useEndpoint(t *testing.T, endpoint *httptest.Server) *client {
	t.Helper()
	t.Setenv("OA_CLICKHOUSE_URL", endpoint.URL)
	t.Setenv("OPS_READER_PASSWORD", "fixture")
	return newClient()
}

func TestClientReuseAndBoundParameters(t *testing.T) {
	var connections atomic.Int32
	endpoint := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("param_shop") != "abc'xyz" || r.Header.Get("X-ClickHouse-User") != "ops_reader" || r.Header.Get("X-ClickHouse-Key") != "fixture" {
			t.Error("missing bounded query parameters or authentication")
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != "SELECT {shop:String} FORMAT JSON" {
			t.Errorf("unexpected SQL: %s", body)
		}
		_, _ = io.WriteString(w, `{"data":[{"amount":"0.00"}]}`)
	}))
	endpoint.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			connections.Add(1)
		}
	}
	endpoint.Start()
	defer endpoint.Close()
	client := useEndpoint(t, endpoint)
	for range 2 {
		if _, err := client.rows(context.Background(), "SELECT {shop:String}", url.Values{"param_shop": {"abc'xyz"}}); err != nil {
			t.Fatal(err)
		}
	}
	if connections.Load() != 1 {
		t.Fatalf("expected one connection, got %d", connections.Load())
	}
}

func TestClientRetriesDialAndClassifiesErrors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{"denied", 500, "DB::Exception: (ACCESS_DENIED)", errDenied},
		{"busy", 500, "DB::Exception: (TOO_MANY_SIMULTANEOUS_QUERIES)", errBusy},
		{"timeout", 500, "DB::Exception: (TIMEOUT_EXCEEDED)", errTimeout},
		{"unavailable", 503, "offline", errUnavailable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.status)
				_, _ = io.WriteString(w, test.body)
			}))
			defer endpoint.Close()
			client := useEndpoint(t, endpoint)
			transport := client.http.Transport.(*http.Transport)
			dial := transport.DialContext
			var attempts atomic.Int32
			transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
				if attempts.Add(1) == 1 {
					return nil, &net.OpError{Op: "dial", Err: errors.New("fixture connection refused")}
				}
				return dial(ctx, network, addr)
			}
			_, err := client.rows(context.Background(), "SELECT 1", nil)
			if !errors.Is(err, test.want) || attempts.Load() != 2 {
				t.Fatalf("got %v after %d attempts, want %v after 2", err, attempts.Load(), test.want)
			}
		})
	}
}

func TestCacheCoalescesAndRejectsUnauthorizedStaleData(t *testing.T) {
	var calls atomic.Int32
	var code atomic.Int32
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if status := code.Load(); status != 0 {
			w.WriteHeader(int(status))
			return
		}
		time.Sleep(10 * time.Millisecond)
		_, _ = io.WriteString(w, `{"data":[{"source_committed_at":"2026-11-18 09:00:00"}]}`)
	}))
	defer endpoint.Close()
	s := &server{ch: useEndpoint(t, endpoint), cache: map[string]cached{}, active: map[string]*flight{}}
	results := make(chan error, 8)
	for range 8 {
		go func() { _, err := s.load(context.Background(), "same", "summary", "SELECT 1", nil); results <- err }()
	}
	for range 8 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != 2 {
		t.Fatalf("got %d upstream calls, want 2", calls.Load())
	}
	previous := s.cache["same"]
	previous.fetched = time.Now().Add(-time.Minute)
	s.cache["same"] = previous
	code.Store(503)
	if data, err := s.load(context.Background(), "same", "summary", "SELECT 1", nil); err != nil || !data.Stale {
		t.Fatalf("expected same-key stale result, got %v %v", data.Stale, err)
	}
	if _, err := s.load(context.Background(), "new", "summary", "SELECT 1", nil); !errors.Is(err, errUnavailable) {
		t.Fatalf("fresh key inherited data: %v", err)
	}
	code.Store(403)
	if _, err := s.load(context.Background(), "same", "summary", "SELECT 1", nil); !errors.Is(err, errDenied) || len(s.cache) != 0 {
		t.Fatalf("expected cache purge and denial, got %v", err)
	}
}
