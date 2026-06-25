function parseRgbChannels(value: string): [string, string, string] | null {
  const parts = value.match(/\d+(?:\.\d+)?/g)
  if (!parts || parts.length < 3) return null
  return [parts[0], parts[1], parts[2]]
}

export function resolveCssRgbToken(tokenName: string, fallbackChannels: string, alpha?: number): string {
  const propName = tokenName.startsWith('--') ? tokenName : `--${tokenName}`
  const fallback = parseRgbChannels(fallbackChannels) ?? ['0', '0', '0']
  const raw =
    typeof window === 'undefined'
      ? ''
      : window.getComputedStyle(document.documentElement).getPropertyValue(propName).trim()
  const [r, g, b] = parseRgbChannels(raw) ?? fallback
  return typeof alpha === 'number' ? `rgba(${r}, ${g}, ${b}, ${alpha})` : `rgb(${r}, ${g}, ${b})`
}
