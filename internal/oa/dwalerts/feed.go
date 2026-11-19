// Package dwalerts exposes the private watchdog inbox to authenticated OA administrators.
package dwalerts

import (
	"context"
	"io"
	"net"
	"net/http"
	"time"

	"workflow/internal/oa/platform"
)

const feedSocket = "/run/oa-dw-alerts/feed.sock"

var client = &http.Client{
	Timeout: 2 * time.Second,
	Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", feedSocket)
	}},
}

func List(w http.ResponseWriter, r *http.Request) {
	if !platform.ActorFrom(r).SuperAdmin {
		platform.Error(w, http.StatusForbidden, "仅超级管理员可查看数据层告警")
		return
	}
	path := "/recent"
	if r.URL.Query().Get("summary") == "1" {
		path = "/summary"
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, "http://localhost"+path, nil)
	if err == nil {
		var resp *http.Response
		resp, err = client.Do(req)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				body, readErr := io.ReadAll(resp.Body)
				if readErr == nil {
					w.Header().Set("Content-Type", "application/json; charset=utf-8")
					w.Header().Set("Cache-Control", "no-store")
					_, _ = w.Write(body)
					return
				}
			}
		}
	}
	platform.Error(w, http.StatusServiceUnavailable, "数据告警通知暂时不可用")
}
