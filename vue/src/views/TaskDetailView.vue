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
                  返回
                </BaseButton>
                <BaseButton
                  type="button"
                  class="detail-top-chip"
                  variant="ghost"
                  size="sm"
                  @click="refreshDetail"
                >刷新</BaseButton>
                <BaseButton
                  v-if="task && !isTempId"
                  type="button"
                  class="detail-top-chip"
                  variant="ghost"
                  size="sm"
                  @click="eventLogOpen = true"
                >
                  事件日志
                </BaseButton>
                <BaseButton
                  v-if="canAccessPage('task_assets')"
                  type="button"
                  class="detail-top-chip"
                  variant="ghost"
                  size="sm"
                  @click="openTaskAssetsPage"
                >
                  任务资产页
                </BaseButton>
                <button
                  v-if="canCancelTask"
                  type="button"
                  class="detail-top-chip detail-top-chip--danger"
                  @click="openCancel = true"
                >
                  终止任务
                </button>
                <button
                  v-if="can('task.close') && canShowCloseTaskButton"
                  type="button"
                  class="detail-top-chip detail-top-chip--primary"
                  @click="doClose"
                >
                  结单
                </button>
              </div>
            </div>
          </div>
        </header>

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
                      <p class="detail-v3-card-kicker">需求说明</p>
                      <p class="detail-v3-card-text">{{ detailRequirementLabel }}</p>
                    </article>

                    <article class="detail-v3-info-card">
                      <p class="detail-v3-card-kicker">运营备注</p>
                      <p class="detail-v3-card-text">{{ detailNoteLabel }}</p>
                    </article>

                    <article class="detail-v3-info-card detail-v3-info-card--refs">
                      <p class="detail-v3-card-kicker">
                        {{ isBatchTask ? '全部参考图汇总（母任务）' : '参考图 / 附件' }}
                      </p>
                      <p class="detail-v3-card-text">{{ detailReferenceLabel }}</p>
                      <AssetThumbStrip
                        v-if="opsReferenceThumbItems.length > 0"
                        :items="opsReferenceThumbItems"
                        empty-text="暂无参考图"
                        size="sm"
                      />
                      <details
                        v-if="isBatchTask && topLevelReferenceThumbItems.length > 0"
                        class="detail-v3-summary-fold"
                      >
                        <summary>展开母任务汇总参考图</summary>
                        <AssetThumbStrip
                          :items="topLevelReferenceThumbItems"
                          empty-text="暂无母任务汇总图"
                          size="sm"
                        />
                      </details>
                      <div
                        v-if="
                          canUploadReferenceFromOps ||
                          opsReferenceThumbItems.length > 0 ||
                          topLevelReferenceThumbItems.length > 0
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
                          @click="opsReferenceUploadInputRef?.click()"
                        >
                          上传参考图
                        </button>
                        <button
                          v-if="opsReferenceThumbItems.length > 0 || topLevelReferenceThumbItems.length > 0"
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
                      </dl>
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
                  :can-upload-design="canDirectSkuDesignUpload"
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
                        title="任务尚无负责人时，在待指派阶段指定设计师（首次指派）"
                        @click="doAssign"
                      >
                        指派设计师
                      </BaseButton>
                      <BaseButton
                        v-if="showReassignDesignerButton"
                        variant="secondary"
                        size="sm"
                        title="设计阶段任务调度：在进入审核责任链前更换设计负责人"
                        @click="doReassign"
                      >
                        重新指派设计师
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
                          {{ isRetouchTask ? '精修负责人' : '设计负责人' }}
                        </p>
                        <p class="detail-v3-card-text">{{ detailDesignerLabel }}</p>
                        <p class="detail-v3-card-muted">组：{{ detailOwnerLabel }}</p>
                      </article>
                      <article class="detail-v3-info-card">
                        <p class="detail-v3-card-kicker">
                          {{ isRetouchTask ? '精修版本' : '设计资产版本' }}
                        </p>
                        <p class="detail-v3-card-text">
                          {{ isRetouchTask ? retouchVersionSummary : designVersionSummary }}
                        </p>
                        <p class="detail-v3-card-muted">
                          {{ isRetouchTask ? '切换上方时间线可查看各版本精修稿件' : '切换上方时间线可查看各版本设计稿件' }}
                        </p>
                      </article>
                      <article class="detail-v3-info-card">
                        <p class="detail-v3-card-kicker">结果状态</p>
                        <p class="detail-v3-card-text">{{ designStatusText }}</p>
                      </article>
                      <article class="detail-v3-info-card detail-v3-info-card--refs">
                        <p class="detail-v3-card-kicker">资产操作</p>
                        <p class="detail-v3-card-text">
                          在上方{{ isRetouchTask ? '精修稿件' : '设计稿件' }}区预览或下载当前版本文件。
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
                        <p class="detail-v3-card-kicker">设计负责人</p>
                        <p class="detail-v3-card-text">{{ detailDesignerLabel }}</p>
                        <p class="detail-v3-card-muted">组：{{ detailOwnerLabel }}</p>
                      </article>
                      <article class="detail-v3-info-card">
                        <p class="detail-v3-card-kicker">上传设计稿</p>
                        <p class="detail-v3-card-text">请在上方"设计与资产"区域上传文件</p>
                      </article>
                      <article class="detail-v3-info-card">
                        <p class="detail-v3-card-kicker">设计资产版本</p>
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
                              请在上方"设计与资产"区域上传精修稿后点击"提交精修"按钮完成任务。
                            </p>
                          </template>
                        </template>
                        <template v-else>
                          <p class="detail-v3-card-kicker">提交审核</p>
                          <p class="detail-v3-card-text">
                            请在上方“设计与资产”区域选择交付文件后提交审核。
                          </p>
                          <p class="detail-v3-card-muted">提交动作统一由设计与资产面板处理。</p>
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
                      <p class="detail-v3-eyebrow">审核侧</p>
                      <h2 class="detail-v3-module-title">审核模块</h2>
                    </div>
                    <span class="detail-v3-state-pill detail-v3-state-pill--warning">
                      通过 / 打回 / 审核参考
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
                      <p class="detail-v3-card-kicker">待审核稿件</p>
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
                        :disabled="!showAuditActionButtons || Boolean(actionLoading)"
                        clearable
                      />
                      <BaseTextarea
                        v-model="auditComment"
                        :placeholder="auditRejectReasonCategory === AUDIT_REJECT_REASON_OTHER ? '填写其他具体理由...' : '填写通过说明或补充修改建议...'"
                        :rows="4"
                        :disabled="!showAuditActionButtons || Boolean(actionLoading)"
                        :error="auditCommentError"
                      />
                    </article>
                    <article class="detail-v3-info-card detail-v3-info-card--audit">
                      <p class="detail-v3-card-kicker">审核动作</p>
                      <p class="detail-v3-card-text">通过后进入仓库；打回后回到设计模块。</p>
                      <div v-if="showAuditActionButtons" class="detail-v3-inline-actions">
                        <button
                          type="button"
                          class="detail-v3-dark-btn"
                          :disabled="actionLoading === 'audit-pass'"
                          @click="passAuditFromDetail"
                        >
                          {{ actionLoading === 'audit-pass' ? '通过中...' : '通过' }}
                        </button>
                        <button
                          type="button"
                          class="detail-v3-danger-btn"
                          :disabled="actionLoading === 'audit-reject'"
                          @click="rejectAuditFromDetail"
                        >
                          {{ actionLoading === 'audit-reject' ? '打回中...' : '打回' }}
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
                          accept=".jpg,.jpeg,.png,.webp"
                          multiple
                          class="detail-v3-hidden-file-input"
                          @change="(event) => handleAuditAssetUpload(event, 'delivery')"
                        />
                        <div class="detail-v3-inline-actions">
                          <button
                            type="button"
                            class="detail-v3-light-btn"
                            :disabled="actionLoading === 'audit-upload'"
                            @click="auditSourceUploadInputRef?.click()"
                          >
                            上传修订源文件
                          </button>
                          <button
                            type="button"
                            class="detail-v3-dark-btn"
                            :disabled="actionLoading === 'audit-upload'"
                            @click="auditDeliveryUploadInputRef?.click()"
                          >
                            上传最终成品图
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
                          {{ actionLoading === 'warehouse-archive' ? '结单中...' : '结单' }}
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

    <div v-if="lightboxSrc" class="lightbox-overlay" @click="lightboxSrc = null">
      <img :src="lightboxSrc" alt="预览大图" class="lightbox-img" @click.stop />
    </div>
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
  canSubmitAudit,
  canUploadDesignDelivery,
  canReassignDesigner,
  isLegacyTaskStatusInDesignerEditablePhase,
  taskHasRecordedDesignOutput,
  taskHasAssignee,
} from '@/domain/task-actions'
import { getTaskActionAvailability } from '@/domain/task-action-availability'
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
import { isReferenceUrlExpiringSoon } from '@/utils/referenceUrl'
import { tasksApi } from '@/services/api/tasksApi'
import { uploadTaskReferenceFileViaAssetSession } from '@/services/api/design'
import type { AssetKind } from '@/services/api/assetsApi'
import { uploadTaskFileViaAssetSession } from '@/services/upload/assetUploadFlow'
import { buildTimestampedZipFilename, downloadBatchAsZip } from '@/utils/batchZipDownload'
import { resolveApiUserMessage } from '@/utils/api-message-zh'
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

import AsyncStateWrapper from '@/components/base/AsyncStateWrapper.vue'
import BaseButton from '@/components/base/BaseButton.vue'
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

// ── 子区块（通过 provide/inject 访问 task，无 prop drilling）──────────────
import DesignerSelectDialog from '@/components/task/DesignerSelectDialog.vue'
import ReassignDesignerDialog from '@/components/task/ReassignDesignerDialog.vue'
import EventLogDrawer from '@/components/logs/EventLogDrawer.vue'
import CancelReasonModal from '@/components/task-detail/CancelReasonModal.vue'
import TaskInfoEditModal from '@/components/task-detail/TaskInfoEditModal.vue'
import SkuItemsTable from '@/components/task-detail/SkuItemsTable.vue'
import SkuItemEditModal from '@/components/task-detail/SkuItemEditModal.vue'
import DesignAssetBlock from '@/components/task-detail/DesignAssetBlock.vue'
import AssetThumbStrip, { type AssetThumbItem } from '@/components/task-detail/AssetThumbStrip.vue'
import { useDesignerOptions } from '@/composables/useDesignerOptions'
import { warehouseBlockingReasonLine } from '@/utils/warehouse-blocking'
// v0.5 对齐：FRONTEND_ALIGNMENT_v0.5.md 第 D 节，任务详情内指派弹窗使用 GET /v1/users/designers

const OPEN_LIGHTBOX_KEY = 'task-detail-open-lightbox'

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
const isBatchTask = computed(() => task.value?.isBatchTask === true)
const batchSkuItems = computed(() => task.value?.skuItems ?? [])

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
    !!task.value &&
    (task.value.workflowLane === 'customization' ||
      task.value.customizationRequired === true ||
      Boolean(task.value.customizationSourceType)),
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

const TASK_TYPE_LABELS: Record<string, string> = {
  ORIGINAL_PRODUCT_DEV: '原品开发',
  NEW_PRODUCT_DEV: '新品开发',
  PURCHASE_TASK: '采购任务',
  RETOUCH_TASK: 'P 图任务',
  CUSTOMER_CUSTOMIZATION: '客户定制',
  REGULAR_CUSTOMIZATION: '常规定制',
}

const designModuleTitle = computed(() => {
  if (isCustomizationTask.value) return '定制模块'
  if (isRetouchTask.value) return '精修模块'
  return '设计模块'
})

const designModuleEyebrow = computed(() => {
  if (isCustomizationTask.value) return '定制侧'
  if (isRetouchTask.value) return '精修侧'
  return '设计侧'
})

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
  return `${amount} ${t?.costPrice?.currency ?? 'CNY'}`
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
const topLevelReferenceThumbItems = computed((): AssetThumbItem[] =>
  (task.value?.referenceFileRefs ?? [])
    .map((ref, index) => {
      const src = String(ref?.download_url ?? '').trim()
      if (!src) return null
      const filename = String(ref?.filename ?? '').trim()
      return {
        key: `task-level-ref-${index}-${src}`,
        src,
        alt: filename || `汇总参考图 ${index + 1}`,
        label: filename || `汇总参考图 ${index + 1}`,
      }
    })
    .filter((row) => row != null) as AssetThumbItem[],
)
const detailReferenceLabel = computed(() => {
  const taskRefs = task.value?.referenceFileRefs?.length ?? 0
  const skuRefs = task.value?.skuItems?.reduce((sum, item) => sum + (item.referenceFileRefs?.length ?? 0), 0) ?? 0
  const total = taskRefs + skuRefs
  return total > 0 ? `${total} 张图片 · 单文件 <= 300MB` : '暂无参考附件'
})
const totalReferenceCount = computed(() => {
  const taskRefs = task.value?.referenceFileRefs?.length ?? 0
  const skuRefs = task.value?.skuItems?.reduce((sum, item) => sum + (item.referenceFileRefs?.length ?? 0), 0) ?? 0
  return taskRefs + skuRefs
})
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
  if (canSubmitAudit(task.value)) return false
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
  (isBatchTask.value ? task.value?.referenceFileRefs ?? [] : collectTaskReferenceRefs())
    .map((ref, index) => {
      const src = String((ref as { download_url?: string })?.download_url ?? '').trim()
      if (!src) return null
      const filename = String((ref as { filename?: string })?.filename ?? '').trim()
      return {
        key: `ops-ref-${index}-${src}`,
        src,
        alt: filename || `参考图 ${index + 1}`,
        label: filename || `参考图 ${index + 1}`,
      }
    })
    .filter((item) => item != null)
    .slice(0, 6) as AssetThumbItem[],
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

// provide：让所有子区块无需 props 直接注入 task
provide(TASK_DETAIL_KEY, task)
const lightboxSrc = ref<string | null>(null)
function openLightbox(src: string) {
  const url = String(src ?? '').trim()
  if (!url) return
  lightboxSrc.value = url
}
provide(OPEN_LIGHTBOX_KEY, openLightbox)

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

function collectTaskReferenceRefs() {
  const detailTask = task.value
  if (!detailTask) return []
  const rootRefs = detailTask.referenceFileRefs ?? []
  const skuRefs = detailTask.skuItems?.flatMap((item) => item.referenceFileRefs ?? []) ?? []
  return [...rootRefs, ...skuRefs]
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
    if (hasModuleActionProjection(auditModuleSummary.value)) {
      return hasModuleAction(auditModuleSummary.value, ['approve', 'reject'])
    }
    return Boolean(actionAvailability.value?.canShowAuditActions)
  },
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
    // 与后端 maintenance scope（如 role_plus_maintenance_scope / task_out_of_department_scope）对齐
    if (!canOperateTask(task.value)) return false
    if (isPurchaseTask.value) return purchaseWorkflowCanClose.value
    if (hasModuleActionProjection(warehouseModuleSummary.value)) {
      return hasModuleAction(warehouseModuleSummary.value, ['close_task', 'complete'])
    }
    return Boolean(actionAvailability.value?.canShowWarehouseComplete)
  },
)

const canEditBasicInfo = computed(
  () => {
    if (!task.value || !hasTaskScopeAccess.value || !can('task.edit')) return false
    if (hasModuleActionProjection(basicInfoModuleSummary.value)) {
      return hasModuleAction(basicInfoModuleSummary.value, [
        'update_basic_info',
        'update_reference_files',
      ])
    }
    return isLegacyTaskStatusInDesignerEditablePhase(task.value)
  },
)
const canUploadReferenceFromOps = computed(() => canEditBasicInfo.value)
const canDirectSkuDesignUpload = computed(() => {
  if (!task.value || isPurchaseTask.value || !hasTaskScopeAccess.value) return false
  if (!can('design.upload')) return false
  if (isRetouchTask.value) return showRetouchSubmitAction.value
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
  editingSkuItem.value = payload.item
  skuItemEditOpen.value = true
}

function openSkuDesignUpload(payload: { item: TaskSkuItem; index: number }) {
  if (!task.value) return
  if (!canDirectSkuDesignUpload.value) {
    actionError.value = '当前状态不可上传设计稿'
    return
  }
  actionError.value = ''
  detailProductIndex.value = Math.max(0, payload.index)
  const skuCode = String(payload.item.skuCode ?? '').trim()
  const skuSuffix = skuCode ? `（${skuCode}）` : ''
  flashSuccess(`已切换到子项 ${payload.index + 1}${skuSuffix}，请在设计模块上传设计稿`)
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
  flashSuccess('SKU 子项已更新')
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

const showAssignDesignerButton = computed(
  () => {
    if (!task.value || isPurchaseTask.value || taskHasAssignee(task.value)) return false
    if (designModuleAllowsAssign.value) return true
    return (
      canAssignPermission.value &&
      Boolean(actionAvailability.value?.canShowAssign) &&
      canAssign(task.value)
    )
  },
)

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

/** 换人：设计阶段可调度，但进入审核责任链及之后阶段不可重派（见 canReassignDesigner） */
const showReassignDesignerButton = computed(
  () => {
    // 重新指派是“调度动作”而非通用详情写操作：
    // 这里不再强依赖 hasTaskScopeAccess（owner_department 口径），
    // 改由 canReassignPermission + 状态门禁 + 后端 allowed_actions 共同控制。
    if (!task.value || isPurchaseTask.value) return false
    if (!taskHasAssignee(task.value)) return false
    if (designModuleAllowsReassign.value || retouchModuleAllowsReassign.value) return true
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
    actionError.value = '设计与资产区尚未渲染完成，请稍后重试'
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
  const currentTask = task.value
  if (!input.files?.length || !currentTask?.id) return
  const picked = Array.from(input.files)
  input.value = ''
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
    for (const file of validFiles) {
      await uploadTaskReferenceFileViaAssetSession(currentTask.id, file)
    }
    opsReferenceUploadStatus.value = '上传完成'
    await tasksStore.loadTaskById(currentTask.id)
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
      zipFilename: buildTimestampedZipFilename('task-references'),
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
  const currentTask = task.value
  if (!input.files?.length || !currentTask?.id) return
  const picked = Array.from(input.files)
  input.value = ''
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
    const [taskEventsResult, costEventsResult] = await Promise.allSettled([
      tasksApi.listTaskEvents(tid),
      tasksApi.getCostOverrides(tid),
    ])
    if (taskEventsResult.status === 'rejected' && costEventsResult.status === 'rejected') {
      throw taskEventsResult.reason
    }
    const taskEvents =
      taskEventsResult.status === 'fulfilled'
        ? extractTaskEventsList(taskEventsResult.value.data).map((row) =>
            mapTaskEventRowToRecentEvent(row, tid),
          )
        : []
    const costEvents =
      costEventsResult.status === 'fulfilled'
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

watch(taskId, () => {
  if (!taskId.value || isTempId.value) return
  auditRejectReasonCategory.value = ''
  auditComment.value = ''
  auditCommentError.value = ''
  void loadSideEvents()
})

watch(auditComment, () => {
  if (auditCommentError.value) auditCommentError.value = ''
})
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
  if (!task.value) return
  actionError.value = ''
  actionSuccess.value = ''
  assignDialogVisible.value = true
  if (designerOptions.value.length === 0) loadDesigners()
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
    flashSuccess('已领取精修任务，可以开始上传设计稿并提交')
    void loadSideEvents()
  })
}

async function passAuditFromDetail(): Promise<void> {
  if (!task.value) return
  if (!showAuditActionButtons.value) return
  auditCommentError.value = ''
  const comment = auditComment.value.trim() || '审核通过'
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
  if (!showAuditActionButtons.value) return
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
  await runDetailAction('audit-reject', '审核打回失败', async () => {
    await tasksStore.rejectAudit(task.value!.id, {
      stage: auditStageForTask(),
      comment: rejectComment,
    })
    auditRejectReasonCategory.value = ''
    auditComment.value = ''
    flashSuccess('已打回设计')
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
        await tasksApi.warehouseComplete(currentTask.id)
        await tasksStore.loadTaskById(currentTask.id)
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
  await runDetailAction('warehouse-archive', '结单失败', async () => {
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
        await tasksApi.warehouseComplete(currentTask.id)
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
        await tasksApi.closeTask(currentTask.id, {})
        await tasksStore.loadTaskById(currentTask.id)
      } catch (err) {
        throw new Error(formatCloseArchiveError(err))
      }
    } else {
      await tasksStore.archiveTask(task.value!.id)
    }
    flashSuccess('已结单')
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
  if (!task.value) return
  try {
    await tasksStore.assignTask(task.value.id, payload)
    assignDialogVisible.value = false
    flashSuccess(`已指派给 ${payload.assigneeName}`)
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
  gap: 0.5rem;
  align-items: center;
}
.detail-top-chip {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  height: 2.125rem;
  padding: 0 0.9rem;
  border: none;
  border-radius: var(--dv-r-control);
  background: #f2f4f7;
  color: #475467;
  font-size: 0.75rem;
  font-weight: 700;
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease;
}
.detail-top-chip:hover {
  background: #e9edf3;
  color: #1f2937;
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
  background: #151a21;
  color: #fff;
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.1);
}
.detail-top-chip--primary:hover {
  background: #0f1218;
  color: #fff;
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
  background: #151a21 !important;
  border-color: #151a21 !important;
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
.detail-v3-info-card--cost {
  background: #fffaf0;
  border-color: #ffedd4;
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
  border: 1px solid #151a21;
  padding: 0.5rem 0.85rem;
  font-size: 0.75rem;
  font-weight: 800;
  cursor: pointer;
  border-radius: var(--dv-r-control, 0.625rem);
  background: #151a21;
  color: #fff;
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.08);
}
.detail-v3-upload-ref-btn:hover {
  background: #0f1218;
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
  background: #151a21;
  color: #fff;
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.08);
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
.lightbox-overlay {
  --lightbox-header-height: 64px;
  --lightbox-margin: 12px;
  position: fixed;
  left: 0;
  right: 0;
  top: calc(var(--lightbox-header-height) + var(--lightbox-margin));
  bottom: 0;
  padding: var(--lightbox-margin);
  background: rgba(0, 0, 0, 0.7);
  display: flex;
  align-items: flex-start;
  justify-content: center;
  z-index: 9999;
  cursor: zoom-out;
  overflow: auto;
}
.lightbox-img {
  max-width: calc(100vw - (var(--lightbox-margin) * 2));
  max-height: calc(100vh - var(--lightbox-header-height) - (var(--lightbox-margin) * 2));
  object-fit: contain;
  border-radius: 6px;
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
  font-family: 'SF Mono', 'Cascadia Code', 'Consolas', monospace;
  letter-spacing: 0.01em;
}
.batch-sku-tab:hover {
  border-color: #98a2b3;
  background: #f1f5f9;
}
.batch-sku-tab--active {
  background: #151a21;
  color: #fff;
  border-color: #151a21;
}
.batch-sku-tab--active:hover {
  background: #0f1218;
  border-color: #0f1218;
}

/* Apple Music / iOS liquid glass task detail skin. Style-only. */
.task-detail-view {
  color: var(--yb-music-text-2);
  background:
    radial-gradient(circle at 14% 0%, rgba(255, 45, 141, 0.18), transparent 30rem),
    radial-gradient(circle at 88% 8%, rgba(100, 210, 255, 0.16), transparent 32rem),
    transparent;
}

.detail-v6-surface {
  --dv-border-soft: rgba(255, 255, 255, 0.16);
  --dw-title: #ffffff;
  --dw-label: rgba(220, 230, 255, 0.66);
  --dv-surface-elev: var(--yb-glass-shadow);
}

.detail-top-unified.detail-top-v6,
.detail-v3-module,
.detail-v3-side,
.detail-v3-info-card,
.detail-merge-card--center,
.batch-sku-switcher,
.create-success-banner {
  border-color: var(--yb-music-border) !important;
  background:
    linear-gradient(145deg, rgba(255, 255, 255, 0.13), rgba(255, 255, 255, 0.06)) !important;
  color: var(--yb-music-text-2) !important;
  box-shadow: var(--yb-glass-shadow) !important;
  backdrop-filter: blur(var(--yb-glass-blur));
  -webkit-backdrop-filter: blur(var(--yb-glass-blur));
}

.detail-top-unified.detail-top-v6 {
  position: relative;
  overflow: hidden;
}

.detail-top-unified.detail-top-v6::before {
  content: '';
  position: absolute;
  inset: -5rem -3rem auto auto;
  width: 30rem;
  height: 16rem;
  border-radius: 999px;
  pointer-events: none;
  background: radial-gradient(circle, rgba(100, 210, 255, 0.2), transparent 68%);
}

.detail-top-grid,
.detail-v3-module-head,
.detail-v3-info-grid,
.detail-v3-workflow-grid {
  position: relative;
}

.detail-top-taskno,
.detail-v3-module-title,
.detail-v3-side-title,
.detail-v3-card-kicker,
.detail-v3-side-event-title,
.detail-v6-surface :deep(.block-title) {
  color: #fff !important;
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
  color: var(--yb-music-muted) !important;
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
  border-color: rgba(255, 255, 255, 0.13) !important;
  background: rgba(3, 3, 5, 0.34) !important;
  color: var(--yb-music-text-2) !important;
}

.detail-top-current.detail-top-status-pill,
.detail-top-type-pill,
.detail-top-priority-pill--muted,
.detail-top-batch-pill,
.detail-v3-state-pill,
.batch-sku-tab {
  background: rgba(255, 255, 255, 0.1) !important;
  border-color: rgba(255, 255, 255, 0.18) !important;
  color: var(--yb-music-text-2) !important;
}

.detail-top-status-dot {
  background: var(--yb-music-cyan);
  box-shadow: 0 0 0 3px rgba(100, 210, 255, 0.18), 0 0 20px rgba(100, 210, 255, 0.38);
}

.detail-top-chip--danger,
.detail-v3-danger-btn,
.create-success-banner.banner-error {
  background: rgba(255, 69, 58, 0.16) !important;
  border-color: rgba(255, 69, 58, 0.34) !important;
  color: #ffd5d2 !important;
}

.detail-v3-link-chip,
.detail-v3-link-btn,
.detail-v3-ref-status,
.detail-top-flow-shell :deep(.step-current .step-label) {
  color: var(--yb-music-cyan) !important;
}

.action-success,
.create-success-banner.banner-info {
  border: 1px solid rgba(48, 209, 88, 0.32) !important;
  background: rgba(48, 209, 88, 0.14) !important;
  color: #b7fbd0 !important;
}

.action-error,
.create-success-banner.banner-error {
  border: 1px solid rgba(255, 69, 58, 0.34) !important;
  background: rgba(255, 69, 58, 0.14) !important;
  color: #ffd5d2 !important;
}

.create-success-banner.banner-warning {
  border: 1px solid rgba(255, 149, 0, 0.34) !important;
  background: rgba(255, 149, 0, 0.14) !important;
  color: #ffd28a !important;
}

.banner-dismiss {
  color: inherit !important;
}

.detail-top-flow-shell :deep(.step-current .step-dot--sm) {
  background: var(--yb-music-cyan);
  box-shadow: 0 0 0 3px rgba(100, 210, 255, 0.18);
}

.detail-top-flow-shell :deep(.step-done .step-dot--sm),
.detail-top-flow-shell :deep(.step-done .step-label) {
  background: var(--yb-music-green);
  color: var(--yb-music-green) !important;
}

.detail-col--left,
.detail-col--center,
.detail-col--right,
.detail-design-band {
  background: rgba(255, 255, 255, 0.05) !important;
  border-color: rgba(255, 255, 255, 0.12) !important;
}

/* Bmhxp readable-work-panel pass: keep the Apple atmosphere behind the content. */
.task-detail-view {
  background:
    radial-gradient(circle at 10% -8%, rgba(255, 45, 141, 0.11), transparent 28rem),
    radial-gradient(circle at 92% -4%, rgba(100, 210, 255, 0.1), transparent 30rem),
    linear-gradient(135deg, #07080d 0%, #10131b 52%, #071019 100%);
}

.detail-top-unified.detail-top-v6,
.detail-v3-module,
.detail-v3-side,
.detail-v3-info-card,
.detail-merge-card--center,
.batch-sku-switcher {
  border-color: rgba(160, 176, 204, 0.22) !important;
  background: linear-gradient(145deg, rgba(20, 26, 36, 0.97), rgba(10, 13, 20, 0.98)) !important;
  box-shadow: 0 14px 32px -28px rgba(0, 0, 0, 0.9) !important;
}

.detail-top-unified.detail-top-v6::before {
  opacity: 0.42;
  filter: saturate(0.8);
}

.detail-v3-info-card,
.detail-v3-requirement-box,
.detail-v3-fake-textarea,
.detail-v3-layout :deep(section.detail-block),
.detail-v6-surface :deep(section.detail-block),
.detail-col--right :deep(.detail-block),
.detail-col--left :deep(section.detail-block) {
  background: #10151e !important;
  border-color: rgba(160, 176, 204, 0.2) !important;
}

.detail-v3-card-kicker,
.detail-v3-module-title,
.detail-v3-side-title,
.detail-v3-layout :deep(.block-title) {
  color: #f8fafc !important;
}

.detail-v3-eyebrow,
.detail-v3-side-kicker,
.detail-v3-kv-list dt,
.detail-v6-surface :deep(dt),
.detail-v6-surface :deep(.field-label),
.detail-v6-surface :deep(.section-label),
.detail-v3-requirement-box span {
  color: #92a0b8 !important;
}

.detail-v3-kv-list dd,
.detail-v3-card-text,
.detail-v3-requirement-box p,
.detail-v6-surface :deep(dd),
.detail-v6-surface :deep(.field-value),
.detail-v6-surface :deep(.value),
.detail-v6-surface :deep(input),
.detail-v6-surface :deep(textarea) {
  color: #f7faff !important;
}

.detail-v3-card-muted,
.detail-v3-side-desc,
.detail-v3-module-note,
.detail-v3-side-event-desc {
  color: #aeb9cc !important;
}

/* Design stage becomes the primary operation surface without changing DOM. */
.detail-v3-module {
  position: relative;
  overflow: hidden;
}

.detail-v3-module--design {
  border-color: rgba(100, 210, 255, 0.34) !important;
  background:
    linear-gradient(145deg, rgba(16, 24, 36, 0.99), rgba(8, 11, 18, 0.99)) !important;
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.06),
    0 0 0 1px rgba(100, 210, 255, 0.08),
    0 22px 44px -32px rgba(100, 210, 255, 0.52) !important;
}

.detail-v3-module--design::before {
  content: '';
  position: absolute;
  inset: -8rem -6rem auto auto;
  width: 28rem;
  height: 18rem;
  border-radius: 999px;
  pointer-events: none;
  background: radial-gradient(circle, rgba(100, 210, 255, 0.14), transparent 68%);
}

.detail-v3-module--design .detail-v3-module-head,
.detail-v3-module--design .batch-sku-switcher,
.detail-v3-module--design .detail-v3-workflow-grid {
  position: relative;
}

.detail-v3-module--design .detail-v3-info-card--wide {
  border-color: rgba(100, 210, 255, 0.3) !important;
  background: linear-gradient(145deg, rgba(13, 19, 30, 0.99), rgba(7, 11, 18, 0.99)) !important;
}

.detail-v3-module--design .detail-v3-workflow-grid--design > .detail-v3-info-card:nth-child(3),
.detail-v3-module--design .detail-v3-workflow-grid--design > .detail-v3-info-card:nth-child(4),
.detail-v3-module--design .detail-v3-workflow-grid--design > .detail-v3-info-card--refs {
  border-color: rgba(100, 210, 255, 0.32) !important;
  background:
    linear-gradient(145deg, rgba(14, 27, 41, 0.98), rgba(8, 13, 21, 0.99)) !important;
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.06),
    0 12px 28px -24px rgba(100, 210, 255, 0.6) !important;
}

.detail-v3-module--design .detail-v3-workflow-grid--design > .detail-v3-info-card:nth-child(3) .detail-v3-card-kicker,
.detail-v3-module--design .detail-v3-workflow-grid--design > .detail-v3-info-card:nth-child(4) .detail-v3-card-kicker,
.detail-v3-module--design .detail-v3-workflow-grid--design > .detail-v3-info-card--refs .detail-v3-card-kicker {
  color: #e0f7ff !important;
}

.detail-v3-module--design .detail-v3-dark-btn,
.detail-v3-module--design .detail-v3-upload-ref-btn,
.detail-v3-module--design .detail-v3-link-btn {
  background: linear-gradient(120deg, #1d4ed8 0%, #38bdf8 100%) !important;
  border-color: rgba(125, 211, 252, 0.42) !important;
  color: #f8fcff !important;
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.2),
    0 14px 28px -20px rgba(56, 189, 248, 0.86) !important;
}

.detail-v3-module--audit,
.detail-v3-module--warehouse {
  border-color: rgba(148, 163, 184, 0.14) !important;
  background: linear-gradient(145deg, rgba(15, 19, 27, 0.9), rgba(8, 10, 16, 0.94)) !important;
  opacity: 0.78;
}

.detail-v3-module--audit:hover,
.detail-v3-module--warehouse:hover,
.detail-v3-module--audit:focus-within,
.detail-v3-module--warehouse:focus-within {
  opacity: 0.94;
  border-color: rgba(160, 176, 204, 0.24) !important;
}

.detail-v3-module--audit .detail-v3-info-card,
.detail-v3-module--warehouse .detail-v3-info-card,
.detail-v3-module--audit :deep(section.detail-block),
.detail-v3-module--warehouse :deep(section.detail-block) {
  background: rgba(12, 16, 24, 0.88) !important;
  border-color: rgba(148, 163, 184, 0.16) !important;
}

.detail-v3-module--audit .detail-v3-state-pill,
.detail-v3-module--warehouse .detail-v3-state-pill {
  background: rgba(148, 163, 184, 0.1) !important;
  color: #aeb9cc !important;
  border-color: rgba(148, 163, 184, 0.16) !important;
}

/* Right rail activity becomes a readable timeline; newest event gets the only highlight. */
.detail-v3-side {
  background: linear-gradient(180deg, rgba(13, 17, 26, 0.98), rgba(7, 9, 14, 0.98)) !important;
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
  background: linear-gradient(180deg, rgba(100, 210, 255, 0.5), rgba(148, 163, 184, 0.12));
}

.detail-v3-side-event {
  position: relative;
  gap: 0.28rem;
  padding: 0.72rem 0.78rem;
  border-color: rgba(148, 163, 184, 0.14) !important;
  background: rgba(16, 21, 30, 0.82) !important;
  opacity: 0.72;
  transition:
    opacity 0.16s ease,
    border-color 0.16s ease,
    background-color 0.16s ease,
    transform 0.16s ease;
}

.detail-v3-side-event::before {
  content: '';
  position: absolute;
  left: -0.92rem;
  top: 1rem;
  width: 0.48rem;
  height: 0.48rem;
  border-radius: 999px;
  background: #64748b;
  box-shadow: 0 0 0 3px rgba(100, 116, 139, 0.16);
}

.detail-v3-side-event:first-child {
  opacity: 1;
  border-color: rgba(100, 210, 255, 0.35) !important;
  background: linear-gradient(145deg, rgba(17, 31, 45, 0.98), rgba(10, 15, 23, 0.98)) !important;
}

.detail-v3-side-event:first-child::before {
  background: #64d2ff;
  box-shadow: 0 0 0 4px rgba(100, 210, 255, 0.16), 0 0 18px rgba(100, 210, 255, 0.32);
}

.detail-v3-side-event:hover {
  opacity: 1;
  transform: translateX(2px);
  border-color: rgba(160, 176, 204, 0.26) !important;
}

.detail-v3-side-event-title {
  color: #f7faff !important;
  font-weight: 750;
}

.detail-v3-side-event:not(:first-child) .detail-v3-side-event-title {
  color: #dbe5f7 !important;
}

/* Top workflow: restore connector visibility and map CORTP motion to existing markup. */
.detail-top-flow-shell {
  border-color: rgba(100, 210, 255, 0.22) !important;
  background: #070b12 !important;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.06) !important;
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
  border: 1px solid rgba(148, 163, 184, 0.42);
  background: rgba(71, 85, 105, 0.24);
}

.detail-top-flow-shell :deep(.step-done .step-dot--sm) {
  background: #22c55e;
  border-color: #86efac;
}

.detail-top-flow-shell :deep(.step-current .step-dot--sm) {
  background: #64d2ff;
  border-color: #bae6fd;
  box-shadow: 0 0 0 5px rgba(100, 210, 255, 0.14), 0 0 24px rgba(100, 210, 255, 0.32);
}

.detail-top-flow-shell :deep(.workflow-progress--horizontal .step-label) {
  color: #cbd5e1;
  font-size: 0.75rem;
}

.detail-top-flow-shell :deep(.step-done .step-label) {
  color: #eafbf0 !important;
}

.detail-top-flow-shell :deep(.step-current .step-label) {
  color: #e0f7ff !important;
}

.detail-top-flow-shell :deep(.step-sublabel-inline) {
  display: inline;
  color: #92a0b8;
}

.detail-top-flow-shell :deep(.step-sep) {
  display: block;
}

@media (prefers-reduced-motion: reduce) {
  .detail-v3-side-event,
  .detail-top-flow-shell :deep(.step-chip),
  .detail-top-flow-shell :deep(.step-dot--sm),
  .detail-top-flow-shell :deep(.step-sep::after) {
    animation: none !important;
    transition: none !important;
  }
}

.lightbox-overlay {
  background: rgba(0, 0, 0, 0.84);
}

.batch-sku-tab--active,
.detail-v6-surface :deep(.product-tab-active) {
  background: linear-gradient(105deg, var(--yb-music-pink), var(--yb-music-purple), var(--yb-music-cyan)) !important;
  border-color: rgba(255, 255, 255, 0.24) !important;
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
  background: rgba(15, 23, 42, 0.24) !important;
  opacity: 1 !important;
}

.detail-top-flow-shell :deep(.step-chip.step-current) {
  background: rgba(100, 210, 255, 0.1) !important;
}

.detail-top-flow-shell :deep(.step-dot--sm) {
  width: 0.72rem !important;
  height: 0.72rem !important;
  background: rgba(71, 85, 105, 0.34) !important;
  border-color: rgba(148, 163, 184, 0.38) !important;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.08) !important;
}

.detail-top-flow-shell :deep(.step-done .step-dot--sm) {
  background: linear-gradient(135deg, #334155, #38bdf8) !important;
  border-color: rgba(125, 211, 252, 0.72) !important;
  color: #06111d !important;
  box-shadow: 0 0 0 2px rgba(56, 189, 248, 0.11), 0 0 14px rgba(56, 189, 248, 0.18) !important;
}

.detail-top-flow-shell :deep(.step-current .step-dot--sm) {
  background: #64d2ff !important;
  border-color: #e0f7ff !important;
  color: #031420 !important;
  box-shadow: 0 0 0 4px rgba(100, 210, 255, 0.14), 0 0 22px rgba(100, 210, 255, 0.34) !important;
}

.detail-top-flow-shell :deep(.step-skipped .step-dot--sm),
.detail-top-flow-shell :deep(.step-pending .step-dot--sm) {
  background: rgba(71, 85, 105, 0.26) !important;
  border-color: rgba(100, 116, 139, 0.42) !important;
  color: rgba(148, 163, 184, 0.7) !important;
  box-shadow: none !important;
}

.detail-top-flow-shell :deep(.workflow-progress--horizontal .step-label) {
  color: #b7c4d8 !important;
  max-width: 100% !important;
  overflow: hidden !important;
  text-overflow: ellipsis !important;
  white-space: nowrap !important;
}

.detail-top-flow-shell :deep(.step-done .step-label) {
  color: #dcefff !important;
}

.detail-top-flow-shell :deep(.step-current .step-label) {
  color: #f7fcff !important;
}

.detail-top-flow-shell :deep(.step-pending .step-label),
.detail-top-flow-shell :deep(.step-skipped .step-label) {
  color: #7f8da3 !important;
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
  color: #8fa0b8 !important;
}

.detail-top-flow-shell :deep(.step-sep) {
  flex: 0 1 clamp(0.8rem, 2vw, 2.4rem) !important;
  width: auto !important;
  min-width: 0.55rem !important;
  height: 0.125rem !important;
  background: rgba(71, 85, 105, 0.45) !important;
}

.detail-top-flow-shell :deep(.step-chip.step-done + .step-sep) {
  background: linear-gradient(90deg, rgba(56, 189, 248, 0.75), rgba(100, 210, 255, 0.42)) !important;
}

.detail-top-flow-shell :deep(.step-chip.step-current + .step-sep) {
  background: linear-gradient(90deg, rgba(100, 210, 255, 0.62), rgba(71, 85, 105, 0.42)) !important;
}

/* Task detail alignment repair: prevent the top card from inheriting oversized three-column minimums. */
.task-detail-view {
  background:
    radial-gradient(circle at 12% 0%, rgba(56, 189, 248, 0.08), transparent 24rem),
    linear-gradient(145deg, #111827 0%, #0b111a 54%, #060a10 100%) !important;
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
  background:
    linear-gradient(145deg, rgba(31, 39, 54, 0.96), rgba(13, 19, 30, 0.98)) !important;
  border-color: rgba(100, 116, 139, 0.42) !important;
}

.detail-top-unified.detail-top-v6::before {
  inset: -8rem -7rem auto auto !important;
  opacity: 0.22 !important;
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
  border: 1px solid rgba(125, 211, 252, 0.36) !important;
  border-radius: 0.625rem !important;
  background: rgba(8, 145, 178, 0.12) !important;
  color: #9be8ff !important;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.06) !important;
}

.detail-v3-info-card--refs .detail-v3-link-btn:hover {
  border-color: rgba(125, 211, 252, 0.62) !important;
  background: rgba(8, 145, 178, 0.2) !important;
  color: #f0fbff !important;
}

.detail-v3-info-card--refs .detail-v3-card-text {
  color: #dce7f7 !important;
}

.detail-v3-module-note {
  background:
    linear-gradient(135deg, rgba(14, 26, 40, 0.96), rgba(9, 14, 22, 0.98)) !important;
  border: 1px solid rgba(100, 210, 255, 0.22) !important;
  color: #aebdd2 !important;
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.05),
    0 10px 28px rgba(0, 0, 0, 0.16) !important;
}

.detail-v3-module-note::before {
  color: #e8f7ff !important;
  background: rgba(100, 210, 255, 0.16) !important;
  border: 1px solid rgba(125, 211, 252, 0.26) !important;
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
  background: linear-gradient(90deg, rgba(148, 113, 255, 0.38), rgba(100, 210, 255, 0.46)) !important;
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
</style>
