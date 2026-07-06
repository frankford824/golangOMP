// @vitest-environment jsdom
import { mount, flushPromises } from '@vue/test-utils'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import CustomizationReviewForm from './CustomizationReviewForm.vue'
import { uploadTaskFileViaAssetSession } from '@/services/upload/assetUploadFlow'

vi.mock('@/services/upload/assetUploadFlow', () => ({
  uploadTaskFileViaAssetSession: vi.fn(),
}))

const BaseModalStub = {
  props: ['modelValue'],
  template: '<div v-if="modelValue"><slot /><slot name="footer" /></div>',
}

const BaseInputStub = {
  props: ['modelValue', 'label'],
  emits: ['update:modelValue'],
  template: `
    <label>
      <span>{{ label }}</span>
      <input :value="modelValue" @input="$emit('update:modelValue', $event.target.value)" />
    </label>
  `,
}

const BaseTextareaStub = {
  props: ['modelValue', 'label'],
  emits: ['update:modelValue'],
  template: `
    <label>
      <span>{{ label }}</span>
      <textarea :value="modelValue" @input="$emit('update:modelValue', $event.target.value)" />
    </label>
  `,
}

const BaseButtonStub = {
  props: ['disabled', 'loading'],
  emits: ['click'],
  template: '<button :disabled="disabled || loading" @click="$emit(\'click\')"><slot /></button>',
}

function mountForm(props: Record<string, unknown>) {
  return mount(CustomizationReviewForm, {
    props: {
      modelValue: true,
      defaultReviewerId: '7',
      ...props,
    },
    global: {
      stubs: {
        BaseModal: BaseModalStub,
        BaseInput: BaseInputStub,
        BaseTextarea: BaseTextareaStub,
        BaseButton: BaseButtonStub,
      },
    },
  })
}

async function uploadFile(wrapper: ReturnType<typeof mount>) {
  const file = new File(['source'], 'fixed.psd', { type: 'application/octet-stream' })
  const input = wrapper.find('input[type="file"]')
  Object.defineProperty(input.element, 'files', { value: [file], configurable: true })
  await input.trigger('change')
  await flushPromises()
  return file
}

describe('CustomizationReviewForm', () => {
  beforeEach(() => {
    vi.mocked(uploadTaskFileViaAssetSession).mockReset()
  })

  it('uploads source assets and emits source_asset_id for initial review', async () => {
    vi.mocked(uploadTaskFileViaAssetSession).mockResolvedValue({
      asset: { id: '42' },
    } as never)
    const wrapper = mountForm({
      mode: 'initial',
      taskId: '1001',
      canUploadSource: true,
      targetSkuCode: 'SKU-1',
    })

    const file = await uploadFile(wrapper)

    expect(uploadTaskFileViaAssetSession).toHaveBeenCalledWith(
      '1001',
      file,
      {
        asset_kind: 'source',
        owner_module_key: 'customization',
        target_sku_code: 'SKU-1',
        remark: '定制审核源文件：fixed.psd',
      },
      expect.objectContaining({ onProgress: expect.any(Function) }),
    )

    await wrapper.findAll('button').find((button) => button.text() === '确认提交')?.trigger('click')

    expect(wrapper.emitted('submit')?.[0]?.[0]).toMatchObject({
      reviewer_id: 7,
      customization_review_decision: 'approved',
      source_asset_id: '42',
    })
  })

  it('emits current_asset_id for effect review uploads', async () => {
    vi.mocked(uploadTaskFileViaAssetSession).mockResolvedValue({
      asset: { id: '88' },
    } as never)
    const wrapper = mountForm({
      mode: 'effect',
      taskId: '1002',
      canUploadSource: true,
    })

    await uploadFile(wrapper)
    await wrapper.findAll('button').find((button) => button.text() === '确认提交')?.trigger('click')

    expect(wrapper.emitted('submit')?.[0]?.[0]).toMatchObject({
      current_asset_id: '88',
    })
    expect(wrapper.emitted('submit')?.[0]?.[0]).not.toHaveProperty('source_asset_id')
  })

  it('shows upload errors and does not emit an asset id after failure', async () => {
    vi.mocked(uploadTaskFileViaAssetSession).mockRejectedValue(new Error('OSS 上传失败'))
    const wrapper = mountForm({
      mode: 'initial',
      taskId: '1003',
      canUploadSource: true,
    })

    await uploadFile(wrapper)
    expect(wrapper.text()).toContain('OSS 上传失败')

    await wrapper.findAll('button').find((button) => button.text() === '确认提交')?.trigger('click')
    expect(wrapper.emitted('submit')?.[0]?.[0]).not.toHaveProperty('source_asset_id')
  })
})
