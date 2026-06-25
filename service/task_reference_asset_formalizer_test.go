package service

import "testing"

func TestNewTaskAssetStorageRefIDFitsSchema(t *testing.T) {
	id := newTaskAssetStorageRefID()
	if id == "" {
		t.Fatal("expected non-empty storage ref id")
	}
	if len(id) > 64 {
		t.Fatalf("storage ref id length = %d, want <= 64", len(id))
	}
}
