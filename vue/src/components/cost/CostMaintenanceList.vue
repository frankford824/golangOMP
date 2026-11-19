<template>
  <section class="maintenance" aria-label="编码成本维护" @keydown="navigateRows">
    <OAPageReturn v-if="returnTarget" :to="returnTarget" :source="returnSource">{{ oaReturnLabel(returnTarget) }}</OAPageReturn>
    <div class="maintenance-list">
    <div class="maintenance-tools">
      <form @submit.prevent="search">
        <button v-if="oaEnabled" class="conflict-filter" type="button" :aria-pressed="status === 'conflict'" @click="toggleConflicts">冲突待确认</button>
        <label class="sr-only" for="cost-keyword">搜索编码、名称或款式</label
        ><input
          id="cost-keyword"
          v-model.trim="keyword"
          placeholder="搜索编码 / 名称 / 款式"
        /><label class="sr-only" for="cost-review-state">确认状态</label>
        <select id="cost-review-state" v-model="reviewState" @change="search">
          <option value="pending">待确认</option>
          <option value="reviewed">已确认</option>
          <option value="all">全部编码</option>
        </select><label class="sr-only" for="cost-status">同步状态</label
        ><select id="cost-status" v-model="status" @change="search">
          <option value="">全部状态</option>
          <option
            v-for="(label, key) in maintenanceStatuses"
            :key="key"
            :value="key"
          >
            {{ label }}
          </option></select
        ><button type="submit">查询</button>
      </form>
      <div>
        <RouterLink v-if="canPrice" class="pricing-link bulk-pricing-link" :to="pricingLink()">批量核价</RouterLink>
        <button @click="openLogs">修改日志</button
        ><button :disabled="loading" @click="load()">
          {{ loading ? "刷新中…" : "刷新" }}
        </button>
      </div>
    </div>
    <p v-if="error" role="alert" class="maintenance-error">
      {{ error }} <button @click="load()">重新读取</button>
    </p>
    <p v-if="notice" role="status" class="maintenance-notice">{{ notice }}</p>
    <div ref="resultRegion" class="maintenance-table" role="region" @scroll="saveVisit" aria-label="编码成本列表" tabindex="0" :aria-busy="loading">
      <table>
        <thead>
          <tr>
            <th>编码与名称</th>
            <th>款式编码</th>
            <th>尺寸与面积</th>
            <th class="money">{{ oaEnabled ? "ERP 现价 / 系统成本" : "成本价（元）" }}</th>
            <th>状态</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in rows" :key="row.id" :data-cost-row="row.id" :class="{ 'selected-row': selected?.id === row.id }" @click="openRow($event, row)">
            <td>
              <RouterLink v-if="oaEnabled" class="cost-row-link" :to="costDetailLink(row)" @click.exact.prevent="inspect(row)"><strong>{{ row.sku_code }}</strong></RouterLink>
              <strong v-else>{{ row.sku_code }}</strong><span :title="row.product_name">{{ row.product_name || "名称待补充" }}</span>
            </td>
            <td>{{ row.style_code || "待绑定" }}</td>
            <td>
              <div v-if="oaEnabled" class="cost-cell-stack">
                <button
                  v-if="needsDimensions(row) && canEdit && busyID === null && row.revision >= 1"
                  type="button"
                  class="missing-dimensions"
                  @click="editDimensions(row)"
                >{{ dimensionActionLabel(row) }}</button>
                <span v-else-if="needsDimensions(row)" class="missing-dimensions">{{ dimensionActionLabel(row) }}</span>
                <span v-else>{{ dimensions(row) }}</span>
                <small v-if="row.area_m2 != null">{{ area(row.area_m2) }} ㎡</small>
              </div>
              <template v-else>
                <span>{{ dimensions(row) }}</span><small>{{ area(row.area_m2) }} ㎡</small>
              </template>
            </td>
            <td class="money">
              <template v-if="oaEnabled">
                <span>ERP {{ observedPrice(row.erp_cost) }}</span>
                <small>系统 {{ observedPrice(row.cost_price, '待定') }}</small>
              </template>
              <template v-else>{{ money(row.cost_price) }}</template>
            </td>
            <td>
              <div :class="{ 'cost-cell-stack': oaEnabled }">
              <button v-if="oaEnabled && row.status === 'conflict'" type="button" class="conflict-filter" @click="inspect(row)">冲突待确认</button>
              <span v-else :class="['maintenance-state', row.status]">{{
                maintenanceStatuses[row.status] || "待核对"
              }}</span
              ><small v-if="!oaEnabled && row.status === 'conflict'"
                >ERP {{ money(row.erp_cost) }} 元</small
              >
              <small
                v-if="row.base_sync_required"
                :title="row.base_sync_error || '名称与规格资料正在同步'"
                >{{
                  row.base_sync_error ? "资料同步失败" : "资料待同步"
                }}</small
              >
              </div>
            </td>
            <td class="row-actions"><div class="row-action-group">
              <span
                v-if="row.reviewed && row.status !== 'conflict'"
                class="maintenance-reviewed"
                >已确认</span
              >
              <span
                v-else-if="oaEnabled && confirmBlocker(row)"
                class="blocked-confirm"
                tabindex="0"
                :title="confirmBlocker(row)"
                :aria-label="`确认不可用：${confirmBlocker(row)}`"
              >
                <button disabled>{{ busyID === row.id ? "提交中…" : "确认" }}</button>
                <small>{{ confirmBlocker(row) }}</small>
              </span>
              <button v-else :disabled="Boolean(confirmBlocker(row))" @click="confirm(row)">
                {{ busyID === row.id ? "提交中…" : "确认" }}
              </button>
              <button
                :disabled="!canEdit || busyID !== null || row.revision < 1"
                class="accent"
                @click="edit(row)"
              >
                修改
              </button>
              <RouterLink v-if="canPrice" class="pricing-link" :to="pricingLink(row.sku_code)" :aria-label="'核价 ' + row.sku_code">核价</RouterLink>
            </div></td>
          </tr>
          <tr v-if="!rows.length">
            <td colspan="6" class="empty">
              <OAEmpty v-if="oaEnabled && !loading && !error" kind="search" :title="reviewState === 'pending' ? '当前没有待确认的编码' : '没有匹配的已完成编码'" description="可切换确认状态或调整编码重新查询。" />
              <template v-else>{{
                loading
                  ? "正在读取已完成编码…"
                  : error
                    ? "暂未取得数据，请重试。"
                  : reviewState === 'pending' ? "当前没有待确认的编码，可切换筛选查看已确认记录。" : "没有匹配的已完成编码。"
              }}</template>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <DesktopSlot v-if="oaEnabled && refreshed" target="status">
      <span class="cost-refresh-status" :title="updatedLabel"><span aria-hidden="true" />{{ refreshTime }}</span>
    </DesktopSlot>
    <footer class="maintenance-footer">
      <span
        >{{ updatedLabel }}
        <small v-if="editing">编辑期间暂停刷新</small></span
      >
      <div>
        <label
          >每页
          <select v-model.number="pageSize" @change="search">
            <option :value="20">20 条</option>
            <option :value="50">50 条</option>
            <option :value="100">100 条</option>
          </select></label
        ><button :disabled="cursors.length <= 1 || loading" @click="previous">
          <ChevronLeft :size="16" :stroke-width="1.75" aria-hidden="true" />上一页</button
        ><span>第 {{ cursors.length }} 页</span
        ><button :disabled="!nextID || loading" @click="next"><ChevronRight :size="16" :stroke-width="1.75" aria-hidden="true" />下一页</button>
      </div>
    </footer>
    <p v-if="!canEdit" class="permission-note">
      查看模式；确认与修改需要成本维护和 ERP 管理权限。
    </p>
    </div>
    <Teleport to="body" :disabled="inlineInspector">
      <div
        v-if="selected"
        class="maintenance-overlay cost-inspector"
        :class="{ 'inline-inspector': inlineInspector }"
        @keydown="!inlineInspector && navigateRows($event)"
      >
        <button
          v-if="!inlineInspector"
          class="maintenance-backdrop"
          aria-label="关闭修改侧栏"
          :disabled="saving"
          @click="closeEditor()"
        />
        <form
          ref="editorElement"
          class="maintenance-drawer"
          role="dialog"
          :aria-modal="inlineInspector ? undefined : true"
          aria-labelledby="maintenance-edit-title"
          tabindex="-1"
          @submit.prevent="save"
          @keydown.tab="!inlineInspector && trapFocus($event)"
        >
          <header>
            <div>
              <h2 id="maintenance-edit-title">{{ editing ? "修改编码" : "编码详情" }}</h2>
              <RouterLink v-if="oaEnabled" :to="costDetailLink(selected)" target="_blank" rel="noopener" :title="'新标签页打开完整详情'">{{ selected.sku_code }} ↗</RouterLink>
              <p v-else>{{ selected.sku_code }}</p>
            </div>
            <button
              type="button"
              aria-label="关闭"
              class="maintenance-close"
              :disabled="saving"
              @click="closeEditor()"
            >
              <X :size="18" :stroke-width="1.75" aria-hidden="true" />
            </button>
          </header>
          <div v-if="!editing" class="drawer-body">
            <dl class="cost-inspect-fields">
              <dt>产品名称</dt><dd>{{ selected.product_name || "—" }}</dd>
              <dt>款式编码</dt><dd>{{ selected.style_code || "—" }}</dd>
              <dt>尺寸与面积</dt><dd>{{ dimensions(selected) }} · {{ area(selected.area_m2) }} ㎡</dd>
              <dt>系统成本</dt><dd>{{ observedPrice(selected.cost_price, "待定") }}</dd>
              <dt>ERP 现价</dt><dd>{{ observedPrice(selected.erp_cost) }}</dd>
              <dt>同步状态</dt><dd>{{ maintenanceStatuses[selected.status] || "待核对" }}</dd>
            </dl>
          </div>
          <div v-else class="drawer-body">
            <p v-if="editError" class="maintenance-error" role="alert">
              {{ editError }}
            </p>
            <label
              >产品名称<input
                v-model.trim="draft.product_name"
                required
                maxlength="40"
            /></label>
            <label
              >款式编码<input
                v-model.trim="draft.style_code"
                required
                maxlength="128"
              /><small
                >仅调整本编码的款式归属；款式需已绑定计价方案。</small
              ></label
            >
            <template v-if="!selected.cost_input"
              ><div class="dimension-fields">
                <label
                  >宽（cm）<input
                    ref="widthInput"
                    v-model.number="draft.width_cm"
                    type="number"
                    min="0.0001"
                    step="any" /></label
                ><label
                  >高（cm）<input
                    v-model.number="draft.height_cm"
                    type="number"
                    min="0.0001"
                    step="any" /></label
                ><label
                  >面积（㎡）<input
                    v-model.number="draft.area_m2"
                    type="number"
                    min="0.000001"
                    step="any"
                    @input="autoArea = false"
                /></label>
              </div>
              <label class="area-option"
                ><input
                  v-model="autoArea"
                  type="checkbox"
                  @change="deriveArea"
                />按宽 × 高计算面积</label
              ></template
            >
            <details v-else>
              <summary>
                计价规格 · {{ area(selected.area_m2) }} ㎡（点击调整）
              </summary>
              <CostInputFields v-model="structuredInput" />
            </details>
            <label
              >成本价（元）<input
                v-model.number="draft.cost_price"
                @keydown.enter.prevent="oaEnabled ? requestNextSave() : editorElement?.requestSubmit()"
                required
                type="number"
                min="0"
                max="1000000000"
                step="0.0001"
            /></label>
            <p
              v-if="
                selected.status === 'conflict' && selected.erp_cost !== null
              "
              class="erp-price-choice"
            >
              ERP 观察价：{{ money(selected.erp_cost) }} 元
              <button
                type="button"
                @click="
                  draft.cost_price = selected.erp_cost;
                  reason = 'cost_incorrect';
                  reasonDetail = '核对后采用ERP成本';
                "
              >
                采用 ERP 价
              </button>
            </p>
            <fieldset class="reason-options">
              <legend>修改原因</legend>
              <label v-for="(label, key) in maintenanceReasons" :key="key"
                ><input
                  v-model="reason"
                  type="radio"
                  name="maintenance-reason"
                  :value="key"
                  required
                />{{ label }}</label
              >
            </fieldset>
            <label
              >{{ reason === "other" ? "补充说明（必填）" : "补充说明（选填）"
              }}<textarea
                v-model.trim="reasonDetail"
                :required="reason === 'other'"
                maxlength="200"
                rows="2"
              />
            </label>
            <section
              v-if="changes.length"
              class="change-preview"
              aria-label="变更内容"
            >
              <h3>本次变更</h3>
              <p v-for="change in changes" :key="change.label">
                <span>{{ change.label }}</span
                ><del>{{ change.before }}</del
                ><b>→ {{ change.after }}</b>
              </p>
            </section>
            <p v-else class="permission-note">
              修改需要至少变更一项；仅核对价格请使用列表中的“确认”。
            </p>
          </div>
          <footer v-if="!editing">
            <button v-if="!selected.reviewed || selected.status === 'conflict'" type="button" :disabled="Boolean(confirmBlocker(selected))" :title="confirmBlocker(selected)" @click="confirm(selected)">确认</button>
            <button type="button" class="primary" :disabled="!canEdit || busyID !== null || selected.revision < 1" @click="edit(selected)">修改</button>
          </footer>
          <footer v-else>
            <button type="button" :disabled="saving" @click="closeEditor()">
              取消</button
            ><button
              type="submit"
              :class="oaEnabled ? 'secondary' : 'primary'"
              :disabled="saving || !changes.length || !reason || stale"
            >
              {{ saving ? "保存中…" : "保存并同步" }}
            </button>
            <button v-if="oaEnabled" ref="saveNextButton" type="submit" class="primary" data-next :disabled="saving || !changes.length || !reason || stale">保存并下一条</button>
          </footer>
        </form>
      </div>
    </Teleport>
    <Teleport to="body">
      <div
        v-if="logsOpen"
        class="maintenance-overlay"
        @keydown.esc="logsOpen = false"
      >
        <button
          class="maintenance-backdrop"
          aria-label="关闭修改日志"
          @click="logsOpen = false"
        />
        <section
          class="maintenance-drawer logs-drawer"
          role="dialog"
          aria-modal="true"
          aria-labelledby="maintenance-logs-title"
        >
          <header>
            <div>
              <h2 id="maintenance-logs-title">维护日志</h2>
              <p>确认与修改均留痕，支持按原因筛选</p>
            </div>
            <button class="maintenance-close" aria-label="关闭日志" @click="logsOpen = false"><X :size="18" :stroke-width="1.75" aria-hidden="true" /></button>
          </header>
          <div class="drawer-body">
            <form
              class="log-search"
              @submit.prevent="
                logPage = 1;
                loadLogs();
              "
            >
              <input
                v-model.trim="logKeyword"
                placeholder="编码前缀"
                aria-label="日志编码"
              /><select
                v-model="logReason"
                aria-label="日志原因"
                @change="
                  logPage = 1;
                  loadLogs();
                "
              >
                <option value="">全部原因（含确认）</option>
                <option
                  v-for="(label, key) in maintenanceReasons"
                  :key="key"
                  :value="key"
                >
                  {{ label }}
                </option></select
              ><button><Search :size="16" :stroke-width="1.75" aria-hidden="true" />查询</button>
            </form>
            <p v-if="logError" role="alert" class="maintenance-error">
              {{ logError }}
            </p>
            <p v-if="logsLoading">正在读取…</p>
            <article
              v-for="entry in logs"
              :key="entry.id"
              class="maintenance-log"
            >
              <header>
                <strong>{{ entry.payload.sku_code }}</strong
                ><small
                  >{{ date(entry.created_at) }} ·
                  {{
                    entry.operator_name || `用户 #${entry.operator_id}`
                  }}</small
                >
              </header>
              <p>
                {{
                  entry.payload.action === "confirm"
                    ? "确认当前成本"
                    : maintenanceReasons[entry.payload.reason_code] ||
                      entry.payload.reason_code
                }}<template v-if="entry.payload.reason_detail">
                  · {{ entry.payload.reason_detail }}</template
                >
              </p>
              <p
                v-for="change in logChanges(entry)"
                :key="change.label"
                class="log-change"
              >
                {{ change.label }}：{{ change.before }} → {{ change.after }}
              </p>
            </article>
            <p v-if="!logsLoading && !logs.length">没有匹配记录。</p>
          </div>
          <footer>
            <span>共 {{ logTotal }} 条</span
            ><button
              :disabled="logPage <= 1 || logsLoading"
              @click="
                logPage--;
                loadLogs();
              "
            >
              <ChevronLeft :size="16" :stroke-width="1.75" aria-hidden="true" />上一页</button
            ><button
              :disabled="logPage * 50 >= logTotal || logsLoading"
              @click="
                logPage++;
                loadLogs();
              "
            >
              <ChevronRight :size="16" :stroke-width="1.75" aria-hidden="true" />下一页
            </button>
          </footer>
        </section>
      </div>
    </Teleport>
  </section>
</template>

<script setup lang="ts">
import DesktopSlot from '@/oa/components/DesktopSlot.vue';
import OAEmpty from '@/oa/components/OAEmpty.vue';
import { oaReturnLabel } from '@/oa/pageTitle'
import OAPageReturn from '@/oa/components/OAPageReturn.vue';
import { orderReturnQuery, orderReturnSource, orderReturnTarget, orderVisit } from '@/oa/orderFilterMemory';
import { RouterLink, useRoute, useRouter } from "vue-router";
import { oaEnabled } from "@/oa/access";
import { retryCostRead } from '@/utils/cost-read-retry';
import {
  computed,
  nextTick,
  onActivated,
  onBeforeUnmount,
  onDeactivated,
  onMounted,
  watch,
  reactive,
  ref,
} from "vue";
import { Search, X, ChevronLeft, ChevronRight } from "lucide-vue-next";
import { usePermission } from "@/composables/usePermission";
import CostInputFields from "./CostInputFields.vue";
import { emptyCostInput } from "@/domain/cost-model";
import {
  costMaintenanceApi,
  maintenanceReasons,
  maintenanceStatuses,
  type MaintenanceRow,
  type MaintenanceRequest,
  type MaintenanceLog,
} from "@/services/api/costMaintenanceApi";
import "./cost-maintenance.css";
const permission = usePermission(),
  canPrice = computed(() => oaEnabled && permission.can("catalog.view")),
  canEdit = computed(
    () => permission.can("catalog.manage") && permission.can("erp.manage"),
  );
const props = defineProps<{ initialSku?: string; focus?: "cost-conflicts" | "cost-detail"; focusRevision?: string }>();
const route = useRoute();
const router = useRouter();
const returnTarget = computed(() => oaEnabled ? orderReturnTarget(route) : undefined);
const returnSource = computed(() => orderReturnSource(route));
const resultRegion = ref<HTMLElement>();
type Visit = { keyword: string; status: string; review: 'pending' | 'reviewed' | 'all'; pageSize: number; cursors: number[]; row?: number; scroll: number; used: boolean };
let visit = oaEnabled ? orderVisit<Visit>('cost-maintenance') : undefined;
let pendingVisit = visit?.read();
let visitActive = true;
let appliedVisit: Pick<Visit,'keyword'|'status'|'review'|'pageSize'|'cursors'> | undefined;
function saveVisit() {
  if (!visitActive || !visit || pendingVisit || !appliedVisit) return;
  visit.save({ ...appliedVisit, row: selected.value?.id, scroll: resultRegion.value?.scrollTop ?? 0, used: usedInspector });
}
function pricingLink(sku?: string) {
  const query = Object.fromEntries(Object.entries(route.query).filter(([key, value]) =>
    ['tab','sku','focus','returnTo','returnFallback'].includes(key) && typeof value === 'string')) as Record<string, string>;
  if (appliedVisit?.keyword) query.sku = appliedVisit.keyword;
  return { name: 'OperationsPricing', query: { ...(sku ? { sku } : {}),
    ...orderReturnQuery(router, route.path, query, route.fullPath) } };
}
const rows = ref<MaintenanceRow[]>([]),
  keyword = ref(pendingVisit?.keyword ?? props.initialSku ?? ""),
  status = ref(pendingVisit?.status ?? (props.focus === "cost-conflicts" ? "conflict" : "")),
  reviewState = ref<"pending" | "reviewed" | "all">(pendingVisit?.review ?? (props.initialSku && props.focus !== "cost-conflicts" ? "all" : "pending")),
  pageSize = ref(pendingVisit?.pageSize ?? 50),
  cursors = ref(pendingVisit ? [...pendingVisit.cursors] : [0]),
  nextID = ref(0),
  loading = ref(false),
  error = ref(""),
  notice = ref(""),
  refreshed = ref(""),
  busyID = ref<number | null>(null);
const selected = ref<MaintenanceRow | null>(null),
  saving = ref(false),
  editError = ref(""),
  stale = ref(false),
  draft = reactive<Record<string, string | number>>({}),
  reason = ref(""),
  reasonDetail = ref(""),
  autoArea = ref(false),
  structuredInput = ref(emptyCostInput()),
  editorElement = ref<HTMLFormElement>(),
  widthInput = ref<HTMLInputElement>();
const inlineInspector = ref(false);
const editing = ref(false);
const saveNextButton = ref<HTMLButtonElement>();
let inspectorMedia: MediaQueryList | undefined;
let defaultMedia: MediaQueryList | undefined;
let usedInspector = pendingVisit?.used ?? false;
watch(selected, saveVisit, { flush: "post" });
const costDetailLink = (row: MaintenanceRow) => ({ name: "CostRules", query: { sku: row.sku_code, focus: "cost-detail" } });
function syncInspectorSize() {
  inlineInspector.value = Boolean(inspectorMedia?.matches);
  if (!defaultMedia?.matches && !usedInspector && !editing.value) selected.value = null;
  openDefaultInspector();
}
function openDefaultInspector() {
  if (usedInspector || pendingVisit) return;
  if (props.focus === "cost-detail" && props.initialSku) {
    const row = rows.value.find(item => item.sku_code === props.initialSku);
    if (row) inspect(row, true);
  } else if (defaultMedia?.matches && rows.value[0]) {
    inspect(rows.value[0], false);
  }
}
function inspect(row: MaintenanceRow, userInitiated = true) {
  if (saving.value || busyID.value !== null || (editing.value && selected.value?.id === row.id) || !maySwitch(row)) return;
  if (userInitiated) {
    usedInspector = true;
    if (!selected.value) focusBefore = document.activeElement as HTMLElement;
  }
  editing.value = false;
  selected.value = row;
  if (userInitiated) void nextTick(() => editorElement.value?.focus());
}
function openRow(event: MouseEvent, row: MaintenanceRow) {
  if (oaEnabled && !(event.target as HTMLElement).closest("a,button,input,select,[role=button]")) inspect(row);
}
function maySwitch(row?: MaintenanceRow) {
  return !oaEnabled || !editing.value || !changes.value.length || row?.id === selected.value?.id ||
    window.confirm("当前修改尚未保存。放弃修改？");
}
function navigateRows(event: KeyboardEvent) {
  if (!selected.value || logsOpen.value || event.defaultPrevented || event.altKey || event.ctrlKey || event.metaKey) return;
  if (event.key === "Escape") {
    event.preventDefault();
    closeEditor();
    return;
  }
  if (!oaEnabled || saving.value || (event.target as HTMLElement).closest("input,select,textarea,[contenteditable=true]")) return;
  const delta = event.key === "ArrowUp" ? -1 : event.key === "ArrowDown" ? 1 : 0;
  if (!delta) return;
  event.preventDefault();
  const index = rows.value.findIndex(row => row.id === selected.value?.id);
  const target = rows.value[index + delta];
  if (target) {
    inspect(target);
    if (selected.value?.id === target.id) scrollToRow(target.id);
  }
}
function scrollToRow(id: number) {
  void nextTick(() => document.querySelector(`[data-cost-row="${id}"]`)?.scrollIntoView({ block: "nearest" }));
}
function requestNextSave() {
  if (saveNextButton.value && !saveNextButton.value.disabled) editorElement.value?.requestSubmit(saveNextButton.value);
}
const logsOpen = ref(false),
  logs = ref<MaintenanceLog[]>([]),
  logKeyword = ref(""),
  logReason = ref(""),
  logPage = ref(1),
  logTotal = ref(0),
  logsLoading = ref(false),
  logError = ref("");
let epoch = 0,
  controller: AbortController | undefined,
  timer: ReturnType<typeof setInterval> | undefined,
  focusBefore: HTMLElement | null = null;
let savedBodyStyle = "",
  savedScroll = 0;
watch(
  () => (!!selected.value && !inlineInspector.value) || logsOpen.value,
  (open) => {
    if (open) {
      savedBodyStyle = document.body.style.cssText;
      savedScroll = window.scrollY;
      Object.assign(document.body.style, {
        overflow: "hidden",
        position: "fixed",
        top: `-${savedScroll}px`,
        width: "100%",
      });
    } else {
      document.body.style.cssText = savedBodyStyle;
      window.scrollTo(0, savedScroll);
    }
  },
);
const money = (n: number | null) =>
  n == null
    ? "待确认"
    : Number(n)
        .toFixed(4)
        .replace(/\.?0+$/, "") || "0";
const oaPriceFormat = new Intl.NumberFormat("zh-CN", { minimumFractionDigits: 2, maximumFractionDigits: 4 });
function observedPrice(value: number | null, empty = "—"): string {
  return typeof value === "number" && Number.isFinite(value) ? `¥${oaPriceFormat.format(value)}` : empty;
}
const area = (n: number | null) =>
  n == null
    ? "待补充"
    : Number(n)
        .toFixed(4)
        .replace(/\.?0+$/, "") || "0";
const dimensions = (r: MaintenanceRow) =>
  r.width_cm && r.height_cm
    ? `${r.width_cm} × ${r.height_cm} cm`
    : r.cost_input?.area_mode === "faces"
      ? "多面用料"
      : r.cost_input?.area_mode === "total"
        ? "整套总面积"
        : r.size_text || "尺寸待补充";
function needsDimensions(row: MaintenanceRow) {
  return !(row.width_cm && row.height_cm) && !row.size_text?.trim() &&
    row.cost_input?.area_mode !== "faces" && row.cost_input?.area_mode !== "total";
}
function dimensionActionLabel(row: MaintenanceRow) {
  return row.area_m2 == null ? "待补尺寸" : "待补长宽";
}
function confirmBlocker(row: MaintenanceRow): string {
  if (!canEdit.value) return "需要成本维护和 ERP 管理权限";
  if (busyID.value !== null || saving.value) return "请等待当前提交完成";
  if (row.revision < 1) return "当前版本不可维护，请刷新核对";
  if (row.cost_price === null) return needsDimensions(row)
    ? "当前无系统成本，请核对尺寸与计价方案"
    : "当前无系统成本，请核对计价方案";
  return "";
}
function toggleConflicts() {
  status.value = status.value === "conflict" ? "" : "conflict";
  if (status.value === "conflict") reviewState.value = "pending";
  search();
}
const date = (value: string) =>
  new Date(value).toLocaleString("zh-CN", {
    timeZone: "Asia/Shanghai",
    hour12: false,
  });
const refreshTime = computed(() => refreshed.value ? `${new Date(refreshed.value).toLocaleTimeString("zh-CN", { timeZone: "Asia/Shanghai", hour: "2-digit", minute: "2-digit", hour12: false })} 已更新` : "");
const updatedLabel = computed(() =>
  refreshed.value ? `更新于 ${date(refreshed.value)}（北京时间）` : "",
);
async function load(background = false) {
  if (
    background &&
    ((selected.value && (!inlineInspector.value || editing.value)) ||
      logsOpen.value ||
      busyID.value !== null ||
      document.visibilityState === "hidden")
  )
    return;
  const ticket = ++epoch;
  controller?.abort();
  controller = new AbortController();
  const signal = controller.signal;
  loading.value = true;
  error.value = "";
  try {
    const params = { keyword: keyword.value, status: status.value, review_state: reviewState.value,
      before_id: cursors.value.at(-1), page_size: pageSize.value };
    const result = await retryCostRead(() => costMaintenanceApi.list(params, signal), signal);
    if (ticket !== epoch) return;
    rows.value = result.data || [];
    nextID.value = result.next_before_id;
    refreshed.value = result.refreshed_at;
    if (selected.value && !editing.value) selected.value = rows.value.find(row => row.id === selected.value?.id) ?? null;
    appliedVisit = { keyword: params.keyword, status: params.status, review: params.review_state,
      pageSize: params.page_size, cursors: [...cursors.value] };
    if (pendingVisit) {
      const saved = pendingVisit;
      const row = rows.value.find(item => item.id === saved.row);
      pendingVisit = undefined;
      if (row) inspect(row, false);
      await nextTick();
      if (resultRegion.value) resultRegion.value.scrollTop = saved.scroll;
    } else openDefaultInspector();
    saveVisit();
  } catch (e) {
    if (ticket === epoch && !controller?.signal.aborted)
      error.value = e instanceof Error ? e.message : "读取失败";
  } finally {
    if (ticket === epoch) loading.value = false;
  }
}
function search() {
  cursors.value = [0];
  notice.value = "";
  load();
}
watch([() => props.initialSku, () => props.focus, () => props.focusRevision], ([sku, focus], [previousSku, previousFocus]) => {
  if (!visitActive || (!sku && !previousSku && !focus && !previousFocus)) return;
  if (oaEnabled) {
    visit = orderVisit<Visit>('cost-maintenance');
    pendingVisit = visit.read();
    if (pendingVisit) {
      keyword.value = pendingVisit.keyword;
      status.value = pendingVisit.status;
      reviewState.value = pendingVisit.review;
      pageSize.value = pendingVisit.pageSize;
      cursors.value = [...pendingVisit.cursors];
      usedInspector = pendingVisit.used;
      void load();
      return;
    }
  }
  if (focus === "cost-detail") usedInspector = false;
  if (focus === "cost-conflicts") {
    keyword.value = "";
    status.value = "conflict";
    reviewState.value = "pending";
    search();
    return;
  }
  keyword.value = sku ?? "";
  status.value = "";
  reviewState.value = sku ? "all" : "pending";
  search();
});
function next() {
  if (nextID.value) {
    cursors.value.push(nextID.value);
    load();
  }
}
function previous() {
  cursors.value.pop();
  load();
}
async function confirm(row: MaintenanceRow) {
  if (confirmBlocker(row)) return;
  busyID.value = row.id;
  error.value = "";
  notice.value = "";
  try {
    const updated = await costMaintenanceApi.maintain(row.id, {
      action: "confirm",
      token: row.token,
      revision: row.revision,
      erp_cost: row.erp_cost,
    });
    applyCommittedRow(updated);
    notice.value = `${row.sku_code} 已确认，系统已生效；ERP 状态以回读结果为准。`;
    await load();
  } catch (e) {
    error.value = e instanceof Error ? e.message : "确认失败，请刷新核对";
  } finally {
    busyID.value = null;
  }
}
function applyCommittedRow(updated: MaintenanceRow) {
  const reviewed = updated.reviewed && updated.status !== "conflict";
  const matches = reviewState.value === "all" || (reviewState.value === "reviewed" ? reviewed : !reviewed);
  rows.value = rows.value.flatMap((current) => current.id === updated.id ? (matches ? [updated] : []) : [current]);
}
function edit(row: MaintenanceRow) {
  if (saving.value || busyID.value !== null || !maySwitch(row) || (editing.value && selected.value?.id === row.id)) return;
  usedInspector = true;
  if (!selected.value) focusBefore = document.activeElement as HTMLElement;
  editing.value = true;
  selected.value = JSON.parse(JSON.stringify(row));
  Object.assign(draft, {
    product_name: row.product_name,
    style_code: row.style_code,
    width_cm: row.width_cm ?? "",
    height_cm: row.height_cm ?? "",
    area_m2: row.area_m2 ?? "",
    cost_price: row.cost_price ?? "",
  });
  structuredInput.value = JSON.parse(
    JSON.stringify(row.cost_input || emptyCostInput()),
  );
  autoArea.value = !!(
    row.width_cm &&
    row.height_cm &&
    row.area_m2 &&
    Math.abs((row.width_cm * row.height_cm) / 10000 - row.area_m2) < 0.00001
  );
  reason.value = "";
  reasonDetail.value = "";
  editError.value = "";
  stale.value = false;
  nextTick(() => editorElement.value?.focus());
}
function editDimensions(row: MaintenanceRow) {
  edit(row);
  if (!row.cost_input && row.area_m2 == null) autoArea.value = true;
  void nextTick(() => widthInput.value?.focus());
}
function closeEditor(discard = false) {
  if (!saving.value && (discard || maySwitch())) {
    usedInspector = true;
    editing.value = false;
    selected.value = null;
    nextTick(() => focusBefore?.focus());
  }
}
function trapFocus(e: KeyboardEvent) {
  const els = Array.from(
    editorElement.value?.querySelectorAll<HTMLElement>(
      "a[href],button:not(:disabled),input:not(:disabled),select,textarea,summary",
    ) || [],
  );
  if (!els.length) return;
  const first = els[0],
    last = els.at(-1);
  if (
    e.shiftKey &&
    (document.activeElement === first ||
      document.activeElement === editorElement.value)
  ) {
    e.preventDefault();
    last?.focus();
  } else if (!e.shiftKey && document.activeElement === last) {
    e.preventDefault();
    first?.focus();
  }
}
watch(() => [draft.width_cm, draft.height_cm], deriveArea);
function deriveArea() {
  if (
    autoArea.value &&
    Number(draft.width_cm) > 0 &&
    Number(draft.height_cm) > 0
  )
    draft.area_m2 = Number(
      ((Number(draft.width_cm) * Number(draft.height_cm)) / 10000).toFixed(6),
    );
}
const labels: Record<string, string> = {
  product_name: "名称",
  style_code: "款式编码",
  width_cm: "宽（cm）",
  height_cm: "高（cm）",
  area_m2: "面积（㎡）",
  cost_price: "成本价（元）",
};
function changedFields(before: MaintenanceRow, after: Partial<MaintenanceRow>) {
  return Object.entries(labels)
    .filter(([k]) => {
      const key = k as keyof MaintenanceRow;
      return (
        after[key] !== undefined &&
        String(before[key] ?? "") !== String(after[key] ?? "")
      );
    })
    .map(([k, label]) => ({
      label,
      before: String(before[k as keyof MaintenanceRow] ?? "未填写"),
      after: String(after[k as keyof MaintenanceRow] ?? "未填写"),
    }));
}
function modification(): Partial<MaintenanceRequest> {
  if (!selected.value) return {};
  const out: Record<string, unknown> = {};
  for (const key of Object.keys(labels)) {
    const old = selected.value[key as keyof MaintenanceRow],
      value = draft[key];
    if (value !== "" && String(old ?? "") !== String(value)) out[key] = value;
  }
  if (selected.value.cost_input) {
    delete out.width_cm;
    delete out.height_cm;
    delete out.area_m2;
    if (
      JSON.stringify(selected.value.cost_input) !==
      JSON.stringify(structuredInput.value)
    )
      out.cost_input = structuredInput.value;
  }
  return out;
}
const changes = computed(() => {
  if (!selected.value) return [];
  const payload = modification();
  const list = changedFields(
    selected.value,
    payload as Partial<MaintenanceRow>,
  );
  if (payload.cost_input)
    list.push({
      label: "计价规格",
      before: "原用料规格",
      after: "已调整（展开核对）",
    });
  return list;
});
async function save(event?: SubmitEvent) {
  if (!selected.value || !changes.value.length || !canEdit.value || saving.value || !reason.value || stale.value) return;
  const advance = (event?.submitter as HTMLElement | null)?.hasAttribute("data-next");
  const index = rows.value.findIndex(row => row.id === selected.value?.id);
  const following = rows.value.slice(index + 1).map(row => row.id);
  saving.value = true;
  editError.value = "";
  try {
    const row = selected.value;
    const updated = await costMaintenanceApi.maintain(row.id, {
      action: "modify",
      token: row.token,
      revision: row.revision,
      erp_cost: row.erp_cost,
      reason_code: reason.value,
      reason_detail: reasonDetail.value,
      ...modification(),
    });
    applyCommittedRow(updated);
    notice.value = `${row.sku_code} 已保存并记录日志，等待 ERP 同步核对。`;
    saving.value = false;
    closeEditor(true);
    await load();
    if (advance && !error.value) {
      const actionable = () => following.map(id => rows.value.find(item => item.id === id)).find(item => item && item.revision >= 1 && (!item.reviewed || item.status === "conflict"));
      let target = actionable();
      if (!target && nextID.value) {
        cursors.value.push(nextID.value);
        await load();
        if (!error.value) target = rows.value.find(item => item.revision >= 1 && (!item.reviewed || item.status === "conflict"));
      }
      if (target) {
        edit(target);
        scrollToRow(target.id);
      }
    }
  } catch (e) {
    editError.value = e instanceof Error ? e.message : "保存失败";
    stale.value = (e as { status?: number })?.status === 409;
  } finally {
    saving.value = false;
  }
}
function openLogs() {
  logsOpen.value = true;
  logKeyword.value = "";
  logPage.value = 1;
  loadLogs();
}
async function loadLogs() {
  logsLoading.value = true;
  logError.value = "";
  try {
    const result = await costMaintenanceApi.logs({
      keyword: logKeyword.value,
      reason_code: logReason.value,
      page: logPage.value,
      page_size: 50,
    });
    logs.value = result.data;
    logTotal.value = result.pagination.total;
  } catch (e) {
    logError.value = e instanceof Error ? e.message : "日志读取失败";
  } finally {
    logsLoading.value = false;
  }
}
function logChanges(log: MaintenanceLog) {
  const list = changedFields(log.payload.before, log.payload.after);
  if (
    JSON.stringify(log.payload.before.cost_input) !==
    JSON.stringify(log.payload.after.cost_input)
  )
    list.push({
      label: "计价规格",
      before: JSON.stringify(log.payload.before.cost_input),
      after: JSON.stringify(log.payload.after.cost_input),
    });
  return list;
}
onMounted(() => {
  if (oaEnabled && typeof window.matchMedia === "function") {
    inspectorMedia = window.matchMedia("(min-width: 1920px)");
    defaultMedia = window.matchMedia("(min-width: 2300px)");
    inspectorMedia.addEventListener("change", syncInspectorSize);
    defaultMedia.addEventListener("change", syncInspectorSize);
    syncInspectorSize();
  }
  load();
  timer = setInterval(() => {
    if (visitActive && !loading.value) load(true);
  }, 15000);
});
onActivated(() => {
  if (!oaEnabled || visitActive) return;
  visitActive = true;
  visit = orderVisit<Visit>('cost-maintenance');
  pendingVisit = visit.read();
  if (pendingVisit) {
    keyword.value = pendingVisit.keyword;
    status.value = pendingVisit.status;
    reviewState.value = pendingVisit.review;
    pageSize.value = pendingVisit.pageSize;
    cursors.value = [...pendingVisit.cursors];
    usedInspector = pendingVisit.used;
    void load();
  } else {
    keyword.value = props.focus === 'cost-conflicts' ? '' : props.initialSku ?? '';
    status.value = props.focus === 'cost-conflicts' ? 'conflict' : '';
    reviewState.value = props.initialSku && props.focus !== 'cost-conflicts' ? 'all' : 'pending';
    search();
  }
});
onDeactivated(() => {
  if (oaEnabled) {
    visitActive = false;
    epoch++;
    controller?.abort();
  }
  selected.value = null;
  editing.value = false;
  logsOpen.value = false;
  usedInspector = false;
});
onBeforeUnmount(() => {
  epoch++;
  controller?.abort();
  if (timer) clearInterval(timer);
  inspectorMedia?.removeEventListener("change", syncInspectorSize);
  defaultMedia?.removeEventListener("change", syncInspectorSize);
  if ((selected.value && !inlineInspector.value) || logsOpen.value)
    document.body.style.cssText = savedBodyStyle;
});
</script>

<style scoped>
html.oa-live-ui .maintenance .missing-dimensions { background: rgb(var(--yb-warning-soft)); color: rgb(var(--yb-warning-text)); }
html.oa-live-ui .maintenance .blocked-confirm small { display: none; }
html.oa-live-ui .maintenance .blocked-confirm:is(:hover, :focus-visible) small { display: block; }
html.oa-live-ui .maintenance table { font-size: 13px; line-height: 1.5; }
html.oa-live-ui .maintenance th { height: 40px; padding: 6px 10px; }
html.oa-live-ui .maintenance td { height: 56px; padding: 6px 10px; }
html.oa-live-ui .maintenance :is(td.money, td strong) { font-family: var(--yb-font-mono); }
html.oa-live-ui .maintenance td.money small { font-size: 12px; }
html.oa-live-ui .maintenance .row-actions button { min-height: 32px; }
html.oa-live-ui .maintenance .blocked-confirm + button { margin-left: 8px; }
.pricing-link {
  display: inline-flex;
  align-items: center;
  min-height: 32px;
  padding-inline: 9px;
  color: rgb(var(--yb-brand));
  white-space: nowrap;
  text-underline-offset: 3px;
}
.bulk-pricing-link {
  height: 32px;
  padding-inline: 12px;
  border: 1px solid rgb(var(--yb-border-strong));
  border-radius: 6px;
  background: rgb(var(--yb-surface));
  color: rgb(var(--yb-text-secondary));
  text-decoration: none;
}
.bulk-pricing-link:hover { border-color: var(--oa-accent); color: var(--oa-accent); }
.pricing-link:focus-visible { outline: 2px solid rgb(var(--yb-brand)); outline-offset: 2px; }

.cost-refresh-status { display: inline-flex; align-items: center; gap: 5px; white-space: nowrap; font-size: 12px; color: rgb(var(--yb-text-muted)); }
.cost-refresh-status > span { width: 6px; height: 6px; border-radius: 50%; background: rgb(var(--yb-success-teal)); }
.cost-row-link { color: rgb(var(--yb-brand)); display: block; text-decoration-thickness: 1px; text-underline-offset: 2px; }
.conflict-filter { color: rgb(var(--yb-warning-text)); }
.conflict-filter:hover, .conflict-filter[aria-pressed=true] { background: rgb(var(--yb-warning-soft)); }
.cost-cell-stack { display: flex; flex-direction: column; align-items: flex-start; gap: 4px; min-width: 0; }
.maintenance .cost-cell-stack > :is(span, small) {
  margin: 0;
  max-width: 100%;
  line-height: 16px;
  padding-inline-start: 5px;
}
html.oa-live-ui #app .maintenance .cost-cell-stack > button {
  height: 24px;
  min-height: 24px;
  max-inline-size: 100%;
  padding: 0 5px;
  border-radius: 6px;
  font-size: 12px;
  white-space: nowrap;
}
.cost-inspect-fields { display: grid; grid-template-columns: auto minmax(0, 1fr); gap: 12px; margin: 0; }
.cost-inspect-fields dd { margin: 0; overflow-wrap: anywhere; }
.cost-inspect-fields dt { color: rgb(var(--yb-text-muted)); }
.maintenance-list { min-width: 0; min-height: 0; display: flex; flex-direction: column; }
html.oa-live-ui .maintenance tr[data-cost-row] { cursor: pointer; }
html.oa-live-ui #app .maintenance tr[data-cost-row]:hover > td { background: rgb(var(--yb-surface-hover)); }
html.oa-live-ui #app .maintenance tr[data-cost-row].selected-row > td { background: rgb(var(--yb-brand-soft)); }
html.oa-live-ui .maintenance tr.selected-row > td:first-child { box-shadow: inset 3px 0 var(--oa-accent); }
.cost-inspector .maintenance-backdrop { background: var(--oa-inspector-scrim); }
.cost-inspector .maintenance-drawer :is(input, select, textarea):focus {
  border-color: var(--oa-accent);
  box-shadow: var(--yb-focus-ring);
  outline: none;
}
.cost-inspector .maintenance-drawer button[data-next] {
  background: var(--oa-accent);
  border-color: var(--oa-accent);
}
.cost-inspector .maintenance-drawer button.secondary { border-color: rgb(var(--yb-border-strong)); }
@media (min-width: 1280px) {
  html.oa-live-ui .maintenance :is(.maintenance-tools,.maintenance-tools form,.maintenance-tools > div) { flex-wrap: wrap; gap: 8px; }
  html.oa-live-ui .maintenance :is(.maintenance-tools,.maintenance-footer) :is(button,select,input) {
    min-height: 32px;
    height: 32px;
    padding-block: 0;
  }
  html.oa-live-ui .maintenance .cost-refresh-status { display: none; }
  html.oa-live-ui .maintenance :is(.maintenance-error,.maintenance-notice,.permission-note) { margin-block: 4px; }
  html.oa-live-ui .maintenance table { table-layout: fixed; min-width: 890px; }
  html.oa-live-ui .maintenance th { z-index: 2; }
  html.oa-live-ui .maintenance td { height: 52px; box-sizing: border-box; padding-block: 3px; }
  html.oa-live-ui .maintenance td.row-actions { padding-inline: 8px; white-space: nowrap; }
  html.oa-live-ui .maintenance .row-action-group { display: flex; flex-wrap: nowrap; align-items: center; gap: 4px; }
  html.oa-live-ui #app .maintenance .row-action-group :is(button, a) { min-height: 32px; height: 32px; box-sizing: border-box; padding: 0 7px; margin: 0; border-radius: 6px; }
  html.oa-live-ui #app .maintenance .row-action-group .pricing-link {
    border: 1px solid rgb(var(--yb-border));
    text-decoration: none;
  }
  html.oa-live-ui #app .maintenance .row-action-group .pricing-link:hover { border-color: var(--oa-accent); }
  html.oa-live-ui .maintenance .row-action-group .maintenance-reviewed { margin: 0; }
  html.oa-live-ui .maintenance .row-action-group .blocked-confirm small { display: none; }
  html.oa-live-ui .maintenance td:nth-child(5) :is(span,small) { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  html.oa-live-ui .maintenance th:nth-child(2) { width: 140px; }
  html.oa-live-ui .maintenance th:nth-child(3) { width: 123px; }
  html.oa-live-ui .maintenance th:nth-child(4) { width: 154px; }
  html.oa-live-ui .maintenance th:nth-child(5) { width: 100px; }
  html.oa-live-ui .maintenance th:nth-child(6) { box-sizing: border-box; width: 168px; }
  html.oa-live-ui .maintenance td:first-child :is(strong,span) { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  html.oa-live-ui .maintenance th.money { font-family: var(--yb-font-mono); }
  .cost-inspector { z-index: var(--oa-inspector-layer, 140); }
  .cost-inspector .maintenance-drawer { width: min(420px, 100%); }
  .cost-inspector .maintenance-drawer > :is(header,footer) { padding: 10px 12px; gap: 6px; }
  .cost-inspector .drawer-body { padding: 12px; gap: 14px; min-height: 0; }
  .cost-inspector .maintenance-drawer a { color: rgb(var(--yb-brand)); font-size: 13px; }
  .cost-inspector.inline-inspector { z-index: auto; position: relative; inset: auto; min-width: 0; min-height: 0; }
  .cost-inspector.inline-inspector .maintenance-drawer {
    width: 100%;
    border: 1px solid rgb(var(--yb-border));
    border-radius: 8px;
    box-shadow: none;
  }
}
</style>
