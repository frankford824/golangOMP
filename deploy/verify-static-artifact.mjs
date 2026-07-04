#!/usr/bin/env node
import fs from 'node:fs'
import path from 'node:path'

const APPS = {
  'main-ops': {
    entry: 'index.html',
    targetHost: 'yongbo.cloud',
    targetWebRoot: '/var/www/yongbo.cloud',
    allowedBuildCommands: new Set(['npm run build:prod']),
    requiredFiles: ['index.html', 'assets', 'static-artifact-manifest.json'],
    forbiddenFiles: ['asset.html'],
    requiredMarkers: ['id="app"'],
    forbiddenMarkers: ['asset-workbench-app', 'src="/assets/asset-'],
  },
  'asset-workbench': {
    entry: 'asset.html',
    targetHost: 'assets.yongbo.cloud',
    targetWebRoot: '/var/www/assets.yongbo.cloud',
    allowedBuildCommands: new Set(['npm run build:asset']),
    requiredFiles: ['asset.html', 'assets', 'static-artifact-manifest.json'],
    forbiddenFiles: ['index.html'],
    requiredMarkers: ['asset-workbench-app', 'src="/assets/asset-'],
    forbiddenMarkers: ['id="app"'],
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

function fail(message) {
  console.error(`[verify-static-artifact] ERROR: ${message}`)
  process.exit(1)
}

function assertPath(baseDir, relPath, shouldExist) {
  const fullPath = path.join(baseDir, relPath)
  const exists = fs.existsSync(fullPath)
  if (shouldExist && !exists) fail(`Missing required artifact path: ${fullPath}`)
  if (!shouldExist && exists) fail(`Forbidden artifact path exists: ${fullPath}`)
}

let args
try {
  args = parseArgs(process.argv.slice(2))
} catch (error) {
  fail(error instanceof Error ? error.message : String(error))
}

const app = args.app
const config = APPS[app]
if (!config) fail(`Unknown app: ${app}`)

const artifactDir = path.resolve(args.dir || '')
if (!fs.existsSync(artifactDir) || !fs.statSync(artifactDir).isDirectory()) {
  fail(`Artifact directory does not exist: ${artifactDir}`)
}

for (const relPath of config.requiredFiles) assertPath(artifactDir, relPath, true)
for (const relPath of config.forbiddenFiles) assertPath(artifactDir, relPath, false)

const manifestPath = path.join(artifactDir, 'static-artifact-manifest.json')
let manifest
try {
  manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'))
} catch (error) {
  fail(`Could not parse ${manifestPath}: ${error instanceof Error ? error.message : String(error)}`)
}

const expectedFields = {
  schema: 1,
  app,
  entry: config.entry,
  targetHost: config.targetHost,
  targetWebRoot: config.targetWebRoot,
}

for (const [field, expected] of Object.entries(expectedFields)) {
  if (manifest[field] !== expected) {
    fail(`Manifest field ${field} expected ${JSON.stringify(expected)} but got ${JSON.stringify(manifest[field])}`)
  }
}

if (!config.allowedBuildCommands.has(manifest.buildCommand)) {
  fail(`Manifest buildCommand ${JSON.stringify(manifest.buildCommand)} is not allowed for ${app}`)
}
if (typeof manifest.gitCommit !== 'string' || !/^[0-9a-f]{40}$/.test(manifest.gitCommit)) {
  fail('Manifest gitCommit must be a 40-character commit hash')
}
if (typeof manifest.builtAt !== 'string' || Number.isNaN(Date.parse(manifest.builtAt))) {
  fail('Manifest builtAt must be an ISO timestamp')
}

const entryPath = path.join(artifactDir, config.entry)
const entryHtml = fs.readFileSync(entryPath, 'utf8')
for (const marker of config.requiredMarkers) {
  if (!entryHtml.includes(marker)) fail(`${config.entry} does not contain required marker ${marker}`)
}
for (const marker of config.forbiddenMarkers) {
  if (entryHtml.includes(marker)) fail(`${config.entry} contains forbidden marker ${marker}`)
}

const assetRefs = [...entryHtml.matchAll(/(?:src|href)="\/(assets\/[^"]+)"/g)].map((match) => match[1])
for (const relPath of assetRefs) {
  assertPath(artifactDir, relPath, true)
}

console.log(`[verify-static-artifact] OK app=${app} dir=${artifactDir}`)
