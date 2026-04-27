package service

import (
	"context"
	"testing"

	"workflow/domain"
	"workflow/repo"
)

func TestShouldUseLocalERPBridgeClient(t *testing.T) {
	if !ShouldUseLocalERPBridgeClient("8081", "http://127.0.0.1:8081") {
		t.Fatalf("ShouldUseLocalERPBridgeClient() = false, want true")
	}
	if ShouldUseLocalERPBridgeClient("8080", "http://127.0.0.1:8081") {
		t.Fatalf("ShouldUseLocalERPBridgeClient() = true, want false")
	}
}

func TestLocalERPBridgeClientSearchProductsReadsLocalRepo(t *testing.T) {
	client := NewLocalERPBridgeClient(&localERPBridgeProductRepoStub{
		searchProducts: []*domain.Product{
			{
				ERPProductID: "ERP-1001",
				SKUCode:      "CF-001",
				ProductName:  "Poster Product",
				Category:     "Poster",
				SpecJSON:     `{"product_id":"ERP-1001","sku_code":"CF-001","product_name":"Poster Product","category_name":"Poster"}`,
			},
		},
		searchTotal: 1,
	}, nil, nil, nil)

	resp, err := client.SearchProducts(context.Background(), domain.ERPProductSearchFilter{
		Q:        "Poster",
		Page:     1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("SearchProducts() error = %v", err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("SearchProducts() items = %+v", resp.Items)
	}
	if resp.Items[0].ProductID != "ERP-1001" || resp.Items[0].SKUCode != "CF-001" {
		t.Fatalf("SearchProducts() first item = %+v", resp.Items[0])
	}
	if resp.Pagination.Total != 1 {
		t.Fatalf("SearchProducts() pagination = %+v", resp.Pagination)
	}
}

type localERPBridgeProductRepoStub struct {
	searchProducts []*domain.Product
	searchTotal    int64
}

func (s *localERPBridgeProductRepoStub) GetByID(context.Context, int64) (*domain.Product, error) {
	return nil, nil
}

func (s *localERPBridgeProductRepoStub) GetByERPProductID(_ context.Context, erpProductID string) (*domain.Product, error) {
	for _, product := range s.searchProducts {
		if product != nil && product.ERPProductID == erpProductID {
			copyProduct := *product
			return &copyProduct, nil
		}
	}
	return nil, nil
}

func (s *localERPBridgeProductRepoStub) Search(context.Context, repo.ProductSearchFilter) ([]*domain.Product, int64, error) {
	items := make([]*domain.Product, 0, len(s.searchProducts))
	for _, product := range s.searchProducts {
		if product == nil {
			continue
		}
		copyProduct := *product
		items = append(items, &copyProduct)
	}
	return items, s.searchTotal, nil
}

func (s *localERPBridgeProductRepoStub) UpsertBatch(context.Context, repo.Tx, []*domain.Product) (int64, error) {
	return 0, nil
}
