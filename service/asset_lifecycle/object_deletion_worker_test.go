package asset_lifecycle

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
)

func TestObjectDeletionWorkerTreats404AsSuccessAndRetriesWithAlert(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	asset1, asset2, asset3 := int64(101), int64(102), int64(103)
	repository := &objectDeletionOutboxRepoStub{items: []repo.AssetObjectDeletionOutboxItem{
		{ID: 1, TaskAssetID: &asset1, StorageAdapter: domain.AssetStorageAdapterOSSUploadService, StorageKey: "tasks/1/source.psd", Attempt: 1},
		{ID: 2, TaskAssetID: &asset2, StorageAdapter: domain.AssetStorageAdapterOSSUploadService, StorageKey: "tasks/1/missing.webp", Attempt: 2},
		{ID: 3, TaskAssetID: &asset3, StorageAdapter: domain.AssetStorageAdapterOSSUploadService, StorageKey: "tasks/1/retry.ai", Attempt: 5},
	}}
	deleter := &objectDeletionDeleterStub{errorsByKey: map[string]error{
		"tasks/1/missing.webp": errors.New("oss direct delete failed: status=404 body=NoSuchKey"),
		"tasks/1/retry.ai":     errors.New("oss direct delete failed: status=503 body=busy"),
	}}
	worker := NewObjectDeletionWorker(repository, fakeTxRunner{}, deleter, ObjectDeletionWorkerConfig{
		LeaseTTL: 30 * time.Second, RetryBase: time.Minute, RetryMax: time.Hour, AlertAfterAttempt: 5,
	}, nil).WithNow(func() time.Time { return now }).WithLeaseTokenGenerator(func() string { return "lease-1" })

	result, appErr := worker.RunOnce(context.Background(), 10)
	if appErr != nil {
		t.Fatalf("RunOnce() appErr = %+v", appErr)
	}
	if result.Claimed != 3 || result.Succeeded != 2 || result.Retried != 1 || result.Alerted != 1 {
		t.Fatalf("result = %+v", result)
	}
	if len(repository.succeeded) != 2 || repository.succeeded[0].ID != 1 || repository.succeeded[1].ID != 2 {
		t.Fatalf("succeeded = %+v", repository.succeeded)
	}
	if len(repository.retried) != 1 || repository.retried[0].item.ID != 3 || !repository.retried[0].alert {
		t.Fatalf("retried = %+v", repository.retried)
	}
	if want := now.Add(16 * time.Minute); !repository.retried[0].nextRetry.Equal(want) {
		t.Fatalf("next retry = %s, want %s", repository.retried[0].nextRetry, want)
	}
}

func TestObjectDeletionWorkerDispatchesOnlyOSSAdapterAndFailsUnknownClosed(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	repository := &objectDeletionOutboxRepoStub{items: []repo.AssetObjectDeletionOutboxItem{
		{ID: 10, StorageAdapter: domain.AssetStorageAdapterPlaceholderStorage, StorageKey: "placeholder:10", Attempt: 1},
		{ID: 11, StorageAdapter: domain.AssetStorageAdapterMockUpload, StorageKey: "mock:11", Attempt: 1},
		{ID: 12, StorageAdapter: domain.AssetStorageAdapterExportPlaceholder, StorageKey: "export:12", Attempt: 1},
		{ID: 13, StorageAdapter: domain.AssetStorageAdapterOSSUploadService, StorageIsPlaceholder: true, StorageKey: "oss-stub:13", Attempt: 1},
		{ID: 14, StorageAdapter: domain.AssetStorageAdapter("nas"), StorageKey: "/volume/assets/14.psd", Attempt: 1},
	}}
	deleter := &objectDeletionDeleterStub{errorsByKey: map[string]error{}}
	worker := NewObjectDeletionWorker(repository, fakeTxRunner{}, deleter, ObjectDeletionWorkerConfig{
		RetryBase: time.Minute, RetryMax: time.Hour, AlertAfterAttempt: 5,
	}, nil).WithNow(func() time.Time { return now }).WithLeaseTokenGenerator(func() string { return "lease-adapter" })

	result, appErr := worker.RunOnce(context.Background(), 10)
	if appErr != nil {
		t.Fatalf("RunOnce() appErr = %+v", appErr)
	}
	if result.Claimed != 5 || result.Succeeded != 4 || result.Retried != 1 || result.Alerted != 1 {
		t.Fatalf("result = %+v", result)
	}
	if len(deleter.calls) != 0 {
		t.Fatalf("non-physical or unknown adapters reached OSS: %v", deleter.calls)
	}
	if len(repository.retried) != 1 || repository.retried[0].item.ID != 14 || !repository.retried[0].alert ||
		!strings.Contains(repository.retried[0].lastError, "unsupported storage adapter") {
		t.Fatalf("unknown adapter retry = %+v", repository.retried)
	}
}

type objectDeletionRetryCall struct {
	item      repo.AssetObjectDeletionOutboxItem
	lastError string
	nextRetry time.Time
	alert     bool
}

type objectDeletionOutboxRepoStub struct {
	items     []repo.AssetObjectDeletionOutboxItem
	succeeded []repo.AssetObjectDeletionOutboxItem
	retried   []objectDeletionRetryCall
}

func (s *objectDeletionOutboxRepoStub) ClaimObjectDeletions(_ context.Context, _ repo.Tx, _ string, _, _ time.Time, _ int) ([]repo.AssetObjectDeletionOutboxItem, error) {
	items := append([]repo.AssetObjectDeletionOutboxItem(nil), s.items...)
	s.items = nil
	return items, nil
}

func (s *objectDeletionOutboxRepoStub) MarkObjectDeletionSucceeded(_ context.Context, _ repo.Tx, item repo.AssetObjectDeletionOutboxItem, _ string, _ time.Time) error {
	s.succeeded = append(s.succeeded, item)
	return nil
}

func (s *objectDeletionOutboxRepoStub) MarkObjectDeletionRetry(_ context.Context, _ repo.Tx, item repo.AssetObjectDeletionOutboxItem, _, lastError string, nextRetryAt time.Time, alert bool) error {
	s.retried = append(s.retried, objectDeletionRetryCall{item: item, lastError: lastError, nextRetry: nextRetryAt, alert: alert})
	return nil
}

type objectDeletionDeleterStub struct {
	errorsByKey map[string]error
	calls       []string
}

func (*objectDeletionDeleterStub) Enabled() bool { return true }

func (s *objectDeletionDeleterStub) DeleteObject(_ context.Context, key string) error {
	s.calls = append(s.calls, key)
	return s.errorsByKey[key]
}
