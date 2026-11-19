import { createMainApp } from './app/main-app'
import { router } from './router'
import './assets/main.css'
import { installChunkRecovery } from './oa/chunkRecovery'
import { oaRequestError } from './oa/requestErrors'
import { oaEnabled } from './oa/access'
import { watchOASessionChanges } from './services/oaSession'
import { usePermissionsStore } from './stores/permissions'

if (oaEnabled) {
  window.addEventListener('asset-media-session-reset', () => { oaRequestError.value = undefined })
  document.documentElement.classList.add('oa-live-ui')
  if (document.fonts) {
    for (const weight of [400, 500, 600]) {
      void document.fonts.load(weight + ' 13px "YB JetBrains Mono"', '0123456789').catch(() => undefined)
    }
  }
  installChunkRecovery(router)
}

const app = createMainApp(router)
if (oaEnabled) {
  watchOASessionChanges(() => {
    const store = usePermissionsStore()
    if (!store.currentUser) return
    const route = router.currentRoute.value
    store.resetSession()
    if (route.name !== 'Login') void router.replace({ name: 'Login', query: { redirect: route.fullPath } })
  })
}
app.mount('#app')
