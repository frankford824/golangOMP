<template>
  <div class="auth-page flex flex-col min-h-screen w-full bg-surface overflow-auto font-body">
    <!-- 背景水印：极浅色全局底纹 -->
    <div
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
      <div class="text-center mb-10">
        <div class="auth-logo inline-flex items-center justify-center w-12 h-12 rounded-xl bg-[rgb(var(--yb-brand-soft))] mb-4">
          <span class="material-symbols-outlined text-[rgb(var(--yb-brand))] text-2xl">architecture</span>
        </div>
        <h1 class="auth-title text-2xl font-headline font-extrabold text-[rgb(var(--yb-text))] mb-2 text-center whitespace-nowrap">
          永箔运营管理系统
        </h1>
        <p class="auth-subtitle text-xs text-[rgb(var(--yb-text-faint))] tracking-widest uppercase text-center whitespace-nowrap">
          请使用账号密码登录或注册新账号
        </p>
      </div>

      <!-- 错误提示 -->
      <div
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
      <form v-if="activeTab === 'login'" class="space-y-6" @submit.prevent="handleLogin">
        <div>
          <label class="text-xs font-bold text-[rgb(var(--yb-text-muted))] uppercase tracking-widest mb-2 block whitespace-nowrap">用户名</label>
          <input
            v-model="loginForm.username"
            type="text"
            placeholder="请输入用户名"
            class="auth-input w-full h-14 px-4 bg-[rgb(var(--yb-surface-soft))] border-none rounded-xl text-base text-[rgb(var(--yb-text))] placeholder:text-[rgb(var(--yb-text-faint))] whitespace-nowrap"
          />
        </div>
        <div>
          <label class="text-xs font-bold text-[rgb(var(--yb-text-muted))] uppercase tracking-widest mb-2 block whitespace-nowrap">密码</label>
          <input
            v-model="loginForm.password"
            type="password"
            placeholder="请输入密码"
            class="auth-input w-full h-14 px-4 bg-[rgb(var(--yb-surface-soft))] border-none rounded-xl text-base text-[rgb(var(--yb-text))] placeholder:text-[rgb(var(--yb-text-faint))] whitespace-nowrap"
          />
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
      <form v-else class="space-y-6" @submit.prevent="handleRegister">
        <div>
          <label class="text-xs font-bold text-[rgb(var(--yb-text-muted))] uppercase tracking-widest mb-2 block whitespace-nowrap">用户名</label>
          <input
            v-model="registerForm.username"
            type="text"
            placeholder="请输入用户名"
            class="auth-input w-full h-14 px-4 bg-[rgb(var(--yb-surface-soft))] border-none rounded-xl text-base text-[rgb(var(--yb-text))] placeholder:text-[rgb(var(--yb-text-faint))] whitespace-nowrap"
          />
        </div>
        <div>
          <label class="text-xs font-bold text-[rgb(var(--yb-text-muted))] uppercase tracking-widest mb-2 block whitespace-nowrap">密码</label>
          <input
            v-model="registerForm.password"
            type="password"
            placeholder="请输入密码"
            class="auth-input w-full h-14 px-4 bg-[rgb(var(--yb-surface-soft))] border-none rounded-xl text-base text-[rgb(var(--yb-text))] placeholder:text-[rgb(var(--yb-text-faint))] whitespace-nowrap"
          />
          <p class="auth-help mt-1 text-xs text-[rgb(var(--yb-text-muted))]">至少 8 位，需包含字母和数字</p>
        </div>
        <div>
          <label class="text-xs font-bold text-[rgb(var(--yb-text-muted))] uppercase tracking-widest mb-2 block whitespace-nowrap">确认密码</label>
          <input
            v-model="registerForm.confirmPassword"
            type="password"
            placeholder="请再次输入密码"
            class="auth-input w-full h-14 px-4 bg-[rgb(var(--yb-surface-soft))] border-none rounded-xl text-base text-[rgb(var(--yb-text))] placeholder:text-[rgb(var(--yb-text-faint))] whitespace-nowrap"
          />
          <p class="auth-help mt-1 text-xs text-[rgb(var(--yb-text-muted))]">需与上方密码保持一致</p>
        </div>
        <div>
          <label class="text-xs font-bold text-[rgb(var(--yb-text-muted))] uppercase tracking-widest mb-2 block whitespace-nowrap">姓名</label>
          <input
            v-model="registerForm.displayName"
            type="text"
            placeholder="请输入真实姓名"
            class="auth-input w-full h-14 px-4 bg-[rgb(var(--yb-surface-soft))] border-none rounded-xl text-base text-[rgb(var(--yb-text))] placeholder:text-[rgb(var(--yb-text-faint))] whitespace-nowrap"
          />
        </div>
        <div>
          <label class="text-xs font-bold text-[rgb(var(--yb-text-muted))] uppercase tracking-widest mb-2 block whitespace-nowrap">手机号</label>
          <input
            v-model="registerForm.mobile"
            type="tel"
            placeholder="请输入手机号"
            class="auth-input w-full h-14 px-4 bg-[rgb(var(--yb-surface-soft))] border-none rounded-xl text-base text-[rgb(var(--yb-text))] placeholder:text-[rgb(var(--yb-text-faint))] whitespace-nowrap"
          />
        </div>
        <div>
          <label class="text-xs font-bold text-[rgb(var(--yb-text-muted))] uppercase tracking-widest mb-2 block whitespace-nowrap">邮箱（可选）</label>
          <input
            v-model="registerForm.email"
            type="email"
            placeholder="请输入邮箱地址"
            class="auth-input w-full h-14 px-4 bg-[rgb(var(--yb-surface-soft))] border-none rounded-xl text-base text-[rgb(var(--yb-text))] placeholder:text-[rgb(var(--yb-text-faint))] whitespace-nowrap"
          />
        </div>
        <div>
          <label class="text-xs font-bold text-[rgb(var(--yb-text-muted))] uppercase tracking-widest mb-2 block whitespace-nowrap">部门</label>
          <select
            v-model="registerForm.department"
            class="auth-input w-full h-14 px-4 bg-[rgb(var(--yb-surface-soft))] border-none rounded-xl text-base text-[rgb(var(--yb-text))] appearance-none cursor-pointer whitespace-nowrap"
            @change="onDepartmentChange"
          >
            <option value="">请选择部门</option>
            <option v-for="dept in departments" :key="dept.name" :value="dept.name">
              {{ dept.name }}
            </option>
          </select>
        </div>
        <div>
          <label class="text-xs font-bold text-[rgb(var(--yb-text-muted))] uppercase tracking-widest mb-2 block whitespace-nowrap">组</label>
          <select
            v-model="registerForm.team"
            :disabled="!availableTeams.length"
            class="auth-input w-full h-14 px-4 bg-[rgb(var(--yb-surface-soft))] border-none rounded-xl text-base text-[rgb(var(--yb-text))] appearance-none cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed whitespace-nowrap"
          >
            <option value="">{{ availableTeams.length ? '请选择组' : '请先选择部门' }}</option>
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
        class="text-[rgb(var(--yb-text-faint))] underline-offset-2 hover:text-[rgb(var(--yb-text-secondary))] hover:underline"
      >
        苏ICP备2026007026号-1
      </a>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
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
    router.push(landing)
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
    router.push(landing)
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
</style>
