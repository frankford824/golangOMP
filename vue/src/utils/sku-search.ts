// Keep ordinary product names (including spaces) on the keyword-search path.
export function batchSkuCodes(query: string): string[] {
  const terms = query.trim().split(/[\s,，、;；]+/u).filter(Boolean)
  if (terms.length < 2 || !terms.every(term => /^(?=.*\d)[a-z\d_-]+$/i.test(term))) return []
  const codes = [...new Set(terms.map(term => term.toUpperCase()))]
  if (codes.length > 200) throw new Error('每次最多查询 200 个编码，请分批搜索。')
  return codes
}

// A single-line input strips pasted newlines and joins adjacent Excel rows.
export function pasteSkuSearch(event: ClipboardEvent) {
  const text = event.clipboardData?.getData('text/plain')
  const input = event.currentTarget
  if (!text || !/[\r\n]/.test(text) || !(input instanceof HTMLInputElement)) return
  event.preventDefault()
  input.setRangeText(text.replace(/[\r\n]+/g, ' '), input.selectionStart ?? 0, input.selectionEnd ?? 0, 'end')
  input.dispatchEvent(new Event('input', { bubbles: true }))
}
