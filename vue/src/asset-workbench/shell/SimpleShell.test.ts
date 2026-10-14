// @vitest-environment jsdom
import { shallowMount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import SimpleShell from './SimpleShell.vue'

vi.mock('vue-router', () => ({
  useRoute: () => ({ path: '/upload-overview' }),
  RouterLink: { name: 'RouterLink', props: ['to'], template: '<a :href="to"><slot /></a>' },
  RouterView: { name: 'RouterView', template: '<div />' },
}))

describe('SimpleShell personal upload ledger entry', () => {
  it.each([true, false])('shows upload ledger only for submitters: %s', (submitter) => {
    const wrapper = shallowMount(SimpleShell, { props: { bootstrap: { capabilities: submitter ? ['asset.workbench.submit'] : ['asset.workbench.material.download'] } as never },
      global: { stubs: { RouterLink: { props: ['to'], template: '<a :href="to"><slot /></a>' } } } })
    const entry = wrapper.find('a[href="/upload-overview"]')
    expect(entry.exists()).toBe(submitter)
    if (submitter) expect(entry.text()).toBe('上传台账')
    wrapper.unmount()
  })
})
