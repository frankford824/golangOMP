-- Read-only, UTC, rolling seven-day report. Edit these two bounds explicitly
-- for a different period; use daily slices if the query deadline is reached.
SET SESSION MAX_EXECUTION_TIME = 5000;
SET @usage_to = UTC_TIMESTAMP();
SET @usage_from = @usage_to - INTERVAL 7 DAY;
START TRANSACTION READ ONLY;

SELECT
  CASE
    WHEN payload_json->>'$.telemetry_schema' = 'usage.v1' THEN payload_json->>'$.app_id'
    WHEN page_url LIKE 'https://assets.yongbo.cloud/%' THEN 'asset-workbench'
    WHEN page_url LIKE 'https://yongbo.cloud/%' OR page_url LIKE 'https://www.yongbo.cloud/%' THEN 'main-ops'
    ELSE 'unknown'
  END AS app_id,
  DATE(occurred_at) AS day_utc,
  COUNT(DISTINCT COALESCE(NULLIF(payload_json->>'$.client_event_id', ''), event_id)) AS recorded_page_views,
  COUNT(DISTINCT actor_id) AS page_view_accounts
FROM workflow_trace_events FORCE INDEX (idx_workflow_trace_type_time)
WHERE event_type = 'page_view' AND event_source = 'frontend'
  AND actor_source = 'session_token' AND actor_id > 0
  AND occurred_at >= @usage_from AND occurred_at < @usage_to
  AND COALESCE(payload_json->>'$.automation', 'false') <> 'true'
GROUP BY app_id, day_utc ORDER BY day_utc, app_id;

SELECT payload_json->>'$.app_id' AS app_id, action, outcome,
  COUNT(DISTINCT NULLIF(payload_json->>'$.operation_id', '')) AS observed_operations
FROM workflow_trace_events FORCE INDEX (idx_workflow_trace_type_time)
WHERE event_type = 'user_action' AND event_source = 'frontend'
  AND actor_source = 'session_token' AND actor_id > 0
  AND occurred_at >= @usage_from AND occurred_at < @usage_to
  AND payload_json->>'$.telemetry_schema' = 'usage.v1'
  AND payload_json->>'$.phase' = 'finished'
  AND COALESCE(payload_json->>'$.automation', 'false') <> 'true'
GROUP BY app_id, action, outcome ORDER BY app_id, action, outcome;

SELECT app_id, SUM(visible_ms) AS recorded_visible_ms
FROM (
  SELECT payload_json->>'$.app_id' AS app_id,
    COALESCE(NULLIF(payload_json->>'$.client_event_id', ''), event_id) AS segment_id,
    MAX(LEAST(CAST(payload_json->>'$.visible_ms' AS UNSIGNED), 60000)) AS visible_ms
  FROM workflow_trace_events FORCE INDEX (idx_workflow_trace_type_time)
  WHERE event_type = 'page_engagement' AND event_source = 'frontend'
    AND actor_source = 'session_token' AND actor_id > 0
    AND occurred_at >= @usage_from AND occurred_at < @usage_to
    AND payload_json->>'$.telemetry_schema' = 'usage.v1'
    AND COALESCE(payload_json->>'$.automation', 'false') <> 'true'
  GROUP BY app_id, segment_id
) visible_segments
GROUP BY app_id ORDER BY app_id;

ROLLBACK;
