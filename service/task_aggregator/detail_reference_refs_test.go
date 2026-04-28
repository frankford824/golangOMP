package task_aggregator

import (
	"testing"

	"workflow/domain"
)

func TestBuildDetailReferenceFileRefsPrefersTaskDetailJSON(t *testing.T) {
	detail := &domain.TaskDetail{
		ReferenceFileRefsJSON: `[{"asset_id":"ref-1","ref_id":"ref-1","storage_key":"tasks/ref-1.png","download_url":"/v1/assets/files/tasks/ref-1.png"}]`,
	}

	refs := buildDetailReferenceFileRefs(detail, []*domain.ReferenceFileRefFlat{{RefID: "flat-ref"}})
	if len(refs) != 1 {
		t.Fatalf("refs len = %d, want 1", len(refs))
	}
	if refs[0].AssetID != "ref-1" || refs[0].StorageKey != "tasks/ref-1.png" || refs[0].DownloadURL == nil {
		t.Fatalf("refs[0] = %+v, want formal ref object from task_detail JSON", refs[0])
	}
}

func TestBuildDetailReferenceFileRefsFallsBackToFlatRefs(t *testing.T) {
	refs := buildDetailReferenceFileRefs(&domain.TaskDetail{ReferenceFileRefsJSON: "[]"}, []*domain.ReferenceFileRefFlat{{RefID: "flat-ref"}})
	if len(refs) != 1 {
		t.Fatalf("refs len = %d, want 1", len(refs))
	}
	if refs[0].AssetID != "flat-ref" || refs[0].RefID != "flat-ref" {
		t.Fatalf("refs[0] = %+v, want flat-ref fallback", refs[0])
	}
}
