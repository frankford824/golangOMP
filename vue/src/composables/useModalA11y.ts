import { nextTick, onUnmounted, watch, type Ref } from 'vue'

// Shared, dependency-free dialog accessibility helper:
// - module-level scroll-lock counter so stacked Teleport dialogs do not
//   restore <body> scrolling until the last one closes;
// - a modal stack so only the top-most dialog handles Esc / Tab trapping;
// - focus trap, initial focus, and focus restoration on close.

let scrollLockCount = 0
let savedBodyOverflow = ''

function lockBodyScroll() {
  if (typeof document === 'undefined') return
  if (scrollLockCount === 0) {
    savedBodyOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
  }
  scrollLockCount += 1
}

function unlockBodyScroll() {
  if (typeof document === 'undefined') return
  if (scrollLockCount === 0) return
  scrollLockCount -= 1
  if (scrollLockCount === 0) {
    document.body.style.overflow = savedBodyOverflow
    savedBodyOverflow = ''
  }
}

const modalStack: symbol[] = []

const FOCUSABLE_SELECTOR = [
  'a[href]',
  'area[href]',
  'button:not([disabled])',
  'input:not([disabled]):not([type="hidden"])',
  'select:not([disabled])',
  'textarea:not([disabled])',
  '[tabindex]:not([tabindex="-1"])',
].join(',')

interface ModalA11yOptions {
  /** Invoked when Escape is pressed on the top-most dialog. */
  onEsc?: () => void
}

/**
 * Wires dialog accessibility to an open-state ref and a panel element ref.
 * Visual output is unchanged: the panel is focused programmatically (no
 * focus-visible ring) and callers keep their own control focus styles.
 */
export function useModalA11y(
  isOpen: Ref<boolean>,
  panelRef: Ref<HTMLElement | null>,
  options: ModalA11yOptions = {},
) {
  const id = Symbol('modal')
  let active = false
  let previouslyFocused: HTMLElement | null = null

  function isTopMost() {
    return modalStack[modalStack.length - 1] === id
  }

  function focusableItems(): HTMLElement[] {
    const root = panelRef.value
    if (!root) return []
    return Array.from(root.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR)).filter(
      (el) => el.offsetWidth > 0 || el.offsetHeight > 0 || el === document.activeElement,
    )
  }

  function onKeydown(event: KeyboardEvent) {
    if (!active || !isTopMost()) return

    if (event.key === 'Escape') {
      event.stopPropagation()
      options.onEsc?.()
      return
    }

    if (event.key !== 'Tab') return

    const items = focusableItems()
    const panel = panelRef.value
    if (!panel) return

    if (items.length === 0) {
      event.preventDefault()
      panel.focus()
      return
    }

    const first = items[0]
    const last = items[items.length - 1]
    const current = document.activeElement as HTMLElement | null
    const inside = panel.contains(current)

    if (event.shiftKey) {
      if (!inside || current === first) {
        event.preventDefault()
        last.focus()
      }
    } else if (!inside || current === last) {
      event.preventDefault()
      first.focus()
    }
  }

  async function activate() {
    if (active) return
    active = true
    previouslyFocused = (document.activeElement as HTMLElement | null) ?? null
    modalStack.push(id)
    lockBodyScroll()
    document.addEventListener('keydown', onKeydown, true)

    await nextTick()
    const panel = panelRef.value
    if (panel) {
      if (!panel.hasAttribute('tabindex')) {
        panel.setAttribute('tabindex', '-1')
      }
      panel.focus()
    }
  }

  function deactivate() {
    if (!active) return
    active = false
    document.removeEventListener('keydown', onKeydown, true)
    const index = modalStack.lastIndexOf(id)
    if (index !== -1) modalStack.splice(index, 1)
    unlockBodyScroll()

    const target = previouslyFocused
    previouslyFocused = null
    if (target && typeof target.focus === 'function' && document.contains(target)) {
      target.focus()
    }
  }

  watch(
    isOpen,
    (open) => {
      if (open) void activate()
      else deactivate()
    },
    { immediate: true },
  )

  onUnmounted(deactivate)
}
