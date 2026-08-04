-- Migration: 129_ai_chat_vector_retrieval.sql
-- Native data-center chat, traceable evidence, provider accounting and a
-- rebuildable vector-index outbox. MySQL remains the business source of truth.

CREATE TABLE ai_conversations (
  id CHAR(36) NOT NULL,
  owner_user_id BIGINT NOT NULL,
  title VARCHAR(255) NOT NULL DEFAULT '',
  status VARCHAR(24) NOT NULL DEFAULT 'active',
  lock_version BIGINT NOT NULL DEFAULT 0,
  expires_at DATETIME NOT NULL,
  deleted_at DATETIME NULL,
  purge_after DATETIME NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_ai_conversations_owner_updated (owner_user_id, status, updated_at, id),
  KEY idx_ai_conversations_expiry (status, expires_at, id),
  KEY idx_ai_conversations_purge (status, purge_after, id),
  CONSTRAINT fk_ai_conversations_owner FOREIGN KEY (owner_user_id) REFERENCES users(id),
  CONSTRAINT chk_ai_conversations_status CHECK (status IN ('active','deleted'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Owner-scoped native data-center chat conversations';

CREATE TABLE ai_messages (
  id CHAR(36) NOT NULL,
  conversation_id CHAR(36) NOT NULL,
  reply_to_message_id CHAR(36) NULL,
  client_message_id VARCHAR(128) NULL,
  role VARCHAR(16) NOT NULL,
  content MEDIUMTEXT NOT NULL,
  status VARCHAR(24) NOT NULL,
  provider VARCHAR(64) NOT NULL DEFAULT '',
  model VARCHAR(128) NOT NULL DEFAULT '',
  input_tokens BIGINT NOT NULL DEFAULT 0,
  output_tokens BIGINT NOT NULL DEFAULT 0,
  finish_reason VARCHAR(64) NOT NULL DEFAULT '',
  error_code VARCHAR(64) NOT NULL DEFAULT '',
  started_at DATETIME NULL,
  completed_at DATETIME NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_ai_messages_client_id (conversation_id, client_message_id),
  UNIQUE KEY uq_ai_messages_reply (reply_to_message_id),
  KEY idx_ai_messages_conversation_created (conversation_id, created_at, id),
  CONSTRAINT fk_ai_messages_conversation FOREIGN KEY (conversation_id) REFERENCES ai_conversations(id) ON DELETE CASCADE,
  CONSTRAINT fk_ai_messages_reply FOREIGN KEY (reply_to_message_id) REFERENCES ai_messages(id) ON DELETE CASCADE,
  CONSTRAINT chk_ai_messages_role CHECK (role IN ('user','assistant')),
  CONSTRAINT chk_ai_messages_status CHECK (status IN ('completed','streaming','cancelled','failed'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Immutable user messages and progressively finalized assistant messages';

CREATE TABLE ai_message_sources (
  id BIGINT NOT NULL AUTO_INCREMENT,
  message_id CHAR(36) NOT NULL,
  source_id VARCHAR(16) NOT NULL,
  entity_type VARCHAR(32) NOT NULL,
  entity_id VARCHAR(128) NOT NULL,
  title VARCHAR(255) NOT NULL,
  internal_route VARCHAR(512) NOT NULL DEFAULT '',
  evidence_excerpt TEXT NOT NULL,
  source_version VARCHAR(128) NOT NULL DEFAULT '',
  rank_no INT NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_ai_message_sources_source (message_id, source_id),
  KEY idx_ai_message_sources_entity (entity_type, entity_id),
  CONSTRAINT fk_ai_message_sources_message FOREIGN KEY (message_id) REFERENCES ai_messages(id) ON DELETE CASCADE,
  CONSTRAINT chk_ai_message_sources_rank CHECK (rank_no > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Server-issued citation ids and permission-checked evidence snapshots';

CREATE TABLE ai_provider_calls (
  id BIGINT NOT NULL AUTO_INCREMENT,
  conversation_id CHAR(36) NULL,
  message_id CHAR(36) NULL,
  scene VARCHAR(64) NOT NULL,
  provider VARCHAR(64) NOT NULL,
  model VARCHAR(128) NOT NULL,
  status VARCHAR(24) NOT NULL,
  latency_ms BIGINT NOT NULL DEFAULT 0,
  input_tokens BIGINT NOT NULL DEFAULT 0,
  output_tokens BIGINT NOT NULL DEFAULT 0,
  estimated_cost DECIMAL(14,6) NULL,
  request_hash CHAR(64) NOT NULL,
  response_hash CHAR(64) NOT NULL DEFAULT '',
  error_code VARCHAR(64) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_ai_provider_calls_created (created_at, id),
  KEY idx_ai_provider_calls_conversation (conversation_id, id),
  CONSTRAINT fk_ai_provider_calls_conversation FOREIGN KEY (conversation_id) REFERENCES ai_conversations(id) ON DELETE SET NULL,
  CONSTRAINT fk_ai_provider_calls_message FOREIGN KEY (message_id) REFERENCES ai_messages(id) ON DELETE SET NULL,
  CONSTRAINT chk_ai_provider_calls_status CHECK (status IN ('succeeded','failed','cancelled'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Provider observability without prompts, secrets or raw responses';

CREATE TABLE ai_retrieval_documents (
  document_id CHAR(36) NOT NULL,
  entity_type VARCHAR(32) NOT NULL,
  entity_id VARCHAR(128) NOT NULL,
  title VARCHAR(255) NOT NULL,
  internal_route VARCHAR(512) NOT NULL DEFAULT '',
  search_text MEDIUMTEXT NOT NULL,
  content_hash CHAR(64) NOT NULL,
  entity_version VARCHAR(128) NOT NULL DEFAULT '',
  visibility VARCHAR(24) NOT NULL DEFAULT 'internal',
  metadata_json JSON NOT NULL,
  embedding_version VARCHAR(128) NOT NULL,
  vector_indexed_at DATETIME NULL,
  deleted_at DATETIME NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (document_id),
  UNIQUE KEY uq_ai_retrieval_documents_entity (entity_type, entity_id),
  KEY idx_ai_retrieval_documents_hash (content_hash),
  KEY idx_ai_retrieval_documents_version (embedding_version, vector_indexed_at, document_id),
  FULLTEXT KEY ft_ai_retrieval_documents_search (title, search_text),
  CONSTRAINT chk_ai_retrieval_documents_visibility CHECK (visibility IN ('internal','published'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Rebuildable compressed text projection for exact and vector retrieval';

CREATE TABLE ai_retrieval_outbox (
  id BIGINT NOT NULL AUTO_INCREMENT,
  document_id CHAR(36) NOT NULL,
  operation VARCHAR(16) NOT NULL,
  content_hash CHAR(64) NOT NULL,
  embedding_version VARCHAR(128) NOT NULL,
  dedupe_key VARCHAR(255) NOT NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'pending',
  attempt INT NOT NULL DEFAULT 0,
  next_retry_at DATETIME NULL,
  lease_token VARCHAR(64) NULL,
  lease_until DATETIME NULL,
  last_error TEXT NULL,
  alert_status VARCHAR(24) NOT NULL DEFAULT 'none',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_ai_retrieval_outbox_dedupe (dedupe_key),
  KEY idx_ai_retrieval_outbox_claim (status, next_retry_at, lease_until, id),
  KEY idx_ai_retrieval_outbox_document (document_id, id),
  CONSTRAINT chk_ai_retrieval_outbox_operation CHECK (operation IN ('upsert','delete')),
  CONSTRAINT chk_ai_retrieval_outbox_status CHECK (status IN ('pending','processing','succeeded','retry'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Durable embedding and Qdrant projection queue';

CREATE TABLE ai_access_audit (
  id BIGINT NOT NULL AUTO_INCREMENT,
  actor_user_id BIGINT NOT NULL,
  target_user_id BIGINT NULL,
  conversation_id CHAR(36) NULL,
  action VARCHAR(64) NOT NULL,
  outcome VARCHAR(24) NOT NULL,
  reason VARCHAR(512) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_ai_access_audit_actor_created (actor_user_id, created_at, id),
  KEY idx_ai_access_audit_target_created (target_user_id, created_at, id),
  CONSTRAINT fk_ai_access_audit_actor FOREIGN KEY (actor_user_id) REFERENCES users(id),
  CONSTRAINT fk_ai_access_audit_target FOREIGN KEY (target_user_id) REFERENCES users(id),
  CONSTRAINT fk_ai_access_audit_conversation FOREIGN KEY (conversation_id) REFERENCES ai_conversations(id) ON DELETE SET NULL,
  CONSTRAINT chk_ai_access_audit_outcome CHECK (outcome IN ('allowed','denied'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Metadata-only audit for cross-user AI conversation access';

-- ROLLBACK-BEGIN
DROP TABLE IF EXISTS ai_access_audit;
DROP TABLE IF EXISTS ai_retrieval_outbox;
DROP TABLE IF EXISTS ai_retrieval_documents;
DROP TABLE IF EXISTS ai_provider_calls;
DROP TABLE IF EXISTS ai_message_sources;
DROP TABLE IF EXISTS ai_messages;
DROP TABLE IF EXISTS ai_conversations;
-- ROLLBACK-END
