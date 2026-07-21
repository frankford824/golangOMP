import {
  PermissionEnum,
  type PermissionEnumValue,
} from '@/types'
import { normalizeActionKey, usePermissionsStore } from '@/stores/permissions'

export function usePermission() {
  const store = usePermissionsStore()

  /** 能力判断只接受后端当前点号合同，不再展开历史别名。 */
  function can(
    perm: PermissionEnumValue | PermissionEnumValue[] | string | string[],
  ): boolean {
    const list = (Array.isArray(perm) ? perm : [perm]) as string[]
    const enumSet = new Set<string>(Object.values(PermissionEnum) as string[])
    return list.some((raw) => {
      const dotted = normalizeActionKey(raw)
      if (!dotted) return false
      if (enumSet.has(dotted)) {
        return store.hasPermission(dotted as PermissionEnumValue)
      }
      return store.hasAction(dotted)
    })
  }

  function canAccessMenu(key: string): boolean {
    return store.hasMenu(key)
  }

  function canAccessPage(key: string): boolean {
    return store.hasPage(key)
  }

  function canAccessAction(key: string): boolean {
    return store.hasAction(key)
  }

  function canAccessModule(key: string): boolean {
    return store.hasModule(key)
  }

  /**
   * 当后端未下发 `modules`（空数组）时不拦截；下发后须命中模块键。
   * 用于页面内模块块显隐（与侧栏 `menus` 解耦时的补充开关）。
   */
  // 模块显隐完全由后端 `frontend_access.modules` 决定。空列表表示当前
  // 账号未启用模块级细分，此时仍由页面/动作能力继续约束。
  function canAccessModuleWhenDeclared(moduleKey: string): boolean {
    if (!store.currentUser) return false
    const mods = store.modules
    if (!mods.length) return true
    return mods.includes(moduleKey)
  }

  return {
    can,
    canAccessMenu,
    canAccessPage,
    canAccessAction,
    canAccessModule,
    canAccessModuleWhenDeclared,
  }
}
