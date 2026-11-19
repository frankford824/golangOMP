<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import ManagerPicker from './ManagerPicker.vue'
import { api, dateInChina, errorText, send } from '../api'
import '../styles.css'

const props = defineProps<{ userId: number | string }>()
type Profile = {
  profile_version: number; manager_id: number; manager_name: string; manager_override_id: number; manager_source: string;
  joined: string; job_title: string; job_level: string; cost_center: string;
}
const profile = ref<Profile>()
const draft = ref({ version: 0, manager_id: 0, joined: '', job_title: '', job_level: '', cost_center: '' })
const busy = ref(false)
const error = ref('')
const notice = ref('')
const sources: Record<string, string> = { person: '个人指定', team: '小组默认', department: '部门默认' }
function profileDraft(record: Profile) {
  return { version: record.profile_version, manager_id: record.manager_override_id, joined: record.joined,
    job_title: record.job_title, job_level: record.job_level, cost_center: record.cost_center }
}
function apply(record: Profile) {
  profile.value = record
  draft.value = profileDraft(record)
}
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
    apply(await send<Profile>('/oa-api/people/' + props.userId + '/profile', draft.value, 'PUT'))
    notice.value = '人员档案已保存'
  } catch (cause) { error.value = errorText(cause) }
  finally { busy.value = false }
}
watch(() => props.userId, () => { profile.value = undefined; notice.value = ''; void load() }, { immediate: true })
</script>
<template>
  <section class="detail-section personnel-profile">
    <h4>人事档案</h4>
    <p v-if="error" class="oa-error" role="alert">{{ error }} <button type="button" @click="load">重新读取档案</button></p>
    <p v-if="notice" class="oa-notice" role="status">{{ notice }}</p>
    <p v-if="busy && !profile" role="status">读取档案中…</p>
    <form v-if="profile" class="oa-form" @submit.prevent="save">
      <p class="oa-muted">生效的直属负责人：{{ profile.manager_id ? profile.manager_name || '已设置' : '未配置' }}<span v-if="profile.manager_id"> · {{ sources[profile.manager_source] }}</span></p>
      <ManagerPicker v-model="draft.manager_id" :current-name="profile.manager_name" :exclude="Number(userId)" :disabled="busy" />
      <div class="profile-fields">
        <label>入职日期<input v-model="draft.joined" type="date" min="1000-01-01" :max="dateInChina()" :disabled="busy" /></label>
        <label>岗位<input v-model.trim="draft.job_title" maxlength="100" :disabled="busy" /></label>
        <label>职级<input v-model.trim="draft.job_level" maxlength="100" :disabled="busy" /></label>
        <label>成本中心<input v-model.trim="draft.cost_center" maxlength="100" :disabled="busy" /></label>
      </div>
      <div class="oa-actions"><button class="oa-button-primary" type="submit" :disabled="busy || !dirty">{{ busy ? '处理中…' : '保存人事档案' }}</button></div>
    </form>
  </section>
</template>
<style scoped>
.profile-fields { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(12rem, 100%), 1fr)); gap: .75rem; }
</style>
