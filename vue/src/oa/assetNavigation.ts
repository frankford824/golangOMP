import type { RouteLocationRaw } from 'vue-router'
import { safeReturnPath } from '@/oa/returnNavigation'

export function assetListReturn(source: unknown): RouteLocationRaw {
  return safeReturnPath(source, path => path === '/asset-center') ?? { name: 'AssetsIndex' }
}
