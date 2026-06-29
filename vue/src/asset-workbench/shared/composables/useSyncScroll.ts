import { onBeforeUnmount, onMounted, watch, type Ref } from 'vue'

type SyncAxis = 'x' | 'y' | 'both'

interface UseSyncScrollOptions {
  axis?: SyncAxis
}

export function useSyncScroll(
  source: Ref<HTMLElement | null>,
  target: Ref<HTMLElement | null>,
  options: UseSyncScrollOptions = {},
) {
  const axis = options.axis ?? 'x'
  let cleanup: (() => void) | null = null
  let syncing = false

  const sync = () => {
    const sourceEl = source.value
    const targetEl = target.value
    if (!sourceEl || !targetEl || syncing) return

    syncing = true
    if (axis === 'x' || axis === 'both') targetEl.scrollLeft = sourceEl.scrollLeft
    if (axis === 'y' || axis === 'both') targetEl.scrollTop = sourceEl.scrollTop
    requestAnimationFrame(() => {
      syncing = false
    })
  }

  const bind = () => {
    cleanup?.()
    const sourceEl = source.value
    if (!sourceEl) {
      cleanup = null
      return
    }
    sourceEl.addEventListener('scroll', sync, { passive: true })
    cleanup = () => sourceEl.removeEventListener('scroll', sync)
    sync()
  }

  onMounted(bind)
  watch([source, target], bind)
  onBeforeUnmount(() => cleanup?.())

  return { sync }
}
