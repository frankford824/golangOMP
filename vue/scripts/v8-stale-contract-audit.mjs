import { readFile, readdir } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const vueRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const repoRoot = path.resolve(vueRoot, '..')
const excludedSegments = [
  `${path.sep}services${path.sep}v1Types${path.sep}`,
  `${path.sep}domain${path.sep}archive${path.sep}`,
]
const excludedSuffixes = ['_test.go', '.test.ts', '.spec.ts', '.test.tsx', '.spec.tsx']
const sourceExtensions = new Set(['.go', '.ts', '.tsx', '.vue', '.js', '.mjs'])

const frontendPatterns = [
  /\b(?:PendingAuditA|PendingAuditB|RejectedByAuditA|RejectedByAuditB|PendingWarehouseQC|PendingWarehouseReceive|PendingClose)\b/,
  /\bpurchase_task\b/i,
  /\b(?:warehouseApi|outsourceApi)\b/,
  /\/v1\/tasks\/[^\s'"`]+\/(?:warehouse|outsource|close)(?:\/|\b)/i,
  /(?:warehouse|outsource)\\?\/(?:prepare|receive|complete|reject)|tasks\\?\/\[\^\/\]\+\\?\/(?:close)/i,
  /采购任务|待结单|审核\s*[AB]\b/,
  /财务核算|规则与模板|导出中心/,
  /\b(?:FinanceView|RuleConfigView|AuditLogView|ProductManagementView|ReportsHomeView|ExportCenterView|LogsManagementView|KpiView)\b/,
  /\/v1\/(?:audit-logs|rule-templates|export|reports|finance)(?:\/|['"`]|$)/i,
  /\/v1\/(?:product-management|predictions)(?:\/|['"`]|$)/i,
  /\/v1\/(?:workbench\/preferences|task-board\/(?:summary|queues))(?:[?'"`]|$)/i,
]
const authorityPatterns = [
  /\b(?:PendingAuditA|PendingAuditB|RejectedByAuditA|RejectedByAuditB|PendingWarehouseQC|PendingWarehouseReceive|PendingClose)\b/,
  /\/v1\/tasks\/[^\s'"`{}]+\/(?:warehouse|outsource|close)(?:\/|\b)/i,
  /\/v1\/(?:audit-logs|rule-templates|export|reports|finance)(?:\/|\b)/i,
  /\/v1\/(?:product-management|predictions)(?:\/|\b)/i,
  /\/v1\/(?:workbench\/preferences|task-board\/(?:summary|queues))(?:\/|\b)/i,
  /\/v1\/(?:roles|access-rules|users\/[^\s'"`{}]+\/roles)(?:\/|\b)/i,
]
const backendPatterns = [
  /\/v1\/(?:audit-logs|rule-templates|export|reports|finance)(?:\/|\b)/i,
  /\/v1\/(?:product-management|predictions)(?:\/|\b)/i,
  /\/v1\/(?:workbench\/preferences|task-board\/(?:summary|queues))(?:\/|\b)/i,
  /\/v1\/(?:roles|access-rules|users\/[^\s'"`{}]+\/roles)(?:\/|\b)/i,
  /ENABLE_CRON_WAREHOUSE_AUTO_RELEASE|CRON_SCHEDULE_WAREHOUSE_AUTO_RELEASE|WAREHOUSE_AUTO_RELEASE_/,
  /run-migrations-v05\.sh|verify-v05-acceptance\.sh/,
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
const backendFiles = []
for (const relative of ['cmd/server', 'domain', 'repo', 'service', 'transport', 'workers']) {
  backendFiles.push(...await collectFiles(path.join(repoRoot, relative)))
}
for (const file of backendFiles) findings.push(...await findingsFor(file, backendPatterns))
for (const relative of ['transport/http.go', 'docs/api/openapi.yaml']) {
  findings.push(...await findingsFor(path.join(repoRoot, relative), authorityPatterns))
}
for (const relative of [
  'config/erp_short_name_rules.json',
  'deploy/main.env.example',
  'deploy/lib.sh',
  'deploy/DEPLOYMENT_WORKFLOW.md',
]) {
  findings.push(...await findingsFor(path.join(repoRoot, relative), backendPatterns))
}

if (findings.length) {
  console.error('[v8:stale-audit] retired active workflow terms found:')
  for (const finding of findings) console.error(`- ${finding}`)
  process.exit(1)
}
console.log(`[v8:stale-audit] PASS (${frontendFiles.length + backendFiles.length + 2} active files checked)`)
