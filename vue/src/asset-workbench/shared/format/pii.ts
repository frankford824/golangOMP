const empty = '—'

export function maskPhone(value?: string | null) {
  if (!value) return empty
  const compact = value.replace(/\s+/g, '')
  if (compact.length < 7) return compact
  return `${compact.slice(0, 3)}****${compact.slice(-4)}`
}

export function maskIdCard(value?: string | null) {
  if (!value) return empty
  const compact = value.replace(/\s+/g, '')
  if (compact.length < 8) return compact
  return `${compact.slice(0, 3)}***********${compact.slice(-4)}`
}

export function maskAlipay(value?: string | null) {
  if (!value) return empty
  const compact = value.trim()
  if (compact.includes('@')) {
    const [name, domain] = compact.split('@')
    return `${name.slice(0, 2)}***@${domain}`
  }
  if (compact.length < 6) return compact
  return `${compact.slice(0, 2)}***${compact.slice(-2)}`
}
