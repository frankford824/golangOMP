package assetdelivery

import (
	"context"
	"testing"
	"time"
	"workflow/domain"
	"workflow/repo"
	baseservice "workflow/service"
)

type mediaTaskStub struct {
	baseservice.TaskAssetCenterService
	calls   int
	options domain.AssetMediaOptions
}

func (s *mediaTaskStub) GetTaskAssetDownloadInfoByID(ctx context.Context, id int64) (*domain.AssetDownloadInfo, *domain.AppError) {
	s.calls++
	s.options = domain.AssetMediaOptionsFromContext(ctx)
	url := "https://example.com/bound-object"
	return &domain.AssetDownloadInfo{SourceVersion: "v2", Rendition: "original", State: "ready", DownloadURL: &url, Filename: "原件.jpg", FileSize: 100}, nil
}
func (s *mediaTaskStub) GetTaskAssetPreviewInfoByID(ctx context.Context, id int64) (*domain.AssetDownloadInfo, *domain.AppError) {
	info, err := s.GetTaskAssetDownloadInfoByID(ctx, id)
	info.Rendition = "preview"
	return info, err
}

type jobsAccessStub struct {
	repo.AssetMediaJobRepo
	owned  bool
	job    *domain.AssetMediaJob
	access *domain.AssetMediaAccessReference
}

func (s jobsAccessStub) HasRequest(context.Context, string, int64) (bool, error) { return s.owned, nil }
func (s jobsAccessStub) Get(context.Context, string) (*domain.AssetMediaJob, error) {
	return s.job, nil
}
func (s jobsAccessStub) GetRequestAccess(context.Context, string, int64) (*domain.AssetMediaAccessReference, error) {
	return s.access, nil
}

func TestDeliveryChecksActorAndVersionBeforeExposingURL(t *testing.T) {
	center := &mediaTaskStub{}
	s := &Service{TaskCenter: center}
	request := Request{ResourceKind: "task_asset", ResourceID: "1", Purpose: "download", Rendition: "original", Delivery: "lan", ExpectedSourceVersion: "v1"}
	if _, err := s.Resolve(context.Background(), request); err == nil || center.calls != 0 {
		t.Fatal("anonymous request reached source")
	}
	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{ID: 1, Permissions: []domain.PermissionCode{domain.PermissionAssetView}})
	if _, err := s.Resolve(ctx, request); err == nil || center.calls != 0 {
		t.Fatal("preview permission allowed original download")
	}
	ctx = domain.WithRequestActor(context.Background(), domain.RequestActor{ID: 1, Permissions: []domain.PermissionCode{domain.PermissionAssetDownload}})
	if info, err := s.Resolve(ctx, request); err == nil || err.Code != "source_version_changed" || info != nil {
		t.Fatalf("version drift leaked URL: %+v %v", info, err)
	}
	request.ExpectedSourceVersion = "v2"
	info, err := s.Resolve(ctx, request)
	if err != nil || info.State != "ready" || center.options.Delivery != "cloud" || info.LANDelivery != nil {
		t.Fatalf("disabled LAN must stay cloud: %+v %v", info, err)
	}
}

func TestMediaTaskAuthorizationIsReadOnlyAndUserScoped(t *testing.T) {
	center := &mediaTaskStub{}
	jobs := jobsAccessStub{owned: false, job: &domain.AssetMediaJob{ID: "job", SourceVersion: "v2"}, access: &domain.AssetMediaAccessReference{ResourceKind: "task_asset", ResourceID: "1", Purpose: "preview", Rendition: "preview", ExpectedSourceVersion: "v2"}}
	s := &Service{TaskCenter: center, Jobs: jobs}
	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{ID: 2, Permissions: []domain.PermissionCode{domain.PermissionAssetView}})
	if _, err := s.JobForActor(ctx, "job"); err == nil || center.calls != 0 {
		t.Fatal("another user's task was visible")
	}
	jobs.owned = true
	s.Jobs = jobs
	if _, err := s.JobForActor(ctx, "job"); err != nil || !center.options.AuthorizeOnly {
		t.Fatalf("task reauthorization must not enqueue work: %v", err)
	}
	jobs.access.ExpectedSourceVersion = "v1"
	if _, err := s.JobForActor(ctx, "job"); err == nil || err.Code != "source_version_changed" {
		t.Fatalf("stale task accepted: %v", err)
	}
}

func TestNASLeaseRejectsOldEpochAndExpiredWorker(t *testing.T) {
	expires := time.Now().Add(time.Minute)
	job := &domain.AssetMediaJob{ID: "job", Pool: "nas", State: "processing", LeaseOwner: "worker", LeaseEpoch: 2, LeaseExpiresAt: &expires}
	s := &Service{Jobs: jobsAccessStub{job: job}}
	s.Config.NASWorkerEnabled = true
	for _, req := range []WorkerRequest{{JobID: "job", WorkerID: "worker", LeaseEpoch: 1}, {JobID: "job", WorkerID: "other", LeaseEpoch: 2}} {
		if _, err := s.workerLease(context.Background(), req); err == nil {
			t.Fatal("accepted stale worker")
		}
	}
	expires = time.Now().Add(-time.Second)
	if _, err := s.workerLease(context.Background(), WorkerRequest{JobID: "job", WorkerID: "worker", LeaseEpoch: 2}); err == nil {
		t.Fatal("accepted expired lease")
	}
}
