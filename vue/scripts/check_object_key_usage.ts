#!/usr/bin/env npx tsx
/**
 * CI lint: detect forbidden object_key string-parsing patterns.
 *
 * After Round P the object_key is intentionally opaque. Display
 * filenames must come from session.filename / original_filename /
 * asset detail response, never from splitting the key.
 *
 * Escape hatch: add `// eslint-disable-line object-key-usage` on
 * the same line to suppress (e.g. logging-only usages).
 *
 * Exit code 0 = clean, 1 = violations found.
 */

import { readFileSync, readdirSync } from 'fs'
import { join, relative, extname, dirname } from 'path'
import { fileURLToPath } from 'url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = dirname(__filename)
const SRC_ROOT = join(__dirname, '..', 'src')
const EXTENSIONS = new Set(['.ts', '.tsx', '.vue', '.js', '.jsx'])
const ESCAPE_COMMENT = 'eslint-disable-line object-key-usage'

const OBJECT_KEY_RE = /\bobject_key\b|\bobjectKey\b/
const FORBIDDEN_OPS_RE =
  /\.split\b|\.replace\b|\.match\b|\.substr\b|\.substring\b|\.lastIndexOf\b|\.pop\b|\bbasename\b|\bextname\b|\.slice\b/

interface Violation {
  file: string
  line: number
  text: string
}

function walk(dir: string): string[] {
  const out: string[] = []
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const full = join(dir, entry.name)
    if (entry.isDirectory()) {
      if (entry.name === 'node_modules' || entry.name === '__tests__') continue
      out.push(...walk(full))
    } else if (EXTENSIONS.has(extname(entry.name))) {
      out.push(full)
    }
  }
  return out
}

function check(): Violation[] {
  const files = walk(SRC_ROOT)
  const violations: Violation[] = []
  for (const filePath of files) {
    const content = readFileSync(filePath, 'utf-8')
    const lines = content.split('\n')
    const hasObjectKey = OBJECT_KEY_RE.test(content)
    if (!hasObjectKey) continue

    for (let i = 0; i < lines.length; i++) {
      const line = lines[i]!
      if (!OBJECT_KEY_RE.test(line)) continue
      if (line.includes(ESCAPE_COMMENT)) continue
      if (FORBIDDEN_OPS_RE.test(line)) {
        violations.push({
          file: relative(join(SRC_ROOT, '..'), filePath).replace(/\\/g, '/'),
          line: i + 1,
          text: line.trim(),
        })
      }
    }
  }
  return violations
}

const violations = check()
if (violations.length === 0) {
  console.log('[check_object_key_usage] PASS — no forbidden object_key parsing found.')
  process.exit(0)
} else {
  console.error(
    `[check_object_key_usage] FAIL — ${violations.length} violation(s) found.\n` +
      'object_key is opaque after Round P. Use original_filename / file_name instead.\n' +
      `Escape hatch: add "// ${ESCAPE_COMMENT}" on the offending line.\n`,
  )
  for (const v of violations) {
    console.error(`  ${v.file}:${v.line}  ${v.text}`)
  }
  process.exit(1)
}
