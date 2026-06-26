# Asset Workbench Font Notice

The asset workbench uses self-hosted fonts only. No runtime font CDN is used.

## Fonts

| Font | Package | Version | License | Usage |
| --- | --- | --- | --- | --- |
| MiSans CJK Lite | `misans` | `4.1.0` | Apache-2.0 | Common Chinese UI glyph subset, only `MiSans-Regular.117/118/119.woff2` |
| Geist Sans | `@fontsource/geist-sans` | `5.2.5` | OFL-1.1 | Latin UI text |
| Geist Mono | `@fontsource/geist-mono` | `5.2.8` | OFL-1.1 | Numbers, IDs, money, hashes, dates |
| Space Grotesk | `@fontsource/space-grotesk` | `5.2.10` | OFL-1.1 | Display labels and page accents |

## Source License Files

Full license texts are provided by the installed npm packages:

- `node_modules/misans/LICENSE`
- `node_modules/@fontsource/geist-sans/LICENSE`
- `node_modules/@fontsource/geist-mono/LICENSE`
- `node_modules/@fontsource/space-grotesk/LICENSE`

## Subset Policy

The workbench does not import `misans/lib/Normal/*.min.css` because that emits the full MiSans CJK shard set. Instead, `src/asset-workbench/styles/tokens.css` declares only the common UI shards `117`, `118`, and `119` for regular weight. Missing Chinese glyphs intentionally fall back to `HarmonyOS Sans SC` / system CJK fonts.

Keep this notice in sync with `src/asset-workbench/styles/tokens.css` and `package.json`.
