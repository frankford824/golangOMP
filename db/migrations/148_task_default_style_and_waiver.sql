ALTER TABLE task_asset_group_revisions
  ADD COLUMN final_output_waived BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE task_assets
  DROP CHECK chk_task_assets_bound_role,
  DROP CHECK chk_task_assets_staged_role,
  ADD CONSTRAINT chk_task_assets_bound_resource_role CHECK (bound_role IS NULL OR bound_role IN ('source','final','default_style')),
  ADD CONSTRAINT chk_task_assets_staged_resource_role CHECK (staged_role IS NULL OR staged_role IN ('source','final','default_style'));

CREATE TABLE task_default_style_revisions (
  id BIGINT NOT NULL AUTO_INCREMENT,
  group_id BIGINT NOT NULL,
  task_asset_id BIGINT NULL,
  created_by BIGINT NOT NULL,
  reason VARCHAR(512) NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_task_default_style_group_revision (group_id, id),
  UNIQUE KEY uq_task_default_style_asset (task_asset_id),
  CONSTRAINT fk_task_default_style_group FOREIGN KEY (group_id) REFERENCES task_asset_groups(id),
  CONSTRAINT fk_task_default_style_asset FOREIGN KEY (task_asset_id) REFERENCES task_assets(id),
  CONSTRAINT fk_task_default_style_actor FOREIGN KEY (created_by) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

ALTER TABLE task_asset_groups
  ADD COLUMN current_default_style_id BIGINT NULL,
  ADD COLUMN pending_default_style_id BIGINT NULL,
  ADD CONSTRAINT fk_task_current_default_style FOREIGN KEY (id, current_default_style_id)
    REFERENCES task_default_style_revisions(group_id, id),
  ADD CONSTRAINT fk_task_pending_default_style FOREIGN KEY (id, pending_default_style_id)
    REFERENCES task_default_style_revisions(group_id, id);
