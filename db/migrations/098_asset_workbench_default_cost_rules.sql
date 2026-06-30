-- Migration: 098_asset_workbench_default_cost_rules.sql
-- Seed initial asset-workbench piecework price and deduction defaults.
-- All grade/difficulty dimensions are seeded; price amounts are seeded only for confirmed J1 sample rates.

INSERT IGNORE INTO asset_workbench_price_matrix_dimensions (
  worker_type,
  job_grade,
  difficulty_class
)
SELECT
  grades.worker_type,
  grades.job_grade,
  difficulties.difficulty_class
FROM (
  SELECT 'parttime' AS worker_type, 'J1' AS job_grade
  UNION ALL SELECT 'parttime', 'J2'
  UNION ALL SELECT 'parttime', 'J3'
  UNION ALL SELECT 'fulltime', 'P1'
  UNION ALL SELECT 'fulltime', 'P2'
  UNION ALL SELECT 'fulltime', 'P3'
  UNION ALL SELECT 'fulltime', 'P4'
  UNION ALL SELECT 'fulltime', 'S1'
  UNION ALL SELECT 'fulltime', 'S2'
  UNION ALL SELECT 'fulltime', 'M1'
  UNION ALL SELECT 'fulltime', 'M2'
) AS grades
CROSS JOIN (
  SELECT 'A' AS difficulty_class
  UNION ALL SELECT 'B'
  UNION ALL SELECT 'C'
  UNION ALL SELECT 'A+小夜灯'
) AS difficulties;

INSERT INTO asset_workbench_price_matrix (
  worker_type,
  job_grade,
  difficulty_class,
  unit_price,
  effective_from,
  effective_to,
  enabled,
  revision_no,
  created_by,
  remark
)
SELECT
  seed.worker_type,
  seed.job_grade,
  seed.difficulty_class,
  seed.unit_price,
  '2026-06-01',
  NULL,
  1,
  1,
  0,
  'asset_workbench_default_seed'
FROM (
  SELECT 'parttime' AS worker_type, 'J1' AS job_grade, 'A' AS difficulty_class, 1.1400 AS unit_price
  UNION ALL SELECT 'parttime', 'J1', 'B', 0.6300
  UNION ALL SELECT 'parttime', 'J1', 'C', 0.4000
  UNION ALL SELECT 'parttime', 'J1', 'A+小夜灯', 10.0000
) AS seed
WHERE NOT EXISTS (
  SELECT 1
  FROM asset_workbench_price_matrix AS existing
  WHERE existing.worker_type = seed.worker_type
    AND existing.job_grade = seed.job_grade
    AND existing.difficulty_class = seed.difficulty_class
    AND existing.effective_from = '2026-06-01'
    AND existing.effective_to IS NULL
    AND existing.remark = 'asset_workbench_default_seed'
);

INSERT INTO asset_workbench_deduction_rules (
  worker_type,
  job_grade,
  difficulty_class,
  deduction_amount,
  effective_from,
  effective_to,
  enabled,
  revision_no,
  created_by,
  remark
)
SELECT
  seed.worker_type,
  seed.job_grade,
  seed.difficulty_class,
  seed.deduction_amount,
  '2026-06-01',
  NULL,
  1,
  1,
  0,
  'asset_workbench_default_seed'
FROM (
  SELECT 'all' AS worker_type, 'all' AS job_grade, 'A' AS difficulty_class, 10.0000 AS deduction_amount
  UNION ALL SELECT 'all', 'all', 'B', 6.0000
  UNION ALL SELECT 'all', 'all', 'C', 4.0000
) AS seed
WHERE NOT EXISTS (
  SELECT 1
  FROM asset_workbench_deduction_rules AS existing
  WHERE existing.worker_type = seed.worker_type
    AND existing.job_grade = seed.job_grade
    AND existing.difficulty_class = seed.difficulty_class
    AND existing.effective_from = '2026-06-01'
    AND existing.effective_to IS NULL
    AND existing.remark = 'asset_workbench_default_seed'
);
