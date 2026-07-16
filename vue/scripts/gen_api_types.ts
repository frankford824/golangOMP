import { existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, resolve } from 'node:path'
import { spawnSync } from 'node:child_process'

const inputCandidates = [resolve('../docs/api/openapi.yaml'), resolve('docs/api/openapi.yaml')]
const input = inputCandidates.find((candidate) => existsSync(candidate)) ?? inputCandidates[0]
const output = resolve('src/services/v1Types/__generated__.ts')
const checkOnly = process.argv.includes('--check')

if (!checkOnly) mkdirSync(dirname(output), { recursive: true })

if (!existsSync(input)) {
  if (checkOnly) {
    console.error('[gen:api-types] docs/api/openapi.yaml not found; cannot verify generated types.')
    process.exit(1)
  }
  writeFileSync(
    output,
    [
      '// Generated from docs/api/openapi.yaml by `npm run gen:api-types`.',
      '// OpenAPI source is not present in this checkout; keeping a placeholder.',
      'export type paths = Record<string, never>',
      'export type components = Record<string, never>',
      '',
    ].join('\n'),
  )
  console.warn('[gen:api-types] docs/api/openapi.yaml not found; wrote placeholder types.')
  process.exit(0)
}

const temporaryDir = checkOnly ? mkdtempSync(resolve(tmpdir(), 'v1-types-check-')) : ''
const generatedOutput = checkOnly ? resolve(temporaryDir, '__generated__.ts') : output
const result = spawnSync(process.platform === 'win32' ? 'npx.cmd' : 'npx', ['openapi-typescript', input, '-o', generatedOutput], { stdio: 'inherit' })

if ((result.status ?? 1) !== 0) {
  if (temporaryDir) rmSync(temporaryDir, { recursive: true, force: true })
  process.exit(result.status ?? 1)
}

if (checkOnly) {
  const normalize = (value: string) => value.replace(/\r\n/g, '\n')
  const expected = existsSync(output) ? normalize(readFileSync(output, 'utf8')) : ''
  const generated = normalize(readFileSync(generatedOutput, 'utf8'))
  rmSync(temporaryDir, { recursive: true, force: true })
  if (!expected || expected !== generated) {
    console.error('[gen:api-types] generated types are stale; run `npm run gen:api-types`.')
    process.exit(1)
  }
  console.log('[gen:api-types] generated types match docs/api/openapi.yaml.')
}

process.exit(0)
