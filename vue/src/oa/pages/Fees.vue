<script setup lang="ts">
import { ArrowLeft, Check, CircleAlert, FileText, Wallet } from 'lucide-vue-next'
import OAEmpty from '../components/OAEmpty.vue'
import { computed, onBeforeUnmount, nextTick, onMounted, ref, watch, watchEffect } from 'vue'
import { onBeforeRouteLeave, onBeforeRouteUpdate, RouterLink, useRoute, useRouter } from 'vue-router'
import FeePersonSelect from '../components/FeePersonSelect.vue'
import FeeTemplateRules from '../components/FeeTemplateRules.vue'
import { searchFeePeople, approverLabel, type FeePerson as Approver } from '../feePeople'
import { feeTabs } from '../routes'
import DesktopSlot from '@/oa/components/DesktopSlot.vue'
import PageHead from '../components/PageHead.vue'
import { api, dateInChina, errorText, money, send } from '../api'
import { useOAAccess } from '../access'
import { expenseActivityActions, useExpenseActivity } from '../expenseActivity'
import { oaReturnLabel } from '../pageTitle'
import { feeApplicant, feeDateTime, feeStatus as status } from '../feeDisplay'
import { NSelect } from 'naive-ui'
import BaseSelect from '@/components/base/BaseSelect.vue'
import OaDateField from '../components/OaDateField.vue'
import FeeExpenseLines from '../components/FeeExpenseLines.vue'
import FeeUpload from '../components/FeeUpload.vue'
import FeeAccountPicker from '../components/FeeAccountPicker.vue'
import http from '@/services/http'
import { newExpenseLine, parseExpenseCents, uppercaseAmount, writableLine, type ApprovalStep, type ExpenseInput, type ExpenseLine } from '../feeApplication'
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

type ExpenseField = { name: string; type: 'text' | 'number' | 'date' | 'select'; required: boolean; options?: string[] }
type FeeApprovalSpec = { mode: 'AND' | 'OR'; approvers: string[] }
type FeeTemplate = { approval_nodes?: (FeeApprovalSpec & { name: string })[]; amount_rules?: (FeeApprovalSpec & { min_cents: number; max_cents?: number })[]; cc_after_approval?: number[]; cc_after_payment?: number[]; field_definitions?: ExpenseField[]; id: string; name: string; type: string; channels: string[]; fields: string[]; required: string[]; evidence: string; nodes: string[]; threshold_cents: number; extra: string; version: number }
type SourceReference = { kind: string; number: string; url?: string }
type DraftPayload = { request_id: string; template_id: string; amount_input: string; reason: string; channel: string; holder: string; account: string; bank: string; extra: Record<string, string>; evidence: Evidence[]; items?: ExpenseLine[]; item_amounts?: string[]; item_taxes?: string[]; source_reference?: SourceReference }
type RemoteDraft = { draft: DraftPayload | null; version: string }
type Evidence = { id: string; name: string }
type Decision = {
  action: string; comment?: string; actor_id?: number; at: string
  operator_id?: number; operator_name?: string; operator_no?: number
  operator_department?: string; node_name?: string; created_at?: string
  before_amount_cents?: number; after_amount_cents?: number; adjustment_reason?: string
  bank_reference?: string; receipt?: string; payment_date?: string
}
type FeeDoc = { approval_steps?: { name: string; mode: 'AND' | 'OR'; approver_ids: number[] }[]; pending_approver_ids?: number[]; id: string; applicant_id: number; template: FeeTemplate; amount_cents: number; reason: string; channel: string; holder: string; account: string; bank: string; extra: Record<string, string>; files: string[]; site: string; state: string; route: (number | string | { id?: number; name?: string; user_id?: number })[]; current: number; version: number; history?: Decision[]; payment?: { amount_cents: number; date: string; receipt: string; bank_reference?: string }
  applicant_name?: string; applicant_employee_no?: number; department_id?: number; department_name?: string; team_name?: string
  approved_amount_cents?: number; submitted_at?: string; approved_at?: string; route_nodes?: string[]; items?: ExpenseLine[]; source_reference?: SourceReference
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
watch(notice, (value, _previous, onCleanup) => {
  if (!value) return
  const timeout = window.setTimeout(() => { notice.value = '' }, 3000)
  onCleanup(() => window.clearTimeout(timeout))
})
const templates = ref<FeeTemplate[]>([])
function feeTypeLabel(value: string) {
  return value === 'payment' || value === '付款' ? '付款' : '报销'
}
const templateOptions = computed(() => (returnedTemplate.value ? [returnedTemplate.value] : templates.value)
  .map(item => ({ value: item.id, label: feeTypeLabel(item.type) + ' · ' + item.name })))
const sourceOptions = [
  { value: '', label: '无关联' }, { value: 'travel', label: '出差申请' }, { value: 'loan', label: '借款单' },
  { value: 'erp_purchase', label: 'ERP 采购单' }, { value: 'contract', label: '合同' }, { value: 'other', label: '其他' },
]
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
const grantTotal = ref(0)
const grantFilter = ref({ q: '', status: '', sort: 'id', order: 'asc', kind: '' })
const grantPageSize = 15
const templateSegment = ref<'templates' | 'grants'>(route.query.manage === 'publish' ? 'grants' : 'templates')
const hasPayHandler = ref(false)
const grantSelectionLoading = ref(false)
let grantSelectionRequest = 0
const grantSelection = ref<FinanceGrant>()

const grantsLoaded = ref(false)
const grantPage = ref(1)
const grantPages = computed(() => Math.max(1, Math.ceil(grantTotal.value / grantPageSize)))
const grantStatuses: Record<string, string> = { active: '在职', inactive: '停用或离职', deleted: '已删除', missing: '账号已不可用' }
const grantName = (item: Approver) => item.name ? approverLabel(item) : '账号 #' + item.id
const revealed = ref(false)
const approval = ref({ comment: '', reduced: false, amount: '', reason: '' })
const payment = ref({ date: dateInChina(), receipt: '', receiptName: '', bankReference: '' })
const draft = ref<FeeTemplate>()
const editingId = ref('')
const delegatePanel = ref<HTMLDialogElement>()
const financeGrantPanel = ref<HTMLDialogElement>()
function openGrantPanel(panel: HTMLDialogElement | undefined) {
  if (!panel?.open) panel?.showModal?.()
}
async function setTemplateSegment(segment: 'templates' | 'grants') {
  if (segment === templateSegment.value || !(await leaveDraft())) return
  draft.value = undefined
  draftEdited.value = false
  templateSegment.value = segment
  if (segment === 'grants' && !grantsLoaded.value) await run(loadGrants)
}
const templateFields = ref({ threshold: '' })
const knownApprovers = ref<Record<number, Approver>>({})
const templateNodeKeys = ref<number[]>([])
const templateError = ref('')
let templateNodeKey = 0
const grants = ref({ pay: false, report: false })
const delegate = ref({ level: 'draft' })
const grantTarget = ref<Approver>()
const delegateTarget = ref<Approver>()
const canSaveFinanceGrant = computed(() => {
  if (busy.value || grantSelectionLoading.value || !grantTarget.value) return false
  return grants.value.pay !== Boolean(grantSelection.value?.pay) || grants.value.report !== Boolean(grantSelection.value?.report)
})
const inactiveGrant = (target: Approver | undefined) => !!target && grantSelection.value?.id === target.id && grantSelection.value?.status !== 'active'
function useGrantTarget(target: Approver | undefined, kind: 'template' | 'finance', current?: FinanceGrant) {
  grantSelection.value = current
  if (kind === 'template') {
    delegateTarget.value = target
    delegate.value.level = current?.status === 'active' ? current.level : (current ? 'none' : 'draft')
  } else {
    grantTarget.value = target
    grants.value = current?.status === 'active' ? { pay: current.pay, report: current.report } : { pay: false, report: false }
  }
}
async function selectGrantTarget(target: Approver | undefined, kind: 'template' | 'finance') {
  const request = ++grantSelectionRequest
  grantSelectionLoading.value = false
  useGrantTarget(target, kind)
  if (!target) return
  grantSelectionLoading.value = true
  try {
    const data = await api<{ items: FinanceGrant[] }>('/oa-api/finance/grants?id=' + encodeURIComponent(target.id))
    if (request === grantSelectionRequest) useGrantTarget(target, kind, data.items?.[0])
  } catch (cause) {
    if (request === grantSelectionRequest) {
      useGrantTarget(undefined, kind)
      error.value = errorText(cause)
    }
  } finally {
    if (request === grantSelectionRequest) grantSelectionLoading.value = false
  }
}
function editGrant(item: FinanceGrant, kind: 'template' | 'finance') {
  grantSelectionRequest++
  grantSelectionLoading.value = false
  useGrantTarget(item, kind, item)
  openGrantPanel(kind === 'template' ? delegatePanel.value : financeGrantPanel.value)
}
async function startGrant(kind: 'template' | 'finance') {
  void selectGrantTarget(undefined, kind)
  await nextTick()
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
const receiptUploading = ref(false)
const receiptProgress = ref(0)
const channels: Record<string, string> = { bank: '对公银行转账', person: '对私银行卡', wallet: '对私收款平台' }
const decisions: Record<string, string> = { approve: '同意', return: '退回修改', reject: '拒绝' }
const active = computed(() => returnedTemplate.value || templates.value.find(item => item.id === form.value.template_id))
const applicationFields = computed<ExpenseField[]>(() => active.value?.field_definitions?.length
  ? active.value.field_definitions
  : (active.value?.fields || []).map(name => ({ name, type: 'text', required: Boolean(active.value?.required?.includes(name)) })))

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
function safeSourceLink(value?: string) {
  if (!value) return false
  try {
    const url = new URL(value)
    return url.protocol === 'https:' && !!url.hostname && !url.username && !url.password
  } catch { return false }
}
const sourceKinds: Record<string, string> = { travel: '出差申请', loan: '借款单', erp_purchase: 'ERP 采购单', contract: '合同', other: '其他' }
const stageGuide = computed(() => ({
  waiting: '审批按模板顺序流转。',
  returned: '申请人可根据退回意见修改并重新提交。',
  rejected: '本次审批已结束。',
  approved: '审批已完成，等待财务登记实付。',
  paid: '付款已登记，可查看实付信息和回单。',
  withdrawn: '申请人已撤回，可修改并重新提交。',
  voided: '财务已作废，单据不能付款。',
})[detail.value?.state || ''] || '')
const mayDecide = computed(() => {
  const doc = detail.value
  if (doc?.state !== 'waiting' || own.value) return false
  const pending = doc.approval_steps ? doc.pending_approver_ids : [doc.route?.[doc.current]]
  return pending?.some(id => String(id) === String(currentProfile.value?.id))
})
const major = (cents?: number) => cents == null ? '—' : money(cents / 100)
const lastReturn = computed(() => [...(detail.value?.history || [])].reverse().find(entry => entry.action === 'return'))
const showApprovalForm = computed(() => (queue.value === 'todo' || props.mode === 'document') && can('finance.approve') && mayDecide.value)
const showPaymentForm = computed(() => (queue.value === 'pay' || props.mode === 'document') && !own.value && canPay.value && detail.value?.state === 'approved')
const payable = (doc: FeeDoc) => doc.approved_amount_cents ?? doc.amount_cents
const mayVoid = computed(() => detail.value?.state === 'approved' && (isAdmin.value || (canPay.value && !own.value)))
const mayWithdraw = computed(() => {
  const doc = detail.value
  return own.value && doc?.state === 'waiting' && doc.current === 0 && (!doc.approval_steps || doc.pending_approver_ids?.length === doc.approval_steps[0]?.approver_ids.length)
})
const paymentReady = computed(() => payment.value.receipt && payment.value.bankReference.trim() && !dateProblem())
const leaveForm = async () => !edited.value || Boolean(await ask({
  title: '放弃费用申请？', message: '尚未提交的内容会丢失。', confirmText: '放弃并离开', danger: true,
}))
const draftVersion = ref('')
const draftEndpoint = computed(() => '/oa-api/finance/draft?site=' + form.value.site)
const expenseLines = ref<ExpenseInput[]>([newExpenseLine()])
const sourceReference = ref<SourceReference>({ kind: '', number: '', url: '' })
const approvalSteps = ref<ApprovalStep[]>([])
const approvalIssue = ref('')
const approvalLoading = ref(false)
const evidenceUploading = ref(false)
const evidenceProgress = ref(0)
const isReimbursement = computed(() => !!active.value && feeTypeLabel(active.value.type) === '报销')
const totalCents = computed(() => isReimbursement.value
  ? expenseLines.value.reduce((sum, line) => sum + parseExpenseCents(line.amount), 0)
  : parseExpenseCents(form.value.amount))
const totalTax = computed(() => isReimbursement.value
  ? expenseLines.value.reduce((sum, line) => sum + parseExpenseCents(line.tax), 0) : 0)

watch([() => active.value?.id, totalCents, returned], ([template_id, amount_cents, document_id], _, cleanup) => {
  approvalSteps.value = []
  approvalIssue.value = ''
  approvalLoading.value = Boolean(template_id && amount_cents > 0 && amount_cents <= 100000000000)
  if (!template_id || !approvalLoading.value) return
  let cancelled = false
  const timer = window.setTimeout(async () => {
    try {
      const result = await send<{ steps: ApprovalStep[] }>('/oa-api/finance/documents/preview-route', { template_id, amount_cents, document_id })
      if (!cancelled) approvalSteps.value = result.steps ?? []
    } catch (cause) {
      if (!cancelled) approvalIssue.value = errorText(cause)
    } finally {
      if (!cancelled) approvalLoading.value = false
    }
  }, 275)
  cleanup(() => { cancelled = true; window.clearTimeout(timer) })
})

function selectCommonAccount(selection: { holder: string; bank: string; account: string }) {
  Object.assign(form.value, selection)
  for (const field of ['holder', 'account', 'bank']) delete fieldErrors.value[field]
  edited.value = true
}

function draftFormPayload() {
  const { amount, files: evidence, ...values } = form.value
  return {
    ...values, amount_input: amount, evidence,
    items: expenseLines.value.map(writableLine),
    item_amounts: expenseLines.value.map(line => line.amount),
    item_taxes: expenseLines.value.map(line => line.tax),
    source_reference: sourceReference.value.kind ? sourceReference.value : undefined,
  }
}
async function saveApplicationDraft() {
  const payload = draftFormPayload()
  const snapshot = JSON.stringify(payload)
  const result = await send<RemoteDraft>(draftEndpoint.value, { ...payload, version: draftVersion.value }, 'PUT')
  edited.value = snapshot !== JSON.stringify(draftFormPayload())
  draftVersion.value = result.version
  form.value.request_id = result.version
  draftRestored.value = false
  missingTemplateDraft.value = false
  notice.value = edited.value ? '草稿已保存。保存期间的修改仍未保存。' : '草稿已保存，可在其他设备继续填写。'
}
async function discardApplicationDraft() {
  if (!draftVersion.value || !await ask({
    title: '删除费用草稿？', message: '删除后无法恢复。', confirmText: '删除草稿', danger: true,
  })) return
  await api(draftEndpoint.value + '&version=' + encodeURIComponent(draftVersion.value), { method: 'DELETE' })
  form.value = { request_id: '', template_id: '', amount: '', reason: '', channel: '', holder: '', account: '', bank: '', extra: {}, files: [], site: form.value.site }
  expenseLines.value = [newExpenseLine()]
  sourceReference.value = { kind: '', number: '', url: '' }
  draftVersion.value = ''
  draftRestored.value = false
  missingTemplateDraft.value = false
  fieldErrors.value = {}
  touched.clear()
  edited.value = false
  if (templates.value.length === 1) {
    form.value.template_id = templates.value[0]!.id
    selectTemplate()
  }
  notice.value = '草稿已删除。'
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

async function copyFeeValue(value: string, label: string) {
  try {
    await navigator.clipboard.writeText(value)
    notice.value = '已复制' + label
  } catch {
    error.value = '无法复制，请手动选中' + label
  }
}
function node(step: FeeDoc['route'][number]) {
  if (typeof step !== 'object') return step ? '账号 #' + step : '审批节点（未留存名称）'
  const id = step?.user_id || step?.id
  return step?.name || (id ? '账号 #' + id : '审批节点（未留存名称）')
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
function changeTemplateApproverType(type: string, index?: number) {
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
    for (let offset = 0; offset < ids.length; offset += 16) {
      const { selected } = await searchFeePeople('', ids.slice(offset, offset + 16))
      for (const person of selected ?? []) knownApprovers.value[person.id] = person
    }
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
  if (props.mode !== 'apply' || form.value.template_id) { loadedTemplates.value = true; return }
  const stored = await api<RemoteDraft>(draftEndpoint.value)
  loadedTemplates.value = true
  draftVersion.value = stored.version
  const localDraft = stored.draft
  const template = templates.value.find(item => item.id === localDraft?.template_id)
  missingTemplateDraft.value = Boolean(localDraft?.template_id && !template)
  form.value.template_id = template?.id || (templates.value.length === 1 ? templates.value[0]!.id : '')
  selectTemplate()
  if (localDraft) {
    form.value.request_id = localDraft.request_id
    form.value.amount = localDraft.amount_input
    form.value.reason = localDraft.reason
    if (template?.channels.includes(localDraft.channel)) {
      Object.assign(form.value, { channel: localDraft.channel, holder: localDraft.holder, account: localDraft.account, bank: localDraft.bank })
    }
    form.value.extra = Object.fromEntries(Object.entries(localDraft.extra || {}).filter(([key]) => template?.fields?.includes(key)))
    form.value.files = localDraft.evidence ?? []
    expenseLines.value = localDraft.items?.length ? localDraft.items.map((line, index) => ({
      ...newExpenseLine(line), amount: localDraft.item_amounts?.[index] ?? '', tax: localDraft.item_taxes?.[index] ?? '',
    })) : [newExpenseLine()]
    sourceReference.value = localDraft.source_reference ?? { kind: '', number: '', url: '' }
    draftRestored.value = true
  }
}
let loadedQueueQuery = ''
function documentQueueQuery(targetPage: number) {
  const params = new URLSearchParams({ queue: queue.value, page: String(targetPage), size: String(pageSize) })
  for (const key of ['q', 'state', 'template', 'department', 'team', 'person', 'site', 'channel', 'start', 'end', 'basis', 'min_amount_cents', 'max_amount_cents', 'expense_type', 'sort', 'order']) {
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
const templateTypeOptions = ['报销', '付款'].map(value => ({ label: value, value }))
const queryExpenseTypes = [{ label: '全部', value: '' }, { label: '报销', value: 'reimbursement' }, { label: '付款', value: 'payment' }]
const queryStates = [{ label: '全部状态', value: '' }, ...['waiting', 'returned', 'approved', 'paid', 'withdrawn', 'rejected', 'voided'].map(value => ({ label: status(value), value }))]
const queueSortOptions = [{ label: '提交日期', value: '' }, { label: '金额', value: 'amount' }]
const queueOrderOptions = [{ label: '降序', value: '' }, { label: '升序', value: 'asc' }]
const legacyReviewerOptions = [{ label: '指定人员', value: 'fixed' }, { label: '申请人直属负责人', value: 'manager' }]
const grantKinds = [{ label: '全部', value: '' }, { label: '模板维护', value: 'template' }, { label: '财务经办', value: 'finance' }, { label: '付款登记', value: 'pay' }, { label: '费用报表', value: 'report' }]
const grantStatusOptions = [{ label: '全部', value: '' }, ...Object.entries(grantStatuses).map(([value, label]) => ({ label, value }))]
const grantSortOptions = Object.entries({ id: '账号编号', name: '姓名', employee_no: '工号', status: '账号状态', department: '部门' }).map(([value, label]) => ({ label, value }))
const grantOrderOptions = [{ label: '升序', value: 'asc' }, { label: '降序', value: 'desc' }]
const permissionOptions = Object.entries({ ...delegateLevels, none: '无模板维护权' }).map(([value, label]) => ({ label, value }))
const queueFilterKeys = ['q', 'state', 'start', 'end', 'expense_type', 'template', 'sort', 'order', 'min_amount_cents', 'max_amount_cents'] as const
function currentQueueFilters() {
  const entries = queueFilterKeys.map(key => [key, typeof route.query[key] === 'string' ? route.query[key] : ''])
  return Object.fromEntries(entries) as Record<typeof queueFilterKeys[number], string>
}
const queueFilters = ref(currentQueueFilters())
const queueRangeError = ref('')
watch(() => route.fullPath, () => { queueFilters.value = currentQueueFilters(); queueRangeError.value = '' })
function searchQueue() {
  const filters = queueFilters.value
  queueRangeError.value = ''
  if (Boolean(filters.start) !== Boolean(filters.end) || (filters.start && filters.start > filters.end)) {
    queueRangeError.value = '请选择有效的起止日期'
    return
  }
  if (filters.min_amount_cents && filters.max_amount_cents && Number(filters.min_amount_cents) > Number(filters.max_amount_cents)) {
    queueRangeError.value = '金额下限不能高于上限'
    return
  }
  const target = queueLocation(undefined, queue.value, 1)
  const query = { ...target.query, ...Object.fromEntries(queueFilterKeys.map(key => [key, filters[key].trim() || undefined])) }
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
async function upload(file: File, onProgress: (fraction: number) => void) {
  if (file.size > 10 * 1024 * 1024) throw new Error('文件超过 10 MB')
  const data = new FormData()
  data.set('file', file)
  const { data: result } = await http.post<Evidence | { data: Evidence }>('/oa-api/finance/files', data, {
    headers: { 'Content-Type': undefined },
    onUploadProgress(event) { onProgress(event.progress ?? 0) },
  })
  return 'data' in result ? result.data : result
}
function uploadEvidence(selected: File[]) {
  if (evidenceUploading.value || busy.value) return
  if (form.value.files.length + selected.length > 15) {
    fieldErrors.value.files = '每张申请最多上传 15 份凭证'
    return
  }
  delete fieldErrors.value.files
  evidenceUploading.value = true
  evidenceProgress.value = 0
  void run(async () => {
    try {
      for (const [index, file] of selected.entries()) {
        const evidence = await upload(file, fraction => {
          evidenceProgress.value = Math.round((index + fraction) * 100 / selected.length)
        })
        form.value.files.push(evidence)
        edited.value = true
      }
    } catch (cause) {
      fieldErrors.value.files = errorText(cause)
    } finally {
      evidenceUploading.value = false
    }
  })
}
function uploadReceipt(files: File[]) {
  const file = files[0]
  if (!file || busy.value || receiptUploading.value) return
  receiptError.value = ''
  receiptUploading.value = true
  receiptProgress.value = 0
  void run(async () => {
    try {
      const uploaded = await upload(file, fraction => { receiptProgress.value = Math.round(fraction * 100) })
      payment.value.receipt = uploaded.id
      payment.value.receiptName = uploaded.name || file.name
    } catch (cause) {
      receiptError.value = file.name + ' 上传失败。' + errorText(cause)
    } finally {
      receiptUploading.value = false
    }
  })
}
async function submit() {
  if (!active.value) return
  fieldErrors.value = {}
  let amountCents = 0
  const lines = isReimbursement.value ? expenseLines.value.map(writableLine) : undefined
  if (lines) {
    amountCents = totalCents.value
    const invalid = lines.findIndex((line, index) => !line.category || !line.occurred_on || line.occurred_on > dateInChina() ||
      !line.amount_cents || line.tax_cents > line.amount_cents ||
      (!!expenseLines.value[index]!.tax.trim() && !/^(?:\d{1,10})(?:\.\d{1,2})?$/.test(expenseLines.value[index]!.tax.trim())))
    if (invalid >= 0) fieldErrors.value.items = '请核对第 ' + (invalid + 1) + ' 条明细的科目、日期、金额和税额'
    else if (amountCents > 100000000000) fieldErrors.value.items = '申请金额最多 10 亿元'
  } else {
    const amountError = amountProblem()
    if (amountError) fieldErrors.value.amount = amountError
    else amountCents = cents(form.value.amount)
  }
  if (!form.value.reason.trim()) fieldErrors.value.reason = '请输入费用用途'
  if (!form.value.holder.trim()) fieldErrors.value.holder = '请输入收款账户名称'
  if (accountRequired.value && !form.value.account.trim()) fieldErrors.value.account = '请输入收款账号'
  if (!form.value.bank.trim()) fieldErrors.value.bank = '请输入开户银行或收款平台'
  for (const [index, field] of applicationFields.value.entries()) {
    if (field.required && !form.value.extra[field.name]?.trim()) fieldErrors.value['extra-' + index] = '请填写' + field.name
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
    bank: fields.bank.trim(), amount_cents: amountCents, files: evidence.map(file => file.id), items: lines,
    source_reference: sourceReference.value.kind ? sourceReference.value : undefined,
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
    draftVersion.value = ''
    await router.push({ path: '/fees/mine', query: result?.id ? { id: result.id, submitted: '1' } : {} })
  }
}
function editReturned() {
  if (!detail.value || !own.value || !['returned', 'withdrawn'].includes(detail.value.state)) return
  const document = detail.value
  void run(async () => {
    expenseLines.value = document.items?.length ? document.items.map(newExpenseLine) : [{ ...newExpenseLine(), amount: (document.amount_cents / 100).toFixed(2) }]
    sourceReference.value = document.source_reference ?? { kind: '', number: '', url: '' }
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
      reasonChoices: reject ? undefined : ['请补充合规发票', '请核对收款信息'],
    })
    if (!result?.reason.trim()) return
    comment = result.reason.trim()
  }
  detail.value = await send<FeeDoc>(`/oa-api/finance/documents/${encodeURIComponent(id)}/decision`, { decision, comment, current: doc.current, version: doc.version, ...adjustment })
  expenseActivity.invalidate()
  approvalsChanged.value = true
  notice.value = ({
    waiting: detail.value.current === doc.current ? '已同意，等待同节点其他审批人会签' : '已同意，申请已转交下一位审批人',
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
  draft.value = template ? JSON.parse(JSON.stringify(template)) as FeeTemplate : { id: crypto.randomUUID(), name: '', type: '', channels: [], fields: [], required: [], evidence: '', nodes: [], threshold_cents: 0, extra: '', version: 0 }
  editingId.value = template?.id || ''
  if (draft.value.type === 'reimbursement') draft.value.type = '报销'
  if (draft.value.type === 'payment') draft.value.type = '付款'
  draft.value.channels = [...(draft.value.channels || [])]
  draft.value.required = [...(draft.value.required || [])]
  draft.value.nodes = [...(draft.value.nodes || [])]
  if (!draft.value.approval_nodes && draft.value.amount_rules?.length) {
    draft.value.approval_nodes = draft.value.nodes.map(reference => ({ name: '', mode: 'AND', approvers: [reference] }))
  }
  templateNodeKeys.value = draft.value.nodes.map(() => templateNodeKey++)
  draft.value.field_definitions ??= (draft.value.fields ?? []).map(name => ({ name, type: 'text', required: draft.value?.required.includes(name) ?? false }))
  templateFields.value = { threshold: draft.value.threshold_cents ? (draft.value.threshold_cents / 100).toFixed(2) : '' }
  void resolveTemplateApprovers([
    ...draft.value.nodes, draft.value.extra ?? '',
    ...(draft.value.approval_nodes ?? []).flatMap(item => item.approvers),
    ...(draft.value.amount_rules ?? []).flatMap(item => item.approvers),
    ...(draft.value.cc_after_approval ?? []).map(id => 'fixed:' + id),
    ...(draft.value.cc_after_payment ?? []).map(id => 'fixed:' + id),
  ])
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
  const definitions = draft.value.field_definitions ?? []
  const fields = definitions.map(field => field.name)
  const structured = draft.value.approval_nodes
  const rules = draft.value.amount_rules ?? []
  const incompleteGroup = (refs: string[]) => !refs.length || new Set(refs).size !== refs.length || refs.some(value => value !== 'manager' && fixedApproverId(value) === undefined)

  const threshold = templateFields.value.threshold.trim()
  const amountParts = /^(\d+)(?:\.(\d{1,2}))?$/.exec(threshold || '0')
  const thresholdCents = amountParts ? Number(amountParts[1]) * 100 + Number((amountParts[2] || '').padEnd(2, '0')) : NaN
  const byteLength = (value: string) => new TextEncoder().encode(value).length
  const incompleteNode = nodes.findIndex(value => value !== 'manager' && fixedApproverId(value) === undefined)
  if (!name || byteLength(name) > 120) templateError.value = '请填写模板名称（最多 120 字节）'
  else if (type !== '报销' && type !== '付款') templateError.value = '请选择费用类型'
  else if (!evidence || byteLength(evidence) > 240) templateError.value = '请填写凭证要求（最多 240 字节）'
  else if (!selectedChannels.length || selectedChannels.some(channel => !Object.keys(channels).includes(channel))) templateError.value = '请选择至少一种有效支付方式'
  else if (fields.length > 30 || new Set(fields).size !== fields.length || definitions.some(field => !field.name || byteLength(field.name) > 120 || (field.type === 'select' && !field.options?.length))) templateError.value = '请填写不同的字段名称，并为单选字段设置选项'
  else if (!(structured?.length ?? nodes.length) || (structured?.length ?? nodes.length) > 12) templateError.value = '请添加 1 至 12 个审批节点'
  else if (structured?.some(node => incompleteGroup(node.approvers))) templateError.value = '请为每个审批节点选择不同的审批人'
  else if (rules.some((rule, index) => {
    const previous = rules[index - 1]
    return incompleteGroup(rule.approvers) || !Number.isSafeInteger(rule.min_cents) || rule.min_cents < 0 || rule.min_cents > 100000000000 ||
      (rule.max_cents !== undefined && (!Number.isSafeInteger(rule.max_cents) || rule.max_cents < rule.min_cents || rule.max_cents > 100000000000)) ||
      (previous !== undefined && (previous.max_cents === undefined || previous.max_cents >= rule.min_cents))
  })) templateError.value = '请核对金额规则的审批人及金额区间，区间须升序且不重叠'
  else if (!structured && incompleteNode >= 0) templateError.value = '请为第 ' + (incompleteNode + 1) + ' 步选择审批人'
  else if (!structured && !Number.isSafeInteger(thresholdCents)) templateError.value = '请输入有效的加签金额，最多两位小数'
  else if (!structured && thresholdCents > 0 && extra !== 'manager' && fixedApproverId(extra) === undefined) templateError.value = '请选择加签审批人'
  if (templateError.value) return
  const body = {
    ...draft.value, name, evidence, fields, required: definitions.filter(field => field.required).map(field => field.name),
    nodes: structured ? structured.map(node => node.approvers[0]!) : nodes,
    threshold_cents: structured ? 0 : thresholdCents, extra: !structured && thresholdCents > 0 ? extra : '', publish,
  }
  await send('/oa-api/finance/templates/' + encodeURIComponent(draft.value.id), body, 'PUT')
  draftEdited.value = false
  await loadTemplates()
  draft.value = undefined
  notice.value = publish ? '模板已发布，员工可发起申请' : '草稿已保存'
}
async function loadGrants(targetPage = grantPage.value) {
  grantsLoaded.value = false
  const query = new URLSearchParams({ page: String(targetPage), page_size: String(grantPageSize), ...grantFilter.value })
  const data = await api<{ items: FinanceGrant[]; total?: number; has_active_payer?: boolean }>('/oa-api/finance/grants?' + query)
  grantedPeople.value = data.items || []
  grantTotal.value = data.total ?? grantedPeople.value.length
  hasPayHandler.value = data.has_active_payer ?? grantedPeople.value.some(person => person.status === 'active' && person.pay)
  grantPage.value = Math.min(targetPage, grantPages.value)
  if (grantPage.value !== targetPage) {
    await loadGrants(grantPage.value)
    return
  }
  grantsLoaded.value = true
}
async function saveFinanceAccess() {
  const { pay, report } = grants.value
  if (!grantTarget.value) throw new Error('请选择经办人')
  const { id } = grantTarget.value
  const personName = grantName(grantTarget.value)
  financeGrantPanel.value?.close()
  if (!await ask({ title: '保存财务授权？', confirmText: '保存授权', facts: [
    ['人员', grantName(grantTarget.value)], ['付款登记', pay ? '允许' : '不允许'], ['费用报表', report ? '允许' : '不允许'],
  ] })) {
    openGrantPanel(financeGrantPanel.value)
    return
  }
  await send('/oa-api/finance/actions/' + encodeURIComponent(id), { pay, report }, 'PUT')
  grantTarget.value = undefined
  financeGrantPanel.value?.close()
  await loadGrants()
  notice.value = pay || report ? personName + '的财务授权已保存' : personName + '的财务授权已撤销'
}
async function saveDelegate() {
  if (!delegateTarget.value) throw new Error('请选择受托人')
  const { id } = delegateTarget.value
  delegatePanel.value?.close()
  if (!await ask({ title: '保存模板委托？', confirmText: '保存委托', facts: [
    ['人员', grantName(delegateTarget.value)], ['权限', delegateLevels[delegate.value.level]],
  ] })) {
    openGrantPanel(delegatePanel.value)
    return
  }
  await send('/oa-api/finance/delegates/' + encodeURIComponent(id), { level: delegate.value.level }, 'PUT')
  delegateTarget.value = undefined
  delegatePanel.value?.close()
  await loadGrants()
  notice.value = delegate.value.level === 'none' ? '模板维护委托已撤销' : '模板维护委托已保存'
}

onMounted(() => void run(async () => {
  if (!allowed.value) return
  if (props.mode === 'approval' && canUseExpenseActivity.value && expenseActivity.approveCount === null && expenseActivity.payCount === null) void expenseActivity.refresh()
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
  if (props.mode === 'templates' && isAdmin.value) await loadGrants()
  if (props.mode === 'templates' && isAdmin.value && route.query.manage === 'publish') await startGrant('template')
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
    <RouterLink v-if="mode === 'mine'" class="oa-button" to="/fees/accounts">常用收款账户</RouterLink>
    <RouterLink v-if="mode !== 'apply' && can('finance.apply')" class="oa-button oa-button-primary" :to="applicationLink">＋ 发起费用申请</RouterLink>
  </PageHead>
  <DesktopSlot target="tabs">
  <nav class="fee-tabs" aria-label="费用中心">
    <RouterLink v-if="mode === 'apply'" :to="route.fullPath || '/fees/apply'" aria-current="page">发起报销 / 付款</RouterLink>
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
  <p v-if="notice" class="fee-banner fee-banner-success fee-toast" role="status"><Check :size="17" aria-hidden="true" /> {{ notice }}</p>
  <p v-if="busy && !showQueue" class="oa-muted" role="status">{{ detail ? '正在办理单据…' : '正在处理…' }}</p>

  <div v-if="mode === 'apply' || returned" class="oa-split fee-apply" :class="{ 'fee-apply-empty': !active }">
    <section class="oa-panel">
      <div v-if="active || templates.length" class="fee-section-heading"><FileText :size="20" aria-hidden="true" /><h2>{{ returned ? '修改退回单据' : '填写申请' }}</h2></div>
      <p v-if="returned && lastReturn?.comment" class="fee-banner">退回意见　{{ lastReturn.comment }}</p>
      <p v-if="draftRestored" class="fee-banner" role="status">已恢复申请草稿。</p>
      <p v-if="missingTemplateDraft" class="fee-banner" role="status">草稿使用的模板当前不可用。</p>
      <div v-if="draftVersion && !returned" class="oa-actions"><button type="button" :disabled="busy" @click="run(discardApplicationDraft)">删除已保存草稿</button></div>
      <OAEmpty v-if="loadedTemplates && !templates.length && !returnedTemplate" kind="files" title="当前暂无可用费用模板" :description="templateGuide">
        <RouterLink v-if="canManageTemplates" class="oa-button oa-button-primary" :to="{ path: '/fees/templates', query: isAdmin && !canEditTemplate ? { manage: 'publish' } : {} }">前往模板与授权</RouterLink>
      </OAEmpty>
      <form v-if="active || templates.length" ref="applicationForm" class="oa-form fee-required" novalidate @input="edited = true" @change="edited = true" @submit.prevent="!evidenceUploading && !busy && run(submit)">
        <div class="oa-form-row">
          <BaseSelect :model-value="form.template_id" label="申请模板 *" placeholder="选择报销或付款模板" :options="templateOptions" :disabled="!!returned" @update:model-value="form.template_id = String($event); selectTemplate(); edited = true" />
          <label v-if="active && !isReimbursement"><span>金额（元）<i aria-hidden="true">*</i></span><input v-model="form.amount" inputmode="decimal" placeholder="例如 120.50" required v-bind="fieldAttrs('amount')" @blur="validateField('amount', amountProblem, true)" @input="validateField('amount', amountProblem)" /><small v-if="fieldErrors.amount" id="fee-error-amount" class="fee-error">{{ fieldErrors.amount }}</small></label>
        </div>
        <template v-if="active">
        <label><span>用途与说明<i aria-hidden="true">*</i></span><textarea v-model="form.reason" required maxlength="2000" placeholder="说明本笔支出的用途" v-bind="fieldAttrs('reason')" @blur="validateField('reason', () => requiredProblem(form.reason, '请输入费用用途'), true)" @input="validateField('reason', () => requiredProblem(form.reason, '请输入费用用途'))" /><small v-if="fieldErrors.reason" id="fee-error-reason" class="fee-error">{{ fieldErrors.reason }}</small></label>
        <fieldset v-if="applicationFields.length" class="oa-fieldset oa-form-row">
          <legend>模板字段</legend>
          <template v-for="(field, index) in applicationFields" :key="field.name">
            <BaseSelect v-if="field.type === 'select'" :model-value="form.extra[field.name]" :label="field.name + (field.required ? ' *' : '')" :options="(field.options || []).map(value => ({ value, label: value }))" :placeholder="'选择' + field.name" :clearable="!field.required" :error="fieldErrors['extra-' + index]" @update:model-value="form.extra[field.name] = String($event); edited = true; delete fieldErrors['extra-' + index]" />
            <OaDateField v-else-if="field.type === 'date'" :model-value="form.extra[field.name] || ''" :label="field.name" :required="field.required" :error="fieldErrors['extra-' + index]" @update:model-value="form.extra[field.name] = $event; edited = true; delete fieldErrors['extra-' + index]" />
            <label v-else>
              <span>{{ field.name }}<i v-if="field.required" aria-hidden="true">*</i></span>
              <input v-model="form.extra[field.name]" :inputmode="field.type === 'number' ? 'decimal' : undefined" maxlength="1000" :required="field.required" v-bind="fieldAttrs('extra-' + index)" />
              <small v-if="fieldErrors['extra-' + index]" :id="'fee-error-extra-' + index" class="fee-error">{{ fieldErrors['extra-' + index] }}</small>
            </label>
          </template>
        </fieldset>
        <FeeExpenseLines v-if="isReimbursement" v-model="expenseLines" :error="fieldErrors.items" @edit="edited = true" @update:model-value="edited = true" />
        <fieldset class="oa-fieldset oa-form-row">
          <legend>关联业务单</legend>
          <BaseSelect :model-value="sourceReference.kind" label="业务类型" :options="sourceOptions" @update:model-value="sourceReference.kind = String($event); edited = true" />
          <label v-if="sourceReference.kind"><span>业务单号<i aria-hidden="true">*</i></span><input v-model="sourceReference.number" maxlength="128" /></label>
          <label v-if="sourceReference.kind"><span>业务链接</span><input v-model="sourceReference.url" type="url" maxlength="512" placeholder="https://" /></label>
        </fieldset>
        <fieldset class="oa-fieldset oa-form-row">
          <legend>收款信息</legend>
          <div class="fee-account-picker"><FeeAccountPicker :type="active.type" @select="selectCommonAccount" /></div>
          <BaseSelect :model-value="form.channel" label="付款方式 *" :options="active.channels.map(value => ({ value, label: channels[value] || value }))" @update:model-value="form.channel = String($event); changeChannel(); edited = true" />
          <label><span>{{ form.channel === 'bank' ? '单位开户全称' : '收款人姓名' }}<i aria-hidden="true">*</i></span><input v-model="form.holder" required maxlength="240" placeholder="填写与收款账户一致的名称" v-bind="fieldAttrs('holder')" @blur="validateField('holder', () => requiredProblem(form.holder, '请输入收款账户名称'), true)" @input="validateField('holder', () => requiredProblem(form.holder, '请输入收款账户名称'))" /><small v-if="fieldErrors.holder" id="fee-error-holder" class="fee-error">{{ fieldErrors.holder }}</small></label>
          <label><span>{{ form.channel === 'wallet' ? '收款平台账号' : form.channel === 'bank' ? '对公银行账号' : '银行卡号' }}<i v-if="accountRequired" aria-hidden="true">*</i></span><input v-model="form.account" maxlength="128" autocomplete="off" :placeholder="accountRequired ? '填写实际收款账号' : '留空保留原收款账号'" :required="accountRequired" v-bind="fieldAttrs('account')" @blur="validateField('account', () => accountRequired ? requiredProblem(form.account, '请输入收款账号') : '', true)" @input="validateField('account', () => accountRequired ? requiredProblem(form.account, '请输入收款账号') : '')" /><small v-if="fieldErrors.account" id="fee-error-account" class="fee-error">{{ fieldErrors.account }}</small></label>
          <label><span>{{ form.channel === 'wallet' ? '收款平台' : '开户银行' }}<i aria-hidden="true">*</i></span><input v-model="form.bank" required maxlength="240" :placeholder="form.channel === 'wallet' ? '例如支付宝' : '填写收款账户的开户银行名称'" v-bind="fieldAttrs('bank')" @blur="validateField('bank', () => requiredProblem(form.bank, '请输入开户银行或收款平台'), true)" @input="validateField('bank', () => requiredProblem(form.bank, '请输入开户银行或收款平台'))" /><small v-if="fieldErrors.bank" id="fee-error-bank" class="fee-error">{{ fieldErrors.bank }}</small></label>
        </fieldset>
        <FeeUpload :files="form.files" :busy="evidenceUploading" :progress="evidenceProgress" :error="fieldErrors.files" :label="active?.evidence || '凭据附件'" @upload="uploadEvidence" @remove="id => { edited = true; form.files = form.files.filter(item => item.id !== id) }" />
        <div class="oa-actions fee-form-actions">
          <button v-if="!returned" type="button" :disabled="busy || evidenceUploading" @click="run(saveApplicationDraft)">保存草稿</button>
          <button class="oa-button-primary" :disabled="busy || evidenceUploading || !active">{{ returned ? '重新提交' : '提交申请' }}</button>
        </div>
        </template>
      </form>
    </section>
    <aside v-if="active" class="oa-panel fee-application-summary">
      <h2>费用汇总</h2>
      <strong class="fee-application-total">{{ major(totalCents) }}</strong>
      <p>人民币大写　{{ uppercaseAmount(totalCents) }}</p>
      <p v-if="isReimbursement" class="oa-muted">税额 {{ major(totalTax) }} · {{ expenseLines.length }} 条明细</p>
      <h2>审批路径</h2>
      <p v-if="approvalLoading" class="oa-muted" role="status">正在解析审批人…</p>
      <p v-else-if="approvalIssue" class="fee-error" role="alert">{{ approvalIssue }}</p>
      <p v-else-if="!approvalSteps.length" class="oa-muted">填写有效金额后查看审批人。</p>
      <ol v-else class="oa-steps fee-timeline">
        <li v-for="(step, index) in approvalSteps" :key="index">
          <strong>{{ step.name }}</strong>
          <span v-if="step.approvers.length > 1" class="oa-muted"> · {{ step.mode === 'AND' ? '会签' : '或签' }}</span>
          <span v-if="step.amount_min_cents !== undefined" class="fee-amount-trigger">
            {{ step.amount_max_cents !== undefined ? major(step.amount_min_cents) + ' 至 ' + major(step.amount_max_cents) + ' 触发' : '满 ' + major(step.amount_min_cents) + ' 触发' }}
          </span>
          <div v-for="person in step.approvers" :key="person.id" class="fee-preview-person">
            <img v-if="person.avatar_url" :src="person.avatar_url" alt="" referrerpolicy="no-referrer" />{{ person.name }}
          </div>
        </li>
        <li>审批通过后登记付款</li>
      </ol>
    </aside>
  </div>

  <template v-else-if="mode === 'mine' || mode === 'approval' || mode === 'document'">
    <div v-if="mode === 'approval' && (!detail || desktopApproval) && (can('finance.approve') || canPay)" class="oa-actions oa-person-tabs">
      <button v-if="can('finance.approve')" :aria-pressed="queue === 'todo'" :disabled="busy" @click="changeQueue('todo')">审批待办<span v-if="expenseActivity.approveCount !== null"> {{ expenseActivity.approveCount }}</span></button>
      <button v-if="canPay" :aria-pressed="queue === 'pay'" :disabled="busy" @click="changeQueue('pay')">待付款<span v-if="expenseActivity.payCount !== null"> {{ expenseActivity.payCount }}</span></button>
    </div>
    <div class="fee-queue-layout" :class="{ 'has-fee-detail': detail }">
    <section v-if="!detail || desktopApproval" class="oa-panel fee-queue">
      <form class="oa-actions oa-filter fee-queue-filter" @submit.prevent="searchQueue">
        <label>搜索单据<input v-model="queueFilters.q" maxlength="240" placeholder="单号、姓名或模板" /></label>
        <BaseSelect v-if="queue === 'mine'" class="fee-filter-select" label="状态" :model-value="queueFilters.state" :options="queryStates" @update:model-value="queueFilters.state = String($event)" />
        <BaseSelect class="fee-filter-select" label="费用类型" :model-value="queueFilters.expense_type" :options="queryExpenseTypes" @update:model-value="queueFilters.expense_type = String($event)" />
        <label>模板名称<input v-model="queueFilters.template" maxlength="120" placeholder="输入完整模板名称" /></label>
        <OaDateField v-model="queueFilters.start" label="提交起日" />
        <OaDateField v-model="queueFilters.end" label="提交止日" />
        <label>最低金额（分）<input v-model="queueFilters.min_amount_cents" inputmode="numeric" pattern="[0-9]+" /></label>
        <label>最高金额（分）<input v-model="queueFilters.max_amount_cents" inputmode="numeric" pattern="[0-9]+" /></label>
        <BaseSelect class="fee-filter-select" label="排序" :model-value="queueFilters.sort" :options="queueSortOptions" @update:model-value="queueFilters.sort = String($event)" />
        <BaseSelect class="fee-filter-select" label="方向" :model-value="queueFilters.order" :options="queueOrderOptions" @update:model-value="queueFilters.order = String($event)" />
        <button type="submit" :disabled="busy">查询</button>
        <span v-if="queueRangeError" class="oa-error" role="alert">{{ queueRangeError }}</span>
      </form>
      <p v-if="showQueue && busy && (desktopApproval ? !loadedDocs : true)" class="oa-muted fee-loading" role="status">正在读取{{ queueLabel }}…</p>
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
        <div class="oa-actions fee-document-number"><span>单号 <code>{{ detail.id }}</code></span><button type="button" @click="copyFeeValue(detail.id, '单号')">复制单号</button></div>
        <dl class="oa-kv fee-detail">
          <div><dt>申请人</dt><dd>{{ feeApplicant(detail) }}</dd></div>
          <div><dt>申请金额</dt><dd>{{ major(detail.amount_cents) }}</dd></div>
          <div v-if="detail.approved_amount_cents != null"><dt>核准金额</dt><dd>{{ major(detail.approved_amount_cents) }}</dd></div>
          <div v-if="detail.submitted_at"><dt>提交时间</dt><dd>{{ feeDateTime(detail.submitted_at) }}</dd></div>
          <div v-if="detail.approved_at"><dt>审批完成</dt><dd>{{ feeDateTime(detail.approved_at) }}</dd></div>
          <div><dt>支付渠道</dt><dd>{{ channels[detail.channel] || detail.channel }}</dd></div>
          <div v-if="detail.holder"><dt>收款对象</dt><dd>{{ detail.holder }} <button v-if="showPaymentForm" type="button" @click="copyFeeValue(detail.holder, '收款对象')">复制收款对象</button></dd></div><div v-if="detail.account"><dt>收款账号</dt><dd>{{ detail.account }} <button v-if="canPay && !own && detail.state === 'approved' && !revealed" :disabled="busy" @click="run(revealAccount)">查看完整账号</button><button v-if="showPaymentForm && revealed" type="button" @click="copyFeeValue(detail.account, '收款账号')">复制账号</button></dd></div><div v-if="detail.bank"><dt>银行 / 平台</dt><dd>{{ detail.bank }} <button v-if="showPaymentForm" type="button" @click="copyFeeValue(detail.bank, '银行或平台')">复制银行 / 平台</button></dd></div>
          <div v-if="detail.source_reference"><dt>关联业务单</dt><dd>
            {{ sourceKinds[detail.source_reference.kind] }} ·
            <a v-if="safeSourceLink(detail.source_reference.url)" :href="detail.source_reference.url" target="_blank" rel="noopener noreferrer">{{ detail.source_reference.number }} ↗</a>
            <span v-else>{{ detail.source_reference.number }}</span>
          </dd></div>
          <div v-if="detail.reason"><dt>用途与说明</dt><dd>{{ detail.reason }}</dd></div><div v-for="(value, key) in detail.extra" :key="key"><dt>{{ key }}</dt><dd>{{ value }}</dd></div>
        </dl>
        <section v-if="detail.items?.length" class="fee-document-lines">
          <h3>费用明细 · {{ detail.items.length }} 条</h3>
          <div class="oa-table-wrap">
            <table class="oa-table">
              <thead><tr><th>费用科目</th><th>发生日期</th><th>含税金额</th><th>税额</th><th>发票号码</th><th>明细说明</th></tr></thead>
              <tbody><tr v-for="(line, index) in detail.items" :key="index">
                <td>{{ line.category }}</td><td>{{ line.occurred_on }}</td><td>{{ major(line.amount_cents) }}</td>
                <td>{{ major(line.tax_cents) }}</td><td>{{ line.invoice_no || '—' }}</td><td>{{ line.description || '—' }}</td>
              </tr></tbody>
            </table>
          </div>
        </section>
        <FeeEvidence v-if="detail.files?.length" :ids="detail.files" label="单据凭证" />
        <p v-if="detail.state === 'returned' && lastReturn?.comment" class="fee-banner">退回意见　{{ lastReturn.comment }}</p>
      </section>
      <section class="oa-panel">
        <h2>审批与付款</h2>
        <p class="oa-muted">{{ stageGuide }}</p>
        <section v-if="detail.route?.length" aria-label="审批路线">
          <h3>审批路线</h3>
          <ol class="oa-steps fee-timeline">
            <li v-for="(item, index) in detail.route" :key="index" :class="{ 'fee-step-current': detail.state === 'waiting' && detail.current === index }">
              {{ detail.route_nodes?.[index] || node(item) }}
              <small v-if="(detail.approval_steps?.[index]?.approver_ids.length ?? 0) > 1">
                · {{ detail.approval_steps?.[index]?.mode === 'AND' ? '会签' : '或签' }}（{{ detail.approval_steps?.[index]?.approver_ids.length }} 人）
              </small>
              <small v-if="detail.current > index"> · 已通过</small>
              <small v-else-if="detail.state === 'waiting' && detail.current === index">
                · 当前待审批<span v-if="detail.pending_approver_ids?.length">（{{ detail.approval_steps?.[index]?.mode === 'OR' ? detail.pending_approver_ids.length + ' 人可办理' : '剩余 ' + detail.pending_approver_ids.length + ' 人待处理' }}）</span>
              </small>
              <small v-else-if="detail.state === 'waiting'"> · 待流转</small>
              <small v-else-if="index > detail.current"> · 未流转</small>
              <small v-else> · 本轮在此结束</small>
            </li>
          </ol>
        </section>
        <form v-if="showApprovalForm" id="fee-approval-form" class="oa-form" @submit.prevent="run(() => act('approve'))">
          <label>同意意见（选填）<textarea v-model.trim="approval.comment" maxlength="2000" /></label>
          <div class="oa-actions" aria-label="快捷审批意见">
            <button v-for="phrase in ['同意', '情况属实，凭证已核对', detail.template.type === '付款' || detail.template.type === 'payment' ? '同意付款' : '情况属实，同意报销']" :key="phrase" type="button" :disabled="busy" @click="approval.comment = phrase">{{ phrase }}</button>
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
          <OaDateField v-model="payment.date" label="实付日期" :max="dateInChina()" :error="fieldErrors.date" hint="填写实际付款日，最晚为今天。" required @update:model-value="validateField('date', dateProblem, true)" />
          <div class="fee-payment"><Wallet :size="19" aria-hidden="true" /><div><span>应付金额</span><strong>{{ major(payable(detail)) }}</strong></div><button type="button" @click="copyFeeValue((payable(detail) / 100).toFixed(2), '应付金额')">复制金额</button></div>
          <label>交易流水号 *<input v-model.trim="payment.bankReference" required maxlength="128" autocomplete="off" placeholder="银行或收款平台交易流水号" /></label>
          <p class="oa-muted">实际转账后填写实付日期、流水号并上传回单。</p>
          <FeeUpload single :files="payment.receipt ? [{ id: payment.receipt, name: payment.receiptName }] : []" :busy="receiptUploading" :disabled="busy" :progress="receiptProgress" :error="receiptError" label="付款回单" @upload="uploadReceipt" @remove="payment.receipt = ''; payment.receiptName = ''; receiptError = ''" />
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
    <nav v-if="isAdmin" class="oa-actions oa-person-tabs" aria-label="费用配置">
      <button type="button" :aria-pressed="templateSegment === 'templates'" :disabled="busy" @click="setTemplateSegment('templates')">费用模板</button>
      <button type="button" :aria-pressed="templateSegment === 'grants'" :disabled="busy" @click="setTemplateSegment('grants')">经办与委托</button>
    </nav>
    <section v-if="isAdmin && templateSegment === 'templates' && grantsLoaded && !hasPayHandler" class="oa-panel fee-setup-notice" aria-label="付款经办设置">
      <div><h2><CircleAlert :size="17" aria-hidden="true" /> 尚未指定付款经办人</h2><p class="oa-muted">选一位在职同事负责登记付款、上传回单。审批人在各模板中设置。</p></div>
      <button type="button" @click="startGrant('finance')">设置付款经办人</button>
    </section>
    <div v-if="templateSegment === 'templates' || !isAdmin" class="oa-split oa-sidebar-split">
      <section class="oa-panel" :class="{ 'fee-templates-empty': loadedTemplates && !templates.length && !draft }">
        <h2>费用模板</h2>
        <div class="oa-list"><button v-for="item in templates" :key="item.id" @click="beginTemplate(item)" :aria-current="selection === item.id" :disabled="!canEditTemplate">{{ feeTypeLabel(item.type) }} · {{ item.name }} · {{ item.version ? '线上 v' + item.version : '尚未发布' }}</button></div>
        <OAEmpty v-if="loadedTemplates && !templates.length" kind="files" title="当前暂无费用模板"
          :description="canEditTemplate ? '新建模板，配置支付方式、凭证和审批节点。发布后员工可以发起申请。' : templateGuide">
          <button v-if="canEditTemplate && !draft" class="oa-button-primary" @click="beginTemplate()">创建第一个费用模板</button>
          <button v-if="isAdmin && !canEditTemplate && !draft" @click="startGrant('template')">指定模板发布人</button>
        </OAEmpty>
        <button v-if="loadedTemplates && templates.length && canEditTemplate" :class="{ 'oa-button-primary': !draft }" @click="beginTemplate()">新建费用模板</button>
      </section>
      <section v-if="draft && canEditTemplate" class="oa-panel">
        <h2>{{ editingId ? '维护模板' : '新建费用模板' }}</h2>
        <p class="oa-muted">{{ level === 'publish' ? '保存草稿可继续编辑。发布后员工可用，历史单据沿用原模板版本。' : '你可以保存和修改草稿。草稿需由有发布权限的人员发布，员工才可使用。' }}</p>
        <form class="oa-form fee-required" novalidate @input="draftEdited = true" @change="draftEdited = true" @submit.prevent="run(() => saveTemplate(false))">
          <div class="oa-form-row">
            <label><span>名称<i aria-hidden="true">*</i></span><input v-model.trim="draft.name" placeholder="例如差旅报销" required /></label>
            <BaseSelect label="费用类型 *" :model-value="draft.type" :options="templateTypeOptions" @update:model-value="draft.type = String($event)" />
            <label><span>凭证要求<i aria-hidden="true">*</i></span><input v-model.trim="draft.evidence" placeholder="例如发票" required /></label>
          </div>
          <fieldset class="oa-fieldset"><legend>支付方式</legend><div class="oa-checks"><label v-for="(label, key) in channels" :key="key"><input v-model="draft.channels" type="checkbox" :value="key" />{{ label }}</label></div></fieldset>
          <FeeTemplateRules :draft="draft" :known="knownApprovers" :legacy-threshold="templateFields.threshold" @change="draftEdited = true" @person="person => knownApprovers[person.id] = person" />
          <fieldset v-if="!draft.approval_nodes" class="oa-fieldset oa-form"><legend>审批节点</legend>
            <p class="oa-muted">按从上到下的顺序审批，同一人只需审批一次。选择申请人直属负责人前，请确认人员资料已配置负责人。
              指定人员本人申请时，系统会尝试使用直属负责人或备用审批人。</p>
            <div v-for="(item, index) in draft.nodes" :key="templateNodeKeys[index]" class="oa-form">
              <div class="oa-actions">
                <BaseSelect :label="'第 ' + (index + 1) + ' 步审批人类型'" :model-value="approverType(item)" :options="legacyReviewerOptions" placeholder="选择审批人类型" @update:model-value="changeTemplateApproverType(String($event), index)" />
                <button type="button" :disabled="index === 0" :aria-label="'上移第 ' + (index + 1) + ' 步'" @click="movingNode(index, -1)">上移</button>
                <button type="button" :disabled="index + 1 === draft.nodes.length" :aria-label="'下移第 ' + (index + 1) + ' 步'" @click="movingNode(index, 1)">下移</button>
                <button type="button" :aria-label="'移除第 ' + (index + 1) + ' 步'" @click="removeTemplateNode(index)">移除</button>
              </div>
              <FeePersonSelect v-if="approverType(item) === 'fixed'" :model-value="selectedApprover(item)" :label="'第 ' + (index + 1) + ' 步指定审批人'" :disabled="busy" @update:model-value="person => selectTemplateApprover(person, index)" />
            </div>
            <button type="button" :disabled="draft.nodes.length >= 12" @click="addTemplateNode">添加审批节点</button>
          </fieldset>
          <fieldset v-if="!draft.approval_nodes" class="oa-fieldset oa-form"><legend>金额加签</legend>
            <label>加签起始金额（元）<input v-model.trim="templateFields.threshold" inputmode="decimal" placeholder="留空或填 0 则不加签" /></label>
            <template v-if="Number(templateFields.threshold) > 0">
              <p class="oa-muted">申请金额达到该金额时，在常规审批结束后加签。</p>
              <BaseSelect label="加签审批人类型" :model-value="approverType(draft.extra || '')" :options="legacyReviewerOptions" placeholder="选择审批人类型" @update:model-value="changeTemplateApproverType(String($event))" />
              <FeePersonSelect v-if="approverType(draft.extra || '') === 'fixed'" :model-value="selectedApprover(draft.extra)" label="加签指定审批人" :disabled="busy" @update:model-value="person => selectTemplateApprover(person)" />
            </template>
          </fieldset>
          <p v-if="templateError" class="oa-error" role="alert">{{ templateError }}</p>
          <div class="oa-actions"><button type="button" :disabled="busy" @click="closeTemplate">关闭编辑</button><button :disabled="busy">保存草稿</button><button v-if="level === 'publish'" class="oa-button-primary" type="button" :disabled="busy" @click="run(() => saveTemplate(true))">发布模板</button></div>
        </form>
      </section>
    </div>
    <section v-if="isAdmin && templateSegment === 'grants'" class="oa-panel">
      <div class="oa-actions fee-grants-heading"><h2>已授权人员</h2>
        <button type="button" @click="startGrant('template')">新增模板维护授权</button>
        <button type="button" @click="startGrant('finance')">新增财务经办授权</button>
      </div>
      <p class="oa-muted">查看显式授权。超级管理员和具备公司级费用管理权限的账号可直接维护、发布模板。付款登记和报表各自授权。</p>
      <form class="oa-actions oa-filter fee-grant-filter" @submit.prevent="run(() => loadGrants(1))">
        <label>搜索人员<input v-model="grantFilter.q" maxlength="240" placeholder="姓名、工号或部门" /></label>
        <BaseSelect class="fee-filter-select" label="权限类型" :model-value="grantFilter.kind" :options="grantKinds" @update:model-value="grantFilter.kind = String($event)" />
        <BaseSelect class="fee-filter-select" label="账号状态" :model-value="grantFilter.status" :options="grantStatusOptions" @update:model-value="grantFilter.status = String($event)" />
        <BaseSelect class="fee-filter-select" label="排序" :model-value="grantFilter.sort" :options="grantSortOptions" @update:model-value="grantFilter.sort = String($event)" />
        <BaseSelect class="fee-filter-select" label="方向" :model-value="grantFilter.order" :options="grantOrderOptions" @update:model-value="grantFilter.order = String($event)" />
        <button type="submit" :disabled="busy">查询</button>
      </form>
      <p v-if="!grantsLoaded && busy" role="status" class="oa-muted">正在读取已授权人员…</p>
      <button v-else-if="!grantsLoaded" type="button" @click="run(loadGrants)">重新读取已授权人员</button>
      <OAEmpty v-else-if="!grantedPeople.length" kind="list" title="暂无显式授权" description="可为模板维护人员或财务经办人员新增授权。" />
      <div v-else class="oa-table-wrap">
        <table class="oa-table">
          <thead><tr><th>人员 / 部门 / 工号</th><th>账号状态</th><th>模板维护</th><th>付款登记</th><th>费用报表</th><th>操作</th></tr></thead>
          <tbody>
            <tr v-for="item in grantedPeople" :key="item.id">
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
        <span>共 {{ grantTotal }} 人 · {{ grantPage }} / {{ grantPages }} 页</span>
        <button :disabled="grantPage <= 1" @click="run(() => loadGrants(grantPage - 1))">上一页</button>
        <button :disabled="grantPage >= grantPages" @click="run(() => loadGrants(grantPage + 1))">下一页</button>
      </nav>
    </section>
    <dialog v-if="isAdmin" aria-labelledby="fee-delegate-title" ref="delegatePanel" class="oa-panel fee-grant-dialog">
      <h2 id="fee-delegate-title">模板维护委托</h2>
      <form class="oa-form" @submit.prevent="run(saveDelegate)">
        <div class="oa-form-row">
          <p v-if="inactiveGrant(delegateTarget)" class="oa-muted">{{ delegateTarget && grantName(delegateTarget) }} · 只可撤销历史授权</p>
          <FeePersonSelect v-else :model-value="delegateTarget" label="受托人" inline :disabled="busy || grantSelectionLoading" @update:model-value="person => selectGrantTarget(person, 'template')" />
          <label>维护权限<NSelect v-model:value="delegate.level" :to="false" :options="permissionOptions" :input-props="{ 'aria-label': '维护权限' }" :disabled="inactiveGrant(delegateTarget) || grantSelectionLoading" /></label>
        </div>
        <div class="oa-actions"><button type="button" :disabled="busy" @click="delegatePanel?.close()">取消</button><button :disabled="busy || !delegateTarget || grantSelectionLoading">{{ inactiveGrant(delegateTarget) ? '确认撤销模板维护' : '保存委托' }}</button></div>
      </form>
    </dialog>
    <dialog v-if="isAdmin" aria-labelledby="fee-finance-grant-title" ref="financeGrantPanel" class="oa-panel fee-grant-dialog">
      <h2 id="fee-finance-grant-title">财务经办授权</h2><p class="oa-subtitle">付款登记和费用报表分别授权。保存时会应用这里选择的两项权限。</p>
      <form class="oa-form oa-person-tabs" @submit.prevent="run(saveFinanceAccess)">
        <p v-if="inactiveGrant(grantTarget)" class="oa-muted">{{ grantTarget && grantName(grantTarget) }} · 只可撤销历史授权</p>
        <FeePersonSelect v-else :model-value="grantTarget" label="经办人" inline :disabled="busy || grantSelectionLoading" @update:model-value="person => selectGrantTarget(person, 'finance')" />
        <div class="oa-checks"><label><input v-model="grants.pay" type="checkbox" :disabled="inactiveGrant(grantTarget) || grantSelectionLoading" />付款登记</label><label><input v-model="grants.report" type="checkbox" :disabled="inactiveGrant(grantTarget) || grantSelectionLoading" />费用报表</label></div>
        <div class="oa-actions"><button type="button" :disabled="busy" @click="financeGrantPanel?.close()">取消</button><button :disabled="!canSaveFinanceGrant">{{ inactiveGrant(grantTarget) ? '确认撤销财务经办' : '保存财务授权' }}</button></div>
      </form>
    </dialog>
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
.fee-amount-trigger { display: inline-block; margin-inline: 4px; padding: 1px 6px; border: 1px dashed var(--oa-warning); border-radius: 4px; font-size: 12px; background: color-mix(in srgb, var(--oa-surface) 92%, var(--oa-warning)); }
.fee-setup-notice svg { vertical-align: text-bottom; margin-inline-end: 4px; }
.fee-setup-notice { background: color-mix(in srgb, var(--oa-surface) 93%, var(--oa-warning)); }
.fee-filter-select { min-width: 8rem; }
.fee-document-number code { overflow-wrap: anywhere; }
.fee-grant-dialog { width: min(480px, calc(100% - 24px)); max-height: calc(100dvh - 24px); overflow: auto; }
.fee-grant-dialog::backdrop { background: rgb(var(--yb-overlay-night) / 40%); }
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

.fee-totals { display: flex; flex-wrap: wrap; gap: 12px 32px; margin: 16px 0; padding-block: 12px; border-block: 1px solid var(--oa-line); }
.fee-totals dt { color: var(--oa-muted); font-size: 12px; }
.fee-totals dd { margin: 4px 0 0; font-size: 24px; font-weight: 600; font-variant-numeric: tabular-nums; }
.fee-apply { width: 100%; max-width: 1200px; margin-inline: auto; }
.fee-apply.fee-apply-empty { grid-template-columns: minmax(0, 1fr); }
.fee-account-picker { grid-column: 1 / -1; }
.fee-document-lines { margin-block: 16px; }
.fee-document-lines h3 { margin-bottom: 8px; }
.fee-application-total { font-size: 28px; font-variant-numeric: tabular-nums; }
.fee-preview-person { display: flex; align-items: center; gap: 6px; }
.fee-preview-person img { width: 24px; height: 24px; object-fit: cover; border-radius: 50%; }
@media (min-width: 960px) {
  .fee-apply:not(.fee-apply-empty) { grid-template-columns: minmax(0, 68fr) minmax(0, 32fr); align-items: start; }
  .fee-application-summary { position: sticky; top: 16px; }
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
.fee-toast { position:fixed; top:60px; right:16px; z-index:45; width:max-content; max-width:calc(100vw - 32px); background:var(--oa-surface); box-shadow:0 4px 16px rgb(var(--yb-overlay-night) / .12); }
.fee-loading::after { display:block; content:''; width:80%; height:10px; margin:12px 0; border-radius:4px; background:var(--yb-line-hairline); box-shadow:0 20px var(--yb-line-hairline); }
.fee-loading { padding-bottom:20px; }
</style>
