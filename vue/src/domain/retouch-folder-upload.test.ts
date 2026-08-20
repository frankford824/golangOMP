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
    expect(retouchFolderUploadPlanError(plan)).toContain('缺少成品')
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
