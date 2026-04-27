package scheduler

import (
	"context"
	"testing"
)

func TestDefaultDisabled(t *testing.T) {
	if DefaultConfig().Enabled {
		t.Fatalf("default asset cleanup scheduler enabled = true, want false")
	}
	called := false
	if err := Register(context.Background(), DefaultConfig(), func(context.Context) error {
		called = true
		return nil
	}); err != nil {
		t.Fatalf("Register disabled error = %v", err)
	}
	if called {
		t.Fatalf("disabled scheduler called job")
	}
}

func TestEnabledCallsJobOnce(t *testing.T) {
	calls := 0
	if err := Register(context.Background(), Config{Enabled: true}, func(context.Context) error {
		calls++
		return nil
	}); err != nil {
		t.Fatalf("Register enabled error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}
