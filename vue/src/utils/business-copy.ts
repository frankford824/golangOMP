import { mapRawBackendMessageToZh } from '@/utils/api-message-zh'

const TECHNICAL_TOKEN_LABELS: Record<string, string> = {
  filing_error_message: '同步失败原因',
  filing_status: '同步状态',
  erp_sync_status: 'ERP 同步状态',
  erp_sync_required: '仍需同步',
  erp_sync_version: '同步版本',
  last_filing_attempt_at: '最近尝试同步时间',
  last_filed_at: '最近同步成功时间',
  last_filing_payload_json: '最近同步内容',
  request_payload_json: '请求内容',
  response_payload_json: '返回内容',
  integration_call_logs: '同步记录',
  task_details: '任务明细',
  task_sku_items: 'SKU 子项',
  sku_id: '商品编码',
  sku_code: 'SKU 编码',
  skuid: '商品编码',
  i_id: '款式编码',
  iid: '款式编码',
  c_price: '成本价',
  cost_price: '成本价',
  sale_price: '售价',
  pic_url: '图片地址',
  image_url: '图片地址',
  product_name: '产品名称',
  short_name: '商品简称',
  itemskubatchupload: '聚水潭商品资料同步',
  item_style_update: '聚水潭款式资料同步',
  openweb: '聚水潭接口',
  upsert: '新增或更新',
  pending_filing: '待补齐资料后同步',
  filing_failed: '同步失败',
  not_filed: '未同步',
  filed: '已同步',
  pending_claim: '待领取',
  pending_assign: '待指派',
  in_progress: '处理中',
  pending_review: '待审核',
  approved: '已通过',
  rejected: '已打回',
  completed: '已完成',
}

function hasChinese(text: string): boolean {
  return /[\u4e00-\u9fff]/.test(text)
}

function escapeRegExp(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

function replaceTechnicalTokens(raw: string): string {
  let text = raw
  for (const [token, label] of Object.entries(TECHNICAL_TOKEN_LABELS)) {
    const pattern = new RegExp(`(^|[^A-Za-z0-9_])${escapeRegExp(token)}([^A-Za-z0-9_]|$)`, 'gi')
    text = text.replace(pattern, `$1${label}$2`)
  }
  text = text.replace(/\bInProgress\b/g, '处理中')
  text = text.replace(/\bPendingAssign\b/g, '待指派')
  text = text.replace(/\bPendingAudit\b/g, '待审核')
  text = text.replace(/\bmissing\b/gi, '缺少')
  text = text.replace(/\brequired\b/gi, '必填')
  text = text.replace(/\bempty\b/gi, '为空')
  text = text.replace(/\binvalid\b/gi, '无效')
  text = text.replace(/\bnot found\b/gi, '不存在')
  text = text.replace(/\band\b/gi, '和')
  return text
}

function pickReadableMessageFromJson(value: unknown, depth = 0): string {
  if (depth > 3 || value == null) return ''
  if (typeof value === 'string') return value.trim()
  if (Array.isArray(value)) {
    for (const item of value) {
      const picked = pickReadableMessageFromJson(item, depth + 1)
      if (picked) return picked
    }
    return ''
  }
  if (typeof value !== 'object') return ''
  const record = value as Record<string, unknown>
  for (const key of ['message', 'msg', 'error_message', 'errorMessage', 'reason', 'detail']) {
    const direct = record[key]
    if (typeof direct === 'string' && direct.trim()) return direct.trim()
  }
  for (const key of ['error', 'details', 'response', 'data']) {
    const picked = pickReadableMessageFromJson(record[key], depth + 1)
    if (picked) return picked
  }
  return ''
}

function extractReadableMessage(raw: string): string {
  const text = raw.trim()
  if (!text) return ''
  if (!/^[\[{]/.test(text)) return text
  try {
    const picked = pickReadableMessageFromJson(JSON.parse(text))
    return picked || text
  } catch {
    return text
  }
}

function looksTechnical(text: string): boolean {
  const t = text.trim()
  if (!t) return false
  if (/^[a-z][a-z0-9_.-]*(?:\s+[a-z][a-z0-9_.-]*){0,3}$/i.test(t) && !hasChinese(t)) return true
  return /[_{}[\]"']|request_payload|response_payload|openweb|itemskubatchupload|error_message|trace_id|action_type|target_type/i.test(t)
}

function knownErpFailureMessage(raw: string): string {
  const t = raw.toLowerCase()
  if (/(product|商品|产品).*(name|名称).*(length|too long|超过)|erp product name length/.test(t)) {
    return '产品名称过长，聚水潭无法接收。请精简产品名称后重试同步。'
  }
  if (/(short[_\s-]?name|简称).*(length|too long|超过)/.test(t)) {
    return '商品简称过长，聚水潭无法接收。系统会尽量使用自动简称兜底；仍失败时请精简名称后重试。'
  }
  if (/(sku[_\s-]?id|skuid|sku code|商品编码).*(missing|required|empty|invalid|not found|为空|缺失|无效|不存在)/.test(t)) {
    return '商品编码缺失或无效，请确认 SKU 已生成后再同步 ERP。'
  }
  if (/(i[_\s-]?id|style|款式编码).*(missing|required|empty|invalid|not found|为空|缺失|无效|不存在)/.test(t)) {
    return '款式编码缺失或无效，请先选择正确的 ERP 款式编码后重试同步。'
  }
  if (/(c[_\s-]?price|cost[_\s-]?price|成本).*(mismatch|not updated|readback|不一致|未覆盖|未更新)/.test(t)) {
    return '成本价同步后与聚水潭当前值不一致，请重新同步成本，或到 ERP 核对该商品是否允许覆盖成本价。'
  }
  if (/(pic[_\s-]?url|image[_\s-]?url|图片).*(length|too long|超过|failed|失败)/.test(t)) {
    return '图片地址过长或图片同步失败，请重新上传图片后再同步 ERP。'
  }
  if (/(timeout|deadline|timed out|请求超时|超时)/.test(t)) {
    return '连接聚水潭超时，请稍后重试。'
  }
  if (/(too many requests|rate limit|429|频繁|限流)/.test(t)) {
    return '聚水潭接口请求过于频繁，请稍后重试。'
  }
  if (/(unauthorized|auth|required auth|sign|signature|token|app[_\s-]?key|app[_\s-]?secret|签名|授权|鉴权)/.test(t)) {
    return 'ERP 授权异常，请联系管理员检查聚水潭接口授权配置。'
  }
  if (/(connection refused|dial tcp|network error|connection reset|连接失败|网络异常)/.test(t)) {
    return 'ERP 同步服务连接异常，请稍后重试或联系管理员检查同步服务。'
  }
  if (/(duplicate|already exists|重复|已存在)/.test(t)) {
    return 'ERP 中已存在相同编码，请检查商品编码是否重复。'
  }
  if (/(itemskubatchupload|openweb|upsert|jushuitan|聚水潭)/.test(t) && /(fail|error|失败|异常)/.test(t)) {
    return '聚水潭商品资料同步失败，请检查商品编码、款式编码、产品名称、成本和图片后重试。'
  }
  return ''
}

export function formatBusinessTechnicalText(raw: string | null | undefined, fallback = ''): string {
  const extracted = extractReadableMessage(String(raw ?? ''))
  if (!extracted) return fallback
  const mappedApiMessage = mapRawBackendMessageToZh(extracted)
  const base = mappedApiMessage && mappedApiMessage !== '请求参数有误' ? mappedApiMessage : extracted
  const replaced = replaceTechnicalTokens(base).replace(/\s+/g, ' ').trim()
  if (!replaced) return fallback
  if (!hasChinese(replaced) && looksTechnical(replaced)) return fallback || '系统检测到一条需要关注的业务事项'
  return replaced
}

export function formatErpSyncFailureMessage(raw: string | null | undefined): string {
  const extracted = extractReadableMessage(String(raw ?? ''))
  if (!extracted) return ''
  const known = knownErpFailureMessage(extracted)
  if (known) return known
  const formatted = formatBusinessTechnicalText(extracted, '')
  if (!formatted) return 'ERP 同步失败，请检查商品资料后重试。'
  if (!hasChinese(formatted) && looksTechnical(formatted)) return 'ERP 同步失败，请检查商品资料后重试。'
  return formatted
}
