# Frontend Static Publish SOP

## Scope

This SOP publishes the Vue frontend from this monorepo to the production static web root.

It does not deploy the Go backend and does not modify `/root/ecommerce_ai/releases`.

Backend deploys must follow `deploy/DEPLOYMENT_WORKFLOW.md`.

## Repository Boundaries

| Scope | Path | Publish Flow |
| --- | --- | --- |
| Go backend | repository root, excluding `vue/` | `deploy/DEPLOYMENT_WORKFLOW.md` |
| Vue frontend | `vue/` | this SOP |
| Frontend publish artifact | `dist/front` | uploaded to `/var/www/yongbo.cloud` |

Do not mix frontend and backend ownership in review notes or release notes. State whether a change is `frontend`, `backend`, or `both`.

## Branch Rule

`dev/external-developer` is the external development branch.

Future changes on that branch must be reviewed locally before any publish action:

1. Fetch the latest remote branch.
2. Compare `origin/main...origin/dev/external-developer`.
3. Classify the diff as frontend-only, backend-only, or both.
4. Run the matching local validation.
5. Publish only the changed side when possible.
6. After the branch is stable, the project owner decides whether to merge it into `main`.

Do not merge `dev/external-developer` into `main` automatically.

## Fixed Production Targets

| Item | Value |
| --- | --- |
| SSH host | `jst_ecs` by default |
| Static web root | `/var/www/yongbo.cloud` |
| Backup directory | `/var/www/backups/yongbo.cloud_<UTC_TIMESTAMP>/` |
| Staging directory | `/tmp/yongbo.cloud_dist_<UTC_TIMESTAMP>/` |
| Browser API base | same-origin `/v1` through Nginx |
| ICP footer | `苏ICP备2026007026号-1`, linked to `https://beian.miit.gov.cn/` |

## Local Review Before Frontend Publish

From the repository root:

```bash
git fetch origin --prune
git status --short --branch
git log --oneline --decorate --graph --left-right --cherry-pick origin/main...origin/dev/external-developer -20
git diff --stat origin/main...origin/dev/external-developer
git diff --name-status origin/main...origin/dev/external-developer
```

Frontend-only publish is allowed only when the effective publish diff is limited to `vue/` and frontend-facing documentation or configuration that affects the static build.

If the diff includes backend contract files such as `transport/`, `service/`, `domain/`, `db/`, or `docs/api/openapi.yaml`, do not treat it as frontend-only. Review the backend impact first.

## Frontend Build

From the repository root:

```bash
cd vue
npm ci
npm run build:prod
cd ..
rm -rf dist/front
mkdir -p dist/front
cp -a vue/dist/. dist/front/
```

The publish scripts intentionally read from `dist/front`. This keeps the static publish artifact outside `vue/dist` and makes the upload target explicit.

Before publishing, confirm:

- `dist/front/index.html` exists.
- `dist/front/assets/` exists.
- production build does not hardcode `localhost` or `127.0.0.1`.
- browser API traffic remains same-origin and reaches `/v1`.
- relevant frontend tests have passed when the changed area has tests.
- login, list, detail, upload or asset preview pages affected by the change have been smoke-tested locally or in preview.

## Frontend Publish

Windows PowerShell:

```powershell
powershell -ExecutionPolicy Bypass -File .\deploy\publish-front.ps1
```

Bash, Git Bash, WSL, Linux, or macOS:

```bash
bash ./deploy/publish-front.sh
```

Dry run:

```bash
bash ./deploy/publish-front.sh --dry-run
```

The script performs:

1. local `dist/front` checks.
2. remote backup of `/var/www/yongbo.cloud`.
3. upload to a timestamped staging directory.
4. `rsync -a --delete` into `/var/www/yongbo.cloud`.
5. permission normalization.
6. `nginx -t`.
7. `systemctl reload nginx`.
8. HTTP probes for `/`, `/login`, `/health`, and `/v1/auth/login`.

After publish, use a browser and a real account to smoke test the affected workflow.

## Backend Publish Reference

If the reviewed branch contains backend changes, use the backend workflow instead of this SOP:

```bash
bash ./deploy/deploy.sh --version <version> --parallel
```

for side-by-side validation, or:

```bash
bash ./deploy/deploy.sh --version <version>
```

for real cutover after approval.

The backend workflow, release history, runtime paths, SSH key deployment, and remote verification rules are defined in `deploy/DEPLOYMENT_WORKFLOW.md`.

## Both Frontend And Backend Changed

When both sides changed:

1. Review backend API and frontend API usage together.
2. Confirm `docs/api/openapi.yaml` and frontend calls are aligned.
3. Run backend validation required by `AGENTS.md`.
4. Run frontend build and relevant frontend tests.
5. If frontend depends on new backend behavior, publish backend first.
6. Publish frontend after backend verification passes.
7. Smoke test the full browser workflow.

Do not publish a frontend build that depends on backend behavior not yet deployed.

## Rollback

Use the backup path printed by `publish-front`:

```bash
ssh jst_ecs "rsync -a --delete /var/www/backups/yongbo.cloud_<UTC_TIMESTAMP>/ /var/www/yongbo.cloud/ && chmod -R a+rX /var/www/yongbo.cloud && nginx -t && systemctl reload nginx"
```

Backend rollback is not covered by this SOP. Use the backend release workflow and server release layout documented in `deploy/DEPLOYMENT_WORKFLOW.md`.

## Common Mistakes

- Publishing directly from `vue/dist` without copying to `dist/front`.
- Treating a backend contract change as frontend-only.
- Publishing frontend before the backend API it depends on.
- Leaving old hashed assets online by copying without `rsync --delete`.
- Uploading static files into `/root/ecommerce_ai/releases`.
- Merging `dev/external-developer` into `main` before the project owner approves.
