package assetworkbench

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"workflow/domain"
)

func archiveFixtureService(t *testing.T) *Service {
	t.Helper()
	repository := &downloadFileRepo{files: map[int64]*domain.AssetWorkbenchSubmissionFile{}}
	svc := NewService(Config{}, WithRepository(repository, assetWorkbenchTestTxRunner{}), WithOSSDirect(testWorkbenchOSSDirect()))
	svc.archiveCache = map[string]*archiveTempCacheEntry{}
	for i, ext := range []string{"zip", "rar", "tar", "7z"} {
		name := "fixture." + ext
		source := filepath.Join("testdata", "archive", name)
		if ext == "zip" {
			source += ".sample"
		}
		data, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		tmp := filepath.Join(t.TempDir(), name)
		if err = os.WriteFile(tmp, data, 0600); err != nil {
			t.Fatal(err)
		}
		file := &domain.AssetWorkbenchSubmissionFile{ID: int64(i + 1), OwnerUserID: 7, OriginalFilename: name, FileSize: int64(len(data)), ObjectKey: name}
		repository.files[file.ID] = file
		svc.archiveCache[archiveTempCacheKey(file)] = &archiveTempCacheEntry{tmpPath: tmp, expiresAt: time.Now().Add(time.Hour)}
	}
	return svc
}

func TestVirtualArchiveFormats(t *testing.T) {
	svc := archiveFixtureService(t)
	actor := domain.RequestActor{ID: 7, Roles: []domain.Role{domain.RoleAssetSubmitter}}
	ctx := context.Background()
	for i, ext := range []string{"zip", "rar", "tar", "7z"} {
		t.Run(ext, func(t *testing.T) {
			id := int64(i + 1)
			root, err := svc.BrowseArchiveFile(ctx, actor, id, "")
			if err != nil || root.Format != ext || len(root.Folders) != 1 || root.Folders[0].Path != "samples" || root.Folders[0].FileCount != 2 {
				t.Fatalf("root=%+v err=%v", root, err)
			}
			level, err := svc.BrowseArchiveFile(ctx, actor, id, "samples")
			if err != nil || len(level.Files) != 1 || level.Files[0].Name != "notes.txt" || len(level.Folders) != 2 {
				t.Fatalf("nested=%+v err=%v", level, err)
			}
			design, err := svc.BrowseArchiveFile(ctx, actor, id, "samples/design")
			if err != nil || len(design.Files) != 1 || design.Files[0].PreviewURL == "" {
				t.Fatalf("design=%+v err=%v", design, err)
			}
			stream, err := svc.OpenArchiveEntry(ctx, actor, id, design.Files[0].Path)
			if err != nil {
				t.Fatal(err)
			}
			defer stream.Close()
			data, readErr := io.ReadAll(stream.Body)
			if readErr != nil || int64(len(data)) != design.Files[0].FileSize || fmt.Sprintf("%x", sha256.Sum256(data)) != "d89f31bc4d17ee73367434afd8fc99b8fb612204047c41630c22ef83fabf3c4d" {
				t.Fatalf("wrong poster.png: %v", readErr)
			}
			other := actor
			other.ID = 8
			if _, err := svc.OpenArchiveEntry(ctx, other, id, design.Files[0].Path); err == nil || err.Code != domain.ErrCodePermissionDenied {
				t.Fatalf("other owner: %v", err)
			}
			if _, err := svc.OpenArchiveEntry(ctx, actor, id, "samples/missing"); err == nil || err.Code != domain.ErrCodeNotFound {
				t.Fatalf("missing entry: %v", err)
			}
		})
	}
}

func TestArchiveIndexSharedAcrossFoldersAndCallers(t *testing.T) {
	svc := archiveFixtureService(t)
	owner := domain.RequestActor{ID: 7, Roles: []domain.Role{domain.RoleAssetSubmitter}}
	other := owner
	other.ID++
	ctx := context.Background()
	type result struct {
		dir *ArchiveBrowseResult
		err *domain.AppError
	}
	results := make(chan result, 12)
	for range cap(results) {
		go func() { dir, err := svc.BrowseArchiveFile(ctx, owner, 1, ""); results <- result{dir, err} }()
	}
	var root *ArchiveBrowseResult
	for range cap(results) {
		got := <-results
		if got.err != nil || got.dir == nil {
			t.Fatal(got.err)
		}
		if root != nil && root != got.dir {
			t.Fatal("concurrent callers rebuilt the directory")
		}
		root = got.dir
	}
	file, _ := svc.repo.GetSubmissionFile(ctx, 1)
	if err := os.Remove(svc.archiveCache[archiveTempCacheKey(file)].tmpPath); err != nil {
		t.Fatal(err)
	}
	folder, err := svc.BrowseArchiveFile(ctx, owner, 1, "samples/design")
	if err != nil || len(folder.Files) != 1 {
		t.Fatalf("nested folder was not indexed: %v", err)
	}
	if _, err := svc.BrowseArchiveFile(ctx, other, 1, "samples/design"); err == nil || err.Code != domain.ErrCodePermissionDenied {
		t.Fatal("cache bypassed ownership")
	}
}

func TestArchiveStreamsShareInstanceLimit(t *testing.T) {
	svc := archiveFixtureService(t)
	actor := domain.RequestActor{ID: 7, Roles: []domain.Role{domain.RoleAssetSubmitter}}
	var streams []*ArchiveEntryStream
	for i := range cap(svc.archiveSlots) {
		stream, err := svc.OpenArchiveEntry(context.Background(), actor, int64(i+1), "samples/notes.txt")
		if err != nil {
			t.Fatal(err)
		}
		streams = append(streams, stream)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := svc.OpenArchiveEntry(ctx, actor, 1, "samples/notes.txt"); err == nil || ctx.Err() == nil {
		t.Fatal("expected pending read to stop on cancellation")
	}
	for _, stream := range streams {
		stream.Close()
	}
	stream, err := svc.OpenArchiveEntry(context.Background(), actor, 1, "samples/notes.txt")
	if err != nil {
		t.Fatal(err)
	}
	stream.Close()
}
