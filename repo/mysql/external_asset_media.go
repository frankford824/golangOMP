package mysqlrepo

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"hash/fnv"
	"strings"
	"time"

	"workflow/domain"
)

const externalMediaFPColumns = `origin_path_hash,file_size,modified_ns,changed_ns,file_identity,root_identity,
 source_version,sha256,checksum_state,original_source_version,preview_source_version,
 event_agent,event_epoch,event_seq,scan_id,COALESCE(media_result,JSON_OBJECT())`

func scanExternalMediaFP(row interface{ Scan(...interface{}) error }) (*domain.ExternalMediaFingerprint, error) {
	f := &domain.ExternalMediaFingerprint{}
	err := row.Scan(&f.OriginHash, &f.Size, &f.ModifiedNS, &f.ChangedNS, &f.FileIdentity, &f.RootIdentity, &f.SourceVersion,
		&f.SHA256, &f.ChecksumState, &f.OriginalSourceVersion, &f.PreviewSourceVersion, &f.AgentID, &f.AgentEpoch, &f.Sequence, &f.ScanID, &f.Result)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return f, err
}

func (r *externalAssetRepo) GetMediaFingerprint(ctx context.Context, id int64) (*domain.ExternalMediaFingerprint, error) {
	return scanExternalMediaFP(r.db.db.QueryRowContext(ctx, `SELECT `+externalMediaFPColumns+` FROM external_asset_source_fingerprints
 WHERE origin_path_hash=(SELECT origin_path_hash FROM external_asset_records WHERE id=?)`, id))
}

func mediaFingerprintForUpdate(ctx context.Context, tx *sql.Tx, hash string) (*domain.ExternalMediaFingerprint, error) {
	return scanExternalMediaFP(tx.QueryRowContext(ctx, `SELECT `+externalMediaFPColumns+` FROM external_asset_source_fingerprints WHERE origin_path_hash=? FOR UPDATE`, hash))
}

// The scanner and event lane share sequence space; scans may mark an item seen
// without replacing a newer event. No wall-clock ordering is used here.
func mediaEventIsStale(old, next *domain.ExternalMediaFingerprint) bool {
	if old == nil || old.AgentEpoch == "" || old.AgentEpoch != next.AgentEpoch || old.AgentID != next.AgentID {
		return false
	}
	if next.ScanID != "" {
		return next.Sequence < old.Sequence
	}
	return next.Sequence <= old.Sequence
}

func (r *externalAssetRepo) PendingRequiredMediaIDs(ctx context.Context, prefixes []string, limit int) ([]int64, error) {
	if len(prefixes) == 0 {
		return nil, nil
	}
	if limit <= 0 || limit > 50 {
		limit = 50
	}
	conditions := []string{}
	args := []any{}
	for _, prefix := range prefixes {
		conditions = append(conditions, "(e.origin_path=? OR e.origin_path LIKE ?)")
		escaped := strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(strings.TrimRight(prefix, "/"))
		args = append(args, prefix, escaped+"/%")
	}
	args = append(args, limit)
	rows, err := r.db.db.QueryContext(ctx, `SELECT e.id FROM external_asset_records e JOIN external_asset_source_fingerprints f USING(origin_path_hash)
 WHERE e.status='indexed' AND e.is_dir=0 AND e.oss_sync_status<>'ready' AND f.source_version<>'' AND (`+strings.Join(conditions, " OR ")+`)
 AND NOT EXISTS (SELECT 1 FROM asset_media_jobs j WHERE j.kind='external_original' AND j.resource_id=CONCAT('ext-',e.id) AND j.source_version=f.source_version)
 ORDER BY e.id LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func checkMediaEpoch(ctx context.Context, tx *sql.Tx, f *domain.ExternalMediaFingerprint) error {
	var epoch, root, scanID string
	err := tx.QueryRowContext(ctx, `SELECT agent_epoch,root_identity,scan_id FROM external_asset_sync_runs
 WHERE agent_id=? AND run_type='nas_snapshot' ORDER BY id DESC LIMIT 1`, f.AgentID).Scan(&epoch, &root, &scanID)
	if err != nil {
		return fmt.Errorf("NAS event requires registered scan epoch: %w", err)
	}
	if epoch != f.AgentEpoch || root != f.RootIdentity {
		return fmt.Errorf("NAS event epoch or root mismatch")
	}
	if f.ScanID != "" && scanID != f.ScanID {
		return fmt.Errorf("source_version_changed: scan superseded")
	}
	return nil
}

func writeMediaFingerprint(ctx context.Context, tx *sql.Tx, hash string, f *domain.ExternalMediaFingerprint) error {
	_, err := tx.ExecContext(ctx, `UPDATE external_asset_source_fingerprints SET
 original_source_version=IF(source_version=?,original_source_version,''),
 preview_source_version=IF(source_version=?,preview_source_version,''),
 sha256=IF(source_version=?,sha256,''),checksum_state=IF(source_version=?,checksum_state,'unverified'),
 media_result=IF(source_version=?,media_result,NULL),
 modified_ns=?,changed_ns=?,file_identity=?,root_identity=?,source_version=?,event_agent=?,event_epoch=?,event_seq=?,
 scan_id=IF(?='',scan_id,?) WHERE origin_path_hash=?`,
		f.SourceVersion, f.SourceVersion, f.SourceVersion, f.SourceVersion, f.SourceVersion,
		f.ModifiedNS, f.ChangedNS, f.FileIdentity, f.RootIdentity, f.SourceVersion, f.AgentID, f.AgentEpoch, f.Sequence, f.ScanID, f.ScanID, hash)
	return err
}

func (r *externalAssetRepo) ApplyMediaDelete(ctx context.Context, agent string, event domain.ExternalAssetFilesystemEvent) error {
	hash := domain.ExternalAssetOriginHash("alist", event.MountPath, event.OriginPath)
	return retryExternalAssetLockConflict(ctx, func() error {
		tx, err := r.db.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		existing, err := getExternalAssetCountStateForUpdate(ctx, tx, hash)
		if err != nil {
			return err
		}
		old, err := mediaFingerprintForUpdate(ctx, tx, hash)
		if err != nil {
			return err
		}
		next := &domain.ExternalMediaFingerprint{AgentID: agent, AgentEpoch: event.AgentEpoch, RootIdentity: event.RootIdentity, Sequence: event.Sequence, ScanID: event.ScanID}
		if err = checkMediaEpoch(ctx, tx, next); err != nil {
			return err
		}
		if old != nil && old.AgentEpoch == next.AgentEpoch {
			if (event.ScanID == "" && old.Sequence >= event.Sequence) || (event.ScanID != "" && old.Sequence > event.Sequence) {
				return tx.Commit()
			}
		}
		// Keep a tombstone even if deletion arrived before the first scan row.
		// Otherwise that scan could resurrect a file which no longer exists.
		if existing == nil {
			_, err = tx.ExecContext(ctx, `INSERT INTO external_asset_source_fingerprints
 (origin_path_hash,event_agent,event_epoch,event_seq,root_identity)
 VALUES (?,?,?,?,?) ON DUPLICATE KEY UPDATE event_agent=VALUES(event_agent),event_epoch=VALUES(event_epoch),event_seq=VALUES(event_seq),root_identity=VALUES(root_identity)`, hash, agent, event.AgentEpoch, event.Sequence, event.RootIdentity)
			if err != nil {
				return err
			}
			return tx.Commit()
		}
		if existing.Status != domain.ExternalAssetStatusMissing && !existing.IsDir {
			if err = applyExternalAssetDirectoryCountDelta(ctx, tx, existing.Provider, existing.Kind, existing.Driver, existing.MountPath, existing.ParentPath, event.ObservedAt, -1); err != nil {
				return err
			}
		}
		_, err = tx.ExecContext(ctx, `UPDATE external_asset_records SET status='missing',updated_at=UTC_TIMESTAMP() WHERE origin_path_hash=?`, hash)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE external_asset_source_fingerprints SET event_agent=?,event_epoch=?,event_seq=?,scan_id=IF(?='',scan_id,?) WHERE origin_path_hash=?`, agent, event.AgentEpoch, event.Sequence, event.ScanID, event.ScanID, hash)
		if err != nil {
			return err
		}
		if r.embeddingVersion != "" && r.embeddingVersion != "disabled" {
			if err = upsertAIExternalAssetProjection(ctx, tx, hash, r.embeddingVersion); err != nil {
				return err
			}
		}
		return tx.Commit()
	})
}

func (r *externalAssetRepo) CommitMediaResult(ctx context.Context, job *domain.AssetMediaJob, result domain.ExternalMediaResult) error {
	var input domain.AssetMediaJobInput
	if err := json.Unmarshal(job.Payload, &input); err != nil {
		return err
	}
	tx, err := r.db.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var lease string
	if err = tx.QueryRowContext(ctx, `SELECT job_id FROM asset_media_jobs WHERE job_id=? AND state='processing' AND lease_owner=? AND lease_epoch=? AND lease_expires_at>UTC_TIMESTAMP(6) FOR UPDATE`, job.ID, job.LeaseOwner, job.LeaseEpoch).Scan(&lease); err != nil {
		return fmt.Errorf("media_lease_lost: %w", err)
	}
	var hash, status string
	if err = tx.QueryRowContext(ctx, `SELECT origin_path_hash,status FROM external_asset_records WHERE id=? FOR UPDATE`, input.ExternalID).Scan(&hash, &status); err != nil {
		return err
	}
	fp, err := mediaFingerprintForUpdate(ctx, tx, hash)
	if err != nil {
		return err
	}
	if fp == nil || fp.SourceVersion != job.SourceVersion || status != "indexed" {
		return fmt.Errorf("source_version_changed")
	}
	if fp.SHA256 != "" && result.SourceSHA256 != "" && fp.SHA256 != result.SourceSHA256 {
		return fmt.Errorf("source_version_changed: content checksum differs")
	}
	var old domain.ExternalMediaResult
	_ = json.Unmarshal(fp.Result, &old)
	if result.Original != nil {
		old.Original = result.Original
		fp.OriginalSourceVersion = job.SourceVersion
	}
	if result.Preview != nil {
		old.Preview = result.Preview
	}
	if result.Thumbnail != nil {
		old.Thumbnail = result.Thumbnail
	}
	if old.Preview != nil && old.Thumbnail != nil && old.Preview.SourceVersion == job.SourceVersion && old.Thumbnail.SourceVersion == job.SourceVersion {
		fp.PreviewSourceVersion = job.SourceVersion
	}
	if result.SourceSHA256 != "" {
		old.SourceSHA256 = result.SourceSHA256
		fp.SHA256 = result.SourceSHA256
		fp.ChecksumState = "verified"
	}
	raw, _ := json.Marshal(old)
	_, err = tx.ExecContext(ctx, `UPDATE external_asset_source_fingerprints SET original_source_version=?,preview_source_version=?,sha256=?,checksum_state=?,media_result=? WHERE origin_path_hash=?`, fp.OriginalSourceVersion, fp.PreviewSourceVersion, fp.SHA256, fp.ChecksumState, raw, hash)
	if err != nil {
		return err
	}
	if result.Original != nil {
		_, err = tx.ExecContext(ctx, `UPDATE external_asset_records SET oss_original_key=?,oss_sync_status='ready',updated_at=UTC_TIMESTAMP() WHERE id=?`, result.Original.Key, input.ExternalID)
		if err != nil {
			return err
		}
	}
	if result.Preview != nil && result.Thumbnail != nil {
		_, err = tx.ExecContext(ctx, `UPDATE external_asset_records SET oss_preview_key=?,oss_thumb_key=?,preview_status='ready',updated_at=UTC_TIMESTAMP() WHERE id=?`, result.Preview.Key, result.Thumbnail.Key, input.ExternalID)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *externalAssetRepo) StartMediaScan(ctx context.Context, s domain.ExternalMediaScan) error {
	tx, err := r.db.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	coordinator := uuid.NewSHA1(uuid.NameSpaceOID, []byte("nas-scan-round:"+s.OriginRoot+":"+s.RootIdentity)).String()
	_, err = tx.ExecContext(ctx, `INSERT INTO external_asset_sync_runs (run_type,mount_path,status,scan_id,root_identity) VALUES ('nas_coordinator',?,'running',?,?) ON DUPLICATE KEY UPDATE scan_id=VALUES(scan_id)`, s.OriginRoot, coordinator, s.RootIdentity)
	if err != nil {
		return err
	}
	var generation string
	var mask int
	if err = tx.QueryRowContext(ctx, `SELECT root_generation,generation_shards FROM external_asset_sync_runs WHERE scan_id=? FOR UPDATE`, coordinator).Scan(&generation, &mask); err != nil {
		return err
	}
	var existing string
	err = tx.QueryRowContext(ctx, `SELECT scan_id FROM external_asset_sync_runs WHERE scan_id=?`, s.ScanID).Scan(&existing)
	if err == nil {
		return tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	bit := 1 << s.ShardIndex
	if generation == "" || mask&bit != 0 || mask == 3 {
		generation = uuid.NewString()
		mask = 0
	}
	_, err = tx.ExecContext(ctx, `UPDATE external_asset_sync_runs SET root_generation=?,generation_shards=? WHERE scan_id=?`, generation, mask|bit, coordinator)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO external_asset_sync_runs
 (run_type,mount_path,status,scan_id,agent_id,agent_epoch,root_identity,start_sequence,shard_index,shard_count,root_generation)
 VALUES ('nas_snapshot',?,'running',?,?,?,?,?,?,?,?)`, s.OriginRoot, s.ScanID, s.AgentID, s.AgentEpoch, s.RootIdentity, s.StartSequence, s.ShardIndex, s.ShardCount, generation)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *externalAssetRepo) PutMediaScanPart(ctx context.Context, p domain.ExternalMediaScanPart) error {
	raw, err := json.Marshal(p.Items)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != p.SHA256 {
		return fmt.Errorf("scan part checksum mismatch")
	}
	tx, err := r.db.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var status string
	if err = tx.QueryRowContext(ctx, `SELECT status FROM external_asset_sync_runs WHERE scan_id=? FOR UPDATE`, p.ScanID).Scan(&status); err != nil {
		return err
	}
	if status != "running" {
		return fmt.Errorf("scan is no longer accepting parts")
	}
	var previous string
	err = tx.QueryRowContext(ctx, `SELECT sha256 FROM external_asset_scan_parts WHERE scan_id=? AND part_no=?`, p.ScanID, p.Part).Scan(&previous)
	if err == nil {
		if previous != p.SHA256 {
			return fmt.Errorf("scan part replay changed content")
		}
		return tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO external_asset_scan_parts (scan_id,part_no,sha256,item_count,payload) VALUES (?,?,?,?,?)`, p.ScanID, p.Part, p.SHA256, len(p.Items), raw)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *externalAssetRepo) GetMediaScan(ctx context.Context, id string) (domain.ExternalMediaScan, []domain.ExternalMediaScanPart, error) {
	var s domain.ExternalMediaScan
	err := r.db.db.QueryRowContext(ctx, `SELECT scan_id,agent_id,agent_epoch,root_identity,mount_path,start_sequence,end_sequence,shard_index,shard_count,expected_parts,expected_files,manifest_sha256,root_generation FROM external_asset_sync_runs WHERE scan_id=?`, id).Scan(&s.ScanID, &s.AgentID, &s.AgentEpoch, &s.RootIdentity, &s.OriginRoot, &s.StartSequence, &s.EndSequence, &s.ShardIndex, &s.ShardCount, &s.Parts, &s.Files, &s.ManifestSHA256, &s.RootGeneration)
	if err != nil {
		return s, nil, err
	}
	rows, err := r.db.db.QueryContext(ctx, `SELECT part_no,sha256,payload FROM external_asset_scan_parts WHERE scan_id=? ORDER BY part_no`, id)
	if err != nil {
		return s, nil, err
	}
	defer rows.Close()
	parts := []domain.ExternalMediaScanPart{}
	for rows.Next() {
		p := domain.ExternalMediaScanPart{ScanID: id}
		var raw []byte
		if err = rows.Scan(&p.Part, &p.SHA256, &raw); err != nil {
			return s, nil, err
		}
		if err = json.Unmarshal(raw, &p.Items); err != nil {
			return s, nil, err
		}
		parts = append(parts, p)
	}
	return s, parts, rows.Err()
}

func mediaScanOwns(s domain.ExternalMediaScan, path string) bool {
	if !strings.HasPrefix(path, s.OriginRoot+"/") {
		return false
	}
	rel := strings.TrimPrefix(path, s.OriginRoot+"/")
	top, _, _ := strings.Cut(rel, "/")
	h := fnv.New32a()
	_, _ = h.Write([]byte(top))
	return int(h.Sum32()%uint32(s.ShardCount)) == s.ShardIndex
}

func (r *externalAssetRepo) FinishMediaScan(ctx context.Context, s domain.ExternalMediaScan) error {
	_, err := r.db.db.ExecContext(ctx, `UPDATE external_asset_sync_runs SET status='validated',end_sequence=?,expected_parts=?,expected_files=?,manifest_sha256=? WHERE scan_id=? AND status IN ('running','validated')`, s.EndSequence, s.Parts, s.Files, s.ManifestSHA256, s.ScanID)
	if err != nil {
		return err
	}
	rows, err := r.db.db.QueryContext(ctx, `SELECT scan_id,status FROM external_asset_sync_runs WHERE root_generation=(SELECT root_generation FROM external_asset_sync_runs WHERE scan_id=?) AND run_type='nas_snapshot' AND status IN ('validated','succeeded') ORDER BY shard_index`, s.ScanID)
	if err != nil {
		return err
	}
	ids := []string{}
	for rows.Next() {
		var id, status string
		if err = rows.Scan(&id, &status); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	// No absence transitions until BOTH shards of this server-assigned round
	// have applied a complete manifest. A failed or absent shard preserves rows.
	if len(ids) != 2 {
		return nil
	}
	for _, id := range ids {
		scan, _, e := r.GetMediaScan(ctx, id)
		if e != nil {
			return e
		}
		if e = r.reconcileMediaScanAbsences(ctx, scan); e != nil {
			return e
		}
	}
	return nil
}

func (r *externalAssetRepo) reconcileMediaScanAbsences(ctx context.Context, s domain.ExternalMediaScan) error {
	// All input parts have already been validated and applied by the service.
	// Each individual missing transition still compares the event watermark.
	rows, err := r.db.db.QueryContext(ctx, `SELECT e.origin_path FROM external_asset_records e
 LEFT JOIN external_asset_source_fingerprints f USING(origin_path_hash)
 WHERE e.mount_path='/p3' AND e.is_dir=0 AND e.status='indexed' AND COALESCE(f.scan_id,'')<>?
 AND (COALESCE(f.event_epoch,'')<>? OR COALESCE(f.event_seq,0)<=?)`, s.ScanID, s.AgentEpoch, s.StartSequence)
	if err != nil {
		return err
	}
	paths := []string{}
	for rows.Next() {
		var p string
		if err = rows.Scan(&p); err != nil {
			rows.Close()
			return err
		}
		if mediaScanOwns(s, p) {
			paths = append(paths, p)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, p := range paths {
		err = r.ApplyMediaDelete(ctx, s.AgentID, domain.ExternalAssetFilesystemEvent{Type: domain.ExternalAssetFilesystemEventDelete, MountPath: "/p3", OriginPath: p, RootIdentity: s.RootIdentity, AgentEpoch: s.AgentEpoch, Sequence: s.StartSequence, ScanID: s.ScanID, ObservedAt: time.Now().UTC()})
		if err != nil {
			return err
		}
	}
	_, err = r.db.db.ExecContext(ctx, `UPDATE external_asset_sync_runs SET status='succeeded',finished_at=UTC_TIMESTAMP(),end_sequence=?,expected_parts=?,expected_files=?,manifest_sha256=?,scanned_count=?,upserted_count=? WHERE scan_id=? AND status='validated'`, s.EndSequence, s.Parts, s.Files, s.ManifestSHA256, s.Files, s.Files, s.ScanID)
	return err
}
