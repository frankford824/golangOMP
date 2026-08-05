// @vitest-environment jsdom

import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import TaskAttachmentWorkspace from './TaskAttachmentWorkspace.vue'

describe('TaskAttachmentWorkspace', () => {
  it('shows redacted attachment metadata without exposing a download action', () => {
    const wrapper = mount(TaskAttachmentWorkspace, {
      props: {
        files: [{
          asset_id: 'AST-001',
          filename: '受限参考图.jpg',
          mime_type: 'image/jpeg',
        }],
      },
    })

    expect(wrapper.text()).toContain('受限参考图.jpg')
    expect(wrapper.text()).toContain('不可下载')
    expect(wrapper.text()).toContain('当前账号没有预览或下载权限')
    expect(wrapper.find('a[download]').exists()).toBe(false)
  })

  it('keeps preview and download available when a controlled URL is present', () => {
    const wrapper = mount(TaskAttachmentWorkspace, {
      props: {
        files: [{
          asset_id: 'AST-002',
          filename: '可见参考图.jpg',
          mime_type: 'image/jpeg',
          download_url: '/v1/assets/AST-002/download',
        }],
      },
    })

    expect(wrapper.get('img').attributes('src')).toBe('/v1/assets/AST-002/download')
    expect(wrapper.get('a[download]').attributes('href')).toBe('/v1/assets/AST-002/download')
    expect(wrapper.text()).not.toContain('不可下载')
  })
})
