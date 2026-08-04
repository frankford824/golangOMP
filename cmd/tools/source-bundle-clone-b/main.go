package main

import (
	"bytes"
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
	"reflect"
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
	RollbackJournalFile    string
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
	WriteAheadSHA256       string          `json:"write_ahead_sha256"`
	EvidenceSHA256         string          `json:"evidence_sha256"`
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
	TaskAssetID          int64  `json:"task_asset_id"`
	StorageRefID         string `json:"storage_ref_id"`
	RelativeObjectPath   string `json:"relative_object_path"`
	ExpectedSHA256       string `json:"expected_sha256"`
	OwnershipReceiptPath string `json:"ownership_receipt_path,omitempty"`
}

type bundleOwnershipReceipt struct {
	SchemaVersion  int    `json:"schema_version"`
	Status         string `json:"status"`
	RunID          string `json:"run_id"`
	TargetPath     string `json:"target_path"`
	StagingPath    string `json:"staging_path"`
	Device         uint64 `json:"device"`
	Inode          uint64 `json:"inode"`
	Size           int64  `json:"size"`
	SHA256         string `json:"sha256"`
	EvidenceSHA256 string `json:"evidence_sha256"`
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
	SchemaVersion                 int            `json:"schema_version"`
	Mode                          string         `json:"mode"`
	Status                        string         `json:"status"`
	RunID                         string         `json:"run_id"`
	Database                      string         `json:"database"`
	Host                          string         `json:"host"`
	CandidateSHA256               string         `json:"candidate_sha256"`
	RegistrySHA256                string         `json:"registry_sha256"`
	ManifestSHA256                string         `json:"manifest_sha256"`
	ApplyReportSHA256             string         `json:"apply_report_sha256,omitempty"`
	RollbackJournalSHA256         string         `json:"rollback_journal_sha256"`
	RollbackJournalEvidenceSHA256 string         `json:"rollback_journal_evidence_sha256"`
	ChangedBundleCount            int            `json:"changed_bundle_count"`
	AlreadyAppliedCount           int            `json:"already_applied_bundle_count"`
	MemberBefore                  []memberBefore `json:"member_before,omitempty"`
	Committed                     bool           `json:"database_transaction_committed"`
	ExecutedAt                    time.Time      `json:"executed_at"`
}

type rollbackJournal struct {
	SchemaVersion                       int                  `json:"schema_version"`
	Kind                                string               `json:"kind"`
	Status                              string               `json:"status"`
	RunID                               string               `json:"run_id"`
	Database                            string               `json:"database"`
	Host                                string               `json:"host"`
	CandidateSHA256                     string               `json:"candidate_sha256"`
	RegistrySHA256                      string               `json:"registry_sha256"`
	ManifestSHA256                      string               `json:"manifest_sha256"`
	PreparedBeforeFirstDatabaseMutation bool                 `json:"prepared_before_first_database_mutation"`
	DatabaseCommitState                 string               `json:"database_commit_state"`
	ExpectedBundleCount                 int                  `json:"expected_bundle_count"`
	ExpectedMemberCount                 int                  `json:"expected_member_count"`
	ChangedBundleCount                  int                  `json:"changed_bundle_count"`
	AlreadyAppliedCount                 int                  `json:"already_applied_bundle_count"`
	MemberBefore                        []memberBefore       `json:"member_before"`
	AutoIncrementBefore                 []autoIncrementState `json:"auto_increment_before"`
	AutoIncrementCeilings               []autoIncrementState `json:"auto_increment_ceilings"`
	ProductionWritesExecuted            bool                 `json:"production_writes_executed"`
	EvidenceSHA256                      string               `json:"evidence_sha256,omitempty"`
}

type autoIncrementState struct {
	Table     string `json:"table"`
	NextValue int64  `json:"next_value"`
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
	flag.StringVar(&o.ApplyReportFile, "apply-report", "", "optional original apply report cross-check")
	flag.StringVar(&o.RollbackJournalFile, "rollback-journal", "", "durable pre-commit rollback journal")
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
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("connect Clone B: %w", err)
	}
	if err := configureAutoIncrementSession(ctx, db); err != nil {
		return err
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
	var rollbackSeed executionReport
	var rollbackJournalValue rollbackJournal
	switch o.Mode {
	case "apply":
		persistJournal := func(before []memberBefore) error {
			autoBefore, err := loadBundleAutoIncrementStates(ctx, tx)
			if err != nil {
				return err
			}
			autoCeilings, err := bundleAutoIncrementCeilings(
				autoBefore, reg,
			)
			if err != nil {
				return err
			}
			journal, err := newRollbackJournal(
				reg, manifest, registrySHA, manifestSHA, database, host,
				before, autoBefore, autoCeilings,
			)
			if err != nil {
				return err
			}
			return writeNewJSONDurable(o.RollbackJournalFile, journal)
		}
		changed, already, before, err := applyAll(
			ctx, tx, entries, manifest, persistJournal,
		)
		if err != nil {
			return err
		}
		report.ChangedBundleCount = changed
		report.AlreadyAppliedCount = already
		report.MemberBefore = before
		_, journal, err := loadRollbackJournal(
			o.RollbackJournalFile, reg, manifest, registrySHA, manifestSHA,
			database, host,
		)
		if err != nil {
			return fmt.Errorf("validate apply rollback journal: %w", err)
		}
		rollbackSeed = journal.executionReport()
		rollbackJournalValue = journal
	case "rollback":
		journalRaw, journal, err := loadRollbackJournal(
			o.RollbackJournalFile, reg, manifest, registrySHA, manifestSHA,
			database, host,
		)
		if err != nil {
			return err
		}
		rollbackSeed = journal.executionReport()
		rollbackJournalValue = journal
		if strings.TrimSpace(o.ApplyReportFile) != "" {
			applyRaw, applyReport, err := loadApplyReport(
				o.ApplyReportFile, reg, manifest, registrySHA,
			)
			if err != nil {
				return err
			}
			if !sameMemberBefore(applyReport.MemberBefore, journal.MemberBefore) {
				return errors.New("apply report differs from rollback journal")
			}
			if applyReport.RollbackJournalSHA256 != sha256Hex(journalRaw) ||
				applyReport.RollbackJournalEvidenceSHA256 != journal.EvidenceSHA256 {
				return errors.New("apply report rollback journal binding differs")
			}
			report.ApplyReportSHA256 = sha256Hex(applyRaw)
		}
		changed, already, err := rollbackAll(ctx, tx, entries, rollbackSeed)
		if err != nil {
			return err
		}
		report.ChangedBundleCount = changed
		report.AlreadyAppliedCount = already
	}
	journalRaw, journal, err := loadRollbackJournal(
		o.RollbackJournalFile, reg, manifest, registrySHA, manifestSHA,
		database, host,
	)
	if err != nil {
		return err
	}
	report.RollbackJournalSHA256 = sha256Hex(journalRaw)
	report.RollbackJournalEvidenceSHA256 = journal.EvidenceSHA256
	if err := tx.Commit(); err != nil {
		return err
	}
	if o.Mode == "rollback" {
		if err := restoreBundleAutoIncrementStates(
			ctx, db, rollbackJournalValue.AutoIncrementBefore,
			rollbackJournalValue.AutoIncrementCeilings,
		); err != nil {
			return fmt.Errorf("restore bundle auto-increment state: %w", err)
		}
	}
	report.Committed = true
	report.ExecutedAt = time.Now().UTC()
	if err := writeNewJSON(o.ReportFile, report); err != nil {
		if o.Mode == "apply" && report.ChangedBundleCount > 0 {
			compensationErr := compensateCommittedApply(
				ctx, db, reg, manifest, registrySHA, entries,
				rollbackSeed, rollbackJournalValue,
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
	journal rollbackJournal,
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
	if err := tx.Commit(); err != nil {
		return err
	}
	return restoreBundleAutoIncrementStates(
		ctx, db, journal.AutoIncrementBefore, journal.AutoIncrementCeilings,
	)
}

func validateOptions(o options) (*mysql.Config, string, error) {
	if o.Mode != "apply" && o.Mode != "rollback" {
		return nil, "", errors.New("--mode must be apply or rollback")
	}
	required := map[string]string{
		"--dsn/CLONE_B_MYSQL_DSN": o.DSN, "--registry": o.RegistryFile,
		"--manifest": o.ManifestFile, "--fixture-root": o.FixtureRoot,
		"--rollback-journal": o.RollbackJournalFile,
		"--report-file":      o.ReportFile, "--confirm-database": o.ConfirmDatabase,
		"--confirm-host": o.ConfirmHost, "--confirm-run-id": o.ConfirmRunID,
		"--confirm-candidate-sha256": o.ConfirmCandidateSHA256,
	}
	for name, value := range required {
		if strings.TrimSpace(value) == "" {
			return nil, "", fmt.Errorf("%s is required", name)
		}
	}
	journalPath, err := filepath.Abs(o.RollbackJournalFile)
	if err != nil || journalPath != filepath.Clean(o.RollbackJournalFile) {
		return nil, "", errors.New("--rollback-journal must be an absolute clean path")
	}
	reportPath, err := filepath.Abs(o.ReportFile)
	if err != nil || filepath.Dir(journalPath) != filepath.Dir(reportPath) {
		return nil, "", errors.New("--rollback-journal and --report-file must share a directory")
	}
	if info, err := os.Lstat(journalPath); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return nil, "", errors.New("--rollback-journal must not be a symlink")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, "", err
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
	if err := validateRegistryEvidence(registryRaw, reg); err != nil {
		return nil, reg, nil, confirmedManifest{}, nil, err
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

func validateRegistryEvidence(raw []byte, reg registry) error {
	if !sha256Pattern.MatchString(reg.WriteAheadSHA256) ||
		!sha256Pattern.MatchString(reg.EvidenceSHA256) {
		return errors.New("registry evidence hashes must be lowercase SHA-256")
	}
	return validatePythonSelfHash(
		raw,
		reg.EvidenceSHA256,
		"registry evidence",
	)
}

func validatePythonSelfHash(raw []byte, expected, label string) error {
	var canonical map[string]any
	if err := json.Unmarshal(raw, &canonical); err != nil {
		return fmt.Errorf("decode %s: %w", label, err)
	}
	delete(canonical, "evidence_sha256")
	unsigned, err := json.Marshal(canonical)
	if err != nil {
		return fmt.Errorf("canonicalize %s: %w", label, err)
	}
	// Python's canonical_bytes appends one newline after the compact,
	// key-sorted JSON payload.
	unsigned = append(unsigned, '\n')
	if sha256Hex(unsigned) != expected {
		return fmt.Errorf("%s self hash is missing or stale", label)
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
		if err := validateRollbackOwnership(root, item, reg.RunID); err != nil {
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

func validateRollbackOwnership(root string, item registryEntry, runID string) error {
	recorded := strings.TrimSpace(item.RollbackCandidate.OwnershipReceiptPath)
	if item.Disposition == "reused_identical" {
		if recorded == "" {
			return nil
		}
		if !filepath.IsAbs(recorded) ||
			filepath.Base(recorded) != fmt.Sprintf(
				"bundle-ownership-%d.json",
				item.TaskAssetCandidate.ID,
			) {
			return errors.New("reused bundle ownership receipt path is invalid")
		}
		return nil
	}
	if item.Disposition != "created" || !filepath.IsAbs(recorded) {
		return errors.New("created bundle ownership receipt path is missing")
	}
	receiptPath := filepath.Clean(recorded)
	if receiptPath != recorded ||
		filepath.Base(receiptPath) != fmt.Sprintf(
			"bundle-ownership-%d.json",
			item.TaskAssetCandidate.ID,
		) {
		return errors.New("created bundle ownership receipt path differs")
	}
	runRoot := filepath.Dir(root)
	relativeReceipt, err := filepath.Rel(runRoot, receiptPath)
	if err != nil || relativeReceipt == ".." ||
		strings.HasPrefix(relativeReceipt, ".."+string(filepath.Separator)) {
		return errors.New("created bundle ownership receipt escapes the run root")
	}
	receiptInfo, err := os.Lstat(receiptPath)
	if err != nil || !receiptInfo.Mode().IsRegular() ||
		receiptInfo.Mode()&os.ModeSymlink != 0 {
		return errors.New("created bundle ownership receipt is not a regular file")
	}
	receiptRaw, err := os.ReadFile(receiptPath)
	if err != nil {
		return err
	}
	var receipt bundleOwnershipReceipt
	if err := decodeOne(receiptRaw, &receipt); err != nil {
		return fmt.Errorf("decode bundle ownership receipt: %w", err)
	}
	if !sha256Pattern.MatchString(receipt.EvidenceSHA256) {
		return errors.New("bundle ownership receipt evidence hash is invalid")
	}
	if err := validatePythonSelfHash(
		receiptRaw,
		receipt.EvidenceSHA256,
		"bundle ownership receipt",
	); err != nil {
		return err
	}
	relative := filepath.FromSlash(item.RelativeObjectPath)
	target := filepath.Clean(filepath.Join(root, relative))
	relativeTarget, err := filepath.Rel(root, target)
	if err != nil || relativeTarget == ".." ||
		strings.HasPrefix(relativeTarget, ".."+string(filepath.Separator)) {
		return errors.New("bundle target escapes fixture root")
	}
	targetInfo, err := os.Lstat(target)
	if err != nil || !targetInfo.Mode().IsRegular() ||
		targetInfo.Mode()&os.ModeSymlink != 0 {
		return errors.New("bundle target is not a regular file")
	}
	device, inode, err := fileDeviceInode(targetInfo)
	if err != nil {
		return err
	}
	expectedStage := filepath.Join(
		filepath.Dir(receiptPath),
		fmt.Sprintf(".bundle-stage-%d.zip", item.TaskAssetCandidate.ID),
	)
	if receipt.SchemaVersion != 1 ||
		receipt.Status != "OWNED_LINK" ||
		receipt.RunID != runID ||
		filepath.Clean(receipt.TargetPath) != target ||
		filepath.Clean(receipt.StagingPath) != expectedStage ||
		receipt.Device != device ||
		receipt.Inode != inode ||
		receipt.Size != targetInfo.Size() ||
		receipt.Size != item.Size ||
		receipt.SHA256 != item.BundleSHA256 {
		return errors.New("bundle ownership receipt identity differs")
	}
	return nil
}

func fileDeviceInode(info os.FileInfo) (uint64, uint64, error) {
	value := reflect.ValueOf(info.Sys())
	if !value.IsValid() {
		return 0, 0, errors.New("bundle file identity is unavailable")
	}
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return 0, 0, errors.New("bundle file identity is unavailable")
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return 0, 0, errors.New("bundle file identity is unavailable")
	}
	device := value.FieldByName("Dev")
	inode := value.FieldByName("Ino")
	if !device.IsValid() || !inode.IsValid() ||
		(device.Kind() != reflect.Uint64 &&
			device.Kind() != reflect.Uint &&
			device.Kind() != reflect.Uint32) ||
		(inode.Kind() != reflect.Uint64 &&
			inode.Kind() != reflect.Uint &&
			inode.Kind() != reflect.Uint32) {
		return 0, 0, errors.New("bundle file identity is unavailable")
	}
	return device.Uint(), inode.Uint(), nil
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

func applyAll(
	ctx context.Context,
	tx transaction,
	entries []validatedEntry,
	manifest confirmedManifest,
	persistBeforeMutation func([]memberBefore) error,
) (int, int, []memberBefore, error) {
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
		}
	}
	sort.Slice(before, func(i, j int) bool { return before[i].TaskAssetID < before[j].TaskAssetID })
	if persistBeforeMutation == nil {
		return 0, 0, nil, errors.New("pre-mutation rollback journal callback is required")
	}
	if err := persistBeforeMutation(before); err != nil {
		return 0, 0, nil, err
	}
	for _, item := range items {
		for index, member := range item.members {
			expected := item.entry.manifest.OrderedMembers[index].SHA256
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
	allApplied, allBefore := true, true
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
			if !ok || saved.RecoveredHash != expected {
				return 0, 0, fmt.Errorf("member task_asset %d is absent from rollback journal", member.id)
			}
			applied := member.wholeHash.Valid && member.wholeHash.String == expected
			before := (saved.OriginalHash == nil && !member.wholeHash.Valid) ||
				(saved.OriginalHash != nil && member.wholeHash.Valid &&
					member.wholeHash.String == *saved.OriginalHash)
			allApplied = allApplied && applied
			allBefore = allBefore && before
		}
		state, err := lockBundleState(ctx, tx, entries[index], entries[index].registry.SourceBundle.ConfirmedBy)
		if err != nil {
			return 0, 0, fmt.Errorf("bundle %d state validation failed: %w",
				entries[index].registry.TaskAssetCandidate.ID, err)
		}
		allApplied = allApplied && state == bundleExact
		allBefore = allBefore && state == bundleAbsent
		if state != bundleExact {
			continue
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
	if allBefore {
		return 0, len(entries), nil
	}
	if !allApplied {
		return 0, 0, errors.New("bundle/member rollback state is mixed, drifted, or partially applied")
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

type rowQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func loadBundleAutoIncrementStates(
	ctx context.Context, queryer rowQueryer,
) ([]autoIncrementState, error) {
	if err := validateAutoIncrementSession(ctx, queryer); err != nil {
		return nil, err
	}
	states := make([]autoIncrementState, 0, 2)
	for _, table := range []string{"design_assets", "task_assets"} {
		var next sql.NullInt64
		if err := queryer.QueryRowContext(
			ctx,
			`SELECT AUTO_INCREMENT FROM information_schema.tables
			 WHERE table_schema=DATABASE() AND table_name=?`,
			table,
		).Scan(&next); err != nil {
			return nil, err
		}
		if !next.Valid || next.Int64 <= 0 {
			return nil, fmt.Errorf("%s has no valid AUTO_INCREMENT state", table)
		}
		states = append(states, autoIncrementState{
			Table: table, NextValue: next.Int64,
		})
	}
	return states, nil
}

func configureAutoIncrementSession(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(
		ctx, `SET SESSION information_schema_stats_expiry=0`,
	); err != nil {
		return fmt.Errorf(
			"disable information_schema metadata caching: %w", err,
		)
	}
	return validateAutoIncrementSession(ctx, db)
}

func validateAutoIncrementSession(
	ctx context.Context, queryer rowQueryer,
) error {
	var expiry, increment, offset int64
	if err := queryer.QueryRowContext(
		ctx,
		`SELECT @@SESSION.information_schema_stats_expiry,
		        @@SESSION.auto_increment_increment,
		        @@SESSION.auto_increment_offset`,
	).Scan(&expiry, &increment, &offset); err != nil {
		return err
	}
	if expiry != 0 || increment != 1 || offset != 1 {
		return fmt.Errorf(
			"unsafe auto-increment session settings: expiry=%d increment=%d offset=%d",
			expiry, increment, offset,
		)
	}
	return nil
}

func bundleAutoIncrementCeilings(
	before []autoIncrementState, reg registry,
) ([]autoIncrementState, error) {
	if len(before) != 2 {
		return nil, errors.New("bundle auto-increment baseline must contain two tables")
	}
	maxInserted := map[string]int64{}
	for _, entry := range reg.Entries {
		if entry.TaskAssetCandidate.ID > maxInserted["task_assets"] {
			maxInserted["task_assets"] = entry.TaskAssetCandidate.ID
		}
		if entry.TaskAssetCandidate.AssetID > maxInserted["design_assets"] {
			maxInserted["design_assets"] = entry.TaskAssetCandidate.AssetID
		}
	}
	ceilings := make([]autoIncrementState, 0, len(before))
	for _, state := range before {
		next := state.NextValue
		if inserted := maxInserted[state.Table]; inserted >= next {
			if inserted == int64(^uint64(0)>>1) {
				return nil, errors.New("bundle auto-increment ceiling overflow")
			}
			next = inserted + 1
		}
		ceilings = append(ceilings, autoIncrementState{
			Table: state.Table, NextValue: next,
		})
	}
	if err := validateBundleAutoIncrementStates(before, ceilings); err != nil {
		return nil, err
	}
	return ceilings, nil
}

func validateBundleAutoIncrementStates(
	before []autoIncrementState, ceilings []autoIncrementState,
) error {
	expected := []string{"design_assets", "task_assets"}
	if len(before) != len(expected) || len(ceilings) != len(expected) {
		return errors.New("bundle auto-increment journal must contain two tables")
	}
	for index, table := range expected {
		if before[index].Table != table ||
			ceilings[index].Table != table ||
			before[index].NextValue <= 0 ||
			ceilings[index].NextValue < before[index].NextValue {
			return errors.New("bundle auto-increment journal is invalid")
		}
	}
	return nil
}

func restoreBundleAutoIncrementStates(
	ctx context.Context,
	db *sql.DB,
	before []autoIncrementState,
	ceilings []autoIncrementState,
) error {
	if err := validateBundleAutoIncrementStates(before, ceilings); err != nil {
		return err
	}
	for index, target := range before {
		currentStates, err := loadBundleAutoIncrementStates(ctx, db)
		if err != nil {
			return err
		}
		current := currentStates[index]
		ceiling := ceilings[index]
		if current.NextValue == target.NextValue {
			continue
		}
		if current.NextValue < target.NextValue ||
			current.NextValue > ceiling.NextValue {
			return fmt.Errorf(
				"auto-increment rollback refused for %s: current=%d before=%d ceiling=%d",
				target.Table, current.NextValue, target.NextValue,
				ceiling.NextValue,
			)
		}
		var maxID int64
		query := fmt.Sprintf(
			"SELECT COALESCE(MAX(id),0) FROM `%s`", target.Table,
		)
		if err := db.QueryRowContext(ctx, query).Scan(&maxID); err != nil {
			return err
		}
		if maxID >= target.NextValue {
			return fmt.Errorf(
				"auto-increment rollback refused for %s: max id %d reaches before value %d",
				target.Table, maxID, target.NextValue,
			)
		}
		statement := fmt.Sprintf(
			"ALTER TABLE `%s` AUTO_INCREMENT = %d",
			target.Table, target.NextValue,
		)
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	restored, err := loadBundleAutoIncrementStates(ctx, db)
	if err != nil {
		return err
	}
	for index := range before {
		if restored[index] != before[index] {
			return errors.New("bundle auto-increment rollback verification failed")
		}
	}
	return nil
}

func newRollbackJournal(
	reg registry,
	manifest confirmedManifest,
	registrySHA string,
	manifestSHA string,
	database string,
	host string,
	before []memberBefore,
	autoBefore []autoIncrementState,
	autoCeilings []autoIncrementState,
) (rollbackJournal, error) {
	journal := rollbackJournal{
		SchemaVersion:                       1,
		Kind:                                "source-bundle-clone-b-rollback-journal",
		Status:                              "PREPARED",
		RunID:                               reg.RunID,
		Database:                            database,
		Host:                                host,
		CandidateSHA256:                     manifest.SourceCandidateSHA256,
		RegistrySHA256:                      registrySHA,
		ManifestSHA256:                      manifestSHA,
		PreparedBeforeFirstDatabaseMutation: true,
		DatabaseCommitState:                 "unknown",
		ExpectedBundleCount:                 len(reg.Entries),
		ExpectedMemberCount:                 len(before),
		ChangedBundleCount:                  len(reg.Entries),
		AlreadyAppliedCount:                 0,
		MemberBefore:                        append([]memberBefore(nil), before...),
		AutoIncrementBefore: append(
			[]autoIncrementState(nil), autoBefore...,
		),
		AutoIncrementCeilings: append(
			[]autoIncrementState(nil), autoCeilings...,
		),
		ProductionWritesExecuted: false,
	}
	hash, err := rollbackJournalHash(journal)
	if err != nil {
		return rollbackJournal{}, err
	}
	journal.EvidenceSHA256 = hash
	return journal, nil
}

func rollbackJournalHash(journal rollbackJournal) (string, error) {
	raw, err := json.Marshal(journal)
	if err != nil {
		return "", err
	}
	var canonical map[string]any
	if err := json.Unmarshal(raw, &canonical); err != nil {
		return "", err
	}
	delete(canonical, "evidence_sha256")
	raw, err = json.Marshal(canonical)
	if err != nil {
		return "", err
	}
	return sha256Hex(raw), nil
}

func (journal rollbackJournal) executionReport() executionReport {
	return executionReport{
		SchemaVersion:       1,
		Mode:                "apply",
		Status:              "PREPARED",
		RunID:               journal.RunID,
		Database:            journal.Database,
		Host:                journal.Host,
		CandidateSHA256:     journal.CandidateSHA256,
		RegistrySHA256:      journal.RegistrySHA256,
		ManifestSHA256:      journal.ManifestSHA256,
		ChangedBundleCount:  journal.ChangedBundleCount,
		AlreadyAppliedCount: journal.AlreadyAppliedCount,
		MemberBefore:        append([]memberBefore(nil), journal.MemberBefore...),
	}
}

func loadRollbackJournal(
	path string,
	reg registry,
	manifest confirmedManifest,
	registrySHA string,
	manifestSHA string,
	database string,
	host string,
) ([]byte, rollbackJournal, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, rollbackJournal{}, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, rollbackJournal{}, errors.New("rollback journal must be a regular non-symlink file")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, rollbackJournal{}, err
	}
	var journal rollbackJournal
	if err := decodeOne(raw, &journal); err != nil {
		return nil, journal, fmt.Errorf("decode rollback journal: %w", err)
	}
	expectedHash, err := rollbackJournalHash(journal)
	if err != nil {
		return nil, journal, err
	}
	expectedMembers := 0
	memberHashes := map[int64]string{}
	for _, bundle := range manifest.Bundles {
		expectedMembers += len(bundle.OrderedMembers)
		for _, member := range bundle.OrderedMembers {
			memberHashes[member.TaskAssetID] = member.SHA256
		}
	}
	if journal.SchemaVersion != 1 ||
		journal.Kind != "source-bundle-clone-b-rollback-journal" ||
		journal.Status != "PREPARED" ||
		journal.RunID != reg.RunID ||
		journal.Database != database ||
		journal.Host != host ||
		journal.CandidateSHA256 != manifest.SourceCandidateSHA256 ||
		journal.RegistrySHA256 != registrySHA ||
		journal.ManifestSHA256 != manifestSHA ||
		!journal.PreparedBeforeFirstDatabaseMutation ||
		journal.DatabaseCommitState != "unknown" ||
		journal.ExpectedBundleCount != len(reg.Entries) ||
		journal.ExpectedMemberCount != expectedMembers ||
		journal.ChangedBundleCount != len(reg.Entries) ||
		journal.AlreadyAppliedCount != 0 ||
		journal.ProductionWritesExecuted ||
		journal.EvidenceSHA256 != expectedHash ||
		len(journal.MemberBefore) != expectedMembers ||
		validateBundleAutoIncrementStates(
			journal.AutoIncrementBefore, journal.AutoIncrementCeilings,
		) != nil {
		return nil, journal, errors.New("rollback journal envelope or self hash is invalid")
	}
	var priorID int64
	for index, member := range journal.MemberBefore {
		expected, ok := memberHashes[member.TaskAssetID]
		if !ok || member.RecoveredHash != expected ||
			!sha256Pattern.MatchString(member.RecoveredHash) ||
			(member.OriginalHash != nil && *member.OriginalHash != "") ||
			(index > 0 && member.TaskAssetID <= priorID) {
			return nil, journal, errors.New("rollback journal member before-images are invalid")
		}
		priorID = member.TaskAssetID
	}
	return raw, journal, nil
}

func sameMemberBefore(left, right []memberBefore) bool {
	leftRaw, leftErr := json.Marshal(left)
	rightRaw, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftRaw, rightRaw)
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
	return writeNewJSONDurable(path, value)
}

func writeNewJSONDurable(path string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if info, statErr := os.Lstat(path); statErr == nil {
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("existing rollback journal is not a regular file")
		}
		existing, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if bytes.Equal(existing, raw) {
			if err := syncRegularFile(path); err != nil {
				return err
			}
			return syncDirectory(filepath.Dir(path))
		}
		return fmt.Errorf("refusing to overwrite rollback journal %s", path)
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return statErr
	}
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".rollback-journal.*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(raw); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Link(temporaryPath, path); err != nil {
		if errors.Is(err, os.ErrExist) {
			info, statErr := os.Lstat(path)
			if statErr == nil && info.Mode().IsRegular() &&
				info.Mode()&os.ModeSymlink == 0 {
				existing, readErr := os.ReadFile(path)
				if readErr == nil && bytes.Equal(existing, raw) {
					if syncErr := syncRegularFile(path); syncErr != nil {
						return syncErr
					}
					if removeErr := os.Remove(temporaryPath); removeErr != nil {
						return removeErr
					}
					return syncDirectory(directory)
				}
			}
		}
		return err
	}
	if err := syncDirectory(directory); err != nil {
		return err
	}
	if err := os.Remove(temporaryPath); err != nil {
		return err
	}
	return syncDirectory(directory)
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func syncRegularFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return file.Sync()
}
