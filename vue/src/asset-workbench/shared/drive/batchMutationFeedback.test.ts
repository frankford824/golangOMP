import { describe, expect, it } from 'vitest'

import { batchMutationFailureMessage } from './batchMutationFeedback'

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
    ).toBe('删除未完成：当前作品已进入待确认的结算批次，暂时不能移动或删除。请先取消该批次后再操作。')
  })
})
