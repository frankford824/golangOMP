<template>
  <section class="space-y-4">
    <div class="rounded-xl border border-[var(--v1-border)] bg-white p-4">
      <div class="flex items-center justify-between">
        <div>
          <h1 class="text-base font-semibold text-[var(--v1-text-primary)]">个人中心</h1>
          <p class="mt-1 text-xs text-[var(--v1-text-secondary)]">
            当前登录：{{ user?.name ?? '-' }} · 角色：{{ roleLabel }}
          </p>
        </div>
        <button
          type="button"
          class="rounded-md border border-[var(--v1-border)] bg-white px-3 py-1 text-xs text-[var(--v1-text-secondary)]"
          @click="logout"
        >
          退出登录
        </button>
      </div>
    </div>

    <div class="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">
      <router-link
        v-for="panel in panels"
        :key="panel.to"
        :to="panel.to"
        class="block rounded-xl border border-[var(--v1-border)] bg-white p-4 transition hover:border-[var(--v1-bg-primary)]"
      >
        <p class="text-sm font-semibold text-[var(--v1-text-primary)]">{{ panel.title }}</p>
        <p class="mt-1 text-xs text-[var(--v1-text-secondary)]">{{ panel.desc }}</p>
      </router-link>
    </div>

    <section class="rounded-xl border border-[var(--v1-border)] bg-white p-4">
      <h2 class="text-sm font-semibold text-[var(--v1-text-primary)]">账号资料</h2>
      <div class="mt-3 grid grid-cols-1 gap-2 md:grid-cols-2">
        <input
          v-model="nickname"
          class="rounded border border-[var(--v1-border)] px-2 py-1.5 text-sm"
          placeholder="昵称"
        />
        <input
          v-model="phone"
          class="rounded border border-[var(--v1-border)] px-2 py-1.5 text-sm"
          placeholder="手机号"
        />
        <input
          v-model="email"
          class="rounded border border-[var(--v1-border)] px-2 py-1.5 text-sm md:col-span-2"
          placeholder="邮箱"
        />
      </div>
      <div class="mt-3 flex justify-end">
        <button
          type="button"
          class="rounded bg-[var(--v1-bg-primary)] px-3 py-1 text-xs text-white"
          :disabled="saving"
          @click="saveProfile"
        >
          {{ saving ? '保存中...' : '保存' }}
        </button>
      </div>
      <p v-if="message" class="mt-2 text-xs text-[var(--v1-text-secondary)]">{{ message }}</p>
    </section>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { usePermissionsStore } from '@/stores/permissions'
import { useRouter } from 'vue-router'
import { meApi } from '@/services/api/meApi'
import { resolveApiUserMessage } from '@/utils/api-message-zh'
import type { MeProfile } from '@/services/v1Types'
import { formatUserRoleForDisplay } from '@/domain/user-workflow-roles'

interface Panel {
  title: string
  desc: string
  to: string
}

const permissionsStore = usePermissionsStore()
const user = computed(() => permissionsStore.currentUser)
const roleLabel = computed(() => formatUserRoleForDisplay(user.value?.role))
const router = useRouter()

const nickname = ref(user.value?.name ?? '')
const phone = ref('')
const email = ref('')
const saving = ref(false)
const message = ref('')

const panels: Panel[] = [
  { title: '账号资料', to: '/me', desc: '昵称、联系方式等基础资料' },
  { title: '安全设置', to: '/me/security', desc: '修改密码，保护账户安全' },
  { title: '我的组织', to: '/me/org', desc: '查看部门、团队与角色信息' },
  { title: '通知中心', to: '/me/notifications', desc: '查看系统通知与已读状态' },
  { title: '我的草稿', to: '/me/task-drafts', desc: '管理未提交的任务草稿（≤20 条）' },
  { title: '我的任务', to: '/tasks?tab=mine', desc: '进入「任务中心 → 我的任务」' },
]

function unwrapProfile(raw: unknown): MeProfile | null {
  if (!raw || typeof raw !== 'object') return null
  const root = raw as { data?: unknown }
  const payload = root.data && typeof root.data === 'object' ? root.data : raw
  return payload as MeProfile
}

function applyProfile(profile: MeProfile | null): void {
  if (!profile) return
  nickname.value = profile.nickname ?? profile.display_name ?? user.value?.name ?? ''
  phone.value = profile.phone ?? profile.mobile ?? ''
  email.value = profile.email ?? ''
}

async function loadProfile(): Promise<void> {
  try {
    const res = await meApi.getProfile()
    applyProfile(unwrapProfile(res.data))
  } catch (error) {
    message.value = resolveApiUserMessage(error, { fallback: '账号资料加载失败' })
  }
}

async function saveProfile(): Promise<void> {
  saving.value = true
  message.value = ''
  try {
    const res = await meApi.patchProfile({
      nickname: nickname.value.trim(),
      phone: phone.value.trim(),
      email: email.value.trim(),
    })
    applyProfile(unwrapProfile(res.data))
    message.value = '账号资料已保存'
  } catch (error) {
    message.value = resolveApiUserMessage(error, { fallback: '账号资料保存失败' })
  } finally {
    saving.value = false
  }
}

watch(user, () => {
  if (!nickname.value) nickname.value = user.value?.name ?? ''
})

onMounted(loadProfile)

function logout(): void {
  permissionsStore.logout()
  void router.push('/login')
}
</script>
