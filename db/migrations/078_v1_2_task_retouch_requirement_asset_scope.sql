-- Migration: 078_v1_2_task_retouch_requirement_asset_scope.sql
-- Phase 1B: nullable retouch_requirement_id scope for P图需求级 reference/source assets.

ALTER TABLE task_assets
  ADD COLUMN retouch_requirement_id BIGINT NULL
    COMMENT 'P图需求明细ID；NULL=任务级/SKU级/legacy'
    AFTER scope_sku_code,
  ADD KEY idx_task_assets_task_retouch_req (task_id, retouch_requirement_id),
  ADD CONSTRAINT fk_task_assets_retouch_requirement
    FOREIGN KEY (retouch_requirement_id) REFERENCES task_retouch_requirements (id);

ALTER TABLE design_assets
  ADD COLUMN retouch_requirement_id BIGINT NULL
    COMMENT 'P图需求明细ID；NULL=任务级/SKU级/legacy'
    AFTER scope_sku_code,
  ADD KEY idx_design_assets_task_retouch_req (task_id, retouch_requirement_id),
  ADD CONSTRAINT fk_design_assets_retouch_requirement
    FOREIGN KEY (retouch_requirement_id) REFERENCES task_retouch_requirements (id);

ALTER TABLE reference_file_refs
  ADD COLUMN retouch_requirement_id BIGINT NULL
    COMMENT 'P图需求明细ID；NULL=任务级/SKU级/legacy'
    AFTER sku_item_id,
  ADD KEY idx_reference_file_refs_task_retouch_req (task_id, retouch_requirement_id),
  ADD CONSTRAINT fk_reference_file_refs_retouch_requirement
    FOREIGN KEY (retouch_requirement_id) REFERENCES task_retouch_requirements (id);

ALTER TABLE reference_file_refs
  DROP INDEX uq_reference_file_refs_task_ref_sku,
  ADD UNIQUE KEY uq_reference_file_refs_task_ref_scope (task_id, ref_id, sku_item_id, retouch_requirement_id);

ALTER TABLE upload_requests
  ADD COLUMN retouch_requirement_id BIGINT NULL
    COMMENT 'P图需求明细ID captured at upload-session creation'
    AFTER target_sku_code,
  ADD KEY idx_upload_requests_retouch_requirement_id (retouch_requirement_id),
  ADD CONSTRAINT fk_upload_requests_retouch_requirement
    FOREIGN KEY (retouch_requirement_id) REFERENCES task_retouch_requirements (id);

-- ROLLBACK-BEGIN
ALTER TABLE upload_requests
  DROP FOREIGN KEY fk_upload_requests_retouch_requirement,
  DROP KEY idx_upload_requests_retouch_requirement_id,
  DROP COLUMN retouch_requirement_id;

ALTER TABLE reference_file_refs
  DROP INDEX uq_reference_file_refs_task_ref_scope,
  ADD UNIQUE KEY uq_reference_file_refs_task_ref_sku (task_id, ref_id, sku_item_id),
  DROP FOREIGN KEY fk_reference_file_refs_retouch_requirement,
  DROP KEY idx_reference_file_refs_task_retouch_req,
  DROP COLUMN retouch_requirement_id;

ALTER TABLE design_assets
  DROP FOREIGN KEY fk_design_assets_retouch_requirement,
  DROP KEY idx_design_assets_task_retouch_req,
  DROP COLUMN retouch_requirement_id;

ALTER TABLE task_assets
  DROP FOREIGN KEY fk_task_assets_retouch_requirement,
  DROP KEY idx_task_assets_task_retouch_req,
  DROP COLUMN retouch_requirement_id;
-- ROLLBACK-END
