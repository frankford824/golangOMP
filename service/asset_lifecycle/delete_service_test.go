package asset_lifecycle

import (
	"context"
	"slices"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
)

func TestDeleteRepairsMissingLifecycleModuleAndWritesEvent(t *testing.T) {
	assetID := int64(8801)
	storageKey := "tasks/RW-LEGACY/delivery.psd"
	row := &repo.TaskAssetSearchRow{
		Asset: &domain.TaskAsset{
			ID:              9901,
			TaskID:          7701,
			AssetID:         &assetID,
			AssetType:       domain.TaskAssetTypeDelivery,
			SourceModuleKey: domain.ModuleKeyCustomization,
			StorageKey:      &storageKey,
		},
		Task: &domain.Task{ID: 7701, TaskStatus: domain.TaskStatusCompleted},
	}
	search := &deleteSearchRepoStub{current: row, versions: []*repo.TaskAssetSearchRow{row}}
	lifecycle := &deleteLifecycleRepoStub{current: row, resolvedModuleID: 6601}
	svc := NewService(search, lifecycle, fakeTxRunner{}, nil).WithNow(func() time.Time {
		return time.Date(2026, 7, 13, 6, 0, 0, 0, time.UTC)
	})

	actor := domain.RequestActor{ID: 303, Roles: []domain.Role{domain.RoleCustomizationReviewer}}
	if appErr := svc.Delete(context.Background(), actor, assetID, "文件错误"); appErr != nil {
		t.Fatalf("Delete() appErr = %+v", appErr)
	}
	if lifecycle.resolvedTaskID != row.Task.ID || lifecycle.resolvedModuleKey != domain.ModuleKeyCustomization {
		t.Fatalf("resolved module = task:%d key:%q", lifecycle.resolvedTaskID, lifecycle.resolvedModuleKey)
	}
	if !lifecycle.softDeleted {
		t.Fatal("Delete() did not soft-delete the asset")
	}
	if lifecycle.eventModuleID != lifecycle.resolvedModuleID || lifecycle.eventType != "asset_deleted_by_admin" {
		t.Fatalf("event = module:%d type:%q", lifecycle.eventModuleID, lifecycle.eventType)
	}
}

func TestDeleteRemovesResourceAndDerivedPreviewObjects(t *testing.T) {
	assetID := int64(8802)
	parentKey := "tasks/RW-REPLACE/delivery-B.psd"
	row := &repo.TaskAssetSearchRow{
		Asset: &domain.TaskAsset{
			ID:              9902,
			TaskID:          7702,
			AssetID:         &assetID,
			AssetType:       domain.TaskAssetTypeDelivery,
			SourceModuleKey: domain.ModuleKeyCustomization,
			StorageKey:      &parentKey,
		},
		Task: &domain.Task{ID: 7702, TaskStatus: domain.TaskStatusCompleted},
	}
	search := &deleteSearchRepoStub{current: row, versions: []*repo.TaskAssetSearchRow{row}}
	wantKeys := []string{
		parentKey,
		"tasks/RW-REPLACE/previews/delivery-B-preview.webp",
		"tasks/RW-REPLACE/previews/delivery-B-thumb.webp",
	}
	lifecycle := &deleteLifecycleRepoStub{current: row, resolvedModuleID: 6602, deletionStorageKeys: wantKeys}
	deleter := &recordingObjectDeleter{enabled: true}
	svc := NewService(search, lifecycle, fakeTxRunner{}, deleter)

	actor := domain.RequestActor{ID: 303, Roles: []domain.Role{domain.RoleCustomizationReviewer}}
	if appErr := svc.Delete(context.Background(), actor, assetID, "文件错误"); appErr != nil {
		t.Fatalf("Delete() appErr = %+v", appErr)
	}
	if !slices.Equal(deleter.deletedKeys, wantKeys) {
		t.Fatalf("deleted keys = %v, want %v", deleter.deletedKeys, wantKeys)
	}
}

func TestLifecycleSourceModuleKeyDerivesMissingLegacyValues(t *testing.T) {
	tests := []struct {
		name  string
		asset *domain.TaskAsset
		task  *domain.Task
		want  string
	}{
		{name: "reference", asset: &domain.TaskAsset{AssetType: domain.TaskAssetTypeReference}, task: &domain.Task{}, want: domain.ModuleKeyBasicInfo},
		{name: "customization", asset: &domain.TaskAsset{AssetType: domain.TaskAssetTypeDelivery}, task: &domain.Task{CustomizationRequired: true}, want: domain.ModuleKeyCustomization},
		{name: "retouch", asset: &domain.TaskAsset{AssetType: domain.TaskAssetTypeSource}, task: &domain.Task{TaskType: domain.TaskTypeRetouchTask}, want: domain.ModuleKeyRetouch},
		{name: "explicit", asset: &domain.TaskAsset{AssetType: domain.TaskAssetTypeDelivery, SourceModuleKey: domain.ModuleKeyAudit}, task: &domain.Task{}, want: domain.ModuleKeyAudit},
		{name: "unknown falls back", asset: &domain.TaskAsset{AssetType: domain.TaskAssetTypeDelivery, SourceModuleKey: "legacy_unknown"}, task: &domain.Task{}, want: domain.ModuleKeyDesign},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := lifecycleSourceModuleKey(tt.asset, tt.task); got != tt.want {
				t.Fatalf("lifecycleSourceModuleKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

type deleteSearchRepoStub struct {
	current  *repo.TaskAssetSearchRow
	versions []*repo.TaskAssetSearchRow
}

func (s *deleteSearchRepoStub) Search(context.Context, domain.AssetSearchQuery) ([]*repo.TaskAssetSearchRow, int64, error) {
	return nil, 0, nil
}
func (s *deleteSearchRepoStub) GetCurrentByAssetID(context.Context, int64) (*repo.TaskAssetSearchRow, error) {
	return s.current, nil
}
func (s *deleteSearchRepoStub) ListCurrentByAssetIDs(context.Context, []int64) ([]*repo.TaskAssetSearchRow, error) {
	return nil, nil
}
func (s *deleteSearchRepoStub) ListVersionsByAssetID(context.Context, int64) ([]*repo.TaskAssetSearchRow, error) {
	return s.versions, nil
}
func (s *deleteSearchRepoStub) GetVersion(context.Context, int64, int64) (*repo.TaskAssetSearchRow, error) {
	return nil, nil
}

type deleteLifecycleRepoStub struct {
	current             *repo.TaskAssetSearchRow
	resolvedModuleID    int64
	resolvedTaskID      int64
	resolvedModuleKey   string
	softDeleted         bool
	eventModuleID       int64
	eventType           domain.ModuleEventType
	deletionStorageKeys []string
}

func (s *deleteLifecycleRepoStub) ListResourceDeletionStorageKeys(context.Context, int64) ([]string, error) {
	return append([]string(nil), s.deletionStorageKeys...), nil
}

func (s *deleteLifecycleRepoStub) Archive(context.Context, repo.Tx, repo.TaskAssetLifecycleUpdate) error {
	return nil
}
func (s *deleteLifecycleRepoStub) Restore(context.Context, repo.Tx, repo.TaskAssetLifecycleUpdate) error {
	return nil
}
func (s *deleteLifecycleRepoStub) SoftDelete(context.Context, repo.Tx, repo.TaskAssetLifecycleUpdate) error {
	s.softDeleted = true
	return nil
}
func (s *deleteLifecycleRepoStub) MarkAutoCleaned(context.Context, repo.Tx, int64, time.Time) error {
	return nil
}
func (s *deleteLifecycleRepoStub) ListEligibleForCleanup(context.Context, time.Time, int) ([]*repo.TaskAssetCleanupCandidate, error) {
	return nil, nil
}
func (s *deleteLifecycleRepoStub) GetCurrentForUpdate(context.Context, repo.Tx, int64) (*repo.TaskAssetSearchRow, error) {
	return s.current, nil
}
func (s *deleteLifecycleRepoStub) InsertLifecycleEvent(_ context.Context, _ repo.Tx, moduleID int64, eventType domain.ModuleEventType, _ *int64, _ interface{}) error {
	s.eventModuleID = moduleID
	s.eventType = eventType
	return nil
}
func (s *deleteLifecycleRepoStub) ResolveOrCreateLifecycleEventModule(_ context.Context, _ repo.Tx, taskID int64, moduleKey string) (int64, error) {
	s.resolvedTaskID = taskID
	s.resolvedModuleKey = moduleKey
	return s.resolvedModuleID, nil
}

type recordingObjectDeleter struct {
	enabled     bool
	deletedKeys []string
}

func (d *recordingObjectDeleter) Enabled() bool { return d.enabled }

func (d *recordingObjectDeleter) DeleteObject(_ context.Context, key string) error {
	d.deletedKeys = append(d.deletedKeys, key)
	return nil
}
