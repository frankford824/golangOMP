import { onBeforeUnmount } from 'vue'

export function useScrollLock(className = 'aw-scroll-locked') {
  function lock() {
    document.body.classList.add(className)
  }

  function unlock() {
    document.body.classList.remove(className)
  }

  onBeforeUnmount(unlock)

  return { lock, unlock }
}
