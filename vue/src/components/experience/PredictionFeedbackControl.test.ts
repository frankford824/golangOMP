// @vitest-environment jsdom
import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import PredictionFeedbackControl from './PredictionFeedbackControl.vue'
import { experienceApi } from '@/services/api/experienceApi'
import type { PredictionSuggestion } from '@/services/api/predictionsApi'

vi.mock('@/services/api/experienceApi', () => ({
  experienceApi: {
    clientConfig: vi.fn(),
    reasonTags: vi.fn(),
    behaviorEvents: vi.fn(),
    feedback: vi.fn(),
  },
}))

const suggestion: PredictionSuggestion = {
  id: 'suggestion-1',
  suggestion_event_id: 'event-1',
  type: 'task_next_action',
  title: '建议补充资产',
  detail: '缺少主图',
  action_label: '查看资产',
  action_type: 'open_task_assets',
  target_type: 'task',
  target_id: '123',
  source: '流程状态',
}

const feedbackMock = vi.mocked(experienceApi.feedback)
const clientConfigMock = vi.mocked(experienceApi.clientConfig)
const reasonTagsMock = vi.mocked(experienceApi.reasonTags)
const behaviorEventsMock = vi.mocked(experienceApi.behaviorEvents)
const feedbackResponse = { data: { data: undefined } } as Awaited<ReturnType<typeof experienceApi.feedback>>

describe('PredictionFeedbackControl', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    clientConfigMock.mockResolvedValue({
      data: {
        data: {
          ai_feedback_enabled: true,
          behavior_capture_enabled: false,
          micro_question_enabled: false,
          behavior_sample_rate: 1,
          enabled_surfaces: ['task_detail', 'asset_center'],
        },
      },
    } as Awaited<ReturnType<typeof experienceApi.clientConfig>>)
    reasonTagsMock.mockRejectedValue(new Error('reason tag fallback'))
    behaviorEventsMock.mockResolvedValue({ data: { data: { received: 1, inserted: 1 } } } as Awaited<
      ReturnType<typeof experienceApi.behaviorEvents>
    >)
    feedbackMock.mockResolvedValue(feedbackResponse)
  })

  it('hides when the runtime flag disables feedback', () => {
    const wrapper = mount(PredictionFeedbackControl, {
      props: { suggestion, surface: 'task_detail', enabled: false },
    })

    expect(wrapper.text()).toBe('')
    expect(feedbackMock).not.toHaveBeenCalled()
  })

  it('submits three-state feedback with the fixed payload context', async () => {
    const wrapper = mount(PredictionFeedbackControl, {
      props: { suggestion, surface: 'task_detail', route: '/tasks/123', enabled: true },
    })

    await wrapper.findAll('button')[0].trigger('click')
    expect(feedbackMock).toHaveBeenCalledWith(
      'event-1',
      expect.objectContaining({
        suggestion_event_id: 'event-1',
        feedback_value: 'accepted',
        outcome_source_type: 'task_detail',
        outcome_source_id: '123',
        payload: {
          surface: 'task_detail',
          target_type: 'task',
          target_id: '123',
          suggestion_id: 'suggestion-1',
          suggestion_type: 'task_next_action',
          source: '流程状态',
          action_type: 'open_task_assets',
          action_label: '查看资产',
          route: '/tasks/123',
        },
      }),
    )

    await wrapper.findAll('button')[1].trigger('click')
    expect(feedbackMock).toHaveBeenLastCalledWith(
      'event-1',
      expect.objectContaining({ feedback_value: 'partially_accepted', reason_code: undefined }),
    )
    expect(wrapper.text()).toContain('规格不符')

    await wrapper.findAll('.prediction-feedback__chip')[3].trigger('click')
    expect(feedbackMock).toHaveBeenLastCalledWith(
      'event-1',
      expect.objectContaining({ feedback_value: 'partially_accepted', reason_code: 'missing_context' }),
    )
  })

  it('keeps the main flow unblocked when saving fails', async () => {
    feedbackMock.mockRejectedValueOnce(new Error('network'))
    const wrapper = mount(PredictionFeedbackControl, {
      props: { suggestion, surface: 'asset_center', enabled: true },
    })

    await wrapper.findAll('button')[2].trigger('click')

    expect(wrapper.text()).toContain('反馈未保存')
    expect(wrapper.findAll('button')).toHaveLength(11)
  })
})
