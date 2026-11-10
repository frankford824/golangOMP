package assetworkbench

import (
	"context"
	"testing"
	"time"

	"workflow/domain"
)

func TestPSBUploadUsesDesignPreviewPipeline(t *testing.T) {
	repository := &uploadDirectorySessionRepo{directories: []*domain.AssetWorkbenchUploadDirectory{
		{ID: 11, Name: "成品", OSSPrefix: "finished", Enabled: true, AllowedFileTypes: []string{"psb"}},
	}}
	svc := NewService(Config{},
		WithRepository(repository, assetWorkbenchTestTxRunner{}),
		WithOSSDirect(testWorkbenchOSSDirect()),
	)
	actor := domain.RequestActor{ID: 77, Roles: []domain.Role{domain.RoleAssetSubmitter}}
	for _, mimeType := range []string{"application/octet-stream", "image/vnd.adobe.photoshop"} {
		t.Run(mimeType, func(t *testing.T) {
			result, err := svc.CreateUploadSession(context.Background(), actor, CreateUploadSessionParams{
				OriginalFilename:  "大图.PSB",
				FileSize:          128,
				MimeType:          mimeType,
				UploadDirectoryID: 11,
			})
			if err != nil {
				t.Fatal(err)
			}
			file := submissionFileFromUploadSession(100, 200, actor.ID, result.Session, 0, time.Now())
			if file.FileType != "design" || file.FileExt != "psb" || file.PreviewStatus != domain.AssetWorkbenchPreviewStatusPending || file.OriginalFilename != "大图.PSB" {
				t.Fatalf("incorrect PSB file metadata: %+v", file)
			}
		})
	}
}

type psbPreviewRepo struct {
	downloadFileRepo
	queuedFileID int64
	queuedObject string
}

func (r *psbPreviewRepo) RequeueOriginalPreview(_ context.Context, fileID int64, objectKey string) error {
	r.queuedFileID, r.queuedObject = fileID, objectKey
	return nil
}

func TestLegacyPSBPreviewQueuesOnlyForAuthorizedViewer(t *testing.T) {
	r := &psbPreviewRepo{downloadFileRepo: downloadFileRepo{files: map[int64]*domain.AssetWorkbenchSubmissionFile{
		10: {
			ID: 10, OwnerUserID: 77, OriginalFilename: "poster.PSB", ObjectKey: "uploads/poster.PSB",
			FileType: "psb", PreviewStatus: domain.AssetWorkbenchPreviewStatusNotApplicable,
		},
	}}}
	svc := NewService(Config{}, WithRepository(r, assetWorkbenchTestTxRunner{}), WithOSSDirect(testWorkbenchOSSDirect()))
	actor := domain.RequestActor{ID: 88, Roles: []domain.Role{domain.RoleAssetSubmitter}}
	if _, err := svc.GetFilePreview(context.Background(), actor, 10); err == nil || err.Code != domain.ErrCodePermissionDenied || r.queuedFileID != 0 {
		t.Fatalf("unauthorized preview queued a file or returned wrong error: queued=%d error=%v", r.queuedFileID, err)
	}
	actor.ID = 77
	meta, err := svc.GetFilePreview(context.Background(), actor, 10)
	if err != nil || meta == nil || !meta.Preparing || meta.PreviewURL != "" || meta.DownloadURL != "" || r.queuedFileID != 10 || r.queuedObject != "uploads/poster.PSB" {
		t.Fatalf("legacy PSB should queue a derived preview: meta=%+v error=%v", meta, err)
	}
}
