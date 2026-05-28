package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type retouchRequirementRepoStub struct {
	byTask map[int64][]*domain.TaskRetouchRequirement
	nextID int64
}

func (s *retouchRequirementRepoStub) CreateBatch(_ context.Context, _ repo.Tx, taskID int64, createdBy int64, items []domain.CreateRetouchRequirementItem) error {
	if s.byTask == nil {
		s.byTask = map[int64][]*domain.TaskRetouchRequirement{}
	}
	now := time.Now().UTC()
	for _, item := range items {
		s.nextID++
		row := &domain.TaskRetouchRequirement{
			ID:          s.nextID,
			TaskID:      taskID,
			Description: item.Description,
			SKUCode:     item.SKUCode,
			Spec:        item.Spec,
			Remark:      item.Remark,
			SortOrder:   item.SortOrder,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if createdBy > 0 {
			row.CreatedBy = &createdBy
			row.UpdatedBy = &createdBy
		}
		s.byTask[taskID] = append(s.byTask[taskID], row)
	}
	return nil
}

func (s *retouchRequirementRepoStub) GetByID(_ context.Context, id int64) (*domain.TaskRetouchRequirement, error) {
	if s.byTask == nil {
		return nil, nil
	}
	for _, rows := range s.byTask {
		for _, row := range rows {
			if row != nil && row.ID == id {
				return row, nil
			}
		}
	}
	return nil, nil
}

func (s *retouchRequirementRepoStub) ListByTaskID(_ context.Context, taskID int64) ([]*domain.TaskRetouchRequirement, error) {
	if s.byTask == nil {
		return []*domain.TaskRetouchRequirement{}, nil
	}
	rows := s.byTask[taskID]
	if len(rows) == 0 {
		return []*domain.TaskRetouchRequirement{}, nil
	}
	out := make([]*domain.TaskRetouchRequirement, len(rows))
	copy(out, rows)
	return out, nil
}

func newRetouchTaskServiceTest(t *testing.T, retouchRepo repo.TaskRetouchRequirementRepo) TaskService {
	t.Helper()
	return NewTaskService(
		&prdTaskRepo{},
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		productCodeTestTxRunner{},
		WithTaskRetouchRequirementRepo(retouchRepo),
	)
}

func taskCreateErrorHasViolationCode(appErr *domain.AppError, code string) bool {
	if appErr == nil || appErr.Details == nil {
		return false
	}
	details, ok := appErr.Details.(map[string]interface{})
	if !ok {
		return false
	}
	raw, ok := details["violations"].([]map[string]interface{})
	if !ok {
		if anySlice, ok := details["violations"].([]interface{}); ok {
			for _, item := range anySlice {
				v, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				if violationCode, _ := v["code"].(string); violationCode == code {
					return true
				}
			}
		}
		return false
	}
	for _, v := range raw {
		if violationCode, _ := v["code"].(string); violationCode == code {
			return true
		}
	}
	return false
}

func TestValidateRetouchRequirementsRejectsNonRetouchTask(t *testing.T) {
	appErr := validateRetouchRequirements(CreateTaskParams{
		TaskType: domain.TaskTypeNewProductDevelopment,
		RetouchRequirements: []domain.CreateRetouchRequirementItem{
			{Description: "should not be here"},
		},
	})
	if appErr == nil {
		t.Fatal("validateRetouchRequirements() expected error")
	}
	if appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("error code = %s, want %s", appErr.Code, domain.ErrCodeInvalidRequest)
	}
	if !strings.Contains(appErr.Message, "retouch_requirements is only supported for retouch_task") {
		t.Fatalf("error message = %q", appErr.Message)
	}
	if !taskCreateErrorHasViolationCode(appErr, "field_not_allowed_for_task_type") {
		t.Fatalf("violations = %#v, want field_not_allowed_for_task_type", appErr.Details)
	}
}

func TestTaskServiceCreateRetouchTaskPersistsAndReturnsRequirements(t *testing.T) {
	retouchRepo := &retouchRequirementRepoStub{}
	svc := newRetouchTaskServiceTest(t, retouchRepo)

	created, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:            domain.TaskTypeRetouchTask,
		SourceMode:          domain.TaskSourceModeNewProduct,
		CreatorID:           9,
		OwnerTeam:           domain.AllValidTeams()[0],
		DeadlineAt:          timePtr(),
		ProductNameSnapshot: "Retouch bundle",
		DesignRequirement:   "legacy text still allowed",
		RetouchRequirements: []domain.CreateRetouchRequirementItem{
			{Description: "需求一", SKUCode: "SKU-A", SortOrder: 1},
			{Description: "需求二", Remark: "备注", SortOrder: 2},
		},
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v", appErr)
	}
	if len(retouchRepo.byTask[created.ID]) != 2 {
		t.Fatalf("persisted requirements = %d, want 2", len(retouchRepo.byTask[created.ID]))
	}

	readModel, appErr := svc.GetByID(context.Background(), created.ID)
	if appErr != nil {
		t.Fatalf("GetByID() unexpected error: %+v", appErr)
	}
	if len(readModel.RetouchRequirements) != 2 {
		t.Fatalf("GetByID() retouch_requirements len = %d, want 2", len(readModel.RetouchRequirements))
	}
	if readModel.RetouchRequirements[0].Description != "需求一" || readModel.RetouchRequirements[1].Description != "需求二" {
		t.Fatalf("GetByID() retouch_requirements = %+v", readModel.RetouchRequirements)
	}
}

func TestTaskServiceGetRetouchTaskWithoutRequirementsReturnsEmptyArray(t *testing.T) {
	retouchRepo := &retouchRequirementRepoStub{}
	svc := newRetouchTaskServiceTest(t, retouchRepo)

	created, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:            domain.TaskTypeRetouchTask,
		SourceMode:          domain.TaskSourceModeNewProduct,
		CreatorID:           9,
		OwnerTeam:           domain.AllValidTeams()[0],
		DeadlineAt:          timePtr(),
		ProductNameSnapshot: "Legacy retouch",
		DesignRequirement:   "only legacy text",
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v", appErr)
	}

	readModel, appErr := svc.GetByID(context.Background(), created.ID)
	if appErr != nil {
		t.Fatalf("GetByID() unexpected error: %+v", appErr)
	}
	if readModel.RetouchRequirements == nil || len(readModel.RetouchRequirements) != 0 {
		t.Fatalf("GetByID() retouch_requirements = %#v, want empty slice", readModel.RetouchRequirements)
	}
}

func TestTaskServiceCreateRejectsRetouchRequirementsForNewProduct(t *testing.T) {
	svc := newRetouchTaskServiceTest(t, &retouchRequirementRepoStub{})

	_, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:            domain.TaskTypeNewProductDevelopment,
		SourceMode:          domain.TaskSourceModeNewProduct,
		CreatorID:           9,
		OwnerTeam:           domain.AllValidTeams()[0],
		DeadlineAt:          timePtr(),
		CategoryCode:        "LIGHTBOX",
		MaterialMode:        string(domain.MaterialModePreset),
		Material:            "铝型材",
		ProductNameSnapshot: "New Lightbox",
		ProductShortName:    "Lightbox",
		DesignRequirement:   "need design",
		RetouchRequirements: []domain.CreateRetouchRequirementItem{
			{Description: "invalid"},
		},
	})
	if appErr == nil {
		t.Fatal("Create() expected validation error")
	}
	if appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("Create() error code = %s, want %s", appErr.Code, domain.ErrCodeInvalidRequest)
	}
	if !taskCreateErrorHasViolationCode(appErr, "field_not_allowed_for_task_type") {
		t.Fatalf("Create() violations = %#v, want field_not_allowed_for_task_type", appErr.Details)
	}
}
