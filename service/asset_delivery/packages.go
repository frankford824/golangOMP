package assetdelivery

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"workflow/domain"
	"workflow/internal/assetmedia"
)

type PackageRequest struct {
	Items []domain.AssetMediaSelection `json:"items"`
}

func (s *Service) CreatePackage(ctx context.Context, request PackageRequest) (*domain.AssetDownloadInfo, *domain.AppError) {
	actor, ok := domain.RequestActorFromContext(ctx)
	if !ok || actor.ID <= 0 || !domain.ActorHasPermission(actor, domain.PermissionAssetDownload) {
		return nil, domain.ErrUnauthorized
	}
	if s.Jobs == nil || !s.Config.NASWorkerEnabled || !s.Config.ExternalZIPEnabled {
		return nil, domain.NewAppError("media_temporarily_unavailable", "NAS打包服务尚未启用", nil)
	}
	if len(request.Items) == 0 || len(request.Items) > 500 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "请选择1至500个外部文件", nil)
	}
	selection := []domain.AssetMediaSelection{}
	seen := map[string]bool{}
	failures := []map[string]string{}
	var total int64
	for _, item := range request.Items {
		id, ok := domain.ParseExternalAssetResourceID(item.ResourceID)
		if !ok {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "仅支持外部资源选择集", nil)
		}
		if seen[item.ResourceID] {
			continue
		}
		seen[item.ResourceID] = true
		row, appErr := s.External.GetMediaRecord(ctx, id)
		if appErr != nil {
			failures = append(failures, map[string]string{"resource_id": item.ResourceID, "error_code": appErr.Code})
			continue
		}
		if row.Media == nil || row.Media.SourceVersion == "" || (item.SourceVersion != "" && row.Media.SourceVersion != item.SourceVersion) {
			failures = append(failures, map[string]string{"resource_id": item.ResourceID, "error_code": "source_version_changed"})
			continue
		}
		total += row.FileSize
		selection = append(selection, domain.AssetMediaSelection{ResourceID: row.ResourceID, SourceVersion: row.Media.SourceVersion, Filename: strings.TrimPrefix(row.OriginPath, "/p3/"), Size: row.FileSize})
	}
	if len(failures) > 0 {
		return nil, domain.NewAppError(domain.ErrCodeConflict, "部分文件不可用，请确认后重新选择可用项", map[string]any{"failures": failures, "available_items": selection})
	}
	if total > 512<<20 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "打包原文件总量不能超过512MiB，请缩小选择或逐个下载", nil)
	}
	raw, _ := json.Marshal(selection)
	version := domain.AssetMediaIdentity(string(raw))
	payload, _ := json.Marshal(domain.AssetMediaJobInput{Filename: "外部资源选择集 ZIP", ActorID: actor.ID, Selection: selection})
	// Package ownership is rechecked against every frozen external member.
	ctx = domain.WithAssetMediaOptions(ctx, domain.AssetMediaOptions{AccessReference: &domain.AssetMediaAccessReference{ResourceKind: "package", ResourceID: "selection:" + version, Purpose: "download", Rendition: "original", ExpectedSourceVersion: version}})
	job, requestID, err := s.Jobs.Enqueue(ctx, &domain.AssetMediaJob{Kind: "external_selection_zip", Pool: "nas", ResourceID: "selection:" + version, SourceVersion: version, Recipe: domain.AssetMediaRecipe, Priority: 10, Payload: payload, TotalBytes: total}, actor.ID)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "创建打包任务失败", nil)
	}
	info := packagePendingInfo(job)
	info.RequestID = requestID
	return info, nil
}

func packageContentID(job *domain.AssetMediaJob) string {
	return domain.AssetMediaIdentity("external-selection", job.ID, job.SourceVersion, "original", job.Recipe)
}
func packageFilename(job *domain.AssetMediaJob) string {
	suffix := job.ID
	if len(suffix) > 8 {
		suffix = suffix[:8]
	}
	return "外部资源选集-" + suffix + ".zip"
}
func packagePendingInfo(job *domain.AssetMediaJob) *domain.AssetDownloadInfo {
	return &domain.AssetDownloadInfo{DownloadMode: domain.AssetDownloadModeDirect, Filename: packageFilename(job), MimeType: "application/zip", SourceVersion: job.SourceVersion, ContentID: packageContentID(job), Rendition: "original", State: job.PublicState(), JobID: job.ID, RetryAfter: 5, ErrorCode: job.ErrorCode, Retryable: job.Retryable, AccessHint: "external_zip_prepare_required"}
}

func (s *Service) packageInputs(ctx context.Context, selection []domain.AssetMediaSelection) ([]assetmedia.ReadTarget, error) {
	if len(selection) == 0 || len(selection) > 500 {
		return nil, fmt.Errorf("invalid package selection")
	}
	inputs := []assetmedia.ReadTarget{}
	var total int64
	for _, item := range selection {
		id, ok := domain.ParseExternalAssetResourceID(item.ResourceID)
		if !ok {
			return nil, fmt.Errorf("invalid external selection")
		}
		row, appErr := s.External.GetMediaRecord(ctx, id)
		if appErr != nil {
			return nil, appErr
		}
		if row.Media == nil || row.Media.SourceVersion != item.SourceVersion || row.FileSize != item.Size {
			return nil, fmt.Errorf("source_version_changed")
		}
		fp := row.Media
		total += row.FileSize
		inputs = append(inputs, assetmedia.ReadTarget{Source: "nas", SourceVersion: item.SourceVersion, ContentID: domain.AssetMediaIdentity(row.ResourceID, item.SourceVersion, "original", domain.AssetMediaRecipe), RelativePath: strings.TrimPrefix(row.OriginPath, "/p3/"), Filename: item.Filename, MimeType: row.MimeType, Size: row.FileSize, SHA256: fp.SHA256, ModifiedNS: fp.ModifiedNS, ChangedNS: fp.ChangedNS, FileIdentity: fp.FileIdentity, RootIdentity: fp.RootIdentity})
	}
	if total > 512<<20 {
		return nil, fmt.Errorf("package size exceeded")
	}
	return inputs, nil
}

func (s *Service) claimZIP(ctx context.Context, j *domain.AssetMediaJob, input domain.AssetMediaJobInput) (WorkerClaim, error) {
	inputs, err := s.packageInputs(ctx, input.Selection)
	if err != nil {
		return WorkerClaim{}, err
	}
	parent := j
	if j.Kind == "external_zip_upload" {
		parent, err = s.Jobs.Get(ctx, input.ParentJobID)
		if err != nil || parent == nil {
			return WorkerClaim{}, fmt.Errorf("package build not found")
		}
	}
	source := assetmedia.ReadTarget{Source: "artifact", SourceVersion: parent.SourceVersion, ContentID: packageContentID(parent), Filename: packageFilename(parent), MimeType: "application/zip"}
	var result domain.ExternalMediaResult
	_ = json.Unmarshal(parent.Result, &result)
	if result.Package != nil {
		source.Size = result.Package.Size
		source.SHA256 = result.Package.SHA256
		source.CRC64 = result.Package.CRC64
	}
	var checkpoint WorkerCheckpoint
	_ = json.Unmarshal(j.Checkpoint, &checkpoint)
	return WorkerClaim{Job: j, Source: source, Inputs: inputs, Checkpoint: checkpoint}, nil
}

func (s *Service) completeZIP(ctx context.Context, j *domain.AssetMediaJob, cp WorkerCheckpoint, result domain.ExternalMediaResult) error {
	var input domain.AssetMediaJobInput
	if err := json.Unmarshal(j.Payload, &input); err != nil {
		return err
	}
	if _, err := s.packageInputs(ctx, input.Selection); err != nil {
		return err
	}
	object := result.Package
	if object == nil || object.Size < 0 || object.Size > 520<<20 || !validSHA256(object.SHA256) || object.SourceVersion != j.SourceVersion {
		return fmt.Errorf("invalid package result")
	}
	if j.Kind == "external_zip_upload" {
		u, ok := cp.Uploads["zip"]
		if !ok || !u.Complete || object.Key != u.ObjectKey || object.Size != u.Size || object.SHA256 != u.SHA256 || object.CRC64 != u.CRC64 {
			return fmt.Errorf("package upload not verified")
		}
	} else if object.Key != "" {
		return fmt.Errorf("local package cannot claim an OSS object")
	}
	raw, _ := json.Marshal(result)
	return s.Jobs.Finish(ctx, j.ID, j.LeaseOwner, j.LeaseEpoch, "succeeded", raw, "", false)
}

func (s *Service) resolvePackage(ctx context.Context, r Request, actor domain.RequestActor) (*domain.AssetDownloadInfo, *assetmedia.ReadTarget, *domain.AppError) {
	parent, appErr := s.JobForActor(ctx, r.ResourceID)
	if appErr != nil {
		return nil, nil, appErr
	}
	if parent.Kind != "external_selection_zip" || (parent.State == "succeeded" && time.Since(parent.UpdatedAt) > 24*time.Hour) {
		return nil, nil, domain.NewAppError("source_missing", "打包文件已过期，请重新创建", nil)
	}
	info := packagePendingInfo(parent)
	if parent.State != "succeeded" {
		return info, nil, nil
	}
	var result domain.ExternalMediaResult
	if json.Unmarshal(parent.Result, &result) != nil || result.Package == nil {
		return nil, nil, domain.ErrNotFound
	}
	object := result.Package
	info.FileSize = object.Size
	info.State = "ready"
	info.RetryAfter = 0
	if r.Delivery == "lan" {
		return info, &assetmedia.ReadTarget{Source: "artifact", ContentID: info.ContentID, SourceVersion: info.SourceVersion, Size: object.Size, SHA256: object.SHA256, CRC64: object.CRC64, Filename: info.Filename, MimeType: info.MimeType}, nil
	}
	var input domain.AssetMediaJobInput
	_ = json.Unmarshal(parent.Payload, &input)
	input.ParentJobID = parent.ID
	input.ActorID = actor.ID
	payload, _ := json.Marshal(input)
	child, requestID, err := s.Jobs.Enqueue(ctx, &domain.AssetMediaJob{Kind: "external_zip_upload", Pool: "nas", ResourceID: "zip:" + parent.ID, SourceVersion: parent.SourceVersion, Recipe: domain.AssetMediaRecipe, Priority: 10, Payload: payload, TotalBytes: object.Size}, actor.ID)
	if err != nil {
		return nil, nil, domain.NewAppError(domain.ErrCodeInternalError, "准备云端打包副本失败", nil)
	}
	info.JobID = child.ID
	info.RequestID = requestID
	info.State = child.PublicState()
	info.RetryAfter = 5
	if child.State == "succeeded" {
		var uploaded domain.ExternalMediaResult
		if json.Unmarshal(child.Result, &uploaded) != nil || uploaded.Package == nil {
			return nil, nil, domain.ErrNotFound
		}
		signed := s.OSS.PresignDownloadURLWithFilename(uploaded.Package.Key, info.Filename)
		if signed == nil {
			return nil, nil, domain.ErrNotFound
		}
		info.DownloadURL = &signed.DownloadURL
		info.ExpiresAt = &signed.ExpiresAt
		info.RetryAfter = 0
		info.CloudDelivery = &domain.AssetMediaDelivery{State: "ready", URL: signed.DownloadURL, ExpiresAt: &signed.ExpiresAt}
	}
	return info, nil, nil
}
