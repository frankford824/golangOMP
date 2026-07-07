<template>
  <div class="product-management-view">
    <header class="pm-header">
      <div>
        <p class="pm-eyebrow">ERP 商品资料对照</p>
        <h1>产品管理</h1>
        <p class="pm-subtitle">按 SKU 维护 ERP 图片、成本与同步状态，默认聚焦缺图、成本异常、待同步和失败项。</p>
      </div>
      <div class="pm-header-actions">
        <button
          v-if="canUseCostTools"
          type="button"
          class="pm-btn pm-btn--cost"
          :class="{ 'has-issues': costIssueTotal > 0 }"
          :disabled="costDashboardLoading"
          @click="toggleCostTools"
        >
          成本问题 {{ costIssueTotal }}
        </button>
        <button type="button" class="pm-btn pm-btn--ghost" :disabled="loading" @click="loadRecords">
          {{ loading ? '刷新中' : '刷新' }}
        </button>
      </div>
    </header>

    <section class="pm-filters">
      <label class="pm-field pm-field--wide">
        <span>搜索</span>
        <input
          v-model.trim="filters.keyword"
          type="search"
          placeholder="SKU、任务号、款式编码、商品名、创建人"
          @keyup.enter="applyFilters"
        />
      </label>
      <label class="pm-field">
        <span>显示范围</span>
        <select v-model="filters.display_scope" @change="applyDisplayScope">
          <option value="combo">组合装</option>
          <option value="single">单品 SKU</option>
          <option value="all">全部</option>
        </select>
      </label>
      <label class="pm-field">
        <span>关注范围</span>
        <select v-model="filters.issue_scope" @change="applyFilters">
          <option value="attention">待处理优先</option>
          <option value="all">全部记录</option>
        </select>
      </label>
      <label class="pm-field">
        <span>图片来源</span>
        <select v-model="filters.image_source" @change="applyFilters">
          <option value="">全部</option>
          <option value="manual">人工指定</option>
          <option value="erp_product_image">专项 ERP 商品图</option>
          <option value="auto_on_close">结单自动同步</option>
          <option value="delivery">SKU 成品图</option>
          <option value="derived_preview">派生预览</option>
          <option value="task_reference">任务参考图</option>
          <option value="missing">缺图</option>
        </select>
      </label>
      <label class="pm-field">
        <span>规格 / 面积 / 成本</span>
        <select v-model="filters.cost_status" @change="applyFilters">
          <option value="">全部</option>
          <option value="missing">缺成本</option>
          <option value="ready">已有成本</option>
        </select>
      </label>
      <label class="pm-field">
        <span>基础资料</span>
        <select v-model="filters.base_sync_status" @change="applyFilters">
          <option value="">全部</option>
          <option value="pending_sync">待同步</option>
          <option value="queued">已入队</option>
          <option value="syncing">同步中</option>
          <option value="cooling_down">冷却中</option>
          <option value="failed">失败</option>
          <option value="synced">已同步</option>
        </select>
      </label>
      <label class="pm-field">
        <span>ERP 图片</span>
        <select v-model="filters.image_sync_status" @change="applyFilters">
          <option value="">全部</option>
          <option value="waiting_image">待上传</option>
          <option value="pending_sync">待同步</option>
          <option value="queued">已入队</option>
          <option value="syncing">同步中</option>
          <option value="cooling_down">冷却中</option>
          <option value="failed">失败</option>
          <option value="synced">已同步</option>
        </select>
      </label>
      <button type="button" class="pm-btn pm-btn--primary" @click="applyFilters">查询</button>
      <button type="button" class="pm-btn pm-btn--ghost" :disabled="batchSyncing || syncableRecords.length === 0" @click="syncCurrentPage">
        {{ batchSyncing ? '同步中' : '同步当前页' }}
      </button>
    </section>

    <section v-if="canUseCostTools && costToolsOpen" class="pm-cost-console">
      <header class="pm-cost-console-head">
        <div>
          <p class="pm-eyebrow">成本问题</p>
          <h2>按问题清理 SKU 成本</h2>
          <p>{{ costDashboardHint }}</p>
        </div>
        <div class="pm-cost-console-actions">
          <button type="button" class="pm-btn pm-btn--ghost" :disabled="costDashboardLoading" @click="loadCostDashboard">
            {{ costDashboardLoading ? '加载中' : '刷新问题数' }}
          </button>
          <button type="button" class="pm-btn pm-btn--ghost" @click="costToolsOpen = false">收起</button>
        </div>
      </header>

      <div class="pm-cost-groups">
        <button
          v-for="group in costIssueGroups"
          :key="group.code"
          type="button"
          class="pm-cost-group"
          :class="{ 'is-active': activeCostGroup === group.code }"
          @click="selectCostGroup(group.code)"
        >
          <span>{{ group.label }}</span>
          <strong>{{ group.count }}</strong>
        </button>
      </div>

      <div class="pm-cost-chips">
        <button
          type="button"
          class="pm-cost-chip"
          :class="{ 'is-active': activeCostTag === '' }"
          @click="selectCostTag('')"
        >
          全部问题
        </button>
        <button
          v-for="tag in costIssueTags"
          :key="tag.code"
          type="button"
          class="pm-cost-chip"
          :class="{ 'is-active': activeCostTag === tag.code }"
          @click="selectCostTag(tag.code)"
        >
          {{ tag.label }} {{ tag.count }}
        </button>
      </div>
      <div class="pm-cost-policy">
        <span>{{ costFallbackPolicyText }}</span>
        <span>未关联款式占比 {{ costUnboundRatioText }}</span>
        <span>{{ costUnboundTrendText }}</span>
      </div>

      <div class="pm-cost-console-grid">
        <section class="pm-cost-panel">
          <div class="pm-cost-panel-head">
            <div>
              <h3>未关联款式</h3>
              <p>{{ unboundPanelHint }}</p>
            </div>
            <button type="button" class="pm-btn pm-btn--small" @click="openCostBinding()">新增关联</button>
          </div>
          <div v-if="unboundLoading" class="pm-cost-mini-empty">未关联款式加载中...</div>
          <div v-else-if="unboundCandidates.length === 0" class="pm-cost-mini-empty">{{ unboundEmptyText }}</div>
          <div v-else class="pm-unbound-list">
            <div
              v-for="candidate in unboundCandidates"
              :key="candidate.normalized_i_id"
              class="pm-unbound-item"
            >
              <button type="button" class="pm-unbound-main" @click="openCostBinding(candidate)">
                <strong>{{ candidateDisplayIId(candidate) }}</strong>
                <span>{{ candidate.suggested_display_name || '待选择定价规则' }}</span>
                <small>{{ candidateImpactText(candidate) }}</small>
              </button>
              <button type="button" class="pm-btn pm-btn--small" @click="openCostBinding(candidate)">生成关联草稿</button>
            </div>
          </div>
        </section>

        <section class="pm-cost-panel">
          <div class="pm-cost-panel-head">
            <div>
              <h3>批量修复</h3>
              <p>{{ bulkSelectionHint }}</p>
            </div>
            <button type="button" class="pm-btn pm-btn--small pm-btn--primary" :disabled="!canCreateBulkRun" @click="createBulkRun">
              批量修复
            </button>
          </div>
          <div class="pm-bulk-actions">
            <button type="button" class="pm-btn pm-btn--small" @click="selectCurrentPageRecords">全选当前页</button>
            <button type="button" class="pm-btn pm-btn--small" @click="selectAllMatchingRecords">
              全选全部符合条件
            </button>
            <button type="button" class="pm-btn pm-btn--small" :disabled="selectedRecordCount === 0 && !bulkAllMatching" @click="clearCostSelection">
              清空选择
            </button>
          </div>
          <p v-if="bulkAllMatching" class="pm-cost-note">将按当前筛选条件创建批量修复预览，后端会重新圈选符合条件的 SKU。</p>
          <p v-else class="pm-cost-note">已选择 {{ selectedRecordCount }} 条当前页 SKU。</p>
          <p class="pm-cost-note">{{ activeCostFilterHint }}</p>
        </section>

        <section class="pm-cost-panel pm-cost-panel--calculator">
          <div class="pm-cost-panel-head">
            <div>
              <h3>规则试算器</h3>
              <p>输入尺寸和数量，查看含税倍率、有效单价与最终成本。</p>
            </div>
          </div>
          <div class="pm-calculator-grid">
            <label class="pm-field">
              <span>定价规则</span>
              <select v-model="calculator.rule_group" @change="scheduleCostRulePreview">
                <option value="">请选择</option>
                <option v-for="option in ruleGroupOptions" :key="option.rule_group" :value="option.rule_group">
                  {{ option.display_name }}
                </option>
              </select>
            </label>
            <label class="pm-field">
              <span>宽（米）</span>
              <input v-model.number="calculator.width" inputmode="decimal" type="number" min="0" step="0.001" @input="scheduleCostRulePreview" />
            </label>
            <label class="pm-field">
              <span>高（米）</span>
              <input v-model.number="calculator.height" inputmode="decimal" type="number" min="0" step="0.001" @input="scheduleCostRulePreview" />
            </label>
            <label class="pm-field">
              <span>数量</span>
              <input v-model.number="calculator.quantity" inputmode="numeric" type="number" min="1" step="1" @input="scheduleCostRulePreview" />
            </label>
            <label class="pm-field pm-field--wide">
              <span>工艺</span>
              <input v-model.trim="calculator.process" placeholder="如 开槽、覆膜" @input="scheduleCostRulePreview" />
            </label>
          </div>
          <div class="pm-calculator-result">
            <span v-if="calculatorLoading">试算中...</span>
            <span v-else-if="calculatorError" class="pm-error-text">{{ calculatorError }}</span>
            <template v-else-if="calculatorPreview">
              <strong>{{ formatCost(calculatorPreview.estimated_cost) }}</strong>
              <small>{{ calculatorPreviewText }}</small>
            </template>
            <span v-else>选择规则后输入尺寸自动试算。</span>
          </div>
        </section>
      </div>
      <p v-if="costDashboardError" class="pm-error">{{ costDashboardError }}</p>
    </section>

    <div class="pm-summary">
      <span>当前共 <b>{{ pagination.total }}</b> 条</span>
      <span>当前页 <b>{{ records.length }}</b> 条</span>
      <span>显示 <b>{{ visibleGroups.length }}</b> 个{{ displayScopeLabel }}</span>
      <span v-if="comboSyncSummaryText" class="pm-combo-sync">{{ comboSyncSummaryText }}</span>
      <span v-if="error" class="pm-error">{{ error }}</span>
    </div>

    <section class="pm-table-shell" :class="{ 'is-loading': loading }">
      <div class="pm-table-head">
        <span>ERP 图片</span>
        <span>SKU / 款式</span>
        <span>商品与任务</span>
        <span>成本</span>
        <span>创建信息</span>
        <span>同步</span>
        <span>操作</span>
      </div>

      <section v-for="group in visibleGroups" :key="group.group_key" class="pm-combo-group" :class="`pm-combo-group--${group.group_type}`">
        <button
          type="button"
          class="pm-combo-header"
          :class="{ 'is-expanded': isComboGroupExpanded(group), 'is-static': group.group_type !== 'combo' }"
          @click="toggleComboGroup(group)"
        >
          <span v-if="group.group_type === 'combo'" class="pm-combo-thumb" :class="{ 'is-missing': !group.pic_url }">
            <img v-if="group.pic_url" :src="group.pic_url" alt="" loading="lazy" referrerpolicy="no-referrer" />
            <span v-else>父级图</span>
          </span>
          <div class="pm-combo-primary">
            <p class="pm-combo-kicker">{{ group.group_type === 'combo' ? '组合装父级' : '单品 SKU' }}</p>
            <strong>
              <span class="pm-combo-code">{{ groupTitle(group) }}</span>
              <span v-if="group.group_type === 'combo' && comboParentName(group)" class="pm-combo-name">{{ comboParentName(group) }}</span>
            </strong>
            <small v-if="group.group_type === 'combo'">{{ groupSubtitle(group) || '聚水潭组合装父级资料暂无更多字段' }}</small>
            <span v-if="group.group_type === 'combo' && group.properties_value" class="pm-combo-properties">{{ group.properties_value }}</span>
          </div>
          <span class="pm-combo-meta">
            <span v-if="group.group_type === 'combo'" class="pm-combo-field">
              <b>款式</b>
              {{ comboParentStyle(group) }}
            </span>
            <span v-if="group.group_type === 'combo'" class="pm-combo-field">
              <b>品牌/分类</b>
              {{ comboParentCategory(group) }}
            </span>
            <span v-if="group.group_type === 'combo'" class="pm-combo-field">
              <b>成本/售价</b>
              {{ comboParentPrice(group) }}
            </span>
            <span class="pm-combo-count">{{ group.children.length }} 个系统 SKU</span>
            <span v-if="group.group_type === 'combo'" class="pm-combo-toggle">
              {{ isComboGroupExpanded(group) ? '收起' : '展开' }}
            </span>
          </span>
        </button>

        <template v-if="shouldShowGroupChildren(group)">
          <article v-for="child in group.children" :key="`${group.group_key}:${child.record.id}`" class="pm-row">
            <div class="pm-image-cell">
              <div class="pm-preview" :class="{ 'pm-preview--missing': !previewLoadableForRecord(child.record) }">
                <AssetPreviewMedia
                  v-if="previewLoadableForRecord(child.record)"
                  :asset-id="assetIDForRecord(child.record) || null"
                  :resolved-preview-url="previewURLForRecord(child.record) || null"
                  :fallback-src="directPreviewURL(child.record.image_preview_url) || null"
                  :alt="child.record.sku_code"
                  img-class="pm-preview-apm"
                  inner-img-class="pm-preview-img"
                  defer-until-visible
                />
                <span v-else>{{ child.record.image_missing_reason || 'ERP 图片待补充' }}</span>
              </div>
              <span class="pm-pill" :class="`pm-source--${child.record.image_source}`">{{ child.record.image_source_label }}</span>
            </div>

            <div class="pm-main-cell">
              <label v-if="canUseCostTools && costToolsOpen" class="pm-record-select">
                <input
                  type="checkbox"
                  :checked="isRecordSelected(child.record.id)"
                  @change="toggleRecordSelected(child.record.id)"
                />
                <span>批量修复</span>
              </label>
              <strong class="pm-mono">{{ child.record.sku_code || '-' }}</strong>
              <small>款式 {{ productIIDLabel(child.record) }}</small>
              <small v-if="group.group_type === 'combo'">组合数量 {{ formatQuantity(child.quantity) }}</small>
              <small v-if="child.record.category_name">分类 {{ child.record.category_name }}</small>
            </div>

            <div class="pm-info-cell">
              <strong>{{ child.record.product_name || '未命名商品' }}</strong>
              <button type="button" class="pm-link" @click="openTask(child.record.task_id)">
                {{ child.record.task_no || `任务 ${child.record.task_id}` }}
              </button>
            </div>

            <div class="pm-cost-cell" :class="{ 'is-missing': !hasCost(child.record), 'has-area-warning': hasAreaWarning(child.record) }">
              <div class="pm-cost-topline">
                <span v-if="specSummary(child.record)" class="pm-spec-chip" :title="specSummary(child.record)">
                  {{ specSummary(child.record) }}
                </span>
                <span v-else class="pm-spec-chip pm-spec-chip--empty">规格待补</span>
                <span class="pm-detail-wrap">
                  <button type="button" class="pm-detail-help" :aria-label="productTraceAria(child.record)">
                    明细
                  </button>
                  <span class="pm-cost-popover" role="tooltip">
                    <strong>面积识别</strong>
                    <span v-for="line in areaTraceLines(child.record)" :key="`area-${line}`">{{ line }}</span>
                    <strong>成本计算</strong>
                    <span v-for="line in costTraceLines(child.record)" :key="`cost-${line}`">{{ line }}</span>
                  </span>
                </span>
              </div>
              <div class="pm-metric-stack">
                <span class="pm-metric-row pm-cost-row">
                  <span class="pm-metric-label">成本</span>
                  <span class="pm-cost-value">{{ formatCost(child.record.cost_price) }}</span>
                </span>
                <span class="pm-metric-row pm-area-row">
                  <span class="pm-metric-label">面积</span>
                  <span class="pm-area-value">{{ areaTraceSummary(child.record) }}</span>
                </span>
              </div>
            </div>

            <div class="pm-info-cell">
              <strong>{{ child.record.creator_name || `用户 ${child.record.creator_id}` }}</strong>
              <small>{{ formatDate(child.record.task_created_at) }}</small>
            </div>

            <div class="pm-sync-cell">
              <span class="pm-pill" :class="`pm-sync--${baseSyncStatus(child.record)}`">
                基础 {{ syncStatusLabel(baseSyncStatus(child.record)) }}
              </span>
              <small>{{ child.record.last_base_synced_at ? formatDate(child.record.last_base_synced_at) : '基础资料尚未同步' }}</small>
              <small v-if="child.record.base_sync_error" class="pm-error-text">{{ child.record.base_sync_error }}</small>
              <span class="pm-pill" :class="`pm-sync--${imageSyncStatus(child.record)}`">
                图片 {{ syncStatusLabel(imageSyncStatus(child.record)) }}
              </span>
              <small>{{ child.record.last_image_synced_at ? formatDate(child.record.last_image_synced_at) : 'ERP 图片尚未同步' }}</small>
              <small v-if="child.record.image_sync_error" class="pm-error-text">{{ child.record.image_sync_error }}</small>
              <div v-if="isRecordSyncing(child.record)" class="pm-sync-progress" aria-hidden="true">
                <span></span>
              </div>
              <small v-if="syncMessageForRecord(child.record)" class="pm-sync-message">{{ syncMessageForRecord(child.record) }}</small>
            </div>

            <div class="pm-actions">
              <button type="button" class="pm-btn pm-btn--small" @click="openTask(child.record.task_id)">打开任务</button>
              <button v-if="canUseCostTools" type="button" class="pm-btn pm-btn--small" @click="openQuickFix(child.record)">
                修复
              </button>
              <button type="button" class="pm-btn pm-btn--small" :disabled="!child.record.can_maintain_image" @click="openCandidates(child.record)">
                选图
              </button>
              <button type="button" class="pm-btn pm-btn--small" :disabled="!child.record.can_maintain_image" @click="reparseImage(child.record)">
                重新解析
              </button>
              <button type="button" class="pm-btn pm-btn--small" :disabled="!child.record.can_sync_erp || isRecordSyncing(child.record)" @click="requestBaseSync(child.record)">
                {{ syncActionLabel(child.record, 'base', '同步基础') }}
              </button>
              <button type="button" class="pm-btn pm-btn--small pm-btn--primary" :disabled="!child.record.can_sync_erp || isRecordSyncing(child.record)" @click="requestSync(child.record)">
                {{ syncActionLabel(child.record, 'all', '全部同步') }}
              </button>
              <button type="button" class="pm-btn pm-btn--small pm-btn--primary" :disabled="!child.record.can_sync_erp || !child.record.image_asset_id || isRecordSyncing(child.record)" @click="requestImageSync(child.record)">
                {{ syncActionLabel(child.record, 'image', '同步图片') }}
              </button>
            </div>
          </article>
        </template>
      </section>

      <div v-if="!loading && visibleGroups.length === 0" class="pm-empty">{{ emptyMessage }}</div>
    </section>

    <footer class="pm-pagination">
      <div class="pm-pagination-info">
        <strong>第 {{ pagination.page }} / {{ totalPages }} 页</strong>
        <span>剩余 {{ remainingPages }} 页 · 共 {{ pagination.total }} 条 · 每页 {{ pagination.page_size }} 条</span>
      </div>
      <div class="pm-pagination-actions">
        <button type="button" class="pm-page-btn pm-page-btn--wide" :disabled="!hasPreviousPage || loading" @click="changePage(1)">
          首页
        </button>
        <button type="button" class="pm-page-btn pm-page-btn--wide" :disabled="!hasPreviousPage || loading" @click="changePage(filters.page - 1)">
          上一页
        </button>
        <button
          v-for="page in visiblePageNumbers"
          :key="page"
          type="button"
          class="pm-page-btn"
          :class="{ 'is-active': page === pagination.page }"
          :disabled="loading || page === pagination.page"
          @click="changePage(page)"
        >
          {{ page }}
        </button>
        <button type="button" class="pm-page-btn pm-page-btn--wide" :disabled="!hasNextPage || loading" @click="changePage(filters.page + 1)">
          下一页
        </button>
        <button type="button" class="pm-page-btn pm-page-btn--wide" :disabled="!hasNextPage || loading" @click="changePage(totalPages)">
          末页
        </button>
      </div>
    </footer>

    <div v-if="costBindingModalOpen" class="pm-modal-mask" @click.self="closeCostBinding">
      <section class="pm-modal pm-cost-modal">
        <header>
          <div>
            <p class="pm-eyebrow">款式关联定价规则</p>
            <h2>{{ bindingForm.i_id_raw || '新增关联' }}</h2>
          </div>
          <button type="button" class="pm-btn pm-btn--ghost" @click="closeCostBinding">关闭</button>
        </header>
        <div class="pm-binding-form">
          <IIdSelector v-model="bindingForm.i_id_raw" label="产品款式编码" placeholder="搜索或选择款式编码" />
          <label class="pm-field">
            <span>定价规则</span>
            <select v-model="bindingForm.rule_group">
              <option value="">请选择定价规则</option>
              <option v-for="option in ruleGroupOptions" :key="option.rule_group" :value="option.rule_group">
                {{ option.display_name }}
              </option>
            </select>
          </label>
          <label class="pm-field pm-field--wide">
            <span>显示名称</span>
            <input v-model.trim="bindingForm.display_name" placeholder="如 常规覆膜 KT 板" />
          </label>
        </div>
        <p v-if="bindingError" class="pm-error">{{ bindingError }}</p>
        <div class="pm-modal-actions">
          <button type="button" class="pm-btn pm-btn--ghost" @click="closeCostBinding">取消</button>
          <button type="button" class="pm-btn pm-btn--primary" :disabled="bindingSaving || !bindingForm.i_id_raw || !bindingForm.rule_group" @click="saveCostBinding">
            {{ bindingSaving ? '保存中' : '保存关联' }}
          </button>
        </div>
      </section>
    </div>

    <div v-if="quickFixModalOpen" class="pm-modal-mask" @click.self="closeCostRunModals">
      <section class="pm-modal pm-cost-modal">
        <header>
          <div>
            <p class="pm-eyebrow">单条快速修复</p>
            <h2>{{ quickFixRecord?.sku_code || 'SKU 成本修复' }}</h2>
          </div>
          <button type="button" class="pm-btn pm-btn--ghost" @click="closeCostRunModals">关闭</button>
        </header>
        <div class="pm-quick-summary">
          <span>
            <b>旧成本</b>
            {{ formatCost(quickFixItem?.old_cost_price ?? quickFixRecord?.cost_price) }}
          </span>
          <span>
            <b>新成本</b>
            {{ formatCost(quickFixItem?.new_cost_price) }}
          </span>
          <span>
            <b>差额</b>
            {{ formatSignedCost(quickFixItem?.cost_delta) }}
          </span>
        </div>
        <p class="pm-cost-note">{{ costRunStatusText }}</p>
        <p v-if="quickFixItemReason" class="pm-error-text">{{ quickFixItemReason }}</p>
        <label class="pm-checkline">
          <input v-model="quickFixSyncERP" type="checkbox" />
          <span>同时更新聚水潭成本</span>
        </label>
        <p v-if="costRunError" class="pm-error">{{ costRunError }}</p>
        <div class="pm-modal-actions">
          <button type="button" class="pm-btn pm-btn--ghost" @click="closeCostRunModals">取消</button>
          <button type="button" class="pm-btn pm-btn--primary" :disabled="!canApplyActiveRun" @click="applyActiveCostRun">
            {{ costRunWorking ? '处理中' : '确认修改' }}
          </button>
        </div>
      </section>
    </div>

    <div v-if="bulkRunModalOpen" class="pm-modal-mask" @click.self="closeCostRunModals">
      <section class="pm-modal pm-cost-modal pm-cost-modal--wide">
        <header>
          <div>
            <p class="pm-eyebrow">批量修复预览</p>
            <h2>修复单 {{ activeCostRun?.id ?? '' }}</h2>
          </div>
          <button type="button" class="pm-btn pm-btn--ghost" @click="closeCostRunModals">关闭</button>
        </header>
        <div class="pm-run-summary">
          <span>预览 {{ runSummary.previewed_count ?? runItems.length }}</span>
          <span>跳过 {{ runSummary.skipped_count ?? skippedRunItemCount }}</span>
          <span>冲突 {{ runSummary.conflict_count ?? conflictRunItemCount }}</span>
          <span>可同步 ERP {{ runSummary.erp_syncable_count ?? appliedRunItemCount }}</span>
        </div>
        <p class="pm-cost-note">{{ runConfirmationText }}</p>
        <div class="pm-run-table">
          <div class="pm-run-table-head">
            <span>SKU</span>
            <span>旧成本</span>
            <span>新成本</span>
            <span>差额</span>
            <span>状态</span>
          </div>
          <div v-if="costRunLoading" class="pm-empty">正在生成预览...</div>
          <div v-else-if="runItems.length === 0" class="pm-empty">暂无预览明细。</div>
          <div v-for="item in runItems" v-else :key="item.id" class="pm-run-row">
            <span>{{ item.sku_code || `记录 ${item.product_management_record_id}` }}</span>
            <span>{{ formatCost(item.old_cost_price) }}</span>
            <span>{{ formatCost(item.new_cost_price) }}</span>
            <span>{{ formatSignedCost(item.cost_delta) }}</span>
            <span>{{ runItemStatusText(item) }}</span>
          </div>
        </div>
        <p v-if="costRunError" class="pm-error">{{ costRunError }}</p>
        <div class="pm-modal-actions">
          <button type="button" class="pm-btn pm-btn--ghost" @click="closeCostRunModals">关闭</button>
          <button type="button" class="pm-btn pm-btn--primary" :disabled="!canApplyActiveRun" @click="applyActiveCostRun">
            确认修改
          </button>
          <button type="button" class="pm-btn pm-btn--primary" :disabled="!canSyncActiveRunERP" @click="syncActiveCostRunERP">
            同步 ERP
          </button>
        </div>
      </section>
    </div>

    <div v-if="candidateModalOpen" class="pm-modal-mask" @click.self="closeCandidates">
      <section class="pm-modal">
        <header>
          <div>
            <p class="pm-eyebrow">当前任务图片候选</p>
            <h2>{{ activeRecord?.sku_code }}</h2>
          </div>
          <button type="button" class="pm-btn pm-btn--ghost" @click="closeCandidates">关闭</button>
        </header>
        <div v-if="activeRecord?.can_cross_task_select" class="pm-manual-asset">
          <label class="pm-field">
            <span>跨任务资产 ID</span>
            <input v-model.trim="manualAssetID" inputmode="numeric" placeholder="输入资产 ID 后设为 ERP 图" />
          </label>
          <button type="button" class="pm-btn pm-btn--primary" :disabled="!manualAssetID" @click="setManualImage(Number(manualAssetID))">
            使用该资产
          </button>
        </div>
        <div v-if="candidateLoading" class="pm-empty">候选图加载中...</div>
        <div v-else-if="candidates.length === 0" class="pm-empty">当前任务内暂无可用候选图。</div>
        <div v-else class="pm-candidate-grid">
          <button
            v-for="candidate in candidates"
            :key="`${candidate.asset_id}-${candidate.asset_version_id}`"
            type="button"
            class="pm-candidate"
            @click="setManualImage(candidate.asset_id)"
          >
            <AssetPreviewMedia
              v-if="previewLoadableForCandidate(candidate)"
              :asset-id="assetIDForCandidate(candidate)"
              :resolved-preview-url="previewURLForCandidate(candidate) || null"
              :fallback-src="directPreviewURL(candidate.preview_url) || null"
              :alt="candidate.file_name"
              img-class="pm-candidate-apm"
              inner-img-class="pm-candidate-img"
              defer-until-visible
            />
            <span v-else>无预览</span>
            <strong>{{ candidate.file_name }}</strong>
            <small>{{ candidate.source_label }} · {{ candidate.sku_code || '任务通用' }}</small>
          </button>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  productManagementApi,
  type CostRecalculationRun,
  type CostRecalculationRunItem,
  type CostRuleGroupOption,
  type CostRulePreviewResponse,
  type ProductCostDashboardResponse,
  type ProductCostIssueGroupCode,
  type ProductCostIssueTagCode,
  type UnboundCostRuleCandidate,
  type ProductManagementCostTrace,
  type ProductImageCandidate,
  type ProductManagementComboGroup,
  type ProductManagementComboSyncSummary,
  type ProductManagementListParams,
  type ProductManagementRecord,
  type ProductSyncStatus,
} from '@/services/api/productManagementApi'
import { fetchAssetPreviewMeta } from '@/domain/asset-access'
import AssetPreviewMedia from '@/components/media/AssetPreviewMedia.vue'
import IIdSelector from '@/components/task-create/IIdSelector.vue'
import { usePermission } from '@/composables/usePermission'
import { mapWithConcurrency } from '@/utils/batchZipDownload'

type ProductSyncScope = 'all' | 'base' | 'image'
type ProductManagementDisplayScope = 'combo' | 'single' | 'all'
type ProductManagementLocalFilters = Required<
  Pick<
    ProductManagementListParams,
    'keyword' | 'issue_scope' | 'image_source' | 'cost_status' | 'sync_status' | 'base_sync_status' | 'image_sync_status' | 'page' | 'page_size'
  >
> & {
  display_scope: ProductManagementDisplayScope
}

const router = useRouter()
const route = useRoute()
const { can } = usePermission()

const COST_TOOL_ACTIONS = [
  'product.cost.read',
  'product.cost.binding.manage',
  'product.cost.recalculate',
  'product.cost.erp_sync',
]

const COST_ISSUE_GROUP_LABELS: Record<ProductCostIssueGroupCode, string> = {
  cannot_calculate: '算不出来的',
  possibly_wrong: '可能算错的',
  looks_abnormal: '看着不对劲的',
}

const COST_ISSUE_TAG_LABELS: Record<ProductCostIssueTagCode, string> = {
  cost_missing: '成本缺失',
  manual_quote: '需人工报价',
  erp_mismatch: 'ERP 不一致',
  rule_version_outdated: '规则版本过旧',
  unbound_iid: '未关联款式',
  area_spec_abnormal: '面积/规格异常',
}

const filters = reactive<ProductManagementLocalFilters>({
  keyword: '',
  display_scope: 'combo',
  issue_scope: 'all',
  image_source: '',
  cost_status: '',
  sync_status: '',
  base_sync_status: '',
  image_sync_status: '',
  page: 1,
  page_size: 20,
})

const records = ref<ProductManagementRecord[]>([])
const comboGroups = ref<ProductManagementComboGroup[]>([])
const comboSyncSummary = ref<ProductManagementComboSyncSummary | null>(null)
const pagination = reactive({ page: 1, page_size: 20, total: 0 })
const loading = ref(false)
const error = ref('')
const batchSyncing = ref(false)
const candidateModalOpen = ref(false)
const candidateLoading = ref(false)
const candidates = ref<ProductImageCandidate[]>([])
const activeRecord = ref<ProductManagementRecord | null>(null)
const manualAssetID = ref('')
const recordPreviewURLs = ref<Record<number, string>>({})
const candidatePreviewURLs = ref<Record<number, string>>({})
const syncingRecordScopes = ref<Record<number, ProductSyncScope>>({})
const syncMessages = ref<Record<number, string>>({})
const expandedComboGroups = ref<Record<string, boolean>>({})
const costToolsOpen = ref(false)
const costDashboard = ref<ProductCostDashboardResponse | null>(null)
const costDashboardLoading = ref(false)
const costDashboardError = ref('')
const activeCostGroup = ref<ProductCostIssueGroupCode>('possibly_wrong')
const activeCostTag = ref<ProductCostIssueTagCode | ''>('')
const unboundCandidates = ref<UnboundCostRuleCandidate[]>([])
const unboundLoading = ref(false)
const ruleGroupOptions = ref<CostRuleGroupOption[]>([])
const selectedRecordIds = ref<Record<number, boolean>>({})
const bulkAllMatching = ref(false)
const costBindingModalOpen = ref(false)
const bindingSaving = ref(false)
const bindingError = ref('')
const bindingForm = reactive({
  i_id_raw: '',
  rule_group: '',
  display_name: '',
})
const quickFixModalOpen = ref(false)
const bulkRunModalOpen = ref(false)
const quickFixRecord = ref<ProductManagementRecord | null>(null)
const quickFixSyncERP = ref(false)
const activeCostRun = ref<CostRecalculationRun | null>(null)
const costRunLoading = ref(false)
const costRunWorking = ref(false)
const costRunError = ref('')
const calculator = reactive({
  rule_group: '',
  width: undefined as number | undefined,
  height: undefined as number | undefined,
  quantity: 1,
  process: '',
})
const calculatorPreview = ref<CostRulePreviewResponse | null>(null)
const calculatorLoading = ref(false)
const calculatorError = ref('')
const syncPollTokens = new Map<number, number>()
const PREVIEW_RESOLVE_CONCURRENCY = 4
const COST_RUN_POLL_INTERVAL = 1800
let loadRecordsAbort: AbortController | null = null
let loadRecordsSeq = 0
let recordPreviewResolveSeq = 0
let candidatePreviewResolveSeq = 0
let costRunPollTimer: ReturnType<typeof setTimeout> | null = null
let costRulePreviewTimer: ReturnType<typeof setTimeout> | null = null
const syncableRecords = computed(() => records.value.filter((item) => item.can_sync_erp))
const canUseCostTools = computed(() => COST_TOOL_ACTIONS.some((action) => can(action)))
const totalPages = computed(() => Math.max(1, Math.ceil((pagination.total || 0) / Math.max(1, pagination.page_size || filters.page_size))))
const remainingPages = computed(() => Math.max(0, totalPages.value - pagination.page))
const hasPreviousPage = computed(() => pagination.page > 1)
const hasNextPage = computed(() => pagination.page < totalPages.value)
const visiblePageNumbers = computed(() => {
  const total = totalPages.value
  const current = Math.min(Math.max(1, pagination.page), total)
  const size = 5
  const start = Math.max(1, Math.min(current - Math.floor(size / 2), total - size + 1))
  const end = Math.min(total, start + size - 1)
  const pages: number[] = []
  for (let page = start; page <= end; page += 1) {
    pages.push(page)
  }
  return pages
})
const visibleGroups = computed<ProductManagementComboGroup[]>(() => {
  if (filters.display_scope === 'single') {
    return records.value.map(productManagementSingleGroup)
  }
  const groups = comboGroups.value ?? []
  if (filters.display_scope === 'combo') {
    return groups.filter((group) => group.group_type === 'combo')
  }
  return groups
})
const displayScopeLabel = computed(() => {
  if (filters.display_scope === 'combo') return '组合装'
  if (filters.display_scope === 'single') return '单品 SKU'
  return '条目'
})
const emptyMessage = computed(() => {
  if (records.value.length === 0) return '暂无符合条件的产品记录。'
  if (filters.display_scope === 'combo') return '当前页暂无组合装条目，可切换为单品 SKU 或全部查看。'
  if (filters.display_scope === 'single') return '当前页暂无单品 SKU 条目。'
  return '暂无可展示的产品条目。'
})
const comboSyncSummaryText = computed(() => {
  const state = comboSyncSummary.value
  if (!state) return ''
  if (state.status === 'failed') {
    return `组合关系同步延迟：${state.last_error || '等待自动重试'}`
  }
  if (state.last_success_at) {
    return `组合关系最近同步 ${formatDate(state.last_success_at)}`
  }
  return '组合关系正在建立本地缓存'
})
const costIssueGroups = computed(() => {
  return (Object.keys(COST_ISSUE_GROUP_LABELS) as ProductCostIssueGroupCode[]).map((code) => ({
    code,
    label: COST_ISSUE_GROUP_LABELS[code],
    count: dashboardCount(code, 'groups'),
  }))
})
const costIssueTags = computed(() => {
  return (Object.keys(COST_ISSUE_TAG_LABELS) as ProductCostIssueTagCode[]).map((code) => ({
    code,
    label: COST_ISSUE_TAG_LABELS[code],
    count: dashboardCount(code, 'tags'),
  }))
})
const costIssueTotal = computed(() => {
  const total = Number(costDashboard.value?.total_count)
  if (Number.isFinite(total) && total >= 0) return Math.floor(total)
  const groupSum = costIssueGroups.value.reduce((sum, item) => sum + item.count, 0)
  if (groupSum > 0) return groupSum
  return costIssueTags.value.reduce((sum, item) => sum + item.count, 0)
})
const costDashboardHint = computed(() => {
  if (costDashboardLoading.value) return '正在读取产品中心成本问题。'
  if (costIssueTotal.value <= 0) return '当前没有需要集中处理的成本问题。'
  return `当前有 ${costIssueTotal.value} 个 SKU 命中成本问题；同一个 SKU 可能同时命中多个标签，分组数字不要相加。`
})
const costFallbackPolicyText = computed(() => {
  const dashboard = costDashboard.value
  if (!dashboard) return '猜价策略读取中'
  if (dashboard.legacy_fallback_enabled === false || dashboard.legacy_fallback_mode === 'disabled') {
    return dashboard.legacy_fallback_warning || '未关联款式猜价已关闭'
  }
  return dashboard.legacy_fallback_warning || '未关联款式仍会按名称猜价'
})
const costUnboundRatioText = computed(() => formatRatioPercent(costDashboard.value?.legacy_fallback_ratio))
const costUnboundTrendText = computed(() => {
  const trend = costDashboard.value?.legacy_fallback_trend ?? []
  if (trend.length < 2) return '趋势数据累积中'
  const latest = trend[trend.length - 1]
  const previous = trend[trend.length - 2]
  const latestRatio = Number(latest?.legacy_fallback_ratio ?? 0)
  const previousRatio = Number(previous?.legacy_fallback_ratio ?? 0)
  const delta = latestRatio - previousRatio
  if (Math.abs(delta) < 0.0001) {
    return `较上次持平（${formatRatioPercent(latestRatio)}）`
  }
  const direction = delta > 0 ? '上升' : '下降'
  return `较上次${direction} ${formatRatioPercent(Math.abs(delta))}（当前 ${formatRatioPercent(latestRatio)}）`
})
const unboundIssueCount = computed(() => dashboardCount('unbound_iid', 'tags'))
const unboundPanelHint = computed(() => {
  if (unboundIssueCount.value <= 0) {
    return '未关联款式已清零；当前可能算错的 SKU 主要来自 ERP 不一致等其他问题。'
  }
  return '这些 SKU 目前是按名称猜价，建议先把款式关联到定价规则。'
})
const unboundEmptyText = computed(() => {
  if (unboundIssueCount.value <= 0) return '未关联款式已清零。'
  return '暂无可创建关联草稿的未关联款式。'
})
const activeCostFilterHint = computed(() => {
  if (activeCostTag.value) {
    const tag = costIssueTags.value.find((item) => item.code === activeCostTag.value)
    if (tag) return `当前筛选：${tag.label} ${tag.count} 个标签；可全选全部符合条件后生成批量修复预览。`
  }
  const group = costIssueGroups.value.find((item) => item.code === activeCostGroup.value)
  if (group) return `当前分组：${group.label} ${group.count} 个标签；下方列表仍按当前产品筛选展示。`
  return '请选择问题标签或勾选 SKU 后再批量修复。'
})
const selectedRecordCount = computed(() => Object.values(selectedRecordIds.value).filter(Boolean).length)
const canCreateBulkRun = computed(() => bulkAllMatching.value || selectedRecordCount.value > 0)
const bulkSelectionHint = computed(() => {
  if (bulkAllMatching.value) return `已选择全部符合条件的 SKU，预计 ${costIssueTotal.value || pagination.total} 条。`
  return selectedRecordCount.value > 0 ? `已选择 ${selectedRecordCount.value} 条 SKU。` : '先勾选当前页 SKU，或选择全部符合当前筛选条件。'
})
const runItems = computed<CostRecalculationRunItem[]>(() => activeCostRun.value?.items ?? [])
const runSummary = computed(() => activeCostRun.value?.summary ?? {})
const quickFixItem = computed(() => runItems.value[0] ?? null)
const quickFixItemReason = computed(() => {
  const item = quickFixItem.value
  return firstNonEmptyString(item?.skip_reason, item?.conflict_reason)
})
const skippedRunItemCount = computed(() => runItems.value.filter((item) => item.status === 'skipped').length)
const conflictRunItemCount = computed(() => runItems.value.filter((item) => item.status === 'conflict').length)
const appliedRunItemCount = computed(() => runItems.value.filter((item) => item.status === 'applied').length)
const canApplyActiveRun = computed(() => {
  if (costRunWorking.value || costRunLoading.value) return false
  const status = activeCostRun.value?.status
  return status === 'previewed' || status === 'partially_applied'
})
const canSyncActiveRunERP = computed(() => {
  if (costRunWorking.value || costRunLoading.value) return false
  const status = activeCostRun.value?.status
  return status === 'applied' || status === 'partially_applied'
})
const costRunStatusText = computed(() => {
  const run = activeCostRun.value
  if (!run) return '正在创建修复预览。'
  return costRunStatusLabel(run.status)
})
const runConfirmationText = computed(() => {
  return firstNonEmptyString(
    runSummary.value.confirmation_text,
    `将按预览结果修改 ${runSummary.value.previewed_count ?? runItems.value.length} 条 SKU，接着可按需同步 ERP。`,
  )
})
const calculatorPreviewText = computed(() => {
  const preview = calculatorPreview.value
  if (!preview) return ''
  const parts = [
    preview.explanation ? `公式：${preview.explanation}` : '',
    preview.matched_rule_version ? `规则版本 v${preview.matched_rule_version}` : '',
    preview.requires_manual_review ? '需要人工复核' : '',
  ].filter(Boolean)
  return parts.join(' · ') || '已按当前定价规则试算。'
})

onMounted(() => {
  const keyword = route.query.keyword
  const issueScope = route.query.issue_scope
  if (typeof keyword === 'string' && keyword.trim()) {
    filters.keyword = keyword.trim()
  }
  if (issueScope === 'all' || issueScope === 'attention') {
    filters.issue_scope = issueScope
  }
  void loadRecords()
  if (canUseCostTools.value) {
    void loadCostDashboard()
    void loadRuleGroups()
  }
})

onBeforeUnmount(() => {
  loadRecordsAbort?.abort()
  loadRecordsAbort = null
  recordPreviewResolveSeq += 1
  candidatePreviewResolveSeq += 1
  syncPollTokens.clear()
  clearCostRunPoll()
  if (costRulePreviewTimer) {
    clearTimeout(costRulePreviewTimer)
    costRulePreviewTimer = null
  }
})

async function loadRecords(): Promise<void> {
  normalizeIssueScopeForExplicitSuccessFilter()
  loadRecordsAbort?.abort()
  const requestSeq = ++loadRecordsSeq
  const abortController = new AbortController()
  loadRecordsAbort = abortController
  loading.value = true
  error.value = ''
  try {
    const result = await productManagementApi.listComboTree({
      keyword: filters.keyword,
      display_scope: filters.display_scope,
      issue_scope: filters.issue_scope,
      image_source: filters.image_source,
      cost_status: filters.cost_status,
      sync_status: filters.sync_status,
      base_sync_status: filters.base_sync_status,
      image_sync_status: filters.image_sync_status,
      page: filters.page,
      page_size: filters.page_size,
    }, abortController.signal)
    if (abortController.signal.aborted || requestSeq !== loadRecordsSeq) return
    records.value = result.data ?? []
    comboGroups.value = result.groups ?? []
    comboSyncSummary.value = result.combo_sync_summary ?? null
    resetExpandedGroups()
    pagination.page = result.pagination?.page ?? filters.page
    pagination.page_size = result.pagination?.page_size ?? filters.page_size
    pagination.total = result.pagination?.total ?? records.value.length
    void resolveRecordPreviewURLs(records.value)
    if (canUseCostTools.value) {
      void loadCostDashboard()
    }
  } catch (err) {
    if (abortController.signal.aborted || requestSeq !== loadRecordsSeq) return
    error.value = errorMessage(err)
  } finally {
    if (loadRecordsAbort === abortController) {
      loadRecordsAbort = null
    }
    if (requestSeq === loadRecordsSeq) {
      loading.value = false
    }
  }
}

function costDashboardParams(): Record<string, unknown> {
  return {
    keyword: filters.keyword,
    display_scope: filters.display_scope,
    issue_scope: filters.issue_scope,
    image_source: filters.image_source,
    cost_status: filters.cost_status,
    sync_status: filters.sync_status,
    base_sync_status: filters.base_sync_status,
    image_sync_status: filters.image_sync_status,
    cost_issue_group: activeCostGroup.value,
    cost_issue_tag: activeCostTag.value || undefined,
  }
}

async function loadCostDashboard(): Promise<void> {
  if (!canUseCostTools.value) return
  costDashboardLoading.value = true
  costDashboardError.value = ''
  try {
    costDashboard.value = await productManagementApi.getCostDashboard(costDashboardParams())
    if (costToolsOpen.value) {
      void loadUnboundCandidates()
    }
  } catch (err) {
    costDashboardError.value = errorMessage(err)
  } finally {
    costDashboardLoading.value = false
  }
}

async function loadUnboundCandidates(): Promise<void> {
  unboundLoading.value = true
  try {
    const result = await productManagementApi.listUnboundCostRuleCandidates({
      page: 1,
      page_size: 8,
    })
    unboundCandidates.value = result.data ?? []
  } catch (err) {
    costDashboardError.value = errorMessage(err)
  } finally {
    unboundLoading.value = false
  }
}

async function loadRuleGroups(): Promise<void> {
  try {
    ruleGroupOptions.value = await productManagementApi.listCostRuleGroups()
  } catch (err) {
    costDashboardError.value = errorMessage(err)
  }
}

function toggleCostTools(): void {
  costToolsOpen.value = !costToolsOpen.value
  if (!costToolsOpen.value) return
  void loadCostDashboard()
  void loadUnboundCandidates()
  void loadRuleGroups()
}

function selectCostGroup(code: ProductCostIssueGroupCode): void {
  activeCostGroup.value = code
  activeCostTag.value = ''
  clearCostSelection()
  void loadCostDashboard()
}

function selectCostTag(code: ProductCostIssueTagCode | ''): void {
  activeCostTag.value = code
  clearCostSelection()
  void loadCostDashboard()
}

function dashboardCount(code: string, scope: 'groups' | 'tags'): number {
  const rows = costDashboard.value?.[scope] ?? []
  const match = rows.find((item) => item.code === code)
  const value = Number(match?.count)
  return Number.isFinite(value) && value > 0 ? Math.floor(value) : 0
}

function applyFilters(): void {
  filters.page = 1
  clearCostSelection()
  void loadRecords()
}

function applyDisplayScope(): void {
  resetExpandedGroups()
  applyFilters()
}

function normalizeIssueScopeForExplicitSuccessFilter(): void {
  if (filters.issue_scope !== 'attention') return
  if (filters.sync_status === 'synced' || filters.base_sync_status === 'synced' || filters.image_sync_status === 'synced') {
    filters.issue_scope = 'all'
  }
}

function changePage(page: number): void {
  filters.page = Math.min(Math.max(1, page), totalPages.value)
  void loadRecords()
}

function productManagementSingleGroup(record: ProductManagementRecord): ProductManagementComboGroup {
  return {
    group_key: `single:${record.id}`,
    group_type: 'single',
    children: [{ record, quantity: 1 }],
  }
}

function resetExpandedGroups(): void {
  expandedComboGroups.value = {}
}

function isComboGroupExpanded(group: ProductManagementComboGroup): boolean {
  if (group.group_type !== 'combo') return true
  return Boolean(expandedComboGroups.value[group.group_key])
}

function shouldShowGroupChildren(group: ProductManagementComboGroup): boolean {
  if (group.group_type !== 'combo') return true
  return isComboGroupExpanded(group)
}

function toggleComboGroup(group: ProductManagementComboGroup): void {
  if (group.group_type !== 'combo') return
  expandedComboGroups.value = {
    ...expandedComboGroups.value,
    [group.group_key]: !expandedComboGroups.value[group.group_key],
  }
}

function isRecordSelected(recordId: number): boolean {
  return Boolean(selectedRecordIds.value[recordId])
}

function toggleRecordSelected(recordId: number): void {
  const next = { ...selectedRecordIds.value }
  if (next[recordId]) delete next[recordId]
  else next[recordId] = true
  selectedRecordIds.value = next
  bulkAllMatching.value = false
}

function selectCurrentPageRecords(): void {
  const next: Record<number, boolean> = {}
  for (const record of records.value) {
    next[record.id] = true
  }
  selectedRecordIds.value = next
  bulkAllMatching.value = false
}

function selectAllMatchingRecords(): void {
  selectedRecordIds.value = {}
  bulkAllMatching.value = true
}

function clearCostSelection(): void {
  selectedRecordIds.value = {}
  bulkAllMatching.value = false
}

function openCostBinding(candidate?: UnboundCostRuleCandidate): void {
  bindingError.value = ''
  bindingForm.i_id_raw = firstNonEmptyString(candidate?.display_i_id, candidate?.i_id_raw, candidate?.erp_i_id, candidate?.product_i_id, candidate?.normalized_i_id)
  bindingForm.rule_group = candidate?.suggested_rule_group ?? ''
  bindingForm.display_name = candidate?.suggested_display_name ?? ''
  costBindingModalOpen.value = true
  void loadRuleGroups()
}

function closeCostBinding(): void {
  costBindingModalOpen.value = false
  bindingError.value = ''
  bindingForm.i_id_raw = ''
  bindingForm.rule_group = ''
  bindingForm.display_name = ''
}

async function saveCostBinding(): Promise<void> {
  if (!bindingForm.i_id_raw || !bindingForm.rule_group || bindingSaving.value) return
  bindingSaving.value = true
  bindingError.value = ''
  try {
    const selected = ruleGroupOptions.value.find((item) => item.rule_group === bindingForm.rule_group)
    await productManagementApi.createCostRuleBinding({
      i_id_raw: bindingForm.i_id_raw,
      rule_group: bindingForm.rule_group,
      display_name: bindingForm.display_name || selected?.display_name || bindingForm.i_id_raw,
      source: 'product_management',
      is_active: true,
    })
    closeCostBinding()
    await Promise.all([loadCostDashboard(), loadUnboundCandidates()])
  } catch (err) {
    bindingError.value = errorMessage(err)
  } finally {
    bindingSaving.value = false
  }
}

function candidateDisplayIId(candidate: UnboundCostRuleCandidate): string {
  return firstNonEmptyString(candidate.display_i_id, candidate.i_id_raw, candidate.erp_i_id, candidate.product_i_id, candidate.normalized_i_id, '未知款式')
}

function candidateImpactText(candidate: UnboundCostRuleCandidate): string {
  const skuCount = Number(candidate.sku_count ?? candidate.match_count ?? 0)
  const taskCount = Number(candidate.task_count ?? 0)
  const parts = [
    Number.isFinite(skuCount) && skuCount > 0 ? `${Math.floor(skuCount)} 个 SKU` : '',
    Number.isFinite(taskCount) && taskCount > 0 ? `${Math.floor(taskCount)} 个任务` : '',
  ].filter(Boolean)
  return parts.join(' · ') || '点击后创建关联'
}

function costRunFilters(): Record<string, unknown> {
  return {
    ...costDashboardParams(),
    page: filters.page,
    page_size: filters.page_size,
  }
}

async function openQuickFix(record: ProductManagementRecord): Promise<void> {
  quickFixRecord.value = record
  quickFixSyncERP.value = false
  quickFixModalOpen.value = true
  bulkRunModalOpen.value = false
  activeCostRun.value = null
  costRunError.value = ''
  await createCostRun({
    mode: 'single',
    product_management_record_id: record.id,
    record_ids: [record.id],
    filters: {
      sku_code: record.sku_code,
      task_no: record.task_no,
    },
    issue_group: activeCostGroup.value,
    issue_tag: activeCostTag.value,
  })
}

async function createBulkRun(): Promise<void> {
  if (!canCreateBulkRun.value) return
  bulkRunModalOpen.value = true
  quickFixModalOpen.value = false
  quickFixRecord.value = null
  activeCostRun.value = null
  costRunError.value = ''
  const recordIDs = Object.entries(selectedRecordIds.value)
    .filter(([, selected]) => selected)
    .map(([id]) => Number(id))
    .filter((id) => Number.isSafeInteger(id) && id > 0)
  await createCostRun({
    mode: bulkAllMatching.value ? 'all_matching' : 'explicit',
    record_ids: bulkAllMatching.value ? undefined : recordIDs,
    filters: costRunFilters(),
    issue_group: activeCostGroup.value,
    issue_tag: activeCostTag.value,
  })
}

async function createCostRun(payload: Parameters<typeof productManagementApi.createCostRecalculationRun>[0]): Promise<void> {
  clearCostRunPoll()
  costRunLoading.value = true
  costRunWorking.value = true
  try {
    const run = await productManagementApi.createCostRecalculationRun(payload)
    activeCostRun.value = run
    if (isCostRunBusy(run.status)) {
      scheduleCostRunPoll(run.id)
    }
  } catch (err) {
    costRunError.value = errorMessage(err)
  } finally {
    costRunLoading.value = false
    costRunWorking.value = false
  }
}

async function applyActiveCostRun(): Promise<void> {
  const run = activeCostRun.value
  if (!run || !canApplyActiveRun.value) return
  costRunWorking.value = true
  costRunError.value = ''
  try {
    const result = await productManagementApi.applyCostRecalculationRun(run.id)
    activeCostRun.value = result.run
    const shouldSyncQuickRun =
      quickFixModalOpen.value &&
      quickFixSyncERP.value &&
      (result.run.status === 'applied' || result.run.status === 'partially_applied')
    if (shouldSyncQuickRun) {
      costRunWorking.value = false
      await syncActiveCostRunERP()
      return
    }
    await afterCostRunChanged()
  } catch (err) {
    costRunError.value = errorMessage(err)
  } finally {
    costRunWorking.value = false
  }
}

async function syncActiveCostRunERP(): Promise<void> {
  const run = activeCostRun.value
  if (!run || !canSyncActiveRunERP.value) return
  costRunWorking.value = true
  costRunError.value = ''
  try {
    const result = await productManagementApi.syncCostRecalculationRunERP(run.id)
    activeCostRun.value = result.run
    if (isCostRunBusy(result.run.status)) {
      scheduleCostRunPoll(result.run.id)
    }
    await afterCostRunChanged()
  } catch (err) {
    costRunError.value = errorMessage(err)
  } finally {
    costRunWorking.value = false
  }
}

async function afterCostRunChanged(): Promise<void> {
  await Promise.all([loadRecords(), loadCostDashboard()])
}

function closeCostRunModals(): void {
  clearCostRunPoll()
  quickFixModalOpen.value = false
  bulkRunModalOpen.value = false
  quickFixRecord.value = null
  activeCostRun.value = null
  costRunError.value = ''
}

function scheduleCostRunPoll(runId: number): void {
  clearCostRunPoll()
  costRunPollTimer = setTimeout(() => {
    void pollCostRun(runId)
  }, COST_RUN_POLL_INTERVAL)
}

async function pollCostRun(runId: number): Promise<void> {
  try {
    const run = await productManagementApi.getCostRecalculationRun(runId, { page: 1, page_size: 80 })
    activeCostRun.value = run
    if (isCostRunBusy(run.status) && (quickFixModalOpen.value || bulkRunModalOpen.value)) {
      scheduleCostRunPoll(run.id)
    }
  } catch (err) {
    costRunError.value = errorMessage(err)
  }
}

function clearCostRunPoll(): void {
  if (!costRunPollTimer) return
  clearTimeout(costRunPollTimer)
  costRunPollTimer = null
}

function isCostRunBusy(status?: string): boolean {
  return status === 'previewing' || status === 'applying' || status === 'erp_syncing'
}

function costRunStatusLabel(status?: string): string {
  switch (status) {
    case 'previewing':
      return '正在生成修复预览。'
    case 'previewed':
      return '预览已生成，请确认修改。'
    case 'preview_failed':
      return '预览失败，请检查筛选条件后重试。'
    case 'applying':
      return '正在修改成本。'
    case 'applied':
      return '成本已修改。'
    case 'partially_applied':
      return '部分成本已修改，其余条目需查看冲突或跳过原因。'
    case 'erp_syncing':
      return '正在同步 ERP。'
    case 'erp_synced':
      return 'ERP 成本已同步。'
    case 'partially_erp_synced':
      return '部分 ERP 成本已同步，失败项请查看明细。'
    case 'cancelled':
      return '已取消。'
    default:
      return '等待后台返回预览。'
  }
}

function runItemStatusText(item: CostRecalculationRunItem): string {
  const reason = firstNonEmptyString(item.skip_reason, item.conflict_reason)
  const suffix = reason ? `：${reason}` : ''
  switch (item.status) {
    case 'previewed':
      return `待确认${suffix}`
    case 'applied':
      return `已修改${suffix}`
    case 'skipped':
      return `已跳过${suffix}`
    case 'conflict':
      return `有冲突${suffix}`
    case 'failed':
      return `失败${suffix}`
    case 'erp_queued':
      return `ERP 已入队${suffix}`
    case 'erp_synced':
      return `ERP 已同步${suffix}`
    case 'erp_failed':
      return `ERP 失败${suffix}`
    default:
      return `状态待确认${suffix}`
  }
}

function scheduleCostRulePreview(): void {
  if (costRulePreviewTimer) {
    clearTimeout(costRulePreviewTimer)
  }
  costRulePreviewTimer = setTimeout(() => {
    void runCostRulePreview()
  }, 350)
}

async function runCostRulePreview(): Promise<void> {
  calculatorError.value = ''
  calculatorPreview.value = null
  if (!calculator.rule_group) return
  calculatorLoading.value = true
  try {
    const width = validPositiveNumber(calculator.width)
    const height = validPositiveNumber(calculator.height)
    const quantity = validPositiveNumber(calculator.quantity) ?? 1
    const area = width && height ? Number((width * height).toFixed(6)) : undefined
    calculatorPreview.value = await productManagementApi.previewCostRule({
      rule_group: calculator.rule_group,
      width,
      height,
      area,
      quantity,
      process: calculator.process,
    })
  } catch (err) {
    calculatorError.value = errorMessage(err)
  } finally {
    calculatorLoading.value = false
  }
}

function validPositiveNumber(value: unknown): number | undefined {
  const num = Number(value)
  return Number.isFinite(num) && num > 0 ? num : undefined
}

function openTask(taskId: number): void {
  void router.push({ name: 'TaskDetail', params: { id: String(taskId) } })
}

async function openCandidates(record: ProductManagementRecord): Promise<void> {
  activeRecord.value = record
  manualAssetID.value = ''
  candidateModalOpen.value = true
  candidateLoading.value = true
  candidates.value = []
  error.value = ''
  try {
    candidates.value = await productManagementApi.listImageCandidates(record.id)
    void resolveCandidatePreviewURLs(candidates.value)
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    candidateLoading.value = false
  }
}

function closeCandidates(): void {
  candidateModalOpen.value = false
  candidates.value = []
  candidatePreviewURLs.value = {}
  activeRecord.value = null
  manualAssetID.value = ''
}

async function setManualImage(assetId: number): Promise<void> {
  if (!activeRecord.value) return
  try {
    const updated = await productManagementApi.setManualImage(activeRecord.value.id, assetId)
    replaceRecord(updated)
    closeCandidates()
  } catch (err) {
    error.value = errorMessage(err)
  }
}

async function reparseImage(record: ProductManagementRecord): Promise<void> {
  try {
    replaceRecord(await productManagementApi.reparseImage(record.id))
  } catch (err) {
    error.value = errorMessage(err)
  }
}

async function requestSync(record: ProductManagementRecord): Promise<void> {
  await requestRecordSync(record, 'all')
}

async function requestBaseSync(record: ProductManagementRecord): Promise<void> {
  await requestRecordSync(record, 'base')
}

async function requestImageSync(record: ProductManagementRecord): Promise<void> {
  await requestRecordSync(record, 'image')
}

async function requestRecordSync(record: ProductManagementRecord, scope: ProductSyncScope): Promise<void> {
  const force = Boolean(record.can_force_override)
  markRecordSyncing(record.id, scope, '已提交同步请求，等待 ERP 返回结果。')
  try {
    const next = await requestRecordSyncByScope(record.id, scope, force)
    replaceRecord(next)
    markRecordSyncing(next.id, scope, syncMessageFromRecord(next, scope))
    startRecordSyncPolling(next, scope)
  } catch (err) {
    error.value = errorMessage(err)
    markRecordSyncDone(record.id, `同步请求失败：${errorMessage(err)}`)
  }
}

async function syncCurrentPage(): Promise<void> {
  if (batchSyncing.value) return
  batchSyncing.value = true
  error.value = ''
  try {
    for (const record of syncableRecords.value) {
      await requestRecordSync(record, 'all')
      await delay(350)
    }
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    batchSyncing.value = false
  }
}

async function requestRecordSyncByScope(recordId: number, scope: ProductSyncScope, force: boolean): Promise<ProductManagementRecord> {
  if (scope === 'base') return productManagementApi.requestBaseSync(recordId, force)
  if (scope === 'image') return productManagementApi.requestImageSync(recordId, force)
  return productManagementApi.requestSync(recordId, force)
}

function markRecordSyncing(recordId: number, scope: ProductSyncScope, message: string): void {
  syncingRecordScopes.value = { ...syncingRecordScopes.value, [recordId]: scope }
  syncMessages.value = { ...syncMessages.value, [recordId]: message }
}

function markRecordSyncDone(recordId: number, message: string): void {
  const nextScopes = { ...syncingRecordScopes.value }
  delete nextScopes[recordId]
  syncingRecordScopes.value = nextScopes
  syncMessages.value = { ...syncMessages.value, [recordId]: message }
  syncPollTokens.delete(recordId)
}

function startRecordSyncPolling(record: ProductManagementRecord, scope: ProductSyncScope): void {
  const token = Date.now()
  syncPollTokens.set(record.id, token)
  void pollRecordSync(record, scope, token)
}

async function pollRecordSync(record: ProductManagementRecord, scope: ProductSyncScope, token: number): Promise<void> {
  let current = record
  for (let attempt = 0; attempt < 15; attempt += 1) {
    if (syncPollTokens.get(record.id) !== token) return
    const status = scopedSyncStatus(current, scope)
    if (isFinalSyncStatus(status)) {
      markRecordSyncDone(record.id, syncMessageFromRecord(current, scope))
      return
    }
    markRecordSyncing(record.id, scope, syncMessageFromRecord(current, scope))
    await delay(3000)
    if (syncPollTokens.get(record.id) !== token) return
    const latest = await fetchLatestRecord(current).catch(() => null)
    if (latest) {
      current = latest
      replaceRecord(latest)
    }
  }
  markRecordSyncDone(record.id, '已提交到后台处理，结果未及时返回，请稍后刷新查看。')
}

async function fetchLatestRecord(record: ProductManagementRecord): Promise<ProductManagementRecord | null> {
  const result = await productManagementApi.list({
    keyword: record.sku_code || record.task_no,
    issue_scope: 'all',
    page: 1,
    page_size: 50,
  })
  return (result.data ?? []).find((item) => item.id === record.id) ?? null
}

function delay(ms: number): Promise<void> {
  return new Promise((resolve) => window.setTimeout(resolve, ms))
}

function replaceRecord(next: ProductManagementRecord): void {
  const idx = records.value.findIndex((item) => item.id === next.id)
  if (idx >= 0) {
    records.value.splice(idx, 1, next)
  }
  comboGroups.value = comboGroups.value.map((group) => ({
    ...group,
    children: group.children.map((child) => (child.record.id === next.id ? { ...child, record: next } : child)),
  }))
  void resolveRecordPreviewURLs([next])
}

function hasCost(record: ProductManagementRecord): boolean {
  return typeof record.cost_price === 'number' && record.cost_price > 0
}

function specSummary(record: ProductManagementRecord): string {
  return firstTraceString(record.size_text, record.spec_text)
}

function hasAreaWarning(record: ProductManagementRecord): boolean {
  return Boolean(record.area_trace?.warning)
}

function areaTraceSummary(record: ProductManagementRecord): string {
  const area = record.area_trace?.area_m2
  if (typeof area === 'number' && Number.isFinite(area) && area > 0) {
    return `${formatTraceNumber(area)} ㎡`
  }
  return '面积待核对'
}

function areaTraceLines(record: ProductManagementRecord): string[] {
  const trace = record.area_trace
  if (!trace) {
    return ['暂无面积识别信息。', '请检查任务规格尺寸或在任务详情中补充结构化尺寸。']
  }
  const lines: string[] = []
  const spec = specSummary(record)
  if (spec) {
    lines.push(`规格：${spec}`)
  }
  const dimensionParts = [
    typeof trace.width_m === 'number' ? `宽 ${formatTraceNumber(trace.width_m)}m` : '',
    typeof trace.height_m === 'number' ? `高 ${formatTraceNumber(trace.height_m)}m` : '',
    typeof trace.quantity === 'number' ? `数量 ${formatTraceNumber(trace.quantity)}` : '',
  ].filter(Boolean)
  if (dimensionParts.length > 0) {
    lines.push(`尺寸：${dimensionParts.join('，')}`)
  }
  if (trace.formula) {
    lines.push(`公式：${trace.formula}`)
  } else if (typeof trace.area_m2 === 'number') {
    lines.push(`面积：${formatTraceNumber(trace.area_m2)}㎡`)
  }
  if (trace.source_label || trace.source) {
    lines.push(`来源：${trace.source_label || areaTraceSourceLabel(trace.source)}`)
  }
  if (trace.confidence) {
    lines.push(`可信度：${areaTraceConfidenceLabel(trace.confidence)}`)
  }
  if (trace.warning) {
    lines.push(`提示：${trace.warning}`)
  }
  if (lines.length === 0) {
    lines.push('暂无可展示的面积识别明细。')
  }
  return lines
}

function areaTraceSourceLabel(source?: string): string {
  switch (source) {
    case 'sku_item_variant':
      return 'SKU 子项规格'
    case 'task_detail':
      return '任务规格'
    case 'text_extractor':
      return '规格文本解析'
    case 'missing':
      return '未识别到尺寸'
    default:
      return source || '未知来源'
  }
}

function areaTraceConfidenceLabel(confidence: string): string {
  switch (confidence) {
    case 'high':
      return '高'
    case 'medium':
      return '中'
    case 'low':
      return '低'
    default:
      return confidence
  }
}

function productTraceAria(record: ProductManagementRecord): string {
  const sku = record.sku_code || '当前 SKU'
  return `查看 ${sku} 的面积和成本计算明细`
}

function costTraceLines(record: ProductManagementRecord): string[] {
  const trace: ProductManagementCostTrace | null | undefined = record.cost_trace
  if (!trace) {
    return ['暂无成本计算快照，仅显示当前保存的成本。', '如需核验，请重新触发成本计算或查看任务详情。']
  }
  const input = traceObject(trace.input_snapshot)
  const calculation = traceObject(trace.calculation_snapshot)
  const lines: string[] = []
  const ruleName = firstTraceString(trace.rule_name, traceString(calculation, 'cost_rule_name'), traceString(calculation, 'rule_name'), '未匹配到明确规则')
  const version = trace.matched_rule_version || traceNumber(calculation, 'matched_rule_version')
  lines.push(`规则：${ruleName}${version ? ` v${version}` : ''}`)

  const source = firstTraceString(trace.rule_source, trace.prefill_source, traceString(calculation, 'cost_rule_source'), traceString(calculation, 'prefill_source'))
  if (source) {
    lines.push(`来源：${costTraceSourceLabel(source)}`)
  }

  const inputLine = costTraceInputLine(input)
  if (inputLine) {
    lines.push(`输入：${inputLine}`)
  }

  const matchLine = costTraceMatchLine(input, calculation)
  if (matchLine) {
    lines.push(matchLine)
  }

  const explanation = firstTraceString(
    traceString(calculation, 'explanation'),
    traceString(calculation, 'formula'),
    traceString(calculation, 'formula_expression'),
    traceString(calculation, 'calculation_expression'),
  )
  if (explanation) {
    lines.push(`公式：${explanation}`)
  }

  const estimated = traceNumber(calculation, 'estimated_cost')
  const finalCost = firstTraceNumber(record.cost_price, traceNumber(calculation, 'cost_price'))
  const costParts = [
    estimated !== undefined ? `估算 ${formatCost(estimated)}` : '',
    finalCost !== undefined ? `最终 ${formatCost(finalCost)}` : '',
  ].filter(Boolean)
  if (costParts.length > 0) {
    lines.push(`结果：${costParts.join(' / ')}`)
  }

  const manualOverride = trace.manual_cost_override || traceBoolean(calculation, 'manual_cost_override')
  if (manualOverride) {
    const reason = firstTraceString(trace.manual_cost_override_reason, traceString(calculation, 'manual_cost_override_reason'), '未填写原因')
    lines.push(`人工覆盖：是，${reason}`)
  } else if (trace.requires_manual_review || traceBoolean(calculation, 'requires_manual_review')) {
    lines.push('状态：需要人工复核')
  } else {
    lines.push('状态：系统规则自动生成')
  }

  if (trace.snapshot_at) {
    lines.push(`快照：${formatDate(trace.snapshot_at)}`)
  }
  return lines
}

function costTraceSourceLabel(source: string): string {
  switch (source) {
    case 'cost_rule_preview':
    case 'system_auto':
    case 'governed_rule':
      return '系统计算'
    case 'cost_recalculation_run':
      return '批量修复'
    case 'manual_rule_reference':
      return '人工参考'
    case 'manual_override':
      return '人工维护'
    default:
      return source.includes('_') ? '系统计算' : source
  }
}

function costTraceInputLine(input: Record<string, unknown>): string {
  const sizeText = firstTraceString(traceString(input, 'spec_text'), traceString(input, 'size_text'))
  const width = traceNumber(input, 'width')
  const height = traceNumber(input, 'height')
  const area = traceNumber(input, 'area')
  const quantity = traceNumber(input, 'quantity')
  const category = firstTraceString(traceString(input, 'category_name'), traceString(input, 'product_i_id'), traceString(input, 'normalized_i_id'))
  const parts = [
    category ? `品类 ${category}` : '',
    sizeText ? `尺寸 ${sizeText}` : width !== undefined && height !== undefined ? `尺寸 ${formatTraceNumber(width)}x${formatTraceNumber(height)}` : '',
    area !== undefined ? `面积 ${formatTraceNumber(area)}㎡` : '',
    quantity !== undefined ? `数量 ${formatTraceNumber(quantity)}` : '',
  ].filter(Boolean)
  return parts.join('，')
}

function costTraceMatchLine(input: Record<string, unknown>, calculation: Record<string, unknown>): string {
  const matchMode = firstTraceString(traceString(calculation, 'match_mode'), traceString(input, 'match_mode'))
  const fallback = traceBoolean(calculation, 'legacy_alias_fallback') || traceBoolean(input, 'legacy_alias_fallback')
  if (fallback || matchMode === 'legacy_alias') {
    return '匹配：未关联款式（按名称猜的价）'
  }
  if (matchMode === 'binding_erp_i_id') {
    return '匹配：按 ERP 款式关联定价规则'
  }
  if (matchMode === 'binding_product_i_id') {
    return '匹配：按任务款式关联定价规则'
  }
  if (matchMode === 'no_match') {
    return '匹配：未找到可用定价规则'
  }
  return ''
}

function traceObject(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' && !Array.isArray(value) ? (value as Record<string, unknown>) : {}
}

function traceString(source: Record<string, unknown>, key: string): string {
  const value = source[key]
  return typeof value === 'string' ? value.trim() : ''
}

function traceNumber(source: Record<string, unknown>, key: string): number | undefined {
  return numberFromUnknown(source[key])
}

function traceBoolean(source: Record<string, unknown>, key: string): boolean {
  const value = source[key]
  if (typeof value === 'boolean') return value
  if (typeof value === 'number') return value !== 0
  if (typeof value === 'string') return ['1', 'true', 'yes', '是'].includes(value.trim().toLowerCase())
  return false
}

function firstTraceString(...values: Array<string | undefined | null>): string {
  for (const value of values) {
    const trimmed = typeof value === 'string' ? value.trim() : ''
    if (trimmed) return trimmed
  }
  return ''
}

function firstTraceNumber(...values: Array<number | undefined | null>): number | undefined {
  for (const value of values) {
    if (typeof value === 'number' && Number.isFinite(value)) return value
  }
  return undefined
}

function numberFromUnknown(value: unknown): number | undefined {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string') {
    const parsed = Number(value)
    return Number.isFinite(parsed) ? parsed : undefined
  }
  return undefined
}

function formatTraceNumber(value: number): string {
  return value.toFixed(4).replace(/0+$/, '').replace(/\.$/, '')
}

function productIIDLabel(record: ProductManagementRecord): string {
  return record.erp_i_id?.trim() || record.product_i_id?.trim() || '未绑定 ERP 款式'
}

function baseSyncStatus(record: ProductManagementRecord): ProductSyncStatus {
  return record.base_sync_status || record.erp_sync_status || 'pending_sync'
}

function imageSyncStatus(record: ProductManagementRecord): ProductSyncStatus {
  return record.image_sync_status || record.erp_sync_status || 'waiting_image'
}

function scopedSyncStatus(record: ProductManagementRecord, scope: ProductSyncScope): ProductSyncStatus {
  if (scope === 'base') return baseSyncStatus(record)
  if (scope === 'image') {
    if (isActiveSyncStatus(record.erp_sync_status)) return record.erp_sync_status
    return imageSyncStatus(record)
  }
  if (isActiveSyncStatus(record.erp_sync_status)) return record.erp_sync_status
  const baseStatus = baseSyncStatus(record)
  const imgStatus = imageSyncStatus(record)
  if (isActiveSyncStatus(baseStatus)) return baseStatus
  if (isActiveSyncStatus(imgStatus)) return imgStatus
  if (baseStatus === 'failed' || imgStatus === 'failed') return 'failed'
  if (baseStatus === 'waiting_image' || imgStatus === 'waiting_image') return 'waiting_image'
  if (baseStatus === 'synced' && (!record.image_required || imgStatus === 'synced')) return 'synced'
  return 'pending_sync'
}

function isActiveSyncStatus(status?: ProductSyncStatus): boolean {
  return status === 'queued' || status === 'syncing' || status === 'cooling_down'
}

function isFinalSyncStatus(status: ProductSyncStatus): boolean {
  return status === 'synced' || status === 'failed' || status === 'waiting_image'
}

function isRecordSyncing(record: ProductManagementRecord): boolean {
  return Boolean(syncingRecordScopes.value[record.id]) || isActiveSyncStatus(scopedSyncStatus(record, 'all'))
}

function syncMessageForRecord(record: ProductManagementRecord): string {
  const existing = syncMessages.value[record.id]
  if (existing) return existing
  const status = scopedSyncStatus(record, 'all')
  if (isActiveSyncStatus(status)) return syncMessageFromStatus(status)
  return ''
}

function syncActionLabel(record: ProductManagementRecord, scope: ProductSyncScope, fallback: string): string {
  if (syncingRecordScopes.value[record.id] === scope) return '同步中'
  return fallback
}

function syncMessageFromRecord(record: ProductManagementRecord, scope: ProductSyncScope): string {
  const status = scopedSyncStatus(record, scope)
  if (status === 'synced') {
    if (scope === 'base') return 'ERP 基础资料已同步成功。'
    if (scope === 'image') return 'ERP 图片已同步成功。'
    return 'ERP 基础资料和图片已同步成功。'
  }
  if (status === 'failed') {
    return firstNonEmptyString(record.image_sync_error, record.base_sync_error, record.last_sync_error, '同步失败，请查看错误信息后重试。')
  }
  if (status === 'waiting_image') return '缺少可同步的 ERP 商品图，请先上传或选择图片。'
  return syncMessageFromStatus(status)
}

function syncMessageFromStatus(status: ProductSyncStatus): string {
  if (status === 'queued') return '已进入同步队列，等待后台处理。'
  if (status === 'syncing') return '正在同步 ERP，请稍候。'
  if (status === 'cooling_down') return '同步请求处于冷却队列，系统会自动继续处理。'
  return '等待同步。'
}

function previewURLForRecord(record: ProductManagementRecord): string {
  return recordPreviewURLs.value[record.id] || directPreviewURL(record.image_preview_url) || ''
}

function previewURLForCandidate(candidate: ProductImageCandidate): string {
  return candidatePreviewURLs.value[candidate.asset_id] || directPreviewURL(candidate.preview_url) || ''
}

function assetIDForRecord(record: ProductManagementRecord): string | undefined {
  const raw = record.image_asset_id ?? assetIDFromPreviewPath(record.image_preview_url)
  const id = Number(raw)
  return Number.isSafeInteger(id) && id > 0 ? String(id) : undefined
}

function assetIDForCandidate(candidate: ProductImageCandidate): string {
  return String(candidate.asset_id)
}

function previewLoadableForRecord(record: ProductManagementRecord): boolean {
  return Boolean(assetIDForRecord(record) || previewURLForRecord(record))
}

function previewLoadableForCandidate(candidate: ProductImageCandidate): boolean {
  return Boolean(assetIDForCandidate(candidate) || previewURLForCandidate(candidate))
}

async function resolveRecordPreviewURLs(items: ProductManagementRecord[]): Promise<void> {
  const seq = ++recordPreviewResolveSeq
  const next = { ...recordPreviewURLs.value }
  await mapWithConcurrency(items, PREVIEW_RESOLVE_CONCURRENCY, async (item) => {
    if (seq !== recordPreviewResolveSeq) return
      const assetID = item.image_asset_id ?? assetIDFromPreviewPath(item.image_preview_url)
      const url = await resolveAssetPreviewURL(assetID, item.image_preview_url)
    if (seq !== recordPreviewResolveSeq) return
      if (url) next[item.id] = url
      else delete next[item.id]
  })
  if (seq !== recordPreviewResolveSeq) return
  recordPreviewURLs.value = next
}

async function resolveCandidatePreviewURLs(items: ProductImageCandidate[]): Promise<void> {
  const seq = ++candidatePreviewResolveSeq
  const next = { ...candidatePreviewURLs.value }
  await mapWithConcurrency(items, PREVIEW_RESOLVE_CONCURRENCY, async (item) => {
    if (seq !== candidatePreviewResolveSeq) return
      const url = await resolveAssetPreviewURL(item.asset_id, item.preview_url)
    if (seq !== candidatePreviewResolveSeq) return
      if (url) next[item.asset_id] = url
      else delete next[item.asset_id]
  })
  if (seq !== candidatePreviewResolveSeq) return
  candidatePreviewURLs.value = next
}

async function resolveAssetPreviewURL(assetID?: number | null, fallback?: string): Promise<string> {
  const fallbackAssetID = assetID ?? assetIDFromPreviewPath(fallback)
  const direct = directPreviewURL(fallback)
  if (direct) return direct
  if (!fallbackAssetID || fallbackAssetID <= 0) return ''
  const result = await fetchAssetPreviewMeta(String(fallbackAssetID)).catch(() => null)
  return result?.status === 'ok' && result.displayUrl ? result.displayUrl : ''
}

function directPreviewURL(raw?: string): string {
  const value = String(raw ?? '').trim()
  if (!value) return ''
  if (isAssetPreviewMetaURL(value)) return ''
  if (/^(https?:|data:|blob:)/i.test(value)) return value
  return ''
}

function isAssetPreviewMetaURL(raw: string): boolean {
  const value = raw.trim()
  if (!value) return false
  try {
    const url = new URL(value, window.location.origin)
    return /^\/v1\/assets\/\d+\/preview\b/i.test(url.pathname)
  } catch {
    return /^\/v1\/assets\/\d+\/preview\b/i.test(value)
  }
}

function assetIDFromPreviewPath(raw?: string): number | undefined {
  const value = String(raw ?? '').trim()
  let path = value
  try {
    path = new URL(value, window.location.origin).pathname
  } catch {
    path = value
  }
  const match = path.match(/\/v1\/assets\/(\d+)\/preview\b/)
  if (!match) return undefined
  const id = Number(match[1])
  return Number.isSafeInteger(id) && id > 0 ? id : undefined
}

function formatCost(value?: number | null): string {
  if (typeof value !== 'number' || value <= 0) return '待维护'
  return `￥${value.toFixed(3).replace(/0+$/, '').replace(/\.$/, '')}`
}

function formatSignedCost(value?: number | null): string {
  if (typeof value !== 'number' || !Number.isFinite(value)) return '-'
  const abs = Math.abs(value).toFixed(3).replace(/0+$/, '').replace(/\.$/, '')
  if (value > 0) return `+￥${abs}`
  if (value < 0) return `-￥${abs}`
  return '￥0'
}

function formatRatioPercent(value?: number | null): string {
  if (typeof value !== 'number' || !Number.isFinite(value)) return '0%'
  return `${(value * 100).toFixed(value > 0 && value < 0.01 ? 2 : 1).replace(/\.0$/, '')}%`
}

function formatDate(value?: string): string {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN', { hour12: false })
}

function formatQuantity(value?: number): string {
  const qty = typeof value === 'number' && value > 0 ? value : 1
  return qty.toFixed(4).replace(/0+$/, '').replace(/\.$/, '')
}

function groupTitle(group: ProductManagementComboGroup): string {
  if (group.group_type !== 'combo') {
    return group.children[0]?.record.sku_code || '单品 SKU'
  }
  return firstNonEmptyString(group.combo_sku_code, group.combo_name, '未命名组合装')
}

function groupSubtitle(group: ProductManagementComboGroup): string {
  const entityID = firstNonEmptyString(group.entity_sku_id)
  const synced = group.last_synced_at ? `同步 ${formatDate(group.last_synced_at)}` : ''
  return [
    entityID ? `实体 ${entityID}` : '',
    synced,
  ]
    .filter(Boolean)
    .join(' · ')
}

function comboParentName(group: ProductManagementComboGroup): string {
  return firstNonEmptyString(group.combo_name, group.combo_short_name)
}

function comboParentStyle(group: ProductManagementComboGroup): string {
  return firstNonEmptyString(group.erp_i_id, group.entity_sku_id, '未绑定 ERP 款式')
}

function comboParentCategory(group: ProductManagementComboGroup): string {
  const brand = firstNonEmptyString(group.brand)
  const vcName = firstNonEmptyString(group.vc_name)
  if (brand && vcName) return `${brand} / ${vcName}`
  return firstNonEmptyString(brand, vcName, '未返回品牌分类')
}

function comboParentPrice(group: ProductManagementComboGroup): string {
  const cost = formatNullablePrice(group.cost_price)
  const sale = formatNullablePrice(group.sale_price)
  if (cost && sale) return `${cost} / ${sale}`
  return firstNonEmptyString(cost, sale, '未返回价格')
}

function formatNullablePrice(value?: number | null): string {
  if (typeof value !== 'number' || !Number.isFinite(value) || value <= 0) return ''
  return `￥${value.toFixed(3).replace(/0+$/, '').replace(/\.$/, '')}`
}

function syncStatusLabel(status: ProductSyncStatus): string {
  const labels: Record<ProductSyncStatus, string> = {
    pending_sync: '待同步',
    queued: '已入队',
    syncing: '同步中',
    synced: '已同步',
    failed: '同步失败',
    cooling_down: '冷却中',
    waiting_image: '待上传 ERP 图',
  }
  return labels[status] ?? status
}

function firstNonEmptyString(...values: Array<string | undefined | null>): string {
  for (const value of values) {
    const text = String(value ?? '').trim()
    if (text) return text
  }
  return ''
}

function errorMessage(err: unknown): string {
  if (err instanceof Error && err.message) return err.message
  return '操作失败，请稍后重试。'
}
</script>

<style scoped>
.product-management-view {
  min-height: 100%;
  padding: 1.25rem clamp(0.875rem, 2vw, 1.75rem) 2.5rem;
  color: rgb(var(--yb-text));
  background: rgb(var(--yb-bg-page));
}

.pm-header,
.pm-filters,
.pm-table-shell,
.pm-modal {
  border: 1px solid rgb(var(--yb-border));
  background: rgb(var(--yb-surface));
  box-shadow: 0 8px 24px rgb(var(--yb-shadow) / 0.06);
}

.pm-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 18px;
  padding: 1.1rem 1.25rem;
  border-radius: 1rem;
}

.pm-header-actions,
.pm-cost-console-actions,
.pm-modal-actions,
.pm-bulk-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.55rem;
  flex-wrap: wrap;
}

.pm-eyebrow {
  margin: 0 0 8px;
  color: rgb(var(--yb-brand));
  font-size: 12px;
  font-weight: 800;
  letter-spacing: 0;
}

.pm-header h1,
.pm-modal h2 {
  margin: 0;
  color: rgb(var(--yb-text));
  font-size: clamp(1.65rem, 2.5vw, 2.25rem);
  font-weight: 900;
  letter-spacing: 0;
}

.pm-subtitle {
  max-width: 760px;
  margin: 0.55rem 0 0;
  color: rgb(var(--yb-text-secondary));
  line-height: 1.65;
}

.pm-filters {
  display: grid;
  grid-template-columns: minmax(18rem, 2fr) repeat(6, minmax(8rem, 1fr)) auto auto;
  gap: 0.75rem;
  margin-top: 0.85rem;
  padding: 0.9rem;
  border-radius: 0.875rem;
}

.pm-filters > .pm-btn {
  align-self: end;
}

.pm-field {
  display: grid;
  gap: 0.35rem;
  color: rgb(var(--yb-text-muted));
  font-size: 12px;
  font-weight: 800;
}

.pm-field input,
.pm-field select {
  width: 100%;
  min-height: 2.25rem;
  border: 1px solid rgb(var(--yb-border-strong));
  border-radius: 0.625rem;
  padding: 0 12px;
  color: rgb(var(--yb-text));
  background: rgb(var(--yb-surface));
  outline: none;
}

.pm-field input:focus,
.pm-field select:focus {
  border-color: rgb(var(--yb-brand));
  box-shadow: 0 0 0 3px rgb(var(--yb-brand) / 0.12);
}

.pm-btn {
  min-height: 2.25rem;
  border: 1px solid rgb(var(--yb-border-strong));
  border-radius: 0.625rem;
  padding: 0 16px;
  color: rgb(var(--yb-text-body));
  background: rgb(var(--yb-surface));
  font-weight: 800;
  cursor: pointer;
  transition:
    border-color 0.16s ease,
    background-color 0.16s ease,
    color 0.16s ease,
    box-shadow 0.16s ease;
}

.pm-btn:hover:not(:disabled) {
  border-color: rgb(var(--yb-brand-border-strong));
  color: rgb(var(--yb-brand-strong));
  background: rgb(var(--yb-brand-soft));
}

.pm-btn:disabled {
  cursor: not-allowed;
  opacity: 0.42;
}

.pm-btn--primary {
  border-color: rgb(var(--yb-brand));
  background: rgb(var(--yb-brand));
  color: rgb(var(--yb-surface));
}

.pm-btn--primary:hover:not(:disabled) {
  border-color: rgb(var(--yb-brand-strong));
  color: rgb(var(--yb-surface));
  background: rgb(var(--yb-brand-strong));
}

.pm-btn--ghost {
  background: rgb(var(--yb-surface-soft));
}

.pm-btn--cost {
  position: relative;
  border-color: rgb(var(--yb-border-strong));
  background: rgb(var(--yb-surface));
}

.pm-btn--cost.has-issues {
  border-color: rgb(var(--yb-danger-border));
  color: rgb(var(--yb-danger-text));
  background: rgb(var(--yb-danger-wash));
}

.pm-btn--cost.has-issues::after {
  content: "";
  position: absolute;
  top: 0.45rem;
  right: 0.45rem;
  width: 0.46rem;
  height: 0.46rem;
  border-radius: 999px;
  background: rgb(var(--yb-danger));
  box-shadow: 0 0 0 3px rgb(var(--yb-danger) / 0.16);
}

.pm-btn--small {
  min-height: 1.9rem;
  padding: 0 10px;
  font-size: 12px;
}

.pm-cost-console {
  display: grid;
  gap: 0.9rem;
  margin-top: 0.85rem;
  padding: 1rem;
  border: 1px solid rgb(var(--yb-border));
  border-radius: 1rem;
  background: rgb(var(--yb-surface));
  box-shadow: 0 8px 24px rgb(var(--yb-shadow) / 0.06);
}

.pm-cost-console-head,
.pm-cost-panel-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.pm-cost-console-head h2,
.pm-cost-panel h3 {
  margin: 0;
  color: rgb(var(--yb-text));
  font-size: 1.05rem;
  font-weight: 900;
}

.pm-cost-console-head p,
.pm-cost-panel-head p,
.pm-cost-note {
  margin: 0.25rem 0 0;
  color: rgb(var(--yb-text-muted));
  font-size: 13px;
  line-height: 1.55;
}

.pm-cost-groups,
.pm-cost-chips {
  display: flex;
  gap: 0.6rem;
  flex-wrap: wrap;
}

.pm-cost-policy {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.pm-cost-policy span {
  border: 1px solid rgb(var(--yb-border-strong));
  border-radius: 999px;
  padding: 0.34rem 0.65rem;
  color: rgb(var(--yb-text-muted-strong));
  background: rgb(var(--yb-surface-muted));
  font-size: 12px;
  font-weight: 850;
}

.pm-cost-group,
.pm-cost-chip {
  border: 1px solid rgb(var(--yb-border-strong));
  color: rgb(var(--yb-text-body));
  background: rgb(var(--yb-surface));
  cursor: pointer;
  font-weight: 850;
}

.pm-cost-group {
  display: grid;
  min-width: 10rem;
  grid-template-columns: 1fr auto;
  align-items: center;
  gap: 0.8rem;
  border-radius: 0.75rem;
  padding: 0.7rem 0.85rem;
  text-align: left;
}

.pm-cost-group strong {
  color: rgb(var(--yb-brand-strong));
  font-family: var(--yb-font-data);
  font-size: 1.18rem;
}

.pm-cost-chip {
  border-radius: 999px;
  padding: 0.38rem 0.72rem;
  font-size: 12px;
}

.pm-cost-group:hover,
.pm-cost-chip:hover,
.pm-cost-group.is-active,
.pm-cost-chip.is-active {
  border-color: rgb(var(--yb-brand-border-strong));
  color: rgb(var(--yb-brand-strong));
  background: rgb(var(--yb-brand-soft));
}

.pm-cost-console-grid {
  display: grid;
  grid-template-columns: minmax(18rem, 1fr) minmax(16rem, 0.9fr) minmax(22rem, 1.25fr);
  gap: 0.85rem;
  align-items: stretch;
}

.pm-cost-panel {
  display: grid;
  align-content: start;
  gap: 0.75rem;
  min-width: 0;
  border: 1px solid rgb(var(--yb-border-page-soft));
  border-radius: 0.85rem;
  padding: 0.85rem;
  background: rgb(var(--yb-surface-subtle));
}

.pm-cost-panel--calculator {
  background: rgb(var(--yb-surface-blue-subtle));
}

.pm-cost-mini-empty {
  border: 1px dashed rgb(var(--yb-border-strong));
  border-radius: 0.75rem;
  padding: 1rem;
  color: rgb(var(--yb-text-muted));
  text-align: center;
}

.pm-unbound-list {
  display: grid;
  gap: 0.55rem;
}

.pm-unbound-item {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.55rem;
  border: 1px solid rgb(var(--yb-border-strong));
  border-radius: 0.75rem;
  padding: 0.7rem;
  color: rgb(var(--yb-text));
  background: rgb(var(--yb-surface));
}

.pm-unbound-item:hover {
  border-color: rgb(var(--yb-brand-border-strong));
  background: rgb(var(--yb-surface-brand-panel));
}

.pm-unbound-main {
  display: grid;
  min-width: 0;
  gap: 0.25rem;
  border: 0;
  padding: 0;
  color: inherit;
  background: transparent;
  text-align: left;
  cursor: pointer;
}

.pm-unbound-main span,
.pm-unbound-main small {
  color: rgb(var(--yb-text-muted));
}

.pm-calculator-grid,
.pm-binding-form {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.7rem;
}

.pm-calculator-result,
.pm-quick-summary,
.pm-run-summary {
  display: flex;
  align-items: center;
  gap: 0.65rem;
  flex-wrap: wrap;
  border: 1px solid rgb(var(--yb-brand-border));
  border-radius: 0.75rem;
  padding: 0.7rem;
  color: rgb(var(--yb-text-muted-strong));
  background: rgb(var(--yb-surface));
}

.pm-calculator-result strong {
  color: rgb(var(--yb-brand-strong));
  font-family: var(--yb-font-data);
  font-size: 1.2rem;
}

.pm-calculator-result small {
  color: rgb(var(--yb-text-muted));
}

.pm-record-select,
.pm-checkline {
  display: inline-flex;
  width: fit-content;
  align-items: center;
  gap: 0.35rem;
  color: rgb(var(--yb-text-muted-strong));
  font-size: 12px;
  font-weight: 850;
}

.pm-record-select input,
.pm-checkline input {
  width: 1rem;
  height: 1rem;
  accent-color: rgb(var(--yb-brand));
}

.pm-cost-modal {
  width: min(720px, 100%);
}

.pm-cost-modal--wide {
  width: min(1080px, 100%);
}

.pm-quick-summary span,
.pm-run-summary span {
  display: grid;
  gap: 0.18rem;
  min-width: 7rem;
  color: rgb(var(--yb-text));
  font-family: var(--yb-font-data);
  font-weight: 900;
}

.pm-quick-summary b {
  color: rgb(var(--yb-text-muted));
  font-family: var(--yb-font-text);
  font-size: 12px;
}

.pm-run-table {
  display: grid;
  overflow: hidden;
  border: 1px solid rgb(var(--yb-border));
  border-radius: 0.75rem;
}

.pm-run-table-head,
.pm-run-row {
  display: grid;
  grid-template-columns: minmax(9rem, 1.3fr) repeat(4, minmax(6.5rem, 0.8fr));
  gap: 0.65rem;
  align-items: center;
  padding: 0.65rem 0.75rem;
}

.pm-run-table-head {
  color: rgb(var(--yb-text-muted-strong));
  background: rgb(var(--yb-surface-subtle));
  font-size: 12px;
  font-weight: 900;
}

.pm-run-row {
  border-top: 1px solid rgb(var(--yb-border-page-soft));
  color: rgb(var(--yb-text));
  background: rgb(var(--yb-surface));
  font-size: 13px;
}

.pm-summary {
  display: flex;
  align-items: center;
  gap: 16px;
  margin: 0.8rem 0;
  color: rgb(var(--yb-text-secondary));
}

.pm-pagination {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin: 0.9rem 0 0;
  padding: 0.85rem 1rem;
  border: 1px solid rgb(var(--yb-border));
  border-radius: 0.875rem;
  color: rgb(var(--yb-text-secondary));
  background: rgb(var(--yb-surface));
  box-shadow: 0 8px 24px rgb(var(--yb-shadow) / 0.05);
}

.pm-pagination-info {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.pm-pagination-info strong {
  color: rgb(var(--yb-text));
  font-weight: 900;
}

.pm-pagination-info span {
  color: rgb(var(--yb-text-muted-strong));
  font-size: 13px;
}

.pm-pagination-actions {
  display: flex;
  flex: 1 1 auto;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  min-width: 0;
  flex-wrap: wrap;
}

.pm-page-btn {
  min-width: 2.25rem;
  min-height: 2.25rem;
  border: 1px solid rgb(var(--yb-border-strong));
  border-radius: 0.625rem;
  padding: 0 10px;
  color: rgb(var(--yb-text-body));
  background: rgb(var(--yb-surface));
  font-weight: 900;
  cursor: pointer;
  transition:
    border-color 0.16s ease,
    background-color 0.16s ease,
    color 0.16s ease,
    box-shadow 0.16s ease;
}

.pm-page-btn:hover:not(:disabled) {
  border-color: rgb(var(--yb-brand-border-strong));
  color: rgb(var(--yb-brand-strong));
  background: rgb(var(--yb-brand-soft));
}

.pm-page-btn:disabled {
  cursor: not-allowed;
  opacity: 0.48;
}

.pm-page-btn.is-active {
  border-color: rgb(var(--yb-brand));
  color: rgb(var(--yb-surface));
  background: rgb(var(--yb-brand));
  opacity: 1;
}

.pm-page-btn--wide {
  min-width: 4rem;
}

.pm-error,
.pm-error-text {
  color: rgb(var(--yb-danger));
}

.pm-combo-sync {
  color: rgb(var(--yb-brand));
  font-weight: 800;
}

.pm-table-shell {
  overflow: visible;
  border-radius: 1rem;
}

.pm-combo-group {
  border-top: 1px solid rgb(var(--yb-border));
  background: rgb(var(--yb-surface));
}

.pm-combo-group--combo {
  background: linear-gradient(90deg, rgb(var(--yb-brand) / 0.055), rgb(var(--yb-surface)) 42%);
}

.pm-combo-header {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
  border: 0;
  border-bottom: 1px solid rgb(var(--yb-border-brand-quiet));
  padding: 1rem;
  background: linear-gradient(90deg, rgb(var(--yb-surface-blue-subtle)) 0%, rgb(var(--yb-surface-subtle)) 42%, rgb(var(--yb-surface)) 100%);
  text-align: left;
  cursor: pointer;
  transition:
    border-color 0.16s ease,
    background-color 0.16s ease,
    color 0.16s ease;
}

.pm-combo-header:hover:not(.is-static) {
  background: rgb(var(--yb-surface-blue-subtle));
}

.pm-combo-header:focus-visible {
  outline: 3px solid rgb(var(--yb-brand) / 0.18);
  outline-offset: -3px;
}

.pm-combo-header.is-static {
  cursor: default;
}

.pm-combo-primary {
  display: grid;
  flex: 1 1 auto;
  gap: 6px;
  min-width: 0;
}

.pm-combo-thumb {
  display: grid;
  flex: 0 0 92px;
  place-items: center;
  width: 92px;
  height: 66px;
  overflow: hidden;
  border: 1px solid rgb(var(--yb-brand-border));
  border-radius: 0.75rem;
  background: rgb(var(--yb-surface));
  box-shadow: 0 8px 18px rgb(var(--yb-brand) / 0.08);
}

.pm-combo-thumb img {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.pm-combo-thumb.is-missing {
  border-style: dashed;
  color: rgb(var(--yb-text-muted-strong));
  background: rgb(var(--yb-surface-subtle));
  font-size: 12px;
  font-weight: 900;
}

.pm-combo-header strong {
  display: flex;
  align-items: baseline;
  gap: 10px;
  overflow: hidden;
  color: rgb(var(--yb-text-navy));
  font-size: 15px;
  font-weight: 900;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pm-combo-code {
  flex: 0 0 auto;
  color: rgb(var(--yb-text-navy));
  font-family: var(--yb-font-data);
  font-size: 16px;
  font-weight: 950;
}

.pm-combo-name {
  min-width: 0;
  overflow: hidden;
  color: rgb(var(--yb-text-deep));
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pm-combo-header small {
  overflow: hidden;
  color: rgb(var(--yb-text-muted-strong));
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pm-combo-properties {
  display: block;
  overflow: hidden;
  max-width: 68rem;
  color: rgb(var(--yb-text-soft));
  font-size: 12px;
  font-weight: 800;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pm-combo-kicker {
  margin: 0;
  color: rgb(var(--yb-brand));
  font-size: 11px;
  font-weight: 900;
}

.pm-combo-field {
  display: grid;
  gap: 2px;
  min-width: 7rem;
  max-width: 13rem;
  border: 1px solid rgb(var(--yb-brand-subtle));
  border-radius: 0.75rem;
  padding: 7px 10px;
  color: rgb(var(--yb-text-navy));
  background: rgb(var(--yb-surface) / 0.86);
  font-size: 12px;
  font-weight: 900;
}

.pm-combo-field b {
  color: rgb(var(--yb-text-muted-strong));
  font-size: 11px;
  font-weight: 900;
}

.pm-combo-count {
  flex: 0 0 auto;
  border: 1px solid rgb(var(--yb-brand-border));
  border-radius: 999px;
  padding: 5px 10px;
  color: rgb(var(--yb-brand-strong));
  background: rgb(var(--yb-brand-soft));
  font-size: 12px;
  font-weight: 900;
}

.pm-combo-meta {
  display: flex;
  flex: 0 0 min(48%, 680px);
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  min-width: 0;
  flex-wrap: wrap;
}

.pm-combo-toggle {
  flex: 0 0 auto;
  border: 1px solid rgb(var(--yb-border-strong));
  border-radius: 999px;
  padding: 5px 10px;
  color: rgb(var(--yb-text-soft));
  background: rgb(var(--yb-surface));
  font-size: 12px;
  font-weight: 900;
}

.pm-combo-header.is-expanded .pm-combo-toggle {
  border-color: rgb(var(--yb-brand-border-strong));
  color: rgb(var(--yb-brand-strong));
  background: rgb(var(--yb-brand-subtle));
}

.pm-table-head,
.pm-row {
  display: grid;
  grid-template-columns: 9.5rem minmax(8.5rem, 0.9fr) minmax(18rem, 1.6fr) minmax(8.25rem, 0.75fr) minmax(8rem, 0.8fr) minmax(8.5rem, 0.8fr) minmax(13rem, 0.9fr);
  gap: 0.9rem;
  align-items: center;
}

.pm-table-head {
  padding: 0.75rem 1rem;
  color: rgb(var(--yb-text-muted-strong));
  background: rgb(var(--yb-surface-subtle));
  font-size: 12px;
  font-weight: 900;
}

.pm-row {
  position: relative;
  padding: 0.9rem 1rem;
  border-top: 1px solid rgb(var(--yb-border-page-soft));
  background: rgb(var(--yb-surface));
}

.pm-row:hover {
  z-index: 3;
  background: rgb(var(--yb-surface-brand-panel));
}

.pm-image-cell,
.pm-main-cell,
.pm-info-cell,
.pm-sync-cell,
.pm-actions {
  display: grid;
  gap: 8px;
  min-width: 0;
}

.pm-preview {
  display: grid;
  place-items: center;
  width: 8rem;
  height: 5.25rem;
  overflow: hidden;
  border: 1px solid rgb(var(--yb-border-muted-blue));
  border-radius: 0.75rem;
  background: rgb(var(--yb-surface-subtle));
  color: rgb(var(--yb-danger));
  font-size: 12px;
  font-weight: 800;
  text-align: center;
}

.pm-preview :deep(.pm-preview-apm),
.pm-preview :deep(.apm),
.pm-preview :deep(.apm-img),
.pm-preview :deep(.pm-preview-img) {
  width: 100%;
  height: 100%;
  object-fit: contain;
  background: rgb(var(--yb-surface));
}

.pm-preview :deep(.apm-placeholder),
.pm-preview :deep(.apm-empty) {
  min-height: 0;
  height: 100%;
  border: 0;
  border-radius: 0;
  padding: 0.25rem;
}

.pm-preview--missing {
  border-style: dashed;
}

.pm-mono,
.pm-cost-value,
.pm-area-value {
  font-family: var(--yb-font-data);
}

.pm-main-cell strong,
.pm-info-cell strong {
  overflow: hidden;
  color: rgb(var(--yb-text));
  font-weight: 900;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pm-main-cell small,
.pm-info-cell small,
.pm-sync-cell small {
  color: rgb(var(--yb-text-muted));
}

.pm-cost-cell {
  position: relative;
  display: flex;
  width: min(11.75rem, 100%);
  max-width: 100%;
  min-width: 0;
  flex-direction: column;
  gap: 0.45rem;
  padding: 0.5rem 0.55rem 0.55rem;
  border: 1px solid rgb(var(--yb-border-page-soft));
  border-radius: 0.5rem;
  color: rgb(var(--yb-text));
  background: rgb(var(--yb-surface));
  font-family: var(--yb-font-text);
  font-weight: 900;
  box-shadow: inset 3px 0 0 rgb(var(--yb-success-soft));
}

.pm-cost-topline {
  position: relative;
  display: flex;
  min-width: 0;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  padding-bottom: 0.4rem;
  border-bottom: 1px solid rgba(var(--yb-border-comma), 0.58);
}

.pm-spec-chip {
  display: inline-flex;
  flex: 1 1 auto;
  min-width: 0;
  max-width: 100%;
  align-items: center;
  overflow: hidden;
  padding: 0.16rem 0.44rem;
  border: 1px solid rgb(var(--yb-border-muted-blue));
  border-radius: 999px;
  color: rgb(var(--yb-text-soft));
  background: rgb(var(--yb-surface-blue-subtle));
  font-family: var(--yb-font-text);
  font-size: 0.7rem;
  font-weight: 850;
  line-height: 1.15;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pm-spec-chip--empty {
  border-color: rgba(var(--yb-border-comma), 0.78);
  color: rgb(var(--yb-text-muted));
  background: rgb(var(--yb-surface));
}

.pm-metric-stack {
  display: grid;
  min-width: 0;
  gap: 0.28rem;
}

.pm-metric-row {
  position: relative;
  display: grid;
  width: 100%;
  grid-template-columns: 2.35rem minmax(0, 1fr);
  align-items: center;
  column-gap: 0.4rem;
  min-width: 0;
}

.pm-cost-row {
  grid-template-columns: 2.35rem minmax(0, 1fr);
  align-items: baseline;
}

.pm-area-row {
  opacity: 0.86;
}

.pm-metric-label {
  color: rgb(var(--yb-text-muted-strong));
  font-family: var(--yb-font-text);
  font-size: 0.7rem;
  font-weight: 780;
  line-height: 1;
  white-space: nowrap;
}

.pm-area-value {
  min-width: 0;
  overflow: hidden;
  color: rgb(var(--yb-text-soft));
  font-size: 0.82rem;
  font-weight: 760;
  line-height: 1;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pm-cost-value {
  min-width: 0;
  overflow: hidden;
  color: rgb(var(--yb-text));
  font-size: 1.08rem;
  font-weight: 820;
  line-height: 1.05;
  letter-spacing: 0;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pm-detail-wrap {
  position: relative;
  flex: 0 0 auto;
  display: inline-flex;
}

.pm-detail-help {
  display: inline-flex;
  width: auto;
  min-width: 2.45rem;
  height: 1.35rem;
  align-items: center;
  justify-content: center;
  padding: 0 0.48rem;
  border: 1px solid rgb(var(--yb-border-muted-blue));
  border-radius: 999px;
  color: rgb(var(--yb-text-soft));
  background: rgb(var(--yb-surface));
  box-shadow: none;
  cursor: help;
  font-family: var(--yb-font-text);
  font-size: 0.7rem;
  font-weight: 820;
  line-height: 1;
  outline: none;
  white-space: nowrap;
}

.pm-detail-help:hover,
.pm-detail-help:focus-visible {
  border-color: rgb(var(--yb-brand-border-strong));
  color: rgb(var(--yb-brand-strong));
  background: rgb(var(--yb-brand-soft));
}

.pm-cost-popover {
  pointer-events: none;
  position: absolute;
  top: calc(100% + 0.5rem);
  right: 0;
  z-index: 40;
  display: grid;
  width: min(24rem, 48vw);
  min-width: 19rem;
  max-height: 20rem;
  gap: 0.38rem;
  overflow: auto;
  padding: 0.78rem 0.85rem;
  border: 1px solid rgb(var(--yb-border-strong));
  border-radius: 0.65rem;
  color: rgb(var(--yb-text));
  background: rgb(var(--yb-surface));
  box-shadow: 0 18px 38px rgba(var(--yb-shadow-comma), 0.16);
  opacity: 0;
  text-align: left;
  transform: translateY(-0.25rem) scale(0.98);
  transition: opacity 0.14s ease, transform 0.14s ease;
}

.pm-cost-popover strong {
  display: block;
  justify-self: stretch;
  color: rgb(var(--yb-brand-strong));
  font-family: var(--yb-font-text);
  font-size: 0.82rem;
  line-height: 1.2;
  text-align: left;
}

.pm-cost-popover strong:not(:first-child) {
  margin-top: 0.25rem;
}

.pm-cost-popover span {
  display: block;
  justify-self: stretch;
  color: rgb(var(--yb-text-muted-strong));
  font-family: var(--yb-font-text);
  font-size: 0.76rem;
  font-weight: 720;
  line-height: 1.42;
  text-align: left;
}

.pm-detail-wrap:hover .pm-cost-popover,
.pm-detail-wrap:focus-within .pm-cost-popover {
  opacity: 1;
  transform: translateY(0) scale(1);
}

.pm-cost-cell.is-missing {
  border-color: rgb(var(--yb-danger-border));
  background: rgb(var(--yb-danger-wash));
  box-shadow: inset 3px 0 0 rgb(var(--yb-danger-border));
}

.pm-cost-cell.is-missing .pm-cost-value {
  color: rgb(var(--yb-danger));
}

.pm-cost-cell.is-missing .pm-detail-help {
  border-color: rgb(var(--yb-danger-border));
  color: rgb(var(--yb-danger-text));
  background: rgb(var(--yb-danger-wash));
  box-shadow: 0 7px 18px rgba(var(--yb-danger-comma), 0.12);
}

.pm-cost-cell.has-area-warning .pm-area-value {
  color: rgb(var(--yb-warning));
}

.pm-link {
  width: fit-content;
  border: 0;
  padding: 0;
  color: rgb(var(--yb-brand));
  background: transparent;
  font-weight: 800;
  cursor: pointer;
  text-align: left;
}

.pm-pill {
  width: fit-content;
  max-width: 100%;
  border: 1px solid rgb(var(--yb-brand-subtle));
  border-radius: 999px;
  padding: 4px 9px;
  color: rgb(var(--yb-brand-strong));
  background: rgb(var(--yb-brand-soft));
  font-size: 12px;
  font-weight: 900;
  white-space: nowrap;
}

.pm-source--manual {
  border-color: rgb(var(--yb-warning-yellow));
  color: rgb(var(--yb-warning-strong));
  background: rgb(var(--yb-warning-highlight));
}

.pm-source--delivery {
  border-color: rgb(var(--yb-success-border-bright));
  color: rgb(var(--yb-success-deep));
  background: rgb(var(--yb-success-soft));
}

.pm-source--derived_preview {
  border-color: rgb(var(--yb-info-cyan-border-strong));
  color: rgb(var(--yb-info-deep));
  background: rgb(var(--yb-info-soft));
}

.pm-source--task_reference {
  border-color: rgb(var(--yb-purple-border-strong));
  color: rgb(var(--yb-purple-text-deep));
  background: rgb(var(--yb-purple-soft-strong));
}

.pm-source--erp_product_image {
  border-color: rgb(var(--yb-teal-border-strong));
  color: rgb(var(--yb-teal));
  background: rgb(var(--yb-teal-mint));
}

.pm-source--auto_on_close {
  border-color: rgb(var(--yb-brand-border-strong));
  color: rgb(var(--yb-brand-strong));
  background: rgb(var(--yb-brand-subtle));
}

.pm-source--missing,
.pm-sync--failed {
  border-color: rgb(var(--yb-danger-border));
  color: rgb(var(--yb-danger-text));
  background: rgb(var(--yb-danger-soft));
}

.pm-sync--synced {
  border-color: rgb(var(--yb-success-border-bright));
  color: rgb(var(--yb-success-deep));
  background: rgb(var(--yb-success-soft));
}

.pm-sync--queued,
.pm-sync--syncing,
.pm-sync--pending_sync {
  border-color: rgb(var(--yb-brand-border));
  color: rgb(var(--yb-brand-strong));
  background: rgb(var(--yb-brand-soft));
}

.pm-sync--cooling_down {
  border-color: rgb(var(--yb-warning-border-soft));
  color: rgb(var(--yb-warning-dark));
  background: rgb(var(--yb-warning-soft));
}

.pm-sync--waiting_image {
  border-color: rgb(var(--yb-warning-border-warm));
  color: rgb(var(--yb-warning-orange-text));
  background: rgb(var(--yb-warning-orange-soft));
}

.pm-sync-progress {
  position: relative;
  width: min(13rem, 100%);
  height: 6px;
  overflow: hidden;
  border-radius: 999px;
  background: rgb(var(--yb-surface-blue-muted));
}

.pm-sync-progress span {
  position: absolute;
  inset: 0;
  width: 45%;
  border-radius: inherit;
  background: linear-gradient(90deg, rgb(var(--yb-brand)), rgb(var(--yb-info-sky)));
  animation: pm-sync-flow 1.1s ease-in-out infinite;
}

.pm-sync-message {
  color: rgb(var(--yb-brand-strong));
  font-weight: 800;
}

@keyframes pm-sync-flow {
  0% {
    transform: translateX(-110%);
  }
  100% {
    transform: translateX(230%);
  }
}

@media (prefers-reduced-motion: reduce) {
  .pm-sync-progress span {
    animation: none;
    transform: none;
    width: 100%;
  }
}

.pm-actions {
  grid-template-columns: repeat(2, minmax(5.5rem, 1fr));
}

.pm-empty {
  padding: 36px;
  color: rgb(var(--yb-text-muted));
  text-align: center;
}

.pm-modal-mask {
  position: fixed;
  inset: 0;
  z-index: 80;
  display: grid;
  place-items: center;
  padding: 24px;
  background: rgb(var(--yb-shadow) / 0.42);
}

.pm-modal {
  width: min(980px, 100%);
  max-height: min(760px, 88dvh);
  overflow: auto;
  border-radius: 1rem;
  padding: 22px;
}

.pm-modal header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 18px;
}

.pm-manual-asset {
  display: grid;
  grid-template-columns: minmax(220px, 1fr) auto;
  gap: 12px;
  align-items: end;
  margin-bottom: 16px;
  padding: 14px;
  border: 1px solid rgb(var(--yb-brand-border));
  border-radius: 0.875rem;
  background: rgb(var(--yb-brand-soft));
}

.pm-candidate-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(190px, 1fr));
  gap: 14px;
}

.pm-candidate {
  display: grid;
  gap: 8px;
  border: 1px solid rgb(var(--yb-border-strong));
  border-radius: 0.875rem;
  padding: 12px;
  color: rgb(var(--yb-text));
  background: rgb(var(--yb-surface));
  text-align: left;
  cursor: pointer;
}

.pm-candidate:hover {
  border-color: rgb(var(--yb-brand));
  background: rgb(var(--yb-surface-brand-panel));
}

.pm-candidate :deep(.pm-candidate-apm),
.pm-candidate :deep(.apm) {
  width: 100%;
  aspect-ratio: 4 / 3;
}

.pm-candidate :deep(.apm-img),
.pm-candidate :deep(.pm-candidate-img) {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.pm-candidate :deep(.pm-candidate-apm),
.pm-candidate :deep(.apm-placeholder),
.pm-candidate :deep(.apm-empty) {
  border: 1px solid rgb(var(--yb-border));
  border-radius: 0.75rem;
  background: rgb(var(--yb-surface-subtle));
}

.pm-candidate :deep(.apm-placeholder),
.pm-candidate :deep(.apm-empty) {
  min-height: 0;
  height: auto;
  aspect-ratio: 4 / 3;
  padding: 0.25rem;
}

.pm-candidate strong {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pm-candidate small {
  color: rgb(var(--yb-text-muted));
}

@media (max-width: 1320px) {
  .pm-filters {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .pm-field--wide {
    grid-column: span 2;
  }

  .pm-table-head {
    display: none;
  }

  .pm-combo-header {
    align-items: flex-start;
  }

  .pm-pagination {
    align-items: flex-start;
    flex-direction: column;
  }

  .pm-pagination-actions {
    justify-content: flex-start;
  }

  .pm-combo-meta {
    flex-wrap: wrap;
    justify-content: flex-end;
  }

  .pm-row {
    grid-template-columns: 170px repeat(2, minmax(0, 1fr));
    align-items: start;
  }

  .pm-actions {
    grid-column: 2 / -1;
    grid-template-columns: repeat(4, minmax(86px, max-content));
  }

  .pm-cost-console-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 760px) {
  .product-management-view {
    padding-inline: 12px;
  }

  .pm-header,
  .pm-modal header {
    display: grid;
  }

  .pm-filters,
  .pm-row,
  .pm-manual-asset {
    grid-template-columns: 1fr;
  }

  .pm-field--wide,
  .pm-actions {
    grid-column: auto;
  }

  .pm-actions {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .pm-cost-popover {
    top: calc(100% + 0.65rem);
    right: auto;
    left: 0;
    width: min(22rem, calc(100vw - 2rem));
    min-width: min(22rem, calc(100vw - 2rem));
    transform: translateY(0) translateX(-0.25rem) scale(0.98);
  }

  .pm-detail-wrap:hover .pm-cost-popover,
  .pm-detail-wrap:focus-within .pm-cost-popover {
    transform: translateY(0) translateX(0) scale(1);
  }

  .pm-combo-header {
    flex-wrap: wrap;
  }

  .pm-combo-thumb {
    flex-basis: 64px;
    width: 64px;
    height: 46px;
  }

  .pm-preview {
    width: 100%;
    height: 160px;
  }

  .pm-unbound-item {
    grid-template-columns: 1fr;
  }
}
</style>
