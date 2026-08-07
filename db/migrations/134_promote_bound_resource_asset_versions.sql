-- Promote workflow-bound source/final assets that were left staged outside the
-- global asset search projection. This only repairs the canonical version
-- pointer; it does not copy or rewrite any stored file bytes.

UPDATE design_assets da
JOIN (
  SELECT ta.asset_id, MAX(ta.id) AS task_asset_id
  FROM task_assets ta
  WHERE ta.binding_state = 'bound'
    AND ta.bound_role IN ('source', 'final')
    AND ta.deleted_at IS NULL
    AND ta.cleaned_at IS NULL
  GROUP BY ta.asset_id
) promoted
  ON promoted.asset_id = da.id
SET da.current_version_id = promoted.task_asset_id,
    da.updated_at = CURRENT_TIMESTAMP
WHERE da.current_version_id IS NULL;
