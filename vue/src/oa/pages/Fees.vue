<script lang="ts">
let savedApplication: {
  owner: string
  site: string
  values: { template_id: string; amount: string; reason: string; channel: string; extra: Record<string, string>; files: { id: string; name: string }[] }
} | undefined
</script>

<script setup lang="ts">
import { ArrowLeft, Check, CircleAlert, FileText, Wallet } from 'lucide-vue-next'
import OAEmpty from '../components/OAEmpty.vue'
import { computed, nextTick, onMounted, ref, watch, watchEffect } from 'vue'
import { onBeforeRouteLeave, RouterLink, useRoute, useRouter } from 'vue-router'
import FeePersonSelect from '../components/FeePersonSelect.vue'
import { searchFeePeople, approverLabel, type FeePerson as Approver } from '../feePeople'
import { feeTabs } from '../routes'
import PageHead from '../components/PageHead.vue'
import { api, dateInChina, download, errorText, money, send } from '../api'
import { useOAAccess } from '../access'
import { expenseActivityActions, useExpenseActivity } from '../expenseActivity'
import { feeDateTime, feeStatus as status } from '../feeDisplay'
import '../styles.css'

const { can, currentProfile } = useOAAccess()
const expenseActivity = useExpenseActivity()
const canUseExpenseActivity = computed(() => expenseActivityActions.some(can))

type FeeTemplate = { id: string; name: string; type: string; channels: string[]; fields: string[]; required: string[]; evidence: string; nodes: string[]; threshold_cents: number; extra: string; version: number }
type Evidence = { id: string; name: string }
type Decision = { action: string; comment?: string; actor_id?: number; at: string }
type FeeDoc = { id: string; applicant_id: number; template: FeeTemplate; amount_cents: number; reason: string; channel: string; holder: string; account: string; bank: string; extra: Record<string, string>; files: string[]; site: string; state: string; route: (number | string | { id?: number; name?: string; user_id?: number })[]; current: number; version: number; history?: Decision[]; payment?: { amount_cents: number; date: string; receipt: string } }
type FeeReport = { amount_cents: number; count: number; items: FeeDoc[]; unpaid_cents?: number }
const props = defineProps<{ mode: string }>()
const route = useRoute()
const router = useRouter()
const activeTab = computed(() => feeTabs.find(tab => tab.mode === props.mode))
const title = computed(() => activeTab.value?.label || ({ apply: '发起费用申请', document: '费用单据' }[props.mode] ?? '费用中心'))
const allowed = computed(() => {
  if (activeTab.value) return activeTab.value.actions.some(can)
  if (props.mode === 'apply') return can('finance.apply')
  if (props.mode === 'document') return feeTabs.some(tab => tab.mode !== 'templates' && tab.actions.some(can))
  return false
})
const accessMessage = computed(() => props.mode === 'home'
  ? '当前账号没有费用中心权限。如需办理费用业务，请联系财务管理员。'
  : '当前账号没有' + title.value + '权限。如需开通，请联系财务管理员。')
const isAdmin = computed(() => can('finance.admin'))
const canPay = computed(() => can('finance.pay'))
const queue = ref<'mine' | 'todo' | 'pay'>(props.mode === 'mine' ? 'mine' : props.mode === 'approval' && !can('finance.approve') ? 'pay' : 'todo')
const busy = ref(false)
const error = ref('')
const notice = ref('')
const templates = ref<FeeTemplate[]>([])
const docs = ref<FeeDoc[]>([])
const page = ref(1)
const pages = computed(() => Math.max(1, Math.ceil(docs.value.length / 15)))
const visibleDocs = computed(() => docs.value.slice((page.value - 1) * 15, page.value * 15))
const detail = ref<FeeDoc>()
const report = ref<FeeReport>()
const reportPage = ref(1)
const reportPages = computed(() => Math.max(1, Math.ceil((report.value?.items?.length ?? 0) / 15)))
const visibleReport = computed(() => report.value?.items?.slice((reportPage.value - 1) * 15, reportPage.value * 15) ?? [])
const edited = ref(false)
const draftEdited = ref(false)
const fieldErrors = ref<Record<string, string>>({})
const touched = new Set<string>()
function validateField(key: string, problem: () => string, touch = false) {
  if (touch) touched.add(key)
  if (!touched.has(key)) return
  const message = problem()
  if (message) fieldErrors.value[key] = message
  else delete fieldErrors.value[key]
}
function amountProblem() {
  try { return cents(form.value.amount) > 100000000000 ? '申请金额最多 10 亿元' : '' }
  catch { return '请输入大于零的金额，最多两位小数' }
}
const requiredProblem = (value: string | undefined, message: string) => value?.trim() ? '' : message
const accountRequired = computed(() => !returned.value || form.value.channel !== detail.value?.channel)
const fileProblem = () => form.value.files.length ? '' : '请上传至少一份单据凭证'
const dateProblem = () => !payment.value.date ? '请选择实付日期' : payment.value.date > dateInChina() ? '实付日期不能晚于今天' : ''
function fieldAttrs(key: string, help?: string) {
  const invalid = !!fieldErrors.value[key]
  return { 'aria-invalid': invalid, 'aria-describedby': [help, invalid ? 'fee-error-' + key : undefined].filter(Boolean).join(' ') || undefined }
}
const loadedDocs = ref(false)
const receiptError = ref('')
const applicationForm = ref<HTMLFormElement>()
const heading = ref<HTMLHeadingElement>()
const approvalsChanged = ref(false)
const draftRestored = ref(false)
const missingTemplateDraft = ref(false)
const returnLabel = computed(() => ({ mine: '我的申请', todo: '审批待办', pay: '待付款' })[queue.value])
const queueLabel = computed(() => queue.value === 'pay' ? '待付款单据' : queue.value === 'todo' ? '待你审批的单据' : '我的申请')
const queueEmpty = computed(() => ({
  mine: '暂无费用申请', todo: '没有需要你审批的费用单据', pay: '没有待付款单据',
})[queue.value])
const queueGuide = computed(() => ({
  mine: '提交后可在这里查看审批进度、退回意见和付款记录。',
  todo: '审批流转到你时，单据会出现在这里。',
  pay: '全部审批通过后，其他申请人的单据会出现在这里。',
})[queue.value])
const showQueue = computed(() => allowed.value && !detail.value && ['mine', 'approval'].includes(props.mode))
const requestedPeriod = route.query.period
const period = ref(typeof requestedPeriod === 'string' && /^\d{4}(-\d{2}){0,2}$/.test(requestedPeriod) ? requestedPeriod : dateInChina().slice(0, 7))
const reportPeriod = ref(period.value)
const defaultPeriodExample = dateInChina().slice(0, 7)
const reportLoaded = ref(false)
const selection = ref('')
const form = ref({ request_id: '', template_id: '', amount: '', reason: '', channel: '', holder: '', account: '', bank: '', extra: {} as Record<string, string>, files: [] as Evidence[], site: route.query.site === 'asset' ? 'asset' : 'main' })
const returned = ref<string>()
const returnedTemplate = ref<FeeTemplate>()
const level = ref('')
type FinanceGrant = Approver & { status: string; level: string; pay: boolean; report: boolean }
const grantedPeople = ref<FinanceGrant[]>([])
const grantsLoaded = ref(false)
const grantPage = ref(1)
const grantPages = computed(() => Math.max(1, Math.ceil(grantedPeople.value.length / 15)))
const visibleGrants = computed(() => grantedPeople.value.slice((grantPage.value - 1) * 15, grantPage.value * 15))
const grantStatuses: Record<string, string> = { active: '在职', inactive: '停用或离职', deleted: '已删除', missing: '账号已不可用' }
const grantName = (item: Approver) => item.name ? approverLabel(item) : '账号 #' + item.id
const revealed = ref(false)
const approval = ref({ decision: 'approve', comment: '' })
const payment = ref({ date: dateInChina(), receipt: '', receiptName: '' })
const draft = ref<FeeTemplate>()
const editingId = ref('')
const delegatePanel = ref<HTMLDetailsElement>()
const financeGrantPanel = ref<HTMLDetailsElement>()
function openGrantPanel(panel: HTMLDetailsElement | undefined) {
  if (!panel) return
  panel.open = true
  panel.scrollIntoView({ block: 'start' })
  panel.querySelector('summary')?.focus({ preventScroll: true })
}
const templateFields = ref({ fields: '', threshold: '0.00' })
const templateForm = ref<HTMLFormElement>()
const approverQuery = ref('')
const approverOptions = ref<Approver[]>([])
const knownApprovers = ref<Record<number, Approver>>({})
const approverBusy = ref(false)
const approverError = ref('')
let approverSearch = 0
const approverNext = ref(0)
let requestedApproverQuery = ''
const grants = ref({ pay: false, report: false })
const delegate = ref({ level: 'draft' })
const grantTarget = ref<Approver>()
const delegateTarget = ref<Approver>()
const inactiveGrant = (target: Approver | undefined) => !!target && grantedPeople.value.some(item => item.id === target.id && item.status !== 'active')
function selectGrantTarget(target: Approver | undefined, kind: 'template' | 'finance') {
  const current = grantedPeople.value.find(item => item.id === target?.id)
  if (kind === 'template') {
    delegateTarget.value = target
    delegate.value.level = current?.status === 'active' ? current.level : (current ? 'none' : 'publish')
  } else {
    grantTarget.value = target
    grants.value = current?.status === 'active' ? { pay: current.pay, report: current.report } : { pay: false, report: false }
  }
}
function editGrant(item: FinanceGrant, kind: 'template' | 'finance') {
  selectGrantTarget(item, kind)
  openGrantPanel(kind === 'template' ? delegatePanel.value : financeGrantPanel.value)
}
const delegateLevels: Record<string, string> = { none: '撤销模板维护权', draft: '编辑草稿', publish: '编辑并发布' }
const canEditTemplate = computed(() => level.value === 'draft' || level.value === 'publish')
const canManageTemplates = computed(() => canEditTemplate.value || isAdmin.value)
const templateGuide = computed(() => level.value === 'publish'
  ? '请前往模板与授权，检查现有草稿或创建并发布一个模板。'
  : level.value === 'draft'
    ? '请前往模板与授权准备草稿，并请有发布权限的同事发布。'
    : isAdmin.value
      ? '请前往模板与授权，为维护人员设置编辑并发布权限。'
      : '请联系财务管理员安排模板发布及付款经办。模板发布后即可发起申请。')
const loadedTemplates = ref(false)
const files = ref<HTMLInputElement>()
const receipt = ref<HTMLInputElement>()
const channels: Record<string, string> = { bank: '对公银行转账', person: '对私银行卡', wallet: '对私收款平台' }
const decisions: Record<string, string> = { approve: '同意', return: '退回修改', reject: '拒绝' }
const active = computed(() => returnedTemplate.value || templates.value.find(item => item.id === form.value.template_id))
const own = computed(() => String(detail.value?.applicant_id) === String(currentProfile.value?.id))
const documentReturn = computed(() => {
  if (route.query.from === 'notifications' && canUseExpenseActivity.value) {
    return { name: 'MeNotifications', query: { section: 'expense', ...(route.query.activity === 'results' ? { activity: 'results' } : {}) } }
  }
  if (route.query.from === 'report' && can('finance.report')) {
    return { path: '/fees/report', query: { period: route.query.period, page: route.query.page } }
  }
  if (own.value && can('finance.apply')) return '/fees/mine'
  if (can('finance.approve') || can('finance.pay')) return '/fees/approval'
  if (can('finance.report')) return '/fees/report'
  return '/fees/mine'
})
const stageGuide = computed(() => ({
  waiting: '审批按模板顺序流转。',
  returned: '申请人可根据退回意见修改并重新提交。',
  rejected: '本次审批已结束。',
  approved: '审批已完成，等待财务登记实付。',
  paid: '付款已登记，可查看实付信息和回单。',
})[detail.value?.state || ''] || '')
const mayDecide = computed(() => detail.value?.state === 'waiting' && !own.value && String(detail.value.route?.[detail.value.current]) === String(currentProfile.value?.id))
const major = (cents?: number) => cents == null ? '—' : money(cents / 100)
const lastReturn = computed(() => [...(detail.value?.history || [])].reverse().find(entry => entry.action === 'return'))
const approvalButton = computed(() => ({ approve: '同意申请', return: '退回申请', reject: '拒绝申请' }[approval.value.decision]))
const leaveForm = () => !edited.value || window.confirm('费用申请尚未提交。放弃已填内容并离开？')
const draftOwner = computed(() => String(currentProfile.value?.id || ''))
watch(draftOwner, (owner, previous) => {
  if (previous && owner !== previous) savedApplication = undefined
})
function saveApplicationDraft() {
  if (!draftOwner.value || returned.value) return
  const { template_id, amount, reason, channel, extra, files } = form.value
  savedApplication = {
    owner: draftOwner.value, site: form.value.site,
    values: { template_id, amount, reason, channel, extra: { ...extra }, files: files.map(file => ({ ...file })) },
  }
  edited.value = Boolean(form.value.holder || form.value.account || form.value.bank)
  notice.value = '已暂存申请草稿。重新进入此页可继续填写，收款信息需重新填写。'
}
const leaveDraft = () => !draftEdited.value || window.confirm('模板更改尚未保存。放弃更改？')
onBeforeRouteLeave(() => leaveForm() && leaveDraft())
watchEffect(onCleanup => {
  if (!edited.value && !draftEdited.value) return
  const warnOnUnload = (event: BeforeUnloadEvent) => {
    event.preventDefault()
    event.returnValue = ''
  }
  window.addEventListener('beforeunload', warnOnUnload)
  onCleanup(() => window.removeEventListener('beforeunload', warnOnUnload))
})

function closeReturned() {
  if (!leaveForm()) return
  edited.value = false
  returned.value = undefined
  returnedTemplate.value = undefined
}

async function copyDocumentNumber() {
  if (!detail.value) return
  try {
    await navigator.clipboard.writeText(detail.value.id)
    notice.value = '已复制单号'
  } catch {
    error.value = '无法复制，请手动选中单号'
  }
}
const node = (step: FeeDoc['route'][number]) => typeof step === 'object' ? step.name || ('#' + (step.user_id || step.id)) : '#' + step
function nodeLabel(value: string) {
  if (value === 'manager') return '申请人直属负责人'
  const person = knownApprovers.value[Number(value.replace(/^fixed:/, ''))]
  return person ? approverLabel(person) : '模板指定审批人'
}
function approvalChoices(value: string) {
  const results = approverOptions.value.map(person => ({ value: `fixed:${person.id}`, label: approverLabel(person) }))
  if (value.startsWith('fixed:') && !results.some(option => option.value === value)) results.unshift({ value, label: nodeLabel(value) })
  return [{ value: 'manager', label: '申请人直属负责人' }, ...results]
}
async function findApprovers(afterId = 0) {
  if (!afterId) {
    requestedApproverQuery = approverQuery.value
    approverNext.value = 0
    approverOptions.value = []
  }
  const search = ++approverSearch
  approverBusy.value = true
  approverError.value = ''
  try {
    const selected = [...new Set([...(draft.value?.nodes ?? []), draft.value?.extra ?? ''])]
      .filter(value => value.startsWith('fixed:'))
      .map(value => value.slice(6))
    const { items, selected: resolved, next_after_id } = await searchFeePeople(requestedApproverQuery, selected, afterId)
    if (search !== approverSearch) return
    approverOptions.value = afterId ? [...approverOptions.value, ...(items || [])] : (items || [])
    approverNext.value = next_after_id || 0
    for (const person of [...approverOptions.value, ...(resolved ?? [])]) knownApprovers.value[person.id] = person
  } catch (cause) {
    if (search === approverSearch) approverError.value = errorText(cause)
  } finally {
    if (search === approverSearch) approverBusy.value = false
  }
}
const historyLabel = (action: string) => decisions[action] || ({ submit: '提交申请', resubmit: '重新提交', payment: '登记付款' })[action] || action
const movingNode = (from: number, delta: number) => {
  const nodes = draft.value?.nodes
  const target = from + delta
  if (!nodes || target < 0 || target >= nodes.length) return
  draftEdited.value = true
  ;[nodes[from], nodes[target]] = [nodes[target], nodes[from]]
}
const list = (value: string) => [...new Set(value.split(/[,，\n]/).map(item => item.trim()).filter(Boolean))]

function cents(value: string) {
  const parts = /^(\d{1,10})(?:\.(\d{1,2}))?$/.exec(value.trim())
  if (!parts) throw new Error('请输入有效金额，最多两位小数')
  const result = Number(parts[1]) * 100 + Number((parts[2] || '').padEnd(2, '0'))
  if (!result || !Number.isSafeInteger(result)) throw new Error('申请金额必须大于零')
  return result
}
async function run(action: () => Promise<void>) {
  if (busy.value) return
  busy.value = true
  error.value = ''
  notice.value = ''
  try { await action() }
  catch (cause) { error.value = errorText(cause) }
  finally { busy.value = false }
}

function changeChannel() {
  form.value.holder = ''
  form.value.account = ''
  form.value.bank = ''
  for (const key of ['holder', 'account', 'bank']) {
    touched.delete(key)
    delete fieldErrors.value[key]
  }
}

function selectTemplate() {
  const previousChannel = form.value.channel
  form.value.channel = active.value?.channels?.[0] || ''
  if (previousChannel !== form.value.channel) changeChannel()
  form.value.extra = {}
  for (const key of touched) {
    if (key.startsWith('extra-')) {
      touched.delete(key)
      delete fieldErrors.value[key]
    }
  }
}
async function loadTemplates() {
  const data = await api<{ items: FeeTemplate[]; level?: string }>('/oa-api/finance/templates' + (props.mode === 'templates' ? '?draft=1' : ''))
  level.value = data.level || ''
  templates.value = data.items ?? []
  loadedTemplates.value = true
  if (props.mode !== 'apply' || form.value.template_id) return
  if (savedApplication?.owner !== draftOwner.value) savedApplication = undefined
  const localDraft = savedApplication?.site === form.value.site ? savedApplication?.values : undefined
  const template = templates.value.find(item => item.id === localDraft?.template_id)
  missingTemplateDraft.value = Boolean(localDraft && !template)
  form.value.template_id = template?.id || templates.value[0]?.id || ''
  selectTemplate()
  if (localDraft && template) {
    form.value.amount = localDraft.amount
    form.value.reason = localDraft.reason
    if (template.channels.includes(localDraft.channel)) form.value.channel = localDraft.channel
    form.value.extra = Object.fromEntries(Object.entries(localDraft.extra).filter(([key]) => template.fields?.includes(key)))
    form.value.files = localDraft.files.map(file => ({ ...file }))
    draftRestored.value = true
  }
}
async function loadDocs() {
  detail.value = undefined
  loadedDocs.value = false
  const data = await api<{ items: FeeDoc[] }>('/oa-api/finance/documents?' + new URLSearchParams({ queue: queue.value }))
  docs.value = data.items ?? []
  page.value = Math.min(page.value, pages.value)
  loadedDocs.value = true
  approvalsChanged.value = false
}
async function returnToQueue() {
  detail.value = undefined
  revealed.value = false
  if (route.query.id) await router.replace({ query: {} })
  if (approvalsChanged.value || !loadedDocs.value) await loadDocs()
}
async function open(id: string) {
  revealed.value = false
  touched.delete('date')
  delete fieldErrors.value.date
  detail.value = await api<FeeDoc>('/oa-api/finance/documents/' + encodeURIComponent(id))
  approval.value = { decision: 'approve', comment: '' }
  payment.value = { date: dateInChina(), receipt: '', receiptName: '' }
  receiptError.value = ''
  await nextTick()
  heading.value?.focus()
}
async function revealAccount() {
  if (!detail.value?.id) return
  detail.value = await api<FeeDoc>('/oa-api/finance/documents/' + encodeURIComponent(detail.value.id) + '?reveal=1')
  revealed.value = true
}
async function upload(file: File) {
  const data = new FormData()
  data.set('file', file)
  return api<Evidence>('/oa-api/finance/files', { method: 'POST', body: data })
}
function uploadEvidence() {
  const input = files.value
  if (!input) return
  const selected = Array.from(input.files ?? [])
  input.value = ''
  if (form.value.files.length + selected.length > 15) {
    fieldErrors.value.files = '每张申请最多上传 15 份凭证'
    return
  }
  void run(async () => {
    for (const file of selected) {
      try {
        form.value.files.push(await upload(file))
      } catch (cause) {
        fieldErrors.value.files = file.name + ' 上传失败。' + errorText(cause)
        break
      }
    }
  })
}
function uploadReceipt() {
  void run(async () => {
    const input = receipt.value
    const file = input?.files?.[0]
    if (!input || !file) return
    input.value = ''
    receiptError.value = ''
    try {
      const uploaded = await upload(file)
      payment.value.receipt = uploaded.id
      payment.value.receiptName = uploaded.name || file.name
    } catch (cause) {
      receiptError.value = file.name + ' 上传失败。' + errorText(cause)
    }
  })
}
async function submit() {
  if (!active.value) return
  fieldErrors.value = {}
  let amountCents = 0
  const amountError = amountProblem()
  if (amountError) fieldErrors.value.amount = amountError
  else amountCents = cents(form.value.amount)
  if (!form.value.reason.trim()) fieldErrors.value.reason = '请输入费用用途'
  if (!form.value.holder.trim()) fieldErrors.value.holder = '请输入收款账户名称'
  if (accountRequired.value && !form.value.account.trim()) fieldErrors.value.account = '请输入收款账号'
  if (!form.value.bank.trim()) fieldErrors.value.bank = '请输入开户银行或收款平台'
  for (const [index, field] of (active.value.fields || []).entries()) {
    if (active.value.required?.includes(field) && !form.value.extra[field]?.trim()) fieldErrors.value['extra-' + index] = '请输入' + field
  }
  if (!form.value.files.length) fieldErrors.value.files = '请上传至少一份单据凭证'
  if (Object.keys(fieldErrors.value).length) {
    Object.keys(fieldErrors.value).forEach(key => touched.add(key))
    await nextTick()
    applicationForm.value?.querySelector<HTMLElement>('[aria-invalid="true"]')?.focus()
    return
  }
  if (!form.value.request_id) form.value.request_id = crypto.randomUUID()
  const { amount: _amount, files: evidence, ...fields } = form.value
  const path = '/oa-api/finance/documents' + (returned.value ? '/' + encodeURIComponent(returned.value) + '/resubmit' : '')
  const result = await send<FeeDoc>(path, {
    ...fields, reason: fields.reason.trim(), holder: fields.holder.trim(), account: fields.account.trim(),
    bank: fields.bank.trim(), amount_cents: amountCents, files: evidence.map(file => file.id),
  })
  expenseActivity.invalidate()
  edited.value = false
  if (returned.value && result?.id) {
    returned.value = undefined
    returnedTemplate.value = undefined
    detail.value = result
    approvalsChanged.value = true
    notice.value = '申请已重新提交，等待审批'
  } else {
    savedApplication = undefined
    await router.push({ path: '/fees/mine', query: result?.id ? { id: result.id, submitted: '1' } : {} })
  }
}
function editReturned() {
  if (!detail.value || !own.value || detail.value.state !== 'returned') return
  const document = detail.value
  void run(async () => {
    returned.value = document.id
    returnedTemplate.value = document.template
    fieldErrors.value = {}
    touched.clear()
    form.value = {
      request_id: crypto.randomUUID(), template_id: document.template.id,
      amount: (document.amount_cents / 100).toFixed(2), reason: document.reason, channel: document.channel,
      holder: document.holder, account: '', bank: document.bank, extra: { ...document.extra },
      files: (document.files || []).map(id => ({ id, name: '原附件 ' + id.slice(0, 6) })), site: document.site,
    }
  })
}
async function act() {
  if (!detail.value) return
  const id = detail.value.id
  if (approval.value.decision !== 'approve') {
    const reject = approval.value.decision === 'reject'
    const subject = `${detail.value.template?.name || '费用申请'}（${id}），金额 ${major(detail.value.amount_cents)}`
    if (!window.confirm(reject ? `拒绝${subject}？拒绝将结束本次审批。` : `退回${subject}？申请人修改后可重新提交。`)) return
  }
  detail.value = await send<FeeDoc>(`/oa-api/finance/documents/${encodeURIComponent(id)}/decision`, { ...approval.value, current: detail.value.current, version: detail.value.version })
  expenseActivity.invalidate()
  approvalsChanged.value = true
  notice.value = ({
    waiting: '已同意，申请已转交下一位审批人',
    approved: '审批已完成，单据已进入待付款',
    returned: '已退回申请人修改',
    rejected: '已拒绝，审批已结束',
  } as Record<string, string>)[detail.value.state] || '审批结果已保存'
}
async function pay() {
  if (!detail.value || !payment.value.receipt) return
  const id = detail.value.id
  if (!window.confirm(`为申请「${id}」登记实付 ${major(detail.value.amount_cents)}？\n收款对象：${detail.value.holder || '—'}\n实付日期：${payment.value.date}\n此记录将计入支出报表。`)) return
  detail.value = await send<FeeDoc>(`/oa-api/finance/documents/${encodeURIComponent(id)}/payment`, {
    date: payment.value.date, receipt: payment.value.receipt, amount_cents: detail.value.amount_cents,
  })
  expenseActivity.invalidate()
  revealed.value = false
  approvalsChanged.value = true
  notice.value = '付款已登记，回单已归档'
}
async function loadReport(initial = false) {
  reportLoaded.value = false
  report.value = await api<FeeReport>('/oa-api/finance/report?' + new URLSearchParams({ period: period.value.trim() }))
  reportPeriod.value = period.value
  const requested = Number(route.query.page)
  reportPage.value = initial && Number.isInteger(requested) ? Math.min(Math.max(1, requested), reportPages.value) : 1
  reportLoaded.value = true
}
function beginTemplate(template?: FeeTemplate) {
  if (!leaveDraft()) return
  draftEdited.value = false
  selection.value = template?.id || ''
  draft.value = template ? { ...template } : { id: crypto.randomUUID(), name: '', type: '报销', channels: [], fields: [], required: [], evidence: '', nodes: [], threshold_cents: 0, extra: '', version: 0 }
  editingId.value = template?.id || ''
  draft.value.channels = [...(draft.value.channels || [])]
  draft.value.required = [...(draft.value.required || [])]
  draft.value.nodes = [...(draft.value.nodes || [])]
  templateFields.value = { fields: draft.value.fields?.join(', ') || '', threshold: (draft.value.threshold_cents / 100).toFixed(2) }
  approverQuery.value = ''
  approverOptions.value = []
  void findApprovers()
}
async function saveTemplate(publish: boolean) {
  if (!draft.value?.id) throw new Error('请填写模板 ID')
  if (!templateForm.value?.reportValidity()) return
  if (!draft.value.channels.length) throw new Error('请选择至少一种支付方式')
  if (!draft.value.nodes.length) throw new Error('请添加至少一个审批节点')
  const fields = list(templateFields.value.fields)
  const threshold = Number(templateFields.value.threshold)
  const body = {
    ...draft.value, fields, required: draft.value.required.filter(field => fields.includes(field)),
    nodes: draft.value.nodes.map(value => value.trim()),
    threshold_cents: threshold === 0 ? 0 : cents(templateFields.value.threshold), publish,
  }
  await send('/oa-api/finance/templates/' + encodeURIComponent(draft.value.id), body, 'PUT')
  draftEdited.value = false
  await loadTemplates()
  draft.value = undefined
  notice.value = publish ? '模板已发布' : '草稿已保存'
}
async function loadGrants() {
  grantsLoaded.value = false
  grantedPeople.value = (await api<{ items: FinanceGrant[] }>('/oa-api/finance/grants')).items || []
  grantPage.value = Math.min(grantPage.value, grantPages.value)
  grantsLoaded.value = true
}
async function saveFinanceAccess() {
  const { pay, report } = grants.value
  if (!grantTarget.value) throw new Error('请选择经办人')
  const { id } = grantTarget.value
  if (!window.confirm(`为 ${grantName(grantTarget.value)}（账号 #${id}）保存财务授权？付款登记：${pay ? '允许' : '不允许'}；支出统计：${report ? '允许' : '不允许'}。`)) return
  await send('/oa-api/finance/actions/' + encodeURIComponent(id), { pay, report }, 'PUT')
  grantTarget.value = undefined
  await loadGrants()
  notice.value = pay || report ? '财务经办权限已保存' : '财务经办权限已撤销'
}
async function saveDelegate() {
  if (!delegateTarget.value) throw new Error('请选择受托人')
  const { id } = delegateTarget.value
  if (!window.confirm(`为 ${grantName(delegateTarget.value)}（账号 #${id}）保存模板委托？${delegateLevels[delegate.value.level]}。`)) return
  await send('/oa-api/finance/delegates/' + encodeURIComponent(id), { level: delegate.value.level }, 'PUT')
  delegateTarget.value = undefined
  await loadGrants()
  notice.value = delegate.value.level === 'none' ? '模板维护委托已撤销' : '模板维护委托已保存'
}

onMounted(() => void run(async () => {
  if (!allowed.value) return
  if (props.mode === 'apply' || props.mode === 'templates') await loadTemplates()
  if (props.mode === 'templates' && isAdmin.value && route.query.manage === 'publish') openGrantPanel(delegatePanel.value)
  if (props.mode === 'templates' && isAdmin.value) await loadGrants()
  if (['mine', 'approval', 'document'].includes(props.mode) && typeof route.query.id === 'string') {
    await open(route.query.id)
    if (props.mode === 'mine' && route.query.submitted === '1' && own.value) notice.value = '申请已提交，等待审批'
  } else if (props.mode === 'approval' || props.mode === 'mine') {
    await loadDocs()
  }
  if (props.mode === 'report') await loadReport(true)
  if (props.mode === 'templates' && canEditTemplate.value && route.query.new === '1') beginTemplate()
}))
watch(() => route.query.id, (id, previous) => {
  if (props.mode !== 'document' || !allowed.value || typeof id !== 'string' || id === previous) return
  detail.value = undefined
  void run(() => open(id))
})
</script>

<template>
<div class="oa-page">
  <PageHead :title="title" :subtitle="({ mine: '跟踪费用申请的审批进度与付款记录', approval: '处理轮到自己的审批和已获授权的付款登记', templates: '维护费用类型、审批节点与经办权限', report: '按实付日期查询费用支出' } as Record<string, string>)[mode]">
    <RouterLink v-if="mode !== 'apply' && can('finance.apply')" class="oa-button oa-button-primary" to="/fees/apply">＋ 发起费用申请</RouterLink>
  </PageHead>
  <nav class="fee-tabs" aria-label="费用中心">
    <RouterLink v-for="tab in feeTabs.filter(item => item.actions.some(can))" :key="tab.mode" :to="'/fees/' + tab.mode"
      :aria-current="mode === tab.mode ? 'page' : undefined">{{ tab.mode === 'approval' && !can('finance.approve') ? '待付款' : tab.label }}</RouterLink>
    <RouterLink v-if="canUseExpenseActivity" :to="{ name: 'MeNotifications', query: { section: 'expense' } }">
      费用动态<span v-if="expenseActivity.todoCount"> · {{ expenseActivity.todoCount }} 项待办</span>
    </RouterLink>
  </nav>
  <section v-if="!allowed" class="oa-panel">
    <OAEmpty kind="inbox" title="当前页面不可用" :description="accessMessage" />
  </section>
  <template v-else>
  <div v-if="error && !showQueue" class="fee-banner fee-banner-error" role="alert">
    <CircleAlert :size="17" aria-hidden="true" /> <span>{{ error }}</span>
    <button v-if="mode === 'apply' && !loadedTemplates" type="button" @click="run(loadTemplates)">重试加载模板</button>
    <button v-else-if="mode === 'templates' && !loadedTemplates" type="button" @click="run(loadTemplates)">重试加载模板</button>
    <button v-else-if="mode === 'document' && !detail && typeof route.query.id === 'string'" type="button" @click="run(() => open(String(route.query.id)))">重试读取单据</button>
  </div>
  <p v-if="notice" class="fee-banner fee-banner-success" role="status"><Check :size="17" aria-hidden="true" /> {{ notice }}</p>
  <p v-if="busy && !showQueue" class="oa-muted" role="status">{{ detail ? '正在办理单据…' : '正在处理…' }}</p>

  <div v-if="mode === 'apply' || returned" class="oa-split fee-apply" :class="{ 'fee-apply-empty': !active }">
    <section class="oa-panel">
      <div class="fee-section-heading"><FileText :size="20" aria-hidden="true" /><h2>{{ returned ? '修改退回单据' : '填写申请' }}</h2></div>
      <p v-if="returned && lastReturn?.comment" class="fee-banner">退回意见　{{ lastReturn.comment }}</p>
      <p v-if="draftRestored" class="fee-banner" role="status">已恢复申请草稿，请重新填写收款信息。</p>
      <p v-if="missingTemplateDraft" class="fee-banner" role="status">草稿使用的模板当前不可用。</p>
      <OAEmpty v-if="loadedTemplates && !active" kind="files" title="当前暂无可用费用模板" :description="templateGuide">
        <RouterLink v-if="canManageTemplates" class="oa-button oa-button-primary" :to="{ path: '/fees/templates', query: isAdmin && !canEditTemplate ? { manage: 'publish' } : {} }">前往模板与授权</RouterLink>
        <RouterLink v-else class="oa-button" to="/fees/mine">返回我的申请</RouterLink>
      </OAEmpty>
      <form v-if="active" ref="applicationForm" class="oa-form fee-required" novalidate @input="edited = true" @change="edited = true" @submit.prevent="run(submit)">
        <div class="oa-form-row">
          <label><span>申请模板<i aria-hidden="true">*</i></span><select v-model="form.template_id" :disabled="!!returned" required @change="selectTemplate"><option v-for="item in returnedTemplate ? [returnedTemplate] : templates" :key="item.id" :value="item.id">{{ item.name }}</option></select></label>
          <label><span>金额（元）<i aria-hidden="true">*</i></span><input v-model="form.amount" inputmode="decimal" placeholder="例如 120.50" required v-bind="fieldAttrs('amount')" @blur="validateField('amount', amountProblem, true)" @input="validateField('amount', amountProblem)" /><small v-if="fieldErrors.amount" id="fee-error-amount" class="fee-error">{{ fieldErrors.amount }}</small></label>
        </div>
        <label><span>用途与说明<i aria-hidden="true">*</i></span><textarea v-model="form.reason" required maxlength="2000" placeholder="说明本笔支出的用途" v-bind="fieldAttrs('reason')" @blur="validateField('reason', () => requiredProblem(form.reason, '请输入费用用途'), true)" @input="validateField('reason', () => requiredProblem(form.reason, '请输入费用用途'))" /><small v-if="fieldErrors.reason" id="fee-error-reason" class="fee-error">{{ fieldErrors.reason }}</small></label>
        <fieldset class="oa-fieldset oa-form-row">
          <legend>收款信息</legend>
          <label><span>付款方式<i aria-hidden="true">*</i></span><select v-model="form.channel" required @change="changeChannel"><option v-for="item in active?.channels || []" :key="item" :value="item">{{ channels[item] || item }}</option></select></label>
          <label><span>{{ form.channel === 'bank' ? '单位开户全称' : '收款人姓名' }}<i aria-hidden="true">*</i></span><input v-model="form.holder" required maxlength="240" placeholder="填写与收款账户一致的名称" v-bind="fieldAttrs('holder')" @blur="validateField('holder', () => requiredProblem(form.holder, '请输入收款账户名称'), true)" @input="validateField('holder', () => requiredProblem(form.holder, '请输入收款账户名称'))" /><small v-if="fieldErrors.holder" id="fee-error-holder" class="fee-error">{{ fieldErrors.holder }}</small></label>
          <label><span>{{ form.channel === 'wallet' ? '收款平台账号' : form.channel === 'bank' ? '对公银行账号' : '银行卡号' }}<i v-if="accountRequired" aria-hidden="true">*</i></span><input v-model="form.account" maxlength="128" autocomplete="off" :placeholder="accountRequired ? '填写实际收款账号' : '留空保留原收款账号'" :required="accountRequired" v-bind="fieldAttrs('account')" @blur="validateField('account', () => accountRequired ? requiredProblem(form.account, '请输入收款账号') : '', true)" @input="validateField('account', () => accountRequired ? requiredProblem(form.account, '请输入收款账号') : '')" /><small v-if="fieldErrors.account" id="fee-error-account" class="fee-error">{{ fieldErrors.account }}</small></label>
          <label><span>{{ form.channel === 'wallet' ? '收款平台' : '开户银行' }}<i aria-hidden="true">*</i></span><input v-model="form.bank" required maxlength="240" :placeholder="form.channel === 'wallet' ? '例如支付宝' : '填写收款账户的开户银行名称'" v-bind="fieldAttrs('bank')" @blur="validateField('bank', () => requiredProblem(form.bank, '请输入开户银行或收款平台'), true)" @input="validateField('bank', () => requiredProblem(form.bank, '请输入开户银行或收款平台'))" /><small v-if="fieldErrors.bank" id="fee-error-bank" class="fee-error">{{ fieldErrors.bank }}</small></label>
        </fieldset>
        <div v-if="active?.fields?.length" class="oa-form-row">
          <label v-for="(field, index) in active?.fields || []" :key="field"><span>{{ field }}<i v-if="active?.required?.includes(field)" aria-hidden="true">*</i></span><input v-model="form.extra[field]" maxlength="1000" :placeholder="'填写' + field" :required="active?.required?.includes(field)" v-bind="fieldAttrs('extra-' + index)" @blur="validateField('extra-' + index, () => active?.required?.includes(field) ? requiredProblem(form.extra[field], '请输入' + field) : '', true)" @input="validateField('extra-' + index, () => active?.required?.includes(field) ? requiredProblem(form.extra[field], '请输入' + field) : '')" /><small v-if="fieldErrors['extra-' + index]" :id="'fee-error-extra-' + index" class="fee-error">{{ fieldErrors['extra-' + index] }}</small></label>
        </div>
        <label><span>{{ active?.evidence || '凭据附件' }}<i aria-hidden="true">*</i></span><input ref="files" type="file" accept=".pdf,.png,.jpg,.jpeg" multiple v-bind="fieldAttrs('files', 'fee-file-help')" @blur="validateField('files', fileProblem, true)" @change="delete fieldErrors.files; uploadEvidence()" /><small id="fee-file-help">PDF、PNG、JPEG，单个文件不超过 10 MB，最多 15 份。</small><small v-if="fieldErrors.files" id="fee-error-files" class="fee-error">{{ fieldErrors.files }}</small></label>
        <div v-for="file in form.files" :key="file.id" class="oa-actions"><span>{{ file.name }}</span><button type="button" @click="edited = true; form.files = form.files.filter(item => item.id !== file.id)">移除</button></div>
        <div class="oa-actions fee-form-actions">
          <button v-if="returned" type="button" @click="closeReturned">返回单据</button>
          <button v-else type="button" :disabled="busy || !draftOwner" @click="saveApplicationDraft">暂存草稿</button>
          <button class="oa-button-primary" :disabled="busy || !active">{{ returned ? '重新提交' : '提交申请' }}</button>
        </div>
        <small v-if="!returned" class="oa-muted">草稿保留在当前标签页内存。刷新或关闭页面将清空，收款信息需重新填写。</small>
      </form>
    </section>
    <section v-if="active" class="oa-panel">
      <h2>审批路径</h2>
      <p class="oa-muted">提交后生成实际审批人，按顺序流转。</p>
      <ol class="oa-steps fee-timeline">
        <li v-for="(item, index) in active.nodes || []" :key="index">{{ nodeLabel(item) }}</li>
        <li v-if="active.threshold_cents && active.extra">达到 {{ major(active.threshold_cents) }} 时加签 {{ nodeLabel(active.extra) }}</li>
        <li>审批通过后登记付款</li>
      </ol>
    </section>
  </div>

  <template v-else-if="mode === 'mine' || mode === 'approval' || mode === 'document'">
    <div v-if="mode === 'approval' && !detail && can('finance.approve') && canPay" class="oa-actions oa-person-tabs">
      <button v-if="can('finance.approve')" :aria-pressed="queue === 'todo'" :disabled="busy" @click="queue = 'todo'; page = 1; run(loadDocs)">审批待办</button>
      <button v-if="canPay" :aria-pressed="queue === 'pay'" :disabled="busy" @click="queue = 'pay'; page = 1; run(loadDocs)">待付款</button>
    </div>
    <section v-if="!detail" class="oa-panel">
      <p v-if="showQueue && busy" class="oa-muted" role="status">正在读取{{ queueLabel }}…</p>
      <div v-else-if="showQueue && error" class="fee-banner fee-banner-error" role="alert">
        <CircleAlert :size="17" aria-hidden="true" /><span>{{ error }}</span>
        <button v-if="typeof route.query.id === 'string'" type="button" @click="run(() => open(String(route.query.id)))">重试读取单据</button>
        <button type="button" @click="run(returnToQueue)">{{ route.query.id ? '返回列表' : '重新加载列表' }}</button>
      </div>
      <OAEmpty v-else-if="!busy && !error && (mode === 'document' || !docs.length)" kind="inbox"
        :title="mode === 'document' ? '未选中费用单据' : queueEmpty"
        :description="mode === 'document' ? '请从费用列表或通知打开一张单据。' : queueGuide">
        <RouterLink v-if="mode === 'mine'" class="oa-button oa-button-primary" to="/fees/apply">发起费用申请</RouterLink>
        <RouterLink v-else-if="mode === 'document'" class="oa-button" :to="documentReturn">{{ route.query.from === 'notifications' ? '返回费用动态' : '返回费用列表' }}</RouterLink>
      </OAEmpty>
      <div v-if="!busy && !error && docs.length" class="oa-table-wrap">
        <table class="oa-table"><thead><tr><th>申请 / 单号</th><th>申请人</th><th class="oa-number">金额</th><th>来源</th><th>状态</th><th class="oa-action-col">操作</th></tr></thead><tbody>
          <tr v-for="item in visibleDocs" :key="item.id"><td><b class="oa-ellipsis" :title="item.template?.name">{{ item.template?.name }}</b><small class="oa-muted oa-ellipsis" :title="item.id">{{ item.id }}</small></td><td>#{{ item.applicant_id }}</td><td class="oa-number">{{ major(item.amount_cents) }}</td><td>{{ item.site === 'asset' ? '资产办理区' : 'OA' }}</td><td><span :class="['oa-pill', 'oa-state-' + item.state]">{{ status(item.state) }}</span></td><td class="oa-action-col"><button :disabled="busy" @click="run(() => open(item.id))">打开单据</button></td></tr>
        </tbody></table>
      </div>
      <nav v-if="!busy && !error && pages > 1" class="oa-actions fee-pager" aria-label="费用单据分页">
        <span>共 {{ docs.length }} 项 · {{ page }} / {{ pages }} 页</span>
        <button :disabled="page <= 1" @click="page--">上一页</button>
        <button :disabled="page >= pages" @click="page++">下一页</button>
      </nav>
    </section>
    <template v-else>
      <div class="oa-actions fee-back">
        <RouterLink v-if="mode === 'document'" class="oa-button" :to="documentReturn"><ArrowLeft :size="15" aria-hidden="true" /> {{ route.query.from === 'notifications' ? '返回费用动态' : '返回列表' }}</RouterLink>
        <button v-else type="button" :disabled="busy" @click="run(returnToQueue)"><ArrowLeft :size="15" aria-hidden="true" /> 返回{{ returnLabel }}</button>
        <button v-if="own && detail.state === 'returned'" class="oa-button-primary" :disabled="busy" @click="editReturned">修改并重提</button>
      </div>
      <div class="oa-split fee-document">
      <section class="oa-panel">
        <h2 ref="heading" tabindex="-1" class="fee-wrap">{{ detail.template?.name || '费用单据' }} <span :class="['oa-pill', 'oa-state-' + detail.state]">{{ status(detail.state) }}</span></h2>
        <dl class="oa-kv fee-detail">
          <div><dt>单号</dt><dd>{{ detail.id }} <button type="button" @click="copyDocumentNumber">复制单号</button></dd></div><div><dt>申请人</dt><dd>#{{ detail.applicant_id }}</dd></div>
          <div><dt>申请金额</dt><dd>{{ major(detail.amount_cents) }}</dd></div><div><dt>支付渠道</dt><dd>{{ channels[detail.channel] || detail.channel }}</dd></div>
          <div><dt>收款对象</dt><dd>{{ detail.holder }}</dd></div><div><dt>收款账号</dt><dd>{{ detail.account }} <button v-if="canPay && !own && detail.state === 'approved' && !revealed" :disabled="busy" @click="run(revealAccount)">查看完整账号</button></dd></div><div><dt>银行 / 平台</dt><dd>{{ detail.bank || '—' }}</dd></div>
          <div><dt>用途与说明</dt><dd>{{ detail.reason }}</dd></div><div v-for="(value, key) in detail.extra" :key="key"><dt>{{ key }}</dt><dd>{{ value }}</dd></div>
        </dl>
        <div class="oa-actions">
          <button v-for="id in detail.files || []" :key="id" :disabled="busy" @click="run(() => download('/oa-api/finance/files/' + id, '凭证-' + id))">下载凭证</button>
        </div>
        <p v-if="detail.state === 'returned' && lastReturn?.comment" class="fee-banner">退回意见　{{ lastReturn.comment }}</p>
      </section>
      <section class="oa-panel">
        <h2>审批与付款记录</h2>
        <p class="oa-muted">{{ stageGuide }}</p>
        <ol v-if="detail.route?.length" class="oa-steps fee-timeline"><li v-for="(item, index) in detail.route" :key="index" :class="{ 'fee-step-current': detail.state === 'waiting' && detail.current === index }">{{ node(item) }}<small v-if="detail.current > index"> · 已通过</small><small v-else-if="detail.state === 'waiting' && detail.current === index"> · 当前节点</small></li></ol>
        <form v-if="(queue === 'todo' || mode === 'document') && can('finance.approve') && mayDecide" class="oa-form" @submit.prevent="run(act)">
          <label>审批结果<select v-model="approval.decision"><option v-for="(text, key) in decisions" :key="key" :value="key">{{ text }}</option></select></label>
          <label>审批意见<textarea v-model.trim="approval.comment" maxlength="2000" :required="approval.decision !== 'approve'" /></label>
          <button :class="approval.decision === 'reject' ? 'fee-danger' : 'oa-button-primary'" :disabled="busy">{{ approvalButton }}</button>
        </form>
        <form v-if="(queue === 'pay' || mode === 'document') && !own && canPay && detail.state === 'approved'" class="oa-form" @submit.prevent="run(pay)">
          <label><span>实付日期<i aria-hidden="true">*</i></span><input v-model="payment.date" type="date" :max="dateInChina()" required v-bind="fieldAttrs('date', 'fee-date-help')" @blur="validateField('date', dateProblem, true)" @input="validateField('date', dateProblem)" /><small id="fee-date-help">填写实际付款日，最晚为今天。</small><small v-if="fieldErrors.date" id="fee-error-date" class="fee-error">{{ fieldErrors.date }}</small></label>
          <div class="fee-payment"><Wallet :size="19" aria-hidden="true" /><div><span>应付金额</span><strong>{{ major(detail.amount_cents) }}</strong></div></div>
          <p class="oa-muted">实际转账后填写实付日期、上传回单。</p>
          <label>付款回单 *<input ref="receipt" type="file" accept=".pdf,.png,.jpg,.jpeg" :disabled="busy" aria-describedby="fee-receipt-help" @change="uploadReceipt" /><small id="fee-receipt-help">PDF、PNG、JPEG，单个文件不超过 10 MB。</small></label>
          <p v-if="receiptError" class="fee-error" role="alert">{{ receiptError }}</p>
          <div v-if="payment.receipt" class="oa-actions" role="status"><span>已上传：{{ payment.receiptName }}</span><button type="button" :disabled="busy" @click="payment.receipt = ''; payment.receiptName = ''">移除回单</button></div>
          <button class="oa-button-primary" :disabled="busy || !payment.receipt || !!dateProblem()">确认登记实付</button>
        </form>
        <template v-if="detail.payment">
          <dl class="oa-kv"><div><dt>实付日期</dt><dd>{{ detail.payment.date }}</dd></div><div><dt>实付金额</dt><dd>{{ major(detail.payment.amount_cents) }}</dd></div></dl>
          <button v-if="detail.payment.receipt" @click="run(() => download('/oa-api/finance/files/' + detail?.payment?.receipt, '付款回单'))">下载回单</button>
        </template>
        <details v-if="detail.history?.length" class="fee-disclosure">
          <summary>操作记录（{{ detail.history.length }}）</summary>
          <dl class="oa-kv fee-detail"><div v-for="(entry, index) in detail.history" :key="index"><dt>{{ historyLabel(entry.action) }} <time :datetime="entry.at">{{ feeDateTime(entry.at) }}</time></dt><dd>{{ entry.comment || '—' }}</dd></div></dl>
        </details>
      </section>
      </div>
    </template>
  </template>

  <template v-else-if="mode === 'templates'">
    <div class="oa-split oa-sidebar-split">
      <section class="oa-panel">
        <h2>费用模板</h2>
        <div class="oa-list"><button v-for="item in templates" :key="item.id" @click="beginTemplate(item)" :aria-current="selection === item.id" :disabled="!canEditTemplate">{{ item.name }} · v{{ item.version }}</button></div>
        <OAEmpty v-if="loadedTemplates && !templates.length" kind="files" title="当前暂无费用模板"
          :description="canEditTemplate ? '新建模板，配置支付方式、凭证和审批节点。发布后员工可以发起申请。' : templateGuide">
          <button v-if="canEditTemplate && !draft" class="oa-button-primary" @click="beginTemplate()">创建第一个费用模板</button>
          <button v-if="isAdmin && !draft" @click="openGrantPanel(delegatePanel)">指定模板发布人</button>
          <button v-if="isAdmin" @click="openGrantPanel(financeGrantPanel)">设置付款经办人</button>
        </OAEmpty>
        <button v-if="loadedTemplates && templates.length && canEditTemplate" :class="{ 'oa-button-primary': !draft }" @click="beginTemplate()">新建费用模板</button>
      </section>
      <section v-if="draft && canEditTemplate" class="oa-panel">
        <h2>{{ editingId ? '维护模板' : '新建费用模板' }}</h2>
        <p class="oa-muted">保存草稿供维护人员继续编辑。发布后生效，历史单据继续使用原模板版本。</p>
        <form ref="templateForm" class="oa-form fee-required" @input="draftEdited = true" @change="draftEdited = true" @submit.prevent="run(() => saveTemplate(false))">
          <div class="oa-form-row">
            <label><span>名称<i aria-hidden="true">*</i></span><input v-model.trim="draft.name" placeholder="例如差旅报销" required /></label>
            <label><span>单据类型<i aria-hidden="true">*</i></span><select v-model="draft.type"><option>报销</option><option>付款</option></select></label>
            <label>可填写字段<input v-model="templateFields.fields" placeholder="费用归属，合同编号" /></label>
            <label><span>凭证要求<i aria-hidden="true">*</i></span><input v-model.trim="draft.evidence" placeholder="例如发票" required /></label>
          </div>
          <fieldset class="oa-fieldset"><legend>支付方式</legend><div class="oa-checks"><label v-for="(label, key) in channels" :key="key"><input v-model="draft.channels" type="checkbox" :value="key" />{{ label }}</label></div></fieldset>
          <fieldset v-if="list(templateFields.fields).length" class="oa-fieldset"><legend>必填字段</legend><div class="oa-checks"><label v-for="field in list(templateFields.fields)" :key="field"><input v-model="draft.required" type="checkbox" :value="field" />{{ field }}</label></div></fieldset>
          <fieldset class="oa-fieldset oa-form"><legend>审批节点</legend>
            <div class="oa-actions oa-filter"><label>搜索审批人<input v-model="approverQuery" placeholder="姓名 / 拼音 / 部门 / 工号 / 完整手机号" @keydown.enter.prevent="findApprovers()" /></label><button type="button" :disabled="approverBusy" @click="findApprovers()">{{ approverBusy ? '搜索中…' : '搜索' }}</button></div>
            <p v-if="approverError" class="oa-error" role="alert">{{ approverError }}</p>
            <p v-else-if="!approverBusy && !approverOptions.length" class="oa-muted" role="status">没有匹配的在职人员。</p>
            <div v-if="approverNext" class="oa-actions"><small role="status">已加载 {{ approverOptions.length }} 名候选</small><button type="button" :disabled="approverBusy" @click="findApprovers(approverNext)">加载更多候选</button></div>
            <div v-for="(item, index) in draft.nodes" :key="index" class="oa-actions">
              <label>第 {{ index + 1 }} 步<select v-model="draft.nodes[index]" :aria-label="'第 ' + (index + 1) + ' 步'" required><option value="" disabled>选择审批人</option><option v-for="option in approvalChoices(item)" :key="option.value" :value="option.value">{{ option.label }}</option></select></label>
              <button type="button" :disabled="index === 0" @click="movingNode(index, -1)">上移</button>
              <button type="button" :disabled="index + 1 === draft.nodes.length" @click="movingNode(index, 1)">下移</button>
              <button type="button" @click="draftEdited = true; draft.nodes.splice(index, 1)">移除</button>
            </div>
            <button type="button" @click="draftEdited = true; draft.nodes.push(draft.nodes.length ? '' : 'manager')">添加审批节点</button>
          </fieldset>
          <div class="oa-form-row">
            <label>金额达到时加签（元）<input v-model.trim="templateFields.threshold" inputmode="decimal" pattern="[0-9]+(\.[0-9]{1,2})?" required /></label>
            <label v-if="Number(templateFields.threshold) > 0">加签审批人<select v-model="draft.extra" aria-label="加签审批人" required><option value="" disabled>选择审批人</option><option v-for="option in approvalChoices(draft.extra)" :key="option.value" :value="option.value">{{ option.label }}</option></select></label>
          </div>
          <div class="oa-actions"><button type="button" :disabled="busy" @click="if (leaveDraft()) { draftEdited = false; draft = undefined; selection = '' }">关闭编辑</button><button :disabled="busy">保存草稿</button><button v-if="level === 'publish'" class="oa-button-primary" type="button" :disabled="busy" @click="run(() => saveTemplate(true))">发布模板</button></div>
        </form>
      </section>
    </div>
    <section v-if="isAdmin" class="oa-panel">
      <div class="oa-actions fee-grants-heading"><h2>已授权人员</h2>
        <button type="button" @click="selectGrantTarget(undefined, 'template'); openGrantPanel(delegatePanel)">新增模板维护授权</button>
        <button type="button" @click="selectGrantTarget(undefined, 'finance'); openGrantPanel(financeGrantPanel)">新增财务经办授权</button>
      </div>
      <p class="oa-muted">查看显式授权。超级管理员和具备公司级费用管理权限的账号可直接维护、发布模板。付款登记和报表各自授权。</p>
      <p v-if="!grantsLoaded && busy" role="status" class="oa-muted">正在读取已授权人员…</p>
      <button v-else-if="!grantsLoaded" type="button" @click="run(loadGrants)">重新读取已授权人员</button>
      <OAEmpty v-else-if="!grantedPeople.length" kind="list" title="暂无显式授权" description="可为模板维护人员或财务经办人员新增授权。" />
      <div v-else class="oa-table-wrap">
        <table class="oa-table">
          <thead><tr><th>人员 / 部门 / 工号</th><th>账号状态</th><th>模板维护</th><th>付款登记</th><th>费用报表</th><th>操作</th></tr></thead>
          <tbody>
            <tr v-for="item in visibleGrants" :key="item.id">
              <td>{{ grantName(item) }}</td><td>{{ grantStatuses[item.status] || item.status }}</td>
              <td>{{ item.level === 'none' ? '—' : delegateLevels[item.level] }}</td>
              <td>{{ item.pay ? '已授权' : '—' }}</td><td>{{ item.report ? '已授权' : '—' }}</td>
              <td><div class="oa-actions">
                <button v-if="item.status === 'active' || item.level !== 'none'" type="button" :disabled="busy" @click="editGrant(item, 'template')">{{ item.status === 'active' ? '修改模板维护' : '撤销模板维护' }}</button>
                <button v-if="item.status === 'active' || item.pay || item.report" type="button" :disabled="busy" @click="editGrant(item, 'finance')">{{ item.status === 'active' ? '修改财务经办' : '撤销财务经办' }}</button>
              </div></td>
            </tr>
          </tbody>
        </table>
      </div>
      <nav v-if="grantPages > 1" class="oa-actions fee-pager" aria-label="已授权人员分页">
        <span>共 {{ grantedPeople.length }} 人 · {{ grantPage }} / {{ grantPages }} 页</span>
        <button :disabled="grantPage <= 1" @click="grantPage--">上一页</button>
        <button :disabled="grantPage >= grantPages" @click="grantPage++">下一页</button>
      </nav>
    </section>
    <details v-if="isAdmin" ref="delegatePanel" class="oa-panel fee-disclosure">
      <summary>模板维护委托</summary>
      <form class="oa-form" @submit.prevent="run(saveDelegate)">
        <div class="oa-form-row">
          <p v-if="inactiveGrant(delegateTarget)" class="oa-muted">{{ delegateTarget && grantName(delegateTarget) }} · 只可撤销历史授权</p>
          <FeePersonSelect v-else :model-value="delegateTarget" label="受托人" :disabled="busy || !grantsLoaded" @update:model-value="person => selectGrantTarget(person, 'template')" />
          <label>维护权限<select v-model="delegate.level" :disabled="inactiveGrant(delegateTarget)"><option value="none">无模板维护权</option><option value="draft">编辑草稿</option><option value="publish">编辑并发布</option></select></label>
        </div>
        <div class="oa-actions"><button type="button" :disabled="busy || !delegateTarget" @click="selectGrantTarget(undefined, 'template')">取消选择</button><button :disabled="busy || !delegateTarget || !grantsLoaded">{{ inactiveGrant(delegateTarget) ? '确认撤销模板维护' : '保存委托' }}</button></div>
      </form>
    </details>
    <details v-if="isAdmin" ref="financeGrantPanel" class="oa-panel fee-disclosure">
      <summary>财务经办授权</summary><p class="oa-subtitle">付款登记和费用报表分别授权。保存时会应用这里选择的两项权限。</p>
      <form class="oa-form oa-person-tabs" @submit.prevent="run(saveFinanceAccess)">
        <p v-if="inactiveGrant(grantTarget)" class="oa-muted">{{ grantTarget && grantName(grantTarget) }} · 只可撤销历史授权</p>
        <FeePersonSelect v-else :model-value="grantTarget" label="经办人" :disabled="busy || !grantsLoaded" @update:model-value="person => selectGrantTarget(person, 'finance')" />
        <div class="oa-checks"><label><input v-model="grants.pay" type="checkbox" :disabled="inactiveGrant(grantTarget)" />付款登记</label><label><input v-model="grants.report" type="checkbox" :disabled="inactiveGrant(grantTarget)" />费用报表</label></div>
        <div class="oa-actions"><button type="button" :disabled="busy || !grantTarget" @click="selectGrantTarget(undefined, 'finance')">取消选择</button><button :disabled="busy || !grantTarget || !grantsLoaded">{{ inactiveGrant(grantTarget) ? '确认撤销财务经办' : '保存财务授权' }}</button></div>
      </form>
    </details>
  </template>

  <section v-else-if="mode === 'report'" class="oa-panel">
    <h2>按实付日期统计</h2>
    <p class="oa-muted">支持按年、月或日查询，例如 {{ defaultPeriodExample }}。</p>
      <form class="oa-actions oa-filter" @submit.prevent="run(() => loadReport())"><label>期间（年 / 月 / 日）<input v-model.trim="period" pattern="[0-9]{4}(-[0-9]{2}){0,2}" required /></label><button :disabled="busy">查询</button></form>
      <dl v-if="reportLoaded && report" class="fee-totals"><div><dt>期间实付</dt><dd>{{ major(report.amount_cents) }}</dd></div><div><dt>实付张数</dt><dd>{{ report.count }}</dd></div><div v-if="report.unpaid_cents != null"><dt>已批未付</dt><dd>{{ major(report.unpaid_cents) }}</dd></div></dl>
      <div v-if="reportLoaded && visibleReport.length" class="oa-table-wrap"><table class="oa-table"><thead><tr><th>单号</th><th class="oa-number">实付日期</th><th class="oa-number">实付金额</th><th class="oa-action-col">操作</th></tr></thead><tbody><tr v-for="item in visibleReport" :key="item.id"><td><span class="oa-ellipsis" :title="item.id">{{ item.id }}</span></td><td class="oa-number">{{ item.payment?.date }}</td><td class="oa-number">{{ major(item.payment?.amount_cents) }}</td><td class="oa-action-col"><RouterLink class="fee-row-action" :to="{ path: '/fees/document', query: { id: item.id, from: 'report', period: reportPeriod, page: reportPage } }">打开单据</RouterLink></td></tr></tbody></table></div>
      <OAEmpty v-if="reportLoaded && !visibleReport.length" kind="chart" title="这个期间暂无实付记录" description="请选择其他期间查询。只有已登记实付的单据计入实付明细。" />
      <nav v-if="reportLoaded && reportPages > 1" class="oa-actions fee-pager" aria-label="费用报表分页">
        <span>共 {{ report?.items?.length }} 项 · {{ reportPage }} / {{ reportPages }} 页</span>
        <button :disabled="reportPage <= 1" @click="reportPage--">上一页</button>
        <button :disabled="reportPage >= reportPages" @click="reportPage++">下一页</button>
      </nav>
  </section>
  </template>
</div>
</template>

<style scoped>
.fee-tabs { display: flex; gap: 20px; overflow-x: auto; margin-bottom: 12px; border-bottom: 1px solid var(--oa-line); }
.fee-tabs a { display: inline-flex; align-items: center; min-height: 36px; flex: none; padding-inline: 4px; color: var(--oa-secondary); text-decoration: none; border-bottom: 2px solid transparent; }
.fee-tabs a:hover, .fee-tabs a[aria-current=page] { color: var(--oa-accent); }
.fee-tabs a[aria-current=page] { border-color: currentColor; font-weight: 600; }
.fee-section-heading, .fee-banner, .fee-back a, .fee-back button { display: flex; align-items: center; gap: 8px; }
.fee-section-heading { margin-bottom: 16px; color: var(--oa-accent); }
.fee-section-heading h2 { color: var(--oa-ink); margin: 0; }
.fee-banner { margin: 0 0 12px; padding: 10px 12px; border: 1px solid var(--oa-line); border-radius: 8px; background: var(--oa-surface); overflow-wrap: anywhere; }
.fee-banner svg { flex: 0 0 auto; }
.fee-banner-error { color: var(--oa-danger); }
.fee-banner-success { color: var(--oa-success); }
.fee-back { margin-bottom: 12px; }
.fee-payment { display: flex; align-items: center; gap: 10px; padding: 12px; border: 1px solid var(--oa-line); border-radius: 8px; }
.fee-payment span { display: block; font-size: 12px; color: var(--oa-muted); }
.fee-payment strong { font-size: 22px; font-variant-numeric: tabular-nums; }
.fee-document { align-items: start; }
.fee-timeline li { padding-block: 5px; }
.fee-step-current { color: var(--oa-accent); font-weight: 600; }
.fee-grants-heading { margin-bottom: 8px; }
.fee-grants-heading h2 { margin: 0 auto 0 0; }
.fee-form-actions { padding-top: 10px; border-top: 1px solid var(--oa-line); justify-content: flex-end; }
.fee-pager { justify-content: flex-end; margin-top: 12px; color: var(--oa-muted); }
.fee-disclosure { margin-top: 12px; scroll-margin-top: 12px; }
.fee-disclosure summary { cursor: pointer; font-weight: 600; min-height: 32px; line-height: 32px; }
.fee-row-action { display: inline-flex; align-items: center; min-height: 32px; min-width: 32px; }
.fee-detail > div { display: grid; grid-template-columns: minmax(76px, 23%) minmax(0, 1fr); gap: 8px; }
.fee-detail dd { text-align: start; max-inline-size: 40em; overflow-wrap: anywhere; }
.fee-wrap { overflow-wrap: anywhere; }
.fee-error, .oa-page button.fee-danger { color: var(--oa-danger); }
.oa-form label i[aria-hidden] { margin-inline-start: 4px; color: var(--oa-danger); font-style: normal; }
.fee-disclosure[open] > summary { margin-bottom: 12px; }
.fee-totals { display: flex; flex-wrap: wrap; gap: 12px 32px; margin: 16px 0; padding-block: 12px; border-block: 1px solid var(--oa-line); }
.fee-totals dt { color: var(--oa-muted); font-size: 12px; }
.fee-totals dd { margin: 4px 0 0; font-size: 24px; font-weight: 600; font-variant-numeric: tabular-nums; }
@media (min-width: 960px) {
  .fee-apply:not(.fee-apply-empty) { grid-template-columns: minmax(0, 1fr) minmax(230px, 31%); align-items: start; }
}
</style>
