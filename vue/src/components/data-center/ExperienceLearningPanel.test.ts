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
        review_materialization_enabled: false,
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
          review_materialization_enabled: false,
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
    Object.defineProperty(window, 'confirm', { value: vi.fn(() => true), writable: true })
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

  it('shows a business-safe error when main stats fail', async () => {
    statsMock.mockRejectedValueOnce(new Error('SQL connection refused'))

    const wrapper = mount(ExperienceLearningPanel)
    await flushPromises()
    await flushPromises()

    expect(wrapper.text()).toContain('经验观测主指标暂不可用')
    expect(wrapper.text()).not.toContain('SQL connection refused')
  })

  it('keeps stats and worker diagnostics visible when sample requests fail', async () => {
    samplesMock.mockRejectedValueOnce(new Error('samples unavailable')).mockRejectedValueOnce(new Error('effective samples unavailable'))

    const wrapper = mount(ExperienceLearningPanel)
    await flushPromises()
    await flushPromises()

    expect(statsMock).toHaveBeenCalled()
    expect(samplesMock).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('闭环健康条')
    expect(wrapper.text()).toContain('反馈率')
    expect(wrapper.text()).toContain('L2+ 样本暂不可用')
    expect(wrapper.text()).toContain('样本接口暂不可用')
    expect(wrapper.text()).toContain('接口暂不可用')
    expect(wrapper.text()).toContain('L2+ 样本接口暂时失败，不能解读为真实无有效经验。')
    expect(wrapper.text()).toContain('样本接口暂时失败，不能解读为真实无样本。')
    expect(wrapper.text()).not.toContain('0 条样本')
    expect(wrapper.text()).not.toContain('暂无 L2+ 候选')
    expect(wrapper.text()).not.toContain('暂无样本')
    expect(wrapper.text()).not.toContain('samples unavailable')
    expect(wrapper.text()).not.toContain('加载经验观测失败')
  })

  it('does not count locked worker runs as failures', async () => {
    statsMock.mockResolvedValueOnce({
      data: {
        data: {
          flags: {
            ui_enabled: true,
            capture_enabled: true,
            ai_feedback_enabled: true,
            worker_enabled: true,
            behavior_capture_enabled: true,
            micro_question_enabled: false,
            review_materialization_enabled: false,
            behavior_sample_rate: 1,
          },
          total_events: 1,
          sample_total: 1,
          displayed_events: 1,
          locatable_samples: 1,
          feedback_samples: 0,
          reasoned_feedback_samples: 0,
          reusable_samples: 0,
          feedback_accepted: 0,
          feedback_partially_accepted: 0,
          feedback_rejected: 0,
          outbox_queued: 0,
          outbox_processing: 0,
          outbox_processed_24h: 0,
          outbox_failed_24h: 0,
          outbox_dead_letter: 0,
          capture_success_rate_24h: 1,
          capture_failure_rate_24h: 0,
          tag_total: 8,
          tag_enabled: 8,
          tag_coverage_rate: 1,
          ai_suggestion_events: 1,
          ai_feedback_events: 0,
          ai_feedback_rate: 0,
          task_profiles: 0,
          asset_quality_labels: 0,
          worker_last_runs: [
            {
              worker_name: 'attribution',
              source_name: 'experience_events',
              started_at: '2026-07-01T08:05:00Z',
              status: 'locked',
              scanned_count: 0,
              enqueued_count: 0,
              skipped_count: 1,
              failed_count: 0,
            },
          ],
          generated_at: '2026-07-01T08:00:00Z',
        },
      },
    } as unknown as Awaited<ReturnType<typeof experienceApi.stats>>)

    const wrapper = mount(ExperienceLearningPanel)
    await flushPromises()
    await flushPromises()

    expect(wrapper.text()).toContain('1 个锁定跳过')
    expect(wrapper.text()).toContain('锁定跳过')
    expect(wrapper.text()).toContain('1 跳过')
    expect(wrapper.text()).not.toContain('1 个失败')
    expect(wrapper.text()).not.toContain('1 个需关注')
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

  it('renders zero capture success and readable worker labels when there are processed samples', async () => {
    statsMock.mockResolvedValueOnce({
      data: {
        data: {
          flags: {
            ui_enabled: true,
            capture_enabled: true,
            ai_feedback_enabled: true,
            worker_enabled: true,
            behavior_capture_enabled: true,
            micro_question_enabled: false,
            review_materialization_enabled: false,
            behavior_sample_rate: 1,
            runtime_config_loaded: false,
            runtime_config_error: 'runtime config file is empty',
          },
          total_events: 3,
          sample_total: 3,
          displayed_events: 3,
          locatable_samples: 22,
          locatable_displayed_events: 2,
          feedback_samples: 1,
          reasoned_feedback_samples: 1,
          reusable_samples: 0,
          feedback_accepted: 1,
          feedback_partially_accepted: 0,
          feedback_rejected: 0,
          outbox_queued: 0,
          outbox_processing: 0,
          outbox_processed_24h: 0,
          outbox_failed_24h: 4,
          outbox_dead_letter: 0,
          capture_success_rate_24h: 0,
          capture_failure_rate_24h: 1,
          tag_total: 8,
          tag_enabled: 8,
          tag_coverage_rate: 1,
          ai_suggestion_events: 3,
          ai_feedback_events: 1,
          ai_feedback_rate: 0.33,
          attribution_total: 7,
          attribution_positive: 2,
          attribution_weak: 1,
          attribution_rejected: 4,
          review_items_open: 2,
          review_items_approved: 1,
          review_items_rejected: 1,
          review_items_needs_more_data: 0,
          micro_question_answers: 5,
          micro_question_answered: 4,
          micro_question_dismissed: 1,
          micro_question_rate_limited: 2,
          task_profiles: 1,
          asset_quality_labels: 0,
          worker_last_runs: [
            {
              worker_name: 'outcome_observer',
              source_name: 'task_status_snapshot',
              started_at: '2026-07-01T08:00:00Z',
              status: 'success',
              scanned_count: 18,
              enqueued_count: 4,
              skipped_count: 14,
              failed_count: 0,
            },
            {
              worker_name: 'attribution',
              source_name: 'experience_events',
              started_at: '2026-07-01T08:05:00Z',
              status: 'partial',
              scanned_count: 7,
              enqueued_count: 2,
              skipped_count: 5,
              failed_count: 0,
            },
          ],
          generated_at: '2026-07-01T08:00:00Z',
        },
      },
    } as unknown as Awaited<ReturnType<typeof experienceApi.stats>>)

    const wrapper = mount(ExperienceLearningPanel)
    await flushPromises()
    await flushPromises()

    expect(wrapper.text()).toContain('采集成功')
    expect(wrapper.text()).toContain('0%')
    expect(wrapper.text()).toContain('24h 处理 0 条，失败 4 条')
    expect(wrapper.text()).toContain('展示建议 -> 展示可定位 -> 正式反馈 -> 反馈原因 -> 侧路候选')
    expect(wrapper.text()).toContain('建议展示')
    expect(wrapper.text()).toContain('展示可定位')
    expect(wrapper.text()).toContain('2 / 3')
    expect(wrapper.text()).toContain('正式反馈')
    expect(wrapper.text()).toContain('反馈原因')
    expect(wrapper.text()).toContain('侧路候选')
    expect(wrapper.text()).toContain('0 条')
    expect(wrapper.text()).toContain('归因候选')
    expect(wrapper.text()).toContain('2 / 1 / 4')
    expect(wrapper.text()).toContain('失败队列，24h 失败 4')
    expect(wrapper.text()).toContain('待复核 2')
    expect(wrapper.text()).toContain('微追问')
    expect(wrapper.text()).toContain('4 / 1 / 2')
    expect(wrapper.text()).toContain('采样 100%')
    expect(wrapper.text()).toContain('运行配置 异常')
    expect(wrapper.text()).toContain('运行配置未生效：runtime config file is empty')
    expect(wrapper.text()).toContain('结果观察')
    expect(wrapper.text()).toContain('归因计算')
    expect(wrapper.text()).not.toContain('Outcome Observertask_status_snapshot')
    expect(wrapper.text()).not.toContain('Attributionexperience_events')
  })

  it('keeps the main observation panel usable when a review decision fails', async () => {
    statsMock.mockResolvedValueOnce({
      data: {
        data: {
          flags: {
            ui_enabled: true,
            capture_enabled: true,
            ai_feedback_enabled: true,
            worker_enabled: true,
            behavior_capture_enabled: true,
            micro_question_enabled: false,
            review_materialization_enabled: true,
            behavior_sample_rate: 1,
          },
          total_events: 1,
          sample_total: 1,
          displayed_events: 1,
          outbox_queued: 0,
          outbox_processing: 0,
          outbox_processed_24h: 0,
          outbox_failed_24h: 0,
          outbox_dead_letter: 0,
          capture_success_rate_24h: 1,
          capture_failure_rate_24h: 0,
          tag_total: 0,
          tag_enabled: 0,
          tag_coverage_rate: 0,
          ai_suggestion_events: 1,
          ai_feedback_events: 0,
          ai_feedback_rate: 0,
          task_profiles: 0,
          asset_quality_labels: 0,
          worker_last_runs: [],
          generated_at: '2026-07-01T08:00:00Z',
        },
      },
    } as unknown as Awaited<ReturnType<typeof experienceApi.stats>>)
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

    await wrapper.findAll('button').find((button) => button.text().includes('确认归因（侧路）'))?.trigger('click')
    await flushPromises()

    expect(reviewDecisionMock).toHaveBeenCalledWith(
      'review-1',
      expect.objectContaining({
        decision: 'approve',
        reason_code: 'verified',
        payload: expect.objectContaining({ review_confirmation: true }),
      }),
    )
    expect(wrapper.text()).toContain('闭环健康条')
    expect(wrapper.text()).toContain('反馈率')
    expect(wrapper.text()).toContain('复核结果未保存，请稍后重试。')
    expect(wrapper.text()).not.toContain('decision failed')
    expect(wrapper.text()).not.toContain('加载经验观测失败')
  })

  it('does not submit review approval when confirmation is cancelled', async () => {
    vi.mocked(window.confirm).mockReturnValueOnce(false)
    statsMock.mockResolvedValueOnce({
      data: {
        data: {
          flags: {
            ui_enabled: true,
            capture_enabled: true,
            ai_feedback_enabled: true,
            worker_enabled: true,
            behavior_capture_enabled: true,
            micro_question_enabled: false,
            review_materialization_enabled: true,
            behavior_sample_rate: 1,
          },
          total_events: 1,
          sample_total: 1,
          displayed_events: 1,
          outbox_queued: 0,
          outbox_processing: 0,
          outbox_processed_24h: 0,
          outbox_failed_24h: 0,
          outbox_dead_letter: 0,
          capture_success_rate_24h: 1,
          capture_failure_rate_24h: 0,
          tag_total: 0,
          tag_enabled: 0,
          tag_coverage_rate: 0,
          ai_suggestion_events: 1,
          ai_feedback_events: 0,
          ai_feedback_rate: 0,
          task_profiles: 0,
          asset_quality_labels: 0,
          worker_last_runs: [],
          generated_at: '2026-07-01T08:00:00Z',
        },
      },
    } as unknown as Awaited<ReturnType<typeof experienceApi.stats>>)
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

    const wrapper = mount(ExperienceLearningPanel)
    await flushPromises()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('确认归因（侧路）'))?.trigger('click')
    await flushPromises()

    expect(window.confirm).toHaveBeenCalled()
    expect(reviewDecisionMock).not.toHaveBeenCalled()
  })

  it('keeps review approve disabled during shadow observation', async () => {
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

    const wrapper = mount(ExperienceLearningPanel)
    await flushPromises()
    await flushPromises()

    expect(wrapper.text()).toContain('队列按 outcome 展示当前最佳候选，不代表全部候选')
    expect(wrapper.text()).toContain('当前最佳候选；行为次数 1 / 行为分 5')
    const approveButton = wrapper.findAll('button').find((button) => button.text().includes('Shadow 观察中'))
    expect(approveButton?.attributes('disabled')).toBeDefined()
    await approveButton?.trigger('click')

    expect(reviewDecisionMock).not.toHaveBeenCalled()
  })
})
