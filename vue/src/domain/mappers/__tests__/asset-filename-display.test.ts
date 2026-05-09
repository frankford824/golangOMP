import { describe, it, expect } from 'vitest'
import { normalizeAssetVersionsFromTaskRaw } from '../asset-versions-from-api'

describe('asset filename display — original_filename over object_key', () => {
  it('displays original_filename when object_key is opaque (non-previewable file)', () => {
    const taskRaw = {
      asset_versions: [
        {
          id: 1,
          asset_kind: 'source',
          original_filename: 'a+b test (1).psd',
          object_key:
            'tasks/RW-20260420-A-000454/assets/AST-0013/v1/delivery/1776679600774626288_09e8cd03.psd',
          download_url: '/v1/assets/1/download',
          uploaded_at: '2026-04-20T12:00:00Z',
          uploaded_by: 100,
          mime_type: 'image/vnd.adobe.photoshop',
        },
      ],
    }
    const versions = normalizeAssetVersionsFromTaskRaw(taskRaw)
    expect(versions.length).toBeGreaterThanOrEqual(1)
    const v = versions[0]!
    expect(v.nonPreviewFiles).toBeDefined()
    expect(v.nonPreviewFiles!.length).toBeGreaterThanOrEqual(1)

    const displayed = v.nonPreviewFiles![0]!.label
    expect(displayed).not.toContain('1776679600774626288')
    expect(displayed).not.toContain('09e8cd03')
    expect(displayed).toBe('a+b test (1).psd')
  })

  it('displays filename field when original_filename absent', () => {
    const taskRaw = {
      asset_versions: [
        {
          id: 2,
          asset_kind: 'source',
          filename: '\u624b\u6dd8_SKU_13.psd',
          object_key: 'tasks/xxx/v1/source/9999_abc123.psd',
          download_url: '/v1/assets/2/download',
          uploaded_at: '2026-04-20T13:00:00Z',
          uploaded_by: 100,
          mime_type: 'image/vnd.adobe.photoshop',
        },
      ],
    }
    const versions = normalizeAssetVersionsFromTaskRaw(taskRaw)
    expect(versions.length).toBeGreaterThanOrEqual(1)
    const v = versions[0]!
    expect(v.nonPreviewFiles).toBeDefined()
    const displayed = v.nonPreviewFiles![0]!.label
    expect(displayed).toBe('\u624b\u6dd8_SKU_13.psd')
    expect(displayed).not.toContain('9999_abc123')
  })

  it('does NOT attempt to parse opaque object_key for display', () => {
    const opaqueKey =
      'tasks/RW-20260420-A-000454/assets/AST-0013/v1/source/1776679600774626288_09e8cd03.psd'
    const taskRaw = {
      asset_versions: [
        {
          id: 3,
          asset_kind: 'source',
          original_filename: 'my-design.psd',
          object_key: opaqueKey,
          download_url: '/v1/assets/3/download',
          uploaded_at: '2026-04-20T14:00:00Z',
          uploaded_by: 100,
          mime_type: 'image/vnd.adobe.photoshop',
        },
      ],
    }
    const versions = normalizeAssetVersionsFromTaskRaw(taskRaw)
    expect(versions.length).toBe(1)
    const v = versions[0]!
    expect(v.nonPreviewFiles).toBeDefined()
    const displayed = v.nonPreviewFiles![0]!.label
    expect(displayed).toBe('my-design.psd')

    const segments = opaqueKey.split('/')
    const lastSegment = segments[segments.length - 1]!
    expect(displayed).not.toBe(lastSegment)
  })

  it('previewable image: fileRefs populated, label derived from original_filename not object_key', () => {
    const taskRaw = {
      asset_versions: [
        {
          id: 4,
          asset_kind: 'delivery',
          original_filename: 'a+b test (1).png',
          object_key:
            'tasks/xxx/v1/delivery/1776679600774626288_09e8cd03.png',
          download_url: '/v1/assets/4/download',
          preview_available: true,
          uploaded_at: '2026-04-20T15:00:00Z',
          uploaded_by: 100,
          mime_type: 'image/png',
        },
      ],
    }
    const versions = normalizeAssetVersionsFromTaskRaw(taskRaw)
    expect(versions.length).toBe(1)
    const v = versions[0]!
    expect(v.fileRefs.length).toBeGreaterThanOrEqual(1)
    const url = v.fileRefs[0]!
    expect(url).not.toContain('1776679600774626288_09e8cd03')
  })

  it('detail-shaped row maps preview only when preview_available and download_url coexist', () => {
    const taskRaw = {
      asset_versions: [
        {
          id: 530,
          asset_id: 682,
          scope_sku_code: 'NSGE000005',
          asset_type: 'delivery',
          original_filename: '毕业手持横幅01(1).jpg',
          mime_type: 'image/jpeg',
          preview_available: true,
          download_url:
            '/v1/assets/files/tasks/RW-20260429-A-000614/assets/AST-0002/v1/delivery/1777450131487836376_fda09133.jpg',
        },
      ],
    }
    const versions = normalizeAssetVersionsFromTaskRaw(taskRaw)
    expect(versions).toHaveLength(1)
    expect(versions[0]!.fileRefs).toEqual([
      '/v1/assets/files/tasks/RW-20260429-A-000614/assets/AST-0002/v1/delivery/1777450131487836376_fda09133.jpg',
    ])
  })
})
