# V1.2-D-2 P2 · asset path-param normalization

> Date: 2026-04-26 PT
> Baseline: `tmp/v1_2_d_2_p1_audit.json`
> Output: `tmp/v1_2_d_2_p2_audit.json`

## Summary

| metric | after P1 | after P2 | delta |
|---|---:|---:|---:|
| total_paths | 242 | 238 | -4 |
| clean | 129 | 133 | +4 |
| drift | 39 | 39 | 0 |
| unmapped | 2 | 1 | -1 |
| known_gap | 72 | 65 | -7 |
| missing_in_openapi | 14 | 10 | -4 |
| missing_in_code | 6 | 2 | -4 |

## Direction Correction

The D-2 prompt text said `/v1/assets/{asset_id}` should become `/v1/assets/{id}`. Current locked `transport/http.go` shows the opposite for the global asset routes:

```text
assetGroup.GET("/:asset_id", ...)
assetGroup.DELETE("/:asset_id", ...)
assetGroup.GET("/:asset_id/download", ...)
assetGroup.GET("/:asset_id/preview", ...)
```

Because `transport/http.go` is the mounted-route authority and is SHA-locked in this round, P2 normalized OpenAPI to `{asset_id}`.

## Paths Normalized

| method | old OpenAPI path | new OpenAPI path | verdict |
|---|---|---|---|
| GET | `/v1/assets/{id}` | `/v1/assets/{asset_id}` | clean after AssetDetail schema alignment |
| DELETE | `/v1/assets/{id}` | `/v1/assets/{asset_id}` | clean |
| GET | `/v1/assets/{id}/download` | `/v1/assets/{asset_id}/download` | clean |
| GET | `/v1/assets/{id}/preview` | `/v1/assets/{asset_id}/preview` | clean |

P2 also rewrote `AssetDetail` from a broad `allOf: Asset + detail` shape to the concrete `service/asset_center.AssetDetail` response fields. This avoided exposing the newly matched path as a fresh `both_diff`.

## Verification

```text
go run ./tools/contract_audit ... --output tmp/v1_2_d_2_p2_audit.json
summary: clean=133 drift=39 unmapped=1 known_gap=65

go run ./cmd/tools/openapi-validate docs/api/openapi.yaml
openapi validate: 0 error 0 warning
```
