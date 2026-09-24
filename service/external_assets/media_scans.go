package externalassets

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	"workflow/domain"
	"workflow/repo"
)

func (s *Service) StartMediaScan(ctx context.Context, scan domain.ExternalMediaScan) error {
	if !s.mediaConfig.NASScanEnabled {
		return fmt.Errorf("NAS snapshot ingestion disabled")
	}
	if _, err := uuid.Parse(scan.ScanID); err != nil {
		return fmt.Errorf("invalid scan id")
	}
	if scan.AgentID == "" || len(scan.AgentID) > 128 || scan.AgentEpoch == "" || len(scan.AgentEpoch) > 64 || scan.RootIdentity == "" || len(scan.RootIdentity) > 128 || scan.OriginRoot != "/p3" || scan.StartSequence < 0 || scan.ShardCount != 2 || scan.ShardIndex < 0 || scan.ShardIndex >= scan.ShardCount {
		return fmt.Errorf("invalid NAS scan identity")
	}
	r, ok := s.repo.(repo.ExternalAssetMediaRepo)
	if !ok {
		return fmt.Errorf("NAS snapshot repository unavailable")
	}
	if err := r.StartMediaScan(ctx, scan); err != nil {
		return err
	}
	stored, _, err := r.GetMediaScan(ctx, scan.ScanID)
	if err != nil {
		return err
	}
	if stored.AgentID != scan.AgentID || stored.AgentEpoch != scan.AgentEpoch || stored.RootIdentity != scan.RootIdentity || stored.StartSequence != scan.StartSequence || stored.ShardIndex != scan.ShardIndex {
		return fmt.Errorf("scan replay changed identity")
	}
	return nil
}

func (s *Service) PutMediaScanPart(ctx context.Context, part domain.ExternalMediaScanPart) error {
	if !s.mediaConfig.NASScanEnabled {
		return fmt.Errorf("NAS snapshot ingestion disabled")
	}
	if part.Part < 0 || part.Part > 10000 || len(part.Items) < 1 || len(part.Items) > 500 {
		return fmt.Errorf("invalid scan part size")
	}
	r, ok := s.repo.(repo.ExternalAssetMediaRepo)
	if !ok {
		return fmt.Errorf("NAS snapshot repository unavailable")
	}
	scan, _, err := r.GetMediaScan(ctx, part.ScanID)
	if err != nil {
		return err
	}
	for _, e := range part.Items {
		if e.Type != domain.ExternalAssetFilesystemEventUpsert || e.MountPath != "/p3" || !strings.HasPrefix(e.OriginPath, "/p3/") || e.ScanID != part.ScanID || e.AgentEpoch != scan.AgentEpoch || e.RootIdentity != scan.RootIdentity || e.Sequence != scan.StartSequence || e.ChangedNS <= 0 || e.FileIdentity == "" {
			return fmt.Errorf("scan part contains inconsistent source identity")
		}
		if !validScanPath(e.OriginPath, scan.ShardIndex, scan.ShardCount) {
			return fmt.Errorf("scan path is noncanonical or belongs to another shard")
		}
	}
	return r.PutMediaScanPart(ctx, part)
}

func validScanPath(origin string, shard, count int) bool {
	if count != 2 || shard < 0 || shard >= count || !strings.HasPrefix(origin, "/p3/") || path.Clean(origin) != origin || strings.ContainsAny(origin, "\\\x00") {
		return false
	}
	rel := strings.TrimPrefix(origin, "/p3/")
	if rel == "" {
		return false
	}
	top, _, _ := strings.Cut(rel, "/")
	h := fnv.New32a()
	_, _ = h.Write([]byte(top))
	return int(h.Sum32()%uint32(count)) == shard
}

func (s *Service) FinishMediaScan(ctx context.Context, complete domain.ExternalMediaScan) (jobID string, err error) {
	if !s.mediaConfig.NASScanEnabled {
		return "", fmt.Errorf("NAS snapshot ingestion disabled")
	}
	if complete.ReadErrors != 0 || complete.Files < 1 || complete.Parts < 1 || complete.EndSequence < complete.StartSequence {
		return "", fmt.Errorf("incomplete or empty NAS scan cannot reconcile missing files")
	}
	r, ok := s.repo.(repo.ExternalAssetMediaRepo)
	if !ok {
		return "", fmt.Errorf("NAS snapshot repository unavailable")
	}
	scan, parts, err := r.GetMediaScan(ctx, complete.ScanID)
	if err != nil {
		return "", err
	}
	if scan.AgentID != complete.AgentID || scan.AgentEpoch != complete.AgentEpoch || scan.RootIdentity != complete.RootIdentity || scan.StartSequence != complete.StartSequence || scan.ShardIndex != complete.ShardIndex || scan.ShardCount != complete.ShardCount || scan.OriginRoot != complete.OriginRoot || len(parts) != complete.Parts {
		return "", fmt.Errorf("scan completion identity or part count mismatch")
	}
	h := sha256.New()
	var count int64
	seen := map[string]bool{}
	for i, p := range parts {
		if p.Part != i {
			return "", fmt.Errorf("missing scan part")
		}
		_, _ = h.Write([]byte(p.SHA256 + "\n"))
		for _, e := range p.Items {
			if seen[e.OriginPath] {
				return "", fmt.Errorf("duplicate scan path")
			}
			seen[e.OriginPath] = true
			count++
		}
	}
	if count != complete.Files || hex.EncodeToString(h.Sum(nil)) != complete.ManifestSHA256 {
		return "", fmt.Errorf("scan manifest verification failed")
	}
	if s.mediaJobs == nil {
		return "", fmt.Errorf("scan queue unavailable")
	}
	payload, _ := json.Marshal(complete)
	job, _, err := s.mediaJobs.Enqueue(ctx, &domain.AssetMediaJob{Kind: "external_scan_reconcile", Pool: "scan", ResourceID: "scan:" + complete.ScanID, SourceVersion: complete.ManifestSHA256, Recipe: "nas-scan-v2", Priority: 30, Payload: payload}, 0)
	if err != nil {
		return "", err
	}
	return job.ID, nil
}

func (s *Service) ProcessMediaPreview(ctx context.Context, job *domain.AssetMediaJob) error {
	if job.Kind != "external_scan_reconcile" {
		return fmt.Errorf("unsupported scan job")
	}
	var complete domain.ExternalMediaScan
	if err := json.Unmarshal(job.Payload, &complete); err != nil {
		return err
	}
	r, ok := s.repo.(repo.ExternalAssetMediaRepo)
	if !ok {
		return fmt.Errorf("scan repository unavailable")
	}
	scan, parts, err := r.GetMediaScan(ctx, complete.ScanID)
	if err != nil {
		return err
	}
	// Validate the whole manifest before any absence processing. Upserts are
	// restartable and retain newer journal events through sequence guards.
	h := sha256.New()
	var files int64
	for i, p := range parts {
		raw, e := json.Marshal(p.Items)
		if e != nil {
			return e
		}
		sum := sha256.Sum256(raw)
		if p.Part != i || hex.EncodeToString(sum[:]) != p.SHA256 {
			return fmt.Errorf("persisted scan part corrupted")
		}
		for _, item := range p.Items {
			if !validScanPath(item.OriginPath, scan.ShardIndex, scan.ShardCount) {
				return fmt.Errorf("persisted scan path out of scope")
			}
		}
		files += int64(len(p.Items))
		_, _ = h.Write([]byte(p.SHA256 + "\n"))
	}
	if len(parts) != complete.Parts || files != complete.Files || hex.EncodeToString(h.Sum(nil)) != complete.ManifestSHA256 {
		return fmt.Errorf("persisted scan manifest corrupted")
	}
	for _, p := range parts {
		for i := range p.Items {
			p.Items[i].ObservedAt = time.Now().UTC()
		}
		if _, appErr := s.ApplyFilesystemEvents(ctx, domain.ExternalAssetFilesystemEventBatch{AgentID: scan.AgentID, Events: p.Items}); appErr != nil {
			return appErr
		}
	}
	scan.Parts = complete.Parts
	scan.Files = complete.Files
	scan.EndSequence = complete.EndSequence
	scan.ManifestSHA256 = complete.ManifestSHA256
	return r.FinishMediaScan(ctx, scan)
}
