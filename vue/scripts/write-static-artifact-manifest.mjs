#!/usr/bin/env node
import { execFileSync } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const scriptDir = path.dirname(fileURLToPath(import.meta.url))
const vueRoot = path.resolve(scriptDir, '..')
const repoRoot = path.resolve(vueRoot, '..')

const APPS = {
  'main-ops': {
    entry: 'index.html',
    mountMarker: 'id="app"',
    forbiddenMarkers: ['asset-workbench-app', 'src="/assets/asset-'],
    targetHost: 'yongbo.cloud',
    targetWebRoot: '/var/www/yongbo.cloud',
  },
  'asset-workbench': {
    entry: 'asset.html',
    mountMarker: 'asset-workbench-app',
    requiredMarkers: ['src="/assets/asset-'],
    forbiddenMarkers: ['id="app"'],
    targetHost: 'assets.yongbo.cloud',
    targetWebRoot: '/var/www/assets.yongbo.cloud',
  },
}

function parseArgs(argv) {
  const args = {}
  for (let i = 0; i < argv.length; i += 1) {
    const key = argv[i]
    if (!key.startsWith('--')) throw new Error(`Unexpected argument: ${key}`)
    const value = argv[i + 1]
    if (!value || value.startsWith('--')) throw new Error(`Missing value for ${key}`)
    args[key.slice(2)] = value
    i += 1
  }
  return args
}

function git(args) {
  try {
    return execFileSync('git', ['-C', repoRoot, ...args], { encoding: 'utf8' }).trim()
  } catch {
    return ''
  }
}

function fail(message) {
  console.error(`[write-static-artifact-manifest] ERROR: ${message}`)
  process.exit(1)
}

const args = parseArgs(process.argv.slice(2))
const app = args.app
const config = APPS[app]
if (!config) fail(`Unknown app: ${app}`)

const outDir = path.resolve(vueRoot, args.out || '')
const entry = args.entry || config.entry
const buildCommand = args['build-command'] || ''
if (!buildCommand) fail('--build-command is required')

const entryPath = path.join(outDir, entry)
if (!fs.existsSync(entryPath)) fail(`Missing entry file: ${entryPath}`)

const entryHtml = fs.readFileSync(entryPath, 'utf8')
if (!entryHtml.includes(config.mountMarker)) fail(`${entry} does not contain ${config.mountMarker}`)
for (const marker of config.requiredMarkers || []) {
  if (!entryHtml.includes(marker)) fail(`${entry} does not contain ${marker}`)
}
for (const marker of config.forbiddenMarkers || []) {
  if (entryHtml.includes(marker)) fail(`${entry} contains forbidden marker ${marker}`)
}

const manifest = {
  schema: 1,
  app,
  entry,
  targetHost: config.targetHost,
  targetWebRoot: config.targetWebRoot,
  buildCommand,
  gitCommit: git(['rev-parse', 'HEAD']),
  gitBranch: git(['rev-parse', '--abbrev-ref', 'HEAD']),
  builtAt: new Date().toISOString(),
  sourceDir: path.relative(repoRoot, outDir).replaceAll(path.sep, '/'),
  generator: 'vue/scripts/write-static-artifact-manifest.mjs',
}

fs.writeFileSync(path.join(outDir, 'static-artifact-manifest.json'), `${JSON.stringify(manifest, null, 2)}\n`)
console.log(`[write-static-artifact-manifest] wrote ${manifest.sourceDir}/static-artifact-manifest.json for ${app}`)
