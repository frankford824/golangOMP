-- Persist editable report weights and the optional payroll identifier.
SET @aw_add_coefficient = NOT EXISTS (
  SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'asset_workbench_difficulty_classes' AND COLUMN_NAME = 'difficulty_coefficient'
);
SET @aw_ddl = IF(@aw_add_coefficient,
  'ALTER TABLE asset_workbench_difficulty_classes ADD COLUMN difficulty_coefficient DECIMAL(10,4) NULL',
  'SELECT 1');
PREPARE aw_stmt FROM @aw_ddl;
EXECUTE aw_stmt;
DEALLOCATE PREPARE aw_stmt;

UPDATE asset_workbench_difficulty_classes
SET difficulty_coefficient = CASE
  WHEN code IN ('A', 'D', 'A-视频', 'A+视频') THEN 1.5000
  WHEN code = 'B' THEN 0.8000
  WHEN code = 'C' THEN 0.5000
  WHEN code IN ('A+小夜灯', '小夜灯') THEN 3.5000
  ELSE 1.0000
END
WHERE difficulty_coefficient IS NULL;

SET @aw_add_personnel_no = NOT EXISTS (
  SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'asset_workbench_profiles' AND COLUMN_NAME = 'personnel_no'
);
SET @aw_ddl = IF(@aw_add_personnel_no,
  'ALTER TABLE asset_workbench_profiles ADD COLUMN personnel_no VARCHAR(32) NULL, ADD UNIQUE KEY uk_aw_profile_personnel_no (personnel_no)',
  'SELECT 1');
PREPARE aw_stmt FROM @aw_ddl;
EXECUTE aw_stmt;
DEALLOCATE PREPARE aw_stmt;

-- ROLLBACK-BEGIN
ALTER TABLE asset_workbench_profiles DROP INDEX uk_aw_profile_personnel_no, DROP COLUMN personnel_no;
ALTER TABLE asset_workbench_difficulty_classes DROP COLUMN difficulty_coefficient;
