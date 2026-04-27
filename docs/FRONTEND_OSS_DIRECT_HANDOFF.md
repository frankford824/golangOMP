# Frontend OSS Direct Handoff

Generated: 2026-04-13

## Summary

Backend now supports **true browser-direct OSS upload/download/preview** via presigned URLs.
When `OSS_DIRECT_ENABLED=true`, all asset byte transfer bypasses ECS/nginx/upload-service proxy.
Browser talks directly to Alibaba Cloud OSS using short-lived signed URLs issued by MAIN.

## Upload Flow (Multipart)

### Step 1: Create upload session

```
POST /v1/assets/upload-sessions
Content-Type: application/json
Authorization: Bearer <token>

{
  "task_id": 123,
  "asset_kind": "delivery",
  "file_name": "design.psd",
  "expected_size": 52428800,
  "mime_type": "application/octet-stream",
  "target_sku_code": "SKU001"
}
```

### Step 2: Read response

```json
{
  "session": { "id": "sess_abc123", "upload_mode": "multipart", ... },
  "upload_strategy": "multipart",
  "oss_direct": {
    "mode": "multipart",
    "object_key": "tasks/TASK-001/assets/A0001/v1/delivery/design.psd",
    "upload_id": "oss-upload-id-xyz",
    "parts": [
      { "part_number": 1, "upload_url": "https://bucket.oss-cn-hangzhou.aliyuncs.com/...?Signature=...", "method": "PUT", "expires_at": "2026-04-13T18:15:00Z" },
      { "part_number": 2, "upload_url": "https://...", "method": "PUT", "expires_at": "..." }
    ],
    "part_size": 10485760,
    "bucket": "my-bucket",
    "endpoint": "oss-cn-hangzhou.aliyuncs.com",
    "expires_at": "2026-04-13T18:15:00Z"
  },
  "remote": { ... },
  "complete_endpoint": "/v1/assets/upload-sessions/sess_abc123/complete",
  "cancel_endpoint": "/v1/assets/upload-sessions/sess_abc123/cancel"
}
```

**Frontend MUST prefer `oss_direct` over `remote`.** The `remote` field is compatibility-only.

### Step 3: Upload parts to OSS

For each part:

```javascript
const partData = file.slice(i * partSize, (i + 1) * partSize);
const response = await fetch(part.upload_url, {
  method: 'PUT',
  body: partData,
  // NO Authorization header - the URL is pre-signed
});
const etag = response.headers.get('ETag');
// Store { part_number, etag } for completion
```

**Critical**: Capture the `ETag` response header from each PUT. OSS returns it quoted (e.g. `"abc123"`). Send it as-is.

### Step 4: Complete upload

```
POST /v1/assets/upload-sessions/sess_abc123/complete
Content-Type: application/json
Authorization: Bearer <token>

{
  "file_hash": "sha256:...",
  "oss_upload_id": "oss-upload-id-xyz",
  "oss_object_key": "tasks/TASK-001/assets/A0001/v1/delivery/design.psd",
  "oss_parts": [
    { "part_number": 1, "etag": "\"abc123\"" },
    { "part_number": 2, "etag": "\"def456\"" }
  ]
}
```

## Download Flow

```
GET /v1/assets/{id}/download
Authorization: Bearer <token>
```

Response:

```json
{
  "download_mode": "direct",
  "download_url": "https://bucket.oss-cn-hangzhou.aliyuncs.com/...?Signature=...&response-content-disposition=attachment",
  "access_hint": "oss_presigned",
  "expires_at": "2026-04-13T18:15:00Z",
  "filename": "design.psd",
  "file_size": 52428800,
  "mime_type": "application/octet-stream"
}
```

Frontend opens `download_url` directly. The URL expires after `expires_at` — re-request if needed.

## Preview Flow

```
GET /v1/assets/{id}/preview
Authorization: Bearer <token>
```

Same response shape as download, but the presigned URL includes `response-content-disposition=inline`.
Frontend can use the URL directly in `<img>`, `<iframe>`, or `fetch()`.

## Decision Logic for Frontend

```
if (response.oss_direct) {
  // Use OSS direct presigned URLs — canonical path
  uploadPartsToOSS(response.oss_direct);
} else if (response.remote) {
  // Fallback to upload-service proxy — compatibility only
  uploadPartsToProxy(response.remote);
}
```

## Blacklisted Patterns (do NOT use in new frontend)

- `remote.presigned_upload_url` / `remote.signed_part_urls` — proxy upload URLs
- `GET /v1/assets/files/{path}` — proxy download
- `GET /files/{path}` — proxy download
- `/upload/sessions/{id}/parts/{part_no}` — proxy upload part
- Any URL containing the upload-service internal host
- `UPLOAD_SERVICE_BROWSER_MULTIPART_BASE_URL` dependent logic
- `UPLOAD_SERVICE_BROWSER_DOWNLOAD_BASE_URL` dependent logic

## OSS Bucket CORS Configuration Required

The OSS bucket must have CORS rules allowing:

- **Allowed Origins**: your frontend origins (e.g. `https://app.example.com`, `http://localhost:5173`)
- **Allowed Methods**: `PUT`, `GET`, `HEAD`
- **Allowed Headers**: `Content-Type`, `Content-MD5`, `Content-Length`
- **Expose Headers**: `ETag` (critical for multipart upload completion)
- **Max Age**: `3600`

## Environment Variables for Deployment

```env
OSS_DIRECT_ENABLED=true
OSS_ENDPOINT=oss-cn-hangzhou-internal.aliyuncs.com
OSS_PUBLIC_ENDPOINT=oss-cn-hangzhou.aliyuncs.com
OSS_BUCKET=your-bucket-name
OSS_ACCESS_KEY_ID=LTAI5t...
OSS_ACCESS_KEY_SECRET=...
OSS_PRESIGN_EXPIRY=15m
OSS_PART_SIZE=10485760
```

## Security Notes

- Presigned URLs are short-lived (default 15 minutes)
- No long-lived OSS credentials are exposed to the browser
- Each presigned URL is scoped to a specific HTTP method and object key
- Backend validates auth, permissions, and business rules before issuing presigned URLs
- OSS access key credentials never leave the backend
