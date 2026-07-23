// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'

const mocks = vi.hoisted(() => ({ revisions: vi.fn(), fetchPreview: vi.fn(), materialize: vi.fn(), revoke: vi.fn(), download: vi.fn() }))
vi.mock('@/services/api/resourceGroupsApi', async (loadOriginal) => ({
  ...(await loadOriginal<typeof import('@/services/api/resourceGroupsApi')>()),
  resourceGroupsApi: { revisions: mocks.revisions },
}))
vi.mock('@/domain/asset-access', () => ({ fetchTaskAssetPreviewMeta: mocks.fetchPreview }))
vi.mock('@/domain/asset-preview-image', () => ({ materializePreviewImageUrl: mocks.materialize, revokeMaterializedPreviewImage: mocks.revoke }))
vi.mock('@/utils/assetFileDownload', () => ({ downloadAssetFileWithOriginalFilename: mocks.download }))

import ResourceRevisionDrawer from './ResourceRevisionDrawer.vue'

const group = {
  id: 8, task_id: 9, task_no: 'RW-009', scope_kind: 'sku' as const, sku_code: 'SKU-009',
  working_revision_id: 72, finalized_revision_id: 71, lock_version: 1, migration_incomplete: false,
}

describe('ResourceRevisionDrawer', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.spyOn(window, 'open').mockImplementation(() => null)
    mocks.fetchPreview.mockResolvedValue({ status: 'ok', displayUrl: '/v1/assets/files/preview' })
    mocks.materialize.mockResolvedValue({ displaySrc: 'blob:preview' })
    mocks.download.mockResolvedValue({ ok: true })
    mocks.revisions.mockResolvedValue({
      items: [{
        id: 71, group_id: 8, revision_no: 3, status: 'finalized', mode: 'set', source_stage: 'audit', created_by: 7, created_by_name: '审核员',
        reason: '审核确认 [migration_v2 metadata]', legacy_migration: true, created_at: '2026-07-22T08:00:00Z',
        evidence_summary: {
          schema_version: 'migration_v2', manifest_sha256: 'a'.repeat(64), confidence: 'confirmed_auto', confirmed_by: 7,
          confirmed_at: '2026-07-22T08:00:00Z', evidence_event_ids: ['task_event_log:event-1'], upload_session_ids: [], upload_sessions_known: false,
          business_reason: '审核确认',
        },
        source_file: { task_asset_id: 100, file_name: 'source.psd', preview_url: '/v1/task-assets/100/preview' },
        references: [{ id: 4, reference_file_ref_id: 5, formal_task_asset_id: 102, file_name: 'direction.jpg', preview_url: '/v1/task-assets/102/preview' }],
        items: [{ id: 1, revision_id: 71, task_asset_id: 101, sort_order: 0, file: { task_asset_id: 101, file_name: 'final.png', preview_url: '/v1/task-assets/101/preview' } }],
      }],
      working_revision_id: 72, finalized_revision_id: 71, page: 1, page_size: 20, total: 21,
    })
  })

  it('loads lazily, renders safe preview-only links, and pages', async () => {
    const wrapper = mount(ResourceRevisionDrawer, { props: { group }, global: { stubs: { Teleport: true } } })
    await flushPromises()
    expect(mocks.revisions).toHaveBeenCalledWith(8, { page: 1, page_size: 20 })
    expect(wrapper.text()).toContain('第 3 版')
    expect(wrapper.text()).toContain('当前最终版')
    expect(wrapper.text()).toContain('审核员')
    expect(wrapper.text()).toContain('历史迁移证据')
    expect(wrapper.text()).toContain('旧证据未记录')
    expect(wrapper.text()).not.toContain('[migration_v2 metadata]')
    expect(wrapper.find('a').exists()).toBe(false)
    expect(wrapper.find('img').exists()).toBe(false)
    const preview = wrapper.findAll('button').find((button) => button.text() === '预览')
    expect(preview).toBeTruthy()
    await preview!.trigger('click')
    await flushPromises()
    expect(mocks.fetchPreview).toHaveBeenCalledWith('100')
    expect(mocks.materialize).toHaveBeenCalledWith('/v1/assets/files/preview', 100)

    mocks.revisions.mockResolvedValueOnce({ items: [], page: 2, page_size: 20, total: 21 })
    await wrapper.get('[aria-label="下一页历史修订"]').trigger('click')
    await flushPromises()
    expect(mocks.revisions).toHaveBeenLastCalledWith(8, { page: 2, page_size: 20 })
    wrapper.unmount()
  })

  it('traps focus, closes with Escape, and restores the trigger focus on unmount', async () => {
    const trigger = document.createElement('button')
    trigger.textContent = '历史修订'
    document.body.appendChild(trigger)
    trigger.focus()
    const wrapper = mount(ResourceRevisionDrawer, { props: { group }, attachTo: document.body })
    await flushPromises()
    await nextTick()
    expect(document.activeElement?.getAttribute('aria-label')).toBe('关闭历史修订')

    const close = document.querySelector<HTMLButtonElement>('[aria-label="关闭历史修订"]')!
    close.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', shiftKey: true, bubbles: true, cancelable: true }))
    await nextTick()
    expect(document.activeElement?.getAttribute('aria-label')).toBe('下一页历史修订')
    document.querySelector<HTMLButtonElement>('[aria-label="下一页历史修订"]')!.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', bubbles: true, cancelable: true }))
    await nextTick()
    expect(document.activeElement?.getAttribute('aria-label')).toBe('关闭历史修订')
    close.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true }))
    await nextTick()
    expect(wrapper.emitted('close')).toHaveLength(1)
    wrapper.unmount()
    expect(document.activeElement).toBe(trigger)
    trigger.remove()
  })

  it('does not expose malformed migration metadata in the business reason', async () => {
    mocks.revisions.mockResolvedValueOnce({
      items: [{
        id: 72, group_id: 8, revision_no: 2, status: 'finalized', mode: 'single', source_stage: 'migration',
        created_by: 7, reason: 'Legacy import [migration_v2 bad]', legacy_migration: true,
        created_at: '2026-07-22T08:00:00Z', references: [], items: [],
      }],
      working_revision_id: 72, finalized_revision_id: 72, page: 1, page_size: 20, total: 1,
    })
    const wrapper = mount(ResourceRevisionDrawer, { props: { group }, global: { stubs: { Teleport: true } } })
    await flushPromises()
    expect(wrapper.text()).toContain('Legacy import')
    expect(wrapper.text()).not.toContain('[migration_v2 bad]')
    expect(wrapper.find('.evidence-warning').exists()).toBe(true)
    wrapper.unmount()
  })

  it('shows historical missing metadata without preview or download actions', async () => {
    mocks.revisions.mockResolvedValueOnce({
      items: [{
        id: 73, group_id: 8, revision_no: 1, status: 'superseded', mode: 'single', source_stage: 'migration',
        created_by: 1, legacy_migration: true, created_at: '2026-07-22T08:00:00Z',
        references: [{
          id: 10,
          formal_task_asset_id: 12324,
          file_name: 'lost-reference.png',
          availability: 'historical_unavailable',
          unavailable_reason: 'legacy_original_object_missing',
        }],
        source_file: {
          task_asset_id: 12323,
          file_name: 'lost.psd',
          availability: 'historical_unavailable',
          unavailable_reason: 'legacy_original_object_missing',
        },
        items: [],
      }],
      page: 1, page_size: 20, total: 1,
    })
    const wrapper = mount(ResourceRevisionDrawer, { props: { group }, global: { stubs: { Teleport: true } } })
    await flushPromises()
    expect(wrapper.text()).toContain('历史文件不可用（原始对象已缺失）')
    expect(wrapper.findAll('button').some((button) => ['预览', '下载'].includes(button.text()))).toBe(false)
    wrapper.unmount()
  })
})
