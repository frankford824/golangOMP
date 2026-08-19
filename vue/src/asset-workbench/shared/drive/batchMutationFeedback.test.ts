import { describe, expect, it } from 'vitest'

import { batchMutationFailureMessage, hasSupplementRecordFailure } from './batchMutationFeedback'

describe('batchMutationFailureMessage', () => {
  it('turns priced-work selection failures into actionable Chinese copy', () => {
    expect(
      batchMutationFailureMessage('移动', [
        { file_id: 1, reason: 'All files in one priced work must be selected together.' },
        { file_id: 2, reason: 'All files in one priced work must be selected together.' },
      ]),
    ).toBe('移动未完成：该作品由文件夹内的多个文件组成，请勾选整个文件夹作品后再移动或删除，另有 1 个文件同样未处理')
  })

  it('turns settlement locks into an explicit next action', () => {
    expect(
      batchMutationFailureMessage('删除', [
        { file_id: 1, reason: 'Submission item cannot be changed after settlement batch attachment.' },
      ]),
    ).toBe('删除未完成：当前作品已关联结算批次，不能移动或删除。待确认批次请先取消；已确认批次需由超级管理员执行受控撤销确认。')
  })

  it('identifies supplement files that must be deleted from their supplement record', () => {
    const failures = [
      { file_id: 1, reason: 'Supplement upload files must be managed through their supplement record.' },
      { file_id: 2, reason: 'Supplement upload files must be managed through their supplement record.' },
    ]
    expect(hasSupplementRecordFailure(failures)).toBe(true)
    expect(batchMutationFailureMessage('删除', failures)).toBe(
      '删除未完成：补录文件不能在上传台账中单独删除，请进入对应补录记录执行删除，系统会同步移除文件和未结算金额，另有 1 个文件同样未处理',
    )
  })
})
