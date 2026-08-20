import { describe, expect, it } from 'vitest'

import { buildSelfSupplementPayload, duplicateSupplementFileNames, filterSupplementImageFiles } from './supplementUpload'

describe('asset workbench self supplement upload', () => {
  it('keeps folder images and reports archives, empty files, and other documents as ignored', () => {
    const imageWithoutMime = new File(['image'], '子目录海报.PNG')
    const imageWithMime = new File(['image'], '封面', { type: 'image/jpeg' })
    const archive = new File(['archive'], '历史素材.rar', { type: 'application/vnd.rar' })
    const empty = new File([], '空图.jpg', { type: 'image/jpeg' })

    expect(filterSupplementImageFiles([imageWithoutMime, imageWithMime, archive, empty])).toEqual({
      files: [imageWithoutMime, imageWithMime],
      ignored: 2,
    })
  })

  it('warns for names already uploaded and duplicates in the new selection', () => {
    const files = [{ name: '海报.jpg' }, { name: '挂布.png' }, { name: '挂布.png' }] as File[]
    expect(duplicateSupplementFileNames(files, [{
      id: 1,
      payee_user_id: 1001,
      business_month: '2026-07',
      status: 'approved',
      order_no: '海报.jpg',
      difficulty_class: 'A',
      finalized: true,
      page_count: 1,
      gross_amount: 12,
    }])).toEqual(['海报.jpg', '挂布.png'])
  })

  it('derives payee, settlement month, and price category from permission and upload directory', () => {
    expect(buildSelfSupplementPayload(
      { name: '海报.jpg' } as File,
      'session-1',
      '2026-06-15',
      { id: 1, payee_user_id: 1001, business_month: '2026-07', enabled: true, reason: '', granted_by: 9, granted_at: '' },
      { id: 8, name: 'A类成品', oss_prefix: 'a', description: '', difficulty_class: 'A', allowed_file_types: ['jpg'], enabled: true, sort_order: 1, created_by: 9 },
    )).toEqual(expect.objectContaining({
      payee_user_id: 1001,
      business_month: '2026-07',
      order_no: '海报.jpg',
      supplement_date: '2026-06-15',
      difficulty_class: 'A',
      gross_amount: 0,
      upload_session_ids: ['session-1'],
    }))
  })
})
