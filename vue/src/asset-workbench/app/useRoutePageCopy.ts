import { routeAccessForPath } from './access'

export function useRoutePageCopy(path: string) {
  const access = routeAccessForPath(path)
  return {
    access,
    label: access?.label ?? '',
    subtitle: access?.subtitle ?? '',
  }
}
