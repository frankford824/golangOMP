package service

import (
	"context"
	"encoding/json"
	"testing"
	"workflow/domain"
	"workflow/repo"
)

type specificationStoreStub struct {
	repo.TaskCostSpecificationRepo
	row    *domain.TaskCostSpecification
	saves  int
	filter repo.TaskCostSpecificationFilter
}

func (s *specificationStoreStub) List(_ context.Context, f repo.TaskCostSpecificationFilter) ([]*domain.TaskCostSpecification, int64, error) {
	s.filter = f
	copy := *s.row
	return []*domain.TaskCostSpecification{&copy}, 1, nil
}
func (s *specificationStoreStub) Lock(context.Context, repo.Tx, int64, int64) (*domain.TaskCostSpecification, error) {
	copy := *s.row
	return &copy, nil
}
func (s *specificationStoreStub) Save(_ context.Context, _ repo.Tx, _ *domain.TaskCostSpecification, input domain.CostInput, area float64) error {
	s.saves++
	raw, _ := json.Marshal(map[string]interface{}{"cost_input": input})
	s.row.VariantJSON = raw
	s.row.AreaM2 = &area
	s.row.CostReviewRequired = true
	return nil
}

func TestCompletedSpecificationCorrectionIsOwnScopedAndKeepsPrice(t *testing.T) {
	for _, test := range []struct {
		name   string
		actor  domain.RequestActor
		status domain.TaskStatus
		want   bool
	}{
		{"own creator after completion", specificationActor(7, domain.PermissionTaskCreate, domain.PermissionTaskView), domain.TaskStatusCompleted, true},
		{"another creator with global create is denied", specificationActor(8, domain.PermissionTaskCreate, domain.PermissionTaskView), domain.TaskStatusCompleted, false},
		{"catalog manager remains scoped", specificationActor(8, domain.PermissionCatalogManage, domain.PermissionTaskView), domain.TaskStatusCompleted, true},
		{"read only is denied", specificationActor(7, domain.PermissionTaskView), domain.TaskStatusCompleted, false},
		{"archived remains locked", specificationActor(7, domain.PermissionTaskCreate, domain.PermissionTaskView), domain.TaskStatusArchived, false},
		{"cancelled remains locked", specificationActor(7, domain.PermissionTaskCreate, domain.PermissionTaskView), domain.TaskStatusCancelled, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := &specificationStoreStub{row: &domain.TaskCostSpecification{TaskID: 10, SKUItemID: 20, SKUCode: "QA-1", CreatorID: 7, TaskStatus: test.status, TaskType: domain.TaskTypeNewProductDevelopment, RequiresArea: true, CostPrice: float64Ptr(10), VariantJSON: json.RawMessage(`{}`)}}
			before := *store.row
			hydrateCostSpecification(&before, test.actor)
			events := &finalizerEventRepo{}
			svc := &taskService{costSpecifications: store, txRunner: noopTxRunner{}, taskEventRepo: events, taskRepo: &prdTaskRepo{tasks: map[int64]*domain.Task{10: {ID: 10, CreatorID: 7, TaskStatus: test.status, TaskType: domain.TaskTypeNewProductDevelopment}}}}
			result, appErr := svc.CorrectCostSpecification(domain.WithRequestActor(context.Background(), test.actor), 10, domain.TaskCostSpecificationUpdate{SKUItemID: 20, Token: before.Token, Reason: "补录面积", CostInput: domain.CostInput{AreaMode: "total", AreaM2: .5, Pieces: 1}})
			if !test.want {
				if appErr == nil || appErr.Code != domain.ErrCodePermissionDenied || store.saves != 0 {
					t.Fatalf("unauthorized correction: %v %d", appErr, store.saves)
				}
				return
			}
			if appErr != nil || store.saves != 1 || result.Data[0].MissingArea || !result.Data[0].CostReviewRequired || *result.Data[0].CostPrice != 10 || result.Data[0].TaskStatus != test.status {
				t.Fatalf("correction changed price/state or left missing: %+v %v", result, appErr)
			}
			if len(events.eventTypes) != 1 || events.eventTypes[0] != "task.cost_specification_updated" {
				t.Fatal("missing audit event")
			}
		})
	}
}

func TestSpecificationRejectsStaleAndInvalidArea(t *testing.T) {
	actor := specificationActor(7, domain.PermissionTaskCreate, domain.PermissionTaskView)
	row := &domain.TaskCostSpecification{TaskID: 10, SKUItemID: 20, CreatorID: 7, TaskStatus: domain.TaskStatusCompleted, TaskType: domain.TaskTypeNewProductDevelopment, RequiresArea: true, VariantJSON: json.RawMessage(`{}`)}
	hydrateCostSpecification(row, actor)
	for _, p := range []domain.TaskCostSpecificationUpdate{
		{SKUItemID: 20, Token: "stale", Reason: "补录面积", CostInput: domain.CostInput{AreaMode: "total", AreaM2: .5, Pieces: 1}},
		{SKUItemID: 20, Token: row.Token, Reason: "补录面积", CostInput: domain.CostInput{AreaMode: "total", Pieces: 1}},
		{SKUItemID: 20, Token: row.Token, Reason: "", CostInput: domain.CostInput{AreaMode: "total", AreaM2: .5, Pieces: 1}},
	} {
		store := &specificationStoreStub{row: row}
		svc := &taskService{costSpecifications: store, txRunner: noopTxRunner{}}
		_, appErr := svc.CorrectCostSpecification(domain.WithRequestActor(context.Background(), actor), 10, p)
		if appErr == nil || store.saves != 0 {
			t.Fatal("invalid input committed")
		}
	}
}

func TestSpecificationQueueAlwaysUsesCreatorScope(t *testing.T) {
	actor := specificationActor(7, domain.PermissionTaskCreate, domain.PermissionTaskView)
	store := &specificationStoreStub{row: &domain.TaskCostSpecification{TaskID: 10, TaskType: domain.TaskTypeNewProductDevelopment}}
	svc := &taskService{costSpecifications: store}
	_, appErr := svc.ListCostSpecifications(domain.WithRequestActor(context.Background(), actor), 0, 1, 20)
	if appErr != nil || store.filter.CreatorID != 7 || !store.filter.MissingOnly || store.filter.PageSize != 20 {
		t.Fatalf("queue lost creator scope: %+v %v", store.filter, appErr)
	}
}

func specificationActor(id int64, permissions ...domain.PermissionCode) domain.RequestActor {
	actor := globalCapabilityActor(id, permissions[0])
	for _, permission := range permissions[1:] {
		actor.Permissions = append(actor.Permissions, permission)
		actor.EffectiveAccess.Permissions = append(actor.EffectiveAccess.Permissions, permission)
		source := globalCapabilityActor(id, permission).EffectiveAccess.Sources[0]
		actor.EffectiveAccess.Sources = append(actor.EffectiveAccess.Sources, source)
	}
	return actor
}

func TestOwnCompletedSpecificationStillRequiresCurrentOrganizationScope(t *testing.T) {
	actor := specificationActor(7, domain.PermissionTaskCreate, domain.PermissionTaskView)
	one, two := int64(1), int64(2)
	actor.DepartmentID = &one
	for i := range actor.EffectiveAccess.Assignments {
		actor.EffectiveAccess.Assignments[i].ScopeMode = domain.AccessScopeOwnDepartment
	}
	row := &domain.TaskCostSpecification{TaskID: 10, CreatorID: 7, TaskStatus: domain.TaskStatusCompleted, TaskType: domain.TaskTypeNewProductDevelopment, OwnerDepartmentID: &two}
	if canCorrectCostSpecification(actor, row) {
		t.Fatal("creator bypassed current organization scope")
	}
}
