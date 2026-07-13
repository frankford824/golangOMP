<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { assetWorkbenchApi, type AssetWorkbenchProfile } from '@aw/shared/api/assetWorkbenchApi'
import { useRoutePageCopy } from '@aw/app/useRoutePageCopy'
import { usePageRequest } from '@aw/shared/composables/usePageRequest'
import { maskPhone } from '@aw/shared/format/pii'
import { chipClass, profileStatusMeta, workerTypeMeta } from '@aw/shared/format/status'
import { cityOptions, provinceOptions } from '@aw/shared/profile/chinaRegions'
import { validateClientProfile } from '@aw/shared/profile/clientProfileValidation'

const parttimeGrades = ['J1', 'J2', 'J3']
const fulltimeGrades = ['P1', 'P2', 'P3', 'P4', 'S1', 'S2', 'M1', 'M2']

const form = reactive({
  worker_type: 'parttime',
  job_grade: '',
  real_name: '',
  phone: '',
  province: '',
  city: '',
  id_card: '',
  gender: '',
  alipay_account: '',
  status: 'pending',
})
const profiles = ref<AssetWorkbenchProfile[]>([])
const saving = ref(false)
const hrSaving = ref(false)
const profileDetailLoading = ref(false)
const profileDetailError = ref('')
const notice = ref('')
const selectedProfile = ref<AssetWorkbenchProfile | null>(null)
const route = useRoute()
const router = useRouter()
const { label: pageLabel, subtitle: pageSubtitle } = useRoutePageCopy('/settings/people')
const peopleRequest = usePageRequest(
  async () => {
    const bootstrap = await assetWorkbenchApi.bootstrap()
    if (!bootstrap.capabilities.includes('asset.workbench.profile.manage')) {
      return { bootstrap, profiles: [] as AssetWorkbenchProfile[] }
    }
    const focusedUserID = Number(route.query.user_id || 0)
    const result = await assetWorkbenchApi.listProfiles({
      page: 1,
      page_size: focusedUserID > 0 ? 1 : 100,
      ...(focusedUserID > 0 ? { user_id: focusedUserID } : {}),
    })
    return { bootstrap, profiles: result.items }
  },
  null,
  '人员资料加载失败',
)
const loading = peopleRequest.loading
const error = peopleRequest.error
const capabilities = computed(() => new Set(peopleRequest.data.value?.bootstrap?.capabilities ?? []))
const canViewUploadOverview = computed(() => capabilities.value.has('asset.workbench.manage') || capabilities.value.has('asset.workbench.settlement'))
const hrForm = reactive({
  worker_type: 'parttime',
  job_grade: '',
  real_name: '',
  phone: '',
  province: '',
  city: '',
  id_card: '',
  gender: '',
  alipay_account: '',
  onboarded_at: '',
  grade_hidden: false,
  status: 'pending',
  reason: '',
})
const gradeOptions = computed(() => (hrForm.worker_type === 'fulltime' ? fulltimeGrades : parttimeGrades))
const availableSelfProvinces = computed(() => provinceOptions(form.province))
const availableSelfCities = computed(() => cityOptions(form.province, form.city))
const availableHRProvinces = computed(() => provinceOptions(hrForm.province))
const availableHRCities = computed(() => cityOptions(hrForm.province, hrForm.city))
const focusedUserID = computed(() => Number(route.query.user_id || 0))
const pendingProfiles = computed(() => profiles.value.filter((profile) => profileNeedsGrade(profile)))
const readyProfiles = computed(() => profiles.value.filter((profile) => !profileNeedsGrade(profile)))
const selectedProfileNeedsGrade = computed(() => selectedProfile.value ? profileNeedsGrade(selectedProfile.value) : false)
let profileDetailRequestID = 0
function profileNeedsGrade(profile: AssetWorkbenchProfile) {
  return profile.status !== 'active' || !profile.worker_type || !profile.job_grade
}

function profilePricingLabel(profile: AssetWorkbenchProfile) {
  return profileNeedsGrade(profile) ? '待定级' : '可计价'
}

function profilePricingTone(profile: AssetWorkbenchProfile) {
  return profileNeedsGrade(profile) ? 'warn' : 'success'
}

async function loadPeople() {
  const data = await peopleRequest.run()
  if (!data) return
  if (data.bootstrap.profile) {
    Object.assign(form, {
      worker_type: data.bootstrap.profile.worker_type || 'parttime',
      job_grade: data.bootstrap.profile.job_grade || '',
      real_name: data.bootstrap.profile.real_name || '',
      phone: data.bootstrap.profile.phone || '',
      province: data.bootstrap.profile.province || '',
      city: data.bootstrap.profile.city || '',
      id_card: data.bootstrap.profile.id_card || '',
      gender: data.bootstrap.profile.gender || '',
      alipay_account: data.bootstrap.profile.alipay_account || '',
      status: data.bootstrap.profile.status || 'pending',
    })
  }
  profiles.value = data.profiles
  const selectedUserID = selectedProfile.value?.user_id
  const target = focusedUserID.value > 0
    ? profiles.value.find((profile) => profile.user_id === focusedUserID.value)
    : profiles.value.find((profile) => profile.user_id === selectedUserID) ?? pendingProfiles.value[0] ?? profiles.value[0]
  if (target) await selectProfileRecord(target)
}

async function saveProfile() {
  const validationError = validateClientProfile(form)
  if (validationError) {
    notice.value = ''
    error.value = validationError
    return
  }
  saving.value = true
  error.value = ''
  notice.value = ''
  try {
    const saved = await assetWorkbenchApi.upsertMyProfile({
      worker_type: form.worker_type,
      real_name: form.real_name,
      phone: form.phone,
      province: form.province,
      city: form.city,
      id_card: form.id_card,
      gender: form.gender as 'female' | 'male',
      alipay_account: form.alipay_account,
      reason: 'self profile update',
    })
    Object.assign(form, {
      real_name: saved.real_name || '',
      phone: saved.phone || '',
      province: saved.province || '',
      city: saved.city || '',
      id_card: saved.id_card || '',
      gender: saved.gender || '',
      alipay_account: saved.alipay_account || '',
      status: saved.status || 'pending',
    })
    notice.value = saved.pii_completed ? '资料已保存' : '资料已保存，仍有待补字段'
  } catch (err) {
    error.value = err instanceof Error ? err.message : '资料保存失败'
  } finally {
    saving.value = false
  }
}

function fillHRForm(profile: AssetWorkbenchProfile, includePII: boolean) {
  Object.assign(hrForm, {
    worker_type: profile.worker_type || 'parttime',
    job_grade: profile.job_grade || '',
    real_name: profile.real_name || '',
    phone: includePII ? profile.phone || '' : '',
    province: profile.province || '',
    city: profile.city || '',
    id_card: includePII ? profile.id_card || '' : '',
    gender: profile.gender || '',
    alipay_account: includePII ? profile.alipay_account || '' : '',
    onboarded_at: formatDateInput(profile.onboarded_at),
    grade_hidden: profile.grade_hidden === true,
    status: profileNeedsGrade(profile) ? 'active' : profile.status || 'pending',
    reason: profileNeedsGrade(profile) ? '人员定级' : '',
  })
}

async function selectProfileRecord(profile: AssetWorkbenchProfile) {
  const requestID = ++profileDetailRequestID
  selectedProfile.value = profile
  fillHRForm(profile, false)
  profileDetailLoading.value = true
  profileDetailError.value = ''
  try {
    const detail = await assetWorkbenchApi.getProfile(profile.user_id)
    if (requestID !== profileDetailRequestID) return
    selectedProfile.value = detail
    fillHRForm(detail, true)
  } catch (err) {
    if (requestID !== profileDetailRequestID) return
    profileDetailError.value = err instanceof Error ? err.message : '完整资料加载失败'
  } finally {
    if (requestID === profileDetailRequestID) profileDetailLoading.value = false
  }
}

async function clearFocusedProfile() {
  await router.replace({ path: '/settings/people' })
  await loadPeople()
}

function clearSelectedProfile() {
  profileDetailRequestID += 1
  profileDetailLoading.value = false
  profileDetailError.value = ''
  selectedProfile.value = null
}

async function goToPendingSubmissions() {
  await router.push(canViewUploadOverview.value ? { path: '/upload-overview' } : { path: '/drive', query: { scope: 'orders' } })
}

async function saveHRProfile() {
  const profile = selectedProfile.value
  if (!profile) return
  hrSaving.value = true
  error.value = ''
  notice.value = ''
  try {
    await assetWorkbenchApi.upsertProfile(profile.user_id, {
      worker_type: hrForm.worker_type,
      job_grade: hrForm.job_grade,
      real_name: hrForm.real_name,
      phone: hrForm.phone || undefined,
      province: hrForm.province,
      city: hrForm.city,
      id_card: hrForm.id_card || undefined,
      gender: hrForm.gender,
      alipay_account: hrForm.alipay_account || undefined,
      onboarded_at: hrForm.onboarded_at ? `${hrForm.onboarded_at}T00:00:00+08:00` : undefined,
      grade_hidden: hrForm.grade_hidden,
      status: hrForm.status,
      reason: hrForm.reason || '管理人员资料',
    })
    notice.value = `已更新 ${hrForm.real_name || profile.user_id} 的人员资料`
    await loadPeople()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '人员资料更新失败'
  } finally {
    hrSaving.value = false
  }
}

function formatDateInput(value?: string) {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value.slice(0, 10)
  return new Intl.DateTimeFormat('en-CA', { timeZone: 'Asia/Shanghai' }).format(date)
}

onMounted(() => {
  void loadPeople()
})
</script>

<template>
  <section class="aw-page-stack">
    <div class="aw-page-bar">
      <div class="aw-page-bar__copy">
        <p class="aw-eyebrow">设置 · 人事</p>
        <h2>{{ pageLabel }}</h2>
        <p>{{ pageSubtitle }}。工作台资料只服务资产交付和计件结算。岗级变化只影响新的提交快照。</p>
      </div>
      <div class="aw-page-bar__actions">
        <button class="aw-secondary-button" type="button" :disabled="loading" @click="loadPeople">
          {{ loading ? '刷新中' : '刷新' }}
        </button>
        <button class="aw-primary-button" type="button" :disabled="saving" @click="saveProfile">保存资料</button>
      </div>
    </div>

    <div v-if="profiles.length" class="aw-panel aw-grade-console">
      <div class="aw-panel__head">
        <div>
          <h3>人员定级处理台</h3>
          <p class="aw-copy">先选人员，再维护人员类型、岗级和状态。状态为“可计价”后，新的上传会直接进入计价。</p>
        </div>
        <div class="aw-inline-actions aw-inline-actions--compact">
          <span class="aw-chip aw-chip--warn">待定级 {{ pendingProfiles.length }}</span>
          <span class="aw-chip aw-chip--success">可计价 {{ readyProfiles.length }}</span>
        </div>
      </div>

      <div class="aw-grade-console__body">
        <div class="aw-grade-queue">
          <div class="aw-grade-queue__head">
            <span>人员名单</span>
            <button v-if="focusedUserID > 0" class="aw-secondary-button" type="button" @click="clearFocusedProfile">
              查看全部
            </button>
          </div>
          <div class="aw-inline-actions aw-inline-actions--compact">
            <span class="aw-chip aw-chip--warn">待定级 {{ pendingProfiles.length }}</span>
            <span class="aw-chip aw-chip--success">可计价 {{ readyProfiles.length }}</span>
          </div>
          <div v-if="profiles.length" class="aw-grade-queue__list aw-grade-queue__list--scroll">
            <button
              v-for="profile in profiles"
              :key="profile.id"
              type="button"
              class="aw-grade-queue__item"
              :class="{ 'aw-grade-queue__item--active': selectedProfile?.id === profile.id }"
              @click="selectProfileRecord(profile)"
            >
              <span class="aw-grade-queue__item-head">
                <strong>{{ profile.real_name || `用户 ${profile.user_id}` }}</strong>
                <span :class="chipClass(profilePricingTone(profile))">{{ profilePricingLabel(profile) }}</span>
              </span>
              <span>{{ workerTypeMeta(profile.worker_type).label }} · {{ profile.job_grade || '未定级' }} · {{ maskPhone(profile.phone) }}</span>
            </button>
          </div>
          <div v-else class="aw-grade-queue__empty">
            <strong>当前没有可维护人员</strong>
            <span>刷新后仍为空时，请先在成员管理中开通资产工作台账号。</span>
          </div>
          <button class="aw-secondary-button" type="button" @click="goToPendingSubmissions">查看待定级作品</button>
        </div>

        <div v-if="selectedProfile" class="aw-grade-editor">
          <div class="aw-grade-editor__head">
            <div>
              <p class="aw-eyebrow">当前人员</p>
              <h3>{{ selectedProfile.real_name || `用户 ${selectedProfile.user_id}` }}</h3>
              <p class="aw-copy">人员编号 {{ selectedProfile.user_id }} · {{ maskPhone(selectedProfile.phone) }}</p>
            </div>
            <span :class="chipClass(profilePricingTone(selectedProfile))">
              {{ profilePricingLabel(selectedProfile) }}
            </span>
          </div>
          <div v-if="selectedProfileNeedsGrade" class="aw-inline-alert">
            该人员缺少可计价定级。保存人员类型、岗级并设为“可计价”后，历史待定级作品需要在“资产维护专区”点击重计价。
          </div>
          <div v-if="profileDetailLoading" class="aw-inline-alert">正在读取完整人员资料，请稍候。</div>
          <div v-else-if="profileDetailError" class="aw-inline-alert">{{ profileDetailError }}</div>
          <div class="aw-form-grid">
            <label>
              人员类型
              <select v-model="hrForm.worker_type">
                <option value="parttime">兼职</option>
                <option value="fulltime">全职</option>
              </select>
            </label>
            <label>
              岗级
              <select v-model="hrForm.job_grade">
                <option value="">未定级</option>
                <option v-for="grade in gradeOptions" :key="grade" :value="grade">{{ grade }}</option>
              </select>
            </label>
            <label>
              状态
              <select v-model="hrForm.status">
                <option value="active">可计价</option>
                <option value="pending">待定级</option>
                <option value="suspended">暂停</option>
              </select>
            </label>
            <label>
              姓名
              <input v-model="hrForm.real_name" />
            </label>
            <label>
              入职时间
              <input v-model="hrForm.onboarded_at" type="date" />
            </label>
            <label class="aw-inline-check">
              <input v-model="hrForm.grade_hidden" type="checkbox" />
              <span>客户端隐藏定级</span>
            </label>
            <label>
              手机
              <input v-model="hrForm.phone" autocomplete="tel" :disabled="profileDetailLoading" />
            </label>
            <label>
              身份证号
              <input v-model="hrForm.id_card" autocomplete="off" :disabled="profileDetailLoading" />
            </label>
            <label>
              性别
              <select v-model="hrForm.gender">
                <option value="">不填写</option>
                <option value="female">女</option>
                <option value="male">男</option>
              </select>
            </label>
            <label>
              支付宝账号
              <input v-model="hrForm.alipay_account" autocomplete="off" :disabled="profileDetailLoading" />
            </label>
            <label>
              省份
              <select v-model="hrForm.province" autocomplete="address-level1" @change="hrForm.city = ''">
                <option value="">请选择省份</option>
                <option v-for="province in availableHRProvinces" :key="province" :value="province">{{ province }}</option>
              </select>
            </label>
            <label>
              城市
              <select v-model="hrForm.city" autocomplete="address-level2" :disabled="!hrForm.province">
                <option value="">请选择城市</option>
                <option v-for="city in availableHRCities" :key="city" :value="city">{{ city }}</option>
              </select>
            </label>
            <label class="aw-form-grid__full">
              变更原因
              <input v-model="hrForm.reason" placeholder="定级、转岗、资料补录" />
            </label>
          </div>
          <div class="aw-inline-actions">
            <button class="aw-primary-button" type="button" :disabled="hrSaving || profileDetailLoading || !!profileDetailError || !hrForm.job_grade" @click="saveHRProfile">
              {{ profileDetailLoading ? '正在读取资料' : hrSaving ? '保存中' : '保存人员资料' }}
            </button>
            <button class="aw-secondary-button" type="button" @click="clearSelectedProfile">取消选择</button>
          </div>
        </div>
        <div v-else class="aw-grade-editor aw-grade-editor--empty">
          <strong>请选择人员</strong>
          <span>从左侧人员名单选择姓名后，再维护人员类型、岗级、证件和支付资料。</span>
        </div>
      </div>
    </div>

    <div class="aw-two-column">
      <div class="aw-panel">
        <div class="aw-panel__head">
          <div>
            <h3>我的资料</h3>
            <p class="aw-copy">这里的信息会用于之后的上传计价。</p>
          </div>
          <span :class="chipClass(profileStatusMeta(form.status).tone)">{{ profileStatusMeta(form.status).label }}</span>
        </div>
        <div class="aw-form-grid">
          <label>
            人员类型
            <select v-model="form.worker_type">
              <option value="parttime">兼职</option>
              <option value="fulltime">全职</option>
            </select>
          </label>
          <label>
            岗级
            <input v-model="form.job_grade" disabled placeholder="HR 定级后显示" />
          </label>
          <label>
            <span>姓名 <small class="aw-field__mark">必填</small></span>
            <input v-model="form.real_name" required />
          </label>
          <label>
            <span>手机号 <small class="aw-field__mark">必填</small></span>
            <input v-model="form.phone" autocomplete="tel" required />
          </label>
          <label>
            <span>省份 <small class="aw-field__mark">必填</small></span>
            <select v-model="form.province" autocomplete="address-level1" required @change="form.city = ''">
              <option value="">请选择省份</option>
              <option v-for="province in availableSelfProvinces" :key="province" :value="province">{{ province }}</option>
            </select>
          </label>
          <label>
            <span>城市 <small class="aw-field__mark">必填</small></span>
            <select v-model="form.city" autocomplete="address-level2" required :disabled="!form.province">
              <option value="">请选择城市</option>
              <option v-for="city in availableSelfCities" :key="city" :value="city">{{ city }}</option>
            </select>
          </label>
          <label>
            <span>身份证号 <small class="aw-field__mark">必填</small></span>
            <input v-model="form.id_card" autocomplete="off" inputmode="numeric" minlength="18" maxlength="18" pattern="[0-9]{18}" placeholder="请输入 18 位数字" required />
          </label>
          <label>
            <span>性别 <small class="aw-field__mark">必填</small></span>
            <select v-model="form.gender" required>
              <option value="" disabled>请选择性别</option>
              <option value="female">女</option>
              <option value="male">男</option>
            </select>
          </label>
          <label>
            <span>支付宝账号 <small class="aw-field__mark">必填</small></span>
            <input v-model="form.alipay_account" autocomplete="off" required />
          </label>
        </div>
        <p v-if="notice" class="aw-copy">{{ notice }}</p>
        <p v-if="error" class="aw-copy">{{ error }}</p>
      </div>

      <div class="aw-panel">
        <div class="aw-panel__head">
          <h3>资料用途</h3>
          <span class="aw-chip aw-chip--neutral">隐私保护</span>
        </div>
        <p class="aw-copy">
          工作台资料只服务资产交付和计件结算，不用于其他业务页面。
        </p>
        <p class="aw-copy">
          身份证、手机号、支付账号默认脱敏展示；查看和导出完整信息会留下操作记录。
        </p>
      </div>
    </div>

  </section>
</template>
