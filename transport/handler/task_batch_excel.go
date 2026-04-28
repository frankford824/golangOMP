package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	taskbatchexcel "workflow/service/task_batch_excel"
)

type TaskBatchExcelHandler struct {
	templateSvc taskbatchexcel.TemplateService
	parseSvc    taskbatchexcel.ParseService
}

func NewTaskBatchExcelHandler(templateSvc taskbatchexcel.TemplateService, parseSvc taskbatchexcel.ParseService) *TaskBatchExcelHandler {
	return &TaskBatchExcelHandler{templateSvc: templateSvc, parseSvc: parseSvc}
}

func (h *TaskBatchExcelHandler) DownloadTemplate(c *gin.Context) {
	taskType := parseBatchExcelTaskType(c.Query("task_type"))
	if taskType == "" {
		taskType = domain.TaskTypeNewProductDevelopment
	}
	content, appErr := h.templateSvc.Generate(c.Request.Context(), taskType)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	filename := fmt.Sprintf("batch_create_template_%s_%s.xlsx", taskType, time.Now().Format("20060102"))
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", content)
}

func (h *TaskBatchExcelHandler) ParseUpload(c *gin.Context) {
	taskType := parseBatchExcelTaskType(c.PostForm("task_type"))
	if taskType == "" {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "task_type is required", nil))
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "file is required", nil))
		return
	}
	src, err := file.Open()
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "file cannot be opened", nil))
		return
	}
	defer src.Close()
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	result, appErr := h.parseSvc.Parse(c.Request.Context(), taskType, src, taskbatchexcel.WithActorID(actor.ID))
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func parseBatchExcelTaskType(raw string) domain.TaskType {
	return domain.TaskType(strings.TrimSpace(raw))
}
