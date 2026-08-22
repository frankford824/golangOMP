import { isSupportedFinalUploadFilename } from '@/domain/resource-workflow-files'

export interface RetouchFolderUploadTarget {
  groupId: number
  order: number
  requirementId?: number | null
  skuCode?: string
  sourceFileNames?: string[]
}

export interface RetouchFolderUploadPlanItem {
  target: RetouchFolderUploadTarget
  files: File[]
}

export interface RetouchFolderUploadPlan {
  items: RetouchFolderUploadPlanItem[]
  unmatchedFiles: string[]
  ambiguousFiles: string[]
  unsupportedFiles: string[]
  missingTargets: RetouchFolderUploadTarget[]
  ignoredMetadataCount: number
}

type FolderFile = File & { webkitRelativePath?: string }

function normalize(value: string): string {
  return value.trim().normalize('NFKC').toLowerCase()
}

function extensionless(value: string): string {
  const name = value.replace(/\\/g, '/').split('/').pop() || ''
  const dot = name.lastIndexOf('.')
  return normalize(dot > 0 ? name.slice(0, dot) : name)
}

function relativePath(file: FolderFile): string {
  return String(file.webkitRelativePath || file.name).replace(/\\/g, '/').replace(/^\/+/, '')
}

function isMetadataFile(file: FolderFile): boolean {
  const path = relativePath(file)
  const segments = path.split('/').filter(Boolean)
  const name = segments.at(-1) || ''
  return name === '.DS_Store' || segments.includes('__MACOSX') || name.startsWith('._')
}

function directorySegments(file: FolderFile): string[] {
  const segments = relativePath(file).split('/').filter(Boolean)
  return segments.slice(0, -1).map(normalize)
}

function parentDirectoryKey(file: FolderFile): string {
  return directorySegments(file).join('/')
}

function requirementDirectoryTokens(target: RetouchFolderUploadTarget): string[] {
  const order = String(target.order)
  const paddedOrder = order.padStart(2, '0')
  const tokens = [
    `需求${order}`,
    `需求-${order}`,
    `需求_${order}`,
    `需求${paddedOrder}`,
    `requirement${order}`,
    `requirement-${order}`,
    `requirement_${order}`,
  ]
  if (target.requirementId != null && Number.isSafeInteger(target.requirementId)) {
    const id = String(target.requirementId)
    tokens.push(id, `需求${id}`, `需求-${id}`, `req-${id}`, `requirement-${id}`)
  }
  if (target.skuCode?.trim()) tokens.push(target.skuCode)
  return tokens.map(normalize)
}

function matchByDirectory(file: FolderFile, targets: RetouchFolderUploadTarget[]): RetouchFolderUploadTarget[] {
  const directories = directorySegments(file)
  if (!directories.length) return []
  return targets.filter((target) => requirementDirectoryTokens(target).some((token) => directories.includes(token)))
}

function matchBySKU(file: FolderFile, targets: RetouchFolderUploadTarget[]): RetouchFolderUploadTarget[] {
  const path = normalize(relativePath(file))
  return targets.filter((target) => {
    const sku = normalize(target.skuCode || '')
    return sku.length > 0 && path.includes(sku)
  })
}

function sourceStemMatches(outputStem: string, sourceName: string): boolean {
  const sourceStem = extensionless(sourceName)
  if (!sourceStem) return false
  return outputStem === sourceStem ||
    outputStem.startsWith(`${sourceStem}-`) ||
    outputStem.startsWith(`${sourceStem}_`) ||
    outputStem.startsWith(`${sourceStem} `)
}

function matchBySourceName(file: FolderFile, targets: RetouchFolderUploadTarget[]): RetouchFolderUploadTarget[] {
  const outputStem = extensionless(file.name)
  return targets.filter((target) => (target.sourceFileNames || []).some((name) => sourceStemMatches(outputStem, name)))
}

function matchBySequenceFilename(file: FolderFile, targets: RetouchFolderUploadTarget[]): RetouchFolderUploadTarget[] {
  const match = extensionless(file.name).match(/^(?:需求|requirement|req)?[-_ ]*0*(\d+)$/)
  if (!match) return []
  const sequence = Number.parseInt(match[1], 10)
  if (!Number.isSafeInteger(sequence) || sequence <= 0) return []
  let offset = 0
  for (const target of [...targets].sort((left, right) => left.order - right.order)) {
    const sourceCount = Math.max(1, (target.sourceFileNames || []).filter((name) => name.trim()).length)
    if (sequence > offset && sequence <= offset + sourceCount) return [target]
    offset += sourceCount
  }
  return []
}

function uniqueTargets(targets: RetouchFolderUploadTarget[]): RetouchFolderUploadTarget[] {
  return [...new Map(targets.map((target) => [target.groupId, target])).values()]
}

function resolveTarget(file: FolderFile, targets: RetouchFolderUploadTarget[]): RetouchFolderUploadTarget[] {
  for (const matches of [matchByDirectory(file, targets), matchBySKU(file, targets), matchBySourceName(file, targets), matchBySequenceFilename(file, targets)]) {
    const unique = uniqueTargets(matches)
    if (unique.length) return unique
  }
  return targets.length === 1 ? targets : []
}

export function buildRetouchFolderUploadPlan(
  files: File[],
  targets: RetouchFolderUploadTarget[],
): RetouchFolderUploadPlan {
  const grouped = new Map<number, File[]>()
  const unmatchedFiles: string[] = []
  const ambiguousFiles: string[] = []
  const unsupportedFiles: string[] = []
  let ignoredMetadataCount = 0

  const candidates: Array<{
    file: FolderFile
    path: string
    directory: string
    matches: RetouchFolderUploadTarget[]
  }> = []

  for (const rawFile of files) {
    const file = rawFile as FolderFile
    if (isMetadataFile(file)) {
      ignoredMetadataCount += 1
      continue
    }
    if (!isSupportedFinalUploadFilename(file.name)) {
      unsupportedFiles.push(relativePath(file))
      continue
    }
    candidates.push({
      file,
      path: relativePath(file),
      directory: parentDirectoryKey(file),
      matches: resolveTarget(file, targets),
    })
  }

  // If at least one sibling identifies exactly one requirement, use that
  // requirement for otherwise-unmatched renamed siblings. A folder that points
  // at multiple requirements still fails closed.
  const directoryTargets = new Map<string, Set<number>>()
  for (const candidate of candidates) {
    if (candidate.matches.length !== 1) continue
    const ids = directoryTargets.get(candidate.directory) || new Set<number>()
    ids.add(candidate.matches[0].groupId)
    directoryTargets.set(candidate.directory, ids)
  }

  for (const candidate of candidates) {
    let matches = candidate.matches
    if (matches.length !== 1) {
      const siblingTargets = [...(directoryTargets.get(candidate.directory) || [])]
      if (siblingTargets.length === 1) {
        const siblingTarget = targets.find((target) => target.groupId === siblingTargets[0])
        if (siblingTarget && (matches.length === 0 || matches.some((target) => target.groupId === siblingTarget.groupId))) {
          matches = [siblingTarget]
        }
      }
    }
    if (!matches.length) {
      unmatchedFiles.push(candidate.path)
      continue
    }
    if (matches.length > 1) {
      ambiguousFiles.push(`${candidate.path} → ${matches.map((target) => `需求${target.order}`).join('、')}`)
      continue
    }
    const target = matches[0]
    grouped.set(target.groupId, [...(grouped.get(target.groupId) || []), candidate.file])
  }

  return {
    items: targets
      .filter((target) => grouped.has(target.groupId))
      .map((target) => ({ target, files: grouped.get(target.groupId) || [] })),
    unmatchedFiles,
    ambiguousFiles,
    unsupportedFiles,
    missingTargets: targets.filter((target) => !grouped.has(target.groupId)),
    ignoredMetadataCount,
  }
}

export function retouchFolderUploadPlanError(plan: RetouchFolderUploadPlan): string {
  const parts: string[] = []
  if (plan.unsupportedFiles.length) parts.push(`不支持的文件：${plan.unsupportedFiles.slice(0, 3).join('、')}`)
  if (plan.unmatchedFiles.length) parts.push(`无法匹配需求：${plan.unmatchedFiles.slice(0, 3).join('、')}`)
  if (plan.ambiguousFiles.length) parts.push(`匹配不唯一：${plan.ambiguousFiles.slice(0, 3).join('、')}`)
  if (!plan.items.length && !parts.length) parts.push('未识别到可上传的需求成品')
  return parts.join('；')
}
