// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const listUploadDirectoriesAdmin = vi.hoisted(() => vi.fn())
const listDifficultyClasses = vi.hoisted(() => vi.fn())
const createUploadDirectory = vi.hoisted(() => vi.fn())
const updateUploadDirectory = vi.hoisted(() => vi.fn())

vi.mock('@aw/shared/api/assetWorkbenchApi', async (loadOriginal) => {
  const original = await loadOriginal<typeof import('@aw/shared/api/assetWorkbenchApi')>()
  return {
    ...original,
    assetWorkbenchApi: {
      ...original.assetWorkbenchApi,
      listUploadDirectoriesAdmin,
      listDifficultyClasses,
      createUploadDirectory,
      updateUploadDirectory,
    },
  }
})

import UploadDirectoriesSettingsPage from './UploadDirectoriesSettingsPage.vue'

const rows = [
  { id: 1, name: 'A类', oss_prefix: 'a-final', description: 'A 类成品', difficulty_class: 'A', allowed_file_types: ['jpg', 'psd'], enabled: true, sort_order: 1, created_by: 1 },
  { id: 2, name: '历史目录', oss_prefix: 'legacy', description: '', difficulty_class: 'B', allowed_file_types: [], enabled: false, sort_order: 2, created_by: 1 },
]

function textButton(wrapper: ReturnType<typeof mount>, text: string) {
  const found = wrapper.findAll('button').find((button) => button.text().trim() === text)
  if (!found) throw new Error(`missing button ${text}`)
  return found
}

describe('UploadDirectoriesSettingsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    listUploadDirectoriesAdmin.mockResolvedValue(rows)
    listDifficultyClasses.mockResolvedValue([
      { id: 1, code: 'A', name: 'A', description: '', enabled: true, sort_order: 1 },
      { id: 2, code: 'B', name: 'B', description: '', enabled: true, sort_order: 2 },
    ])
    createUploadDirectory.mockImplementation(async (payload) => ({ id: 3, created_by: 1, ...payload }))
    updateUploadDirectory.mockImplementation(async (id, payload) => ({ ...rows.find((row) => row.id === id), ...payload }))
  })

  it('creates, edits, and safely disables upload directories from one visible admin page', async () => {
    const wrapper = mount(UploadDirectoriesSettingsPage)
    await flushPromises()

    expect(wrapper.text()).toContain('已配置 2 个上传目录')
    expect(wrapper.text()).toContain('历史目录')
    expect(wrapper.text()).toContain('已停用')

    await textButton(wrapper, '新建目录').trigger('click')
    const createDialog = wrapper.get('[role="dialog"][aria-label="新建上传目录"]')
    await createDialog.get('input[aria-label="目录名称"]').setValue('C类定稿')
    await createDialog.get('input[aria-label="存储路径"]').setValue('c-final')
    await createDialog.get('select[aria-label="计价分类"]').setValue('B')
    await createDialog.get('input[aria-label="允许格式"]').setValue('jpg, png')
    await createDialog.get('input[aria-label="目录排序"]').setValue(3)
    await textButton(wrapper, '保存目录').trigger('click')
    await flushPromises()

    expect(createUploadDirectory).toHaveBeenCalledWith({
      name: 'C类定稿',
      oss_prefix: 'c-final',
      description: '',
      difficulty_class: 'B',
      allowed_file_types: ['jpg', 'png'],
      enabled: true,
      sort_order: 3,
    })
    expect(wrapper.find('[role="dialog"][aria-label="新建上传目录"]').exists()).toBe(false)

    await wrapper.findAll('button').find((button) => button.text().trim() === '编辑')!.trigger('click')
    const editDialog = wrapper.get('[role="dialog"][aria-label="编辑上传目录"]')
    await editDialog.get('input[aria-label="目录名称"]').setValue('A类新名称')
    await editDialog.get('input[aria-label="存储路径"]').setValue('a-new')
    await textButton(wrapper, '保存目录').trigger('click')
    await flushPromises()

    expect(updateUploadDirectory).toHaveBeenCalledWith(1, expect.objectContaining({ name: 'A类新名称', oss_prefix: 'a-new' }))
    expect(wrapper.find('[role="dialog"][aria-label="编辑上传目录"]').exists()).toBe(false)

    await textButton(wrapper, '停用目录').trigger('click')
    expect(wrapper.get('[role="dialog"][aria-label="确认上传目录状态"]').text()).toContain('历史文件和计价记录不会删除')
    await textButton(wrapper, '确认停用').trigger('click')
    await flushPromises()

    expect(updateUploadDirectory).toHaveBeenCalledWith(1, { enabled: false })
  })
})
