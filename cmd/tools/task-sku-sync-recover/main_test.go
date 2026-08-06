package main

import "testing"

func TestResolveIIDPriority(t *testing.T) {
	item := &candidate{
		CurrentIID: " explicit ", VariantIID: "variant", PlanningIID: "planning",
		ProductIID: "product", SyncRecordIID: "sync", CategoryExactIID: "category",
	}
	got, source := resolveIID(item)
	if got != "explicit" || source != "task_sku_item.product_i_id" {
		t.Fatalf("resolveIID = %q/%q", got, source)
	}
}

func TestBuildPlansBlocksWholeTaskAndExcludesTerminal(t *testing.T) {
	plans, excludedTasks, excludedItems := buildPlans([]*candidate{
		{TaskID: 1, TaskNo: "A", TaskType: "new_product_development", TaskStatus: "InProgress", ItemID: 11, ResolvedIID: "x"},
		{TaskID: 1, TaskNo: "A", TaskType: "new_product_development", TaskStatus: "InProgress", ItemID: 12},
		{TaskID: 2, TaskNo: "B", TaskType: "new_product_development", TaskStatus: "Cancelled", ItemID: 21, ResolvedIID: "y"},
	})
	if len(plans) != 1 || !plans[0].Blocked {
		t.Fatalf("plans = %+v, want one blocked atomic plan", plans)
	}
	if excludedTasks != 1 || excludedItems != 1 {
		t.Fatalf("excluded = %d/%d, want 1/1", excludedTasks, excludedItems)
	}
}
