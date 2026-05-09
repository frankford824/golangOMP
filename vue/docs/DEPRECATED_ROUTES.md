# Deprecated Frontend Blacklist

This project follows the canonical backend contract. New frontend code must not rely on compatibility-only routes or alias fields below.

## Route Blacklist

- `POST /v1/task-create/asset-center/upload-sessions*`
- `/v1/tasks/{id}/asset-center/*`
- `POST /v1/tasks/{id}/assets/upload`
- `/v1/assets/files/{path}`

## Field/Behavior Blacklist

- `reference_images` in task creation payload
- Using `upload_mode` as upload strategy selector
- Treating `owner_team` as the only ownership field
- Building new logic on user aliases: `account`, `name`, `group`, `phone`
- Building new logic on access aliases: `menu_keys`, `page_keys`, `access_scopes`, `permission_flags`, `module_keys`
- Any NAS direct-connect assumptions or local path derivation in upload/download flows

## Canonical Replacements

- Task create refs: `POST /v1/tasks/reference-upload` then `reference_file_refs`
- Asset upload: `POST /v1/assets/upload-sessions` + complete/cancel
- Asset access: `download_url` + `download_mode`
- Ownership: `owner_department` + `owner_org_team` (`owner_team` only for compatibility)
