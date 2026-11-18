import type { Router } from 'vue-router'

const storageKey = 'oa.last-chunk-recovery'
const chunkError = /Unable to preload CSS|Failed to fetch dynamically imported module|Importing a module script failed|error loading dynamically imported module|Loading chunk [^ ]+ failed/i

export function installChunkRecovery(router: Router) {
  let pending: number | undefined
  let navigating = false
  function recover(path: string) {
    if (navigating) return
    const destination = new URL(path, location.href)
    if (destination.origin !== location.origin) return
    let recent = false
    try {
      const last = Number(sessionStorage.getItem(storageKey))
      recent = Date.now() - last < 60_000
      if (!recent) sessionStorage.setItem(storageKey, String(Date.now()))
    } catch {
      recent = true
    }
    if (!recent && navigator.onLine) {
      navigating = true
      if (destination.href === location.href) location.reload()
      else location.assign(destination.href)
      return
    }
    if (document.getElementById('oa-page-recovery')) return
    const box = document.createElement('div')
    box.id = 'oa-page-recovery'
    box.setAttribute('role', 'alert')
    box.append('页面资源暂未加载，请检查网络后重试。')
    const link = document.createElement('a')
    link.href = destination.href
    link.textContent = '重新打开页面'
    box.append(link)
    document.body.prepend(box)
  }
  // Let the router supply the target when a route import triggers the Vite event.
  window.addEventListener('vite:preloadError', () => {
    window.clearTimeout(pending)
    pending = window.setTimeout(() => recover(location.href), 0)
  })
  router.onError((error, to) => {
    if (!chunkError.test(String(error))) return
    window.clearTimeout(pending)
    recover(router.resolve(to).href)
  })
}
