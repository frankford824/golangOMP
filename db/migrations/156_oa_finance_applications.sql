-- Application drafts, reimbursement lines and transactionally enforced invoice claims.
CREATE TABLE IF NOT EXISTS oa_office_finance_drafts (
  owner_id BIGINT NOT NULL,
  site VARCHAR(8) NOT NULL,
  request_id VARCHAR(80) NOT NULL,
  payload MEDIUMTEXT NOT NULL,
  PRIMARY KEY (owner_id, site)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS oa_office_document_items (
  document_id CHAR(32) NOT NULL,
  line_no SMALLINT UNSIGNED NOT NULL,
  category VARCHAR(80) NOT NULL,
  occurred_on DATE NOT NULL,
  amount_cents BIGINT NOT NULL,
  tax_cents BIGINT NOT NULL,
  invoice_no VARCHAR(64) NOT NULL,
  description VARCHAR(1000) NOT NULL,
  PRIMARY KEY (document_id, line_no),
  KEY (category, occurred_on)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS oa_office_invoice_locks (
  document_id CHAR(32) NOT NULL,
  invoice_no VARCHAR(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
  state ENUM('LOCKED','USED','RELEASED') NOT NULL,
  active_invoice VARCHAR(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin
    GENERATED ALWAYS AS (CASE WHEN state IN ('LOCKED','USED') THEN invoice_no ELSE NULL END) STORED,
  PRIMARY KEY (document_id, invoice_no),
  UNIQUE KEY (active_invoice)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
