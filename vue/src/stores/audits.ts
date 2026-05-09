import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { AuditRecord, AuditHandover } from '@/types'
import { mockAuditRecords, mockHandovers } from '@/mock/audits'
import { auditLogApi } from '@/services/api/auditLogApi'
import { nowISO } from '@/utils/date'

/** 后端返回 snake_case（task_id, auditor_name, created_at 等），映射为前端 camelCase */
function mapAuditRecord(raw: Record<string, unknown>): AuditRecord {
  return {
    id: String(raw.id ?? `ar-${raw.task_id ?? ''}-${raw.created_at ?? Date.now()}`),
    taskId: String(raw.task_id ?? raw.taskId ?? ''),
    auditorId: String(raw.auditor_id ?? raw.auditorId ?? ''),
    auditorName: String(raw.auditor_name ?? raw.auditorName ?? ''),
    action: (raw.action ?? 'pass') as AuditRecord['action'],
    comment: raw.comment != null ? String(raw.comment) : undefined,
    problemCategory: raw.problem_category != null ? String(raw.problem_category) : raw.problemCategory as string | undefined,
    affectLaunch: raw.affect_launch != null ? Boolean(raw.affect_launch) : raw.affectLaunch as boolean | undefined,
    needOutsource: raw.need_outsource != null ? Boolean(raw.need_outsource) : raw.needOutsource as boolean | undefined,
    createdAt: String(raw.created_at ?? raw.createdAt ?? nowISO()),
  }
}

export const useAuditsStore = defineStore('audits', () => {
  const records = ref<AuditRecord[]>([...mockAuditRecords])
  const handovers = ref<AuditHandover[]>([...mockHandovers])
  const loading = ref(false)
  const loadError = ref('')

  async function loadAuditLogs(params?: { taskNo?: string; auditor?: string; action?: string; start?: string; end?: string }) {
    loading.value = true
    loadError.value = ''
    try {
      const res = await auditLogApi.list(params)
      const body = res?.data as { data?: unknown[]; items?: unknown[] } | undefined
      const list = Array.isArray(body) ? body : (body?.data ?? body?.items ?? [])
      const rawList = Array.isArray(list) ? list : []
      records.value = rawList.map((r) => mapAuditRecord(typeof r === 'object' && r !== null ? (r as Record<string, unknown>) : {}))
    } catch {
      loadError.value = ''
      records.value = [...mockAuditRecords]
    } finally {
      loading.value = false
    }
  }

  function addRecord(record: AuditRecord, _action_id?: string) {
    records.value.push(record)
  }

  function addHandover(handover: AuditHandover, _action_id?: string) {
    handovers.value.push(handover)
  }

  return { records, handovers, loading, loadError, loadAuditLogs, addRecord, addHandover }
})
