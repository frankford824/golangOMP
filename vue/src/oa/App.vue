<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { RouterLink, RouterView, useRoute, useRouter } from 'vue-router'
import { Bell, Box, ChartNoAxesCombined, ClipboardList, CreditCard, House, LogOut, Menu, Receipt, UserRound, UsersRound, Wallet } from 'lucide-vue-next'
import { api, clearToken, errorText, getToken, OAError, send, setToken, tokenPresent } from './api'
import { can, currentProfile, loadProfile, resetProfile } from './session'

const route = useRoute()
const router = useRouter()
const busy = ref(false)
const error = ref('')
const username = ref('')
const password = ref('')
const menu = ref(false)
const templateAccess = ref(false)
const hasOrders = computed(() => can('orders.view') || currentProfile.value?.super_admin || currentProfile.value?.roles?.includes('ops'))

const groups = computed(() => [
  { name: '工作台', items: [
    { label: '经营首页', path: '/overview', icon: House, show: hasOrders.value },
    { label: '我归属的商品', path: '/products', icon: Box, show: hasOrders.value },
    { label: '经营分析', path: '/ranking', icon: ChartNoAxesCombined, show: hasOrders.value },
    { label: '任务中心', path: '/work/tasks', icon: ClipboardList, show: true },
    { label: 'SKU 成本中心', path: '/work/cost', icon: Box, show: true },
    { label: '资产交付与结算', path: '/work/assets', icon: Wallet, show: true },
  ] },
  { name: '公司事务', items: [
    { label: '人员管理', path: '/people', icon: UsersRound, show: can('people.manage') || !!currentProfile.value?.super_admin },
    { label: '组织任职与花名册', path: '/roster', icon: UsersRound, show: can('people.manage') || !!currentProfile.value?.super_admin },
    { label: '审批与付款', path: '/fees/approval', icon: CreditCard, show: true },
    { label: '费用模板', path: '/fees/templates', icon: ClipboardList, show: templateAccess.value || can('finance.templates') || !!currentProfile.value?.super_admin },
    { label: '支出统计', path: '/fees/report', icon: ChartNoAxesCombined, show: can('finance.report') || can('finance.pay') || !!currentProfile.value?.super_admin },
  ] },
  { name: '个人事项', items: [
    { label: '发起费用申请', path: '/fees/apply', icon: Receipt, show: true },
    { label: '我的申请', path: '/fees/mine', icon: Wallet, show: true },
    { label: '个人中心', path: '/work/account', icon: UserRound, show: true },
    { label: '通知中心', path: '/work/notifications', icon: Bell, show: true },
  ] },
].map(group => ({ ...group, items: group.items.filter(item => item.show) })).filter(group => group.items.length))

let profileRequest = 0
async function restore() {
  if (!getToken()) return
  const sequence = ++profileRequest
  busy.value = true
  error.value = ''
  try {
    await loadProfile()
    if (sequence !== profileRequest) return
    if (!currentProfile.value?.super_admin) {
      const permissions = await api<{ level?: string }>('/oa-api/finance/delegates').catch(() => undefined)
      if (sequence !== profileRequest) return
      templateAccess.value = permissions?.level === 'draft' || permissions?.level === 'publish'
    }
    if (!hasOrders.value && route.path === '/overview') await router.replace('/fees/mine')
  } catch (cause) {
    if (sequence === profileRequest && !(cause instanceof OAError && cause.status === 401)) error.value = errorText(cause)
  } finally {
    if (sequence === profileRequest) busy.value = false
  }
}

async function login() {
  busy.value = true
  error.value = ''
  try {
    const data = await send<{ session?: { token?: string }; token?: string }>('/v1/auth/login', { username: username.value.trim(), password: password.value })
    const token = data.session?.token || data.token
    if (!token) throw new Error('登录服务未返回会话凭证')
    setToken(token)
    await restore()
  } catch (cause) {
    error.value = errorText(cause)
  } finally {
    password.value = ''
    busy.value = false
  }
}

async function logout() {
  if (busy.value) return
  busy.value = true
  try { await send('/v1/auth/logout', {}) }
  catch { /* The local session is cleared even when logout cannot reach the service. */ }
  finally { signOut() }
}
function signOut() {
  ++profileRequest
  clearToken()
  resetProfile()
  templateAccess.value = false
  busy.value = false
}
function syncSession(event: StorageEvent) {
  if (event.key !== 'access_token') return
  if (event.newValue) {
    tokenPresent.value = true
    void restore()
  } else {
    signOut()
  }
}
function onExpiry() {
  signOut()
  error.value = '会话已过期，请重新登录'
}

onMounted(() => {
  window.addEventListener('storage', syncSession)
  window.addEventListener('oa-session-expired', onExpiry)
  void restore()
})
onUnmounted(() => {
  window.removeEventListener('storage', syncSession)
  window.removeEventListener('oa-session-expired', onExpiry)
})
</script>

<template>
  <div v-if="currentProfile" class="oa-shell">
    <header class="oa-top">
      <button class="oa-top-button oa-mobile" type="button" aria-label="展开工作模块" :aria-expanded="menu" @click="menu = !menu"><Menu :size="21" /></button>
      <RouterLink class="oa-brand" :to="hasOrders ? '/overview' : '/work/account'"><span class="oa-brand-mark" aria-hidden="true">Y</span>永箔 OA</RouterLink>
      <span class="oa-env">验证环境</span>
      <div class="oa-top-actions">
        <RouterLink class="oa-top-button" to="/work/notifications" aria-label="通知中心"><Bell :size="18" /></RouterLink>
        <RouterLink class="oa-account" to="/work/account"><span class="oa-avatar">{{ currentProfile.name.slice(-1) }}</span>{{ currentProfile.name }}</RouterLink>
        <button class="oa-top-button" type="button" aria-label="退出登录" title="退出登录" @click="logout"><LogOut :size="18" /></button>
      </div>
    </header>
    <aside :class="['oa-nav', { 'oa-nav-open': menu }]">
      <nav aria-label="OA 工作模块">
        <section v-for="group of groups" :key="group.name">
          <h2>{{ group.name }}</h2>
          <RouterLink v-for="item of group.items" :key="item.path" :to="item.path" @click="menu = false">
            <component :is="item.icon" :size="17" aria-hidden="true" />
            {{ item.label }}
          </RouterLink>
        </section>
      </nav>
    </aside>
    <main id="oa-main" class="oa-main">
      <RouterView v-slot="{ Component }"><component :is="Component" :key="route.path" /></RouterView>
    </main>
  </div>
  <main v-else class="oa-login">
    <div class="oa-login-card">
      <div class="oa-brand"><span class="oa-brand-mark" aria-hidden="true">Y</span>永箔 OA</div>
      <h1>{{ tokenPresent ? '正在进入工作台' : '登录工作台' }}</h1>
      <p v-if="error" class="oa-error" role="alert">{{ error }}</p>
      <form v-if="!tokenPresent" class="oa-form" @submit.prevent="login">
        <label>账号<input v-model="username" autofocus autocomplete="username" required /></label>
        <label>密码<input v-model="password" type="password" autocomplete="current-password" required /></label>
        <button class="oa-button-primary" :disabled="busy">{{ busy ? '登录中…' : '登录' }}</button>
      </form>
      <div v-else-if="error" class="oa-actions"><button class="oa-button-primary" :disabled="busy" @click="restore">重试</button><button @click="signOut">重新登录</button></div>
      <p v-else role="status">正在读取账号权限…</p>
    </div>
  </main>
</template>
