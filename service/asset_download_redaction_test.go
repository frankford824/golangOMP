package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"workflow/domain"
)

func TestTaskAssetDownloadAllowedUsesExactTaskScope(t *testing.T) {
	const actorID int64 = 23
	task := &domain.Task{
		ID:        90,
		CreatorID: actorID,
		TaskType:  domain.TaskTypeNewProductDevelopment,
	}
	allowed := domain.WithRequestActor(
		context.Background(),
		taskActionTestActor(actorID, domain.PermissionAssetDownload, domain.AccessScopeSelf),
	)
	if !TaskAssetDownloadAllowed(allowed, task) {
		t.Fatal("TaskAssetDownloadAllowed() = false, want true for self-scoped asset.download")
	}

	task.CreatorID = 999
	if TaskAssetDownloadAllowed(allowed, task) {
		t.Fatal("TaskAssetDownloadAllowed() = true outside the granted task scope")
	}
	if !TaskAssetDownloadAllowed(context.Background(), task) {
		t.Fatal("TaskAssetDownloadAllowed() = false for a trusted internal call without a request actor")
	}
}

func TestRedactTaskReadModelDownloadsCopiesNestedValues(t *testing.T) {
	downloadURL := "https://objects.example/presigned?signature=secret"
	expiresAt := time.Now().UTC().Add(time.Minute)
	readModel := &domain.TaskReadModel{
		ReferenceFileRefs: []domain.ReferenceFileRef{{
			AssetID:              "task-ref",
			URL:                  &downloadURL,
			DownloadURL:          &downloadURL,
			StorageKey:           "tasks/90/task-ref.png",
			DownloadURLExpiresAt: &expiresAt,
		}},
		SKUItems: []*domain.TaskSKUItem{{
			ID: 4,
			ReferenceFileRefs: []domain.ReferenceFileRef{{
				AssetID:     "sku-ref",
				DownloadURL: &downloadURL,
				StorageKey:  "tasks/90/sku-ref.png",
			}},
		}},
		RetouchRequirements: []domain.TaskRetouchRequirement{{
			ID: 5,
			ReferenceFileRefs: []domain.ReferenceFileRef{{
				AssetID:     "retouch-ref",
				DownloadURL: &downloadURL,
				StorageKey:  "tasks/90/retouch-ref.png",
			}},
			SourceAssets: []*domain.DesignAsset{{
				ID: 6,
				CurrentVersion: &domain.DesignAssetVersion{
					ID:                    7,
					StorageKey:            "tasks/90/source.psd",
					DownloadURL:           &downloadURL,
					PublicDownloadAllowed: true,
					AccessHint:            "download",
				},
			}},
		}},
	}
	originalSKU := readModel.SKUItems[0]
	originalSource := readModel.RetouchRequirements[0].SourceAssets[0]

	RedactTaskReadModelDownloads(readModel)

	assertReferenceRedacted(t, readModel.ReferenceFileRefs[0])
	assertReferenceRedacted(t, readModel.SKUItems[0].ReferenceFileRefs[0])
	assertReferenceRedacted(t, readModel.RetouchRequirements[0].ReferenceFileRefs[0])
	version := readModel.RetouchRequirements[0].SourceAssets[0].CurrentVersion
	if version.DownloadURL != nil || version.StorageKey != "" || version.PublicDownloadAllowed || version.AccessHint != "" {
		t.Fatalf("source version not redacted: %+v", version)
	}
	encodedVersion, err := json.Marshal(version)
	if err != nil {
		t.Fatalf("json.Marshal(redacted version) error = %v", err)
	}
	if strings.Contains(string(encodedVersion), "download_url") ||
		strings.Contains(string(encodedVersion), "storage_key") {
		t.Fatalf("redacted version JSON leaked a locator field: %s", encodedVersion)
	}
	if readModel.SKUItems[0] == originalSKU || readModel.RetouchRequirements[0].SourceAssets[0] == originalSource {
		t.Fatal("redaction reused nested repository-backed pointers")
	}
	if originalSKU.ReferenceFileRefs[0].DownloadURL == nil || originalSource.CurrentVersion.DownloadURL == nil {
		t.Fatal("redaction mutated the original nested values")
	}
}

func TestRedactTaskEventDownloadsRemovesAssetFieldsOnly(t *testing.T) {
	original := &domain.TaskEvent{
		ID: "event-1",
		Payload: json.RawMessage(`{
			"reference_file_refs":[{
				"asset_id":"ref-1",
				"storage_key":"tasks/90/ref.png",
				"file_path":"/mnt/private/tasks/90/ref.png",
				"object_key":"tasks/90/ref.png",
				"signed_url":"https://objects.example/ref?signed=secret",
				"presigned_url":"https://objects.example/ref?presigned=secret",
				"url":"https://objects.example/ref",
				"download_url":"https://objects.example/ref?signature=secret",
				"download_url_expires_at":"2026-07-27T00:00:00Z"
			}],
			"documentation_url":"https://docs.example/keep"
		}`),
	}

	events := RedactTaskEventDownloads([]*domain.TaskEvent{original})
	if len(events) != 1 || events[0] == original {
		t.Fatalf("RedactTaskEventDownloads() = %+v, want a copied event", events)
	}
	payload := string(events[0].Payload)
	for _, forbidden := range []string{
		"download_url",
		"storage_key",
		"file_path",
		"object_key",
		"signed_url",
		"presigned_url",
		"signature=secret",
		"signed=secret",
		"presigned=secret",
	} {
		if strings.Contains(payload, forbidden) {
			t.Fatalf("redacted payload still contains %q: %s", forbidden, payload)
		}
	}
	if !strings.Contains(payload, "https://docs.example/keep") {
		t.Fatalf("redaction removed an unrelated URL: %s", payload)
	}
	if !strings.Contains(string(original.Payload), "signature=secret") {
		t.Fatal("redaction mutated the stored event payload")
	}
}

func TestRedactAssetDownloadJSONFailsClosedForInvalidHistory(t *testing.T) {
	got := RedactAssetDownloadJSON(json.RawMessage(`{"download_url":"secret"`))
	if string(got) != "{}" {
		t.Fatalf("RedactAssetDownloadJSON(invalid) = %s, want {}", got)
	}
}

func TestRedactLegacyReferenceImagesJSONKeepsIdentifiersAndDropsLocators(t *testing.T) {
	got := RedactLegacyReferenceImagesJSON(`[
		"ref-123",
		"poster final.png",
		"https://objects.example/tasks/90/ref.png?signature=secret",
		"data:image/png;base64,c2VjcmV0",
		"tasks/90/ref.png",
		"tasks%2F90%2Fencoded.png",
		"C:\\assets\\ref.png",
		"/v1/assets/files/tasks/90/ref.png"
	]`)
	if got != `["ref-123","poster final.png"]` {
		t.Fatalf("RedactLegacyReferenceImagesJSON() = %s, want safe identifiers only", got)
	}
}

func TestRedactLegacyReferenceImagesJSONFailsClosedForAmbiguousShape(t *testing.T) {
	for _, raw := range []string{
		`{"url":"https://objects.example/ref.png"}`,
		`[{"asset_id":"ref-1","url":"https://objects.example/ref.png"}]`,
		`not-json`,
	} {
		if got := RedactLegacyReferenceImagesJSON(raw); got != "[]" {
			t.Fatalf("RedactLegacyReferenceImagesJSON(%q) = %s, want []", raw, got)
		}
	}
}

func assertReferenceRedacted(t *testing.T, ref domain.ReferenceFileRef) {
	t.Helper()
	if ref.URL != nil || ref.DownloadURL != nil || ref.StorageKey != "" || ref.DownloadURLExpiresAt != nil {
		t.Fatalf("reference not redacted: %+v", ref)
	}
}
