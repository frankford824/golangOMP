-- OA personnel profiles and scoped organization manager defaults.
CREATE TABLE IF NOT EXISTS oa_office_person_profiles (
  user_id BIGINT PRIMARY KEY,
  version BIGINT NOT NULL,
  manager_id BIGINT NOT NULL DEFAULT 0,
  joined DATE NULL,
  job_title VARCHAR(100) NOT NULL DEFAULT '',
  job_level VARCHAR(100) NOT NULL DEFAULT '',
  cost_center VARCHAR(100) NOT NULL DEFAULT '',
  updated_by BIGINT NOT NULL,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS oa_office_org_managers (
  org_kind ENUM('lock','department','team') NOT NULL,
  org_id BIGINT NOT NULL,
  manager_id BIGINT NOT NULL DEFAULT 0,
  version BIGINT NOT NULL,
  updated_by BIGINT NOT NULL,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (org_kind,org_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
INSERT IGNORE INTO oa_office_org_managers(org_kind,org_id,version,updated_by) VALUES('lock',0,1,0);

-- ROLLBACK-BEGIN
DROP TABLE IF EXISTS oa_office_org_managers;
DROP TABLE IF EXISTS oa_office_person_profiles;

-- ROLLBACK-END
