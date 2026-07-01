import type { DifficultyClassRow } from '@aw/shared/api/assetWorkbenchApi'

export function activeDifficultyClasses(rows: DifficultyClassRow[]) {
  return [...rows]
    .filter((row) => row.enabled)
    .sort((a, b) => a.sort_order - b.sort_order || a.code.localeCompare(b.code, 'zh-Hans-CN'))
}

export function difficultyCodes(rows: DifficultyClassRow[]) {
  return activeDifficultyClasses(rows).map((row) => row.code)
}

export function difficultyOptionsWithAll(rows: DifficultyClassRow[]) {
  return ['all', ...difficultyCodes(rows)]
}

export function firstDifficultyCode(rows: DifficultyClassRow[], fallback = '') {
  return difficultyCodes(rows)[0] || fallback
}

export function difficultyLabel(code: string, rows: DifficultyClassRow[]) {
  if (code === 'all') return '全部'
  const match = rows.find((row) => row.code === code)
  return match?.name || code
}
