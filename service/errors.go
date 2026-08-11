package service

import (
	"fmt"
	"log"

	"workflow/domain"
)

// infraError keeps infrastructure details out of API responses while preserving
// the operation name for stable error classification.
func infraError(op string, err error) *domain.AppError {
	// Keep the response stable and non-sensitive, but retain the underlying
	// infrastructure failure in server logs so production 5xx responses can be
	// diagnosed by trace time instead of being reduced to an opaque status.
	log.Printf("infrastructure_error op=%q error=%q", op, err)
	return domain.NewAppError(
		domain.ErrCodeInternalError,
		fmt.Sprintf("internal error during %s", op),
		nil,
	)
}
