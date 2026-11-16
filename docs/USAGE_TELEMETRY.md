# Two-application usage measurement

Implementation reference; route/schema authority remains `transport/http.go` and
`docs/api/openapi.yaml`. This covers authenticated accounts only, as requested.

## Recorded signals

| Signal | Source and meaning |
| --- | --- |
| `page_view` / `page.open` | Both routers, once per successful navigation including query-only navigation. Failed/duplicate navigation and public/login pages are excluded. URLs omit all query values and fragments. |
| `page_engagement` / `page.visible` | Visible intervals, capped at 60 seconds each, flushed on navigation, hiding and page exit. This is **visible time**, not employee work time or proof of activity. |
| `user_action` / `upload.workflow` | Client upload workflow started and server-acknowledged completion, failure, cancellation or uncertain confirmation (`pending`). |
| `user_action` / `upload.transfer` | Direct-upload transfer stage. Successful transfer does not mean the server has committed a business attachment. |
| `user_action` / `download.file`, `download.batch` | File preparation/transfer attempts and results. Native browser handling is `handed_off`, never proof that a file was saved. Partial ZIP handoff carries successful/failed file counts. |
| `user_action` / `client.error` | Global unhandled errors/rejections, with error type/code only; at most one per 30 seconds. No message, stack, filename or signed URL. |
| Existing `api_request` | One server event per existing traced request, with application, session/page correlation, request kind, traffic hint, status and error code. All HTTP 4xx/5xx are failed. Search requests are included; telemetry requests never trace themselves. |
| Existing business ledgers | `task.closed`, task records, workbench submissions/files/events remain the authority for committed business quantities. Do not add frontend attempts, transfer success and business completion together. |

## Identity, correlation and limits

- `payload.telemetry_schema=usage.v1` identifies the new metadata. `app_id` is
  `main-ops` or `asset-workbench`; canonical Origin/Referer attribution wins over
  client hints. The authenticated actor comes exclusively from server context.
- The session ID is random and local to one loaded tab, reset on account change
  and after 30 minutes without observed navigation/interaction when activity
  resumes. It is not a person identifier or a cross-device/browser session.
- Page views have separate random IDs. An operation's start/end share an
  operation ID and retain their originating page. Late completion after an
  account/session change is discarded rather than attributed to the new user.
- `automation`/`traffic_class` are observed hints (webdriver, user agent, route
  and declared application), not security attestations. API polling/refresh,
  media reads, integration calls and mutation **requests** remain separate from
  human page views and confirmed business operations.
- Server receipt time in UTC owns new usage event timestamps. Client clocks
  measure durations only. Legacy business feedback retains its existing payload
  and timestamp behavior.
- Collection is best-effort: no persistent client queue or blind retry; at most
  16 sends are in flight, each bounded to 5 seconds. Request bodies are limited
  to 32 KiB and payloads to 16 KiB. `dropped_events` counts locally suppressed
  sends or delivery failures, not a proven exact count of missing database rows.
  Browser exit/network failure can lose events. Server trace writes have a
  2-second context deadline and must never change a business action's result.
- Usage payloads whitelist scalar fields. No query values, signed links,
  credentials, filenames, raw request bodies or free-form error messages are
  stored by the new usage path. Existing cost-feedback payloads are separate.

## Reporting

Use `scripts/usage-report.sql` for bounded, read-only UTC usage summaries. Compare
applications using the same time window and identity definition. Count distinct
accounts, not IP addresses; account sharing and automation using real accounts
remain limitations. Exclude explicitly identified test accounts when producing
a business report rather than guessing from usernames.

For PV, deduplicate `client_event_id` (falling back to server `event_id` for old
events). For operations, count distinct `operation_id` by action and terminal
outcome; `pending` does not establish failure or completion. Never infer exact
bytes downloaded from nominal file size or browser handoff.

Older main-site page views can be attributed from their sanitized page URL.
Historical workbench PV cannot be reconstructed or backfilled from API requests.
The new events do not alter historical records. Deployment must place backend
support before the two frontend builds.
