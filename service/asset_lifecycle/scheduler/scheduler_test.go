package scheduler

import (
	"context"
	"testing"
	"time"
)

func TestCron_New_DefaultLogger(t *testing.T) {
	c := New(context.Background(), nil)
	if c == nil {
		t.Fatal("New returned nil")
	}
	if c.logger == nil {
		t.Fatal("logger is nil")
	}
}

func TestCron_Add_BadSpec(t *testing.T) {
	c := New(context.Background(), nil)
	if err := c.Add("bad", "not a cron", func(context.Context) error { return nil }); err == nil {
		t.Fatal("Add bad spec error = nil, want non-nil")
	}
}

func TestCron_AddStart_Stop_NoEntry(t *testing.T) {
	c := New(context.Background(), nil)
	c.Start()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if err := c.Stop(ctx); err != nil {
		t.Fatalf("Stop no-entry: %v", err)
	}
}

func TestCron_Entries(t *testing.T) {
	c := New(context.Background(), nil)
	if err := c.Add("one", "0 3 * * *", func(context.Context) error { return nil }); err != nil {
		t.Fatalf("add one: %v", err)
	}
	if err := c.Add("two", "0 4 * * *", func(context.Context) error { return nil }); err != nil {
		t.Fatalf("add two: %v", err)
	}
	entries := c.Entries()
	if len(entries) != 2 {
		t.Fatalf("entries len = %d, want 2", len(entries))
	}
	got := map[string]string{}
	for _, entry := range entries {
		got[entry.Name] = entry.Spec
	}
	if got["one"] != "0 3 * * *" || got["two"] != "0 4 * * *" {
		t.Fatalf("entries = %#v, want one/two specs", entries)
	}
}
