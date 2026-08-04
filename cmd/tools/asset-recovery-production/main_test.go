package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"workflow/service"
)

func validOptions(t *testing.T) options {
	t.Helper()
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "sources")
	reportRoot := filepath.Join(root, "reports")
	if err := os.Mkdir(sourceRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(reportRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	return options{
		DSN:               "user:pass@tcp(127.0.0.1:3306)/workflow",
		Mode:              "apply",
		PlanFile:          filepath.Join(root, "plan.json"),
		SourceRoot:        sourceRoot,
		ReportFile:        filepath.Join(reportRoot, "report.json"),
		CutoverMarker:     filepath.Join(root, "marker.env"),
		ConfirmDatabase:   "workflow",
		ConfirmHost:       "127.0.0.1",
		ConfirmRunID:      "production-v1295",
		ConfirmRelease:    "v1.295",
		ConfirmCommit:     strings.Repeat("a", 40),
		ConfirmProduction: confirmPhrase,
	}
}

func TestValidateOptionsRequiresProductionIdentityAndLoopback(t *testing.T) {
	o := validOptions(t)
	if _, _, err := validateOptions(o); err != nil {
		t.Fatalf("valid production options rejected: %v", err)
	}
	o.DSN = "user:pass@tcp(127.0.0.1:3306)/ab_test_b"
	o.ConfirmDatabase = "ab_test_b"
	if _, _, err := validateOptions(o); err == nil ||
		!strings.Contains(err.Error(), "Clone B") {
		t.Fatalf("expected Clone B rejection, got %v", err)
	}
	o = validOptions(t)
	o.DSN = "user:pass@tcp(192.168.0.20:3306)/workflow"
	o.ConfirmHost = "192.168.0.20"
	if _, _, err := validateOptions(o); err == nil ||
		!strings.Contains(err.Error(), "non-loopback") {
		t.Fatalf("expected non-loopback rejection, got %v", err)
	}
	o = validOptions(t)
	o.ConfirmProduction = ""
	if _, _, err := validateOptions(o); err == nil ||
		!strings.Contains(err.Error(), "confirm-production") {
		t.Fatalf("expected phrase rejection, got %v", err)
	}
}

func TestCutoverMarkerRequiresExactCommitAndRealFile(t *testing.T) {
	root := t.TempDir()
	commit := strings.Repeat("b", 40)
	path := filepath.Join(root, "cutover.env")
	if err := os.WriteFile(
		path,
		[]byte("APPROVED_COMMIT="+commit+"\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if err := validateCutoverMarker(path, commit); err != nil {
		t.Fatalf("exact marker rejected: %v", err)
	}
	if err := validateCutoverMarker(path, strings.Repeat("c", 40)); err == nil {
		t.Fatal("expected stale marker rejection")
	}
	link := filepath.Join(root, "marker-link")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if err := validateCutoverMarker(link, commit); err == nil {
		t.Fatal("expected symlink marker rejection")
	}
}

func TestValidatePlanBytesChecksPythonCompatibleSelfHash(t *testing.T) {
	object := map[string]any{
		"version":                    float64(1),
		"status":                     "PREPARED",
		"run_id":                     "production-v1295",
		"target_environment":         targetEnvironment,
		"production_release":         "v1.295",
		"mapping_sha256":             strings.Repeat("d", 64),
		"database_writes_executed":   false,
		"production_writes_executed": false,
		"entries":                    []any{},
	}
	canonical, err := json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	evidence := sha256Hex(canonical)
	object["evidence_sha256"] = evidence
	raw, err := json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	var plan recoveryPlan
	if err := json.Unmarshal(raw, &plan); err != nil {
		t.Fatal(err)
	}
	if err := validatePlanBytes(raw, plan); err != nil {
		t.Fatalf("valid self-bound plan rejected: %v", err)
	}
	object["run_id"] = "drift"
	raw, _ = json.Marshal(object)
	_ = json.Unmarshal(raw, &plan)
	if err := validatePlanBytes(raw, plan); err == nil {
		t.Fatal("expected stale evidence rejection")
	}
}

type memoryObjectStore struct {
	objects map[string][]byte
}

func (s *memoryObjectStore) UploadObjectFromReader(
	_ context.Context,
	key string,
	_ string,
	body io.Reader,
) error {
	value, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	s.objects[key] = value
	return nil
}

func (s *memoryObjectStore) StatObject(
	_ context.Context,
	key string,
) (*service.OSSObjectInfo, bool, error) {
	value, ok := s.objects[key]
	if !ok {
		return nil, false, nil
	}
	return &service.OSSObjectInfo{
		ContentLength: int64(len(value)),
		ContentType:   "application/octet-stream",
	}, true, nil
}

func (s *memoryObjectStore) OpenObject(
	_ context.Context,
	key string,
) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(string(s.objects[key]))), nil
}

func (s *memoryObjectStore) DeleteObject(
	_ context.Context,
	key string,
) error {
	delete(s.objects, key)
	return nil
}

func TestVerifiedObjectDeletionRefusesUnrelatedContent(t *testing.T) {
	entry := recoveryEntry{
		MissingTaskAssetID: 23989,
		SourceSHA256:       strings.Repeat("0", 64),
		SourceSize:         3,
		TargetObjectKey:    "target",
	}
	store := &memoryObjectStore{
		objects: map[string][]byte{"target": []byte("bad")},
	}
	deleted, err := deleteVerifiedObjectIfPresent(
		context.Background(),
		store,
		entry,
	)
	if err == nil || deleted {
		t.Fatalf("expected hash mismatch without deletion, got deleted=%v err=%v", deleted, err)
	}
	if _, exists := store.objects["target"]; !exists {
		t.Fatal("unrelated object was deleted")
	}
}
