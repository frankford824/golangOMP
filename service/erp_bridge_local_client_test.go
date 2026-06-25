package service

import (
	"context"
	"strings"
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

func TestLocalERPBridgeClientUpsertProductPersistsCostForReadback(t *testing.T) {
	productRepo := &localERPBridgeProductRepoStub{}
	client := NewLocalERPBridgeClient(productRepo, nil, erpBridgeTxRunner{}, nil)

	cost := 14.133
	_, err := client.UpsertProduct(context.Background(), domain.ERPProductUpsertPayload{
		ProductID:   "DZK000013",
		SKUID:       "DZK000013",
		SKUCode:     "DZK000013",
		IID:         "定制kt板",
		ProductName: "真/定制kt板/双层宠物迎宾牌",
		CostPrice:   &cost,
	})
	if err != nil {
		t.Fatalf("UpsertProduct() error = %v", err)
	}

	product, err := client.GetProductByID(context.Background(), "DZK000013")
	if err != nil {
		t.Fatalf("GetProductByID() error = %v", err)
	}
	if product == nil {
		t.Fatal("GetProductByID() product = nil")
	}
	if product.CostPrice == nil || *product.CostPrice != cost {
		t.Fatalf("CostPrice = %+v, want %.3f", product.CostPrice, cost)
	}
	if len(productRepo.searchProducts) != 1 || !strings.Contains(productRepo.searchProducts[0].SpecJSON, `"c_price":14.133`) {
		t.Fatalf("local spec json did not persist c_price: %+v", productRepo.searchProducts)
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

func (s *localERPBridgeProductRepoStub) ListIIDs(context.Context, repo.ProductIIDListFilter) ([]*domain.ERPIIDOption, int64, error) {
	return []*domain.ERPIIDOption{}, 0, nil
}

func (s *localERPBridgeProductRepoStub) UpsertBatch(_ context.Context, _ repo.Tx, products []*domain.Product) (int64, error) {
	for _, product := range products {
		if product == nil {
			continue
		}
		replaced := false
		for idx, existing := range s.searchProducts {
			if existing != nil && existing.ERPProductID == product.ERPProductID {
				copyProduct := *product
				s.searchProducts[idx] = &copyProduct
				replaced = true
				break
			}
		}
		if !replaced {
			copyProduct := *product
			s.searchProducts = append(s.searchProducts, &copyProduct)
		}
	}
	s.searchTotal = int64(len(s.searchProducts))
	return int64(len(products)), nil
}
