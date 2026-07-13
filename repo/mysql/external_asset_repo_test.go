package mysqlrepo

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"

	"workflow/domain"
)

func TestPendingExternalAssetQueriesScopeMountsAndBackOffFailures(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expected string, actual string) error {
		requiredParts := []string{
			"mount_path IN (?,?)",
			"updated_at <= UTC_TIMESTAMP() - INTERVAL 10 MINUTE",
		}
		if strings.Contains(expected, "OSS") {
			requiredParts = append(requiredParts, "kind = 'nas_local'", "status <> 'missing'")
		}
		for _, required := range requiredParts {
			if !strings.Contains(actual, required) {
				return fmt.Errorf("query missing %q: %s", required, actual)
			}
		}
		return nil
	})))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("pending OSS").
		WithArgs("/p3", "/quark", 25).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery("pending preview").
		WithArgs("/p3", "/quark", 25).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	repository := NewExternalAssetRepo(New(db))
	mounts := []string{"/p3", "/quark"}
	if _, err := repository.ListPendingOSS(context.Background(), mounts, 25); err != nil {
		t.Fatalf("ListPendingOSS() error = %v", err)
	}
	if _, err := repository.ListPendingPreview(context.Background(), mounts, 25); err != nil {
		t.Fatalf("ListPendingPreview() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations = %v", err)
	}
}

func TestClaimPendingOSSUsesLockedLeaseSelection(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expected, actual string) error {
		normalized := strings.Join(strings.Fields(actual), " ")
		for _, fragment := range []string{
			"oss_sync_status = 'uploading' AND updated_at <= ?",
			"FOR UPDATE SKIP LOCKED",
			"status <> 'missing'",
		} {
			if !strings.Contains(normalized, fragment) {
				return fmt.Errorf("claim query missing %q: %s", fragment, normalized)
			}
		}
		return nil
	})))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	leaseBefore := time.Date(2026, 7, 12, 20, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery("claim pending OSS").
		WithArgs("/p3", leaseBefore, 25).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectCommit()

	repository := NewExternalAssetRepo(New(db))
	rows, err := repository.ClaimPendingOSSPrioritized(context.Background(), nil, []string{"/p3"}, 25, leaseBefore)
	if err != nil {
		t.Fatalf("ClaimPendingOSSPrioritized() error = %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("rows = %+v, want empty", rows)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations = %v", err)
	}
}

func TestClaimedOSSCompletionUsesClaimTokenCAS(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectExec("UPDATE external_asset_records").
		WithArgs("object/key.jpg", int64(91), "claim:token-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE external_asset_records").
		WithArgs("upload failed", int64(92), "claim:token-2").
		WillReturnResult(sqlmock.NewResult(0, 0))

	repository := NewExternalAssetRepo(New(db))
	ready, err := repository.MarkClaimedOSSReady(context.Background(), 91, "object/key.jpg", "token-1")
	if err != nil || !ready {
		t.Fatalf("MarkClaimedOSSReady() = %v, %v", ready, err)
	}
	failed, err := repository.MarkClaimedOSSFailed(context.Background(), 92, "token-2", "upload failed")
	if err != nil || failed {
		t.Fatalf("MarkClaimedOSSFailed() = %v, %v; want stale claim rejected", failed, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations = %v", err)
	}
}

func TestBuildExternalAssetWhereUsesFullTextForKeyword(t *testing.T) {
	where, args, orderBy := buildExternalAssetWhere(domain.ExternalAssetSearchQuery{Keyword: "KT poster", Page: 1, Size: 20})

	if !strings.Contains(where, "MATCH(file_name, origin_path, parent_path, searchable_text) AGAINST (? IN BOOLEAN MODE)") {
		t.Fatalf("where clause should use fulltext: %s", where)
	}
	if strings.Contains(where, "file_name LIKE ? OR origin_path LIKE ?") {
		t.Fatalf("keyword clause should not use contains LIKE in fulltext mode: %s", where)
	}
	if len(args) == 0 || args[0] != "+KT* +poster*" {
		t.Fatalf("args = %#v, want fulltext boolean query", args)
	}
	if !strings.Contains(orderBy, "updated_at DESC") {
		t.Fatalf("orderBy = %q, want updated_at order", orderBy)
	}
}

func TestBuildExternalAssetWhereRestrictsVisibleOriginPrefixes(t *testing.T) {
	where, args, _ := buildExternalAssetWhere(domain.ExternalAssetSearchQuery{
		OriginPrefixes: []string{"/p3", "/quark/海报", "/quark/kt板"},
	})
	if got := strings.Count(where, "origin_path = ? OR origin_path LIKE ?"); got != 3 {
		t.Fatalf("visible prefix clauses = %d, want 3: %s", got, where)
	}
	for _, expected := range []string{"/p3", "/p3/%", "/quark/海报", "/quark/海报/%"} {
		if !containsExternalAssetArg(args, expected) {
			t.Fatalf("args missing %q: %#v", expected, args)
		}
	}
}

func containsExternalAssetArg(args []interface{}, expected string) bool {
	for _, arg := range args {
		if value, ok := arg.(string); ok && value == expected {
			return true
		}
	}
	return false
}

func TestIsMySQLLockConflictRecognizesRetryableWriteConflicts(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{err: fmt.Errorf("wrapped: %w", &mysql.MySQLError{Number: 1213}), want: true},
		{err: fmt.Errorf("wrapped: %w", &mysql.MySQLError{Number: 1205}), want: true},
		{err: fmt.Errorf("wrapped: %w", &mysql.MySQLError{Number: 1062}), want: false},
	}
	for _, tc := range cases {
		if got := isMySQLLockConflict(tc.err); got != tc.want {
			t.Fatalf("isMySQLLockConflict(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}

func TestBuildExternalAssetLikeWhereKeepsFallbackContainsMatch(t *testing.T) {
	where, args, _ := buildExternalAssetLikeWhere(domain.ExternalAssetSearchQuery{Keyword: "海报", Page: 1, Size: 20})

	if strings.Contains(where, "MATCH(") {
		t.Fatalf("fallback where should not use fulltext: %s", where)
	}
	if got := strings.Count(where, "LIKE ?"); got < 4 {
		t.Fatalf("LIKE placeholders = %d, want at least 4 in %s", got, where)
	}
	if len(args) < 4 || args[0] != "%海报%" {
		t.Fatalf("args = %#v, want contains fallback args", args)
	}
}

func TestExternalAssetDirectoryPathsIncludeMountAncestors(t *testing.T) {
	paths := externalAssetDirectoryPaths(`/quark/A06杨玲/2025杨玲`, `/quark`)
	want := []string{`/quark`, `/quark/A06杨玲`, `/quark/A06杨玲/2025杨玲`}

	if len(paths) != len(want) {
		t.Fatalf("paths = %#v, want %#v", paths, want)
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Fatalf("paths[%d] = %q, want %q in %#v", i, paths[i], want[i], paths)
		}
	}
}

func TestExternalAssetDirectoryPathsRejectsOutsideMount(t *testing.T) {
	if got := externalAssetDirectoryPaths(`/p3/A`, `/quark`); len(got) != 0 {
		t.Fatalf("paths outside mount = %#v, want empty", got)
	}
}

func TestExternalAssetDirectoryClausesUseParentHash(t *testing.T) {
	clauses, args := externalAssetDirectoryClauses(`/quark/A06杨玲`, []string{`/quark`, `/quark`})

	joined := strings.Join(clauses, " AND ")
	if !strings.Contains(joined, `parent_path_hash = ?`) {
		t.Fatalf("clauses = %#v, want parent_path_hash predicate", clauses)
	}
	if !strings.Contains(joined, `mount_path IN (?)`) {
		t.Fatalf("clauses = %#v, want deduplicated mount predicate", clauses)
	}
	if len(args) != 2 {
		t.Fatalf("args = %#v, want parent hash and one mount", args)
	}
	if args[0] != externalAssetParentPathHash(`/quark/A06杨玲`) || args[1] != `/quark` {
		t.Fatalf("args = %#v, want parent hash and /quark", args)
	}
}

func TestExternalAssetPrepareLimitAllowsServiceBatchSize(t *testing.T) {
	tests := []struct {
		name  string
		limit int
		want  int
	}{
		{name: "default", limit: 0, want: 20},
		{name: "normal", limit: 100, want: 100},
		{name: "service batch", limit: 200, want: 200},
		{name: "larger backfill", limit: 1000, want: 1000},
		{name: "cap", limit: 1500, want: 1000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := externalAssetPrepareLimit(tt.limit); got != tt.want {
				t.Fatalf("externalAssetPrepareLimit(%d) = %d, want %d", tt.limit, got, tt.want)
			}
		})
	}
}

func TestExternalAssetNeedsOSSRealignmentOnSourceFingerprintChange(t *testing.T) {
	oldModified := time.Date(2026, 7, 8, 8, 0, 0, 0, time.UTC)
	newModified := oldModified.Add(time.Hour)
	existing := &externalAssetCountState{
		Kind:           domain.ExternalAssetKindNASLocal,
		FileSize:       1024,
		IsDir:          false,
		Status:         domain.ExternalAssetStatusIndexed,
		OSSSyncStatus:  domain.ExternalAssetOSSStatusReady,
		PreviewStatus:  domain.ExternalAssetPreviewStatusReady,
		OSSOriginalKey: "external-assets/alist/original/p3/hash/file.png",
		OSSPreviewKey:  "external-assets/alist/preview/p3/hash/file.webp",
	}

	sourceFingerprint := &externalAssetSourceFingerprintState{FileSize: 1024, SourceModifiedAt: &oldModified}
	if !externalAssetNeedsOSSRealignment(existing, sourceFingerprint, domain.ExternalAssetUpsert{
		Kind:             domain.ExternalAssetKindNASLocal,
		FileSize:         1024,
		SourceModifiedAt: &newModified,
	}) {
		t.Fatal("modified timestamp change should re-queue NAS OSS copy")
	}
	if !externalAssetNeedsOSSRealignment(existing, sourceFingerprint, domain.ExternalAssetUpsert{
		Kind:             domain.ExternalAssetKindNASLocal,
		FileSize:         2048,
		SourceModifiedAt: &oldModified,
	}) {
		t.Fatal("file size change should re-queue NAS OSS copy")
	}
	if externalAssetNeedsOSSRealignment(existing, sourceFingerprint, domain.ExternalAssetUpsert{
		Kind:             domain.ExternalAssetKindNASLocal,
		FileSize:         1024,
		SourceModifiedAt: &oldModified,
	}) {
		t.Fatal("same source fingerprint should not re-queue OSS copy")
	}
	if externalAssetNeedsOSSRealignment(existing, sourceFingerprint, domain.ExternalAssetUpsert{
		Kind:             domain.ExternalAssetKindNetdisk,
		FileSize:         2048,
		SourceModifiedAt: &newModified,
	}) {
		t.Fatal("netdisk rows should not use NAS OSS realignment")
	}
	if !externalAssetNeedsOSSRealignment(existing, nil, domain.ExternalAssetUpsert{
		Kind:     domain.ExternalAssetKindNASLocal,
		FileSize: 2048,
	}) {
		t.Fatal("without source fingerprint state, file size change should still re-queue OSS copy")
	}
}
