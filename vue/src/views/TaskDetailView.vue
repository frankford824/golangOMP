<template>
  <div class="task-detail-view">
    <!-- 失序 / CAS 冲突横幅 -->
    <SequenceGapBanner
      :visible="syncStore.sequenceGap"
      @recalibrate="onRecalibrate"
    />
    <CASConflictModal
      :model-value="syncStore.versionConflict"
      :client-version="syncStore.clientVersion"
      :server-version="syncStore.serverVersion"
      @update:model-value="syncStore.clearConflict()"
      @use-server="syncStore.clearConflict"
    />

    <!-- 三态包裹 -->
    <AsyncStateWrapper
      :loading="detailLoading || storeLoading"
      :error="detailError || storeError"
      :empty="!task"
      empty-title="任务不存在"
      empty-description="该任务可能已被删除，请返回任务列表"
      :skeleton-rows="6"
      @retry="loadTask"
    >
      <template #empty-action>
        <BaseButton variant="secondary" size="sm" @click="navigateBackToTaskList">
          返回任务列表
        </BaseButton>
      </template>
      <div v-if="task" class="detail-shell detail-shell--v6">
        <!-- Step 87：创建成功轻量提示 -->
        <div
          v-if="createSuccessBannerVisible"
          class="create-success-banner"
          :class="createSuccessBannerTone"
        >
          <span>{{ createSuccessMessage }}</span>
          <button type="button" class="banner-dismiss" @click="dismissCreateBanner">×</button>
        </div>
        <div
          v-if="createPrefillSyncWarningVisible"
          class="create-success-banner banner-warning"
        >
          <span>{{ createPrefillSyncWarningMessage }}</span>
          <button type="button" class="banner-dismiss" @click="dismissCreateBanner">×</button>
        </div>
        <div
          v-if="createProcurementSyncWarningVisible"
          class="create-success-banner banner-warning"
        >
          <span>{{ createProcurementSyncWarningMessage }}</span>
          <button type="button" class="banner-dismiss" @click="dismissCreateBanner">×</button>
        </div>
        <div
          v-if="createRetouchRequirementUploadWarningVisible"
          class="create-success-banner banner-warning"
        >
          <span>{{ createRetouchRequirementUploadWarningMessage }}</span>
          <button type="button" class="banner-dismiss" @click="dismissCreateBanner">×</button>
        </div>
        <!-- V4：圆角卡片 + 模块内就地操作；右侧仅动态与评论 -->
        <div class="detail-v6-surface">
        <!-- Pencil 对齐：任务身份 + 流程 + 全局动作 -->
        <header class="detail-top-unified detail-top-v6">
          <p v-if="actionError" class="action-error">{{ actionError }}</p>
          <p v-if="actionSuccess" class="action-success">{{ actionSuccess }}</p>
          <div class="detail-top-grid">
            <div class="detail-top-left">
              <div class="detail-top-identity">
                <h1 class="detail-top-taskno">
                  {{ task.taskNo }}<template v-if="detailHeadlineSuffix"> · {{ detailHeadlineSuffix }}</template>
                </h1>
                <div class="detail-top-badge-row">
                  <span v-if="detailShowTypeBadge" class="detail-top-type-pill">{{
                    detailTypeLabel
                  }}</span>
                  <span
                    class="detail-top-priority-pill"
                    :class="
                      detailPriorityTone === 'danger'
                        ? 'detail-top-priority-pill--danger'
                        : 'detail-top-priority-pill--muted'
                    "
                    >{{ detailPriorityLabel }}</span
                  >
                </div>
                <p class="detail-top-sub">
                  {{ headerSubtitle || detailTypeLabel }} · 创建：{{ detailCreatorLabel }} · 截止：{{ detailDueLabel }}
                </p>
                <p class="detail-top-current detail-top-status-pill">
                  <span class="detail-top-status-dot" aria-hidden="true" />
                  当前状态：{{ headerStatusLabel }}，基础信息仍可由运营编辑
                </p>
                <p v-if="isBatchTask" class="detail-top-batch-pill">
                  批量任务：{{ task.batchItemCount ?? batchSkuItems.length }} 个子项
                </p>
              </div>
            </div>
            <div class="detail-top-mid">
              <div
                class="detail-top-flow-shell"
                role="region"
                aria-label="主流程"
              >
                <WorkflowProgress variant="horizontal" :task="task" />
              </div>
            </div>
            <div class="detail-top-right">
              <div class="detail-top-actions">
                <BaseButton
                  type="button"
                  class="detail-top-chip"
                  variant="ghost"
                  size="sm"
                  @click="navigateBackToTaskList"
                >
                  <ArrowLeft class="detail-top-chip-icon" aria-hidden="true" />
                  返回
                </BaseButton>
                <BaseButton
                  type="button"
                  class="detail-top-chip"
                  variant="ghost"
                  size="sm"
                  @click="refreshDetail"
                >
                  <RotateCcw class="detail-top-chip-icon" aria-hidden="true" />
                  刷新
                </BaseButton>
                <BaseButton
                  v-if="task && !isTempId"
                  type="button"
                  class="detail-top-chip"
                  variant="ghost"
                  size="sm"
                  :loading="aiSummaryLoading"
                  :disabled="aiSummaryLoading"
                  @click="openAiSummary"
                >
                  <Sparkles class="detail-top-chip-icon" aria-hidden="true" />
                  AI 摘要
                </BaseButton>
                <BaseButton
                  v-if="task && !isTempId"
                  type="button"
                  class="detail-top-chip"
                  variant="ghost"
                  size="sm"
                  @click="eventLogOpen = true"
                >
                  <ScrollText class="detail-top-chip-icon" aria-hidden="true" />
                  事件日志
                </BaseButton>
                <BaseButton
                  v-if="showErpFilingRetryButton"
                  type="button"
                  class="detail-top-chip"
                  variant="secondary"
                  size="sm"
                  :loading="erpFilingRetrying"
                  :disabled="erpFilingRetrying"
                  @click="onErpFilingRetry"
                >
                  <RefreshCcw class="detail-top-chip-icon" aria-hidden="true" />
                  重试同步
                </BaseButton>
                <BaseButton
                  v-if="canAccessPage('task_assets')"
                  type="button"
                  class="detail-top-chip"
                  variant="ghost"
                  size="sm"
                  @click="openTaskAssetsPage"
                >
                  <Images class="detail-top-chip-icon" aria-hidden="true" />
                  任务资产页
                </BaseButton>
                <button
                  v-if="canCancelTask"
                  type="button"
                  class="detail-top-chip detail-top-chip--danger"
                  @click="openCancel = true"
                >
                  <XCircle class="detail-top-chip-icon" aria-hidden="true" />
                  终止任务
                </button>
                <button
                  v-if="can('task.close') && canShowCloseTaskButton"
                  type="button"
                  class="detail-top-chip detail-top-chip--primary"
                  @click="doClose"
                >
                  <CheckCircle2 class="detail-top-chip-icon" aria-hidden="true" />
                  结单
                </button>
              </div>
            </div>
          </div>
        </header>

        <section v-if="taskPredictionSuggestions.length" class="detail-prediction-panel">
          <div class="detail-prediction-head">
            <div>
              <p>预测提示</p>
              <h2>系统建议的下一步</h2>
            </div>
            <button type="button" :disabled="taskPredictionLoading" @click="loadTaskPredictions">
              {{ taskPredictionLoading ? '刷新中' : '刷新提示' }}
            </button>
          </div>
          <div class="detail-prediction-list">
            <button
              v-for="item in taskPredictionSuggestions"
              :key="item.id"
              type="button"
              class="detail-prediction-item"
              @click="handleTaskPrediction(item)"
            >
              <span>{{ item.source || '流程状态' }}</span>
              <strong>{{ item.title }}</strong>
              <small v-if="item.detail">{{ item.detail }}</small>
              <em>{{ item.action_label || '查看' }}</em>
            </button>
          </div>
        </section>

        <!-- 主区 V3：业务模块纵向展开，操作留在对应模块内部 -->
        <main class="detail-main detail-main-v6">
          <div class="detail-body-v6">
            <div class="detail-v3-layout">
              <section class="detail-v3-main" aria-label="任务详情模块">
                <section class="detail-v3-module detail-v3-module--basic" aria-label="基础信息与运营创建侧">
                  <div class="detail-v3-module-head">
                    <div>
                      <p class="detail-v3-eyebrow">运营创建侧</p>
                      <h2 class="detail-v3-module-title">
                        {{ isBatchTask ? '母任务基础信息' : '基础信息 / 商品需求' }}
                      </h2>
                    </div>
                    <div class="detail-v3-module-actions">
                      <button type="button" class="detail-v3-link-chip" @click="eventLogOpen = true">
                        设计提交审核时间线
                      </button>
                      <BaseButton
                        v-if="canEditBasicInfo"
                        class="detail-v3-edit-base-btn"
                        variant="secondary"
                        size="sm"
                        @click="openBasicEdit"
                      >
                        {{ isBatchTask ? '编辑母任务' : '编辑信息' }}
                      </BaseButton>
                      <button
                        v-if="canUploadReferenceFromOps"
                        type="button"
                        class="detail-v3-soft-chip"
                        @click="triggerReferenceUploadFromDetail"
                      >
                        重传参考图
                      </button>
                    </div>
                  </div>
                  <div class="detail-v3-info-grid">
                    <article v-if="!isBatchTask" class="detail-v3-info-card detail-v3-info-card--product">
                      <p class="detail-v3-card-kicker">商品快照</p>
                      <dl class="detail-v3-kv-list">
                        <div>
                          <dt>SKU</dt>
                          <dd>{{ detailSkuLabel }}</dd>
                        </div>
                        <div>
                          <dt>产品</dt>
                          <dd>{{ detailProductNameLabel }}</dd>
                        </div>
                        <div>
                          <dt>分类</dt>
                          <dd>{{ detailCategoryLabel }}</dd>
                        </div>
                        <div>
                          <dt>规格尺寸</dt>
                          <dd>{{ detailSpecLabel }}</dd>
                        </div>
                      </dl>
                    </article>
                    <article v-else class="detail-v3-info-card detail-v3-info-card--product">
                      <p class="detail-v3-card-kicker">母任务信息</p>
                      <dl class="detail-v3-kv-list">
                        <div>
                          <dt>任务号</dt>
                          <dd>{{ task.taskNo }}</dd>
                        </div>
                        <div>
                          <dt>任务类型</dt>
                          <dd>{{ detailTypeLabel }}</dd>
                        </div>
                        <div>
                          <dt>创建人</dt>
                          <dd>{{ detailCreatorLabel }}</dd>
                        </div>
                        <div>
                          <dt>创建时间</dt>
                          <dd>{{ formatDateOnlyBeijing(task.createdAt) }}</dd>
                        </div>
                        <div>
                          <dt>更新时间</dt>
                          <dd>{{ formatDateOnlyBeijing(task.updatedAt) }}</dd>
                        </div>
                        <div>
                          <dt>子项数量</dt>
                          <dd>{{ task.batchItemCount ?? batchSkuItems.length }}</dd>
                        </div>
                      </dl>
                    </article>

                    <article class="detail-v3-info-card">
                      <p class="detail-v3-card-kicker">{{ detailRequirementKicker }}</p>
                      <p class="detail-v3-card-text">{{ detailRequirementLabel }}</p>
                    </article>

                    <RetouchRequirementsBlock
                      v-if="showRetouchRequirementsBlock"
                      :requirements="task!.retouchRequirements ?? []"
                      :task-title="detailRetouchDownloadTitle"
                      class="detail-v3-retouch-requirements"
                    />

                    <article class="detail-v3-info-card">
                      <p class="detail-v3-card-kicker">运营备注</p>
                      <p class="detail-v3-card-text">{{ detailNoteLabel }}</p>
                    </article>

                    <article
                      class="detail-v3-info-card detail-v3-info-card--refs"
                      :class="{ 'detail-v3-file-drop-active': isActiveDetailUploadTarget('reference') }"
                      :tabindex="canUploadReferenceFromOps ? 0 : undefined"
                      @focusin="activateDetailFileReceiver('reference')"
                      @pointerenter="activateDetailFileReceiver('reference')"
                      @dragover.prevent="onDetailUploadDragOver('reference', $event)"
                      @drop.prevent="onDetailUploadDrop('reference', $event)"
                      @paste="onDetailUploadPaste('reference', $event)"
                    >
                      <p class="detail-v3-card-kicker">
                        {{ isBatchTask ? '全部参考图汇总（母任务）' : '参考图 / 附件' }}
                      </p>
                      <p class="detail-v3-card-text">{{ detailReferenceLabel }}</p>
                      <AssetThumbStrip
                        v-if="opsReferenceThumbItems.length > 0"
                        :items="opsReferenceThumbItems"
                        empty-text="暂无参考图"
                        size="md"
                      />
                      <div
                        v-if="
                          canUploadReferenceFromOps ||
                          opsReferenceThumbItems.length > 0
                        "
                        class="detail-v3-ref-actions"
                      >
                        <input
                          ref="opsReferenceUploadInputRef"
                          type="file"
                          :accept="UPLOAD_ACCEPT_ATTRIBUTE"
                          multiple
                          class="detail-v3-hidden-file-input"
                          @change="handleOpsReferenceUpload"
                        />
                        <button
                          v-if="canUploadReferenceFromOps"
                          type="button"
                          class="detail-v3-upload-ref-btn"
                          @focusin="activateDetailFileReceiver('reference')"
                          @pointerenter="activateDetailFileReceiver('reference')"
                          @click="opsReferenceUploadInputRef?.click()"
                        >
                          上传/拖拽/粘贴参考图
                        </button>
                        <button
                          v-if="opsReferenceThumbItems.length > 0"
                          type="button"
                          class="detail-v3-link-btn"
                          @click="focusReferenceSectionFromDetail"
                        >
                          查看全部
                        </button>
                        <button
                          v-if="totalReferenceCount > 0"
                          type="button"
                          class="detail-v3-link-btn"
                          :disabled="referenceBatchDownloading"
                          @click="handleReferenceBatchDownload"
                        >
                          {{ referenceBatchDownloading ? '打包中...' : `下载全部参考图（${totalReferenceCount}）` }}
                        </button>
                      </div>
                      <p v-if="opsReferenceUploadStatus" class="detail-v3-ref-status">{{ opsReferenceUploadStatus }}</p>
                      <p v-if="opsReferenceUploadError" class="detail-v3-ref-error">{{ opsReferenceUploadError }}</p>
                      <p v-if="referenceBatchDownloadStatus" class="detail-v3-ref-status">{{ referenceBatchDownloadStatus }}</p>
                      <p v-if="referenceBatchDownloadError" class="detail-v3-ref-error">{{ referenceBatchDownloadError }}</p>
                    </article>

                    <article class="detail-v3-info-card">
                      <p class="detail-v3-card-kicker">任务设置</p>
                      <dl class="detail-v3-kv-list">
                        <div>
                          <dt>优先级</dt>
                          <dd :class="{ 'detail-v3-danger': detailPriorityTone === 'danger' }">
                            {{ detailPriorityLabel }}
                          </dd>
                        </div>
                        <div>
                          <dt>截止</dt>
                          <dd>{{ detailDueLabel }}</dd>
                        </div>
                        <div>
                          <dt>归属</dt>
                          <dd>{{ detailOwnerLabel }}</dd>
                        </div>
                      </dl>
                    </article>

                    <article v-if="showCostInDetail" class="detail-v3-info-card detail-v3-info-card--cost">
                      <p class="detail-v3-card-kicker">成本 / 采购</p>
                      <dl class="detail-v3-kv-list">
                        <div>
                          <dt>成本</dt>
                          <dd>{{ detailCostLabel }}</dd>
                        </div>
                        <div>
                          <dt>成本模式</dt>
                          <dd>{{ detailCostModeLabel }}</dd>
                        </div>
                        <div>
                          <dt>数量</dt>
                          <dd>{{ detailQuantityLabel }}</dd>
                        </div>
                        <div>
                          <dt>成本状态</dt>
                          <dd>{{ detailCostStatusLabel }}</dd>
                        </div>
                        <div>
                          <dt>覆盖原因</dt>
                          <dd>{{ detailCostOverrideReasonLabel }}</dd>
                        </div>
                        <div>
                          <dt>最新操作</dt>
                          <dd>{{ detailCostLatestActionLabel }}</dd>
                        </div>
                        <div v-if="detailErpSyncStatusLabel">
                          <dt>ERP 同步</dt>
                          <dd :class="detailErpSyncStatusToneClass">{{ detailErpSyncStatusLabel }}</dd>
                        </div>
                        <div v-if="detailErpSyncFailureMessage">
                          <dt>同步失败原因</dt>
                          <dd class="detail-erp-sync-error">{{ detailErpSyncFailureMessage }}</dd>
                        </div>
                      </dl>
                      <div v-if="showErpFilingRetryButton" class="detail-v3-erp-retry-row">
                        <BaseButton
                          variant="secondary"
                          size="sm"
                          :loading="erpFilingRetrying"
                          :disabled="erpFilingRetrying"
                          @click="onErpFilingRetry"
                        >
                          重试同步
                        </BaseButton>
                      </div>
                    </article>

                    <article
                      v-if="showProductManagementPanel"
                      class="detail-v3-info-card detail-product-management-card"
                    >
                      <input
                        ref="productManagementUploadInput"
                        class="detail-product-management-file-input"
                        type="file"
                        accept="image/*,.jpg,.jpeg,.png,.webp,.gif"
                        @change="onProductManagementImagePicked"
                      />
                      <div class="detail-product-management-head">
                        <div>
                          <p class="detail-v3-card-kicker">ERP 商品资料</p>
                          <strong>{{ productManagementRecords.length }} 个 SKU 对照记录</strong>
                        </div>
                        <button type="button" class="detail-v3-link-btn" @click="openProductManagement()">
                          进入产品管理
                        </button>
                      </div>
                      <p v-if="isPurchaseTask" class="detail-v3-card-text detail-product-management-hint">
                        采购任务不会自动产生设计成品图；如需同步 ERP 图片，请上传 ERP 商品图。
                      </p>
                      <p v-if="productManagementLoading" class="detail-v3-card-text">产品管理状态加载中...</p>
                      <p v-else-if="productManagementError" class="detail-v3-ref-error">{{ productManagementError }}</p>
                      <div v-else class="detail-product-management-list">
                        <div
                          v-for="record in productManagementPreviewRecords"
                          :key="record.id"
                          class="detail-product-management-item"
                        >
                          <div
                            class="detail-product-management-preview"
                            :class="{ 'is-missing': !productManagementPreviewLoadable(record) }"
                          >
                            <AssetPreviewMedia
                              v-if="productManagementPreviewLoadable(record)"
                              :asset-id="productManagementPreviewAssetID(record) || null"
                              :resolved-preview-url="productManagementPreviewURL(record) || null"
                              :fallback-src="directProductManagementPreviewURL(record.image_preview_url) || null"
                              :alt="record.sku_code"
                              img-class="detail-product-management-apm"
                              inner-img-class="detail-product-management-img"
                              @open-full="(url, context) => openProductManagementImagePreview(record, url, context)"
                            />
                            <span v-else>待补图</span>
                          </div>
                          <div class="detail-product-management-meta">
                            <strong>{{ record.sku_code || '-' }}</strong>
                            <small>款式：{{ productManagementERPIID(record) }}</small>
                            <small>{{ formatProductManagementCost(record) }} · {{ record.image_source_label }}</small>
                            <small
                              v-if="record.image_sync_source === 'auto_on_close' && record.image_sync_status === 'synced'"
                              class="detail-product-management-success"
                            >
                              已在结单后自动同步 ERP 图片
                            </small>
                            <small v-else-if="record.image_sync_status === 'waiting_image'" class="detail-product-management-warning">
                              {{ record.image_missing_reason || '未找到最终成品图，可上传 ERP 商品图' }}
                            </small>
                            <small class="detail-product-management-sync">
                              基础资料：{{ productManagementSyncStatusLabel(record.base_sync_status || record.erp_sync_status) }}
                            </small>
                            <small class="detail-product-management-sync">
                              ERP 图片：{{ productManagementSyncStatusLabel(record.image_sync_status || record.erp_sync_status) }}
                            </small>
                            <small v-if="record.base_sync_error" class="detail-product-management-error">{{ record.base_sync_error }}</small>
                            <small v-if="record.image_sync_error" class="detail-product-management-error">{{ record.image_sync_error }}</small>
                          </div>
                          <div class="detail-product-management-actions">
                            <button type="button" class="detail-v3-link-btn" @click="openProductManagement(record)">
                              打开
                            </button>
                            <button type="button" class="detail-v3-link-btn" @click="openProductManagement(record)">
                              选图
                            </button>
                            <button
                              type="button"
                              class="detail-v3-link-btn"
                              :disabled="!record.can_maintain_image || productManagementUploadingID === record.id"
                              @click="startProductManagementImageUpload(record)"
                            >
                              {{ productManagementUploadingID === record.id ? '上传中' : '上传 ERP 图' }}
                            </button>
                            <button
                              type="button"
                              class="detail-v3-link-btn"
                              :disabled="!record.can_maintain_image"
                              @click="reparseProductManagementImage(record)"
                            >
                              重解析
                            </button>
                            <button
                              type="button"
                              class="detail-v3-link-btn"
                              :disabled="!record.can_sync_erp"
                              @click="syncProductManagementBaseRecord(record)"
                            >
                              同步基础
                            </button>
                            <button
                              type="button"
                              class="detail-v3-link-btn"
                              :disabled="!record.can_sync_erp || !record.image_asset_id"
                              @click="syncProductManagementImageRecord(record)"
                            >
                              同步图片
                            </button>
                          </div>
                        </div>
                      </div>
                    </article>
                  </div>
                  <p class="detail-v3-module-note">
                    运营创建者、运营管理员可在设计提交审核前维护创建侧信息与参考图；参考图入口位于当前卡片内。
                  </p>
                </section>

                <SkuItemsTable
                  v-if="isBatchTask && batchSkuItems.length > 0"
                  :items="batchSkuItems"
                  :filing-status="task.filing_status ?? null"
                  :can-edit="canEditSkuItemInfo"
                  :can-upload-design="canDirectSkuDesignUpload"
                  :upload-design-label="skuUploadDesignLabel"
                  :disabled-upload-title="skuUploadDisabledTitle"
                  @edit="openSkuItemEdit"
                  @upload-design="openSkuDesignUpload"
                />

                <section
                  v-if="!isPurchaseTask"
                  class="detail-v3-module detail-v3-module--design"
                  aria-label="设计或定制模块"
                >
                  <div class="detail-v3-module-head">
                    <div>
                      <p class="detail-v3-eyebrow">{{ designModuleEyebrow }}</p>
                      <h2 class="detail-v3-module-title">{{ designModuleTitle }}</h2>
                    </div>
                    <div class="detail-v3-module-actions">
                      <span class="detail-v3-state-pill detail-v3-state-pill--purple">
                        {{ designStatusText }}
                      </span>
                      <BaseButton
                        v-if="showAssignDesignerButton"
                        variant="secondary"
                        size="sm"
                        :title="assignDesignerTitle"
                        @click="doAssign"
                      >
                        {{ assignDesignerLabel }}
                      </BaseButton>
                      <BaseButton
                        v-if="showReassignDesignerButton"
                        variant="secondary"
                        size="sm"
                        :title="reassignDesignerTitle"
                        @click="doReassign"
                      >
                        {{ reassignDesignerLabel }}
                      </BaseButton>
                    </div>
                  </div>
                  <div v-if="isBatchTask && batchSkuItems.length > 1" class="batch-sku-switcher">
                    <span class="batch-sku-switcher-label">切换商品</span>
                    <div class="batch-sku-tabs">
                      <button
                        v-for="(skuItem, idx) in batchSkuItems"
                        :key="`design-sku-${skuItem.id ?? idx}`"
                        type="button"
                        class="batch-sku-tab"
                        :class="{ 'batch-sku-tab--active': detailProductIndex === idx }"
                        @click="detailProductIndex = idx"
                      >
                        {{ skuItem.skuCode?.trim() || `子项 ${idx + 1}` }}
                      </button>
                    </div>
                  </div>
                  <div class="detail-v3-workflow-grid detail-v3-workflow-grid--design">
                    <article class="detail-v3-info-card detail-v3-info-card--wide">
                      <DesignAssetBlock />
                    </article>
                    <template v-if="isDesignOrRetouchModuleResultState">
                      <article class="detail-v3-info-card">
                        <p class="detail-v3-card-kicker">
                          {{ designOwnerLabel }}
                        </p>
                        <p class="detail-v3-card-text">{{ detailDesignerLabel }}</p>
                        <p class="detail-v3-card-muted">组：{{ detailOwnerLabel }}</p>
                      </article>
                      <article class="detail-v3-info-card">
                        <p class="detail-v3-card-kicker">
                          {{ designAssetVersionLabel }}
                        </p>
                        <p class="detail-v3-card-text">
                          {{ isRetouchTask ? retouchVersionSummary : designVersionSummary }}
                        </p>
                        <p class="detail-v3-card-muted">
                          {{ designAssetVersionHint }}
                        </p>
                      </article>
                      <article class="detail-v3-info-card">
                        <p class="detail-v3-card-kicker">结果状态</p>
                        <p class="detail-v3-card-text">{{ designStatusText }}</p>
                      </article>
                      <article class="detail-v3-info-card detail-v3-info-card--refs">
                        <p class="detail-v3-card-kicker">资产操作</p>
                        <p class="detail-v3-card-text">
                          在上方{{ designAssetPreviewAreaLabel }}区预览或下载当前版本文件。
                        </p>
                        <button
                          type="button"
                          class="detail-v3-link-btn"
                          @click="openTaskAssetsPage"
                        >
                          打开任务资产页
                        </button>
                      </article>
                    </template>
                    <template v-else>
                      <article class="detail-v3-info-card">
                        <p class="detail-v3-card-kicker">{{ designOwnerLabel }}</p>
                        <p class="detail-v3-card-text">{{ detailDesignerLabel }}</p>
                        <p class="detail-v3-card-muted">组：{{ detailOwnerLabel }}</p>
                      </article>
                      <article class="detail-v3-info-card">
                        <p class="detail-v3-card-kicker">{{ uploadDesignCardTitle }}</p>
                        <p class="detail-v3-card-text">请在上方"{{ designAssetPanelName }}"区域上传文件</p>
                      </article>
                      <article class="detail-v3-info-card">
                        <p class="detail-v3-card-kicker">{{ designAssetVersionLabel }}</p>
                        <p class="detail-v3-card-text">{{ designVersionSummary }}</p>
                      </article>
                      <article class="detail-v3-info-card detail-v3-info-card--refs">
                        <template v-if="isRetouchTask">
                          <p class="detail-v3-card-kicker">精修任务操作</p>
                          <template v-if="showRetouchClaimAction">
                            <p class="detail-v3-card-text">精修任务尚未领取，请先领取后再上传设计稿并提交。</p>
                            <button
                              type="button"
                              class="detail-v3-dark-btn"
                              :disabled="actionLoading === 'claim-retouch'"
                              @click="claimRetouchFromDetail"
                            >
                              {{ actionLoading === 'claim-retouch' ? '领取中...' : '领取精修任务' }}
                            </button>
                          </template>
                          <template v-else>
                            <p class="detail-v3-card-text">
                              请在上方"{{ designAssetPanelName }}"区域上传精修稿后点击"提交精修"按钮完成任务。
                            </p>
                          </template>
                        </template>
                        <template v-else>
                          <p class="detail-v3-card-kicker">{{ submitAuditCardTitle }}</p>
                          <p class="detail-v3-card-text">
                            请在上方“{{ designAssetPanelName }}”区域选择交付文件后{{ submitAuditActionText }}。
                          </p>
                          <p class="detail-v3-card-muted">{{ submitAuditCardHint }}</p>
                        </template>
                      </article>
                    </template>
                  </div>
                </section>

                <section
                  v-if="!isPurchaseTask && !isRetouchTask"
                  class="detail-v3-module detail-v3-module--audit"
                  aria-label="审核模块"
                >
                  <div class="detail-v3-module-head">
                    <div>
                      <p class="detail-v3-eyebrow">{{ auditModuleEyebrow }}</p>
                      <h2 class="detail-v3-module-title">{{ auditModuleTitle }}</h2>
                    </div>
                    <span class="detail-v3-state-pill detail-v3-state-pill--warning">
                      {{ auditModulePillText }}
                    </span>
                  </div>
                  <div v-if="isBatchTask && batchSkuItems.length > 1" class="batch-sku-switcher">
                    <span class="batch-sku-switcher-label">切换商品</span>
                    <div class="batch-sku-tabs">
                      <button
                        v-for="(skuItem, idx) in batchSkuItems"
                        :key="`audit-sku-${skuItem.id ?? idx}`"
                        type="button"
                        class="batch-sku-tab"
                        :class="{ 'batch-sku-tab--active': detailProductIndex === idx }"
                        @click="detailProductIndex = idx"
                      >
                        {{ skuItem.skuCode?.trim() || `子项 ${idx + 1}` }}
                      </button>
                    </div>
                  </div>
                  <div class="detail-v3-workflow-grid detail-v3-workflow-grid--audit">
                    <article class="detail-v3-info-card">
                      <p class="detail-v3-card-kicker">{{ auditPendingAssetTitle }}</p>
                      <p class="detail-v3-card-text">{{ designVersionSummary }}</p>
                      <p class="detail-v3-card-muted">点击预览 / 下载 / 对比历史版本</p>
                      <AssetThumbStrip
                        :items="auditPendingThumbItems"
                        empty-text="暂无可审核稿件"
                        size="sm"
                      />
                    </article>
                    <article class="detail-v3-info-card detail-v3-info-card--audit-comment">
                      <p class="detail-v3-card-kicker">审核意见</p>
                      <BaseSelect
                        v-model="auditRejectReasonCategory"
                        label="驳回分类"
                        placeholder="打回时请选择"
                        :options="AUDIT_REJECT_REASON_OPTIONS"
                        :disabled="!showActiveAuditActionButtons || Boolean(actionLoading)"
                        clearable
                      />
                      <BaseTextarea
                        v-model="auditComment"
                        :placeholder="auditRejectReasonCategory === AUDIT_REJECT_REASON_OTHER ? '填写其他具体理由...' : '填写通过说明或补充修改建议...'"
                        :rows="4"
                        :disabled="!showActiveAuditActionButtons || Boolean(actionLoading)"
                        :error="auditCommentError"
                      />
                    </article>
                    <article class="detail-v3-info-card detail-v3-info-card--audit">
                      <p class="detail-v3-card-kicker">{{ auditActionCardTitle }}</p>
                      <p class="detail-v3-card-text">{{ auditActionDescription }}</p>
                      <div v-if="showActiveAuditActionButtons" class="detail-v3-inline-actions">
                        <button
                          type="button"
                          class="detail-v3-dark-btn"
                          :disabled="actionLoading === 'audit-pass'"
                          @click="passAuditFromDetail"
                        >
                          {{ actionLoading === 'audit-pass' ? '通过中...' : approveButtonLabel }}
                        </button>
                        <button
                          type="button"
                          class="detail-v3-danger-btn"
                          :disabled="actionLoading === 'audit-reject'"
                          @click="rejectAuditFromDetail"
                        >
                          {{ actionLoading === 'audit-reject' ? '打回中...' : rejectButtonLabel }}
                        </button>
                      </div>
                      <p v-else class="detail-v3-card-muted">当前不在审核处理阶段，仅展示审核结果与稿件。</p>
                      <div v-if="canUploadAuditAssets" class="detail-v3-audit-upload">
                        <p class="detail-v3-card-muted">审核上传仅允许 source 修订源文件或 delivery 最终成品图。</p>
                        <input
                          ref="auditSourceUploadInputRef"
                          type="file"
                          :accept="UPLOAD_ACCEPT_ATTRIBUTE"
                          multiple
                          class="detail-v3-hidden-file-input"
                          @change="(event) => handleAuditAssetUpload(event, 'source')"
                        />
                        <input
                          ref="auditDeliveryUploadInputRef"
                          type="file"
                          accept="image/*,.jpg,.jpeg,.png,.webp"
                          multiple
                          class="detail-v3-hidden-file-input"
                          @change="(event) => handleAuditAssetUpload(event, 'delivery')"
                        />
                        <div class="detail-v3-inline-actions">
                          <button
                            type="button"
                            class="detail-v3-light-btn"
                            :disabled="actionLoading === 'audit-upload'"
                            @focusin="activateDetailFileReceiver('audit-source')"
                            @pointerenter="activateDetailFileReceiver('audit-source')"
                            @dragover.prevent="onDetailUploadDragOver('audit-source', $event)"
                            @drop.prevent="onDetailUploadDrop('audit-source', $event)"
                            @paste="onDetailUploadPaste('audit-source', $event)"
                            @click="auditSourceUploadInputRef?.click()"
                          >
                            上传/拖拽/粘贴修订源文件
                          </button>
                          <button
                            type="button"
                            class="detail-v3-dark-btn"
                            :disabled="actionLoading === 'audit-upload'"
                            @focusin="activateDetailFileReceiver('audit-delivery')"
                            @pointerenter="activateDetailFileReceiver('audit-delivery')"
                            @dragover.prevent="onDetailUploadDragOver('audit-delivery', $event)"
                            @drop.prevent="onDetailUploadDrop('audit-delivery', $event)"
                            @paste="onDetailUploadPaste('audit-delivery', $event)"
                            @click="auditDeliveryUploadInputRef?.click()"
                          >
                            上传/拖拽/粘贴最终成品图
                          </button>
                        </div>
                        <p v-if="auditAssetUploadStatus" class="detail-v3-ref-status">{{ auditAssetUploadStatus }}</p>
                        <p v-if="auditAssetUploadError" class="detail-v3-ref-error">{{ auditAssetUploadError }}</p>
                      </div>
                    </article>
                  </div>
                </section>

                <section
                  v-if="!isRetouchTask"
                  class="detail-v3-module detail-v3-module--warehouse"
                  aria-label="仓库模块"
                >
                  <div class="detail-v3-module-head">
                    <div>
                      <p class="detail-v3-eyebrow">仓库侧</p>
                      <h2 class="detail-v3-module-title">仓库接收 / 归档</h2>
                    </div>
                    <span class="detail-v3-state-pill detail-v3-state-pill--success">
                      接收入库 / 退回 / 归档
                    </span>
                  </div>
                  <div class="detail-v3-workflow-grid detail-v3-workflow-grid--warehouse">
                    <article class="detail-v3-info-card">
                      <p class="detail-v3-card-kicker">接收信息</p>
                      <p class="detail-v3-card-text">仓库：{{ warehouseStatusText }}</p>
                      <p class="detail-v3-card-muted">收货数量、库位、备注在这里填写。</p>
                      <AssetThumbStrip
                        :items="warehouseProofThumbItems"
                        empty-text="暂无入库凭证图"
                        size="sm"
                      />
                    </article>
                    <article class="detail-v3-info-card">
                      <p class="detail-v3-card-kicker">入库信息</p>
                      <div class="detail-v3-fake-textarea">填写库位、收货数量、备注...</div>
                    </article>
                    <article class="detail-v3-info-card detail-v3-info-card--warehouse">
                      <p class="detail-v3-card-kicker">仓库动作</p>
                      <p class="detail-v3-card-text">仓库接收后确认无误，可在此完成结单。</p>
                      <div
                        v-if="showWarehouseActionButtons || showWarehouseCompleteActionButton"
                        class="detail-v3-inline-actions"
                      >
                        <button
                          v-if="showWarehouseReceiveActionButtons"
                          type="button"
                          class="detail-v3-dark-btn"
                          :disabled="actionLoading === 'warehouse-receive'"
                          @click="receiveWarehouseFromDetail"
                        >
                          {{ actionLoading === 'warehouse-receive' ? '接收中...' : '接收入库' }}
                        </button>
                        <button
                          v-if="showWarehouseReturnActionButton"
                          type="button"
                          class="detail-v3-light-btn"
                          :disabled="actionLoading === 'warehouse-reject'"
                          @click="rejectWarehouseFromDetail"
                        >
                          {{ actionLoading === 'warehouse-reject' ? '退回中...' : '退回' }}
                        </button>
                        <button
                          v-if="showWarehouseCompleteActionButton"
                          type="button"
                          class="detail-v3-dark-btn"
                          :disabled="actionLoading === 'warehouse-archive'"
                          @click="archiveWarehouseFromDetail"
                        >
                          {{
                            actionLoading === 'warehouse-archive'
                              ? '完成中...'
                              : '完成仓库处理'
                          }}
                        </button>
                      </div>
                      <p v-else class="detail-v3-card-muted">当前不在仓库可操作阶段，仅展示仓库状态。</p>
                    </article>
                  </div>
                </section>
              </section>

              <aside class="detail-v3-side" aria-label="任务动态与评论">
                <div class="detail-v3-side-head">
                  <p class="detail-v3-side-kicker">活动</p>
                  <h2 class="detail-v3-side-title">任务动态</h2>
                  <p class="detail-v3-side-desc">
                    右侧只承载辅助信息，不放主业务操作。
                  </p>
                </div>

                <div v-if="sideEventsLoading" class="detail-v3-side-empty">事件加载中...</div>
                <div v-else-if="sideEventsError" class="detail-v3-side-empty">{{ sideEventsError }}</div>
                <div v-else-if="!sideEventsView.length" class="detail-v3-side-empty">暂无事件</div>
                <div v-else class="detail-v3-side-events">
                  <article
                    v-for="ev in sideEventsView"
                    :key="ev.id"
                    class="detail-v3-side-event"
                  >
                    <p class="detail-v3-side-event-title">{{ ev.headline }}</p>
                    <p v-if="ev.subline" class="detail-v3-side-event-desc">{{ ev.subline }}</p>
                  </article>
                </div>

              </aside>
            </div>
          </div>
        </main>
        </div>
      </div>
    </AsyncStateWrapper>

    <!-- 指派设计师弹窗 -->
    <DesignerSelectDialog
      v-model="assignDialogVisible"
      :designers="designerOptions"
      :loading="designersLoading"
      :title="assignDialogTitle"
      :description="assignDialogDescription"
      :loading-label="assignDialogLoadingLabel"
      :empty-hint="assignDialogEmptyHint"
      :confirm-label="assignDialogConfirmLabel"
      :assignee-role-label="assignDialogAssigneeRoleLabel"
      :current-assignee-id="
        task?.designerId != null
          ? String(task.designerId)
          : task?.assigneeId != null
            ? String(task.assigneeId)
            : null
      "
      @confirm="onAssignConfirm"
    />

    <EventLogDrawer v-model="eventLogOpen" :task-id="taskId" />

    <BaseModal
      v-model="aiSummaryOpen"
      title="任务全链路 AI 摘要"
      :show-confirm="false"
      panel-class="max-w-5xl"
    >
      <section class="ai-summary-modal">
        <div v-if="aiSummaryLoading" class="ai-summary-loading" role="status">
          <div class="ai-summary-loading-dot" aria-hidden="true" />
          <div>
            <p class="ai-summary-loading-title">正在生成摘要</p>
            <p class="ai-summary-loading-sub">系统正在读取任务、SKU、资产、ERP 与成本链路。</p>
          </div>
        </div>

        <div v-else-if="aiSummaryError" class="ai-summary-error">
          <p>{{ aiSummaryError }}</p>
          <BaseButton size="sm" variant="primary" @click="loadAiSummary">重新生成</BaseButton>
        </div>

        <div v-else-if="aiSummary" class="ai-summary-content ai-summary-content--compact">
          <header class="ai-summary-hero">
            <p class="ai-summary-eyebrow">AI 处置建议</p>
            <h3>{{ aiSummaryDecision }}</h3>
            <p>{{ aiSummaryImpact }}</p>
          </header>

          <div class="ai-summary-action-grid">
            <article class="ai-summary-panel ai-summary-panel--risk">
              <h4>当前卡点</h4>
              <div class="ai-summary-blocker">
                <strong>{{ aiSummaryBlocker.title }}</strong>
                <p>{{ aiSummaryBlocker.reason || '系统暂未识别到明确原因。' }}</p>
                <span v-if="aiSummaryBlocker.owner">责任方：{{ aiSummaryBlocker.owner }}</span>
              </div>
            </article>

            <article class="ai-summary-panel">
              <h4>下一步动作</h4>
              <ol v-if="aiSummaryActionList.length" class="ai-summary-next-actions">
                <li v-for="action in aiSummaryActionList" :key="`${action.role}-${action.action}`">
                  <span>{{ action.timing || '下一步' }}</span>
                  <strong>{{ action.role || '相关责任人' }}</strong>
                  <p>{{ action.action }}</p>
                </li>
              </ol>
              <p v-else class="ai-summary-muted">系统暂未识别到明确动作。</p>
            </article>
          </div>

          <details class="ai-summary-evidence">
            <summary>查看证据</summary>
            <ul v-if="aiSummaryEvidenceLines.length">
              <li v-for="line in aiSummaryEvidenceLines" :key="line">{{ line }}</li>
            </ul>
            <p v-else>系统暂无可展示证据。</p>
          </details>
        </div>
      </section>
      <template #footer>
        <footer class="ai-summary-footer">
          <span v-if="aiSummary" class="ai-summary-meta">{{ aiSummary.model || 'AI' }} · 简短处置卡片</span>
          <div class="ai-summary-footer-actions">
            <BaseButton size="sm" variant="secondary" :disabled="aiSummaryLoading" @click="aiSummaryOpen = false">关闭</BaseButton>
            <BaseButton size="sm" variant="primary" :loading="aiSummaryLoading" :disabled="aiSummaryLoading" @click="loadAiSummary">重新生成</BaseButton>
          </div>
        </footer>
      </template>
    </BaseModal>

    <ReassignDesignerDialog
      v-model="reassignDialogVisible"
      :designers="designerOptions"
      :loading="designersLoading"
      :current-assignee-id="
        task?.designerId != null
          ? String(task.designerId)
          : task?.assigneeId != null
            ? String(task.assigneeId)
            : null
      "
      :current-assignee-name="task?.designerName ?? task?.assigneeName ?? null"
      :has-design-output-hint="hasDesignOutputHint"
      @confirm="onReassignConfirm"
    />

    <CancelReasonModal
      v-if="openCancel"
      :error-text="cancelErrorText"
      :suggest-force-close="cancelSuggestForce"
      :show-direct-force="isDeptAdminPlus"
      @close="closeCancelModal"
      @submit="submitCancel"
      @force="submitForceFromCancel"
    />

    <TaskInfoEditModal
      v-if="task"
      v-model="taskInfoEditOpen"
      :task="task"
      :product-index="detailProductIndex"
      @saved="onTaskInfoEditSaved"
    />
    <SkuItemEditModal
      v-if="task"
      v-model="skuItemEditOpen"
      :task-id="task.id"
      :sku-item="editingSkuItem"
      @saved="onSkuItemEditSaved"
    />
    <ImagePreviewLightbox
      v-model="lightboxOpen"
      :items="lightboxItems"
      :initial-index="lightboxInitialIndex"
      fallback-title="预览大图"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, provide, onMounted, onBeforeUnmount, watch, watchEffect, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useTasksStore } from '@/stores/tasks'
import { useSyncStore } from '@/stores/sync'
import {
  canCloseTaskForArchive,
  formatCloseArchiveError,
  isTaskCloseFlowTerminal,
} from '@/domain/task-close-eligibility'
import {
  canAssign,
  canAssignCustomizationArtOperator,
  canSubmitAudit,
  canUploadDesignDelivery,
  canReassignDesigner,
  isInCustomizationArtReassignmentPhase,
  isInDesignerReassignmentPhase,
  isCustomizationTask as isCustomizationTaskByDomain,
  isLegacyTaskStatusInDesignerEditablePhase,
  canMaintainTaskProductInfoAtAnyStage,
  taskHasRecordedDesignOutput,
  taskHasAssignee,
} from '@/domain/task-actions'
import {
  getTaskActionAvailability,
  shouldHideWarehouseCompleteAction,
  shouldHideWarehouseReceiveActions,
} from '@/domain/task-action-availability'
import { formatTaskActionDenyMessage } from '@/domain/task-action-deny'
import {
  canUserScheduleDesignerAssignment,
  canUserScheduleDesignerReassignment,
} from '@/domain/task-reassign-policy'
import { hasModuleAction, hasModuleActionProjection } from '@/domain/module-actions'
import { usePermission } from '@/composables/usePermission'
import { usePermissionsStore } from '@/stores/permissions'
import { useAuth } from '@/composables/useAuth'
import { useTaskCancel } from '@/composables/useTaskCancel'
import {
  getFilesFromClipboardEvent,
  getFilesFromDataTransfer,
  hasFileDataTransfer,
  useFileDropPasteReceiver,
} from '@/composables/useFileDropPasteReceiver'
import { isReferenceUrlExpiringSoon } from '@/utils/referenceUrl'
import { tasksApi } from '@/services/api/tasksApi'
import { predictionsApi, type PredictionSuggestion } from '@/services/api/predictionsApi'
import { uploadTaskReferenceFileViaAssetSession } from '@/services/api/design'
import { assetsApi, type AssetKind } from '@/services/api/assetsApi'
import {
  productManagementApi,
  type ProductManagementRecord,
  type ProductSyncStatus,
} from '@/services/api/productManagementApi'
import { fetchAssetPreviewMeta } from '@/domain/asset-access'
import type { BackendAsset } from '@/services/apiTypes'
import { uploadTaskFileViaAssetSession } from '@/services/upload/assetUploadFlow'
import { buildTimestampedZipFilename, downloadBatchAsZip, sanitizeZipEntryName } from '@/utils/batchZipDownload'
import { resolveApiUserMessage } from '@/utils/api-message-zh'
import { formatErpSyncFailureMessage } from '@/utils/business-copy'
import { TASK_DETAIL_KEY } from '@/composables/task-detail-key'
import {
  TASK_DETAIL_PRODUCT_INDEX_KEY,
  type TaskDetailProductIndexContext,
} from '@/composables/task-detail-product-index'
import {
  assetVersionMatchesActiveSku,
  parallelProductTabCount,
  selectionFromProductIndex,
  targetSkuCodeForUpload,
  taskHasSkuItemsForBatchUi,
} from '@/domain/task-batch-assets'
import { latestDeliveryBatchVersionsForSelection } from '@/domain/task-final-delivery'
import { taskCreatorDisplayName, taskDesignerDisplayName } from '@/domain/task-actors'
import { formatDateOnlyBeijing, formatMonthDayTimeBeijingOffsetAware, formatTaskDueAtDisplay } from '@/utils/date'
import {
  extractCostOverrideEventsList,
  extractTaskEventsList,
  mapCostOverrideEventToRecentEvent,
  mapTaskEventRowToRecentEvent,
} from '@/domain/mappers/task-events-from-api'
import { workflowGateReasonLabelCn } from '@/domain/mappers/read-model-labels-cn'
import { canUserRetryErpFiling, taskNeedsErpFilingRetry } from '@/domain/erp-filing-retry'
import {
  getTaskFilingStatusLabel,
  getTaskFilingStatusTone,
} from '@/utils/filing-status'
import {
  dedupeReferenceFileRefs,
  isTaskLevelBackendReferenceAsset,
  mergeReferenceFileRefsPreferBackend,
  referenceFileRefsFromBackendReferenceAssets,
} from '@/domain/mappers/reference-file-refs'
import type { ReferenceFileRef } from '@/services/api/assetsApi'
import type { RecentEvent } from '@/domain/types/dashboard'
import type { TaskSkuItem } from '@/domain/types/task'
import { formatUploadFailureMessage } from '@/utils/upload-errors'
import {
  REFERENCE_UPLOAD_MAX_FILE_SIZE_BYTES,
  REFERENCE_UPLOAD_MAX_FILE_SIZE_MB,
  isAcceptableReferenceFile,
  referenceFileTooLargeMessage,
} from '@/domain/constants/reference-upload'
import { UPLOAD_ACCEPT_ATTRIBUTE, isAllowedUploadFile, isBitmapDeliveryFile } from '@/domain/constants/upload-types'
import { DESIGN_UPLOAD_MAX_FILE_SIZE_BYTES, designUploadTooLargeMessage } from '@/domain/copy/design-upload'
import {
  ArrowLeft,
  CheckCircle2,
  Images,
  RefreshCcw,
  RotateCcw,
  ScrollText,
  Sparkles,
  XCircle,
} from 'lucide-vue-next'

import AsyncStateWrapper from '@/components/base/AsyncStateWrapper.vue'
import BaseButton from '@/components/base/BaseButton.vue'
import BaseModal from '@/components/base/BaseModal.vue'
import BaseSelect from '@/components/base/BaseSelect.vue'
import BaseTextarea from '@/components/base/BaseTextarea.vue'
import SequenceGapBanner from '@/components/business/SequenceGapBanner.vue'
import CASConflictModal from '@/components/business/CASConflictModal.vue'
import WorkflowProgress from '@/components/task/WorkflowProgress.vue'
import {
  getDesignSubStatusLabel,
  getMainTaskStatusLabel,
  getWarehouseSubStatusLabel,
} from '@/domain/enums/task-status'
import { getCustomizationDetailStatusLabel } from '@/domain/task-center-card-status'

// ── 子区块（通过 provide/inject 访问 task，无 prop drilling）──────────────
import DesignerSelectDialog from '@/components/task/DesignerSelectDialog.vue'
import RetouchRequirementsBlock from '@/components/task-detail/RetouchRequirementsBlock.vue'
import ReassignDesignerDialog from '@/components/task/ReassignDesignerDialog.vue'
import EventLogDrawer from '@/components/logs/EventLogDrawer.vue'
import CancelReasonModal from '@/components/task-detail/CancelReasonModal.vue'
import TaskInfoEditModal from '@/components/task-detail/TaskInfoEditModal.vue'
import SkuItemsTable from '@/components/task-detail/SkuItemsTable.vue'
import SkuItemEditModal from '@/components/task-detail/SkuItemEditModal.vue'
import DesignAssetBlock from '@/components/task-detail/DesignAssetBlock.vue'
import AssetThumbStrip, { type AssetThumbItem } from '@/components/task-detail/AssetThumbStrip.vue'
import AssetPreviewMedia from '@/components/media/AssetPreviewMedia.vue'
import ImagePreviewLightbox from '@/components/media/ImagePreviewLightbox.vue'
import {
  IMAGE_PREVIEW_LIGHTBOX_KEY,
  type ImagePreviewLightboxItem,
  type OpenImagePreviewLightboxOptions,
} from '@/components/media/imagePreviewLightbox'
import { useDesignerOptions } from '@/composables/useDesignerOptions'
import { warehouseBlockingReasonLine } from '@/utils/warehouse-blocking'
import type { TaskAiSummaryResponse } from '@/services/api/tasksApi'
// v0.5 对齐：FRONTEND_ALIGNMENT_v0.5.md 第 D 节，任务详情内指派弹窗使用 GET /v1/users/designers

/** 与 /me `frontend_access.actions` 对齐：审核岗可能仅有细粒度 key（如 task.audit.review），未带历史 PermissionEnum `task:audit`。 */
const AUDIT_PRIMARY_TOOLBAR_PERMISSION_KEYS = [
  'task.audit',
  'task.audit.review',
  'task.audit.claim',
  'task.audit.approve',
  'task.audit.reject',
] as const

/** 仓库岗同上：后端可能下发粗粒度 warehouse:receive 或 task.warehouse / task.warehouse.receive 等 */
const WAREHOUSE_RECEIVE_PERMISSION_KEYS = [
  'warehouse.receive',
  'task.warehouse',
  'task.warehouse.receive',
] as const
const WAREHOUSE_RETURN_PERMISSION_KEYS = [
  'warehouse.return',
  'task.warehouse.reject',
  'task.warehouse.return',
] as const
const WAREHOUSE_FLOW_COMPLETE_PERMISSION_KEYS = [
  'task.close',
  'task.warehouse.complete',
  'warehouse.complete',
] as const
const COST_OVERRIDE_TIMELINE_ROLES = ['Ops', 'Warehouse', 'Admin', 'SuperAdmin'] as const

const route = useRoute()
const router = useRouter()
const tasksStore = useTasksStore()
const syncStore = useSyncStore()
const permissionsStore = usePermissionsStore()
const { can, currentUser, canAccessPage, canAccessAction, canAccessTask, canOperateTask } =
  usePermission()
const { isDeptAdminPlus } = useAuth()
const { cancel, needForceConfirm } = useTaskCancel()

const taskId = computed(() => route.params.id as string)
const isTempId = computed(() => taskId.value?.startsWith('t-'))
const task = computed(() => tasksStore.getById(taskId.value) ?? null)
const taskPredictionSuggestions = ref<PredictionSuggestion[]>([])
const taskPredictionLoading = ref(false)
const basicInfoModuleSummary = computed(() =>
  task.value?.moduleSummaries?.find((module) => module.module_key === 'basic_info'),
)
const designModuleSummary = computed(() =>
  task.value?.moduleSummaries?.find((module) => module.module_key === 'design'),
)
const retouchModuleSummary = computed(() =>
  task.value?.moduleSummaries?.find((module) => module.module_key === 'retouch'),
)
const auditModuleSummary = computed(() =>
  task.value?.moduleSummaries?.find((module) => module.module_key === 'audit'),
)
const warehouseModuleSummary = computed(() =>
  task.value?.moduleSummaries?.find((module) => module.module_key === 'warehouse'),
)
const customizationModuleSummary = computed(() =>
  task.value?.moduleSummaries?.find((module) => module.module_key === 'customization'),
)
const isBatchTask = computed(() => task.value?.isBatchTask === true)
const batchSkuItems = computed(() => task.value?.skuItems ?? [])
const productManagementRecords = ref<ProductManagementRecord[]>([])
const productManagementLoading = ref(false)
const productManagementError = ref('')
const productManagementPreviewURLs = ref<Record<number, string>>({})
const productManagementUploadInput = ref<HTMLInputElement | null>(null)
const productManagementUploadTarget = ref<ProductManagementRecord | null>(null)
const productManagementUploadingID = ref<number | null>(null)
const productManagementPreviewRecords = computed(() =>
  productManagementRecords.value.slice(0, isBatchTask.value ? 6 : 1),
)
const showProductManagementPanel = computed(
  () =>
    canAccessPage('product_management') &&
    (productManagementLoading.value ||
      productManagementError.value ||
      productManagementRecords.value.length > 0),
)

const isPurchaseTask = computed(
  () =>
    !!task.value &&
    (task.value.businessType === 'PURCHASE_TASK' || task.value.taskType === 'PURCHASE_TASK'),
)
const isRetouchTask = computed(
  () =>
    !!task.value &&
    (task.value.businessType === 'RETOUCH_TASK' || task.value.taskType === 'RETOUCH_TASK'),
)
const isCustomizationTask = computed(
  () =>
    !!task.value && (
      isCustomizationTaskByDomain(task.value) ||
      Boolean(task.value.customizationSourceType)
    ),
)
/** 成本同步已经覆盖原品、新品、采购；P 图等任务有成本数据时也展示。 */
const showCostInDetail = computed(
  () => {
    const t = task.value
    if (!t) return false
    if (
      t.businessType === 'ORIGINAL_PRODUCT_DEV' ||
      t.taskType === 'ORIGINAL_PRODUCT_DEV' ||
      t.businessType === 'NEW_PRODUCT_DEV' ||
      t.taskType === 'NEW_PRODUCT_DEV' ||
      t.businessType === 'PURCHASE_TASK' ||
      t.taskType === 'PURCHASE_TASK'
    ) {
      return true
    }
    return Boolean(t.costPrice || t.costOverrideSummary || t.governanceAuditSummary || t.procurementSummary)
  },
)
const canReadCostOverrideTimeline = computed(() =>
  permissionsStore.hasAnyRole(COST_OVERRIDE_TIMELINE_ROLES),
)

const TASK_TYPE_LABELS: Record<string, string> = {
  ORIGINAL_PRODUCT_DEV: '原品开发',
  NEW_PRODUCT_DEV: '新品开发',
  PURCHASE_TASK: '采购任务',
  RETOUCH_TASK: 'P 图任务',
  CUSTOMER_CUSTOMIZATION: '客户定制',
  REGULAR_CUSTOMIZATION: '常规定制',
}

const designModuleTitle = computed(() => {
  if (isCustomizationTask.value) return '美工提交设计稿'
  if (isRetouchTask.value) return '精修模块'
  return '设计模块'
})

const designModuleEyebrow = computed(() => {
  if (isCustomizationTask.value) return '定制美工侧'
  if (isRetouchTask.value) return '精修侧'
  return '设计侧'
})

const designOwnerLabel = computed(() => {
  if (isCustomizationTask.value) return '美工处理人'
  if (isRetouchTask.value) return '精修负责人'
  return '设计负责人'
})
const designAssetPanelName = computed(() => isCustomizationTask.value ? '定制稿与资产' : '设计与资产')
const uploadDesignCardTitle = computed(() => isCustomizationTask.value ? '上传定制设计稿' : '上传设计稿')
const submitAuditCardTitle = computed(() => isCustomizationTask.value ? '提交定制审核' : '提交审核')
const submitAuditActionText = computed(() => isCustomizationTask.value ? '提交定制审核' : '提交审核')
const submitAuditCardHint = computed(() =>
  isCustomizationTask.value
    ? '提交动作统一由定制稿与资产面板处理。'
    : '提交动作统一由设计与资产面板处理。',
)
const designAssetVersionLabel = computed(() => isCustomizationTask.value ? '定制稿版本' : '设计资产版本')
const designAssetVersionHint = computed(() =>
  isRetouchTask.value
    ? '切换上方时间线可查看各版本精修稿件'
    : isCustomizationTask.value
      ? '切换上方时间线可查看各版本定制稿件'
      : '切换上方时间线可查看各版本设计稿件',
)
const designAssetPreviewAreaLabel = computed(() =>
  isRetouchTask.value ? '精修稿件' : isCustomizationTask.value ? '定制稿件' : '设计稿件',
)
const assignDesignerLabel = computed(() => isCustomizationTask.value ? '指派美工' : '指派设计师')
const reassignDesignerLabel = computed(() => isCustomizationTask.value ? '重新指派美工' : '重新指派设计师')
const assignDialogTitle = computed(() => assignDesignerLabel.value)
const assignDialogDescription = computed(() =>
  isCustomizationTask.value
    ? '请选择本次任务的负责美工，后续定制审核与交班将以此为基础。'
    : '请选择本次任务的负责设计师，后续审核与交班将以此为基础。',
)
const assignDialogLoadingLabel = computed(() =>
  isCustomizationTask.value ? '加载美工列表...' : '加载设计师列表...',
)
const assignDialogEmptyHint = computed(() =>
  isCustomizationTask.value
    ? '暂无可指派的美工，请先在用户管理中配置定制美工角色'
    : '暂无可指派的设计师，请先在用户管理中配置设计师角色',
)
const assignDialogConfirmLabel = computed(() =>
  isCustomizationTask.value ? '确认指派美工' : '确认指派',
)
const assignDialogAssigneeRoleLabel = computed(() =>
  isCustomizationTask.value ? '美工' : '设计师',
)
const assignDesignerTitle = computed(() =>
  isCustomizationTask.value
    ? '任务尚无美工处理人时，在待处理阶段指定美工'
    : '任务尚无负责人时，在待指派阶段指定设计师（首次指派）',
)
const reassignDesignerTitle = computed(() =>
  isCustomizationTask.value
    ? '定制任务调度：在进入定制审核前更换美工处理人'
    : '设计阶段任务调度：在进入审核责任链前更换设计负责人',
)
const skuUploadDesignLabel = computed(() => isCustomizationTask.value ? '上传定制设计稿' : '上传设计稿')
const skuUploadDisabledTitle = computed(() =>
  isCustomizationTask.value ? '当前状态不可上传定制设计稿' : '当前状态不可上传设计稿',
)
const auditModuleEyebrow = computed(() => isCustomizationTask.value ? '定制审核侧' : '审核侧')
const auditModuleTitle = computed(() => isCustomizationTask.value ? '定制审核' : '审核模块')
const auditModulePillText = computed(() =>
  isCustomizationTask.value ? '定制审核 / 打回美工处理' : '通过 / 打回 / 审核参考',
)
const auditPendingAssetTitle = computed(() => isCustomizationTask.value ? '待审核定制稿' : '待审核稿件')
const auditActionCardTitle = computed(() => isCustomizationTask.value ? '定制审核动作' : '审核动作')
const auditActionDescription = computed(() =>
  isCustomizationTask.value
    ? '通过后进入仓库接收；打回后回到美工处理。'
    : '通过后进入仓库；打回后回到设计模块。',
)
const approveButtonLabel = computed(() =>
  isCustomizationTask.value ? '审核通过' : '通过',
)
const rejectButtonLabel = computed(() =>
  isCustomizationTask.value ? '打回美工处理' : '打回',
)

/** 顶栏左列副标题：类型 · 主状态 · 团队（与右侧徽标呼应，避免一行堆满徽标） */
const headerSubtitle = computed(() => {
  const t = task.value
  if (!t) return ''
  const bt = String(t.businessType ?? t.taskType ?? '')
  const typeLabel = TASK_TYPE_LABELS[bt] ?? bt
  const statusPart = t.mainStatus ? getMainTaskStatusLabel(t.mainStatus) : ''
  const team = t.groupName?.trim() || ''
  return [typeLabel, statusPart, team].filter(Boolean).join(' · ')
})
const headerStatusLabel = computed(() =>
  task.value?.mainStatus ? getMainTaskStatusLabel(task.value.mainStatus) : dash(task.value?.status),
)

function dash(value: unknown) {
  const text = String(value ?? '').trim()
  return text || '-'
}

const detailTypeLabel = computed(() => {
  const t = task.value
  if (!t) return '-'
  const key = String(t.businessType ?? t.taskType ?? '')
  return TASK_TYPE_LABELS[key] ?? dash(key)
})

const detailPriorityLabel = computed(() => {
  const priority = task.value?.priority
  const labels: Record<string, string> = {
    low: '低',
    medium: '中',
    normal: '普通',
    high: '高',
    urgent: '加急',
    critical: '加急',
  }
  return labels[String(priority ?? '')] ?? dash(priority)
})

const detailPriorityTone = computed(() => {
  const p = task.value?.priority
  return p === 'critical' || p === 'high' ? 'danger' : 'normal'
})

const detailCreatorLabel = computed(() => (task.value ? taskCreatorDisplayName(task.value) : '-'))
const detailDesignerLabel = computed(() => (task.value ? taskDesignerDisplayName(task.value) : '-'))
const detailOwnerLabel = computed(() => {
  const t = task.value
  if (!t) return '-'
  return dash(t.ownerOrgTeam || t.ownerDepartment || t.groupName)
})
const detailDueLabel = computed(() => {
  const due = task.value?.dueAt
  return due ? formatTaskDueAtDisplay(due) : '无截止时间'
})
const activeSkuItem = computed(() => task.value?.skuItems?.[detailProductIndex.value] ?? null)
const detailSkuLabel = computed(() => dash(activeSkuItem.value?.skuCode ?? task.value?.sku))
const detailProductNameLabel = computed(() =>
  dash(task.value?.productName ?? task.value?.productNameSnapshot ?? activeSkuItem.value?.productNameSnapshot),
)
const detailRetouchDownloadTitle = computed(() => {
  const product = detailProductNameLabel.value
  if (product && product !== '-') return product
  return dash(task.value?.taskNo)
})

/** V4 标题副线：优先产品名，否则任务类型 */
const detailHeadlineSuffix = computed(() => {
  const p = detailProductNameLabel.value
  if (p && p !== '-') return p
  return detailTypeLabel.value
})

/** 标题已含产品名时，类型单独用圆角徽标展示，避免与 h1 重复 */
const detailShowTypeBadge = computed(() => {
  const p = detailProductNameLabel.value
  return !!(p && p !== '-')
})

function formatCategoryNameWithCode(name?: string | null, code?: string | null): string {
  const n = String(name ?? '').trim()
  const c = String(code ?? '').trim()
  if (n && c && n !== c) return `${n}（${c}）`
  return n || c
}

const detailCategoryLabel = computed(() => {
  const t = task.value
  const code = activeSkuItem.value?.categoryCode ?? t?.newProductCategoryCode ?? t?.erpCategoryCode ?? t?.erpIId
  const name = t?.categoryName ?? t?.category ?? t?.erpCategoryName
  return dash(formatCategoryNameWithCode(name, code))
})
const showRetouchRequirementsBlock = computed(
  () =>
    isRetouchTask.value &&
    Array.isArray(task.value?.retouchRequirements) &&
    task.value.retouchRequirements.length > 0,
)

const detailRequirementKicker = computed(() =>
  showRetouchRequirementsBlock.value ? '任务总述' : '需求说明',
)

/** 设计/修改需求与文案，不含运营 note（见 detailNoteLabel） */
const detailRequirementLabel = computed(() => {
  // 非批量任务优先使用任务级 designRequirement（编辑弹窗写入目标），
  // 避免 SKU 子项的旧值覆盖已编辑的任务级值。
  if (!isBatchTask.value) {
    return dash(task.value?.designRequirement ?? task.value?.copyContent)
  }
  return dash(activeSkuItem.value?.designRequirement ?? task.value?.designRequirement ?? task.value?.copyContent)
})
/** 运营创建侧备注：与「需求说明」分开展示，便于下一环节阅读 */
const detailNoteLabel = computed(() => dash(task.value?.note))
const detailSpecLabel = computed(() => {
  const t = task.value
  if (!t) return '-'
  const parts = [t.specText, t.sizeText].map((x) => String(x ?? '').trim()).filter(Boolean)
  return parts.length > 0 ? parts.join('；') : '-'
})
const detailCostLabel = computed(() => {
  const t = task.value
  const amount = t?.costPrice?.amount ?? t?.newProductCostUnitPrice
  if (amount == null) return '-'
  return `${Number(amount).toFixed(3)} ${t?.costPrice?.currency ?? 'CNY'}`
})
const detailCostModeLabel = computed(() => {
  const mode = String(task.value?.costPriceMode ?? '').trim().toLowerCase()
  if (!mode) {
    const active = task.value?.costOverrideSummary?.current_override_active ??
      task.value?.costOverrideSummary?.currentOverrideActive
    if (active === true) return '手动录入'
    if (task.value?.costPrice) return '按系统规则计算'
    return '-'
  }
  if (mode === 'manual') return '手动录入'
  if (mode === 'template') return '按模板/系统计算'
  return dash(task.value?.costPriceMode)
})
const detailQuantityLabel = computed(() =>
  dash(activeSkuItem.value?.quantity ?? task.value?.newProductQuantity),
)
function recordField(record: Record<string, unknown> | undefined, ...keys: string[]): unknown {
  if (!record) return undefined
  for (const key of keys) {
    const value = record[key]
    if (value != null && String(value).trim() !== '') return value
  }
  return undefined
}
const detailCostStatusLabel = computed(() => {
  const t = task.value
  const summary = t?.costOverrideSummary
  const active = recordField(summary, 'current_override_active', 'currentOverrideActive')
  const finance = recordField(t?.costOverrideBoundary, 'finance_status', 'financeStatus')
  const review = recordField(t?.costOverrideBoundary, 'review_status', 'reviewStatus')
  const bits: string[] = []
  if (active === true) bits.push('人工覆盖生效')
  else if (active === false) bits.push('系统规则成本')
  if (review) bits.push(`审核：${review}`)
  if (finance) bits.push(`财务：${finance}`)
  return bits.length > 0 ? bits.join('；') : '-'
})
const detailCostOverrideReasonLabel = computed(() =>
  dash(recordField(task.value?.costOverrideSummary, 'current_override_reason', 'currentOverrideReason')),
)
const detailCostLatestActionLabel = computed(() => {
  const summary = task.value?.costOverrideSummary
  const latest = recordField(summary, 'latest_audit_event', 'latestAuditEvent')
  if (!latest || typeof latest !== 'object') return '-'
  const row = latest as Record<string, unknown>
  const eventType = String(row.event_type ?? row.eventType ?? '').trim()
  const actor = String(row.actor ?? row.override_actor ?? row.overrideActor ?? '').trim()
  const at = String(row.occurred_at ?? row.occurredAt ?? row.override_at ?? row.overrideAt ?? '').trim()
  const typeLabel =
    eventType === 'override_applied'
      ? '人工覆盖'
      : eventType === 'override_updated'
        ? '覆盖更新'
        : eventType === 'override_released'
          ? '解除覆盖'
          : eventType || '成本操作'
  const timeLabel = at ? formatMonthDayTimeBeijingOffsetAware(at) : ''
  return [typeLabel, actor, timeLabel].filter(Boolean).join(' · ') || '-'
})
function mergeTaskAndSkuReferenceRefs(detailTask: {
  referenceFileRefs?: ReferenceFileRef[]
  skuItems?: Array<{ referenceFileRefs?: ReferenceFileRef[] }>
}): ReferenceFileRef[] {
  const rootRefs = detailTask.referenceFileRefs ?? []
  const skuRefs = detailTask.skuItems?.flatMap((item) => item.referenceFileRefs ?? []) ?? []
  return [...rootRefs, ...skuRefs]
}

/** 基础信息区「参考图/母任务汇总」展示用：批量仅 task union；单品 task+sku 去重。 */
function motherTaskReferenceRefsForOps(detailTask: {
  referenceFileRefs?: ReferenceFileRef[]
  skuItems?: Array<{ referenceFileRefs?: ReferenceFileRef[] }>
} | null | undefined, batch: boolean): ReferenceFileRef[] {
  if (!detailTask) return []
  if (batch) {
    return dedupeReferenceFileRefs(detailTask.referenceFileRefs ?? [])
  }
  return dedupeReferenceFileRefs(mergeTaskAndSkuReferenceRefs(detailTask))
}

function referenceRefsToThumbItems(
  refs: ReferenceFileRef[],
  keyPrefix: string,
  labelFallback: string,
): AssetThumbItem[] {
  return refs
    .map((ref, index) => {
      const src = String(ref?.download_url ?? '').trim()
      const previewAssetId = referenceRefPreviewAssetId(ref)
      if (!src && !previewAssetId) return null
      const filename = String(ref?.filename ?? '').trim()
      return {
        key: `${keyPrefix}-${index}-${src || previewAssetId}`,
        src,
        previewAssetId,
        downloadUrl: src,
        alt: filename || `${labelFallback} ${index + 1}`,
        label: filename || `${labelFallback} ${index + 1}`,
      }
    })
    .filter((row) => row != null) as AssetThumbItem[]
}

function referenceRefPreviewAssetId(ref: ReferenceFileRef): string | undefined {
  const id = String(ref.asset_id ?? ref.ref_id ?? '').trim()
  return id || undefined
}

/** 任务详情 GET /v1/tasks/{id}/assets 中的 reference，用于合并运营侧顶部参考图展示 */
const opsReferenceBackendAssets = ref<BackendAsset[]>([])

function backendAssetID(asset: BackendAsset | undefined): string | undefined {
  if (!asset) return undefined
  const rec = asset as Record<string, unknown>
  const raw = rec.asset_id ?? rec.assetId ?? asset.id
  const id = String(raw ?? '').trim()
  return id || undefined
}

function unwrapOpsReferenceBackendAssetList(data: unknown): BackendAsset[] {
  if (Array.isArray(data)) return data as BackendAsset[]
  if (data && typeof data === 'object') {
    const root = data as Record<string, unknown>
    const inner = root.data ?? root.items
    if (Array.isArray(inner)) return inner as BackendAsset[]
    if (inner && typeof inner === 'object') {
      const mid = inner as Record<string, unknown>
      if (Array.isArray(mid.items)) return mid.items as BackendAsset[]
      if (Array.isArray(mid.data)) return mid.data as BackendAsset[]
    }
  }
  return []
}

async function loadOpsReferenceBackendAssets(): Promise<void> {
  const id = taskId.value
  if (!id || isTempId.value) {
    opsReferenceBackendAssets.value = []
    return
  }
  try {
    const res = await assetsApi.list(id)
    opsReferenceBackendAssets.value = unwrapOpsReferenceBackendAssetList(res?.data)
  } catch {
    opsReferenceBackendAssets.value = []
  }
}

const motherTaskOpsReferenceRefs = computed((): ReferenceFileRef[] => {
  const detailTask = task.value
  if (!detailTask) return []
  const legacyRefs = motherTaskReferenceRefsForOps(detailTask, isBatchTask.value)
  const assetRefs = referenceFileRefsFromBackendReferenceAssets(opsReferenceBackendAssets.value)
  return mergeReferenceFileRefsPreferBackend(legacyRefs, assetRefs)
})

const taskLevelReferenceBackendAssets = computed(() =>
  opsReferenceBackendAssets.value.filter(isTaskLevelBackendReferenceAsset),
)

const replaceableOpsReferenceAssetID = computed((): string | undefined => {
  const assets = taskLevelReferenceBackendAssets.value
  if (!assets.length) return undefined
  if (isBatchTask.value && assets.length !== 1) return undefined
  return backendAssetID(assets[0])
})

const detailReferenceLabel = computed(() => {
  const total = motherTaskOpsReferenceRefs.value.length
  return total > 0 ? `${total} 张图片 · 单文件 <= 300MB` : '暂无参考附件'
})
const totalReferenceCount = computed(() => motherTaskOpsReferenceRefs.value.length)
const RETOUCH_MODULE_STATE_LABELS: Record<string, string> = {
  pending_claim: '待领取',
  in_progress: '精修中',
  submitted: '已提交',
  closed: '已完成',
  completed: '已完成',
}
const effectiveRetouchLabel = computed(() => {
  const modState = retouchModuleState.value
  if (
    modState === 'in_progress' ||
    modState === 'submitted' ||
    modState === 'closed' ||
    modState === 'completed'
  ) {
    return RETOUCH_MODULE_STATE_LABELS[modState]
  }
  const ts = task.value?.status
  if (ts === 'PendingAuditA' || ts === 'PendingAuditB' || ts === 'Completed' || ts === 'Archived') {
    return '已提交'
  }
  if (ts === 'InProgress' && task.value?.designerId) return '精修中'
  return RETOUCH_MODULE_STATE_LABELS[modState] || '精修待处理'
})
const designStatusText = computed(() => {
  if (isRetouchTask.value) return effectiveRetouchLabel.value
  const customizationLabel = task.value
    ? getCustomizationDetailStatusLabel(task.value)
    : null
  if (customizationLabel) return customizationLabel
  const status = task.value?.designSubStatus
  if (status) return getDesignSubStatusLabel(status)
  if (isCustomizationTask.value) return '定制待处理'
  return '等待设计处理'
})
const designVersionSummary = computed(() => {
  const versions = task.value?.assetVersions ?? []
  if (!versions.length) return '暂无设计稿版本'
  return versions
    .slice(-2)
    .map((v) => `v${v.rootVersionNo ?? 1} ${v.assetNo ?? v.note ?? '设计稿'}`)
    .join('；')
})

function isTimelineEligibleAssetKind(kind: string | undefined): boolean {
  const k = (kind ?? '').trim().toLowerCase()
  return k === 'delivery' || k === 'source'
}

const detailScopedAssetVersionCount = computed(() => {
  const t = task.value
  if (!t) return 0
  const all = (t.assetVersions ?? []).filter((v) => isTimelineEligibleAssetKind(v.assetKind))
  if (!taskHasSkuItemsForBatchUi(t) || isPurchaseTask.value) return all.length
  const sel = selectionFromProductIndex(t, detailProductIndex.value)
  return all.filter((v) => assetVersionMatchesActiveSku(v, sel, t)).length
})

const isDesignModuleResultState = computed(() => {
  if (!task.value || isPurchaseTask.value || isRetouchTask.value) return false
  if (detailScopedAssetVersionCount.value === 0) return false
  if (canUploadDesignDelivery(task.value)) return false
  if (canSubmitAudit(task.value) && !isCustomizationTask.value) return false
  return true
})

const isRetouchModuleResultState = computed(() => {
  if (!task.value || !isRetouchTask.value) return false
  if (detailScopedAssetVersionCount.value === 0) return false
  const state = retouchModuleState.value
  if (state === 'submitted' || state === 'closed' || state === 'completed') return true
  const ts = task.value.status
  if (ts === 'PendingAuditA' || ts === 'PendingAuditB' || ts === 'Completed' || ts === 'Archived') {
    return true
  }
  return false
})

const isDesignOrRetouchModuleResultState = computed(
  () => isDesignModuleResultState.value || isRetouchModuleResultState.value,
)

const retouchVersionSummary = computed(() => {
  if (!isRetouchTask.value) return designVersionSummary.value
  const versions = task.value?.assetVersions ?? []
  if (!versions.length) return '暂无精修稿版本'
  return versions
    .slice(-2)
    .map((v) => `v${v.rootVersionNo ?? 1} ${v.assetNo ?? v.note ?? '精修稿'}`)
    .join('；')
})
/** `warehouse_receive_status` 过渡期读模型枚举，仅 UI 映射 */
const LEGACY_WAREHOUSE_RECEIVE_CN = {
  pending: '待接收',
  received: '已接收',
  returned: '已退回',
  archived: '已归档',
} as const

const warehouseStatusText = computed(() => {
  const t = task.value
  if (!t) return '-'
  if (t.warehouseSubStatus) return dash(getWarehouseSubStatusLabel(t.warehouseSubStatus))
  const recv = t.warehouseReceiveStatus
  if (!recv) return '-'
  const mapped = LEGACY_WAREHOUSE_RECEIVE_CN[recv]
  return dash(mapped ?? recv)
})

const auditPendingThumbItems = computed((): AssetThumbItem[] => {
  const currentTask = task.value
  if (!currentTask) return []
  const latestDeliveryVersions = latestDeliveryBatchVersionsForSelection(
    currentTask,
    selectionFromProductIndex(currentTask, detailProductIndex.value),
  )
  if (!latestDeliveryVersions.length) return []

  const items: AssetThumbItem[] = []
  for (const version of latestDeliveryVersions) {
    const versionPrefix = version.assetNo?.trim() || `版本 ${version.id}`
    const previewItems = (version.fileRefs ?? [])
      .filter((src) => String(src ?? '').trim().length > 0)
      .map((src, index) => ({
        key: `${version.id}-audit-${index}`,
        src,
        downloadUrl: src,
        alt: `${versionPrefix} 稿件 ${index + 1}`,
        label: `${versionPrefix} 图 ${index + 1}`,
      }))
    const sourceOffset = previewItems.length
    const nonPreviewItems = (version.nonPreviewFiles ?? []).map((item, index) => ({
      key: `${version.id}-audit-file-${index}`,
      alt: item.label || `${versionPrefix} 文件 ${sourceOffset + index + 1}`,
      label: item.label || `${versionPrefix} 文件 ${sourceOffset + index + 1}`,
      downloadUrl: item.url || '',
      unavailable: true,
    }))
    items.push(...previewItems, ...nonPreviewItems)
  }
  return items
})

const warehouseProofThumbItems = computed((): AssetThumbItem[] => {
  // 后端尚未提供入库凭证附件字段，先保留统一缩略图容器占位，后续仅替换数据源即可。
  return []
})
const opsReferenceThumbItems = computed((): AssetThumbItem[] =>
  referenceRefsToThumbItems(motherTaskOpsReferenceRefs.value, 'ops-ref', '参考图').slice(0, 6),
)
const opsReferenceUploadInputRef = ref<HTMLInputElement | null>(null)
const opsReferenceUploadError = ref('')
const opsReferenceUploadStatus = ref('')
const referenceBatchDownloading = ref(false)
const referenceBatchDownloadStatus = ref('')
const referenceBatchDownloadError = ref('')
const auditSourceUploadInputRef = ref<HTMLInputElement | null>(null)
const auditDeliveryUploadInputRef = ref<HTMLInputElement | null>(null)
const auditAssetUploadError = ref('')
const auditAssetUploadStatus = ref('')
type DetailUploadTarget = 'reference' | 'audit-source' | 'audit-delivery'
const activeDetailUploadTarget = ref<DetailUploadTarget | null>(null)

// provide：让所有子区块无需 props 直接注入 task
provide(TASK_DETAIL_KEY, task)
const lightboxOpen = ref(false)
const lightboxItems = ref<ImagePreviewLightboxItem[]>([])
const lightboxInitialIndex = ref(0)

function normalizeLightboxItems(src: string, options?: OpenImagePreviewLightboxOptions): ImagePreviewLightboxItem[] {
  const fallbackTitle = options?.title?.trim() || '预览大图'
  const normalized = (options?.items ?? [])
    .map((item) => ({
      ...item,
      src: String(item.src ?? '').trim(),
      previewAssetId: String(item.previewAssetId ?? '').trim(),
      fallbackAssetId: String(item.fallbackAssetId ?? '').trim(),
      fallbackSrc: String(item.fallbackSrc ?? '').trim(),
      resolvedPreviewUrl: String(item.resolvedPreviewUrl ?? '').trim(),
      title: String(item.title ?? '').trim(),
      alt: String(item.alt ?? '').trim(),
      downloadUrl: String(item.downloadUrl ?? '').trim(),
    }))
    .filter((item) =>
      Boolean(item.src || item.previewAssetId || item.fallbackAssetId || item.fallbackSrc || item.resolvedPreviewUrl || item.downloadUrl),
    )
  if (normalized.length > 0) return normalized
  return src ? [{ src, title: fallbackTitle, alt: fallbackTitle, downloadUrl: src }] : []
}

function openLightbox(src: string, options?: OpenImagePreviewLightboxOptions) {
  const url = String(src ?? '').trim()
  const items = normalizeLightboxItems(url, options)
  if (!url && items.length === 0) return
  const requestedIndex = typeof options?.index === 'number' ? options.index : items.findIndex((item) => item.src === url)
  lightboxItems.value = items
  lightboxInitialIndex.value = Math.max(0, requestedIndex >= 0 ? requestedIndex : 0)
  lightboxOpen.value = true
}
provide(IMAGE_PREVIEW_LIGHTBOX_KEY, openLightbox)

const detailProductIndex = ref(0)
const productIndexContext: TaskDetailProductIndexContext = {
  productIndex: detailProductIndex,
  setProductIndex(i: number) {
    detailProductIndex.value = Math.max(0, i)
  },
}
provide(TASK_DETAIL_PRODUCT_INDEX_KEY, productIndexContext)

watch(
  () => task.value?.id,
  () => {
    detailProductIndex.value = 0
  },
)

/** URL 刷新：覆盖任务级与各 SKU 参考图（去重），不影响母任务区展示计数。 */
function collectTaskReferenceRefs(): ReferenceFileRef[] {
  const detailTask = task.value
  if (!detailTask) return []
  return dedupeReferenceFileRefs(mergeTaskAndSkuReferenceRefs(detailTask))
}

watchEffect(() => {
  const refs = collectTaskReferenceRefs()
  if (refs.some(isReferenceUrlExpiringSoon)) {
    tasksStore.refreshReferenceUrls(taskId.value)
  }
})

function onVisibilityChangeForRefs() {
  if (document.visibilityState !== 'visible') return
  const refs = collectTaskReferenceRefs()
  if (refs.some(isReferenceUrlExpiringSoon)) {
    tasksStore.refreshReferenceUrls(taskId.value)
  }
}
onMounted(() => document.addEventListener('visibilitychange', onVisibilityChangeForRefs))
onBeforeUnmount(() => document.removeEventListener('visibilitychange', onVisibilityChangeForRefs))

const storeLoading = computed(() => tasksStore.loading)
const storeError = computed(() => tasksStore.loadError)

const detailLoading = ref(false)
const detailError = ref<string | null>(null)

const createSuccessBannerVisible = computed(() => route.query.fromCreate === '1')
const createPrefillSyncWarningVisible = computed(() => route.query.prefillSyncFailed === '1')
const createPrefillSyncWarningMessage =
  '任务已创建，但主档预填同步失败，请在「仓库与结单」中补录后重试建档。'
const createProcurementSyncWarningVisible = computed(() => route.query.procurementSyncFailed === '1')
const createProcurementSyncWarningMessage =
  '任务已创建，但采购记录同步失败，请先补齐采购价、数量和供应商并完成采购流程，再执行交仓。'
const createRetouchRequirementUploadWarningVisible = computed(
  () => route.query.retouchRequirementUploadFailed === '1',
)
const createRetouchRequirementUploadWarningMessage =
  '任务已创建，但部分 P 图需求附件上传失败，请检查需求明细并补传。'

const createSuccessMessage = computed(() => {
  const t = task.value
  if (!t) return ''
  if (t.isBatchTask) {
    const n = parallelProductTabCount(t)
    if (n > 1) return `任务创建成功，已生成 ${n} 个并列商品`
  }
  const isOriginal = t.businessType === 'ORIGINAL_PRODUCT_DEV' || t.taskType === 'ORIGINAL_PRODUCT_DEV'
  if (isOriginal) return '任务创建成功。审核通过后系统将自动更新 ERP。'
  const status = t.filing_status
  if (status === 'filed') return '已自动同步 ERP'
  if (status === 'pending_filing') return '资料未齐，补齐后系统将自动同步 ERP'
  if (status === 'filing') return '系统正在自动同步 ERP'
  if (status === 'filing_failed') return 'ERP 同步失败，可在详情中重试'
  return '任务创建成功'
})

const createSuccessBannerTone = computed(() => {
  const status = task.value?.filing_status
  if (status === 'filing_failed') return 'banner-error'
  if (status === 'pending_filing') return 'banner-warning'
  return 'banner-info'
})

const actionAvailability = computed(() =>
  task.value ? getTaskActionAvailability(task.value) : null,
)

const hasTaskScopeAccess = computed(() => {
  if (!task.value) return false
  return canAccessTask(task.value)
})

const isCurrentHandler = computed(() => {
  if (!task.value) return false
  const handlerId = String(task.value.currentHandlerId ?? '').trim()
  if (!handlerId) return true
  return String(currentUser.value?.id ?? '').trim() === handlerId
})

const canOperateTaskActions = computed(
  () => hasTaskScopeAccess.value && isCurrentHandler.value,
)

const designModuleAllowsAssign = computed(() =>
  hasModuleAction(designModuleSummary.value, ['assign', 'task.assign']),
)

const showAuditActionButtons = computed(
  () => {
    // 不因 hasTaskScopeAccess（owner_org_team / owner_department）拦截审核按钮：
    // 跨组发起的任务常被审核账号处理，但若审核员既非责任人又非同组，`canAccessTask` 为 false，
    // 会误藏「通过/打回」；门禁以 RBAC + 模块 allowed_actions / 任务状态兜底为准，与 showReassignDesignerButton 口径一致。
    if (!task.value || !can([...AUDIT_PRIMARY_TOOLBAR_PERMISSION_KEYS])) return false
    if (isCustomizationTask.value) return false
    if (hasModuleActionProjection(auditModuleSummary.value)) {
      return hasModuleAction(auditModuleSummary.value, ['approve', 'reject'])
    }
    return Boolean(actionAvailability.value?.canShowAuditActions)
  },
)
const showCustomizationReviewActionButtons = computed(() => {
  if (!task.value || !isCustomizationTask.value) return false
  if (task.value.status !== 'PendingCustomizationReview') return false
  return (
    can('task.customization.review') ||
    permissionsStore.hasAnyRole([
      'CustomizationReviewer',
      'customization_reviewer',
      'customizationreviewer',
    ])
  )
})
const showActiveAuditActionButtons = computed(
  () => showAuditActionButtons.value || showCustomizationReviewActionButtons.value,
)
const retouchModuleState = computed(() => retouchModuleSummary.value?.state ?? '')

const retouchTaskHasDesigner = computed(() => Boolean(task.value?.designerId))

const showRetouchClaimAction = computed(() => {
  if (!task.value || !isRetouchTask.value || !hasTaskScopeAccess.value) return false
  if (retouchTaskHasDesigner.value) return false
  if (retouchModuleState.value === 'pending_claim') return true
  if (hasModuleActionProjection(retouchModuleSummary.value)) {
    return hasModuleAction(retouchModuleSummary.value, ['claim'])
  }
  return false
})

const showRetouchSubmitAction = computed(() => {
  if (!task.value || !isRetouchTask.value || !hasTaskScopeAccess.value) return false
  if (hasModuleActionProjection(retouchModuleSummary.value)) {
    return hasModuleAction(retouchModuleSummary.value, ['submit'])
  }
  if (retouchModuleState.value === 'in_progress') return true
  if (retouchTaskHasDesigner.value) return true
  return false
})
const canUploadAuditAssets = computed(() => showAuditActionButtons.value)
const purchaseWorkflowCanClose = computed(() => {
  if (!isPurchaseTask.value || !task.value) return false
  return task.value.workflowCanClose === true
})
const purchaseWorkflowCanPrepareWarehouse = computed(() => {
  if (!isPurchaseTask.value || !task.value) return false
  if (task.value.workflowCanClose === true) return false
  const statusRaw = String(task.value.status ?? '').trim().toLowerCase()
  if (statusRaw === 'pendingclose') return false
  return task.value.canPrepareWarehouse === true
})
const canShowCloseTaskButton = computed(() => {
  if (!task.value) return false
  if (isPurchaseTask.value) return purchaseWorkflowCanClose.value
  return canCloseTask.value
})
const showWarehouseReceiveActionButtons = computed(
  () => {
    // 与审核条一致：仓管常跨组处理「待仓库接收」任务，不因 owner_department 拦截。
    if (!task.value || !can([...WAREHOUSE_RECEIVE_PERMISSION_KEYS])) return false
    if (shouldHideWarehouseReceiveActions(task.value)) return false
    if (isPurchaseTask.value) return purchaseWorkflowCanPrepareWarehouse.value
    if (hasModuleActionProjection(warehouseModuleSummary.value)) {
      return hasModuleAction(warehouseModuleSummary.value, ['receive', 'submit'])
    }
    return Boolean(actionAvailability.value?.canShowWarehouseReceiveActions)
  },
)
const showWarehouseReturnActionButton = computed(
  () => {
    if (!task.value || !can([...WAREHOUSE_RETURN_PERMISSION_KEYS])) return false
    if (shouldHideWarehouseReceiveActions(task.value)) return false
    if (isPurchaseTask.value) return false
    if (hasModuleActionProjection(warehouseModuleSummary.value)) {
      return hasModuleAction(warehouseModuleSummary.value, ['reject', 'return'])
    }
    return Boolean(actionAvailability.value?.canShowWarehouseReceiveActions)
  },
)
const showWarehouseActionButtons = computed(
  () => showWarehouseReceiveActionButtons.value || showWarehouseReturnActionButton.value,
)
const showWarehouseCompleteActionButton = computed(
  () => {
    if (!task.value || !can([...WAREHOUSE_FLOW_COMPLETE_PERMISSION_KEYS])) return false
    if (shouldHideWarehouseCompleteAction(task.value)) return false
    // 与后端 maintenance scope（如 role_plus_maintenance_scope / task_out_of_department_scope）对齐
    if (!canOperateTask(task.value)) return false
    if (isPurchaseTask.value) return purchaseWorkflowCanClose.value
    if (hasModuleActionProjection(warehouseModuleSummary.value)) {
      return hasModuleAction(warehouseModuleSummary.value, ['close_task', 'complete'])
    }
    return Boolean(actionAvailability.value?.canShowWarehouseComplete)
  },
)

const showErpFilingRetryButton = computed(() => {
  if (!task.value || !hasTaskScopeAccess.value) return false
  if (!canUserRetryErpFiling((roles) => permissionsStore.hasAnyRole(roles))) return false
  return taskNeedsErpFilingRetry(task.value)
})

const erpFilingRetrying = ref(false)

const detailErpSyncStatusLabel = computed(() => {
  const t = task.value
  if (!t?.filing_status && !t?.erp_sync_required && !t?.filing_error_message) return ''
  return getTaskFilingStatusLabel(t.filing_status, t.businessType ?? t.taskType)
})

const detailErpSyncStatusToneClass = computed(() => {
  const tone = getTaskFilingStatusTone(
    task.value?.filing_status,
    task.value?.businessType ?? task.value?.taskType,
  )
  if (tone === 'error') return 'detail-erp-sync-status--error'
  if (tone === 'warning') return 'detail-erp-sync-status--warning'
  if (tone === 'success') return 'detail-erp-sync-status--success'
  return ''
})

const detailErpSyncFailureMessage = computed(() =>
  formatErpSyncFailureMessage(task.value?.filing_error_message ?? ''),
)

async function onErpFilingRetry() {
  const id = taskId.value
  if (!id || isTempId.value || erpFilingRetrying.value) return
  erpFilingRetrying.value = true
  actionError.value = ''
  try {
    await tasksApi.retryFiling(id)
    await tasksStore.loadTaskById(id)
    flashSuccess('已发起 ERP 同步重试')
  } catch (err) {
    actionError.value = resolveApiUserMessage(err, {
      fallback: 'ERP 同步重试失败，请稍后重试',
    })
  } finally {
    erpFilingRetrying.value = false
  }
}

const canEditBasicInfo = computed(
  () => {
    // 运营维护入口以 basic_info.allowed_actions 为准，不与通用 task.edit 绑定：
    // Ops 创建人常具备 update_basic_info 投影但无 task.edit，强绑会导致「编辑信息 / 重传参考图」误隐藏。
    if (!task.value || !hasTaskScopeAccess.value) return false
    if (
      canMaintainTaskProductInfoAtAnyStage(
        (roles) => permissionsStore.hasAnyRole(roles),
        hasTaskScopeAccess.value,
      )
    ) {
      return true
    }
    if (hasModuleActionProjection(basicInfoModuleSummary.value)) {
      return hasModuleAction(basicInfoModuleSummary.value, [
        'update_basic_info',
        'update_reference_files',
      ])
    }
    return isLegacyTaskStatusInDesignerEditablePhase(task.value)
  },
)
const canUploadReferenceFromOps = computed(() => {
  if (!task.value || !hasTaskScopeAccess.value) return false
  if (hasModuleActionProjection(basicInfoModuleSummary.value)) {
    return hasModuleAction(basicInfoModuleSummary.value, ['update_reference_files'])
  }
  return isLegacyTaskStatusInDesignerEditablePhase(task.value)
})
const canEditSkuItemInfo = computed(() => {
  if (!task.value || !hasTaskScopeAccess.value) return false
  return canMaintainTaskProductInfoAtAnyStage(
    (roles) => permissionsStore.hasAnyRole(roles),
    hasTaskScopeAccess.value,
  )
})
function detailUploadTargetEnabled(target: DetailUploadTarget | null): boolean {
  if (target === 'reference') return canUploadReferenceFromOps.value
  if (target === 'audit-source' || target === 'audit-delivery') {
    return canUploadAuditAssets.value && actionLoading.value !== 'audit-upload'
  }
  return false
}

const { activateFileReceiver: activateDetailGlobalFileReceiver } = useFileDropPasteReceiver({
  enabled: () => detailUploadTargetEnabled(activeDetailUploadTarget.value),
  onFiles: (files) => {
    const target = activeDetailUploadTarget.value
    if (target === 'reference') {
      void handleOpsReferenceFiles(files)
      return
    }
    if (target === 'audit-source') {
      void handleAuditAssetFiles(files, 'source')
      return
    }
    if (target === 'audit-delivery') {
      void handleAuditAssetFiles(files, 'delivery')
    }
  },
})

function activateDetailFileReceiver(target: DetailUploadTarget) {
  if (!detailUploadTargetEnabled(target)) return
  activeDetailUploadTarget.value = target
  activateDetailGlobalFileReceiver()
}

function isActiveDetailUploadTarget(target: DetailUploadTarget): boolean {
  return activeDetailUploadTarget.value === target && detailUploadTargetEnabled(target)
}

function onDetailUploadDragOver(target: DetailUploadTarget, event: DragEvent) {
  if (!detailUploadTargetEnabled(target) || !hasFileDataTransfer(event.dataTransfer)) return
  activateDetailFileReceiver(target)
  if (event.dataTransfer) event.dataTransfer.dropEffect = 'copy'
}

function onDetailUploadDrop(target: DetailUploadTarget, event: DragEvent) {
  if (!detailUploadTargetEnabled(target)) return
  const files = getFilesFromDataTransfer(event.dataTransfer)
  if (!files.length) return
  activateDetailFileReceiver(target)
  if (target === 'reference') {
    void handleOpsReferenceFiles(files)
  } else {
    void handleAuditAssetFiles(files, target === 'audit-delivery' ? 'delivery' : 'source')
  }
}

function onDetailUploadPaste(target: DetailUploadTarget, event: ClipboardEvent) {
  if (!detailUploadTargetEnabled(target)) return
  const files = getFilesFromClipboardEvent(event)
  if (!files.length) return
  event.preventDefault()
  activateDetailFileReceiver(target)
  if (target === 'reference') {
    void handleOpsReferenceFiles(files)
  } else {
    void handleAuditAssetFiles(files, target === 'audit-delivery' ? 'delivery' : 'source')
  }
}

const canDirectSkuDesignUpload = computed(() => {
  if (!task.value || isPurchaseTask.value || !hasTaskScopeAccess.value) return false
  if (!can('design.upload')) return false
  if (isRetouchTask.value) return showRetouchSubmitAction.value
  if (isCustomizationTask.value && !permissionsStore.isCustomizationOperator) return false
  return canUploadDesignDelivery(task.value)
})

const taskInfoEditOpen = ref(false)
const skuItemEditOpen = ref(false)
const editingSkuItem = ref<TaskSkuItem | null>(null)

function openBasicEdit() {
  if (!task.value) return
  taskInfoEditOpen.value = true
}

function openSkuItemEdit(payload: { item: TaskSkuItem; index: number }) {
  if (!canEditSkuItemInfo.value) {
    actionError.value = '当前账号不可维护子项商品资料'
    return
  }
  actionError.value = ''
  editingSkuItem.value = payload.item
  skuItemEditOpen.value = true
}

function openSkuDesignUpload(payload: { item: TaskSkuItem; index: number }) {
  if (!task.value) return
  if (!canDirectSkuDesignUpload.value) {
    actionError.value = isCustomizationTask.value ? '当前状态不可上传定制设计稿' : '当前状态不可上传设计稿'
    return
  }
  actionError.value = ''
  detailProductIndex.value = Math.max(0, payload.index)
  const skuCode = String(payload.item.skuCode ?? '').trim()
  const skuSuffix = skuCode ? `（${skuCode}）` : ''
  flashSuccess(`已切换到子项 ${payload.index + 1}${skuSuffix}，请在${designModuleTitle.value}上传${isCustomizationTask.value ? '定制设计稿' : '设计稿'}`)
  void nextTick(() => {
    focusReferenceSectionFromDetail()
  })
}

async function onTaskInfoEditSaved() {
  const id = taskId.value
  if (!id || isTempId.value) return
  await tasksStore.loadTaskById(id)
  flashSuccess('任务信息已更新')
}

async function onSkuItemEditSaved() {
  const id = taskId.value
  if (!id || isTempId.value) return
  await tasksStore.loadTaskById(id)
  flashSuccess('子项商品资料已更新')
}

function dismissCreateBanner() {
  router.replace({ path: route.path, query: {} })
}

/** 首次指派：待指派且尚无负责人（与「重新指派」互斥） */
const canAssignPermission = computed(() => {
  if (!task.value || !currentUser.value) return false
  return canUserScheduleDesignerAssignment(task.value, currentUser.value, {
    hasPermission: (p) => can(p),
    hasAction: (key) => canAccessAction(key),
    isGroupLeader: permissionsStore.isGroupLeader,
  })
})

const showAssignCustomizationArtButton = computed(() => {
  if (!task.value || !isCustomizationTask.value || isPurchaseTask.value) return false
  if (!canAssignPermission.value || !canAssignCustomizationArtOperator(task.value)) return false
  return Boolean(actionAvailability.value?.canShowAssign)
})

const showAssignDesignerButton = computed(() => {
  if (showAssignCustomizationArtButton.value) return true
  if (!task.value || isPurchaseTask.value || taskHasAssignee(task.value)) return false
  if (designModuleAllowsAssign.value) return true
  return (
    canAssignPermission.value &&
    Boolean(actionAvailability.value?.canShowAssign) &&
    canAssign(task.value)
  )
})

const canReassignPermission = computed(() => {
  if (!task.value || !currentUser.value) return false
  return canUserScheduleDesignerReassignment(task.value, currentUser.value, {
    hasPermission: (p) => can(p),
    hasAction: (key) => canAccessAction(key),
    isGroupLeader: permissionsStore.isGroupLeader,
  })
})

const designModuleAllowsReassign = computed(() =>
  hasModuleAction(designModuleSummary.value, [
    'reassign',
    'pool_reassign',
    'task.reassign',
    'task.reassign.team',
    'task.reassign.department',
  ]),
)
const retouchModuleAllowsReassign = computed(() =>
  hasModuleAction(retouchModuleSummary.value, [
    'reassign',
    'pool_reassign',
    'task.reassign',
    'task.reassign.team',
    'task.reassign.department',
  ]),
)
const customizationModuleAllowsReassign = computed(() =>
  hasModuleAction(customizationModuleSummary.value, [
    'reassign',
    'pool_reassign',
    'task.reassign',
    'task.reassign.team',
    'task.reassign.department',
  ]),
)

/** 换人：设计阶段可调度，但进入审核责任链及之后阶段不可重派（见 canReassignDesigner） */
const showReassignDesignerButton = computed(
  () => {
    // 重新指派是“调度动作”而非通用详情写操作：
    // 这里不再强依赖 hasTaskScopeAccess（owner_department 口径），
    // 改由 canReassignPermission + 状态门禁 + 后端 allowed_actions 共同控制。
    if (!task.value || isPurchaseTask.value) return false
    if (
      !taskHasAssignee(task.value) &&
      !isInCustomizationArtReassignmentPhase(task.value)
    ) {
      return false
    }
    // design/retouch 模块 reassign 投影：与历史逻辑一致，模块有动作即展示。
    if (designModuleAllowsReassign.value || retouchModuleAllowsReassign.value) return true
    // customization 模块仅加速展示；缺模块时走下方权限+状态兜底（历史任务 807 等）。
    if (
      customizationModuleAllowsReassign.value &&
      (isInCustomizationArtReassignmentPhase(task.value) ||
        isInDesignerReassignmentPhase(task.value))
    ) {
      return true
    }
    return (
      canReassignPermission.value &&
      Boolean(actionAvailability.value?.canShowReassign) &&
      canReassignDesigner(task.value)
    )
  },
)

const hasDesignOutputHint = computed(() => {
  if (!task.value) return false
  return taskHasRecordedDesignOutput(task.value)
})

const canCloseTask = computed(() =>
  !!task.value &&
  canOperateTaskActions.value &&
  !isTaskCloseFlowTerminal(task.value) &&
  canCloseTaskForArchive(task.value).allowed &&
  can('task.close'),
)

/** 普通终止：与 cancel `force:false` 一致——创建人，或具备创建类动作的运营入口（最终以接口为准）。 */
const isTaskCreator = computed(() => {
  if (!task.value || !currentUser.value) return false
  const cid = String(task.value.creatorId ?? '').trim()
  if (!cid) return false
  return cid === String(currentUser.value.id).trim()
})
const canInitiateNormalTaskCancel = computed(
  () => !!task.value && !!currentUser.value && (isTaskCreator.value || can('task.create')),
)
/** 强制终止：部门管理员 / 超管（见 useAuth isDeptAdminPlus）。 */
const canForceCancelTask = computed(() => isDeptAdminPlus.value)
/** 终止任务：不再使用 current_handler_id；与 POST /v1/tasks/{id}/cancel 授权对齐。 */
const canCancelTask = computed(() => {
  if (!task.value) return false
  if (['Archived', 'Completed', 'Cancelled'].includes(task.value.status)) return false
  return canInitiateNormalTaskCancel.value || canForceCancelTask.value
})

function openTaskAssetsPage() {
  if (!task.value?.id) {
    actionError.value = '任务 ID 缺失，无法打开任务资产页'
    return
  }
  void router.push({ name: 'TaskAssets', params: { id: task.value.id } })
}

function findDesignAssetSection(): HTMLElement | null {
  return document.querySelector('.detail-v3-info-card--wide') as HTMLElement | null
}

function focusReferenceSectionFromDetail(): void {
  const section = findDesignAssetSection()
  if (!section) {
    actionError.value = `${designAssetPanelName.value}区尚未渲染完成，请稍后重试`
    return
  }
  actionError.value = ''
  section.scrollIntoView({ behavior: 'smooth', block: 'center' })
}


function triggerReferenceUploadFromDetail(): void {
  if (!canUploadReferenceFromOps.value) {
    actionError.value = '当前状态不可上传参考图'
    return
  }
  actionError.value = ''
  opsReferenceUploadInputRef.value?.click()
}

async function handleOpsReferenceUpload(e: Event) {
  const input = e.target as HTMLInputElement
  const files = input.files
  input.value = ''
  await handleOpsReferenceFiles(files ?? [])
}

async function handleOpsReferenceFiles(files: FileList | File[]) {
  const currentTask = task.value
  if (!files.length || !currentTask?.id) return
  const picked = Array.from(files)
  opsReferenceUploadError.value = ''
  opsReferenceUploadStatus.value = ''

  const oversized = picked.filter((f) => f.size > REFERENCE_UPLOAD_MAX_FILE_SIZE_BYTES)
  const unsupported = picked.filter((f) => !isAllowedUploadFile(f.name))
  const validFiles = picked.filter(
    (f) =>
      isAllowedUploadFile(f.name) &&
      isAcceptableReferenceFile(f) &&
      f.size <= REFERENCE_UPLOAD_MAX_FILE_SIZE_BYTES,
  )
  const errors: string[] = []
  if (oversized.length > 0) {
    errors.push(
      oversized.length === 1
        ? referenceFileTooLargeMessage(oversized[0]?.name)
        : `有 ${oversized.length} 个文件超过 ${REFERENCE_UPLOAD_MAX_FILE_SIZE_MB}MB，已拒绝上传`,
    )
  }
  if (unsupported.length > 0) {
    errors.push(
      unsupported.length === 1
        ? `不支持的文件类型：${unsupported[0]?.name ?? ''}`
        : `有 ${unsupported.length} 个文件类型不受支持，已拒绝上传`,
    )
  }
  opsReferenceUploadError.value = errors.join('；')
  if (!validFiles.length) return

  opsReferenceUploadStatus.value = '上传中...'
  try {
    await loadOpsReferenceBackendAssets()
    const replaceAssetId = validFiles.length === 1 ? replaceableOpsReferenceAssetID.value : undefined
    for (const file of validFiles) {
      await uploadTaskReferenceFileViaAssetSession(currentTask.id, file, {
        assetId: replaceAssetId,
        ownerModuleKey: 'basic_info',
        uploadPolicy: replaceAssetId ? 'replace' : 'append_only',
      })
    }
    opsReferenceUploadStatus.value = replaceAssetId ? '参考图已替换' : '参考图已上传'
    await tasksStore.loadTaskById(currentTask.id)
    await loadOpsReferenceBackendAssets()
  } catch (err) {
    opsReferenceUploadError.value = formatUploadFailureMessage('reference_upload', err)
    opsReferenceUploadStatus.value = ''
  }
}

function formatTaskReferenceBatchFailure(item: {
  reason?: string
  key?: string
  source_kind?: string
  filename?: string
  asset_id?: number | null
  ref_id?: string | null
}): string {
  const bits = [
    item.key ? `key=${item.key}` : '',
    item.source_kind ? `source=${item.source_kind}` : '',
    item.asset_id != null ? `asset_id=${item.asset_id}` : '',
    item.ref_id ? `ref_id=${item.ref_id}` : '',
    item.filename ? `filename=${item.filename}` : '',
    `reason=${item.reason || 'unavailable'}`,
  ]
  return bits.filter(Boolean).join(' ')
}

function readTaskBatchDownloadField(source: unknown, keys: string[]): string {
  if (!source || typeof source !== 'object') return ''
  const record = source as Record<string, unknown>
  for (const key of keys) {
    const value = record[key]
    if (typeof value === 'string' && value.trim()) return value.trim()
  }
  return ''
}

function resolveTaskReferenceBatchZipFilename(currentTask: unknown): string {
  const sku = readTaskBatchDownloadField(currentTask, ['sku', 'skuCode', 'sku_code', 'primarySkuCode', 'primary_sku_code'])
  const product = readTaskBatchDownloadField(currentTask, [
    'productName',
    'product_name',
    'productNameSnapshot',
    'product_name_snapshot',
    'taskName',
    'task_name',
  ])
  const businessName = [sku, product].filter(Boolean).join('-')
  return buildTimestampedZipFilename(sanitizeZipEntryName(businessName ? `task-references-${businessName}` : 'task-references', 'task-references'))
}

async function handleReferenceBatchDownload() {
  if (referenceBatchDownloading.value) return
  const currentTask = task.value
  if (!currentTask?.id) {
    referenceBatchDownloadError.value = '任务 ID 缺失，无法下载参考图'
    return
  }
  if (totalReferenceCount.value <= 0) {
    referenceBatchDownloadError.value = '暂无参考图可下载'
    return
  }
  referenceBatchDownloading.value = true
  referenceBatchDownloadStatus.value = ''
  referenceBatchDownloadError.value = ''
  try {
    const response = await tasksApi.batchDownloadTaskReferences(currentTask.id)
    const manifest = response.data?.data
    const items = Array.isArray(manifest?.items) ? manifest.items : []
    if (!items.length) {
      referenceBatchDownloadError.value = '没有可下载的参考图'
      return
    }
    const failures = Array.isArray(manifest?.failures) ? manifest.failures : []
    const result = await downloadBatchAsZip({
      items: items.map((item, index) => ({
        key: item.key || `ref-${index + 1}`,
        filename: item.filename,
        downloadURL: item.download_url,
        fallbackName: item.asset_id ? `asset-${item.asset_id}` : `ref-${index + 1}`,
        failureHint: formatTaskReferenceBatchFailure({
          key: item.key,
          source_kind: item.source_kind,
          asset_id: item.asset_id ?? null,
          ref_id: item.ref_id ?? null,
          filename: item.filename,
          reason: 'fetch_failed',
        }),
      })),
      zipFilename: resolveTaskReferenceBatchZipFilename(currentTask),
      serverFailures: failures.map((entry) => formatTaskReferenceBatchFailure(entry)),
      onStatus: (message) => {
        referenceBatchDownloadStatus.value = message
      },
    })
    referenceBatchDownloadStatus.value = `已生成 ZIP，共 ${items.length} 个文件`
    if (result.failureCount > 0) {
      referenceBatchDownloadError.value = `部分文件下载失败（${result.failureCount} 项），ZIP 内已附带 download_errors.txt`
    } else {
      flashSuccess(`参考图打包完成（${items.length} 个）`)
    }
  } catch (err) {
    referenceBatchDownloadError.value = resolveApiUserMessage(err, { fallback: '下载参考图失败，请稍后重试' })
    referenceBatchDownloadStatus.value = ''
  } finally {
    referenceBatchDownloading.value = false
  }
}

const AUDIT_UPLOAD_ALLOWED_KINDS = new Set<AssetKind>(['source', 'delivery'])

function validateAuditUploadFiles(files: File[], kind: AssetKind): { validFiles: File[]; errors: string[] } {
  const errors: string[] = []
  if (!AUDIT_UPLOAD_ALLOWED_KINDS.has(kind)) {
    return { validFiles: [], errors: ['审核阶段仅允许上传 source 或 delivery 类型资产'] }
  }
  const oversized = files.filter((file) => file.size > DESIGN_UPLOAD_MAX_FILE_SIZE_BYTES)
  if (oversized.length > 0) {
    errors.push(
      oversized.length === 1
        ? designUploadTooLargeMessage(oversized[0]?.name)
        : `有 ${oversized.length} 个文件超过上传上限，已拒绝上传`,
    )
  }
  const unsupported = files.filter((file) => !isAllowedUploadFile(file.name))
  if (unsupported.length > 0) {
    errors.push(
      unsupported.length === 1
        ? `不支持的文件类型：${unsupported[0]?.name ?? ''}`
        : `有 ${unsupported.length} 个文件类型不受支持，已拒绝上传`,
    )
  }
  const invalidDelivery = kind === 'delivery' ? files.filter((file) => !isBitmapDeliveryFile(file.name)) : []
  if (invalidDelivery.length > 0) {
    errors.push(
      invalidDelivery.length === 1
        ? `最终成品图仅支持 JPG / PNG / WebP：${invalidDelivery[0]?.name ?? ''}`
        : `有 ${invalidDelivery.length} 个文件不是 JPG / PNG / WebP，已拒绝上传`,
    )
  }
  const validFiles = files.filter(
    (file) =>
      file.size > 0 &&
      file.size <= DESIGN_UPLOAD_MAX_FILE_SIZE_BYTES &&
      isAllowedUploadFile(file.name) &&
      (kind !== 'delivery' || isBitmapDeliveryFile(file.name)),
  )
  return { validFiles, errors }
}

async function handleAuditAssetUpload(e: Event, kind: AssetKind) {
  const input = e.target as HTMLInputElement
  const files = input.files
  input.value = ''
  await handleAuditAssetFiles(files ?? [], kind)
}

async function handleAuditAssetFiles(files: FileList | File[], kind: AssetKind) {
  const currentTask = task.value
  if (!files.length || !currentTask?.id) return
  const picked = Array.from(files)
  auditAssetUploadError.value = ''
  auditAssetUploadStatus.value = ''

  if (!canUploadAuditAssets.value) {
    auditAssetUploadError.value = '当前状态不可上传审核资产'
    return
  }

  const { validFiles, errors } = validateAuditUploadFiles(picked, kind)
  auditAssetUploadError.value = errors.join('；')
  if (!validFiles.length) return

  actionLoading.value = 'audit-upload'
  const targetSku = targetSkuCodeForUpload(
    currentTask,
    selectionFromProductIndex(currentTask, detailProductIndex.value),
    { isPurchase: isPurchaseTask.value },
  )
  auditAssetUploadStatus.value = `上传中 0/${validFiles.length}`
  try {
    for (let index = 0; index < validFiles.length; index += 1) {
      const file = validFiles[index]!
      auditAssetUploadStatus.value = `上传中 ${index + 1}/${validFiles.length}：${file.name}`
      await uploadTaskFileViaAssetSession(currentTask.id, file, {
        asset_kind: kind,
        target_sku_code: targetSku || undefined,
        remark: file.name,
      })
    }
    await tasksStore.loadTaskById(currentTask.id)
    auditAssetUploadStatus.value = kind === 'delivery' ? '最终成品图已上传' : '审核修订源文件已上传'
    flashSuccess(auditAssetUploadStatus.value)
  } catch (err) {
    auditAssetUploadError.value = formatUploadFailureMessage('part_upload', err)
    auditAssetUploadStatus.value = ''
  } finally {
    actionLoading.value = ''
  }
}

function navigateBackToTaskList() {
  void router.push('/tasks')
}

function openProductManagement(record?: ProductManagementRecord): void {
  const keyword = String(record?.sku_code || task.value?.taskNo || '').trim()
  void router.push({
    name: 'ProductManagement',
    query: keyword ? { keyword, issue_scope: 'all' } : { issue_scope: 'all' },
  })
}

async function loadProductManagementRecords(): Promise<void> {
  const currentID = Number(taskId.value)
  if (!currentID || Number.isNaN(currentID) || !canAccessPage('product_management')) {
    productManagementRecords.value = []
    productManagementError.value = ''
    return
  }
  productManagementLoading.value = true
  productManagementError.value = ''
  try {
    productManagementRecords.value = await productManagementApi.listByTask(currentID)
    void resolveProductManagementPreviewURLs(productManagementRecords.value)
  } catch (err) {
    productManagementError.value = resolveApiUserMessage(err, { fallback: '读取产品管理状态失败' })
  } finally {
    productManagementLoading.value = false
  }
}

function replaceProductManagementRecord(next: ProductManagementRecord): void {
  const idx = productManagementRecords.value.findIndex((item) => item.id === next.id)
  if (idx >= 0) {
    productManagementRecords.value.splice(idx, 1, next)
  }
  void resolveProductManagementPreviewURLs([next])
}

async function reparseProductManagementImage(record: ProductManagementRecord): Promise<void> {
  try {
    replaceProductManagementRecord(await productManagementApi.reparseImage(record.id))
  } catch (err) {
    productManagementError.value = resolveApiUserMessage(err, { fallback: '重新解析 ERP 图片失败' })
  }
}

async function syncProductManagementBaseRecord(record: ProductManagementRecord): Promise<void> {
  try {
    replaceProductManagementRecord(await productManagementApi.requestBaseSync(record.id))
    flashSuccess('已提交 ERP 基础资料同步')
  } catch (err) {
    productManagementError.value = resolveApiUserMessage(err, { fallback: '提交 ERP 基础资料同步失败' })
  }
}

async function syncProductManagementImageRecord(record: ProductManagementRecord): Promise<void> {
  try {
    replaceProductManagementRecord(await productManagementApi.requestImageSync(record.id))
    flashSuccess('已提交 ERP 图片同步')
  } catch (err) {
    productManagementError.value = resolveApiUserMessage(err, { fallback: '提交 ERP 图片同步失败' })
  }
}

function startProductManagementImageUpload(record: ProductManagementRecord): void {
  productManagementError.value = ''
  productManagementUploadTarget.value = record
  if (productManagementUploadInput.value) {
    productManagementUploadInput.value.value = ''
    productManagementUploadInput.value.click()
  }
}

async function onProductManagementImagePicked(event: Event): Promise<void> {
  const input = event.target as HTMLInputElement | null
  const file = input?.files?.[0]
  const record = productManagementUploadTarget.value
  if (!file || !record || !task.value?.id) return
  productManagementUploadingID.value = record.id
  productManagementError.value = ''
  try {
    const uploaded = await uploadTaskFileViaAssetSession(String(task.value.id), file, {
      asset_kind: 'erp_product_image',
      target_sku_code: record.sku_code || undefined,
      remark: `ERP 商品图：${record.sku_code || record.task_no || file.name}`,
    })
    const assetID = extractProductManagementUploadedAssetID(uploaded)
    if (!assetID) {
      throw new Error('上传完成但未返回资产 ID')
    }
    const updated = await productManagementApi.setManualImage(record.id, assetID)
    replaceProductManagementRecord(updated)
    flashSuccess('ERP 商品图已上传并绑定')
  } catch (err) {
    productManagementError.value = resolveApiUserMessage(err, { fallback: '上传 ERP 商品图失败' })
  } finally {
    productManagementUploadingID.value = null
    productManagementUploadTarget.value = null
    if (input) input.value = ''
  }
}

function extractProductManagementUploadedAssetID(uploaded: unknown): number | null {
  const root = (uploaded && typeof uploaded === 'object' ? uploaded : {}) as Record<string, unknown>
  const asset = (root.asset && typeof root.asset === 'object' ? root.asset : {}) as Record<string, unknown>
  const raw = asset.id ?? asset.asset_id
  const id = Number(raw)
  return Number.isFinite(id) && id > 0 ? id : null
}

function formatProductManagementCost(record: ProductManagementRecord): string {
  const value = record.cost_price
  if (typeof value !== 'number' || value <= 0) return '成本待维护'
  return `￥${value.toFixed(3).replace(/0+$/, '').replace(/\.$/, '')}`
}

function productManagementERPIID(record: ProductManagementRecord): string {
  return record.erp_i_id?.trim() || record.product_i_id?.trim() || '未绑定 ERP 款式'
}

function productManagementPreviewURL(record: ProductManagementRecord): string {
  return productManagementPreviewURLs.value[record.id] || directProductManagementPreviewURL(record.image_preview_url) || ''
}

function productManagementPreviewAssetID(record: ProductManagementRecord): string | undefined {
  const raw = record.image_asset_id ?? productManagementAssetIDFromPreviewPath(record.image_preview_url)
  const id = Number(raw)
  return Number.isSafeInteger(id) && id > 0 ? String(id) : undefined
}

function productManagementPreviewLoadable(record: ProductManagementRecord): boolean {
  return Boolean(productManagementPreviewAssetID(record) || productManagementPreviewURL(record))
}

function productManagementLightboxItems(activeRecord?: ProductManagementRecord): ImagePreviewLightboxItem[] {
  return productManagementPreviewRecords.value
    .map((record) => {
      const src = record.id === activeRecord?.id ? '' : productManagementPreviewURL(record)
      const assetId = productManagementPreviewAssetID(record)
      if (!src && !assetId) return null
      const title = record.sku_code || record.task_no || 'ERP 商品图'
      return {
        src,
        previewAssetId: assetId,
        resolvedPreviewUrl: src || undefined,
        fallbackSrc: directProductManagementPreviewURL(record.image_preview_url) || undefined,
        title,
        alt: title,
        preferredFilename: title,
        downloadUrl: src || directProductManagementPreviewURL(record.image_preview_url) || '',
      }
    })
    .filter((item) => item != null) as ImagePreviewLightboxItem[]
}

function openProductManagementImagePreview(
  record: ProductManagementRecord,
  url: string,
  context?: {
    assetId?: string
    fallbackAssetId?: string
    fallbackSrc?: string
    resolvedPreviewUrl?: string
  },
): void {
  const activeUrl = url.trim()
  if (!activeUrl) return
  const title = record.sku_code || record.task_no || 'ERP 商品图'
  const items = productManagementLightboxItems(record)
  const index = Math.max(0, items.findIndex((item) => item.previewAssetId === productManagementPreviewAssetID(record)))
  const activeItem: ImagePreviewLightboxItem = {
    src: activeUrl,
    previewAssetId: context?.assetId || productManagementPreviewAssetID(record),
    fallbackAssetId: context?.fallbackAssetId,
    fallbackSrc: context?.fallbackSrc || directProductManagementPreviewURL(record.image_preview_url) || undefined,
    resolvedPreviewUrl: context?.resolvedPreviewUrl || productManagementPreviewURL(record) || undefined,
    title,
    alt: title,
    preferredFilename: title,
    downloadUrl: productManagementPreviewURL(record) || activeUrl,
  }
  const nextItems = items.length > 0 ? [...items] : [activeItem]
  nextItems[Math.min(index, nextItems.length - 1)] = activeItem
  openLightbox(activeUrl, {
    title,
    items: nextItems,
    index,
  })
}

async function resolveProductManagementPreviewURLs(items: ProductManagementRecord[]): Promise<void> {
  const next = { ...productManagementPreviewURLs.value }
  await Promise.all(
    items.map(async (item) => {
      const assetID = item.image_asset_id ?? productManagementAssetIDFromPreviewPath(item.image_preview_url)
      const url = await resolveProductManagementAssetPreviewURL(assetID, item.image_preview_url)
      if (url) next[item.id] = url
      else delete next[item.id]
    }),
  )
  productManagementPreviewURLs.value = next
}

async function resolveProductManagementAssetPreviewURL(assetID?: number | null, fallback?: string): Promise<string> {
  const direct = directProductManagementPreviewURL(fallback)
  if (direct) return direct
  if (!assetID || assetID <= 0) return ''
  const result = await fetchAssetPreviewMeta(String(assetID)).catch(() => null)
  return result?.status === 'ok' && result.displayUrl ? result.displayUrl : ''
}

function directProductManagementPreviewURL(raw?: string): string {
  const value = String(raw ?? '').trim()
  if (!value) return ''
  if (/^(https?:|data:|blob:)/i.test(value)) return value
  return ''
}

function productManagementAssetIDFromPreviewPath(raw?: string): number | undefined {
  const match = String(raw ?? '').match(/\/v1\/assets\/(\d+)\/preview\b/)
  if (!match) return undefined
  const id = Number(match[1])
  return Number.isSafeInteger(id) && id > 0 ? id : undefined
}

function productManagementSyncStatusLabel(status: ProductSyncStatus): string {
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

async function loadTask() {
  if (!taskId.value) return

  detailLoading.value = true
  detailError.value = null

  if (taskId.value && !isTempId.value) {
    try {
      await tasksStore.loadTaskById(taskId.value)
    } catch (e) {
      // 允许按旧逻辑做列表兜底：但门控只在可渲染时打开
      detailError.value = e instanceof Error ? e.message : '刷新任务失败'
      try {
        await tasksStore.loadTasks()
      } catch {
        // ignore
      }
      if (tasksStore.getById(taskId.value)) detailError.value = null
    }
  } else {
    // 临时/无效 id 场景：保持与旧逻辑一致，转去任务列表
    try {
      await tasksStore.loadTasks()
    } finally {
      // 该分支一般不会真正渲染详情
    }
  }

  detailLoading.value = false
  if (taskId.value && !isTempId.value) {
    void loadOpsReferenceBackendAssets()
    void loadTaskPredictions()
    void loadProductManagementRecords()
  } else {
    opsReferenceBackendAssets.value = []
    taskPredictionSuggestions.value = []
    productManagementRecords.value = []
  }
}

async function loadTaskPredictions(): Promise<void> {
  if (!taskId.value || isTempId.value) {
    taskPredictionSuggestions.value = []
    return
  }
  taskPredictionSuggestions.value = []
  taskPredictionLoading.value = true
  try {
    const bundle = await predictionsApi.taskNextActions(taskId.value, { limit: 5 })
    taskPredictionSuggestions.value = bundle.suggestions
  } catch {
    taskPredictionSuggestions.value = []
  } finally {
    taskPredictionLoading.value = false
  }
}

function handleTaskPrediction(item: PredictionSuggestion): void {
  if (item.action_type === 'open_task_assets') {
    openTaskAssetsPage()
    return
  }
  if (item.target_type === 'asset' && item.target_id) {
    void router.push({ name: 'AssetDetail', params: { id: item.target_id } })
    return
  }
  if (item.target_type === 'task' && item.target_id && item.target_id !== taskId.value) {
    void router.push({ name: 'TaskDetail', params: { id: item.target_id } })
  }
}

async function refreshDetail() {
  await loadTask()
  await loadSideEvents()
  flashSuccess('任务详情已刷新')
}

const actionError = ref('')
const actionSuccess = ref('')
const AUDIT_REJECT_REASON_OTHER = '其他'
const AUDIT_REJECT_REASON_OPTIONS = [
  { value: '文案错误', label: '文案错误' },
  { value: '图片错误', label: '图片错误' },
  { value: '保存格式错误', label: '保存格式错误' },
  { value: '尺寸错误', label: '尺寸错误' },
  { value: '订单备注错误', label: '订单备注错误' },
  { value: '无边框线', label: '无边框线' },
  { value: '边框线没闭合', label: '边框线没闭合' },
  { value: '排版错误', label: '排版错误' },
  { value: '缺少素材', label: '缺少素材' },
  { value: '素材模糊', label: '素材模糊' },
  { value: AUDIT_REJECT_REASON_OTHER, label: '其他' },
]
const auditRejectReasonCategory = ref('')
const auditComment = ref('')
const auditCommentError = ref('')
const actionLoading = ref<
  | ''
  | 'claim-retouch'
  | 'submit-retouch'
  | 'audit-pass'
  | 'audit-reject'
  | 'audit-upload'
  | 'warehouse-receive'
  | 'warehouse-reject'
  | 'warehouse-archive'
>('')
const assignDialogVisible = ref(false)
const reassignDialogVisible = ref(false)
const eventLogOpen = ref(false)
const aiSummaryOpen = ref(false)
const aiSummaryLoading = ref(false)
const aiSummaryError = ref('')
const aiSummary = ref<TaskAiSummaryResponse | null>(null)
const openCancel = ref(false)
const cancelErrorText = ref('')
const cancelSuggestForce = ref(false)

const sideEventsLoading = ref(false)
const sideEventsError = ref('')
const sideEvents = ref<RecentEvent[]>([])

function formatSideEventTime(ev: RecentEvent): string {
  const iso = ev.createdAtIso
  if (iso) {
    const compact = formatMonthDayTimeBeijingOffsetAware(iso)
    if (compact) return compact
  }
  return ev.at || ''
}

const sideEventsView = computed(() =>
  sideEvents.value.slice(0, 6).map((e) => {
    const time = formatSideEventTime(e)
    const refLabel = e.refNo && e.refNo !== '—' ? e.refNo : ''
    const headline = e.summary
      ? [e.summary.trim(), refLabel && !e.summary.includes(refLabel) ? refLabel : ''].filter(Boolean).join(' · ')
      : [e.title, refLabel, e.replacement_note].filter(Boolean).join(' · ')
    const subline = time
    return { id: e.id, headline, subline }
  }),
)

async function loadSideEvents() {
  const tid = taskId.value
  sideEventsError.value = ''
  if (!tid || isTempId.value) {
    sideEvents.value = []
    return
  }
  sideEventsLoading.value = true
  try {
    const shouldLoadCostEvents = canReadCostOverrideTimeline.value
    const [taskEventsResult, costEventsResult] = await Promise.allSettled([
      tasksApi.listTaskEvents(tid),
      shouldLoadCostEvents ? tasksApi.getCostOverrides(tid) : Promise.resolve(null),
    ])
    if (
      taskEventsResult.status === 'rejected' &&
      (!shouldLoadCostEvents || costEventsResult.status === 'rejected')
    ) {
      throw taskEventsResult.reason
    }
    const taskEvents =
      taskEventsResult.status === 'fulfilled'
        ? extractTaskEventsList(taskEventsResult.value.data).map((row) =>
            mapTaskEventRowToRecentEvent(row, tid),
          )
        : []
    const costEvents =
      shouldLoadCostEvents && costEventsResult.status === 'fulfilled' && costEventsResult.value
        ? extractCostOverrideEventsList(costEventsResult.value.data).map((row) =>
            mapCostOverrideEventToRecentEvent(row, tid, task.value?.taskNo),
          )
        : []
    sideEvents.value = [...costEvents, ...taskEvents].sort((a, b) => {
      const at = a.createdAtIso ? Date.parse(a.createdAtIso) : 0
      const bt = b.createdAtIso ? Date.parse(b.createdAtIso) : 0
      return bt - at
    })
  } catch (e) {
    sideEventsError.value = e instanceof Error ? e.message : '事件加载失败'
  } finally {
    sideEventsLoading.value = false
  }
}

function unwrapAiSummaryResponse(payload: unknown): TaskAiSummaryResponse | null {
  if (!payload || typeof payload !== 'object') return null
  const root = payload as Record<string, unknown>
  const nested = root.data && typeof root.data === 'object' ? (root.data as Record<string, unknown>) : root
  return nested as unknown as TaskAiSummaryResponse
}

async function openAiSummary() {
  aiSummaryOpen.value = true
  if (!aiSummary.value && !aiSummaryLoading.value) {
    await loadAiSummary()
  }
}

async function loadAiSummary() {
  const tid = taskId.value
  if (!tid || isTempId.value) return
  aiSummaryLoading.value = true
  aiSummaryError.value = ''
  try {
    const response = await tasksApi.generateAiSummary(tid)
    const summary = unwrapAiSummaryResponse(response.data)
    if (!summary) throw new Error('AI 摘要返回为空')
    aiSummary.value = {
      ...summary,
      actions: Array.isArray(summary.actions) ? summary.actions : [],
      evidence: Array.isArray(summary.evidence) ? summary.evidence : [],
      people: Array.isArray(summary.people) ? summary.people : [],
      timeline: Array.isArray(summary.timeline) ? summary.timeline : [],
      stuck_points: Array.isArray(summary.stuck_points) ? summary.stuck_points : [],
      sku_asset_erp_cost: Array.isArray(summary.sku_asset_erp_cost) ? summary.sku_asset_erp_cost : [],
      next_actions: Array.isArray(summary.next_actions) ? summary.next_actions : [],
    }
  } catch (e) {
    aiSummaryError.value = resolveApiUserMessage(e) || 'AI 摘要生成失败'
  } finally {
    aiSummaryLoading.value = false
  }
}

const aiSummaryDecision = computed(() => {
  const summary = aiSummary.value
  return summary?.decision?.trim() || summary?.headline?.trim() || '系统暂无明确判断'
})

const aiSummaryImpact = computed(() => {
  const summary = aiSummary.value
  return summary?.impact?.trim() || summary?.current_status?.trim() || '系统暂无影响说明。'
})

const aiSummaryBlocker = computed(() => {
  const summary = aiSummary.value
  const blocker = summary?.primary_blocker
  if (blocker && (blocker.title || blocker.reason || blocker.owner)) {
    return {
      title: blocker.title || '待确认卡点',
      owner: blocker.owner || '',
      reason: blocker.reason || '',
    }
  }
  const point = summary?.stuck_points?.[0]
  if (point) {
    return {
      title: point.title || '待确认卡点',
      owner: point.owner || '',
      reason: point.reason || '',
    }
  }
  return { title: '暂未识别主卡点', owner: '', reason: '系统暂无明确异常证据。' }
})

const aiSummaryActionList = computed(() => {
  const summary = aiSummary.value
  const actions = summary?.actions?.filter((item) => item && item.action?.trim()) ?? []
  if (actions.length) return actions.slice(0, 3)
  return (summary?.next_actions ?? [])
    .filter((action) => action?.trim())
    .slice(0, 3)
    .map((action) => ({ role: '相关责任人', action, timing: '下一步' }))
})

const aiSummaryEvidenceLines = computed(() => {
  const summary = aiSummary.value
  const direct = summary?.evidence?.filter((line) => line?.trim()) ?? []
  if (direct.length) return direct.slice(0, 4)
  const lines: string[] = []
  for (const item of summary?.sku_asset_erp_cost ?? []) {
    const line = [item.sku, item.erp_status, item.cost_status, item.asset_status].filter(Boolean).join(' · ')
    if (line) lines.push(line)
  }
  for (const item of summary?.timeline ?? []) {
    const line = [item.stage, item.actor, item.summary].filter(Boolean).join(' · ')
    if (line) lines.push(line)
  }
  return [...new Set(lines)].slice(0, 4)
})

watch(taskId, () => {
  if (!taskId.value || isTempId.value) return
  aiSummary.value = null
  aiSummaryError.value = ''
  auditRejectReasonCategory.value = ''
  auditComment.value = ''
  auditCommentError.value = ''
  void loadSideEvents()
})

watch(auditComment, () => {
  if (auditCommentError.value) auditCommentError.value = ''
})
const assignDesignerWorkflowLane = computed(() =>
  isCustomizationTask.value ? ('customization' as const) : undefined,
)

const {
  designers: designerOptions,
  loading: designersLoading,
  loadDesigners,
} = useDesignerOptions({
  includeEmpty: false,
  autoLoad: false,
  requiredActions: [
    'task.assign',
    'task.assign.team',
    'task.assign.department',
    'task.reassign',
    'task.reassign.team',
    'task.reassign.department',
    'task.create',
  ],
  workflowLane: assignDesignerWorkflowLane,
})

let successClearTimer: ReturnType<typeof setTimeout> | null = null
function flashSuccess(message: string) {
  actionError.value = ''
  actionSuccess.value = message
  if (successClearTimer) clearTimeout(successClearTimer)
  successClearTimer = setTimeout(() => {
    actionSuccess.value = ''
    successClearTimer = null
  }, 6000)
}

function doAssign() {
  if (!task.value || !showAssignDesignerButton.value) return
  actionError.value = ''
  actionSuccess.value = ''
  assignDialogVisible.value = true
  void loadDesigners()
}

function doReassign() {
  if (!task.value || !showReassignDesignerButton.value) return
  actionError.value = ''
  actionSuccess.value = ''
  reassignDialogVisible.value = true
  if (designerOptions.value.length === 0) loadDesigners()
}

function auditStageForTask(): string {
  const status = String(task.value?.status ?? '')
  if (status === 'PendingAuditB' || status === 'RejectedByAuditB') return 'B'
  return 'A'
}

async function runDetailAction(
  key: Exclude<typeof actionLoading.value, ''>,
  fallback: string,
  action: () => Promise<void>,
): Promise<void> {
  if (actionLoading.value) return
  actionLoading.value = key
  actionError.value = ''
  actionSuccess.value = ''
  try {
    await action()
  } catch (e) {
    actionError.value = formatTaskActionDenyMessage(e, fallback)
  } finally {
    actionLoading.value = ''
  }
}

async function claimRetouchFromDetail(): Promise<void> {
  if (!task.value || !showRetouchClaimAction.value) return
  await runDetailAction('claim-retouch', '领取精修任务失败', async () => {
    try {
      await tasksStore.claimRetouchModule(task.value!.id)
    } catch (err: unknown) {
      const code =
        (err as any)?.response?.data?.error?.code ??
        (err as any)?.response?.data?.code
      if (code === 'task_already_claimed') {
        await tasksStore.loadTaskById(task.value!.id)
        flashSuccess('任务已被认领，已刷新最新状态')
        void loadSideEvents()
        return
      }
      throw err
    }
    flashSuccess('已领取精修任务，可以开始上传精修稿并提交')
    void loadSideEvents()
  })
}

async function passAuditFromDetail(): Promise<void> {
  if (!task.value) return
  if (!showActiveAuditActionButtons.value) return
  auditCommentError.value = ''
  const comment = auditComment.value.trim() || '审核通过'
  if (showCustomizationReviewActionButtons.value) {
    await runDetailAction('audit-pass', '定制审核通过失败', async () => {
      await tasksStore.submitCustomizationReview(task.value!.id, {
        customization_review_decision: 'approved',
        customization_note: comment,
      })
      auditRejectReasonCategory.value = ''
      auditComment.value = ''
      flashSuccess('定制审核已通过，任务已进入仓库接收')
      void loadSideEvents()
    })
    return
  }
  await runDetailAction('audit-pass', '审核通过失败', async () => {
    await tasksStore.passAudit(task.value!.id, {
      stage: auditStageForTask(),
      next_status: 'PendingWarehouseReceive',
      comment,
    })
    auditRejectReasonCategory.value = ''
    auditComment.value = ''
    flashSuccess('已审核通过')
    void loadSideEvents()
  })
}

async function rejectAuditFromDetail(): Promise<void> {
  if (!task.value) return
  if (!showActiveAuditActionButtons.value) return
  const category = auditRejectReasonCategory.value.trim()
  const comment = auditComment.value.trim()
  if (!category) {
    auditCommentError.value = '请选择驳回分类'
    return
  }
  if (category === AUDIT_REJECT_REASON_OTHER && !comment) {
    auditCommentError.value = '选择其他时请填写具体理由'
    return
  }
  const rejectComment = comment ? `${category}：${comment}` : category
  auditCommentError.value = ''
  if (showCustomizationReviewActionButtons.value) {
    await runDetailAction('audit-reject', '定制审核打回失败', async () => {
      await tasksStore.submitCustomizationReview(task.value!.id, {
        reviewer_id: currentUser.value?.id ?? '',
        customization_review_decision: 'return_to_designer',
        customization_note: rejectComment,
      })
      auditRejectReasonCategory.value = ''
      auditComment.value = ''
      flashSuccess('已打回美工处理')
      void loadSideEvents()
    })
    return
  }
  await runDetailAction('audit-reject', '审核打回失败', async () => {
    await tasksStore.rejectAudit(task.value!.id, {
      stage: auditStageForTask(),
      comment: rejectComment,
    })
    auditRejectReasonCategory.value = ''
    auditComment.value = ''
    flashSuccess('已打回设计处理')
    void loadSideEvents()
  })
}

async function receiveWarehouseFromDetail(): Promise<void> {
  if (!task.value) return
  if (!showWarehouseReceiveActionButtons.value) return
  await runDetailAction('warehouse-receive', '仓库接收失败', async () => {
    const currentTask = task.value
    if (!currentTask) return
    if (isPurchaseTask.value) {
      if (currentTask.procurementApiStatus !== 'completed') {
        const bootstrapPayload = resolveProcurementBootstrapPayload(currentTask)
        if (!bootstrapPayload) {
          throw new Error('请先维护采购信息：采购价、采购数量')
        }
        await tasksStore.bootstrapProcurement(currentTask.id, bootstrapPayload)
      }

      try {
        await tasksApi.warehousePrepare(currentTask.id)
        await tasksStore.loadTaskById(currentTask.id)
      } catch (err: unknown) {
        if (isWarehouseProgressConflictError(err)) {
          await tasksStore.loadTaskById(currentTask.id)
        } else {
          const friendly = parseWarehouseActionError(err)
          if (friendly) throw new Error(friendly)
          throw err
        }
      }
      try {
        await tasksStore.receiveInWarehouse(currentTask.id)
      } catch (err: unknown) {
        if (isWarehouseProgressConflictError(err)) {
          await tasksStore.loadTaskById(currentTask.id)
        } else {
          const friendly = parseWarehouseActionError(err)
          if (friendly) throw new Error(friendly)
          throw err
        }
      }
      try {
        await tasksStore.completeWarehouseFlow(currentTask.id)
      } catch (err: unknown) {
        if (isWarehouseProgressConflictError(err)) {
          await tasksStore.loadTaskById(currentTask.id)
          return
        }
        const friendly = parseWarehouseActionError(err)
        if (friendly) throw new Error(friendly)
        throw err
      }
    } else {
      await tasksStore.receiveInWarehouse(currentTask.id)
    }
    flashSuccess(isPurchaseTask.value ? '已接收入库并推进到可结单状态' : '已接收入库')
    void loadSideEvents()
  })
}

async function rejectWarehouseFromDetail(): Promise<void> {
  if (!task.value) return
  if (!showWarehouseReturnActionButton.value) return
  await runDetailAction('warehouse-reject', '仓库退回失败', async () => {
    await tasksStore.rejectInWarehouse(task.value!.id, {
      reject_reason: '仓库退回',
      reject_category: 'quality',
    })
    flashSuccess('已退回')
    void loadSideEvents()
  })
}

async function archiveWarehouseFromDetail(): Promise<void> {
  if (!task.value) return
  if (!showWarehouseCompleteActionButton.value && !canCloseTask.value) return
  await runDetailAction('warehouse-archive', '仓库处理或结单失败', async () => {
    if (isPurchaseTask.value) {
      const currentTask = task.value!
      if (currentTask.workflowCanClose === true) {
        await tasksApi.closeTask(currentTask.id, {})
        await tasksStore.loadTaskById(currentTask.id)
        flashSuccess('任务已结单')
        void loadSideEvents()
        return
      }
      if (currentTask.procurementApiStatus !== 'completed') {
        const bootstrapPayload = resolveProcurementBootstrapPayload(currentTask)
        if (!bootstrapPayload) {
          throw new Error('请先维护采购信息：采购价、采购数量')
        }
        await tasksStore.bootstrapProcurement(currentTask.id, bootstrapPayload)
      }

      try {
        await tasksApi.warehousePrepare(currentTask.id)
        await tasksStore.loadTaskById(currentTask.id)
      } catch (err) {
        if (isWarehouseProgressConflictError(err)) {
          await tasksStore.loadTaskById(currentTask.id)
        } else {
          const friendly = parseWarehouseActionError(err)
          if (friendly) throw new Error(friendly)
          throw err
        }
      }
      try {
        await tasksStore.receiveInWarehouse(currentTask.id)
      } catch (err) {
        if (isWarehouseProgressConflictError(err)) {
          await tasksStore.loadTaskById(currentTask.id)
        } else {
          const friendly = parseWarehouseActionError(err)
          if (friendly && !friendly.includes('已接收')) throw new Error(friendly)
        }
      }
      try {
        await tasksStore.completeWarehouseFlow(currentTask.id)
      } catch (err) {
        if (isWarehouseProgressConflictError(err)) {
          await tasksStore.loadTaskById(currentTask.id)
        } else {
          const friendly = parseWarehouseActionError(err)
          if (friendly) throw new Error(friendly)
          throw err
        }
      }
      try {
        await tasksApi.closeTask(currentTask.id, {})
        await tasksStore.loadTaskById(currentTask.id)
      } catch (err) {
        throw new Error(formatCloseArchiveError(err))
      }
    } else {
      await tasksStore.archiveTask(task.value!.id)
    }
    flashSuccess(
      task.value?.status === 'PendingClose' || task.value?.status === 'Completed'
        ? '已结单'
        : '已完成仓库处理，请使用顶部「结单」完成归档',
    )
    void loadSideEvents()
  })
}

function resolveProcurementBootstrapPayload(current: NonNullable<typeof task.value>) {
  const procurementPrice =
    current.purchaseInfo?.purchasePrice?.amount ??
    current.costPrice?.amount ??
    current.newProductCostUnitPrice
  const procurementQuantity =
    current.purchaseInfo?.quantity ??
    current.newProductQuantity ??
    (current.skuItems?.[0]?.quantity as number | undefined)
  const supplierName = String(current.purchaseInfo?.supplierName ?? '').trim()
  if (
    typeof procurementPrice !== 'number' ||
    !Number.isFinite(procurementPrice) ||
    typeof procurementQuantity !== 'number' ||
    !Number.isFinite(procurementQuantity) ||
    procurementQuantity <= 0
  ) {
    return null
  }
  return {
    procurement_price: procurementPrice,
    quantity: procurementQuantity,
    supplier_name: supplierName || '[默认]',
    purchase_remark: String(current.note ?? '').trim() || undefined,
  }
}

function parseWarehouseActionError(err: unknown): string | null {
  const details =
    (err as any)?.response?.data?.error?.details ??
    (err as any)?.data?.error?.details ??
    (err as any)?.response?.data?.details
  const missingSummary = details?.missing_fields_summary_cn
  if (typeof missingSummary === 'string' && missingSummary.trim()) {
    return missingSummary.trim()
  }
  const missingFields = details?.missing_fields
  if (Array.isArray(missingFields) && missingFields.length > 0) {
    const labels = missingFields
      .map((code: unknown) => workflowGateReasonLabelCn(String(code ?? ''), ''))
      .filter((line: string) => line.trim().length > 0)
    if (labels.length > 0) return labels.join('；')
  }
  const blockingReasons = details?.warehouse_blocking_reasons
  if (Array.isArray(blockingReasons) && blockingReasons.length > 0) {
    const lines = blockingReasons
      .map((r: any) => warehouseBlockingReasonLine(String(r?.code ?? ''), String(r?.message ?? '')))
      .filter((line: string) => line.trim().length > 0)
    if (lines.length > 0) {
      return lines.join('；')
    }
  }
  return null
}

function isWarehouseProgressConflictError(err: unknown): boolean {
  const details =
    (err as any)?.response?.data?.error?.details ??
    (err as any)?.data?.error?.details ??
    (err as any)?.response?.data?.details
  const reasons = details?.warehouse_blocking_reasons
  if (!Array.isArray(reasons)) return false
  const codes = new Set(['warehouse_already_received', 'warehouse_already_completed'])
  return reasons.some(
    (item: any) => codes.has(String(item?.code ?? '').trim().toLowerCase()),
  )
}

async function onAssignConfirm(payload: { assigneeId: string; assigneeName: string }) {
  if (!task.value || !showAssignDesignerButton.value) return
  try {
    await tasksStore.assignTask(task.value.id, payload)
    assignDialogVisible.value = false
    const roleNoun = isCustomizationTask.value ? '美工' : '设计师'
    flashSuccess(`已指派${roleNoun} ${payload.assigneeName}`)
  } catch (e) {
    actionError.value = formatTaskActionDenyMessage(e, '指派失败')
  }
}

async function onReassignConfirm(payload: {
  mode: 'reassign' | 'clear'
  assigneeId: string | null
  assigneeName: string | null
  reasonLabel: string
  reasonNote: string
}) {
  if (!task.value || !showReassignDesignerButton.value) return
  try {
    if (payload.mode === 'clear') {
      await tasksStore.clearDesignerAssignee(task.value.id, payload.reasonNote || '清空指派')
    } else if (payload.assigneeId && payload.assigneeName) {
      await tasksStore.reassignDesignerTask(task.value.id, {
        assigneeId: payload.assigneeId,
        assigneeName: payload.assigneeName,
      })
    } else {
      throw new Error('请选择新设计师')
    }
    // 不再 forceRefreshList：整表替换会用列表瘦模型覆盖刚 loadTaskById 写入的完整详情（负责人/参考图/批量等丢失）。
    const note = payload.reasonNote ? `（${payload.reasonLabel}：${payload.reasonNote}）` : `（${payload.reasonLabel}）`
    if (payload.mode === 'clear') {
      flashSuccess(`已清空指派，任务已退回待指派 ${note}`)
    } else {
      flashSuccess(`已重新指派给 ${payload.assigneeName} ${note}`)
    }
  } catch (e) {
    actionError.value = formatTaskActionDenyMessage(
      e,
      payload.mode === 'clear' ? '清空指派失败' : '重新指派失败',
    )
  }
}

async function doClose() {
  if (!task.value) return
  if (isPurchaseTask.value) {
    await archiveWarehouseFromDetail()
    return
  }
  actionError.value = ''
  try {
    await tasksStore.archiveTask(task.value.id)
  } catch (e) {
    actionError.value = formatTaskActionDenyMessage(e, formatCloseArchiveError(e))
  }
}

function closeCancelModal() {
  openCancel.value = false
  cancelSuggestForce.value = false
  cancelErrorText.value = ''
}

async function submitCancel(reason: string) {
  if (!task.value) return
  cancelErrorText.value = ''
  cancelSuggestForce.value = false
  if (!reason.trim()) {
    cancelErrorText.value = '请填写终止原因'
    return
  }
  await cancel(task.value.id, { reason: reason.trim(), force: false })
  if (needForceConfirm.value) {
    if (isDeptAdminPlus.value) {
      cancelSuggestForce.value = true
      return
    }
    cancelErrorText.value = '任务存在业务限制，请联系部门管理员执行强制终止'
    return
  }
  closeCancelModal()
  await loadTask()
}

async function submitForceFromCancel(reason: string) {
  if (!task.value) return
  if (!reason.trim()) {
    cancelErrorText.value = '请填写终止原因'
    return
  }
  await cancel(task.value.id, { reason: reason.trim(), force: true })
  closeCancelModal()
  await loadTask()
}

function onRecalibrate() {
  syncStore.setSequenceGap(false)
  loadTask()
}

onMounted(async () => {
  const id = taskId.value
  if (!id) return
  if (isTempId.value) {
    void router.replace('/tasks')
    return
  }
  await loadTask()
  void loadSideEvents()
})
watch(taskId, (id) => {
  if (!id || id.startsWith('t-')) return
  loadTask()
})
</script>

<style scoped>
.task-detail-view {
  min-height: 100dvh;
  background: #eef2f8;
  display: flex;
  flex-direction: column;
}
.detail-shell {
  width: 100%;
  padding: 0.75rem 0.75rem 1.5rem;
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 0.55rem;
}
@media (min-width: 900px) {
  .detail-shell {
    padding-left: 0.75rem;
    padding-right: 0.75rem;
  }
}
/* v6 + V4：圆角工作面（小黑盒 / Pencil V4 token） */
.detail-v6-surface {
  --dv-r-outer: 1.25rem;
  --dv-r-card: 0.875rem;
  --dv-r-control: 0.625rem;
  --dv-surface-elev: 0 1px 3px rgba(15, 23, 42, 0.07);
  --dv-border-soft: #e8ecf4;
  --dw-title: #151a21;
  --dw-label: #667085;
  flex: 1 1 auto;
  min-height: 0;
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 1rem;
  background: transparent;
  border: none;
  border-radius: 0;
  box-shadow: none;
  overflow: visible;
}
.detail-main {
  margin-top: 0;
  min-width: 0;
}
.detail-main-v6 {
  flex: 1 1 auto;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

/* ── 顶栏一体（V4 白卡 + 大圆角） ── */
.detail-top-unified.detail-top-v6 {
  background: #fff;
  border: 1px solid var(--dv-border-soft);
  border-radius: var(--dv-r-outer);
  box-shadow: var(--dv-surface-elev);
  padding: 1.125rem 1.25rem;
}
.detail-top-unified {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}
.detail-top-grid {
  display: grid;
  grid-template-columns: minmax(22rem, 1fr) minmax(30rem, auto) minmax(20rem, 0.9fr);
  gap: 1rem;
  align-items: center;
  min-width: 0;
}
.detail-top-left {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  justify-content: center;
  gap: 0.35rem;
  min-width: 0;
  justify-self: start;
}
.detail-top-left-meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.35rem 0.5rem;
}
.detail-top-identity {
  display: flex;
  flex-direction: column;
  gap: 0.1rem;
  min-width: 0;
}
.detail-top-kicker {
  margin: 0;
  font-size: 0.625rem;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: #94a3b8;
}
.detail-top-left .back-btn {
  margin: 0;
}
.detail-top-taskno {
  margin: 0;
  font-size: 1.65rem;
  font-weight: 900;
  color: #151a21;
  line-height: 1.15;
}
.detail-top-sub {
  margin: 0;
  font-size: 0.8125rem;
  color: #64748b;
  line-height: 1.35;
}
.detail-top-badge-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.4rem 0.5rem;
  margin: 0.35rem 0 0.15rem;
}
.detail-top-type-pill {
  display: inline-flex;
  align-items: center;
  min-height: 1.4rem;
  padding: 0.15rem 0.5rem;
  border-radius: 9999px;
  font-size: 0.6875rem;
  font-weight: 800;
  color: #475467;
  background: #f1f5f9;
}
.detail-top-priority-pill--muted {
  display: inline-flex;
  align-items: center;
  min-height: 1.4rem;
  padding: 0.15rem 0.5rem;
  border-radius: 9999px;
  font-size: 0.6875rem;
  font-weight: 800;
  color: #475467;
  background: #f1f5f9;
  border: 1px solid #e2e8f0;
}
.detail-top-priority-pill--danger {
  display: inline-flex;
  align-items: center;
  min-height: 1.4rem;
  padding: 0.15rem 0.5rem;
  border-radius: 9999px;
  font-size: 0.6875rem;
  font-weight: 800;
  color: #dc2626;
  background: #fef2f2;
  border: 1px solid #fecdd3;
}
.detail-top-current.detail-top-status-pill {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  max-width: 100%;
  margin: 0.5rem 0 0;
  padding: 0.35rem 0.75rem;
  border-radius: 9999px;
  background: #eff6ff;
  color: #1d4ed8;
  font-size: 0.8125rem;
  font-weight: 700;
  line-height: 1.4;
}
.detail-top-batch-pill {
  display: inline-flex;
  align-items: center;
  margin: 0.45rem 0 0;
  padding: 0.28rem 0.62rem;
  border-radius: 9999px;
  background: #f5f3ff;
  color: #6d28d9;
  font-size: 0.75rem;
  font-weight: 700;
}
.detail-top-status-dot {
  display: inline-block;
  width: 0.4rem;
  height: 0.4rem;
  flex: 0 0 auto;
  border-radius: 50%;
  background: #2f80ed;
  box-shadow: 0 0 0 3px rgba(47, 128, 237, 0.2);
}
.detail-top-mid {
  display: flex;
  flex-direction: row;
  align-items: center;
  justify-content: center;
  min-width: 0;
  justify-self: center;
}
.detail-top-flow-shell {
  width: 100%;
  max-width: 37rem;
  margin: 0 auto;
  background: #f8fafc;
  border: 1px solid var(--dv-border-soft);
  border-radius: 1rem;
  padding: 0.65rem 0.9rem;
  display: flex;
  align-items: center;
  justify-content: center;
}
.detail-top-right {
  display: flex;
  flex-direction: row;
  flex-wrap: wrap;
  align-items: center;
  justify-content: flex-end;
  gap: 0.5rem 0.65rem;
  min-width: 0;
  justify-self: end;
}
.detail-top-badges {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 0.35rem;
  align-items: center;
}
.detail-top-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 0.4rem;
  align-items: center;
  max-width: 100%;
}
.detail-top-chip {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.35rem;
  min-width: 4.75rem;
  height: 2rem;
  padding: 0 0.7rem;
  border: none;
  border-radius: var(--dv-r-control);
  background: #f2f4f7;
  color: #475467;
  font-size: 0.73rem;
  font-weight: 700;
  line-height: 1;
  white-space: nowrap;
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease;
}
.detail-top-chip :deep(span) {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.35rem;
  white-space: nowrap;
}
.detail-top-chip:hover {
  background: #e9edf3;
  color: #1f2937;
}
.detail-top-chip-icon {
  width: 0.86rem;
  height: 0.86rem;
  flex: 0 0 auto;
}
.detail-top-chip:focus-visible {
  outline: 2px solid #98a2b3;
  outline-offset: 2px;
}
.detail-top-chip--danger {
  background: #fff1f2;
  color: #be123c;
}
.detail-top-chip--danger:hover {
  background: #ffe4e6;
  color: #9f1239;
}
.detail-top-chip--primary {
  background: #2563eb;
  color: #fff;
  box-shadow: 0 1px 2px rgba(37, 99, 235, 0.2);
}
.detail-top-chip--primary:hover {
  background: #1d4ed8;
  color: #fff;
}

.ai-summary-modal {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  padding-bottom: 0.75rem;
}
.ai-summary-loading,
.ai-summary-error {
  display: flex;
  align-items: center;
  gap: 0.9rem;
  min-height: 8rem;
  border: 1px solid #e5e7eb;
  border-radius: 0.75rem;
  background: #f8fafc;
  padding: 1rem;
}
.ai-summary-error {
  justify-content: space-between;
  color: #991b1b;
  background: #fff5f5;
  border-color: #fecaca;
}
.ai-summary-loading-dot {
  width: 1rem;
  height: 1rem;
  flex: 0 0 auto;
  border: 2px solid #c7d2fe;
  border-top-color: #2563eb;
  border-radius: 999px;
  animation: ai-summary-spin 0.8s linear infinite;
}
.ai-summary-loading-title {
  margin: 0;
  color: #111827;
  font-weight: 800;
}
.ai-summary-loading-sub {
  margin: 0.25rem 0 0;
  color: #64748b;
  font-size: 0.82rem;
}
.ai-summary-content {
  display: grid;
  gap: 0.9rem;
}
.ai-summary-content--compact {
  gap: 0.75rem;
}
.ai-summary-hero,
.ai-summary-panel {
  border: 1px solid #e5e7eb;
  border-radius: 0.75rem;
  background: #ffffff;
  padding: 1rem;
}
.ai-summary-hero {
  background: linear-gradient(180deg, #eff6ff 0%, #ffffff 100%);
  border-color: #bfdbfe;
}
.ai-summary-eyebrow {
  margin: 0 0 0.35rem;
  color: #2563eb;
  font-size: 0.75rem;
  font-weight: 800;
}
.ai-summary-hero h3,
.ai-summary-panel h4 {
  margin: 0;
  color: #0f172a;
  font-weight: 850;
}
.ai-summary-hero h3 {
  font-size: 1.05rem;
  line-height: 1.45;
}
.ai-summary-hero p:last-child {
  margin: 0.45rem 0 0;
  color: #475569;
  line-height: 1.7;
}
.ai-summary-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 0.9rem;
}
.ai-summary-action-grid {
  display: grid;
  grid-template-columns: minmax(0, 0.9fr) minmax(0, 1.1fr);
  gap: 0.75rem;
}
.ai-summary-blocker {
  display: grid;
  gap: 0.35rem;
  margin-top: 0.75rem;
}
.ai-summary-blocker strong {
  color: #111827;
  font-size: 0.95rem;
  line-height: 1.45;
  overflow-wrap: anywhere;
}
.ai-summary-blocker p {
  margin: 0;
  color: #475569;
  line-height: 1.55;
  overflow-wrap: anywhere;
}
.ai-summary-blocker span {
  color: #b45309;
  font-size: 0.78rem;
  font-weight: 800;
}
.ai-summary-next-actions {
  display: grid;
  gap: 0.55rem;
  margin: 0.75rem 0 0;
  padding: 0;
  list-style: none;
  counter-reset: ai-action;
}
.ai-summary-next-actions li {
  counter-increment: ai-action;
  display: grid;
  grid-template-columns: auto minmax(4rem, auto) minmax(0, 1fr);
  gap: 0.5rem;
  align-items: start;
  padding: 0.6rem 0.7rem;
  border: 1px solid #dbeafe;
  border-radius: 0.6rem;
  background: #f8fbff;
}
.ai-summary-next-actions li::before {
  content: counter(ai-action);
  display: inline-flex;
  width: 1.25rem;
  height: 1.25rem;
  align-items: center;
  justify-content: center;
  border-radius: 999px;
  background: #2563eb;
  color: #fff;
  font-size: 0.72rem;
  font-weight: 900;
}
.ai-summary-next-actions span {
  color: #2563eb;
  font-size: 0.72rem;
  font-weight: 800;
  white-space: nowrap;
}
.ai-summary-next-actions strong {
  min-width: 0;
  color: #111827;
  font-weight: 850;
  overflow-wrap: anywhere;
}
.ai-summary-next-actions p {
  grid-column: 2 / -1;
  margin: 0;
  color: #334155;
  line-height: 1.55;
  overflow-wrap: anywhere;
}
.ai-summary-evidence {
  border: 1px solid #e5e7eb;
  border-radius: 0.7rem;
  background: #ffffff;
  padding: 0.75rem 0.9rem;
}
.ai-summary-evidence summary {
  cursor: pointer;
  color: #475569;
  font-size: 0.8rem;
  font-weight: 850;
}
.ai-summary-evidence ul {
  display: grid;
  gap: 0.45rem;
  margin: 0.7rem 0 0;
  padding-left: 1rem;
  color: #64748b;
  line-height: 1.55;
}
.ai-summary-evidence p {
  margin: 0.7rem 0 0;
  color: #94a3b8;
}
.ai-summary-panel ul,
.ai-summary-actions {
  display: grid;
  gap: 0.65rem;
  margin: 0.75rem 0 0;
  padding: 0;
  list-style: none;
}
.ai-summary-panel li {
  display: grid;
  gap: 0.2rem;
  min-width: 0;
}
.ai-summary-panel li span {
  color: #64748b;
  font-size: 0.74rem;
  font-weight: 700;
}
.ai-summary-panel li strong,
.ai-summary-sku-row strong,
.ai-summary-timeline strong {
  min-width: 0;
  color: #111827;
  font-weight: 800;
  overflow-wrap: anywhere;
}
.ai-summary-panel li small,
.ai-summary-sku-row small {
  color: #64748b;
  line-height: 1.55;
  overflow-wrap: anywhere;
}
.ai-summary-panel--risk {
  border-color: #fed7aa;
  background: #fffaf5;
}
.ai-summary-sku-list {
  display: grid;
  gap: 0.5rem;
  margin-top: 0.75rem;
}
.ai-summary-sku-row {
  display: grid;
  grid-template-columns: minmax(8rem, 1.1fr) repeat(3, minmax(0, 1fr));
  gap: 0.6rem;
  align-items: start;
  padding: 0.65rem 0.75rem;
  border-radius: 0.6rem;
  background: #f8fafc;
}
.ai-summary-sku-row span {
  min-width: 0;
  color: #475569;
  font-size: 0.82rem;
  overflow-wrap: anywhere;
}
.ai-summary-sku-row small {
  grid-column: 1 / -1;
}
.ai-summary-timeline {
  display: grid;
  gap: 0.7rem;
  margin: 0.8rem 0 0;
  padding: 0;
  list-style: none;
}
.ai-summary-timeline li {
  display: grid;
  grid-template-columns: 5.5rem minmax(0, 1fr);
  gap: 0.75rem;
}
.ai-summary-timeline time {
  color: #64748b;
  font-size: 0.76rem;
  font-weight: 700;
}
.ai-summary-timeline div {
  display: grid;
  gap: 0.2rem;
}
.ai-summary-timeline span {
  color: #475569;
  line-height: 1.55;
  overflow-wrap: anywhere;
}
.ai-summary-actions li {
  padding-left: 0.7rem;
  border-left: 3px solid #2563eb;
  color: #334155;
  line-height: 1.55;
}
.ai-summary-muted {
  margin: 0.75rem 0 0;
  color: #94a3b8;
}
.ai-summary-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  width: 100%;
  padding: 0.85rem 1.25rem;
  border-top: 1px solid #e5e7eb;
  background: #ffffff;
}
.ai-summary-meta {
  min-width: 0;
  color: #64748b;
  font-size: 0.78rem;
  overflow-wrap: anywhere;
}
.ai-summary-footer-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 0.5rem;
}
@keyframes ai-summary-spin {
  to {
    transform: rotate(360deg);
  }
}

/* 顶栏 WorkflowProgress 的 Pencil 风格覆写：色点 + 内联文字 */
.detail-top-flow-shell :deep(.workflow-progress--horizontal) {
  flex-wrap: wrap;
  justify-content: flex-start;
  gap: 0.35rem 1.25rem;
  padding: 0;
}
.detail-top-flow-shell :deep(.step-chip) {
  gap: 0.35rem;
}
.detail-top-flow-shell :deep(.step-dot--sm) {
  width: 0.5rem;
  height: 0.5rem;
  border: none;
  border-radius: 50%;
  background: #d0d5dd;
  color: transparent;
}
.detail-top-flow-shell :deep(.step-done .step-dot--sm) {
  background: #16a34a;
}
.detail-top-flow-shell :deep(.step-current .step-dot--sm) {
  background: #2f80ed;
  box-shadow: 0 0 0 3px rgb(47 128 237 / 0.18);
}
.detail-top-flow-shell :deep(.step-skipped .step-dot--sm) {
  background: #d0d5dd;
}
.detail-top-flow-shell :deep(.step-check) {
  display: none !important;
}
.detail-top-flow-shell :deep(.workflow-progress--horizontal .step-label) {
  font-size: 0.8125rem;
  font-weight: 800;
  color: #98a2b3;
  line-height: 1.25;
}
.detail-top-flow-shell :deep(.step-done .step-label) {
  color: #16a34a;
}
.detail-top-flow-shell :deep(.step-current .step-label) {
  color: #2f80ed;
}
.detail-top-flow-shell :deep(.step-sublabel-inline) {
  display: none;
}
.detail-top-flow-shell :deep(.step-sep) {
  display: none;
}

/* ── v9：三栏 + 底通栏设计与资产（分栏微色，标题与正文区分） ── */
.detail-body-v6 {
  display: flex;
  flex-direction: column;
  min-width: 0;
  flex: 1 1 auto;
  min-height: 0;
}

/* ── V3 布局 + V4 圆角模块卡 ── */
.detail-v3-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 24rem;
  gap: 1rem;
  width: 100%;
  min-width: 0;
  padding: 0 0.15rem 0.75rem;
  align-items: start;
}
.detail-v3-main {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  min-width: 0;
  padding: 0.25rem 0.35rem 1rem 0.15rem;
}
.detail-v3-module,
.detail-v3-side-card {
  border: 1px solid var(--dv-border-soft, #e8ecf4);
  border-radius: var(--dv-r-outer, 1.25rem);
  background: #fff;
  box-shadow: var(--dv-surface-elev, 0 1px 3px rgba(15, 23, 42, 0.07));
}
.detail-v3-module {
  padding: 1.15rem 1.2rem 1.2rem;
}
.detail-v3-module-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  margin-bottom: 0.85rem;
}
.detail-v3-eyebrow,
.detail-v3-side-kicker {
  margin: 0 0 0.18rem;
  font-size: 0.6875rem;
  font-weight: 800;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: #98a2b3;
}
.detail-v3-module-title,
.detail-v3-side-title {
  margin: 0;
  color: #151a21;
  font-weight: 800;
  line-height: 1.2;
}
.detail-v3-module-title {
  font-size: 1.125rem;
}
.detail-v3-side-title {
  font-size: 1.05rem;
}
.detail-v3-module-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 0.4rem;
  align-items: center;
}
.detail-v3-link-chip,
.detail-v3-soft-chip {
  min-height: 2rem;
  border: none;
  padding: 0.4rem 0.7rem;
  font-size: 0.75rem;
  font-weight: 800;
  cursor: pointer;
  border-radius: 9999px;
}
.detail-v3-link-chip {
  color: #1d4ed8;
  background: #eff6ff;
}
.detail-v3-soft-chip {
  color: #344054;
  background: #f2f4f7;
}
.detail-v3-edit-base-btn {
  background: #2563eb !important;
  border-color: #2563eb !important;
  color: #fff !important;
  border-radius: 0.625rem !important;
}
.detail-v3-module-actions :deep(button) {
  border-radius: 0.625rem;
}
.detail-v3-state-pill {
  display: inline-flex;
  align-items: center;
  min-height: 1.7rem;
  padding: 0.25rem 0.6rem;
  border-radius: 9999px;
  font-size: 0.75rem;
  font-weight: 800;
  white-space: nowrap;
}
.detail-v3-state-pill--info {
  color: #175cd3;
  background: #eff8ff;
  border: 1px solid #bfdbfe;
}
.detail-v3-state-pill--purple {
  color: #6d28d9;
  background: #f4f3ff;
  border: 1px solid #ddd6fe;
}
.detail-v3-state-pill--warning {
  color: #b45309;
  background: #fffaeb;
  border: 1px solid #fedf89;
}
.detail-v3-state-pill--success {
  color: #067647;
  background: #ecfdf3;
  border: 1px solid #a7f3d0;
}
.detail-v3-info-grid {
  display: grid;
  grid-template-columns: 1fr 1.35fr 1fr 1fr;
  gap: 0.75rem;
  min-width: 0;
  align-items: stretch;
}
.detail-v3-retouch-requirements {
  grid-column: 1 / -1;
}
.detail-v3-info-card {
  min-width: 0;
  min-height: 8.5rem;
  padding: 0.9rem 1rem;
  border: 1px solid #e8ecf0;
  border-radius: var(--dv-r-card, 0.875rem);
  background: #f4f6fa;
}
.detail-v3-info-card--product {
  background: #f4f6fa;
}
.detail-v3-info-card--refs {
  background: #eef5ff;
  border-color: #dbeafe;
}
.detail-v3-file-drop-active {
  border-color: #60a5fa;
  box-shadow: 0 0 0 2px rgba(96, 165, 250, 0.14);
  outline: none;
}
.detail-v3-info-card--cost {
  background: #fffaf0;
  border-color: #ffedd4;
}
.detail-product-management-card {
  grid-column: 1 / -1;
  background: #f8fbff;
  border-color: #bfdbfe;
  color: #111827;
}
.detail-product-management-file-input {
  position: fixed;
  width: 1px;
  height: 1px;
  opacity: 0;
  pointer-events: none;
}
.detail-product-management-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
  margin-bottom: 0.75rem;
}
.detail-product-management-head strong {
  display: block;
  color: #111827;
  font-size: 0.92rem;
}
.detail-product-management-hint {
  margin-bottom: 0.75rem;
  padding: 0.55rem 0.7rem;
  border: 1px solid #fed7aa;
  border-radius: 0.75rem;
  color: #9a3412;
  background: #fff7ed;
}
.detail-product-management-list {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 0.65rem;
}
.detail-product-management-item {
  display: grid;
  grid-template-columns: 5.75rem minmax(0, 1fr) minmax(9rem, auto);
  align-items: center;
  gap: 0.65rem;
  min-width: 0;
  padding: 0.55rem;
  border: 1px solid #dbeafe;
  border-radius: 0.875rem;
  background: #ffffff;
}
.detail-product-management-preview {
  display: grid;
  place-items: center;
  width: 5.75rem;
  height: 4.5rem;
  overflow: hidden;
  border: 1px solid #dbe3ee;
  border-radius: 0.625rem;
  color: #dc2626;
  background: #f8fafc;
  font-size: 0.75rem;
  font-weight: 800;
}
.detail-product-management-preview :deep(.detail-product-management-apm),
.detail-product-management-preview :deep(.apm),
.detail-product-management-preview :deep(.apm-img),
.detail-product-management-preview :deep(.detail-product-management-img) {
  width: 100%;
  height: 100%;
  object-fit: contain;
  background: #ffffff;
}
.detail-product-management-preview :deep(.apm-placeholder),
.detail-product-management-preview :deep(.apm-empty) {
  min-height: 0;
  height: 100%;
  border: 0;
  border-radius: 0;
  padding: 0.25rem;
}
.detail-product-management-preview.is-missing {
  border-style: dashed;
}
.detail-product-management-meta {
  display: grid;
  gap: 0.18rem;
  min-width: 0;
}
.detail-product-management-meta strong {
  overflow: hidden;
  color: #111827;
  font-family: var(--yb-font-data);
  font-size: 0.82rem;
  font-weight: 900;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.detail-product-management-meta small {
  overflow: hidden;
  color: #4b5563;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.detail-product-management-success {
  color: #047857 !important;
  font-weight: 800;
}
.detail-product-management-warning {
  color: #b45309 !important;
  font-weight: 800;
}
.detail-product-management-error {
  color: #b91c1c !important;
  font-weight: 800;
}
.detail-product-management-sync {
  color: #1e40af !important;
}
.detail-product-management-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 0.35rem;
}
.detail-product-management-actions .detail-v3-link-btn:disabled {
  cursor: not-allowed;
  opacity: 0.42;
}
.detail-v3-erp-retry-row {
  margin-top: 0.75rem;
  padding-top: 0.75rem;
  border-top: 1px solid #ffedd4;
}
.detail-erp-sync-error {
  color: #b42318;
}
.detail-erp-sync-status--error {
  color: #b42318;
  font-weight: 600;
}
.detail-erp-sync-status--warning {
  color: #b54708;
  font-weight: 600;
}
.detail-erp-sync-status--success {
  color: #027a48;
  font-weight: 600;
}
.detail-v3-info-card--audit,
.detail-v3-info-card--warehouse {
  background: #f4f6fa;
}
.detail-v3-info-card--audit-comment {
  display: flex;
  flex-direction: column;
  gap: 0.65rem;
}
.detail-v3-info-card--wide {
  grid-column: 1 / -1;
  padding: 0;
  overflow: hidden;
}
.detail-v3-info-card--wide :deep(.detail-block) {
  border: 0;
  box-shadow: none;
}
.detail-v3-card-kicker {
  margin: 0 0 0.7rem;
  color: #344054;
  font-size: 0.875rem;
  font-weight: 900;
}
.detail-v3-kv-list {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  margin: 0;
}
.detail-v3-kv-list--compact {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.55rem 0.75rem;
}
.detail-v3-kv-list div {
  display: grid;
  grid-template-columns: 4.75rem minmax(0, 1fr);
  gap: 0.5rem;
  align-items: baseline;
}
.detail-v3-kv-list--compact div {
  grid-template-columns: 4.5rem minmax(0, 1fr);
}
.detail-v3-kv-list dt {
  margin: 0;
  color: #667085;
  font-size: 0.75rem;
  font-weight: 800;
}
.detail-v3-kv-list dd {
  margin: 0;
  min-width: 0;
  color: #101828;
  font-size: 0.8125rem;
  font-weight: 650;
  text-align: left;
  word-break: break-word;
}
.detail-v3-danger {
  color: #d92d20 !important;
}
.detail-v3-requirement-box {
  margin-top: 0.75rem;
  padding: 0.65rem 0.75rem;
  border: 1px solid #e4e7ec;
  border-radius: 0.65rem;
  background: #fff;
}
.detail-v3-card-text {
  margin: 0;
  color: #344054;
  font-size: 0.8125rem;
  line-height: 1.65;
}
.detail-v3-card-muted {
  margin: 0.4rem 0 0;
  color: #98a2b3;
  font-size: 0.75rem;
}
.detail-v3-link-btn,
.detail-v3-dark-btn,
.detail-v3-danger-btn,
.detail-v3-light-btn {
  min-height: 2.25rem;
  border: none;
  padding: 0.5rem 0.85rem;
  font-size: 0.75rem;
  font-weight: 800;
  cursor: pointer;
  border-radius: var(--dv-r-control, 0.625rem);
}
.detail-v3-link-btn {
  background: #fff;
  color: #1d4ed8;
  border: 1px solid #bfdbfe;
  align-self: flex-start;
}
.detail-v3-ref-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem;
  margin-top: 0.65rem;
}
.detail-v3-upload-ref-btn {
  min-height: 2.25rem;
  border: 1px solid #2563eb;
  padding: 0.5rem 0.85rem;
  font-size: 0.75rem;
  font-weight: 800;
  cursor: pointer;
  border-radius: var(--dv-r-control, 0.625rem);
  background: #2563eb;
  color: #fff;
  box-shadow: 0 1px 2px rgba(37, 99, 235, 0.2);
}
.detail-v3-upload-ref-btn:hover {
  background: #1d4ed8;
}
.detail-v3-hidden-file-input {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}
.detail-v3-ref-status {
  margin: 0.45rem 0 0;
  font-size: 0.75rem;
  color: #2563eb;
}
.detail-v3-ref-error {
  margin: 0.45rem 0 0;
  font-size: 0.75rem;
  color: #dc2626;
}
.detail-v3-summary-fold {
  margin-top: 0.55rem;
}
.detail-v3-summary-fold > summary {
  cursor: pointer;
  font-size: 0.75rem;
  color: #475467;
  font-weight: 700;
  margin-bottom: 0.45rem;
}
.detail-v3-dark-btn {
  margin-top: 0.6rem;
  background: #2563eb;
  color: #fff;
  box-shadow: 0 1px 2px rgba(37, 99, 235, 0.2);
}
.detail-v3-danger-btn {
  margin-top: 0.6rem;
  background: #ffe4e6;
  color: #be123c;
  border: 1px solid #fecdd3;
}
.detail-v3-light-btn {
  margin-top: 0.6rem;
  background: #f2f4f7;
  color: #344054;
  border: 1px solid #e4e7ec;
}
.detail-v3-workflow-grid {
  display: grid;
  gap: 0.75rem;
  align-items: stretch;
}
.detail-v3-workflow-grid--design {
  grid-template-columns: 1fr 1.45fr 1.1fr 0.85fr;
}
.detail-v3-workflow-grid--audit,
.detail-v3-workflow-grid--warehouse {
  grid-template-columns: 1.1fr 1.25fr 1fr;
}
.detail-v3-fake-textarea {
  min-height: 5.2rem;
  padding: 0.75rem;
  background: #fff;
  color: #98a2b3;
  font-size: 0.8125rem;
  border-radius: var(--dv-r-card, 0.875rem);
  border: 1px solid #e4e7ec;
}
.detail-v3-inline-actions {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
}
.detail-v3-audit-upload {
  margin-top: 0.75rem;
  padding-top: 0.65rem;
  border-top: 1px solid #e4e7ec;
}
.detail-v3-requirement-box span {
  display: block;
  margin-bottom: 0.35rem;
  color: #667085;
  font-size: 0.75rem;
  font-weight: 800;
}
.detail-v3-requirement-box p {
  margin: 0;
  color: #344054;
  font-size: 0.8125rem;
  line-height: 1.55;
}
.detail-v3-module-note,
.detail-v3-side-desc {
  margin: 0;
  color: #667085;
  font-size: 0.8125rem;
  line-height: 1.55;
}
.detail-v3-module-note {
  display: flex;
  align-items: flex-start;
  gap: 0.5rem;
  margin-top: 0.75rem;
  padding: 0.65rem 0.8rem;
  border-radius: var(--dv-r-card, 0.875rem);
  background: #f8fafc;
  border: 1px solid #e8ecf0;
}
.detail-v3-module-note::before {
  content: 'i';
  flex: 0 0 auto;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 1.1rem;
  height: 1.1rem;
  border-radius: 50%;
  font-size: 0.6rem;
  font-weight: 900;
  color: #0369a1;
  background: #e0f2fe;
}
.detail-v3-side {
  position: sticky;
  top: 0.5rem;
  display: flex;
  flex-direction: column;
  gap: 0.9rem;
  min-width: 0;
  min-height: min(calc(100dvh - 6rem), 48rem);
  padding: 1.1rem 1rem 1.15rem;
  border: 1px solid var(--dv-border-soft, #e8ecf4);
  border-radius: var(--dv-r-outer, 1.25rem);
  background: #fff;
  box-shadow: var(--dv-surface-elev, 0 1px 3px rgba(15, 23, 42, 0.07));
}
.detail-v3-side-head {
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
}
.detail-v3-side-events {
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
}
.detail-v3-side-event {
  background: #f8fafc;
  border: 1px solid #eef2f6;
  border-radius: var(--dv-r-card, 0.875rem);
  padding: 0.8rem 0.85rem;
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}
.detail-v3-side-event-title {
  margin: 0;
  font-size: 0.8125rem;
  font-weight: 800;
  color: #151a21;
  line-height: 1.4;
}
.detail-v3-side-event-desc {
  margin: 0;
  font-size: 0.75rem;
  color: #667085;
  line-height: 1.4;
}
.detail-v3-side-empty {
  padding: 0.8rem 0.85rem;
  color: #98a2b3;
  font-size: 0.8125rem;
  background: #f8fafc;
  border: 1px solid #eef2f6;
  border-radius: var(--dv-r-card, 0.875rem);
}
.detail-v3-layout :deep(section.detail-block) {
  margin: 0;
  height: 100%;
  border-color: rgb(222 228 237) !important;
  border-radius: 0.75rem !important;
  background: #f8fafc !important;
  box-shadow: none !important;
}
.detail-v3-layout :deep(.block-title) {
  color: #151a21;
  font-weight: 800;
}
.detail-v3-layout :deep(.block-icon) {
  background: #fff;
  color: #667085;
}
.detail-v3-module--design :deep(section.detail-block),
.detail-v3-module--audit :deep(section.detail-block),
.detail-v3-module--warehouse :deep(section.detail-block) {
  background: #fff !important;
}
/* 三栏铺满工作面宽度：左固定 + 中栏伸展 + 右栏自适应，避免整组居中导致两侧大留白 */
.detail-main-three-col {
  display: flex;
  flex-direction: row;
  flex-wrap: nowrap;
  align-items: stretch;
  justify-content: flex-start;
  gap: 0;
  min-width: 0;
  flex: 1 1 auto;
  min-height: 0;
  width: 100%;
}
/* 左栏「基础信息」略加宽；中间栏 flex 收缩让出横向空间（中间栏内部样式保持原样） */
.detail-main-three-col > .detail-col--left {
  flex: 0 0 clamp(15rem, 24vw, 23rem);
  max-width: 23rem;
  min-width: 0;
}
.detail-main-three-col > .detail-col--center {
  flex: 1 1 14rem;
  min-width: 0;
  max-width: none;
}
.detail-main-three-col > .detail-col--right {
  flex: 0 1 clamp(15rem, 22vw, 26rem);
  min-width: 0;
  max-width: min(28rem, 34vw);
}
.detail-col--stack {
  display: flex;
  flex-direction: column;
  gap: 0.65rem;
  min-width: 0;
}
.detail-meta-stack {
  display: flex;
  flex-direction: column;
  gap: 0.65rem;
  min-width: 0;
}
.detail-meta-stack :deep(section.detail-block) {
  margin: 0;
}
.detail-warehouse-slot :deep(section.detail-block) {
  margin: 0;
}
.detail-design-band {
  flex: 0 0 auto;
  min-width: 0;
  padding: 0.85rem 1rem 1rem;
  border-top: 1px solid rgb(219 228 240 / 0.95);
  background: rgb(248 250 252 / 0.65);
}
.detail-design-band :deep(section.detail-block) {
  margin: 0;
}
.detail-col {
  min-width: 0;
}
.detail-col--left {
  display: flex;
  flex-direction: column;
  gap: 0.65rem;
  padding: 0.75rem 0.875rem 1rem;
  background: #eceff7;
  border-right: 1px solid rgb(199 210 254 / 0.85);
}
.detail-col--center {
  padding: 0.75rem 0.625rem 1rem;
  background: linear-gradient(180deg, rgb(240 249 255 / 0.55) 0%, rgb(248 250 252 / 0.4) 100%);
  border-left: 1px solid rgb(191 219 254 / 0.45);
  border-right: 1px solid rgb(191 219 254 / 0.35);
}
.detail-col--right {
  display: flex;
  flex-direction: column;
  gap: 0.65rem;
  padding: 0.65rem 0.75rem 0.85rem;
  /* 与中栏统一为冷灰蓝系，避免「仓库与结单」单独偏黄 */
  background: linear-gradient(180deg, rgb(240 249 255 / 0.5) 0%, rgb(248 250 252 / 0.45) 100%);
  border-left: 1px solid rgb(191 219 254 / 0.4);
  min-width: 0;
}
/* 与设计 / 审核工作台一致：主标题 = 交付必读同色 indigo，字段名 = 同色系更淡 */
.detail-v6-surface :deep(.block-title) {
  color: var(--dw-title);
}
.detail-v6-surface :deep(dt),
.detail-v6-surface :deep(.req-label),
.detail-v6-surface :deep(.main-ref-label),
.detail-v6-surface :deep(.main-ref-hint),
.detail-v6-surface :deep(.ref-pane-hint),
.detail-v6-surface :deep(.section-label),
.detail-v6-surface :deep(.ownership-block-title),
.detail-v6-surface :deep(.field-label),
.detail-v6-surface :deep(h5),
.detail-v6-surface :deep(.status-pill-label),
.detail-v6-surface :deep(.row-label),
.detail-v6-surface :deep(.product-thumb-label) {
  color: var(--dw-label);
}
.detail-v6-surface :deep(.legacy-row dt) {
  color: var(--dw-label);
}
.detail-v6-surface :deep(.cost-tile-label) {
  color: var(--dw-label);
}
.detail-v6-surface :deep(.cost-block--tiles .block-title) {
  color: var(--dw-title);
}
.detail-v6-surface :deep(.cost-title-suffix) {
  color: var(--dw-label);
}
.detail-v6-surface :deep(.product-tab-active) {
  background: var(--dw-title);
  border-color: var(--dw-title);
  color: #fff;
}
.detail-col--right :deep(.detail-block) {
  border-color: rgb(203 213 225 / 0.55) !important;
  background: rgb(255 255 255 / 0.5) !important;
}
.detail-v6-surface :deep(section.detail-block) {
  box-shadow: none !important;
  border: 1px solid rgb(203 213 225 / 0.55) !important;
  background: rgb(255 255 255 / 0.5) !important;
  border-radius: 0.75rem !important;
}
.detail-col--left :deep(section.detail-block) {
  border-color: rgb(199 210 254 / 0.55) !important;
  background: rgb(255 255 255 / 0.38) !important;
}
.detail-merge-card--center {
  background: rgb(255 255 255 / 0.45);
  border: 1px solid rgb(203 213 225 / 0.5);
  border-radius: 0.75rem;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}
.detail-merge-card--center :deep(section.detail-block) {
  border: none !important;
  box-shadow: none !important;
  border-radius: 0 !important;
  margin: 0;
  background: transparent !important;
}
.detail-meta-slot :deep(section.detail-block) {
  margin: 0;
}

@media (max-width: 1199px) {
  .detail-v3-layout {
    grid-template-columns: 1fr;
  }
  .detail-v3-side {
    position: static;
  }
  .detail-v3-info-grid {
    grid-template-columns: 1fr;
  }
  .detail-v3-module-head {
    flex-direction: column;
  }
  .detail-v3-module-actions {
    justify-content: flex-start;
  }
  .detail-top-grid {
    grid-template-columns: 1fr;
    justify-items: stretch;
  }
  .detail-top-mid {
    align-items: stretch;
  }
  .detail-top-flow-shell {
    max-width: none;
  }
  .detail-top-right {
    align-items: flex-start;
    min-width: 0;
  }
  .detail-top-badges,
  .detail-top-actions {
    justify-content: flex-start;
  }
  .detail-main-three-col {
    flex-direction: column;
    justify-content: flex-start;
  }
  .detail-main-three-col > .detail-col--left,
  .detail-main-three-col > .detail-col--center,
  .detail-main-three-col > .detail-col--right {
    flex: 1 1 auto;
    max-width: none;
    width: 100%;
  }
  .detail-col--left {
    border-right: none;
    border-bottom: 1px solid rgb(219 228 240 / 0.9);
  }
  .detail-col--center {
    border-left: none;
    border-right: none;
    border-bottom: 1px solid rgb(191 219 254 / 0.5);
  }
  .detail-col--right {
    border-left: none;
  }
}

@media (max-width: 760px) {
  .detail-top-actions {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    width: 100%;
  }
  .detail-top-chip {
    min-width: 0;
    width: 100%;
  }
  .detail-top-chip :deep(span) {
    min-width: 0;
  }
  .detail-top-chip :deep(span),
  .detail-top-chip {
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .ai-summary-action-grid,
  .ai-summary-grid,
  .ai-summary-sku-row {
    grid-template-columns: 1fr;
  }
  .ai-summary-next-actions li {
    grid-template-columns: auto minmax(0, 1fr);
  }
  .ai-summary-next-actions span {
    grid-column: 2;
  }
  .ai-summary-next-actions strong,
  .ai-summary-next-actions p {
    grid-column: 2;
  }
  .ai-summary-timeline li {
    grid-template-columns: 1fr;
    gap: 0.25rem;
  }
  .ai-summary-footer {
    align-items: stretch;
    flex-direction: column;
    padding: 0.85rem 1rem;
  }
  .ai-summary-footer-actions {
    justify-content: stretch;
  }
  .ai-summary-footer-actions :deep(button) {
    flex: 1 1 7rem;
  }
}
.action-error {
  width: 100%;
  margin: 0 0 0.5rem;
  padding: 0.5rem 0.75rem;
  font-size: 0.875rem;
  color: #b91c1c;
  background: #fef2f2;
  border-radius: 6px;
}
.action-success {
  width: 100%;
  margin: 0 0 0.5rem;
  padding: 0.5rem 0.75rem;
  font-size: 0.875rem;
  color: #047857;
  background: #ecfdf5;
  border: 1px solid #a7f3d0;
  border-radius: 6px;
}
.back-btn {
  font-size: 0.8125rem;
  color: rgb(100 116 139);
  background: none;
  border: none;
  cursor: pointer;
  padding: 0;
  transition: color 0.15s;
}
.back-btn:hover {
  color: rgb(15 23 42);
}

/* Step 87：创建成功提示横幅 */
.create-success-banner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.5rem 0.75rem;
  border-radius: 0.5rem;
  font-size: 0.8125rem;
  margin-bottom: 0.75rem;
}
.create-success-banner.banner-info {
  background: rgb(239 246 255);
  border: 1px solid rgb(191 219 254);
  color: rgb(30 64 175);
}
.create-success-banner.banner-warning {
  background: rgb(255 251 235);
  border: 1px solid rgb(253 230 138);
  color: rgb(146 64 14);
}
.create-success-banner.banner-error {
  background: rgb(254 242 242);
  border: 1px solid rgb(254 202 202);
  color: rgb(185 28 28);
}
.banner-dismiss {
  background: none;
  border: none;
  font-size: 1.25rem;
  line-height: 1;
  cursor: pointer;
  opacity: 0.7;
  padding: 0 0.25rem;
}
.banner-dismiss:hover {
  opacity: 1;
}
.batch-sku-switcher {
  display: flex;
  align-items: center;
  gap: 0.65rem;
  margin-bottom: 0.75rem;
  padding: 0.55rem 0.75rem;
  border-radius: var(--dv-r-card, 0.875rem);
  background: #f8fafc;
  border: 1px solid #eaecf0;
}
.batch-sku-switcher-label {
  flex-shrink: 0;
  font-size: 0.6875rem;
  font-weight: 700;
  color: #667085;
  letter-spacing: 0.03em;
}
.batch-sku-tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
}
.batch-sku-tab {
  padding: 0.25rem 0.6rem;
  border-radius: 9999px;
  border: 1px solid #e4e7ec;
  background: #fff;
  font-size: 0.75rem;
  font-weight: 600;
  color: #475467;
  cursor: pointer;
  transition: background 0.12s ease, border-color 0.12s ease, color 0.12s ease;
  font-family: var(--yb-font-data);
  letter-spacing: 0.01em;
}
.batch-sku-tab:hover {
  border-color: #98a2b3;
  background: #f1f5f9;
}
.batch-sku-tab--active {
  background: #2563eb;
  color: #fff;
  border-color: #2563eb;
}
.batch-sku-tab--active:hover {
  background: #1d4ed8;
  border-color: #1d4ed8;
}

/* Phase 3: light admin task detail — final overrides (style-only). */
.task-detail-view {
  color: #374151;
  background: transparent !important;
  overflow-x: hidden;
}

.detail-v6-surface {
  --dv-border-soft: #e5e7eb;
  --dw-title: #111827;
  --dw-label: #6b7280;
  --dv-surface-elev: 0 1px 3px rgba(15, 23, 42, 0.06);
}

.detail-top-unified.detail-top-v6,
.detail-v3-module,
.detail-v3-side,
.detail-v3-info-card,
.detail-merge-card--center,
.batch-sku-switcher,
.create-success-banner {
  border-color: #e5e7eb !important;
  background: #ffffff !important;
  color: #374151 !important;
  box-shadow: 0 1px 3px rgba(15, 23, 42, 0.06) !important;
  backdrop-filter: none !important;
  -webkit-backdrop-filter: none !important;
}

.detail-top-unified.detail-top-v6::before {
  display: none !important;
}

.detail-top-taskno,
.detail-v3-module-title,
.detail-v3-side-title,
.detail-v3-card-kicker,
.detail-v3-side-event-title,
.detail-v6-surface :deep(.block-title) {
  color: #111827 !important;
}

.detail-top-sub,
.detail-v3-card-text,
.detail-v3-card-muted,
.detail-v3-side-event-desc,
.detail-v3-module-note,
.detail-v3-side-desc,
.detail-v3-kv-list dt,
.detail-v3-kv-list dd,
.detail-v6-surface :deep(dt),
.detail-v6-surface :deep(.field-label),
.detail-v6-surface :deep(.section-label) {
  color: #6b7280 !important;
}

.detail-v3-kv-list dd,
.detail-v6-surface :deep(dd),
.detail-v6-surface :deep(.field-value),
.detail-v6-surface :deep(.value) {
  color: #111827 !important;
}

.detail-top-flow-shell,
.detail-v3-requirement-box,
.detail-v3-fake-textarea,
.detail-v3-side-event,
.detail-v3-side-empty,
.detail-v6-surface :deep(section.detail-block),
.detail-col--right :deep(.detail-block),
.detail-col--left :deep(section.detail-block),
.detail-v3-layout :deep(section.detail-block) {
  border-color: #e5e7eb !important;
  background: #ffffff !important;
  color: #374151 !important;
}

.detail-top-status-dot {
  background: #2563eb;
  box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.15);
}

.detail-v3-link-chip,
.detail-v3-link-btn,
.detail-v3-ref-status,
.detail-top-flow-shell :deep(.step-current .step-label) {
  color: #2563eb !important;
}

.action-success,
.create-success-banner.banner-info {
  border: 1px solid #bbf7d0 !important;
  background: #ecfdf5 !important;
  color: #15803d !important;
}

.action-error,
.create-success-banner.banner-error {
  border: 1px solid #fecaca !important;
  background: #fef2f2 !important;
  color: #b91c1c !important;
}

.create-success-banner.banner-warning {
  border: 1px solid #fde68a !important;
  background: #fffbeb !important;
  color: #b45309 !important;
}

.detail-col--left,
.detail-col--center,
.detail-col--right,
.detail-design-band {
  background: #f9fafb !important;
  border-color: #e5e7eb !important;
}

.detail-v3-info-card {
  background: #f9fafb !important;
  border-color: #e5e7eb !important;
}

.detail-v3-info-card--refs {
  background: #eff6ff !important;
  border-color: #dbeafe !important;
}

.detail-v3-info-card--cost {
  background: #fffbeb !important;
  border-color: #fde68a !important;
}

.detail-v3-module {
  position: relative;
  overflow: hidden;
}

.detail-v3-module--design {
  border-color: #bfdbfe !important;
  background: #ffffff !important;
  box-shadow: 0 1px 3px rgba(15, 23, 42, 0.06) !important;
}

.detail-v3-module--design::before {
  display: none;
}

.detail-v3-module--design .detail-v3-module-head,
.detail-v3-module--design .batch-sku-switcher,
.detail-v3-module--design .detail-v3-workflow-grid {
  position: relative;
}

.detail-v3-module--design .detail-v3-dark-btn,
.detail-v3-module--design .detail-v3-upload-ref-btn {
  background: #2563eb !important;
  border-color: #2563eb !important;
  color: #ffffff !important;
  box-shadow: 0 1px 2px rgba(37, 99, 235, 0.2) !important;
}

.detail-v3-module--audit,
.detail-v3-module--warehouse {
  border-color: #e5e7eb !important;
  background: #ffffff !important;
  opacity: 1;
}

.detail-v3-module--audit .detail-v3-info-card,
.detail-v3-module--warehouse .detail-v3-info-card,
.detail-v3-module--audit :deep(section.detail-block),
.detail-v3-module--warehouse :deep(section.detail-block) {
  background: #ffffff !important;
  border-color: #e5e7eb !important;
}

.detail-v3-side {
  background: #ffffff !important;
}

.detail-v3-side-events {
  position: relative;
  gap: 0.5rem;
  padding-left: 0.9rem;
}

.detail-v3-side-events::before {
  content: '';
  position: absolute;
  left: 0.25rem;
  top: 0.45rem;
  bottom: 0.45rem;
  width: 1px;
  background: #e5e7eb;
}

.detail-v3-side-event {
  position: relative;
  gap: 0.28rem;
  padding: 0.72rem 0.78rem;
  border-color: #e5e7eb !important;
  background: #f9fafb !important;
  opacity: 1;
}

.detail-v3-side-event::before {
  content: '';
  position: absolute;
  left: -0.92rem;
  top: 1rem;
  width: 0.48rem;
  height: 0.48rem;
  border-radius: 999px;
  background: #9ca3af;
  box-shadow: none;
}

.detail-v3-side-event:first-child {
  border-color: #bfdbfe !important;
  background: #eff6ff !important;
}

.detail-v3-side-event:first-child::before {
  background: #2563eb;
  box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.12);
}

.detail-v3-side-event:hover {
  border-color: #d1d5db !important;
  background: #ffffff !important;
}

.detail-v3-side-event-title {
  color: #111827 !important;
  font-weight: 750;
}

.detail-v3-side-event:not(:first-child) .detail-v3-side-event-title {
  color: #374151 !important;
}

.detail-top-flow-shell {
  border-color: #e5e7eb !important;
  background: #f9fafb !important;
  box-shadow: none !important;
}

.detail-top-flow-shell :deep(.workflow-progress--horizontal) {
  align-items: center;
  gap: 0.45rem;
}

.detail-top-flow-shell :deep(.step-chip) {
  min-height: 2.15rem;
}

.detail-top-flow-shell :deep(.step-dot--sm) {
  width: 1rem;
  height: 1rem;
  border: 1px solid #d1d5db;
  background: #e5e7eb;
}

.detail-top-flow-shell :deep(.step-done .step-dot--sm) {
  background: #22c55e;
  border-color: #86efac;
  box-shadow: none;
}

.detail-top-flow-shell :deep(.step-current .step-dot--sm) {
  background: #2563eb;
  border-color: #93c5fd;
  box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.12);
}

.detail-top-flow-shell :deep(.workflow-progress--horizontal .step-label) {
  color: #6b7280;
  font-size: 0.75rem;
}

.detail-top-flow-shell :deep(.step-done .step-label) {
  color: #15803d !important;
}

.detail-top-flow-shell :deep(.step-current .step-label) {
  color: #2563eb !important;
}

.detail-top-flow-shell :deep(.step-sublabel-inline) {
  display: inline;
  color: #9ca3af;
}

.detail-top-flow-shell :deep(.step-sep) {
  display: block;
}

@media (prefers-reduced-motion: reduce) {
  .ai-summary-loading-dot,
  .detail-v3-side-event,
  .detail-top-flow-shell :deep(.step-chip),
  .detail-top-flow-shell :deep(.step-dot--sm),
  .detail-top-flow-shell :deep(.step-sep::after) {
    animation: none !important;
    transition: none !important;
  }
}

.batch-sku-tab--active,
.detail-v6-surface :deep(.product-tab-active) {
  background: #2563eb !important;
  border-color: #2563eb !important;
  color: #fff !important;
}

/* Final workflow rail pass: no legacy green, no scrollbar, compact equal-width stage rail. */
.detail-top-flow-shell {
  overflow: hidden !important;
}

.detail-top-flow-shell :deep(.workflow-progress--horizontal) {
  display: flex !important;
  flex-wrap: nowrap !important;
  justify-content: flex-start !important;
  align-items: center !important;
  gap: clamp(0.25rem, 0.7vw, 0.55rem) !important;
  overflow-x: hidden !important;
  overflow-y: hidden !important;
  padding: 0.35rem 0.2rem !important;
}

.detail-top-flow-shell :deep(.step-chip) {
  flex: 1 1 0 !important;
  min-width: 0 !important;
  min-height: 2rem !important;
  justify-content: center !important;
  border-radius: 999px !important;
  padding: 0.15rem 0.32rem !important;
  background: transparent !important;
  opacity: 1 !important;
}

.detail-top-flow-shell :deep(.step-chip.step-current) {
  background: #eff6ff !important;
}

.detail-top-flow-shell :deep(.step-dot--sm) {
  width: 0.72rem !important;
  height: 0.72rem !important;
  background: #e5e7eb !important;
  border-color: #d1d5db !important;
  box-shadow: none !important;
}

.detail-top-flow-shell :deep(.step-done .step-dot--sm) {
  background: #22c55e !important;
  border-color: #86efac !important;
  box-shadow: none !important;
}

.detail-top-flow-shell :deep(.step-current .step-dot--sm) {
  background: #2563eb !important;
  border-color: #93c5fd !important;
  box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.12) !important;
}

.detail-top-flow-shell :deep(.step-skipped .step-dot--sm),
.detail-top-flow-shell :deep(.step-pending .step-dot--sm) {
  background: #e5e7eb !important;
  border-color: #d1d5db !important;
  box-shadow: none !important;
}

.detail-top-flow-shell :deep(.workflow-progress--horizontal .step-label) {
  color: #6b7280 !important;
  max-width: 100% !important;
  overflow: hidden !important;
  text-overflow: ellipsis !important;
  white-space: nowrap !important;
}

.detail-top-flow-shell :deep(.step-done .step-label) {
  color: #15803d !important;
}

.detail-top-flow-shell :deep(.step-current .step-label) {
  color: #2563eb !important;
}

.detail-top-flow-shell :deep(.step-pending .step-label),
.detail-top-flow-shell :deep(.step-skipped .step-label) {
  color: #9ca3af !important;
}

.detail-top-flow-shell :deep(.step-sublabel-inline) {
  display: none !important;
}

.detail-top-flow-shell :deep(.step-current .step-sublabel-inline) {
  display: inline !important;
  max-width: min(7.5rem, 45%) !important;
  overflow: hidden !important;
  text-overflow: ellipsis !important;
  white-space: nowrap !important;
  color: #9ca3af !important;
}

.detail-top-flow-shell :deep(.step-sep) {
  flex: 0 1 clamp(0.8rem, 2vw, 2.4rem) !important;
  width: auto !important;
  min-width: 0.55rem !important;
  height: 0.125rem !important;
  background: #d1d5db !important;
}

.detail-top-flow-shell :deep(.step-chip.step-done + .step-sep) {
  background: #93c5fd !important;
}

.detail-top-flow-shell :deep(.step-chip.step-current + .step-sep) {
  background: #bfdbfe !important;
}

/* Task detail alignment repair: prevent the top card from inheriting oversized three-column minimums. */
.task-detail-view {
  background: transparent !important;
  overflow-x: hidden;
}

.detail-shell {
  max-width: 100%;
  padding: 0.9rem clamp(0.9rem, 1.4vw, 1.35rem) 1.35rem !important;
  overflow-x: hidden;
}

.detail-v6-surface {
  max-width: 100%;
  overflow: visible;
}

.detail-top-unified.detail-top-v6 {
  max-width: 100%;
  margin: 0 !important;
  padding: clamp(0.95rem, 1.25vw, 1.2rem) !important;
  border-radius: 1rem !important;
  background: #ffffff !important;
  border-color: #e5e7eb !important;
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.06) !important;
}

.detail-top-unified.detail-top-v6::before {
  display: none !important;
}

.detail-top-grid {
  grid-template-columns: minmax(0, 1fr) auto !important;
  gap: clamp(0.75rem, 1vw, 1rem) !important;
  width: 100%;
  min-width: 0;
  align-items: start !important;
}

.detail-top-left,
.detail-top-mid,
.detail-top-right,
.detail-top-identity {
  min-width: 0;
}

.detail-top-taskno {
  max-width: 100%;
  overflow-wrap: anywhere;
  word-break: break-word;
}

.detail-top-sub,
.detail-top-current.detail-top-status-pill {
  max-width: 100%;
}

.detail-top-flow-shell {
  max-width: 100% !important;
  min-width: 0;
}

.detail-top-mid {
  grid-column: 1 / -1;
  grid-row: 2;
  justify-self: stretch;
  width: 100%;
}

.detail-top-right {
  justify-self: end;
}

.detail-top-flow-shell :deep(.workflow-progress--horizontal) {
  justify-content: center !important;
  width: fit-content !important;
  max-width: 100% !important;
  margin-inline: auto !important;
  overflow: visible !important;
}

.detail-top-flow-shell :deep(.step-chip) {
  flex: 0 0 auto !important;
  justify-content: flex-start !important;
  min-width: auto !important;
  max-width: none !important;
  padding: 0.2rem 0.3rem !important;
  background: transparent !important;
}

.detail-top-flow-shell :deep(.workflow-progress--horizontal .step-label),
.detail-top-flow-shell :deep(.step-sublabel-inline) {
  max-width: none !important;
  overflow: visible !important;
  text-overflow: clip !important;
  white-space: nowrap !important;
}

.detail-top-flow-shell :deep(.step-sublabel-inline) {
  display: inline !important;
}

.detail-top-flow-shell :deep(.step-sep) {
  flex: 1 1 clamp(1.5rem, 5vw, 5rem) !important;
  max-width: 5.5rem !important;
}

.detail-v3-info-card--refs .detail-v3-link-btn {
  min-height: 2rem !important;
  border: 1px solid #d1d5db !important;
  border-radius: 0.625rem !important;
  background: #f9fafb !important;
  color: #2563eb !important;
  box-shadow: none !important;
}

.detail-v3-info-card--refs .detail-v3-link-btn:hover {
  border-color: #93c5fd !important;
  background: #eff6ff !important;
  color: #1d4ed8 !important;
}

.detail-v3-info-card--refs .detail-v3-card-text {
  color: #6b7280 !important;
}

.detail-v3-module-note {
  background: #f9fafb !important;
  border: 1px solid #e5e7eb !important;
  color: #6b7280 !important;
  box-shadow: none !important;
}

.detail-v3-module-note::before {
  color: #111827 !important;
  background: #eff6ff !important;
  border: 1px solid #bfdbfe !important;
}

/* Naive Steps redraw: remove the old black wrapper and keep the rail aligned with the global glass skin. */
.detail-top-flow-shell {
  width: auto !important;
  max-width: 100% !important;
  padding: 0.16rem 0 !important;
  overflow: visible !important;
  background: transparent !important;
  border: none !important;
  box-shadow: none !important;
}

.detail-top-flow-shell :deep(.workflow-progress--naive.workflow-progress--horizontal) {
  width: auto !important;
  max-width: 100% !important;
  margin-inline: auto !important;
  display: flex !important;
  flex-wrap: nowrap !important;
  align-items: center !important;
  justify-content: center !important;
  overflow: visible !important;
}

.detail-top-flow-shell :deep(.workflow-progress--naive .n-step) {
  flex: 0 0 auto !important;
  width: auto !important;
  min-width: max-content !important;
}

.detail-top-flow-shell :deep(.workflow-progress--naive .n-step-content) {
  min-width: 0 !important;
  overflow: visible !important;
}

.detail-top-flow-shell :deep(.workflow-progress--naive .n-step-content-header__title),
.detail-top-flow-shell :deep(.workflow-progress--naive .n-step-content__description) {
  max-width: none !important;
  overflow: visible !important;
  text-overflow: clip !important;
  white-space: nowrap !important;
}

.detail-top-flow-shell :deep(.workflow-progress--naive .n-step-splitor) {
  flex: 0 0 clamp(1.25rem, 2.4vw, 2.6rem) !important;
  width: clamp(1.25rem, 2.4vw, 2.6rem) !important;
  min-width: clamp(1.25rem, 2.4vw, 2.6rem) !important;
  background: #d1d5db !important;
}

.detail-prediction-panel {
  display: grid;
  gap: 0.75rem;
  margin: 0.85rem 0 0;
  padding: 0.85rem;
  border: 1px solid #bfdbfe;
  border-radius: 0.75rem;
  background:
    linear-gradient(120deg, rgba(37, 99, 235, 0.08), rgba(14, 165, 233, 0.08), rgba(37, 99, 235, 0.08)),
    #eff6ff;
  background-size: 220% 100%;
  animation: detail-stream-panel 8s linear infinite;
}

.detail-prediction-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.detail-prediction-head p {
  margin: 0;
  color: #1d4ed8;
  font-size: 0.72rem;
  font-weight: 700;
}

.detail-prediction-head h2 {
  margin: 0.12rem 0 0;
  color: #111827;
  font-size: 0.95rem;
  font-weight: 800;
}

.detail-prediction-head button {
  min-height: 2rem;
  padding: 0 0.75rem;
  border: 1px solid #bfdbfe;
  border-radius: 0.5rem;
  background: #ffffff;
  color: #1d4ed8;
  font-size: 0.75rem;
  font-weight: 700;
}

.detail-prediction-list {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(13rem, 1fr));
  gap: 0.625rem;
}

.detail-prediction-item {
  position: relative;
  display: grid;
  gap: 0.25rem;
  min-height: 6rem;
  padding: 0.7rem;
  overflow: hidden;
  border: 1px solid #dbeafe;
  border-radius: 0.625rem;
  background: #ffffff;
  text-align: left;
  transition: transform 180ms ease, border-color 180ms ease, box-shadow 180ms ease;
  animation: detail-card-enter 420ms ease both;
}

.detail-prediction-item:hover {
  transform: translateY(-2px);
  border-color: #93c5fd;
  box-shadow: 0 14px 28px -22px rgba(37, 99, 235, 0.75);
}

.detail-prediction-item::after {
  content: "";
  position: absolute;
  inset: 0;
  pointer-events: none;
  background: linear-gradient(110deg, transparent 0%, rgba(59, 130, 246, 0.13) 42%, transparent 72%);
  transform: translateX(-120%);
  transition: transform 650ms ease;
}

.detail-prediction-item:hover::after {
  transform: translateX(120%);
}

.detail-prediction-item span {
  color: #2563eb;
  font-size: 0.6875rem;
  font-weight: 700;
}

.detail-prediction-item strong {
  color: #111827;
  font-size: 0.8125rem;
  line-height: 1.35;
}

.detail-prediction-item small {
  color: #475569;
  font-size: 0.75rem;
  line-height: 1.35;
}

.detail-prediction-item em {
  width: max-content;
  padding: 0.12rem 0.45rem;
  border-radius: 999px;
  background: #dbeafe;
  color: #1d4ed8;
  font-size: 0.6875rem;
  font-style: normal;
}

@keyframes detail-stream-panel {
  from { background-position: 0% 50%; }
  to { background-position: 220% 50%; }
}

@keyframes detail-card-enter {
  from {
    opacity: 0;
    transform: translateY(6px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@media (prefers-reduced-motion: reduce) {
  .detail-prediction-panel,
  .detail-prediction-item {
    animation: none !important;
  }

  .detail-prediction-item,
  .detail-prediction-item::after {
    transition: none !important;
  }
}

@media (max-width: 1280px) {
  .detail-top-grid {
    grid-template-columns: minmax(0, 1fr) auto !important;
    align-items: start !important;
  }

  .detail-top-mid {
    grid-column: 1 / -1;
    grid-row: 2;
    justify-self: stretch;
    width: 100%;
  }

  .detail-top-right {
    justify-self: end;
  }
}

@media (max-width: 1100px) {
  .detail-top-grid {
    grid-template-columns: 1fr !important;
    gap: 0.85rem !important;
  }

  .detail-top-left,
  .detail-top-mid,
  .detail-top-right,
  .detail-top-identity {
    width: 100%;
    justify-self: stretch !important;
  }

  .detail-top-right {
    justify-content: flex-start;
  }

  .detail-top-actions {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(7.5rem, 1fr));
    width: 100%;
    justify-content: stretch !important;
  }

  .detail-top-chip {
    width: 100%;
  }
}

@media (max-width: 760px) {
  .detail-top-unified.detail-top-v6 {
    padding: 0.95rem !important;
  }

  .detail-top-taskno {
    font-size: 1.35rem;
    line-height: 1.2;
  }

  .detail-top-sub {
    overflow-wrap: anywhere;
  }

  .detail-top-current.detail-top-status-pill {
    width: 100%;
    align-items: flex-start;
    border-radius: 0.85rem;
    white-space: normal;
  }

  .detail-top-flow-shell {
    overflow-x: hidden !important;
    padding-bottom: 0 !important;
  }

  .detail-top-flow-shell :deep(.workflow-progress--horizontal),
  .detail-top-flow-shell :deep(.workflow-progress--naive.workflow-progress--horizontal) {
    width: 100% !important;
    min-width: 0 !important;
    justify-content: center !important;
    gap: 0.02rem !important;
    padding: 0.36rem 0.28rem !important;
    overflow: hidden !important;
  }

  .detail-top-flow-shell :deep(.workflow-progress--naive .n-step) {
    flex: 0 0 auto !important;
    min-width: 0 !important;
  }

  .detail-top-flow-shell :deep(.workflow-progress--naive .n-step--process-status) {
    padding: 0.18rem 0.45rem 0.18rem 0.25rem !important;
  }

  .detail-top-flow-shell :deep(.workflow-progress--naive .n-step-content) {
    flex: 0 0 0 !important;
    width: 0 !important;
    min-width: 0 !important;
    overflow: hidden !important;
    padding-left: 0 !important;
  }

  .detail-top-flow-shell :deep(.workflow-progress--naive .n-step--process-status .n-step-content) {
    flex: 0 0 auto !important;
    width: auto !important;
    padding-left: 0.18rem !important;
  }

  .detail-top-flow-shell :deep(.workflow-progress--naive .n-step-content-header__title) {
    display: none !important;
  }

  .detail-top-flow-shell :deep(.workflow-progress--naive .n-step--process-status .n-step-content-header__title) {
    display: block !important;
    flex: 0 0 auto !important;
    width: auto !important;
    max-width: 2.4rem !important;
    overflow: hidden !important;
    font-size: 0.7rem !important;
    text-overflow: ellipsis !important;
  }

  .detail-top-flow-shell :deep(.workflow-progress--naive .n-step-content__description) {
    display: none !important;
  }

  .detail-top-flow-shell :deep(.workflow-progress--naive .n-step-splitor) {
    flex: 0 0 0.38rem !important;
    width: 0.38rem !important;
    min-width: 0.38rem !important;
    margin-inline: 0.04rem !important;
  }

  .detail-top-actions {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0.45rem;
  }

  .detail-top-chip {
    min-height: 2.15rem;
    height: auto;
    padding: 0.45rem 0.55rem;
  }
}
</style>
