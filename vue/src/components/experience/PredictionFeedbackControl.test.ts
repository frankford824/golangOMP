// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
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
    microQuestionEligibility: vi.fn(),
    microQuestionAnswer: vi.fn(),
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
const microQuestionEligibilityMock = vi.mocked(experienceApi.microQuestionEligibility)
const microQuestionAnswerMock = vi.mocked(experienceApi.microQuestionAnswer)
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
    microQuestionEligibilityMock.mockResolvedValue({
      data: {
        data: {
          eligible: false,
          reason: 'disabled',
          remaining_daily: 0,
          reason_tags: [],
        },
      },
    } as unknown as Awaited<ReturnType<typeof experienceApi.microQuestionEligibility>>)
    microQuestionAnswerMock.mockResolvedValue({
      data: {
        data: {
          answer_event_key: 'answer-1',
          answer_value: 'answered',
        },
      },
    } as unknown as Awaited<ReturnType<typeof experienceApi.microQuestionAnswer>>)
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

  it('opens a low-interruption micro question only after the user opts out', async () => {
    clientConfigMock.mockResolvedValueOnce({
      data: {
        data: {
          ai_feedback_enabled: true,
          behavior_capture_enabled: false,
          micro_question_enabled: true,
          behavior_sample_rate: 1,
          enabled_surfaces: ['task_detail'],
        },
      },
    } as Awaited<ReturnType<typeof experienceApi.clientConfig>>)
    microQuestionEligibilityMock.mockResolvedValueOnce({
      data: {
        data: {
          eligible: true,
          answer_event_key: 'answer-1',
          remaining_daily: 2,
          reason_tags: [
            { scene: 'ai_suggestion_micro_question', code: 'temporarily_not_needed', name: '暂时不需要', group: 'micro_question_reason', sort_order: 10 },
            { scene: 'ai_suggestion_micro_question', code: 'will_handle_later', name: '稍后处理', group: 'micro_question_reason', sort_order: 20 },
          ],
        },
      },
    } as unknown as Awaited<ReturnType<typeof experienceApi.microQuestionEligibility>>)
    const wrapper = mount(PredictionFeedbackControl, {
      props: {
        suggestion: { ...suggestion, suggestion_stable_key: 'stable-1', attribution_eligible: true },
        surface: 'task_detail',
        route: '/tasks/123',
        enabled: true,
      },
    })
    await flushPromises()

    expect(wrapper.text()).not.toContain('暂不处理')
    expect(microQuestionEligibilityMock).not.toHaveBeenCalled()

    await wrapper.findAll('button')[1].trigger('click')
    feedbackMock.mockClear()
    expect(wrapper.text()).toContain('暂不处理')

    await wrapper.find('.prediction-feedback__micro-toggle').trigger('click')
    await flushPromises()

    expect(microQuestionEligibilityMock).toHaveBeenCalledWith({
      suggestion_event_id: 'event-1',
      suggestion_stable_key: 'stable-1',
      surface: 'task_detail',
      target_type: 'task',
      target_id: '123',
    })
    expect(wrapper.text()).toContain('暂时不需要')

    await wrapper.findAll('.prediction-feedback__chip').find((chip) => chip.text() === '稍后处理')?.trigger('click')

    expect(microQuestionAnswerMock).toHaveBeenCalledWith({
      answer_event_key: 'answer-1',
      suggestion_event_id: 'event-1',
      suggestion_stable_key: 'stable-1',
      surface: 'task_detail',
      target_type: 'task',
      target_id: '123',
      answer_value: 'answered',
      reason_code: 'will_handle_later',
      payload: {
        route: '/tasks/123',
        suggestion_id: 'suggestion-1',
        suggestion_type: 'task_next_action',
        source: '流程状态',
        action_type: 'open_task_assets',
        action_label: '查看资产',
      },
    })
    expect(feedbackMock).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('暂不处理原因已记')
  })

  it('resets local feedback state when a refreshed display event arrives', async () => {
    const wrapper = mount(PredictionFeedbackControl, {
      props: { suggestion, surface: 'task_detail', enabled: true },
    })

    await wrapper.findAll('button')[0].trigger('click')
    expect(wrapper.find('.prediction-feedback__button--active').text()).toBe('有用')

    await wrapper.setProps({
      suggestion: {
        ...suggestion,
        suggestion_event_id: 'event-2',
      },
    })

    expect(wrapper.find('.prediction-feedback__button--active').exists()).toBe(false)
  })
})
