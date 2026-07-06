import axios from 'axios'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { runOssDirectUploadPlan } from '../ossDirectUpload'

vi.mock('axios', () => ({
  default: {
    request: vi.fn(),
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

    expect(axios.request).toHaveBeenCalledTimes(1)
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
})
