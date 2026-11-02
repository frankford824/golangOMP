// @vitest-environment jsdom
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import SimpleHomePage from './SimpleHomePage.vue'

describe('ordinary client home income visibility',()=>{
 it.each([false,true])('income card follows the personnel switch: %s',visible=>{
  const wrapper=mount(SimpleHomePage,{props:{incomeVisible:visible},global:{stubs:{RouterLink:{props:['to'],template:'<a :href="to"><slot /></a>'}}}})
  expect(wrapper.find('a[href="/my-settlement"]').exists()).toBe(visible)
  expect(wrapper.text().includes('金额')).toBe(visible)
  expect(wrapper.find('a[href="/upload"]').exists()).toBe(true)
  expect(wrapper.find('a[href="/drive"]').exists()).toBe(true)
  wrapper.unmount()
 })
})
