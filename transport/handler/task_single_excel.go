package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	tasksingleexcel "workflow/service/task_single_excel"
)

type TaskSingleExcelHandler struct {
	templateSvc tasksingleexcel.TemplateService
	parseSvc    tasksingleexcel.ParseService
}

func NewTaskSingleExcelHandler(templateSvc tasksingleexcel.TemplateService, parseSvc tasksingleexcel.ParseService) *TaskSingleExcelHandler {
	return &TaskSingleExcelHandler{templateSvc: templateSvc, parseSvc: parseSvc}
}

func (h *TaskSingleExcelHandler) DownloadTemplate(c *gin.Context) {
	mode := strings.TrimSpace(c.Query("mode"))
	if mode == "" {
		mode = strings.TrimSpace(c.Query("excel_assist_mode"))
	}
	if err := validateExcelAssistMode(mode); err != nil {
		respondError(c, err)
		return
	}
	taskType := domain.TaskType(strings.TrimSpace(c.Query("task_type")))
	if err := validateExcelAssistTaskType(taskType); err != nil {
		respondError(c, err)
		return
	}
	content, appErr := h.templateSvc.Generate(c.Request.Context(), taskType, mode)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	filename := fmt.Sprintf("excel_assist_single_%s_%s.xlsx", taskType, time.Now().Format("20060102"))
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", content)
}

func (h *TaskSingleExcelHandler) ParseUpload(c *gin.Context) {
	mode := strings.TrimSpace(c.PostForm("mode"))
	if err := validateExcelAssistMode(mode); err != nil {
		respondError(c, err)
		return
	}
	taskType := domain.TaskType(strings.TrimSpace(c.PostForm("task_type")))
	if err := validateExcelAssistTaskType(taskType); err != nil {
		respondError(c, err)
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
	result, appErr := h.parseSvc.Parse(c.Request.Context(), taskType, mode, src, tasksingleexcel.WithActorID(actor.ID))
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func validateExcelAssistMode(mode string) *domain.AppError {
	if strings.TrimSpace(mode) != tasksingleexcel.AssistModeSingle {
		return domain.NewAppError("invalid_excel_assist_mode", "Excel assist mode must be single", nil)
	}
	return nil
}

func validateExcelAssistTaskType(taskType domain.TaskType) *domain.AppError {
	switch taskType {
	case domain.TaskTypeNewProductDevelopment, domain.TaskTypePurchaseTask, domain.TaskTypeOriginalProductDevelopment:
		return nil
	default:
		return domain.NewAppError("excel_assist_task_type_not_supported", "task_type is not supported for single-task Excel assist", nil)
	}
}
