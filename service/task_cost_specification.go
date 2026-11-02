package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"strings"
	"unicode/utf8"
	"workflow/domain"
	"workflow/repo"
)

type TaskCostSpecificationService interface {
	ListCostSpecifications(context.Context, int64, int, int) (*domain.TaskCostSpecificationList, *domain.AppError)
	CorrectCostSpecification(context.Context, int64, domain.TaskCostSpecificationUpdate) (*domain.TaskCostSpecificationList, *domain.AppError)
}

func WithTaskCostSpecificationRepo(repository repo.TaskCostSpecificationRepo) TaskServiceOption {
	return func(s *taskService) { s.costSpecifications = repository }
}
func costSpecificationSubject(r *domain.TaskCostSpecification) domain.TaskAccessSubject {
	return domain.TaskAccessSubject{TaskID: r.TaskID, CreatorID: r.CreatorID, RequesterID: r.RequesterID, DesignerID: r.DesignerID, CurrentHandlerID: r.CurrentHandlerID, OwnerDepartmentID: r.OwnerDepartmentID, OwnerTeamID: r.OwnerTeamID, TaskType: r.TaskType}
}
func canCorrectCostSpecification(actor domain.RequestActor, r *domain.TaskCostSpecification) bool {
	if r.TaskType != domain.TaskTypeOriginalProductDevelopment && r.TaskType != domain.TaskTypeNewProductDevelopment && r.TaskType != domain.TaskTypeRetouchTask {
		return false
	}
	if r.TaskStatus == domain.TaskStatusArchived || r.TaskStatus == domain.TaskStatusCancelled {
		return false
	}
	subject := costSpecificationSubject(r)
	return domain.EffectiveAccessAllowsTask(actor, domain.PermissionCatalogManage, subject) || (actor.ID == r.CreatorID && domain.EffectiveAccessAllowsTask(actor, domain.PermissionTaskCreate, subject))
}
func hydrateCostSpecification(r *domain.TaskCostSpecification, actor domain.RequestActor) {
	raw, _ := json.Marshal([]interface{}{r.TaskID, r.SKUItemID, r.TaskStatus, r.VariantJSON, r.WidthCM, r.HeightCM, r.AreaM2, r.CostPrice, r.RequiresArea})
	h := sha256.Sum256(raw)
	r.Token = hex.EncodeToString(h[:])
	r.CanEdit = canCorrectCostSpecification(actor, r)
	r.MissingArea = r.RequiresArea && (r.AreaM2 == nil || *r.AreaM2 <= 0)
	r.CostInput = domain.CostInput{AreaMode: "total", Pieces: 1, Processes: map[string]bool{}}
	if input := skuCostInput(&domain.TaskSKUItem{VariantJSON: r.VariantJSON}); input != nil {
		r.CostInput = *input
		return
	}
	if r.AreaM2 != nil {
		r.CostInput.AreaM2 = *r.AreaM2
	}
	if r.WidthCM != nil && r.HeightCM != nil && r.CostInput.AreaM2 > 0 && math.Abs(*r.WidthCM**r.HeightCM/10000-r.CostInput.AreaM2) < 0.000001 {
		r.CostInput.AreaMode = "flat"
		r.CostInput.WidthM = *r.WidthCM / 100
		r.CostInput.HeightM = *r.HeightCM / 100
	}
}
func (s *taskService) ListCostSpecifications(ctx context.Context, taskID int64, page, size int) (*domain.TaskCostSpecificationList, *domain.AppError) {
	actor, ok := domain.RequestActorFromContext(ctx)
	if !ok || !domain.ActorHasPermission(actor, domain.PermissionTaskView) {
		return nil, domain.ErrPermissionDenied
	}
	if s.costSpecifications == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "计价资料服务未配置", nil)
	}
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	if page > 200 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "请缩小查询范围", nil)
	}
	f := repo.TaskCostSpecificationFilter{TaskID: taskID, Page: page, PageSize: size, Access: domain.ResourceGroupAccessFilterForActor(actor, domain.PermissionTaskView)}
	if taskID == 0 {
		f.CreatorID = actor.ID
		f.MissingOnly = true
	} else {
		task, err := s.taskRepo.GetByID(ctx, taskID)
		if err != nil {
			return nil, infraError("read cost specification task", err)
		}
		if task == nil {
			return nil, domain.ErrNotFound
		}
		if !domain.EffectiveAccessAllowsTask(actor, domain.PermissionTaskView, task.AccessSubject()) {
			return nil, domain.ErrPermissionDenied
		}
	}
	rows, total, err := s.costSpecifications.List(ctx, f)
	if err != nil {
		return nil, infraError("list cost specifications", err)
	}
	for _, row := range rows {
		hydrateCostSpecification(row, actor)
	}
	return &domain.TaskCostSpecificationList{Data: rows, Total: total, Page: page, PageSize: size}, nil
}
func (s *taskService) CorrectCostSpecification(ctx context.Context, taskID int64, p domain.TaskCostSpecificationUpdate) (*domain.TaskCostSpecificationList, *domain.AppError) {
	actor, ok := domain.RequestActorFromContext(ctx)
	if !ok || actor.ID <= 0 {
		return nil, domain.ErrPermissionDenied
	}
	if taskID <= 0 || p.SKUItemID < 0 || p.Token == "" || strings.TrimSpace(p.Reason) == "" || utf8.RuneCountInString(p.Reason) > 200 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "请刷新资料并填写补录原因（最多200字）", nil)
	}
	if s.costSpecifications == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "计价资料服务未配置", nil)
	}
	err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		row, err := s.costSpecifications.Lock(ctx, tx, taskID, p.SKUItemID)
		if err != nil {
			return err
		}
		if row == nil {
			return domain.ErrNotFound
		}
		hydrateCostSpecification(row, actor)
		if !row.CanEdit || !domain.EffectiveAccessAllowsTask(actor, domain.PermissionTaskView, costSpecificationSubject(row)) {
			return domain.ErrPermissionDenied
		}
		if row.Token != p.Token {
			return domain.NewAppError(domain.ErrCodeConflict, "计价资料已更新，请刷新后重试", nil)
		}
		input := p.CostInput
		if row.SKUItemID == 0 && (input.AreaMode != "total" || input.ThicknessMM != 0 || input.Shape != "" || len(input.Faces) > 0) {
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "编码生成前请先补录整套总面积；生成后可补录分面、厚度和形状", nil)
		}
		// This endpoint changes geometry, never processing selections or quantities
		// used to order/ship goods. Preserve existing processing confirmation.
		input.Processes = row.CostInput.Processes
		input.HoleCount = row.CostInput.HoleCount
		input.ProcessLengthM = row.CostInput.ProcessLengthM
		check := input
		check.Processes = nil
		basis := "set"
		if row.RequiresArea {
			basis = "area"
		}
		calc, _ := calculateCostModel(&domain.CostModel{Basis: basis, UnitPrice: 1, Multiplier: 1}, check)
		if len(calc.Missing) > 0 {
			return domain.NewAppError(domain.ErrCodeInvalidRequest, strings.Join(calc.Missing, "；"), nil)
		}
		if input.WidthM*100 > 999999.9999 || input.HeightM*100 > 999999.9999 || calc.AreaM2 > 999999.9999 {
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "尺寸或面积超过可保存范围，请核对单位", nil)
		}
		if row.SKUItemID == 0 && row.RequiresArea && calc.AreaM2 < 0.0001 {
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "请填写不少于0.0001平方米的总面积", nil)
		}
		// Area must still be derived for non-area schemes when dimensions are supplied.
		area := calc.AreaM2
		if !row.RequiresArea {
			if c, _ := calculateCostModel(&domain.CostModel{Basis: "area", UnitPrice: 1, Multiplier: 1}, check); len(c.Missing) == 0 {
				area = c.AreaM2
			}
		}
		before := *row
		if err = s.costSpecifications.Save(ctx, tx, row, input, area); err != nil {
			return err
		}
		payload := map[string]interface{}{"sku_item_id": row.SKUItemID, "sku_code": row.SKUCode, "reason": strings.TrimSpace(p.Reason), "before": before.CostInput, "after": input, "cost_price_preserved": row.CostPrice, "cost_review_required": true, "summary": "补录计价资料；原价格保留，成本待核对"}
		if _, err = s.taskEventRepo.Append(ctx, tx, taskID, "task.cost_specification_updated", &actor.ID, payload); err != nil {
			return err
		}
		if s.costMaintenance != nil && row.SKUItemID > 0 {
			return s.costMaintenance.AppendLog(ctx, tx, taskID, row.SKUItemID, row.SKUCode, "specification", "area_incorrect", actor.ID, map[string]interface{}{"action": "specification", "reason_code": "area_incorrect", "reason_detail": p.Reason, "before": before, "after": map[string]interface{}{"cost_input": input, "cost_price": row.CostPrice}, "task_id": taskID, "sku_code": row.SKUCode})
		}
		return nil
	})
	if err != nil {
		if app, ok := err.(*domain.AppError); ok {
			return nil, app
		}
		return nil, infraError("correct cost specification", err)
	}
	return s.ListCostSpecifications(ctx, taskID, 1, 100)
}
