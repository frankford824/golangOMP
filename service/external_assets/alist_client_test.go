package externalassets

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestBFFOpenFetchRetriesBusyUpstream(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"code":"UPSTREAM_BUSY","retry_after_ms":1}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	}))
	defer server.Close()

	client := NewBFFClient(server.URL, "", time.Second)
	body, err := client.OpenFetch(context.Background(), "/quark/poster.psd", false)
	if err != nil {
		t.Fatalf("OpenFetch() error = %v", err)
	}
	defer body.Close()
	raw, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(raw) != "ready" || calls.Load() != 2 {
		t.Fatalf("body=%q calls=%d, want ready/2", raw, calls.Load())
	}
}
