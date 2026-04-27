package asset_lifecycle

import (
	"database/sql"
	"errors"

	"workflow/domain"
)

func toAppError(err error) *domain.AppError {
	if err == nil {
		return nil
	}
	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrNotFound
	}
	return domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
}
