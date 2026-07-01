// @vitest-environment jsdom
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { experienceApi } from '@/services/api/experienceApi'
import {
  flushExperienceBehaviorQueue,
  recordExperienceBehavior,
  setExperienceBehaviorSampleRate,
} from '@/services/experienceBehavior'

vi.mock('@/services/api/experienceApi', () => ({
  experienceApi: {
    behaviorEvents: vi.fn(),
  },
}))

vi.mock('@/services/http', () => ({
  getToken: vi.fn(() => ''),
}))

const behaviorEventsMock = vi.mocked(experienceApi.behaviorEvents)

describe('experienceBehavior', () => {
  beforeEach(async () => {
    vi.clearAllMocks()
    setExperienceBehaviorSampleRate(1)
    await flushExperienceBehaviorQueue()
  })

  it('drops behavior events when sample rate is zero', async () => {
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
})
