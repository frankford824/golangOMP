<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import ManagerPicker from './ManagerPicker.vue'
import { useOAAccess } from '../access'
import { api, dateInChina, errorText, send } from '../api'
import '../styles.css'

const props = defineProps<{ userId: number | string; nativeContacts?: boolean }>()
const emit = defineEmits<{ loaded: [managerName: string] }>()
const { currentProfile } = useOAAccess()
type Contact = { name: string; relationship: string; phone: string }
const emptyContact = (): Contact => ({ name: '', relationship: '', phone: '' })
const personnelStatus: Record<string, string> = { active: '在职', probation: '试用', regular: '正式', leaving: '待离职', left: '已离职', disabled: '已停用' }
const workerType: Record<string, string> = { full_time: '全职', part_time: '兼职', outsourced: '外包' }
type Profile = {
  profile_version: number; manager_id: number; manager_name: string; manager_override_id: number; manager_source: string;
  joined: string; confirmed?: string; left?: string; hr_status?: string; employment_type?: string; mobile?: string; email?: string; emergency_contact?: Contact; job_title: string; job_level: string; cost_center: string;
}
const profile = ref<Profile>()
const draft = ref({ version: 0, manager_id: 0, employment_type: '', joined: '', job_title: '', job_level: '', cost_center: '', emergency_contact: emptyContact() })
const busy = ref(false)
const error = ref('')
const notice = ref('')
const changeDate = ref(dateInChina())
const sources: Record<string, string> = { person: '个人指定', team: '小组默认', department: '部门默认' }
function profileDraft(record: Profile) {
  return { version: record.profile_version, manager_id: record.manager_override_id, employment_type: record.employment_type || '', joined: record.joined,
    job_title: record.job_title, job_level: record.job_level, cost_center: record.cost_center, emergency_contact: { ...(record.emergency_contact || emptyContact()) } }
}
function apply(record: Profile) {
  profile.value = record
  emit('loaded', record.manager_name)
  changeDate.value = dateInChina()
  draft.value = profileDraft(record)
}
const changedAssignment = computed(() => profile.value && (draft.value.job_title !== profile.value.job_title || draft.value.job_level !== profile.value.job_level || draft.value.manager_id !== profile.value.manager_override_id))
const dirty = computed(() => profile.value && JSON.stringify(draft.value) !== JSON.stringify(profileDraft(profile.value)))
async function load() {
  error.value = ''
  busy.value = true
  try { apply(await api<Profile>('/oa-api/people/' + props.userId)) }
  catch (cause) { error.value = errorText(cause) }
  finally { busy.value = false }
}
async function save() {
  if (!dirty.value || busy.value) return
  error.value = ''
  notice.value = ''
  busy.value = true
  try {
    const { employment_type, ...profileFields } = draft.value
    apply(await send<Profile>('/oa-api/people/' + props.userId + '/profile', {
      ...profileFields,
      ...(changedAssignment.value ? { effective_date: changeDate.value } : {}),
      ...(employment_type !== (profile.value?.employment_type || '') ? { employment_type } : {}),
    }, 'PUT'))
    notice.value = '人员档案已保存'
  } catch (cause) { error.value = errorText(cause) }
  finally { busy.value = false }
}
watch(() => props.userId, () => { profile.value = undefined; notice.value = ''; void load() }, { immediate: true })
</script>
<template>
  <section class="detail-section personnel-profile">
    <h4>在职与组织档案</h4>
    <p v-if="error" class="oa-error" role="alert">{{ error }} <button type="button" @click="load">重新读取档案</button></p>
    <p v-if="notice" class="oa-notice" role="status">{{ notice }}</p>
    <p v-if="busy && !profile" role="status">读取档案中…</p>
    <form v-if="profile" class="oa-form" @submit.prevent="save">
      <dl class="oa-kv">
        <div><dt>在职状态</dt><dd>{{ personnelStatus[profile.hr_status || ''] || '—' }}</dd></div>
        <div><dt>转正日期</dt><dd>{{ profile.confirmed || '—' }}</dd></div>
        <div><dt>离职日期</dt><dd>{{ profile.left || '—' }}</dd></div>
        <div v-if="!nativeContacts"><dt>手机号</dt><dd>{{ profile.mobile || '—' }}</dd></div>
        <div v-if="!nativeContacts"><dt>邮箱</dt><dd>{{ profile.email || '—' }}</dd></div>
      </dl>
      <p class="oa-muted">生效的直属负责人：{{ profile.manager_id ? profile.manager_name || '已设置' : '未配置' }}<span v-if="profile.manager_id"> · {{ sources[profile.manager_source] }}</span></p>
      <ManagerPicker v-model="draft.manager_id" :current-name="profile.manager_name" :exclude="Number(userId)" :disabled="busy" />
      <div class="profile-fields">
        <label>用工类型<select v-model="draft.employment_type" :disabled="busy"><option value="">跟随账号配置</option><option v-for="(label, value) in workerType" :key="value" :value="value">{{ label }}</option></select></label>
        <label>入职日期<input v-model="draft.joined" type="date" min="1000-01-01" :max="dateInChina()" :disabled="busy" /></label>
        <label>岗位<input v-model.trim="draft.job_title" maxlength="100" :disabled="busy" /></label>
        <label>职级<input v-model.trim="draft.job_level" maxlength="100" :disabled="busy" /></label>
        <label v-if="changedAssignment">异动生效日期<input v-model="changeDate" type="date" min="1000-01-01" :max="dateInChina()" :disabled="busy" required /></label>
        <label>成本中心<input v-model.trim="draft.cost_center" maxlength="100" :disabled="busy" /></label>
      </div>
      <fieldset class="profile-fields">
        <legend>紧急联系人</legend>
        <label>姓名<input v-model.trim="draft.emergency_contact.name" maxlength="100" :disabled="busy" /></label>
        <label>关系<input v-model.trim="draft.emergency_contact.relationship" maxlength="60" :disabled="busy" /></label>
        <label>电话<input v-model.trim="draft.emergency_contact.phone" type="tel" maxlength="40" :disabled="busy" /></label>
      </fieldset>
      <RouterLink v-if="String(currentProfile?.id) === String(userId)" to="/fees/accounts">维护我的常用收款账户</RouterLink>
      <div class="oa-actions"><button class="oa-button-primary" type="submit" :disabled="busy || !dirty">{{ busy ? '处理中…' : '保存人事档案' }}</button></div>
    </form>
  </section>
</template>
<style scoped>
.profile-fields { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(12rem, 100%), 1fr)); gap: .75rem; }
</style>
