-- Migration: 038_v7_jst_user_prewire.sql
-- JST user sync pre-wiring: jst_u_id for association, jst_raw_snapshot_json for traceability.
-- Does NOT change auth/permission logic. Import is manual and controlled.

ALTER TABLE users
  ADD COLUMN jst_u_id BIGINT NULL COMMENT 'JST u_id for association; null if not from JST' AFTER updated_at,
  ADD COLUMN jst_raw_snapshot_json TEXT NULL COMMENT 'JSON snapshot of last JST import for traceability' AFTER jst_u_id;

CREATE INDEX idx_users_jst_u_id ON users (jst_u_id);
