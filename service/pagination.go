package service

import "workflow/domain"

func buildPaginationMeta(page, pageSize int, total int64) domain.PaginationMeta {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return domain.PaginationMeta{
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}
}
