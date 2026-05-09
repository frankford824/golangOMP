import { createMainApp } from './app/main-app'
import { router } from './router'
import './assets/main.css'

const app = createMainApp(router)
app.mount('#app')
