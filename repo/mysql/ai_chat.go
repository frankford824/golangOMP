package mysqlrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type aiChatRepo struct{ db *DB }

func NewAIChatRepo(db *DB) repo.AIChatRepo { return &aiChatRepo{db: db} }

func (r *aiChatRepo) CreateConversation(ctx context.Context, tx repo.Tx, item domain.AIConversation) error {
	_, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO ai_conversations
		  (id, owner_user_id, title, status, lock_version, expires_at, created_at, updated_at)
		VALUES (?, ?, ?, 'active', 0, ?, ?, ?)`,
		item.ID, item.OwnerUserID, item.Title, item.ExpiresAt, item.CreatedAt, item.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create ai conversation: %w", err)
	}
	return nil
}

func (r *aiChatRepo) ListConversations(ctx context.Context, ownerUserID *int64, filter domain.AIAdminConversationFilter) ([]domain.AIConversation, int64, error) {
	page, pageSize := filter.Page, filter.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	where := []string{"c.status <> 'deleted'", "c.expires_at > UTC_TIMESTAMP()"}
	args := make([]any, 0, 8)
	if ownerUserID != nil {
		where = append(where, "c.owner_user_id = ?")
		args = append(args, *ownerUserID)
	} else if filter.OwnerUserID != nil {
		where = append(where, "c.owner_user_id = ?")
		args = append(args, *filter.OwnerUserID)
	}
	if status := strings.TrimSpace(filter.Status); status != "" {
		where = append(where, "c.status = ?")
		args = append(args, status)
	}
	if filter.From != nil {
		where = append(where, "c.updated_at >= ?")
		args = append(args, *filter.From)
	}
	if filter.To != nil {
		where = append(where, "c.updated_at < ?")
		args = append(args, *filter.To)
	}
	whereSQL := strings.Join(where, " AND ")
	var total int64
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ai_conversations c WHERE `+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count ai conversations: %w", err)
	}
	queryArgs := append(append([]any{}, args...), pageSize, (page-1)*pageSize)
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT c.id, c.owner_user_id, COALESCE(NULLIF(u.display_name,''), u.username, ''),
		       c.title, c.status, c.lock_version, c.expires_at, c.deleted_at, c.created_at, c.updated_at
		FROM ai_conversations c
		JOIN users u ON u.id = c.owner_user_id
		WHERE `+whereSQL+`
		ORDER BY c.updated_at DESC, c.id DESC
		LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list ai conversations: %w", err)
	}
	defer rows.Close()
	items := make([]domain.AIConversation, 0, pageSize)
	for rows.Next() {
		var item domain.AIConversation
		var deletedAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.OwnerUserID, &item.OwnerName, &item.Title, &item.Status, &item.LockVersion, &item.ExpiresAt, &deletedAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan ai conversation: %w", err)
		}
		if deletedAt.Valid {
			item.DeletedAt = &deletedAt.Time
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate ai conversations: %w", err)
	}
	return items, total, nil
}

func (r *aiChatRepo) GetConversation(ctx context.Context, id string) (*domain.AIConversation, error) {
	var item domain.AIConversation
	var deletedAt sql.NullTime
	err := r.db.db.QueryRowContext(ctx, `
		SELECT c.id, c.owner_user_id, COALESCE(NULLIF(u.display_name,''), u.username, ''),
		       c.title, c.status, c.lock_version, c.expires_at, c.deleted_at, c.created_at, c.updated_at
		FROM ai_conversations c
		JOIN users u ON u.id = c.owner_user_id
		WHERE c.id = ? AND c.status <> 'deleted' AND c.expires_at > UTC_TIMESTAMP()`, id).
		Scan(&item.ID, &item.OwnerUserID, &item.OwnerName, &item.Title, &item.Status, &item.LockVersion, &item.ExpiresAt, &deletedAt, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if deletedAt.Valid {
		item.DeletedAt = &deletedAt.Time
	}
	return &item, nil
}

func (r *aiChatRepo) ListMessages(ctx context.Context, conversationID string, limit int) ([]domain.AIMessage, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT id, conversation_id, COALESCE(reply_to_message_id,''), COALESCE(client_message_id,''), role, content, status,
		       provider, model, input_tokens, output_tokens, finish_reason, error_code,
		       started_at, completed_at, created_at, updated_at
		FROM ai_messages
		WHERE conversation_id = ?
		ORDER BY created_at ASC, id ASC
		LIMIT ?`, conversationID, limit)
	if err != nil {
		return nil, fmt.Errorf("list ai messages: %w", err)
	}
	defer rows.Close()
	items := make([]domain.AIMessage, 0, limit)
	ids := make([]string, 0, limit)
	for rows.Next() {
		item, err := scanAIMessage(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
		ids = append(ids, item.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ai messages: %w", err)
	}
	if len(ids) == 0 {
		return items, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, len(ids))
	for i := range ids {
		args[i] = ids[i]
	}
	sourceRows, err := r.db.db.QueryContext(ctx, `
		SELECT id, message_id, source_id, entity_type, entity_id, title, internal_route,
		       evidence_excerpt, source_version, rank_no
		FROM ai_message_sources WHERE message_id IN (`+placeholders+`)
		ORDER BY message_id, rank_no`, args...)
	if err != nil {
		return nil, fmt.Errorf("list ai message sources: %w", err)
	}
	defer sourceRows.Close()
	byMessage := make(map[string][]domain.AIMessageSource)
	for sourceRows.Next() {
		var source domain.AIMessageSource
		if err := sourceRows.Scan(&source.ID, &source.MessageID, &source.SourceID, &source.EntityType, &source.EntityID, &source.Title, &source.InternalRoute, &source.EvidenceExcerpt, &source.SourceVersion, &source.Rank); err != nil {
			return nil, fmt.Errorf("scan ai message source: %w", err)
		}
		byMessage[source.MessageID] = append(byMessage[source.MessageID], source)
	}
	for i := range items {
		items[i].Sources = byMessage[items[i].ID]
		if items[i].Sources == nil {
			items[i].Sources = []domain.AIMessageSource{}
		}
	}
	return items, nil
}

func (r *aiChatRepo) FindUserMessageByClientID(ctx context.Context, conversationID, clientMessageID string) (*domain.AIMessage, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT id, conversation_id, COALESCE(reply_to_message_id,''), COALESCE(client_message_id,''), role, content, status,
		       provider, model, input_tokens, output_tokens, finish_reason, error_code,
		       started_at, completed_at, created_at, updated_at
		FROM ai_messages WHERE conversation_id=? AND client_message_id=?`, conversationID, clientMessageID)
	return scanAIMessage(row)
}

func (r *aiChatRepo) FindAssistantByReplyTo(ctx context.Context, userMessageID string) (*domain.AIMessage, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT id, conversation_id, COALESCE(reply_to_message_id,''), COALESCE(client_message_id,''), role, content, status,
		       provider, model, input_tokens, output_tokens, finish_reason, error_code,
		       started_at, completed_at, created_at, updated_at
		FROM ai_messages WHERE reply_to_message_id=?`, userMessageID)
	return scanAIMessage(row)
}

type aiMessageScanner interface{ Scan(...any) error }

func scanAIMessage(row aiMessageScanner) (*domain.AIMessage, error) {
	var item domain.AIMessage
	var startedAt, completedAt sql.NullTime
	if err := row.Scan(&item.ID, &item.ConversationID, &item.ReplyToMessageID, &item.ClientMessageID, &item.Role, &item.Content, &item.Status,
		&item.Provider, &item.Model, &item.InputTokens, &item.OutputTokens, &item.FinishReason, &item.ErrorCode,
		&startedAt, &completedAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return nil, err
	}
	if startedAt.Valid {
		item.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		item.CompletedAt = &completedAt.Time
	}
	return &item, nil
}

func (r *aiChatRepo) CreateMessage(ctx context.Context, tx repo.Tx, item domain.AIMessage) error {
	_, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO ai_messages
		  (id, conversation_id, reply_to_message_id, client_message_id, role, content, status, provider, model,
		   input_tokens, output_tokens, finish_reason, error_code, started_at, completed_at, created_at, updated_at)
		VALUES (?, ?, NULLIF(?,''), NULLIF(?,''), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.ID, item.ConversationID, item.ReplyToMessageID, item.ClientMessageID, item.Role, item.Content, item.Status,
		item.Provider, item.Model, item.InputTokens, item.OutputTokens, item.FinishReason, item.ErrorCode,
		item.StartedAt, item.CompletedAt, item.CreatedAt, item.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create ai message: %w", err)
	}
	_, err = Unwrap(tx).ExecContext(ctx, `
		UPDATE ai_conversations SET updated_at=?, lock_version=lock_version+1,
		  title=CASE WHEN title='' AND ?='user' THEN LEFT(?, 80) ELSE title END
		WHERE id=? AND status='active'`, item.UpdatedAt, item.Role, item.Content, item.ConversationID)
	if err != nil {
		return fmt.Errorf("touch ai conversation: %w", err)
	}
	return nil
}

func (r *aiChatRepo) FinalizeMessage(ctx context.Context, tx repo.Tx, item domain.AIMessage) error {
	result, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE ai_messages
		SET content=?, status=?, provider=?, model=?, input_tokens=?, output_tokens=?,
		    finish_reason=?, error_code=?, completed_at=?, updated_at=?
		WHERE id=? AND conversation_id=? AND status='streaming'`,
		item.Content, item.Status, item.Provider, item.Model, item.InputTokens, item.OutputTokens,
		item.FinishReason, item.ErrorCode, item.CompletedAt, item.UpdatedAt, item.ID, item.ConversationID)
	if err != nil {
		return fmt.Errorf("finalize ai message: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return fmt.Errorf("finalize ai message affected %d rows", affected)
	}
	return nil
}

func (r *aiChatRepo) ReplaceMessageSources(ctx context.Context, tx repo.Tx, messageID string, sources []domain.AIMessageSource) error {
	if _, err := Unwrap(tx).ExecContext(ctx, `DELETE FROM ai_message_sources WHERE message_id=?`, messageID); err != nil {
		return fmt.Errorf("clear ai message sources: %w", err)
	}
	for _, source := range sources {
		if _, err := Unwrap(tx).ExecContext(ctx, `
			INSERT INTO ai_message_sources
			  (message_id, source_id, entity_type, entity_id, title, internal_route, evidence_excerpt, source_version, rank_no)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			messageID, source.SourceID, source.EntityType, source.EntityID, source.Title, source.InternalRoute,
			source.EvidenceExcerpt, source.SourceVersion, source.Rank); err != nil {
			return fmt.Errorf("insert ai message source: %w", err)
		}
	}
	return nil
}

func (r *aiChatRepo) SoftDeleteConversation(ctx context.Context, tx repo.Tx, id string, ownerUserID int64, now time.Time) (bool, error) {
	result, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE ai_conversations
		SET status='deleted', deleted_at=?, purge_after=?, lock_version=lock_version+1, updated_at=?
		WHERE id=? AND owner_user_id=? AND status='active'`, now, now.Add(24*time.Hour), now, id, ownerUserID)
	if err != nil {
		return false, fmt.Errorf("soft delete ai conversation: %w", err)
	}
	affected, _ := result.RowsAffected()
	return affected == 1, nil
}

func (r *aiChatRepo) PurgeExpiredConversations(ctx context.Context, tx repo.Tx, now time.Time, limit int) (int64, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	result, err := Unwrap(tx).ExecContext(ctx, `
		DELETE FROM ai_conversations
		WHERE (status='deleted' AND purge_after IS NOT NULL AND purge_after <= ?)
		   OR (status='active' AND expires_at <= ?)
		ORDER BY updated_at, id
		LIMIT ?`, now, now, limit)
	if err != nil {
		return 0, fmt.Errorf("purge ai conversations: %w", err)
	}
	return result.RowsAffected()
}

func (r *aiChatRepo) InsertProviderCall(ctx context.Context, tx repo.Tx, item domain.AIProviderCall) error {
	_, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO ai_provider_calls
		  (conversation_id, message_id, scene, provider, model, status, latency_ms, input_tokens,
		   output_tokens, request_hash, response_hash, error_code)
		VALUES (NULLIF(?,''), NULLIF(?,''), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.ConversationID, item.MessageID, item.Scene, item.Provider, item.Model, item.Status, item.LatencyMS,
		item.InputTokens, item.OutputTokens, item.RequestHash, item.ResponseHash, item.ErrorCode)
	if err != nil {
		return fmt.Errorf("insert ai provider call: %w", err)
	}
	return nil
}

func (r *aiChatRepo) InsertAccessAudit(ctx context.Context, tx repo.Tx, item domain.AIAccessAudit) error {
	_, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO ai_access_audit
		  (actor_user_id, target_user_id, conversation_id, action, outcome, reason)
		VALUES (?, ?, ?, ?, ?, ?)`, item.ActorUserID, item.TargetUserID, item.ConversationID, item.Action, item.Outcome, item.Reason)
	if err != nil {
		return fmt.Errorf("insert ai access audit: %w", err)
	}
	return nil
}

type aiRetrievalRepo struct{ db *DB }

func NewAIRetrievalRepo(db *DB) repo.AIRetrievalRepo { return &aiRetrievalRepo{db: db} }

func (r *aiRetrievalRepo) UpsertRetrievalDocument(ctx context.Context, tx repo.Tx, item domain.AIRetrievalDocument) error {
	metadata, err := json.Marshal(item.Metadata)
	if err != nil {
		return fmt.Errorf("marshal retrieval metadata: %w", err)
	}
	_, err = Unwrap(tx).ExecContext(ctx, `
		INSERT INTO ai_retrieval_documents
		  (document_id, entity_type, entity_id, title, internal_route, search_text, content_hash,
		   entity_version, visibility, metadata_json, embedding_version, deleted_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		  document_id=VALUES(document_id), title=VALUES(title), internal_route=VALUES(internal_route),
		  vector_indexed_at=IF(content_hash=VALUES(content_hash) AND embedding_version=VALUES(embedding_version), vector_indexed_at, NULL),
		  search_text=VALUES(search_text), content_hash=VALUES(content_hash), entity_version=VALUES(entity_version),
		  visibility=VALUES(visibility), metadata_json=VALUES(metadata_json), embedding_version=VALUES(embedding_version),
		  deleted_at=VALUES(deleted_at)`,
		item.DocumentID, item.EntityType, item.EntityID, item.Title, item.InternalRoute, item.SearchText,
		item.ContentHash, item.EntityVersion, item.Visibility, metadata, item.EmbeddingVersion, item.DeletedAt)
	if err != nil {
		return fmt.Errorf("upsert ai retrieval document: %w", err)
	}
	return nil
}

func (r *aiRetrievalRepo) GetRetrievalDocument(ctx context.Context, documentID string) (*domain.AIRetrievalDocument, error) {
	var item domain.AIRetrievalDocument
	var metadata []byte
	var indexedAt, deletedAt sql.NullTime
	err := r.db.db.QueryRowContext(ctx, `
		SELECT document_id, entity_type, entity_id, title, internal_route, search_text, content_hash,
		       entity_version, visibility, metadata_json, embedding_version, vector_indexed_at, deleted_at
		FROM ai_retrieval_documents WHERE document_id=?`, documentID).
		Scan(&item.DocumentID, &item.EntityType, &item.EntityID, &item.Title, &item.InternalRoute, &item.SearchText,
			&item.ContentHash, &item.EntityVersion, &item.Visibility, &metadata, &item.EmbeddingVersion, &indexedAt, &deletedAt)
	if err != nil {
		return nil, err
	}
	if len(metadata) > 0 {
		_ = json.Unmarshal(metadata, &item.Metadata)
	}
	if indexedAt.Valid {
		item.VectorIndexedAt = &indexedAt.Time
	}
	if deletedAt.Valid {
		item.DeletedAt = &deletedAt.Time
	}
	return &item, nil
}

func (r *aiRetrievalRepo) SearchRetrievalDocuments(ctx context.Context, query string, limit int) ([]domain.AIRetrievalHit, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT document_id, entity_type, entity_id, title, internal_route,
		       LEFT(search_text, 1200), entity_version,
		       MATCH(title, search_text) AGAINST (? IN NATURAL LANGUAGE MODE) AS score,
		       metadata_json
		FROM ai_retrieval_documents
		WHERE deleted_at IS NULL
		  AND MATCH(title, search_text) AGAINST (? IN NATURAL LANGUAGE MODE)
		ORDER BY score DESC, updated_at DESC, document_id
		LIMIT ?`, query, query, limit)
	if err != nil {
		return nil, fmt.Errorf("search ai retrieval documents: %w", err)
	}
	defer rows.Close()
	items := make([]domain.AIRetrievalHit, 0, limit)
	for rows.Next() {
		var item domain.AIRetrievalHit
		var metadata []byte
		if err := rows.Scan(&item.DocumentID, &item.EntityType, &item.EntityID, &item.Title, &item.InternalRoute,
			&item.Excerpt, &item.SourceVersion, &item.Score, &metadata); err != nil {
			return nil, fmt.Errorf("scan ai retrieval document: %w", err)
		}
		item.Source = "mysql"
		_ = json.Unmarshal(metadata, &item.Metadata)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ai retrieval documents: %w", err)
	}
	return items, nil
}

func (r *aiRetrievalRepo) AuthorizeRetrievalDocument(ctx context.Context, actor domain.RequestActor, documentID string) (bool, error) {
	var entityType, entityID, visibility string
	if err := r.db.db.QueryRowContext(ctx, `
		SELECT entity_type, entity_id, visibility
		FROM ai_retrieval_documents
		WHERE document_id=? AND deleted_at IS NULL`, documentID).Scan(&entityType, &entityID, &visibility); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("load retrieval authorization target: %w", err)
	}
	switch entityType {
	case "task", "task_resource_group", "system_asset":
		var taskID int64
		switch entityType {
		case "task":
			if _, err := fmt.Sscan(entityID, &taskID); err != nil {
				return false, nil
			}
		case "task_resource_group":
			if err := r.db.db.QueryRowContext(ctx, `SELECT task_id FROM task_asset_groups WHERE id=? AND finalized_revision_id IS NOT NULL`, entityID).Scan(&taskID); err != nil {
				if err == sql.ErrNoRows {
					return false, nil
				}
				return false, err
			}
		case "system_asset":
			if err := r.db.db.QueryRowContext(ctx, `SELECT task_id FROM task_assets WHERE id=? AND deleted_at IS NULL AND access_revoked_at IS NULL`, entityID).Scan(&taskID); err != nil {
				if err == sql.ErrNoRows {
					return false, nil
				}
				return false, err
			}
		}
		var subject domain.TaskAccessSubject
		var requesterID, designerID, handlerID, departmentID, teamID sql.NullInt64
		if err := r.db.db.QueryRowContext(ctx, `
			SELECT id, creator_id, requester_id, designer_id, current_handler_id,
			       owner_department_id, owner_team_id, task_type
			FROM tasks WHERE id=?`, taskID).
			Scan(&subject.TaskID, &subject.CreatorID, &requesterID, &designerID, &handlerID, &departmentID, &teamID, &subject.TaskType); err != nil {
			if err == sql.ErrNoRows {
				return false, nil
			}
			return false, fmt.Errorf("load retrieval task authorization: %w", err)
		}
		subject.RequesterID = fromNullInt64(requesterID)
		subject.DesignerID = fromNullInt64(designerID)
		subject.CurrentHandlerID = fromNullInt64(handlerID)
		subject.OwnerDepartmentID = fromNullInt64(departmentID)
		subject.OwnerTeamID = fromNullInt64(teamID)
		permission := domain.PermissionTaskView
		if entityType != "task" {
			permission = domain.PermissionAssetView
		}
		return domain.EffectiveAccessAllowsTask(actor, permission, subject), nil
	case "external_asset":
		if !domain.ActorHasPermission(actor, domain.PermissionAssetView) {
			return false, nil
		}
		if retrievalPublishedOnly(actor) {
			return visibility == "published", nil
		}
		return true, nil
	case "product":
		return domain.ActorHasPermission(actor, domain.PermissionCatalogView), nil
	default:
		return false, nil
	}
}

func retrievalPublishedOnly(actor domain.RequestActor) bool {
	if actor.EffectiveAccess == nil {
		return false
	}
	found := false
	for _, source := range actor.EffectiveAccess.Sources {
		if source.Permission != domain.PermissionAssetView {
			continue
		}
		found = true
		if source.RoleCode != "asset_submitter" {
			return false
		}
	}
	return found
}

func (r *aiRetrievalRepo) EnqueueRetrievalDocument(ctx context.Context, tx repo.Tx, item domain.AIRetrievalOutboxItem) error {
	dedupe := strings.Join([]string{item.DocumentID, item.Operation, item.ContentHash, item.EmbeddingVersion}, ":")
	_, err := Unwrap(tx).ExecContext(ctx, `
		INSERT IGNORE INTO ai_retrieval_outbox
		  (document_id, operation, content_hash, embedding_version, dedupe_key)
		VALUES (?, ?, ?, ?, ?)`, item.DocumentID, item.Operation, item.ContentHash, item.EmbeddingVersion, dedupe)
	if err != nil {
		return fmt.Errorf("enqueue ai retrieval document: %w", err)
	}
	return nil
}

func (r *aiRetrievalRepo) ClaimRetrievalOutbox(ctx context.Context, tx repo.Tx, leaseToken string, now, leaseUntil time.Time, limit int) ([]domain.AIRetrievalOutboxItem, error) {
	rows, err := Unwrap(tx).QueryContext(ctx, `
		SELECT id, document_id, operation, content_hash, embedding_version, attempt
		FROM ai_retrieval_outbox
		WHERE ((status IN ('pending','retry') AND (next_retry_at IS NULL OR next_retry_at <= ?))
		   OR (status='processing' AND lease_until IS NOT NULL AND lease_until <= ?))
		ORDER BY id
		LIMIT ? FOR UPDATE SKIP LOCKED`, now, now, limit)
	if err != nil {
		return nil, fmt.Errorf("claim ai retrieval outbox: %w", err)
	}
	defer rows.Close()
	items := make([]domain.AIRetrievalOutboxItem, 0, limit)
	ids := make([]int64, 0, limit)
	for rows.Next() {
		var item domain.AIRetrievalOutboxItem
		if err := rows.Scan(&item.ID, &item.DocumentID, &item.Operation, &item.ContentHash, &item.EmbeddingVersion, &item.Attempt); err != nil {
			return nil, fmt.Errorf("scan ai retrieval outbox: %w", err)
		}
		item.Attempt++
		items = append(items, item)
		ids = append(ids, item.ID)
	}
	for _, id := range ids {
		if _, err := Unwrap(tx).ExecContext(ctx, `
			UPDATE ai_retrieval_outbox
			SET status='processing', attempt=attempt+1, lease_token=?, lease_until=?, updated_at=?
			WHERE id=?`, leaseToken, leaseUntil, now, id); err != nil {
			return nil, fmt.Errorf("lease ai retrieval outbox: %w", err)
		}
	}
	return items, nil
}

func (r *aiRetrievalRepo) MarkRetrievalOutboxSucceeded(ctx context.Context, tx repo.Tx, itemID int64, leaseToken string, indexedAt time.Time) error {
	result, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE ai_retrieval_outbox
		SET status='succeeded', lease_token=NULL, lease_until=NULL, last_error=NULL, updated_at=?
		WHERE id=? AND status='processing' AND lease_token=?`, indexedAt, itemID, leaseToken)
	if err != nil {
		return fmt.Errorf("complete ai retrieval outbox: %w", err)
	}
	return requireAffected(result, "complete ai retrieval outbox")
}

func (r *aiRetrievalRepo) MarkRetrievalOutboxRetry(ctx context.Context, tx repo.Tx, itemID int64, leaseToken, lastError string, nextRetryAt time.Time, alert bool) error {
	alertStatus := "none"
	if alert {
		alertStatus = "alert"
	}
	result, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE ai_retrieval_outbox
		SET status='retry', next_retry_at=?, lease_token=NULL, lease_until=NULL,
		    last_error=?, alert_status=?, updated_at=UTC_TIMESTAMP()
		WHERE id=? AND status='processing' AND lease_token=?`, nextRetryAt, lastError, alertStatus, itemID, leaseToken)
	if err != nil {
		return fmt.Errorf("retry ai retrieval outbox: %w", err)
	}
	return requireAffected(result, "retry ai retrieval outbox")
}

func (r *aiRetrievalRepo) MarkRetrievalDocumentIndexed(ctx context.Context, tx repo.Tx, documentID, contentHash, embeddingVersion string, indexedAt time.Time) error {
	result, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE ai_retrieval_documents SET vector_indexed_at=?
		WHERE document_id=? AND content_hash=? AND embedding_version=? AND deleted_at IS NULL`,
		indexedAt, documentID, contentHash, embeddingVersion)
	if err != nil {
		return fmt.Errorf("mark ai retrieval document indexed: %w", err)
	}
	return requireAffected(result, "mark ai retrieval document indexed")
}
