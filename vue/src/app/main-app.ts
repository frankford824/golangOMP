import { createApp, type App } from 'vue'
import { createPinia } from 'pinia'
import type { Router } from 'vue-router'
import AppRoot from '../App.vue'

export function createMainApp(router: Router): App {
  const app = createApp(AppRoot)
  app.use(createPinia())
  app.use(router)
  return app
}
