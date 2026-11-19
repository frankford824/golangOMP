package platform

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testTrace = "dededede77774444aaaa000000000001"

func TestTraceErrorFormatsAndBusinessCode(t *testing.T) {
	handler := Trace(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		ErrorWithCode(w, http.StatusConflict, "INVOICE_ALREADY_CLAIMED", "发票号码已被其他单据占用")
	}))
	for _, format := range []string{"", "v2"} {
		r := httptest.NewRequest(http.MethodPost, "/oa-api/finance/documents", nil)
		r.Header.Set(TraceHeader, testTrace)
		r.Header.Set(ErrorFormatHeader, format)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != http.StatusConflict || w.Header().Get(TraceHeader) != testTrace || w.Header().Get("Vary") != ErrorFormatHeader {
			t.Fatalf("format=%q got status=%d headers=%v", format, w.Code, w.Header())
		}
		var body map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if format == "v2" {
			envelope, ok := body["error"].(map[string]any)
			if !ok || envelope["code"] != "INVOICE_ALREADY_CLAIMED" || envelope["trace_id"] != testTrace || envelope["message"] != "发票号码已被其他单据占用" {
				t.Fatalf("invalid v2 body: %v", body)
			}
		} else if body["error"] != "发票号码已被其他单据占用" || body["code"] != "INVOICE_ALREADY_CLAIMED" || body["trace_id"] != testTrace {
			t.Fatalf("invalid legacy body: %v", body)
		}
	}
}

func TestTraceHandlesRouterErrorsAndRejectsMalformedRequestID(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /oa-api/me", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	r := httptest.NewRequest(http.MethodPost, "/oa-api/me", nil)
	r.Header.Set(TraceHeader, "private-query-data")
	r.Header.Set(ErrorFormatHeader, "v2")
	w := httptest.NewRecorder()
	Trace(mux).ServeHTTP(w, r)
	if w.Code != http.StatusMethodNotAllowed || !strings.Contains(w.Header().Get("Allow"), "GET") {
		t.Fatalf("unexpected router response: %v", w.Result())
	}
	trace := w.Header().Get(TraceHeader)
	if trace == "" || trace == "private-query-data" ||
		!strings.Contains(w.Body.String(), fmt.Sprintf("\"trace_id\":\"%s\"", trace)) ||
		!strings.Contains(w.Body.String(), "\"code\":\"METHOD_NOT_ALLOWED\"") {
		t.Fatalf("invalid fallback trace: %s / %s", trace, w.Body)
	}
}

func TestTracePropagatesToIdentityAndNativeCore(t *testing.T) {
	paths := make(chan string, 2)
	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths <- r.URL.Path
		if trace := r.Header.Get(TraceHeader); trace != testTrace {
			t.Errorf("%s core received trace=%q", r.URL.Path, trace)
		}
		if r.URL.Path == "/v1/auth/me" {
			fmt.Fprint(w, "{\"data\":{\"id\":7,\"status\":\"active\"}}")
			return
		}
		w.WriteHeader(http.StatusConflict)
		fmt.Fprint(w, "{\"error\":{\"code\":\"TEST_CONFLICT\"}}")
	}))
	defer core.Close()
	handler := Trace(NewIdentity(core.URL).Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = "/v1/users/7"
		Native(core.URL).ServeHTTP(w, r)
	})))
	r := httptest.NewRequest(http.MethodGet, "/oa-api/people/7", nil)
	r.Header.Set(TraceHeader, testTrace)
	r.Header.Set("Authorization", "Bearer fixture")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusConflict || w.Header().Get(TraceHeader) != testTrace {
		t.Fatalf("unexpected status/trace = %d / %s", w.Code, w.Header().Get(TraceHeader))
	}
	if got := len(paths); got != 2 {
		t.Fatalf("received %d core requests, want 2", got)
	}
}
