package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

const guardEnvironment = "clone_b"

var (
	sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
	runIDPattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{2,127}$`)
)

var exactScopes = map[string][]int64{
	"485/sku/365/1":   {293, 297},
	"523/sku/398/1":   {402, 403, 404, 405},
	"523/sku/400/1":   {358, 359, 360, 361},
	"2234/sku/2401/2": {12672, 12673},
	"2251/sku/2417/2": {13103, 13104, 13105, 13106, 13107},
	"2477/sku/2725/2": {18989, 18991, 18993},
	"2598/sku/2869/2": {20799, 20802},
}

type options struct {
	DSN                    string
	Mode                   string
	RegistryFile           string
	ManifestFile           string
	FixtureRoot            string
	ApplyReportFile        string
	ReportFile             string
	ConfirmDatabase        string
	ConfirmHost            string
	ConfirmRunID           string
	ConfirmCandidateSHA256 string
}

type registry struct {
	SchemaVersion          int             `json:"schema_version"`
	Status                 string          `json:"status"`
	RunID                  string          `json:"run_id"`
	ManifestSHA256         string          `json:"manifest_sha256"`
	BRoot                  string          `json:"b_root"`
	DatabaseWritePerformed bool            `json:"database_write_performed"`
	Entries                []registryEntry `json:"entries"`
}

type registryEntry struct {
	TaskID                   int64               `json:"task_id"`
	ScopeKind                string              `json:"scope_kind"`
	ScopeRefID               int64               `json:"scope_ref_id"`
	RevisionNo               int                 `json:"revision_no"`
	RelativeObjectPath       string              `json:"relative_object_path"`
	ObjectKey                string              `json:"object_key"`
	BundleSHA256             string              `json:"bundle_sha256"`
	Size                     int64               `json:"size"`
	Disposition              string              `json:"disposition"`
	SourceBundle             sourceBundle        `json:"source_bundle"`
	AssetStorageRefCandidate storageRefCandidate `json:"asset_storage_ref_candidate"`
	TaskAssetCandidate       taskAssetCandidate  `json:"task_asset_candidate"`
	RollbackCandidate        rollbackCandidate   `json:"rollback_candidate"`
}

type sourceBundle struct {
	TaskAssetID      int64          `json:"task_asset_id"`
	Format           string         `json:"format"`
	BundleSHA256     string         `json:"bundle_sha256"`
	ManifestSHA256   string         `json:"manifest_sha256"`
	Members          []bundleMember `json:"members"`
	ConfirmedBy      int64          `json:"confirmed_by"`
	ConfirmedAt      string         `json:"confirmed_at"`
	ConfirmationNote string         `json:"confirmation_note"`
}

type bundleMember struct {
	TaskAssetID int64  `json:"task_asset_id"`
	SHA256      string `json:"sha256"`
	Confirmed   bool   `json:"confirmed"`
}

type storageRefCandidate struct {
	RefID          string `json:"ref_id"`
	StorageAdapter string `json:"storage_adapter"`
	RefKey         string `json:"ref_key"`
	FileName       string `json:"file_name"`
	FileSize       int64  `json:"file_size"`
	MIMEType       string `json:"mime_type"`
	ChecksumHint   string `json:"checksum_hint"`
	Status         string `json:"status"`
	IsPlaceholder  bool   `json:"is_placeholder"`
}

type taskAssetCandidate struct {
	ID              int64  `json:"id"`
	TaskID          int64  `json:"task_id"`
	AssetID         int64  `json:"asset_id"`
	AssetType       string `json:"asset_type"`
	ScopeKind       string `json:"scope_kind"`
	ScopeRefID      int64  `json:"scope_ref_id"`
	StorageRefID    string `json:"storage_ref_id"`
	FileName        string `json:"file_name"`
	MIMEType        string `json:"mime_type"`
	FileSize        int64  `json:"file_size"`
	StorageKey      string `json:"storage_key"`
	WholeHash       string `json:"whole_hash"`
	UploadStatus    string `json:"upload_status"`
	SourceModuleKey string `json:"source_module_key"`
}

type rollbackCandidate struct {
	TaskAssetID        int64  `json:"task_asset_id"`
	StorageRefID       string `json:"storage_ref_id"`
	RelativeObjectPath string `json:"relative_object_path"`
	ExpectedSHA256     string `json:"expected_sha256"`
}

type confirmedManifest struct {
	SchemaVersion         int              `json:"schema_version"`
	Status                string           `json:"status"`
	RunID                 string           `json:"run_id"`
	SourceCandidateSHA256 string           `json:"source_candidate_sha256"`
	ConfirmedBy           int64            `json:"confirmed_by"`
	ConfirmedAt           string           `json:"confirmed_at"`
	ConfirmationNote      string           `json:"confirmation_note"`
	Bundles               []manifestBundle `json:"bundles"`
}

type manifestBundle struct {
	TaskID             int64            `json:"task_id"`
	ScopeKind          string           `json:"scope_kind"`
	ScopeRefID         int64            `json:"scope_ref_id"`
	RevisionNo         int              `json:"revision_no"`
	BundleTaskAssetID  int64            `json:"bundle_task_asset_id"`
	BundleAssetID      int64            `json:"bundle_asset_id"`
	BundleStorageRefID string           `json:"bundle_storage_ref_id"`
	Confirmed          bool             `json:"confirmed"`
	OrderedMembers     []manifestMember `json:"ordered_members"`
}

type manifestMember struct {
	TaskAssetID  int64  `json:"task_asset_id"`
	AssetID      int64  `json:"asset_id"`
	TaskID       int64  `json:"task_id"`
	StorageRefID string `json:"storage_ref_id"`
	SHA256       string `json:"sha256"`
	Confirmed    bool   `json:"confirmed"`
}

type memberBefore struct {
	TaskAssetID   int64   `json:"task_asset_id"`
	OriginalHash  *string `json:"original_whole_hash"`
	RecoveredHash string  `json:"recovered_whole_hash"`
}

type executionReport struct {
	SchemaVersion       int            `json:"schema_version"`
	Mode                string         `json:"mode"`
	Status              string         `json:"status"`
	RunID               string         `json:"run_id"`
	Database            string         `json:"database"`
	Host                string         `json:"host"`
	CandidateSHA256     string         `json:"candidate_sha256"`
	RegistrySHA256      string         `json:"registry_sha256"`
	ManifestSHA256      string         `json:"manifest_sha256"`
	ApplyReportSHA256   string         `json:"apply_report_sha256,omitempty"`
	ChangedBundleCount  int            `json:"changed_bundle_count"`
	AlreadyAppliedCount int            `json:"already_applied_bundle_count"`
	MemberBefore        []memberBefore `json:"member_before,omitempty"`
	Committed           bool           `json:"database_transaction_committed"`
	ExecutedAt          time.Time      `json:"executed_at"`
}

type transaction interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	Commit() error
	Rollback() error
}

func main() {
	var o options
	flag.StringVar(&o.DSN, "dsn", os.Getenv("CLONE_B_MYSQL_DSN"), "Clone B MySQL DSN")
	flag.StringVar(&o.Mode, "mode", "", "apply or rollback")
	flag.StringVar(&o.RegistryFile, "registry", "", "materializer registry JSON")
	flag.StringVar(&o.ManifestFile, "manifest", "", "administrator-confirmed bundle manifest")
	flag.StringVar(&o.FixtureRoot, "fixture-root", "", "exact materializer b_root")
	flag.StringVar(&o.ApplyReportFile, "apply-report", "", "original apply report; required for rollback")
	flag.StringVar(&o.ReportFile, "report-file", "", "new execution report path")
	flag.StringVar(&o.ConfirmDatabase, "confirm-database", "", "exact Clone B database")
	flag.StringVar(&o.ConfirmHost, "confirm-host", "", "exact loopback DSN host")
	flag.StringVar(&o.ConfirmRunID, "confirm-run-id", "", "exact materializer run id")
	flag.StringVar(&o.ConfirmCandidateSHA256, "confirm-candidate-sha256", "", "exact reviewed candidate SHA-256")
	flag.Parse()
	if err := run(context.Background(), o); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, o options) error {
	cfg, host, err := validateOptions(o)
	if err != nil {
		return err
	}
	registryRaw, reg, manifestRaw, manifest, entries, err := loadAndValidateInputs(o)
	if err != nil {
		return err
	}
	registrySHA := sha256Hex(registryRaw)
	manifestSHA := sha256Hex(manifestRaw)

	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("connect Clone B: %w", err)
	}
	var database string
	if err := db.QueryRowContext(ctx, `SELECT DATABASE()`).Scan(&database); err != nil {
		return err
	}
	if database != o.ConfirmDatabase || database != cfg.DBName {
		return fmt.Errorf("database identity mismatch: connected=%q confirmed=%q dsn=%q", database, o.ConfirmDatabase, cfg.DBName)
	}

	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := validateGuard(ctx, tx, reg.RunID, manifest.SourceCandidateSHA256, registrySHA); err != nil {
		return err
	}

	report := executionReport{
		SchemaVersion: 1, Mode: o.Mode, Status: "PASS", RunID: reg.RunID,
		Database: database, Host: host, CandidateSHA256: manifest.SourceCandidateSHA256,
		RegistrySHA256: registrySHA, ManifestSHA256: manifestSHA,
	}
	switch o.Mode {
	case "apply":
		changed, already, before, err := applyAll(ctx, tx, entries, manifest)
		if err != nil {
			return err
		}
		report.ChangedBundleCount = changed
		report.AlreadyAppliedCount = already
		report.MemberBefore = before
	case "rollback":
		applyRaw, applyReport, err := loadApplyReport(o.ApplyReportFile, reg, manifest, registrySHA)
		if err != nil {
			return err
		}
		report.ApplyReportSHA256 = sha256Hex(applyRaw)
		changed, already, err := rollbackAll(ctx, tx, entries, applyReport)
		if err != nil {
			return err
		}
		report.ChangedBundleCount = changed
		report.AlreadyAppliedCount = already
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	report.Committed = true
	report.ExecutedAt = time.Now().UTC()
	if err := writeNewJSON(o.ReportFile, report); err != nil {
		if o.Mode == "apply" && report.ChangedBundleCount > 0 {
			compensationErr := compensateCommittedApply(
				ctx, db, reg, manifest, registrySHA, entries, report,
			)
			if compensationErr != nil {
				return fmt.Errorf(
					"write apply report: %v; committed bundle apply compensation failed: %w",
					err, compensationErr,
				)
			}
			return fmt.Errorf(
				"write apply report: %w; committed bundle apply was compensated",
				err,
			)
		}
		return err
	}
	return nil
}

func compensateCommittedApply(
	ctx context.Context,
	db *sql.DB,
	reg registry,
	manifest confirmedManifest,
	registrySHA string,
	entries []validatedEntry,
	report executionReport,
) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := validateGuard(
		ctx,
		tx,
		reg.RunID,
		manifest.SourceCandidateSHA256,
		registrySHA,
	); err != nil {
		return err
	}
	changed, already, err := rollbackAll(ctx, tx, entries, report)
	if err != nil {
		return err
	}
	if changed != len(entries) || already != 0 {
		return fmt.Errorf(
			"bundle apply compensation changed=%d already=%d expected=%d/0",
			changed, already, len(entries),
		)
	}
	return tx.Commit()
}

func validateOptions(o options) (*mysql.Config, string, error) {
	if o.Mode != "apply" && o.Mode != "rollback" {
		return nil, "", errors.New("--mode must be apply or rollback")
	}
	required := map[string]string{
		"--dsn/CLONE_B_MYSQL_DSN": o.DSN, "--registry": o.RegistryFile,
		"--manifest": o.ManifestFile, "--fixture-root": o.FixtureRoot,
		"--report-file": o.ReportFile, "--confirm-database": o.ConfirmDatabase,
		"--confirm-host": o.ConfirmHost, "--confirm-run-id": o.ConfirmRunID,
		"--confirm-candidate-sha256": o.ConfirmCandidateSHA256,
	}
	if o.Mode == "rollback" {
		required["--apply-report"] = o.ApplyReportFile
	}
	for name, value := range required {
		if strings.TrimSpace(value) == "" {
			return nil, "", fmt.Errorf("%s is required", name)
		}
	}
	if !runIDPattern.MatchString(o.ConfirmRunID) || !sha256Pattern.MatchString(o.ConfirmCandidateSHA256) {
		return nil, "", errors.New("run/candidate confirmation is invalid")
	}
	cfg, err := mysql.ParseDSN(o.DSN)
	if err != nil {
		return nil, "", fmt.Errorf("parse Clone B DSN: %w", err)
	}
	if cfg.Net != "tcp" {
		return nil, "", errors.New("only a TCP DSN is accepted")
	}
	host, _, err := net.SplitHostPort(cfg.Addr)
	if err != nil || !isLoopback(host) {
		return nil, "", fmt.Errorf("Clone B DSN must use an explicit loopback host and port")
	}
	if host != o.ConfirmHost {
		return nil, "", errors.New("DSN host does not match --confirm-host")
	}
	if cfg.DBName != o.ConfirmDatabase || !isSafeCloneBDatabaseName(cfg.DBName) {
		return nil, "", fmt.Errorf("database must exactly match --confirm-database and follow ab_*_b Clone B naming")
	}
	cfg.ParseTime = true
	cfg.MultiStatements = false
	return cfg, host, nil
}

func isLoopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func isSafeCloneBDatabaseName(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	return strings.HasPrefix(lower, "ab_") && strings.HasSuffix(lower, "_b")
}

func loadAndValidateInputs(o options) ([]byte, registry, []byte, confirmedManifest, []validatedEntry, error) {
	registryRaw, err := os.ReadFile(o.RegistryFile)
	if err != nil {
		return nil, registry{}, nil, confirmedManifest{}, nil, err
	}
	manifestRaw, err := os.ReadFile(o.ManifestFile)
	if err != nil {
		return nil, registry{}, nil, confirmedManifest{}, nil, err
	}
	var reg registry
	if err := decodeOne(registryRaw, &reg); err != nil {
		return nil, reg, nil, confirmedManifest{}, nil, fmt.Errorf("decode registry: %w", err)
	}
	var manifest confirmedManifest
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		return nil, reg, nil, manifest, nil, fmt.Errorf("decode manifest: %w", err)
	}
	entries, err := validateDocuments(reg, manifest, o, sha256Hex(manifestRaw))
	if err != nil {
		return nil, reg, manifestRaw, manifest, nil, err
	}
	return registryRaw, reg, manifestRaw, manifest, entries, nil
}

func decodeOne(raw []byte, value any) error {
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("trailing JSON values are not allowed")
	}
	return nil
}

type validatedEntry struct {
	registry     registryEntry
	manifest     manifestBundle
	scopeSKUCode string
	confirmedAt  time.Time
	manifestSHA  string
}

func validateFixtureBoundary(fixtureRoot, recordedRoot, runID string) (string, error) {
	if !filepath.IsAbs(fixtureRoot) {
		return "", errors.New("--fixture-root must be absolute")
	}
	root, err := filepath.Abs(fixtureRoot)
	if err != nil {
		return "", err
	}
	recorded, err := filepath.Abs(recordedRoot)
	if err != nil || root != recorded {
		return "", errors.New("--fixture-root must exactly match registry.b_root")
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("--fixture-root must be an existing non-symlink directory")
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil || filepath.Clean(resolved) != filepath.Clean(root) {
		return "", errors.New("--fixture-root and its ancestors must not use symlinks")
	}
	if filepath.Base(root) != "fixture-upload-b" {
		return "", errors.New("--fixture-root must end in fixture-upload-b")
	}
	runRoot := filepath.Dir(root)
	if filepath.Base(runRoot) != runID {
		return "", errors.New("--fixture-root parent must exactly match run_id")
	}
	trustedAncestor := false
	for cursor := filepath.Dir(runRoot); ; {
		parent := filepath.Dir(cursor)
		if filepath.Base(cursor) == "v8-ab" && filepath.Base(parent) == "tmp" {
			trustedAncestor = true
			break
		}
		if parent == cursor {
			break
		}
		cursor = parent
	}
	if !trustedAncestor {
		return "", errors.New("fixture root must be below a trusted tmp/v8-ab ancestor")
	}
	return root, nil
}

func validateDocuments(reg registry, manifest confirmedManifest, o options, manifestSHA string) ([]validatedEntry, error) {
	if reg.SchemaVersion != 1 || reg.Status != "MATERIALIZED" || reg.DatabaseWritePerformed {
		return nil, errors.New("registry must be an un-applied schema_version=1 MATERIALIZED document")
	}
	if manifest.SchemaVersion != 1 || manifest.Status != "CONFIRMED" ||
		manifest.ConfirmedBy <= 0 || strings.TrimSpace(manifest.ConfirmationNote) == "" {
		return nil, errors.New("manifest confirmation metadata is incomplete")
	}
	if reg.RunID != o.ConfirmRunID || manifest.RunID != reg.RunID ||
		manifest.SourceCandidateSHA256 != o.ConfirmCandidateSHA256 ||
		reg.ManifestSHA256 != manifestSHA {
		return nil, errors.New("registry/manifest/run/candidate binding mismatch")
	}
	confirmedAt, err := time.Parse(time.RFC3339, manifest.ConfirmedAt)
	if err != nil {
		return nil, errors.New("manifest.confirmed_at must be RFC3339")
	}
	root, err := validateFixtureBoundary(o.FixtureRoot, reg.BRoot, reg.RunID)
	if err != nil {
		return nil, err
	}
	if len(reg.Entries) != len(exactScopes) || len(manifest.Bundles) != len(exactScopes) {
		return nil, errors.New("registry/manifest must contain the seven exact bundle scopes")
	}
	manifestByScope := map[string]manifestBundle{}
	for _, item := range manifest.Bundles {
		manifestByScope[scopeKey(item.TaskID, item.ScopeKind, item.ScopeRefID, item.RevisionNo)] = item
	}
	seenBundleAssetIDs := map[int64]struct{}{}
	seenRootAssetIDs := map[int64]struct{}{}
	seenRefs := map[string]struct{}{}
	seenMembers := map[int64]struct{}{}
	var result []validatedEntry
	for _, item := range reg.Entries {
		key := scopeKey(item.TaskID, item.ScopeKind, item.ScopeRefID, item.RevisionNo)
		expectedMembers, allowed := exactScopes[key]
		manifestItem, found := manifestByScope[key]
		if !allowed || !found || item.ScopeKind != "sku" {
			return nil, fmt.Errorf("scope %s is outside the exact seven-scope allowlist", key)
		}
		if !manifestItem.Confirmed ||
			item.SourceBundle.TaskAssetID != manifestItem.BundleTaskAssetID ||
			item.TaskAssetCandidate.ID != manifestItem.BundleTaskAssetID ||
			item.TaskAssetCandidate.AssetID != manifestItem.BundleAssetID ||
			item.AssetStorageRefCandidate.RefID != manifestItem.BundleStorageRefID {
			return nil, fmt.Errorf("scope %s bundle identities differ between registry and manifest", key)
		}
		if item.SourceBundle.ConfirmedBy != manifest.ConfirmedBy ||
			item.SourceBundle.ConfirmedAt != manifest.ConfirmedAt ||
			item.SourceBundle.ConfirmationNote != manifest.ConfirmationNote {
			return nil, fmt.Errorf("scope %s bundle confirmation differs from the administrator manifest", key)
		}
		if _, duplicate := seenBundleAssetIDs[manifestItem.BundleTaskAssetID]; duplicate {
			return nil, errors.New("duplicate bundle task_asset id")
		}
		if _, duplicate := seenRootAssetIDs[manifestItem.BundleAssetID]; duplicate {
			return nil, errors.New("duplicate bundle design_asset id")
		}
		if _, duplicate := seenRefs[manifestItem.BundleStorageRefID]; duplicate {
			return nil, errors.New("duplicate bundle storage ref id")
		}
		seenBundleAssetIDs[manifestItem.BundleTaskAssetID] = struct{}{}
		seenRootAssetIDs[manifestItem.BundleAssetID] = struct{}{}
		seenRefs[manifestItem.BundleStorageRefID] = struct{}{}
		if err := validateEntryIdentity(item, manifestItem, reg.RunID, expectedMembers, seenMembers); err != nil {
			return nil, fmt.Errorf("scope %s: %w", key, err)
		}
		if err := validateBundleBytes(root, item); err != nil {
			return nil, fmt.Errorf("scope %s: %w", key, err)
		}
		result = append(result, validatedEntry{
			registry: item, manifest: manifestItem, confirmedAt: confirmedAt.UTC(),
			manifestSHA: manifestSHA,
		})
	}
	if len(seenMembers) != 22 || len(manifestByScope) != len(exactScopes) {
		return nil, errors.New("bundle documents must identify exactly 22 unique members")
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].registry.TaskAssetCandidate.ID < result[j].registry.TaskAssetCandidate.ID
	})
	return result, nil
}

func validateEntryIdentity(item registryEntry, manifest manifestBundle, runID string, expectedMembers []int64, seen map[int64]struct{}) error {
	candidate := item.TaskAssetCandidate
	ref := item.AssetStorageRefCandidate
	if item.TaskID != candidate.TaskID || candidate.AssetType != "source" ||
		candidate.ScopeKind != item.ScopeKind || candidate.ScopeRefID != item.ScopeRefID ||
		candidate.StorageRefID != ref.RefID || candidate.FileName != "source-bundle.zip" ||
		candidate.MIMEType != "application/zip" || candidate.FileSize != item.Size ||
		candidate.StorageKey != item.ObjectKey || candidate.WholeHash != item.BundleSHA256 ||
		candidate.UploadStatus != "uploaded" || candidate.SourceModuleKey != "migration" ||
		item.SourceBundle.Format != "zip" || item.SourceBundle.BundleSHA256 != item.BundleSHA256 ||
		ref.RefKey != item.ObjectKey || ref.FileName != "source-bundle.zip" ||
		ref.FileSize != item.Size || ref.MIMEType != "application/zip" ||
		ref.ChecksumHint != item.BundleSHA256 || ref.Status != "recorded" || ref.IsPlaceholder ||
		!sha256Pattern.MatchString(item.BundleSHA256) || !sha256Pattern.MatchString(item.SourceBundle.ManifestSHA256) {
		return errors.New("bundle task/storage candidate is incomplete or inconsistent")
	}
	if item.Disposition != "created" && item.Disposition != "reused_identical" {
		return errors.New("bundle disposition is invalid")
	}
	expectedObjectKey := fmt.Sprintf(
		"fixture/%s/migration-bundles/task-%d/%s-%d/revision-%d/source-bundle.zip",
		runID, item.TaskID, item.ScopeKind, item.ScopeRefID, item.RevisionNo,
	)
	if item.ObjectKey != expectedObjectKey {
		return errors.New("bundle object key is not the deterministic run-scoped key")
	}
	// The materializer historically emitted "upload_service"; database rows
	// are normalized by this executor to the current domain value
	// "oss_upload_service".
	if ref.StorageAdapter != "upload_service" && ref.StorageAdapter != "oss_upload_service" {
		return errors.New("unexpected materializer storage adapter")
	}
	if item.RollbackCandidate.TaskAssetID != candidate.ID ||
		item.RollbackCandidate.StorageRefID != ref.RefID ||
		item.RollbackCandidate.RelativeObjectPath != item.RelativeObjectPath ||
		item.RollbackCandidate.ExpectedSHA256 != item.BundleSHA256 {
		return errors.New("rollback candidate does not match bundle identity")
	}
	if len(item.SourceBundle.Members) != len(expectedMembers) || len(manifest.OrderedMembers) != len(expectedMembers) {
		return errors.New("member count differs from exact scope allowlist")
	}
	for index, memberID := range expectedMembers {
		sourceMember := item.SourceBundle.Members[index]
		manifestMember := manifest.OrderedMembers[index]
		if sourceMember.TaskAssetID != memberID || manifestMember.TaskAssetID != memberID ||
			manifestMember.TaskID != item.TaskID || !sourceMember.Confirmed || !manifestMember.Confirmed ||
			!sha256Pattern.MatchString(sourceMember.SHA256) || sourceMember.SHA256 != manifestMember.SHA256 ||
			manifestMember.AssetID <= 0 || strings.TrimSpace(manifestMember.StorageRefID) == "" {
			return errors.New("ordered member identity/hash differs from exact confirmed manifest")
		}
		if _, duplicate := seen[memberID]; duplicate {
			return errors.New("member task_asset is reused across bundle scopes")
		}
		seen[memberID] = struct{}{}
	}
	return nil
}

func validateBundleBytes(root string, item registryEntry) error {
	relative := filepath.Clean(filepath.FromSlash(item.RelativeObjectPath))
	if filepath.IsAbs(relative) || relative == "." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return errors.New("relative object path escapes fixture root")
	}
	target := filepath.Join(root, relative)
	resolved, err := filepath.Abs(target)
	if err != nil || (resolved != root && !strings.HasPrefix(resolved, root+string(filepath.Separator))) {
		return errors.New("bundle object path escapes fixture root")
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.Mode().IsRegular() || info.Size() != item.Size {
		return errors.New("materialized bundle object is missing or has wrong size")
	}
	if linkInfo, err := os.Lstat(resolved); err != nil || linkInfo.Mode()&os.ModeSymlink != 0 {
		return errors.New("materialized bundle object must not be a symlink")
	}
	hash, err := sha256File(resolved)
	if err != nil || hash != item.BundleSHA256 {
		return errors.New("materialized bundle object SHA-256 drifted")
	}
	expectedRelative := filepath.ToSlash(filepath.Join("objects", filepath.FromSlash(item.ObjectKey)))
	if filepath.ToSlash(relative) != expectedRelative {
		return errors.New("relative path does not identify object_key below objects/")
	}
	return nil
}

func scopeKey(taskID int64, kind string, refID int64, revision int) string {
	return fmt.Sprintf("%d/%s/%d/%d", taskID, kind, refID, revision)
}

func validateGuard(ctx context.Context, tx transaction, runID, candidateSHA, registrySHA string) error {
	// Provisioned by the Clone-B runner, never by this tool:
	// CREATE TABLE v8_ab_source_bundle_guard (
	//   singleton_id TINYINT PRIMARY KEY,
	//   environment VARCHAR(32) NOT NULL,
	//   run_id VARCHAR(128) NOT NULL,
	//   candidate_sha256 CHAR(64) NOT NULL,
	//   registry_sha256 CHAR(64) NOT NULL
	// );
	var environment, guardedRun, guardedCandidate, guardedRegistry string
	if err := tx.QueryRowContext(ctx, `
		SELECT environment,run_id,candidate_sha256,registry_sha256
		FROM v8_ab_source_bundle_guard
		WHERE singleton_id=1
		FOR UPDATE`).Scan(&environment, &guardedRun, &guardedCandidate, &guardedRegistry); err != nil {
		return fmt.Errorf("source bundle Clone-B guard missing or unreadable: %w", err)
	}
	if environment != guardEnvironment || guardedRun != runID ||
		guardedCandidate != candidateSHA || guardedRegistry != registrySHA {
		return errors.New("source bundle guard does not exactly bind clone_b/run/candidate/registry")
	}
	return nil
}

type memberState struct {
	id              int64
	taskID          int64
	assetID         int64
	storageRefID    string
	wholeHash       sql.NullString
	assetType       string
	deletedAt       sql.NullTime
	cleanedAt       sql.NullTime
	objectDeletedAt sql.NullTime
}

func lockAndValidateMembers(ctx context.Context, tx transaction, entry validatedEntry) ([]memberState, error) {
	states := make([]memberState, 0, len(entry.manifest.OrderedMembers))
	for _, expected := range entry.manifest.OrderedMembers {
		var state memberState
		if err := tx.QueryRowContext(ctx, `
			SELECT id,task_id,asset_id,COALESCE(storage_ref_id,''),whole_hash,asset_type,
			       deleted_at,cleaned_at,object_deleted_at
			FROM task_assets WHERE id=? FOR UPDATE`, expected.TaskAssetID).Scan(
			&state.id, &state.taskID, &state.assetID, &state.storageRefID, &state.wholeHash,
			&state.assetType, &state.deletedAt, &state.cleanedAt, &state.objectDeletedAt,
		); err != nil {
			return nil, err
		}
		if state.taskID != entry.registry.TaskID || state.assetID != expected.AssetID ||
			state.storageRefID != expected.StorageRefID || state.assetType != "source" ||
			state.deletedAt.Valid || state.cleanedAt.Valid || state.objectDeletedAt.Valid {
			return nil, fmt.Errorf("member task_asset %d identity/lifecycle drifted", expected.TaskAssetID)
		}
		var storageAssetID sql.NullInt64
		var ownerType, refKey, status string
		var ownerID int64
		if err := tx.QueryRowContext(ctx, `
			SELECT asset_id,owner_type,owner_id,ref_key,status
			FROM asset_storage_refs WHERE ref_id=? FOR UPDATE`, expected.StorageRefID).Scan(
			&storageAssetID, &ownerType, &ownerID, &refKey, &status,
		); err != nil {
			return nil, err
		}
		if !storageAssetID.Valid || storageAssetID.Int64 != expected.TaskAssetID ||
			ownerType != "task_asset" || ownerID != expected.TaskAssetID ||
			strings.TrimSpace(refKey) == "" || status != "recorded" {
			return nil, fmt.Errorf("member task_asset %d storage identity drifted", expected.TaskAssetID)
		}
		states = append(states, state)
	}
	return states, nil
}

type bundleDBState int

const (
	bundleAbsent bundleDBState = iota
	bundleExact
)

func lockBundleState(ctx context.Context, tx transaction, entry validatedEntry, reviewer int64) (bundleDBState, error) {
	var collisions int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM design_assets
		WHERE task_id=? AND asset_no=? AND id<>? FOR UPDATE`,
		entry.registry.TaskID, bundleAssetNo(entry.registry.TaskAssetCandidate.ID),
		entry.manifest.BundleAssetID,
	).Scan(&collisions); err != nil {
		return bundleAbsent, err
	}
	if collisions != 0 {
		return bundleAbsent, errors.New("bundle design asset_no collision")
	}
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM task_assets
		WHERE task_id=? AND version_no=? AND id<>? FOR UPDATE`,
		entry.registry.TaskID, entry.registry.TaskAssetCandidate.ID,
		entry.registry.TaskAssetCandidate.ID,
	).Scan(&collisions); err != nil {
		return bundleAbsent, err
	}
	if collisions != 0 {
		return bundleAbsent, errors.New("bundle task version collision")
	}
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM asset_storage_refs
		WHERE ref_key=? AND ref_id<>? FOR UPDATE`,
		entry.registry.ObjectKey, entry.registry.AssetStorageRefCandidate.RefID,
	).Scan(&collisions); err != nil {
		return bundleAbsent, err
	}
	if collisions != 0 {
		return bundleAbsent, errors.New("bundle storage ref_key collision")
	}
	var taskID, currentVersionID, createdBy int64
	var assetNo, assetType, scopeSKU string
	err := tx.QueryRowContext(ctx, `
		SELECT task_id,asset_no,scope_sku_code,asset_type,current_version_id,created_by
		FROM design_assets WHERE id=? FOR UPDATE`, entry.manifest.BundleAssetID).Scan(
		&taskID, &assetNo, &scopeSKU, &assetType, &currentVersionID, &createdBy,
	)
	designAbsent := errors.Is(err, sql.ErrNoRows)
	if err != nil && !designAbsent {
		return bundleAbsent, err
	}
	if !designAbsent && (taskID != entry.registry.TaskID ||
		assetNo != bundleAssetNo(entry.registry.TaskAssetCandidate.ID) ||
		scopeSKU != entry.scopeSKUCode || assetType != "source" ||
		currentVersionID != entry.registry.TaskAssetCandidate.ID || createdBy != reviewer) {
		return bundleAbsent, errors.New("bundle design_asset id collision or drift")
	}

	var taTaskID, taAssetID, versionNo, assetVersionNo, uploadedBy, fileSize int64
	var taScope, taType, bindingState, storageRefID, fileName, mimeType, storageKey, wholeHash, uploadStatus, previewStatus, sourceModule, remark string
	err = tx.QueryRowContext(ctx, `
		SELECT task_id,asset_id,COALESCE(scope_sku_code,''),asset_type,binding_state,
		       version_no,asset_version_no,COALESCE(storage_ref_id,''),file_name,COALESCE(mime_type,''),
		       file_size,COALESCE(storage_key,''),COALESCE(whole_hash,''),COALESCE(upload_status,''),
		       COALESCE(preview_status,''),uploaded_by,source_module_key,remark
		FROM task_assets WHERE id=? FOR UPDATE`, entry.registry.TaskAssetCandidate.ID).Scan(
		&taTaskID, &taAssetID, &taScope, &taType, &bindingState, &versionNo, &assetVersionNo,
		&storageRefID, &fileName, &mimeType, &fileSize, &storageKey, &wholeHash,
		&uploadStatus, &previewStatus, &uploadedBy, &sourceModule, &remark,
	)
	taskAbsent := errors.Is(err, sql.ErrNoRows)
	if err != nil && !taskAbsent {
		return bundleAbsent, err
	}
	expectedRemark := bundleRemark(entry.registry, entry.manifestSHA)
	if !taskAbsent && (taTaskID != entry.registry.TaskID || taAssetID != entry.manifest.BundleAssetID ||
		taScope != entry.scopeSKUCode || taType != "source" || bindingState != "legacy" ||
		versionNo != entry.registry.TaskAssetCandidate.ID || assetVersionNo != 1 ||
		storageRefID != entry.registry.AssetStorageRefCandidate.RefID ||
		fileName != "source-bundle.zip" || mimeType != "application/zip" ||
		fileSize != entry.registry.Size || storageKey != entry.registry.ObjectKey ||
		wholeHash != entry.registry.BundleSHA256 || uploadStatus != "uploaded" ||
		previewStatus != "not_applicable" || uploadedBy != reviewer ||
		sourceModule != "migration" || remark != expectedRemark) {
		return bundleAbsent, errors.New("bundle task_asset id collision or drift")
	}

	var refAssetID sql.NullInt64
	var ownerType, adapter, refType, refKey, refFile, refMIME, checksum, refStatus string
	var ownerID, refSize, placeholder int64
	err = tx.QueryRowContext(ctx, `
		SELECT asset_id,owner_type,owner_id,storage_adapter,ref_type,ref_key,file_name,
		       mime_type,file_size,is_placeholder,checksum_hint,status
		FROM asset_storage_refs WHERE ref_id=? FOR UPDATE`,
		entry.registry.AssetStorageRefCandidate.RefID).Scan(
		&refAssetID, &ownerType, &ownerID, &adapter, &refType, &refKey, &refFile,
		&refMIME, &refSize, &placeholder, &checksum, &refStatus,
	)
	refAbsent := errors.Is(err, sql.ErrNoRows)
	if err != nil && !refAbsent {
		return bundleAbsent, err
	}
	if !refAbsent && (!refAssetID.Valid || refAssetID.Int64 != entry.registry.TaskAssetCandidate.ID ||
		ownerType != "task_asset" || ownerID != entry.registry.TaskAssetCandidate.ID ||
		adapter != "oss_upload_service" || refType != "task_asset_object" ||
		refKey != entry.registry.ObjectKey || refFile != "source-bundle.zip" ||
		refMIME != "application/zip" || refSize != entry.registry.Size ||
		placeholder != 0 || checksum != entry.registry.BundleSHA256 || refStatus != "recorded") {
		return bundleAbsent, errors.New("bundle asset_storage_ref collision or drift")
	}
	absentCount := 0
	for _, absent := range []bool{designAbsent, taskAbsent, refAbsent} {
		if absent {
			absentCount++
		}
	}
	if absentCount == 3 {
		return bundleAbsent, nil
	}
	if absentCount == 0 {
		return bundleExact, nil
	}
	return bundleAbsent, errors.New("bundle database rows are partially applied")
}

func bundleRemark(entry registryEntry, manifestSHA string) string {
	return fmt.Sprintf("v8-migration-source-bundle:%s:%s", entry.ObjectKey, manifestSHA)
}

func bundleAssetNo(taskAssetID int64) string {
	return fmt.Sprintf("MIG-BUNDLE-%d", taskAssetID)
}

func applyAll(ctx context.Context, tx transaction, entries []validatedEntry, manifest confirmedManifest) (int, int, []memberBefore, error) {
	// Fill scope codes first while the transaction is serializable.
	for index := range entries {
		if err := tx.QueryRowContext(ctx, `
			SELECT sku_code FROM task_sku_items
			WHERE id=? AND task_id=? FOR UPDATE`,
			entries[index].registry.ScopeRefID, entries[index].registry.TaskID,
		).Scan(&entries[index].scopeSKUCode); err != nil {
			return 0, 0, nil, fmt.Errorf("scope %s SKU identity: %w",
				scopeKey(entries[index].registry.TaskID, entries[index].registry.ScopeKind,
					entries[index].registry.ScopeRefID, entries[index].registry.RevisionNo), err)
		}
		if strings.TrimSpace(entries[index].scopeSKUCode) == "" {
			return 0, 0, nil, errors.New("bundle SKU code is empty")
		}
	}
	type prepared struct {
		entry   validatedEntry
		members []memberState
		state   bundleDBState
	}
	items := make([]prepared, 0, len(entries))
	allAbsent, allExact := true, true
	for _, entry := range entries {
		members, err := lockAndValidateMembers(ctx, tx, entry)
		if err != nil {
			return 0, 0, nil, err
		}
		state, err := lockBundleState(ctx, tx, entry, manifest.ConfirmedBy)
		if err != nil {
			return 0, 0, nil, err
		}
		allAbsent = allAbsent && state == bundleAbsent
		allExact = allExact && state == bundleExact
		for index, member := range members {
			expected := entry.manifest.OrderedMembers[index].SHA256
			before := !member.wholeHash.Valid || member.wholeHash.String == ""
			after := member.wholeHash.Valid && member.wholeHash.String == expected
			allAbsent = allAbsent && before
			allExact = allExact && after
		}
		items = append(items, prepared{entry: entry, members: members, state: state})
	}
	if !allAbsent && !allExact {
		return 0, 0, nil, errors.New("bundle/member state is mixed, drifted, or partially applied")
	}
	if allExact {
		return 0, len(items), nil, nil
	}

	var before []memberBefore
	for _, item := range items {
		for index, member := range item.members {
			expected := item.entry.manifest.OrderedMembers[index].SHA256
			var original *string
			if member.wholeHash.Valid {
				value := member.wholeHash.String
				original = &value
			}
			before = append(before, memberBefore{TaskAssetID: member.id, OriginalHash: original, RecoveredHash: expected})
			result, err := tx.ExecContext(ctx, `
				UPDATE task_assets SET whole_hash=?
				WHERE id=? AND (whole_hash IS NULL OR whole_hash='')`, expected, member.id)
			if err != nil {
				return 0, 0, nil, err
			}
			if err := requireOne(result, "update bundle member whole_hash"); err != nil {
				return 0, 0, nil, err
			}
		}
		if err := insertBundleRows(ctx, tx, item.entry, manifest); err != nil {
			return 0, 0, nil, err
		}
	}
	sort.Slice(before, func(i, j int) bool { return before[i].TaskAssetID < before[j].TaskAssetID })
	return len(items), 0, before, nil
}

func insertBundleRows(ctx context.Context, tx transaction, entry validatedEntry, manifest confirmedManifest) error {
	createdAt := entry.confirmedAt.Format("2006-01-02 15:04:05")
	if result, err := tx.ExecContext(ctx, `
		INSERT INTO design_assets
		  (id,task_id,asset_no,source_asset_id,scope_sku_code,retouch_requirement_id,
		   asset_type,current_version_id,created_by,created_at,updated_at)
		VALUES (?,?,?,?,?,NULL,'source',?,?,?,?)`,
		entry.manifest.BundleAssetID, entry.registry.TaskID,
		bundleAssetNo(entry.registry.TaskAssetCandidate.ID), nil, entry.scopeSKUCode,
		entry.registry.TaskAssetCandidate.ID, manifest.ConfirmedBy, createdAt, createdAt,
	); err != nil {
		return err
	} else if err := requireOne(result, "insert bundle design_asset"); err != nil {
		return err
	}
	if result, err := tx.ExecContext(ctx, `
		INSERT INTO task_assets
		  (id,task_id,asset_id,scope_sku_code,retouch_requirement_id,asset_type,binding_state,
		   version_no,asset_version_no,upload_mode,upload_request_id,storage_ref_id,
		   file_name,original_filename,mime_type,file_size,storage_key,whole_hash,
		   upload_status,preview_status,uploaded_by,uploaded_at,remark,source_module_key,
		   is_archived,flow_review_status,created_at)
		VALUES (?,?,?,?,NULL,'source','legacy',?,1,'migration',NULL,?,
		        'source-bundle.zip','source-bundle.zip','application/zip',?,?,?,
		        'uploaded','not_applicable',?,?,?,'migration',0,'not_applicable',?)`,
		entry.registry.TaskAssetCandidate.ID, entry.registry.TaskID, entry.manifest.BundleAssetID,
		entry.scopeSKUCode, entry.registry.TaskAssetCandidate.ID,
		entry.registry.AssetStorageRefCandidate.RefID, entry.registry.Size,
		entry.registry.ObjectKey, entry.registry.BundleSHA256, manifest.ConfirmedBy,
		createdAt, bundleRemark(entry.registry, entry.manifestSHA), createdAt,
	); err != nil {
		return err
	} else if err := requireOne(result, "insert bundle task_asset"); err != nil {
		return err
	}
	if result, err := tx.ExecContext(ctx, `
		INSERT INTO asset_storage_refs
		  (ref_id,asset_id,owner_type,owner_id,upload_request_id,storage_adapter,ref_type,
		   ref_key,file_name,mime_type,file_size,is_placeholder,checksum_hint,status,created_at)
		VALUES (?,?,'task_asset',?,NULL,'oss_upload_service','task_asset_object',
		        ?,'source-bundle.zip','application/zip',?,0,?,'recorded',?)`,
		entry.registry.AssetStorageRefCandidate.RefID, entry.registry.TaskAssetCandidate.ID,
		entry.registry.TaskAssetCandidate.ID, entry.registry.ObjectKey, entry.registry.Size,
		entry.registry.BundleSHA256, createdAt,
	); err != nil {
		return err
	} else {
		return requireOne(result, "insert bundle asset_storage_ref")
	}
}

func rollbackAll(ctx context.Context, tx transaction, entries []validatedEntry, report executionReport) (int, int, error) {
	expectedMembers := 0
	for _, entry := range entries {
		expectedMembers += len(entry.manifest.OrderedMembers)
	}
	if report.ChangedBundleCount != len(entries) || report.AlreadyAppliedCount != 0 || len(report.MemberBefore) != expectedMembers {
		return 0, 0, errors.New("rollback requires the original changing apply report for all seven bundles and 22 members")
	}
	beforeByID := map[int64]memberBefore{}
	for _, member := range report.MemberBefore {
		if _, duplicate := beforeByID[member.TaskAssetID]; duplicate {
			return 0, 0, errors.New("apply report contains duplicate member state")
		}
		beforeByID[member.TaskAssetID] = member
	}
	for index := range entries {
		if err := tx.QueryRowContext(ctx, `
			SELECT sku_code FROM task_sku_items
			WHERE id=? AND task_id=? FOR UPDATE`,
			entries[index].registry.ScopeRefID, entries[index].registry.TaskID,
		).Scan(&entries[index].scopeSKUCode); err != nil {
			return 0, 0, err
		}
		members, err := lockAndValidateMembers(ctx, tx, entries[index])
		if err != nil {
			return 0, 0, err
		}
		for memberIndex, member := range members {
			expected := entries[index].manifest.OrderedMembers[memberIndex].SHA256
			saved, ok := beforeByID[member.id]
			if !ok || saved.RecoveredHash != expected ||
				!member.wholeHash.Valid || member.wholeHash.String != expected {
				return 0, 0, fmt.Errorf("member task_asset %d is not in exact applied state", member.id)
			}
		}
		state, err := lockBundleState(ctx, tx, entries[index], entries[index].registry.SourceBundle.ConfirmedBy)
		if err != nil || state != bundleExact {
			return 0, 0, fmt.Errorf("bundle %d is not in exact applied state: %w",
				entries[index].registry.TaskAssetCandidate.ID, err)
		}
		var references int
		if err := tx.QueryRowContext(ctx, `
			SELECT
			  (SELECT COUNT(*) FROM task_asset_group_revisions WHERE source_task_asset_id=?)
			  +(SELECT COUNT(*) FROM task_asset_group_revision_items WHERE task_asset_id=?)`,
			entries[index].registry.TaskAssetCandidate.ID,
			entries[index].registry.TaskAssetCandidate.ID,
		).Scan(&references); err != nil {
			return 0, 0, err
		}
		if references != 0 {
			return 0, 0, fmt.Errorf("bundle task_asset %d is still referenced by %d resource revision rows",
				entries[index].registry.TaskAssetCandidate.ID, references)
		}
	}
	for index := len(entries) - 1; index >= 0; index-- {
		entry := entries[index]
		for _, statement := range []struct {
			query string
			arg   any
			name  string
		}{
			{`DELETE FROM asset_storage_refs WHERE ref_id=?`, entry.registry.AssetStorageRefCandidate.RefID, "delete bundle storage ref"},
			{`DELETE FROM task_assets WHERE id=?`, entry.registry.TaskAssetCandidate.ID, "delete bundle task asset"},
			{`DELETE FROM design_assets WHERE id=?`, entry.manifest.BundleAssetID, "delete bundle design asset"},
		} {
			result, err := tx.ExecContext(ctx, statement.query, statement.arg)
			if err != nil {
				return 0, 0, err
			}
			if err := requireOne(result, statement.name); err != nil {
				return 0, 0, err
			}
		}
	}
	for _, saved := range report.MemberBefore {
		var (
			result sql.Result
			err    error
		)
		if saved.OriginalHash == nil {
			result, err = tx.ExecContext(ctx, `
				UPDATE task_assets SET whole_hash=NULL
				WHERE id=? AND whole_hash=?`, saved.TaskAssetID, saved.RecoveredHash)
		} else {
			result, err = tx.ExecContext(ctx, `
				UPDATE task_assets SET whole_hash=?
				WHERE id=? AND whole_hash=?`, *saved.OriginalHash, saved.TaskAssetID, saved.RecoveredHash)
		}
		if err != nil {
			return 0, 0, err
		}
		if err := requireOne(result, "restore member whole_hash"); err != nil {
			return 0, 0, err
		}
	}
	return len(entries), 0, nil
}

func loadApplyReport(path string, reg registry, manifest confirmedManifest, registrySHA string) ([]byte, executionReport, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, executionReport{}, err
	}
	var report executionReport
	if err := decodeOne(raw, &report); err != nil {
		return nil, report, err
	}
	if report.SchemaVersion != 1 || report.Mode != "apply" || report.Status != "PASS" ||
		!report.Committed || report.RunID != reg.RunID ||
		report.CandidateSHA256 != manifest.SourceCandidateSHA256 ||
		report.RegistrySHA256 != registrySHA || report.ManifestSHA256 != reg.ManifestSHA256 {
		return nil, report, errors.New("apply report is not bound to this committed bundle apply")
	}
	return raw, report, nil
}

func requireOne(result sql.Result, action string) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return fmt.Errorf("%s affected %d rows; expected exactly 1", action, affected)
	}
	return nil
}

func sha256Hex(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func sha256File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func writeNewJSON(path string, value any) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("refusing to overwrite report %s", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(file)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}
