-- OA finance document events are written in the document state transaction.
-- Grant the OA runtime account SELECT and INSERT on this table.
CREATE TABLE IF NOT EXISTS oa_office_document_events (
    document_id CHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    document_version INT UNSIGNED NOT NULL,
    operator_id BIGINT NOT NULL,
    action VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    occurred_at DATETIME(6) NOT NULL,
    payload JSON NOT NULL,
    PRIMARY KEY (document_id, document_version),
    KEY idx_oa_doc_events_operator (operator_id, action, occurred_at, document_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
