-- Display permission only; never changes piecework, payout or settlement facts.
ALTER TABLE asset_workbench_profiles
  ADD COLUMN settlement_visible TINYINT(1) NOT NULL DEFAULT 1
  COMMENT 'Allow this person to view their client settlement page';
