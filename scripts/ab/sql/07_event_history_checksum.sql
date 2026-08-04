-- Immutable legacy rows are emitted as evidence records. render_evidence.py
-- excludes evidence.* rows from violation_count but includes them in the A/B
-- canonical JSON hash, so any changed, missing, or added row fails parity.
SELECT 'evidence.task_event_log_row' AS violation_code,
       CONCAT(e.task_id, ':', e.sequence) AS entity_key,
       SHA2(CONCAT_WS(CHAR(31), e.id, e.task_id, e.sequence, e.event_type,
         COALESCE(e.operator_id, ''), CAST(e.payload AS CHAR), DATE_FORMAT(e.created_at, '%Y-%m-%dT%H:%i:%s.%f')), 256) AS detail
FROM task_event_logs e
UNION ALL
SELECT 'evidence.task_module_event_row', CONCAT(e.task_module_id, ':', e.id),
       SHA2(CONCAT_WS(CHAR(31), e.id, e.task_module_id, e.event_type,
         COALESCE(e.from_state, ''), COALESCE(e.to_state, ''), COALESCE(e.actor_id, ''),
         COALESCE(CAST(e.actor_snapshot AS CHAR), ''), CAST(e.payload AS CHAR),
         DATE_FORMAT(e.created_at, '%Y-%m-%dT%H:%i:%s.%f')), 256)
FROM task_module_events e
UNION ALL
SELECT 'event_history.trace_module_task_mismatch', CONCAT(e.id), CONCAT('trace_task=', e.task_id, ',module_task=', m.task_id)
FROM workflow_trace_events e JOIN task_modules m ON m.id = e.task_module_id
WHERE @ab_side = 'B' AND e.task_id IS NOT NULL AND e.task_id <> m.task_id
UNION ALL
SELECT 'event_history.revision_evidence_coverage_not_sql_verifiable', '*',
       'hard_blocked: revision reason metadata is not a normalized event-to-revision relation'
WHERE @ab_side = 'B' AND NOT EXISTS (
  SELECT 1 FROM ab_manifest_entities m
  WHERE m.run_id = @ab_run_id AND m.gate_name = 'G03'
    AND m.review_state = 'pass'
)
ORDER BY 1, 2, 3;
