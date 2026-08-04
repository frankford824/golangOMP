package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestValidateOptionsRejectsNonCloneOrNonLoopback(t *testing.T) {
	root := t.TempDir()
	base := options{
		DSN:  "user:pass@tcp(127.0.0.1:3312)/ab_formal_clone_b",
		Mode: "apply", RegistryFile: "registry.json", ManifestFile: "manifest.json",
		FixtureRoot:         "/tmp/v8-ab/run/fixture-upload-b",
		ReportFile:          filepath.Join(root, "report.json"),
		RollbackJournalFile: filepath.Join(root, "rollback-journal.json"),
		ConfirmDatabase:     "ab_formal_clone_b", ConfirmHost: "127.0.0.1",
		ConfirmRunID: "formal-run-1", ConfirmCandidateSHA256: strings.Repeat("a", 64),
	}
	if _, _, err := validateOptions(base); err != nil {
		t.Fatalf("valid clone options: %v", err)
	}
	remote := base
	remote.DSN = "user:pass@tcp(10.0.0.2:3306)/ab_formal_clone_b"
	remote.ConfirmHost = "10.0.0.2"
	if _, _, err := validateOptions(remote); err == nil || !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("remote DSN error = %v", err)
	}
	production := base
	production.DSN = "user:pass@tcp(127.0.0.1:3306)/jst_erp"
	production.ConfirmDatabase = "jst_erp"
	if _, _, err := validateOptions(production); err == nil || !strings.Contains(err.Error(), "ab_*_b") {
		t.Fatalf("production DB error = %v", err)
	}
	formal := base
	formal.DSN = "user:pass@tcp(127.0.0.1:3312)/ab_r20260723_01_b"
	formal.ConfirmDatabase = "ab_r20260723_01_b"
	if _, _, err := validateOptions(formal); err != nil {
		t.Fatalf("formal ab_*_b database should be accepted: %v", err)
	}
}

func TestRegistryDecoderAcceptsMaterializerOwnershipReceipt(t *testing.T) {
	unsigned := []byte(`{
		"schema_version":1,
		"status":"MATERIALIZED",
		"run_id":"formal-bundle-run",
		"manifest_sha256":"` + strings.Repeat("a", 64) + `",
		"b_root":"/tmp/v8-ab/formal-bundle-run/fixture-upload-b",
		"database_write_performed":false,
		"write_ahead_sha256":"` + strings.Repeat("c", 64) + `",
		"entries":[{
			"task_id":1,
			"scope_kind":"sku",
			"scope_ref_id":2,
			"revision_no":1,
			"relative_object_path":"objects/source-bundle.zip",
			"object_key":"fixture/source-bundle.zip",
			"bundle_sha256":"` + strings.Repeat("b", 64) + `",
			"size":1,
			"disposition":"created",
			"source_bundle":{},
			"asset_storage_ref_candidate":{},
			"task_asset_candidate":{},
			"rollback_candidate":{
				"task_asset_id":3,
				"storage_ref_id":"bundle-ref",
				"relative_object_path":"objects/source-bundle.zip",
				"expected_sha256":"` + strings.Repeat("b", 64) + `",
				"ownership_receipt_path":"/tmp/v8-ab/formal-bundle-run/bundle-ownership-3.json"
			}
		}]
	}`)
	var document map[string]any
	if err := json.Unmarshal(unsigned, &document); err != nil {
		t.Fatal(err)
	}
	canonicalUnsigned, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	document["evidence_sha256"] = sha256Hex(
		append(canonicalUnsigned, '\n'),
	)
	raw, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	var got registry
	if err := decodeOne(raw, &got); err != nil {
		t.Fatalf("decode registry with ownership receipt: %v", err)
	}
	if err := validateRegistryEvidence(raw, got); err != nil {
		t.Fatalf("validate materializer registry evidence: %v", err)
	}
	if len(got.Entries) != 1 ||
		got.Entries[0].RollbackCandidate.OwnershipReceiptPath !=
			"/tmp/v8-ab/formal-bundle-run/bundle-ownership-3.json" {
		t.Fatalf("ownership receipt was not preserved: %#v", got.Entries)
	}
	unknown := bytes.ReplaceAll(
		raw,
		[]byte(`"ownership_receipt_path"`),
		[]byte(`"ownership_receipt_path_typo"`),
	)
	if err := decodeOne(unknown, &registry{}); err == nil ||
		!strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown registry field was not rejected: %v", err)
	}
	stale := append([]byte(nil), raw...)
	stale = bytes.Replace(
		stale,
		[]byte(`"status":"MATERIALIZED"`),
		[]byte(`"status":"MATERIALIZEd"`),
		1,
	)
	var staleRegistry registry
	if err := decodeOne(stale, &staleRegistry); err != nil {
		t.Fatalf("decode stale registry fixture: %v", err)
	}
	if err := validateRegistryEvidence(stale, staleRegistry); err == nil ||
		!strings.Contains(err.Error(), "stale") {
		t.Fatalf("stale registry evidence was not rejected: %v", err)
	}
}

func TestValidateDocumentsRequiresExactSevenScopesAndBytes(t *testing.T) {
	root := t.TempDir()
	runID := "formal-bundle-run"
	bRoot := filepath.Join(
		root, "tmp", "v8-ab", "formal-20260723-01", "g4-canonical",
		runID, "fixture-upload-b",
	)
	if err := os.MkdirAll(filepath.Join(bRoot, "objects"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := confirmedManifest{
		SchemaVersion: 1, Status: "CONFIRMED", RunID: runID,
		SourceCandidateSHA256: strings.Repeat("a", 64), ConfirmedBy: 1,
		ConfirmedAt: "2026-07-23T10:00:00Z", ConfirmationNote: "administrator confirmed exact members and order",
	}
	reg := registry{
		SchemaVersion: 1, Status: "MATERIALIZED", RunID: runID, BRoot: bRoot,
		DatabaseWritePerformed: false,
	}
	bundleIndex := int64(0)
	for key, memberIDs := range exactScopes {
		parts := strings.Split(key, "/")
		var taskID, scopeRef int64
		var revision int
		if len(parts) != 4 {
			t.Fatal("invalid exact scope fixture")
		}
		if _, err := fmt.Sscanf(parts[0], "%d", &taskID); err != nil {
			t.Fatal(err)
		}
		if _, err := fmt.Sscanf(parts[2], "%d", &scopeRef); err != nil {
			t.Fatal(err)
		}
		if _, err := fmt.Sscanf(parts[3], "%d", &revision); err != nil {
			t.Fatal(err)
		}
		bundleIndex++
		taskAssetID := int64(90000) + bundleIndex
		assetID := int64(91000) + bundleIndex
		refID := fmt.Sprintf("bundle-ref-%d", bundleIndex)
		objectKey := fmt.Sprintf(
			"fixture/%s/migration-bundles/task-%d/sku-%d/revision-%d/source-bundle.zip",
			runID, taskID, scopeRef, revision,
		)
		content := []byte(fmt.Sprintf("bundle-%d", bundleIndex))
		sum := sha256.Sum256(content)
		bundleHash := hex.EncodeToString(sum[:])
		path := filepath.Join(bRoot, "objects", filepath.FromSlash(objectKey))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatal(err)
		}
		targetInfo, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		device, inode, err := fileDeviceInode(targetInfo)
		if err != nil {
			t.Fatal(err)
		}
		receiptPath := filepath.Join(
			filepath.Dir(bRoot),
			fmt.Sprintf("bundle-ownership-%d.json", taskAssetID),
		)
		receipt := map[string]any{
			"schema_version": 1,
			"status":         "OWNED_LINK",
			"run_id":         runID,
			"target_path":    path,
			"staging_path": filepath.Join(
				filepath.Dir(receiptPath),
				fmt.Sprintf(".bundle-stage-%d.zip", taskAssetID),
			),
			"device": device,
			"inode":  inode,
			"size":   int64(len(content)),
			"sha256": bundleHash,
		}
		unsignedReceipt, err := json.Marshal(receipt)
		if err != nil {
			t.Fatal(err)
		}
		receipt["evidence_sha256"] = sha256Hex(
			append(unsignedReceipt, '\n'),
		)
		receiptRaw, err := json.Marshal(receipt)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(receiptPath, receiptRaw, 0o600); err != nil {
			t.Fatal(err)
		}
		var sourceMembers []bundleMember
		var manifestMembers []manifestMember
		for _, memberID := range memberIDs {
			memberHash := fmt.Sprintf("%064x", memberID)
			sourceMembers = append(sourceMembers, bundleMember{TaskAssetID: memberID, SHA256: memberHash, Confirmed: true})
			manifestMembers = append(manifestMembers, manifestMember{
				TaskAssetID: memberID, AssetID: memberID + 100000, TaskID: taskID,
				StorageRefID: fmt.Sprintf("member-ref-%d", memberID), SHA256: memberHash, Confirmed: true,
			})
		}
		manifest.Bundles = append(manifest.Bundles, manifestBundle{
			TaskID: taskID, ScopeKind: "sku", ScopeRefID: scopeRef, RevisionNo: revision,
			BundleTaskAssetID: taskAssetID, BundleAssetID: assetID,
			BundleStorageRefID: refID, Confirmed: true, OrderedMembers: manifestMembers,
		})
		reg.Entries = append(reg.Entries, registryEntry{
			TaskID: taskID, ScopeKind: "sku", ScopeRefID: scopeRef, RevisionNo: revision,
			RelativeObjectPath: filepath.ToSlash(filepath.Join("objects", filepath.FromSlash(objectKey))),
			ObjectKey:          objectKey, BundleSHA256: bundleHash, Size: int64(len(content)), Disposition: "created",
			SourceBundle: sourceBundle{
				TaskAssetID: taskAssetID, Format: "zip", BundleSHA256: bundleHash,
				ManifestSHA256: strings.Repeat("b", 64), Members: sourceMembers,
				ConfirmedBy: 1, ConfirmedAt: manifest.ConfirmedAt,
				ConfirmationNote: manifest.ConfirmationNote,
			},
			AssetStorageRefCandidate: storageRefCandidate{
				RefID: refID, StorageAdapter: "upload_service", RefKey: objectKey,
				FileName: "source-bundle.zip", FileSize: int64(len(content)),
				MIMEType: "application/zip", ChecksumHint: bundleHash, Status: "recorded",
			},
			TaskAssetCandidate: taskAssetCandidate{
				ID: taskAssetID, TaskID: taskID, AssetID: assetID, AssetType: "source",
				ScopeKind: "sku", ScopeRefID: scopeRef, StorageRefID: refID,
				FileName: "source-bundle.zip", MIMEType: "application/zip",
				FileSize: int64(len(content)), StorageKey: objectKey, WholeHash: bundleHash,
				UploadStatus: "uploaded", SourceModuleKey: "migration",
			},
			RollbackCandidate: rollbackCandidate{
				TaskAssetID: taskAssetID, StorageRefID: refID,
				RelativeObjectPath:   filepath.ToSlash(filepath.Join("objects", filepath.FromSlash(objectKey))),
				ExpectedSHA256:       bundleHash,
				OwnershipReceiptPath: receiptPath,
			},
		})
	}
	manifestRaw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	reg.ManifestSHA256 = sha256Hex(manifestRaw)
	o := options{
		FixtureRoot: bRoot, ConfirmRunID: runID,
		ConfirmCandidateSHA256: manifest.SourceCandidateSHA256,
	}
	entries, err := validateDocuments(reg, manifest, o, reg.ManifestSHA256)
	if err != nil {
		t.Fatalf("validateDocuments() error = %v", err)
	}
	if len(entries) != 7 {
		t.Fatalf("entry count = %d", len(entries))
	}
	drifted := reg
	drifted.Entries = append([]registryEntry(nil), reg.Entries...)
	drifted.Entries[0].SourceBundle.Members = append([]bundleMember(nil), reg.Entries[0].SourceBundle.Members...)
	drifted.Entries[0].SourceBundle.Members[0].TaskAssetID++
	if _, err := validateDocuments(drifted, manifest, o, reg.ManifestSHA256); err == nil || !strings.Contains(err.Error(), "member") {
		t.Fatalf("member drift error = %v", err)
	}
	firstTarget := filepath.Join(
		bRoot,
		filepath.FromSlash(reg.Entries[0].RelativeObjectPath),
	)
	firstBytes, err := os.ReadFile(firstTarget)
	if err != nil {
		t.Fatal(err)
	}
	replacement := firstTarget + ".replacement"
	if err := os.WriteFile(replacement, firstBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	originalInfo, err := os.Lstat(firstTarget)
	if err != nil {
		t.Fatal(err)
	}
	replacementInfo, err := os.Lstat(replacement)
	if err != nil {
		t.Fatal(err)
	}
	_, originalInode, err := fileDeviceInode(originalInfo)
	if err != nil {
		t.Fatal(err)
	}
	_, replacementInode, err := fileDeviceInode(replacementInfo)
	if err != nil {
		t.Fatal(err)
	}
	if originalInode == replacementInode {
		t.Fatal("replacement fixture unexpectedly reused the target inode")
	}
	if err := os.Rename(replacement, firstTarget); err != nil {
		t.Fatal(err)
	}
	if _, err := validateDocuments(reg, manifest, o, reg.ManifestSHA256); err == nil ||
		!strings.Contains(err.Error(), "ownership receipt identity") {
		t.Fatalf("same-byte replacement ownership error = %v", err)
	}
}

func TestValidateFixtureBoundaryAllowsNestedFormalRootAndRejectsEscapeOrSymlink(t *testing.T) {
	base := t.TempDir()
	runID := "bundle-materialization-20260723-29"
	nested := filepath.Join(
		base, "tmp", "v8-ab", "formal-20260723-01", "g4-canonical",
		runID, "fixture-upload-b",
	)
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if got, err := validateFixtureBoundary(nested, nested, runID); err != nil || got != nested {
		t.Fatalf("nested formal root = %q, %v", got, err)
	}

	outside := filepath.Join(base, "untrusted", runID, "fixture-upload-b")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := validateFixtureBoundary(outside, outside, runID); err == nil ||
		!strings.Contains(err.Error(), "trusted tmp/v8-ab") {
		t.Fatalf("untrusted ancestor error = %v", err)
	}

	realRunRoot := filepath.Dir(nested)
	aliasRunRoot := filepath.Join(base, "tmp", "v8-ab", runID)
	if err := os.Symlink(realRunRoot, aliasRunRoot); err != nil {
		t.Skipf("symlink setup unavailable: %v", err)
	}
	aliased := filepath.Join(aliasRunRoot, "fixture-upload-b")
	if _, err := validateFixtureBoundary(aliased, aliased, runID); err == nil ||
		!strings.Contains(err.Error(), "symlinks") {
		t.Fatalf("symlink ancestor error = %v", err)
	}
}

func TestValidateGuardRequiresExactBinding(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery("SELECT environment,run_id,candidate_sha256,registry_sha256").
		WillReturnRows(sqlmock.NewRows([]string{"environment", "run_id", "candidate_sha256", "registry_sha256"}).
			AddRow("clone_b", "run-1", strings.Repeat("a", 64), strings.Repeat("b", 64)))
	if err := validateGuard(context.Background(), tx, "run-1", strings.Repeat("a", 64), strings.Repeat("b", 64)); err != nil {
		t.Fatalf("validateGuard() error = %v", err)
	}
	mock.ExpectQuery("SELECT environment,run_id,candidate_sha256,registry_sha256").
		WillReturnRows(sqlmock.NewRows([]string{"environment", "run_id", "candidate_sha256", "registry_sha256"}).
			AddRow("production", "run-1", strings.Repeat("a", 64), strings.Repeat("b", 64)))
	if err := validateGuard(context.Background(), tx, "run-1", strings.Repeat("a", 64), strings.Repeat("b", 64)); err == nil {
		t.Fatal("production guard unexpectedly accepted")
	}
	mock.ExpectRollback()
	_ = tx.Rollback()
}

func TestApplyIsExactIdempotentAndRollbackRestoresMemberHashes(t *testing.T) {
	entry, manifest := sqlFixture()

	t.Run("apply", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		mock.ExpectBegin()
		tx, err := db.BeginTx(context.Background(), nil)
		if err != nil {
			t.Fatal(err)
		}
		expectScope(mock, entry)
		expectMembers(mock, entry, false)
		expectBundleState(mock, entry, manifest.ConfirmedBy, false)
		for _, member := range entry.manifest.OrderedMembers {
			mock.ExpectExec("UPDATE task_assets SET whole_hash=").
				WithArgs(member.SHA256, member.TaskAssetID).
				WillReturnResult(sqlmock.NewResult(0, 1))
		}
		mock.ExpectExec("INSERT INTO design_assets").WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec("INSERT INTO task_assets").WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec("INSERT INTO asset_storage_refs").
			WillReturnResult(sqlmock.NewResult(0, 1))
		journalCalled := false
		changed, already, before, err := applyAll(
			context.Background(), tx, []validatedEntry{entry}, manifest,
			func(before []memberBefore) error {
				journalCalled = true
				if len(before) != 2 {
					t.Fatalf("journal before count = %d", len(before))
				}
				return nil
			},
		)
		if err != nil || changed != 1 || already != 0 || len(before) != 2 {
			t.Fatalf("applyAll = changed=%d already=%d before=%d err=%v", changed, already, len(before), err)
		}
		if !journalCalled {
			t.Fatal("journal callback was not called before mutation")
		}
		mock.ExpectRollback()
		_ = tx.Rollback()
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("idempotent", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		mock.ExpectBegin()
		tx, _ := db.BeginTx(context.Background(), nil)
		expectScope(mock, entry)
		expectMembers(mock, entry, true)
		expectBundleState(mock, entry, manifest.ConfirmedBy, true)
		changed, already, before, err := applyAll(
			context.Background(), tx, []validatedEntry{entry}, manifest,
			func([]memberBefore) error {
				t.Fatal("idempotent apply must not rewrite rollback journal")
				return nil
			},
		)
		if err != nil || changed != 0 || already != 1 || len(before) != 0 {
			t.Fatalf("idempotent apply = changed=%d already=%d before=%d err=%v", changed, already, len(before), err)
		}
		mock.ExpectRollback()
		_ = tx.Rollback()
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("rollback", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		mock.ExpectBegin()
		tx, _ := db.BeginTx(context.Background(), nil)
		expectScope(mock, entry)
		expectMembers(mock, entry, true)
		expectBundleState(mock, entry, manifest.ConfirmedBy, true)
		mock.ExpectQuery("SELECT.*task_asset_group_revisions").
			WithArgs(entry.registry.TaskAssetCandidate.ID, entry.registry.TaskAssetCandidate.ID).
			WillReturnRows(sqlmock.NewRows([]string{"references"}).AddRow(0))
		mock.ExpectExec("DELETE FROM asset_storage_refs").WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec("DELETE FROM task_assets").WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec("DELETE FROM design_assets").WillReturnResult(sqlmock.NewResult(0, 1))
		for _, member := range entry.manifest.OrderedMembers {
			mock.ExpectExec("UPDATE task_assets SET whole_hash=NULL").
				WithArgs(member.TaskAssetID, member.SHA256).
				WillReturnResult(sqlmock.NewResult(0, 1))
		}
		report := executionReport{ChangedBundleCount: 1, MemberBefore: []memberBefore{
			{TaskAssetID: 293, RecoveredHash: entry.manifest.OrderedMembers[0].SHA256},
			{TaskAssetID: 297, RecoveredHash: entry.manifest.OrderedMembers[1].SHA256},
		}}
		changed, already, err := rollbackAll(context.Background(), tx, []validatedEntry{entry}, report)
		if err != nil || changed != 1 || already != 0 {
			t.Fatalf("rollbackAll = changed=%d already=%d err=%v", changed, already, err)
		}
		mock.ExpectRollback()
		_ = tx.Rollback()
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestCommittedApplyReportFailureCompensationRestoresDatabase(t *testing.T) {
	entry, manifest := sqlFixture()
	manifest.SourceCandidateSHA256 = strings.Repeat("a", 64)
	registrySHA := strings.Repeat("b", 64)
	reg := registry{RunID: "run-1"}
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(
		"SELECT environment,run_id,candidate_sha256,registry_sha256",
	).WillReturnRows(
		sqlmock.NewRows(
			[]string{
				"environment",
				"run_id",
				"candidate_sha256",
				"registry_sha256",
			},
		).AddRow(
			"clone_b",
			reg.RunID,
			manifest.SourceCandidateSHA256,
			registrySHA,
		),
	)
	expectScope(mock, entry)
	expectMembers(mock, entry, true)
	expectBundleState(mock, entry, manifest.ConfirmedBy, true)
	mock.ExpectQuery("SELECT.*task_asset_group_revisions").
		WithArgs(
			entry.registry.TaskAssetCandidate.ID,
			entry.registry.TaskAssetCandidate.ID,
		).
		WillReturnRows(sqlmock.NewRows([]string{"references"}).AddRow(0))
	mock.ExpectExec("DELETE FROM asset_storage_refs").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM task_assets").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM design_assets").
		WillReturnResult(sqlmock.NewResult(0, 1))
	for _, member := range entry.manifest.OrderedMembers {
		mock.ExpectExec("UPDATE task_assets SET whole_hash=NULL").
			WithArgs(member.TaskAssetID, member.SHA256).
			WillReturnResult(sqlmock.NewResult(0, 1))
	}
	mock.ExpectCommit()
	for range 3 {
		expectAutoIncrementStates(mock, 24000, 26000)
	}
	report := executionReport{
		ChangedBundleCount: 1,
		MemberBefore: []memberBefore{
			{
				TaskAssetID:   293,
				RecoveredHash: entry.manifest.OrderedMembers[0].SHA256,
			},
			{
				TaskAssetID:   297,
				RecoveredHash: entry.manifest.OrderedMembers[1].SHA256,
			},
		},
	}
	autoState := []autoIncrementState{
		{Table: "design_assets", NextValue: 24000},
		{Table: "task_assets", NextValue: 26000},
	}
	if err := compensateCommittedApply(
		context.Background(),
		db,
		reg,
		manifest,
		registrySHA,
		[]validatedEntry{entry},
		report,
		rollbackJournal{
			AutoIncrementBefore:   autoState,
			AutoIncrementCeilings: autoState,
		},
	); err != nil {
		t.Fatalf("compensateCommittedApply() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestApplyJournalFailurePrecedesEveryDatabaseMutation(t *testing.T) {
	entry, manifest := sqlFixture()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	tx, _ := db.BeginTx(context.Background(), nil)
	expectScope(mock, entry)
	expectMembers(mock, entry, false)
	expectBundleState(mock, entry, manifest.ConfirmedBy, false)
	journalErr := errors.New("injected durable journal failure")
	changed, already, before, err := applyAll(
		context.Background(),
		tx,
		[]validatedEntry{entry},
		manifest,
		func([]memberBefore) error { return journalErr },
	)
	if !errors.Is(err, journalErr) || changed != 0 || already != 0 || before != nil {
		t.Fatalf("applyAll journal failure = %d/%d %#v %v", changed, already, before, err)
	}
	mock.ExpectRollback()
	_ = tx.Rollback()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("a database mutation occurred before the journal: %v", err)
	}
}

func TestRollbackJournalCoversPreCommitStateAndIsTamperEvident(t *testing.T) {
	entry, manifest := sqlFixture()
	manifest.SourceCandidateSHA256 = strings.Repeat("e", 64)
	manifest.Bundles = []manifestBundle{entry.manifest}
	reg := registry{RunID: "run-1", Entries: []registryEntry{entry.registry}}
	before := []memberBefore{
		{TaskAssetID: 293, RecoveredHash: entry.manifest.OrderedMembers[0].SHA256},
		{TaskAssetID: 297, RecoveredHash: entry.manifest.OrderedMembers[1].SHA256},
	}
	journal, err := newRollbackJournal(
		reg,
		manifest,
		strings.Repeat("f", 64),
		strings.Repeat("d", 64),
		"ab_formal_clone_b",
		"127.0.0.1",
		before,
		[]autoIncrementState{
			{Table: "design_assets", NextValue: 24000},
			{Table: "task_assets", NextValue: 26000},
		},
		[]autoIncrementState{
			{Table: "design_assets", NextValue: 24000},
			{Table: "task_assets", NextValue: 26000},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "rollback-journal.json")
	if err := writeNewJSONDurable(path, journal); err != nil {
		t.Fatal(err)
	}
	first, loaded, err := loadRollbackJournal(
		path,
		reg,
		manifest,
		strings.Repeat("f", 64),
		strings.Repeat("d", 64),
		"ab_formal_clone_b",
		"127.0.0.1",
	)
	if err != nil || loaded.EvidenceSHA256 != journal.EvidenceSHA256 {
		t.Fatalf("loadRollbackJournal() = %v, %v", loaded, err)
	}
	if err := writeNewJSONDurable(path, journal); err != nil {
		t.Fatalf("identical journal reuse failed: %v", err)
	}
	second, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(first, second) {
		t.Fatal("idempotent journal reuse changed durable bytes")
	}
	tampered := append([]byte(nil), second...)
	tampered[0] = ' '
	if err := os.WriteFile(path, tampered, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadRollbackJournal(
		path,
		reg,
		manifest,
		strings.Repeat("f", 64),
		strings.Repeat("d", 64),
		"ab_formal_clone_b",
		"127.0.0.1",
	); err == nil {
		t.Fatal("tampered rollback journal was accepted")
	}
}

func TestRollbackJournalIsIdempotentWhenCommitNeverHappened(t *testing.T) {
	entry, manifest := sqlFixture()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	tx, _ := db.BeginTx(context.Background(), nil)
	expectScope(mock, entry)
	expectMembers(mock, entry, false)
	expectBundleState(mock, entry, manifest.ConfirmedBy, false)
	report := executionReport{
		ChangedBundleCount: 1,
		MemberBefore: []memberBefore{
			{TaskAssetID: 293, RecoveredHash: entry.manifest.OrderedMembers[0].SHA256},
			{TaskAssetID: 297, RecoveredHash: entry.manifest.OrderedMembers[1].SHA256},
		},
	}
	changed, already, err := rollbackAll(
		context.Background(), tx, []validatedEntry{entry}, report,
	)
	if err != nil || changed != 0 || already != 1 {
		t.Fatalf("pre-commit rollback = %d/%d %v", changed, already, err)
	}
	mock.ExpectRollback()
	_ = tx.Rollback()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestBundleAutoIncrementRollbackRestoresOnlyWithinJournalCeilings(t *testing.T) {
	before := []autoIncrementState{
		{Table: "design_assets", NextValue: 23000},
		{Table: "task_assets", NextValue: 25000},
	}
	reg := registry{Entries: []registryEntry{
		{
			TaskAssetCandidate: taskAssetCandidate{
				ID: 25563, AssetID: 23995,
			},
		},
	}}
	ceilings, err := bundleAutoIncrementCeilings(before, reg)
	if err != nil {
		t.Fatal(err)
	}
	if ceilings[0].NextValue != 23996 || ceilings[1].NextValue != 25564 {
		t.Fatalf("ceilings = %#v", ceilings)
	}
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	expectAutoIncrementStates(mock, 23996, 25564)
	mock.ExpectQuery("SELECT COALESCE\\(MAX\\(id\\),0\\) FROM `design_assets`").
		WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(22999))
	mock.ExpectExec("ALTER TABLE `design_assets` AUTO_INCREMENT = 23000").
		WillReturnResult(sqlmock.NewResult(0, 0))
	expectAutoIncrementStates(mock, 23000, 25564)
	mock.ExpectQuery("SELECT COALESCE\\(MAX\\(id\\),0\\) FROM `task_assets`").
		WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(24999))
	mock.ExpectExec("ALTER TABLE `task_assets` AUTO_INCREMENT = 25000").
		WillReturnResult(sqlmock.NewResult(0, 0))
	expectAutoIncrementStates(mock, 23000, 25000)
	if err := restoreBundleAutoIncrementStates(
		context.Background(), db, before, ceilings,
	); err != nil {
		t.Fatalf("restoreBundleAutoIncrementStates() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func sqlFixture() (validatedEntry, confirmedManifest) {
	hashA := strings.Repeat("a", 64)
	hashB := strings.Repeat("b", 64)
	entry := validatedEntry{
		registry: registryEntry{
			TaskID: 485, ScopeKind: "sku", ScopeRefID: 365, RevisionNo: 1,
			ObjectKey:    "fixture/run-1/migration-bundles/task-485/sku-365/revision-1/source-bundle.zip",
			BundleSHA256: strings.Repeat("c", 64), Size: 1234,
			SourceBundle:             sourceBundle{TaskAssetID: 9001, ConfirmedBy: 1},
			TaskAssetCandidate:       taskAssetCandidate{ID: 9001},
			AssetStorageRefCandidate: storageRefCandidate{RefID: "bundle-ref-9001"},
		},
		manifest: manifestBundle{
			TaskID: 485, ScopeKind: "sku", ScopeRefID: 365, RevisionNo: 1,
			BundleTaskAssetID: 9001, BundleAssetID: 9002, BundleStorageRefID: "bundle-ref-9001",
			OrderedMembers: []manifestMember{
				{TaskAssetID: 293, AssetID: 441, TaskID: 485, StorageRefID: "member-ref-293", SHA256: hashA},
				{TaskAssetID: 297, AssetID: 445, TaskID: 485, StorageRefID: "member-ref-297", SHA256: hashB},
			},
		},
		scopeSKUCode: "NSKT000181",
		confirmedAt:  time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC),
		manifestSHA:  strings.Repeat("d", 64),
	}
	return entry, confirmedManifest{ConfirmedBy: 1}
}

func expectScope(mock sqlmock.Sqlmock, entry validatedEntry) {
	mock.ExpectQuery("SELECT sku_code FROM task_sku_items").
		WithArgs(entry.registry.ScopeRefID, entry.registry.TaskID).
		WillReturnRows(sqlmock.NewRows([]string{"sku_code"}).AddRow(entry.scopeSKUCode))
}

func expectAutoIncrementStates(
	mock sqlmock.Sqlmock, designNext int64, taskNext int64,
) {
	mock.ExpectQuery("SELECT @@SESSION.information_schema_stats_expiry").
		WillReturnRows(
			sqlmock.NewRows(
				[]string{
					"information_schema_stats_expiry",
					"auto_increment_increment",
					"auto_increment_offset",
				},
			).AddRow(0, 1, 1),
		)
	for _, value := range []int64{designNext, taskNext} {
		mock.ExpectQuery("SELECT AUTO_INCREMENT FROM information_schema.tables").
			WithArgs(sqlmock.AnyArg()).
			WillReturnRows(
				sqlmock.NewRows([]string{"AUTO_INCREMENT"}).AddRow(value),
			)
	}
}

func expectMembers(mock sqlmock.Sqlmock, entry validatedEntry, applied bool) {
	for _, member := range entry.manifest.OrderedMembers {
		var wholeHash any
		if applied {
			wholeHash = member.SHA256
		}
		mock.ExpectQuery("SELECT id,task_id,asset_id,COALESCE\\(storage_ref_id,''\\),whole_hash,asset_type").
			WithArgs(member.TaskAssetID).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "task_id", "asset_id", "storage_ref_id", "whole_hash", "asset_type",
				"deleted_at", "cleaned_at", "object_deleted_at",
			}).AddRow(member.TaskAssetID, entry.registry.TaskID, member.AssetID, member.StorageRefID,
				wholeHash, "source", nil, nil, nil))
		mock.ExpectQuery("SELECT asset_id,owner_type,owner_id,ref_key,status").
			WithArgs(member.StorageRefID).
			WillReturnRows(sqlmock.NewRows([]string{"asset_id", "owner_type", "owner_id", "ref_key", "status"}).
				AddRow(member.TaskAssetID, "task_asset", member.TaskAssetID, "legacy/source.psd", "recorded"))
	}
}

func expectBundleState(mock sqlmock.Sqlmock, entry validatedEntry, reviewer int64, exact bool) {
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM design_assets").
		WithArgs(entry.registry.TaskID, bundleAssetNo(entry.registry.TaskAssetCandidate.ID), entry.manifest.BundleAssetID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM task_assets").
		WithArgs(entry.registry.TaskID, entry.registry.TaskAssetCandidate.ID, entry.registry.TaskAssetCandidate.ID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM asset_storage_refs").
		WithArgs(entry.registry.ObjectKey, entry.registry.AssetStorageRefCandidate.RefID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	if !exact {
		mock.ExpectQuery("SELECT task_id,asset_no,scope_sku_code,asset_type,current_version_id,created_by").
			WithArgs(entry.manifest.BundleAssetID).WillReturnError(sql.ErrNoRows)
		mock.ExpectQuery("SELECT task_id,asset_id,COALESCE\\(scope_sku_code,''\\),asset_type,binding_state").
			WithArgs(entry.registry.TaskAssetCandidate.ID).WillReturnError(sql.ErrNoRows)
		mock.ExpectQuery("SELECT asset_id,owner_type,owner_id,storage_adapter,ref_type,ref_key,file_name").
			WithArgs(entry.registry.AssetStorageRefCandidate.RefID).WillReturnError(sql.ErrNoRows)
		return
	}
	mock.ExpectQuery("SELECT task_id,asset_no,scope_sku_code,asset_type,current_version_id,created_by").
		WithArgs(entry.manifest.BundleAssetID).
		WillReturnRows(sqlmock.NewRows([]string{
			"task_id", "asset_no", "scope_sku_code", "asset_type", "current_version_id", "created_by",
		}).AddRow(entry.registry.TaskID, bundleAssetNo(entry.registry.TaskAssetCandidate.ID),
			entry.scopeSKUCode, "source", entry.registry.TaskAssetCandidate.ID, reviewer))
	mock.ExpectQuery("SELECT task_id,asset_id,COALESCE\\(scope_sku_code,''\\),asset_type,binding_state").
		WithArgs(entry.registry.TaskAssetCandidate.ID).
		WillReturnRows(sqlmock.NewRows([]string{
			"task_id", "asset_id", "scope_sku_code", "asset_type", "binding_state",
			"version_no", "asset_version_no", "storage_ref_id", "file_name", "mime_type",
			"file_size", "storage_key", "whole_hash", "upload_status", "preview_status",
			"uploaded_by", "source_module_key", "remark",
		}).AddRow(
			entry.registry.TaskID, entry.manifest.BundleAssetID, entry.scopeSKUCode, "source", "legacy",
			entry.registry.TaskAssetCandidate.ID, 1, entry.registry.AssetStorageRefCandidate.RefID,
			"source-bundle.zip", "application/zip", entry.registry.Size, entry.registry.ObjectKey,
			entry.registry.BundleSHA256, "uploaded", "not_applicable", reviewer, "migration",
			bundleRemark(entry.registry, entry.manifestSHA),
		))
	mock.ExpectQuery("SELECT asset_id,owner_type,owner_id,storage_adapter,ref_type,ref_key,file_name").
		WithArgs(entry.registry.AssetStorageRefCandidate.RefID).
		WillReturnRows(sqlmock.NewRows([]string{
			"asset_id", "owner_type", "owner_id", "storage_adapter", "ref_type", "ref_key",
			"file_name", "mime_type", "file_size", "is_placeholder", "checksum_hint", "status",
		}).AddRow(
			entry.registry.TaskAssetCandidate.ID, "task_asset", entry.registry.TaskAssetCandidate.ID,
			"oss_upload_service", "task_asset_object", entry.registry.ObjectKey,
			"source-bundle.zip", "application/zip", entry.registry.Size, 0,
			entry.registry.BundleSHA256, "recorded",
		))
}
