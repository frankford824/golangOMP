-- Durable, per-SKU reconciliation. Historical mismatches are quarantined,
-- never overwritten during migration. Capture every canonical cost mutation.
ALTER TABLE task_sku_items MODIFY cost_price DECIMAL(14,4) NULL;
ALTER TABLE task_details MODIFY cost_price DECIMAL(14,4) NULL;
ALTER TABLE omp_sku_records MODIFY cost_price DECIMAL(14,4) NULL;
ALTER TABLE omp_sku_cost_snapshots MODIFY cost_price DECIMAL(14,4) NULL;
ALTER TABLE erp_product_sync_records MODIFY cost_price DECIMAL(14,4) NULL;
ALTER TABLE omp_sku_erp_trace_logs MODIFY request_cost_price DECIMAL(14,4) NULL, MODIFY response_cost_price DECIMAL(14,4) NULL;
ALTER TABLE cost_override_events MODIFY previous_cost_price DECIMAL(14,4) NULL, MODIFY override_cost DECIMAL(14,4) NULL, MODIFY result_cost_price DECIMAL(14,4) NULL;
ALTER TABLE cost_recalculation_run_items MODIFY old_cost_price DECIMAL(14,4) NULL, MODIFY new_cost_price DECIMAL(14,4) NULL;

CREATE TABLE sku_cost_sync_states (
 sku_code VARCHAR(64) NOT NULL PRIMARY KEY,
 local_cost DECIMAL(14,4) NULL,
 local_confirmed TINYINT(1) NOT NULL DEFAULT 0,
 erp_cost DECIMAL(14,4) NULL,
 revision BIGINT NOT NULL DEFAULT 1,
 ack_revision BIGINT NOT NULL DEFAULT 0,
 projected_revision BIGINT NOT NULL DEFAULT 0,
 erp_revision BIGINT NOT NULL DEFAULT 0,
 manual_lock TINYINT(1) NOT NULL DEFAULT 0,
 manual_origin VARCHAR(16) NOT NULL DEFAULT '',
 status VARCHAR(24) NOT NULL DEFAULT 'pending',
 needs_check TINYINT(1) NOT NULL DEFAULT 1,
 reason VARCHAR(512) NOT NULL DEFAULT '',
 attempts INT NOT NULL DEFAULT 0,
 next_check_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
 checked_at DATETIME(3) NULL,
 updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
 KEY idx_cost_sync_due(needs_check,next_check_at,sku_code),
 KEY idx_cost_sync_status(status,sku_code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE sku_cost_sync_cursor (
 name VARCHAR(32) PRIMARY KEY, event_id BIGINT NOT NULL DEFAULT 0
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE sku_cost_sync_audit (
 id BIGINT AUTO_INCREMENT PRIMARY KEY, sku_code VARCHAR(64) NOT NULL,
 action VARCHAR(32) NOT NULL, local_cost DECIMAL(14,4) NULL, erp_cost DECIMAL(14,4) NULL,
 revision BIGINT NOT NULL, actor_id BIGINT NULL, reason VARCHAR(512) NOT NULL DEFAULT '',
 created_at DATETIME(3) NOT NULL, KEY idx_cost_sync_audit_sku(sku_code,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

INSERT INTO sku_cost_sync_cursor(name,event_id) SELECT 'erp',COALESCE(MAX(id),0) FROM jst_cost_changes;

CREATE TRIGGER trg_sku_cost_sync_insert AFTER INSERT ON task_sku_items FOR EACH ROW
 INSERT INTO sku_cost_sync_states(sku_code,local_cost,local_confirmed,ack_revision,manual_lock,manual_origin,status,next_check_at,updated_at)
 VALUES(NEW.sku_code,NEW.cost_price,NEW.cost_price IS NOT NULL AND (NEW.requires_manual_review=0 OR NEW.manual_cost_override=1),IF(NEW.cost_price IS NULL,1,0),NEW.manual_cost_override,IF(NEW.manual_cost_override=0,'',IF(NEW.override_actor='erp_cost_sync','erp','local')),IF(NEW.cost_price IS NULL,'no_price','pending'),UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))
 ON DUPLICATE KEY UPDATE local_cost=VALUES(local_cost),local_confirmed=VALUES(local_confirmed),manual_lock=VALUES(manual_lock),manual_origin=VALUES(manual_origin),revision=revision+1,needs_check=1,next_check_at=UTC_TIMESTAMP(3),updated_at=UTC_TIMESTAMP(3);

CREATE TRIGGER trg_sku_cost_sync_update AFTER UPDATE ON task_sku_items FOR EACH ROW
 UPDATE sku_cost_sync_states SET local_cost=NEW.cost_price,manual_lock=NEW.manual_cost_override,
 local_confirmed=NEW.cost_price IS NOT NULL AND (NEW.requires_manual_review=0 OR NEW.manual_cost_override=1),
 manual_origin=IF(NEW.manual_cost_override=0,'',IF(NEW.override_actor='erp_cost_sync','erp','local')),
 revision=revision+1,needs_check=1,attempts=0,next_check_at=UTC_TIMESTAMP(3),updated_at=UTC_TIMESTAMP(3),
 status=IF(status='conflict','conflict',IF(NEW.cost_price IS NULL,'no_price','pending'))
 WHERE sku_code=NEW.sku_code AND (NOT(OLD.cost_price <=> NEW.cost_price) OR OLD.manual_cost_override<>NEW.manual_cost_override OR OLD.requires_manual_review<>NEW.requires_manual_review OR (NEW.manual_cost_override=1 AND NOT(OLD.override_actor <=> NEW.override_actor)));

-- Triggers precede bootstrap so concurrent task creation cannot fall through a
-- seed/trigger gap. Existing/newly captured rows are never overwritten.
INSERT IGNORE INTO sku_cost_sync_states(sku_code,local_cost,local_confirmed,erp_cost,ack_revision,manual_lock,manual_origin,status,needs_check,reason,next_check_at,updated_at)
 SELECT s.sku_code,s.cost_price,s.cost_price IS NOT NULL AND (s.requires_manual_review=0 OR s.manual_cost_override=1),j.cost_price,
 IF(s.cost_price <=> j.cost_price,1,0),s.manual_cost_override,IF(s.manual_cost_override=0,'',IF(s.override_actor='erp_cost_sync','erp','local')),
 CASE WHEN s.cost_price IS NOT NULL AND s.requires_manual_review=1 AND s.manual_cost_override=0 THEN 'conflict'
      WHEN s.cost_price<0 OR j.cost_price<0 OR s.cost_price>1000000000 OR j.cost_price>1000000000 THEN 'conflict'
      WHEN s.cost_price IS NULL AND j.cost_price IS NULL THEN 'no_price'
      WHEN s.cost_price <=> j.cost_price THEN 'baseline' ELSE 'conflict' END,
 IF(s.cost_price IS NOT NULL AND s.cost_price <=> j.cost_price,1,0),IF(s.cost_price <=> j.cost_price,'副本一致，等待首次实时核对','历史成本不一致，首次启用需人工确认'),UTC_TIMESTAMP(3),UTC_TIMESTAMP(3)
 FROM task_sku_items s LEFT JOIN jst_inventory j ON j.sku_id=s.sku_code WHERE s.sku_code<>'';
