import { describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { goBackTo, safeReturnPath, workspaceOrigin } from './returnNavigation'

const assets = (path: string) => path === '/asset-center'
describe('OA return navigation', () => {
  it('accepts local asset state and rejects external or ambiguous paths', () => {
    expect(safeReturnPath('/asset-center?page=2#sku',assets)).toBe('/asset-center?page=2#sku')
    for (const path of ['https://example.org', '//example.org', '/users', '/asset-center%5cfoo','/x/..//outside']) {
      expect(safeReturnPath(path, assets)).toBeUndefined()
    }
  })
  it('uses browser back for a known predecessor and replaces an independent detail entry', async () => {
    const router = createRouter({ history:createMemoryHistory(), routes:['/asset-center','/asset-center/:id','/me'].map(path=>({path,component:{},meta:{requiresAuth:true}})) })
    await router.push('/asset-center?page=3')
    await router.push('/asset-center/6')
    // The memory router has no browser-maintained back property.
    Object.assign(router.options.history.state,{back:'/asset-center?page=3'})
    const back=vi.spyOn(router,'back').mockImplementation(()=>{})
    await goBackTo(router,'/asset-center')
    expect(back).toHaveBeenCalledOnce()
    const replace=vi.spyOn(router,'replace')
    await goBackTo(router,'/me')
    expect(replace).toHaveBeenCalledWith('/me')
    expect(workspaceOrigin(router,'/asset-center')).toBe('/asset-center')
    expect(workspaceOrigin(router,'//external.test')).toBeUndefined()
  })
})
