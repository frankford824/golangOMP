package handler

import (
	"strings"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

type TaskCreateReferenceUploadHandler struct {
	svc service.TaskCreateReferenceUploadService
}

func NewTaskCreateReferenceUploadHandler(svc service.TaskCreateReferenceUploadService) *TaskCreateReferenceUploadHandler {
	return &TaskCreateReferenceUploadHandler{svc: svc}
}

type createTaskCreateReferenceUploadSessionReq struct {
	CreatedBy    *int64 `json:"created_by"`
	Filename     string `json:"filename" binding:"required"`
	ExpectedSize *int64 `json:"expected_size"`
	MimeType     string `json:"mime_type"`
	FileHash     string `json:"file_hash"`
	Remark       string `json:"remark"`
}

type completeTaskCreateReferenceUploadSessionReq struct {
	CompletedBy *int64 `json:"completed_by"`
	FileHash    string `json:"file_hash"`
	Remark      string `json:"remark"`
}

type cancelTaskCreateReferenceUploadSessionReq struct {
	CancelledBy *int64 `json:"cancelled_by"`
	Remark      string `json:"remark"`
}

func (h *TaskCreateReferenceUploadHandler) CreateUploadSession(c *gin.Context) {
	var req createTaskCreateReferenceUploadSessionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	createdBy, appErr := actorIDOrRequestValue(c, req.CreatedBy, "created_by")
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	result, appErr := h.svc.CreateUploadSession(c.Request.Context(), service.CreateTaskReferenceUploadSessionParams{
		CreatedBy:    createdBy,
		Filename:     strings.TrimSpace(req.Filename),
		ExpectedSize: req.ExpectedSize,
		MimeType:     strings.TrimSpace(req.MimeType),
		FileHash:     strings.TrimSpace(req.FileHash),
		Remark:       strings.TrimSpace(req.Remark),
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondCreated(c, result)
}

func (h *TaskCreateReferenceUploadHandler) UploadFile(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "file is required", nil))
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "open uploaded file", nil))
		return
	}
	defer file.Close()

	var createdByValue *int64
	if raw := strings.TrimSpace(c.PostForm("created_by")); raw != "" {
		parsed, parseErr := parseInt64(raw)
		if parseErr != nil {
			respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "created_by must be an integer", nil))
			return
		}
		createdByValue = &parsed
	}
	createdBy, appErr := actorIDOrRequestValue(c, createdByValue, "created_by")
	if appErr != nil {
		respondError(c, appErr)
		return
	}

	filename := strings.TrimSpace(fileHeader.Filename)
	if filename == "" {
		filename = "reference-upload"
	}
	expectedSize := fileHeader.Size
	expectedSizePtr := &expectedSize
	mimeType := strings.TrimSpace(fileHeader.Header.Get("Content-Type"))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	refObject, appErr := h.svc.UploadFile(c.Request.Context(), service.UploadTaskReferenceFileParams{
		CreatedBy:    createdBy,
		Filename:     filename,
		ExpectedSize: expectedSizePtr,
		MimeType:     mimeType,
		FileHash:     strings.TrimSpace(c.PostForm("file_hash")),
		Remark:       strings.TrimSpace(c.PostForm("remark")),
		File:         file,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondCreated(c, refObject)
}

func (h *TaskCreateReferenceUploadHandler) GetUploadSession(c *gin.Context) {
	requesterID, appErr := actorIDOrRequestValue(c, nil, "requester_id")
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	session, appErr := h.svc.GetUploadSession(c.Request.Context(), strings.TrimSpace(c.Param("session_id")), requesterID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, session)
}

func (h *TaskCreateReferenceUploadHandler) CompleteUploadSession(c *gin.Context) {
	var req completeTaskCreateReferenceUploadSessionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	completedBy, appErr := actorIDOrRequestValue(c, req.CompletedBy, "completed_by")
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	result, appErr := h.svc.CompleteUploadSession(c.Request.Context(), service.CompleteTaskReferenceUploadSessionParams{
		SessionID:   strings.TrimSpace(c.Param("session_id")),
		CompletedBy: completedBy,
		Remark:      strings.TrimSpace(req.Remark),
		FileHash:    strings.TrimSpace(req.FileHash),
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *TaskCreateReferenceUploadHandler) AbortUploadSession(c *gin.Context) {
	var req cancelTaskCreateReferenceUploadSessionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	cancelledBy, appErr := actorIDOrRequestValue(c, req.CancelledBy, "cancelled_by")
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	session, appErr := h.svc.CancelUploadSession(c.Request.Context(), service.CancelTaskReferenceUploadSessionParams{
		SessionID:   strings.TrimSpace(c.Param("session_id")),
		CancelledBy: cancelledBy,
		Remark:      strings.TrimSpace(req.Remark),
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, session)
}
