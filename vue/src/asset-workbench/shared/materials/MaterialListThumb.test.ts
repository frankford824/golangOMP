// @vitest-environment jsdom
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { describe, expect, it } from 'vitest'

import MaterialListThumb from './MaterialListThumb.vue'

const asset = {
  id: 14354,
  material_id: 982,
  resource_id: '14354',
  source_type: 'system',
  original_filename: 'poster.psd',
  mime_type: 'image/vnd.adobe.photoshop',
}

describe('MaterialListThumb', () => {
  it('requests a preview once for repeated render objects with the same identity', async () => {
    const wrapper = mount(MaterialListThumb, { props: { asset } })
    await nextTick()

    expect(wrapper.emitted('previewNeeded')).toHaveLength(1)

    await wrapper.setProps({ asset: { ...asset } })
    await nextTick()

    expect(wrapper.emitted('previewNeeded')).toHaveLength(1)
  })

  it('reports a failed signed preview URL so the parent can refresh it', async () => {
    const failedUrl = 'https://oss.example.com/expired-preview.webp'
    const wrapper = mount(MaterialListThumb, { props: { asset, cachedUrl: failedUrl } })

    await wrapper.find('img').trigger('error')

    expect(wrapper.emitted('previewFailed')).toEqual([[failedUrl]])
  })
})
