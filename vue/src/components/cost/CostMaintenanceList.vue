<template>
  <section class="maintenance" aria-label="编码成本维护">
    <div class="maintenance-tools">
      <form @submit.prevent="search">
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
    <div class="maintenance-table" :aria-busy="loading">
      <table>
        <thead>
          <tr>
            <th>编码与名称</th>
            <th>款式编码</th>
            <th>尺寸与面积</th>
            <th class="money">成本价（元）</th>
            <th>状态</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in rows" :key="row.id">
            <td>
              <strong>{{ row.sku_code }}</strong
              ><span>{{ row.product_name || "名称待补充" }}</span>
            </td>
            <td>{{ row.style_code || "待绑定" }}</td>
            <td>
              <span>{{ dimensions(row) }}</span
              ><small>{{ area(row.area_m2) }} ㎡</small>
            </td>
            <td class="money">{{ money(row.cost_price) }}</td>
            <td>
              <span :class="['maintenance-state', row.status]">{{
                maintenanceStatuses[row.status] || "待核对"
              }}</span
              ><small v-if="row.status === 'conflict'"
                >ERP {{ money(row.erp_cost) }} 元</small
              >
              <small
                v-if="row.base_sync_required"
                :title="row.base_sync_error || '名称与规格资料正在同步'"
                >{{
                  row.base_sync_error ? "资料同步失败" : "资料待同步"
                }}</small
              >
            </td>
            <td class="row-actions">
              <span
                v-if="row.reviewed && row.status !== 'conflict'"
                class="maintenance-reviewed"
                >已确认</span
              >
              <button
                v-else
                :disabled="
                  !canEdit ||
                  busyID !== null ||
                  row.cost_price === null ||
                  row.revision < 1
                "
                @click="confirm(row)"
              >
                {{ busyID === row.id ? "提交中…" : "确认" }}</button
              ><button
                :disabled="!canEdit || busyID !== null || row.revision < 1"
                class="accent"
                @click="edit(row)"
              >
                修改
              </button>
            </td>
          </tr>
          <tr v-if="!rows.length">
            <td colspan="6" class="empty">
              {{
                loading
                  ? "正在读取已完成编码…"
                  : error
                    ? "暂未取得数据，请重试。"
                  : reviewState === 'pending' ? "当前没有待确认的编码，可切换筛选查看已确认记录。" : "没有匹配的已完成编码。"
              }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <footer class="maintenance-footer">
      <span
        >{{ updatedLabel }}
        <small>{{
          selected ? "编辑期间暂停刷新" : "每15秒更新当前页"
        }}</small></span
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
          上一页</button
        ><span>第 {{ cursors.length }} 页</span
        ><button :disabled="!nextID || loading" @click="next">下一页</button>
      </div>
    </footer>
    <p v-if="!canEdit" class="permission-note">
      查看模式；确认与修改需要成本维护和 ERP 管理权限。
    </p>
    <Teleport to="body">
      <div
        v-if="selected"
        class="maintenance-overlay"
        @keydown.esc="closeEditor"
      >
        <button
          class="maintenance-backdrop"
          aria-label="关闭修改侧栏"
          :disabled="saving"
          @click="closeEditor"
        />
        <form
          ref="editorElement"
          class="maintenance-drawer"
          role="dialog"
          aria-modal="true"
          aria-labelledby="maintenance-edit-title"
          tabindex="-1"
          @submit.prevent="save"
          @keydown.tab="trapFocus"
        >
          <header>
            <div>
              <h2 id="maintenance-edit-title">修改编码</h2>
              <p>{{ selected.sku_code }}</p>
            </div>
            <button
              type="button"
              aria-label="关闭"
              :disabled="saving"
              @click="closeEditor"
            >
              ×
            </button>
          </header>
          <div class="drawer-body">
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
                    v-model.number="draft.width_cm"
                    type="number"
                    min="0.0001"
                    step="any"
                    @input="deriveArea" /></label
                ><label
                  >高（cm）<input
                    v-model.number="draft.height_cm"
                    type="number"
                    min="0.0001"
                    step="any"
                    @input="deriveArea" /></label
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
          <footer>
            <button type="button" :disabled="saving" @click="closeEditor">
              取消</button
            ><button
              type="submit"
              class="primary"
              :disabled="saving || !changes.length || !reason || stale"
            >
              {{ saving ? "保存中…" : "保存并同步" }}
            </button>
          </footer>
        </form>
      </div>
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
            <button aria-label="关闭日志" @click="logsOpen = false">×</button>
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
              ><button>查询</button>
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
              上一页</button
            ><button
              :disabled="logPage * 50 >= logTotal || logsLoading"
              @click="
                logPage++;
                loadLogs();
              "
            >
              下一页
            </button>
          </footer>
        </section>
      </div>
    </Teleport>
  </section>
</template>

<script setup lang="ts">
import {
  computed,
  nextTick,
  onBeforeUnmount,
  onMounted,
  watch,
  reactive,
  ref,
} from "vue";
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
  canEdit = computed(
    () => permission.can("catalog.manage") && permission.can("erp.manage"),
  );
const rows = ref<MaintenanceRow[]>([]),
  keyword = ref(""),
  status = ref(""),
  reviewState = ref<"pending" | "reviewed" | "all">("pending"),
  pageSize = ref(50),
  cursors = ref([0]),
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
  editorElement = ref<HTMLFormElement>();
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
  () => !!selected.value || logsOpen.value,
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
const date = (value: string) =>
  new Date(value).toLocaleString("zh-CN", {
    timeZone: "Asia/Shanghai",
    hour12: false,
  });
const updatedLabel = computed(() =>
  refreshed.value ? `更新于 ${date(refreshed.value)}（北京时间）` : "",
);
async function load(background = false) {
  if (
    background &&
    (selected.value ||
      logsOpen.value ||
      busyID.value !== null ||
      document.visibilityState === "hidden")
  )
    return;
  const ticket = ++epoch;
  controller?.abort();
  controller = new AbortController();
  loading.value = true;
  error.value = "";
  try {
    const result = await costMaintenanceApi.list(
      {
        keyword: keyword.value,
        status: status.value,
        review_state: reviewState.value,
        before_id: cursors.value.at(-1),
        page_size: pageSize.value,
      },
      controller.signal,
    );
    if (ticket !== epoch) return;
    rows.value = result.data || [];
    nextID.value = result.next_before_id;
    refreshed.value = result.refreshed_at;
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
  focusBefore = document.activeElement as HTMLElement;
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
function closeEditor() {
  if (!saving.value) {
    selected.value = null;
    nextTick(() => focusBefore?.focus());
  }
}
function trapFocus(e: KeyboardEvent) {
  const els = Array.from(
    editorElement.value?.querySelectorAll<HTMLElement>(
      "button:not(:disabled),input:not(:disabled),select,textarea,summary",
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
async function save() {
  if (!selected.value || !changes.value.length) return;
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
    closeEditor();
    await load();
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
  load();
  timer = setInterval(() => {
    if (!loading.value) load(true);
  }, 15000);
});
onBeforeUnmount(() => {
  epoch++;
  controller?.abort();
  if (timer) clearInterval(timer);
  if (selected.value || logsOpen.value)
    document.body.style.cssText = savedBodyStyle;
});
</script>
