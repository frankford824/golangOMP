package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/crc64"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"workflow/domain"
	"workflow/internal/assetmedia"
	baseservice "workflow/service"
	assetdelivery "workflow/service/asset_delivery"
)

type agent struct {
	uploadSlots                            chan struct{}
	base, token, id, root, work, artifacts string
	client                                 *http.Client
	renderer                               baseservice.AssetPreviewRenderer
	renderSlots                            chan struct{}
	limiter                                *assetmedia.DirectionLimiter
	budgetMu                               sync.Mutex
	reserved                               int64
	maxTemp                                int64
}

type jobProgress struct {
	mu          sync.Mutex
	phase       string
	done, total int64
}

func (p *jobProgress) set(phase string, done, total int64) {
	p.mu.Lock()
	p.phase = phase
	p.done = done
	p.total = total
	p.mu.Unlock()
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
func number(key string, fallback int64) int64 {
	n, e := strconv.ParseInt(os.Getenv(key), 10, 64)
	if e != nil {
		return fallback
	}
	return n
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	a := &agent{base: env("MEDIA_CONTROL_URL", "https://yongbo.cloud"), token: os.Getenv("ASSET_MEDIA_WORKER_TOKEN"), id: env("MEDIA_WORKER_ID", "company-nas-worker"),
		root: env("MEDIA_SOURCE_ROOT", "/data/image_lib"), work: env("MEDIA_WORK_DIR", "/work"), artifacts: env("MEDIA_ARTIFACT_DIR", "/artifacts"),
		client:   &http.Client{Timeout: 45 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
		renderer: baseservice.NewExternalAssetPreviewRenderer(), renderSlots: make(chan struct{}, 1), uploadSlots: make(chan struct{}, 2),
		limiter: &assetmedia.DirectionLimiter{DayMbps: number("MEDIA_DAY_MBPS", 30), NightMbps: number("MEDIA_NIGHT_MBPS", 60)}, maxTemp: number("MEDIA_TEMP_BYTES", 20<<30)}
	u, err := url.Parse(a.base)
	if err != nil || u.Scheme != "https" || a.token == "" {
		log.Fatal("HTTPS control URL and worker credential required")
	}
	if err = os.MkdirAll(a.work, 0700); err != nil {
		log.Fatal(err)
	}
	if err = os.MkdirAll(a.artifacts, 0700); err != nil {
		log.Fatal(err)
	}
	workspaceLock, err := assetmedia.AcquireWorkerWorkspace(a.root, a.work)
	if err != nil {
		log.Fatal(err)
	}
	defer workspaceLock.Close()
	if err = a.run(ctx); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
}

func (a *agent) api(ctx context.Context, path string, body, result any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(a.base, "/")+"/v1/integration/asset-media/workers/"+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Asset-Media-Worker-Token", a.token)
	response, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("control transport unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		var envelope struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		_ = json.NewDecoder(io.LimitReader(response.Body, 16<<10)).Decode(&envelope)
		return fmt.Errorf("%s: control request rejected HTTP %d", envelope.Error.Code, response.StatusCode)
	}
	if result == nil {
		return nil
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err = json.NewDecoder(io.LimitReader(response.Body, 8<<20)).Decode(&envelope); err != nil {
		return err
	}
	return json.Unmarshal(envelope.Data, result)
}

func (a *agent) run(ctx context.Context) error {
	active := map[string]chan struct{}{"render": make(chan struct{}, 1), "transfer": make(chan struct{}, 2)}
	var wg sync.WaitGroup
	defer wg.Wait()
	for ctx.Err() == nil {
		for _, class := range []string{"render", "transfer"} {
			slots := active[class]
			available := cap(slots) - len(slots)
			if available > 0 {
				var claims []assetdelivery.WorkerClaim
				if err := a.api(ctx, "claim", map[string]any{"worker_id": a.id, "limit": available, "class": class}, &claims); err != nil {
					log.Printf("media claim unavailable")
				} else {
					for _, claim := range claims {
						slots <- struct{}{}
						wg.Add(1)
						go func(c assetdelivery.WorkerClaim, slots chan struct{}) {
							defer wg.Done()
							defer func() { <-slots }()
							a.process(ctx, c)
						}(claim, slots)
					}
				}
			}
		}
		timer := time.NewTimer(3 * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return ctx.Err()
}

func (a *agent) process(parent context.Context, claim assetdelivery.WorkerClaim) {
	job := claim.Job
	if job == nil {
		return
	}
	timeout := 6 * time.Hour
	if job.Kind == "external_renditions" {
		timeout = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	progress := &jobProgress{phase: "preparing"}
	done := make(chan struct{})
	heartbeatDone := make(chan struct{})
	go func() {
		defer close(heartbeatDone)
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				progress.mu.Lock()
				request := assetdelivery.WorkerRequest{JobID: job.ID, WorkerID: a.id, LeaseEpoch: job.LeaseEpoch, Phase: progress.phase, ProcessedBytes: progress.done, TotalBytes: progress.total}
				progress.mu.Unlock()
				heartCtx, stop := context.WithTimeout(ctx, 10*time.Second)
				err := a.api(heartCtx, "heartbeat", request, nil)
				stop()
				if err != nil {
					cancel()
					return
				}
			}
		}
	}()
	result, err := a.execute(ctx, claim, progress)
	close(done)
	<-heartbeatDone
	request := assetdelivery.WorkerRequest{JobID: job.ID, WorkerID: a.id, LeaseEpoch: job.LeaseEpoch, Result: result}
	if err != nil {
		request.ErrorCode = "media_processing_failed"
		request.Retryable = true
		msg := strings.ToLower(err.Error())
		switch {
		case strings.Contains(msg, "source_disabled"), strings.Contains(msg, "media_integrity_failed"):
			request.ErrorCode = "media_source_or_integrity_rejected"
			request.Retryable = false
		case strings.Contains(msg, "source_version_changed"):
			request.ErrorCode = "source_version_changed"
			request.Retryable = false
		case os.IsNotExist(err), strings.Contains(msg, "source_missing"):
			request.ErrorCode = "source_missing"
			request.Retryable = false
		case baseservice.MediaRenderFailureIsPermanent(err):
			request.ErrorCode = "preview_unsupported_or_invalid"
			request.Retryable = false
		case strings.Contains(msg, "temporary_space"):
			request.ErrorCode = "temporary_space_limit"
			request.Retryable = false
		}
	}
	finishCtx, stop := context.WithTimeout(context.Background(), 30*time.Second)
	finishErr := a.api(finishCtx, "complete", request, nil)
	stop()
	log.Printf("media job finished job_id=%s kind=%s error_code=%s acknowledged=%t", job.ID, job.Kind, request.ErrorCode, finishErr == nil)
}

func (a *agent) reserve(ctx context.Context, size int64) (func(), error) {
	if size > a.maxTemp {
		return nil, fmt.Errorf("temporary_space_limit")
	}
	for {
		a.budgetMu.Lock()
		if a.reserved+size <= a.maxTemp {
			a.reserved += size
			a.budgetMu.Unlock()
			return func() { a.budgetMu.Lock(); a.reserved -= size; a.budgetMu.Unlock() }, nil
		}
		a.budgetMu.Unlock()
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

func (a *agent) execute(ctx context.Context, claim assetdelivery.WorkerClaim, progress *jobProgress) (domain.ExternalMediaResult, error) {
	var result domain.ExternalMediaResult
	job := claim.Job
	if job.Recipe != domain.AssetMediaRecipe {
		return result, fmt.Errorf("unsupported media recipe")
	}
	reserve := claim.Source.Size
	if job.Kind == "external_renditions" {
		reserve += 8 << 30
	}
	if len(claim.Inputs) > 0 {
		reserve = 0
		for _, input := range claim.Inputs {
			reserve += input.Size * 3
		}
	}
	release, err := a.reserve(ctx, reserve)
	if err != nil {
		return result, err
	}
	defer release()
	work, err := os.MkdirTemp(a.work, job.ID+"-")
	if err != nil {
		return result, err
	}
	defer os.RemoveAll(work)
	if job.Kind == "external_selection_zip" || job.Kind == "external_zip_upload" {
		return a.executeZIP(ctx, claim, progress, work)
	}
	rootInfo, err := os.Stat(a.root)
	if err != nil {
		return result, fmt.Errorf("NAS source root unavailable")
	}
	identity, _, err := assetmedia.FileIdentity(rootInfo)
	if err != nil || identity != claim.Source.RootIdentity {
		return result, fmt.Errorf("NAS source root identity changed")
	}
	sourcePath := filepath.Join(work, "source"+filepath.Ext(claim.Source.Filename))
	progress.set("snapshotting", 0, 0)
	if err = assetmedia.CloneStableSource(a.root, claim.Source.RelativePath, claim.Source, sourcePath); err != nil {
		return result, err
	}
	sha, crc, size, err := checksum(ctx, sourcePath)
	if err != nil {
		return result, err
	}
	if claim.Source.SHA256 != "" && claim.Source.SHA256 != sha {
		return result, fmt.Errorf("source_version_changed")
	}
	result.SourceSHA256 = sha
	if job.Kind == "external_original" {
		object, err := a.upload(ctx, claim, "original", sourcePath, claim.Source.Filename, claim.Source.MimeType, sha, crc, size, progress)
		result.Original = object
		return result, err
	}
	if job.Kind != "external_renditions" {
		return result, fmt.Errorf("unsupported job kind")
	}
	select {
	case a.renderSlots <- struct{}{}:
		defer func() { <-a.renderSlots }()
	case <-ctx.Done():
		return result, ctx.Err()
	}
	progress.set("rendering", 0, 0)
	preview, err := a.renderer.Render(ctx, sourcePath, baseservice.AssetPreviewSourceMeta{Filename: claim.Source.Filename, MimeType: claim.Source.MimeType}, baseservice.AssetPreviewRenderSpec{MaxWidth: 1600, MaxHeight: 1600, Quality: 82})
	if err != nil {
		return result, err
	}
	previewPath := filepath.Join(work, "preview.webp")
	if err = os.WriteFile(previewPath, preview, 0600); err != nil {
		return result, err
	}
	thumb, err := a.renderer.Render(ctx, previewPath, baseservice.AssetPreviewSourceMeta{Filename: "preview.webp", MimeType: "image/webp"}, baseservice.AssetPreviewRenderSpec{MaxWidth: 480, MaxHeight: 480, Quality: 75})
	if err != nil {
		return result, err
	}
	thumbPath := filepath.Join(work, "thumbnail.webp")
	if err = os.WriteFile(thumbPath, thumb, 0600); err != nil {
		return result, err
	}
	for _, item := range []struct{ name, path string }{{"preview", previewPath}, {"thumbnail", thumbPath}} {
		sha, crc, size, err := checksum(ctx, item.path)
		if err != nil {
			return result, err
		}
		object, err := a.upload(ctx, claim, item.name, item.path, item.name+".webp", "image/webp", sha, crc, size, progress)
		if err != nil {
			return result, err
		}
		content := domain.AssetMediaIdentity(job.ResourceID, job.SourceVersion, item.name, job.Recipe)
		if err = assetmedia.StoreArtifact(ctx, a.artifacts, assetmedia.ReadTarget{ContentID: content, SourceVersion: job.SourceVersion, Size: size, SHA256: sha, CRC64: crc}, item.path, time.Now().Add(7*24*time.Hour)); err != nil {
			return result, err
		}
		if item.name == "preview" {
			result.Preview = object
		} else {
			result.Thumbnail = object
		}
	}
	return result, nil
}

func checksum(ctx context.Context, path string) (string, string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", "", 0, err
	}
	defer f.Close()
	sha := sha256.New()
	crc := crc64.New(crc64.MakeTable(crc64.ECMA))
	buffer := make([]byte, 256<<10)
	var count int64
	for {
		if err = ctx.Err(); err != nil {
			return "", "", 0, err
		}
		n, e := f.Read(buffer)
		if n > 0 {
			_, _ = sha.Write(buffer[:n])
			_, _ = crc.Write(buffer[:n])
			count += int64(n)
		}
		if e == io.EOF {
			break
		}
		if e != nil {
			return "", "", 0, e
		}
	}
	return hex.EncodeToString(sha.Sum(nil)), strconv.FormatUint(crc.Sum64(), 10), count, nil
}

type limitedReader struct {
	ctx     context.Context
	r       io.Reader
	limiter *assetmedia.DirectionLimiter
}

func (r *limitedReader) Read(p []byte) (int, error) {
	if len(p) > 64<<10 {
		p = p[:64<<10]
	}
	n, err := r.r.Read(p)
	if n > 0 {
		if e := r.limiter.Wait(r.ctx, n); e != nil {
			return 0, e
		}
	}
	return n, err
}

func (a *agent) upload(ctx context.Context, claim assetdelivery.WorkerClaim, rendition, filePath, filename, mimeType, sha, crc string, size int64, progress *jobProgress) (*domain.AssetMediaObject, error) {
	if rendition == "original" || rendition == "zip" {
		select {
		case a.uploadSlots <- struct{}{}:
			defer func() { <-a.uploadSlots }()
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	request := assetdelivery.UploadRequest{WorkerRequest: assetdelivery.WorkerRequest{JobID: claim.Job.ID, WorkerID: a.id, LeaseEpoch: claim.Job.LeaseEpoch}, Rendition: rendition, Size: size, SHA256: sha, CRC64: crc}
	var grant assetdelivery.UploadResponse
	if err := a.api(ctx, "uploads", request, &grant); err != nil {
		return nil, err
	}
	for grant.Pending {
		progress.set("verifying_copy", 0, size)
		timer := time.NewTimer(5 * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
		grant = assetdelivery.UploadResponse{}
		if err := a.api(ctx, "uploads", request, &grant); err != nil {
			return nil, err
		}
	}
	if grant.Checkpoint.Complete {
		return &domain.AssetMediaObject{Key: grant.Checkpoint.ObjectKey, Filename: filename, MimeType: mimeType, Size: size, SHA256: sha, CRC64: crc, SourceVersion: claim.Job.SourceVersion, Recipe: claim.Job.Recipe}, nil
	}
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	completed := map[int]string{}
	var sent int64
	for _, part := range grant.Checkpoint.Parts {
		completed[part.PartNumber] = part.ETag
		length := grant.Checkpoint.PartSize
		offset := int64(part.PartNumber-1) * length
		if offset+length > size {
			length = size - offset
		}
		sent += length
	}
	for _, part := range grant.Plan.Parts {
		if _, ok := completed[part.PartNumber]; ok {
			continue
		}
		if time.Until(part.ExpiresAt) < time.Minute {
			if err = a.api(ctx, "uploads", request, &grant); err != nil {
				return nil, err
			}
			part = grant.Plan.Parts[part.PartNumber-1]
		}
		offset := int64(part.PartNumber-1) * grant.Checkpoint.PartSize
		length := grant.Checkpoint.PartSize
		if offset+length > size {
			length = size - offset
		}
		progress.set("uploading_"+rendition, sent, size)
		var etag string
		for attempt := 0; attempt < 3; attempt++ {
			reader := &limitedReader{ctx: ctx, r: io.NewSectionReader(f, offset, length), limiter: a.limiter}
			req, e := http.NewRequestWithContext(ctx, http.MethodPut, part.UploadURL, reader)
			if e != nil {
				return nil, fmt.Errorf("invalid scoped upload URL")
			}
			req.ContentLength = length
			req.Header.Set("Content-Type", grant.Plan.RequiredContentType)
			client := &http.Client{Timeout: 10 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
			response, e := client.Do(req)
			if e == nil {
				_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
				response.Body.Close()
				if response.StatusCode == 200 {
					etag = strings.Trim(response.Header.Get("ETag"), "\"")
					break
				}
			}
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			timer := time.NewTimer(time.Duration(attempt+1) * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
		}
		if etag == "" {
			return nil, fmt.Errorf("multipart upload failed")
		}
		request.Parts = []baseservice.OSSCompletePart{{PartNumber: part.PartNumber, ETag: etag}}
		if err = a.api(ctx, "uploads", request, &grant); err != nil {
			return nil, err
		}
		sent += length
		progress.set("uploading_"+rendition, sent, size)
	}
	request.Parts = nil
	request.Complete = true
	if err = a.api(ctx, "uploads", request, &grant); err != nil {
		return nil, err
	}
	return &domain.AssetMediaObject{Key: grant.Checkpoint.ObjectKey, Filename: filename, MimeType: mimeType, Size: size, SHA256: sha, CRC64: crc, SourceVersion: claim.Job.SourceVersion, Recipe: claim.Job.Recipe}, nil
}

func (a *agent) executeZIP(ctx context.Context, claim assetdelivery.WorkerClaim, progress *jobProgress, work string) (domain.ExternalMediaResult, error) {
	result := domain.ExternalMediaResult{SourceSHA256: claim.Job.SourceVersion}
	zipPath := filepath.Join(work, "selection.zip")
	if existing, err := assetmedia.OpenArtifact(a.artifacts, claim.Source); err == nil {
		out, e := os.Create(zipPath)
		if e != nil {
			existing.Close()
			return result, e
		}
		_, e = io.Copy(out, existing)
		existing.Close()
		out.Close()
		if e != nil {
			return result, e
		}
	} else {
		out, e := os.Create(zipPath)
		if e != nil {
			return result, e
		}
		writer := zip.NewWriter(out)
		var copied int64
		for i, input := range claim.Inputs {
			if err = ctx.Err(); err != nil {
				writer.Close()
				out.Close()
				return result, err
			}
			snapshot := filepath.Join(work, fmt.Sprintf("input-%d", i))
			if err = assetmedia.CloneStableSource(a.root, input.RelativePath, input, snapshot); err != nil {
				writer.Close()
				out.Close()
				return result, err
			}
			header := &zip.FileHeader{Name: input.Filename, Method: zip.Store, Modified: time.Unix(0, input.ModifiedNS).UTC()}
			header.SetMode(0644)
			entry, e := writer.CreateHeader(header)
			if e != nil {
				writer.Close()
				out.Close()
				return result, e
			}
			file, e := os.Open(snapshot)
			if e != nil {
				writer.Close()
				out.Close()
				return result, e
			}
			n, e := io.Copy(entry, file)
			file.Close()
			_ = os.Remove(snapshot)
			if e != nil {
				writer.Close()
				out.Close()
				return result, e
			}
			copied += n
			progress.set("packaging", copied, claim.Job.TotalBytes)
		}
		if e = writer.Close(); e != nil {
			out.Close()
			return result, e
		}
		if e = out.Close(); e != nil {
			return result, e
		}
	}
	sha, crc, size, err := checksum(ctx, zipPath)
	if err != nil {
		return result, err
	}
	if claim.Source.SHA256 != "" && claim.Source.SHA256 != sha {
		return result, fmt.Errorf("source_version_changed: package checksum differs")
	}
	object := &domain.AssetMediaObject{Filename: claim.Source.Filename, MimeType: "application/zip", Size: size, SHA256: sha, CRC64: crc, SourceVersion: claim.Job.SourceVersion, Recipe: claim.Job.Recipe}
	if claim.Job.Kind == "external_zip_upload" {
		object, err = a.upload(ctx, claim, "zip", zipPath, claim.Source.Filename, "application/zip", sha, crc, size, progress)
		if err != nil {
			return result, err
		}
	}
	target := claim.Source
	target.Size = size
	target.SHA256 = sha
	target.CRC64 = crc
	if err = assetmedia.StoreArtifact(ctx, a.artifacts, target, zipPath, time.Now().Add(24*time.Hour)); err != nil {
		return result, err
	}
	result.Package = object
	return result, nil
}
