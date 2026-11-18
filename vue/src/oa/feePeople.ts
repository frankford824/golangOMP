import { send } from './api'

export type FeePerson = { id: number; name: string; employee_no?: number | null; department?: string }

export function approverLabel(person: FeePerson) {
  const number = person.employee_no == null ? '' : String(person.employee_no).padStart(4, '0')
  return [person.name, person.department, number].filter(Boolean).join(' · ')
}

export function searchFeePeople(q: string, ids: string[] = [], afterId = 0) {
  return send<{ items: FeePerson[]; selected?: FeePerson[]; next_after_id?: number }>('/oa-api/finance/approvers/search', { q: q.trim(), ids: ids.join(','), after_id: afterId })
}
