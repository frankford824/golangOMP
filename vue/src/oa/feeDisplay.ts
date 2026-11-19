const states: Record<string, string> = {
  waiting: '待审批', pending: '待审批', approved: '已批待付',
  returned: '已退回', rejected: '已拒绝', paid: '已登记付款', withdrawn: '已撤回', voided: '已作废',
}

export const feeStatus = (state: string) => states[state] || '状态更新'

export function feeDateTime(value: string) {
  const date = new Date(/[zZ]$|[+-]\d{2}:\d{2}$/.test(value) ? value : value.replace(' ', 'T') + '+08:00')
  if (Number.isNaN(date.getTime())) return '—'
  return new Intl.DateTimeFormat('sv-SE', {
    timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit',
  }).format(date)
}

export function feeApplicant(doc: {
  applicant_id: string | number
  applicant_name?: string
  applicant_employee_no?: string | number
  department_name?: string
  team_name?: string
}) {
  const name = doc.applicant_name?.trim() || '#' + doc.applicant_id
  const organization = [doc.department_name, doc.team_name].filter(Boolean).join(' / ')
  const display = organization ? name + ' · ' + organization : name
  return doc.applicant_employee_no != null && doc.applicant_employee_no !== ''
    ? display + '（' + doc.applicant_employee_no + '）'
    : display
}
