CREATE TABLE IF NOT EXISTS oa_office_recipient_accounts (
 id CHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
 scope ENUM('personal','supplier') NOT NULL,
 owner_id BIGINT NOT NULL,
 holder VARCHAR(160) NOT NULL,
 bank VARCHAR(160) NOT NULL,
 last_four CHAR(4) CHARACTER SET ascii NOT NULL,
 account_cipher VARBINARY(512) NOT NULL,
 version BIGINT UNSIGNED NOT NULL DEFAULT 1,
 active BOOLEAN NOT NULL DEFAULT TRUE,
 updated_by BIGINT NOT NULL,
 updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
 personal_default BIGINT GENERATED ALWAYS AS (CASE WHEN scope='personal' AND active=1 THEN owner_id ELSE NULL END) STORED,
 UNIQUE KEY uq_recipient_personal (personal_default),
 KEY ix_recipient_directory(scope,owner_id,active,holder)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS oa_office_recipient_access (
 id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
 recipient_id CHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 actor_id BIGINT NOT NULL,
 action VARCHAR(16) CHARACTER SET ascii NOT NULL,
 created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
 KEY ix_recipient_events(recipient_id,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
