package platform

import (
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
)

const (
	oaBrowserHost   = "oa.yongbo.cloud"
	oaSessionCookie = "__Host-yb-oa-session"
	oaBrowserOrigin = "https://" + oaBrowserHost
)

var oaSessionToken = regexp.MustCompile(`^[0-9a-f]{64}$`)

// BrowserSessions adapts core sessions to the OA browser contract. The core
// still validates the session and extends its idle expiry on authenticated requests.
func BrowserSessions(next http.Handler, coreURL string) http.Handler {
	identity := NewIdentity(coreURL)
	routes := http.NewServeMux()
	for _, path := range []string{"/v1/auth/login", "/v1/auth/register", "/v1/auth/asset-cookie", "/v1/auth/logout", "/oa-api/session/migrate"} {
		routes.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			if r.Method != http.MethodPost {
				w.Header().Set("Allow", http.MethodPost)
				Error(w, http.StatusMethodNotAllowed, "请求方法不支持")
				return
			}
			if r.Header.Get("Origin") != oaBrowserOrigin {
				Error(w, http.StatusForbidden, "请求来源无权操作 OA 会话")
				return
			}
			switch path {
			case "/v1/auth/login", "/v1/auth/register":
				identity.issueBrowserSession(w, r)
			case "/v1/auth/asset-cookie", "/oa-api/session/migrate":
				token := browserSessionBearer(r)
				if !oaSessionToken.MatchString(token) {
					Error(w, http.StatusUnauthorized, "登录已失效，请重新登录")
					return
				}
				if path == "/oa-api/session/migrate" {
					// A legacy tab must never overwrite a valid cookie session.
					if existing, err := r.Cookie(oaSessionCookie); err == nil && oaSessionToken.MatchString(existing.Value) && existing.Value != token {
						if _, status := identity.resolve(r.Context(), existing.Value); status == http.StatusOK {
							expireLegacyOACookies(w)
							JSON(w, http.StatusOK, map[string]string{"status": "ok"})
							return
						} else if status != http.StatusUnauthorized && status != http.StatusForbidden {
							Error(w, status, "账号服务暂时不可用")
							return
						}
					}
				}
				if _, status := identity.resolve(r.Context(), token); status != http.StatusOK {
					if status == http.StatusUnauthorized || status == http.StatusForbidden {
						Error(w, status, "登录已失效或账号未启用")
					} else {
						Error(w, status, "账号服务暂时不可用")
					}
					return
				}
				if path == "/oa-api/session/migrate" {
					setBrowserSession(w, token, 0)
				}
				JSON(w, http.StatusOK, map[string]string{"status": "ok"})
			case "/v1/auth/logout":
				setBrowserSession(w, "", -1)
				JSON(w, http.StatusOK, map[string]string{"status": "ok"})
			}
		})
	}
	routes.Handle("/", next)
	return routes
}

func browserSessionBearer(r *http.Request) string {
	fields := strings.Fields(r.Header.Get("Authorization"))
	if len(fields) == 2 && strings.EqualFold(fields[0], "Bearer") {
		return fields[1]
	}
	return ""
}

func setBrowserSession(w http.ResponseWriter, token string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name: oaSessionCookie, Value: token, Path: "/", Secure: true, HttpOnly: true,
		SameSite: http.SameSiteLaxMode, MaxAge: maxAge,
	})
	expireLegacyOACookies(w)
}

// P1 used separate, path-scoped attachment and WS credentials. Remove
// OA-local variants without touching any cookie written to the main domain.
func expireLegacyOACookies(w http.ResponseWriter) {
	for _, legacy := range []struct{ name, path string }{
		{"wf_asset_token", "/v1/assets/files"},
		{"wf_asset_stream_token", "/v1/assets"},
		{"wf_ws_token", "/ws"},
	} {
		for _, domain := range []string{"", oaBrowserHost} {
			http.SetCookie(w, &http.Cookie{
				Name: legacy.name, Path: legacy.path, Domain: domain,
				Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: -1,
			})
		}
	}
}

func (i Identity) issueBrowserSession(w http.ResponseWriter, r *http.Request) {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, i.Base+r.URL.Path, http.MaxBytesReader(w, r.Body, 16<<10))
	if err != nil {
		Error(w, http.StatusBadGateway, "账号服务暂时不可用")
		return
	}
	req.Host = r.Host
	req.Header.Set("Content-Type", "application/json")
	for _, header := range []string{"X-Real-IP", "X-Forwarded-For", "X-Forwarded-Proto"} {
		req.Header.Set(header, r.Header.Get(header))
	}
	req.Header.Set(TraceHeader, TraceID(r.Context()))
	resp, err := i.Client.Do(req)
	if err != nil {
		Error(w, http.StatusBadGateway, "账号服务暂时不可用")
		return
	}
	defer resp.Body.Close()
	const maxResponse = 1 << 20
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		// Preserve validation and permission envelopes from the existing core.
		if resp.StatusCode >= 400 && resp.StatusCode < 500 && strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(resp.StatusCode)
			_, _ = io.Copy(w, io.LimitReader(resp.Body, maxResponse))
			return
		}
		Error(w, http.StatusBadGateway, "账号服务暂时不可用")
		return
	}
	var result struct {
		Data struct {
			User    json.RawMessage `json:"user"`
			Session struct {
				Token string `json:"token"`
			} `json:"session"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponse)).Decode(&result); err != nil ||
		!oaSessionToken.MatchString(result.Data.Session.Token) || len(result.Data.User) == 0 || string(result.Data.User) == "null" {
		Error(w, http.StatusBadGateway, "账号服务返回的登录信息不完整")
		return
	}
	setBrowserSession(w, result.Data.Session.Token, 0)
	JSON(w, resp.StatusCode, map[string]json.RawMessage{"user": result.Data.User})
}
