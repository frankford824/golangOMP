package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

func (s *taskAssetCenterService) enqueueMediaPreview(ctx context.Context, taskID, assetID int64, version *domain.DesignAssetVersion, actorID int64, refresh ...bool) (*domain.AssetMediaJob, string, error) {
	if s.mediaJobs == nil {
		return nil, "", fmt.Errorf("durable preview queue is not configured")
	}
	uploadedBy := version.UploadedBy
	if uploadedBy <= 0 && assetID > 0 {
		if asset, err := s.designAssetRepo.GetByID(ctx, assetID); err == nil && asset != nil {
			uploadedBy = asset.CreatedBy
		}
	}
	input := domain.AssetMediaJobInput{Filename: version.OriginalFilename, TaskID: taskID, AssetID: assetID, TaskAssetID: version.ID, ActorID: uploadedBy}
	payload, err := json.Marshal(input)
	if err != nil {
		return nil, "", err
	}
	job := &domain.AssetMediaJob{Kind: "system_renditions", Pool: "ecs", ResourceID: fmt.Sprintf("task-asset:%d", version.ID),
		SourceVersion: domain.TaskAssetSourceVersion(version.ID), Recipe: domain.AssetMediaRecipe, Priority: 0, Payload: payload}
	if actorID == 0 {
		job.Priority = 20
	} else if domain.AssetMediaOptionsFromContext(ctx).Rendition == "thumbnail" {
		job.Priority = 15
	}
	if version.FileSize != nil {
		job.TotalBytes = *version.FileSize
	}
	job.RefreshMissing = len(refresh) > 0 && refresh[0]
	return s.mediaJobs.Enqueue(ctx, job, actorID)
}

func (s *taskAssetCenterService) pendingMediaPreview(ctx context.Context, taskID, assetID int64, version *domain.DesignAssetVersion, thumbnail bool) (*domain.AssetDownloadInfo, *domain.AppError) {
	rendition := "preview"
	if thumbnail {
		rendition = "thumbnail"
	}
	info := &domain.AssetDownloadInfo{DownloadMode: domain.AssetDownloadModeDirect, AccessHint: "preview_prepare_required",
		Filename: version.OriginalFilename, MimeType: "image/webp", SourceVersion: domain.TaskAssetSourceVersion(version.ID), Rendition: rendition, State: "queued", RetryAfter: 5}
	info.ContentID = domain.AssetMediaIdentity(info.SourceVersion, rendition, domain.AssetMediaRecipe)
	if expected := domain.AssetMediaOptionsFromContext(ctx).ExpectedVersion; expected != "" && expected != info.SourceVersion {
		return nil, domain.NewAppError("source_version_changed", "文件已更新，请刷新后重新选择", nil)
	}
	if domain.AssetMediaOptionsFromContext(ctx).AuthorizeOnly {
		return info, nil
	}
	if !isDerivedPreviewGenerationCandidate(version) {
		info.State = "unsupported"
		info.AccessHint = "preview_unsupported"
		info.RetryAfter = 0
		return info, nil
	}
	actor, _ := domain.RequestActorFromContext(ctx)
	options := domain.AssetMediaOptionsFromContext(ctx)
	options.Rendition = rendition
	ctx = domain.WithAssetMediaOptions(ctx, options)
	if options.AccessReference == nil {
		options.AccessReference = &domain.AssetMediaAccessReference{ResourceKind: "task_asset", ResourceID: fmt.Sprint(version.ID), Purpose: "preview", Rendition: rendition, ExpectedSourceVersion: info.SourceVersion}
		ctx = domain.WithAssetMediaOptions(ctx, options)
	}
	j, requestID, err := s.enqueueMediaPreview(ctx, taskID, assetID, version, actor.ID, true)
	if err != nil {
		return nil, infraError("queue bounded asset preview", err)
	}
	info.State = j.PublicState()
	info.JobID = j.ID
	info.RequestID = requestID
	info.ErrorCode = j.ErrorCode
	info.Retryable = j.Retryable
	if info.State == "failed" {
		info.AccessHint = "preview_failed"
		info.RetryAfter = 0
	}
	return info, nil
}

// ProcessMediaPreview resolves the frozen task_assets row, never a subsequently
// changed design_assets.current_version_id. The queue lease is owned by the caller.
func (s *taskAssetCenterService) ProcessMediaPreview(ctx context.Context, job *domain.AssetMediaJob) error {
	if job == nil || job.Kind != "system_renditions" {
		return fmt.Errorf("unsupported system media job")
	}
	var input domain.AssetMediaJobInput
	if err := json.Unmarshal(job.Payload, &input); err != nil {
		return err
	}
	if job.SourceVersion != domain.TaskAssetSourceVersion(input.TaskAssetID) {
		return fmt.Errorf("source_version_changed")
	}
	return s.ensureDerivedPreviewVersion(ctx, input.TaskID, input.AssetID, input.TaskAssetID, input.ActorID)
}

type SystemMediaProcessor interface {
	ProcessMediaPreview(context.Context, *domain.AssetMediaJob) error
}

type publicationPreviewContextKey struct{}

// GetAuthorizedPublicationPreview is only called after the workbench has
// resolved an enabled publication and selected a file from its pinned revision.
// This narrow internal capability does not accept HTTP input directly.
func (s *taskAssetCenterService) GetAuthorizedPublicationPreview(ctx context.Context, publication domain.ResourceGroupPublicationSnapshot, itemID int64, thumbnail bool) (*domain.AssetDownloadInfo, *domain.AppError) {
	actor, ok := domain.RequestActorFromContext(ctx)
	if !ok || actor.ID <= 0 || !domain.ActorHasPermission(actor, domain.PermissionAssetView) {
		return nil, domain.ErrUnauthorized
	}
	for _, file := range publication.Files {
		if file.TaskAssetID == itemID && publication.GroupID > 0 && publication.FinalizedRevisionID > 0 {
			ctx = context.WithValue(ctx, publicationPreviewContextKey{}, itemID)
			return s.getTaskAssetPreviewInfoByID(ctx, itemID, thumbnail)
		}
	}
	return nil, domain.ErrNotFound
}

type mediaLeaseGuardContextKey struct{}

func unpreparedVersionMediaInfo(version *domain.DesignAssetVersion, rendition string) *domain.AssetDownloadInfo {
	if version.IsPreviewFile || version.IsDesignThumb {
		return &domain.AssetDownloadInfo{DownloadMode: domain.AssetDownloadModeDirect, AccessHint: "preview_failed", State: "failed", ErrorCode: "preview_output_exceeds_limit", Rendition: rendition, Filename: version.OriginalFilename}
	}
	return &domain.AssetDownloadInfo{DownloadMode: domain.AssetDownloadModeDirect, AccessHint: "preview_prepare_required",
		Filename: version.OriginalFilename, MimeType: "image/webp", State: "queued", Rendition: rendition, RetryAfter: 5}
}

func decorateVersionMediaInfo(info *domain.AssetDownloadInfo, version *domain.DesignAssetVersion, rendition string) {
	if info == nil || version == nil {
		return
	}
	id := version.ID
	if rendition != "original" && version.SourceAssetVersionID != nil {
		id = *version.SourceAssetVersionID
	}
	info.SourceVersion = domain.TaskAssetSourceVersion(id)
	info.Rendition = rendition
	info.ObjectKey = version.StorageKey
	info.ContentID = domain.AssetMediaIdentity(info.SourceVersion, rendition, domain.AssetMediaRecipe)
	if info.DownloadURL != nil && *info.DownloadURL != "" {
		info.State = "ready"
		info.CloudDelivery = &domain.AssetMediaDelivery{State: "ready", URL: *info.DownloadURL, ExpiresAt: info.ExpiresAt}
	}
	if info.State == "" {
		info.State = "unsupported"
	}
}

// RunSystemMediaWorker is invoked only by the dedicated worker binary; MAIN
// merely enqueues work and never competes for renderer memory.
func RunSystemMediaWorker(ctx context.Context, jobs repo.AssetMediaJobRepo, processor SystemMediaProcessor, workerID string, pools ...string) error {
	pool := "ecs"
	if len(pools) > 0 {
		pool = pools[0]
	}
	for ctx.Err() == nil {
		batch, err := jobs.Claim(ctx, pool, workerID, 1)
		if err != nil {
			log.Printf("media_claim_failed error=%v", err)
		}
		for _, job := range batch {
			timeout := 10 * time.Minute
			if pool == "scan" {
				timeout = 6 * time.Hour
			}
			jobCtx, cancel := context.WithTimeout(ctx, timeout)
			jobCtx = context.WithValue(jobCtx, mediaLeaseGuardContextKey{}, func(c context.Context, tx repo.Tx) error {
				return jobs.AssertLease(c, tx, job.ID, workerID, job.LeaseEpoch)
			})
			stopped := make(chan struct{})
			heartbeatDone := make(chan struct{})
			go func(j *domain.AssetMediaJob) {
				defer close(heartbeatDone)
				ticker := time.NewTicker(30 * time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-stopped:
						return
					case <-jobCtx.Done():
						return
					case <-ticker.C:
						phase := "rendering"
						if pool == "scan" {
							phase = "indexing"
						}
						if j.Kind == "external_copy_verify" {
							phase = "verifying"
						}
						if e := jobs.Heartbeat(jobCtx, j.ID, workerID, j.LeaseEpoch, phase, 0, j.TotalBytes); e != nil {
							cancel()
							return
						}
					}
				}
			}(job)
			err = processor.ProcessMediaPreview(jobCtx, job)
			close(stopped)
			<-heartbeatDone
			cancel()
			state, code, retryable := "succeeded", "", false
			if err != nil {
				state = "failed"
				code = "preview_render_failed"
				retryable = true
				msg := strings.ToLower(err.Error())
				if strings.Contains(msg, "nosuchkey") || strings.Contains(msg, "not found") {
					code = "source_missing"
					retryable = false
				}
				if strings.Contains(msg, "source_version_changed") {
					state = "stale"
					code = "source_version_changed"
					retryable = false
				}
				if MediaRenderFailureIsPermanent(err) {
					code = "preview_unsupported_or_invalid"
					retryable = false
				}
			}
			finishCtx, finishCancel := context.WithTimeout(context.Background(), 10*time.Second)
			result := job.Result
			if len(result) == 0 {
				result = json.RawMessage(`{}`)
			}
			finishErr := jobs.Finish(finishCtx, job.ID, workerID, job.LeaseEpoch, state, result, code, retryable)
			finishCancel()
			log.Printf("media_preview_finished job_id=%s state=%s error_code=%s committed=%t", job.ID, state, code, finishErr == nil)
		}
		if existing, ok := processor.(interface {
			ProcessExistingPreviews(context.Context) (int, error)
		}); ok && ctx.Err() == nil {
			if _, e := existing.ProcessExistingPreviews(ctx); e != nil {
				log.Printf("workbench_media_queue_failed error=%v", e)
			}
		}
		timer := time.NewTimer(2 * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return ctx.Err()
}
