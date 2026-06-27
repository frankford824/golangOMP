-- Migration: 094_asset_workbench_memberships_access_merge.sql
-- App-level asset workbench membership gate, cross-app identity audit, account merge, and payout identity snapshots.

CREATE TABLE IF NOT EXISTS app_memberships (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  app_code VARCHAR(64) NOT NULL,
  user_id BIGINT NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'pending',
  identity_type VARCHAR(32) NOT NULL DEFAULT 'staff',
  source VARCHAR(64) NOT NULL DEFAULT '',
  last_asset_roles_json JSON NULL,
  opened_by BIGINT NULL,
  disabled_by BIGINT NULL,
  disabled_reason VARCHAR(512) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_app_memberships_app_user (app_code, user_id),
  KEY idx_app_memberships_app_status (app_code, status),
  KEY idx_app_memberships_user (user_id),
  CONSTRAINT fk_app_memberships_user FOREIGN KEY (user_id) REFERENCES users(id),
  CONSTRAINT fk_app_memberships_opened_by FOREIGN KEY (opened_by) REFERENCES users(id),
  CONSTRAINT fk_app_memberships_disabled_by FOREIGN KEY (disabled_by) REFERENCES users(id),
  CONSTRAINT ck_app_memberships_status CHECK (status IN ('pending', 'active', 'disabled', 'merged')),
  CONSTRAINT ck_app_memberships_identity_type CHECK (identity_type IN ('staff', 'external', 'contractor'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS app_identity_events (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  actor_user_id BIGINT NULL,
  target_user_id BIGINT NULL,
  source_app VARCHAR(64) NOT NULL DEFAULT '',
  target_app VARCHAR(64) NOT NULL DEFAULT '',
  action VARCHAR(64) NOT NULL,
  before_json JSON NULL,
  after_json JSON NULL,
  reason VARCHAR(512) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_app_identity_events_target (target_app, target_user_id, created_at),
  KEY idx_app_identity_events_actor (actor_user_id, created_at),
  KEY idx_app_identity_events_action (action, created_at),
  CONSTRAINT fk_app_identity_events_actor FOREIGN KEY (actor_user_id) REFERENCES users(id),
  CONSTRAINT fk_app_identity_events_target FOREIGN KEY (target_user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS asset_workbench_account_links (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  source_user_id BIGINT NOT NULL,
  canonical_user_id BIGINT NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'merged',
  created_by BIGINT NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uk_aw_account_links_source (source_user_id),
  KEY idx_aw_account_links_canonical (canonical_user_id),
  CONSTRAINT fk_aw_account_links_source FOREIGN KEY (source_user_id) REFERENCES users(id),
  CONSTRAINT fk_aw_account_links_canonical FOREIGN KEY (canonical_user_id) REFERENCES users(id),
  CONSTRAINT fk_aw_account_links_created_by FOREIGN KEY (created_by) REFERENCES users(id),
  CONSTRAINT ck_aw_account_links_distinct CHECK (source_user_id <> canonical_user_id),
  CONSTRAINT ck_aw_account_links_status CHECK (status IN ('merged'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

ALTER TABLE asset_workbench_settlement_items
  ADD COLUMN paid_to_user_id BIGINT NULL AFTER payee_user_id,
  ADD COLUMN payout_snapshot_json JSON NULL AFTER snapshot_json,
  ADD KEY idx_aw_settlement_items_paid_to (paid_to_user_id, business_month),
  ADD CONSTRAINT fk_aw_settlement_items_paid_to FOREIGN KEY (paid_to_user_id) REFERENCES users(id);

UPDATE app_memberships am
JOIN (
  SELECT DISTINCT ur.user_id
  FROM user_roles ur
  WHERE ur.role IN ('AssetSubmitter', 'AssetManager', 'AssetTemplateAdmin', 'AssetSettlement', 'SuperAdmin', 'HRAdmin')
) seeded ON seeded.user_id = am.user_id
SET am.status = CASE WHEN am.status IN ('disabled', 'merged') THEN am.status ELSE 'active' END,
    am.source = CASE WHEN am.source = '' THEN 'migration_backfill' ELSE am.source END,
    am.updated_at = CURRENT_TIMESTAMP
WHERE am.app_code = 'asset_workbench';

INSERT INTO app_memberships (
  app_code, user_id, status, identity_type, source, opened_by, created_at, updated_at
)
SELECT 'asset_workbench', seeded.user_id, 'active', 'staff', 'migration_backfill', NULL, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM (
  SELECT DISTINCT ur.user_id
  FROM user_roles ur
  WHERE ur.role IN ('AssetSubmitter', 'AssetManager', 'AssetTemplateAdmin', 'AssetSettlement', 'SuperAdmin', 'HRAdmin')
) seeded
LEFT JOIN app_memberships am
  ON am.app_code = 'asset_workbench' AND am.user_id = seeded.user_id
WHERE am.id IS NULL;

UPDATE asset_workbench_settlement_items si
JOIN asset_workbench_settlement_batches sb ON sb.id = si.batch_id
SET si.paid_to_user_id = si.payee_user_id,
    si.payout_snapshot_json = JSON_OBJECT(
      'source', 'legacy_backfill',
      'payee_user_id', si.payee_user_id,
      'business_month', si.business_month,
      'backfilled_at', DATE_FORMAT(UTC_TIMESTAMP(), '%Y-%m-%dT%H:%i:%sZ')
    )
WHERE sb.status = 'confirmed'
  AND si.paid_to_user_id IS NULL;
