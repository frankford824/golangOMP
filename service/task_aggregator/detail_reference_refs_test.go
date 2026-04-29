package task_aggregator

import (
	"context"
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

func TestBuildDetailEnrichesActorNamesAndDesignWorkflow(t *testing.T) {
	designerID := int64(203)
	task := &domain.Task{
		ID:               606,
		TaskType:         domain.TaskTypeNewProductDevelopment,
		TaskStatus:       domain.TaskStatusInProgress,
		CreatorID:        1,
		DesignerID:       &designerID,
		CurrentHandlerID: &designerID,
	}
	svc := &DetailService{nameResolver: detailNameResolverStub{names: map[int64]string{1: "系统管理员", 203: "设计测试账号2"}}}

	detail := svc.buildDetail(context.Background(), task, &domain.TaskDetail{}, []*domain.TaskModule{{
		ID:        1,
		TaskID:    606,
		ModuleKey: domain.ModuleKeyDesign,
		State:     domain.ModuleStateInProgress,
		ClaimedBy: &designerID,
	}}, nil, nil)

	if detail.DesignerName != "设计测试账号2" || detail.AssigneeName != "设计测试账号2" {
		t.Fatalf("designer/assignee names = %q/%q, want 设计测试账号2", detail.DesignerName, detail.AssigneeName)
	}
	if detail.DesignSubStatus != string(domain.TaskSubStatusInProgress) {
		t.Fatalf("design_sub_status = %q, want in_progress", detail.DesignSubStatus)
	}
	if detail.Workflow.SubStatus.Design.Code != domain.TaskSubStatusInProgress {
		t.Fatalf("workflow.sub_status.design = %+v, want in_progress", detail.Workflow.SubStatus.Design)
	}
}

type detailNameResolverStub struct {
	names map[int64]string
}

func (r detailNameResolverStub) GetDisplayName(_ context.Context, id int64) string {
	return r.names[id]
}
