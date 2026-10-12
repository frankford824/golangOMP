<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { LockKeyhole, LogOut, Save, UserRound } from 'lucide-vue-next'

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

    <p v-if="saveError" class="aw-inline-alert">{{ saveError }}</p>
    <p v-else-if="notice" class="aw-inline-alert">{{ notice }}</p>

    <AsyncBoundary
      :loading="loading && !bootstrap"
      :error="error"
      :empty="!bootstrap"
      loading-label="正在加载账号资料"
      empty-label="暂无账号资料"
      @retry="refresh"
    >
      <div class="aw-two-column">
      <section class="aw-panel">
        <div class="aw-panel__head">
          <div>
            <p class="aw-eyebrow">当前账号</p>
            <h3>{{ displayName }}</h3>
          </div>
          <UserRound :size="18" aria-hidden="true" />
        </div>
        <div class="aw-compact-list">
          <article class="aw-compact-list__item">
            <div>
              <strong>账号</strong>
              <span>{{ accountName }}</span>
            </div>
          </article>
          <article class="aw-compact-list__item">
            <div>
              <strong>身份</strong>
              <span class="aw-chip" :class="bootstrap?.is_admin ? 'aw-chip--info' : 'aw-chip--neutral'">{{ identityLabel }}</span>
            </div>
          </article>
          <article class="aw-compact-list__item">
            <div>
              <strong>资料状态</strong>
              <span class="aw-chip" :class="bootstrap?.profile?.pii_completed ? 'aw-chip--success' : 'aw-chip--warn'">{{ profileState }}</span>
            </div>
          </article>
          <article class="aw-compact-list__item">
            <div>
              <strong>联系资料</strong>
              <span>{{ maskedPhone || '未填写' }}</span>
            </div>
          </article>
          <article class="aw-compact-list__item">
            <div>
              <strong>证件 / 收款</strong>
              <span>{{ maskedIdCard || '未填写' }} · {{ maskedAlipay || '未填写' }}</span>
            </div>
          </article>
        </div>
      </section>

      <section class="aw-panel">
        <div class="aw-panel__head">
          <div>
            <p class="aw-eyebrow">资料</p>
            <h3>我的资料</h3>
          </div>
        </div>
        <div class="aw-form-grid">
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
          <button class="aw-primary-button aw-form-grid__full" type="button" :disabled="saving" @click="saveProfile">
            <Save :size="16" aria-hidden="true" />
            保存资料
          </button>
        </div>
      </section>
      </div>
      <section class="aw-panel" aria-labelledby="aw-password-title">
        <div class="aw-panel__head">
          <div><p class="aw-eyebrow">账号安全</p><h3 id="aw-password-title">修改密码</h3></div>
          <LockKeyhole :size="18" aria-hidden="true" />
        </div>
        <form class="aw-form-grid" @submit.prevent="changePassword">
          <label><span>旧密码</span><input v-model="passwordForm.old_password" type="password" autocomplete="current-password" required /></label>
          <label><span>新密码</span><input v-model="passwordForm.new_password" type="password" autocomplete="new-password" minlength="8" placeholder="至少8位，包含字母和数字" required /></label>
          <label><span>确认新密码</span><input v-model="passwordForm.confirm" type="password" autocomplete="new-password" minlength="8" required /></label>
          <p v-if="passwordError" class="aw-inline-alert aw-form-grid__full" role="alert">{{ passwordError }}</p>
          <p v-if="passwordNotice" class="aw-inline-alert aw-form-grid__full" role="status">{{ passwordNotice }}</p>
          <button class="aw-primary-button aw-form-grid__full" type="submit" :disabled="passwordSaving">{{ passwordSaving ? '正在修改' : '修改密码' }}</button>
        </form>
      </section>
    </AsyncBoundary>
  </section>
</template>
