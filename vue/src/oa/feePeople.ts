export type FeePerson = { id: number; name: string; employee_no?: number | null; department?: string }

export function approverLabel(person: FeePerson) {
  const number = person.employee_no == null ? '' : String(person.employee_no).padStart(4, '0')
  return [person.name, person.department, number].filter(Boolean).join(' · ')
}
