# ITERATION 124

Date: 2026-04-13  
Model: GPT-5 Codex  
Mode: EXPLICIT RELEASE AUTHORIZED

## Goal

Overwrite deploy live `v0.9`, run focused backend self-test, execute backup-first destructive clean reset of task/history/resource data while preserving users, then re-test clean baseline to classify historical image visibility.

## Result

- `v0.9` overwrite release completed after rerun.
- Pre-reset snapshot confirmed historical task/image/resource rows existed.
- Reset cleared task/history/resource business rows and preserved user/role/org access.
- NAS side task-linked upload objects and sqlite metadata were cleaned through documented `synology-dsm` access.
- Post-reset old historical task image APIs no longer returned historical rows.
- Clean-state re-test recreated fresh task/reference data and confirmed fresh reference-file download path.

## Important Runtime Notes

- First deploy attempt failed due live schema drift (`users.employment_type` missing) and CRLF script issues in remote verify helpers.
- Minimal live schema hotfix was applied (backup-first) to restore `/v1/tasks*` clean-state behavior.
- Multipart upload-session remote completion remained blocked by NAS endpoint timeout to `192.168.0.125`, so backend complete stayed `409 INVALID_STATE_TRANSITION` in that lane.

## Classification Conclusion

Historical task image visibility observed before reset was caused by historical leftover task/resource records.  
After clean reset and NAS cleanup, old historical task image paths no longer appeared through normal task/resource APIs.
