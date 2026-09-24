package assetdelivery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"workflow/domain"
	"workflow/internal/assetmedia"
	baseservice "workflow/service"
)

type UploadCheckpoint struct {
	ObjectKey string                        `json:"object_key"`
	UploadID  string                        `json:"upload_id"`
	PartSize  int64                         `json:"part_size"`
	Size      int64                         `json:"size"`
	MimeType  string                        `json:"mime_type"`
	SHA256    string                        `json:"sha256"`
	CRC64     string                        `json:"crc64"`
	Parts     []baseservice.OSSCompletePart `json:"parts,omitempty"`
	Complete  bool                          `json:"complete"`
}
type WorkerCheckpoint struct {
	Uploads map[string]UploadCheckpoint `json:"uploads"`
}

type WorkerClaim struct {
	Job        *domain.AssetMediaJob   `json:"job"`
	Source     assetmedia.ReadTarget   `json:"source"`
	Inputs     []assetmedia.ReadTarget `json:"inputs,omitempty"`
	Checkpoint WorkerCheckpoint        `json:"checkpoint"`
}

type WorkerRequest struct {
	JobID          string                     `json:"job_id"`
	WorkerID       string                     `json:"worker_id"`
	LeaseEpoch     int64                      `json:"lease_epoch"`
	Phase          string                     `json:"phase,omitempty"`
	ProcessedBytes int64                      `json:"processed_bytes,omitempty"`
	TotalBytes     int64                      `json:"total_bytes,omitempty"`
	ErrorCode      string                     `json:"error_code,omitempty"`
	Retryable      bool                       `json:"retryable,omitempty"`
	Result         domain.ExternalMediaResult `json:"result,omitempty"`
}

type UploadRequest struct {
	WorkerRequest
	Rendition string                        `json:"rendition"`
	Size      int64                         `json:"size"`
	SHA256    string                        `json:"sha256"`
	CRC64     string                        `json:"crc64"`
	Parts     []baseservice.OSSCompletePart `json:"parts,omitempty"`
	Complete  bool                          `json:"complete,omitempty"`
}

type UploadResponse struct {
	Pending    bool                             `json:"pending,omitempty"`
	Plan       *baseservice.OSSDirectUploadPlan `json:"plan,omitempty"`
	Checkpoint UploadCheckpoint                 `json:"checkpoint"`
}

func (s *Service) workerLease(ctx context.Context, r WorkerRequest) (*domain.AssetMediaJob, error) {
	if !s.Config.NASWorkerEnabled || s.Jobs == nil {
		return nil, fmt.Errorf("NAS worker disabled")
	}
	j, err := s.Jobs.Get(ctx, r.JobID)
	if err != nil {
		return nil, err
	}
	if j == nil || j.Pool != "nas" || j.State != "processing" || j.LeaseOwner != r.WorkerID || j.LeaseEpoch != r.LeaseEpoch || j.LeaseExpiresAt == nil || !j.LeaseExpiresAt.After(time.Now().UTC()) {
		return nil, fmt.Errorf("media_lease_lost")
	}
	return j, nil
}

func (s *Service) ClaimNAS(ctx context.Context, worker string, limit int, classes ...string) ([]WorkerClaim, error) {
	if !s.Config.NASWorkerEnabled || s.Jobs == nil {
		return nil, fmt.Errorf("NAS worker disabled")
	}
	var kinds []string
	if len(classes) > 0 && classes[0] != "" {
		switch classes[0] {
		case "render":
			kinds = []string{"external_renditions"}
			limit = 1
		case "transfer":
			kinds = []string{"external_original", "external_selection_zip", "external_zip_upload"}
		default:
			return nil, fmt.Errorf("invalid NAS execution class")
		}
	}
	jobs, err := s.Jobs.Claim(ctx, "nas", worker, limit, kinds...)
	if err != nil {
		return nil, err
	}
	claims := []WorkerClaim{}
	for _, j := range jobs {
		var input domain.AssetMediaJobInput
		if json.Unmarshal(j.Payload, &input) != nil {
			return nil, fmt.Errorf("invalid persisted media input")
		}
		if j.Kind == "external_selection_zip" || j.Kind == "external_zip_upload" {
			if !s.Config.ExternalZIPEnabled {
				_ = s.Jobs.Finish(ctx, j.ID, worker, j.LeaseEpoch, "failed", nil, "external_zip_disabled", false)
				continue
			}
			claim, e := s.claimZIP(ctx, j, input)
			if e != nil {
				_ = s.Jobs.Finish(ctx, j.ID, worker, j.LeaseEpoch, "failed", nil, "zip_source_unavailable", false)
				continue
			}
			claims = append(claims, claim)
			continue
		}
		row, appErr := s.External.GetMediaRecord(ctx, input.ExternalID)
		if appErr != nil {
			retry := appErr.Code == domain.ErrCodeInternalError || appErr.Code == "media_temporarily_unavailable"
			if e := s.Jobs.Finish(ctx, j.ID, worker, j.LeaseEpoch, "failed", nil, appErr.Code, retry); e == nil && !retry {
				s.abortTerminalUploads(ctx, j)
			}
			continue
		}
		if row == nil || row.Media == nil {
			_ = s.Jobs.Finish(ctx, j.ID, worker, j.LeaseEpoch, "failed", nil, "source_index_pending", true)
			continue
		}
		if row.Media.SourceVersion != j.SourceVersion {
			if e := s.Jobs.Finish(ctx, j.ID, worker, j.LeaseEpoch, "stale", nil, "source_version_changed", false); e == nil {
				s.abortTerminalUploads(ctx, j)
			}
			continue
		}
		fp := row.Media
		source := assetmedia.ReadTarget{Source: "nas", SourceVersion: j.SourceVersion, ContentID: domain.AssetMediaIdentity(row.ResourceID, j.SourceVersion, "original", domain.AssetMediaRecipe),
			RelativePath: strings.TrimPrefix(row.OriginPath, "/p3/"), Filename: row.FileName, MimeType: row.MimeType, Size: row.FileSize, SHA256: fp.SHA256,
			ModifiedNS: fp.ModifiedNS, ChangedNS: fp.ChangedNS, FileIdentity: fp.FileIdentity, RootIdentity: fp.RootIdentity}
		var checkpoint WorkerCheckpoint
		_ = json.Unmarshal(j.Checkpoint, &checkpoint)
		claims = append(claims, WorkerClaim{Job: j, Source: source, Checkpoint: checkpoint})
	}
	return claims, nil
}

func (s *Service) HeartbeatNAS(ctx context.Context, r WorkerRequest) error {
	if _, err := s.workerLease(ctx, r); err != nil {
		return err
	}
	return s.Jobs.Heartbeat(ctx, r.JobID, r.WorkerID, r.LeaseEpoch, r.Phase, r.ProcessedBytes, r.TotalBytes)
}

func (s *Service) UploadNAS(ctx context.Context, r UploadRequest) (*UploadResponse, error) {
	j, err := s.workerLease(ctx, r.WorkerRequest)
	if err != nil {
		return nil, err
	}
	if !validSHA256(r.SHA256) || r.Size < 0 {
		return nil, fmt.Errorf("invalid upload integrity metadata")
	}
	if r.Size == 0 && r.Rendition != "original" {
		return nil, fmt.Errorf("empty rendition is invalid")
	}
	if r.Size == 0 {
		empty := sha256.Sum256(nil)
		if r.SHA256 != hex.EncodeToString(empty[:]) || r.CRC64 != "0" {
			return nil, fmt.Errorf("empty source integrity mismatch")
		}
	}
	if _, err = strconv.ParseUint(r.CRC64, 10, 64); err != nil {
		return nil, fmt.Errorf("CRC64 is required")
	}
	var input domain.AssetMediaJobInput
	if err = json.Unmarshal(j.Payload, &input); err != nil {
		return nil, err
	}
	mimeType := "image/webp"
	filename := r.Rendition + ".webp"
	maxSize := int64(2 << 20)
	if r.Rendition == "thumbnail" {
		maxSize = 200 << 10
	}
	switch j.Kind {
	case "external_renditions":
		if r.Rendition != "thumbnail" && r.Rendition != "preview" {
			return nil, fmt.Errorf("rendition not allowed")
		}
	case "external_original":
		if r.Rendition != "original" {
			return nil, fmt.Errorf("rendition not allowed")
		}
		row, appErr := s.External.GetMediaRecord(ctx, input.ExternalID)
		if appErr != nil {
			return nil, appErr
		}
		if row.Media == nil || row.Media.SourceVersion != j.SourceVersion {
			return nil, fmt.Errorf("source_version_changed")
		}
		if r.Size != row.FileSize {
			return nil, fmt.Errorf("source length changed")
		}
		maxSize = row.FileSize
		mimeType = row.MimeType
		filename = "original" + strings.ToLower(path.Ext(row.FileName))
	case "external_zip_upload":
		if r.Rendition != "zip" {
			return nil, fmt.Errorf("rendition not allowed")
		}
		maxSize = 520 << 20
		mimeType = "application/zip"
		filename = "selection.zip"
	default:
		return nil, fmt.Errorf("unsupported NAS job")
	}
	if r.Size > maxSize {
		return nil, fmt.Errorf("rendition exceeds size budget")
	}
	var checkpoint WorkerCheckpoint
	_ = json.Unmarshal(j.Checkpoint, &checkpoint)
	if checkpoint.Uploads == nil {
		checkpoint.Uploads = map[string]UploadCheckpoint{}
	}
	upload, exists := checkpoint.Uploads[r.Rendition]
	if exists && (upload.Size != r.Size || upload.SHA256 != r.SHA256 || upload.CRC64 != r.CRC64) {
		return nil, fmt.Errorf("upload replay changed representation bytes")
	}
	if !exists {
		if j.Kind == "external_original" {
			row, appErr := s.External.GetMediaRecord(ctx, input.ExternalID)
			if appErr != nil {
				return nil, appErr
			}
			if row.Media != nil && row.Media.SHA256 != "" && row.Media.SHA256 != r.SHA256 {
				return nil, fmt.Errorf("source_version_changed")
			}
			if row.OSSOriginalKey != "" {
				stat, found, e := s.OSS.StatObject(ctx, row.OSSOriginalKey)
				if e != nil {
					return nil, fmt.Errorf("legacy object verification unavailable")
				}
				matched := found && stat.ContentLength == r.Size && stat.CRC64ECMA != "" && stat.CRC64ECMA == r.CRC64
				if found && stat.ContentLength == r.Size && stat.CRC64ECMA == "" {
					proof := domain.AssetMediaVerification{ObjectKey: row.OSSOriginalKey, ETag: stat.ETag, SHA256: r.SHA256, Size: r.Size}
					payload, _ := json.Marshal(domain.AssetMediaJobInput{ExternalID: input.ExternalID, Verification: &proof})
					verification, _, e := s.Jobs.Enqueue(ctx, &domain.AssetMediaJob{Kind: "external_copy_verify", Pool: "ecs", ResourceID: row.ResourceID, SourceVersion: domain.AssetMediaIdentity(j.SourceVersion, row.OSSOriginalKey, stat.ETag, r.SHA256), Recipe: domain.AssetMediaRecipe, Priority: 10, Payload: payload, TotalBytes: r.Size}, 0)
					if e != nil {
						return nil, e
					}
					if verification.State == "failed" || verification.State == "stale" {
						return nil, fmt.Errorf("legacy object verification failed")
					}
					if verification.State != "succeeded" {
						return &UploadResponse{Pending: true}, nil
					}
					var result struct {
						Matched bool `json:"matched"`
					}
					if json.Unmarshal(verification.Result, &result) != nil {
						return nil, fmt.Errorf("legacy verification result invalid")
					}
					matched = result.Matched
				}
				if matched {
					upload = UploadCheckpoint{ObjectKey: row.OSSOriginalKey, Size: r.Size, MimeType: mimeType, SHA256: r.SHA256, CRC64: r.CRC64, Complete: true}
					checkpoint.Uploads[r.Rendition] = upload
					raw, _ := json.Marshal(checkpoint)
					if e = s.Jobs.SaveCheckpoint(ctx, j.ID, r.WorkerID, r.LeaseEpoch, raw); e != nil {
						return nil, e
					}
					return &UploadResponse{Checkpoint: upload}, nil
				}
			}
		}
		key := fmt.Sprintf("external-assets/media-v2/%d/%s/%s/%s", input.ExternalID, j.SourceVersion, j.ID, filename)
		if j.Kind == "external_zip_upload" {
			key = "external-download-packages/" + j.ID + "/selection.zip"
		}
		if r.Size == 0 {
			empty := sha256.Sum256(nil)
			if r.SHA256 != hex.EncodeToString(empty[:]) || r.CRC64 != "0" {
				return nil, fmt.Errorf("empty source integrity mismatch")
			}
			// An empty original has no data body to relay. OSS multipart plans
			// require nonzero size, so create this exact zero-byte object here.
			if e := s.OSS.UploadObject(ctx, key, mimeType, nil); e != nil {
				return nil, fmt.Errorf("create empty original failed")
			}
			upload = UploadCheckpoint{ObjectKey: key, Size: 0, MimeType: mimeType, SHA256: r.SHA256, CRC64: r.CRC64, Complete: true}
			checkpoint.Uploads[r.Rendition] = upload
			raw, _ := json.Marshal(checkpoint)
			if e := s.Jobs.SaveCheckpoint(ctx, j.ID, r.WorkerID, r.LeaseEpoch, raw); e != nil {
				return nil, e
			}
			return &UploadResponse{Checkpoint: upload}, nil
		}
		plan, e := s.OSS.CreateMultipartUploadPlan(ctx, key, r.Size, mimeType)
		if e != nil {
			return nil, fmt.Errorf("create scoped upload session failed")
		}
		upload = UploadCheckpoint{ObjectKey: key, UploadID: plan.UploadID, PartSize: plan.PartSize, Size: r.Size, MimeType: mimeType, SHA256: r.SHA256, CRC64: r.CRC64}
		checkpoint.Uploads[r.Rendition] = upload
		raw, _ := json.Marshal(checkpoint)
		if e = s.Jobs.SaveCheckpoint(ctx, j.ID, r.WorkerID, r.LeaseEpoch, raw); e != nil {
			_ = s.OSS.AbortMultipartUpload(ctx, key, plan.UploadID)
			return nil, e
		}
		return &UploadResponse{Plan: plan, Checkpoint: upload}, nil
	}
	if len(r.Parts) > 0 {
		byPart := map[int]string{}
		for _, p := range upload.Parts {
			byPart[p.PartNumber] = p.ETag
		}
		maxParts := (upload.Size + upload.PartSize - 1) / upload.PartSize
		if maxParts == 0 {
			maxParts = 1
		}
		for _, p := range r.Parts {
			if p.PartNumber < 1 || int64(p.PartNumber) > maxParts || strings.TrimSpace(p.ETag) == "" {
				return nil, fmt.Errorf("invalid upload part receipt")
			}
			byPart[p.PartNumber] = p.ETag
		}
		upload.Parts = nil
		for i := 1; int64(i) <= maxParts; i++ {
			if etag, ok := byPart[i]; ok {
				upload.Parts = append(upload.Parts, baseservice.OSSCompletePart{PartNumber: i, ETag: etag})
			}
		}
	}
	if r.Complete && !upload.Complete {
		maxParts := (upload.Size + upload.PartSize - 1) / upload.PartSize
		if maxParts == 0 {
			maxParts = 1
		}
		if int64(len(upload.Parts)) != maxParts {
			return nil, fmt.Errorf("multipart receipts incomplete")
		}
		if err = s.OSS.CompleteMultipartUpload(ctx, upload.ObjectKey, upload.UploadID, upload.Parts); err != nil {
			stat, found, e := s.OSS.StatObject(ctx, upload.ObjectKey)
			if e != nil || !found || stat.ContentLength != upload.Size || stat.CRC64ECMA != upload.CRC64 {
				return nil, fmt.Errorf("multipart completion failed")
			}
		}
		stat, found, e := s.OSS.StatObject(ctx, upload.ObjectKey)
		if e != nil || !found || stat.ContentLength != upload.Size || stat.CRC64ECMA != upload.CRC64 {
			return nil, fmt.Errorf("uploaded object integrity mismatch")
		}
		upload.Complete = true
	}
	checkpoint.Uploads[r.Rendition] = upload
	raw, _ := json.Marshal(checkpoint)
	if err = s.Jobs.SaveCheckpoint(ctx, j.ID, r.WorkerID, r.LeaseEpoch, raw); err != nil {
		return nil, err
	}
	response := &UploadResponse{Checkpoint: upload}
	if !upload.Complete {
		response.Plan, err = s.OSS.ResumeMultipartUploadPlan(upload.ObjectKey, upload.UploadID, upload.Size, upload.PartSize, upload.MimeType)
	}
	return response, err
}

func validSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil && value == strings.ToLower(value)
}

// Call only after a fenced terminal transition. Never abort a resumable job
// or an upload owned by a new lease; immutable job-key prefixes enforce scope.
func (s *Service) abortTerminalUploads(ctx context.Context, j *domain.AssetMediaJob) {
	if s.OSS == nil {
		return
	}
	var checkpoint WorkerCheckpoint
	if json.Unmarshal(j.Checkpoint, &checkpoint) != nil {
		return
	}
	for _, upload := range checkpoint.Uploads {
		owned := strings.HasPrefix(upload.ObjectKey, "external-assets/media-v2/") && strings.Contains(upload.ObjectKey, "/"+j.ID+"/")
		owned = owned || strings.HasPrefix(upload.ObjectKey, "external-download-packages/"+j.ID+"/")
		if owned && !upload.Complete && upload.UploadID != "" {
			_ = s.OSS.AbortMultipartUpload(ctx, upload.ObjectKey, upload.UploadID)
		}
	}
}

func (s *Service) CompleteNAS(ctx context.Context, r WorkerRequest) error {
	if s.Jobs != nil {
		if done, err := s.Jobs.Get(ctx, r.JobID); err == nil && done != nil && done.Pool == "nas" && done.State == "succeeded" && done.LeaseEpoch == r.LeaseEpoch {
			return nil
		}
	}
	j, err := s.workerLease(ctx, r)
	if err != nil {
		return err
	}
	if r.ErrorCode != "" {
		code := r.ErrorCode
		if len(code) > 128 {
			return fmt.Errorf("invalid error code")
		}
		state := "failed"
		if code == "source_version_changed" {
			state = "stale"
		}
		err := s.Jobs.Finish(ctx, j.ID, r.WorkerID, r.LeaseEpoch, state, nil, code, r.Retryable)
		if err == nil && !r.Retryable {
			s.abortTerminalUploads(ctx, j)
		}
		return err
	}
	var cp WorkerCheckpoint
	if err = json.Unmarshal(j.Checkpoint, &cp); err != nil {
		return err
	}
	check := func(name string, obj *domain.AssetMediaObject) error {
		u, ok := cp.Uploads[name]
		if !ok || !u.Complete || obj == nil || obj.Key != u.ObjectKey || obj.Size != u.Size || obj.SHA256 != u.SHA256 || obj.CRC64 != u.CRC64 || obj.SourceVersion != j.SourceVersion || obj.Recipe != j.Recipe {
			return fmt.Errorf("media result does not match verified upload")
		}
		return nil
	}
	if !validSHA256(r.Result.SourceSHA256) {
		return fmt.Errorf("source SHA256 required")
	}
	switch j.Kind {
	case "external_original":
		if err = check("original", r.Result.Original); err != nil {
			return err
		}
		if r.Result.SourceSHA256 != r.Result.Original.SHA256 {
			return fmt.Errorf("source checksum does not match original")
		}
	case "external_renditions":
		if err = check("preview", r.Result.Preview); err != nil {
			return err
		}
		if err = check("thumbnail", r.Result.Thumbnail); err != nil {
			return err
		}
	case "external_selection_zip", "external_zip_upload":
		return s.completeZIP(ctx, j, cp, r.Result)
	default:
		return fmt.Errorf("unsupported media job")
	}
	rawRepo, ok := s.External.MediaRepository()
	if !ok {
		return fmt.Errorf("media repository unavailable")
	}
	if err = rawRepo.CommitMediaResult(ctx, j, r.Result); err != nil {
		return err
	}
	result, _ := json.Marshal(r.Result)
	return s.Jobs.Finish(ctx, j.ID, r.WorkerID, r.LeaseEpoch, "succeeded", result, "", false)
}
