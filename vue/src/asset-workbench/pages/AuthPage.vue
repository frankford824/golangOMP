<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { ArrowRight, LockKeyhole, UserRound } from 'lucide-vue-next'

import { authApi } from '@/services/api/authApi'
import { clearToken, setToken } from '@/services/http'
import { assetWorkbenchApi } from '../shared/api/assetWorkbenchApi'

const route = useRoute()
const router = useRouter()
const loading = ref(false)
const error = ref('')

const loginForm = reactive({
  account: '',
  password: '',
})

const registerForm = reactive({
  account: '',
  name: '',
  phone: '',
  email: '',
  password: '',
  worker_type: 'parttime',
  gender: '',
  province: '',
  city: '',
  id_card: '',
  alipay_account: '',
})

const isRegister = computed(() => route.path.includes('register'))
const title = computed(() => (isRegister.value ? '创建工作台账号' : '登录资产工作台'))
const redirectTarget = computed(() => {
  const redirect = route.query.redirect
  if (typeof redirect !== 'string' || !redirect.startsWith('/')) return '/'
  if (redirect === '/asset.html' || redirect.startsWith('/asset.html?')) return '/'
  return redirect
})

function pickAuthToken(payload: unknown): string {
  if (!payload || typeof payload !== 'object') return ''
  const root = payload as Record<string, unknown>
  const data = root.data && typeof root.data === 'object' ? (root.data as Record<string, unknown>) : root
  const auth = data.auth && typeof data.auth === 'object' ? (data.auth as Record<string, unknown>) : data
  const session = auth.session && typeof auth.session === 'object' ? (auth.session as Record<string, unknown>) : undefined
  return String(session?.token ?? auth.token ?? '')
}

async function activateSession(token: string) {
  if (!token) throw new Error('登录响应缺少 token')
  setToken(token)
  await authApi.refreshAssetCookie().catch(() => undefined)
  await router.replace(redirectTarget.value)
}

async function submitLogin() {
  if (loading.value) return
  loading.value = true
  error.value = ''
  try {
    const response = await authApi.login({
      username: loginForm.account,
      password: loginForm.password,
    })
    await activateSession(pickAuthToken(response.data))
  } catch (err) {
    clearToken()
    error.value = err instanceof Error ? err.message : '登录失败'
  } finally {
    loading.value = false
  }
}

async function submitRegister() {
  if (loading.value) return
  loading.value = true
  error.value = ''
  try {
    const response = await assetWorkbenchApi.register({
      account: registerForm.account,
      name: registerForm.name,
      phone: registerForm.phone,
      email: registerForm.email,
      password: registerForm.password,
      worker_type: registerForm.worker_type,
      province: registerForm.province,
      city: registerForm.city,
      id_card: registerForm.id_card,
      gender: registerForm.gender,
      alipay_account: registerForm.alipay_account,
    })
    await activateSession(pickAuthToken(response))
  } catch (err) {
    clearToken()
    error.value = err instanceof Error ? err.message : '注册失败'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <main class="aw-auth" :class="{ 'aw-auth--register': isRegister }" aria-labelledby="aw-auth-title">
    <section class="aw-auth__intro" aria-label="资产工作台">
      <div class="aw-auth__brandbar">
        <span class="aw-auth__brandmark" aria-hidden="true">AW</span>
        <strong>资产工作台</strong>
      </div>
    </section>

    <section class="aw-auth__panel">
      <strong class="aw-auth__wordmark" aria-hidden="true">AW</strong>
      <div class="aw-auth__tabs" role="tablist" aria-label="资产工作台认证">
        <RouterLink to="/login" class="aw-auth__tab" :class="{ 'aw-auth__tab--active': !isRegister }">
          <LockKeyhole :size="16" aria-hidden="true" />
          <span>登录</span>
        </RouterLink>
        <RouterLink to="/register" class="aw-auth__tab" :class="{ 'aw-auth__tab--active': isRegister }">
          <UserRound :size="16" aria-hidden="true" />
          <span>注册</span>
        </RouterLink>
      </div>

      <form v-if="!isRegister" class="aw-auth__form" @submit.prevent="submitLogin">
        <h1 id="aw-auth-title">{{ title }}</h1>
        <p class="aw-auth__lead">使用已开通账号进入素材网盘。</p>
        <label class="aw-field">
          <span>账号</span>
          <input v-model.trim="loginForm.account" autocomplete="username" placeholder="请输入账号" required autofocus />
        </label>
        <label class="aw-field">
          <span>密码</span>
          <input v-model="loginForm.password" type="password" autocomplete="current-password" placeholder="请输入密码" required />
        </label>
        <p v-if="error" class="aw-form-error">{{ error }}</p>
        <button class="aw-primary-button aw-auth__submit" type="submit" :disabled="loading">
          <span>{{ loading ? '登录中' : '登录' }}</span>
          <ArrowRight :size="16" aria-hidden="true" />
        </button>
      </form>

      <form v-else class="aw-auth__form" @submit.prevent="submitRegister">
        <h1 id="aw-auth-title">{{ title }}</h1>
        <p class="aw-auth__lead">提交后进入成员开通流程。</p>
        <div class="aw-form-grid">
          <label class="aw-field">
            <span>账号</span>
            <input v-model.trim="registerForm.account" autocomplete="username" placeholder="用于登录" required autofocus />
          </label>
          <label class="aw-field">
            <span>姓名</span>
            <input v-model.trim="registerForm.name" autocomplete="name" placeholder="真实姓名" required />
          </label>
          <label class="aw-field">
            <span>手机号</span>
            <input v-model.trim="registerForm.phone" autocomplete="tel" inputmode="tel" placeholder="手机号" required />
          </label>
          <label class="aw-field">
            <span>邮箱</span>
            <input v-model.trim="registerForm.email" type="email" autocomplete="email" placeholder="选填" />
          </label>
          <label class="aw-field">
            <span>所属类型</span>
            <select v-model="registerForm.worker_type" required>
              <option value="parttime">兼职</option>
              <option value="fulltime">全职</option>
            </select>
          </label>
          <label class="aw-field">
            <span>性别</span>
            <select v-model="registerForm.gender">
              <option value="">不填写</option>
              <option value="female">女</option>
              <option value="male">男</option>
            </select>
          </label>
          <label class="aw-field">
            <span>省份</span>
            <input v-model.trim="registerForm.province" autocomplete="address-level1" placeholder="选填" />
          </label>
          <label class="aw-field">
            <span>城市</span>
            <input v-model.trim="registerForm.city" autocomplete="address-level2" placeholder="选填" />
          </label>
          <label class="aw-field aw-form-grid__full">
            <span>身份证号</span>
            <input v-model.trim="registerForm.id_card" autocomplete="off" placeholder="选填" />
          </label>
          <label class="aw-field aw-form-grid__full">
            <span>支付宝账号</span>
            <input v-model.trim="registerForm.alipay_account" autocomplete="off" placeholder="选填" />
          </label>
          <label class="aw-field aw-form-grid__full">
            <span>密码</span>
            <input v-model="registerForm.password" type="password" autocomplete="new-password" placeholder="设置登录密码" required />
          </label>
        </div>
        <p v-if="error" class="aw-form-error">{{ error }}</p>
        <button class="aw-primary-button aw-auth__submit" type="submit" :disabled="loading">
          <span>{{ loading ? '提交中' : '创建账号' }}</span>
          <ArrowRight :size="16" aria-hidden="true" />
        </button>
      </form>
    </section>
  </main>
</template>
