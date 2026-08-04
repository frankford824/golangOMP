package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"
)

type AIRetrievalReindexSummary struct {
	Tasks          int64 `json:"tasks"`
	ResourceGroups int64 `json:"resource_groups"`
	ExternalAssets int64 `json:"external_assets"`
	Documents      int64 `json:"documents"`
	Queued         int64 `json:"queued"`
}

type AIRetrievalProjector struct{ db *DB }

func NewAIRetrievalProjector(db *DB) *AIRetrievalProjector { return &AIRetrievalProjector{db: db} }

func (p *AIRetrievalProjector) Rebuild(ctx context.Context, embeddingVersion string, batchSize int) (AIRetrievalReindexSummary, error) {
	if p == nil || p.db == nil || p.db.db == nil {
		return AIRetrievalReindexSummary{}, fmt.Errorf("AI retrieval projector is not configured")
	}
	if batchSize <= 0 || batchSize > 1000 {
		batchSize = 200
	}
	summary := AIRetrievalReindexSummary{}
	for _, spec := range []struct {
		entityType, table, idColumn string
		count                       *int64
	}{
		{"task", "task_search_documents", "task_id", &summary.Tasks},
		{"task_resource_group", "task_asset_group_search_documents", "group_id", &summary.ResourceGroups},
	} {
		var cursor int64
		for {
			rows, err := p.db.db.QueryContext(ctx, `SELECT `+spec.idColumn+` FROM `+spec.table+` WHERE `+spec.idColumn+`>? ORDER BY `+spec.idColumn+` LIMIT ?`, cursor, batchSize)
			if err != nil {
				return summary, fmt.Errorf("list %s projections: %w", spec.entityType, err)
			}
			ids := make([]int64, 0, batchSize)
			for rows.Next() {
				var id int64
				if err := rows.Scan(&id); err != nil {
					rows.Close()
					return summary, err
				}
				ids = append(ids, id)
			}
			rows.Close()
			if len(ids) == 0 {
				break
			}
			if err := p.projectIDs(ctx, spec.entityType, ids, embeddingVersion); err != nil {
				return summary, err
			}
			*spec.count += int64(len(ids))
			cursor = ids[len(ids)-1]
		}
	}
	var cursor int64
	for {
		rows, err := p.db.db.QueryContext(ctx, `SELECT id, origin_path_hash FROM external_asset_records WHERE id>? AND status='indexed' AND is_dir=0 ORDER BY id LIMIT ?`, cursor, batchSize)
		if err != nil {
			return summary, fmt.Errorf("list external projections: %w", err)
		}
		items := make([]int64, 0, batchSize)
		for rows.Next() {
			var id int64
			var originHash string
			if err := rows.Scan(&id, &originHash); err != nil {
				rows.Close()
				return summary, err
			}
			items = append(items, id)
		}
		rows.Close()
		if len(items) == 0 {
			break
		}
		tx, err := p.db.db.BeginTx(ctx, nil)
		if err != nil {
			return summary, err
		}
		if err := upsertAIExternalAssetProjectionBatch(ctx, tx, items, embeddingVersion); err != nil {
			_ = tx.Rollback()
			return summary, err
		}
		if err := tx.Commit(); err != nil {
			return summary, err
		}
		summary.ExternalAssets += int64(len(items))
		cursor = items[len(items)-1]
	}
	if err := p.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ai_retrieval_documents WHERE deleted_at IS NULL`).Scan(&summary.Documents); err != nil {
		return summary, err
	}
	if err := p.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ai_retrieval_outbox WHERE status IN ('pending','retry','processing')`).Scan(&summary.Queued); err != nil {
		return summary, err
	}
	return summary, nil
}

func (p *AIRetrievalProjector) projectIDs(ctx context.Context, entityType string, ids []int64, embeddingVersion string) error {
	tx, err := p.db.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := upsertAIRetrievalProjectionBatch(ctx, tx, entityType, ids, embeddingVersion); err != nil {
		return err
	}
	return tx.Commit()
}

func (p *AIRetrievalProjector) ListDocuments(ctx context.Context, after string, limit int) ([]string, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	rows, err := p.db.db.QueryContext(ctx, `SELECT document_id FROM ai_retrieval_documents WHERE deleted_at IS NULL AND document_id>? ORDER BY document_id LIMIT ?`, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]string, 0, limit)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		items = append(items, id)
	}
	return items, rows.Err()
}

func (p *AIRetrievalProjector) MarkIndexed(ctx context.Context, documentID string) error {
	_, err := p.db.db.ExecContext(ctx, `UPDATE ai_retrieval_documents SET vector_indexed_at=UTC_TIMESTAMP() WHERE document_id=? AND deleted_at IS NULL`, documentID)
	return err
}
