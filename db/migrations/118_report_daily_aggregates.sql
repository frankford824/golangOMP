-- Daily report aggregates for L1 throughput.

CREATE TABLE IF NOT EXISTS report_task_daily (
  day DATE NOT NULL,
  owner_department VARCHAR(255) NOT NULL DEFAULT '',
  task_type VARCHAR(64) NOT NULL DEFAULT '',
  created_count BIGINT NOT NULL DEFAULT 0,
  completed_count BIGINT NOT NULL DEFAULT 0,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (day, owner_department, task_type),
  KEY idx_report_task_daily_department_day (owner_department, day),
  KEY idx_report_task_daily_type_day (task_type, day)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Pre-aggregated daily L1 throughput metrics';

INSERT INTO report_task_daily (day, owner_department, task_type, created_count, completed_count)
SELECT day, owner_department, task_type, SUM(created_count) AS created_count, SUM(completed_count) AS completed_count
  FROM (
        SELECT DATE(t.created_at) AS day,
               COALESCE(t.owner_department, '') AS owner_department,
               COALESCE(t.task_type, '') AS task_type,
               COUNT(*) AS created_count,
               0 AS completed_count
          FROM tasks t
         WHERE t.created_at IS NOT NULL
         GROUP BY DATE(t.created_at), COALESCE(t.owner_department, ''), COALESCE(t.task_type, '')
        UNION ALL
        SELECT DATE(tel.created_at) AS day,
               COALESCE(t.owner_department, '') AS owner_department,
               COALESCE(t.task_type, '') AS task_type,
               0 AS created_count,
               COUNT(DISTINCT tel.task_id) AS completed_count
          FROM task_event_logs tel
          JOIN tasks t ON t.id = tel.task_id
         WHERE tel.event_type IN (
               'task.audit.approved',
               'task.customization.reviewed',
               'task.warehouse.completed',
               'task.closed'
         )
           AND tel.created_at IS NOT NULL
         GROUP BY DATE(tel.created_at), COALESCE(t.owner_department, ''), COALESCE(t.task_type, '')
       ) daily
 GROUP BY day, owner_department, task_type
ON DUPLICATE KEY UPDATE
  created_count = VALUES(created_count),
  completed_count = VALUES(completed_count);

-- ROLLBACK-BEGIN
DROP TABLE IF EXISTS report_task_daily;
-- ROLLBACK-END
