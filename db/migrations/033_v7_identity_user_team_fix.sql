-- Migration: 033_v7_identity_user_team_fix.sql
-- Step 66: ensure users.team column exists for org/team auth extension.

ALTER TABLE users
  ADD COLUMN team VARCHAR(64) NOT NULL AFTER department;

