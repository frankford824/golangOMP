package service

import (
	"sync"
	"testing"
)

func TestClaimDerivedPreviewGenerationDeduplicatesConcurrentKey(t *testing.T) {
	svc := &taskAssetCenterService{}

	const contenders = 64
	start := make(chan struct{})
	results := make(chan bool, contenders)
	var wg sync.WaitGroup
	for range contenders {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results <- svc.claimDerivedPreviewGeneration("task:asset:version:key")
		}()
	}

	close(start)
	wg.Wait()
	close(results)

	claimed := 0
	for ok := range results {
		if ok {
			claimed++
		}
	}
	if claimed != 1 {
		t.Fatalf("successful concurrent claims = %d, want exactly 1", claimed)
	}
	if svc.claimDerivedPreviewGeneration("task:asset:version:key") {
		t.Fatal("duplicate claim succeeded while the key was still in flight")
	}

	svc.releaseDerivedPreviewGeneration("task:asset:version:key")
	if !svc.claimDerivedPreviewGeneration("task:asset:version:key") {
		t.Fatal("claim did not become available after release")
	}
	svc.releaseDerivedPreviewGeneration("task:asset:version:key")
}

func TestTaskAssetCenterServiceDerivedPreviewSlotsAreBounded(t *testing.T) {
	svc := NewTaskAssetCenterService(nil, nil, nil, nil, nil, nil, nil, nil).(*taskAssetCenterService)
	if svc.derivedPreviewSlots == nil {
		t.Fatal("derivedPreviewSlots = nil, want bounded worker slots")
	}
	if got := cap(svc.derivedPreviewSlots); got != 2 {
		t.Fatalf("derivedPreviewSlots capacity = %d, want 2", got)
	}

	svc.derivedPreviewSlots <- struct{}{}
	svc.derivedPreviewSlots <- struct{}{}
	select {
	case svc.derivedPreviewSlots <- struct{}{}:
		t.Fatal("third derived preview job acquired a slot while both slots were occupied")
	default:
	}
}
