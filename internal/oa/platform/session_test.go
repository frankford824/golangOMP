package platform

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOABrowserSession(t *testing.T) {
	issued := strings.Repeat("a", 64)
	legacy := strings.Repeat("b", 64)
	expired := strings.Repeat("c", 64)
	unavailable := strings.Repeat("d", 64)
	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/auth/login", "/v1/auth/register":
			if r.Method != http.MethodPost || r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
				t.Error("login request forwarded browser credentials")
			}
			http.SetCookie(w, &http.Cookie{Name: "upstream-only", Value: issued})
			if r.URL.Path == "/v1/auth/register" {
				w.WriteHeader(http.StatusCreated)
			}
			fmt.Fprintf(w, "{\"data\":{\"user\":{\"id\":42},\"session\":{\"token\":%q,\"session_id\":\"internal\"}}}", issued)
		case "/v1/auth/me":
			switch browserSessionBearer(r) {
			case issued, legacy:
				fmt.Fprint(w, "{\"data\":{\"id\":42,\"status\":\"active\"}}")
			case unavailable:
				w.WriteHeader(http.StatusServiceUnavailable)
			default:
				w.WriteHeader(http.StatusUnauthorized)
			}
		default:
			t.Error("unexpected upstream request")
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer core.Close()
	handler := BrowserSessions(http.NotFoundHandler(), core.URL)
	request := func(method, path, origin, bearer, cookie string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader("{}"))
		req.Header.Set("Origin", origin)
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		if cookie != "" {
			req.AddCookie(&http.Cookie{Name: oaSessionCookie, Value: cookie})
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w
	}
	for _, tc := range []struct {
		name, method, path, origin, bearer, cookie, expectedCookie string
		status                                                     int
	}{
		{"login", "POST", "/v1/auth/login", oaBrowserOrigin, "", "", issued, 200},
		{"register", "POST", "/v1/auth/register", oaBrowserOrigin, "", "", issued, 201},
		{"cross-site login", "POST", "/v1/auth/login", "https://evil.example", "", "", "", 403},
		{"same-site sibling login", "POST", "/v1/auth/login", "https://yongbo.cloud", "", "", "", 403},
		{"logout", "POST", "/v1/auth/logout", oaBrowserOrigin, issued, "", "delete", 200},
		{"missing-origin logout", "POST", "/v1/auth/logout", "", issued, "", "", 403},
		{"unsafe logout method", "GET", "/v1/auth/logout", oaBrowserOrigin, issued, "", "", 405},
		{"legacy migration", "POST", "/oa-api/session/migrate", oaBrowserOrigin, legacy, "", legacy, 200},
		{"legacy cannot replace current cookie", "POST", "/oa-api/session/migrate", oaBrowserOrigin, legacy, issued, "legacy-deletions-only", 200},
		{"legacy replaces expired cookie", "POST", "/oa-api/session/migrate", oaBrowserOrigin, legacy, expired, legacy, 200},
		{"legacy cookie validation outage", "POST", "/oa-api/session/migrate", oaBrowserOrigin, legacy, unavailable, "", 503},
		{"expired legacy", "POST", "/oa-api/session/migrate", oaBrowserOrigin, expired, "", "", 401},
		{"stale refresh cannot overwrite cookie", "POST", "/v1/auth/asset-cookie", oaBrowserOrigin, issued, "", "", 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := request(tc.method, tc.path, tc.origin, tc.bearer, tc.cookie)
			if w.Code != tc.status || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("unexpected status or cache policy: %d %q", w.Code, w.Header().Get("Cache-Control"))
			}
			body := w.Body.String()
			if strings.Contains(body, issued) || strings.Contains(body, legacy) || strings.Contains(body, "session_id") {
				t.Fatal("browser response body contains a core session credential")
			}
			cookies := w.Result().Cookies()
			if tc.expectedCookie == "" {
				if len(cookies) != 0 {
					t.Fatal("unexpected browser cookie")
				}
				return
			}
			if tc.expectedCookie == "legacy-deletions-only" {
				assertExpiredLegacyOACookies(t, cookies)
				return
			}
			if len(cookies) == 0 {
				t.Fatal("missing OA session cookie")
			}
			c := cookies[0]
			assertExpiredLegacyOACookies(t, cookies[1:])
			if c.Name != oaSessionCookie || c.Domain != "" || c.Path != "/" || !c.Secure || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode {
				t.Fatal("session cookie attributes do not enforce OA scope")
			}
			if tc.expectedCookie == "delete" {
				if c.Value != "" || c.MaxAge >= 0 {
					t.Fatal("logout did not expire the session cookie")
				}
			} else if c.Value != tc.expectedCookie || c.MaxAge != 0 {
				t.Fatal("incorrect cookie session lifetime or value")
			}
		})
	}
}

func assertExpiredLegacyOACookies(t *testing.T, cookies []*http.Cookie) {
	t.Helper()
	want := map[[3]string]bool{}
	for name, path := range map[string]string{
		"wf_asset_token": "/v1/assets/files", "wf_asset_stream_token": "/v1/assets", "wf_ws_token": "/ws",
	} {
		want[[3]string{name, path, ""}] = true
		want[[3]string{name, path, oaBrowserHost}] = true
	}
	for _, c := range cookies {
		key := [3]string{c.Name, c.Path, c.Domain}
		if !want[key] || c.MaxAge >= 0 || c.Value != "" {
			t.Fatal("unexpected legacy cookie deletion target")
		}
		delete(want, key)
	}
	if len(want) != 0 {
		t.Fatal("missing legacy OA cookie deletions")
	}
}
