package asset_center

import (
	"context"
	"fmt"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type retouchVersionsRepo struct {
	batchRepoStub
	versions map[int64][]*repo.TaskAssetSearchRow
}

func (r *retouchVersionsRepo) ListVersionsByAssetID(_ context.Context, id int64) ([]*repo.TaskAssetSearchRow, error) {
	return r.versions[id], nil
}

type retouchRequirementStub struct {
	requirement *domain.TaskRetouchRequirement
}

func (r *retouchRequirementStub) GetByID(_ context.Context, id int64) (*domain.TaskRetouchRequirement, error) {
	if r.requirement != nil && r.requirement.ID == id {
		return r.requirement, nil
	}
	return nil, nil
}
func retouchDownloadActor(permission domain.PermissionCode, scope domain.AccessScopeMode) context.Context {
	actor := domain.RequestActor{ID: 228, EffectiveAccess: &domain.EffectiveAccess{
		UserID: 228, Permissions: []domain.PermissionCode{permission},
		Assignments: []domain.AccessAssignment{{UserID: 228, RoleID: 1, ScopeMode: scope}},
		Sources:     []domain.EffectiveAccessNote{{Permission: permission, RoleID: 1, ScopeMode: scope}},
	}}
	return domain.WithRequestActor(context.Background(), actor)
}
func retouchDownloadFixture() (*Service, *retouchVersionsRepo, *retouchRequirementStub, []int64) {
	r := &retouchVersionsRepo{versions: map[int64][]*repo.TaskAssetSearchRow{}}
	requirements := &retouchRequirementStub{&domain.TaskRetouchRequirement{ID: 425, TaskID: 6623}}
	signer := &batchPresignerStub{enabled: true, expiresAt: time.Now().Add(time.Hour), urlByKey: map[string]string{}}
	ids := []int64{}
	for i := int64(0); i < 12; i++ {
		id, key := int64(76063)+i, fmt.Sprintf("retouch-input-%d", i)
		ids = append(ids, id)
		r.versions[id] = []*repo.TaskAssetSearchRow{{Asset: &domain.TaskAsset{ID: 82067 + i, AssetID: int64PtrBatchSvc(id), TaskID: 6623, AssetType: domain.TaskAssetTypeSource, RetouchRequirementID: int64PtrBatchSvc(425), FileName: fmt.Sprintf("待修素材%02d.jpg", i+1), FileSize: int64PtrBatchSvc(100), StorageKey: strPtr(key), UploadStatus: strPtr("uploaded")}, Task: &domain.Task{ID: 6623, CreatorID: 7, TaskType: domain.TaskTypeRetouchTask, TaskStatus: domain.TaskStatusInProgress}}}
		signer.urlByKey[key] = "https://oss.example/" + key
	}
	return NewService(r, signer, nil, WithRetouchInputDownloads(requirements)), r, requirements, ids
}
func TestRetouchInputBatchDownloadWithoutCurrentPointers(t *testing.T) {
	svc, _, _, ids := retouchDownloadFixture()
	ctx := retouchDownloadActor(domain.PermissionAssetDownload, domain.AccessScopeGlobal)
	manifest, err := svc.BuildBatchDownloadManifest(ctx, ids)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.SuccessCount != 12 || manifest.FailureCount != 0 || manifest.TotalSize != 1200 {
		t.Fatalf("manifest=%+v", manifest)
	}
	for i, item := range manifest.Items {
		if item.AssetID != ids[i] || item.DownloadURL == "" {
			t.Fatalf("item=%+v", item)
		}
	}
	info, err := svc.DownloadLatest(ctx, ids[0])
	if err != nil || info.DownloadURL == nil {
		t.Fatalf("single=%+v err=%+v", info, err)
	}
}
func TestRetouchInputDownloadFailsClosed(t *testing.T) {
	for _, reason := range []string{"anonymous", "view_only", "outside_scope", "not_retouch", "final_output", "no_requirement", "deleted_requirement", "foreign_requirement", "foreign_task", "deleted", "cleaned", "incomplete", "missing_object", "old_version_after_delete"} {
		t.Run(reason, func(t *testing.T) {
			svc, r, requirements, ids := retouchDownloadFixture()
			ctx := retouchDownloadActor(domain.PermissionAssetDownload, domain.AccessScopeGlobal)
			row := r.versions[ids[0]][0]
			now := time.Now()
			switch reason {
			case "anonymous":
				ctx = context.Background()
			case "view_only":
				ctx = retouchDownloadActor(domain.PermissionAssetView, domain.AccessScopeGlobal)
			case "outside_scope":
				ctx = retouchDownloadActor(domain.PermissionAssetDownload, domain.AccessScopeSelf)
			case "not_retouch":
				row.Task.TaskType = domain.TaskTypeNewProductDevelopment
			case "final_output":
				row.Asset.AssetType = domain.TaskAssetTypeDelivery
			case "no_requirement":
				row.Asset.RetouchRequirementID = nil
			case "deleted_requirement":
				requirements.requirement = nil
			case "foreign_requirement":
				requirements.requirement.TaskID = 999
			case "foreign_task":
				row.Asset.TaskID = 999
			case "deleted":
				row.Asset.DeletedAt = &now
			case "cleaned":
				row.Asset.CleanedAt = &now
			case "incomplete":
				row.Asset.UploadStatus = strPtr("pending")
			case "missing_object":
				row.Asset.StorageKey = nil
			case "old_version_after_delete":
				newAsset := *row.Asset
				newAsset.ID++
				newAsset.DeletedAt = &now
				newRow := *row
				newRow.Asset = &newAsset
				r.versions[ids[0]] = append(r.versions[ids[0]], &newRow)
			}
			if _, err := svc.BuildBatchDownloadManifest(ctx, ids[:1]); err == nil {
				t.Fatal("batch unexpectedly granted access")
			}
			if _, err := svc.DownloadLatest(ctx, ids[0]); err == nil {
				t.Fatal("single unexpectedly granted access")
			}
		})
	}
}
