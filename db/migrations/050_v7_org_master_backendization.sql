-- Migration: 050_v7_org_master_backendization.sql
-- Step 118: backendized organization master data for departments and teams.

CREATE TABLE IF NOT EXISTS org_departments (
  id         BIGINT       NOT NULL AUTO_INCREMENT,
  name       VARCHAR(128) NOT NULL,
  enabled    TINYINT(1)   NOT NULL DEFAULT 1,
  created_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_org_departments_name (name),
  KEY idx_org_departments_enabled (enabled)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Organization department master data';

CREATE TABLE IF NOT EXISTS org_teams (
  id            BIGINT       NOT NULL AUTO_INCREMENT,
  department_id BIGINT       NOT NULL,
  name          VARCHAR(128) NOT NULL,
  enabled       TINYINT(1)   NOT NULL DEFAULT 1,
  created_at    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_org_teams_name (name),
  UNIQUE KEY uq_org_teams_department_name (department_id, name),
  KEY idx_org_teams_department_id (department_id),
  KEY idx_org_teams_enabled (enabled),
  CONSTRAINT fk_org_teams_department FOREIGN KEY (department_id) REFERENCES org_departments (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Organization team master data';
