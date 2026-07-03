// @vitest-environment jsdom

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import RetouchRequirementsBlock from './RetouchRequirementsBlock.vue'
import { uploadReferenceFileRef, uploadTaskFileViaAssetSession } from '@/services/upload/assetUploadFlow'

vi.mock('@/services/upload/assetUploadFlow', () => ({
  uploadReferenceFileRef: vi.fn(),
  uploadTaskFileViaAssetSession: vi.fn(),
}))

function mountBlock() {
  return mount(RetouchRequirementsBlock, {
    props: {
      taskId: '2000',
      canUpload: true,
      requirements: [
        {
          id: 77,
          taskId: 2000,
          description: '替换产品背景',
          sortOrder: 1,
          referenceFileRefs: [],
          sourceAssets: [],
        },
      ],
    },
    global: {
      stubs: {
        AssetPreviewMedia: { template: '<div class="asset-preview-stub" />' },
        FileIconFallback: { template: '<div class="file-icon-stub" />' },
      },
    },
  })
}

async function setFileInput(
  wrapper: ReturnType<typeof mountBlock>,
  ariaLabel: string,
  files: File[],
) {
  const input = wrapper.find(`input[aria-label="${ariaLabel}"]`)
  Object.defineProperty(input.element, 'files', {
    value: files,
    configurable: true,
  })
  await input.trigger('change')
  await flushPromises()
}

describe('RetouchRequirementsBlock', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(uploadReferenceFileRef).mockResolvedValue({} as never)
    vi.mocked(uploadTaskFileViaAssetSession).mockResolvedValue({} as never)
  })

  it('uploads supplemental source files with retouch requirement scope', async () => {
    const wrapper = mountBlock()
    const sourceFile = new File(['source'], 'material.psd', { type: 'application/octet-stream' })

    await setFileInput(wrapper, '补传本条素材文件', [sourceFile])

    expect(uploadTaskFileViaAssetSession).toHaveBeenCalledWith(
      '2000',
      sourceFile,
      { asset_kind: 'source', remark: 'material.psd' },
      { retouchRequirementId: 77 },
    )
    expect(wrapper.emitted('uploaded')).toHaveLength(1)
  })

  it('uploads supplemental reference files with retouch requirement scope', async () => {
    const wrapper = mountBlock()
    const referenceFile = new File(['ref'], 'reference.png', { type: 'image/png' })

    await setFileInput(wrapper, '补传本条参考图', [referenceFile])

    expect(uploadReferenceFileRef).toHaveBeenCalledWith(referenceFile, {
      taskId: '2000',
      retouchRequirementId: 77,
    })
    expect(wrapper.emitted('uploaded')).toHaveLength(1)
  })
})
