import fs from 'node:fs'
import path from 'node:path'
import process from 'node:process'

const repoRoot = process.cwd()
const assetRoot = path.join(repoRoot, 'src', 'asset-workbench')

const sourceExtensions = new Set(['.vue', '.ts', '.tsx', '.css'])
const cssExtensions = new Set(['.vue', '.css'])
const tokenFiles = new Set([
  path.join(assetRoot, 'styles', 'tokens.css'),
])

const forbiddenImports = [
  { pattern: /from\s+['"]@\/views(?:\/|['"])/, message: '禁止引用主站 views' },
  { pattern: /from\s+['"]@\/layouts(?:\/|['"])/, message: '禁止引用主站 layouts/AppShell' },
  { pattern: /from\s+['"]@\/router(?:\/|['"])/, message: '禁止引用主站 router' },
  { pattern: /from\s+['"]@\/assets\/main\.css['"]/, message: '禁止引用主站 main.css' },
  { pattern: /import\s+['"]@\/assets\/main\.css['"]/, message: '禁止引用主站 main.css' },
  { pattern: /from\s+['"](?:\.\.\/)+views(?:\/|['"])/, message: '禁止通过相对路径引用主站 views' },
  { pattern: /from\s+['"](?:\.\.\/)+layouts(?:\/|['"])/, message: '禁止通过相对路径引用主站 layouts' },
]

const hardcodedColorPattern = /#[0-9a-fA-F]{3,8}\b|rgba?\(|hsla?\(/g
const importantPattern = /!important\b/g
const applyPattern = /@apply\b/g
const previewContractPattern = /<AssetPreviewMedia[\s\S]*?(?:\basset-id\b|:asset-id=|\bassetId=)/g
const staticClassPattern = /(?<![-:@])\bclass\s*=\s*["']([^"']+)["']/g
const hardcodedLengthPattern = /(?:^|[^\w-])(\d+(?:\.\d+)?(?:px|ms))\b/g
const blockingBrowserDialogPattern = /\b(?:window\.)?(?:prompt|confirm|alert)\s*\(/g
const legacyListClassNames = new Set(['aw-rule-list', 'aw-table-like'])

function listFiles(dir) {
  if (!fs.existsSync(dir)) {
    return []
  }
  const entries = fs.readdirSync(dir, { withFileTypes: true })
  return entries.flatMap((entry) => {
    const fullPath = path.join(dir, entry.name)
    if (entry.isDirectory()) {
      return listFiles(fullPath)
    }
    return sourceExtensions.has(path.extname(entry.name)) ? [fullPath] : []
  })
}

function lineForIndex(source, index) {
  return source.slice(0, index).split('\n').length
}

function report(violations, file, message, index = 0) {
  violations.push({
    file: path.relative(repoRoot, file),
    line: lineForIndex(fs.readFileSync(file, 'utf8'), index),
    message,
  })
}

function scanFile(file, violations) {
  const source = fs.readFileSync(file, 'utf8')

  for (const rule of forbiddenImports) {
    const match = source.match(rule.pattern)
    if (match?.index !== undefined) {
      report(violations, file, rule.message, match.index)
    }
  }

  if (previewContractPattern.test(source)) {
    report(
      violations,
      file,
      'AssetPreviewMedia 只能通过 resolvedPreviewUrl 使用，禁止直接走主站 assetId preview 契约',
      source.indexOf('AssetPreviewMedia'),
    )
  }
  previewContractPattern.lastIndex = 0

  if (path.extname(file) === '.vue') {
    for (const match of source.matchAll(staticClassPattern)) {
      const classNames = match[1].split(/\s+/).filter(Boolean)
      for (const className of classNames) {
        if (!className.startsWith('aw-')) {
          report(violations, file, `工作台静态 class 必须使用 aw- 前缀：${className}`, match.index || 0)
        }
        if (legacyListClassNames.has(className)) {
          report(violations, file, `复杂列表禁止使用 ${className}，请统一使用 WorkbenchDataGrid`, match.index || 0)
        }
      }
    }
  }

  if (['.vue', '.ts', '.tsx'].includes(path.extname(file))) {
    for (const match of source.matchAll(hardcodedLengthPattern)) {
      const token = match[1]
      report(violations, file, `业务源码禁止硬编码尺寸/动效 ${token}，请使用 --aw-* token 或 styles/recipes.css`, match.index || 0)
    }
    for (const match of source.matchAll(blockingBrowserDialogPattern)) {
      report(violations, file, '工作台禁止使用 prompt/confirm/alert，原因类操作必须使用页面内表单', match.index || 0)
    }
  }

  if (cssExtensions.has(path.extname(file))) {
    const isTokenFile = tokenFiles.has(file)
    if (!isTokenFile) {
      const colorMatch = source.match(hardcodedColorPattern)
      if (colorMatch) {
        report(violations, file, `禁止硬编码颜色 ${colorMatch[0]}，请使用 --aw-* token`, source.indexOf(colorMatch[0]))
      }
    }

    const importantMatch = source.match(importantPattern)
    if (importantMatch) {
      report(violations, file, '禁止使用 !important', source.indexOf(importantMatch[0]))
    }

    const applyMatch = source.match(applyPattern)
    if (applyMatch) {
      report(violations, file, '禁止在工作台 CSS 中使用 @apply', source.indexOf(applyMatch[0]))
    }

    const scopedStyles = [...source.matchAll(/<style\s+scoped[^>]*>([\s\S]*?)<\/style>/g)]
    for (const scopedStyle of scopedStyles) {
      const lineCount = scopedStyle[1].split('\n').length
      if (lineCount > 120) {
        report(violations, file, `scoped style 超过 120 行：${lineCount} 行，请迁移到 styles/ 模型`, scopedStyle.index || 0)
      }
    }
  }
}

const files = listFiles(assetRoot)
const violations = []
for (const file of files) {
  scanFile(file, violations)
}

if (violations.length > 0) {
  console.error('asset-workbench architecture audit failed:')
  for (const violation of violations) {
    console.error(`- ${violation.file}:${violation.line} ${violation.message}`)
  }
  process.exit(1)
}

console.log(`asset-workbench architecture audit passed (${files.length} files scanned)`)
