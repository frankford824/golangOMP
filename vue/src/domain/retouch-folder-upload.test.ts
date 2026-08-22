// @vitest-environment jsdom
import { describe, expect, it } from 'vitest'
import { buildRetouchFolderUploadPlan, retouchFolderUploadPlanError, type RetouchFolderUploadTarget } from './retouch-folder-upload'

function folderFile(path: string, content = 'file'): File {
  const name = path.split('/').pop() || 'file.png'
  const file = new File([content], name, { type: name.endsWith('.png') ? 'image/png' : 'application/octet-stream' })
  Object.defineProperty(file, 'webkitRelativePath', { configurable: true, value: path })
  return file
}

const targets: RetouchFolderUploadTarget[] = [
  { groupId: 91, requirementId: 501, order: 1, skuCode: 'SKU-A', sourceFileNames: ['主图.png'] },
  { groupId: 92, requirementId: 502, order: 2, skuCode: 'SKU-B', sourceFileNames: ['细节图.png'] },
]

describe('retouch folder upload planning', () => {
  it('reuses the batch-download requirement directory structure', () => {
    const plan = buildRetouchFolderUploadPlan([
      folderFile('成品/需求1/主图.png'),
      folderFile('成品/需求2/细节图.psd'),
      folderFile('成品/需求2/补充图.tif'),
      folderFile('成品/__MACOSX/._主图.png'),
    ], targets)

    expect(plan.items.map((item) => [item.target.groupId, item.files.map((file) => file.name)])).toEqual([
      [91, ['主图.png']],
      [92, ['细节图.psd', '补充图.tif']],
    ])
    expect(plan.ignoredMetadataCount).toBe(1)
    expect(retouchFolderUploadPlanError(plan)).toBe('')
  })

  it('matches a flat finished folder by unique source filename or SKU', () => {
    const plan = buildRetouchFolderUploadPlan([
      folderFile('成品/主图-已修.png'),
      folderFile('成品/SKU-B-最终.psd'),
    ], targets)
    expect(plan.items.map((item) => item.target.groupId)).toEqual([91, 92])
    expect(retouchFolderUploadPlanError(plan)).toBe('')
  })

  it('accepts one requirement folder at a time and maps renamed siblings by a unique match', () => {
    const plan = buildRetouchFolderUploadPlan([
      folderFile('8.11 谷本文 平衡kt板/细节图-完成.png'),
      folderFile('8.11 谷本文 平衡kt板/正面.jpg'),
      folderFile('8.11 谷本文 平衡kt板/背面.jpg'),
    ], targets)

    expect(plan.items).toHaveLength(1)
    expect(plan.items[0].target.groupId).toBe(92)
    expect(plan.items[0].files.map((file) => file.name)).toEqual(['细节图-完成.png', '正面.jpg', '背面.jpg'])
    expect(plan.missingTargets.map((target) => target.groupId)).toEqual([91])
    expect(retouchFolderUploadPlanError(plan)).toBe('')
  })

  it('maps plain numeric filenames to the flattened source-material sequence', () => {
    const plan = buildRetouchFolderUploadPlan([
      folderFile('8.11 谷本文 平衡kt板/1.jpg'),
      folderFile('8.11 谷本文 平衡kt板/2.jpg'),
    ], targets)

    expect(plan.items.map((item) => [item.target.order, item.files.map((file) => file.name)])).toEqual([
      [1, ['1.jpg']],
      [2, ['2.jpg']],
    ])
    expect(retouchFolderUploadPlanError(plan)).toBe('')
  })

  it('keeps a multi-source requirement together when numeric files use the global sequence', () => {
    const sequencedTargets: RetouchFolderUploadTarget[] = [
      { groupId: 1, order: 1, sourceFileNames: ['1-1.png'] },
      { groupId: 2, order: 2, sourceFileNames: ['2-2.png'] },
      { groupId: 3, order: 3, sourceFileNames: ['1-1.png'] },
      { groupId: 4, order: 4, sourceFileNames: ['4-4.png'] },
      { groupId: 5, order: 5, sourceFileNames: ['5-5.png'] },
      { groupId: 6, order: 6, sourceFileNames: ['2-2.png', '4-4.png', '1-1.png', '5-5.png', '3-3.png'] },
    ]
    const plan = buildRetouchFolderUploadPlan([
      folderFile('8.11 谷本文 平衡kt板/1.jpg'),
      folderFile('8.11 谷本文 平衡kt板/2.jpg'),
      folderFile('8.11 谷本文 平衡kt板/3.jpg'),
      folderFile('8.11 谷本文 平衡kt板/6.jpg'),
      folderFile('8.11 谷本文 平衡kt板/10.jpg'),
    ], sequencedTargets)

    expect(plan.items.map((item) => [item.target.order, item.files.map((file) => file.name)])).toEqual([
      [1, ['1.jpg']],
      [2, ['2.jpg']],
      [3, ['3.jpg']],
      [6, ['6.jpg', '10.jpg']],
    ])
    expect(retouchFolderUploadPlanError(plan)).toBe('')
  })

  it('fails closed for ambiguous, unsupported and missing results', () => {
    const duplicateTargets: RetouchFolderUploadTarget[] = targets.map((target) => ({ ...target, sourceFileNames: ['同名.png'] }))
    const plan = buildRetouchFolderUploadPlan([
      folderFile('成品/同名-完成.png'),
      folderFile('成品/readme.txt'),
    ], duplicateTargets)

    expect(plan.ambiguousFiles).toHaveLength(1)
    expect(plan.unsupportedFiles).toEqual(['成品/readme.txt'])
    expect(plan.missingTargets).toHaveLength(2)
    expect(retouchFolderUploadPlanError(plan)).toContain('匹配不唯一')
    expect(retouchFolderUploadPlanError(plan)).not.toContain('缺少成品')
  })

  it('assigns every supported file when the task has one requirement', () => {
    const plan = buildRetouchFolderUploadPlan([
      folderFile('成品/任意名称.png'),
      folderFile('成品/另一个文件.psd'),
    ], [targets[0]])
    expect(plan.items[0].files).toHaveLength(2)
    expect(retouchFolderUploadPlanError(plan)).toBe('')
  })
})
