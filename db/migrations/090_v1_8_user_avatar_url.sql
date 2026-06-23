-- Migration: 090_v1_8_user_avatar_url.sql
-- Persist current-user avatar URLs for the profile center.

ALTER TABLE users
  ADD COLUMN avatar_url varchar(512) NULL AFTER email;

-- ROLLBACK-BEGIN
ALTER TABLE users
  DROP COLUMN avatar_url;
-- ROLLBACK-END
