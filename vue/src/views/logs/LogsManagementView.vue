<template>
  <div class="logs-management-view min-h-[100dvh]" :class="{ 'is-embedded': props.embedded }">
    <div v-if="!props.embedded" class="page-header">
      <div>
        <h2 class="page-title">业务追踪中心</h2>
        <p class="page-subtitle">人员、部门、任务、SKU、资产、ERP 全链路</p>
      </div>
      <BaseButton variant="secondary" size="sm" @click="refreshActiveTab">刷新</BaseButton>
    </div>

    <div v-if="!canView" class="mt-6">
      <BaseEmptyState title="无查看权限" description="仅超级管理员或人事管理员可查看业务追踪。" />
    </div>

    <template v-else>
      <div v-if="!props.lockedTab" class="tabs mb-4">
        <button
          type="button"
          class="tab"
          :class="{ active: activeTab === 'business' }"
          @click="activeTab = 'business'"
        >
          业务追踪
        </button>
        <button
          type="button"
          class="tab"
          :class="{ active: activeTab === 'operation' }"
          @click="activeTab = 'operation'"
        >
          操作明细
        </button>
        <button
          type="button"
          class="tab"
          :class="{ active: activeTab === 'permission' }"
          @click="activeTab = 'permission'"
        >
          权限明细
        </button>
        <button
          v-if="canViewServerLogs"
          type="button"
          class="tab"
          :class="{ active: activeTab === 'server' }"
          @click="activeTab = 'server'"
        >
          系统排查
        </button>
      </div>

      <section v-show="activeTab === 'business'" class="business-panel executive-view">
        <section class="executive-metrics">
          <article class="executive-metric primary">
            <span>业务动作</span>
            <strong>{{ traceData.total }}</strong>
            <small>{{ analysisSampleLabel }}</small>
          </article>
          <article class="executive-metric" :class="{ danger: traceFailedTotal > 0 }">
            <span>需要处理</span>
            <strong>{{ traceFailedTotal }}</strong>
            <small>{{ failureRateText }} 异常率</small>
          </article>
          <article class="executive-metric">
            <span>活跃人员</span>
            <strong>{{ analysisActivePeople }}</strong>
            <small>已登录员工</small>
          </article>
          <article class="executive-metric">
            <span>任务 / SKU</span>
            <strong>{{ analysisTaskCount }} / {{ analysisSkuCount }}</strong>
            <small>按当前条件统计</small>
          </article>
        </section>

        <section class="executive-insight">
          <div>
            <p class="eyebrow">智能业务判断</p>
            <h3>{{ insightHeadline }}</h3>
            <ul v-if="!analysisLoading" class="insight-list compact">
              <li v-for="line in businessInsightLines" :key="line">{{ line }}</li>
            </ul>
            <div v-else class="space-y-2">
              <BaseSkeleton width="100%" height="1.25rem" />
              <BaseSkeleton width="88%" height="1.25rem" />
              <BaseSkeleton width="72%" height="1.25rem" />
            </div>
          </div>
          <div class="quick-actions">
            <BaseButton variant="primary" size="sm" @click="applyFailedTrace">只看需要处理</BaseButton>
            <BaseButton variant="secondary" size="sm" @click="applyTodayTrace">回到今日</BaseButton>
          </div>
        </section>

        <section class="business-focus-grid">
          <article class="content-card">
            <div class="section-header">
              <h3 class="section-title">需要优先处理</h3>
              <span class="section-meta">{{ traceFailedTotal }} 条</span>
            </div>
            <div v-if="traceLoading" class="space-y-2">
              <BaseSkeleton width="100%" height="3.25rem" />
              <BaseSkeleton width="100%" height="3.25rem" />
            </div>
            <BaseEmptyState
              v-else-if="!failedTraceItems.length"
              title="暂无需要优先处理的事项"
              description="当前业务口径下没有异常记录。"
            />
            <div v-else class="priority-list">
              <button
                v-for="event in failedTraceItems"
                :key="event.event_id"
                type="button"
                class="priority-item"
                @click="openTraceDetail(event)"
              >
                <span class="priority-time">{{ formatAt(event.occurred_at || event.created_at) }}</span>
                <strong>{{ formatTraceActor(event) }} · {{ formatBusinessAction(event) }}</strong>
                <small>{{ formatBusinessObjects(event) }}</small>
              </button>
            </div>
          </article>

          <article class="content-card">
            <div class="section-header">
              <h3 class="section-title">人员与部门使用</h3>
              <span class="section-meta">点击看明细</span>
            </div>
            <div class="rank-columns">
              <div>
                <h4 class="rank-title">人员</h4>
                <RankList
                  :items="actorRank"
                  empty-label="暂无人员数据"
                  @select="openAnalysisDetail('actor', $event)"
                />
              </div>
              <div>
                <h4 class="rank-title">部门</h4>
                <RankList
                  :items="departmentRank"
                  empty-label="暂无部门数据"
                  @select="openAnalysisDetail('department', $event)"
                />
              </div>
            </div>
          </article>
        </section>

        <section class="content-card filter-card compact-filter-card">
          <div class="section-header">
            <h3 class="section-title">查询条件</h3>
            <span class="section-meta">默认只看员工真实业务动作</span>
          </div>
          <div class="filter-grid executive-filter-grid">
            <label class="filter-field">
              <span>开始时间</span>
              <input v-model="traceFrom" type="datetime-local" class="filter-input" />
            </label>
            <label class="filter-field">
              <span>结束时间</span>
              <input v-model="traceTo" type="datetime-local" class="filter-input" />
            </label>
            <label class="filter-field">
              <span>人员</span>
              <input v-model="traceActorName" type="text" class="filter-input" placeholder="姓名 / 账号" />
            </label>
            <label class="filter-field">
              <span>部门</span>
              <input v-model="traceDepartment" type="text" class="filter-input" placeholder="例如：运营部" />
            </label>
            <label class="filter-field">
              <span>团队</span>
              <input v-model="traceTeam" type="text" class="filter-input" placeholder="例如：运营三组" />
            </label>
            <label class="filter-field">
              <span>任务ID</span>
              <input v-model="traceTaskId" type="text" inputmode="numeric" class="filter-input" placeholder="任务ID" />
            </label>
            <label class="filter-field">
              <span>SKU</span>
              <input v-model="traceSkuCode" type="text" class="filter-input" placeholder="SKU编码" />
            </label>
            <label class="filter-field">
              <span>结果</span>
              <select v-model="traceOutcome" class="filter-select">
                <option value="">全部</option>
                <option value="succeeded">正常</option>
                <option value="failed">需要处理</option>
              </select>
            </label>
          </div>
          <div class="filter-actions">
            <BaseButton variant="primary" size="sm" @click="applyTraceFilters">查询</BaseButton>
            <BaseButton variant="secondary" size="sm" @click="resetTraceFilters">重置</BaseButton>
          </div>
        </section>

        <section class="content-card">
          <div class="section-header">
            <h3 class="section-title">业务明细</h3>
            <span class="section-meta">第 {{ tracePage }} 页 / 共 {{ traceTotalPages }} 页</span>
          </div>
          <div v-if="traceLoading" class="space-y-2">
            <BaseSkeleton width="100%" height="3.5rem" />
            <BaseSkeleton width="100%" height="3.5rem" />
            <BaseSkeleton width="100%" height="3.5rem" />
          </div>
          <BaseErrorState v-else-if="traceError" :title="traceError" @retry="loadTraceEvents" />
          <BaseEmptyState
            v-else-if="!traceItems.length"
            title="暂无业务记录"
            description="根据当前条件未找到已登录员工的有效业务动作。"
          />
          <template v-else>
            <div class="business-event-list">
              <button
                v-for="event in traceItems"
                :key="event.event_id"
                type="button"
                class="business-event-item"
                @click="openTraceDetail(event)"
              >
                <span class="event-time">{{ formatAt(event.occurred_at || event.created_at) }}</span>
                <span class="event-main">
                  <strong>{{ formatTraceActor(event) }} · {{ formatBusinessAction(event) }}</strong>
                  <small>{{ formatTraceActorOrg(event) }} · {{ formatBusinessObjects(event) }}</small>
                </span>
                <span class="event-status">
                  <span class="status-badge" :class="traceOutcomeClass(event)">
                    {{ outcomeLabel(event.outcome, event.http_status) }}
                  </span>
                </span>
              </button>
            </div>
            <div class="pager">
              <button type="button" class="pager-btn" :disabled="tracePage <= 1" @click="tracePage--">上一页</button>
              <span class="pager-info text-xs text-slate-500">第 {{ tracePage }} 页</span>
              <button type="button" class="pager-btn" :disabled="tracePage >= traceTotalPages" @click="tracePage++">下一页</button>
            </div>
          </template>
        </section>
      </section>

      <section v-show="activeTab === 'operation'" class="content-card">
        <h3 class="section-title">操作明细</h3>
        <div class="filter-row">
          <select v-model="opSource" class="filter-select">
            <option value="">全部来源</option>
            <option value="task_event">任务事件</option>
            <option value="export_event">导出任务</option>
            <option value="integration_call">集成调用</option>
          </select>
          <input
            v-model="opEventType"
            type="text"
            class="filter-input"
            placeholder="事件类型"
          />
          <BaseButton variant="primary" size="sm" @click="applyOperationFilters">查询</BaseButton>
        </div>
        <div v-if="opLoading" class="space-y-2">
          <BaseSkeleton width="100%" height="2rem" />
          <BaseSkeleton width="100%" height="2rem" />
        </div>
        <BaseErrorState v-else-if="opError" :title="opError" @retry="loadOperationLogs" />
        <BaseEmptyState
          v-else-if="!operationItems.length"
          title="暂无操作记录"
          description="根据当前条件未找到记录。"
        />
        <template v-else>
          <div class="table-scroll">
            <table class="simple-table">
              <thead>
                <tr>
                  <th>时间</th>
                  <th>来源</th>
                  <th>摘要</th>
                  <th>业务类型</th>
                  <th>操作者</th>
                  <th>关联对象</th>
                  <th>状态</th>
                  <th>原始信息</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="r in operationItems" :key="r.log_id">
                  <td>{{ formatAt(r.created_at) }}</td>
                  <td>{{ opSourceLabel(r.source) }}</td>
                  <td class="summary-cell">{{ r.summary || '—' }}</td>
                  <td class="text-xs text-slate-800" :title="r.event_type || undefined">
                    {{ getOperationEventTypeLabel(r.event_type) }}
                  </td>
                  <td>{{ formatOperationActor(r) }}</td>
                  <td class="ref-cell">
                    <span class="text-slate-600" :title="r.reference_type || undefined">{{ referenceTypeLabel(r.reference_type) }}</span>
                    <span class="font-mono text-xs block">{{ r.reference_id }}</span>
                  </td>
                  <td :title="r.status || undefined">{{ statusLabel(r.status) }}</td>
                  <td class="json-cell-wrap">
                    <template v-if="hasPayload(r)">
                      <div class="json-cell-row">
                        <pre class="json-cell-preview">{{ jsonPreviewLine(r.payload) }}</pre>
                        <BaseButton
                          variant="secondary"
                          size="sm"
                          class="shrink-0"
                          @click="openJsonModal('载荷', r.payload)"
                        >
                          完整
                        </BaseButton>
                      </div>
                    </template>
                    <span v-else class="text-slate-400">—</span>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <div class="pager">
            <button type="button" class="pager-btn" :disabled="opPage <= 1" @click="opPage--">上一页</button>
            <span class="pager-info text-xs text-slate-500">第 {{ opPage }} 页</span>
            <button type="button" class="pager-btn" :disabled="opPage >= opTotalPages" @click="opPage++">下一页</button>
          </div>
        </template>
      </section>

      <section v-show="activeTab === 'permission'" class="content-card">
        <h3 class="section-title">权限明细</h3>
        <div v-if="permLoading" class="space-y-2">
          <BaseSkeleton width="100%" height="2rem" />
          <BaseSkeleton width="100%" height="2rem" />
        </div>
        <BaseErrorState v-else-if="permError" :title="permError" @retry="loadPermissionLogs" />
        <BaseEmptyState
          v-else-if="!permissionItems.length"
          title="暂无权限变更"
          description="根据当前条件未找到记录。"
        />
        <template v-else>
          <div class="table-scroll">
            <table class="simple-table">
              <thead>
                <tr>
                  <th>时间</th>
                  <th>操作人</th>
                  <th>目标用户</th>
                  <th>动作</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="r in permissionItems" :key="String(r.id)">
                  <td>{{ formatAt(r.created_at) }}</td>
                  <td>{{ formatPermActor(r) }}</td>
                  <td>{{ formatPermTarget(r) }}</td>
                  <td>{{ formatPermAction(r) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
          <div class="pager">
            <button type="button" class="pager-btn" :disabled="permPage <= 1" @click="permPage--">上一页</button>
            <span class="pager-info text-xs text-slate-500">第 {{ permPage }} 页</span>
            <button type="button" class="pager-btn" :disabled="permPage >= permTotalPages" @click="permPage++">下一页</button>
          </div>
        </template>
      </section>

      <section v-show="activeTab === 'server' && canViewServerLogs" class="content-card">
        <div class="section-header">
          <h3 class="section-title">系统排查</h3>
          <BaseButton variant="secondary" size="sm" @click="showCleanModal = true">清理旧日志</BaseButton>
        </div>
        <div class="filter-row">
          <select v-model="serverLevel" class="filter-select">
            <option value="">全部级别</option>
            <option value="info">信息</option>
            <option value="warn">警告</option>
            <option value="error">错误</option>
          </select>
          <input
            v-model="serverKeyword"
            type="text"
            class="filter-input"
            placeholder="关键词"
          />
          <input
            v-model="serverSince"
            type="datetime-local"
            class="filter-input"
            placeholder="起始时间"
          />
          <input
            v-model="serverUntil"
            type="datetime-local"
            class="filter-input"
            placeholder="截止时间"
          />
          <BaseButton variant="primary" size="sm" @click="applyServerFilters">查询</BaseButton>
        </div>
        <div v-if="serverLoading" class="space-y-2">
          <BaseSkeleton width="100%" height="2rem" />
          <BaseSkeleton width="100%" height="2rem" />
        </div>
        <BaseErrorState v-else-if="serverError" :title="serverError" @retry="loadServerLogs" />
        <BaseEmptyState
          v-else-if="!serverItems.length"
          title="暂无服务器日志"
          description="根据当前条件未找到记录。"
        />
        <template v-else>
          <div class="table-scroll">
            <table class="simple-table">
              <thead>
                <tr>
                  <th>时间</th>
                  <th>级别</th>
                  <th>消息</th>
                  <th>详情</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="r in serverItems" :key="r.id">
                  <td>{{ formatAt(r.created_at) }}</td>
                  <td><span class="level-badge" :class="'level-' + r.level" :title="r.level || undefined">{{ levelLabel(r.level) }}</span></td>
                  <td class="msg-cell">{{ r.msg }}</td>
                  <td class="json-cell-wrap">
                    <template v-if="hasServerDetails(r)">
                      <div class="json-cell-row">
                        <pre class="json-cell-preview">{{ jsonPreviewLine(normalizeServerDetails(r.details)) }}</pre>
                        <BaseButton
                          variant="secondary"
                          size="sm"
                          class="shrink-0"
                          @click="openJsonModal('详情', normalizeServerDetails(r.details))"
                        >
                          完整
                        </BaseButton>
                      </div>
                    </template>
                    <span v-else class="text-slate-400">—</span>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <div class="pager">
            <button type="button" class="pager-btn" :disabled="serverPage <= 1" @click="serverPage--">上一页</button>
            <span class="pager-info text-xs text-slate-500">第 {{ serverPage }} 页</span>
            <button type="button" class="pager-btn" :disabled="serverPage >= serverTotalPages" @click="serverPage++">下一页</button>
          </div>
        </template>
      </section>

      <BaseModal
        v-model="showAnalysisDetailModal"
        :title="analysisDetailTitle"
        :show-confirm="false"
        :custom-footer="true"
      >
        <div class="analysis-detail">
          <BaseEmptyState
            v-if="!analysisDetailEvents.length"
            title="暂无明细"
            description="当前样本内没有匹配记录。"
          />
          <div v-else class="analysis-detail-list">
            <button
              v-for="event in analysisDetailEvents"
              :key="event.event_id"
              type="button"
              class="analysis-detail-item"
              @click="openTraceDetail(event)"
            >
              <span class="analysis-detail-time">{{ formatAt(event.occurred_at || event.created_at) }}</span>
              <span>
                <strong>{{ formatTraceActor(event) }} · {{ formatBusinessAction(event) }}</strong>
                <small>{{ formatBusinessObjects(event) }}</small>
              </span>
              <span class="status-badge" :class="traceOutcomeClass(event)">
                {{ outcomeLabel(event.outcome, event.http_status) }}
              </span>
            </button>
          </div>
        </div>
        <template #footer>
          <BaseButton variant="primary" size="sm" @click="showAnalysisDetailModal = false">关闭</BaseButton>
        </template>
      </BaseModal>

      <BaseModal
        v-model="showTraceDetailModal"
        title="业务事件详情"
        :show-confirm="false"
        :custom-footer="true"
      >
        <div v-if="selectedTraceEvent" class="trace-detail">
          <div class="detail-grid">
            <div>
              <span class="detail-label">时间</span>
              <strong>{{ formatAt(selectedTraceEvent.occurred_at || selectedTraceEvent.created_at) }}</strong>
            </div>
            <div>
              <span class="detail-label">人员</span>
              <strong>{{ formatTraceActor(selectedTraceEvent) }}</strong>
            </div>
            <div>
              <span class="detail-label">部门 / 团队</span>
              <strong>{{ formatTraceActorOrg(selectedTraceEvent) }}</strong>
            </div>
            <div>
              <span class="detail-label">动作</span>
              <strong>{{ formatBusinessAction(selectedTraceEvent) }}</strong>
            </div>
            <div>
              <span class="detail-label">业务对象</span>
              <strong>{{ formatBusinessObjects(selectedTraceEvent) }}</strong>
            </div>
            <div>
              <span class="detail-label">结果</span>
              <strong>{{ outcomeLabel(selectedTraceEvent.outcome, selectedTraceEvent.http_status) }}</strong>
            </div>
          </div>
          <div class="detail-block">
            <h4>定位信息</h4>
            <div class="detail-grid">
              <div>
                <span class="detail-label">页面 / 位置</span>
                <strong>{{ formatTraceLocation(selectedTraceEvent) }}</strong>
              </div>
              <div>
                <span class="detail-label">记录来源</span>
                <strong>{{ formatBusinessAction(selectedTraceEvent) }}</strong>
              </div>
            </div>
          </div>
        </div>
        <template #footer>
          <BaseButton variant="primary" size="sm" @click="showTraceDetailModal = false">关闭</BaseButton>
        </template>
      </BaseModal>

      <BaseModal
        v-model="showCleanModal"
        title="清理旧服务器日志"
        :show-confirm="false"
        :custom-footer="true"
      >
        <div class="clean-form">
          <label class="clean-label">清理原因 <span class="required">*</span></label>
          <textarea
            v-model="cleanReason"
            class="clean-textarea"
            rows="3"
            placeholder="请输入清理原因（必填）"
          />
          <label class="clean-label">清理早于（小时）</label>
          <input
            v-model.number="cleanOlderThanHours"
            type="number"
            class="clean-input"
            min="1"
            placeholder="24"
          />
        </div>
        <template #footer>
          <BaseButton variant="secondary" size="sm" @click="showCleanModal = false">取消</BaseButton>
          <BaseButton variant="primary" size="sm" :disabled="!cleanReason.trim() || cleanLoading" @click="doCleanLogs">
            {{ cleanLoading ? '清理中...' : '确认清理' }}
          </BaseButton>
        </template>
      </BaseModal>

      <BaseModal
        v-model="showJsonModal"
        :title="jsonModalTitle"
        :show-confirm="false"
        :custom-footer="true"
      >
        <pre class="json-body">{{ jsonModalBody }}</pre>
        <template #footer>
          <BaseButton variant="primary" size="sm" @click="showJsonModal = false">关闭</BaseButton>
        </template>
      </BaseModal>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, defineComponent, h, type PropType } from 'vue'
import { logsApi } from '@/services/api/logsApi'
import type {
  OperationLogEntry,
  PermissionLog,
  ServerLog,
  WorkflowTraceEvent,
} from '@/services/apiTypes'
import BaseSkeleton from '@/components/base/BaseSkeleton.vue'
import BaseEmptyState from '@/components/base/BaseEmptyState.vue'
import BaseErrorState from '@/components/base/BaseErrorState.vue'
import BaseButton from '@/components/base/BaseButton.vue'
import BaseModal from '@/components/base/BaseModal.vue'
import { usePermissionsStore } from '@/stores/permissions'
import { usePermission } from '@/composables/usePermission'
import { beijingDateTimeLocalToISO, formatDateTimeBeijing } from '@/utils/date'
import { getOperationEventTypeLabel } from '@/utils/operation-event-type-labels'
import { userAccountDisplay } from '@/domain/user-display'

type ActiveTab = 'business' | 'operation' | 'permission' | 'server'
type AnalysisDetailKind = 'department' | 'actor' | 'risk'

interface PaginationEnvelope<T> {
  data?: T[]
  pagination?: { total?: unknown }
}

interface RankItem {
  label: string
  count: number
  failed: number
}

const RankList = defineComponent({
  name: 'RankList',
  props: {
    items: {
      type: Array as PropType<RankItem[]>,
      required: true,
    },
    emptyLabel: {
      type: String,
      default: '暂无数据',
    },
  },
  emits: {
    select: (_item: RankItem) => true,
  },
  setup(props, { emit }) {
    return () => {
      if (!props.items.length) {
        return h('div', { class: 'rank-empty' }, props.emptyLabel)
      }
      const max = Math.max(...props.items.map((item) => item.count), 1)
      return h(
        'ul',
        { class: 'rank-list' },
        props.items.map((item, index) => {
          const width = `${Math.max(8, Math.round((item.count / max) * 100))}%`
          return h('li', { class: 'rank-item' }, [
            h(
              'button',
              {
                type: 'button',
                class: 'rank-click',
                onClick: () => emit('select', item),
              },
              [
                h('div', { class: 'rank-row' }, [
                  h('span', { class: 'rank-name', title: item.label }, `${index + 1}. ${item.label}`),
                  h(
                    'span',
                    { class: ['rank-count', item.failed > 0 ? 'is-danger' : ''] },
                    item.failed > 0 ? `${item.count}次 / 异常${item.failed}` : `${item.count}次`,
                  ),
                ]),
                h('span', { class: 'rank-bar' }, [
                  h('i', {
                    class: item.failed > 0 ? 'danger' : '',
                    style: { width },
                  }),
                ]),
              ],
            ),
          ])
        }),
      )
    }
  },
})

const permissionsStore = usePermissionsStore()
const { can } = usePermission()
const canView = computed(() => can('logs.view') || permissionsStore.hasMenu('logs_center'))
const canViewServerLogs = computed(() => can('logs.server.view'))

const props = withDefaults(
  defineProps<{
    embedded?: boolean
    defaultTab?: ActiveTab
    lockedTab?: ActiveTab | ''
  }>(),
  {
    embedded: false,
    defaultTab: 'business',
    lockedTab: '',
  },
)

const activeTab = ref<ActiveTab>(props.lockedTab || props.defaultTab || 'business')

const tracePage = ref(1)
const tracePageSize = 20
const traceLoading = ref(false)
const traceError = ref('')
const traceData = ref<{ items: WorkflowTraceEvent[]; total: number }>({ items: [], total: 0 })
const traceAnalysisData = ref<{ items: WorkflowTraceEvent[]; total: number }>({ items: [], total: 0 })
const analysisLoading = ref(false)
const traceFailedTotal = ref(0)
const traceFrom = ref(beijingTodayStartInput())
const traceTo = ref('')
const traceDepartment = ref('')
const traceTeam = ref('')
const traceActorName = ref('')
const traceTaskId = ref('')
const traceSkuCode = ref('')
const traceOutcome = ref('')
const selectedTraceEvent = ref<WorkflowTraceEvent | null>(null)
const showTraceDetailModal = ref(false)
const showAnalysisDetailModal = ref(false)
const analysisDetailTitle = ref('')
const analysisDetailEvents = ref<WorkflowTraceEvent[]>([])

const traceItems = computed(() => traceData.value.items)
const analysisItems = computed(() => traceAnalysisData.value.items)
const traceTotalPages = computed(() => Math.max(1, Math.ceil(traceData.value.total / tracePageSize)))
const failedTraceItems = computed(() => analysisItems.value.filter(isFailedTrace).slice(0, 5))
const analysisSampleLabel = computed(() => {
  if (analysisLoading.value) return '分析中'
  if (!analysisItems.value.length) return '员工业务口径'
  return `已分析 ${analysisItems.value.length} 条`
})
const failureRateText = computed(() => {
  if (traceData.value.total <= 0) return '—'
  return formatPercent(traceFailedTotal.value / traceData.value.total)
})
const analysisActivePeople = computed(() => uniqueTraceCount(analysisItems.value, actorKey))
const analysisTaskCount = computed(() => uniqueTraceCount(analysisItems.value, (event) => event.task_id ? String(event.task_id) : ''))
const analysisSkuCount = computed(() => uniqueTraceCount(analysisItems.value, (event) => event.sku_code?.trim() ?? ''))
const departmentRank = computed(() =>
  rankTraceEvents(analysisItems.value, (event) => event.actor_department?.trim() || '未归属部门', 5),
)
const actorRank = computed(() =>
  rankTraceEvents(analysisItems.value, (event) => formatTraceActor(event), 5),
)
const riskObjectRank = computed(() => {
  const failed = analysisItems.value.filter(isFailedTrace)
  return rankTraceEvents(failed.length ? failed : analysisItems.value, primaryBusinessObject, 5)
})
const insightHeadline = computed(() => {
  if (!analysisItems.value.length) return '等待业务数据进入后生成判断'
  if (traceFailedTotal.value > 0) return `有 ${traceFailedTotal.value} 条业务异常需要处理`
  return '当前业务运行正常，重点看人员和部门使用情况'
})
const businessInsightLines = computed(() => {
  if (!analysisItems.value.length) {
    return ['当前筛选范围内暂无员工有效业务动作。', '页面已自动聚焦真实业务操作。']
  }
  const lines: string[] = []
  const topRisk = riskObjectRank.value[0]
  const topDept = departmentRank.value[0]
  const topActor = actorRank.value[0]

  if (traceFailedTotal.value > 0) {
    const target = topRisk?.label && topRisk.label !== '系统记录' ? `，优先看 ${topRisk.label}` : ''
    lines.push(`当前有 ${traceData.value.total} 次有效业务动作，其中 ${traceFailedTotal.value} 条需要处理${target}。`)
  } else {
    lines.push(`当前有 ${traceData.value.total} 次有效业务动作，暂未发现需要处理的异常。`)
  }
  if (topActor) lines.push(`${topActor.label} 当前最活跃，共 ${topActor.count} 次操作。`)
  if (topDept) lines.push(`${topDept.label} 使用量最高，共 ${topDept.count} 次，异常 ${topDept.failed} 次。`)
  if (analysisTaskCount.value || analysisSkuCount.value) {
    lines.push(`涉及 ${analysisTaskCount.value} 条任务、${analysisSkuCount.value} 个 SKU，可点击明细看证据链。`)
  }
  return lines
})

const opPage = ref(1)
const skipNextOpPageWatch = ref(false)
const opPageSize = 20
const opLoading = ref(false)
const opError = ref('')
const opData = ref<{ items: OperationLogEntry[]; total: number }>({ items: [], total: 0 })
const opSource = ref<'' | OperationLogEntry['source']>('')
const opEventType = ref('')

const permPage = ref(1)
const permPageSize = 20
const permLoading = ref(false)
const permError = ref('')
const permData = ref<{ items: PermissionLog[]; total: number }>({ items: [], total: 0 })

const operationItems = computed(() => opData.value.items)
const opTotalPages = computed(() => Math.max(1, Math.ceil(opData.value.total / opPageSize)))

const permissionItems = computed(() => permData.value.items)
const permTotalPages = computed(() => Math.max(1, Math.ceil(permData.value.total / permPageSize)))

const serverPage = ref(1)
const serverPageSize = 20
const serverLoading = ref(false)
const serverError = ref('')
const serverData = ref<{ items: ServerLog[]; total: number }>({ items: [], total: 0 })
const serverLevel = ref<'info' | 'warn' | 'error' | ''>('')
const serverKeyword = ref('')
const serverSince = ref('')
const serverUntil = ref('')
const showCleanModal = ref(false)
const cleanReason = ref('')
const cleanOlderThanHours = ref(24)
const cleanLoading = ref(false)

const showJsonModal = ref(false)
const jsonModalTitle = ref('')
const jsonModalBody = ref('')

const serverItems = computed(() => serverData.value.items)
const serverTotalPages = computed(() => Math.max(1, Math.ceil(serverData.value.total / serverPageSize)))

const OP_SOURCE_LABEL: Record<OperationLogEntry['source'], string> = {
  task_event: '任务',
  export_event: '导出',
  integration_call: '集成',
}

const REFERENCE_TYPE_LABEL: Record<string, string> = {
  task: '任务',
  asset: '资产',
  customization_job: '定制任务',
  user: '用户',
  role: '角色',
  department: '部门',
  team: '团队',
  export: '导出',
  outsource: '外协单',
  warehouse: '仓库',
}

const LOG_LEVEL_LABEL: Record<string, string> = {
  trace: '追踪',
  debug: '调试',
  info: '信息',
  warn: '警告',
  warning: '警告',
  error: '错误',
  fatal: '致命',
}

const LOG_STATUS_LABEL: Record<string, string> = {
  success: '成功',
  ok: '成功',
  pass: '成功',
  succeeded: '成功',
  failed: '失败',
  fail: '失败',
  error: '失败',
  pending: '处理中',
  running: '进行中',
  retrying: '重试中',
  skipped: '已跳过',
  ignored: '已忽略',
  timeout: '已超时',
  cancelled: '已取消',
}

const TRACE_EVENT_TYPE_LABEL: Record<string, string> = {
  api_request: '接口请求',
  page_view: '页面访问',
  user_action: '用户操作',
}

function formatAt(iso: string) {
  return formatDateTimeBeijing(iso)
}

function opSourceLabel(s: OperationLogEntry['source']) {
  return OP_SOURCE_LABEL[s] ?? s
}

function localizeEnum(dict: Record<string, string>, raw: unknown): string {
  if (raw === null || raw === undefined) return '—'
  const key = String(raw).trim()
  if (!key) return '—'
  return dict[key.toLowerCase()] ?? key
}

function referenceTypeLabel(t: string | undefined | null): string {
  return localizeEnum(REFERENCE_TYPE_LABEL, t)
}

function levelLabel(t: string | undefined | null): string {
  return localizeEnum(LOG_LEVEL_LABEL, t)
}

function statusLabel(t: string | undefined | null): string {
  return localizeEnum(LOG_STATUS_LABEL, t)
}

function knownUserDisplay(...candidates: unknown[]): string {
  const text = userAccountDisplay(...candidates)
  return text === '未知用户' ? '' : text
}

function formatOperationActor(r: OperationLogEntry) {
  const displayName = knownUserDisplay(r.actor_username)
  if (displayName) return displayName
  if (r.actor_id) return `用户#${r.actor_id}`
  if (r.actor_type === 'system') return '系统自动处理'
  return '未识别身份'
}

function hasPayload(r: OperationLogEntry) {
  const p = r.payload
  if (p === null || p === undefined) return false
  if (typeof p === 'object' && !Array.isArray(p)) return Object.keys(p as object).length > 0
  return true
}

function formatJsonPretty(value: unknown): string {
  if (value === null || value === undefined) return ''
  if (typeof value === 'string') {
    try {
      return JSON.stringify(JSON.parse(value), null, 2)
    } catch {
      return value
    }
  }
  try {
    return JSON.stringify(value, null, 2)
  } catch {
    return String(value)
  }
}

function jsonPreviewLine(value: unknown): string {
  try {
    let s: string
    if (typeof value === 'string') {
      try {
        s = JSON.stringify(JSON.parse(value))
      } catch {
        s = value
      }
    } else {
      s = JSON.stringify(value)
    }
    return s.length > 120 ? `${s.slice(0, 120)}...` : s
  } catch {
    return String(value)
  }
}

function openJsonModal(title: string, value: unknown) {
  jsonModalTitle.value = title
  jsonModalBody.value = formatJsonPretty(value)
  showJsonModal.value = true
}

function normalizeServerDetails(raw: ServerLog['details']): unknown {
  if (raw === null || raw === undefined) return null
  if (typeof raw === 'string') {
    try {
      return JSON.parse(raw)
    } catch {
      return raw
    }
  }
  return raw
}

function hasServerDetails(r: ServerLog): boolean {
  const d = r.details
  if (d === null || d === undefined) return false
  if (typeof d === 'string') return d.trim().length > 0
  if (typeof d === 'object') return Object.keys(d as object).length > 0
  return true
}

function formatPermActor(r: PermissionLog) {
  const displayName = knownUserDisplay(r.actor_username)
  if (displayName) return displayName
  if (r.actor_id) return `用户#${r.actor_id}`
  if (r.actor_source === 'system') return '系统自动处理'
  return '未识别身份'
}

function formatPermTarget(r: PermissionLog) {
  const displayName = knownUserDisplay(r.target_username)
  if (displayName) return displayName
  if (r.target_user_id) return `用户#${r.target_user_id}`
  return '未识别身份'
}

const PERM_ACTION_LABEL: Record<string, string> = {
  'role.assign': '分配角色',
  'role.revoke': '撤销角色',
  'role.create': '创建角色',
  'role.update': '更新角色',
  'role.delete': '删除角色',
  'user.create': '创建用户',
  'user.update': '更新用户',
  'user.delete': '删除用户',
  'user.disable': '停用用户',
  'user.enable': '启用用户',
  'user.password_reset': '重置密码',
  'permission.grant': '授予权限',
  'permission.revoke': '撤销权限',
  'department.create': '创建部门',
  'department.update': '更新部门',
  'department.delete': '删除部门',
  'team.create': '创建团队',
  'team.update': '更新团队',
  'team.delete': '删除团队',
  login: '登录',
  logout: '退出登录',
}

function formatPermAction(r: PermissionLog) {
  const raw = r.action_type?.trim()
  if (raw) return PERM_ACTION_LABEL[raw.toLowerCase()] ?? raw
  const route = [r.method, r.route_path].filter(Boolean).join(' ')
  return route.trim() || '—'
}

function beijingTodayStartInput(): string {
  const beijing = new Date(Date.now() + 8 * 60 * 60 * 1000)
  const year = beijing.getUTCFullYear()
  const month = String(beijing.getUTCMonth() + 1).padStart(2, '0')
  const day = String(beijing.getUTCDate()).padStart(2, '0')
  return `${year}-${month}-${day}T00:00`
}

function parsePositiveIntFilter(value: string, label: string): number | undefined {
  const trimmed = value.trim()
  if (!trimmed) return undefined
  if (!/^\d+$/.test(trimmed)) throw new Error(`${label}必须是正整数`)
  const parsed = Number(trimmed)
  if (!Number.isSafeInteger(parsed) || parsed <= 0) throw new Error(`${label}必须是正整数`)
  return parsed
}

function buildTraceParams(page: number, pageSize = tracePageSize) {
  const params: {
    page: number
    page_size: number
    actor_username?: string
    actor_source?: string
    actor_department?: string
    actor_team?: string
    task_id?: number
    sku_code?: string
    outcome?: string
    business_only?: boolean
    from?: string
    to?: string
  } = { page, page_size: pageSize, actor_source: 'session_token', business_only: true }

  const taskID = parsePositiveIntFilter(traceTaskId.value, '任务ID')
  if (taskID) params.task_id = taskID
  if (traceActorName.value.trim()) params.actor_username = traceActorName.value.trim()
  if (traceDepartment.value.trim()) params.actor_department = traceDepartment.value.trim()
  if (traceTeam.value.trim()) params.actor_team = traceTeam.value.trim()
  if (traceSkuCode.value.trim()) params.sku_code = traceSkuCode.value.trim()
  if (traceOutcome.value) params.outcome = traceOutcome.value
  if (traceFrom.value) {
    const from = beijingDateTimeLocalToISO(traceFrom.value)
    if (from) params.from = from
  }
  if (traceTo.value) {
    const to = beijingDateTimeLocalToISO(traceTo.value)
    if (to) params.to = to
  }
  return params
}

function parsePaginationTotal(pagination: { total?: unknown } | undefined): number | null {
  const t = pagination?.total
  if (typeof t === 'number' && Number.isFinite(t) && t >= 0) return Math.floor(t)
  if (typeof t === 'string' && t.trim() !== '') {
    const n = Number(t)
    if (Number.isFinite(n) && n >= 0) return Math.floor(n)
  }
  return null
}

function unpackPaginated<T>(body: PaginationEnvelope<T> | undefined): { items: T[]; total: number } {
  const items = Array.isArray(body?.data) ? body.data : []
  const total = parsePaginationTotal(body?.pagination) ?? items.length
  return { items, total }
}

function formatPercent(value: number): string {
  if (!Number.isFinite(value)) return '—'
  return `${Math.round(value * 100)}%`
}

function actorKey(event: WorkflowTraceEvent): string {
  if (event.actor_id) return String(event.actor_id)
  return event.actor_username?.trim() ?? ''
}

function uniqueTraceCount(events: WorkflowTraceEvent[], keyFn: (event: WorkflowTraceEvent) => string): number {
  const values = new Set<string>()
  for (const event of events) {
    const value = keyFn(event).trim()
    if (value) values.add(value)
  }
  return values.size
}

function rankTraceEvents(
  events: WorkflowTraceEvent[],
  labelFn: (event: WorkflowTraceEvent) => string,
  limit: number,
): RankItem[] {
  const map = new Map<string, RankItem>()
  for (const event of events) {
    const label = labelFn(event).trim() || '未分类'
    const item = map.get(label) ?? { label, count: 0, failed: 0 }
    item.count += 1
    if (isFailedTrace(event)) item.failed += 1
    map.set(label, item)
  }
  return [...map.values()]
    .sort((a, b) => b.failed - a.failed || b.count - a.count || a.label.localeCompare(b.label, 'zh-Hans-CN'))
    .slice(0, limit)
}

async function loadTraceEvents() {
  if (!canView.value) return
  traceLoading.value = true
  traceError.value = ''
  try {
    const res = await logsApi.traceEvents(buildTraceParams(tracePage.value))
    const body = res?.data as PaginationEnvelope<WorkflowTraceEvent> | undefined
    traceData.value = unpackPaginated(body)
  } catch (e) {
    traceError.value = e instanceof Error ? e.message : '加载业务追踪失败'
  } finally {
    traceLoading.value = false
  }
}

async function loadTraceAnalysis() {
  if (!canView.value) return
  analysisLoading.value = true
  try {
    const res = await logsApi.traceEvents(buildTraceParams(1, 100))
    const body = res?.data as PaginationEnvelope<WorkflowTraceEvent> | undefined
    traceAnalysisData.value = unpackPaginated(body)
  } catch {
    traceAnalysisData.value = { items: [], total: 0 }
  } finally {
    analysisLoading.value = false
  }
}

async function loadTraceStats() {
  if (!canView.value) return
  try {
    const params = buildTraceParams(1, 1)
    delete params.outcome
    const res = await logsApi.traceEvents({ ...params, outcome: 'failed' })
    const body = res?.data as PaginationEnvelope<WorkflowTraceEvent> | undefined
    traceFailedTotal.value = parsePaginationTotal(body?.pagination) ?? 0
  } catch {
    traceFailedTotal.value = 0
  }
}

function applyTraceFilters() {
  tracePage.value = 1
  loadTraceEvents()
  loadTraceAnalysis()
  loadTraceStats()
}

function resetTraceFilters() {
  traceFrom.value = beijingTodayStartInput()
  traceTo.value = ''
  traceDepartment.value = ''
  traceTeam.value = ''
  traceActorName.value = ''
  traceTaskId.value = ''
  traceSkuCode.value = ''
  traceOutcome.value = ''
  applyTraceFilters()
}

function applyTodayTrace() {
  traceFrom.value = beijingTodayStartInput()
  traceTo.value = ''
  applyTraceFilters()
}

function applyFailedTrace() {
  traceOutcome.value = 'failed'
  applyTraceFilters()
}

function refreshActiveTab() {
  if (activeTab.value === 'business') {
    loadTraceEvents()
    loadTraceAnalysis()
    loadTraceStats()
  } else if (activeTab.value === 'operation') {
    loadOperationLogs()
  } else if (activeTab.value === 'permission') {
    loadPermissionLogs()
  } else if (activeTab.value === 'server') {
    loadServerLogs()
  }
}

function formatTraceActor(event: WorkflowTraceEvent): string {
  const displayName = knownUserDisplay(event.actor_username)
  if (displayName) return displayName
  if (event.actor_id) return `用户#${event.actor_id}`
  if (event.actor_source === 'anonymous') return '未登录访问'
  if (event.actor_source === 'system_fallback' || event.event_source === 'system') return '系统自动处理'
  return '未识别身份'
}

function formatTraceActorOrg(event: WorkflowTraceEvent): string {
  const parts = [event.actor_department, event.actor_team]
    .map((v) => v?.trim())
    .filter((v): v is string => Boolean(v))
  return parts.length ? parts.join(' / ') : '—'
}

function methodBusinessVerb(method: string | undefined): string {
  const m = method?.trim().toUpperCase()
  if (m === 'GET') return '查看'
  if (m === 'POST') return '提交'
  if (m === 'PUT' || m === 'PATCH') return '更新'
  if (m === 'DELETE') return '删除'
  return '处理'
}

function routeBusinessTarget(event: WorkflowTraceEvent): string {
  const raw = [event.route_path, event.route_full_path, event.page_url, event.action].join(' ').toLowerCase()
  if (raw.includes('/tasks') || raw.includes('task')) return '任务'
  if (raw.includes('/asset') || raw.includes('asset')) return '资产'
  if (raw.includes('/erp') || raw.includes('erp')) return 'ERP数据'
  if (raw.includes('/sku') || raw.includes('sku')) return 'SKU'
  if (raw.includes('/warehouse') || raw.includes('warehouse')) return '仓库'
  if (raw.includes('/audit') || raw.includes('audit')) return '审核'
  if (raw.includes('/org') || raw.includes('/users')) return '组织人员'
  if (raw.includes('/reports')) return '报表'
  if (raw.includes('/finance')) return '财务'
  return '系统记录'
}

function formatBusinessAction(event: WorkflowTraceEvent): string {
  const eventType = event.event_type?.trim()
  if (eventType === 'page_view') {
    return event.page_name ? `打开${event.page_name}` : '打开页面'
  }
  if (eventType === 'user_action') return event.action?.trim() || '用户操作'
  if (event.event_source === 'integration') return event.action?.trim() || '外部系统调用'
  if (event.event_source === 'api') {
    return `${methodBusinessVerb(event.route_method)}${routeBusinessTarget(event)}`
  }
  return event.action?.trim() || localizeEnum(TRACE_EVENT_TYPE_LABEL, event.event_type)
}

function businessObjectChips(event: WorkflowTraceEvent): string[] {
  const chips: string[] = []
  if (event.task_id) chips.push(`任务#${event.task_id}`)
  if (event.sku_code?.trim()) chips.push(`SKU ${event.sku_code.trim()}`)
  if (event.asset_id) chips.push(`资产#${event.asset_id}`)
  if (event.design_asset_id) chips.push(`设计资产#${event.design_asset_id}`)
  if (event.task_asset_id) chips.push(`任务资产#${event.task_asset_id}`)
  if (event.integration_call_log_id) chips.push(`集成记录#${event.integration_call_log_id}`)
  if (event.resource_type?.trim() && event.resource_id?.trim()) {
    chips.push(`${referenceTypeLabel(event.resource_type)}#${event.resource_id}`)
  }
  if (!chips.length) chips.push(routeBusinessTarget(event))
  return chips
}

function primaryBusinessObject(event: WorkflowTraceEvent): string {
  return businessObjectChips(event)[0] ?? '系统记录'
}

function formatBusinessObjects(event: WorkflowTraceEvent): string {
  return businessObjectChips(event).join('，')
}

function formatTraceLocation(event: WorkflowTraceEvent): string {
  return (
    event.page_name?.trim() ||
    event.route_path?.trim() ||
    event.route_full_path?.trim() ||
    event.page_url?.trim() ||
    event.component_id?.trim() ||
    '—'
  )
}

function outcomeLabel(outcome: string | undefined, httpStatus?: number | null): string {
  if (httpStatus && httpStatus >= 500) return '异常'
  if (httpStatus === 403) return '无权限'
  if (httpStatus === 404) return '未找到'
  if (httpStatus && httpStatus >= 400) return '未完成'
  const raw = outcome?.trim()
  if (!raw) return '—'
  if (raw === 'succeeded') return '成功'
  if (raw === 'failed') return '异常'
  return statusLabel(raw)
}

function isFailedTrace(event: WorkflowTraceEvent): boolean {
  if (event.http_status) return event.http_status >= 500
  return event.outcome === 'failed'
}

function traceOutcomeClass(event: WorkflowTraceEvent): string {
  if (isFailedTrace(event)) return 'status-failed'
  if (event.outcome === 'succeeded') return 'status-success'
  return 'status-neutral'
}

function openAnalysisDetail(kind: AnalysisDetailKind, item: RankItem) {
  const matcher = analysisDetailMatcher(kind, item.label)
  analysisDetailEvents.value = analysisItems.value.filter(matcher).slice(0, 30)
  analysisDetailTitle.value = `${analysisKindLabel(kind)}：${item.label}`
  showAnalysisDetailModal.value = true
}

function analysisKindLabel(kind: AnalysisDetailKind): string {
  if (kind === 'department') return '部门使用明细'
  if (kind === 'actor') return '人员活跃明细'
  return '任务/SKU风险明细'
}

function analysisDetailMatcher(kind: AnalysisDetailKind, label: string): (event: WorkflowTraceEvent) => boolean {
  return (event) => {
    if (kind === 'department') return (event.actor_department?.trim() || '未归属部门') === label
    if (kind === 'actor') return formatTraceActor(event) === label
    return primaryBusinessObject(event) === label
  }
}

function openTraceDetail(event: WorkflowTraceEvent) {
  selectedTraceEvent.value = event
  showTraceDetailModal.value = true
}

function applyOperationFilters() {
  opPage.value = 1
  loadOperationLogs()
}

function resolveOperationLogTotal(
  pagination: { total?: unknown } | undefined,
  items: OperationLogEntry[],
  pageRequested: number,
  pageSize: number,
  previousTotal: number,
): number {
  const parsed = parsePaginationTotal(pagination)
  if (parsed !== null) return parsed
  if (items.length > 0) {
    return Math.max(previousTotal, (pageRequested - 1) * pageSize + items.length)
  }
  if (pageRequested > 1) return previousTotal
  return 0
}

async function loadOperationLogs() {
  if (!canView.value) return
  opLoading.value = true
  opError.value = ''
  try {
    const buildParams = (page: number) => {
      const params: {
        page: number
        page_size: number
        source?: OperationLogEntry['source']
        event_type?: string
      } = { page, page_size: opPageSize }
      if (opSource.value) params.source = opSource.value
      const et = opEventType.value.trim()
      if (et) params.event_type = et
      return params
    }

    let pageToFetch = opPage.value
    let res = await logsApi.operationLogs(buildParams(pageToFetch))
    let body = res?.data as PaginationEnvelope<OperationLogEntry> | undefined
    let items = Array.isArray(body?.data) ? body.data : []
    let total = resolveOperationLogTotal(
      body?.pagination,
      items,
      pageToFetch,
      opPageSize,
      opData.value.total,
    )
    let maxPage = Math.max(1, Math.ceil(total / opPageSize) || 1)

    if (pageToFetch > maxPage) {
      pageToFetch = maxPage
      res = await logsApi.operationLogs(buildParams(pageToFetch))
      body = res?.data as typeof body
      items = Array.isArray(body?.data) ? body.data : []
      total = resolveOperationLogTotal(
        body?.pagination,
        items,
        pageToFetch,
        opPageSize,
        total,
      )
      maxPage = Math.max(1, Math.ceil(total / opPageSize) || 1)
    }

    opData.value = { items, total }
    if (opPage.value !== pageToFetch) {
      skipNextOpPageWatch.value = true
      opPage.value = pageToFetch
    }
  } catch (e) {
    opError.value = e instanceof Error ? e.message : '加载操作记录失败'
  } finally {
    opLoading.value = false
  }
}

async function loadPermissionLogs() {
  if (!canView.value) return
  permLoading.value = true
  permError.value = ''
  try {
    const res = await logsApi.permissionLogs({ page: permPage.value, page_size: permPageSize })
    const httpBody = res?.data as PaginationEnvelope<PermissionLog> | undefined
    permData.value = unpackPaginated(httpBody)
  } catch (e) {
    permError.value = e instanceof Error ? e.message : '加载权限变更失败'
  } finally {
    permLoading.value = false
  }
}

async function loadServerLogs() {
  if (!canViewServerLogs.value) return
  serverLoading.value = true
  serverError.value = ''
  try {
    const params: Record<string, unknown> = {
      page: serverPage.value,
      page_size: serverPageSize,
    }
    if (serverLevel.value) params.level = serverLevel.value
    if (serverKeyword.value.trim()) params.keyword = serverKeyword.value.trim()
    if (serverSince.value) {
      const since = beijingDateTimeLocalToISO(serverSince.value)
      if (since) params.since = since
    }
    if (serverUntil.value) {
      const until = beijingDateTimeLocalToISO(serverUntil.value)
      if (until) params.until = until
    }
    const res = await logsApi.serverLogs(params)
    const body = res?.data
    const items = Array.isArray(body) ? body : (body?.data ?? body?.items ?? [])
    const total = typeof body?.pagination?.total === 'number' ? body.pagination.total : items.length
    serverData.value = { items, total }
  } catch (e) {
    serverError.value = e instanceof Error ? e.message : '加载服务器日志失败'
  } finally {
    serverLoading.value = false
  }
}

function applyServerFilters() {
  serverPage.value = 1
  loadServerLogs()
}

async function doCleanLogs() {
  if (!cleanReason.value.trim() || !canViewServerLogs.value) return
  cleanLoading.value = true
  try {
    await logsApi.serverLogsClean({
      reason: cleanReason.value.trim(),
      older_than_hours: cleanOlderThanHours.value || 24,
    })
    showCleanModal.value = false
    cleanReason.value = ''
    cleanOlderThanHours.value = 24
    await loadServerLogs()
  } catch (e) {
    serverError.value = e instanceof Error ? e.message : '清理失败'
  } finally {
    cleanLoading.value = false
  }
}

watch(activeTab, (tab) => {
  if (tab === 'business' && !traceData.value.items.length && !traceLoading.value) {
    loadTraceEvents()
    loadTraceAnalysis()
    loadTraceStats()
  }
  if (tab === 'operation' && !opData.value.items.length && !opLoading.value) loadOperationLogs()
  if (tab === 'permission' && !permData.value.items.length && !permLoading.value) loadPermissionLogs()
  if (tab === 'server' && canViewServerLogs.value) loadServerLogs()
})
watch(
  () => props.lockedTab,
  (tab) => {
    if (tab && activeTab.value !== tab) activeTab.value = tab
  },
)
watch(tracePage, loadTraceEvents)
watch(opPage, () => {
  if (skipNextOpPageWatch.value) {
    skipNextOpPageWatch.value = false
    return
  }
  loadOperationLogs()
})
watch(permPage, loadPermissionLogs)
watch(serverPage, loadServerLogs)

onMounted(() => {
  refreshActiveTab()
})
</script>

<style scoped>
.logs-management-view {
  width: 100%;
  max-width: 100%;
  overflow-x: hidden;
  padding: 0;
  color: #111827;
}
.page-header {
  margin-bottom: 1rem;
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}
.page-title { margin: 0; font-size: 1.125rem; font-weight: 700; color: #111827; }
.page-subtitle { margin: 0.25rem 0 0; font-size: 0.75rem; color: #6b7280; }
.tabs {
  display: flex;
  width: 100%;
  padding: 0.25rem;
  gap: 0.25rem;
  border-radius: 0.9rem;
  background: #f3f4f6;
  border: 1px solid #e5e7eb;
  max-width: 100%;
  overflow-x: auto;
}
.tab {
  flex: 0 0 auto;
  padding: 0.5rem 1rem;
  border-radius: 0.7rem;
  border: 1px solid transparent;
  background: transparent;
  color: #6b7280;
  cursor: pointer;
  font-size: 0.875rem;
  font-weight: 700;
  white-space: nowrap;
}
.tab:hover { background: #ffffff; color: #111827; }
.tab.active { background: #2563eb; border-color: #2563eb; color: #fff; }
.business-panel { display: grid; gap: 1rem; }
.executive-view {
  width: 100%;
  max-width: 100%;
}
.executive-metrics {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(100%, 11rem), 1fr));
  gap: 0.75rem;
}
.executive-metric {
  min-width: 0;
  border: 1px solid #e5e7eb;
  border-radius: 0.5rem;
  background: #ffffff;
  padding: 0.9rem 1rem;
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.05);
}
.executive-metric.primary {
  border-color: #bfdbfe;
  background: #eff6ff;
}
.executive-metric.danger {
  border-color: #fecaca;
  background: #fff7f7;
}
.executive-metric span,
.executive-metric small {
  display: block;
  color: #6b7280;
  font-size: 0.72rem;
  line-height: 1.35;
}
.executive-metric strong {
  display: block;
  margin: 0.35rem 0 0.2rem;
  color: #111827;
  font-size: 1.55rem;
  line-height: 1;
  font-weight: 850;
}
.executive-insight {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 1rem;
  align-items: start;
  border: 1px solid #dbeafe;
  border-radius: 0.5rem;
  background: #ffffff;
  padding: 1rem;
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.05);
}
.executive-insight h3 {
  margin: 0 0 0.65rem;
  color: #111827;
  font-size: 1.1rem;
  line-height: 1.35;
  font-weight: 800;
}
.quick-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 0.5rem;
}
.business-focus-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 0.75rem;
}
.priority-list,
.business-event-list {
  display: grid;
  gap: 0.55rem;
}
.priority-item,
.business-event-item {
  width: 100%;
  min-width: 0;
  border: 1px solid #e5e7eb;
  border-radius: 0.45rem;
  background: #ffffff;
  padding: 0.65rem 0.75rem;
  text-align: left;
  cursor: pointer;
}
.priority-item:hover,
.business-event-item:hover {
  border-color: #bfdbfe;
  background: #f8fbff;
}
.priority-time,
.event-time {
  color: #6b7280;
  font-size: 0.72rem;
  white-space: nowrap;
}
.priority-item strong,
.priority-item small,
.event-main strong,
.event-main small {
  display: block;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.priority-item strong,
.event-main strong {
  margin-top: 0.18rem;
  color: #111827;
  font-size: 0.82rem;
  font-weight: 800;
}
.priority-item small,
.event-main small {
  margin-top: 0.2rem;
  color: #6b7280;
  font-size: 0.72rem;
}
.rank-columns {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 1rem;
}
.rank-title {
  margin: 0 0 0.55rem;
  color: #374151;
  font-size: 0.75rem;
  font-weight: 800;
}
.compact-filter-card { padding-bottom: 0.85rem; }
.executive-filter-grid {
  grid-template-columns: repeat(4, minmax(0, 1fr));
}
.business-event-item {
  display: grid;
  grid-template-columns: 10rem minmax(0, 1fr) auto;
  gap: 0.75rem;
  align-items: center;
}
.event-status {
  display: flex;
  justify-content: flex-end;
}
.insight-board {
  display: grid;
  grid-template-columns: minmax(0, 1.5fr) minmax(18rem, 0.9fr);
  gap: 1rem;
}
.insight-hero,
.insight-mini-card,
.analysis-card {
  border: 1px solid #e5e7eb;
  border-radius: 0.5rem;
  background: #ffffff;
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.05);
}
.insight-hero { padding: 1rem; }
.insight-hero-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  margin-bottom: 0.9rem;
}
.eyebrow {
  margin: 0 0 0.25rem;
  font-size: 0.7rem;
  color: #2563eb;
  font-weight: 800;
}
.insight-hero h3 {
  margin: 0;
  color: #111827;
  font-size: 1.2rem;
  line-height: 1.35;
  font-weight: 800;
}
.sample-badge {
  display: inline-flex;
  align-items: center;
  white-space: nowrap;
  border-radius: 999px;
  background: #ecfdf5;
  color: #047857;
  padding: 0.2rem 0.55rem;
  font-size: 0.7rem;
  font-weight: 800;
}
.insight-list {
  margin: 0;
  padding: 0;
  display: grid;
  gap: 0.45rem;
  list-style: none;
}
.insight-list.compact {
  gap: 0.35rem;
}
.insight-list li {
  position: relative;
  padding-left: 1rem;
  color: #374151;
  font-size: 0.82rem;
  line-height: 1.55;
}
.insight-list li::before {
  content: '';
  position: absolute;
  left: 0;
  top: 0.62rem;
  width: 0.38rem;
  height: 0.38rem;
  border-radius: 999px;
  background: #2563eb;
}
.insight-side-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.75rem;
}
.insight-mini-card {
  padding: 0.85rem;
  min-height: 6.2rem;
}
.insight-mini-card.danger {
  border-color: #fecaca;
  background: #fff7f7;
}
.insight-mini-card span,
.insight-mini-card small {
  display: block;
  color: #6b7280;
  font-size: 0.72rem;
  line-height: 1.35;
}
.insight-mini-card strong {
  display: block;
  margin: 0.4rem 0 0.2rem;
  color: #111827;
  font-size: 1.45rem;
  line-height: 1;
  font-weight: 850;
}
.analysis-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 0.75rem;
}
.analysis-card {
  padding: 0.85rem;
  min-width: 0;
}
.section-header.compact { margin-bottom: 0.55rem; align-items: flex-start; }
.rank-list {
  display: grid;
  gap: 0.55rem;
  margin: 0;
  padding: 0;
  list-style: none;
}
.rank-item { min-width: 0; }
.rank-click {
  display: grid;
  gap: 0.3rem;
  width: 100%;
  padding: 0.2rem 0;
  border: 0;
  background: transparent;
  text-align: left;
  cursor: pointer;
}
.rank-click:hover .rank-name { color: #2563eb; }
.rank-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}
.rank-name {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: #111827;
  font-size: 0.75rem;
  font-weight: 700;
}
.rank-count {
  flex: 0 0 auto;
  color: #6b7280;
  font-size: 0.68rem;
}
.rank-count.is-danger { color: #b91c1c; font-weight: 800; }
.rank-bar {
  display: block;
  height: 0.35rem;
  border-radius: 999px;
  background: #e5e7eb;
  overflow: hidden;
}
.rank-bar i {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: #2563eb;
}
.rank-bar i.danger { background: #dc2626; }
.rank-empty {
  padding: 1.1rem 0;
  color: #9ca3af;
  font-size: 0.75rem;
  text-align: center;
}
.metric-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 0.75rem;
}
.metric-card {
  border: 1px solid #e5e7eb;
  background: #ffffff;
  border-radius: 0.5rem;
  padding: 0.85rem 1rem;
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.05);
}
.metric-danger { border-color: #fecaca; background: #fff7f7; }
.metric-label { margin: 0; font-size: 0.75rem; color: #6b7280; }
.metric-value { margin: 0.35rem 0 0; font-size: 1.5rem; line-height: 1; font-weight: 800; color: #111827; }
.content-card,
.exception-panel {
  border: 1px solid #e5e7eb;
  border-radius: 0.5rem;
  background: #ffffff;
  padding: 1rem;
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.05);
}
.filter-card { padding-bottom: 0.85rem; }
.filter-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 0.75rem;
}
.filter-field {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 0.3rem;
  font-size: 0.75rem;
  color: #4b5563;
  font-weight: 700;
}
.filter-row,
.filter-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.65rem;
  margin-bottom: 0.75rem;
  align-items: center;
}
.filter-actions { margin: 0.85rem 0 0; }
.filter-select,
.filter-input,
.clean-textarea,
.clean-input {
  min-width: 0;
  width: 100%;
  padding: 0.4rem 0.55rem;
  font-size: 0.75rem;
  border: 1px solid #d1d5db;
  border-radius: 0.375rem;
  background: #ffffff;
  color: #111827;
}
.filter-row .filter-select,
.filter-row .filter-input { width: auto; min-width: 140px; }
.section-header { display: flex; align-items: center; justify-content: space-between; gap: 1rem; margin-bottom: 0.75rem; }
.section-title { margin: 0; font-size: 0.875rem; font-weight: 700; color: #111827; }
.section-meta { font-size: 0.75rem; color: #6b7280; }
.exception-panel { border-color: #fecaca; background: #fffafa; }
.exception-list { display: grid; gap: 0.5rem; }
.exception-item {
  display: grid;
  grid-template-columns: 11rem minmax(0, 1fr) minmax(9rem, 0.7fr);
  gap: 0.75rem;
  align-items: center;
  width: 100%;
  border: 1px solid #fee2e2;
  border-radius: 0.45rem;
  background: #ffffff;
  padding: 0.55rem 0.65rem;
  text-align: left;
  font-size: 0.75rem;
}
.exception-item:hover { border-color: #fca5a5; background: #fff7f7; }
.exception-time { color: #6b7280; }
.exception-main { color: #111827; font-weight: 700; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.exception-object { color: #4b5563; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.table-scroll {
  width: 100%;
  max-width: 100%;
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
}
.simple-table { width: 100%; min-width: 780px; border-collapse: collapse; font-size: 0.75rem; }
.business-table { min-width: 960px; }
.simple-table th,
.simple-table td { border: 1px solid #e2e8f0; padding: 0.45rem 0.55rem; text-align: left; vertical-align: top; }
.simple-table th { background: #f3f4f6; color: #374151; font-weight: 700; }
.simple-table td { background: #ffffff; color: #111827; }
.simple-table tbody tr:hover td { background: #f9fafb; }
.time-cell { width: 9.5rem; white-space: nowrap; }
.strong { color: #111827; font-weight: 700; }
.muted { margin-top: 0.18rem; color: #6b7280; font-size: 0.7rem; line-height: 1.35; }
.mono { font-family: var(--yb-font-data); }
.object-chip-list { display: flex; flex-wrap: wrap; gap: 0.3rem; max-width: 18rem; }
.object-chip {
  display: inline-flex;
  align-items: center;
  max-width: 100%;
  border-radius: 999px;
  border: 1px solid #dbeafe;
  background: #eff6ff;
  color: #1d4ed8;
  padding: 0.13rem 0.45rem;
  font-size: 0.68rem;
  font-weight: 700;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.status-badge,
.level-badge {
  display: inline-flex;
  align-items: center;
  padding: 0.15rem 0.45rem;
  border-radius: 0.35rem;
  font-size: 0.7rem;
  font-weight: 700;
}
.status-success { background: #ecfdf5; color: #047857; }
.status-failed { background: #fef2f2; color: #b91c1c; }
.status-neutral { background: #f3f4f6; color: #374151; }
.level-info { background: #eff6ff; color: #1d4ed8; }
.level-warn { background: #fffbeb; color: #b45309; }
.level-error { background: #fef2f2; color: #b91c1c; }
.location-cell,
.msg-cell,
.summary-cell {
  max-width: 300px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.summary-cell { max-width: 220px; }
.ref-cell { max-width: 140px; }
.json-cell-wrap { vertical-align: top; min-width: 12rem; max-width: 22rem; }
.json-cell-row { display: flex; align-items: flex-start; gap: 0.5rem; }
.json-cell-preview,
.json-body {
  border: 1px solid #e5e7eb;
  background: #f9fafb;
  color: #111827;
  border-radius: 0.5rem;
  font-family: var(--yb-font-data);
  white-space: pre-wrap;
  word-break: break-all;
}
.json-cell-preview {
  margin: 0;
  flex: 1;
  min-width: 0;
  max-height: 4.5rem;
  overflow: hidden;
  padding: 0.35rem;
  font-size: 0.65rem;
}
.json-body {
  margin: 0;
  padding: 0.75rem;
  max-height: 55dvh;
  overflow: auto;
  font-size: 0.75rem;
  line-height: 1.45;
}
.trace-detail { display: grid; gap: 1rem; }
.detail-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.75rem;
}
.detail-grid > div {
  min-width: 0;
  border: 1px solid #e5e7eb;
  border-radius: 0.45rem;
  background: #f9fafb;
  padding: 0.65rem;
}
.detail-label {
  display: block;
  margin-bottom: 0.25rem;
  font-size: 0.7rem;
  color: #6b7280;
  font-weight: 700;
}
.detail-grid strong {
  display: block;
  overflow-wrap: anywhere;
  font-size: 0.8rem;
  color: #111827;
}
.detail-block h4 {
  margin: 0 0 0.55rem;
  font-size: 0.8rem;
  font-weight: 700;
  color: #111827;
}
.pager { margin-top: 0.75rem; display: flex; align-items: center; gap: 0.5rem; }
.pager-btn { padding: 0.25rem 0.75rem; font-size: 0.75rem; border-radius: 9999px; border: 1px solid #d1d5db; background: #fff; color: #374151; }
.pager-btn:not(:disabled):hover { background: #f9fafb; color: #111827; }
.pager-btn:disabled { opacity: 0.5; cursor: not-allowed; }
.clean-form { display: flex; flex-direction: column; gap: 0.5rem; }
.clean-label { font-size: 0.875rem; font-weight: 500; color: #334155; }
.clean-label .required { color: #dc2626; }
.clean-textarea { min-height: 5rem; resize: vertical; }
.clean-input { max-width: 120px; }
.analysis-detail { min-height: 4rem; }
.analysis-detail-list { display: grid; gap: 0.5rem; }
.analysis-detail-item {
  display: grid;
  grid-template-columns: 10rem minmax(0, 1fr) auto;
  gap: 0.75rem;
  align-items: center;
  width: 100%;
  border: 1px solid #e5e7eb;
  border-radius: 0.45rem;
  background: #ffffff;
  padding: 0.6rem 0.7rem;
  text-align: left;
}
.analysis-detail-item:hover { background: #f9fafb; border-color: #cbd5e1; }
.analysis-detail-time {
  color: #6b7280;
  font-size: 0.72rem;
  white-space: nowrap;
}
.analysis-detail-item strong {
  display: block;
  color: #111827;
  font-size: 0.78rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.analysis-detail-item small {
  display: block;
  margin-top: 0.2rem;
  color: #6b7280;
  font-size: 0.7rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

@media (max-width: 960px) {
  .executive-metrics,
  .business-focus-grid,
  .rank-columns { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .executive-insight { grid-template-columns: 1fr; }
  .quick-actions { justify-content: flex-start; }
  .business-event-item { grid-template-columns: 1fr; align-items: flex-start; gap: 0.35rem; }
  .event-status { justify-content: flex-start; }
  .insight-board,
  .analysis-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .metric-grid,
  .filter-grid,
  .detail-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .exception-item { grid-template-columns: 1fr; gap: 0.25rem; }
  .analysis-detail-item { grid-template-columns: 1fr; align-items: flex-start; }
}

@media (max-width: 640px) {
  .page-header { flex-direction: column; align-items: stretch; }
  .tabs {
    display: grid;
    grid-auto-flow: column;
    grid-auto-columns: max-content;
  }
  .executive-metrics,
  .business-focus-grid,
  .rank-columns,
  .executive-filter-grid,
  .insight-board,
  .insight-side-grid,
  .analysis-grid,
  .metric-grid,
  .filter-grid,
  .detail-grid { grid-template-columns: 1fr; }
  .filter-row .filter-select,
  .filter-row .filter-input,
  .filter-actions :deep(button),
  .quick-actions :deep(button) {
    width: 100%;
    min-width: 0;
  }
  .section-header {
    align-items: flex-start;
    flex-direction: column;
    gap: 0.35rem;
  }
  .priority-item strong,
  .priority-item small,
  .event-main strong,
  .event-main small,
  .analysis-detail-item strong,
  .analysis-detail-item small {
    white-space: normal;
  }
}
</style>
