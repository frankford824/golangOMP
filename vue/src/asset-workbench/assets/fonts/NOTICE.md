# Asset Workbench Font Notice

The asset workbench uses self-hosted fonts only. No runtime font CDN is used.

## Fonts

| Font | Package | Version | License | Usage |
| --- | --- | --- | --- | --- |
| Source Han Sans CN VF | `@fontpkg/source-han-sans-cn-vf` | `2.5.2` | OFL-1.1 | Chinese UI text and headings |
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

The workbench imports the single Source Han Sans CN VF WOFF2 file already present in the repository dependencies and keeps `Noto Sans SC` / system CJK fonts as fallbacks. MiSans is not used as the workbench Chinese UI font.

Keep this notice in sync with `src/asset-workbench/styles/tokens.css` and `package.json`.
