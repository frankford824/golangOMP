package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/repo"
)

func parseDriveDirectory(c *gin.Context) (*int64, bool) {
	if strings.EqualFold(strings.TrimSpace(c.Query("unassigned")), "1") ||
		strings.EqualFold(strings.TrimSpace(c.Query("unassigned")), "true") {
		return nil, true
	}
	if raw := strings.TrimSpace(c.Query("dir_id")); raw != "" {
		if value, err := strconv.ParseInt(raw, 10, 64); err == nil && value > 0 {
			return &value, false
		}
	}
	return nil, false
}

func parseUploadLedgerFilters(c *gin.Context) (repo.AssetWorkbenchDriveFilter, *domain.AppError) {
	dirID, unassigned := parseDriveDirectory(c)
	filter := repo.AssetWorkbenchDriveFilter{
		UploadDirectoryID: dirID,
		Unassigned:        unassigned,
		Keyword:           c.Query("q"),
		OwnerKeyword:      c.Query("owner"),
		WorkerType:        strings.TrimSpace(c.Query("worker_type")),
		OperationSource:   strings.TrimSpace(c.Query("operation_source")),
	}
	switch filter.WorkerType {
	case "", domain.AssetWorkbenchWorkerTypeFulltime, domain.AssetWorkbenchWorkerTypeParttime:
	default:
		return filter, domain.NewAppError(domain.ErrCodeInvalidRequest, "worker_type must be fulltime or parttime.", nil)
	}
	if value := strings.TrimSpace(c.Query("owner_user_id")); value != "" {
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil || id <= 0 {
			return filter, domain.NewAppError(domain.ErrCodeInvalidRequest, "owner_user_id must be a positive user ID.", nil)
		}
		filter.OwnerUserID = &id
	}
	raw := c.Query("dir_ids")
	if raw == "" {
		return filter, nil
	}
	if strings.TrimSpace(c.Query("dir_id")) != "" {
		return filter, domain.NewAppError(domain.ErrCodeInvalidRequest, "dir_id and dir_ids cannot be used together.", nil)
	}
	seen := make(map[int64]bool)
	for _, value := range strings.Split(raw, ",") {
		id, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		if err != nil || id <= 0 {
			return filter, domain.NewAppError(domain.ErrCodeInvalidRequest, "dir_ids must contain positive directory IDs.", nil)
		}
		if !seen[id] {
			filter.UploadDirectoryIDs = append(filter.UploadDirectoryIDs, id)
			seen[id] = true
		}
	}
	return filter, nil
}

func parseDriveTimeQuery(c *gin.Context, name string, endOfDay bool) (*time.Time, *domain.AppError) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return nil, nil
	}
	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		return &parsed, nil
	}
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		location = time.Local
	}
	day, err := time.ParseInLocation("2006-01-02", raw, location)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid "+name, map[string]string{name: raw})
	}
	if endOfDay {
		day = day.Add(24*time.Hour - time.Nanosecond)
	}
	return &day, nil
}

func (h *AssetWorkbenchHandler) DriveListDirectories(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	items, appErr := h.svc.ListDriveDirectories(c.Request.Context(), actor)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, items)
}

func (h *AssetWorkbenchHandler) DriveListOrders(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	dirID, unassigned := parseDriveDirectory(c)
	items, appErr := h.svc.ListDriveOrders(c.Request.Context(), actor, dirID, unassigned)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, items)
}

func parseDriveFileFilters(c *gin.Context) (repo.AssetWorkbenchDriveFilter, *domain.AppError) {
	filter, appErr := parseUploadLedgerFilters(c)
	if appErr != nil {
		return filter, appErr
	}
	filter.OrderNo = c.Query("order_no")
	filter.SortBy = c.Query("sort_by")
	filter.SortDir = c.Query("sort_dir")
	filter.CreatedFrom, appErr = parseDriveTimeQuery(c, "created_from", false)
	if appErr != nil {
		return filter, appErr
	}
	filter.CreatedTo, appErr = parseDriveTimeQuery(c, "created_to", true)
	if appErr != nil {
		return filter, appErr
	}
	if filter.CreatedFrom != nil && filter.CreatedTo != nil && filter.CreatedFrom.After(*filter.CreatedTo) {
		return filter, domain.NewAppError(domain.ErrCodeInvalidRequest, "created_from cannot be after created_to.", nil)
	}
	return filter, nil
}

func (h *AssetWorkbenchHandler) DriveListFiles(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	filter, appErr := parseDriveFileFilters(c)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	filter.Page, _ = strconv.Atoi(c.Query("page"))
	filter.PageSize, _ = strconv.Atoi(c.Query("page_size"))
	result, appErr := h.svc.ListDriveFiles(c.Request.Context(), actor, filter)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOKWithPagination(c, result.Items, gin.H{"total": result.Total, "page": result.Page, "page_size": result.Size})
}

func (h *AssetWorkbenchHandler) DriveExportFiles(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	filter, appErr := parseDriveFileFilters(c)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	var items []*domain.AssetWorkbenchDriveFile
	items, appErr = h.svc.ExportDriveFiles(c.Request.Context(), actor, filter)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	c.Header("Cache-Control", "no-store")
	respondOK(c, items)
}

func (h *AssetWorkbenchHandler) DriveUploadDays(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	filter, appErr := parseUploadLedgerFilters(c)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	days, appErr := h.svc.ListDriveUploadDays(c.Request.Context(), actor, strings.TrimSpace(c.Query("month")), filter)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, days)
}

func (h *AssetWorkbenchHandler) DriveBrowseFolder(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	dirID, unassigned := parseDriveDirectory(c)
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	result, appErr := h.svc.BrowseDriveFolder(c.Request.Context(), actor, dirID, unassigned, c.Query("path"), page, pageSize)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) DriveSearch(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	result, appErr := h.svc.SearchDriveFiles(c.Request.Context(), actor, c.Query("q"), page, pageSize)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOKWithPagination(c, result.Items, gin.H{"total": result.Total, "page": result.Page, "page_size": result.Size})
}

func (h *AssetWorkbenchHandler) DriveLocate(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	fileID, err := strconv.ParseInt(strings.TrimSpace(c.Query("file_id")), 10, 64)
	if err != nil || fileID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid file_id", nil))
		return
	}
	result, appErr := h.svc.LocateDriveFile(c.Request.Context(), actor, fileID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) UpdateSubmissionFile(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	fileID, err := strconv.ParseInt(strings.TrimSpace(c.Param("file_id")), 10, 64)
	if err != nil || fileID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid file_id", nil))
		return
	}
	var req struct {
		DisplayName string `json:"display_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.UpdateSubmissionFileDisplayName(c.Request.Context(), actor, fileID, req.DisplayName)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) BrowseArchiveFile(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	fileID, err := strconv.ParseInt(strings.TrimSpace(c.Param("file_id")), 10, 64)
	if err != nil || fileID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid file_id", nil))
		return
	}
	result, appErr := h.svc.BrowseArchiveFile(c.Request.Context(), actor, fileID, c.Query("path"))
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) ServeArchiveEntry(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	fileID, err := strconv.ParseInt(strings.TrimSpace(c.Param("file_id")), 10, 64)
	if err != nil || fileID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid file_id", nil))
		return
	}
	stream, appErr := h.svc.OpenArchiveEntry(c.Request.Context(), actor, fileID, c.Query("path"))
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	defer stream.Close()
	disposition := "attachment"
	if strings.EqualFold(strings.TrimSpace(c.Query("disposition")), "inline") {
		disposition = "inline"
	}
	c.DataFromReader(http.StatusOK, stream.FileSize, stream.MimeType, stream.Body, map[string]string{
		"Content-Disposition": fmt.Sprintf(`%s; filename="%s"`, disposition, strings.ReplaceAll(stream.Filename, `"`, "")),
	})
}
