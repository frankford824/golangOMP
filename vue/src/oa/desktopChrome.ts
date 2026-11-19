import type { InjectionKey, Ref } from 'vue'

type DesktopChrome = {
  desktop: Readonly<Ref<boolean>>
  title: Ref<HTMLElement | null>
  tabs: Ref<HTMLElement | null>
  status: Ref<HTMLElement | null>
  actions: Ref<HTMLElement | null>
}
export const desktopChromeKey: InjectionKey<DesktopChrome> = Symbol('oa-desktop-chrome')
