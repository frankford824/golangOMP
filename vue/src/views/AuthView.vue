<template>
  <div :class="{ 'oa-auth': oaEnabled, 'oa-auth-register': oaEnabled && activeTab === 'register' }" class="auth-page flex flex-col min-h-screen w-full bg-surface overflow-auto font-body">
    <!-- 背景水印：极浅色全局底纹 -->
    <div
      v-if="!oaEnabled"
      class="fixed inset-0 pointer-events-none select-none overflow-hidden flex items-center justify-center"
      aria-hidden="true"
    >
      <span class="auth-watermark font-black text-[20rem] text-[rgb(var(--yb-text-faint)/0.3)] leading-none">YONGBO</span>
    </div>

    <div class="relative z-10 flex w-full flex-1 flex-col items-center justify-center px-4 py-8">
      <!-- 居中卡片 -->
      <div
        class="auth-card relative z-10 my-8 rounded-[2rem] p-6 sm:p-12"
      >
      <!-- 顶部 Header -->
      <div class="auth-heading text-center mb-10">
        <div class="auth-logo inline-flex items-center justify-center w-12 h-12 rounded-xl bg-[rgb(var(--yb-brand-soft))] mb-4">
          <img v-if="oaEnabled" :src="oaBrandMark" width="36" height="36" alt="" />
          <span v-else class="material-symbols-outlined text-[rgb(var(--yb-brand))] text-2xl">architecture</span>
        </div>
        <h1 class="auth-title text-2xl font-headline font-extrabold text-[rgb(var(--yb-text))] mb-2 text-center whitespace-nowrap">
          永箔运营管理系统
        </h1>
        <p v-if="!oaEnabled" class="auth-subtitle text-xs text-[rgb(var(--yb-text-faint))] tracking-widest uppercase text-center whitespace-nowrap">
          请使用账号密码登录或注册新账号
        </p>
        <p v-if="oaEnabled" class="auth-formal-note">使用正式账号登录，业务提交写入正式数据。</p>
      </div>

      <!-- 错误提示 -->
      <div
        role="alert"
        v-if="errorMessage"
        class="auth-error mb-6 p-3 bg-[rgb(var(--yb-danger-soft))] border border-[rgb(var(--yb-danger-border))] rounded-xl text-sm text-[rgb(var(--yb-danger))] break-words leading-relaxed"
      >
        {{ errorMessage }}
      </div>

      <!-- Switcher：w-full 胶囊 h-12，选中项白色浮起 -->
      <div class="mb-8">
        <div class="auth-switch bg-[rgb(var(--yb-surface-muted))] rounded-full p-1 flex h-12 w-full">
          <button
            type="button"
            class="flex-1 h-full text-sm font-medium rounded-full transition-all duration-200 whitespace-nowrap"
            :class="activeTab === 'login'
              ? 'is-active'
              : ''
            "
            @click="activeTab = 'login'"
          >
            登录
          </button>
          <button
            type="button"
            class="flex-1 h-full text-sm font-medium rounded-full transition-all duration-200 whitespace-nowrap"
            :class="activeTab === 'register'
              ? 'is-active'
              : ''
            "
            @click="activeTab = 'register'"
          >
            注册
          </button>
        </div>
      </div>

      <!-- 登录表单 -->
      <form v-if="activeTab === 'login'" :class="{ 'space-y-6': !oaEnabled }" @submit.prevent="handleLogin">
        <div>
          <label for="loginForm.username" class="text-xs font-bold text-[rgb(var(--yb-text-muted))] uppercase tracking-widest mb-2 block whitespace-nowrap">用户名</label>
          <input
            id="loginForm.username" autocomplete="username" v-model="loginForm.username"
            type="text"
            placeholder="请输入用户名"
            class="auth-input w-full h-14 px-4 bg-[rgb(var(--yb-surface-soft))] border-none rounded-xl text-base text-[rgb(var(--yb-text))] placeholder:text-[rgb(var(--yb-text-faint))] whitespace-nowrap"
          />
        </div>
        <div>
          <label for="loginForm.password" class="text-xs font-bold text-[rgb(var(--yb-text-muted))] uppercase tracking-widest mb-2 block whitespace-nowrap">密码</label>
          <div class="auth-password"><input
            id="loginForm.password" autocomplete="current-password" v-model="loginForm.password"
            :type="oaEnabled && revealPassword ? 'text' : 'password'"
            placeholder="请输入密码"
            class="auth-input w-full h-14 px-4 bg-[rgb(var(--yb-surface-soft))] border-none rounded-xl text-base text-[rgb(var(--yb-text))] placeholder:text-[rgb(var(--yb-text-faint))] whitespace-nowrap"
          />
            <button v-if="oaEnabled" type="button" class="password-toggle" :aria-label="revealPassword ? '隐藏密码' : '显示密码'" :aria-pressed="revealPassword" @click="revealPassword = !revealPassword"><EyeOff v-if="revealPassword" :size="16" /><Eye v-else :size="16" /></button>
          </div>
        </div>
        <button
          type="submit"
          class="auth-submit w-full h-12 bg-[rgb(var(--yb-brand))] text-[rgb(var(--yb-text-inverse))] text-sm font-medium rounded-xl transition-all hover:bg-[rgb(var(--yb-brand-strong))] active:scale-[0.98] disabled:opacity-50 disabled:cursor-not-allowed whitespace-nowrap"
          :disabled="isLoading"
        >
          {{ isLoading ? '登录中...' : '登录' }}
        </button>
      </form>

      <!-- 注册表单 -->
      <form v-else class="register-form" :class="{ 'space-y-6': !oaEnabled }" @submit.prevent="handleRegister">
        <div>
          <label for="registerForm.username" class="text-xs font-bold text-[rgb(var(--yb-text-muted))] uppercase tracking-widest mb-2 block whitespace-nowrap">用户名</label>
          <input
            id="registerForm.username" autocomplete="username" v-model="registerForm.username"
            type="text"
            placeholder="请输入用户名"
            class="auth-input w-full h-14 px-4 bg-[rgb(var(--yb-surface-soft))] border-none rounded-xl text-base text-[rgb(var(--yb-text))] placeholder:text-[rgb(var(--yb-text-faint))] whitespace-nowrap"
          />
        </div>
        <div>
          <label for="registerForm.displayName" class="text-xs font-bold text-[rgb(var(--yb-text-muted))] uppercase tracking-widest mb-2 block whitespace-nowrap">姓名</label>
          <input
            id="registerForm.displayName" autocomplete="name" v-model="registerForm.displayName"
            type="text"
            placeholder="请输入真实姓名"
            class="auth-input w-full h-14 px-4 bg-[rgb(var(--yb-surface-soft))] border-none rounded-xl text-base text-[rgb(var(--yb-text))] placeholder:text-[rgb(var(--yb-text-faint))] whitespace-nowrap"
          />
        </div>
        <div>
          <label for="registerForm.password" class="text-xs font-bold text-[rgb(var(--yb-text-muted))] uppercase tracking-widest mb-2 block whitespace-nowrap">密码</label>
          <div class="auth-password"><input
            id="registerForm.password" autocomplete="new-password" v-model="registerForm.password"
            :type="oaEnabled && revealPassword ? 'text' : 'password'"
            placeholder="至少 8 位，含字母和数字"
            class="auth-input w-full h-14 px-4 bg-[rgb(var(--yb-surface-soft))] border-none rounded-xl text-base text-[rgb(var(--yb-text))] placeholder:text-[rgb(var(--yb-text-faint))] whitespace-nowrap"
          />
            <button v-if="oaEnabled" type="button" class="password-toggle" :aria-label="revealPassword ? '隐藏密码' : '显示密码'" :aria-pressed="revealPassword" @click="revealPassword = !revealPassword"><EyeOff v-if="revealPassword" :size="16" /><Eye v-else :size="16" /></button>
          </div>
          <p v-if="!oaEnabled" class="auth-help mt-1 text-xs text-[rgb(var(--yb-text-muted))]">至少 8 位，需包含字母和数字</p>
        </div>
        <div>
          <label for="registerForm.confirmPassword" class="text-xs font-bold text-[rgb(var(--yb-text-muted))] uppercase tracking-widest mb-2 block whitespace-nowrap">确认密码</label>
          <div class="auth-password">
          <input
            id="registerForm.confirmPassword" autocomplete="new-password" v-model="registerForm.confirmPassword"
            :type="oaEnabled && revealConfirmation ? 'text' : 'password'"
            placeholder="请再次输入密码"
            class="auth-input w-full h-14 px-4 bg-[rgb(var(--yb-surface-soft))] border-none rounded-xl text-base text-[rgb(var(--yb-text))] placeholder:text-[rgb(var(--yb-text-faint))] whitespace-nowrap"
          />
            <button v-if="oaEnabled" type="button" class="password-toggle" :aria-label="revealConfirmation ? '隐藏确认密码' : '显示确认密码'" :aria-pressed="revealConfirmation" @click="revealConfirmation = !revealConfirmation">
              <EyeOff v-if="revealConfirmation" :size="16" /><Eye v-else :size="16" />
            </button>
          </div>
          <p v-if="!oaEnabled" class="auth-help mt-1 text-xs text-[rgb(var(--yb-text-muted))]">需与上方密码保持一致</p>
        </div>
        <div>
          <label for="registerForm.mobile" class="text-xs font-bold text-[rgb(var(--yb-text-muted))] uppercase tracking-widest mb-2 block whitespace-nowrap">手机号</label>
          <input
            id="registerForm.mobile" autocomplete="tel" v-model="registerForm.mobile"
            type="tel"
            placeholder="请输入手机号"
            class="auth-input w-full h-14 px-4 bg-[rgb(var(--yb-surface-soft))] border-none rounded-xl text-base text-[rgb(var(--yb-text))] placeholder:text-[rgb(var(--yb-text-faint))] whitespace-nowrap"
          />
        </div>
        <div>
          <label for="registerForm.email" class="text-xs font-bold text-[rgb(var(--yb-text-muted))] uppercase tracking-widest mb-2 block whitespace-nowrap">邮箱（可选）</label>
          <input
            id="registerForm.email" autocomplete="email" v-model="registerForm.email"
            type="email"
            placeholder="请输入邮箱地址"
            class="auth-input w-full h-14 px-4 bg-[rgb(var(--yb-surface-soft))] border-none rounded-xl text-base text-[rgb(var(--yb-text))] placeholder:text-[rgb(var(--yb-text-faint))] whitespace-nowrap"
          />
        </div>
        <div>
          <label for="registerForm.department" class="text-xs font-bold text-[rgb(var(--yb-text-muted))] uppercase tracking-widest mb-2 block whitespace-nowrap">部门</label>
          <select
            id="registerForm.department" v-model="registerForm.department"
            class="auth-input w-full h-14 px-4 bg-[rgb(var(--yb-surface-soft))] border-none rounded-xl text-base text-[rgb(var(--yb-text))] appearance-none cursor-pointer whitespace-nowrap"
            @change="onDepartmentChange"
          >
            <option value="">请选择部门</option>
            <option v-for="dept in registrationDepartments" :key="dept.name" :value="dept.name">
              {{ dept.name }}
            </option>
          </select>
        </div>
        <div>
          <label for="registerForm.team" class="text-xs font-bold text-[rgb(var(--yb-text-muted))] uppercase tracking-widest mb-2 block whitespace-nowrap">小组</label>
          <select
            id="registerForm.team" v-model="registerForm.team"
            :disabled="!availableTeams.length"
            class="auth-input w-full h-14 px-4 bg-[rgb(var(--yb-surface-soft))] border-none rounded-xl text-base text-[rgb(var(--yb-text))] appearance-none cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed whitespace-nowrap"
          >
            <option value="">{{ availableTeams.length ? '请选择小组' : '请先选择部门' }}</option>
            <option v-for="team in availableTeams" :key="team" :value="team">
              {{ team }}
            </option>
          </select>
        </div>
        <button
          type="submit"
          class="auth-submit w-full h-12 bg-[rgb(var(--yb-brand))] text-[rgb(var(--yb-text-inverse))] text-sm font-medium rounded-xl transition-all hover:bg-[rgb(var(--yb-brand-strong))] active:scale-[0.98] disabled:opacity-50 disabled:cursor-not-allowed whitespace-nowrap"
          :disabled="isLoading"
        >
          {{ isLoading ? '注册中...' : '注册' }}
        </button>
      </form>
    </div>
    </div>

    <footer class="auth-footer app-footer app-footer--icp relative z-10 shrink-0 py-4 text-center text-xs text-[rgb(var(--yb-text-faint))]">
      <a
        href="https://beian.miit.gov.cn/"
        target="_blank"
        rel="noopener noreferrer"
        class="inline-flex min-h-8 items-center text-[rgb(var(--yb-text-faint))] underline-offset-2 hover:text-[rgb(var(--yb-text-secondary))] hover:underline"
      >
        苏ICP备2026007026号-1
      </a>
    </footer>
  </div>
</template>

<script setup lang="ts">
import oaBrandMark from '@/oa/assets/yongbo-mark.svg'
import { Eye, EyeOff } from 'lucide-vue-next'
import { oaEnabled } from '@/oa/access'
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { usePermissionsStore } from '@/stores/permissions'
import { resolvePostLoginLandingRoute } from '@/router/home-fallback'
import { authApi } from '@/services/api/authApi'
import { formatAuthPageError } from '@/utils/auth-page-error-zh'

const router = useRouter()
const route = useRoute()
const permissionsStore = usePermissionsStore()

// 当前激活的标签页
const activeTab = ref<'login' | 'register'>('login')

// 加载状态和错误信息
const isLoading = ref(false)
const errorMessage = ref('')

// 登录表单
const revealPassword = ref(false)
const revealConfirmation = ref(false)
watch(activeTab, () => {
  revealPassword.value = false
  revealConfirmation.value = false
})
const loginForm = ref({
  username: '',
  password: '',
})

// 注册表单
const registerForm = ref({
  username: '',
  password: '',
  confirmPassword: '',
  displayName: '',
  mobile: '',
  email: '',
  department: '',
  team: '',
})

// 部门/组选项
const departments = ref<Array<{ name: string; teams?: string[] }>>([])

// 根据选中部门计算可用的组
const availableTeams = computed(() => {
  const dept = departments.value.find(d => d.name === registerForm.value.department)
  return dept?.teams || []
})

// 部门变化时重置组选择
function onDepartmentChange() {
  registerForm.value.team = ''
}

// 页面加载时获取注册选项（兼容后端返回数组 [{ data: { departments } }] 或对象 { data: { departments } }）
const registrationDepartments = computed(() =>
  oaEnabled ? departments.value.filter(dept => dept.name.trim() !== '未分配') : departments.value,
)

onMounted(async () => {
  try {
    const res = await authApi.registerOptions()
    let raw = res.data
    if (Array.isArray(raw) && raw.length > 0) raw = raw[0]
    const data = (raw && typeof raw === 'object' && 'data' in raw
      ? (raw as { data?: { departments?: Array<string | { name: string; teams?: string[] }> } }).data
      : raw) as { departments?: Array<string | { name: string; teams?: string[] }> } | undefined
    departments.value = (data?.departments ?? []).map((dept) =>
      typeof dept === 'string' ? { name: dept, teams: [] } : dept,
    )
  } catch {
    // 静默处理，用户仍然可以手动输入
  }
})

// 处理登录
async function handleLogin() {
  // 表单校验
  if (!loginForm.value.username.trim()) {
    errorMessage.value = '请输入用户名'
    return
  }
  if (!loginForm.value.password) {
    errorMessage.value = '请输入密码'
    return
  }
  if (loginForm.value.password.length < 8) {
    errorMessage.value = '密码至少 8 个字符'
    return
  }

  errorMessage.value = ''
  isLoading.value = true

  try {
    await permissionsStore.loginWithCredentials(
      loginForm.value.username.trim(),
      loginForm.value.password,
    )
    // 登录成功，跳转
    const redirect = (route.query.redirect as string | undefined) ?? undefined
    const landing = resolvePostLoginLandingRoute(router, permissionsStore, redirect)
    await router.push(landing)
  } catch (err: unknown) {
    errorMessage.value = formatAuthPageError(err)
  } finally {
    isLoading.value = false
  }
}

// 处理注册
async function handleRegister() {
  // 表单校验
  if (!registerForm.value.username.trim()) {
    errorMessage.value = '请输入用户名'
    return
  }
  if (!registerForm.value.password) {
    errorMessage.value = '请输入密码'
    return
  }
  if (registerForm.value.password.length < 8) {
    errorMessage.value = '密码至少 8 个字符'
    return
  }
  if (registerForm.value.password !== registerForm.value.confirmPassword) {
    errorMessage.value = '两次输入的密码不一致'
    return
  }
  if (!registerForm.value.displayName.trim()) {
    errorMessage.value = '请输入姓名'
    return
  }
  if (!registerForm.value.mobile.trim()) {
    errorMessage.value = '请输入手机号'
    return
  }
  if (!registerForm.value.department) {
    errorMessage.value = '请选择部门'
    return
  }

  errorMessage.value = ''
  isLoading.value = true

  try {
    // 调用注册接口
    const payload: Parameters<typeof authApi.register>[0] = {
      username: registerForm.value.username.trim(),
      password: registerForm.value.password,
      display_name: registerForm.value.displayName.trim(),
      department: registerForm.value.department,
      team: registerForm.value.team || registerForm.value.department,
      mobile: registerForm.value.mobile.trim(),
      email: registerForm.value.email.trim() || undefined,
    }
    await authApi.register(payload)

    // 注册成功，自动登录
    await permissionsStore.loginWithCredentials(
      registerForm.value.username.trim(),
      registerForm.value.password,
    )

    // 登录成功，跳转
    const redirect = (route.query.redirect as string | undefined) ?? undefined
    const landing = resolvePostLoginLandingRoute(router, permissionsStore, redirect)
    await router.push(landing)
  } catch (err: unknown) {
    errorMessage.value = formatAuthPageError(err)
  } finally {
    isLoading.value = false
  }
}
</script>

<style scoped>
.auth-page {
  background: linear-gradient(
    180deg,
    rgb(var(--yb-bg-page)) 0%,
    rgb(var(--yb-surface-muted)) 48%,
    rgb(var(--yb-brand-wash)) 100%
  );
  color: rgb(var(--yb-text));
}

.auth-watermark {
  color: rgb(var(--yb-text-faint) / 0.25);
}

.auth-card {
  width: min(100%, 480px);
  max-width: min(480px, calc(100vw - 4rem));
  border: 1px solid rgb(var(--yb-border));
  background: rgb(var(--yb-surface));
  box-shadow: 0 10px 40px rgb(var(--yb-shadow) / 0.08);
  backdrop-filter: none;
  -webkit-backdrop-filter: none;
}

.auth-logo {
  background: rgb(var(--yb-brand-soft));
  color: rgb(var(--yb-brand));
  box-shadow: none;
}

.auth-title {
  color: rgb(var(--yb-text));
}

.auth-subtitle,
.auth-help,
.auth-page label,
.auth-footer,
.auth-footer a {
  color: rgb(var(--yb-text-muted));
}

.auth-error {
  border-color: rgb(var(--yb-danger-border));
  background: rgb(var(--yb-danger-soft));
  color: rgb(var(--yb-danger-text));
}

.auth-switch {
  border: 1px solid rgb(var(--yb-border));
  background: rgb(var(--yb-surface-soft));
}

.auth-switch button {
  color: rgb(var(--yb-text-muted));
}

.auth-switch button.is-active {
  background: rgb(var(--yb-brand));
  color: rgb(var(--yb-text-inverse));
  box-shadow: none;
}

:global(#app .auth-input) {
  border: 1px solid rgb(var(--yb-border-strong));
  background: rgb(var(--yb-surface));
  color: rgb(var(--yb-text));
  outline: none;
  transition: background-color 0.2s, border-color 0.2s, box-shadow 0.2s;
}

:global(#app .auth-input::placeholder) {
  color: rgb(var(--yb-text-faint));
}

:global(#app .auth-input:focus) {
  border-color: rgb(var(--yb-brand));
  background: rgb(var(--yb-surface));
  box-shadow: 0 0 0 3px rgb(var(--yb-brand) / 0.12);
}

:global(#app select.auth-input) {
  background-color: rgb(var(--yb-surface));
  background-image:
    linear-gradient(45deg, transparent 50%, rgb(var(--yb-text-muted)) 50%),
    linear-gradient(135deg, rgb(var(--yb-text-muted)) 50%, transparent 50%);
  background-position:
    calc(100% - 18px) 50%,
    calc(100% - 12px) 50%;
  background-repeat: no-repeat;
  background-size:
    6px 6px,
    6px 6px;
  padding-right: 2.5rem;
}

.auth-submit {
  background: rgb(var(--yb-brand));
  color: rgb(var(--yb-text-inverse));
  box-shadow: none;
}

.auth-submit:hover:not(:disabled) {
  background: rgb(var(--yb-brand-strong));
}

.auth-footer a:hover {
  color: rgb(var(--yb-text));
}

::-webkit-scrollbar {
  width: 0;
  height: 0;
}

.oa-auth { background: rgb(var(--yb-bg-page)); }
.oa-auth .auth-card { width: min(100%, 420px); max-width: 100%; margin-block: 0; border-radius: 12px; padding: 24px 32px; }
.oa-auth-register .auth-card { width: min(100%, 620px); }
.oa-auth .auth-card > .text-center { margin-bottom: 16px; }
.oa-auth .auth-logo { width: 36px; height: 36px; margin-bottom: 8px; }
.oa-auth .auth-card > .mb-8 { margin-bottom: 16px; }
.oa-auth .auth-switch { height: 40px; border-radius: 8px; }
.oa-auth .auth-switch button { border-radius: 6px; }
.oa-auth .auth-switch button.is-active { background: rgb(var(--yb-surface)); color: rgb(var(--yb-text)); box-shadow: 0 1px 4px rgb(var(--yb-shadow)/0.12); }
.oa-auth .auth-card .auth-input, .oa-auth .auth-submit { height: 40px; border-radius: 6px; font-size: 14px; }
.oa-auth label { letter-spacing: 0; font-size: 14px; font-weight: 500; margin-bottom: 4px; }
.oa-auth form { display: grid; gap: 16px; }
.oa-auth form > * { margin-block: 0; min-width: 0; }
.oa-auth .auth-env { margin-top: 8px; }
.oa-auth .auth-password { position: relative; }
.oa-auth .auth-password input { padding-inline-end: 44px; }
.password-toggle { position: absolute; inset: 0 0 0 auto; width: 40px; display: grid; place-items: center; color: var(--oa-secondary); }
@media(min-width: 600px) { .oa-auth .register-form { grid-template-columns: repeat(2,minmax(0,1fr)); } .oa-auth .register-form .auth-submit { grid-column: 1/-1; } }
@media(max-width: 599px) { .oa-auth .auth-card { padding: 24px 16px; } }

.oa-auth .auth-heading { display: grid; grid-template-columns: 36px auto; align-items: center; justify-content: center; gap: 4px 8px; }
.oa-auth .auth-heading .auth-title, .oa-auth .auth-heading .auth-logo { margin: 0; }
.oa-auth .auth-formal-note { grid-column: 1/-1; font: 400 var(--oa-meta-size)/1.45 var(--oa-font); color: var(--oa-secondary); margin-block: 4px 0; }
.oa-auth .auth-card { margin: 0; }
.oa-auth .register-form { gap: 12px 16px; }
.oa-auth .register-form label { line-height: 1.45; }

@media (prefers-reduced-motion: no-preference) {
  .oa-auth .auth-card { transition: width 180ms ease; }
}
</style>
