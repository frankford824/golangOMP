-- New OA extensions for the existing workflow installation.

CREATE TABLE IF NOT EXISTS oa_office_templates (id VARCHAR(64) PRIMARY KEY, version INT NOT NULL, draft JSON, published JSON) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS oa_office_template_versions (id VARCHAR(64) NOT NULL, version INT NOT NULL, publisher BIGINT NOT NULL, published_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, payload JSON NOT NULL, PRIMARY KEY(id,version)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS oa_office_delegates (user_id BIGINT PRIMARY KEY, level VARCHAR(16) NOT NULL) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS oa_office_finance_actions (user_id BIGINT PRIMARY KEY, can_pay BOOLEAN NOT NULL, can_report BOOLEAN NOT NULL) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS oa_office_documents (id CHAR(32) PRIMARY KEY, applicant BIGINT NOT NULL, request_id VARCHAR(80) NOT NULL, state VARCHAR(16) NOT NULL, approver BIGINT NULL, paid_date DATE NULL, payload JSON NOT NULL, UNIQUE KEY (applicant,request_id), KEY (state,approver), KEY (paid_date)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS oa_office_files (id CHAR(32) PRIMARY KEY, owner_id BIGINT NOT NULL, name VARCHAR(255) NOT NULL, mime VARCHAR(64) NOT NULL, document_id CHAR(32) NULL) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS oa_office_people (id BIGINT PRIMARY KEY, version INT NOT NULL, payload JSON NOT NULL) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS oa_office_notifications (id BIGINT AUTO_INCREMENT PRIMARY KEY,user_id BIGINT NOT NULL,payload JSON NOT NULL,read_at TIMESTAMP NULL,created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,KEY(user_id,id)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS oa_office_imports (token CHAR(32) PRIMARY KEY, actor BIGINT NOT NULL, payload JSON NOT NULL, confirmed_count INT NULL, expires_at TIMESTAMP NOT NULL) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS oa_order_feedback (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  actor_id BIGINT NOT NULL,
  shop_id VARCHAR(128) NOT NULL,
  sku VARCHAR(128) NOT NULL,
  feedback_note TEXT NOT NULL,
  adjusted_on DATE NOT NULL,
  task_id BIGINT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_oa_order_feedback_sku (sku,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS oa_order_task_links (
  shop_id VARBINARY(128) NOT NULL,
  sku VARBINARY(128) NOT NULL,
  task_id BIGINT NOT NULL,
  actor_id BIGINT NOT NULL,
  period_from DATE NOT NULL,
  period_to DATE NOT NULL,
  metrics JSON NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY(shop_id,sku,task_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- ROLLBACK-BEGIN
-- Preserve extension data before dropping tables.
DROP TABLE IF EXISTS oa_order_task_links;
DROP TABLE IF EXISTS oa_order_feedback;
DROP TABLE IF EXISTS oa_office_templates;
DROP TABLE IF EXISTS oa_office_template_versions;
DROP TABLE IF EXISTS oa_office_delegates;
DROP TABLE IF EXISTS oa_office_finance_actions;
DROP TABLE IF EXISTS oa_office_documents;
DROP TABLE IF EXISTS oa_office_files;
DROP TABLE IF EXISTS oa_office_people;
DROP TABLE IF EXISTS oa_office_notifications;
DROP TABLE IF EXISTS oa_office_imports;
-- ROLLBACK-END
