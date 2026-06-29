import { readAssetMotionTokens } from './tokens'

export interface FlipOptions {
  duration: number
  easing: string
  reverse?: boolean
}

export function animateOpacity(element: Element, from: number, to: number, duration: number, easing: string) {
  return element.animate([{ opacity: from }, { opacity: to }], { duration, easing, fill: 'both' })
}

export function animateFlip(target: HTMLElement, origin: HTMLElement, options: FlipOptions) {
  const originBox = origin.getBoundingClientRect()
  const targetBox = target.getBoundingClientRect()
  const dx = originBox.left - targetBox.left
  const dy = originBox.top - targetBox.top
  const sx = Math.max(0.0001, originBox.width / targetBox.width)
  const sy = Math.max(0.0001, originBox.height / targetBox.height)
  const collapsed = { transform: `translate(${dx}px, ${dy}px) scale(${sx}, ${sy})` }
  const expanded = { transform: 'translate(0, 0) scale(1, 1)' }

  target.style.transformOrigin = 'top left'
  return target.animate(options.reverse ? [expanded, collapsed] : [collapsed, expanded], {
    duration: options.duration,
    easing: options.easing,
    fill: 'both',
  })
}

export function ledgerOpenMotion(target: HTMLElement, origin: HTMLElement) {
  const tokens = readAssetMotionTokens(target)
  return animateFlip(target, origin, { duration: tokens.enter, easing: tokens.easeEnter })
}

export function ledgerCloseMotion(target: HTMLElement, origin: HTMLElement) {
  const tokens = readAssetMotionTokens(target)
  return animateFlip(target, origin, { duration: tokens.exit, easing: tokens.easeExit, reverse: true })
}
