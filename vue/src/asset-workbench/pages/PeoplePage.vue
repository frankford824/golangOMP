<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { assetWorkbenchApi, type AssetWorkbenchProfile } from '@aw/shared/api/assetWorkbenchApi'
import { useRoutePageCopy } from '@aw/app/useRoutePageCopy'
import { usePageRequest } from '@aw/shared/composables/usePageRequest'
import { maskAlipay, maskIdCard, maskPhone } from '@aw/shared/format/pii'
import { chipClass, profileStatusMeta, workerTypeMeta } from '@aw/shared/format/status'
import WorkbenchDataGrid from '@aw/shared/grid/WorkbenchDataGrid.vue'
import AsyncBoundary from '@aw/shared/ui/AsyncBoundary.vue'

const parttimeGrades = ['J1', 'J2', 'J3']
const fulltimeGrades = ['P1', 'P2', 'P3', 'P4', 'S1', 'S2', 'M1', 'M2']

const form = reactive({
  worker_type: 'parttime',
  job_grade: '',
  real_name: '',
  phone: '',
  province: '',
  city: '',
  gender: '',
  alipay_account: '',
  status: 'pending',
})
const profiles = ref<AssetWorkbenchProfile[]>([])
const saving = ref(false)
const hrSaving = ref(false)
const notice = ref('')
const selectedProfile = ref<AssetWorkbenchProfile | null>(null)
const route = useRoute()
const router = useRouter()
const { label: pageLabel, subtitle: pageSubtitle } = useRoutePageCopy('/settings/people')
const peopleRequest = usePageRequest(
  async () => {
    const bootstrap = await assetWorkbenchApi.bootstrap()
    const focusedUserID = Number(route.query.user_id || 0)
    const result = await assetWorkbenchApi.listProfiles({
      page: 1,
      page_size: focusedUserID > 0 ? 1 : 100,
      ...(focusedUserID > 0 ? { user_id: focusedUserID } : {}),
    }).catch(() => ({ items: [], total: 0 }))
    return { bootstrap, profiles: result.items }
  },
  null,
  '人员资料加载失败',
)
const loading = peopleRequest.loading
const error = peopleRequest.error
const hrForm = reactive({
  worker_type: 'parttime',
  job_grade: '',
  real_name: '',
  phone: '',
  province: '',
  city: '',
  gender: '',
  alipay_account: '',
  onboarded_at: '',
  grade_hidden: false,
  status: 'pending',
  reason: '',
})
const gradeOptions = computed(() => (hrForm.worker_type === 'fulltime' ? fulltimeGrades : parttimeGrades))
const focusedUserID = computed(() => Number(route.query.user_id || 0))
const pendingProfiles = computed(() => profiles.value.filter((profile) => profileNeedsGrade(profile)))
const readyProfiles = computed(() => profiles.value.filter((profile) => !profileNeedsGrade(profile)))
const selectedProfileNeedsGrade = computed(() => selectedProfile.value ? profileNeedsGrade(selectedProfile.value) : false)
const profileGridRows = computed(() =>
  profiles.value.map((profile) => ({
    ...profile,
    worker_type_label: workerTypeMeta(profile.worker_type).label,
    pricing_label: profilePricingLabel(profile),
    status_label: profileStatusMeta(profile.status).label,
    phone_label: maskPhone(profile.phone),
    id_card_label: maskIdCard(profile.id_card),
    alipay_label: maskAlipay(profile.alipay_account),
    onboarded_label: formatDateInput(profile.onboarded_at),
    grade_hidden_label: profile.grade_hidden ? '隐藏' : '展示',
  })) as unknown as Record<string, unknown>[],
)
const profileGridColumns = computed<Array<{ key: string; label: string; width: number; align?: 'left' | 'right' | 'center' }>>(() => [
  { key: 'real_name', label: '姓名', width: 128 },
  { key: 'pricing_label', label: '计价状态', width: 108, align: 'center' },
  { key: 'worker_type_label', label: '类型', width: 96 },
  { key: 'job_grade', label: '岗级', width: 96 },
  { key: 'phone_label', label: '手机', width: 132 },
  { key: 'id_card_label', label: '证件', width: 150 },
  { key: 'alipay_label', label: '支付账号', width: 140 },
  { key: 'onboarded_label', label: '入职时间', width: 112 },
  { key: 'grade_hidden_label', label: '定级展示', width: 104 },
  { key: 'province', label: '省份', width: 96 },
  { key: 'city', label: '城市', width: 96 },
  { key: 'status_label', label: '状态', width: 100 },
  { key: 'action', label: '操作', width: 96, align: 'center' },
])

function gridRowAsProfile(row: Record<string, unknown>) {
  return row as unknown as AssetWorkbenchProfile
}

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
      gender: data.bootstrap.profile.gender || '',
      alipay_account: data.bootstrap.profile.alipay_account || '',
      status: data.bootstrap.profile.status || 'pending',
    })
  }
  profiles.value = data.profiles
  if (focusedUserID.value > 0 && selectProfileByUserID(focusedUserID.value)) return
  if (selectedProfile.value && profiles.value.some((profile) => profile.id === selectedProfile.value?.id)) return
  selectFirstPendingProfile()
}

async function saveProfile() {
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
      gender: form.gender,
      alipay_account: form.alipay_account,
      reason: 'self profile update',
    })
    notice.value = saved.pii_completed ? '资料已保存' : '资料已保存，仍有待补字段'
  } catch (err) {
    error.value = err instanceof Error ? err.message : '资料保存失败'
  } finally {
    saving.value = false
  }
}

function selectProfile(row: Record<string, unknown>) {
  const profile = profiles.value.find((item) => item.id === Number(row.id))
  if (!profile) return
  selectProfileRecord(profile)
}

function selectProfileRecord(profile: AssetWorkbenchProfile) {
  selectedProfile.value = profile
  Object.assign(hrForm, {
    worker_type: profile.worker_type || 'parttime',
    job_grade: profile.job_grade || '',
    real_name: profile.real_name || '',
    phone: '',
    province: profile.province || '',
    city: profile.city || '',
    gender: profile.gender || '',
    alipay_account: '',
    onboarded_at: formatDateInput(profile.onboarded_at),
    grade_hidden: profile.grade_hidden === true,
    status: profileNeedsGrade(profile) ? 'active' : profile.status || 'pending',
    reason: profileNeedsGrade(profile) ? '人员定级' : '',
  })
}

function selectProfileByUserID(userID: number) {
  const profile = profiles.value.find((item) => item.user_id === userID)
  if (!profile) return false
  selectProfileRecord(profile)
  return true
}

function selectFirstPendingProfile() {
  const profile = pendingProfiles.value[0] ?? profiles.value[0]
  if (profile) selectProfileRecord(profile)
}

async function clearFocusedProfile() {
  await router.replace({ path: '/settings/people' })
  await loadPeople()
}

async function goToPendingSubmissions() {
  await router.push('/submissions')
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
      gender: hrForm.gender,
      alipay_account: hrForm.alipay_account || undefined,
      onboarded_at: hrForm.onboarded_at ? `${hrForm.onboarded_at}T00:00:00+08:00` : undefined,
      grade_hidden: hrForm.grade_hidden,
      status: hrForm.status,
      reason: hrForm.reason || '管理人员资料',
    })
    notice.value = `已更新 ${hrForm.real_name || profile.user_id} 的人员资料`
    await loadPeople()
    const refreshed = profiles.value.find((item) => item.user_id === profile.user_id)
    selectedProfile.value = refreshed ?? null
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
        <button class="aw-secondary-button" type="button" @click="loadPeople">刷新</button>
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
            <span>待处理人员</span>
            <button v-if="focusedUserID > 0" class="aw-secondary-button" type="button" @click="clearFocusedProfile">
              查看全部
            </button>
          </div>
          <div v-if="pendingProfiles.length" class="aw-grade-queue__list">
            <button
              v-for="profile in pendingProfiles.slice(0, 8)"
              :key="profile.id"
              type="button"
              class="aw-grade-queue__item"
              :class="{ 'aw-grade-queue__item--active': selectedProfile?.id === profile.id }"
              @click="selectProfileRecord(profile)"
            >
              <strong>{{ profile.real_name || `用户 ${profile.user_id}` }}</strong>
              <span>{{ workerTypeMeta(profile.worker_type).label }} · {{ profile.job_grade || '未定级' }}</span>
            </button>
          </div>
          <div v-else class="aw-grade-queue__empty">
            <strong>当前加载范围内没有待定级人员</strong>
            <span>如作品仍显示待定级，可从作品行点击“去定级”直接定位。</span>
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
              <input v-model="hrForm.phone" placeholder="不填则保留原值" />
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
              支付账号
              <input v-model="hrForm.alipay_account" placeholder="不填则保留原值" />
            </label>
            <label>
              省份
              <input v-model="hrForm.province" />
            </label>
            <label>
              城市
              <input v-model="hrForm.city" />
            </label>
            <label class="aw-form-grid__full">
              变更原因
              <input v-model="hrForm.reason" placeholder="定级、转岗、资料补录" />
            </label>
          </div>
          <div class="aw-inline-actions">
            <button class="aw-primary-button" type="button" :disabled="hrSaving || !hrForm.job_grade" @click="saveHRProfile">
              {{ hrSaving ? '保存中' : '保存定级' }}
            </button>
            <button class="aw-secondary-button" type="button" @click="selectedProfile = null">取消选择</button>
          </div>
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
            姓名
            <input v-model="form.real_name" />
          </label>
          <label>
            手机
            <input v-model="form.phone" />
          </label>
          <label>
            省份
            <input v-model="form.province" />
          </label>
          <label>
            城市
            <input v-model="form.city" />
          </label>
          <label>
            性别
            <select v-model="form.gender">
              <option value="">不填写</option>
              <option value="female">女</option>
              <option value="male">男</option>
            </select>
          </label>
          <label>
            支付账号
            <input v-model="form.alipay_account" />
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

    <div class="aw-data-surface">
      <div class="aw-grid-toolbar">
        <span>管理视图</span>
        <span>{{ profiles.length }} 人</span>
      </div>
      <AsyncBoundary
        :loading="loading"
        :error="error"
        loading-label="正在加载人员资料"
        @retry="loadPeople"
      >
        <WorkbenchDataGrid
          v-if="profiles.length"
          :columns="profileGridColumns"
          :rows="profileGridRows"
          row-key="id"
          storage-key="people-profiles"
          group-by="worker_type_label"
          row-clickable
          :selected-row-key="selectedProfile?.id"
          @row-click="selectProfile"
        >
          <template #cell="{ row, column, value }">
            <button
              v-if="column.key === 'real_name'"
              type="button"
              class="aw-link-button"
              @click="selectProfile(row)"
            >
              {{ value || row.user_id }}
            </button>
            <span
              v-else-if="column.key === 'worker_type_label'"
              :class="chipClass(workerTypeMeta(gridRowAsProfile(row).worker_type).tone)"
            >
              {{ value }}
            </span>
            <span
              v-else-if="column.key === 'pricing_label'"
              :class="chipClass(profilePricingTone(gridRowAsProfile(row)))"
            >
              {{ value }}
            </span>
            <span v-else-if="column.key === 'job_grade'">{{ value || '未定级' }}</span>
            <span
              v-else-if="column.key === 'status_label'"
              :class="chipClass(profileStatusMeta(gridRowAsProfile(row).status).tone)"
            >
              {{ value }}
            </span>
            <button
              v-else-if="column.key === 'action'"
              type="button"
              class="aw-grid-button aw-grid-button--strong"
              @click.stop="selectProfile(row)"
            >
              定级
            </button>
            <span v-else>{{ value || '—' }}</span>
          </template>
        </WorkbenchDataGrid>
        <div v-else class="aw-empty-state">
          <h3>没有可见资料</h3>
          <p>普通提交人只维护自己的资料；管理和结算角色可查看人员列表。</p>
        </div>
      </AsyncBoundary>
    </div>
  </section>
</template>
