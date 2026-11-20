// The shell remains mounted while cross-module views fetch and remount their contents.
type Point = { top: number; left: number }
type Visit = { path: string; owner: string; points: Record<string, Point> }

function regions(root: HTMLElement) {
  const result = new Map<string, HTMLElement>([['page', root]])
  for (const item of root.querySelectorAll<HTMLElement>('[data-oa-return-scroll]')) {
    if (item.offsetWidth) result.set(item.dataset.oaReturnScroll!, item)
  }
  return result
}

export function createReturnScroll() {
  const visits = new Map<string, Visit>()
  let currentKey = ''
  let currentOwner = ''
  let cancel = () => {}

  function capture(root: HTMLElement, path: string, owner: string) {
    cancel()
    if (!currentKey || !owner || currentOwner !== owner) return
    const points: Visit['points'] = {}
    for (const [key, el] of regions(root)) {
      if (el.scrollTop || el.scrollLeft) points[key] = { top: el.scrollTop, left: el.scrollLeft }
    }
    const saved = { path, owner, points }
    visits.set(currentKey, saved)
    if (visits.size > 64) visits.delete(visits.keys().next().value!)
    // On popstate the browser has already moved to the destination entry.
    if (history.state?.current === path) history.replaceState({ ...history.state, oaReturnScroll: saved }, '')
  }

  function restore(root: HTMLElement, path: string, owner: string) {
    cancel()
    const state = history.state
    if (!state || state.current !== path) return
    const id = state.oaScrollId || crypto.randomUUID()
    if (!state.oaScrollId) history.replaceState({ ...state, oaScrollId: id }, '')
    currentKey = id + ':' + path
    currentOwner = owner
    const saved = visits.get(currentKey) ?? state.oaReturnScroll as Visit | undefined
    if (!owner || saved?.owner !== owner || saved.path !== path) {
      root.scrollTo(0, 0)
      return
    }
    if (!saved.points.page) root.scrollTo(0, 0)
    const pending = new Map(Object.entries(saved.points))
    if (!pending.size) return

    const changes = new MutationObserver(schedule)
    const resize = new ResizeObserver(schedule)
    const controller = new AbortController()
    const options = { capture: true, passive: true, signal: controller.signal }
    let active = true
    let frame = 0
    const timeout = window.setTimeout(stop, 15000)
    function stop() {
      active = false
      changes.disconnect()
      resize.disconnect()
      controller.abort()
      cancelAnimationFrame(frame)
      clearTimeout(timeout)
      cancel = () => {}
    }
    function schedule() {
      if (active && !frame) frame = requestAnimationFrame(apply)
    }
    function apply() {
      frame = 0
      const available = regions(root)
      for (const [key, point] of pending) {
        const el = available.get(key)
        if (!el) continue
        el.scrollTo(point.left, point.top)
        if (Math.abs(el.scrollTop - point.top) <= 1 && Math.abs(el.scrollLeft - point.left) <= 1) pending.delete(key)
      }
      if (!pending.size) stop()
    }
    cancel = stop
    changes.observe(root, { childList: true, subtree: true })
    if (root.firstElementChild) resize.observe(root.firstElementChild)
    root.addEventListener('load', schedule, options)
    for (const event of ['wheel','pointerdown','touchstart','keydown']) root.addEventListener(event, stop, options)
    apply()
  }

  return { capture, restore, dispose() { cancel(); visits.clear(); currentKey = ''; currentOwner = '' } }
}
