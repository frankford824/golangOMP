# ITERATION_117

Title: User-management backend closure on existing `v0.8`

Date: 2026-04-08
Model: GPT-5 Codex

## Goal
- Audit the real MAIN backend user-management capability instead of assuming frontend behavior reflects backend truth.
- Close the missing backend pieces required for a formal user-management page:
  - server-side user list pagination/search/filtering
  - admin-managed user creation with initial password
  - admin-managed password reset
- Preserve existing truth sources:
  - `/v1/auth/me`
  - `/v1/org/options`
  - computed `frontend_access`
  - existing role semantics and route-role authorization
- Overwrite deploy to existing `v0.8` and verify live admin flows.

## Audit result before code changes

### Already supported before this iteration
- `GET /v1/users`
  - pagination
  - `keyword`
  - `status`
  - returns `roles` and computed `frontend_access`
- `GET /v1/users/{id}`
  - user detail with org fields and current roles
- `PATCH /v1/users/{id}`
  - profile/org/status update
  - formal disable semantics already existed via `status=disabled`
- Role management:
  - `GET /v1/roles`
  - `PUT /v1/users/{id}/roles`
  - `POST /v1/users/{id}/roles`
  - `DELETE /v1/users/{id}/roles/{role}`
- User self-service password change:
  - `PUT /v1/auth/password`

### Missing before this iteration
- No formal admin-managed create-user endpoint.
- No formal admin-managed password reset endpoint.
- User list could not filter by `department` / `team` at the backend.
- No physical delete endpoint.

## Final backend solution

### User list
- Kept the formal list endpoint:
  - `GET /v1/users`
- Extended server-side filter support:
  - `keyword`
  - `status`
  - `role`
  - `department`
  - `team`
  - `page`
  - `page_size`

### User detail
- Kept the formal detail endpoint:
  - `GET /v1/users/{id}`
- Contract remains sufficient for frontend detail/modal usage because it already returns:
  - base profile
  - canonical `department/team`
  - current `roles`
  - `status`
  - computed `frontend_access`

### User creation
- Added:
  - `POST /v1/users`
- Request supports:
  - `username/account`
  - `display_name/name`
  - `password`
  - `department`
  - `team/group`
  - `mobile/phone`
  - `email`
  - `roles`
  - optional `status`
- Backend validation:
  - `department/team` must match `/v1/org/options`
  - `roles` must be known workflow roles
  - password must satisfy the existing password rule
  - username/mobile uniqueness still enforced

### Password reset
- Added:
  - `PUT /v1/users/{id}/password`
- Semantics:
  - replaces the stored local password hash
  - returns the updated user object
  - does not revoke already-issued session tokens in this minimal version

### Disable / delete semantics
- Kept disable as the formal write path:
  - `PATCH /v1/users/{id}` with `status=disabled`
- Did not add physical delete in this round.
- Frontend should map “删除/禁用” to disable unless a future formal delete semantic is added.

## Code changes

### Runtime code
- `service/identity_service.go`
  - added `CreateManagedUser`
  - added `ResetUserPassword`
  - extended `UserFilter` with `department/team`
  - added backend validation for list `role/department`
- `repo/interfaces.go`
  - extended `UserListFilter`
- `repo/mysql/identity.go`
  - added `department/team` SQL filtering in user list query
- `transport/handler/user_admin.go`
  - added `CreateUser`
  - added `ResetPassword`
  - list handler now passes `role/department/team` filters
- `transport/http.go`
  - routed `POST /v1/users`
  - routed `PUT /v1/users/{id}/password`
- `domain/frontend_access.go`
  - added audit action constants for user creation and password reset

### Tests
- `service/identity_service_test.go`
  - added managed create test
  - added managed reset-password test
  - added list filter test
  - added disabled-user login denial test
  - upgraded in-memory repo stub to honor list filters/pagination
- `transport/handler/user_admin_test.go`
  - added handler tests for list-filter binding, create payload binding, and reset-password binding

### OpenAPI
- `docs/api/openapi.yaml`
  - documented `POST /v1/users`
  - documented `PUT /v1/users/{id}/password`
  - documented new `GET /v1/users` filter params
  - added create/reset request schemas
  - expanded `V7Role` enum to match the current backend-known role set

## Local verification

### Required commands
- `go test ./service ./transport/handler` -> passed
- `go build ./cmd/server` -> passed
- `go build ./repo/mysql ./service ./transport/handler` -> passed
- `go test ./repo/mysql` -> passed

## Deploy
- Command:
  - `bash ./deploy/deploy.sh --version v0.8 --skip-tests --release-note "overwrite v0.8 user management backend closure: managed create/reset/list filters"`
- Result:
  - overwrite deploy to existing `v0.8` succeeded
  - runtime verify succeeded
  - `8080 /health = 200`
  - `8081 /health = 200`
  - `8082 /health = 200`
  - `/proc/<pid>/exe` healthy and not deleted

## Live acceptance
- Base admin/runtime:
  - admin login `200`
  - `GET /v1/auth/me` `200`
  - `GET /v1/org/options` `200`
- Create:
  - `POST /v1/users` created live user `id=177`
- List:
  - `GET /v1/users` with `keyword+department+team+role+page/page_size` returned the created user and `pagination.total=1`
- Role management:
  - `GET /v1/roles` returned `17` entries
  - `GET /v1/users/177` returned current roles plus `frontend_access`
  - `PUT /v1/users/177/roles` changed `Ops` -> `Designer`
  - created user `GET /v1/auth/me` reflected `Designer` role and matching frontend pages/actions
- Password reset:
  - `PUT /v1/users/177/password` returned `200`
  - old password login returned `401`
  - new password login returned `200`
- Disable:
  - `PATCH /v1/users/177` with `status=disabled` returned `200`
  - disabled user login returned `403`

## Documentation updates
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `ITERATION_INDEX.md`
- `docs/iterations/ITERATION_117.md`
- `docs/api/openapi.yaml`

## Risks / remaining boundary
- No physical delete endpoint yet.
- Password reset does not revoke already-issued sessions.
- No new live task-create write probe was executed in this iteration; task-create/org-bridge regression remained covered by local tests/builds plus `/v1/org/options` live verification only.
