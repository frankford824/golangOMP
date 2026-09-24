package assetmedia

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"
)

type mediaMetrics struct {
	Requests, AuthRejected, VersionRejected, OriginRequests, OriginBytes, CacheHitBytes, CoalescedBytes, NASOriginalBytes, ArtifactBytes, TransferFailures atomic.Uint64
}

func (m *mediaMetrics) snapshot() map[string]uint64 {
	return map[string]uint64{"requests": m.Requests.Load(), "authorization_rejected": m.AuthRejected.Load(), "version_rejected": m.VersionRejected.Load(), "origin_requests": m.OriginRequests.Load(), "origin_received_bytes": m.OriginBytes.Load(), "cache_hit_sent_bytes": m.CacheHitBytes.Load(), "coalesced_sent_bytes": m.CoalescedBytes.Load(), "nas_original_sent_bytes": m.NASOriginalBytes.Load(), "artifact_sent_bytes": m.ArtifactBytes.Load(), "transfer_failures": m.TransferFailures.Load()}
}

type countedWriter struct {
	io.Writer
	count *atomic.Uint64
}

func (w countedWriter) Write(p []byte) (int, error) {
	n, e := w.Writer.Write(p)
	w.count.Add(uint64(n))
	return n, e
}
func (w countedWriter) ReadFrom(r io.Reader) (int64, error) {
	if fast, ok := w.Writer.(io.ReaderFrom); ok {
		n, e := fast.ReadFrom(r)
		w.count.Add(uint64(n))
		return n, e
	}
	return io.Copy(struct{ io.Writer }{w}, r)
}

// Product metrics only: no user identities, paths, signed URLs or credentials.
// The snapshot may lag by one minute after an unclean crash and is not billing.
func (g *Gateway) RunMetricsPersistence(ctx context.Context) {
	file := filepath.Join(g.cfg.CacheDir, ".asset-media-metrics.json")
	if raw, err := os.ReadFile(file); err == nil {
		var values map[string]uint64
		if json.Unmarshal(raw, &values) == nil {
			for key, counter := range map[string]*atomic.Uint64{"requests": &g.cache.metrics.Requests, "authorization_rejected": &g.cache.metrics.AuthRejected, "version_rejected": &g.cache.metrics.VersionRejected, "origin_requests": &g.cache.metrics.OriginRequests, "origin_received_bytes": &g.cache.metrics.OriginBytes, "cache_hit_sent_bytes": &g.cache.metrics.CacheHitBytes, "coalesced_sent_bytes": &g.cache.metrics.CoalescedBytes, "nas_original_sent_bytes": &g.cache.metrics.NASOriginalBytes, "artifact_sent_bytes": &g.cache.metrics.ArtifactBytes, "transfer_failures": &g.cache.metrics.TransferFailures} {
				counter.Add(values[key])
			}
		}
	}
	persist := func() {
		raw, _ := json.Marshal(g.cache.metrics.snapshot())
		tmp := file + ".tmp"
		if os.WriteFile(tmp, raw, 0600) == nil {
			_ = os.Rename(tmp, file)
		}
	}
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			persist()
			return
		case <-ticker.C:
			persist()
		}
	}
}
