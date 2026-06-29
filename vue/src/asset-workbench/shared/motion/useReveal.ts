import { onBeforeUnmount, onMounted, type Ref } from 'vue'

export function useReveal(target: Ref<HTMLElement | null>, className = 'aw-motion-revealed') {
  let observer: IntersectionObserver | null = null

  onMounted(() => {
    const element = target.value
    if (!element || typeof IntersectionObserver === 'undefined') {
      element?.classList.add(className)
      return
    }
    observer = new IntersectionObserver((entries) => {
      for (const entry of entries) {
        if (entry.isIntersecting) {
          element.classList.add(className)
          observer?.disconnect()
          observer = null
        }
      }
    })
    observer.observe(element)
  })

  onBeforeUnmount(() => observer?.disconnect())
}
