export interface AssetMotionTokens {
  instant: number
  fast: number
  standard: number
  slow: number
  deliberate: number
  enter: number
  exit: number
  stagger: number
  shift: number
  shiftDistance: string
  easeStandard: string
  easeEnter: string
  easeExit: string
}

const fallbackTokens: AssetMotionTokens = {
  instant: 0,
  fast: 140,
  standard: 200,
  slow: 320,
  deliberate: 400,
  enter: 400,
  exit: 300,
  stagger: 140,
  shift: 130,
  shiftDistance: 'var(--aw-motion-shift-distance)',
  easeStandard: 'cubic-bezier(0.2, 0, 0, 1)',
  easeEnter: 'cubic-bezier(0.2, 0, 0, 1)',
  easeExit: 'cubic-bezier(0.4, 0, 1, 1)',
}

function cssSource(element?: Element | null) {
  if (typeof window === 'undefined') return null
  return window.getComputedStyle(element ?? document.querySelector('.aw-root') ?? document.documentElement)
}

function readMs(style: CSSStyleDeclaration | null, name: string, fallback: number): number {
  const raw = style?.getPropertyValue(name).trim()
  if (!raw) return fallback
  if (raw.endsWith('ms')) return Number.parseFloat(raw)
  if (raw.endsWith('s')) return Number.parseFloat(raw) * 1000
  const parsed = Number.parseFloat(raw)
  return Number.isFinite(parsed) ? parsed : fallback
}

function readValue(style: CSSStyleDeclaration | null, name: string, fallback: string): string {
  return style?.getPropertyValue(name).trim() || fallback
}

export function readAssetMotionTokens(element?: Element | null): AssetMotionTokens {
  const style = cssSource(element)
  return {
    instant: readMs(style, '--aw-motion-instant', fallbackTokens.instant),
    fast: readMs(style, '--aw-motion-fast', fallbackTokens.fast),
    standard: readMs(style, '--aw-motion-standard', fallbackTokens.standard),
    slow: readMs(style, '--aw-motion-slow', fallbackTokens.slow),
    deliberate: readMs(style, '--aw-motion-deliberate', fallbackTokens.deliberate),
    enter: readMs(style, '--aw-motion-enter', fallbackTokens.enter),
    exit: readMs(style, '--aw-motion-exit', fallbackTokens.exit),
    stagger: readMs(style, '--aw-motion-stagger', fallbackTokens.stagger),
    shift: readMs(style, '--aw-motion-shift', fallbackTokens.shift),
    shiftDistance: readValue(style, '--aw-motion-shift-distance', fallbackTokens.shiftDistance),
    easeStandard: readValue(style, '--aw-ease-standard', fallbackTokens.easeStandard),
    easeEnter: readValue(style, '--aw-ease-enter', fallbackTokens.easeEnter),
    easeExit: readValue(style, '--aw-ease-exit', fallbackTokens.easeExit),
  }
}

export function prefersReducedMotion() {
  return typeof window !== 'undefined' && window.matchMedia('(prefers-reduced-motion: reduce)').matches
}
