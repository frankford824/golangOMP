package main

import "testing"

func TestGroupPreviewJobsByTaskPreservesTaskAndAssetOrder(t *testing.T) {
	jobs := []sourceAssetJob{
		{TaskID: 10, SourceAssetID: 101},
		{TaskID: 20, SourceAssetID: 201},
		{TaskID: 10, SourceAssetID: 102},
		{TaskID: 30, SourceAssetID: 301},
		{TaskID: 20, SourceAssetID: 202},
	}
	batches := groupPreviewJobsByTask(jobs)
	if len(batches) != 3 {
		t.Fatalf("batch count = %d", len(batches))
	}
	want := [][]int64{{101, 102}, {201, 202}, {301}}
	for batchIndex, batch := range batches {
		if len(batch) != len(want[batchIndex]) {
			t.Fatalf("batch %d length = %d", batchIndex, len(batch))
		}
		for jobIndex, job := range batch {
			if job.SourceAssetID != want[batchIndex][jobIndex] {
				t.Fatalf("batch %d job %d asset = %d", batchIndex, jobIndex, job.SourceAssetID)
			}
		}
	}
}
