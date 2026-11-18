import { createMainApp } from './app/main-app'
import { router } from './router'
import './assets/main.css'
import { oaEnabled } from './oa/access'

if (oaEnabled) document.documentElement.classList.add('oa-live-ui')

if (oaEnabled && window.location.hostname === 'oa.yongbo.cloud') {
  const marker = 'yb.oa.formal-session-initialized.v2'
  if (localStorage.getItem(marker) === null) {
    localStorage.removeItem('access_token')
    localStorage.setItem(marker, '1')
  }
}

const app = createMainApp(router)
app.mount('#app')
