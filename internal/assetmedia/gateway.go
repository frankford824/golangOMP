package assetmedia

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type GatewayConfig struct {
	MetricsToken    string
	ID              string
	Root            string
	CacheDir        string
	ArtifactDir     string
	Keys            map[string]ed25519.PublicKey
	AllowedOrigins  []string
	AllowedOSSHosts []string
	QuotaBytes      int64
	TemporaryBytes  int64
	FreeFloorBytes  uint64
	DayMbps         int64
	NightMbps       int64
	Authorize       func(context.Context, string) (ReadTarget, error)
}

type Gateway struct {
	cfg   GatewayConfig
	cache *MediaCache
}

type AuthorizationError struct{ Status int }

func (e *AuthorizationError) Error() string { return "delivery authorization rejected" }

func NewGateway(cfg GatewayConfig) (*Gateway, error) {
	if cfg.ID == "" || cfg.Authorize == nil || len(cfg.Keys) == 0 {
		return nil, fmt.Errorf("gateway identity, verifier and authorizer are required")
	}
	cache, err := NewMediaCache(cfg)
	if err != nil {
		return nil, err
	}
	return &Gateway{cfg: cfg, cache: cache}, nil
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	origin := r.Header.Get("Origin")
	if origin != "" {
		allowed := false
		for _, o := range g.cfg.AllowedOrigins {
			if origin == o {
				allowed = true
				break
			}
		}
		if !allowed {
			http.Error(w, "origin denied", http.StatusForbidden)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
		w.Header().Set("Access-Control-Expose-Headers", "Content-Length,Content-Range,Content-Disposition,ETag,X-Media-Cache,X-Media-Gateway")
	}
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "GET,HEAD,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Range,If-Range,If-None-Match")
		w.Header().Set("Access-Control-Allow-Private-Network", "true")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.URL.Path == "/edge/v1/ping" {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"gateway_id": g.cfg.ID, "status": "ready"})
		return
	}
	if r.URL.Path == "/edge/v1/metrics" {
		if g.cfg.MetricsToken == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Asset-Media-Gateway-Token")), []byte(g.cfg.MetricsToken)) != 1 {
			http.Error(w, "unauthorized", 401)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(g.cache.metrics.snapshot())
		return
	}
	const prefix = "/edge/v1/content/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		http.NotFound(w, r)
		return
	}
	token := r.URL.Query().Get("ticket")
	g.cache.metrics.Requests.Add(1)
	claim, err := VerifyTicket(token, g.cfg.ID, g.cfg.Keys, time.Now())
	if err != nil {
		g.cache.metrics.AuthRejected.Add(1)
		http.Error(w, "invalid or expired ticket", http.StatusUnauthorized)
		return
	}
	if strings.TrimPrefix(r.URL.Path, prefix) != claim.ContentID {
		http.Error(w, "content identity mismatch", http.StatusForbidden)
		return
	}
	authCtx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	target, err := g.cfg.Authorize(authCtx, token)
	cancel()
	if err != nil {
		g.cache.metrics.AuthRejected.Add(1)
		status := http.StatusServiceUnavailable
		var rejected *AuthorizationError
		if errors.As(err, &rejected) {
			switch rejected.Status {
			case 401, 403, 404, 409, 410:
				status = rejected.Status
			}
		}
		http.Error(w, "delivery authorization unavailable", status)
		return
	}
	if target.ContentID != claim.ContentID || target.SourceVersion != claim.SourceVersion || target.Size < 0 {
		g.cache.metrics.VersionRejected.Add(1)
		http.Error(w, "source version changed", http.StatusConflict)
		return
	}
	if target.Source != "nas" && target.Source != "oss" && target.Source != "artifact" {
		http.Error(w, "invalid source", http.StatusForbidden)
		return
	}
	start, end, err := parseSingleRange(r.Header.Get("Range"), target.Size)
	if err != nil {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", target.Size))
		http.Error(w, "invalid or unsupported range", http.StatusRequestedRangeNotSatisfiable)
		return
	}
	fingerprint := claim.ContentID
	if cacheIDValid(target.SHA256) {
		fingerprint += "-" + target.SHA256
	} else if _, e := strconv.ParseUint(target.CRC64, 10, 64); e == nil {
		fingerprint += "-" + target.CRC64
	}
	etag := `"` + fingerprint + `"`
	partial := r.Header.Get("Range") != ""
	if ifRange := r.Header.Get("If-Range"); ifRange != "" && ifRange != etag {
		start = 0
		end = target.Size - 1
		partial = false
	}
	if target.Source == "nas" {
		f, e := OpenBeneath(g.cfg.Root, target.RelativePath)
		if e != nil {
			http.Error(w, "source unavailable", http.StatusServiceUnavailable)
			return
		}
		st, e := f.Stat()
		f.Close()
		if e != nil || CheckLocalVersion(st, target) != nil {
			http.Error(w, "source version changed", http.StatusConflict)
			return
		}
	}
	if target.Source == "oss" && !g.cache.originAllowed(target.OriginURL) {
		http.Error(w, "origin denied", http.StatusForbidden)
		return
	}
	var artifact io.ReadSeekCloser
	if target.Source == "artifact" {
		file, e := OpenArtifact(g.cfg.ArtifactDir, target)
		if e != nil {
			if target.OriginURL == "" || !g.cache.originAllowed(target.OriginURL) {
				http.Error(w, "local artifact unavailable; request cloud preparation", http.StatusServiceUnavailable)
				return
			}
			target.Source = "oss"
		} else {
			artifact = file
			defer file.Close()
		}
	}
	w.Header().Set("X-Media-Gateway", g.cfg.ID)
	w.Header().Set("ETag", etag)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Type", target.MimeType)
	if target.MimeType == "" {
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	if claim.Purpose == "download" {
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filepath.Base(strings.ReplaceAll(target.Filename, "\\", "/"))}))
	} else {
		remaining := claim.ExpiresAt - time.Now().Unix()
		if remaining < 0 {
			remaining = 0
		}
		w.Header().Set("Cache-Control", fmt.Sprintf("private, max-age=%d", remaining))
	}
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	length := target.Size
	if partial {
		length = end - start + 1
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, target.Size))
	}
	if r.Method == http.MethodHead {
		w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
		if partial {
			w.WriteHeader(http.StatusPartialContent)
		}
		return
	}
	if artifact != nil {
		w.Header().Set("X-Media-Cache", "artifact")
		w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
		if _, err = artifact.Seek(start, io.SeekStart); err != nil {
			http.Error(w, "artifact unavailable", http.StatusServiceUnavailable)
			return
		}
		if partial {
			w.WriteHeader(http.StatusPartialContent)
		}
		if _, err = io.CopyN(countedWriter{Writer: w, count: &g.cache.metrics.ArtifactBytes}, artifact, length); err != nil {
			g.cache.metrics.TransferFailures.Add(1)
			panic(http.ErrAbortHandler)
		}
		return
	}
	entry, hit, err := g.cache.Open(r.Context(), target)
	if err != nil {
		w.Header().Del("Content-Disposition")
		http.Error(w, "local preparation unavailable; request cloud delivery", http.StatusServiceUnavailable)
		return
	}
	defer g.cache.Release(entry)
	if err = entry.Wait(r.Context(), start); err != nil {
		w.Header().Del("Content-Disposition")
		http.Error(w, "source transfer failed", http.StatusBadGateway)
		return
	}
	w.Header().Set("X-Media-Cache", hit)
	w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
	if partial {
		w.WriteHeader(http.StatusPartialContent)
	}
	var output io.Writer = w
	if target.Source == "nas" {
		output = countedWriter{Writer: w, count: &g.cache.metrics.NASOriginalBytes}
	} else if hit == "hit" {
		output = countedWriter{Writer: w, count: &g.cache.metrics.CacheHitBytes}
	} else if hit == "coalesced" {
		output = countedWriter{Writer: w, count: &g.cache.metrics.CoalescedBytes}
	}
	if err = entry.CopyRange(r.Context(), output, start, length); err != nil {
		g.cache.metrics.TransferFailures.Add(1)
		panic(http.ErrAbortHandler)
	}
}

func parseSingleRange(raw string, size int64) (int64, int64, error) {
	if raw == "" {
		return 0, size - 1, nil
	}
	if !strings.HasPrefix(raw, "bytes=") || strings.Contains(raw, ",") || size <= 0 {
		return 0, 0, fmt.Errorf("invalid range")
	}
	left, right, ok := strings.Cut(strings.TrimPrefix(raw, "bytes="), "-")
	if !ok {
		return 0, 0, fmt.Errorf("invalid range")
	}
	if left == "" {
		n, e := strconv.ParseInt(right, 10, 64)
		if e != nil || n <= 0 {
			return 0, 0, fmt.Errorf("invalid suffix")
		}
		if n > size {
			n = size
		}
		return size - n, size - 1, nil
	}
	start, e := strconv.ParseInt(left, 10, 64)
	if e != nil || start < 0 || start >= size {
		return 0, 0, fmt.Errorf("invalid start")
	}
	end := size - 1
	if right != "" {
		end, e = strconv.ParseInt(right, 10, 64)
		if e != nil || end < start {
			return 0, 0, fmt.Errorf("invalid end")
		}
		if end >= size {
			end = size - 1
		}
	}
	return start, end, nil
}

func HTTPAuthorizer(baseURL, credential string, client *http.Client) func(context.Context, string) (ReadTarget, error) {
	return func(ctx context.Context, ticket string) (ReadTarget, error) {
		var target ReadTarget
		parsed, err := url.Parse(baseURL)
		if err != nil || parsed.Scheme != "https" || credential == "" {
			return target, fmt.Errorf("invalid authorization configuration")
		}
		raw, _ := json.Marshal(map[string]string{"ticket": ticket})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/v1/integration/asset-media/validate-ticket", bytes.NewReader(raw))
		if err != nil {
			return target, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Asset-Media-Gateway-Token", credential)
		resp, err := client.Do(req)
		if err != nil {
			return target, fmt.Errorf("authorization transport failed")
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return target, &AuthorizationError{Status: resp.StatusCode}
		}
		var envelope struct {
			Data ReadTarget `json:"data"`
		}
		err = json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&envelope)
		return envelope.Data, err
	}
}
