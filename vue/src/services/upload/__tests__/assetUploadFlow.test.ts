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
    cancelAssetUploadSession: vi.fn(),
    uploadToRemoteUrl: vi.fn(),
  },
  normalizeAssetCenterCompleteData: vi.fn((d: unknown) => d),
  assertAssetCenterUploadCompleteOk: vi.fn(),
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
  normalizeRetouchRequirementId,
  prepareTaskAssetUploadSession,
  completePreparedTaskAssetUploadSession,
} from '../assetUploadFlow'
import { assetsApi } from '@/services/api/assetsApi'

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

  it('fx3: both absent -> throws user-friendly error with upload_transport_unavailable', async () => {
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
    ).rejects.toThrow('upload_transport_unavailable')
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

  it('throws when both ossDirect and remote are absent', async () => {
    const prepared = {
      sessionId: 'sess-none',
      assetKind: 'delivery' as const,
      remark: 'test',
    }
    await expect(
      completePreparedTaskAssetUploadSession(prepared, fakeFile()),
    ).rejects.toThrow('upload_transport_unavailable')
  })
})
