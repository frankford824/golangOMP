import { readFile, readdir } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const vueRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const repoRoot = path.resolve(vueRoot, '..')
const excludedSegments = [
  `${path.sep}services${path.sep}v1Types${path.sep}`,
  `${path.sep}domain${path.sep}archive${path.sep}`,
]
const excludedSuffixes = ['.test.ts', '.spec.ts', '.test.tsx', '.spec.tsx']
const sourceExtensions = new Set(['.ts', '.tsx', '.vue', '.js', '.mjs'])

const frontendPatterns = [
  /\b(?:PendingAuditA|PendingAuditB|RejectedByAuditA|RejectedByAuditB|PendingWarehouseQC|PendingWarehouseReceive|PendingClose)\b/,
  /\bpurchase_task\b/i,
  /\b(?:warehouseApi|outsourceApi)\b/,
  /\/v1\/tasks\/[^\s'"`]+\/(?:warehouse|outsource|close)(?:\/|\b)/i,
  /(?:warehouse|outsource)\\?\/(?:prepare|receive|complete|reject)|tasks\\?\/\[\^\/\]\+\\?\/(?:close)/i,
  /采购任务|待结单|审核\s*[AB]\b/,
]
const authorityPatterns = [
  /\b(?:PendingAuditA|PendingAuditB|RejectedByAuditA|RejectedByAuditB|PendingWarehouseQC|PendingWarehouseReceive|PendingClose)\b/,
  /\/v1\/tasks\/[^\s'"`{}]+\/(?:warehouse|outsource|close)(?:\/|\b)/i,
]

async function collectFiles(root) {
  const output = []
  for (const entry of await readdir(root, { withFileTypes: true })) {
    const absolute = path.join(root, entry.name)
    if (excludedSegments.some((segment) => absolute.includes(segment))) continue
    if (entry.isDirectory()) output.push(...await collectFiles(absolute))
    else if (sourceExtensions.has(path.extname(entry.name)) && !excludedSuffixes.some((suffix) => entry.name.endsWith(suffix))) output.push(absolute)
  }
  return output
}

async function findingsFor(file, patterns) {
  const text = await readFile(file, 'utf8')
  const findings = []
  for (const [index, line] of text.split(/\r?\n/).entries()) {
    if (patterns.some((pattern) => pattern.test(line))) findings.push(`${path.relative(repoRoot, file)}:${index + 1}: ${line.trim()}`)
  }
  return findings
}

const frontendFiles = await collectFiles(path.join(vueRoot, 'src'))
const findings = []
for (const file of frontendFiles) findings.push(...await findingsFor(file, frontendPatterns))
for (const relative of ['transport/http.go', 'docs/api/openapi.yaml']) {
  findings.push(...await findingsFor(path.join(repoRoot, relative), authorityPatterns))
}

if (findings.length) {
  console.error('[v8:stale-audit] retired active workflow terms found:')
  for (const finding of findings) console.error(`- ${finding}`)
  process.exit(1)
}
console.log(`[v8:stale-audit] PASS (${frontendFiles.length + 2} active files checked)`)
