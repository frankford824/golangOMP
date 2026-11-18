package platform

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIdentityNeedsAnActiveSandboxSession(t *testing.T) {
	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/auth/me" || r.Header.Get("Authorization") != "Bearer sandbox" {
			w.WriteHeader(401)
			return
		}
		w.Write([]byte(`{"data":{"id":7,"status":"active","department_id":2,"employee_no":0,"frontend_access":{"actions":["task.view"],"roles":["member"]}}}`))
	}))
	defer core.Close()
	handler := NewIdentity(core.URL).Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a := ActorFrom(r)
		if a.ID != 7 || a.SuperAdmin || a.EmployeeNo == nil || !a.Actions["task.view"] {
			t.Fatal("incorrect subject")
		}
		w.WriteHeader(204)
	}))
	for _, tt := range []struct {
		token  string
		status int
	}{{"", 401}, {"production", 401}, {"sandbox", 204}} {
		r := httptest.NewRequest("GET", "/oa-api/me", nil)
		r.Header.Set("X-User-ID", "1")
		r.Header.Set("X-SuperAdmin", "true")
		if tt.token != "" {
			r.Header.Set("Authorization", "Bearer "+tt.token)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != tt.status {
			t.Fatalf("status=%d, want %d", w.Code, tt.status)
		}
	}
}
