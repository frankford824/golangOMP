package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestMainFlow_TopLevelClean(t *testing.T) {
	report := buildFixtureReport(t, "clean")
	if report.Summary.Drift != 0 {
		t.Fatalf("expected clean drift=0, got %#v", report.Summary)
	}
	if got := firstVerdict(report); got != "clean" {
		t.Fatalf("expected clean verdict, got %s", got)
	}
}

func TestMainFlow_FieldDriftSeed(t *testing.T) {
	report := buildFixtureReport(t, "only_openapi")
	if report.Summary.Drift < 1 {
		t.Fatalf("expected drift > 0, got %#v", report.Summary)
	}
	if got := firstVerdict(report); got != "only_in_openapi" {
		t.Fatalf("expected only_in_openapi verdict, got %s", got)
	}
}

func TestMainFlow_OnlyInCode(t *testing.T) {
	report := buildFixtureReport(t, "only_code")
	if report.Summary.Drift < 1 {
		t.Fatalf("expected drift > 0, got %#v", report.Summary)
	}
	if got := firstVerdict(report); got != "only_in_code" {
		t.Fatalf("expected only_in_code verdict, got %s", got)
	}
}

func TestMainFlow_BothDiff(t *testing.T) {
	report := buildFixtureReport(t, "both_diff")
	if report.Summary.Drift < 1 {
		t.Fatalf("expected drift > 0, got %#v", report.Summary)
	}
	if got := firstVerdict(report); got != "both_diff" {
		t.Fatalf("expected both_diff verdict, got %s", got)
	}
}

func TestParseTransportRoutes_RealRepo(t *testing.T) {
	routes, err := ParseTransportRoutes("../../transport/http.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) < 200 {
		t.Fatalf("expected at least 200 routes, got %d", len(routes))
	}
}

func TestFailOnDriftExitCode(t *testing.T) {
	tmp := t.TempDir()
	cmd := exec.Command("go", "run", ".", "--transport", fixturePath("only_openapi", "transport/http.go"), "--handlers", fixturePath("only_openapi", "transport/handler"), "--domain", fixturePath("only_openapi", "domain"), "--openapi", fixturePath("only_openapi", "openapi.yaml"), "--output", filepath.Join(tmp, "out.json"), "--markdown", filepath.Join(tmp, "out.md"), "--fail-on-drift", "true")
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit for drift seed")
	}
}

func TestStructJSONFieldsDetectsOnlyInCode(t *testing.T) {
	dir := t.TempDir()
	goFile := filepath.Join(dir, "domain.go")
	if err := os.WriteFile(goFile, []byte("package domain\ntype Task struct { ID int `json:\"id\"`; XYZ string `json:\"xyz\"` }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fields, err := StructJSONFields(goFile, "Task")
	if err != nil {
		t.Fatal(err)
	}
	onlyCode, onlyOpenAPI := DiffFields(fields, []string{"id"})
	if len(onlyCode) != 1 || onlyCode[0] != "xyz" || len(onlyOpenAPI) != 0 {
		t.Fatalf("unexpected diff: onlyCode=%v onlyOpenAPI=%v", onlyCode, onlyOpenAPI)
	}
}

func buildFixtureReport(t *testing.T, name string) Report {
	t.Helper()
	report, err := BuildReport(fixturePath(name, "transport/http.go"), fixturePath(name, "transport/handler"), fixturePath(name, "domain"), fixturePath(name, "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func fixturePath(name, rel string) string {
	return filepath.Join("testdata", name, rel)
}

func firstVerdict(report Report) string {
	for _, path := range report.Paths {
		if path.Verdict != "mounted_not_found" && path.Verdict != "documented_not_found" {
			return path.Verdict
		}
	}
	return ""
}
