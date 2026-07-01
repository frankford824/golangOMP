// @vitest-environment jsdom
import { mount, flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import ExperienceLearningPanel from './ExperienceLearningPanel.vue'
import { experienceApi } from '@/services/api/experienceApi'

vi.mock('@/services/api/experienceApi', () => ({
  experienceApi: {
    config: vi.fn(),
    stats: vi.fn(),
    samples: vi.fn(),
    reasonTags: vi.fn(),
    reviewItems: vi.fn(),
    reviewDecision: vi.fn(),
  },
}))

const configMock = vi.mocked(experienceApi.config)
const statsMock = vi.mocked(experienceApi.stats)
const samplesMock = vi.mocked(experienceApi.samples)
const reasonTagsMock = vi.mocked(experienceApi.reasonTags)
const reviewItemsMock = vi.mocked(experienceApi.reviewItems)
const reviewDecisionMock = vi.mocked(experienceApi.reviewDecision)

function setupExperiencePanelMocks() {
  configMock.mockResolvedValue({
    data: {
      data: {
        ui_enabled: true,
        capture_enabled: true,
        ai_feedback_enabled: true,
        worker_enabled: true,
        behavior_capture_enabled: true,
        micro_question_enabled: false,
        behavior_sample_rate: 1,
      },
    },
  } as Awaited<ReturnType<typeof experienceApi.config>>)
  statsMock.mockResolvedValue({
    data: {
      data: {
        flags: {
          ui_enabled: true,
          capture_enabled: true,
          ai_feedback_enabled: true,
          worker_enabled: true,
          behavior_capture_enabled: true,
          micro_question_enabled: false,
          behavior_sample_rate: 1,
        },
        total_events: 3,
        sample_total: 3,
        displayed_events: 3,
        locatable_samples: 2,
        feedback_samples: 1,
        reasoned_feedback_samples: 1,
        reusable_samples: 0,
        feedback_accepted: 1,
        feedback_partially_accepted: 0,
        feedback_rejected: 0,
        outbox_queued: 0,
        outbox_processing: 0,
        outbox_processed_24h: 1,
        outbox_failed_24h: 0,
        outbox_dead_letter: 0,
        capture_success_rate_24h: 1,
        capture_failure_rate_24h: 0,
        tag_total: 8,
        tag_enabled: 8,
        tag_coverage_rate: 1,
        ai_suggestion_events: 3,
        ai_feedback_events: 1,
        ai_feedback_rate: 0.33,
        task_profiles: 1,
        asset_quality_labels: 0,
        worker_last_runs: [],
        generated_at: '2026-07-01T08:00:00Z',
      },
    },
  } as unknown as Awaited<ReturnType<typeof experienceApi.stats>>)
  samplesMock.mockResolvedValue({
    data: {
      data: [],
      pagination: { page: 1, page_size: 20, total: 0 },
    },
  } as unknown as Awaited<ReturnType<typeof experienceApi.samples>>)
  reasonTagsMock.mockResolvedValue({
    data: {
      data: [{ scene: 'ai_suggestion_feedback', code: 'missing_context', name: '缺上下文', group: 'feedback_reason', sort_order: 40 }],
    },
  } as Awaited<ReturnType<typeof experienceApi.reasonTags>>)
  reviewItemsMock.mockResolvedValue({
    data: {
      data: [],
      pagination: { page: 1, page_size: 8, total: 0 },
    },
  } as unknown as Awaited<ReturnType<typeof experienceApi.reviewItems>>)
  reviewDecisionMock.mockResolvedValue({ data: { data: undefined } } as Awaited<
    ReturnType<typeof experienceApi.reviewDecision>
  >)
}

describe('ExperienceLearningPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setupExperiencePanelMocks()
  })

  it('keeps the main observation panel usable when the review queue fails', async () => {
    reviewItemsMock.mockRejectedValueOnce(new Error('review queue unavailable'))

    const wrapper = mount(ExperienceLearningPanel)
    await flushPromises()
    await flushPromises()

    expect(reviewItemsMock).toHaveBeenCalledWith(
      { status: 'open', item_type: 'attribution_candidate', page: 1, page_size: 8 },
    )
    expect(wrapper.text()).toContain('闭环健康条')
    expect(wrapper.text()).toContain('反馈率')
    expect(wrapper.text()).toContain('复核队列暂不可用')
    expect(wrapper.text()).toContain('不能解读为真实无候选')
    expect(wrapper.text()).not.toContain('review queue unavailable')
    expect(wrapper.text()).not.toContain('加载经验观测失败')
  })

  it('keeps the main observation panel usable when reason tags fail', async () => {
    reasonTagsMock.mockRejectedValueOnce(new Error('reason tags unavailable'))

    const wrapper = mount(ExperienceLearningPanel)
    await flushPromises()
    await flushPromises()

    expect(reasonTagsMock).toHaveBeenCalledWith({ scene: 'ai_suggestion_feedback' })
    expect(wrapper.text()).toContain('闭环健康条')
    expect(wrapper.text()).toContain('反馈率')
    expect(wrapper.text()).not.toContain('reason tags unavailable')
    expect(wrapper.text()).not.toContain('加载经验观测失败')
  })

  it('keeps the main observation panel usable when a review decision fails', async () => {
    reviewItemsMock.mockResolvedValueOnce({
      data: {
        data: [
          {
            item_key: 'review-1',
            item_type: 'attribution_candidate',
            status: 'open',
            priority: 'high',
            evidence_summary: {
              status: 'positive_candidate',
              confidence: 'high',
              score: 0.91,
              suggestion: { target_type: 'task', target_id: '42' },
              behavior: { count: 1, score: 5 },
              feedback: { value: 'accepted' },
              outcome: { action: 'task_status_changed', changed_fields: [{ field: 'task_status' }] },
            },
          },
        ],
        pagination: { page: 1, page_size: 8, total: 1 },
      },
    } as unknown as Awaited<ReturnType<typeof experienceApi.reviewItems>>)
    reviewDecisionMock.mockRejectedValueOnce(new Error('decision failed'))

    const wrapper = mount(ExperienceLearningPanel)
    await flushPromises()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('通过并写入候选'))?.trigger('click')
    await flushPromises()

    expect(reviewDecisionMock).toHaveBeenCalledWith(
      'review-1',
      expect.objectContaining({ decision: 'approve', reason_code: 'verified' }),
    )
    expect(wrapper.text()).toContain('闭环健康条')
    expect(wrapper.text()).toContain('反馈率')
    expect(wrapper.text()).toContain('复核结果未保存，请稍后重试。')
    expect(wrapper.text()).not.toContain('decision failed')
    expect(wrapper.text()).not.toContain('加载经验观测失败')
  })
})
