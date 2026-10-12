<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { LockKeyhole, LogOut, Save, UserRound, X } from 'lucide-vue-next'
import { DialogContent, DialogDescription, DialogOverlay, DialogPortal, DialogRoot, DialogTitle } from 'reka-ui'

import { useAssetWorkbenchBootstrap } from '@aw/app/useAssetWorkbenchBootstrap'
import { useWorkbenchSession } from '@aw/app/useWorkbenchSession'
import { assetWorkbenchApi } from '@aw/shared/api/assetWorkbenchApi'
import { maskAlipay, maskIdCard, maskPhone } from '@aw/shared/format/pii'
import { roleDisplayList } from '@aw/shared/format/roleDisplay'
import { cityOptions, provinceOptions } from '@aw/shared/profile/chinaRegions'
import { validateClientProfile } from '@aw/shared/profile/clientProfileValidation'
import { validateWorkbenchPassword } from '@aw/shared/profile/passwordValidation'
import { resolveApiUserMessage } from '@/utils/api-message-zh'
import AsyncBoundary from '@aw/shared/ui/AsyncBoundary.vue'

const { bootstrap, loading, error, refresh } = useAssetWorkbenchBootstrap()
const { logout } = useWorkbenchSession()
const saving = ref(false)
const notice = ref('')
const saveError = ref('')
const passwordForm = reactive({ old_password: '', new_password: '', confirm: '' })
const passwordSaving = ref(false)
const passwordError = ref('')
const passwordNotice = ref('')
const editor = ref<'profile' | 'password' | null>(null)
let editorTrigger: HTMLElement | null = null

function openEditor(kind: 'profile' | 'password', event: MouseEvent) {
  editorTrigger = event.currentTarget as HTMLElement
  syncForm()
  saveError.value = ''
  passwordError.value = ''
  passwordNotice.value = ''
  editor.value = kind
}

function closeEditor() {
  if (saving.value || passwordSaving.value) return
  editor.value = null
  passwordForm.old_password = ''
  passwordForm.new_password = ''
  passwordForm.confirm = ''
  passwordError.value = ''
  saveError.value = ''
  syncForm()
}

function restoreEditorFocus() {
  editorTrigger?.focus()
}

async function changePassword() {
  if (passwordSaving.value) return
  passwordError.value = ''
  passwordNotice.value = ''
  const validation = validateWorkbenchPassword(passwordForm.new_password)
  if (!passwordForm.old_password) passwordError.value = '请输入旧密码'
  else if (validation) passwordError.value = validation
  else if (passwordForm.new_password !== passwordForm.confirm) passwordError.value = '两次新密码不一致'
  else if (passwordForm.new_password === passwordForm.old_password) passwordError.value = '新密码不能与旧密码相同'
  if (passwordError.value) return
  passwordSaving.value = true
  try {
    await assetWorkbenchApi.changeMyPassword({ ...passwordForm })
    passwordForm.old_password = ''
    passwordForm.new_password = ''
    passwordForm.confirm = ''
    passwordNotice.value = '密码已修改，下次登录请使用新密码'
    notice.value = passwordNotice.value
    passwordSaving.value = false
    closeEditor()
  } catch (err) {
    passwordError.value = resolveApiUserMessage(err, { fallback: '密码修改失败，请重试' })
  } finally {
    passwordSaving.value = false
  }
}

const form = reactive({
  real_name: '',
  phone: '',
  province: '',
  city: '',
  id_card: '',
  gender: '',
  alipay_account: '',
})

const displayName = computed(() => bootstrap.value?.profile?.real_name || bootstrap.value?.user?.name || bootstrap.value?.user?.username || '我的账号')
const identityLabel = computed(() => {
  const labels = bootstrap.value?.role_labels?.length
    ? bootstrap.value.role_labels
    : roleDisplayList(bootstrap.value?.access?.asset_roles ?? [])
  return labels.length ? labels.join(' / ') : '未开通'
})
const profileState = computed(() => {
  if (!bootstrap.value?.profile) return '资料待填写'
  if (!bootstrap.value.profile.pii_completed) return '资料待补全'
  return '资料已完成'
})
const accountName = computed(() => bootstrap.value?.user?.username || bootstrap.value?.user?.account || '-')
const maskedPhone = computed(() => maskPhone(form.phone))
const maskedIdCard = computed(() => maskIdCard(form.id_card))
const maskedAlipay = computed(() => maskAlipay(form.alipay_account))
const availableProvinces = computed(() => provinceOptions(form.province))
const availableCities = computed(() => cityOptions(form.province, form.city))

function syncForm() {
  const profile = bootstrap.value?.profile
  form.real_name = profile?.real_name || bootstrap.value?.user?.name || ''
  form.phone = profile?.phone || ''
  form.province = profile?.province || ''
  form.city = profile?.city || ''
  form.id_card = profile?.id_card || ''
  form.gender = profile?.gender || ''
  form.alipay_account = profile?.alipay_account || ''
}

async function saveProfile() {
  const validationError = validateClientProfile(form)
  if (validationError) {
    notice.value = ''
    saveError.value = validationError
    return
  }
  saving.value = true
  notice.value = ''
  saveError.value = ''
  try {
    await assetWorkbenchApi.upsertMyProfile({
      real_name: form.real_name,
      phone: form.phone,
      province: form.province,
      city: form.city,
      id_card: form.id_card.trim().toUpperCase(),
      gender: form.gender as 'female' | 'male',
      alipay_account: form.alipay_account,
      reason: 'self profile update',
    })
    notice.value = '资料已保存'
    await refresh({ force: true })
    saving.value = false
    closeEditor()
  } catch (err) {
    saveError.value = err instanceof Error ? err.message : '资料保存失败'
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  void refresh()
})

watch(bootstrap, syncForm, { immediate: true })
</script>

<template>
  <section class="aw-page-stack">
    <div class="aw-page-bar">
      <div class="aw-page-bar__copy">
        <p class="aw-eyebrow">账号</p>
        <h2>个人中心</h2>
        <p>维护个人资料、修改登录密码，或退出当前账号。</p>
      </div>
      <div class="aw-page-bar__actions">
        <button class="aw-secondary-button" type="button" @click="logout">
          <LogOut :size="16" aria-hidden="true" />
          退出登录
        </button>
      </div>
    </div>

    <p v-if="notice" class="aw-inline-alert" role="status">{{ notice }}</p>

    <AsyncBoundary
      :loading="loading && !bootstrap"
      :error="error"
      :empty="!bootstrap"
      loading-label="正在加载账号资料"
      empty-label="暂无账号资料"
      @retry="refresh"
    >
      <section class="aw-panel aw-account-overview">
        <div class="aw-panel__head">
          <div>
            <p class="aw-eyebrow">当前账号</p>
            <h3>{{ displayName }}</h3>
          </div>
          <UserRound :size="18" aria-hidden="true" />
        </div>
        <dl class="aw-account-facts">
          <div><dt>账号</dt><dd>{{ accountName }}</dd></div>
          <div><dt>身份</dt><dd><span class="aw-chip" :class="bootstrap?.is_admin ? 'aw-chip--info' : 'aw-chip--neutral'">{{ identityLabel }}</span></dd></div>
          <div><dt>资料状态</dt><dd><span class="aw-chip" :class="bootstrap?.profile?.pii_completed ? 'aw-chip--success' : 'aw-chip--warn'">{{ profileState }}</span></dd></div>
          <div><dt>联系资料</dt><dd>{{ maskedPhone || '未填写' }}</dd></div>
          <div><dt>证件 / 收款</dt><dd>{{ maskedIdCard || '未填写' }} · {{ maskedAlipay || '未填写' }}</dd></div>
        </dl>
        <div class="aw-account-actions">
          <div class="aw-account-actions__row">
            <div><strong>我的资料</strong><p class="aw-copy">维护姓名、联系资料和收款信息。</p></div>
            <button class="aw-secondary-button" data-testid="account-edit-profile" type="button" @click="openEditor('profile', $event)">{{ bootstrap?.profile?.pii_completed ? '编辑资料' : '补全资料' }}</button>
          </div>
          <div class="aw-account-actions__row">
            <div><strong>账号安全</strong><p class="aw-copy">使用旧密码修改当前账号的登录密码。</p></div>
            <button class="aw-secondary-button" data-testid="account-edit-password" type="button" @click="openEditor('password', $event)"><LockKeyhole :size="16" aria-hidden="true" />修改密码</button>
          </div>
        </div>
      </section>
    </AsyncBoundary>

    <DialogRoot :open="Boolean(editor)" @update:open="(open) => { if (!open) closeEditor() }">
      <DialogPortal>
        <div v-if="editor" class="aw-token-scope aw-account-dialog-layer">
          <DialogOverlay class="aw-account-dialog-backdrop" />
          <DialogContent class="aw-account-dialog" :class="{ 'aw-account-dialog--password': editor === 'password' }" @close-auto-focus.prevent="restoreEditorFocus">
            <header class="aw-panel__head">
              <div>
                <DialogTitle as="h3">{{ editor === 'profile' ? '编辑我的资料' : '修改密码' }}</DialogTitle>
                <DialogDescription class="aw-copy">{{ editor === 'profile' ? '资料用于联系和结算，请核对后保存。' : '新密码至少8位，包含字母和数字。' }}</DialogDescription>
              </div>
              <button class="aw-secondary-button aw-account-dialog__close" type="button" aria-label="关闭" :disabled="saving || passwordSaving" @click="closeEditor"><X :size="18" aria-hidden="true" /></button>
            </header>
        <form v-if="editor === 'profile'" class="aw-form-grid" @submit.prevent="saveProfile">
          <label>
            <span>姓名 <small class="aw-field__mark">必填</small></span>
            <input v-model="form.real_name" autocomplete="name" required />
          </label>
          <label>
            <span>手机号 <small class="aw-field__mark">必填</small></span>
            <input v-model="form.phone" autocomplete="tel" required />
          </label>
          <label>
            <span>省份 <small class="aw-field__mark">必填</small></span>
            <select v-model="form.province" autocomplete="address-level1" required @change="form.city = ''">
              <option value="">请选择省份</option>
              <option v-for="province in availableProvinces" :key="province" :value="province">{{ province }}</option>
            </select>
          </label>
          <label>
            <span>城市 <small class="aw-field__mark">必填</small></span>
            <select v-model="form.city" autocomplete="address-level2" required :disabled="!form.province">
              <option value="">请选择城市</option>
              <option v-for="city in availableCities" :key="city" :value="city">{{ city }}</option>
            </select>
          </label>
          <label>
            <span>身份证号 <small class="aw-field__mark">必填</small></span>
            <input v-model="form.id_card" autocomplete="off" minlength="18" maxlength="18" pattern="[0-9]{17}[0-9Xx]" placeholder="18位，末位可为X" required />
          </label>
          <label>
            <span>支付宝账号 <small class="aw-field__mark">必填</small></span>
            <input v-model="form.alipay_account" autocomplete="off" required />
          </label>
          <label class="aw-form-grid__full">
            <span>性别 <small class="aw-field__mark">必填</small></span>
            <select v-model="form.gender" required>
              <option value="" disabled>请选择性别</option>
              <option value="female">女</option>
              <option value="male">男</option>
            </select>
          </label>
          <p v-if="saveError" class="aw-inline-alert aw-form-grid__full" role="alert">{{ saveError }}</p>
          <div class="aw-account-dialog__footer aw-form-grid__full">
          <button class="aw-secondary-button" type="button" :disabled="saving" @click="closeEditor">取消</button>
          <button class="aw-primary-button" type="submit" :disabled="saving">
            <Save :size="16" aria-hidden="true" />
            保存资料
          </button>
          </div>
        </form>
        <form v-else class="aw-form-grid aw-account-password-form" @submit.prevent="changePassword">
          <label><span>旧密码</span><input v-model="passwordForm.old_password" type="password" autocomplete="current-password" required /></label>
          <label><span>新密码</span><input v-model="passwordForm.new_password" type="password" autocomplete="new-password" minlength="8" placeholder="至少8位，包含字母和数字" required /></label>
          <label><span>确认新密码</span><input v-model="passwordForm.confirm" type="password" autocomplete="new-password" minlength="8" required /></label>
          <p v-if="passwordError" class="aw-inline-alert aw-form-grid__full" role="alert">{{ passwordError }}</p>
          <div class="aw-account-dialog__footer aw-form-grid__full">
            <button class="aw-secondary-button" type="button" :disabled="passwordSaving" @click="closeEditor">取消</button>
            <button class="aw-primary-button" type="submit" :disabled="passwordSaving">{{ passwordSaving ? '正在修改' : '确认修改' }}</button>
          </div>
        </form>
          </DialogContent>
        </div>
      </DialogPortal>
    </DialogRoot>
  </section>
</template>
