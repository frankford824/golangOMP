package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProvisionDoesNotReplaceKeysOrLeakSigningKeyToNASFiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "credentials")
	if err := provision(dir); err != nil {
		t.Fatal(err)
	}
	if err := provision(dir); err == nil {
		t.Fatal("existing keys replaced")
	}
	for _, name := range []string{"signing.key", "ecs.env", "gateway.env", "worker.env"} {
		st, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if st.Mode().Perm() != 0600 {
			t.Errorf("insecure mode for %s", name)
		}
	}
	for _, name := range []string{"gateway.env", "worker.env"} {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "SIGNING_KEY") || strings.Contains(string(raw), "ACCESS_KEY_SECRET") || strings.Contains(string(raw), "DB_PASS") {
			t.Fatalf("broad secret leaked into %s", name)
		}
	}
}
