<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'

import { assetWorkbenchApi, type AssetWorkbenchProfile } from '@aw/shared/api/assetWorkbenchApi'
import { usePageRequest } from '@aw/shared/composables/usePageRequest'
import { maskAlipay, maskIdCard, maskPhone } from '@aw/shared/format/pii'
import { chipClass, profileStatusMeta, workerTypeMeta } from '@aw/shared/format/status'
import WorkbenchDataGrid from '@aw/shared/grid/WorkbenchDataGrid.vue'
import AsyncBoundary from '@aw/shared/ui/AsyncBoundary.vue'

const form = reactive({
  worker_type: 'parttime',
  job_grade: '',
  real_name: '',
  phone: '',
  province: '',
  city: '',
  alipay_account: '',
  status: 'pending',
})
const profiles = ref<AssetWorkbenchProfile[]>([])
const saving = ref(false)
const hrSaving = ref(false)
const notice = ref('')
const selectedProfile = ref<AssetWorkbenchProfile | null>(null)
const peopleRequest = usePageRequest(
  async () => {
    const bootstrap = await assetWorkbenchApi.bootstrap()
    const result = await assetWorkbenchApi.listProfiles({ page: 1, page_size: 20 }).catch(() => ({ items: [], total: 0 }))
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
  alipay_account: '',
  status: 'pending',
  reason: '',
})
const profileGridRows = computed(() =>
  profiles.value.map((profile) => ({
    ...profile,
    worker_type_label: workerTypeMeta(profile.worker_type).label,
    status_label: profileStatusMeta(profile.status).label,
    phone_label: maskPhone(profile.phone),
    id_card_label: maskIdCard(profile.id_card),
    alipay_label: maskAlipay(profile.alipay_account),
  })) as unknown as Record<string, unknown>[],
)
const profileGridColumns = computed<Array<{ key: string; label: string; width: number; align?: 'left' | 'right' | 'center' }>>(() => [
  { key: 'real_name', label: '姓名', width: 128 },
  { key: 'worker_type_label', label: '类型', width: 96 },
  { key: 'job_grade', label: '岗级', width: 96 },
  { key: 'phone_label', label: '手机', width: 132 },
  { key: 'id_card_label', label: '证件', width: 150 },
  { key: 'alipay_label', label: '支付账号', width: 140 },
  { key: 'province', label: '省份', width: 96 },
  { key: 'city', label: '城市', width: 96 },
  { key: 'status_label', label: '状态', width: 100 },
])

function gridRowAsProfile(row: Record<string, unknown>) {
  return row as unknown as AssetWorkbenchProfile
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
      alipay_account: data.bootstrap.profile.alipay_account || '',
      status: data.bootstrap.profile.status || 'pending',
    })
  }
  profiles.value = data.profiles
}

async function saveProfile() {
  saving.value = true
  error.value = ''
  notice.value = ''
  try {
    const saved = await assetWorkbenchApi.upsertMyProfile({ ...form })
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
  selectedProfile.value = profile
  Object.assign(hrForm, {
    worker_type: profile.worker_type || 'parttime',
    job_grade: profile.job_grade || '',
    real_name: profile.real_name || '',
    phone: '',
    province: profile.province || '',
    city: profile.city || '',
    alipay_account: '',
    status: profile.status || 'pending',
    reason: '',
  })
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
      alipay_account: hrForm.alipay_account || undefined,
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

onMounted(() => {
  void loadPeople()
})
</script>

<template>
  <section class="aw-page-stack">
    <div class="aw-page-bar">
      <div class="aw-page-bar__copy">
        <p class="aw-eyebrow">人员资料</p>
        <h2>人员账户维护</h2>
        <p>工作台资料只服务资产交付和计件结算。岗级变化只影响新的提交快照。</p>
      </div>
      <div class="aw-page-bar__actions">
        <button class="aw-secondary-button" type="button" @click="loadPeople">刷新</button>
        <button class="aw-primary-button" type="button" :disabled="saving" @click="saveProfile">保存资料</button>
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
            <input v-model="form.job_grade" placeholder="P1 / J1" />
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
            <span v-else-if="column.key === 'job_grade'">{{ value || '未定级' }}</span>
            <span
              v-else-if="column.key === 'status_label'"
              :class="chipClass(profileStatusMeta(gridRowAsProfile(row).status).tone)"
            >
              {{ value }}
            </span>
            <span v-else>{{ value || '—' }}</span>
          </template>
        </WorkbenchDataGrid>
        <div v-else class="aw-empty-state">
          <h3>没有可见资料</h3>
          <p>普通提交人只维护自己的资料；管理和结算角色可查看人员列表。</p>
        </div>
      </AsyncBoundary>
    </div>

    <div v-if="selectedProfile" class="aw-panel">
      <div class="aw-panel__head">
        <div>
          <h3>人员定级维护</h3>
          <p class="aw-copy">人员编号 {{ selectedProfile.user_id }} · {{ maskPhone(selectedProfile.phone) }}</p>
        </div>
        <span :class="chipClass(profileStatusMeta(selectedProfile.status).tone)">
          {{ profileStatusMeta(selectedProfile.status).label }}
        </span>
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
          <input v-model="hrForm.job_grade" placeholder="P1 / J1" />
        </label>
        <label>
          姓名
          <input v-model="hrForm.real_name" />
        </label>
        <label>
          状态
          <select v-model="hrForm.status">
            <option value="pending">待定级</option>
            <option value="active">可计价</option>
            <option value="suspended">暂停</option>
          </select>
        </label>
        <label>
          手机
          <input v-model="hrForm.phone" placeholder="不填则保留原值" />
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
        <button class="aw-primary-button" type="button" :disabled="hrSaving" @click="saveHRProfile">
          {{ hrSaving ? '保存中' : '保存人员资料' }}
        </button>
        <button class="aw-secondary-button" type="button" @click="selectedProfile = null">取消</button>
      </div>
    </div>
  </section>
</template>
