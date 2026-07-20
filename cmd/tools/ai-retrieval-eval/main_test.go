package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExecuteCaseReadsTheAuthoritativeSearchResponseContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/search" || request.URL.Query().Get("mode") != "hybrid" || request.URL.Query().Get("scope") != "all" {
			t.Fatalf("unexpected request %s", request.URL.String())
		}
		if request.Header.Get("Authorization") != "Bearer reviewed-token" {
			t.Fatalf("authorization header = %q", request.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"query":"交付风险","results":{"tasks":[{"id":12}],"assets":[{"asset_id":81,"resource_group_id":32,"source_type":"task_resource_group"},{"asset_id":91,"source_type":"external_asset"}],"products":[{"erp_code":"ERP-7","i_id":"IID-7"}],"users":[{"user_id":5}]},"retrieval":{"mode":"hybrid"}}`))
	}))
	defer server.Close()

	items, err := executeCase(context.Background(), server.Client(), server.URL, "reviewed-token", goldenCase{Query: "交付风险", Mode: "hybrid"})
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(items, ",")
	want := "task:12,task_resource_group:32,external_asset:91,product:ERP-7,user:5"
	if got != want {
		t.Fatalf("items = %q, want %q", got, want)
	}
}

func TestEvaluateEnforcesRecallNDCGAndLeakage(t *testing.T) {
	set := goldenSet{}
	results := map[string][]string{}
	for index := 0; index < 25; index++ {
		id := fmt.Sprintf("exact-%d", index)
		expected := fmt.Sprintf("task:%d", index+1)
		set.Cases = append(set.Cases, goldenCase{ID: id, Query: expected, Mode: "exact", Expected: []string{expected}})
		results[id] = []string{expected}
	}
	for index := 0; index < 25; index++ {
		id := fmt.Sprintf("semantic-%d", index)
		expected := fmt.Sprintf("task_resource_group:%d", index+1)
		set.Cases = append(set.Cases, goldenCase{ID: id, Query: "适合复用的套装", Mode: "hybrid", Expected: []string{expected}})
		results[id] = []string{expected}
	}
	if err := validateGoldenSet(set); err != nil {
		t.Fatal(err)
	}
	report := evaluate(set, results)
	if !report.Passed || report.ExactRecallAt1 != 1 || report.SemanticRecallAt10 != 1 || report.SemanticNDCGAt10 != 1 {
		t.Fatalf("report=%+v", report)
	}

	set.Cases[49].Forbidden = []string{"task:999"}
	results[set.Cases[49].ID] = []string{"task:999", set.Cases[49].Expected[0]}
	report = evaluate(set, results)
	if report.Passed || len(report.LeakageCases) != 1 {
		t.Fatalf("leakage report=%+v", report)
	}
}

func TestValidateGoldenSetRequiresFiftyReviewedMixedCases(t *testing.T) {
	set := goldenSet{Cases: []goldenCase{{ID: "one", Query: "Q", Mode: "exact", Expected: []string{"task:1"}}}}
	if err := validateGoldenSet(set); err == nil {
		t.Fatal("expected minimum-size failure")
	}
}
