import { describe, it, expect, vi, beforeEach } from 'vitest'
import { parseOssDirectPlan } from '../ossDirectUpload'

// ---------------------------------------------------------------------------
// parseOssDirectPlan — pure function tests for transport detection
// ---------------------------------------------------------------------------

describe('parseOssDirectPlan', () => {
  it('fx1: oss_direct present -> returns ossDirect, remote null', () => {
    const body = {
      data: {
        session: { id: 'sess-001' },
        oss_direct: {
          upload_strategy: 'single_part',
          upload_url: 'https://oss.example.com/put?signed=1',
          method: 'PUT',
          required_upload_content_type: 'image/png',
          object_key: 'tasks/xxx/v1/delivery/1776679600774626288_09e8cd03.png',
        },
      },
    }
    const result = parseOssDirectPlan(body)
    expect(result.sessionId).toBe('sess-001')
    expect(result.ossDirect).not.toBeNull()
    expect(result.ossDirect!.upload_url).toBe('https://oss.example.com/put?signed=1')
    expect(result.ossDirect!.upload_strategy).toBe('single_part')
    expect(result.remote).toBeNull()
  })

  it('fx2: oss_direct absent, remote present -> returns ossDirect null, remote populated', () => {
    const body = {
      data: {
        session: { id: 'sess-002', expected_size: 12345 },
        remote: {
          upload_url: 'https://upload-proxy.internal/presigned/abc',
          required_upload_content_type: 'image/jpeg',
          method: 'PUT',
        },
        complete_endpoint: '/v1/assets/upload-sessions/sess-002/complete',
      },
    }
    const result = parseOssDirectPlan(body)
    expect(result.sessionId).toBe('sess-002')
    expect(result.ossDirect).toBeNull()
    expect(result.remote).not.toBeNull()
    expect(result.remote!.upload_url).toBe('https://upload-proxy.internal/presigned/abc')
    expect(result.remote!.required_upload_content_type).toBe('image/jpeg')
    expect(result.completeEndpoint).toBe('/v1/assets/upload-sessions/sess-002/complete')
    expect(result.expectedSize).toBe(12345)
  })

  it('fx3: both oss_direct and remote absent -> returns both null', () => {
    const body = {
      data: {
        session: { id: 'sess-003' },
      },
    }
    const result = parseOssDirectPlan(body)
    expect(result.sessionId).toBe('sess-003')
    expect(result.ossDirect).toBeNull()
    expect(result.remote).toBeNull()
  })

  it('handles remote with missing upload_url as null', () => {
    const body = {
      data: {
        session: { id: 'sess-004' },
        remote: { method: 'PUT' },
      },
    }
    const result = parseOssDirectPlan(body)
    expect(result.remote).toBeNull()
  })
})

// ---------------------------------------------------------------------------
// prepareTaskAssetUploadSession — integration test with mocked API
// ---------------------------------------------------------------------------

vi.mock('@/services/api/assetsApi', () => ({
  assetsApi: {
    createAssetUploadSession: vi.fn(),
    completeAssetUploadSession: vi.fn(),
    completeAssetUploadSessionAtEndpoint: vi.fn(),
    cancelAssetUploadSession: vi.fn(),
    uploadToRemoteUrl: vi.fn(),
  },
  normalizeAssetCenterCompleteData: vi.fn((d: unknown) => d),
  assertAssetCenterUploadCompleteOk: vi.fn(),
}))

vi.mock('@/services/api/taskAssetsApi', () => ({
  taskAssetsApi: {
    createTaskCreateUploadSession: vi.fn(),
    completeTaskCreateUploadSession: vi.fn(),
    abortTaskCreateUploadSession: vi.fn(),
  },
}))

vi.mock('@/utils/mime', () => ({
  resolveFileMimeType: vi.fn(() => 'application/octet-stream'),
}))

vi.mock('@/utils/upload-errors', () => ({
  formatUploadFailureMessage: vi.fn(
    (phase: string, _err: unknown) => `upload failed at ${phase}`,
  ),
}))

import {
  normalizeUploadSessionNumericID,
  normalizeRetouchRequirementId,
  prepareTaskAssetUploadSession,
  completePreparedTaskAssetUploadSession,
  cancelPreparedTaskAssetUploadSession,
  uploadTaskFileViaAssetSession,
} from '../assetUploadFlow'
import { assetsApi } from '@/services/api/assetsApi'
import { taskAssetsApi } from '@/services/api/taskAssetsApi'

describe('normalizeUploadSessionNumericID', () => {
  it('converts positive numeric strings to JSON numbers', () => {
    expect(normalizeUploadSessionNumericID('4477', 'asset_id')).toBe(4477)
    expect(normalizeUploadSessionNumericID(4477, 'asset_id')).toBe(4477)
    expect(normalizeUploadSessionNumericID(undefined, 'asset_id')).toBeUndefined()
    expect(normalizeUploadSessionNumericID('', 'asset_id')).toBeUndefined()
  })

  it('rejects UUID and non-integer asset identifiers before create-session', () => {
    expect(() =>
      normalizeUploadSessionNumericID('0ec54522-98fb-4d66-a7af-74cafb59e088', 'asset_id'),
    ).toThrow('asset_id 必须是有效的数字资产 ID')
    expect(() => normalizeUploadSessionNumericID('4477.1', 'asset_id')).toThrow(
      'asset_id 必须是有效的数字资产 ID',
    )
    expect(() => normalizeUploadSessionNumericID(0, 'asset_id')).toThrow(
      'asset_id 必须是有效的数字资产 ID',
    )
  })
})

describe('normalizeRetouchRequirementId', () => {
  it('accepts positive integers only', () => {
    expect(normalizeRetouchRequirementId(42)).toBe(42)
    expect(normalizeRetouchRequirementId(42.9)).toBe(42)
    expect(normalizeRetouchRequirementId(0)).toBeUndefined()
    expect(normalizeRetouchRequirementId(-1)).toBeUndefined()
    expect(normalizeRetouchRequirementId(undefined)).toBeUndefined()
  })
})

function fakeFile(name = 'test.png', size = 1024): File {
  const blob = new Blob([new ArrayBuffer(size)], { type: 'image/png' })
  return new File([blob], name, { type: 'image/png' })
}

describe('prepareTaskAssetUploadSession — transport fallback', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('fx1: oss_direct present -> ossDirect populated, no warning', async () => {
    const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
    vi.mocked(assetsApi.createAssetUploadSession).mockResolvedValue({
      data: {
        data: {
          session: { id: 'sess-oss' },
          oss_direct: {
            upload_strategy: 'single_part',
            upload_url: 'https://oss.example.com/upload',
            method: 'PUT',
            required_upload_content_type: 'image/png',
          },
        },
      },
    } as never)

    const result = await prepareTaskAssetUploadSession(
      'task-1',
      fakeFile(),
      { asset_kind: 'delivery', remark: 'test' },
    )
    expect(result.ossDirect).toBeDefined()
    expect(result.remote).toBeUndefined()
    expect(warnSpy).not.toHaveBeenCalled()
    warnSpy.mockRestore()
  })

  it('fx2: oss_direct absent, remote present -> falls back to remote with console.warn', async () => {
    const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
    vi.mocked(assetsApi.createAssetUploadSession).mockResolvedValue({
      data: {
        data: {
          session: { id: 'sess-remote' },
          remote: {
            upload_url: 'https://proxy.internal/presigned/xyz',
            required_upload_content_type: 'image/png',
          },
          complete_endpoint: '/v1/assets/upload-sessions/sess-remote/complete',
        },
      },
    } as never)

    const result = await prepareTaskAssetUploadSession(
      'task-2',
      fakeFile(),
      { asset_kind: 'delivery', remark: 'test' },
    )
    expect(result.ossDirect).toBeUndefined()
    expect(result.remote).toBeDefined()
    expect(result.remote!.upload_url).toBe('https://proxy.internal/presigned/xyz')
    expect(result.completeEndpoint).toBe('/v1/assets/upload-sessions/sess-remote/complete')
    expect(warnSpy).toHaveBeenCalledWith(
      '[upload] oss_direct absent, falling back to remote',
      expect.objectContaining({ session_id: 'sess-remote' }),
    )
    warnSpy.mockRestore()
  })

  it('writes retouch_requirement_id when retouchRequirementId option is set', async () => {
    vi.spyOn(console, 'warn').mockImplementation(() => {})
    vi.mocked(assetsApi.createAssetUploadSession).mockResolvedValue({
      data: {
        data: {
          session: { id: 'sess-retouch' },
          oss_direct: {
            upload_strategy: 'single_part',
            upload_url: 'https://oss.example.com/upload',
          },
        },
      },
    } as never)

    await prepareTaskAssetUploadSession(
      '905',
      fakeFile(),
      { asset_kind: 'source', remark: 'material' },
      { retouchRequirementId: 12 },
    )

    expect(assetsApi.createAssetUploadSession).toHaveBeenCalledWith(
      expect.objectContaining({
        retouch_requirement_id: 12,
        asset_kind: 'source',
      }),
      undefined,
    )
  })

  it('keeps target_sku_code when retouchRequirementId is also set', async () => {
    vi.spyOn(console, 'warn').mockImplementation(() => {})
    vi.mocked(assetsApi.createAssetUploadSession).mockResolvedValue({
      data: {
        data: {
          session: { id: 'sess-batch' },
          oss_direct: {
            upload_strategy: 'single_part',
            upload_url: 'https://oss.example.com/upload',
          },
        },
      },
    } as never)

    await prepareTaskAssetUploadSession(
      '100',
      fakeFile(),
      { asset_kind: 'delivery', remark: 'batch', target_sku_code: 'SKU-001' },
      { retouchRequirementId: 7 },
    )

    expect(assetsApi.createAssetUploadSession).toHaveBeenCalledWith(
      expect.objectContaining({
        retouch_requirement_id: 7,
        target_sku_code: 'SKU-001',
      }),
      undefined,
    )
  })

  it('passes replace metadata when replacing an existing reference asset', async () => {
    vi.spyOn(console, 'warn').mockImplementation(() => {})
    vi.mocked(assetsApi.createAssetUploadSession).mockResolvedValue({
      data: {
        data: {
          session: { id: 'sess-reference-replace' },
          oss_direct: {
            upload_strategy: 'single_part',
            upload_url: 'https://oss.example.com/upload',
          },
        },
      },
    } as never)

    await prepareTaskAssetUploadSession(
      '1093',
      fakeFile('new-ref.png'),
      {
        asset_kind: 'reference',
        asset_id: '4198',
        owner_module_key: 'basic_info',
        upload_policy: 'replace',
        remark: 'new-ref.png',
      },
    )

    expect(assetsApi.createAssetUploadSession).toHaveBeenCalledWith(
      expect.objectContaining({
        task_id: 1093,
        asset_kind: 'reference',
        asset_id: 4198,
        owner_module_key: 'basic_info',
        upload_policy: 'replace',
      }),
      undefined,
    )
  })

  it('does not call create-session when replacement asset_id is a legacy UUID ref', async () => {
    await expect(
      prepareTaskAssetUploadSession(
        '1093',
        fakeFile('new-ref.png'),
        {
          asset_kind: 'reference',
          asset_id: '0ec54522-98fb-4d66-a7af-74cafb59e088',
          owner_module_key: 'basic_info',
          upload_policy: 'replace',
          remark: 'new-ref.png',
        },
      ),
    ).rejects.toThrow('asset_id 必须是有效的数字资产 ID')

    expect(assetsApi.createAssetUploadSession).not.toHaveBeenCalled()
  })

  it('fx3: both absent -> throws business copy without technical error code', async () => {
    vi.mocked(assetsApi.createAssetUploadSession).mockResolvedValue({
      data: {
        data: {
          session: { id: 'sess-empty' },
        },
      },
    } as never)

    await expect(
      prepareTaskAssetUploadSession(
        'task-3',
        fakeFile(),
        { asset_kind: 'delivery', remark: 'test' },
      ),
    ).rejects.toThrow('上传入口未准备好')
  })
})

describe('completePreparedTaskAssetUploadSession — remote transport', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('remote transport: uploads via uploadToRemoteUrl then completes', async () => {
    vi.mocked(assetsApi.uploadToRemoteUrl).mockResolvedValue({} as never)
    vi.mocked(assetsApi.completeAssetUploadSession).mockResolvedValue({
      data: {
        data: {
          session: { id: 'sess-r', session_status: 'completed', upload_status: 'uploaded' },
          asset: { id: 'a1', file_role: 'delivery' },
        },
      },
    } as never)

    const prepared = {
      sessionId: 'sess-r',
      remote: {
        upload_url: 'https://proxy.internal/upload',
        required_upload_content_type: 'image/png',
      },
      assetKind: 'delivery' as const,
      remark: 'test',
      sessionMime: 'image/png',
    }
    await completePreparedTaskAssetUploadSession(prepared, fakeFile())

    expect(assetsApi.uploadToRemoteUrl).toHaveBeenCalledWith(
      'https://proxy.internal/upload',
      expect.any(File),
      expect.objectContaining({
        method: 'PUT',
        extraHeaders: { 'Content-Type': 'image/png' },
      }),
    )
    expect(assetsApi.completeAssetUploadSession).toHaveBeenCalledWith(
      'sess-r',
      expect.objectContaining({ upload_content_type: 'image/png' }),
      undefined,
    )
  })

  it('remote transport: completes with returned endpoint when present', async () => {
    vi.mocked(assetsApi.uploadToRemoteUrl).mockResolvedValue({} as never)
    vi.mocked(assetsApi.completeAssetUploadSessionAtEndpoint).mockResolvedValue({
      data: {
        data: {
          session: { id: 'sess-supp', session_status: 'completed', upload_status: 'uploaded' },
          asset: { id: 'a2', file_role: 'delivery' },
        },
      },
    } as never)

    const prepared = {
      sessionId: 'sess-supp',
      remote: {
        upload_url: 'https://proxy.internal/supplement',
        required_upload_content_type: 'image/png',
      },
      assetKind: 'delivery' as const,
      remark: '漏传补传',
      sessionMime: 'image/png',
      completeEndpoint: '/v1/tasks/789/audit-supplements/upload-sessions/sess-supp/complete',
    }
    await completePreparedTaskAssetUploadSession(prepared, fakeFile())

    expect(assetsApi.completeAssetUploadSessionAtEndpoint).toHaveBeenCalledWith(
      '/v1/tasks/789/audit-supplements/upload-sessions/sess-supp/complete',
      expect.objectContaining({ upload_content_type: 'image/png' }),
      undefined,
    )
    expect(assetsApi.completeAssetUploadSession).not.toHaveBeenCalled()
  })

  it('uses an independent request to cancel a session after the upload signal aborts', async () => {
    const controller = new AbortController()
    controller.abort()
    vi.mocked(assetsApi.uploadToRemoteUrl).mockRejectedValue(
      new DOMException('Aborted', 'AbortError'),
    )
    vi.mocked(assetsApi.cancelAssetUploadSession).mockResolvedValue({} as never)

    const prepared = {
      sessionId: 'sess-aborted',
      taskId: '123',
      remote: {
        upload_url: 'https://proxy.internal/upload',
        required_upload_content_type: 'image/png',
      },
      assetKind: 'delivery' as const,
      remark: 'test',
      sessionMime: 'image/png',
    }

    await expect(
      completePreparedTaskAssetUploadSession(prepared, fakeFile(), {
        signal: controller.signal,
      }),
    ).rejects.toThrow('upload failed at part_upload')

    expect(assetsApi.cancelAssetUploadSession).toHaveBeenCalledWith('sess-aborted', {})
  })

  it('uses the canonical pre-task abort route and forwards OSS cleanup identifiers', async () => {
    vi.mocked(taskAssetsApi.abortTaskCreateUploadSession).mockResolvedValue({} as never)

    await cancelPreparedTaskAssetUploadSession(
      'sess-pretask',
      undefined,
      undefined,
      {
        mode: 'multipart',
        object_key: 'tasks/task-create-reference/upload-sessions/sess-pretask/sess-pretask.png',
        upload_id: 'oss-upload-1',
      },
    )

    expect(taskAssetsApi.abortTaskCreateUploadSession).toHaveBeenCalledWith(
      'sess-pretask',
      {
        oss_object_key: 'tasks/task-create-reference/upload-sessions/sess-pretask/sess-pretask.png',
        oss_upload_id: 'oss-upload-1',
      },
    )
    expect(assetsApi.cancelAssetUploadSession).not.toHaveBeenCalled()
  })

  it('throws when both ossDirect and remote are absent', async () => {
    const prepared = {
      sessionId: 'sess-none',
      assetKind: 'delivery' as const,
      remark: 'test',
    }
    await expect(
      completePreparedTaskAssetUploadSession(prepared, fakeFile()),
    ).rejects.toThrow('上传入口未准备好')
  })
})

describe('uploadTaskFileViaAssetSession — replacement race recovery', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('cancels the losing session, recreates it once, and completes the replacement', async () => {
    vi.spyOn(console, 'warn').mockImplementation(() => {})
    vi.mocked(assetsApi.createAssetUploadSession)
      .mockResolvedValueOnce({
        data: {
          data: {
            session: { id: 'sess-race-1' },
            remote: { upload_url: 'https://proxy.internal/race-1', required_upload_content_type: 'image/png' },
          },
        },
      } as never)
      .mockResolvedValueOnce({
        data: {
          data: {
            session: { id: 'sess-race-2' },
            remote: { upload_url: 'https://proxy.internal/race-2', required_upload_content_type: 'image/png' },
          },
        },
      } as never)
    vi.mocked(assetsApi.uploadToRemoteUrl).mockResolvedValue({} as never)
    vi.mocked(assetsApi.cancelAssetUploadSession).mockResolvedValue({} as never)
    vi.mocked(assetsApi.completeAssetUploadSession)
      .mockRejectedValueOnce(Object.assign(new Error('资产版本发生并发更新，请刷新后重试'), {
        status: 409,
        code: 'CONFLICT',
        denyCode: 'asset_version_race_retry',
        responseData: {
          error: {
            code: 'CONFLICT',
            details: { deny_code: 'asset_version_race_retry' },
          },
        },
      }))
      .mockResolvedValueOnce({
        data: {
          data: {
            session: { id: 'sess-race-2', session_status: 'completed', upload_status: 'uploaded' },
            asset: { id: '12401', file_role: 'delivery' },
          },
        },
      } as never)

    await uploadTaskFileViaAssetSession(
      '2199',
      fakeFile('replacement.png'),
      { asset_kind: 'delivery', asset_id: 12401, remark: 'replace current resource' },
    )

    expect(assetsApi.createAssetUploadSession).toHaveBeenCalledTimes(2)
    expect(assetsApi.uploadToRemoteUrl).toHaveBeenCalledTimes(2)
    expect(assetsApi.completeAssetUploadSession).toHaveBeenCalledTimes(2)
    expect(assetsApi.cancelAssetUploadSession).toHaveBeenCalledWith('sess-race-1', {})
  })
})
