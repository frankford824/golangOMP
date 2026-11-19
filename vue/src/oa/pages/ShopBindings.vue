
<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { positiveQueryNumber } from '../peopleNavigation'
import type { HttpAppError } from '@/services/http'
import { api, errorText, send } from '../api'
import { useOAAccess } from '../access'
import OAEmpty from '../components/OAEmpty.vue'
import PageHead from '../components/PageHead.vue'
import '../styles.css'

type Person = { id: number; name: string; employee_no?: number | null; department?: string; team?: string; status: string }
type Shop = { shop_id: string; shop_name: string; platform: string; selectable: boolean; baseline: boolean; visible: boolean; override?: 'granted' | 'revoked' }
type View = { version: number; match: 'matched' | 'missing_employee' | 'unmatched'; shops: Shop[]; editable: boolean }
type Action = 'bind' | 'move' | 'unbind'

const { currentProfile } = useOAAccess()
const route = useRoute()
const router = useRouter()
const admin = computed(() => !!currentProfile.value?.super_admin)
const search = ref(typeof route.query.q === 'string' ? route.query.q : '')
const people = ref<Person[]>([])
const personId = ref<number>()
const person = computed(() => people.value.find(item => item.id === personId.value))
const result = ref<View>()
const action = ref<Action>('bind')
const from = ref('')
const shop = ref('')
const confirm = ref(false)
const busy = ref(false)
const searching = ref(false)
const searchError = ref('')
const error = ref('')
const notice = ref('')
let request = 0
const matchLabels: Record<View['match'], string> = {
  matched: '工号已匹配提成名册',
  missing_employee: '平台未录入工号',
  unmatched: '工号未匹配唯一有效的提成名册人员',
}
const actionLabels = { bind: '绑定店铺', move: '调店', unbind: '解除绑定' }
const source = computed(() => result.value?.shops.filter(item => item.visible) ?? [])
const destinations = computed(() => result.value?.shops.filter(item => !item.visible && item.selectable) ?? [])
const history = computed(() => result.value?.shops.filter(item => item.visible || item.override) ?? [])
const canChange = computed(() => !!result.value?.editable && (action.value === 'bind' || !!from.value) && (action.value === 'unbind' || !!shop.value))
const shopName = (id: string) => result.value?.shops.find(item => item.shop_id === id)?.shop_name || id

function resetChange() {
  from.value = ''
  shop.value = ''
  confirm.value = false
}
async function searchPeople() {
  if (!admin.value || searching.value) return
  searching.value = true
  searchError.value = ''
  try {
    const response = await api<{ items: Person[] }>('/oa-api/people/list?' + new URLSearchParams({ q: search.value.trim() }))
    const oldPerson = person.value
    if (!Array.isArray(response?.items)) throw new Error('人员列表读取失败，请重新搜索')
    people.value = response.items
    if (oldPerson && !people.value.some(item => item.id === oldPerson.id)) people.value.unshift(oldPerson)
  } catch (cause) { searchError.value = errorText(cause) }
  finally { searching.value = false }
}
async function load() {
  const id = personId.value
  const sequence = ++request
  result.value = undefined
  error.value = ''
  resetChange()
  if (!id || !admin.value) { busy.value = false; return }
  busy.value = true
  try {
    const value = await api<View>('/oa-api/people/' + id + '/shop-bindings')
    if (sequence === request) result.value = value
  } catch (cause) {
    if (sequence === request) error.value = errorText(cause)
  } finally { if (sequence === request) busy.value = false }
}
async function apply() {
  if (busy.value || !canChange.value || !personId.value || !result.value) return
  if (!confirm.value) {
    confirm.value = true
    return
  }
  busy.value = true
  error.value = ''
  notice.value = ''
  try {
    result.value = await send<View>('/oa-api/people/' + personId.value + '/shop-bindings', {
      kind: action.value, version: result.value.version,
      ...(action.value !== 'bind' ? { from_shop_id: from.value } : {}),
      ...(action.value !== 'unbind' ? { shop_id: shop.value } : {}),
    })
    notice.value = '店铺范围已更新'
    resetChange()
  } catch (cause) {
    if ((cause as HttpAppError)?.status === 409) {
      await load()
      if (result.value) error.value = '店铺范围已变化，或已有未来生效的任期。已重新读取，请核对后再办理。'
    } else error.value = errorText(cause)
  } finally { busy.value = false }
}
async function searchAndRemember() {
  await searchPeople()
  if (!searchError.value) await router.replace({ query: { ...route.query, q: search.value.trim() || undefined } })
}

async function selectAndRemember() {
  notice.value = ''
  void router.replace({ query: { ...route.query, user_id: personId.value ? String(personId.value) : undefined } })
  await load()
}

async function restoreSelectedPerson() {
  const id = positiveQueryNumber(route.query.user_id, 0)
  if (!admin.value || personId.value === (id || undefined)) return
  personId.value = id || undefined
  const selection = ++request
  result.value = undefined
  error.value = ''
  busy.value = false
  resetChange()
  if (id && !people.value.some(item => item.id === id)) {
    try {
      const selected = await api<Person>('/oa-api/people/' + id)
      if (selection !== request || personId.value !== id) return
      people.value.unshift(selected)
    } catch (cause) {
      if (selection === request && personId.value === id) searchError.value = errorText(cause)
      return
    }
  }
  await load()
}
onMounted(async () => {
  await searchPeople()
  await restoreSelectedPerson()
})
watch(() => route.query.user_id, () => { void restoreSelectedPerson() })
watch(() => route.query.q, (q) => {
  const next = typeof q === 'string' ? q : ''
  if (search.value === next) return
  search.value = next
  void searchPeople()
})
</script>

<template>
  <div class="oa-page shop-bindings">
    <PageHead title="店铺人员绑定" />
    <p v-if="!admin" class="oa-error" role="alert">仅超级管理员可以管理店铺绑定。</p>
    <template v-else>
      <section class="oa-panel binding-search" aria-label="选择人员">
        <form class="oa-actions oa-filter" @submit.prevent="searchAndRemember">
          <label>搜索人员<input v-model="search" placeholder="姓名或工号" /></label>
          <button type="submit" :disabled="searching || busy">搜索</button>
          <label>平台人员
            <select v-model="personId" :disabled="busy" @change="selectAndRemember">
              <option :value="undefined">选择人员</option>
              <option v-for="candidate in people" :key="candidate.id" :value="candidate.id">
                {{ candidate.name }} · {{ candidate.employee_no == null ? '未录工号' : String(candidate.employee_no).padStart(4, '0') }}{{ candidate.department ? ' · ' + candidate.department : '' }}
              </option>
            </select>
          </label>
        </form>
        <p v-if="searchError" class="oa-error" role="alert">{{ searchError }}</p>
        <p v-else-if="searching" role="status" class="oa-muted">正在搜索人员…</p>
        <p v-else-if="!people.length" class="oa-muted">暂无符合条件的人员</p>
      </section>
      <p v-if="error" class="oa-error" role="alert">{{ error }} <button v-if="!result && personId" type="button" :disabled="busy" @click="load">重试</button></p>
      <p v-if="notice" class="oa-notice" role="status">{{ notice }}</p>
      <p v-if="busy && !result" role="status" class="oa-muted">正在读取店铺归属…</p>
      <template v-if="result">
        <section class="oa-panel" aria-label="当前店铺范围">
          <h2>{{ person?.name || '当前人员' }} · 店铺范围</h2>
          <p class="oa-muted">{{ matchLabels[result.match] }}</p>
          <p v-if="result.match !== 'matched'" class="oa-muted">OA 可以单独开放经营店铺查询。个人业绩与个人商品仍需有效提成名册归属。</p>
          <div v-if="history.length" class="oa-table-wrap desktop-region-scroll" tabindex="0" role="region" aria-label="店铺范围明细">
            <table class="oa-table">
              <thead><tr><th scope="col">店铺</th><th scope="col">平台</th><th scope="col">数据层归属</th><th scope="col">OA 维护</th><th scope="col">经营可见</th></tr></thead>
              <tbody><tr v-for="item in history" :key="item.shop_id">
                <td>{{ item.shop_name || item.shop_id }}</td><td>{{ item.platform || '—' }}</td>
                <td>{{ item.baseline ? '已归属' : '—' }}</td>
                <td>{{ item.override === 'granted' ? '已开放' : item.override === 'revoked' ? '已解除' : '—' }}</td>
                <td>{{ item.visible && !['线下店铺', '其它平台'].includes(item.platform) ? '可见' : '不可见' }}</td>
              </tr></tbody>
            </table>
          </div>
          <OAEmpty v-else title="暂无经营店铺" kind="list" />
        </section>
        <section class="oa-panel" aria-label="变更店铺范围">
          <h2>变更店铺范围</h2>
          <p class="oa-muted">今日生效，仅调整 OA 店铺查询范围。经营入口仍按账号权限开放，历史数据和提成归属保持原样。</p>
          <p v-if="!result.editable" class="oa-muted">仅可维护在职非超级管理员的店铺范围。</p>
          <form v-else class="oa-form binding-form" @submit.prevent="apply">
            <div v-if="!confirm" class="oa-form-row">
              <label>办理事项<select v-model="action" @change="resetChange"><option v-for="(label, kind) in actionLabels" :key="kind" :value="kind">{{ label }}</option></select></label>
              <label v-if="action !== 'bind'">原店铺<select v-model="from" required><option value="">选择原店铺</option><option v-for="item in source" :key="item.shop_id" :value="item.shop_id">{{ item.shop_name || item.shop_id }} · {{ item.platform }}</option></select></label>
              <label v-if="action !== 'unbind'">新店铺<select v-model="shop" required><option value="">选择新店铺</option><option v-for="item in destinations" :key="item.shop_id" :value="item.shop_id">{{ item.shop_name || item.shop_id }} · {{ item.platform }}</option></select></label>
            </div>
            <p v-else role="status">确认{{ actionLabels[action] }}：{{ person?.name }}<span v-if="action !== 'bind'"> · 解除 {{ shopName(from) }}</span><span v-if="action !== 'unbind'"> · 开放 {{ shopName(shop) }}</span></p>
            <div class="oa-actions">
              <button v-if="confirm" type="button" :disabled="busy" @click="confirm = false">返回修改</button>
              <button type="submit" class="oa-button-primary" :disabled="busy || !canChange">{{ confirm ? '确认保存' : '预览变更' }}</button>
            </div>
          </form>
        </section>
      </template>
    </template>
  </div>
</template>

<style scoped>
.binding-search .oa-filter, .binding-form { margin: 0; gap: 12px; }
.binding-search select { max-inline-size: min(60vw, 340px); min-inline-size: min(45vw, 220px); }
.binding-form { margin-block-start: 12px; }
.shop-bindings { min-inline-size: 0; }
.shop-bindings :deep(.oa-table-wrap) { margin-block-start: 10px; }
.shop-bindings :deep(.oa-panel p + p) { margin-block-start: 6px; }
@media (max-width: 620px) {
  .shop-bindings :deep(.oa-table th),
  .shop-bindings :deep(.oa-table td) { padding-inline: 5px; }
}
</style>
