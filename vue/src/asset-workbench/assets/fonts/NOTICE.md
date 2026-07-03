# Asset Workbench Font Notice

The asset workbench uses self-hosted fonts only. No runtime font CDN is used.

## Fonts

| Font | Package | Version | License | Usage |
| --- | --- | --- | --- | --- |
| Alimama DaoLiTi | user-provided local font files | Unknown | Alimama DaoLiTi legal statement | Global asset workbench UI font |
| Source Han Sans CN AW Core | generated from `@fontpkg/source-han-sans-cn-vf` | `2.5.2` | OFL-1.1 | Asset workbench Chinese UI subset |
| Source Han Sans CN VF Fallback | `@fontpkg/source-han-sans-cn-vf` | `2.5.2` | OFL-1.1 | On-demand fallback for CJK glyphs outside the UI subset |
| Geist Sans | `@fontsource/geist-sans` | `5.2.5` | OFL-1.1 | Latin UI text |
| Geist Mono | `@fontsource/geist-mono` | `5.2.8` | OFL-1.1 | Numbers, IDs, money, hashes, dates |

## Source License Files

Full license texts are provided by the installed npm packages:

- `node_modules/@fontpkg/source-han-sans-cn-vf/README.md`
- `node_modules/@fontsource/geist-sans/LICENSE`
- `node_modules/@fontsource/geist-mono/LICENSE`
- `src/asset-workbench/assets/fonts/AlimamaDaoLiTi-LICENSE.txt`
- `src/asset-workbench/assets/fonts/AlimamaDaoLiTi-INSTRUCTION.txt`

The Alimama DaoLiTi files are copied from a user-provided local download. The bundled legal statement says personal and enterprise users may download, install, and use the font for lawful commercial, non-commercial, and embedded use under a free ordinary license. The same statement prohibits actions including modifying or reverse-engineering the font software, creating derivative font software, paid transfer or sublicensing to third parties, incorrect copyright attribution where attribution is possible, and false claims of commercial association with Alimama or affiliates.

## Loading Policy

The workbench loads `AlimamaDaoLiTi.woff2` as the global UI face. Source Han Sans CN remains available as the CJK fallback for glyphs not covered by Alimama DaoLiTi. `SourceHanSansCN-AW-Core.woff2` is generated from the installed Source Han Sans CN VF package and contains only the workbench UI glyph set plus common business text. The full Source Han Sans CN VF file remains available through a separate `unicode-range` fallback face and should load only when user/content text uses CJK glyphs outside the core subset.

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
