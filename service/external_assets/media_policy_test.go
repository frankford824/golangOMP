package externalassets

import (
	"context"
	"testing"
	"workflow/config"
	"workflow/domain"
)

func TestQuarkPolicyCoversOldIDsWithoutDisablingP3Names(t *testing.T) {
	r := &externalAssetRepoStub{getRow: &domain.ExternalAssetRecord{ID: 7, MountPath: "/quark", OriginPath: "/quark/old.psd", Kind: domain.ExternalAssetKindNetdisk, Status: domain.ExternalAssetStatusIndexed}}
	s := NewService(r, Config{Enabled: true, Mounts: []MountConfig{{Path: "/quark", Kind: domain.ExternalAssetKindNetdisk}, {Path: "/p3", Kind: domain.ExternalAssetKindNASLocal}}, OSSRequiredPrefixes: []string{"/p3/仓库素材区/徐凯"}, SyncExportRoots: []string{"/p3/仓库素材区/徐凯"}}, nil)
	s.ConfigureMedia(nil, config.AssetMediaConfig{QuarkDisabled: true, NASScanEnabled: true})
	for _, entry := range []func(context.Context, int64) (*domain.AssetDownloadInfo, *domain.AppError){s.DownloadInfo, s.BatchDownloadInfo, func(ctx context.Context, id int64) (*domain.AssetDownloadInfo, *domain.AppError) {
		return s.PreviewInfo(ctx, id)
	}} {
		if info, err := entry(context.Background(), 7); info != nil || err == nil || err.Code != "source_disabled" {
			t.Fatalf("old source remained available: %+v %v", info, err)
		}
	}
	if target, err := s.ResolveNetdiskStream(context.Background(), 7); target != nil || err == nil || err.Code != "source_disabled" {
		t.Fatalf("byte stream bypass: %+v %v", target, err)
	}
	if err := s.sourcePolicyError(&domain.ExternalAssetRecord{MountPath: "/p3", OriginPath: "/p3/定制/夸克导入文件.psd"}); err != nil {
		t.Fatal("filename text must not disable a NAS source")
	}
	if len(s.cfg.OSSRequiredPrefixes) != 1 || s.cfg.OSSRequiredPrefixes[0] != "/p3/仓库素材区/徐凯" || len(s.cfg.SyncExportRoots) != 1 || s.cfg.SyncExportRoots[0] != "/p3/仓库素材区/徐凯" {
		t.Fatal("index expansion changed upload/export scope")
	}
	if len(s.cfg.VisibleRoots) != 1 || s.cfg.VisibleRoots[0] != "/p3" || s.cfg.FullSyncEnabled {
		t.Fatal("source policy did not transfer index authority")
	}
}
