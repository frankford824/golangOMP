import { createApp } from 'vue'
import { createPinia } from 'pinia'

import App from '../App.vue'
import { router } from '../router'

export function createAssetWorkbenchApp() {
  const app = createApp(App)
  app.use(createPinia())
  app.use(router)
  return app
}
