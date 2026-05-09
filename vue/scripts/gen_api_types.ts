import { existsSync, mkdirSync, writeFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { spawnSync } from 'node:child_process'

const input = resolve('docs/api/openapi.yaml')
const output = resolve('src/services/v1Types/__generated__.ts')

mkdirSync(dirname(output), { recursive: true })

if (!existsSync(input)) {
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

const result = spawnSync(
  process.platform === 'win32' ? 'npx.cmd' : 'npx',
  ['openapi-typescript', input, '-o', output],
  { stdio: 'inherit' },
)

process.exit(result.status ?? 1)
