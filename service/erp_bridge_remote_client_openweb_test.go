package service

import "testing"

func TestSignERPRemoteOpenWeb(t *testing.T) {
	params := map[string]string{
		"app_key":      "ak",
		"access_token": "tk",
		"timestamp":    "1710000000",
		"charset":      "utf-8",
		"version":      "2",
		"biz":          `{"x":1}`,
		"sign":         "should_be_ignored",
	}
	got := signERPRemoteOpenWeb("secret", params)
	// secret + access_tokentkapp_keyakbiz{"x":1}charsetutf-8timestamp1710000000version2
	const want = "90b0520907de1766606ae41a07474bdc"
	if got != want {
		t.Fatalf("signERPRemoteOpenWeb() = %q, want %q", got, want)
	}
}

func TestBuildERPRemoteOpenWebBizUpsert(t *testing.T) {
	raw := []byte(`{
		"product_id":"p-001",
		"sku_id":"sku-id-1",
		"sku_code":"sku-code-1",
		"product_name":"Demo Product",
		"category_name":"Demo Category",
		"source":"task_business_info_filing",
		"business_info":{"cost_price":12.5}
	}`)
	biz, err := buildERPRemoteOpenWebBiz("upsert", raw)
	if err != nil {
		t.Fatalf("buildERPRemoteOpenWebBiz upsert error: %v", err)
	}
	items, ok := biz["items"].([]map[string]interface{})
	if !ok || len(items) != 1 {
		t.Fatalf("unexpected items: %#v", biz["items"])
	}
	item := items[0]
	if item["sku_id"] != "sku-id-1" {
		t.Fatalf("sku_id = %#v, want sku-id-1", item["sku_id"])
	}
	if item["name"] != "Demo Product" {
		t.Fatalf("name = %#v, want Demo Product", item["name"])
	}
	if item["category_name"] != "Demo Category" {
		t.Fatalf("category_name = %#v, want Demo Category", item["category_name"])
	}
	if item["cost_price"] != 12.5 {
		t.Fatalf("cost_price = %#v, want 12.5", item["cost_price"])
	}
	if item["remark"] != "task_business_info_filing" {
		t.Fatalf("remark = %#v, want task_business_info_filing", item["remark"])
	}
}

func TestBuildERPRemoteOpenWebBizBatchAndVirtual(t *testing.T) {
	shelveRaw := []byte(`{
		"items":[
			{"sku_code":"sku-a"},
			{"sku_id":"sku-b"},
			{"product_id":"sku-c"}
		]
	}`)
	shelveBiz, err := buildERPRemoteOpenWebBiz("shelve_batch", shelveRaw)
	if err != nil {
		t.Fatalf("buildERPRemoteOpenWebBiz shelve error: %v", err)
	}
	if got := shelveBiz["sku_codes"]; got != "sku-a,sku-b,sku-c" {
		t.Fatalf("sku_codes = %#v, want sku-a,sku-b,sku-c", got)
	}
	items, ok := shelveBiz["items"].([]map[string]interface{})
	if !ok || len(items) != 3 {
		t.Fatalf("shelve items unexpected: %#v", shelveBiz["items"])
	}
	if items[0]["sku_id"] != "sku-a" || items[1]["sku_id"] != "sku-b" || items[2]["sku_id"] != "sku-c" {
		t.Fatalf("shelve items sku_id unexpected: %#v", items)
	}

	virtualRaw := []byte(`{
		"items":[
			{"sku_code":"sku-a","virtual_qty":3},
			{"sku_id":"sku-b","virtual_qty":5}
		]
	}`)
	virtualBiz, err := buildERPRemoteOpenWebBiz("virtual_inventory", virtualRaw)
	if err != nil {
		t.Fatalf("buildERPRemoteOpenWebBiz virtual error: %v", err)
	}
	list, ok := virtualBiz["list"].([]map[string]interface{})
	if !ok || len(list) != 2 {
		t.Fatalf("virtual list unexpected: %#v", virtualBiz["list"])
	}
	if list[0]["sku_id"] != "sku-a" || list[0]["virtual_qty"] != int64(3) || list[0]["qty"] != int64(3) {
		t.Fatalf("virtual list[0] unexpected: %#v", list[0])
	}
	if list[1]["sku_id"] != "sku-b" || list[1]["virtual_qty"] != int64(5) || list[1]["qty"] != int64(5) {
		t.Fatalf("virtual list[1] unexpected: %#v", list[1])
	}
	itemsAlias, ok := virtualBiz["items"].([]map[string]interface{})
	if !ok || len(itemsAlias) != 2 {
		t.Fatalf("virtual items alias unexpected: %#v", virtualBiz["items"])
	}
}

func TestBuildERPRemoteOpenWebBizItemStyleUpdate(t *testing.T) {
	raw := []byte(`{
		"i_id":"STYLE-1",
		"sku_id":"SKU-1",
		"name":"测试商品",
		"short_name":"测试-STYLE-1",
		"pic":"https://img.example.com/a.png"
	}`)
	biz, err := buildERPRemoteOpenWebBiz("item_style_update", raw)
	if err != nil {
		t.Fatalf("buildERPRemoteOpenWebBiz item_style_update error: %v", err)
	}
	items, ok := biz["items"].([]map[string]interface{})
	if !ok || len(items) != 1 {
		t.Fatalf("unexpected items: %#v", biz["items"])
	}
	item := items[0]
	if item["i_id"] != "STYLE-1" {
		t.Fatalf("i_id = %#v, want STYLE-1", item["i_id"])
	}
	if item["sku_id"] != "SKU-1" {
		t.Fatalf("sku_id = %#v, want SKU-1", item["sku_id"])
	}
}

func TestValidateERPRemoteOpenWebResponseVirtualInventory(t *testing.T) {
	err := validateERPRemoteOpenWebResponse(
		"virtual_inventory",
		"https://openapi.jushuitan.com/open/webapi/itemapi/iteminventory/batchupdatewmsvirtualqtys",
		0,
		[]byte(`{"code":0,"msg":"未获取到有效的传入数据","data":null}`),
	)
	if err == nil {
		t.Fatalf("expected validation error for invalid virtual inventory response")
	}
	if err.Code != 100 {
		t.Fatalf("unexpected code: %d", err.Code)
	}
}
