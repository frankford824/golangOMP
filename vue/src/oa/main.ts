import { createApp } from 'vue'
import { createRouter, createWebHashHistory } from 'vue-router'
import App from './App.vue'
import './styles.css'

const Fees = () => import('./pages/Fees.vue')
const People = () => import('./pages/People.vue')
const Commerce = () => import('./pages/Commerce.vue')
const OrderData = () => import('./pages/OrderData.vue')
const Existing = () => import('./pages/Existing.vue')

const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    { path: '/', redirect: '/overview' },
    { path: '/overview', component: Commerce, props: { mode: 'overview' } },
    { path: '/products', component: Commerce, props: { mode: 'products' } },
    { path: '/products/:id', component: Commerce, props: route => ({ mode: 'detail', productId: route.params.id }) },
    { path: '/personal', component: Commerce, props: { mode: 'personal' } },
    { path: '/ranking', component: Commerce, props: { mode: 'ranking' } },
    { path: '/orders/detail', component: OrderData, props: { mode: 'detail' } },
    { path: '/orders/refunds', component: OrderData, props: { mode: 'refunds' } },
    { path: '/orders/ledger', component: OrderData, props: { mode: 'ledger' } },
    { path: '/people', component: People, props: { mode: 'people' } },
    { path: '/roster', component: People, props: { mode: 'roster' } },
    { path: '/fees/:mode', component: Fees, props: true },
    { path: '/work/:mode', component: Existing, props: true },
    { path: '/:unknown(.*)*', redirect: '/overview' },
  ],
})

createApp(App).use(router).mount('#oa-app')
