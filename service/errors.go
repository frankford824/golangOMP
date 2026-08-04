package service

import (
	"fmt"

	"workflow/domain"
)

// infraError keeps infrastructure details out of API responses while preserving
// the operation name for stable error classification.
func infraError(op string, err error) *domain.AppError {
	_ = err
	return domain.NewAppError(
		domain.ErrCodeInternalError,
		fmt.Sprintf("internal error during %s", op),
		nil,
	)
}
