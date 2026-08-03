import { describe, expect, it } from 'vitest'
import { createComposeRow } from '@/domain/unified-task-compose'
import { composeRowIdsFromSelection } from './compose-grid-selection'

describe('composeRowIdsFromSelection', () => {
  it('maps one or more spreadsheet ranges to unique business row ids and skips the header', () => {
    const rows = [
      createComposeRow({ id: 'row-1' }),
      createComposeRow({ id: 'row-2' }),
      createComposeRow({ id: 'row-3' }),
    ]
    expect(composeRowIdsFromSelection(rows, {
      selections: [
        { range: { startRow: 0, endRow: 2 } },
        { range: { startRow: 2, endRow: 3 } },
      ],
    })).toEqual(['row-1', 'row-2', 'row-3'])
  })

  it('accepts the singular selection event shape', () => {
    const rows = [createComposeRow({ id: 'row-1' }), createComposeRow({ id: 'row-2' })]
    expect(composeRowIdsFromSelection(rows, {
      selection: { startRowIndex: 2, endRowIndex: 2 },
    })).toEqual(['row-2'])
  })
})
