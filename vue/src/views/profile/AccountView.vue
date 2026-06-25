<template>
  <section class="profile-workbench">
    <header class="profile-hero">
      <div class="profile-hero__avatar">
        <img v-if="avatarUrl" :src="avatarUrl" alt="头像" />
        <span v-else>{{ avatarInitial }}</span>
      </div>
      <div class="profile-hero__main">
        <p class="profile-eyebrow">{{ roleLabel }}</p>
        <h1>{{ displayName || '未设置展示名称' }}</h1>
        <p>{{ accountLabel || '账号信息待同步' }}</p>
      </div>
      <button type="button" class="profile-logout" @click="logout">
        <LogOut :size="15" aria-hidden="true" />
        退出登录
      </button>
    </header>

    <div class="profile-grid">
      <section class="profile-panel profile-panel--avatar">
        <div class="section-heading">
          <div>
            <h2>头像</h2>
            <p>用于右上角菜单和个人资料展示</p>
          </div>
          <Camera :size="20" aria-hidden="true" />
        </div>

        <div class="avatar-editor">
          <div class="avatar-editor__preview">
            <img v-if="avatarUrl" :src="avatarUrl" alt="当前头像" />
            <span v-else>{{ avatarInitial }}</span>
          </div>
          <div class="avatar-editor__actions">
            <input
              ref="avatarInputRef"
              class="sr-only"
              type="file"
              accept="image/jpeg,image/png,image/webp"
              @change="handleAvatarSelected"
            />
            <button type="button" class="profile-button profile-button--primary" :disabled="avatarUploading" @click="openAvatarPicker">
              <Upload :size="15" aria-hidden="true" />
              {{ avatarUploading ? '上传中' : '上传头像' }}
            </button>
            <button
              type="button"
              class="profile-button profile-button--ghost"
              :disabled="avatarDeleting || !avatarUrl"
              @click="deleteAvatar"
            >
              <Trash2 :size="15" aria-hidden="true" />
              移除
            </button>
          </div>
        </div>
      </section>

      <section class="profile-panel">
        <div class="section-heading">
          <div>
            <h2>账号资料</h2>
            <p>登录账号只读，展示资料可维护</p>
          </div>
          <UserRound :size="20" aria-hidden="true" />
        </div>

        <div class="readonly-grid">
          <div class="readonly-item">
            <span>登录账号</span>
            <strong>{{ accountLabel || '-' }}</strong>
          </div>
          <div class="readonly-item">
            <span>当前角色</span>
            <strong>{{ roleLabel }}</strong>
          </div>
          <div class="readonly-item">
            <span>所属部门</span>
            <strong>{{ departmentLabel }}</strong>
          </div>
          <div class="readonly-item">
            <span>所属团队</span>
            <strong>{{ teamLabel }}</strong>
          </div>
        </div>
      </section>
    </div>

    <section class="profile-panel">
      <div class="section-heading">
        <div>
          <h2>展示信息</h2>
          <p>保存后会同步到系统顶部名称</p>
        </div>
        <Save :size="20" aria-hidden="true" />
      </div>

      <form class="profile-form" @submit.prevent="saveProfile">
        <label class="profile-field">
          <span>展示名称</span>
          <input v-model="displayNameInput" autocomplete="name" placeholder="请输入展示名称" />
        </label>
        <label class="profile-field">
          <span>手机号</span>
          <input v-model="mobileInput" inputmode="tel" autocomplete="tel" placeholder="请输入手机号" />
        </label>
        <label class="profile-field profile-field--wide">
          <span>邮箱</span>
          <input v-model="emailInput" type="email" autocomplete="email" placeholder="请输入邮箱" />
        </label>

        <div class="profile-form__footer">
          <p v-if="feedbackText" :class="feedbackClass">{{ feedbackText }}</p>
          <span v-else aria-hidden="true"></span>
          <button type="submit" class="profile-button profile-button--primary" :disabled="saving || loading">
            <Save :size="15" aria-hidden="true" />
            {{ saving ? '保存中' : '保存资料' }}
          </button>
        </div>
      </form>
    </section>

    <nav class="profile-shortcuts" aria-label="个人中心快捷入口">
      <router-link v-for="panel in panels" :key="panel.to" :to="panel.to">
        <span>{{ panel.title }}</span>
        <small>{{ panel.desc }}</small>
      </router-link>
    </nav>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { usePermissionsStore } from '@/stores/permissions'
import { useRouter } from 'vue-router'
import { Camera, LogOut, Save, Trash2, Upload, UserRound } from 'lucide-vue-next'
import { meApi } from '@/services/api/meApi'
import { resolveApiUserMessage } from '@/utils/api-message-zh'
import type { MeProfile } from '@/services/v1Types'
import { formatUserRoleForDisplay } from '@/domain/user-workflow-roles'

interface Panel {
  title: string
  desc: string
  to: string
}

const maxAvatarBytes = 2 * 1024 * 1024
const allowedAvatarTypes = new Set(['image/jpeg', 'image/png', 'image/webp'])

const permissionsStore = usePermissionsStore()
const router = useRouter()
const user = computed(() => permissionsStore.currentUser)

const profile = ref<MeProfile | null>(null)
const avatarInputRef = ref<HTMLInputElement | null>(null)
const displayNameInput = ref('')
const mobileInput = ref('')
const emailInput = ref('')
const loading = ref(false)
const saving = ref(false)
const avatarUploading = ref(false)
const avatarDeleting = ref(false)
const feedbackText = ref('')
const feedbackKind = ref<'success' | 'error' | 'muted'>('muted')

const roleLabel = computed(() => formatUserRoleForDisplay(user.value?.role))
const displayName = computed(() => profile.value?.display_name || profile.value?.name || user.value?.name || '')
const avatarUrl = computed(() => profile.value?.avatar_url || profile.value?.avatar || user.value?.avatarUrl || user.value?.avatar || '')
const avatarInitial = computed(() => (displayName.value || accountLabel.value || '我').trim().slice(0, 1).toUpperCase())
const accountLabel = computed(() => profile.value?.account || profile.value?.username || user.value?.account || user.value?.username || '')
const departmentLabel = computed(() => profile.value?.department || permissionsStore.actorDepartment || user.value?.departmentId || '-')
const teamLabel = computed(() => profile.value?.team || permissionsStore.actorTeam || user.value?.groupId || '-')
const feedbackClass = computed(() => ({
  'profile-feedback': true,
  'profile-feedback--success': feedbackKind.value === 'success',
  'profile-feedback--error': feedbackKind.value === 'error',
}))

const panels: Panel[] = [
  { title: '安全设置', to: '/me/security', desc: '密码与登录安全' },
  { title: '我的组织', to: '/me/org', desc: '部门、团队与角色' },
  { title: '通知中心', to: '/me/notifications', desc: '系统消息' },
  { title: '我的草稿', to: '/me/task-drafts', desc: '未提交任务' },
  { title: '我的任务', to: '/tasks?tab=mine', desc: '任务状态筛选' },
]

function unwrapProfile(raw: unknown): MeProfile | null {
  if (!raw || typeof raw !== 'object') return null
  const root = raw as { data?: unknown }
  const payload = root.data && typeof root.data === 'object' ? root.data : raw
  return payload as MeProfile
}

function setFeedback(text: string, kind: 'success' | 'error' | 'muted' = 'muted'): void {
  feedbackText.value = text
  feedbackKind.value = kind
}

function syncCurrentUser(nextProfile: MeProfile): void {
  const current = user.value
  if (!current) return
  const nextName = nextProfile.display_name || nextProfile.name || current.name
  const nextAvatar = nextProfile.avatar_url || nextProfile.avatar || ''
  permissionsStore.setCurrentUser({
    ...current,
    name: nextName,
    account: nextProfile.account || nextProfile.username || current.account,
    username: nextProfile.username || current.username,
    avatar: nextAvatar || undefined,
    avatarUrl: nextAvatar || undefined,
  })
}

function applyProfile(nextProfile: MeProfile | null): void {
  if (!nextProfile) return
  profile.value = nextProfile
  displayNameInput.value = nextProfile.display_name || nextProfile.name || user.value?.name || ''
  mobileInput.value = nextProfile.mobile || nextProfile.phone || ''
  emailInput.value = nextProfile.email || ''
  syncCurrentUser(nextProfile)
}

async function loadProfile(): Promise<void> {
  loading.value = true
  setFeedback('', 'muted')
  try {
    const res = await meApi.getProfile()
    applyProfile(unwrapProfile(res.data))
  } catch (error) {
    setFeedback(resolveApiUserMessage(error, { fallback: '账号资料加载失败' }), 'error')
  } finally {
    loading.value = false
  }
}

async function saveProfile(): Promise<void> {
  saving.value = true
  setFeedback('', 'muted')
  try {
    const res = await meApi.patchProfile({
      display_name: displayNameInput.value.trim(),
      mobile: mobileInput.value.trim(),
      email: emailInput.value.trim(),
    })
    applyProfile(unwrapProfile(res.data))
    setFeedback('账号资料已保存', 'success')
  } catch (error) {
    setFeedback(resolveApiUserMessage(error, { fallback: '账号资料保存失败' }), 'error')
  } finally {
    saving.value = false
  }
}

function openAvatarPicker(): void {
  avatarInputRef.value?.click()
}

async function handleAvatarSelected(event: Event): Promise<void> {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  if (!allowedAvatarTypes.has(file.type)) {
    setFeedback('头像仅支持 JPG、PNG 或 WebP', 'error')
    return
  }
  if (file.size > maxAvatarBytes) {
    setFeedback('头像大小不能超过 2MB', 'error')
    return
  }
  avatarUploading.value = true
  setFeedback('', 'muted')
  try {
    const res = await meApi.uploadAvatar(file)
    applyProfile(unwrapProfile(res.data))
    setFeedback('头像已更新', 'success')
  } catch (error) {
    setFeedback(resolveApiUserMessage(error, { fallback: '头像上传失败' }), 'error')
  } finally {
    avatarUploading.value = false
  }
}

async function deleteAvatar(): Promise<void> {
  if (!avatarUrl.value) return
  avatarDeleting.value = true
  setFeedback('', 'muted')
  try {
    const res = await meApi.deleteAvatar()
    applyProfile(unwrapProfile(res.data))
    setFeedback('头像已移除', 'success')
  } catch (error) {
    setFeedback(resolveApiUserMessage(error, { fallback: '头像移除失败' }), 'error')
  } finally {
    avatarDeleting.value = false
  }
}

watch(user, () => {
  if (!profile.value) {
    displayNameInput.value = user.value?.name ?? ''
  }
})

onMounted(loadProfile)

function logout(): void {
  permissionsStore.logout()
  void router.push('/login')
}
</script>

<style scoped>
.profile-workbench {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.profile-hero,
.profile-panel,
.profile-shortcuts {
  border: 1px solid var(--v1-border);
  background: #fff;
  box-shadow: 0 16px 36px -28px rgba(15, 23, 42, 0.28);
}

.profile-hero {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 16px;
  min-height: 124px;
  padding: 22px;
  border-radius: 18px;
  background:
    linear-gradient(135deg, rgba(239, 246, 255, 0.92), rgba(255, 255, 255, 0.96) 48%),
    #fff;
}

.profile-hero__avatar,
.avatar-editor__preview {
  display: grid;
  place-items: center;
  overflow: hidden;
  border-radius: 50%;
  background: linear-gradient(135deg, #2563eb, #14b8a6);
  color: #fff;
  font-weight: 800;
}

.profile-hero__avatar {
  width: 76px;
  height: 76px;
  font-size: 28px;
}

.profile-hero__avatar img,
.avatar-editor__preview img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.profile-hero__main {
  min-width: 0;
}

.profile-eyebrow {
  margin: 0;
  font-size: 12px;
  font-weight: 700;
  color: #2563eb;
}

.profile-hero h1 {
  margin: 4px 0 0;
  font-size: 24px;
  font-weight: 800;
  line-height: 1.25;
  color: var(--v1-text-primary);
  letter-spacing: 0;
}

.profile-hero__main p:last-child {
  margin: 6px 0 0;
  font-size: 13px;
  color: var(--v1-text-secondary);
}

.profile-logout,
.profile-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  border-radius: 8px;
  font-size: 13px;
  font-weight: 700;
  line-height: 1;
  transition:
    background-color 0.15s ease,
    border-color 0.15s ease,
    color 0.15s ease;
}

.profile-logout {
  height: 34px;
  border: 1px solid #e5e7eb;
  background: #fff;
  padding: 0 12px;
  color: #334155;
}

.profile-logout:hover,
.profile-logout:focus-visible {
  border-color: #cbd5e1;
  background: #f8fafc;
  outline: none;
}

.profile-grid {
  display: grid;
  grid-template-columns: minmax(280px, 0.82fr) minmax(0, 1.18fr);
  gap: 16px;
}

.profile-panel {
  border-radius: 16px;
  padding: 18px;
}

.section-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  color: #64748b;
}

.section-heading h2 {
  margin: 0;
  font-size: 16px;
  font-weight: 800;
  color: var(--v1-text-primary);
}

.section-heading p {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--v1-text-secondary);
}

.avatar-editor {
  display: flex;
  align-items: center;
  gap: 18px;
  margin-top: 18px;
}

.avatar-editor__preview {
  width: 88px;
  height: 88px;
  flex: 0 0 auto;
  font-size: 30px;
}

.avatar-editor__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.profile-button {
  min-height: 36px;
  border: 1px solid transparent;
  padding: 0 13px;
  cursor: pointer;
}

.profile-button:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}

.profile-button--primary {
  background: #2563eb;
  color: #fff;
}

.profile-button--primary:hover:not(:disabled),
.profile-button--primary:focus-visible:not(:disabled) {
  background: #1d4ed8;
  outline: none;
}

.profile-button--ghost {
  border-color: #e2e8f0;
  background: #fff;
  color: #475569;
}

.profile-button--ghost:hover:not(:disabled),
.profile-button--ghost:focus-visible:not(:disabled) {
  border-color: #cbd5e1;
  background: #f8fafc;
  outline: none;
}

.readonly-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  margin-top: 18px;
}

.readonly-item {
  min-width: 0;
  border-radius: 10px;
  background: #f8fafc;
  padding: 12px;
}

.readonly-item span {
  display: block;
  font-size: 12px;
  color: var(--v1-text-secondary);
}

.readonly-item strong {
  display: block;
  overflow: hidden;
  margin-top: 5px;
  color: var(--v1-text-primary);
  font-size: 14px;
  font-weight: 800;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.profile-form {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
  margin-top: 18px;
}

.profile-field {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 7px;
}

.profile-field--wide,
.profile-form__footer {
  grid-column: 1 / -1;
}

.profile-field span {
  font-size: 12px;
  font-weight: 700;
  color: #475569;
}

.profile-field input {
  height: 40px;
  width: 100%;
  border: 1px solid #dbe3ef;
  border-radius: 8px;
  background: #fff;
  padding: 0 12px;
  color: var(--v1-text-primary);
  font-size: 14px;
}

.profile-field input:focus {
  border-color: #2563eb;
  box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.12);
  outline: none;
}

.profile-form__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.profile-feedback {
  margin: 0;
  font-size: 13px;
  font-weight: 700;
  color: var(--v1-text-secondary);
}

.profile-feedback--success {
  color: #047857;
}

.profile-feedback--error {
  color: var(--v1-danger);
}

.profile-shortcuts {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 10px;
  border-radius: 16px;
  padding: 12px;
}

.profile-shortcuts a {
  min-width: 0;
  border-radius: 10px;
  padding: 10px 12px;
  color: var(--v1-text-primary);
  text-decoration: none;
}

.profile-shortcuts a:hover,
.profile-shortcuts a:focus-visible,
.profile-shortcuts a.router-link-active {
  background: #eff6ff;
  color: #1d4ed8;
  outline: none;
}

.profile-shortcuts span,
.profile-shortcuts small {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.profile-shortcuts span {
  font-size: 13px;
  font-weight: 800;
}

.profile-shortcuts small {
  margin-top: 3px;
  font-size: 11px;
  color: var(--v1-text-secondary);
}

.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
}

@media (max-width: 920px) {
  .profile-grid,
  .profile-form,
  .profile-shortcuts {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 640px) {
  .profile-hero {
    grid-template-columns: auto minmax(0, 1fr);
    padding: 18px;
  }

  .profile-logout {
    grid-column: 1 / -1;
    width: 100%;
  }

  .avatar-editor,
  .profile-form__footer {
    align-items: stretch;
    flex-direction: column;
  }

  .readonly-grid {
    grid-template-columns: 1fr;
  }

  .profile-button {
    width: 100%;
  }
}
</style>
