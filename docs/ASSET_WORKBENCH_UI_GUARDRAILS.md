# Asset Workbench UI Guardrails

Scope: `vue/src/asset-workbench/**`.

- Treat `/drive` as the primary surface for uploaded work records, operational materials, file preview, download, and unified search. Keep `/submissions`, `/materials`, and `/overview` as compatibility redirects, not primary navigation.
- Keep asset-workbench independent from the main-ops shell, router, and `main.css`.
- Use dense operational layouts for repeated work: columns, tables, toolbars, filters, and inline actions. Avoid marketing-style hero sections and decorative cards.
- Do not nest cards inside cards. Page sections should be unframed layouts or full-width panels; cards are for repeated items, tools, and dialogs.
- Buttons and chips must have stable dimensions and wrapping behavior so Chinese labels, counts, and status text do not overlap at desktop or mobile widths.
- Upload directory UI must show both difficulty class and allowed file types. Empty `allowed_file_types` means all formats are accepted.
- User-side uploads must enforce the selected directory's allowed file types for picker selection and drag-drop before creating upload sessions.
- Operational materials must preserve external/system preview and download layering; do not collapse external-resource download logic into uploaded-file logic.
- Global search commands should route to `/drive?scope=all&q=...` and return locatable rows rather than opening legacy overview-only pages.
