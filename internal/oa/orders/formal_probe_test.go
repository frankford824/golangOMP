package orders

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"sort"
	"sync"
	"testing"
	"time"
)

func TestFormalOrderReadProbe(t *testing.T) {
	if os.Getenv("OA_ORDER_READ_PROBE") != "1" {
		t.Skip("set OA_ORDER_READ_PROBE=1")
	}
	ch := newClient()
	if ch == nil {
		t.Fatal("configure OPS_READER_PASSWORD and optional OA_CLICKHOUSE_URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	end := time.Now().In(beijing).AddDate(0, 0, -1)
	if raw := os.Getenv("OA_ORDER_READ_PROBE_DATE"); raw != "" {
		var err error
		end, err = time.ParseInLocation(time.DateOnly, raw, beijing)
		if err != nil {
			t.Fatal("invalid OA_ORDER_READ_PROBE_DATE")
		}
	}
	start := end.AddDate(0, 0, -29)
	if start.Before(firstDay) {
		t.Fatal("probe needs 30 days within the documented order range")
	}
	shops, err := ch.rows(ctx, shopNamesSQL, url.Values{"param_all": {"1"}, "param_shops": {"[]"}})
	if err != nil {
		t.Fatal(err)
	}
	type sample struct{ shop, date string }
	samples := []sample{}
	for day := end; !day.Before(start) && len(samples) < 100; day = day.AddDate(0, 0, -1) {
		for _, record := range shops {
			var id string
			if json.Unmarshal(record["shop_id"], &id) == nil && id != "" {
				samples = append(samples, sample{id, day.Format(time.DateOnly)})
				if len(samples) == 100 {
					break
				}
			}
		}
	}
	if len(samples) < 100 {
		t.Fatal("fewer than 100 shop/date combinations")
	}
	for _, endpoint := range []struct{ name, sql string }{{"summary", summarySQL}, {"products", productsSQL}} {
		for _, mode := range []string{"fixed", "varying"} {
			var waits [100]time.Duration
			var failures [100]error
			var workers sync.WaitGroup
			ready := make(chan struct{})
			for worker := 0; worker < 8; worker++ {
				workers.Add(1)
				go func() {
					defer workers.Done()
					<-ready
					for pos := worker; pos < 100; pos += 8 {
						sel := sample{"", end.Format(time.DateOnly)}
						if mode == "varying" {
							sel = samples[pos]
						}
						date, _ := time.ParseInLocation(time.DateOnly, sel.date, beijing)
						params := url.Values{"param_from": {sel.date}, "param_until": {date.AddDate(0, 0, 1).Format(time.DateOnly)}, "param_shop": {sel.shop}, "param_all": {"1"}, "param_shops": {"[]"}, "param_platform": {""}, "param_sku": {""}, "param_product": {""}, "param_brush": {"0"}, "param_offset": {"0"}}
						started := time.Now()
						rows, err := ch.rows(ctx, endpoint.sql, params)
						if err == nil && endpoint.name == "summary" {
							app := server{ch: ch}
							err = app.markVerifiedZero(ctx, params, rows)
						}
						waits[pos] = time.Since(started)
						failures[pos] = err
					}
				}()
			}
			close(ready)
			workers.Wait()
			durations := []time.Duration{}
			for pos, latency := range waits {
				if failures[pos] == nil {
					durations = append(durations, latency)
				}
			}
			sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
			p95 := "n/a"
			if len(durations) > 0 {
				p95 = fmt.Sprintf("%.1fms", float64(durations[(95*len(durations)+99)/100-1])/float64(time.Millisecond))
			}
			errors := len(waits) - len(durations)
			t.Logf("%s %s c8 requests=%d errors=%d p95_success=%s", endpoint.name, mode, len(waits), errors, p95)
			if errors > 0 {
				t.Fail()
			}
		}
	}
}
