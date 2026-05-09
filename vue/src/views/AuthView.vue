<template>
  <div class="flex flex-col min-h-screen w-full bg-surface overflow-auto font-body">
    <!-- 背景水印：极浅色全局底纹 -->
    <div
      class="fixed inset-0 pointer-events-none select-none overflow-hidden flex items-center justify-center"
      aria-hidden="true"
    >
      <span class="font-black text-[20rem] text-slate-200/30 leading-none">YONGBO</span>
    </div>

    <div class="relative z-10 flex flex-1 flex-col items-center justify-center px-4 py-8">
    <!-- 居中卡片 -->
    <div
      class="relative z-10 w-full max-w-[480px] rounded-[2rem] p-12 my-8"
      style="background: rgba(255, 255, 255, 0.9); backdrop-filter: blur(20px); border: 1px solid rgba(255, 255, 255, 0.7); box-shadow: 0 12px 48px -12px rgba(28, 25, 23, 0.08);"
    >
      <!-- 顶部 Header -->
      <div class="text-center mb-10">
        <div class="inline-flex items-center justify-center w-12 h-12 rounded-xl bg-stone-200/80 mb-4">
          <span class="material-symbols-outlined text-stone-600 text-2xl">architecture</span>
        </div>
        <h1 class="text-2xl font-headline font-extrabold text-slate-900 mb-2 text-center whitespace-nowrap">
          永箔运营管理系统
        </h1>
        <p class="text-xs text-slate-400 tracking-widest uppercase text-center whitespace-nowrap">
          请使用账号密码登录或注册新账号
        </p>
      </div>

      <!-- 错误提示 -->
      <div
        v-if="errorMessage"
        class="mb-6 p-3 bg-red-50 border border-red-200 rounded-xl text-sm text-red-600 break-words leading-relaxed"
      >
        {{ errorMessage }}
      </div>

      <!-- Switcher：w-full 胶囊 h-12，选中项白色浮起 -->
      <div class="mb-8">
        <div class="bg-slate-100 rounded-full p-1 flex h-12 w-full">
          <button
            type="button"
            class="flex-1 h-full text-sm font-medium rounded-full transition-all duration-200 whitespace-nowrap"
            :class="activeTab === 'login'
              ? 'bg-white text-slate-900 shadow-sm'
              : 'bg-transparent text-slate-500 hover:text-slate-700'
            "
            @click="activeTab = 'login'"
          >
            登录
          </button>
          <button
            type="button"
            class="flex-1 h-full text-sm font-medium rounded-full transition-all duration-200 whitespace-nowrap"
            :class="activeTab === 'register'
              ? 'bg-white text-slate-900 shadow-sm'
              : 'bg-transparent text-slate-500 hover:text-slate-700'
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
          <label class="text-xs font-bold text-slate-500 uppercase tracking-widest mb-2 block whitespace-nowrap">用户名</label>
          <input
            v-model="loginForm.username"
            type="text"
            placeholder="请输入用户名"
            class="auth-input w-full h-14 px-4 bg-slate-50 border-none rounded-xl text-base text-slate-900 placeholder:text-slate-400 whitespace-nowrap"
          />
        </div>
        <div>
          <label class="text-xs font-bold text-slate-500 uppercase tracking-widest mb-2 block whitespace-nowrap">密码</label>
          <input
            v-model="loginForm.password"
            type="password"
            placeholder="请输入密码"
            class="auth-input w-full h-14 px-4 bg-slate-50 border-none rounded-xl text-base text-slate-900 placeholder:text-slate-400 whitespace-nowrap"
          />
        </div>
        <button
          type="submit"
          class="w-full h-12 bg-slate-900 text-white text-sm font-medium rounded-xl transition-all hover:bg-slate-800 active:scale-[0.98] disabled:opacity-50 disabled:cursor-not-allowed whitespace-nowrap"
          :disabled="isLoading"
        >
          {{ isLoading ? '登录中...' : '登录' }}
        </button>
      </form>

      <!-- 注册表单 -->
      <form v-else class="space-y-6" @submit.prevent="handleRegister">
        <div>
          <label class="text-xs font-bold text-slate-500 uppercase tracking-widest mb-2 block whitespace-nowrap">用户名</label>
          <input
            v-model="registerForm.username"
            type="text"
            placeholder="请输入用户名"
            class="auth-input w-full h-14 px-4 bg-slate-50 border-none rounded-xl text-base text-slate-900 placeholder:text-slate-400 whitespace-nowrap"
          />
        </div>
        <div>
          <label class="text-xs font-bold text-slate-500 uppercase tracking-widest mb-2 block whitespace-nowrap">密码</label>
          <input
            v-model="registerForm.password"
            type="password"
            placeholder="请输入密码"
            class="auth-input w-full h-14 px-4 bg-slate-50 border-none rounded-xl text-base text-slate-900 placeholder:text-slate-400 whitespace-nowrap"
          />
          <p class="mt-1 text-xs text-slate-500">至少 8 位，需包含字母和数字</p>
        </div>
        <div>
          <label class="text-xs font-bold text-slate-500 uppercase tracking-widest mb-2 block whitespace-nowrap">确认密码</label>
          <input
            v-model="registerForm.confirmPassword"
            type="password"
            placeholder="请再次输入密码"
            class="auth-input w-full h-14 px-4 bg-slate-50 border-none rounded-xl text-base text-slate-900 placeholder:text-slate-400 whitespace-nowrap"
          />
          <p class="mt-1 text-xs text-slate-500">需与上方密码保持一致</p>
        </div>
        <div>
          <label class="text-xs font-bold text-slate-500 uppercase tracking-widest mb-2 block whitespace-nowrap">姓名</label>
          <input
            v-model="registerForm.displayName"
            type="text"
            placeholder="请输入真实姓名"
            class="auth-input w-full h-14 px-4 bg-slate-50 border-none rounded-xl text-base text-slate-900 placeholder:text-slate-400 whitespace-nowrap"
          />
        </div>
        <div>
          <label class="text-xs font-bold text-slate-500 uppercase tracking-widest mb-2 block whitespace-nowrap">手机号</label>
          <input
            v-model="registerForm.mobile"
            type="tel"
            placeholder="请输入手机号"
            class="auth-input w-full h-14 px-4 bg-slate-50 border-none rounded-xl text-base text-slate-900 placeholder:text-slate-400 whitespace-nowrap"
          />
        </div>
        <div>
          <label class="text-xs font-bold text-slate-500 uppercase tracking-widest mb-2 block whitespace-nowrap">邮箱（可选）</label>
          <input
            v-model="registerForm.email"
            type="email"
            placeholder="请输入邮箱地址"
            class="auth-input w-full h-14 px-4 bg-slate-50 border-none rounded-xl text-base text-slate-900 placeholder:text-slate-400 whitespace-nowrap"
          />
        </div>
        <div>
          <label class="text-xs font-bold text-slate-500 uppercase tracking-widest mb-2 block whitespace-nowrap">部门</label>
          <select
            v-model="registerForm.department"
            class="auth-input w-full h-14 px-4 bg-slate-50 border-none rounded-xl text-base text-slate-900 appearance-none cursor-pointer whitespace-nowrap"
            @change="onDepartmentChange"
          >
            <option value="">请选择部门</option>
            <option v-for="dept in departments" :key="dept.name" :value="dept.name">
              {{ dept.name }}
            </option>
          </select>
        </div>
        <div>
          <label class="text-xs font-bold text-slate-500 uppercase tracking-widest mb-2 block whitespace-nowrap">组</label>
          <select
            v-model="registerForm.team"
            :disabled="!availableTeams.length"
            class="auth-input w-full h-14 px-4 bg-slate-50 border-none rounded-xl text-base text-slate-900 appearance-none cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed whitespace-nowrap"
          >
            <option value="">{{ availableTeams.length ? '请选择组' : '请先选择部门' }}</option>
            <option v-for="team in availableTeams" :key="team" :value="team">
              {{ team }}
            </option>
          </select>
        </div>
        <div>
          <label class="text-xs font-bold text-slate-500 uppercase tracking-widest mb-2 block whitespace-nowrap">管理员密钥（可选）</label>
          <input
            v-model="registerForm.adminKey"
            type="password"
            placeholder="仅部门管理员注册时填写"
            class="auth-input w-full h-14 px-4 bg-slate-50 border-none rounded-xl text-base text-slate-900 placeholder:text-slate-400 whitespace-nowrap"
          />
        </div>
        <button
          type="submit"
          class="w-full h-12 bg-slate-900 text-white text-sm font-medium rounded-xl transition-all hover:bg-slate-800 active:scale-[0.98] disabled:opacity-50 disabled:cursor-not-allowed whitespace-nowrap"
          :disabled="isLoading"
        >
          {{ isLoading ? '注册中...' : '注册' }}
        </button>
      </form>
    </div>
    </div>

    <footer class="app-footer app-footer--icp relative z-10 shrink-0 py-4 text-center text-xs text-slate-400">
      <a
        href="https://beian.miit.gov.cn/"
        target="_blank"
        rel="noopener noreferrer"
        class="text-slate-400 underline-offset-2 hover:text-slate-600 hover:underline"
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
  adminKey: '',
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
    if (registerForm.value.adminKey?.trim()) {
      payload.admin_key = registerForm.value.adminKey.trim()
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
/* 输入框 focus：背景微变 + 轻微阴影 */
.auth-input {
  outline: none;
  transition: background-color 0.2s, box-shadow 0.2s;
}
.auth-input:focus {
  background-color: rgb(241 245 249); /* slate-100，略深于默认 slate-50 */
  box-shadow: 0 0 0 2px rgb(15 23 42 / 0.06), 0 1px 2px 0 rgb(0 0 0 / 0.05);
}

/* 隐藏 select 默认箭头，使用自定义样式 */
select.auth-input {
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' fill='none' viewBox='0 0 20 20'%3E%3Cpath stroke='%236B7280' stroke-linecap='round' stroke-linejoin='round' stroke-width='1.5' d='m6 8 4 4 4-4'/%3E%3C/svg%3E");
  background-position: right 1rem center;
  background-repeat: no-repeat;
  background-size: 1.25rem;
  padding-right: 2.5rem;
}

/* 隐藏滚动条但保持可滚动 */
::-webkit-scrollbar {
  width: 0;
  height: 0;
}
</style>
