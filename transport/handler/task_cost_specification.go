package handler

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"io"
	"strconv"
	"workflow/domain"
	"workflow/service"
)

func (h *TaskHandler) CostSpecificationTodos(c *gin.Context) {
	out, appErr := h.costSpecificationList(c, 0)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	response := domain.TaskCostSpecificationList{Data: out.Data, Total: out.Total, Page: out.Page, PageSize: out.PageSize}
	c.JSON(200, response)
}
func (h *TaskHandler) CostSpecifications(c *gin.Context) {
	id, err := parseID(c)
	if err != nil || id <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	out, appErr := h.costSpecificationList(c, id)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	response := domain.TaskCostSpecificationList{Data: out.Data, Total: out.Total, Page: out.Page, PageSize: out.PageSize}
	c.JSON(200, response)
}
func (h *TaskHandler) costSpecificationList(c *gin.Context, id int64) (*domain.TaskCostSpecificationList, *domain.AppError) {
	svc, ok := h.svc.(service.TaskCostSpecificationService)
	if !ok {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "计价资料服务未配置", nil)
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	return svc.ListCostSpecifications(c.Request.Context(), id, page, size)
}
func (h *TaskHandler) CorrectCostSpecification(c *gin.Context) {
	id, err := parseID(c)
	if err != nil || id <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	var input struct {
		SKUItemID *int64            `json:"sku_item_id"`
		Token     string            `json:"token"`
		Reason    string            `json:"reason"`
		CostInput *domain.CostInput `json:"cost_input"`
	}
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&input); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "只允许补录计价规格、版本和原因", nil))
		return
	}
	if decoder.Decode(new(interface{})) != io.EOF {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "请提交单个有效的计价资料对象", nil))
		return
	}
	if input.SKUItemID == nil || input.CostInput == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "缺少编码行标识或计价规格，请刷新后重试", nil))
		return
	}
	svc, ok := h.svc.(service.TaskCostSpecificationService)
	if !ok {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "计价资料服务未配置", nil))
		return
	}
	result, appErr := svc.CorrectCostSpecification(c.Request.Context(), id, domain.TaskCostSpecificationUpdate{SKUItemID: *input.SKUItemID, Token: input.Token, Reason: input.Reason, CostInput: *input.CostInput})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	response := domain.TaskCostSpecificationList{Data: result.Data, Total: result.Total, Page: result.Page, PageSize: result.PageSize}
	c.JSON(200, response)
}
