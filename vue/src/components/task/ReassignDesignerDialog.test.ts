// @vitest-environment jsdom

import { afterEach, describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import ReassignDesignerDialog from './ReassignDesignerDialog.vue'
import type { Designer } from '@/mock/designers'

function mountDialog() {
  return mount(ReassignDesignerDialog, {
    props: {
      modelValue: true,
      currentAssigneeId: '228',
      currentAssigneeName: '王亚琳',
      designers: [
        { id: '228', name: '王亚琳', role: 'designer' },
        { id: 266, name: '张明月', role: 'designer' } as unknown as Designer,
      ],
    },
    attachTo: document.body,
  })
}

async function clickBodyButton(text: string) {
  const button = [...document.querySelectorAll('button')].find((item) => item.textContent?.trim() === text)
  expect(button).toBeTruthy()
  ;(button as HTMLButtonElement).click()
  await nextTick()
}

describe('ReassignDesignerDialog', () => {
  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('accepts numeric candidate ids and waits for parent success before closing', async () => {
    const wrapper = mountDialog()
    const modalPanel = document.body.querySelector('[role="dialog"] > div') as HTMLElement | null
    expect(modalPanel?.style.maxWidth).toBe('48rem')
    const vm = wrapper.vm as unknown as {
      reasonCode: string
      selectedId: string | number
    }

    vm.reasonCode = 'priority'
    vm.selectedId = '266'
    await nextTick()

    await clickBodyButton('下一步：确认')

    expect(document.body.textContent ?? '').toContain('确认将该任务从')
    expect(document.body.textContent ?? '').toContain('张明月')

    await clickBodyButton('确认重新指派')

    expect(wrapper.emitted('confirm')?.[0]).toEqual([
      {
        mode: 'reassign',
        assigneeId: '266',
        assigneeName: '张明月',
        reasonCode: 'priority',
        reasonLabel: '任务优先级调整',
        reasonNote: '',
      },
    ])
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })

  it('allows first assignment without asking for a reassignment reason', async () => {
    const wrapper = mount(ReassignDesignerDialog, {
      props: {
        modelValue: true,
        currentAssigneeId: null,
        currentAssigneeName: null,
        designers: [{ id: 266, name: '张明月', role: 'designer' } as unknown as Designer],
      },
      attachTo: document.body,
    })
    const vm = wrapper.vm as unknown as { selectedId: string | number }
    vm.selectedId = '266'
    await nextTick()

    expect(document.body.textContent ?? '').toContain('指派设计师')
    expect(document.body.textContent ?? '').not.toContain('转派原因')
    await clickBodyButton('下一步：确认')
    expect(document.body.textContent ?? '').toContain('确认将该任务指派给')
    await clickBodyButton('确认指派')

    expect(wrapper.emitted('confirm')?.[0]).toEqual([
      {
        mode: 'reassign',
        assigneeId: '266',
        assigneeName: '张明月',
        reasonCode: '',
        reasonLabel: '',
        reasonNote: '',
      },
    ])
  })
})
