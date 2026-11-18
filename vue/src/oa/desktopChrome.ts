import type { InjectionKey, Ref } from 'vue'

type DesktopChrome = {
  desktop: Readonly<Ref<boolean>>
  tabs: Ref<HTMLElement | null>
  status: Ref<HTMLElement | null>
  actions: Ref<HTMLElement | null>
}
export const desktopChromeKey: InjectionKey<DesktopChrome> = Symbol('oa-desktop-chrome')
