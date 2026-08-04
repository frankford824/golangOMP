package service

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"io"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"

	"workflow/domain"
	"workflow/repo"
)

type planningResultRepoStub struct {
	PlanningSKURepository
	result *domain.PlanningSKUCreateResult
	rows   []domain.PlanningSKUExportRow
}

func (s *planningResultRepoStub) GetTaskAccessSubject(context.Context, int64) (domain.TaskAccessSubject, error) {
	return domain.TaskAccessSubject{TaskID: 42, CreatorID: 99, TaskType: domain.TaskTypeSKUPlanning}, nil
}

func (s *planningResultRepoStub) LoadCreateResult(context.Context, int64) (*domain.PlanningSKUCreateResult, error) {
	return s.result, nil
}

func (s *planningResultRepoStub) ListExportRows(context.Context, []int64, []int64) ([]domain.PlanningSKUExportRow, error) {
	return s.rows, nil
}

type planningStorageRefRepoStub struct {
	repo.AssetStorageRefRepo
	ref *domain.AssetStorageRef
}

func (s *planningStorageRefRepoStub) GetByRefID(context.Context, string) (*domain.AssetStorageRef, error) {
	return s.ref, nil
}

type planningImageStreamStub struct{ payload []byte }

func (s planningImageStreamStub) Open(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(s.payload)), nil
}

func planningReadActor() domain.RequestActor {
	permissions := []domain.PermissionCode{domain.PermissionPlanningSKUView, domain.PermissionPlanningSKUExport}
	return domain.RequestActor{
		ID:          99,
		Permissions: permissions,
		EffectiveAccess: &domain.EffectiveAccess{
			Permissions: permissions,
			Assignments: []domain.AccessAssignment{{RoleID: 1, ScopeMode: domain.AccessScopeGlobal}},
			Sources: []domain.EffectiveAccessNote{
				{RoleID: 1, Permission: domain.PermissionPlanningSKUView},
				{RoleID: 1, Permission: domain.PermissionPlanningSKUExport},
			},
		},
	}
}

func planningImageFixture(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.NRGBA{R: 255, A: 255})
	img.Set(1, 0, color.NRGBA{G: 255, A: 255})
	img.Set(0, 1, color.NRGBA{B: 255, A: 255})
	img.Set(1, 1, color.NRGBA{R: 255, G: 255, A: 255})
	var payload bytes.Buffer
	if err := png.Encode(&payload, img); err != nil {
		t.Fatal(err)
	}
	return payload.Bytes()
}

func TestPlanningSKUGetResultPresignsRevisionImage(t *testing.T) {
	result := &domain.PlanningSKUCreateResult{
		TaskID: 42,
		Items: []domain.PlanningSKUResultItem{{
			TaskSKUItemID: 7,
			SKUCode:       "PLAN-001",
			Revision:      &domain.PlanningSKURevision{ID: 51, ProductImageRefID: "ref-image"},
		}},
	}
	size := int64(len(planningImageFixture(t)))
	storageRef := &domain.AssetStorageRef{
		RefID: "ref-image", OwnerType: domain.AssetOwnerTypePlanningSKURevision, OwnerID: 51,
		StorageAdapter: domain.AssetStorageAdapterOSSUploadService, RefType: domain.AssetStorageRefTypeGenericObject,
		RefKey: "tasks/planning/image.png", FileName: "image.png", MimeType: "image/png",
		FileSize: &size, Status: domain.AssetStorageRefStatusRecorded,
	}
	oss := NewOSSDirectService(OSSDirectConfig{
		Enabled: true, Endpoint: "oss-cn-hangzhou.aliyuncs.com", Bucket: "cloneb-private",
		AccessKeyID: "test-ak", AccessKeySecret: "test-secret",
	})
	svc := &planningSKUService{
		repo:        &planningResultRepoStub{result: result},
		storageRefs: &planningStorageRefRepoStub{ref: storageRef},
		ossDirect:   oss,
	}

	got, appErr := svc.GetResult(context.Background(), planningReadActor(), 42)
	if appErr != nil {
		t.Fatalf("GetResult() error = %+v", appErr)
	}
	revision := got.Items[0].Revision
	if revision.ProductImageName != "image.png" || revision.ProductImageURL == "" {
		t.Fatalf("revision image projection = %+v", revision)
	}
}

func TestPlanningSKUExportEmbedsRevisionImage(t *testing.T) {
	payload := planningImageFixture(t)
	size := int64(len(payload))
	storageRef := &domain.AssetStorageRef{
		RefID: "ref-image", OwnerType: domain.AssetOwnerTypePlanningSKURevision, OwnerID: 51,
		StorageAdapter: domain.AssetStorageAdapterOSSUploadService, RefType: domain.AssetStorageRefTypeGenericObject,
		RefKey: "tasks/planning/image.png", FileName: "image.png", MimeType: "image/png",
		FileSize: &size, Status: domain.AssetStorageRefStatusRecorded,
	}
	svc := &planningSKUService{
		repo: &planningResultRepoStub{rows: []domain.PlanningSKUExportRow{{
			TaskID: 42, TaskNo: "RW-42", SequenceNo: 1, TaskSKUItemID: 7, SKUCode: "PLAN-001",
			ImageRefID: "ref-image", DescriptionSpec: "红色礼盒", Quantity: 20, CreatorName: "运营甲",
		}}},
		storageRefs: &planningStorageRefRepoStub{ref: storageRef},
		streams:     planningImageStreamStub{payload: payload},
		now:         func() time.Time { return time.Date(2026, 8, 4, 10, 0, 0, 0, time.UTC) },
	}

	content, _, appErr := svc.Export(context.Background(), planningReadActor(), domain.PlanningSKUExportRequest{TaskIDs: []int64{42}})
	if appErr != nil {
		t.Fatalf("Export() error = %+v", appErr)
	}
	workbook, err := excelize.OpenReader(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("open exported workbook: %v", err)
	}
	defer workbook.Close()
	pictures, err := workbook.GetPictures("策划SKU", "D2")
	if err != nil {
		t.Fatalf("GetPictures() error = %v", err)
	}
	if len(pictures) != 1 {
		t.Fatalf("embedded pictures = %d, want 1", len(pictures))
	}
}
