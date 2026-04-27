//go:build integration

package search

import (
	"sort"
	"testing"
	"time"

	"workflow/domain"
)

func TestSADI11_SearchP95Performance(t *testing.T) {
	db, svc := sadSearchDBSvc(t)
	sadCleanup(t, db, []int64{50011}, nil)
	t.Cleanup(func() { sadCleanup(t, db, []int64{50011}, nil) })
	sadInsertTaskAsset(t, db, 50011, "SADPERF-50011", "SADPERF-SKU-50011", "sad_perf.psd")
	ctx, cancel := sadCtx(t)
	defer cancel()
	durations := make([]time.Duration, 0, 100)
	for i := 0; i < 100; i++ {
		start := time.Now()
		if _, appErr := svc.Search(ctx, sadActor(50011, domain.RoleMember), "SADPERF", "all", 20); appErr != nil {
			t.Fatal(appErr)
		}
		durations = append(durations, time.Since(start))
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	p95 := durations[94]
	t.Logf("SA-D search perf avg=%s median=%s p95=%s", avgDuration(durations), durations[49], p95)
	if p95 > time.Second {
		t.Fatalf("p95=%s want <1s", p95)
	}
}

func avgDuration(values []time.Duration) time.Duration {
	var total time.Duration
	for _, v := range values {
		total += v
	}
	return total / time.Duration(len(values))
}
