package erp_product

import (
	"context"
	"encoding/json"
	"strings"

	"workflow/domain"
	"workflow/service"
)

type Service struct {
	erp service.ERPBridgeService
}

func NewService(erp service.ERPBridgeService) *Service {
	return &Service{erp: erp}
}

func (s *Service) LookupByCode(ctx context.Context, code string) (*domain.ERPProductSnapshot, *domain.AppError) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "code is required", nil)
	}
	product, appErr := s.erp.GetProductByID(ctx, code)
	if appErr != nil {
		switch appErr.Code {
		case domain.ErrCodeNotFound:
			return nil, domain.NewAppError("erp_product_not_found", "erp product not found", nil)
		default:
			return nil, domain.NewAppError("erp_upstream_failure", "erp upstream failure", appErr.Details)
		}
	}
	raw, _ := json.Marshal(product)
	return &domain.ERPProductSnapshot{
		Code:        code,
		ProductName: product.ProductName,
		Snapshot:    raw,
	}, nil
}
