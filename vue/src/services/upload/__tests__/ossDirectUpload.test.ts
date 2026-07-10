import axios from 'axios'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { runOssDirectUploadPlan } from '../ossDirectUpload'

vi.mock('axios', () => ({
  default: {
    request: vi.fn(),
    isAxiosError: (value: unknown) =>
      Boolean(value && typeof value === 'object' && 'isAxiosError' in value),
  },
}))

function fakeFile(name = 'material.psd', size = 12): File {
  const blob = new Blob([new ArrayBuffer(size)], { type: 'image/vnd.adobe.photoshop' })
  return new File([blob], name, { type: 'image/vnd.adobe.photoshop' })
}

describe('runOssDirectUploadPlan', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('throws business copy when multipart PUT does not expose the uploaded part confirmation', async () => {
    vi.mocked(axios.request).mockResolvedValue({
      headers: {},
    } as never)

    await expect(
      runOssDirectUploadPlan(
        fakeFile(),
        {
          upload_strategy: 'multipart',
          method: 'PUT',
          required_upload_content_type: 'image/vnd.adobe.photoshop',
          object_key: 'tasks/RW/assets/A/v1/source/material.psd',
          upload_id: 'oss-upload-1',
          part_size_hint: 10,
          part_urls: ['https://oss.example.com/part1', 'https://oss.example.com/part2'],
        },
        { mimeTypeForUpload: 'image/vnd.adobe.photoshop' },
      ),
    ).rejects.toThrow('浏览器无法确认上传结果')

    expect(axios.request).toHaveBeenCalledTimes(2)
  })

  it('reads ETag through AxiosHeaders-style get()', async () => {
    vi.mocked(axios.request).mockResolvedValue({
      headers: {
        get: (name: string) => (name.toLowerCase() === 'etag' ? '"etag-1"' : undefined),
      },
    } as never)

    const result = await runOssDirectUploadPlan(
      fakeFile('material.png', 5),
      {
        upload_strategy: 'multipart',
        method: 'PUT',
        required_upload_content_type: 'image/png',
        object_key: 'tasks/RW/assets/A/v1/source/material.png',
        upload_id: 'oss-upload-2',
        part_size_hint: 10,
        part_urls: ['https://oss.example.com/part1'],
      },
      { mimeTypeForUpload: 'image/png' },
    )

    expect(result.oss_parts).toEqual([{ part_number: 1, etag: 'etag-1' }])
  })

  it('uploads at most four parts concurrently and preserves completion order', async () => {
    let active = 0
    let maxActive = 0
    vi.mocked(axios.request).mockImplementation(async (config) => {
      active += 1
      maxActive = Math.max(maxActive, active)
      await new Promise((resolve) => setTimeout(resolve, 1))
      active -= 1
      const partNo = Number(String(config.url).split('part').pop())
      return { headers: { ETag: `"etag-${partNo}"` } } as never
    })

    const result = await runOssDirectUploadPlan(
      fakeFile('material.psd', 60),
      {
        upload_strategy: 'multipart',
        method: 'PUT',
        part_size_hint: 10,
        part_urls: Array.from({ length: 6 }, (_, index) => `https://oss.example.com/part${index + 1}`),
      },
    )

    expect(maxActive).toBe(4)
    expect(result.oss_parts).toEqual(
      Array.from({ length: 6 }, (_, index) => ({
        part_number: index + 1,
        etag: `etag-${index + 1}`,
      })),
    )
  })

  it('retries transient OSS PUT failures', async () => {
    vi.useFakeTimers()
    vi.spyOn(Math, 'random').mockReturnValue(0)
    vi.mocked(axios.request)
      .mockRejectedValueOnce({ isAxiosError: true, response: { status: 503 } })
      .mockResolvedValueOnce({ headers: { ETag: '"etag-after-retry"' } } as never)

    const pending = runOssDirectUploadPlan(
      fakeFile('material.png', 5),
      {
        upload_strategy: 'multipart',
        method: 'PUT',
        part_size_hint: 10,
        part_urls: ['https://oss.example.com/part1'],
      },
    )
    const expectation = expect(pending).resolves.toMatchObject({
      oss_parts: [{ part_number: 1, etag: 'etag-after-retry' }],
    })
    await vi.runAllTimersAsync()

    await expectation
    expect(axios.request).toHaveBeenCalledTimes(2)
  })
})
