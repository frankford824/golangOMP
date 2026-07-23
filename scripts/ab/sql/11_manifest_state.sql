-- B-only exact comparison between approved manifest rows and canonical database
-- observations. A executes this query in the same adapter/session but emits no
-- rows because it is the immutable legacy baseline, not the migration target.
WITH observed AS (
  SELECT CAST('G01' AS BINARY) AS gate_name, CAST(CONCAT('task:', t.id) AS BINARY) AS entity_key,
         CAST(SHA2(CONCAT_WS(CHAR(31), t.id, t.task_type, t.task_status,
           COALESCE(t.current_handler_id, ''), t.workflow_revision), 256) AS BINARY) AS actual_hash
  FROM tasks t
  UNION ALL
  SELECT CAST('G02' AS BINARY), CAST(CONCAT('group:', g.task_id, ':', g.scope_kind, ':', g.scope_ref_id) AS BINARY),
         CAST(SHA2(CONCAT_WS(CHAR(31), g.task_id, g.scope_kind, g.scope_ref_id,
           COALESCE(w.revision_no, ''), COALESCE(w.status, ''),
           COALESCE(f.revision_no, ''), COALESCE(f.status, ''),
           g.migration_incomplete, g.migration_issue), 256) AS BINARY)
  FROM task_asset_groups g
  LEFT JOIN task_asset_group_revisions w ON w.group_id = g.id AND w.id = g.working_revision_id
  LEFT JOIN task_asset_group_revisions f ON f.group_id = g.id AND f.id = g.finalized_revision_id
  UNION ALL
  SELECT CAST('G03' AS BINARY), CAST(CONCAT('revision:', g.task_id, ':', g.scope_kind, ':', g.scope_ref_id, ':', r.revision_no) AS BINARY),
         CAST(SHA2(CONCAT_WS(CHAR(31), g.task_id, g.scope_kind, g.scope_ref_id, r.revision_no,
           r.status, r.mode, COALESCE(r.source_task_asset_id, ''), r.source_stage,
           r.created_by, r.reason, COALESCE(DATE_FORMAT(r.submitted_at, '%Y-%m-%dT%H:%i:%s.%f'), ''),
           COALESCE(DATE_FORMAT(r.finalized_at, '%Y-%m-%dT%H:%i:%s.%f'), '')), 256) AS BINARY)
  FROM task_asset_group_revisions r JOIN task_asset_groups g ON g.id = r.group_id
  UNION ALL
  SELECT CAST('G04' AS BINARY), CAST(CONCAT('revision-source:', g.task_id, ':', g.scope_kind, ':', g.scope_ref_id, ':', r.revision_no) AS BINARY),
         CAST(SHA2(CONCAT_WS(CHAR(31), COALESCE(a.id, ''), COALESCE(a.asset_type, ''),
           COALESCE(a.whole_hash, ''), COALESCE(a.binding_state, ''),
           COALESCE(a.bound_role, ''), COALESCE(a.scope_sku_code, ''),
           COALESCE(a.retouch_requirement_id, '')), 256) AS BINARY)
  FROM task_asset_group_revisions r JOIN task_asset_groups g ON g.id = r.group_id
  LEFT JOIN task_assets a ON a.id = r.source_task_asset_id
  UNION ALL
  SELECT CAST('G04' AS BINARY), CAST(CONCAT('revision-final:', g.task_id, ':', g.scope_kind, ':', g.scope_ref_id, ':', r.revision_no, ':', i.sort_order) AS BINARY),
         CAST(SHA2(CONCAT_WS(CHAR(31), i.task_asset_id, i.sort_order, i.item_name,
           a.asset_type, COALESCE(a.whole_hash, ''), a.binding_state,
           COALESCE(a.bound_role, ''), COALESCE(a.scope_sku_code, ''),
           COALESCE(a.retouch_requirement_id, '')), 256) AS BINARY)
  FROM task_asset_group_revision_items i JOIN task_asset_group_revisions r ON r.id = i.revision_id
  JOIN task_asset_groups g ON g.id = r.group_id JOIN task_assets a ON a.id = i.task_asset_id
  UNION ALL
  SELECT CAST('G05' AS BINARY), CAST(CONCAT('revision-reference:', g.task_id, ':', g.scope_kind, ':', g.scope_ref_id, ':', r.revision_no, ':', rr.sort_order) AS BINARY),
         CAST(SHA2(CONCAT_WS(CHAR(31), rr.reference_file_ref_id,
           COALESCE(fa.storage_ref_id, ''), rr.sort_order,
           rr.ref_id_snapshot, rr.file_name_snapshot, rr.scope_snapshot), 256) AS BINARY)
  FROM task_asset_group_revision_references rr JOIN task_asset_group_revisions r ON r.id = rr.revision_id
  JOIN task_asset_groups g ON g.id = r.group_id
  LEFT JOIN task_assets fa ON fa.id = rr.formal_task_asset_id
  UNION ALL
  SELECT CAST('G07' AS BINARY), CAST(CONCAT('task-event:', e.task_id, ':', e.sequence) AS BINARY),
         CAST(SHA2(CONCAT_WS(CHAR(31), e.id, e.task_id, e.sequence, e.event_type,
           COALESCE(e.operator_id, ''), CAST(e.payload AS CHAR),
           DATE_FORMAT(e.created_at, '%Y-%m-%dT%H:%i:%s.%f')), 256) AS BINARY)
  FROM task_event_logs e
  UNION ALL
  SELECT CAST('G07' AS BINARY), CAST(CONCAT('module-event:', e.task_module_id, ':', e.id) AS BINARY),
         CAST(SHA2(CONCAT_WS(CHAR(31), e.id, e.task_module_id, e.event_type,
           COALESCE(e.from_state, ''), COALESCE(e.to_state, ''),
           COALESCE(e.actor_id, ''), COALESCE(CAST(e.actor_snapshot AS CHAR), ''),
           CAST(e.payload AS CHAR), DATE_FORMAT(e.created_at, '%Y-%m-%dT%H:%i:%s.%f')), 256) AS BINARY)
  FROM task_module_events e
  UNION ALL
  SELECT CAST('G08' AS BINARY), CAST(CONCAT('planning-revision:', r.task_sku_item_id, ':', r.version_no) AS BINARY),
         CAST(SHA2(CONCAT_WS(CHAR(31), r.task_sku_item_id, r.version_no,
           r.description_spec, r.quantity, COALESCE(r.target_price, ''), r.currency,
           r.note, r.reference_url, r.erp_product_i_id, r.erp_product_name,
           r.reason, r.created_by, COALESCE(i.storage_ref_id, '')), 256) AS BINARY)
  FROM task_planning_sku_revisions r LEFT JOIN task_planning_sku_revision_images i ON i.revision_id = r.id
  UNION ALL
  SELECT CAST('G08' AS BINARY), CAST(CONCAT('retouch-requirement:', q.task_id, ':', q.id) AS BINARY),
         CAST(SHA2(CONCAT_WS(CHAR(31), q.task_id, q.id, q.description,
           COALESCE(q.sku_code, ''), COALESCE(q.spec, ''), COALESCE(q.remark, ''),
           q.sort_order, IF(q.deleted_at IS NULL, 0, 1)), 256) AS BINARY)
  FROM task_retouch_requirements q
  UNION ALL
  SELECT CAST('G09' AS BINARY), CAST(CONCAT('task-search:', d.task_id) AS BINARY),
         CAST(SHA2(CONCAT_WS(CHAR(31), d.task_id, d.task_type, d.task_status,
           COALESCE(d.current_handler_id, ''), SHA2(d.search_text, 256)), 256) AS BINARY)
  FROM task_search_documents d
  UNION ALL
  SELECT CAST('G09' AS BINARY), CAST(CONCAT('group-search:', d.task_id, ':', g.scope_kind, ':', g.scope_ref_id) AS BINARY),
         CAST(SHA2(CONCAT_WS(CHAR(31), d.task_id, g.scope_kind, g.scope_ref_id,
           COALESCE(r.revision_no, ''), SHA2(d.internal_text, 256),
           SHA2(d.final_text, 256)), 256) AS BINARY)
  FROM task_asset_group_search_documents d
  JOIN task_asset_groups g ON g.id = d.group_id
  LEFT JOIN task_asset_group_revisions r ON r.group_id = g.id AND r.id = d.finalized_revision_id
  UNION ALL
  SELECT CAST('G09' AS BINARY), CAST(CONCAT('client-pin:', c.id) AS BINARY),
         CAST(SHA2(CONCAT_WS(CHAR(31), c.id, c.source_type, c.source_ref,
           COALESCE(c.asset_id, ''), c.enabled, COALESCE(g.task_id, ''),
           COALESCE(g.scope_kind, ''), COALESCE(g.scope_ref_id, ''),
           COALESCE(r.revision_no, ''), COALESCE(item.sort_order, '')), 256) AS BINARY)
  FROM asset_workbench_client_materials c
  LEFT JOIN task_asset_groups g ON g.id = c.resource_group_id
  LEFT JOIN task_asset_group_revisions r ON r.group_id = g.id AND r.id = c.finalized_revision_id
  LEFT JOIN task_asset_group_revision_items item ON item.revision_id = r.id AND item.id = c.cover_revision_item_id
), combined AS (
  -- The TEMPORARY table is read exactly once. MySQL cannot reopen one TEMP
  -- table multiple times in a UNION statement, even through mergeable CTEs.
  SELECT m.gate_name, m.entity_key, m.expected_hash, NULL AS actual_hash,
         m.expected_state, m.review_state, 1 AS manifest_count, 0 AS observed_count
  FROM ab_manifest_entities m
  WHERE m.run_id = @ab_run_id
  UNION ALL
  SELECT o.gate_name, o.entity_key, NULL, o.actual_hash, NULL, NULL, 0, 1
  FROM observed o
), compared AS (
  SELECT gate_name, entity_key,
         MAX(expected_hash) AS expected_hash,
         MAX(actual_hash) AS actual_hash,
         MAX(expected_state) AS expected_state,
         MAX(review_state) AS review_state,
         SUM(manifest_count) AS manifest_count,
         SUM(observed_count) AS observed_count
  FROM combined
  GROUP BY gate_name, entity_key
)
SELECT
  CASE
    WHEN c.manifest_count > 0 AND c.review_state <> 'pass' THEN 'manifest.review_not_pass'
    WHEN c.observed_count > 1 THEN 'manifest.duplicate_observed_entity'
    WHEN c.gate_name IN ('G01','G02','G03','G04','G05','G07','G08','G09') AND c.manifest_count = 0 THEN 'manifest.unreviewed_entity'
    WHEN c.gate_name IN ('G01','G02','G03','G04','G05','G07','G08','G09') AND c.observed_count = 0 THEN 'manifest.expected_entity_missing'
    WHEN c.expected_hash <> c.actual_hash THEN 'manifest.entity_hash_mismatch'
    ELSE CONCAT('evidence.manifest_state.', c.gate_name)
  END AS violation_code,
  CASE
    WHEN c.manifest_count > 0 AND c.review_state <> 'pass' THEN CONCAT(c.gate_name, ':', c.entity_key)
    WHEN c.observed_count > 1 THEN CONCAT(c.gate_name, ':', c.entity_key)
    WHEN c.gate_name IN ('G01','G02','G03','G04','G05','G07','G08','G09') AND (c.manifest_count = 0 OR c.observed_count = 0 OR c.expected_hash <> c.actual_hash)
      THEN CONCAT(c.gate_name, ':', c.entity_key)
    ELSE c.entity_key
  END AS entity_key,
  CASE
    WHEN c.manifest_count > 0 AND c.review_state <> 'pass' THEN CONCAT('review_state=', c.review_state)
    WHEN c.observed_count > 1 THEN CONCAT('count=', c.observed_count)
    WHEN c.manifest_count = 0 THEN CONCAT('present in clone B but absent from approved manifest; actual=', c.actual_hash)
    WHEN c.observed_count = 0 THEN 'missing from clone B'
    WHEN c.expected_hash <> c.actual_hash THEN CONCAT('expected=', c.expected_hash, ',actual=', c.actual_hash)
    ELSE c.actual_hash
  END AS detail
FROM compared c
WHERE @ab_side = 'B' AND (
  (c.manifest_count > 0 AND c.review_state <> 'pass')
  OR c.observed_count > 1
  OR (
    c.gate_name IN ('G01','G02','G03','G04','G05','G07','G08','G09')
    AND (c.manifest_count = 0 OR c.observed_count = 0 OR c.expected_hash <> c.actual_hash OR c.actual_hash IS NOT NULL)
  )
)
ORDER BY 1, 2, 3;
