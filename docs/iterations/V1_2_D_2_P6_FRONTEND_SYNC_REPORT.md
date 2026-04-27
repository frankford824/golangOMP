# V1.2-D-2 P6 · frontend documentation sync

- date: 2026-04-26 PT
- generator: `scripts/docs/generate_frontend_docs.py`
- source: `docs/api/openapi.yaml`

## Result

- generated files: 16 family docs + `INDEX.md` + `V1_API_CHEATSHEET.md`
- `/v1` path count used by generator: 209
- all generated docs include `V1.2-D-2 residual drift triage` revision marker
- `docs/frontend/INDEX.md` links to `docs/V1_BACKEND_SOURCE_OF_TRUTH.md`

## Touched Families

| doc | reason |
|---|---|
| `V1_API_TASKS.md` | task list, task events, warehouse, batch, V6 legacy SKU/incident/policy sections regenerated |
| `V1_API_ASSETS.md` | global asset path parameter and upload-request response fields regenerated |
| `V1_API_ERP.md` | ERP Bridge and admin/JST sections regenerated |
| `V1_API_USERS.md` / `V1_API_ORG.md` | org-move response schema regenerated |
| all other family docs | top revision marker regenerated from shared generator |

## Verification

- marker check: PASS
- INDEX SoT link: PASS
- OpenAPI validator after doc generation: PASS
