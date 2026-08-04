package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

func upsertAIRetrievalProjection(ctx context.Context, tx *sql.Tx, entityType string, entityID int64, embeddingVersion string) error {
	if tx == nil || entityID <= 0 || strings.TrimSpace(embeddingVersion) == "" {
		return nil
	}
	documentID := retrievalDocumentID(entityType, entityID)
	var sourceCount int
	var err error
	switch entityType {
	case "task":
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_search_documents WHERE task_id=?`, entityID).Scan(&sourceCount); err != nil {
			return err
		}
		if sourceCount == 0 {
			return markAIRetrievalProjectionDeleted(ctx, tx, documentID, entityType, entityID, embeddingVersion)
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO ai_retrieval_documents (
			  document_id, entity_type, entity_id, title, internal_route, search_text, content_hash,
			  entity_version, visibility, metadata_json, embedding_version, deleted_at
			)
			SELECT ?, 'task', CAST(d.task_id AS CHAR),
			       CONCAT_WS(' · ', d.task_no, NULLIF(d.product_name_snapshot,'')),
			       CONCAT('/tasks/', d.task_id), d.search_text,
			       SHA2(CONCAT_WS('|', d.task_no, d.search_text, UNIX_TIMESTAMP(d.updated_at)), 256),
			       CAST(UNIX_TIMESTAMP(d.updated_at) AS CHAR), 'internal',
			       JSON_OBJECT('task_id', d.task_id, 'task_no', d.task_no, 'task_type', d.task_type,
			         'task_status', d.task_status, 'owner_department_id', t.owner_department_id,
			         'owner_team_id', t.owner_team_id), ?, NULL
			FROM task_search_documents d
			JOIN tasks t ON t.id=d.task_id
			WHERE d.task_id=?
			ON DUPLICATE KEY UPDATE
			  vector_indexed_at=IF(content_hash=VALUES(content_hash) AND embedding_version=VALUES(embedding_version), vector_indexed_at, NULL),
			  title=VALUES(title), internal_route=VALUES(internal_route), search_text=VALUES(search_text),
			  content_hash=VALUES(content_hash), entity_version=VALUES(entity_version), visibility=VALUES(visibility),
			  metadata_json=VALUES(metadata_json), embedding_version=VALUES(embedding_version), deleted_at=NULL`,
			documentID, embeddingVersion, entityID)
	case "task_resource_group":
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_asset_group_search_documents WHERE group_id=?`, entityID).Scan(&sourceCount); err != nil {
			return err
		}
		if sourceCount == 0 {
			return markAIRetrievalProjectionDeleted(ctx, tx, documentID, entityType, entityID, embeddingVersion)
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO ai_retrieval_documents (
			  document_id, entity_type, entity_id, title, internal_route, search_text, content_hash,
			  entity_version, visibility, metadata_json, embedding_version, deleted_at
			)
			SELECT ?, 'task_resource_group', CAST(d.group_id AS CHAR),
			       CONCAT_WS(' · ', t.task_no, NULLIF(tsi.sku_code,'')), CONCAT('/asset-center/', d.group_id),
			       d.internal_text, SHA2(CONCAT_WS('|', d.internal_text, d.finalized_revision_id), 256),
			       CAST(d.finalized_revision_id AS CHAR),
			       IF(EXISTS (SELECT 1 FROM asset_workbench_client_materials cm
			                 WHERE cm.resource_group_id=d.group_id AND cm.finalized_revision_id=d.finalized_revision_id AND cm.enabled=1), 'published', 'internal'),
			       JSON_OBJECT('resource_group_id', d.group_id, 'task_id', d.task_id, 'task_no', t.task_no,
			         'sku_code', COALESCE(tsi.sku_code,''), 'finalized_revision_id', d.finalized_revision_id,
			         'mode', revision.mode, 'owner_department_id', t.owner_department_id, 'owner_team_id', t.owner_team_id),
			       ?, NULL
			FROM task_asset_group_search_documents d
			JOIN task_asset_groups g ON g.id=d.group_id AND g.finalized_revision_id=d.finalized_revision_id
			JOIN task_asset_group_revisions revision ON revision.id=d.finalized_revision_id
			JOIN tasks t ON t.id=d.task_id
			LEFT JOIN task_sku_items tsi ON tsi.id=g.task_sku_item_id
			WHERE d.group_id=?
			ON DUPLICATE KEY UPDATE
			  vector_indexed_at=IF(content_hash=VALUES(content_hash) AND embedding_version=VALUES(embedding_version), vector_indexed_at, NULL),
			  title=VALUES(title), internal_route=VALUES(internal_route), search_text=VALUES(search_text),
			  content_hash=VALUES(content_hash), entity_version=VALUES(entity_version), visibility=VALUES(visibility),
			  metadata_json=VALUES(metadata_json), embedding_version=VALUES(embedding_version), deleted_at=NULL`,
			documentID, embeddingVersion, entityID)
	default:
		return fmt.Errorf("unsupported AI retrieval projection type %q", entityType)
	}
	if err != nil {
		return fmt.Errorf("upsert AI retrieval projection %s:%d: %w", entityType, entityID, err)
	}
	return enqueueAIRetrievalProjection(ctx, tx, documentID)
}

func enqueueAIRetrievalProjection(ctx context.Context, tx *sql.Tx, documentID string) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO ai_retrieval_outbox (document_id, operation, content_hash, embedding_version, dedupe_key)
		SELECT document_id, 'upsert', content_hash, embedding_version,
		       CONCAT(document_id, ':upsert:', content_hash, ':', embedding_version)
		FROM ai_retrieval_documents WHERE document_id=? AND deleted_at IS NULL
		ON DUPLICATE KEY UPDATE status='pending', next_retry_at=NULL, lease_token=NULL, lease_until=NULL`, documentID)
	if err != nil {
		return fmt.Errorf("enqueue AI retrieval projection: %w", err)
	}
	return nil
}

func markAIRetrievalProjectionDeleted(ctx context.Context, tx *sql.Tx, documentID, entityType string, entityID int64, embeddingVersion string) error {
	result, err := tx.ExecContext(ctx, `
		UPDATE ai_retrieval_documents SET deleted_at=UTC_TIMESTAMP(), vector_indexed_at=NULL
		WHERE document_id=? AND deleted_at IS NULL`, documentID)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return nil
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO ai_retrieval_outbox (document_id, operation, content_hash, embedding_version, dedupe_key)
		VALUES (?, 'delete', SHA2(CONCAT(?, ':', ?), 256), ?, CONCAT(?, ':delete:', ?))
		ON DUPLICATE KEY UPDATE status='pending', next_retry_at=NULL, lease_token=NULL, lease_until=NULL`,
		documentID, entityType, entityID, embeddingVersion, documentID, embeddingVersion)
	return err
}

func upsertAIExternalAssetProjection(ctx context.Context, tx *sql.Tx, originHash, embeddingVersion string) error {
	var entityID int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM external_asset_records WHERE origin_path_hash=? AND status='indexed' AND is_dir=0`, originHash).Scan(&entityID); err != nil {
		return fmt.Errorf("load external asset projection target: %w", err)
	}
	documentID := retrievalDocumentID("external_asset", entityID)
	_, err := tx.ExecContext(ctx, `
		INSERT INTO ai_retrieval_documents (
		  document_id, entity_type, entity_id, title, internal_route, search_text, content_hash,
		  entity_version, visibility, metadata_json, embedding_version, deleted_at
		)
		SELECT ?, 'external_asset', CAST(a.id AS CHAR), a.file_name, CONCAT('/asset-center/ext-', a.id),
		       CONCAT_WS(' ', a.file_name, a.origin_path, a.parent_path, a.searchable_text, a.file_ext, a.mime_type),
		       SHA2(CONCAT_WS('|', a.file_name, a.origin_path, a.searchable_text, a.file_size, UNIX_TIMESTAMP(a.updated_at)), 256),
		       CAST(UNIX_TIMESTAMP(a.updated_at) AS CHAR),
		       IF(EXISTS (SELECT 1 FROM asset_workbench_client_materials cm
		                 WHERE cm.asset_id=a.id AND cm.source_type IN ('external','external_asset') AND cm.enabled=1), 'published', 'internal'),
		       JSON_OBJECT('external_asset_id', a.id, 'kind', a.kind, 'driver', a.driver,
		         'mount_path', a.mount_path, 'file_name', a.file_name, 'file_ext', a.file_ext), ?, NULL
		FROM external_asset_records a WHERE a.id=? AND a.status='indexed' AND a.is_dir=0
		ON DUPLICATE KEY UPDATE
		  vector_indexed_at=IF(content_hash=VALUES(content_hash) AND embedding_version=VALUES(embedding_version), vector_indexed_at, NULL),
		  title=VALUES(title), internal_route=VALUES(internal_route), search_text=VALUES(search_text),
		  content_hash=VALUES(content_hash), entity_version=VALUES(entity_version), visibility=VALUES(visibility),
		  metadata_json=VALUES(metadata_json), embedding_version=VALUES(embedding_version), deleted_at=NULL`,
		documentID, embeddingVersion, entityID)
	if err != nil {
		return fmt.Errorf("upsert external asset AI retrieval projection: %w", err)
	}
	return enqueueAIRetrievalProjection(ctx, tx, documentID)
}

func markAIExternalAssetProjectionsMissing(ctx context.Context, tx *sql.Tx, where string, args []any, embeddingVersion string) error {
	rows, err := tx.QueryContext(ctx, `SELECT id FROM external_asset_records WHERE is_dir=0 AND `+where+` ORDER BY id`, args...)
	if err != nil {
		return fmt.Errorf("list missing external asset projections: %w", err)
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, id := range ids {
		documentID := retrievalDocumentID("external_asset", id)
		if err := markAIRetrievalProjectionDeleted(ctx, tx, documentID, "external_asset", id, embeddingVersion); err != nil {
			return err
		}
	}
	return nil
}

func retrievalDocumentID(entityType string, entityID int64) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(entityType+":"+strconv.FormatInt(entityID, 10))).String()
}

func upsertAIRetrievalProjectionBatch(ctx context.Context, tx *sql.Tx, entityType string, entityIDs []int64, embeddingVersion string) error {
	derived, args, documentIDs := aiProjectionIDTable(entityType, entityIDs)
	if derived == "" {
		return nil
	}
	var statement string
	switch entityType {
	case "task":
		statement = `
			INSERT INTO ai_retrieval_documents (
			  document_id, entity_type, entity_id, title, internal_route, search_text, content_hash,
			  entity_version, visibility, metadata_json, embedding_version, deleted_at
			)
			SELECT ids.document_id, 'task', CAST(d.task_id AS CHAR),
			       CONCAT_WS(' · ', d.task_no, NULLIF(d.product_name_snapshot,'')),
			       CONCAT('/tasks/', d.task_id), d.search_text,
			       SHA2(CONCAT_WS('|', d.task_no, d.search_text, UNIX_TIMESTAMP(d.updated_at)), 256),
			       CAST(UNIX_TIMESTAMP(d.updated_at) AS CHAR), 'internal',
			       JSON_OBJECT('task_id', d.task_id, 'task_no', d.task_no, 'task_type', d.task_type,
			         'task_status', d.task_status, 'owner_department_id', t.owner_department_id,
			         'owner_team_id', t.owner_team_id), ?, NULL
			FROM (` + derived + `) ids
			JOIN task_search_documents d ON d.task_id=ids.entity_id
			JOIN tasks t ON t.id=d.task_id
			ON DUPLICATE KEY UPDATE
			  vector_indexed_at=IF(content_hash=VALUES(content_hash) AND embedding_version=VALUES(embedding_version), vector_indexed_at, NULL),
			  title=VALUES(title), internal_route=VALUES(internal_route), search_text=VALUES(search_text),
			  content_hash=VALUES(content_hash), entity_version=VALUES(entity_version), visibility=VALUES(visibility),
			  metadata_json=VALUES(metadata_json), embedding_version=VALUES(embedding_version), deleted_at=NULL`
	case "task_resource_group":
		statement = `
			INSERT INTO ai_retrieval_documents (
			  document_id, entity_type, entity_id, title, internal_route, search_text, content_hash,
			  entity_version, visibility, metadata_json, embedding_version, deleted_at
			)
			SELECT ids.document_id, 'task_resource_group', CAST(d.group_id AS CHAR),
			       CONCAT_WS(' · ', t.task_no, NULLIF(tsi.sku_code,'')), CONCAT('/asset-center/', d.group_id),
			       d.internal_text, SHA2(CONCAT_WS('|', d.internal_text, d.finalized_revision_id), 256),
			       CAST(d.finalized_revision_id AS CHAR),
			       IF(EXISTS (SELECT 1 FROM asset_workbench_client_materials cm
			                 WHERE cm.resource_group_id=d.group_id AND cm.finalized_revision_id=d.finalized_revision_id AND cm.enabled=1), 'published', 'internal'),
			       JSON_OBJECT('resource_group_id', d.group_id, 'task_id', d.task_id, 'task_no', t.task_no,
			         'sku_code', COALESCE(tsi.sku_code,''), 'finalized_revision_id', d.finalized_revision_id,
			         'mode', revision.mode, 'owner_department_id', t.owner_department_id, 'owner_team_id', t.owner_team_id),
			       ?, NULL
			FROM (` + derived + `) ids
			JOIN task_asset_group_search_documents d ON d.group_id=ids.entity_id
			JOIN task_asset_groups g ON g.id=d.group_id AND g.finalized_revision_id=d.finalized_revision_id
			JOIN task_asset_group_revisions revision ON revision.id=d.finalized_revision_id
			JOIN tasks t ON t.id=d.task_id
			LEFT JOIN task_sku_items tsi ON tsi.id=g.task_sku_item_id
			ON DUPLICATE KEY UPDATE
			  vector_indexed_at=IF(content_hash=VALUES(content_hash) AND embedding_version=VALUES(embedding_version), vector_indexed_at, NULL),
			  title=VALUES(title), internal_route=VALUES(internal_route), search_text=VALUES(search_text),
			  content_hash=VALUES(content_hash), entity_version=VALUES(entity_version), visibility=VALUES(visibility),
			  metadata_json=VALUES(metadata_json), embedding_version=VALUES(embedding_version), deleted_at=NULL`
	default:
		return fmt.Errorf("unsupported AI retrieval projection type %q", entityType)
	}
	execArgs := make([]any, 0, len(args)+1)
	execArgs = append(execArgs, embeddingVersion)
	execArgs = append(execArgs, args...)
	if _, err := tx.ExecContext(ctx, statement, execArgs...); err != nil {
		return fmt.Errorf("batch upsert AI retrieval projection %s: %w", entityType, err)
	}
	return enqueueAIRetrievalProjectionBatch(ctx, tx, documentIDs)
}

func upsertAIExternalAssetProjectionBatch(ctx context.Context, tx *sql.Tx, entityIDs []int64, embeddingVersion string) error {
	derived, args, documentIDs := aiProjectionIDTable("external_asset", entityIDs)
	if derived == "" {
		return nil
	}
	execArgs := make([]any, 0, len(args)+1)
	execArgs = append(execArgs, embeddingVersion)
	execArgs = append(execArgs, args...)
	_, err := tx.ExecContext(ctx, `
		INSERT INTO ai_retrieval_documents (
		  document_id, entity_type, entity_id, title, internal_route, search_text, content_hash,
		  entity_version, visibility, metadata_json, embedding_version, deleted_at
		)
		SELECT ids.document_id, 'external_asset', CAST(a.id AS CHAR), a.file_name, CONCAT('/asset-center/ext-', a.id),
		       CONCAT_WS(' ', a.file_name, a.origin_path, a.parent_path, a.searchable_text, a.file_ext, a.mime_type),
		       SHA2(CONCAT_WS('|', a.file_name, a.origin_path, a.searchable_text, a.file_size, UNIX_TIMESTAMP(a.updated_at)), 256),
		       CAST(UNIX_TIMESTAMP(a.updated_at) AS CHAR),
		       IF(EXISTS (SELECT 1 FROM asset_workbench_client_materials cm
		                 WHERE cm.asset_id=a.id AND cm.source_type IN ('external','external_asset') AND cm.enabled=1), 'published', 'internal'),
		       JSON_OBJECT('external_asset_id', a.id, 'kind', a.kind, 'driver', a.driver,
		         'mount_path', a.mount_path, 'file_name', a.file_name, 'file_ext', a.file_ext), ?, NULL
		FROM (`+derived+`) ids
		JOIN external_asset_records a ON a.id=ids.entity_id AND a.status='indexed' AND a.is_dir=0
		ON DUPLICATE KEY UPDATE
		  vector_indexed_at=IF(content_hash=VALUES(content_hash) AND embedding_version=VALUES(embedding_version), vector_indexed_at, NULL),
		  title=VALUES(title), internal_route=VALUES(internal_route), search_text=VALUES(search_text),
		  content_hash=VALUES(content_hash), entity_version=VALUES(entity_version), visibility=VALUES(visibility),
		  metadata_json=VALUES(metadata_json), embedding_version=VALUES(embedding_version), deleted_at=NULL`, execArgs...)
	if err != nil {
		return fmt.Errorf("batch upsert external asset AI retrieval projection: %w", err)
	}
	return enqueueAIRetrievalProjectionBatch(ctx, tx, documentIDs)
}

func enqueueAIRetrievalProjectionBatch(ctx context.Context, tx *sql.Tx, documentIDs []string) error {
	if len(documentIDs) == 0 {
		return nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(documentIDs)), ",")
	args := make([]any, len(documentIDs))
	for index, id := range documentIDs {
		args[index] = id
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO ai_retrieval_outbox (document_id, operation, content_hash, embedding_version, dedupe_key)
		SELECT document_id, 'upsert', content_hash, embedding_version,
		       CONCAT(document_id, ':upsert:', content_hash, ':', embedding_version)
		FROM ai_retrieval_documents WHERE document_id IN (`+placeholders+`) AND deleted_at IS NULL
		ON DUPLICATE KEY UPDATE status='pending', next_retry_at=NULL, lease_token=NULL, lease_until=NULL`, args...)
	if err != nil {
		return fmt.Errorf("batch enqueue AI retrieval projections: %w", err)
	}
	return nil
}

func aiProjectionIDTable(entityType string, entityIDs []int64) (string, []any, []string) {
	parts := make([]string, 0, len(entityIDs))
	args := make([]any, 0, len(entityIDs)*2)
	documentIDs := make([]string, 0, len(entityIDs))
	seen := make(map[int64]struct{}, len(entityIDs))
	for _, entityID := range entityIDs {
		if entityID <= 0 {
			continue
		}
		if _, exists := seen[entityID]; exists {
			continue
		}
		seen[entityID] = struct{}{}
		documentID := retrievalDocumentID(entityType, entityID)
		if len(parts) == 0 {
			parts = append(parts, "SELECT ? AS entity_id, ? AS document_id")
		} else {
			parts = append(parts, "UNION ALL SELECT ?, ?")
		}
		args = append(args, entityID, documentID)
		documentIDs = append(documentIDs, documentID)
	}
	return strings.Join(parts, " "), args, documentIDs
}
