package platform

import (
	"io"
	"net/http"
	"strings"
	"time"
)

// Native forwards only explicit business operations to the existing platform.
// No privileged service token, caller-selected host, redirects or synthetic identities.
func Native(base string) http.Handler {
	client := &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		q := r.URL.Query()
		taskSearch := path == "/v1/search" && q.Get("scope") == "tasks" && q.Get("mode") == "exact"
		allowed := r.Method == "GET" && (taskSearch || path == "/v1/cost-management/sku-costs" || numericPath(path, "/v1/tasks/") || numericPath(path, "/v1/users/")) ||
			r.Method == "PATCH" && numericPath(path, "/v1/users/")
		if !allowed {
			http.Error(w, "unsupported native operation", 403)
			return
		}
		target := base + path
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}
		req, err := http.NewRequestWithContext(r.Context(), r.Method, target, r.Body)
		if err != nil {
			Error(w, 500, "无法构造主站请求")
			return
		}
		req.Header.Set("Authorization", r.Header.Get("Authorization"))
		if r.Method == "PATCH" {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := client.Do(req)
		if err != nil {
			Error(w, 503, "主站业务服务暂不可用")
			return
		}
		defer resp.Body.Close()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, io.LimitReader(resp.Body, 8<<20))
	})
}
func numericPath(path, prefix string) bool {
	id, ok := strings.CutPrefix(path, prefix)
	if !ok || id == "" || len(id) > 19 {
		return false
	}
	for _, c := range id {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
