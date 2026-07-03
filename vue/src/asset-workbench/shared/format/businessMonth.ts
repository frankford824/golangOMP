export function currentBusinessMonth(date = new Date(), timeZone = 'Asia/Shanghai') {
  const parts = new Intl.DateTimeFormat('en-CA', {
    timeZone,
    year: 'numeric',
    month: 'numeric',
  }).formatToParts(date)
  const year = parts.find((part) => part.type === 'year')?.value || date.getFullYear().toString()
  const month = parts.find((part) => part.type === 'month')?.value || String(date.getMonth() + 1)
  return `${year}-${month.padStart(2, '0')}`
}
