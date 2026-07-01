// @vitest-environment jsdom
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { experienceApi, type ExperienceClientConfig } from '@/services/api/experienceApi'
import {
  configureExperienceBehavior,
  flushExperienceBehaviorQueue,
  recordExperienceBehavior,
  resetExperienceBehaviorForTests,
  setExperienceBehaviorEnabled,
  setExperienceBehaviorEnabledSurfaces,
  setExperienceBehaviorSampleRate,
} from '@/services/experienceBehavior'

vi.mock('@/services/api/experienceApi', () => ({
  experienceApi: {
    clientConfig: vi.fn(),
    behaviorEvents: vi.fn(),
  },
}))

vi.mock('@/services/http', () => ({
  getToken: vi.fn(() => ''),
}))

const behaviorEventsMock = vi.mocked(experienceApi.behaviorEvents)
const clientConfigMock = vi.mocked(experienceApi.clientConfig)

function clientConfigResponse(data: ExperienceClientConfig): Awaited<ReturnType<typeof experienceApi.clientConfig>> {
  return {
    data: { data },
    status: 200,
    statusText: 'OK',
    headers: {},
    config: { headers: {} },
  } as Awaited<ReturnType<typeof experienceApi.clientConfig>>
}

describe('experienceBehavior', () => {
  beforeEach(async () => {
    vi.clearAllMocks()
    resetExperienceBehaviorForTests()
    clientConfigMock.mockResolvedValue(
      clientConfigResponse({
        ai_feedback_enabled: true,
        behavior_capture_enabled: false,
        micro_question_enabled: false,
        behavior_sample_rate: 1,
        enabled_surfaces: [],
      }),
    )
    setExperienceBehaviorEnabled(false)
    setExperienceBehaviorEnabledSurfaces([])
    setExperienceBehaviorSampleRate(1)
    await flushExperienceBehaviorQueue()
  })

  it('drops behavior events until capture is explicitly enabled', async () => {
    recordExperienceBehavior({
      action: 'jump',
      surface: 'task_detail',
      target_type: 'task',
      target_id: '42',
      suggestion_event_id: 'display-1',
    })
    await flushExperienceBehaviorQueue()

    expect(behaviorEventsMock).not.toHaveBeenCalled()
  })

  it('drops behavior events when sample rate is zero', async () => {
    setExperienceBehaviorEnabled(true)
    setExperienceBehaviorEnabledSurfaces(['task_detail'])
    setExperienceBehaviorSampleRate(0)

    recordExperienceBehavior({
      action: 'impression',
      surface: 'task_detail',
      target_type: 'task',
      target_id: '42',
      suggestion_event_id: 'display-1',
    })
    await flushExperienceBehaviorQueue()

    expect(behaviorEventsMock).not.toHaveBeenCalled()
  })

  it('sends behavior events when sample rate is one', async () => {
    setExperienceBehaviorEnabled(true)
    setExperienceBehaviorEnabledSurfaces(['task_detail'])
    setExperienceBehaviorSampleRate(1)

    recordExperienceBehavior({
      action: 'impression',
      surface: 'task_detail',
      target_type: 'task',
      target_id: '42',
      suggestion_event_id: 'display-1',
    })
    await flushExperienceBehaviorQueue()

    expect(behaviorEventsMock).toHaveBeenCalledWith({
      events: [
        expect.objectContaining({
          action: 'impression',
          surface: 'task_detail',
          target_type: 'task',
          target_id: '42',
          suggestion_event_id: 'display-1',
        }),
      ],
    })
  })

  it('keeps sampling consistent for the same suggestion chain', async () => {
    const randomSpy = vi.spyOn(Math, 'random').mockReturnValueOnce(0.4).mockReturnValue(0.99)
    setExperienceBehaviorEnabled(true)
    setExperienceBehaviorEnabledSurfaces(['task_detail'])
    setExperienceBehaviorSampleRate(0.5)

    recordExperienceBehavior({
      client_event_id: 'evt-1',
      page_instance_id: 'page-1',
      action: 'impression',
      surface: 'task_detail',
      target_type: 'task',
      target_id: '42',
      suggestion_event_id: 'display-1',
    })
    recordExperienceBehavior({
      client_event_id: 'evt-2',
      page_instance_id: 'page-1',
      action: 'jump',
      surface: 'task_detail',
      target_type: 'task',
      target_id: '42',
      suggestion_event_id: 'display-1',
    })
    await flushExperienceBehaviorQueue()

    expect(randomSpy).toHaveBeenCalledTimes(1)
    expect(behaviorEventsMock).toHaveBeenCalledWith({
      events: [
        expect.objectContaining({ action: 'impression', client_event_id: 'evt-1' }),
        expect.objectContaining({ action: 'jump', client_event_id: 'evt-2' }),
      ],
    })
    randomSpy.mockRestore()
  })

  it('drops behavior events outside enabled surfaces', async () => {
    setExperienceBehaviorEnabled(true)
    setExperienceBehaviorEnabledSurfaces(['task_detail'])
    setExperienceBehaviorSampleRate(1)

    recordExperienceBehavior({
      action: 'impression',
      surface: 'asset_center',
      target_type: 'asset',
      target_id: '7001',
      suggestion_event_id: 'display-asset',
    })
    await flushExperienceBehaviorQueue()

    expect(behaviorEventsMock).not.toHaveBeenCalled()
  })

  it('lazy loads client config before recording early page-level events', async () => {
    resetExperienceBehaviorForTests()
    clientConfigMock.mockResolvedValue(
      clientConfigResponse({
        ai_feedback_enabled: true,
        behavior_capture_enabled: true,
        micro_question_enabled: false,
        behavior_sample_rate: 1,
        enabled_surfaces: ['task_detail'],
      }),
    )

    recordExperienceBehavior({
      action: 'visible',
      surface: 'task_detail',
      target_type: 'task',
      target_id: '42',
      suggestion_event_id: 'display-1',
    })
    await Promise.resolve()
    await Promise.resolve()
    await flushExperienceBehaviorQueue()

    expect(clientConfigMock).toHaveBeenCalledTimes(1)
    expect(behaviorEventsMock).toHaveBeenCalledWith({
      events: [
        expect.objectContaining({
          action: 'visible',
          surface: 'task_detail',
          target_type: 'task',
          target_id: '42',
          suggestion_event_id: 'display-1',
        }),
      ],
    })
  })

  it('drops pending early events when lazy client config disables capture', async () => {
    resetExperienceBehaviorForTests()
    clientConfigMock.mockResolvedValue(
      clientConfigResponse({
        ai_feedback_enabled: true,
        behavior_capture_enabled: false,
        micro_question_enabled: false,
        behavior_sample_rate: 1,
        enabled_surfaces: ['task_detail'],
      }),
    )

    recordExperienceBehavior({
      action: 'visible',
      surface: 'task_detail',
      target_type: 'task',
      target_id: '42',
      suggestion_event_id: 'display-1',
    })
    await Promise.resolve()
    await Promise.resolve()
    await flushExperienceBehaviorQueue()

    expect(clientConfigMock).toHaveBeenCalledTimes(1)
    expect(behaviorEventsMock).not.toHaveBeenCalled()
  })

  it('does not let a late lazy config failure override explicit component config', async () => {
    resetExperienceBehaviorForTests()
    clientConfigMock.mockRejectedValue(new Error('network down'))

    recordExperienceBehavior({
      action: 'visible',
      surface: 'task_detail',
      target_type: 'task',
      target_id: '42',
      suggestion_event_id: 'display-1',
    })
    configureExperienceBehavior({
      ai_feedback_enabled: true,
      behavior_capture_enabled: true,
      micro_question_enabled: false,
      behavior_sample_rate: 1,
      enabled_surfaces: ['task_detail'],
    })
    await Promise.resolve()
    await Promise.resolve()
    await flushExperienceBehaviorQueue()

    expect(clientConfigMock).toHaveBeenCalledTimes(1)
    expect(behaviorEventsMock).toHaveBeenCalledWith({
      events: [
        expect.objectContaining({
          action: 'visible',
          surface: 'task_detail',
          target_type: 'task',
          target_id: '42',
          suggestion_event_id: 'display-1',
        }),
      ],
    })
  })
})
