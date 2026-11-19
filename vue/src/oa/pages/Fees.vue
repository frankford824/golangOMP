<script lang="ts">
let savedApplication: {
  owner: string
  site: string
  values: { template_id: string; amount: string; reason: string; channel: string; extra: Record<string, string>; files: { id: string; name: string }[] }
} | undefined
</script>

<script setup lang="ts">
import { ArrowLeft, Check, ChevronDown, ChevronRight, CircleAlert, FileText, Wallet } from 'lucide-vue-next'
import OAEmpty from '../components/OAEmpty.vue'
import { computed, onBeforeUnmount, nextTick, onMounted, ref, watch, watchEffect } from 'vue'
import { onBeforeRouteLeave, onBeforeRouteUpdate, RouterLink, useRoute, useRouter } from 'vue-router'
import FeePersonSelect from '../components/FeePersonSelect.vue'
import { searchFeePeople, approverLabel, type FeePerson as Approver } from '../feePeople'
import { feeTabs } from '../routes'
import DesktopSlot from '@/oa/components/DesktopSlot.vue'
import PageHead from '../components/PageHead.vue'
import { api, dateInChina, errorText, money, send } from '../api'
import { useOAAccess } from '../access'
import { expenseActivityActions, useExpenseActivity } from '../expenseActivity'
import { oaReturnLabel } from '../pageTitle'
import { feeApplicant, feeDateTime, feeStatus as status } from '../feeDisplay'
import FeeEvidence from '../components/FeeEvidence.vue'
import FeeTimeline from '../components/FeeTimeline.vue'
import FeeArchive from '../components/FeeArchive.vue'
import FeeConfirmModal from '../components/FeeConfirmModal.vue'
import { useFeeConfirm } from '../useFeeConfirm'
import FinanceRecords from './FinanceRecords.vue'
import OAPageReturn from '../components/OAPageReturn.vue'
import { goBackTo, safeReturnPath } from '../returnNavigation'
import '../styles.css'

const { can, currentProfile } = useOAAccess()
const expenseActivity = useExpenseActivity()
const { ask, state: confirmation, finish: finishConfirmation } = useFeeConfirm()
const canUseExpenseActivity = computed(() => expenseActivityActions.some(can))

type FeeTemplate = { id: string; name: string; type: string; channels: string[]; fields: string[]; required: string[]; evidence: string; nodes: string[]; threshold_cents: number; extra: string; version: number }
type Evidence = { id: string; name: string }
type Decision = {
  action: string; comment?: string; actor_id?: number; at: string
  operator_id?: number; operator_name?: string; operator_no?: number
  operator_department?: string; node_name?: string; created_at?: string
  before_amount_cents?: number; after_amount_cents?: number; adjustment_reason?: string
  bank_reference?: string; receipt?: string; payment_date?: string
}
type FeeDoc = { id: string; applicant_id: number; template: FeeTemplate; amount_cents: number; reason: string; channel: string; holder: string; account: string; bank: string; extra: Record<string, string>; files: string[]; site: string; state: string; route: (number | string | { id?: number; name?: string; user_id?: number })[]; current: number; version: number; history?: Decision[]; payment?: { amount_cents: number; date: string; receipt: string; bank_reference?: string }
  applicant_name?: string; applicant_employee_no?: number; department_id?: number; department_name?: string; team_name?: string
  approved_amount_cents?: number; submitted_at?: string; approved_at?: string; route_nodes?: string[]
}
const props = defineProps<{ mode: string }>()
const route = useRoute()
const router = useRouter()
const isApplicationOrigin = (path: string) => [
  '/', '/operations', '/me', '/tasks', '/cost-rules', '/asset-center', '/users',
  '/fees/approval', '/fees/mine', '/fees/templates', '/fees/report', '/fees/document', '/fees/done', '/fees/cc', '/fees/ledger',
].includes(path)
const applicationReturn = computed(() => safeReturnPath(route.query.applyFrom, isApplicationOrigin)
  || safeReturnPath(router.options.history.state?.back, isApplicationOrigin) || '/fees/mine')
const applicationReturnLabel = computed(() => oaReturnLabel(applicationReturn.value))
const applicationLink = computed(() => ({ path: '/fees/apply', query: { applyFrom: route.fullPath } }))
const activeTab = computed(() => feeTabs.find(tab => tab.mode === props.mode))
const recordsMode = computed(() => {
  if (props.mode === 'ledger') return 'all'
  if (props.mode === 'report' || props.mode === 'done' || props.mode === 'cc') return props.mode
})
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
type FeeQueue = 'mine' | 'todo' | 'pay'
const queue = ref<FeeQueue>(requestedQueue())
function requestedQueue(): FeeQueue {
  if (props.mode === 'mine') return 'mine'
  return can('finance.pay') && (route.query.queue === 'pay' || !can('finance.approve')) ? 'pay' : 'todo'
}
function requestedPage(value: unknown) {
  return typeof value === 'string' && /^\d+$/.test(value) && Number.isSafeInteger(Number(value)) ? Math.max(1, Number(value)) : 1
}
function queueLocation(id?: string, nextQueue = queue.value, nextPage = page.value) {
  return {
    path: props.mode === 'mine' ? '/fees/mine' : '/fees/approval',
    query: { ...route.query, state: nextQueue === queue.value ? route.query.state : undefined, submitted: undefined, queue: nextQueue === 'pay' ? 'pay' : undefined, page: nextPage > 1 ? String(nextPage) : undefined, id },
  }
}
const busy = ref(false)
const error = ref('')
const notice = ref('')
const templates = ref<FeeTemplate[]>([])
function feeTypeLabel(value: string) {
  return value === 'payment' || value === '付款' ? '付款' : '报销'
}
const applicationTemplates = computed(() => ['报销', '付款'].map(type => ({
  type, items: templates.value.filter(item => feeTypeLabel(item.type) === type),
})).filter(group => group.items.length))
const docs = ref<FeeDoc[]>([])
const page = ref(requestedPage(route.query.page))
const pageSize = 15
const totalDocs = ref(0)
const pages = computed(() => Math.max(1, Math.ceil(totalDocs.value / pageSize)))
const visibleDocs = docs
const detail = ref<FeeDoc>()
const archiveDocument = computed(() => detail.value ? { ...detail.value, account: detail.value.account ? '•••• ' + detail.value.account.slice(-4) : '' } : undefined)
let pendingLocationSync = false
let disposed = false
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
const desktopApproval = ref(false)
const inlineApproval = ref(false)
const pinApproval = ref(false)
const approvalClosed = ref(false)
const queuePage = ref<HTMLElement>()
let desktopQuery: MediaQueryList | undefined
let inlineQuery: MediaQueryList | undefined
let pinnedQuery: MediaQueryList | undefined
const showQueue = computed(() => allowed.value && (!detail.value || desktopApproval.value) && ['mine', 'approval'].includes(props.mode))
const openDocumentLink = (id: string, keepSelection = false) => ({
  path: '/fees/document',
  query: { id, queue: queue.value, feeFrom: router.resolve(queueLocation(keepSelection ? id : undefined)).fullPath },
})
function selectDocument(id: string) {
  if (busy.value) return
  if (route.query.id === id) {
    void run(() => open(id))
  } else {
    const target = queueLocation(id)
    if (typeof route.query.id === 'string') void router.replace(target)
    else void router.push(target)
  }
}
function linkToInspector(event: MouseEvent, id: string) {
  if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return
  event.preventDefault()
  selectDocument(id)
}
function syncApprovalSize() {
  desktopApproval.value = Boolean(desktopQuery?.matches)
  inlineApproval.value = Boolean(inlineQuery?.matches)
  pinApproval.value = Boolean(pinnedQuery?.matches)
  if (pinApproval.value && !approvalClosed.value && !detail.value && visibleDocs.value[0]) {
    void run(() => open(visibleDocs.value[0].id, false))
  }
}
function selectFeeRow(event: MouseEvent, id: string) {
  if (desktopApproval.value && !(event.target as HTMLElement).closest('a,button,input,select,[role=button]')) selectDocument(id)
}
function navigateApproval(event: KeyboardEvent) {
  if (!desktopApproval.value || !detail.value || busy.value || event.defaultPrevented || event.altKey || event.ctrlKey || event.metaKey) return
  if (event.key === 'Escape') {
    event.preventDefault()
    void run(returnToQueue)
    return
  }
  if ((event.target as HTMLElement).closest('input,select,textarea,[contenteditable=true]')) return
  const delta = event.key === 'ArrowUp' ? -1 : event.key === 'ArrowDown' ? 1 : 0
  if (!delta) return
  event.preventDefault()
  const target = visibleDocs.value[visibleDocs.value.findIndex(item => item.id === detail.value?.id) + delta]
  if (target) selectDocument(target.id)
}
async function refreshAfterAction(id: string, source: string, advance: boolean) {
  if (disposed || route.path !== '/fees/approval' || route.fullPath !== source) return
  const index = Math.max(0, docs.value.findIndex(item => item.id === id))
  await loadDocs(false)
  if (!advance || !desktopApproval.value || disposed || route.fullPath !== source) return
  let next = docs.value.slice(index).find(item => item.id !== id)
  if (!next && page.value < pages.value) {
    await loadDocs(false, page.value + 1)
    next = docs.value.find(item => item.id !== id)
  }
  next ||= docs.value.find(item => item.id !== id)
  if (disposed || route.fullPath !== source) return
  if (!next) {
    approvalClosed.value = true
    detail.value = undefined
  }
  await router.replace(queueLocation(next?.id))
}
const selection = ref('')
const form = ref({ request_id: '', template_id: '', amount: '', reason: '', channel: '', holder: '', account: '', bank: '', extra: {} as Record<string, string>, files: [] as Evidence[], site: route.query.site === 'asset' ? 'asset' : 'main' })
const returned = ref<string>()
const returnedTemplate = ref<FeeTemplate>()
const level = ref('')
type FinanceGrant = Approver & { status: string; level: string; pay: boolean; report: boolean }
const grantedPeople = ref<FinanceGrant[]>([])
const grantsLoaded = ref(false)
const hasPayHandler = computed(() => grantedPeople.value.some(person => person.status === 'active' && person.pay))
const grantPage = ref(1)
const grantPages = computed(() => Math.max(1, Math.ceil(grantedPeople.value.length / 15)))
const visibleGrants = computed(() => grantedPeople.value.slice((grantPage.value - 1) * 15, grantPage.value * 15))
const grantStatuses: Record<string, string> = { active: '在职', inactive: '停用或离职', deleted: '已删除', missing: '账号已不可用' }
const grantName = (item: Approver) => item.name ? approverLabel(item) : '账号 #' + item.id
const revealed = ref(false)
const approval = ref({ comment: '', reduced: false, amount: '', reason: '' })
const payment = ref({ date: dateInChina(), receipt: '', receiptName: '', bankReference: '' })
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
const knownApprovers = ref<Record<number, Approver>>({})
const templateNodeKeys = ref<number[]>([])
const templateError = ref('')
let templateNodeKey = 0
const grants = ref({ pay: false, report: false })
const delegate = ref({ level: 'draft' })
const grantTarget = ref<Approver>()
const delegateTarget = ref<Approver>()
const canSaveFinanceGrant = computed(() => {
  if (busy.value || !grantsLoaded.value || !grantTarget.value) return false
  const current = grantedPeople.value.find(person => person.id === grantTarget.value?.id)
  return grants.value.pay !== Boolean(current?.pay) || grants.value.report !== Boolean(current?.report)
})
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
  const source = safeReturnPath(route.query.feeFrom || route.query.return_to, path => {
    if (path === '/me/notifications') return canUseExpenseActivity.value
    const tab = feeTabs.find(tab => path === '/fees/' + tab.mode)
    if (tab && tab.mode !== 'templates') return tab.actions.some(can)
    if (path === '/fees/mine') return can('finance.apply')
    return path === '/fees/approval' && (can('finance.approve') || can('finance.pay'))
  })
  if (source) return source
  if (route.query.from === 'notifications' && canUseExpenseActivity.value) {
    return { name: 'MeNotifications', query: { section: 'expense', ...(route.query.activity === 'results' ? { activity: 'results' } : {}), expensePage: route.query.expensePage } }
  }
  if (route.query.from === 'report' && can('finance.report')) {
    return { path: '/fees/report', query: { period: route.query.period, page: route.query.page } }
  }
  if (own.value && can('finance.apply')) return '/fees/mine'
  if (can('finance.approve') || can('finance.pay')) return '/fees/approval'
  if (can('finance.report')) return '/fees/report'
  return '/fees/mine'
})
const documentReturnLabel = computed(() => {
  if (route.query.from === 'notifications') return '返回费用动态'
  if (typeof documentReturn.value === 'string') {
    const path = documentReturn.value.split(/[?#]/, 1)[0]
    const tab = feeTabs.find(tab => path === '/fees/' + tab.mode)
    if (tab) return '返回' + tab.label
  }
  if (route.query.from === 'report') return '返回费用报表'
  return '返回费用列表'
})
const stageGuide = computed(() => ({
  waiting: '审批按模板顺序流转。',
  returned: '申请人可根据退回意见修改并重新提交。',
  rejected: '本次审批已结束。',
  approved: '审批已完成，等待财务登记实付。',
  paid: '付款已登记，可查看实付信息和回单。',
  withdrawn: '申请人已撤回，可修改并重新提交。',
  voided: '财务已作废，单据不能付款。',
})[detail.value?.state || ''] || '')
const mayDecide = computed(() => detail.value?.state === 'waiting' && !own.value && String(detail.value.route?.[detail.value.current]) === String(currentProfile.value?.id))
const major = (cents?: number) => cents == null ? '—' : money(cents / 100)
const lastReturn = computed(() => [...(detail.value?.history || [])].reverse().find(entry => entry.action === 'return'))
const showApprovalForm = computed(() => (queue.value === 'todo' || props.mode === 'document') && can('finance.approve') && mayDecide.value)
const showPaymentForm = computed(() => (queue.value === 'pay' || props.mode === 'document') && !own.value && canPay.value && detail.value?.state === 'approved')
const payable = (doc: FeeDoc) => doc.approved_amount_cents ?? doc.amount_cents
const mayVoid = computed(() => detail.value?.state === 'approved' && (isAdmin.value || (canPay.value && !own.value)))
const mayWithdraw = computed(() => own.value && detail.value?.state === 'waiting' && detail.value.current === 0)
const paymentReady = computed(() => payment.value.receipt && payment.value.bankReference.trim() && !dateProblem())
const leaveForm = async () => !edited.value || Boolean(await ask({
  title: '放弃费用申请？', message: '尚未提交的内容会丢失。', confirmText: '放弃并离开', danger: true,
}))
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
const leaveDraft = async () => !draftEdited.value || Boolean(await ask({
  title: '放弃模板更改？', message: '模板更改尚未保存。', confirmText: '放弃并离开', danger: true,
}))
onBeforeRouteLeave(async () => await leaveForm() && await leaveDraft())
onBeforeRouteUpdate(async (to, from) => {
  if (to.query.id === from.query.id) return true
  if (!(await leaveForm()) || !(await leaveDraft())) return false
  edited.value = false
  draftEdited.value = false
  returned.value = undefined
  returnedTemplate.value = undefined
  return true
})
watchEffect(onCleanup => {
  if (!edited.value && !draftEdited.value) return
  const warnOnUnload = (event: BeforeUnloadEvent) => {
    event.preventDefault()
    event.returnValue = ''
  }
  window.addEventListener('beforeunload', warnOnUnload)
  onCleanup(() => window.removeEventListener('beforeunload', warnOnUnload))
})

async function closeReturned() {
  if (!(await leaveForm())) return
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
function fixedApproverId(value: string) {
  const match = /^fixed:([1-9][0-9]*)$/.exec(value)
  const id = Number(match?.[1])
  return Number.isSafeInteger(id) ? id : undefined
}
function selectedApprover(value: string) {
  const id = fixedApproverId(value)
  if (id === undefined) return undefined
  return knownApprovers.value[id] ?? { id, name: '账号 #' + id }
}
function approverType(value: string) {
  if (value === 'manager') return value
  return value.startsWith('fixed:') ? 'fixed' : ''
}
function setTemplateApprover(value: string, index?: number) {
  if (!draft.value) return
  if (index === undefined) draft.value.extra = value
  else draft.value.nodes[index] = value
  draftEdited.value = true
}
function changeTemplateApproverType(event: Event, index?: number) {
  const type = (event.target as HTMLSelectElement).value
  setTemplateApprover(type === 'fixed' ? 'fixed:' : type, index)
}
function selectTemplateApprover(person: Approver | undefined, index?: number) {
  if (person) knownApprovers.value[person.id] = person
  setTemplateApprover(person ? `fixed:${person.id}` : 'fixed:', index)
}
function addTemplateNode() {
  if (!draft.value || draft.value.nodes.length >= 12) return
  draft.value.nodes.push('')
  templateNodeKeys.value.push(templateNodeKey++)
  draftEdited.value = true
}
function removeTemplateNode(index: number) {
  draft.value?.nodes.splice(index, 1)
  templateNodeKeys.value.splice(index, 1)
  draftEdited.value = true
}
async function resolveTemplateApprovers(values: string[]) {
  const ids = [...new Set(values.map(fixedApproverId).filter(id => id !== undefined))].map(String)
  if (!ids.length) return
  try {
    const { selected } = await searchFeePeople('', ids)
    for (const person of selected ?? []) knownApprovers.value[person.id] = person
  } catch {
    // Keep the saved account ID visible if the directory cannot resolve its name.
  }
}
const movingNode = (from: number, delta: number) => {
  const nodes = draft.value?.nodes
  const target = from + delta
  if (!nodes || target < 0 || target >= nodes.length) return
  draftEdited.value = true
  const keys = templateNodeKeys.value
  ;[nodes[from], nodes[target]] = [nodes[target], nodes[from]]
  ;[keys[from], keys[target]] = [keys[target], keys[from]]
}
const list = (value: string) => [...new Set(value.split(/[,，\n]/).map(item => item.trim()).filter(Boolean))]

function cents(value: string) {
  const parts = /^(\d{1,10})(?:\.(\d{1,2}))?$/.exec(value.trim())
  if (!parts) throw new Error('请输入有效金额，最多两位小数')
  const result = Number(parts[1]) * 100 + Number((parts[2] || '').padEnd(2, '0'))
  if (!result || !Number.isSafeInteger(result)) throw new Error('申请金额必须大于零')
  return result
}
async function run(action: () => Promise<void>, clearNotice = true) {
  if (busy.value) return
  busy.value = true
  error.value = ''
  if (clearNotice) notice.value = ''
  try { await action() }
  catch (cause) { error.value = errorText(cause) }
  finally {
    busy.value = false
    if (pendingLocationSync && !disposed) {
      pendingLocationSync = false
      queueMicrotask(syncLocation)
    }
  }
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
  form.value.template_id = template?.id || (templates.value.length === 1 ? templates.value[0]!.id : '')
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
let loadedQueueQuery = ''
function documentQueueQuery(targetPage: number) {
  const params = new URLSearchParams({ queue: queue.value, page: String(targetPage), size: String(pageSize) })
  for (const key of ['q', 'state', 'template', 'department', 'team', 'person', 'site', 'channel', 'start', 'end', 'basis', 'min_amount_cents', 'max_amount_cents']) {
    const value = route.query[key]
    if (typeof value === 'string') params.set(key, value)
  }
  return params.toString()
}
async function loadDocs(reset = true, targetPage = page.value) {
  if (reset) detail.value = undefined
  loadedDocs.value = false
  const requestedQuery = documentQueueQuery(targetPage)
  const data = await api<{ items: FeeDoc[]; total: number }>('/oa-api/finance/documents?' + requestedQuery)
  totalDocs.value = data.total ?? data.items?.length ?? 0
  if (targetPage > pages.value) {
    await loadDocs(reset, pages.value)
    return
  }
  docs.value = data.items ?? []
  page.value = targetPage
  loadedQueueQuery = requestedQuery
  loadedDocs.value = true
  approvalsChanged.value = false
  if (reset && pinApproval.value && !approvalClosed.value && visibleDocs.value[0]) await open(visibleDocs.value[0].id, false)
}
function changeQueue(nextQueue: 'todo' | 'pay') {
  if (nextQueue === queue.value) return
  approvalClosed.value = false
  void router.replace(queueLocation(undefined, nextQueue, 1))
}
const queueSearch = ref(typeof route.query.q === 'string' ? route.query.q : '')
const queueState = ref(typeof route.query.state === 'string' ? route.query.state : '')
watch(() => [route.query.q, route.query.state], ([keyword, state]) => {
  queueSearch.value = typeof keyword === 'string' ? keyword : ''
  queueState.value = typeof state === 'string' ? state : ''
})
function searchQueue() {
  const target = queueLocation(undefined, queue.value, 1)
  const query = { ...target.query, q: queueSearch.value.trim() || undefined, state: queueState.value || undefined }
  const location = { ...target, query }
  if (router.resolve(location).fullPath === route.fullPath) void run(() => loadDocs(true, 1))
  else void router.replace(location)
}
function changePage(target: number) {
  void router.replace(queueLocation(undefined, queue.value, target))
}
async function returnToQueue() {
  approvalClosed.value = true
  if (typeof route.query.id === 'string') {
    await goBackTo(router, queueLocation())
    return
  }
  detail.value = undefined
  revealed.value = false
  if (approvalsChanged.value || !loadedDocs.value) await loadDocs()
  await nextTick()
  queuePage.value?.focus()
}
async function open(id: string, focus = true) {
  revealed.value = false
  touched.delete('date')
  delete fieldErrors.value.date
  detail.value = await api<FeeDoc>('/oa-api/finance/documents/' + encodeURIComponent(id))
  approval.value = { comment: '', reduced: false, amount: '', reason: '' }
  payment.value = { date: dateInChina(), receipt: '', receiptName: '', bankReference: '' }
  receiptError.value = ''
  if (focus) {
    approvalClosed.value = false
    await nextTick()
    heading.value?.focus()
    queuePage.value?.querySelector('[aria-selected=true]')?.scrollIntoView({ block: 'nearest' })
  }
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
  if (!detail.value || !own.value || !['returned', 'withdrawn'].includes(detail.value.state)) return
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
function documentFacts(doc: FeeDoc): [string, string][] {
  return [['单号', doc.id], ['申请人', feeApplicant(doc)], ['申请金额', major(doc.amount_cents)]]
}
async function act(decision: 'approve' | 'return' | 'reject', advance = false) {
  if (!detail.value || !showApprovalForm.value) return
  const doc = detail.value
  const id = doc.id
  const source = route.fullPath
  let comment = approval.value.comment
  let adjustment: { approved_amount_cents?: number; adjustment_reason?: string } = {}
  if (decision === 'approve' && isAdmin.value && approval.value.reduced) {
    const amount = cents(approval.value.amount)
    if (amount >= payable(doc) || !approval.value.reason.trim()) throw new Error('请填写低于当前应付金额的核准额和核减原因')
    adjustment = { approved_amount_cents: amount, adjustment_reason: approval.value.reason.trim() }
    if (!await ask({ title: '确认核减并同意？', confirmText: '核减并同意', facts: [
      ...documentFacts(doc), ['当前应付金额', major(payable(doc))], ['核减后金额', major(amount)],
    ] })) return
  }
  if (decision !== 'approve') {
    const reject = decision === 'reject'
    const result = await ask({
      title: reject ? '拒绝费用申请？' : '退回费用申请？',
      message: reject ? '拒绝后本次审批结束。' : '申请人修改后可重新提交。',
      facts: documentFacts(doc), confirmText: decisions[decision], danger: reject,
      reasonLabel: reject ? '拒绝原因' : '退回原因',
    })
    if (!result?.reason.trim()) return
    comment = result.reason.trim()
  }
  detail.value = await send<FeeDoc>(`/oa-api/finance/documents/${encodeURIComponent(id)}/decision`, { decision, comment, current: doc.current, version: doc.version, ...adjustment })
  expenseActivity.invalidate()
  approvalsChanged.value = true
  notice.value = ({
    waiting: '已同意，申请已转交下一位审批人',
    approved: '审批已完成，单据已进入待付款',
    returned: '已退回申请人修改',
    rejected: '已拒绝，审批已结束',
  } as Record<string, string>)[detail.value.state] || '审批结果已保存'
  await refreshAfterAction(id, source, advance)
}
async function changeDocumentState(operation: 'withdraw' | 'void') {
  const doc = detail.value
  if (!doc || !(operation === 'withdraw' ? mayWithdraw.value : mayVoid.value)) return
  const withdraw = operation === 'withdraw'
  const result = await ask({
    title: withdraw ? '撤回费用申请？' : '作废费用单据？',
    message: withdraw ? '撤回后可以修改并重新提交。' : '作废后不能登记付款。',
    facts: documentFacts(doc), confirmText: withdraw ? '撤回申请' : '作废单据',
    danger: !withdraw, reasonLabel: withdraw ? undefined : '作废原因',
  })
  if (!result || (!withdraw && !result.reason.trim())) return
  const source = route.fullPath
  detail.value = await send<FeeDoc>(`/oa-api/finance/documents/${encodeURIComponent(doc.id)}/${operation}`, {
    version: doc.version, ...(!withdraw ? { reason: result.reason.trim() } : {}),
  })
  expenseActivity.invalidate()
  approvalsChanged.value = true
  notice.value = withdraw ? '申请已撤回，可以修改并重提' : '单据已作废'
  await refreshAfterAction(doc.id, source, false)
}
async function pay() {
  if (!detail.value || !showPaymentForm.value || !paymentReady.value) return
  const doc = detail.value
  const source = route.fullPath
  const bankReference = payment.value.bankReference.trim()
  if (!await ask({
    title: '确认登记实付？', message: '请确认款项已实际转出。保存后将计入实付报表。',
    confirmText: '登记实付', facts: [
      ['单号', doc.id], ['收款对象', doc.holder || '—'], ['银行 / 平台', doc.bank || '—'],
      ['收款账号', revealed.value ? (doc.account ? '尾号 ' + doc.account.slice(-4) : '—') : (doc.account || '—')], ['实付金额', major(payable(doc))],
      ['实付日期', payment.value.date], ['交易流水号', bankReference], ['付款回单', payment.value.receiptName],
    ],
  })) return
  detail.value = await send<FeeDoc>(`/oa-api/finance/documents/${encodeURIComponent(doc.id)}/payment`, {
    date: payment.value.date, receipt: payment.value.receipt, amount_cents: payable(doc), bank_reference: bankReference,
  })
  expenseActivity.invalidate()
  revealed.value = false
  approvalsChanged.value = true
  notice.value = '付款已登记，回单已归档'
  await refreshAfterAction(doc.id, source, desktopApproval.value)
}
async function beginTemplate(template?: FeeTemplate) {
  if (!(await leaveDraft())) return
  draftEdited.value = false
  templateError.value = ''
  selection.value = template?.id || ''
  draft.value = template ? { ...template } : { id: crypto.randomUUID(), name: '', type: '', channels: [], fields: [], required: [], evidence: '', nodes: [], threshold_cents: 0, extra: '', version: 0 }
  editingId.value = template?.id || ''
  if (draft.value.type === 'reimbursement') draft.value.type = '报销'
  if (draft.value.type === 'payment') draft.value.type = '付款'
  draft.value.channels = [...(draft.value.channels || [])]
  draft.value.required = [...(draft.value.required || [])]
  draft.value.nodes = [...(draft.value.nodes || [])]
  templateNodeKeys.value = draft.value.nodes.map(() => templateNodeKey++)
  templateFields.value = { fields: draft.value.fields?.join(', ') || '', threshold: draft.value.threshold_cents ? (draft.value.threshold_cents / 100).toFixed(2) : '' }
  void resolveTemplateApprovers([...draft.value.nodes, draft.value.extra ?? ''])
}
async function closeTemplate() {
  if (!(await leaveDraft())) return
  draftEdited.value = false
  draft.value = undefined
  selection.value = ''
}
async function saveTemplate(publish: boolean) {
  if (!draft.value || !canEditTemplate.value || (publish && level.value !== 'publish')) return
  templateError.value = ''
  const { type, channels: selectedChannels, nodes, extra } = draft.value
  const name = draft.value.name.trim()
  const evidence = draft.value.evidence.trim()
  const fields = list(templateFields.value.fields)
  const threshold = templateFields.value.threshold.trim()
  const amountParts = /^(\d+)(?:\.(\d{1,2}))?$/.exec(threshold || '0')
  const thresholdCents = amountParts ? Number(amountParts[1]) * 100 + Number((amountParts[2] || '').padEnd(2, '0')) : NaN
  const byteLength = (value: string) => new TextEncoder().encode(value).length
  const incompleteNode = nodes.findIndex(value => value !== 'manager' && fixedApproverId(value) === undefined)
  if (!name || byteLength(name) > 120) templateError.value = '请填写模板名称（最多 120 字节）'
  else if (type !== '报销' && type !== '付款') templateError.value = '请选择费用类型'
  else if (!evidence || byteLength(evidence) > 240) templateError.value = '请填写凭证要求（最多 240 字节）'
  else if (!selectedChannels.length || selectedChannels.some(channel => !Object.keys(channels).includes(channel))) templateError.value = '请选择至少一种有效支付方式'
  else if (fields.length > 30) templateError.value = '最多可设置 30 个自定义字段'
  else if (!nodes.length || nodes.length > 12) templateError.value = '请添加 1 至 12 个审批节点'
  else if (incompleteNode >= 0) templateError.value = `请为第 ${incompleteNode + 1} 步选择审批人`
  else if (!Number.isSafeInteger(thresholdCents)) templateError.value = '请输入有效的加签金额，最多两位小数'
  else if (thresholdCents > 0 && extra !== 'manager' && fixedApproverId(extra) === undefined) templateError.value = '请选择加签审批人'
  if (templateError.value) return
  const body = {
    ...draft.value, name, evidence, fields, required: draft.value.required.filter(field => fields.includes(field)),
    threshold_cents: thresholdCents, extra: thresholdCents > 0 ? extra : '', publish,
  }
  await send('/oa-api/finance/templates/' + encodeURIComponent(draft.value.id), body, 'PUT')
  draftEdited.value = false
  await loadTemplates()
  draft.value = undefined
  notice.value = publish ? '模板已发布，员工可发起申请' : '草稿已保存'
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
  const personName = grantName(grantTarget.value)
  if (!await ask({ title: '保存财务授权？', confirmText: '保存授权', facts: [
    ['人员', grantName(grantTarget.value)], ['付款登记', pay ? '允许' : '不允许'], ['费用报表', report ? '允许' : '不允许'],
  ] })) return
  await send('/oa-api/finance/actions/' + encodeURIComponent(id), { pay, report }, 'PUT')
  grantTarget.value = undefined
  await loadGrants()
  notice.value = pay || report ? personName + '的财务授权已保存' : personName + '的财务授权已撤销'
}
async function saveDelegate() {
  if (!delegateTarget.value) throw new Error('请选择受托人')
  const { id } = delegateTarget.value
  if (!await ask({ title: '保存模板委托？', confirmText: '保存委托', facts: [
    ['人员', grantName(delegateTarget.value)], ['权限', delegateLevels[delegate.value.level]],
  ] })) return
  await send('/oa-api/finance/delegates/' + encodeURIComponent(id), { level: delegate.value.level }, 'PUT')
  delegateTarget.value = undefined
  await loadGrants()
  notice.value = delegate.value.level === 'none' ? '模板维护委托已撤销' : '模板维护委托已保存'
}

onMounted(() => void run(async () => {
  if (!allowed.value) return
  if (props.mode === 'approval' && typeof window.matchMedia === 'function') {
    desktopQuery = window.matchMedia('(min-width: 1280px)')
    inlineQuery = window.matchMedia('(min-width: 1920px)')
    pinnedQuery = window.matchMedia('(min-width: 2300px)')
    desktopQuery.addEventListener('change', syncApprovalSize)
    inlineQuery.addEventListener('change', syncApprovalSize)
    pinnedQuery.addEventListener('change', syncApprovalSize)
    syncApprovalSize()
  }
  if (props.mode === 'apply' || props.mode === 'templates') await loadTemplates()
  if (props.mode === 'templates' && isAdmin.value && route.query.manage === 'publish') openGrantPanel(delegatePanel.value)
  if (props.mode === 'templates' && isAdmin.value) await loadGrants()
  await applyLocation(true)
  if (props.mode === 'templates' && canEditTemplate.value && route.query.new === '1') await beginTemplate()
}))
onBeforeUnmount(() => {
  disposed = true
  desktopQuery?.removeEventListener('change', syncApprovalSize)
  inlineQuery?.removeEventListener('change', syncApprovalSize)
  pinnedQuery?.removeEventListener('change', syncApprovalSize)
})
async function applyLocation(initial: boolean) {
  const id = typeof route.query.id === 'string' ? route.query.id : undefined
  if (props.mode === 'document') {
    if (id !== detail.value?.id) {
      detail.value = undefined
      if (id) await open(id)
    }
    return
  }
  if (props.mode !== 'approval' && props.mode !== 'mine') return
  const nextQueue = requestedQueue()
  const queueChanged = nextQueue !== queue.value
  const nextPage = requestedPage(route.query.page)
  if (queueChanged) docs.value = []
  queue.value = nextQueue
  const reload = !loadedDocs.value || approvalsChanged.value || documentQueueQuery(nextPage) !== loadedQueueQuery
  if (!id) {
    if (!initial && !queueChanged && detail.value && nextPage === page.value) approvalClosed.value = true
    detail.value = undefined
    revealed.value = false
  }
  if (reload) {
    try { await loadDocs(!id && (initial || !approvalClosed.value), nextPage)
      if (nextPage !== page.value) await router.replace(queueLocation(id)) }
    catch (cause) {
      if (!id) throw cause
      error.value = errorText(cause)
    }
  }
  if (id && detail.value?.id !== id) {
    detail.value = undefined
    await open(id)
  }
  if (id && initial && props.mode === 'mine' && route.query.submitted === '1' && own.value) {
    notice.value = '申请已提交，等待审批'
  }
  if (!id && !initial) {
    await nextTick()
    queuePage.value?.focus()
  }
}
function syncLocation() {
  if (disposed || !allowed.value || !['mine', 'approval', 'document'].includes(props.mode) ||
      (route.path && route.path !== '/fees/' + props.mode)) return
  if (busy.value) {
    pendingLocationSync = true
    return
  }
  void run(() => applyLocation(false), false)
}
watch(() => route.fullPath, syncLocation)
</script>

<template>
<div class="oa-page" :class="{ 'fee-approval-page': desktopApproval }" @keydown="navigateApproval">
  <OAPageReturn v-if="allowed && returned" @activate="closeReturned">返回单据</OAPageReturn>
  <OAPageReturn v-else-if="allowed && mode === 'document'" :to="documentReturn">{{ documentReturnLabel }}</OAPageReturn>
  <OAPageReturn v-else-if="allowed && mode === 'apply'" :to="applicationReturn">{{ applicationReturnLabel }}</OAPageReturn>
  <OAPageReturn v-else-if="allowed && !desktopApproval && (mode === 'mine' || mode === 'approval') && typeof route.query.id === 'string'" :disabled="busy" @activate="run(returnToQueue)">返回{{ returnLabel }}</OAPageReturn>
  <PageHead :title="title" :subtitle="({ mine: '跟踪费用申请的审批进度与付款记录', approval: '处理轮到自己的审批和已获授权的付款登记', templates: '维护费用类型、审批节点与经办权限', report: '按实付日期查询费用支出' } as Record<string, string>)[mode]">
    <RouterLink v-if="mode !== 'apply' && can('finance.apply')" class="oa-button oa-button-primary" :to="applicationLink">＋ 发起费用申请</RouterLink>
  </PageHead>
  <DesktopSlot target="tabs">
  <nav class="fee-tabs" aria-label="费用中心">
    <RouterLink v-for="tab in feeTabs.filter(item => item.actions.some(can))" :key="tab.mode" :to="'/fees/' + tab.mode"
      :aria-current="mode === tab.mode ? 'page' : undefined">{{ tab.mode === 'approval' && !can('finance.approve') ? '待付款' : tab.label }}</RouterLink>
  </nav>
  </DesktopSlot>
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
      <OAEmpty v-if="loadedTemplates && !templates.length && !returnedTemplate" kind="files" title="当前暂无可用费用模板" :description="templateGuide">
        <RouterLink v-if="canManageTemplates" class="oa-button oa-button-primary" :to="{ path: '/fees/templates', query: isAdmin && !canEditTemplate ? { manage: 'publish' } : {} }">前往模板与授权</RouterLink>
      </OAEmpty>
      <form v-if="active || templates.length" ref="applicationForm" class="oa-form fee-required" novalidate @input="edited = true" @change="edited = true" @submit.prevent="run(submit)">
        <div class="oa-form-row">
          <label><span>申请模板<i aria-hidden="true">*</i></span><select v-model="form.template_id" :disabled="!!returned" required @change="selectTemplate">
            <option disabled value="">选择报销或付款模板</option>
            <option v-if="returnedTemplate" :value="returnedTemplate.id">{{ returnedTemplate.name }}</option>
            <template v-else>
              <optgroup v-for="group in applicationTemplates" :key="group.type" :label="group.type">
                <option v-for="item in group.items" :key="item.id" :value="item.id">{{ item.name }}</option>
              </optgroup>
            </template>
          </select></label>
          <label v-if="active"><span>金额（元）<i aria-hidden="true">*</i></span><input v-model="form.amount" inputmode="decimal" placeholder="例如 120.50" required v-bind="fieldAttrs('amount')" @blur="validateField('amount', amountProblem, true)" @input="validateField('amount', amountProblem)" /><small v-if="fieldErrors.amount" id="fee-error-amount" class="fee-error">{{ fieldErrors.amount }}</small></label>
        </div>
        <template v-if="active">
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
          <button v-if="!returned" type="button" :disabled="busy || !draftOwner" @click="saveApplicationDraft">暂存草稿</button>
          <button class="oa-button-primary" :disabled="busy || !active">{{ returned ? '重新提交' : '提交申请' }}</button>
        </div>
        <small v-if="!returned" class="oa-muted">草稿保留在当前标签页内存。刷新或关闭页面将清空，收款信息需重新填写。</small>
        </template>
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
    <div v-if="mode === 'approval' && (!detail || desktopApproval) && can('finance.approve') && canPay" class="oa-actions oa-person-tabs">
      <button v-if="can('finance.approve')" :aria-pressed="queue === 'todo'" :disabled="busy" @click="changeQueue('todo')">审批待办</button>
      <button v-if="canPay" :aria-pressed="queue === 'pay'" :disabled="busy" @click="changeQueue('pay')">待付款</button>
    </div>
    <div class="fee-queue-layout" :class="{ 'has-fee-detail': detail }">
    <section v-if="!detail || desktopApproval" class="oa-panel fee-queue">
      <form class="oa-actions oa-filter fee-queue-filter" @submit.prevent="searchQueue">
        <label>搜索单据<input v-model="queueSearch" maxlength="240" placeholder="单号、姓名或模板" /></label>
        <label v-if="queue === 'mine'">状态
          <select v-model="queueState">
            <option value="">全部状态</option>
            <option v-for="state in ['waiting', 'returned', 'approved', 'paid', 'withdrawn', 'rejected', 'voided']" :key="state" :value="state">{{ status(state) }}</option>
          </select>
        </label>
        <button type="submit" :disabled="busy">查询</button>
      </form>
      <p v-if="showQueue && busy && (desktopApproval ? !loadedDocs : true)" class="oa-muted" role="status">正在读取{{ queueLabel }}…</p>
      <div v-else-if="showQueue && error" class="fee-banner fee-banner-error" role="alert">
        <CircleAlert :size="17" aria-hidden="true" /><span>{{ error }}</span>
        <button v-if="typeof route.query.id === 'string'" type="button" @click="run(() => open(String(route.query.id)))">重试读取单据</button>
        <button v-if="!route.query.id || desktopApproval" type="button" @click="run(returnToQueue)">{{ route.query.id ? '返回列表' : '重新加载列表' }}</button>
      </div>
      <OAEmpty v-else-if="!busy && !error && (mode === 'document' || !docs.length)" kind="inbox"
        :title="mode === 'document' ? '未选中费用单据' : queueEmpty"
        :description="mode === 'document' ? '请从费用列表或通知打开一张单据。' : queueGuide">
        <RouterLink v-if="mode === 'mine'" class="oa-button oa-button-primary" :to="applicationLink">发起费用申请</RouterLink>
      </OAEmpty>
      <div v-if="docs.length && (desktopApproval || (!busy && !error))" ref="queuePage" class="oa-table-wrap" role="region" aria-label="费用单据" tabindex="0">
        <table class="oa-table" role="table">
          <thead><tr><th>申请 / 单号</th><th>申请人</th><th class="oa-number">金额</th><th>来源</th><th>状态</th><th class="oa-action-col">操作</th></tr></thead>
          <tbody>
            <tr v-for="item in visibleDocs" :key="item.id" class="fee-queue-row" role="row"
              :class="{ 'fee-row-selected': detail?.id === item.id }" :aria-selected="desktopApproval ? detail?.id === item.id : undefined"
              @click="selectFeeRow($event, item.id)">
              <td role="cell">
                <RouterLink v-if="desktopApproval" :to="openDocumentLink(item.id)" custom v-slot="{ href }">
                  <a class="oa-ellipsis" :title="item.template?.name" :href="href" @click="linkToInspector($event, item.id)">{{ item.template?.name }}</a>
                </RouterLink>
                <b v-else class="oa-ellipsis" :title="item.template?.name">{{ item.template?.name }}</b>
                <small class="oa-muted oa-ellipsis" :title="item.id">{{ item.id }}</small>
              </td>
              <td role="cell" class="fee-queue-applicant">{{ feeApplicant(item) }}</td>
              <td role="cell" class="oa-number fee-queue-amount">{{ major(queue === 'pay' ? payable(item) : item.amount_cents) }}</td>
              <td role="cell" class="fee-queue-origin">{{ item.site === 'asset' ? '资产办理区' : 'OA' }}</td>
              <td role="cell" class="fee-queue-state"><span :class="['oa-pill', 'oa-state-' + item.state]">{{ status(item.state) }}</span></td>
              <td role="cell" class="oa-action-col"><button :disabled="busy" @click="selectDocument(item.id)">{{ desktopApproval ? (queue === 'pay' ? '登记付款' : '去审批') : '打开单据' }}</button></td>
            </tr>
        </tbody></table>
      </div>
      <nav v-if="docs.length && (desktopApproval || (!busy && !error && pages > 1))" class="oa-actions fee-pager" aria-label="费用单据分页">
        <span>共 {{ totalDocs }} 项 · {{ page }} / {{ pages }} 页</span>
        <button :disabled="busy || page <= 1" @click="changePage(page - 1)">上一页</button>
        <button :disabled="busy || page >= pages" @click="changePage(page + 1)">下一页</button>
      </nav>
    </section>
    <div v-if="detail && (!desktopApproval || !returned)" class="fee-inspector" :role="desktopApproval ? 'region' : undefined" :aria-label="desktopApproval ? '费用单据详情' : undefined">
      <button v-if="desktopApproval && !inlineApproval" class="fee-inspector-backdrop" type="button" aria-label="关闭单据详情" @click="run(returnToQueue)"></button>
      <div class="fee-inspector-body">
      <div class="oa-actions fee-back">
        <button v-if="desktopApproval" type="button" :disabled="busy" @click="run(returnToQueue)"><ArrowLeft :size="15" aria-hidden="true" /> 返回{{ returnLabel }}</button>
        <RouterLink v-if="desktopApproval" class="oa-button" :to="openDocumentLink(detail.id, true)" target="_blank" rel="noopener">完整详情 ↗</RouterLink>
        <button v-if="own && ['returned', 'withdrawn'].includes(detail.state)" class="oa-button-primary" :disabled="busy" @click="editReturned">修改并重提</button>
        <button v-if="mayWithdraw" type="button" :disabled="busy" @click="run(() => changeDocumentState('withdraw'))">撤回申请</button>
        <button v-if="mayVoid" type="button" class="fee-danger" :disabled="busy" @click="run(() => changeDocumentState('void'))">作废单据</button>
        <FeeArchive v-if="archiveDocument?.history?.length" :document="archiveDocument" />
      </div>
      <div class="oa-split fee-document">
      <section class="oa-panel">
        <h2 ref="heading" tabindex="-1" class="fee-wrap">{{ detail.template?.name || '费用单据' }} <span :class="['oa-pill', 'oa-state-' + detail.state]">{{ status(detail.state) }}</span></h2>
        <dl class="oa-kv fee-detail">
          <div><dt>单号</dt><dd>{{ detail.id }} <button type="button" @click="copyDocumentNumber">复制单号</button></dd></div><div><dt>申请人</dt><dd>{{ feeApplicant(detail) }}</dd></div>
          <div><dt>申请金额</dt><dd>{{ major(detail.amount_cents) }}</dd></div>
          <div v-if="detail.approved_amount_cents != null"><dt>核准金额</dt><dd>{{ major(detail.approved_amount_cents) }}</dd></div>
          <div v-if="detail.submitted_at"><dt>提交时间</dt><dd>{{ feeDateTime(detail.submitted_at) }}</dd></div>
          <div v-if="detail.approved_at"><dt>审批完成</dt><dd>{{ feeDateTime(detail.approved_at) }}</dd></div>
          <div><dt>支付渠道</dt><dd>{{ channels[detail.channel] || detail.channel }}</dd></div>
          <div v-if="detail.holder"><dt>收款对象</dt><dd>{{ detail.holder }}</dd></div><div v-if="detail.account"><dt>收款账号</dt><dd>{{ detail.account }} <button v-if="canPay && !own && detail.state === 'approved' && !revealed" :disabled="busy" @click="run(revealAccount)">查看完整账号</button></dd></div><div v-if="detail.bank"><dt>银行 / 平台</dt><dd>{{ detail.bank }}</dd></div>
          <div v-if="detail.reason"><dt>用途与说明</dt><dd>{{ detail.reason }}</dd></div><div v-for="(value, key) in detail.extra" :key="key"><dt>{{ key }}</dt><dd>{{ value }}</dd></div>
        </dl>
        <FeeEvidence v-if="detail.files?.length" :ids="detail.files" label="单据凭证" />
        <p v-if="detail.state === 'returned' && lastReturn?.comment" class="fee-banner">退回意见　{{ lastReturn.comment }}</p>
      </section>
      <section class="oa-panel">
        <h2>审批与付款记录</h2>
        <p class="oa-muted">{{ stageGuide }}</p>
        <ol v-if="detail.route?.length" class="oa-steps fee-timeline"><li v-for="(item, index) in detail.route" :key="index" :class="{ 'fee-step-current': detail.state === 'waiting' && detail.current === index }">{{ detail.route_nodes?.[index] || node(item) }}<small v-if="detail.current > index"> · 已通过</small><small v-else-if="detail.state === 'waiting' && detail.current === index"> · 当前节点</small></li></ol>
        <form v-if="showApprovalForm" id="fee-approval-form" class="oa-form" @submit.prevent="run(() => act('approve'))">
          <label>同意意见（选填）<textarea v-model.trim="approval.comment" maxlength="2000" /></label>
          <div class="oa-actions" aria-label="快捷审批意见">
            <button v-for="phrase in ['同意', '情况属实，同意报销']" :key="phrase" type="button" :disabled="busy" @click="approval.comment = phrase">{{ phrase }}</button>
          </div>
          <template v-if="isAdmin">
            <label class="fee-check"><input v-model="approval.reduced" type="checkbox" :disabled="busy" /> 核减应付金额</label>
            <template v-if="approval.reduced">
              <label>核准金额（元）<input v-model.trim="approval.amount" inputmode="decimal" placeholder="填写核减后的金额" /></label>
              <label>核减原因<textarea v-model.trim="approval.reason" maxlength="2000" /></label>
            </template>
          </template>
        </form>
        <form v-if="showPaymentForm" id="fee-payment-form" class="oa-form" @submit.prevent="run(pay)">
          <label><span>实付日期<i aria-hidden="true">*</i></span><input v-model="payment.date" type="date" :max="dateInChina()" required v-bind="fieldAttrs('date', 'fee-date-help')" @blur="validateField('date', dateProblem, true)" @input="validateField('date', dateProblem)" /><small id="fee-date-help">填写实际付款日，最晚为今天。</small><small v-if="fieldErrors.date" id="fee-error-date" class="fee-error">{{ fieldErrors.date }}</small></label>
          <div class="fee-payment"><Wallet :size="19" aria-hidden="true" /><div><span>应付金额</span><strong>{{ major(payable(detail)) }}</strong></div></div>
          <label>交易流水号 *<input v-model.trim="payment.bankReference" required maxlength="128" autocomplete="off" placeholder="银行或收款平台交易流水号" /></label>
          <p class="oa-muted">实际转账后填写实付日期、流水号并上传回单。</p>
          <label>付款回单 *<input ref="receipt" type="file" accept=".pdf,.png,.jpg,.jpeg" :disabled="busy" aria-describedby="fee-receipt-help" @change="uploadReceipt" /><small id="fee-receipt-help">PDF、PNG、JPEG，单个文件不超过 10 MB。</small></label>
          <p v-if="receiptError" class="fee-error" role="alert">{{ receiptError }}</p>
          <div v-if="payment.receipt" class="oa-actions" role="status"><span>已上传：{{ payment.receiptName }}</span><button type="button" :disabled="busy" @click="payment.receipt = ''; payment.receiptName = ''">移除回单</button></div>
        </form>
        <template v-if="detail.payment">
          <dl class="oa-kv">
            <div><dt>实付日期</dt><dd>{{ detail.payment.date }}</dd></div>
            <div><dt>实付金额</dt><dd>{{ major(detail.payment.amount_cents) }}</dd></div>
            <div v-if="detail.payment.bank_reference"><dt>交易流水号</dt><dd>{{ detail.payment.bank_reference }}</dd></div>
          </dl>
          <FeeEvidence v-if="detail.payment.receipt" :ids="[detail.payment.receipt]" label="付款回单" />
        </template>
        <FeeTimeline v-if="detail.history?.length" :entries="detail.history" />
      </section>
      </div>
      <footer v-if="showApprovalForm || showPaymentForm" class="fee-inspector-footer">
        <template v-if="showApprovalForm">
          <button type="submit" form="fee-approval-form" class="oa-button-primary" :disabled="busy">同意申请</button>
          <button v-if="desktopApproval" type="button" :disabled="busy" @click="run(() => act('approve', true))">同意并下一条</button>
          <button type="button" class="fee-action-return" :disabled="busy" @click="run(() => act('return'))">退回修改</button>
          <button type="button" class="fee-danger" :disabled="busy" @click="run(() => act('reject'))">拒绝申请</button>
        </template>
        <button v-else type="submit" form="fee-payment-form" class="oa-button-primary" :disabled="busy || !paymentReady">{{ desktopApproval ? '登记实付并下一条' : '确认登记实付' }}</button>
      </footer>
    </div>
    </div>
    </div>
  </template>

  <template v-else-if="mode === 'templates'">
    <section v-if="isAdmin && grantsLoaded && !hasPayHandler" class="oa-panel fee-setup-notice" aria-label="付款经办设置">
      <div><h2>尚未指定付款经办人</h2><p class="oa-muted">选一位在职同事负责登记付款、上传回单。审批人在各模板中设置。</p></div>
      <button type="button" @click="selectGrantTarget(undefined, 'finance'); openGrantPanel(financeGrantPanel)">设置付款经办人</button>
    </section>
    <div class="oa-split oa-sidebar-split">
      <section class="oa-panel" :class="{ 'fee-templates-empty': loadedTemplates && !templates.length && !draft }">
        <h2>费用模板</h2>
        <div class="oa-list"><button v-for="item in templates" :key="item.id" @click="beginTemplate(item)" :aria-current="selection === item.id" :disabled="!canEditTemplate">{{ feeTypeLabel(item.type) }} · {{ item.name }} · {{ item.version ? '线上 v' + item.version : '尚未发布' }}</button></div>
        <OAEmpty v-if="loadedTemplates && !templates.length" kind="files" title="当前暂无费用模板"
          :description="canEditTemplate ? '新建模板，配置支付方式、凭证和审批节点。发布后员工可以发起申请。' : templateGuide">
          <button v-if="canEditTemplate && !draft" class="oa-button-primary" @click="beginTemplate()">创建第一个费用模板</button>
          <button v-if="isAdmin && !canEditTemplate && !draft" @click="openGrantPanel(delegatePanel)">指定模板发布人</button>
        </OAEmpty>
        <button v-if="loadedTemplates && templates.length && canEditTemplate" :class="{ 'oa-button-primary': !draft }" @click="beginTemplate()">新建费用模板</button>
      </section>
      <section v-if="draft && canEditTemplate" class="oa-panel">
        <h2>{{ editingId ? '维护模板' : '新建费用模板' }}</h2>
        <p class="oa-muted">{{ level === 'publish' ? '保存草稿可继续编辑。发布后员工可用，历史单据沿用原模板版本。' : '你可以保存和修改草稿。草稿需由有发布权限的人员发布，员工才可使用。' }}</p>
        <form class="oa-form fee-required" novalidate @input="draftEdited = true" @change="draftEdited = true" @submit.prevent="run(() => saveTemplate(false))">
          <div class="oa-form-row">
            <label><span>名称<i aria-hidden="true">*</i></span><input v-model.trim="draft.name" placeholder="例如差旅报销" required /></label>
            <label><span>费用类型<i aria-hidden="true">*</i></span><select v-model="draft.type" required><option value="" disabled>选择费用类型</option><option>报销</option><option>付款</option></select></label>
            <label>自定义字段<input v-model="templateFields.fields" placeholder="例如费用归属，合同编号" /></label>
            <label><span>凭证要求<i aria-hidden="true">*</i></span><input v-model.trim="draft.evidence" placeholder="例如发票" required /></label>
          </div>
          <fieldset class="oa-fieldset"><legend>支付方式</legend><div class="oa-checks"><label v-for="(label, key) in channels" :key="key"><input v-model="draft.channels" type="checkbox" :value="key" />{{ label }}</label></div></fieldset>
          <fieldset v-if="list(templateFields.fields).length" class="oa-fieldset"><legend>必填字段</legend><div class="oa-checks"><label v-for="field in list(templateFields.fields)" :key="field"><input v-model="draft.required" type="checkbox" :value="field" />{{ field }}</label></div></fieldset>
          <fieldset class="oa-fieldset oa-form"><legend>审批节点</legend>
            <p class="oa-muted">按从上到下的顺序审批，同一人只需审批一次。选择申请人直属负责人前，请确认人员资料已配置负责人。
              指定人员本人申请时，系统会尝试使用直属负责人或备用审批人。</p>
            <div v-for="(item, index) in draft.nodes" :key="templateNodeKeys[index]" class="oa-form">
              <div class="oa-actions">
                <label>第 {{ index + 1 }} 步审批人类型<select :value="approverType(item)" required @change="changeTemplateApproverType($event, index)"><option value="" disabled>选择审批人类型</option><option value="fixed">指定人员</option><option value="manager">申请人直属负责人</option></select></label>
                <button type="button" :disabled="index === 0" :aria-label="'上移第 ' + (index + 1) + ' 步'" @click="movingNode(index, -1)">上移</button>
                <button type="button" :disabled="index + 1 === draft.nodes.length" :aria-label="'下移第 ' + (index + 1) + ' 步'" @click="movingNode(index, 1)">下移</button>
                <button type="button" :aria-label="'移除第 ' + (index + 1) + ' 步'" @click="removeTemplateNode(index)">移除</button>
              </div>
              <FeePersonSelect v-if="approverType(item) === 'fixed'" :model-value="selectedApprover(item)" :label="'第 ' + (index + 1) + ' 步指定审批人'" :disabled="busy" @update:model-value="person => selectTemplateApprover(person, index)" />
            </div>
            <button type="button" :disabled="draft.nodes.length >= 12" @click="addTemplateNode">添加审批节点</button>
          </fieldset>
          <fieldset class="oa-fieldset oa-form"><legend>金额加签</legend>
            <label>加签起始金额（元）<input v-model.trim="templateFields.threshold" inputmode="decimal" placeholder="留空或填 0 则不加签" /></label>
            <template v-if="Number(templateFields.threshold) > 0">
              <p class="oa-muted">申请金额达到该金额时，在常规审批结束后加签。</p>
              <label>加签审批人类型<select :value="approverType(draft.extra || '')" required @change="changeTemplateApproverType($event)"><option value="" disabled>选择审批人类型</option><option value="fixed">指定人员</option><option value="manager">申请人直属负责人</option></select></label>
              <FeePersonSelect v-if="approverType(draft.extra || '') === 'fixed'" :model-value="selectedApprover(draft.extra)" label="加签指定审批人" :disabled="busy" @update:model-value="person => selectTemplateApprover(person)" />
            </template>
          </fieldset>
          <p v-if="templateError" class="oa-error" role="alert">{{ templateError }}</p>
          <div class="oa-actions"><button type="button" :disabled="busy" @click="closeTemplate">关闭编辑</button><button :disabled="busy">保存草稿</button><button v-if="level === 'publish'" class="oa-button-primary" type="button" :disabled="busy" @click="run(() => saveTemplate(true))">发布模板</button></div>
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
    <details v-if="isAdmin" ref="delegatePanel" class="oa-panel fee-disclosure fee-grant-disclosure">
      <summary>模板维护委托<ChevronRight class="fee-grant-closed" :size="18" aria-hidden="true" /><ChevronDown class="fee-grant-open" :size="18" aria-hidden="true" /></summary>
      <form class="oa-form" @submit.prevent="run(saveDelegate)">
        <div class="oa-form-row">
          <p v-if="inactiveGrant(delegateTarget)" class="oa-muted">{{ delegateTarget && grantName(delegateTarget) }} · 只可撤销历史授权</p>
          <FeePersonSelect v-else :model-value="delegateTarget" label="受托人" :disabled="busy || !grantsLoaded" @update:model-value="person => selectGrantTarget(person, 'template')" />
          <label>维护权限<select v-model="delegate.level" :disabled="inactiveGrant(delegateTarget)"><option value="none">无模板维护权</option><option value="draft">编辑草稿</option><option value="publish">编辑并发布</option></select></label>
        </div>
        <div class="oa-actions"><button type="button" :disabled="busy || !delegateTarget" @click="selectGrantTarget(undefined, 'template')">取消选择</button><button :disabled="busy || !delegateTarget || !grantsLoaded">{{ inactiveGrant(delegateTarget) ? '确认撤销模板维护' : '保存委托' }}</button></div>
      </form>
    </details>
    <details v-if="isAdmin" ref="financeGrantPanel" class="oa-panel fee-disclosure fee-grant-disclosure">
      <summary>财务经办授权<ChevronRight class="fee-grant-closed" :size="18" aria-hidden="true" /><ChevronDown class="fee-grant-open" :size="18" aria-hidden="true" /></summary><p class="oa-subtitle">付款登记和费用报表分别授权。保存时会应用这里选择的两项权限。</p>
      <form class="oa-form oa-person-tabs" @submit.prevent="run(saveFinanceAccess)">
        <p v-if="inactiveGrant(grantTarget)" class="oa-muted">{{ grantTarget && grantName(grantTarget) }} · 只可撤销历史授权</p>
        <FeePersonSelect v-else :model-value="grantTarget" label="经办人" :disabled="busy || !grantsLoaded" @update:model-value="person => selectGrantTarget(person, 'finance')" />
        <div class="oa-checks"><label><input v-model="grants.pay" type="checkbox" :disabled="inactiveGrant(grantTarget)" />付款登记</label><label><input v-model="grants.report" type="checkbox" :disabled="inactiveGrant(grantTarget)" />费用报表</label></div>
        <div class="oa-actions"><button type="button" :disabled="busy || !grantTarget" @click="selectGrantTarget(undefined, 'finance')">取消选择</button><button :disabled="!canSaveFinanceGrant">{{ inactiveGrant(grantTarget) ? '确认撤销财务经办' : '保存财务授权' }}</button></div>
      </form>
    </details>
  </template>

  <FinanceRecords v-else-if="recordsMode" :mode="recordsMode" />
  </template>
  <FeeConfirmModal :config="confirmation" @finish="finishConfirmation" />
</div>
</template>

<style scoped>
.fee-action-return { color: var(--oa-warning); border-color: rgb(var(--oa-warning-rgb) / .4); }
.oa-pill.oa-state-waiting { background: rgb(var(--oa-brand-rgb) / .08); color: var(--oa-accent); border-inline-start-color: var(--oa-accent); }
.oa-pill.oa-state-approved, .oa-pill.oa-state-returned { background: rgb(var(--oa-warning-rgb) / .08); color: var(--oa-warning); border-inline-start-color: var(--oa-warning); }
.oa-pill.oa-state-paid { background: rgb(var(--oa-success-rgb) / .08); color: var(--oa-success); border-inline-start-color: var(--oa-success); }
.oa-pill.oa-state-rejected { background: rgb(var(--oa-danger-rgb) / .08); color: var(--oa-danger); }
.oa-pill.oa-state-withdrawn, .oa-pill.oa-state-voided { background: rgb(var(--yb-surface-neutral-muted)); color: var(--oa-secondary); }
.oa-form .fee-check { display: flex; align-items: center; gap: 8px; inline-size: fit-content; }
.oa-form .fee-check input { inline-size: auto; min-block-size: 0; flex: none; }
@media (max-width: 767px) {
  .fee-queue .oa-table { display: block; min-width: 0; }
  .fee-queue .oa-table thead { position: absolute; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%); }
  .fee-queue .oa-table tbody { display: grid; gap: 8px; }
  .fee-queue-row { display: grid; grid-template-columns: minmax(0, 1fr) auto; align-items: center; border: 1px solid var(--oa-line); border-radius: 8px; }
  .fee-queue-row td { display: block; min-inline-size: 0; border: 0; }
  .fee-queue-row .fee-queue-amount { grid-column: 2; grid-row: 1; color: var(--oa-accent); font-weight: 600; }
  .fee-queue-row .fee-queue-applicant { grid-row: 2; }
  .fee-queue-row .fee-queue-origin { display: none; }
  .fee-queue-row .fee-queue-state { grid-column: 2; grid-row: 2; text-align: end; }
  .fee-queue-row .oa-action-col { grid-column: 1 / -1; text-align: end; }
  .fee-inspector-body:has(.fee-inspector-footer) { padding-bottom: calc(70px + env(safe-area-inset-bottom)); }
  .fee-inspector-body .fee-inspector-footer {
    position: fixed; inset: auto 0 0; z-index: 15;
    padding: 10px 12px calc(10px + env(safe-area-inset-bottom));
    background: var(--oa-surface); border-top: 1px solid var(--oa-line);
    display: flex; justify-content: center; flex-wrap: wrap; gap: 8px;
  }
}
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
.fee-setup-notice { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
.fee-setup-notice p { margin-top: 4px; }
.fee-grants-heading { margin-bottom: 8px; }
.fee-grants-heading h2 { margin: 0 auto 0 0; }
.fee-form-actions { padding-top: 10px; border-top: 1px solid var(--oa-line); justify-content: flex-end; }
.fee-pager { justify-content: flex-end; margin-top: 12px; color: var(--oa-muted); }
.oa-form input[type='file']::file-selector-button {
  margin-inline-end: 8px;
  padding: 2px 8px;
  border: 1px solid rgb(var(--yb-border-strong));
  border-radius: 6px;
  background: rgb(var(--yb-surface));
  color: inherit;
  font: inherit;
}
.fee-disclosure { margin-top: 12px; scroll-margin-top: 12px; }
.fee-disclosure summary { cursor: pointer; font-weight: 600; min-height: 32px; line-height: 32px; }
.fee-row-action { display: inline-flex; align-items: center; min-height: 32px; min-width: 32px; }
.fee-detail > div { display: grid; grid-template-columns: minmax(76px, 23%) minmax(0, 1fr); gap: 8px; }
.fee-detail dd { text-align: start; max-inline-size: 40em; overflow-wrap: anywhere; }
.fee-wrap { overflow-wrap: anywhere; }
.fee-error, .oa-page button.fee-danger { color: var(--oa-danger); }
.oa-form label i[aria-hidden] { margin-inline-start: 4px; color: var(--oa-danger); font-style: normal; }
.fee-disclosure[open] > summary { margin-bottom: 12px; }
.fee-templates-empty { grid-column: 1 / -1; }
.fee-templates-empty :deep(.oa-empty-state__description) { max-width: 100%; }
.fee-templates-empty :deep(.oa-empty-state__actions) { display: flex; flex-wrap: wrap; justify-content: center; gap: 8px; }
.fee-grant-disclosure > summary {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  list-style: none;
  border-radius: 6px;
  padding-inline: 8px;
}
.fee-grant-disclosure > summary:hover { background: rgb(var(--yb-surface-muted)); }
.fee-grant-disclosure > summary::-webkit-details-marker,
.fee-grant-disclosure:not([open]) .fee-grant-open,
.fee-grant-disclosure[open] .fee-grant-closed { display: none; }
.fee-totals { display: flex; flex-wrap: wrap; gap: 12px 32px; margin: 16px 0; padding-block: 12px; border-block: 1px solid var(--oa-line); }
.fee-totals dt { color: var(--oa-muted); font-size: 12px; }
.fee-totals dd { margin: 4px 0 0; font-size: 24px; font-weight: 600; font-variant-numeric: tabular-nums; }
@media (min-width: 960px) {
  .fee-apply:not(.fee-apply-empty) { grid-template-columns: minmax(0, 1fr) minmax(230px, 31%); align-items: start; }
}

@media (min-width: 1280px) {
  .fee-tabs { display: flex; flex-wrap: nowrap; gap: 0; align-items: center; margin: 0; border: 0; }
  .fee-tabs > a {
    display: inline-flex;
    align-items: center;
    min-height: 43px;
    padding: 0 8px;
    border: 0;
    border-radius: 0;
    border-bottom: 2px solid transparent;
    white-space: nowrap;
    background: transparent;
    font-size: 13px;
  }
  .fee-tabs > a[aria-current='page'] { border-bottom-color: rgb(var(--yb-brand)); background: transparent; }

  .oa-page.fee-approval-page {
    display: flex;
    flex-direction: column;
    height: calc(100dvh - 68px);
    min-height: 0;
    overflow: hidden;
  }
  .fee-approval-page .fee-queue-layout {
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    flex: 1 1 0;
    min-height: 0;
  }
  .fee-approval-page .fee-queue {
    display: flex;
    flex-direction: column;
    min-height: 0;
    min-width: 0;
    overflow: hidden;
    padding: 0;
  }
  .fee-approval-page .fee-queue > .oa-empty-state { flex: 1 1 0; }
  .fee-approval-page .fee-queue .oa-table-wrap {
    flex: 1 1 0;
    min-height: 0;
    max-height: none;
    overflow: auto;
    border-radius: 0;
  }
  .fee-approval-page .fee-queue table { table-layout: fixed; min-width: 710px; }
  .fee-approval-page .fee-queue th { position: sticky; top: 0; z-index: 2; background: rgb(var(--yb-surface-muted)); }
  .fee-approval-page .fee-queue th:nth-child(2) { width: 88px; }
  .fee-approval-page .fee-queue th:nth-child(3) { width: 128px; }
  .fee-approval-page .fee-queue th:nth-child(4) { width: 100px; }
  .fee-approval-page .fee-queue th:nth-child(5) { width: 100px; }
  .fee-approval-page .fee-queue th:nth-child(6) { width: 110px; }
  .fee-approval-page .fee-queue td { height: 50px; padding-block: 4px; }
  .fee-approval-page .fee-queue td:first-child a { display: block; min-block-size: 20px; }
  .fee-approval-page .fee-queue tr { cursor: pointer; }
  .fee-approval-page .fee-queue tbody tr:hover { background: rgb(var(--yb-surface-hover)); }
  .fee-approval-page .fee-queue tbody .fee-row-selected { background: rgb(var(--yb-brand-soft)); }
  .fee-approval-page .fee-queue :is(th,td).oa-number { font-family: var(--yb-font-mono); }
  .fee-approval-page .fee-pager {
    margin: auto 0 0;
    padding-inline: 10px;
    min-height: 40px;
    flex: 0 0 40px;
    border-top: 1px solid rgb(var(--yb-border));
  }
  .fee-approval-page .fee-pager button { min-height: 30px; padding-block: 0; }
  .fee-approval-page .fee-inspector {
    position: fixed;
    inset: 44px 0 0;
    z-index: 90;
    display: grid;
    justify-content: end;
    grid-template-columns: minmax(0, 400px);
  }
  .fee-approval-page .fee-inspector button.fee-inspector-backdrop {
    width: 100%;
    position: absolute;
    inset: 0;
    border: 0;
    border-radius: 0;
    background: rgb(var(--yb-shadow) / .22);
  }
  .fee-approval-page .fee-inspector-body {
    z-index: 1;
    min-width: 0;
    min-height: 0;
    overflow: auto;
    background: rgb(var(--yb-surface));
    border-left: 1px solid rgb(var(--yb-border));
  }
  .fee-approval-page .fee-back {
    position: sticky;
    top: 0;
    z-index: 2;
    display: flex;
    justify-content: space-between;
    gap: 8px;
    padding: 8px 12px;
    margin: 0;
    border-bottom: 1px solid rgb(var(--yb-border));
    background: rgb(var(--yb-surface));
  }
  .fee-approval-page .fee-document {
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    gap: 8px;
    padding: 10px;
  }
  .fee-inspector-footer {
    position: sticky;
    bottom: 0;
    z-index: 2;
    display: flex;
    justify-content: flex-end;
    padding: 8px 12px;
    border-top: 1px solid rgb(var(--yb-border));
    background: rgb(var(--yb-surface));
  }
  .fee-inspector .fee-document > .oa-panel { min-width: 0; }
  .fee-approval-page .fee-document > .oa-panel { padding: 10px 12px; }
  .fee-approval-page .fee-document :is(.oa-kv,.oa-steps) { margin-block: 6px; }
  .fee-approval-page .fee-document .oa-kv > div { padding-block: 4px; }
  .fee-inspector .fee-document .oa-form { gap: 8px; }
}

@media (min-width: 1920px) {
  .fee-approval-page .fee-queue-layout.has-fee-detail {
    grid-template-columns: minmax(0, 1fr) var(--fee-inspector-width, 420px);
    gap: 12px;
  }
  .fee-approval-page .fee-inspector { position: static; inset: auto; z-index: auto; display: contents; }
  .fee-approval-page .fee-inspector-body { border: 1px solid rgb(var(--yb-border)); border-radius: 8px; }
}

@media (min-width: 2300px) {
  .fee-approval-page { --fee-inspector-width: 480px; }
}
</style>
