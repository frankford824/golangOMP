-- Supplemental HR profile and immutable personnel history.
CREATE TABLE IF NOT EXISTS oa_office_person_contacts (
  user_id BIGINT PRIMARY KEY,
  payload JSON NOT NULL,
  employment_type VARCHAR(16) NOT NULL DEFAULT ''
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS oa_office_person_events (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  user_id BIGINT NOT NULL,
  actor_id BIGINT NOT NULL,
  effective_date DATE NOT NULL,
  kind VARCHAR(32) NOT NULL,
  payload JSON NOT NULL,
  recorded_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  KEY idx_oa_person_events (user_id,effective_date,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

INSERT IGNORE INTO oa_office_org_managers(org_kind,org_id,version,updated_by) VALUES('lock',1,1,0);

-- ROLLBACK-BEGIN
DELETE FROM oa_office_org_managers WHERE org_kind='lock' AND org_id=1;
DROP TABLE IF EXISTS oa_office_person_events;
DROP TABLE IF EXISTS oa_office_person_contacts;
-- ROLLBACK-END
