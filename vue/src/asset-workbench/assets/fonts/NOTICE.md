# Asset Workbench Font Notice

The asset workbench uses self-hosted fonts only. No runtime font CDN is used.

## Fonts

| Font | Package | Version | License | Usage |
| --- | --- | --- | --- | --- |
| Alibaba PuHuiTi 2.0 | user-provided local font package | 2.0 | Alibaba font legal statement | Retained legacy font asset; not shipped by the current workbench build |
| Source Han Sans CN AW Core | generated from `@fontpkg/source-han-sans-cn-vf` | `2.5.2` | OFL-1.1 | Asset workbench Chinese UI subset |
| Source Han Sans CN VF Fallback | `@fontpkg/source-han-sans-cn-vf` | `2.5.2` | OFL-1.1 | Historical source only; not shipped by the current workbench build |
| Geist Sans | `@fontsource/geist-sans` | `5.2.5` | OFL-1.1 | Latin UI text |
| Geist Mono | `@fontsource/geist-mono` | `5.2.8` | OFL-1.1 | Numbers, IDs, money, hashes, dates |

## Source License Files

Full license texts are provided by the installed npm packages:

- `node_modules/@fontpkg/source-han-sans-cn-vf/README.md`
- `node_modules/@fontsource/geist-sans/LICENSE`
- `node_modules/@fontsource/geist-mono/LICENSE`
- `src/asset-workbench/assets/fonts/AlibabaFonts-LICENSE.txt`
- `src/asset-workbench/assets/fonts/AlibabaFonts-INSTRUCTION.txt`

The Alibaba PuHuiTi 2.0 files are copied from a user-provided local package. The user confirmed this package uses the same Alibaba font legal statement bundled with the previous local font package. The statement says personal and enterprise users may download, install, and use the font for lawful commercial, non-commercial, and embedded use under a free ordinary license. The same statement prohibits actions including modifying or reverse-engineering the font software, creating derivative font software, paid transfer or sublicensing to third parties, incorrect copyright attribution where attribution is possible, and false claims of commercial association with Alimama or affiliates.

## Loading Policy

The current workbench build ships `SourceHanSansCN-AW-Core.woff2`, generated from the installed Source Han Sans CN VF package and containing the workbench UI glyph set plus common business text. Content outside that subset uses the local operating-system CJK font stack. The retained full-glyph Alibaba and Source Han files are no longer imported into the build, avoiding 11.8 MB of optional font transfer on constrained networks.

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
