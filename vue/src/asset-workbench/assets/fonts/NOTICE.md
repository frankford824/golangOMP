# Asset Workbench Font Notice

The asset workbench uses self-hosted fonts only. No runtime font CDN is used.

## Fonts

| Font | Package | Version | License | Usage |
| --- | --- | --- | --- | --- |
| Source Han Sans CN AW Core | generated from `@fontpkg/source-han-sans-cn-vf` | `2.5.2` | OFL-1.1 | Asset workbench Chinese UI subset |
| Source Han Sans CN VF Fallback | `@fontpkg/source-han-sans-cn-vf` | `2.5.2` | OFL-1.1 | On-demand fallback for CJK glyphs outside the UI subset |
| Geist Sans | `@fontsource/geist-sans` | `5.2.5` | OFL-1.1 | Latin UI text |
| Geist Mono | `@fontsource/geist-mono` | `5.2.8` | OFL-1.1 | Numbers, IDs, money, hashes, dates |
| Space Grotesk | `@fontsource/space-grotesk` | `5.2.10` | OFL-1.1 | Display labels and page accents |

## Source License Files

Full license texts are provided by the installed npm packages:

- `node_modules/@fontpkg/source-han-sans-cn-vf/README.md`
- `node_modules/@fontsource/geist-sans/LICENSE`
- `node_modules/@fontsource/geist-mono/LICENSE`
- `node_modules/@fontsource/space-grotesk/LICENSE`

## Loading Policy

The workbench loads `SourceHanSansCN-AW-Core.woff2` first. It is generated from the installed Source Han Sans CN VF package and contains only the workbench UI glyph set plus common business text. The full Source Han Sans CN VF file remains available through a separate `unicode-range` fallback face and should load only when user/content text uses CJK glyphs outside the core subset.

Regenerate the subset from the `vue/` directory after adding substantial new Chinese UI copy:

```bash
PYFTSUBSET=/path/to/pyftsubset node scripts/generate-asset-cjk-subset.mjs
```

If `pyftsubset` is already on `PATH`, run:

```bash
node scripts/generate-asset-cjk-subset.mjs
```

`Noto Sans SC` / system CJK fonts remain after the Source Han faces as platform fallbacks. MiSans is not used as the workbench Chinese UI font.

Keep this notice in sync with `src/asset-workbench/styles/tokens.css` and `package.json`.
