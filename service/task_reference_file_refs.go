package service

import (
	"context"
	"strings"

	"workflow/domain"
)

const referenceFileRefsSuggestion = "use /v1/tasks/reference-upload and pass returned reference_file_refs objects"

func rejectReferenceImagesOnTaskCreate() *domain.AppError {
	return domain.NewAppError(domain.ErrCodeInvalidRequest, "reference_images is no longer accepted in task creation; upload files first and use reference_file_refs", map[string]interface{}{
		"field":      "reference_images",
		"suggestion": referenceFileRefsSuggestion,
	})
}

func RejectReferenceImagesOnTaskCreateForHandler() *domain.AppError {
	return rejectReferenceImagesOnTaskCreate()
}

func (s *taskService) validateReferenceFileRefs(ctx context.Context, expectedOwnerID *int64, refs []domain.ReferenceFileRef) *domain.AppError {
	if len(refs) == 0 {
		return nil
	}
	if s.assetStorageRefRepo == nil || s.uploadRequestRepo == nil {
		return domain.NewAppError(domain.ErrCodeInternalError, "reference_file_refs validation is not configured", nil)
	}

	invalid := make([]map[string]interface{}, 0)
	seen := make(map[string]struct{}, len(refs))
	for _, inputRef := range refs {
		refID := inputRef.CanonicalID()
		if refID == "" {
			invalid = append(invalid, map[string]interface{}{
				"ref":    inputRef.AssetID,
				"input":  inputRef,
				"reason": "empty_reference_file_ref",
			})
			continue
		}
		if _, ok := seen[refID]; ok {
			continue
		}
		seen[refID] = struct{}{}

		storageRef, err := s.assetStorageRefRepo.GetByRefID(ctx, refID)
		if err != nil {
			return infraError("get asset storage ref for reference_file_refs validation", err)
		}
		if storageRef == nil {
			invalid = append(invalid, map[string]interface{}{
				"ref":    refID,
				"input":  inputRef,
				"reason": "reference_file_ref_not_found",
			})
			continue
		}
		if storageRef.OwnerType != domain.AssetOwnerTypeTaskCreateReference {
			invalid = append(invalid, map[string]interface{}{
				"ref":    refID,
				"input":  inputRef,
				"reason": "reference_file_ref_not_from_task_create_asset_center",
			})
			continue
		}
		if expectedOwnerID != nil && storageRef.OwnerID != *expectedOwnerID {
			invalid = append(invalid, map[string]interface{}{
				"ref":    refID,
				"input":  inputRef,
				"reason": "reference_file_ref_owner_mismatch",
			})
			continue
		}
		if storageRef.Status != domain.AssetStorageRefStatusRecorded {
			invalid = append(invalid, map[string]interface{}{
				"ref":    refID,
				"input":  inputRef,
				"reason": "reference_file_ref_not_recorded",
			})
			continue
		}
		uploadRequestID := strings.TrimSpace(storageRef.UploadRequestID)
		if uploadRequestID == "" {
			invalid = append(invalid, map[string]interface{}{
				"ref":    refID,
				"input":  inputRef,
				"reason": "reference_file_ref_missing_upload_request",
			})
			continue
		}
		request, err := s.uploadRequestRepo.GetByRequestID(ctx, uploadRequestID)
		if err != nil {
			return infraError("get upload request for reference_file_refs validation", err)
		}
		if request == nil {
			invalid = append(invalid, map[string]interface{}{
				"ref":    refID,
				"input":  inputRef,
				"reason": "reference_file_ref_upload_request_not_found",
			})
			continue
		}
		if request.OwnerType != domain.AssetOwnerTypeTaskCreateReference {
			invalid = append(invalid, map[string]interface{}{
				"ref":    refID,
				"input":  inputRef,
				"reason": "reference_file_ref_upload_request_owner_invalid",
			})
			continue
		}
		if expectedOwnerID != nil && request.OwnerID != *expectedOwnerID {
			invalid = append(invalid, map[string]interface{}{
				"ref":    refID,
				"input":  inputRef,
				"reason": "reference_file_ref_upload_request_owner_mismatch",
			})
			continue
		}
		if request.TaskAssetType == nil || request.TaskAssetType.Canonical() != domain.TaskAssetTypeReference {
			invalid = append(invalid, map[string]interface{}{
				"ref":    refID,
				"input":  inputRef,
				"reason": "reference_file_ref_asset_type_invalid",
			})
			continue
		}
		if request.Status != domain.UploadRequestStatusBound {
			invalid = append(invalid, map[string]interface{}{
				"ref":    refID,
				"input":  inputRef,
				"reason": "reference_file_ref_upload_not_bound",
			})
			continue
		}
		if request.SessionStatus != domain.DesignAssetSessionStatusCompleted {
			invalid = append(invalid, map[string]interface{}{
				"ref":    refID,
				"input":  inputRef,
				"reason": "reference_file_ref_upload_not_completed",
			})
			continue
		}
		if strings.TrimSpace(request.BoundRefID) != refID {
			invalid = append(invalid, map[string]interface{}{
				"ref":    refID,
				"input":  inputRef,
				"reason": "reference_file_ref_binding_mismatch",
			})
			continue
		}
	}

	if len(invalid) == 0 {
		return nil
	}
	return domain.NewAppError(domain.ErrCodeInvalidRequest, "reference_file_refs contain invalid or unauthorized refs", map[string]interface{}{
		"invalid_reference_file_refs": invalid,
		"reason":                      "reference_file_refs must come from completed task reference uploads",
		"suggestion":                  referenceFileRefsSuggestion,
	})
}
