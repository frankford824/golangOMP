-- Migration: 016_v7_export_job_events.sql
-- Adds durable lifecycle audit trace for export jobs.

CREATE TABLE export_job_events (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  event_id VARCHAR(64) NOT NULL,
  export_job_id BIGINT NOT NULL,
  sequence BIGINT NOT NULL,
  event_type VARCHAR(128) NOT NULL,
  from_status VARCHAR(32) NULL,
  to_status VARCHAR(32) NULL,
  actor_id BIGINT NOT NULL,
  actor_type VARCHAR(64) NOT NULL DEFAULT '',
  note VARCHAR(255) NOT NULL DEFAULT '',
  payload JSON NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uq_export_job_events_event_id (event_id),
  UNIQUE KEY uq_export_job_events_seq (export_job_id, sequence),
  KEY idx_export_job_events_job_created_at (export_job_id, created_at),
  KEY idx_export_job_events_job_event_type (export_job_id, event_type),
  CONSTRAINT fk_export_job_events_job FOREIGN KEY (export_job_id) REFERENCES export_jobs (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Export job lifecycle audit trace';

CREATE TABLE export_job_event_sequences (
  export_job_id BIGINT NOT NULL PRIMARY KEY,
  last_sequence BIGINT NOT NULL,
  CONSTRAINT fk_export_job_event_sequences_job FOREIGN KEY (export_job_id) REFERENCES export_jobs (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Per-export-job event sequence counter';
