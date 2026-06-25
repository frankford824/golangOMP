package service

import "testing"

func TestJSTMapsToERPProductsReadsImageFields(t *testing.T) {
	products := jstMapsToERPProducts([]map[string]interface{}{
		{
			"sku_id":             "CGK000181",
			"i_id":               "10001",
			"name":               "ERP Product",
			"product_short_name": "ERP Short Product",
			"sku_pic":            "https://img.example.com/sku.jpg",
			"pic_big":            "https://img.example.com/big.jpg",
		},
		{
			"sku_id":     "CGK000182",
			"i_id":       "10002",
			"name":       "ERP Product",
			"short_name": "ERP Short Alias",
			"pic_big":    "https://img.example.com/only-big.jpg",
		},
	}, "")
	if len(products) != 2 {
		t.Fatalf("len(products) = %d, want 2", len(products))
	}
	if products[0].ImageURL != "https://img.example.com/sku.jpg" {
		t.Fatalf("products[0].ImageURL = %q, want sku_pic", products[0].ImageURL)
	}
	if products[1].ImageURL != "https://img.example.com/only-big.jpg" {
		t.Fatalf("products[1].ImageURL = %q, want pic_big", products[1].ImageURL)
	}
	if products[0].ProductShortName != "ERP Short Product" || products[0].ShortName != "ERP Short Product" {
		t.Fatalf("products[0] short names not mapped: %+v", products[0])
	}
	if products[1].ProductShortName != "ERP Short Alias" || products[1].ShortName != "ERP Short Alias" {
		t.Fatalf("products[1] short names not mapped from short_name: %+v", products[1])
	}
}
