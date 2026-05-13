function cleanText(value: unknown): string {
  return String(value ?? '').trim()
}

export function userAccountDisplay(...candidates: unknown[]): string {
  for (const candidate of candidates) {
    const text = cleanText(candidate)
    if (!text) continue
    if (/^用户\s*#?\d+$/i.test(text)) continue
    if (/^#?\d+$/.test(text)) continue
    if (/^session_actor\s*#?\d+$/i.test(text)) continue
    return text
  }
  return '未知用户'
}

export function userAccountOrEmpty(...candidates: unknown[]): string {
  const text = userAccountDisplay(...candidates)
  return text === '未知用户' ? '' : text
}
