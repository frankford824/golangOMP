<template>
  <div
    class="assets-index-view min-h-[100dvh] pb-16"
    :data-selected-count="selectedCount"
    :data-selected-assets="selectedAssets.length"
  >
    <header class="ac-header">
      <div class="ac-nav-box">
        <h1 class="ac-brand">资产管理</h1>
        <div class="ac-search-wrap">
          <svg
            class="ac-search-icon"
            width="18"
            height="18"
            fill="none"
            stroke="currentColor"
            stroke-width="2.5"
            aria-hidden="true"
          >
            <circle cx="9" cy="9" r="7" />
            <path d="M14 14l4.5 4.5" />
          </svg>
          <input
            v-model="filters.keyword"
            type="search"
            class="ac-search-input"
            placeholder="搜索系统资产、外部文件名、路径…"
            autocomplete="off"
            enterkeyhint="search"
          />
        </div>
        <div class="ac-header-actions">
          <span class="ac-aria-hint" aria-live="polite">{{ copyHint }}</span>
          <button
            type="button"
            class="ac-icon-btn"
            :aria-expanded="filtersExpanded"
            aria-controls="asset-filters-panel"
            @click="filtersExpanded = !filtersExpanded"
          >
            筛选
          </button>
          <button type="button" class="ac-icon-btn" :disabled="loading" @click="reload">
            {{ loading ? '刷新中' : '刷新' }}
          </button>
          <button type="button" class="ac-icon-btn ac-icon-btn--primary" @click="openBulkSearchModal">
            批量搜索下载
          </button>
          <button type="button" class="ac-icon-btn ac-icon-btn--primary" :disabled="excelPackaging" @click="openExcelPicker">
            {{ excelPackaging ? '模板打包中' : 'Excel 图片分拣下载' }}
          </button>
          <input
            ref="excelFileInput"
            type="file"
            class="ac-hidden-file"
            accept=".xlsx"
            aria-label="选择 Excel 图片分拣文件"
            @change="handleExcelPackageFile"
          />
          <input
            ref="replacementFileInput"
            type="file"
            class="ac-hidden-file"
            aria-label="选择资产替换文件"
            @change="handleReplacementFile"
          />
          <button type="button" class="ac-icon-btn ac-icon-btn--primary" @click="selectedModalOpen = true">
            已选资产
          </button>
        </div>
      </div>

      <div
        v-show="filtersExpanded"
        id="asset-filters-panel"
        class="ac-filters-panel"
      >
        <div class="ac-filters-grid">
          <BaseSelect
            v-model="filters.resourceSource"
            label="资源来源"
            :options="assetSourceOptions"
          />
          <BaseSelect
            v-model="filters.timeBasis"
            label="时间口径"
            :options="assetTimeBasisOptions"
          />
          <BaseInput
            v-model="filters.createdFrom"
            type="date"
            label="时间范围开始"
          />
          <BaseInput
            v-model="filters.createdTo"
            type="date"
            label="时间范围结束"
          />
          <BaseSelect
            v-model="filters.taskLane"
            label="任务类型"
            :options="assetTaskLaneOptions"
          />
          <BaseSelect
            v-model="filters.assetKind"
            label="文件类型"
            :options="assetKindFilterOptions"
            :disabled="filters.resourceSource === 'external'"
          />
          <BaseSelect
            v-model="filters.moduleKey"
            label="产生环节"
            :options="assetSourceModuleOptions"
            :disabled="filters.resourceSource === 'external'"
          />
          <BaseSelect
            v-model="filters.formatCategory"
            label="格式分类"
            :options="assetFormatCategoryOptions"
          />
        </div>
      </div>
    </header>

    <div
      v-if="!loading && !error"
      class="ac-status-bar"
      role="status"
    >
      当前共 <b>{{ listTotal }}</b> 条，当前页返回 <b>{{ assets.length }}</b> 条，展示 <b>{{ pagedAssets.length }}</b> 条
    </div>
    <section v-if="assetPredictionSuggestions.length" class="ac-prediction-strip">
      <div class="ac-prediction-head">
        <div>
          <span>预测提示</span>
          <strong>可能要用的资源</strong>
        </div>
        <small>按可用状态和最近匹配排序</small>
      </div>
      <div class="ac-prediction-list">
        <article
          v-for="item in assetPredictionSuggestions"
          :key="item.id"
          class="ac-prediction-item"
        >
          <button type="button" class="ac-prediction-action" @click="openPredictionAsset(item)">
            <span>{{ item.source || '资产中心' }}</span>
            <strong>{{ item.title }}</strong>
            <small v-if="item.detail">{{ item.detail }}</small>
            <em>{{ item.action_label || '打开资产' }}</em>
          </button>
          <PredictionFeedbackControl :suggestion="item" surface="asset_center" />
        </article>
      </div>
    </section>
    <div v-if="selectedCount > 0" class="ac-batch-bar">
      <span class="ac-batch-count">已选 {{ selectedCount }} 项</span>
      <button type="button" class="ac-batch-btn" @click="selectedModalOpen = true">查看已选</button>
      <button type="button" class="ac-batch-btn ac-batch-btn--ghost" @click="clearSelectedAssets">
        清空选择
      </button>
      <button
        type="button"
        class="ac-batch-btn ac-batch-btn--primary"
        :disabled="!canBatchDownload"
        @click="handleBatchDownload"
      >
        {{ batchDownloading ? '批量下载中...' : '批量下载' }}
      </button>
      <span v-if="batchDownloadStatus" class="ac-batch-status">{{ batchDownloadStatus }}</span>
      <span v-if="batchDownloadError" class="ac-batch-error">{{ batchDownloadError }}</span>
    </div>
    <div v-if="excelPackageStatus || excelPackageError" class="ac-excel-package-bar">
      <span v-if="excelPackageStatus" class="ac-batch-status">{{ excelPackageStatus }}</span>
      <span v-if="excelPackageError" class="ac-batch-error">{{ excelPackageError }}</span>
    </div>
    <div v-if="replacementStatus || replacementError" class="ac-excel-package-bar">
      <span v-if="replacementStatus" class="ac-batch-status">{{ replacementStatus }}</span>
      <span v-if="replacementError" class="ac-batch-error">{{ replacementError }}</span>
    </div>
    <main class="ac-grid">
      <div v-if="loading" class="ac-loading-state">
        <p class="ac-loading-title">正在加载</p>
        <p class="ac-loading-sub">请稍候…</p>
      </div>
      <div v-else-if="error" class="ac-loading-state ac-state-error">{{ error }}</div>
      <div v-else-if="!pagedAssets.length" class="ac-grid-empty">
        <BaseEmptyState
          title="暂无资产"
          description="请输入关键词或调整筛选条件；若无匹配将显示此提示。"
        />
      </div>
      <template v-else>
        <article
          v-for="asset in pagedAssets"
          :key="assetResourceId(asset)"
          class="ac-card"
          :class="{
            'ac-card--active': selectedAssetId === assetResourceId(asset),
            'ac-card--selected': isAssetSelected(asset),
            'ac-card--external': isExternalAsset(asset),
          }"
        >
          <label class="ac-card-check" @click.stop>
            <input
              type="checkbox"
              class="ac-card-checkbox"
              :aria-label="`选择资产 ${assetDisplayTitle(asset)}`"
              :disabled="isExternalAsset(asset)"
              :title="isExternalAsset(asset) ? '外部资源请单个下载' : ''"
              :checked="isAssetSelected(asset)"
              @change.stop="onAssetSelectionChange(asset, $event)"
            />
          </label>
          <div class="ac-card-img-box">
            <AssetPreviewMedia
              :asset-id="assetResourceId(asset)"
              :resolved-preview-url="listCardResolvedPreviewUrl(asset)"
              defer-until-visible
              alt=""
              img-class="ac-card-apm"
              inner-img-class="ac-card-preview-img"
              @open-full="(u) => openAssetPreviewLightbox(asset, u, pagedAssets)"
            />
            <AssetDownloadLink
              class="ac-card-download-fab"
              variant="button"
              :asset-id="assetResourceId(asset)"
              :href="listCardResolvedPreviewUrl(asset)"
              :aria-label="`下载 ${businessSku(asset)} 资产文件`"
              @click.stop
            >
              下载
            </AssetDownloadLink>
          </div>
          <div class="ac-card-info">
            <div class="ac-title-row">
              <h2 class="ac-card-title" :title="assetDisplayTitle(asset)">{{ assetDisplayTitle(asset) }}</h2>
              <span class="ac-source-pill" :class="{ 'ac-source-pill--external': isExternalAsset(asset) }">
                {{ assetSourceLabel(asset) }}
              </span>
              <span
                v-if="!isExternalAsset(asset)"
                class="ac-usability-pill"
                :class="assetUsableToneClass(asset)"
              >
                {{ assetUsableLabel(asset) }}
              </span>
              <span
                v-if="assetCanBeReplaced(asset)"
                class="ac-editable-pill"
              >
                可修改资源
              </span>
              <span
                class="ac-format-pill"
                :class="assetTypeToneClass(asset)"
                :title="imageBusinessTypeLabel(asset)"
              >
                {{ compactImageBusinessTypeLabel(asset) }}
              </span>
              <span
                v-if="!isExternalAsset(asset)"
                class="ac-format-pill ac-format-pill--module"
                :title="assetSourceModuleLabel(asset)"
              >
                {{ assetSourceModuleLabel(asset) }}
              </span>
            </div>
            <div class="ac-card-meta">
              <div class="ac-business-row">
                <span class="ac-business-key">{{ isExternalAsset(asset) ? '文件' : '任务' }}</span>
                <span class="ac-business-value" :class="{ 'ac-mono': !isExternalAsset(asset) }">
                  {{ isExternalAsset(asset) ? fileInfoLabel(asset) : businessTaskNo(asset) }}
                </span>
              </div>
              <div class="ac-business-row">
                <span class="ac-business-key">{{ isExternalAsset(asset) ? '路径' : '文件' }}</span>
                <span class="ac-business-value">{{ isExternalAsset(asset) ? externalOriginPath(asset) : assetFileName(asset) }}</span>
              </div>
              <button
                type="button"
                class="ac-copy-tag"
                @click.stop="copyBusinessKey(asset)"
              >
                {{ isExternalAsset(asset) ? '复制路径' : '复制 SKU' }}
              </button>
            </div>
          </div>
          <div v-if="isExternalAsset(asset)" class="ac-card-footer">
            <div>
              <div class="ac-footer-label">{{ isExternalAsset(asset) ? '准备状态' : '使用状态' }}</div>
              <div
                class="ac-footer-stat ac-footer-stat--operator"
                :class="isExternalAsset(asset) ? 'ac-footer-stat--external' : assetUsableToneClass(asset)"
              >
                {{ isExternalAsset(asset) ? externalAssetStatusLabel(asset) : assetUsableLabel(asset) }}
              </div>
            </div>
            <div class="ac-footer-right">
              <span class="ac-footer-tag">{{ assetFooterLabel(asset) }}</span>
            </div>
          </div>
          <div class="ac-card-actions">
            <button
              v-if="!isExternalAsset(asset)"
              type="button"
              class="ac-card-link-btn ac-card-link-btn--task"
              :disabled="!assetTaskId(asset)"
              @click.stop="openRelatedTask(asset)"
            >
              打开任务
            </button>
            <button
              v-if="assetCanBeReplaced(asset)"
              type="button"
              class="ac-card-link-btn ac-card-link-btn--edit"
              :disabled="replacementUploading"
              @click.stop="startReplaceAsset(asset)"
            >
              {{ replacementUploading && replacementTargetId === assetResourceId(asset) ? '上传中' : '修改资源' }}
            </button>
            <button
              type="button"
              class="ac-card-link-btn"
              @click.stop="openAssetDetail(assetResourceId(asset))"
            >
              资产详情
            </button>
          </div>
        </article>
      </template>
    </main>

    <div v-if="!loading && !error && listTotal > 0" class="ac-pagination">
      <label class="ac-page-size">
        每页
        <select v-model.number="listPageSize" class="ac-page-size-select">
          <option :value="20">20</option>
          <option :value="50">50</option>
          <option :value="100">100</option>
        </select>
        条
      </label>
      <button
        type="button"
        class="ac-pg-btn"
        :disabled="listPage <= 1"
        @click="goListPage(listPage - 1)"
      >
        上一页
      </button>
      <span class="ac-pg-meta">
        第 {{ listPage }} / {{ listTotalPages }} 页（本页 {{ pagedAssets.length }} / 总计 {{ listTotal }}）
      </span>
      <label class="ac-page-jump">
        跳至
        <input
          v-model.number="listJumpPage"
          type="number"
          min="1"
          :max="listTotalPages"
          class="ac-page-jump-input"
          @keyup.enter="jumpListPage"
        />
        页
      </label>
      <button type="button" class="ac-pg-btn" @click="jumpListPage">跳转</button>
      <button
        type="button"
        class="ac-pg-btn"
        :disabled="listPage >= listTotalPages"
        @click="goListPage(listPage + 1)"
      >
        下一页
      </button>
    </div>

    <BaseModal
      v-model="selectedModalOpen"
      title="已选资产"
      :show-confirm="false"
      cancel-text="关闭"
      panel-class="max-w-2xl"
    >
      <BaseEmptyState
        v-if="selectedAssets.length === 0"
        title="暂无已选资产"
        description="请在列表中勾选资产。"
      />
      <div v-else class="ac-selected-list">
        <article v-for="asset in selectedAssets" :key="asset.id" class="ac-selected-item">
          <div class="ac-selected-main">
            <h4 class="ac-selected-title" :title="asset.title">{{ asset.title }}</h4>
            <p class="ac-selected-meta">
              任务号：<span class="cell-mono">{{ asset.taskNo }}</span>
              <span class="ac-selected-divider">|</span>
              SKU：<span class="cell-mono">{{ asset.sku }}</span>
              <span class="ac-selected-divider">|</span>
              类型：{{ asset.kind }}
            </p>
          </div>
          <button
            type="button"
            class="ac-selected-remove"
            @click="removeSelectedAsset(asset.id)"
          >
            取消选择
          </button>
        </article>
      </div>
    </BaseModal>

    <BaseModal
      v-model="bulkSearchModalOpen"
      title="批量搜索下载"
      :show-confirm="false"
      cancel-text="关闭"
      panel-class="max-w-6xl"
    >
      <section class="bulk-search-panel">
        <div class="bulk-search-input-card">
          <label class="bulk-search-label" for="bulk-asset-search-input">SKU / 任务单号</label>
          <textarea
            id="bulk-asset-search-input"
            v-model="bulkSearchInput"
            class="bulk-search-textarea"
            rows="8"
            placeholder="一行一个，例如：&#10;NSKT000261&#10;NSKT000294&#10;RW-20260513-A-000689"
          />
          <div class="bulk-search-filter-grid">
            <BaseSelect
              v-model="bulkSearchFilters.format"
              label="下载格式"
              :options="bulkSearchFormatOptions"
            />
            <BaseSelect
              v-model="bulkSearchFilters.assetKind"
              label="资源类型"
              :options="bulkSearchAssetKindOptions"
            />
          </div>
          <div class="bulk-search-actions">
            <button type="button" class="ac-batch-btn ac-batch-btn--primary" :disabled="bulkSearchRunning" @click="runBulkAssetSearch">
              {{ bulkSearchRunning ? '搜索中...' : '生成下载明细' }}
            </button>
            <button type="button" class="ac-batch-btn" :disabled="bulkSearchDownloading || !bulkSearchMatchedCount" @click="downloadBulkSearchResults">
              {{ bulkSearchDownloading ? '打包中...' : `一键下载 ${bulkSearchMatchedCount} 项` }}
            </button>
            <button type="button" class="ac-batch-btn ac-batch-btn--ghost" :disabled="bulkSearchRunning || bulkSearchDownloading" @click="clearBulkSearch">
              清空
            </button>
          </div>
          <p class="bulk-search-hint">
            支持粘贴多行 SKU 或任务单号，自动去重后按所选格式和资源类型筛选。默认只搜索最终成品图 JPG / PNG；如需源文件、参考图或全部格式请手动切换。
          </p>
          <p v-if="bulkSearchStatus" class="ac-batch-status">{{ bulkSearchStatus }}</p>
          <p v-if="bulkSearchError" class="ac-batch-error">{{ bulkSearchError }}</p>
        </div>

        <div v-if="bulkSearchResults.length" class="bulk-search-summary">
          <span>输入 {{ bulkSearchTermCount }} 项</span>
          <span>命中 {{ bulkSearchMatchedCount }} 项</span>
          <span>未命中 {{ bulkSearchFailedCount }} 项</span>
          <span>格式：{{ bulkSearchFormatFilterLabel }}</span>
          <span>类型：{{ bulkSearchAssetKindFilterLabel }}</span>
        </div>

        <div v-if="bulkSearchResults.length" class="bulk-result-list">
          <article
            v-for="result in bulkSearchResults"
            :key="result.term"
            class="bulk-result-card"
            :class="{ 'bulk-result-card--failed': result.status !== 'matched' }"
          >
            <div class="bulk-result-preview">
              <AssetPreviewMedia
                v-if="result.asset"
                :asset-id="assetResourceId(result.asset)"
                :resolved-preview-url="listCardResolvedPreviewUrl(result.asset)"
                defer-until-visible
                alt=""
                img-class="bulk-result-apm"
                inner-img-class="bulk-result-img"
                @open-full="(u) => result.asset && openAssetPreviewLightbox(result.asset, u, bulkSearchMatchedAssets)"
              />
              <span v-else class="bulk-result-empty">未命中</span>
            </div>
            <div class="bulk-result-main">
              <div class="bulk-result-top">
                <span class="bulk-result-term cell-mono">{{ result.term }}</span>
                <span class="bulk-result-pill">{{ result.status === 'matched' ? '已匹配' : '未匹配' }}</span>
              </div>
              <template v-if="result.asset">
                <h4 class="bulk-result-title" :title="cardTitle(result.asset)">{{ cardTitle(result.asset) }}</h4>
                <dl class="bulk-result-meta">
                  <div>
                    <dt>SKU</dt>
                    <dd class="cell-mono">{{ businessSku(result.asset) }}</dd>
                  </div>
                  <div>
                    <dt>任务号</dt>
                    <dd class="cell-mono">{{ businessTaskNo(result.asset) }}</dd>
                  </div>
                  <div>
                    <dt>资源类型</dt>
                    <dd>{{ assetKind(result.asset) }}</dd>
                  </div>
                  <div>
                    <dt>文件格式</dt>
                    <dd>{{ fileFormatLabel(result.asset) }}</dd>
                  </div>
                  <div>
                    <dt>创建运营</dt>
                    <dd>{{ taskCreatorLabel(result.asset) }}</dd>
                  </div>
                </dl>
              </template>
              <p v-else class="bulk-result-message">{{ result.message }}</p>
              <p v-if="result.asset && result.candidates > 1" class="bulk-result-message">
                共找到 {{ result.candidates }} 个符合筛选的候选资源，已按当前规则选择最新匹配项。
              </p>
            </div>
          </article>
        </div>
      </section>
    </BaseModal>

    <BaseModal
      v-model="detailModalOpen"
      title="资产详情"
      :show-confirm="false"
      cancel-text="关闭"
      panel-class="max-w-4xl"
    >
      <div v-if="detailLoading" class="state-text">详情加载中…</div>
      <div v-else-if="detailError" class="state-text state-error">{{ detailError }}</div>
      <BaseEmptyState
        v-else-if="!selectedAsset"
        title="未选择资产"
        description="请从列表中选择一条资产。"
      />
      <template v-else>
        <section class="preview-panel">
          <h4 class="subsection-title">预览内容</h4>
          <div class="preview-media-shell">
            <AssetPreviewMedia
              :asset-id="selectedAssetIdForPreview"
              :fallback-src="selectedPreviewFallbackUrl"
              :resolved-preview-url="selectedPreviewFallbackUrl"
              alt="资产预览"
              inner-img-class="preview-media-img"
              @open-full="(u) => selectedAsset && openAssetPreviewLightbox(selectedAsset, u, pagedAssets)"
            />
          </div>
          <div class="preview-actions">
            <AssetDownloadLink
              v-if="selectedAssetIdForPreview || selectedPreviewFallbackUrl"
              variant="button"
              :asset-id="selectedAssetIdForPreview"
              :href="selectedPreviewFallbackUrl"
            >
              下载文件
            </AssetDownloadLink>
            <span class="preview-state-hint">预览状态：{{ previewStateLabel }}</span>
          </div>
        </section>

        <dl class="detail-grid">
          <div class="detail-row">
            <dt>资源来源</dt>
            <dd>{{ assetSourceLabel(selectedAsset) }}</dd>
          </div>
          <div class="detail-row">
            <dt>{{ isExternalAsset(selectedAsset) ? '文件名' : 'SKU' }}</dt>
            <dd :class="{ 'cell-mono': !isExternalAsset(selectedAsset) }">
              {{ isExternalAsset(selectedAsset) ? assetFileName(selectedAsset) : businessSku(selectedAsset) }}
            </dd>
          </div>
          <div v-if="!isExternalAsset(selectedAsset)" class="detail-row">
            <dt>所属任务号</dt>
            <dd class="cell-mono">{{ businessTaskNo(selectedAsset) }}</dd>
          </div>
          <div v-if="!isExternalAsset(selectedAsset)" class="detail-row">
            <dt>任务创建运营</dt>
            <dd>{{ taskCreatorLabel(selectedAsset) }}</dd>
          </div>
          <div v-if="!isExternalAsset(selectedAsset)" class="detail-row">
            <dt>使用状态</dt>
            <dd>
              <span class="detail-state-pill" :class="assetUsableToneClass(selectedAsset)">
                {{ assetUsableLabel(selectedAsset) }}
              </span>
            </dd>
          </div>
          <div v-if="!isExternalAsset(selectedAsset) && selectedAsset.cleanup_after_at" class="detail-row">
            <dt>旧版清理时间</dt>
            <dd>{{ displayTime(selectedAsset.cleanup_after_at) }}</dd>
          </div>
          <div class="detail-row">
            <dt>文件类型</dt>
            <dd>{{ imageBusinessTypeLabel(selectedAsset) }}</dd>
          </div>
          <div v-if="!isExternalAsset(selectedAsset)" class="detail-row">
            <dt>产生环节</dt>
            <dd>{{ assetSourceModuleLabel(selectedAsset) }}</dd>
          </div>
          <div v-if="!isExternalAsset(selectedAsset)" class="detail-row">
            <dt>上传时间</dt>
            <dd>{{ displayTime(selectedAsset.uploaded_at ?? selectedAsset.created_at) }}</dd>
          </div>
          <div v-if="!isExternalAsset(selectedAsset)" class="detail-row">
            <dt>任务创建时间</dt>
            <dd>{{ displayTime(selectedAsset.task_created_at) }}</dd>
          </div>
          <div class="detail-row">
            <dt>文件名</dt>
            <dd>{{ assetFileName(selectedAsset) }}</dd>
          </div>
          <div v-if="!isExternalAsset(selectedAsset)" class="detail-row">
            <dt>上传状态</dt>
            <dd>{{ assetUploadStatus(selectedAsset.upload_status) }}</dd>
          </div>
          <div v-if="!isExternalAsset(selectedAsset)" class="detail-row">
            <dt>归档状态</dt>
            <dd>{{ assetArchiveStatus(selectedAsset.archive_status) }}</dd>
          </div>
          <div class="detail-row">
            <dt>{{ isExternalAsset(selectedAsset) ? '资源编号' : '系统资产号' }}</dt>
            <dd class="cell-mono">{{ displayText(assetResourceId(selectedAsset)) }}</dd>
          </div>
          <div v-if="isExternalAsset(selectedAsset)" class="detail-row">
            <dt>外部资源状态</dt>
            <dd>{{ externalAssetStatusLabel(selectedAsset) }}</dd>
          </div>
          <div v-if="isExternalAsset(selectedAsset)" class="detail-row detail-row-full">
            <dt>外部路径</dt>
            <dd>{{ externalOriginPath(selectedAsset) }}</dd>
          </div>
          <div class="detail-row">
            <dt>下载模式</dt>
            <dd>{{ assetDownloadMode(downloadMeta?.download_mode) }}</dd>
          </div>
          <div class="detail-row">
            <dt>预览可用</dt>
            <dd>{{ previewStateLabel }}</dd>
          </div>
        </dl>
        <div class="detail-business-actions">
          <button
            v-if="!isExternalAsset(selectedAsset)"
            type="button"
            class="ac-card-link-btn ac-card-link-btn--task"
            :disabled="!assetTaskId(selectedAsset)"
            @click="openRelatedTask(selectedAsset)"
          >
            打开对应任务
          </button>
          <button
            v-if="assetCanBeReplaced(selectedAsset)"
            type="button"
            class="ac-card-link-btn ac-card-link-btn--edit"
            :disabled="replacementUploading"
            @click="startReplaceAsset(selectedAsset)"
          >
            {{ replacementUploading && replacementTargetId === assetResourceId(selectedAsset) ? '上传中' : '修改资源' }}
          </button>
        </div>

        <div class="versions-section">
          <h4 class="subsection-title">版本记录</h4>
          <BaseEmptyState
            v-if="!selectedVersions.length"
            title="暂无版本"
            description="该资产当前未返回嵌套版本。"
          />
          <div v-else class="version-list">
            <article v-for="version in selectedVersions" :key="version.id" class="version-card">
              <div class="version-top">
                <span class="version-title">版本 {{ displayText(version.version ?? version.id) }}</span>
                <span class="version-pill">{{ assetKind(version.file_role) }}</span>
              </div>
              <dl class="version-grid">
                <div class="detail-row">
                  <dt>文件名</dt>
                  <dd>{{ displayText(version.file_name) }}</dd>
                </div>
                <div class="detail-row">
                  <dt>MIME</dt>
                  <dd>{{ displayText(version.mime_type) }}</dd>
                </div>
                <div class="detail-row">
                  <dt>下载模式</dt>
                  <dd>{{ assetDownloadMode(version.download_mode) }}</dd>
                </div>
                <div class="detail-row">
                  <dt>可预览</dt>
                  <dd>{{ version.preview_available === true ? '是' : version.preview_available === false ? '否' : '—' }}</dd>
                </div>
                <div class="detail-row">
                  <dt>创建时间</dt>
                  <dd>{{ displayTime(version.created_at) }}</dd>
                </div>
                <div class="detail-row">
                  <dt>使用状态</dt>
                  <dd>
                    <span class="detail-state-pill" :class="versionUsableToneClass(version)">
                      {{ versionUsableLabel(version) }}
                    </span>
                  </dd>
                </div>
                <div v-if="version.cleanup_after_at" class="detail-row">
                  <dt>清理时间</dt>
                  <dd>{{ displayTime(version.cleanup_after_at) }}</dd>
                </div>
              </dl>
            </article>
          </div>
        </div>
      </template>
    </BaseModal>

    <ImagePreviewLightbox
      v-model="previewLightboxOpen"
      :items="previewLightboxItems"
      :initial-index="previewLightboxInitialIndex"
      fallback-title="资产预览"
      aria-label="资产图片预览"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import BaseEmptyState from '@/components/base/BaseEmptyState.vue'
import BaseInput from '@/components/base/BaseInput.vue'
import BaseModal from '@/components/base/BaseModal.vue'
import BaseSelect, { type BaseSelectOption } from '@/components/base/BaseSelect.vue'
import AssetPreviewMedia from '@/components/media/AssetPreviewMedia.vue'
import AssetDownloadLink from '@/components/media/AssetDownloadLink.vue'
import ImagePreviewLightbox from '@/components/media/ImagePreviewLightbox.vue'
import type { ImagePreviewLightboxItem } from '@/components/media/imagePreviewLightbox'
import { usePermission } from '@/composables/usePermission'
import {
  assetArchiveStatusLabelCn,
  assetDownloadModeLabelCn,
  assetUploadStatusLabelCn,
} from '@/domain/mappers/read-model-labels-cn'
import PredictionFeedbackControl from '@/components/experience/PredictionFeedbackControl.vue'
import {
  assetsApi,
  type AssetBatchDownloadFailure,
  type AssetBatchDownloadItem,
  type AssetExcelPackageFailure,
  type AssetExcelPackageItem,
  type AssetExcelPackageRow,
  type AssetKind,
} from '@/services/api/assetsApi'
import { predictionsApi, type PredictionSuggestion } from '@/services/api/predictionsApi'
import { recordExperienceBehavior } from '@/services/experienceBehavior'
import type { AssetResourceSource, BackendAsset, BackendAssetVersion } from '@/services/apiTypes'
import { formatDateTimeBeijing } from '@/utils/date'
import { resolveApiUserMessage } from '@/utils/api-message-zh'
import { userAccountDisplay } from '@/domain/user-display'
import { formatFileSizeBytes } from '@/domain/formatters/file-size'
import { uploadTaskFileViaAssetSession } from '@/services/upload/assetUploadFlow'
import {
  buildTimestampedZipFilename,
  downloadBatchAsZip,
  mapWithConcurrency,
  sanitizeZipEntryName,
} from '@/utils/batchZipDownload'

const route = useRoute()
const router = useRouter()
const { canAccessPage } = usePermission()
const loading = ref(false)
const error = ref('')
const assets = ref<BackendAsset[]>([])
const selectedAssetId = ref('')
const detailLoading = ref(false)
const detailError = ref('')
const selectedAssetDetail = ref<BackendAsset | null>(null)
const downloadMeta = ref<Record<string, unknown> | null>(null)
const previewMeta = ref<Record<string, unknown> | null>(null)
const previewUnavailable = ref(false)
const previewNotFound = ref(false)
const detailModalOpen = ref(false)
const listPage = ref(1)
const listJumpPage = ref(1)
const listPageSize = ref(20)
const listTotal = ref(0)
const AUTO_RELOAD_DELAY_MS = 400
const MAX_BATCH_DOWNLOAD_ASSETS = 100
const MAX_BULK_SEARCH_TERMS = 200
let reloadTimer: ReturnType<typeof setTimeout> | null = null
let reloadAbort: AbortController | null = null
let reloadRequestSeq = 0
const previewLightboxSrc = ref<string | null>(null)
const previewLightboxTitle = ref('')
const previewLightboxItems = ref<ImagePreviewLightboxItem[]>([])
const previewLightboxInitialIndex = ref(0)
const previewLightboxOpen = computed({
  get: () => Boolean(previewLightboxSrc.value),
  set: (open) => {
    if (!open) closePreviewLightbox()
  },
})
const filtersExpanded = ref(false)
const copyHint = ref('')
const selectedModalOpen = ref(false)
const batchDownloading = ref(false)
const batchDownloadStatus = ref('')
const batchDownloadError = ref('')
const excelFileInput = ref<HTMLInputElement | null>(null)
const excelPackaging = ref(false)
const excelPackageStatus = ref('')
const excelPackageError = ref('')
const replacementFileInput = ref<HTMLInputElement | null>(null)
const replacementTargetAsset = ref<BackendAsset | null>(null)
const replacementUploading = ref(false)
const replacementStatus = ref('')
const replacementError = ref('')
const bulkSearchModalOpen = ref(false)
const bulkSearchInput = ref('')
const bulkSearchRunning = ref(false)
const bulkSearchDownloading = ref(false)
const bulkSearchStatus = ref('')
const bulkSearchError = ref('')
const assetPredictionSuggestions = ref<PredictionSuggestion[]>([])
let assetPredictionAbort: AbortController | null = null

type AssetTaskLaneFilter = 'all' | 'normal' | 'customization'
type AssetKindFilter = 'all' | 'delivery' | 'reference' | 'source'
type AssetFormatFilter = 'all' | 'image' | 'design' | 'pdf' | 'video' | 'archive'
type AssetTimeBasisFilter = 'asset_uploaded_at' | 'task_created_at'
type AssetSourceModuleFilter = 'all' | 'basic_info' | 'design' | 'audit' | 'customization' | 'retouch'

const ASSET_CENTER_FILE_TYPE_LABELS: Record<string, string> = {
  delivery: '成品图',
  reference: '参考图',
  source: '源文件',
  preview: '预览图',
  design_thumb: '预览图',
}

type BulkSearchFormatFilter =
  | 'jpg_png'
  | 'jpg'
  | 'png'
  | 'webp'
  | 'image'
  | 'design'
  | 'pdf'
  | 'archive'
  | 'all'
type BulkSearchAssetKindFilter =
  | 'auto'
  | 'all'
  | 'delivery'
  | 'reference'
  | 'source'
  | 'preview'
  | 'other'

const filters = reactive({
  keyword: '',
  resourceSource: 'all' as AssetResourceSource,
  timeBasis: 'asset_uploaded_at' as AssetTimeBasisFilter,
  createdFrom: '',
  createdTo: '',
  taskLane: 'all' as AssetTaskLaneFilter,
  assetKind: 'all' as AssetKindFilter,
  moduleKey: 'all' as AssetSourceModuleFilter,
  formatCategory: 'all' as AssetFormatFilter,
})

const bulkSearchFilters = reactive({
  format: 'jpg_png' as BulkSearchFormatFilter,
  assetKind: 'delivery' as BulkSearchAssetKindFilter,
})

const assetSourceOptions: BaseSelectOption[] = [
  { value: 'all', label: '全部资源' },
  { value: 'system', label: '系统资源' },
  { value: 'external', label: '外部资源' },
]

const assetTimeBasisOptions: BaseSelectOption[] = [
  { value: 'asset_uploaded_at', label: '文件上传时间' },
  { value: 'task_created_at', label: '任务创建时间' },
]

const assetTaskLaneOptions: BaseSelectOption[] = [
  { value: 'all', label: '全部任务' },
  { value: 'normal', label: '常规任务' },
  { value: 'customization', label: '定制任务' },
]

const assetKindFilterOptions: BaseSelectOption[] = [
  { value: 'all', label: '全部文件类型' },
  { value: 'delivery', label: '成品图' },
  { value: 'reference', label: '参考图' },
  { value: 'source', label: '源文件' },
]

const assetSourceModuleOptions: BaseSelectOption[] = [
  { value: 'all', label: '全部产生环节' },
  { value: 'basic_info', label: '基础信息参考' },
  { value: 'design', label: '设计提交' },
  { value: 'audit', label: '常规审核修订' },
  { value: 'customization', label: '定制链路上传' },
  { value: 'retouch', label: '精修需求素材' },
]

const assetFormatCategoryOptions: BaseSelectOption[] = [
  { value: 'all', label: '全部常用格式' },
  { value: 'image', label: '图片' },
  { value: 'design', label: '设计源文件' },
  { value: 'pdf', label: 'PDF' },
  { value: 'video', label: '视频' },
  { value: 'archive', label: '压缩包' },
]

const bulkSearchFormatOptions: BaseSelectOption[] = [
  { value: 'jpg_png', label: 'JPG / PNG' },
  { value: 'image', label: '全部图片' },
  { value: 'jpg', label: '仅 JPG' },
  { value: 'png', label: '仅 PNG' },
  { value: 'webp', label: '仅 WEBP' },
  { value: 'design', label: '设计源文件' },
  { value: 'pdf', label: 'PDF' },
  { value: 'archive', label: '压缩包' },
  { value: 'all', label: '全部格式' },
]

const bulkSearchAssetKindOptions: BaseSelectOption[] = [
  { value: 'delivery', label: '最终成品图' },
  { value: 'auto', label: '自动匹配（成品图优先）' },
  { value: 'reference', label: '参考图' },
  { value: 'source', label: '源文件 / 修订源文件' },
  { value: 'preview', label: '预览辅助图' },
  { value: 'other', label: '其他类型' },
  { value: 'all', label: '全部类型' },
]

const bulkSearchFormatFilterLabel = computed(() =>
  bulkSearchFormatOptions.find((item) => item.value === bulkSearchFilters.format)?.label ?? '全部格式',
)

const bulkSearchAssetKindFilterLabel = computed(() =>
  bulkSearchAssetKindOptions.find((item) => item.value === bulkSearchFilters.assetKind)?.label ?? '全部类型',
)

const effectiveAssetSearchSource = computed<AssetResourceSource>(() => filters.resourceSource)

const requestedTaskId = computed(() => {
  const raw = route.query.task_id
  return typeof raw === 'string' ? raw.trim() : ''
})

const requestedAssetId = computed(() => {
  const raw = route.query.asset_id
  return typeof raw === 'string' ? raw.trim() : ''
})

const selectedAsset = computed(
  () =>
    selectedAssetDetail.value ??
    assets.value.find((item) => assetResourceId(item) === selectedAssetId.value) ??
    null,
)

const selectedVersions = computed<BackendAssetVersion[]>(() => selectedAsset.value?.versions ?? [])

interface SelectedAssetSummary {
  id: string
  taskId: string
  taskNo: string
  sku: string
  productName: string
  title: string
  kind: string
}

type BulkSearchResultStatus = 'matched' | 'not_found' | 'error'

interface BulkSearchResult {
  term: string
  status: BulkSearchResultStatus
  message: string
  candidates: number
  asset?: BackendAsset
}

const selectedAssetMap = reactive(new Map<string, SelectedAssetSummary>())
const selectedCount = computed(() => selectedAssetMap.size)
const selectedAssets = computed(() => Array.from(selectedAssetMap.values()))
const canBatchDownload = computed(
  () => selectedCount.value > 0 && selectedCount.value <= MAX_BATCH_DOWNLOAD_ASSETS && !batchDownloading.value,
)
const replacementTargetId = computed(() =>
  replacementTargetAsset.value ? assetResourceId(replacementTargetAsset.value) : '',
)

const EXCEL_PACKAGE_CONCURRENCY = 4
const bulkSearchResults = ref<BulkSearchResult[]>([])
let bulkSearchRunSeq = 0
let bulkSearchAbort: AbortController | null = null
const bulkSearchTermCache = new Map<string, BulkSearchResult>()
const bulkSearchTermCount = computed(() => parseBulkSearchTerms(bulkSearchInput.value).length)
const bulkSearchMatchedResults = computed(() => bulkSearchResults.value.filter((item) => item.status === 'matched' && item.asset))
const bulkSearchMatchedAssets = computed(() => bulkSearchMatchedResults.value.map((item) => item.asset!).filter(Boolean))
const bulkSearchMatchedCount = computed(() => bulkSearchMatchedResults.value.length)
const bulkSearchFailedCount = computed(() => bulkSearchResults.value.filter((item) => item.status !== 'matched').length)

const effectiveSearchKeyword = computed(() => filters.keyword.trim())
const effectiveTaskLaneFilter = computed<AssetTaskLaneFilter>(() =>
  filters.resourceSource === 'external' ? 'all' : filters.taskLane,
)
const effectiveAssetKindFilter = computed<AssetKindFilter>(() =>
  filters.resourceSource === 'external' ? 'all' : filters.assetKind,
)
const effectiveSourceModuleFilter = computed<AssetSourceModuleFilter>(() =>
  filters.resourceSource === 'external' ? 'all' : filters.moduleKey,
)

const listTotalPages = computed(() =>
  Math.max(1, Math.ceil(listTotal.value / listPageSize.value)),
)

const pagedAssets = computed(() => assets.value)

watch(listTotalPages, (tp) => {
  if (listPage.value > tp) listPage.value = tp
})

watch(
  () => [
    filters.keyword,
    filters.resourceSource,
    filters.timeBasis,
    filters.createdFrom,
    filters.createdTo,
    filters.taskLane,
    filters.assetKind,
    filters.moduleKey,
    filters.formatCategory,
  ],
  () => {
    listPage.value = 1
    scheduleReload()
  },
)

watch(
  () => filters.resourceSource,
  (source) => {
    if (source === 'external') {
      if (filters.timeBasis !== 'asset_uploaded_at') filters.timeBasis = 'asset_uploaded_at'
      if (filters.taskLane !== 'all') filters.taskLane = 'all'
      if (filters.assetKind !== 'all') filters.assetKind = 'all'
      if (filters.moduleKey !== 'all') filters.moduleKey = 'all'
    }
  },
)

watch(
  () => [bulkSearchFilters.format, bulkSearchFilters.assetKind],
  () => {
    bulkSearchTermCache.clear()
    if (!bulkSearchResults.value.length && !bulkSearchStatus.value && !bulkSearchError.value) return
    bulkSearchResults.value = []
    bulkSearchError.value = ''
    bulkSearchStatus.value = '筛选条件已更新，请重新生成下载明细'
  },
)

watch(listPageSize, () => {
  listPage.value = 1
  scheduleReload()
})

watch(listPage, (p) => {
  listJumpPage.value = p
  scheduleReload()
})

const previewStateLabel = computed(() => {
  if (previewUnavailable.value) return '当前不可预览（仅可下载，非不存在）'
  if (previewNotFound.value) return '预览资源不存在（404）'
  if (previewMeta.value?.preview_available === true) return '可预览'
  if (previewMeta.value?.preview_available === false) return '不可预览'
  if (previewMeta.value?.download_url) return '可预览'
  return '—'
})

const selectedAssetIdForPreview = computed(() => {
  if (previewMeta.value?.download_url || previewUnavailable.value || previewNotFound.value) {
    return undefined
  }
  const id = String(selectedAsset.value?.id ?? '').trim()
  return selectedAsset.value ? assetResourceId(selectedAsset.value) : id || undefined
})

const selectedPreviewFallbackUrl = computed(() => {
  const url = String(previewMeta.value?.download_url ?? '').trim()
  return url || undefined
})

function rawAssetSourceType(asset: BackendAsset | null | undefined): string {
  const r = asset as Record<string, unknown> | null | undefined
  return String(r?.source_type ?? r?.sourceType ?? '').trim().toLowerCase()
}

function isExternalAsset(asset: BackendAsset | null | undefined): boolean {
  if (!asset) return false
  const r = asset as Record<string, unknown>
  const resourceID = String(r.resource_id ?? r.resourceId ?? '').trim()
  return rawAssetSourceType(asset) === 'external' || resourceID.startsWith('ext-')
}

function assetResourceId(asset: BackendAsset): string {
  const r = asset as Record<string, unknown>
  const resourceID = String(r.resource_id ?? r.resourceId ?? '').trim()
  if (resourceID) return resourceID
  const id = String(asset.id ?? '').trim()
  return isExternalAsset(asset) && id && !id.startsWith('ext-') ? `ext-${id}` : id
}

function assetSourceLabel(asset: BackendAsset | null | undefined): string {
  const r = asset as Record<string, unknown> | null | undefined
  const label = String(r?.source_label ?? r?.sourceLabel ?? '').trim()
  if (label) return label
  return isExternalAsset(asset) ? '外部资源' : '系统资源'
}

function externalOriginPath(asset: BackendAsset): string {
  const r = asset as Record<string, unknown>
  const path = String(r.origin_path ?? r.originPath ?? r.product_name ?? '').trim()
  return path || assetFileName(asset)
}

function assetDisplayTitle(asset: BackendAsset): string {
  return isExternalAsset(asset) ? assetFileName(asset) : businessSku(asset)
}

function previewTitleForAsset(asset: BackendAsset | null | undefined): string {
  if (!asset) return '资产预览'
  const sku = businessSku(asset)
  const name = assetFileName(asset)
  if (isExternalAsset(asset)) return name || '外部资源预览'
  return sku && sku !== '未绑定 SKU' ? `${sku} · ${name}` : name || '资产预览'
}

function normalizePreviewLightboxItems(
  url: string,
  title: string,
  items: ImagePreviewLightboxItem[] | undefined,
): ImagePreviewLightboxItem[] {
  const normalized = (items ?? [])
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
  return url ? [{ src: url, title, alt: title, downloadUrl: url }] : []
}

function openPreviewLightbox(
  url: string,
  title = '资产预览',
  items?: ImagePreviewLightboxItem[],
  index?: number,
): void {
  const trimmedUrl = url.trim()
  const trimmedTitle = title.trim() || '资产预览'
  const normalizedItems = normalizePreviewLightboxItems(trimmedUrl, trimmedTitle, items)
  if (!trimmedUrl && normalizedItems.length === 0) return
  const fallbackIndex = normalizedItems.findIndex((item) => item.src === trimmedUrl)
  previewLightboxSrc.value = trimmedUrl
  previewLightboxTitle.value = trimmedTitle
  previewLightboxItems.value = normalizedItems
  previewLightboxInitialIndex.value = Math.max(0, typeof index === 'number' ? index : fallbackIndex)
}

function closePreviewLightbox(): void {
  previewLightboxSrc.value = null
  previewLightboxTitle.value = ''
  previewLightboxItems.value = []
  previewLightboxInitialIndex.value = 0
}

function assetPreviewLightboxItem(asset: BackendAsset, resolvedUrl?: string): ImagePreviewLightboxItem | null {
  const src = String(resolvedUrl || listCardResolvedPreviewUrl(asset) || '').trim()
  const previewAssetId = assetResourceId(asset)
  if (!src && !previewAssetId) return null
  const title = previewTitleForAsset(asset)
  return {
    src,
    previewAssetId,
    resolvedPreviewUrl: src || undefined,
    fallbackSrc: src || undefined,
    title,
    alt: title,
    preferredFilename: title,
    downloadUrl: src,
  }
}

function assetPreviewGallery(
  assetsSource: BackendAsset[],
  activeAsset: BackendAsset,
  activeUrl: string,
): { items: ImagePreviewLightboxItem[]; index: number } {
  const items: ImagePreviewLightboxItem[] = []
  let activeIndex = -1
  const activeId = assetResourceId(activeAsset)
  for (const asset of assetsSource) {
    const item = assetPreviewLightboxItem(asset, assetResourceId(asset) === activeId ? activeUrl : undefined)
    if (!item) continue
    if (assetResourceId(asset) === activeId) activeIndex = items.length
    items.push(item)
  }
  if (activeIndex < 0) {
    const active = assetPreviewLightboxItem(activeAsset, activeUrl)
    if (active) {
      activeIndex = 0
      items.unshift(active)
    }
  }
  return { items, index: Math.max(0, activeIndex) }
}

function openAssetPreviewLightbox(asset: BackendAsset, url: string, assetsSource: BackendAsset[]): void {
  const title = previewTitleForAsset(asset)
  const gallery = assetPreviewGallery(assetsSource, asset, url)
  openPreviewLightbox(url, title, gallery.items, gallery.index)
}

function numericFileSize(asset: BackendAsset): number {
  const r = asset as Record<string, unknown>
  const raw = r.file_size ?? r.fileSize
  const n = typeof raw === 'number' ? raw : Number(raw)
  return Number.isFinite(n) && n > 0 ? n : 0
}

function fileInfoLabel(asset: BackendAsset): string {
  const parts = [fileFormatLabel(asset)]
  const size = numericFileSize(asset)
  if (size > 0) parts.push(formatFileSizeBytes(size))
  return parts.join(' · ')
}

function assetFooterLabel(asset: BackendAsset): string {
  return isExternalAsset(asset) ? assetSourceLabel(asset) : assetProductLabel(asset)
}

function externalAssetStatusLabel(asset: BackendAsset): string {
  const r = asset as Record<string, unknown>
  const oss = String(r.oss_sync_status ?? r.ossSyncStatus ?? '').trim()
  const preview = String(r.external_preview_status ?? r.externalPreviewStatus ?? '').trim()
  const hasDisplayUrl = ['download_url', 'downloadUrl', 'preview_url', 'previewUrl'].some((key) => {
    const v = r[key]
    return typeof v === 'string' && v.trim().length > 5
  })
  const canPreview = r.preview_available === true || r.previewAvailable === true || hasDisplayUrl
  if (preview === 'ready' || canPreview) return '可预览'
  if (oss === 'ready') return '可下载'
  if (preview === 'pending') return '正在准备预览'
  if (oss === 'pending') return '正在准备下载'
  if (preview === 'failed' || oss === 'failed') return '外部资源暂时不可用'
  return '按需准备'
}

function rawUsableState(row: Record<string, unknown>): string {
  const state = String(row.usable_state ?? row.usableState ?? '').trim()
  if (state) return state
  const flow = String(row.flow_review_status ?? row.flowReviewStatus ?? '').trim()
  if (flow === 'approved') return 'ready_for_use'
  if (flow === 'pending_review') return 'pending_review'
  if (flow === 'rejected') return 'rejected'
  if (flow === 'superseded') return 'history'
  if (flow === 'cleaned') return 'cleaned'
  return 'not_applicable'
}

function usableLabelFromState(state: string): string {
  if (state === 'ready_for_use') return '可直接使用'
  if (state === 'pending_review') return '待审核'
  if (state === 'rejected') return '审核未通过'
  if (state === 'history') return '历史版本'
  if (state === 'cleaned') return '文件已清理'
  return '不进入审核流'
}

function assetUsableLabel(asset: BackendAsset): string {
  const r = asset as Record<string, unknown>
  const label = String(r.usable_label ?? r.usableLabel ?? '').trim()
  return label || usableLabelFromState(rawUsableState(r))
}

function versionUsableLabel(version: BackendAssetVersion): string {
  const r = version as Record<string, unknown>
  const label = String(r.usable_label ?? r.usableLabel ?? '').trim()
  return label || usableLabelFromState(rawUsableState(r))
}

function usableToneClass(row: Record<string, unknown>): string {
  const state = rawUsableState(row)
  if (state === 'ready_for_use') return 'ac-usability--ready'
  if (state === 'pending_review') return 'ac-usability--pending'
  if (state === 'rejected') return 'ac-usability--rejected'
  if (state === 'history') return 'ac-usability--history'
  if (state === 'cleaned') return 'ac-usability--cleaned'
  return 'ac-usability--neutral'
}

function assetUsableToneClass(asset: BackendAsset): string {
  return usableToneClass(asset as Record<string, unknown>)
}

function versionUsableToneClass(version: BackendAssetVersion): string {
  return usableToneClass(version as Record<string, unknown>)
}

function rawAssetKind(asset: BackendAsset | null | undefined): string {
  if (!asset) return ''
  const record = asset as Record<string, unknown>
  return String(record.asset_kind ?? record.asset_type ?? asset.file_role ?? '').trim().toLowerCase()
}

function assetScopeSkuCode(asset: BackendAsset | null | undefined): string {
  if (!asset) return ''
  const record = asset as Record<string, unknown>
  for (const key of ['scope_sku_code', 'sku_code', 'primary_sku_code', 'target_sku_code'] as const) {
    const value = record[key]
    if (typeof value === 'string' && value.trim()) return value.trim()
  }
  return ''
}

function assetCanBeReplaced(asset: BackendAsset | null | undefined): boolean {
  if (!asset || isExternalAsset(asset)) return false
  if (!assetTaskId(asset) || !positiveID(assetResourceId(asset))) return false
  const kind = rawAssetKind(asset)
  if (kind !== 'delivery' && kind !== 'source' && kind !== 'reference') return false
  const state = rawUsableState(asset as Record<string, unknown>)
  return state !== 'history' && state !== 'cleaned'
}

function cardTitle(asset: BackendAsset): string {
  const r = asset as Record<string, unknown>
  const fn = r.file_name
  if (typeof fn === 'string' && fn.trim()) return fn.trim()
  return `${assetKind(asset)} #${asset.id}`
}

function assetFileName(asset: BackendAsset): string {
  const r = asset as Record<string, unknown>
  for (const key of ['file_name', 'original_filename', 'filename'] as const) {
    const value = r[key]
    if (typeof value === 'string' && value.trim()) return value.trim()
  }
  return cardTitle(asset)
}

function assetSourceModuleKey(asset: BackendAsset | null | undefined): string {
  if (!asset) return ''
  const r = asset as Record<string, unknown>
  return String(r.source_module_key ?? r.module_key ?? '').trim().toLowerCase()
}

function assetSourceModuleLabel(asset: BackendAsset | null | undefined): string {
  if (!asset) return '—'
  if (isExternalAsset(asset)) return '外部资源'
  switch (assetSourceModuleKey(asset)) {
    case 'basic_info':
      return '基础信息参考'
    case 'design':
      return '设计提交'
    case 'audit':
      return '常规审核修订'
    case 'customization':
      return '定制链路上传'
    case 'retouch':
      return '精修需求素材'
    case 'warehouse':
      return '仓库处理'
    case 'procurement':
      return '采购资料'
    default:
      return '历史资产'
  }
}

function businessSku(asset: BackendAsset): string {
  const r = asset as Record<string, unknown>
  for (const key of ['scope_sku_code', 'sku_code', 'primary_sku_code', 'target_sku_code'] as const) {
    const value = r[key]
    if (typeof value === 'string' && value.trim()) return value.trim()
  }
  return '未绑定 SKU'
}

function businessTaskNo(asset: BackendAsset): string {
  const r = asset as Record<string, unknown>
  for (const key of ['task_no', 'taskNo'] as const) {
    const value = r[key]
    if (typeof value === 'string' && value.trim()) return value.trim()
  }
  const id = positiveID(asset.task_id)
  return id ? `任务 ${id}` : '未绑定任务'
}

function assetTaskId(asset: BackendAsset | null | undefined): string {
  return positiveID(asset?.task_id)
}

function taskCreatorLabel(asset: BackendAsset): string {
  const r = asset as Record<string, unknown>
  return userAccountDisplay(
    r.task_creator_username,
    r.task_creator_name,
    r.creator_username,
    r.creator_name,
    r.created_by_username,
    r.created_by_name,
  )
}

function assetProductLabel(asset: BackendAsset): string {
  const r = asset as Record<string, unknown>
  const product = String(r.product_name ?? r.product_name_snapshot ?? '').trim()
  if (product) return product
  return assetKind(asset)
}

function imageBusinessTypeLabel(asset: BackendAsset): string {
  return `${assetKind(asset)} / ${fileFormatLabel(asset)}`
}

function compactImageBusinessTypeLabel(asset: BackendAsset): string {
  const record = asset as Record<string, unknown>
  const rawKind = String(record.asset_kind ?? record.asset_type ?? asset.file_role ?? '').trim().toLowerCase()
  const kind =
    rawKind === 'source'
      ? '源文件'
      : rawKind === 'delivery'
      ? '成品图'
      : rawKind === 'reference'
      ? '参考图'
      : rawKind === 'preview' || rawKind === 'design_thumb'
      ? '预览图'
      : assetKind(asset)
  return `${kind} / ${fileFormatLabel(asset)}`
}

function assetTypeToneClass(asset: BackendAsset): string {
  const record = asset as Record<string, unknown>
  const rawKind = String(record.asset_kind ?? record.asset_type ?? asset.file_role ?? '').trim().toLowerCase()
  if (rawKind === 'delivery') return 'ac-format-pill--delivery'
  if (rawKind === 'source') return 'ac-format-pill--source'
  if (rawKind === 'reference') return 'ac-format-pill--reference'
  if (rawKind === 'preview' || rawKind === 'design_thumb') return 'ac-format-pill--preview'
  return 'ac-format-pill--other'
}

function fileFormatLabel(asset: BackendAsset): string {
  const title = assetFileName(asset)
  const match = /\.([a-z0-9]{2,8})(?:$|[?#])/i.exec(title)
  if (match?.[1]) return match[1].toUpperCase()

  const r = asset as Record<string, unknown>
  const mime = r.mime_type
  if (typeof mime === 'string' && mime.includes('/')) {
    const subtype = mime.split('/').pop()?.split(/[;+]/)[0]?.trim()
    if (subtype) return subtype.toUpperCase().replace('JPEG', 'JPG')
  }
  return '文件'
}

function toSelectedAssetSummary(asset: BackendAsset): SelectedAssetSummary {
  const id = assetResourceId(asset)
  return {
    id,
    taskId: displayText(asset.task_id),
    taskNo: businessTaskNo(asset),
    sku: isExternalAsset(asset) ? '外部资源' : businessSku(asset),
    productName: isExternalAsset(asset) ? assetFileName(asset) : assetProductLabel(asset),
    title: isExternalAsset(asset) ? assetFileName(asset) : `${businessSku(asset)} · ${assetFileName(asset)}`,
    kind: imageBusinessTypeLabel(asset),
  }
}

function isAssetSelected(asset: BackendAsset): boolean {
  return selectedAssetMap.has(assetResourceId(asset))
}

function toggleAssetSelection(asset: BackendAsset, checked?: boolean) {
  if (isExternalAsset(asset)) {
    batchDownloadError.value = '外部资源请在卡片上单个下载'
    return
  }
  const id = assetResourceId(asset)
  const nextChecked = typeof checked === 'boolean' ? checked : !selectedAssetMap.has(id)
  if (!nextChecked) {
    selectedAssetMap.delete(id)
    if (selectedAssetMap.size <= MAX_BATCH_DOWNLOAD_ASSETS) batchDownloadError.value = ''
    return
  }
  if (!selectedAssetMap.has(id) && selectedAssetMap.size >= MAX_BATCH_DOWNLOAD_ASSETS) {
    batchDownloadError.value = `最多一次选择 ${MAX_BATCH_DOWNLOAD_ASSETS} 个资产`
    return
  }
  batchDownloadError.value = ''
  selectedAssetMap.set(id, toSelectedAssetSummary(asset))
}

function clearSelectedAssets() {
  selectedAssetMap.clear()
  batchDownloadError.value = ''
}

function removeSelectedAsset(assetId: string) {
  selectedAssetMap.delete(assetId)
  if (selectedAssetMap.size <= MAX_BATCH_DOWNLOAD_ASSETS) batchDownloadError.value = ''
}

function onAssetSelectionChange(asset: BackendAsset, event: Event) {
  const checked = (event.target as HTMLInputElement | null)?.checked
  toggleAssetSelection(asset, checked)
}

function normalizeSelectedAssetIDs(): number[] {
  const ids = selectedAssets.value
    .map((item) => Number(item.id))
    .filter((id) => Number.isInteger(id) && id > 0)
  return Array.from(new Set(ids))
}

function resolveBatchZipFilename(): string {
  const businessName = sharedSelectedBusinessName()
  if (businessName) return buildTimestampedZipFilename(sanitizeZipEntryName(`assets-${businessName}`, 'assets'))
  return buildTimestampedZipFilename('assets')
}

function sharedSelectedBusinessName(): string {
  const selected = selectedAssets.value
  if (!selected.length) return ''
  const first = selected[0]
  if (!first.sku || first.sku === '未绑定 SKU' || !first.productName) return ''
  const firstKey = `${first.sku}__${first.productName}`
  const allSame = selected.every((item) => `${item.sku}__${item.productName}` === firstKey)
  if (!allSame) return ''
  return `${first.sku}-${first.productName}`
}

function openBulkSearchModal() {
  bulkSearchModalOpen.value = true
  bulkSearchStatus.value = ''
  bulkSearchError.value = ''
}

function parseBulkSearchTerms(raw: string): string[] {
  const seen = new Set<string>()
  const terms: string[] = []
  raw
    .split(/[\s,，;；]+/)
    .map((item) => item.trim())
    .filter(Boolean)
    .forEach((item) => {
      const normalized = item.toUpperCase()
      if (seen.has(normalized)) return
      seen.add(normalized)
      terms.push(normalized)
    })
  return terms
}

function clearBulkSearch() {
  bulkSearchAbort?.abort()
  bulkSearchAbort = null
  bulkSearchRunSeq += 1
  bulkSearchTermCache.clear()
  bulkSearchInput.value = ''
  bulkSearchResults.value = []
  bulkSearchStatus.value = ''
  bulkSearchError.value = ''
}

function bulkSearchCacheKey(term: string): string {
  return `${bulkSearchFilters.format}|${bulkSearchFilters.assetKind}|${term}`
}

function normalizeBulkSearchResult(term: string, raw: unknown): BulkSearchResult {
  const record = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>
  const status = String(record.status ?? '').trim() as BulkSearchResultStatus
  const safeStatus: BulkSearchResultStatus =
    status === 'matched' || status === 'not_found' || status === 'error' ? status : 'error'
  const asset = record.asset && typeof record.asset === 'object' ? (record.asset as BackendAsset) : undefined
  return {
    term,
    status: safeStatus,
    message: String(record.message ?? (safeStatus === 'matched' ? '已匹配' : '未找到匹配资产')),
    candidates: Number(record.candidates ?? 0) || 0,
    ...(asset ? { asset } : {}),
  }
}

async function searchBulkAssetTerms(terms: string[], signal?: AbortSignal): Promise<BulkSearchResult[]> {
  const missingTerms: string[] = []
  for (const term of terms) {
    if (!bulkSearchTermCache.has(bulkSearchCacheKey(term))) {
      missingTerms.push(term)
    }
  }
  if (missingTerms.length) {
    const response = await assetsApi.batchSearchAssets({
      terms: missingTerms,
      format_filter: bulkSearchFilters.format,
      asset_kind: bulkSearchFilters.assetKind,
    }, signal)
    if (signal?.aborted) {
      throw new DOMException('aborted', 'AbortError')
    }
    const resultRows = Array.isArray(response.data?.data?.results) ? response.data.data.results : []
    const resultByTerm = new Map(resultRows.map((item) => [String(item.term ?? '').toUpperCase(), item]))
    for (const term of missingTerms) {
      const result = normalizeBulkSearchResult(term, resultByTerm.get(term) ?? {
        status: 'error',
        message: '后端未返回该关键词的搜索结果',
        candidates: 0,
      })
      bulkSearchTermCache.set(bulkSearchCacheKey(term), result)
    }
  }
  return terms.map((term) => {
    const cached = bulkSearchTermCache.get(bulkSearchCacheKey(term))
    if (cached) return cached
    return {
      term,
      status: 'error',
      message: '未生成搜索结果',
      candidates: 0,
    }
  })
}

async function runBulkAssetSearch() {
  if (bulkSearchRunning.value) return
  const terms = parseBulkSearchTerms(bulkSearchInput.value)
  bulkSearchError.value = ''
  bulkSearchStatus.value = ''
  bulkSearchResults.value = []
  if (!terms.length) {
    bulkSearchError.value = '请先粘贴 SKU 或任务单号'
    return
  }
  if (terms.length > MAX_BULK_SEARCH_TERMS) {
    bulkSearchError.value = `最多一次搜索 ${MAX_BULK_SEARCH_TERMS} 个 SKU / 任务单号`
    return
  }

  const runSeq = ++bulkSearchRunSeq
  bulkSearchAbort?.abort()
  const abortController = new AbortController()
  bulkSearchAbort = abortController
  bulkSearchRunning.value = true
  try {
    bulkSearchStatus.value = `正在批量搜索 ${terms.length} 项`
    const results = await searchBulkAssetTerms(terms, abortController.signal)
    if (abortController.signal.aborted || runSeq !== bulkSearchRunSeq) return
    bulkSearchResults.value = results
    bulkSearchStatus.value = `已生成明细：命中 ${results.filter((item) => item.status === 'matched').length} 项，未命中 ${results.filter((item) => item.status !== 'matched').length} 项`
  } catch (err) {
    if (abortController.signal.aborted || runSeq !== bulkSearchRunSeq) return
    bulkSearchError.value = resolveApiUserMessage(err, { fallback: '批量搜索失败，请稍后重试' })
  } finally {
    if (bulkSearchAbort === abortController) {
      bulkSearchAbort = null
    }
    if (runSeq === bulkSearchRunSeq) {
      bulkSearchRunning.value = false
    }
  }
}

function normalizeBulkSearchAssetIDs(): number[] {
  const ids = bulkSearchMatchedResults.value
    .map((item) => Number(item.asset?.id))
    .filter((id) => Number.isInteger(id) && id > 0)
  return Array.from(new Set(ids))
}

async function downloadBulkSearchResults() {
  if (bulkSearchDownloading.value) return
  bulkSearchError.value = ''
  const assetIDs = normalizeBulkSearchAssetIDs()
  if (!assetIDs.length) {
    bulkSearchError.value = '当前没有可下载的命中资产'
    return
  }
  bulkSearchDownloading.value = true
  try {
    const res = await assetsApi.batchDownload(assetIDs, { namingMode: 'business' })
    const manifest = res.data?.data
    const items = Array.isArray(manifest?.items) ? manifest.items : []
    if (!items.length) {
      bulkSearchError.value = '没有可下载的资产'
      return
    }
    const serverFailures = Array.isArray(manifest?.failures) ? manifest.failures : []
    const result = await downloadBatchAsZip({
      items: items.map((item) => ({
        key: `asset-${item.asset_id}`,
        filename: item.filename,
        downloadURL: item.download_url,
        fallbackName: `asset-${item.asset_id}`,
        failureHint: `asset_id=${item.asset_id} filename=${item.filename || `asset-${item.asset_id}`} reason=fetch_failed`,
      })),
      zipFilename: buildTimestampedZipFilename('bulk-assets'),
      serverFailures: serverFailures.map(formatServerBatchDownloadFailure),
      onStatus: (message) => {
        bulkSearchStatus.value = message
      },
    })
    bulkSearchStatus.value = `已生成 ZIP，共 ${result.writtenCount} 个文件`
    bulkSearchError.value = result.failureCount > 0 ? `${result.failureCount} 个文件未打包，详情见 ZIP 内 download_errors.txt` : ''
  } catch (err) {
    bulkSearchError.value = resolveApiUserMessage(err, { fallback: '批量搜索下载失败' })
  } finally {
    bulkSearchDownloading.value = false
  }
}

function downloadBlob(blob: Blob, filename: string) {
  const objectURL = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = objectURL
  link.download = filename
  link.rel = 'noopener'
  document.body.appendChild(link)
  link.click()
  link.remove()
  window.setTimeout(() => URL.revokeObjectURL(objectURL), 1000)
}

function openExcelPicker() {
  if (excelPackaging.value) return
  excelPackageStatus.value = ''
  excelPackageError.value = ''
  if (excelFileInput.value) {
    excelFileInput.value.value = ''
    excelFileInput.value.click()
  }
}

function startReplaceAsset(asset: BackendAsset | null | undefined) {
  if (!asset || !assetCanBeReplaced(asset)) {
    replacementStatus.value = ''
    replacementError.value = '当前资源不可修改；只有系统内的参考图、源文件、最终成品图可替换'
    return
  }
  replacementTargetAsset.value = asset
  replacementStatus.value = ''
  replacementError.value = ''
  if (replacementFileInput.value) {
    replacementFileInput.value.value = ''
    replacementFileInput.value.click()
  }
}

async function handleReplacementFile(event: Event) {
  const input = event.target as HTMLInputElement | null
  const file = input?.files?.[0]
  const asset = replacementTargetAsset.value
  if (!file || !asset) return

  const taskId = assetTaskId(asset)
  const assetId = positiveID(assetResourceId(asset))
  const kind = rawAssetKind(asset)
  if (!taskId || !assetId || (kind !== 'delivery' && kind !== 'source' && kind !== 'reference')) {
    replacementStatus.value = ''
    replacementError.value = '当前资源缺少任务或资产信息，不能在资产中心直接修改'
    if (input) input.value = ''
    return
  }

  replacementUploading.value = true
  replacementStatus.value = '正在上传并生成新版本'
  replacementError.value = ''
  try {
    await uploadTaskFileViaAssetSession(
      taskId,
      file,
      {
        asset_id: assetId,
        asset_kind: kind as AssetKind,
        target_sku_code: assetScopeSkuCode(asset) || undefined,
        remark: `资产中心修改资源：${file.name}`,
      },
      {
        onProgress: (progress) => {
          const percent = Number(progress.percent)
          replacementStatus.value = Number.isFinite(percent)
            ? `正在上传并生成新版本 ${Math.max(0, Math.min(100, Math.round(percent)))}%`
            : '正在上传并生成新版本'
        },
      },
    )
    replacementStatus.value = '资源已修改，新版本已进入对应审核状态'
    await reload()
  } catch (err) {
    replacementStatus.value = ''
    replacementError.value = resolveApiUserMessage(err, { fallback: '修改资源失败，请稍后重试' })
  } finally {
    replacementUploading.value = false
    if (input) input.value = ''
  }
}

function normalizeExcelCell(value: unknown): string {
  if (value == null) return ''
  if (value instanceof Date) return value.toISOString()
  if (typeof value === 'object') {
    const record = value as Record<string, unknown>
    if (typeof record.text === 'string') return record.text.trim()
    if (typeof record.result === 'string' || typeof record.result === 'number') return normalizeExcelCell(record.result)
    if (Array.isArray(record.richText)) {
      return record.richText
        .map((item) => (item && typeof item === 'object' ? String((item as Record<string, unknown>).text ?? '') : ''))
        .join('')
        .trim()
    }
  }
  const text = String(value).trim()
  if (/^\d+\.0$/.test(text)) return String(Math.trunc(Number(text)))
  return text
}

function normalizeExcelQuantity(value: unknown): number {
  const text = normalizeExcelCell(value)
  if (!text) return 1
  const n = Number(text)
  if (!Number.isFinite(n)) return 1
  return Math.max(1, Math.trunc(n))
}

function normalizeExcelHeader(value: unknown): string {
  return normalizeExcelCell(value).replace(/\s+/g, '').toLowerCase()
}

function resolveExcelColumn(headers: string[], candidates: string[], fallbackIndex: number): number {
  const normalizedCandidates = candidates.map((item) => normalizeExcelHeader(item))
  const found = headers.findIndex((header) => normalizedCandidates.includes(header))
  return found >= 0 ? found : fallbackIndex
}

async function parseExcelPackageRows(file: File): Promise<AssetExcelPackageRow[]> {
  if (!/\.xlsx$/i.test(file.name)) throw new Error('当前仅支持 .xlsx 模板，请将 .xls 另存为 .xlsx 后再上传')
  const ExcelJS = await import('exceljs')
  const data = await file.arrayBuffer()
  const workbook = new ExcelJS.Workbook()
  await workbook.xlsx.load(data)
  const worksheet = workbook.worksheets[0]
  if (!worksheet) throw new Error('Excel 文件没有工作表')
  const table: unknown[][] = []
  worksheet.eachRow({ includeEmpty: true }, (row) => {
    const values = Array.isArray(row.values) ? row.values.slice(1) : []
    table.push(values)
  })
  if (table.length < 2) throw new Error('Excel 至少需要表头和一行数据')

  const headers = (table[0] ?? []).map(normalizeExcelHeader)
  const orderCol = resolveExcelColumn(headers, ['订单号', '订单编号', 'order_no', 'order'], 0)
  const skuCol = resolveExcelColumn(headers, ['SKU编码', 'SKU', 'sku_code', '商品编码'], 1)
  const skuNameCol = resolveExcelColumn(headers, ['SKU名称', '商品名称', 'sku_name', '名称'], 2)
  const quantityCol = resolveExcelColumn(headers, ['数量', 'qty', 'quantity', 'num'], 3)
  const keywordCol = resolveExcelColumn(headers, ['匹配关键词', '关键词', 'keyword', 'kw'], 4)

  return table
    .slice(1)
    .map((row, index): AssetExcelPackageRow => {
      const values = Array.isArray(row) ? row : []
      return {
        row_number: index + 2,
        order_no: normalizeExcelCell(values[orderCol]),
        sku_code: normalizeExcelCell(values[skuCol]).toUpperCase(),
        sku_name: normalizeExcelCell(values[skuNameCol]),
        quantity: normalizeExcelQuantity(values[quantityCol]),
        keyword: normalizeExcelCell(values[keywordCol]),
      }
    })
    .filter((row) => row.order_no || row.sku_code || row.sku_name)
}

function resolveExcelPackageFilename(item: AssetExcelPackageItem, copyIndex: number): string {
  const ext = (() => {
    const i = item.filename.lastIndexOf('.')
    return i > 0 ? item.filename.slice(i) : '.jpg'
  })()
  const rawSku = item.sku_code || item.sku_name || `asset-${item.asset_id}`
  const rowSuffix = item.row_number ? `_row${item.row_number}` : ''
  const base = sanitizeZipEntryName(`${rawSku}${rowSuffix}`, `asset-${item.asset_id}`)
  return `${base}_${copyIndex}${ext}`
}

function formatExcelFailure(item: AssetExcelPackageFailure): string {
  return [
    item.row_number ? `row=${item.row_number}` : '',
    item.order_no ? `order=${item.order_no}` : '',
    item.sku_code ? `sku=${item.sku_code}` : '',
    item.quantity ? `qty=${item.quantity}` : '',
    `reason=${item.reason}`,
    item.message ? `message=${item.message}` : '',
  ]
    .filter(Boolean)
    .join(' ')
}

async function downloadExcelPackageAsZip(items: AssetExcelPackageItem[], failures: AssetExcelPackageFailure[]): Promise<number> {
  const { default: JSZip } = await import('jszip')
  const zip = new JSZip()
  const reportLines: string[] = [
    'Excel 图片分拣下载报告',
    `生成时间：${formatDateTimeBeijing(new Date().toISOString())}`,
    `成功行数：${items.length}`,
    `失败行数：${failures.length}`,
    '',
    '失败明细：',
    ...(failures.length ? failures.map(formatExcelFailure) : ['无']),
    '',
  ]
  let completed = 0
  let copied = 0

  await mapWithConcurrency(items, EXCEL_PACKAGE_CONCURRENCY, async (item) => {
    const url = String(item.download_url ?? '').trim()
    if (!url) {
      reportLines.push(formatExcelFailure({
        row_number: item.row_number,
        order_no: item.order_no,
        sku_code: item.sku_code,
        quantity: item.quantity,
        reason: 'missing_download_url',
        message: '下载地址为空',
      }))
      return
    }
    try {
      const response = await fetch(url, { credentials: 'omit', mode: 'cors' })
      if (!response.ok) throw new Error(`http_${response.status}`)
      const blob = await response.blob()
      const folder = sanitizeZipEntryName(item.order_no, '未知订单')
      for (let i = 1; i <= item.quantity; i += 1) {
        zip.file(`${folder}/${resolveExcelPackageFilename(item, i)}`, blob, { binary: true, compression: 'STORE' })
        copied += 1
      }
    } catch (err) {
      const reason = err instanceof Error ? err.message : 'fetch_failed'
      reportLines.push(formatExcelFailure({
        row_number: item.row_number,
        order_no: item.order_no,
        sku_code: item.sku_code,
        quantity: item.quantity,
        reason,
        message: '文件下载失败',
      }))
    } finally {
      completed += 1
      excelPackageStatus.value = `正在下载并分拣 ${completed}/${items.length} 行，已写入 ${copied} 个文件`
    }
  })

  zip.file('打包报告.txt', reportLines.join('\n') + '\n')

  excelPackageStatus.value = '正在生成 ZIP'
  const blob = await zip.generateAsync(
    { type: 'blob', compression: 'STORE', streamFiles: true },
    (metadata: { percent: number }) => {
      excelPackageStatus.value = `正在生成 ZIP ${Math.floor(metadata.percent)}%`
    },
  )
  downloadBlob(blob, buildTimestampedZipFilename('excel-image-package'))
  return copied
}

async function handleExcelPackageFile(event: Event) {
  const file = (event.target as HTMLInputElement | null)?.files?.[0]
  if (!file || excelPackaging.value) return
  excelPackaging.value = true
  excelPackageStatus.value = '正在解析 Excel 模板'
  excelPackageError.value = ''
  try {
    const rows = await parseExcelPackageRows(file)
    if (!rows.length) throw new Error('Excel 中没有可处理的数据行')
    excelPackageStatus.value = `已解析 ${rows.length} 行，正在匹配资产`
    const res = await assetsApi.excelPackagePreview(rows)
    const manifest = res.data?.data
    const items = Array.isArray(manifest?.items) ? manifest.items : []
    const failures = Array.isArray(manifest?.failures) ? manifest.failures : []
    if (!items.length) throw new Error('没有匹配到可下载的 JPG/PNG 资产')
    excelPackageStatus.value = `匹配成功 ${items.length} 行，准备生成 ${manifest?.total_files ?? 0} 个文件`
    const copied = await downloadExcelPackageAsZip(items, failures)
    excelPackageStatus.value = `已生成 ZIP，共写入 ${copied} 个文件`
    const errors: string[] = []
    if (failures.length > 0) errors.push(`${failures.length} 行未匹配`)
    if (copied <= 0) errors.push('没有图片文件下载成功')
    excelPackageError.value = errors.length > 0 ? `${errors.join('；')}，详情见 ZIP 内打包报告.txt` : ''
  } catch (err) {
    excelPackageStatus.value = ''
    excelPackageError.value = resolveApiUserMessage(err, { fallback: 'Excel 图片分拣下载失败' })
  } finally {
    excelPackaging.value = false
    if (excelFileInput.value) excelFileInput.value.value = ''
  }
}

function formatServerBatchDownloadFailure(item: AssetBatchDownloadFailure): string {
  return [
    `asset_id=${item.asset_id}`,
    item.task_id != null ? `task_id=${item.task_id}` : '',
    item.filename ? `filename=${item.filename}` : '',
    `reason=${item.reason || 'unavailable'}`,
  ]
    .filter(Boolean)
    .join(' ')
}

async function downloadBatchAsClientZip(
  items: AssetBatchDownloadItem[],
  serverFailures: AssetBatchDownloadFailure[],
) {
  const result = await downloadBatchAsZip({
    items: items.map((item) => ({
      key: `asset-${item.asset_id}`,
      filename: item.filename,
      downloadURL: item.download_url,
      fallbackName: `asset-${item.asset_id}`,
      failureHint: `asset_id=${item.asset_id} filename=${item.filename || `asset-${item.asset_id}`} reason=fetch_failed`,
    })),
    zipFilename: resolveBatchZipFilename(),
    serverFailures: serverFailures.map(formatServerBatchDownloadFailure),
    onStatus: (message) => {
      batchDownloadStatus.value = message
    },
  })
  return result.failureCount
}

async function handleBatchDownload() {
  if (batchDownloading.value) return
  batchDownloadStatus.value = ''
  batchDownloadError.value = ''

  const assetIDs = normalizeSelectedAssetIDs()
  if (!assetIDs.length) {
    batchDownloadError.value = '未找到可下载的资产 ID，请重新勾选后重试'
    return
  }
  if (assetIDs.length > MAX_BATCH_DOWNLOAD_ASSETS) {
    batchDownloadError.value = `最多一次下载 ${MAX_BATCH_DOWNLOAD_ASSETS} 个资产`
    return
  }

  batchDownloading.value = true
  try {
    const res = await assetsApi.batchDownload(assetIDs, { namingMode: 'business' })
    const manifest = res.data?.data
    const items = Array.isArray(manifest?.items) ? manifest.items : []
    if (!items.length) {
      batchDownloadError.value = '没有可下载的资产'
      return
    }
    const serverFailures = Array.isArray(manifest?.failures) ? manifest.failures : []
    const clientFailureCount = await downloadBatchAsClientZip(items, serverFailures)
    const serverFailureCount = Number(manifest?.failure_count ?? 0)
    const totalFailureCount = serverFailureCount + clientFailureCount
    batchDownloadStatus.value = `已生成 ZIP，共 ${items.length} 个文件`
    batchDownloadError.value = totalFailureCount > 0 ? `${totalFailureCount} 个文件未打包，详情见 ZIP 内 download_errors.txt` : ''
  } catch (err) {
    batchDownloadError.value = resolveApiUserMessage(err, { fallback: '批量下载失败，请稍后重试' })
  } finally {
    batchDownloading.value = false
  }
}

function firstDisplayImageUrl(asset: BackendAsset): string {
  const vers = asset.versions
  if (!Array.isArray(vers)) return ''
  const preferred = vers.find(
    (v) =>
      !assetVersionMustUsePreviewEndpoint(v) &&
      v.preview_available === true &&
      typeof v.download_url === 'string' &&
      v.download_url.trim().length > 5,
  )
  if (preferred?.download_url) return preferred.download_url.trim()
  const anyUrl = vers.find(
    (v) =>
      !assetVersionMustUsePreviewEndpoint(v) &&
      typeof v.download_url === 'string' &&
      v.download_url.trim().length > 5,
  )
  return anyUrl?.download_url?.trim() ?? ''
}

/** 列表 DTO 已带可展示 URL 时跳过 GET /preview */
function listCardResolvedPreviewUrl(asset: BackendAsset): string | undefined {
  if (assetMustUsePreviewEndpoint(asset)) return undefined
  const fromVers = firstDisplayImageUrl(asset)
  if (fromVers) return fromVers
  const r = asset as Record<string, unknown>
  for (const key of ['download_url', 'downloadUrl', 'preview_url', 'previewUrl'] as const) {
    const v = r[key]
    if (typeof v === 'string' && v.trim().length > 5) return v.trim()
  }
  return undefined
}

function textField(record: Record<string, unknown>, keys: readonly string[]): string {
  for (const key of keys) {
    const value = record[key]
    if (typeof value === 'string' && value.trim()) return value.trim()
  }
  return ''
}

function filenameOrUrlLooksTiff(value: string): boolean {
  return /\.(?:tif|tiff)(?:$|[?#])/i.test(value.trim())
}

function recordLooksTiff(record: Record<string, unknown>): boolean {
  const mimeType = textField(record, ['mime_type', 'mimeType', 'content_type', 'contentType']).toLowerCase()
  if (mimeType === 'image/tiff' || mimeType === 'image/x-tiff') return true
  const name = textField(record, [
    'file_name',
    'fileName',
    'original_filename',
    'originalFilename',
    'filename',
    'name',
  ])
  if (filenameOrUrlLooksTiff(name)) return true
  const url = textField(record, [
    'download_url',
    'downloadUrl',
    'preview_url',
    'previewUrl',
    'file_url',
    'fileUrl',
    'url',
  ])
  return filenameOrUrlLooksTiff(url)
}

function assetVersionMustUsePreviewEndpoint(version: BackendAssetVersion): boolean {
  return recordLooksTiff(version as Record<string, unknown>)
}

function assetMustUsePreviewEndpoint(asset: BackendAsset): boolean {
  if (recordLooksTiff(asset as Record<string, unknown>)) return true
  return Array.isArray(asset.versions) && asset.versions.some(assetVersionMustUsePreviewEndpoint)
}

async function copyText(text: string, successMessage: string) {
  try {
    await navigator.clipboard.writeText(text)
    copyHint.value = successMessage
    window.setTimeout(() => {
      copyHint.value = ''
    }, 1200)
  } catch {
    copyHint.value = '复制失败'
    window.setTimeout(() => {
      copyHint.value = ''
    }, 1200)
  }
}

async function copyBusinessKey(asset: BackendAsset) {
  if (isExternalAsset(asset)) {
    await copyText(externalOriginPath(asset), '已复制外部路径')
    return
  }
  const sku = businessSku(asset)
  if (sku === '未绑定 SKU') {
    copyHint.value = '当前资产未绑定 SKU'
    window.setTimeout(() => {
      copyHint.value = ''
    }, 1200)
    return
  }
  await copyText(sku, '已复制 SKU')
}

function displayText(value: unknown): string {
  if (value == null) return '—'
  const text = String(value).trim()
  return text || '—'
}

function positiveID(value: unknown): string {
  const text = String(value ?? '').trim()
  if (!text) return ''
  const numeric = Number(text)
  if (Number.isFinite(numeric) && numeric <= 0) return ''
  return text
}

function displayTime(value: unknown): string {
  const text = displayText(value)
  if (text === '—') return text
  return formatDateTimeBeijing(text) || text
}

function dateFilterToRFC3339(value: string, boundary: 'start' | 'end'): string | undefined {
  const date = value.trim()
  if (!/^\d{4}-\d{2}-\d{2}$/.test(date)) return undefined
  return boundary === 'start' ? `${date}T00:00:00+08:00` : `${date}T23:59:59+08:00`
}

function dateFilterFromQuery(value: unknown): string {
  const text = typeof value === 'string' ? value.trim() : ''
  if (/^\d{4}-\d{2}-\d{2}$/.test(text)) return text
  const match = /^(\d{4}-\d{2}-\d{2})T/.exec(text)
  return match?.[1] ?? ''
}

function assetKind(asset: BackendAsset | string | null | undefined): string {
  if (typeof asset === 'string') return assetFileTypeLabel(asset)
  if (!asset) return '—'
  const record = asset as Record<string, unknown>
  return assetFileTypeLabel(record.asset_kind ?? record.asset_type ?? asset.file_role)
}

function assetFileTypeLabel(value: unknown): string {
  const key = String(value ?? '').trim().toLowerCase()
  if (!key) return '—'
  return ASSET_CENTER_FILE_TYPE_LABELS[key] ?? String(value ?? '').trim()
}

function assetUploadStatus(value: unknown): string {
  return assetUploadStatusLabelCn(typeof value === 'string' ? value : String(value ?? ''))
}

function assetArchiveStatus(value: unknown): string {
  return assetArchiveStatusLabelCn(typeof value === 'string' ? value : String(value ?? ''))
}

function assetDownloadMode(value: unknown): string {
  return assetDownloadModeLabelCn(typeof value === 'string' ? value : String(value ?? ''))
}

function goListPage(next: number) {
  const clamped = Math.min(Math.max(1, next), listTotalPages.value)
  listPage.value = clamped
}

function jumpListPage() {
  const target = Number(listJumpPage.value)
  if (!Number.isFinite(target)) {
    listJumpPage.value = listPage.value
    return
  }
  goListPage(Math.trunc(target))
}

function scheduleReload() {
  if (reloadTimer) {
    clearTimeout(reloadTimer)
    reloadTimer = null
  }
  reloadTimer = setTimeout(() => {
    void reload()
  }, AUTO_RELOAD_DELAY_MS)
}

function syncQuerySelection() {
  const nextQuery: Record<string, string> = {}
  if (filters.resourceSource !== 'all') nextQuery.source = filters.resourceSource
  if (effectiveTaskLaneFilter.value !== 'all') nextQuery.business_lane = effectiveTaskLaneFilter.value
  if (effectiveAssetKindFilter.value !== 'all') nextQuery.asset_type = effectiveAssetKindFilter.value
  if (filters.timeBasis !== 'asset_uploaded_at') nextQuery.time_basis = filters.timeBasis
  if (effectiveSourceModuleFilter.value !== 'all') nextQuery.module_key = effectiveSourceModuleFilter.value
  if (filters.formatCategory !== 'all') nextQuery.format_category = filters.formatCategory
  if (filters.createdFrom) nextQuery.created_from = filters.createdFrom
  if (filters.createdTo) nextQuery.created_to = filters.createdTo
  if (selectedAssetId.value.trim()) nextQuery.asset_id = selectedAssetId.value.trim()
  void router.replace({ query: nextQuery })
}

function openAssetDetail(assetId: string) {
  if (!canAccessPage('asset_detail')) return
  const query: Record<string, string> = {}
  if (filters.resourceSource !== 'all') query.source = filters.resourceSource
  if (effectiveTaskLaneFilter.value !== 'all') query.business_lane = effectiveTaskLaneFilter.value
  if (effectiveAssetKindFilter.value !== 'all') query.asset_type = effectiveAssetKindFilter.value
  if (filters.timeBasis !== 'asset_uploaded_at') query.time_basis = filters.timeBasis
  if (effectiveSourceModuleFilter.value !== 'all') query.module_key = effectiveSourceModuleFilter.value
  if (filters.formatCategory !== 'all') query.format_category = filters.formatCategory
  if (filters.createdFrom) query.created_from = filters.createdFrom
  if (filters.createdTo) query.created_to = filters.createdTo
  void router.push({ name: 'AssetDetail', params: { id: assetId }, query })
}

function openRelatedTask(asset: BackendAsset | null | undefined) {
  const id = assetTaskId(asset)
  if (!id) return
  void router.push({ name: 'TaskDetail', params: { id } })
}

async function reload() {
  reloadAbort?.abort()
  void loadAssetPredictions()
  const requestSeq = ++reloadRequestSeq
  const abortController = new AbortController()
  reloadAbort = abortController
  loading.value = true
  error.value = ''
  try {
    const res = await assetsApi.searchAssets(
      {
        keyword: effectiveSearchKeyword.value || undefined,
        source: effectiveAssetSearchSource.value,
        business_lane: effectiveTaskLaneFilter.value === 'all' ? undefined : effectiveTaskLaneFilter.value,
        asset_type: effectiveAssetKindFilter.value === 'all' ? undefined : effectiveAssetKindFilter.value,
        module_key: effectiveSourceModuleFilter.value === 'all' ? undefined : effectiveSourceModuleFilter.value,
        time_basis: filters.timeBasis,
        format_category: filters.formatCategory === 'all' ? undefined : filters.formatCategory,
        created_from: dateFilterToRFC3339(filters.createdFrom, 'start'),
        created_to: dateFilterToRFC3339(filters.createdTo, 'end'),
        page: listPage.value,
        size: listPageSize.value,
      },
      abortController.signal,
    )
    if (abortController.signal.aborted || requestSeq !== reloadRequestSeq) return
    const body = res.data
    const backendItems = Array.isArray(body?.data) ? body.data : []
    const backendTotal = Number(body?.total)
    const backendPage = Number(body?.page)
    const backendSize = Number(body?.size)

    assets.value = backendItems
    listTotal.value = Number.isFinite(backendTotal) && backendTotal >= 0 ? backendTotal : backendItems.length
    if (Number.isFinite(backendPage) && backendPage > 0) {
      listPage.value = Math.trunc(backendPage)
    }
    if (Number.isFinite(backendSize) && backendSize > 0) {
      listPageSize.value = Math.trunc(backendSize)
    }
    if (!assets.value.length) {
      selectedAssetId.value = ''
      selectedAssetDetail.value = null
      detailModalOpen.value = false
      syncQuerySelection()
    } else {
      let nextId = ''
      if (requestedAssetId.value && assets.value.some((item) => assetResourceId(item) === requestedAssetId.value)) {
        nextId = requestedAssetId.value
      } else if (
        selectedAssetId.value &&
        assets.value.some((item) => assetResourceId(item) === selectedAssetId.value)
      ) {
        nextId = selectedAssetId.value
      }
      selectedAssetId.value = nextId
      syncQuerySelection()
      detailModalOpen.value = false
      selectedAssetDetail.value = null
      downloadMeta.value = null
      previewMeta.value = null
      previewUnavailable.value = false
      previewNotFound.value = false
      detailError.value = ''
    }
  } catch (err) {
    if (abortController.signal.aborted || requestSeq !== reloadRequestSeq) return
    error.value = err instanceof Error ? err.message : '加载资产列表失败'
    assets.value = []
    listTotal.value = 0
    selectedAssetId.value = ''
    selectedAssetDetail.value = null
    detailModalOpen.value = false
  } finally {
    if (reloadAbort === abortController) {
      reloadAbort = null
    }
    if (requestSeq === reloadRequestSeq) {
      loading.value = false
    }
  }
}

async function loadAssetPredictions(): Promise<void> {
  assetPredictionAbort?.abort()
  assetPredictionSuggestions.value = []
  const abortController = new AbortController()
  assetPredictionAbort = abortController
  try {
    const bundle = await predictionsApi.assets(
      { keyword: effectiveSearchKeyword.value, limit: 4 },
      abortController.signal,
    )
    if (abortController.signal.aborted) return
    assetPredictionSuggestions.value = bundle.suggestions
  } catch {
    if (!abortController.signal.aborted) assetPredictionSuggestions.value = []
  } finally {
    if (assetPredictionAbort === abortController) assetPredictionAbort = null
  }
}

function openPredictionAsset(item: PredictionSuggestion): void {
  recordAssetPredictionBehavior(item, 'jump')
  const id = String(item.target_id ?? '').trim()
  if (id) {
    openAssetDetail(id)
  }
}

function recordAssetPredictionBehavior(item: PredictionSuggestion, action: 'jump'): void {
  recordExperienceBehavior({
    action,
    surface: 'asset_center',
    target_type: item.target_type || 'asset',
    target_id: item.target_id || '',
    suggestion_event_id: item.suggestion_event_id || '',
    suggestion_stable_key: item.suggestion_stable_key || '',
    component: 'AssetCenterPredictionCard',
    payload: {
      suggestion_id: item.id,
      suggestion_type: String(item.type || ''),
      source: item.source || '',
      action_type: item.action_type || '',
      action_label: item.action_label || '',
    },
  })
}

onMounted(() => {
  if (requestedTaskId.value) {
    filters.keyword = requestedTaskId.value
  }
  const requestedSource = typeof route.query.source === 'string' ? route.query.source.trim() : ''
  if (requestedSource === 'system' || requestedSource === 'external' || requestedSource === 'all') {
    filters.resourceSource = requestedSource
  }
  const requestedTimeBasis = typeof route.query.time_basis === 'string' ? route.query.time_basis.trim() : ''
  if (assetTimeBasisOptions.some((option) => option.value === requestedTimeBasis)) {
    filters.timeBasis = requestedTimeBasis as AssetTimeBasisFilter
  }
  const requestedTaskLane = typeof route.query.business_lane === 'string' ? route.query.business_lane.trim() : ''
  if (assetTaskLaneOptions.some((option) => option.value === requestedTaskLane)) {
    filters.taskLane = requestedTaskLane as AssetTaskLaneFilter
  }
  const requestedAssetKind = typeof route.query.asset_type === 'string' ? route.query.asset_type.trim() : ''
  if (assetKindFilterOptions.some((option) => option.value === requestedAssetKind)) {
    filters.assetKind = requestedAssetKind as AssetKindFilter
  }
  const requestedModuleKey = typeof route.query.module_key === 'string' ? route.query.module_key.trim() : ''
  if (assetSourceModuleOptions.some((option) => option.value === requestedModuleKey)) {
    filters.moduleKey = requestedModuleKey as AssetSourceModuleFilter
  }
  const requestedFormat = typeof route.query.format_category === 'string' ? route.query.format_category.trim() : ''
  if (assetFormatCategoryOptions.some((option) => option.value === requestedFormat)) {
    filters.formatCategory = requestedFormat as AssetFormatFilter
  }
  filters.createdFrom = dateFilterFromQuery(route.query.created_from)
  filters.createdTo = dateFilterFromQuery(route.query.created_to)
  if (filters.resourceSource === 'external') {
    filters.timeBasis = 'asset_uploaded_at'
    filters.taskLane = 'all'
    filters.assetKind = 'all'
    filters.moduleKey = 'all'
  }
  void reload()
})

onBeforeUnmount(() => {
  if (reloadTimer) {
    clearTimeout(reloadTimer)
    reloadTimer = null
  }
  reloadAbort?.abort()
  reloadAbort = null
  assetPredictionAbort?.abort()
  assetPredictionAbort = null
  bulkSearchAbort?.abort()
  bulkSearchAbort = null
  bulkSearchRunSeq += 1
  clearSelectedAssets()
})
</script>

<style scoped>
.assets-index-view {
  --ac-bg: rgb(var(--yb-surface-apple));
  --ac-card: rgb(var(--yb-surface));
  --ac-text: rgb(var(--yb-text-apple));
  --ac-sec: rgb(var(--yb-text-apple-muted));
  --ac-accent: rgb(var(--yb-apple-blue));
  --ac-page-pad: clamp(1rem, 2vw, 1.5rem);
  /** 资产页铺满主内容区，列数由视口宽度控制，卡片按比例伸缩 */
  --ac-content-max: 100%;
  --ac-grid-columns: 5;
  background: var(--ac-bg);
  color: var(--ac-text);
  padding: 0 0 3.75rem;
}

.ac-header {
  position: sticky;
  top: 0;
  z-index: 100;
  background: transparent;
  backdrop-filter: none;
  border-bottom: 0;
  padding: 0.65rem 0 0.35rem;
  box-shadow: none;
}

.ac-nav-box {
  max-width: var(--ac-content-max);
  margin: 0 auto;
  padding: 0 var(--ac-page-pad);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.ac-brand {
  margin: 0;
  font-size: 22px;
  font-weight: 600;
  color: var(--ac-text);
}

.ac-search-wrap {
  position: relative;
  flex: 1;
  min-width: 200px;
  max-width: 450px;
}

.ac-search-icon {
  position: absolute;
  left: 12px;
  top: 50%;
  transform: translateY(-50%);
  color: var(--ac-sec);
  pointer-events: none;
}

.ac-search-input {
  width: 100%;
  padding: 9px 14px 9px 38px;
  border: 1px solid rgb(var(--yb-border));
  border-radius: 10px;
  background: rgb(var(--yb-surface));
  font-size: 14px;
  outline: none;
  transition: border-color 0.2s, box-shadow 0.2s;
  color: var(--ac-text);
  box-shadow: none;
}

.ac-search-input:focus {
  border-color: rgb(var(--yb-brand) / 0.45);
  box-shadow: 0 0 0 3px rgb(var(--yb-brand) / 0.1);
}

.ac-header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.ac-hidden-file {
  position: absolute;
  width: 1px;
  height: 1px;
  opacity: 0;
  pointer-events: none;
}

.ac-aria-hint {
  font-size: 12px;
  color: var(--ac-accent);
  min-height: 1em;
}

.ac-icon-btn {
  padding: 8px 14px;
  border-radius: 10px;
  border: 1px solid rgb(var(--yb-black) / 0.06);
  background: rgb(var(--yb-surface));
  color: var(--ac-accent);
  font-weight: 500;
  font-size: 14px;
  cursor: pointer;
  transition: opacity 0.2s;
}

.ac-icon-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.ac-icon-btn--primary {
  background: var(--ac-accent);
  border-color: var(--ac-accent);
  color: rgb(var(--yb-surface));
}

.ac-filters-panel {
  max-width: var(--ac-content-max);
  margin: 8px auto 0;
  padding: 0 var(--ac-page-pad) 12px;
  border-top: 0;
}

.ac-filters-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr));
  gap: 12px;
  margin-top: 12px;
}

.ac-filter-hint {
  margin: 10px 0 0;
  font-size: 12px;
  color: var(--ac-sec);
}

.ac-status-bar {
  max-width: var(--ac-content-max);
  margin: 0.25rem auto 0;
  padding: 0 var(--ac-page-pad);
  font-size: 11px;
  color: rgb(var(--yb-text-faint));
  line-height: 1.4;
}

.ac-status-bar b {
  color: rgb(var(--yb-text-muted));
  font-weight: 500;
}

.ac-prediction-strip {
  display: grid;
  gap: 0.75rem;
  max-width: var(--ac-content-max);
  margin: 0.75rem auto 0;
  padding: 0.875rem var(--ac-page-pad);
  border: 1px solid rgb(var(--yb-brand-border));
  border-radius: 0.875rem;
  background:
    linear-gradient(120deg, rgb(var(--yb-brand) / 0.08), rgb(var(--yb-teal-accent) / 0.08), rgb(var(--yb-brand) / 0.08)),
    rgb(var(--yb-surface-brand-panel));
  background-size: 220% 100%;
  animation: ac-stream-panel 8s linear infinite;
}

.ac-prediction-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 0.75rem;
}

.ac-prediction-head div {
  display: grid;
  gap: 0.125rem;
}

.ac-prediction-head span {
  color: rgb(var(--yb-brand));
  font-size: 0.72rem;
  font-weight: 800;
}

.ac-prediction-head strong {
  color: rgb(var(--yb-text-navy));
  font-size: 0.95rem;
}

.ac-prediction-head small {
  color: rgb(var(--yb-text-muted-strong));
  font-size: 0.72rem;
}

.ac-prediction-list {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(13rem, 1fr));
  gap: 0.625rem;
}

.ac-prediction-item {
  position: relative;
  display: grid;
  gap: 0.55rem;
  min-height: 6.25rem;
  padding: 0.75rem;
  overflow: hidden;
  border: 1px solid rgb(var(--yb-brand-subtle));
  border-radius: 0.65rem;
  background: rgb(var(--yb-surface));
  text-align: left;
  transition: transform 180ms ease, border-color 180ms ease, box-shadow 180ms ease;
  animation: ac-card-enter 420ms ease both;
}

.ac-prediction-action {
  position: relative;
  z-index: 1;
  display: grid;
  gap: 0.25rem;
  width: 100%;
  border: 0;
  background: transparent;
  padding: 0;
  text-align: left;
  cursor: pointer;
}

.ac-prediction-item:hover {
  transform: translateY(-2px);
  border-color: rgb(var(--yb-brand-border-strong));
  box-shadow: 0 14px 30px -22px rgb(var(--yb-brand) / 0.7);
}

.ac-prediction-item::after {
  content: "";
  position: absolute;
  inset: 0;
  pointer-events: none;
  background: linear-gradient(110deg, transparent 0%, rgb(var(--yb-brand-bright) / 0.14) 42%, transparent 72%);
  transform: translateX(-120%);
  transition: transform 650ms ease;
}

.ac-prediction-item:hover::after {
  transform: translateX(120%);
}

.ac-prediction-action span {
  color: rgb(var(--yb-brand));
  font-size: 0.7rem;
  font-weight: 800;
}

.ac-prediction-action strong {
  color: rgb(var(--yb-text));
  font-size: 0.875rem;
  line-height: 1.3;
}

.ac-prediction-action small {
  color: rgb(var(--yb-text-soft));
  font-size: 0.75rem;
  line-height: 1.35;
}

.ac-prediction-action em {
  width: max-content;
  padding: 0.12rem 0.45rem;
  border-radius: 999px;
  background: rgb(var(--yb-brand-subtle));
  color: rgb(var(--yb-brand-strong));
  font-size: 0.6875rem;
  font-style: normal;
}

@keyframes ac-stream-panel {
  from { background-position: 0% 50%; }
  to { background-position: 220% 50%; }
}

@keyframes ac-card-enter {
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
  .ac-prediction-strip,
  .ac-prediction-item {
    animation: none ;
  }

  .ac-prediction-item,
  .ac-prediction-item::after {
    transition: none ;
  }
}

.ac-batch-bar {
  max-width: var(--ac-content-max);
  margin: 8px auto 0;
  padding: 0 var(--ac-page-pad);
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.ac-batch-count {
  font-size: 13px;
  color: var(--ac-text);
  font-weight: 600;
}

.ac-batch-btn {
  padding: 6px 12px;
  border-radius: 10px;
  border: 1px solid rgb(var(--yb-black) / 0.08);
  background: rgb(var(--yb-surface));
  color: var(--ac-accent);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
}

.ac-batch-btn--ghost {
  color: rgb(var(--yb-text-slate));
}

.ac-batch-btn--primary {
  color: rgb(var(--yb-surface));
  background: var(--ac-accent);
  border-color: var(--ac-accent);
}

.ac-batch-btn:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.ac-batch-error {
  font-size: 12px;
  color: rgb(var(--yb-danger-text));
}

.ac-batch-status {
  font-size: 12px;
  color: rgb(var(--yb-text-slate));
}

.ac-grid {
  width: 100%;
  max-width: var(--ac-content-max);
  margin: 0.5rem auto 0;
  padding: 0 var(--ac-page-pad);
  display: grid;
  grid-template-columns: repeat(var(--ac-grid-columns), minmax(0, 1fr));
  gap: clamp(16px, 2vw, 22px);
  align-items: stretch;
}

@media (max-width: 1679px) {
  .assets-index-view {
    --ac-grid-columns: 4;
  }
}

@media (max-width: 1279px) {
  .assets-index-view {
    --ac-grid-columns: 3;
  }
}

@media (max-width: 979px) {
  .assets-index-view {
    --ac-grid-columns: 2;
  }
}

@media (max-width: 639px) {
  .assets-index-view {
    --ac-grid-columns: 1;
  }
}

.ac-grid-empty {
  grid-column: 1 / -1;
}

.ac-loading-state {
  grid-column: 1 / -1;
  text-align: center;
  padding: 80px 20px;
  color: var(--ac-sec);
}

.ac-loading-title {
  font-size: 18px;
  font-weight: 600;
  color: var(--ac-text);
  margin: 0 0 8px;
}

.ac-loading-sub {
  margin: 0;
  font-size: 14px;
}

.ac-state-error {
  color: rgb(var(--yb-danger-text));
}

.ac-card {
  background: var(--ac-card);
  border-radius: 22px;
  padding: clamp(16px, 2vw, 24px);
  min-width: 0;
  transition: box-shadow 0.18s;
  display: flex;
  flex-direction: column;
  border: 1px solid rgb(var(--yb-black) / 0.04);
  position: relative;
}

.ac-card:hover {
  box-shadow: 0 20px 40px rgb(var(--yb-black) / 0.06);
}

.ac-card--active {
  box-shadow: 0 0 0 2px var(--ac-accent);
}

.ac-card--selected {
  box-shadow: 0 0 0 2px rgb(var(--yb-apple-blue) / 0.28);
}

.ac-card--external {
  border-color: rgb(var(--yb-info-accent) / 0.18);
}

.ac-card-check {
  position: absolute;
  top: 10px;
  left: 10px;
  z-index: 2;
  width: 24px;
  height: 24px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 999px;
  background: rgb(var(--yb-surface) / 0.94);
  border: 1px solid rgb(var(--yb-black) / 0.12);
}

.ac-card-checkbox {
  width: 14px;
  height: 14px;
  cursor: pointer;
}

.ac-card-checkbox:disabled {
  cursor: not-allowed;
  opacity: 0.35;
}

.ac-card-img-box {
  width: 100%;
  aspect-ratio: 1;
  background: rgb(var(--yb-surface-apple-muted));
  border-radius: 14px;
  overflow: hidden;
  margin-bottom: 18px;
  display: flex;
  align-items: stretch;
  justify-content: center;
}

.ac-card-img-box :deep(.ac-card-apm) {
  width: 100%;
  height: 100%;
  min-height: 0;
}

.ac-card-img-box :deep(.apm-placeholder) {
  border: none;
  background: transparent;
  font-size: 11px;
  min-height: 100%;
  border-radius: 0;
}

.ac-card-preview-img {
  width: 100%;
  height: 100%;
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
  border-radius: 0;
}

.ac-card-info {
  flex: 1;
}

.ac-card-title {
  font-size: 17px;
  font-weight: 600;
  margin: 0 0 6px;
  line-height: 1.3;
  max-height: 44px;
  overflow: hidden;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  color: var(--ac-text);
}

.ac-source-pill {
  flex: 0 0 auto;
  border-radius: 999px;
  padding: 0.16rem 0.48rem;
  font-size: 0.65rem;
  font-weight: 600;
  color: rgb(var(--yb-text-soft));
  background: rgb(var(--yb-surface-slate));
  border: 1px solid rgb(var(--yb-border-slate));
  white-space: nowrap;
}

.ac-source-pill--external {
  color: rgb(var(--yb-info-text));
  background: rgb(var(--yb-info-soft));
  border-color: rgb(var(--yb-info-cyan-border));
}

.ac-usability-pill,
.ac-editable-pill,
.detail-state-pill {
  display: inline-flex;
  align-items: center;
  max-width: 8rem;
  min-width: 0;
  overflow: hidden;
  border-radius: 999px;
  padding: 0.18rem 0.52rem;
  border: 1px solid rgb(var(--yb-border-strong));
  background: rgb(var(--yb-surface-subtle));
  color: rgb(var(--yb-text-soft));
  font-size: 0.68rem;
  font-weight: 800;
  line-height: 1.15;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ac-editable-pill {
  border-color: rgb(var(--yb-teal-border)) ;
  background: rgb(var(--yb-teal-soft)) ;
  color: rgb(var(--yb-teal)) ;
}

.detail-state-pill {
  max-width: 100%;
  font-size: 0.76rem;
}

.ac-usability--ready {
  border-color: rgb(var(--yb-success-border)) ;
  background: rgb(var(--yb-success-ui-soft)) ;
  color: rgb(var(--yb-success-strong)) ;
}

.ac-usability--pending {
  border-color: rgb(var(--yb-warning-border-soft)) ;
  background: rgb(var(--yb-warning-soft)) ;
  color: rgb(var(--yb-warning-text)) ;
}

.ac-usability--rejected {
  border-color: rgb(var(--yb-danger-border)) ;
  background: rgb(var(--yb-danger-soft)) ;
  color: rgb(var(--yb-danger-text)) ;
}

.ac-usability--history,
.ac-usability--cleaned,
.ac-usability--neutral {
  border-color: rgb(var(--yb-border-slate)) ;
  background: rgb(var(--yb-surface-subtle)) ;
  color: rgb(var(--yb-text-muted-strong)) ;
}

.ac-card-meta {
  font-size: 13px;
  color: var(--ac-sec);
  font-family: var(--yb-font-data);
}

.ac-mono {
  margin-right: 4px;
}

.ac-copy-tag {
  color: var(--ac-accent);
  font-size: 11px;
  cursor: pointer;
  margin-left: 8px;
  padding: 0;
  border: none;
  background: none;
  font-family: inherit;
  opacity: 0;
  transition: opacity 0.2s;
}

.ac-card:hover .ac-copy-tag {
  opacity: 1;
}

.ac-card-spec {
  margin-top: 4px;
  opacity: 0.85;
}

.ac-card-footer {
  margin-top: auto;
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  padding-top: 15px;
  border-top: 1px solid rgb(var(--yb-surface-neutral-soft));
}

.ac-footer-label {
  font-size: 11px;
  color: var(--ac-sec);
  text-transform: uppercase;
  letter-spacing: 0.02em;
}

.ac-footer-stat {
  font-size: 26px;
  font-weight: 700;
  color: rgb(var(--yb-text-slate));
  letter-spacing: -1px;
  line-height: 1.1;
}

.ac-footer-right {
  flex: 1;
  max-width: 60%;
  min-width: 0;
  overflow: hidden;
  text-align: right;
}

.ac-footer-tag {
  display: block;
  font-size: 15px;
  font-weight: 500;
  color: var(--ac-text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ac-card-actions {
  margin-top: 12px;
}

.ac-card-link-btn {
  width: 100%;
  padding: 8px 12px;
  border-radius: 10px;
  border: 1px solid rgb(var(--yb-black) / 0.08);
  background: rgb(var(--yb-surface));
  color: var(--ac-accent);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
}

.ac-pagination {
  max-width: var(--ac-content-max);
  margin: 32px auto;
  padding: 0 clamp(30px, 3vw, 50px);
  display: flex;
  justify-content: center;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
}

.ac-pg-btn {
  padding: 10px 20px;
  border-radius: 10px;
  border: 1px solid rgb(var(--yb-black) / 0.05);
  background: rgb(var(--yb-surface));
  color: var(--ac-accent);
  font-weight: 500;
  cursor: pointer;
  transition: opacity 0.2s;
}

.ac-pg-btn:disabled {
  opacity: 0.35;
  cursor: not-allowed;
}

.ac-pg-meta {
  font-size: 14px;
  color: var(--ac-sec);
}

.ac-page-jump {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 14px;
  color: var(--ac-sec);
}

.ac-page-jump-input {
  width: 3.5rem;
  height: 32px;
  border-radius: 8px;
  border: 1px solid rgb(var(--yb-black) / 0.1);
  background: rgb(var(--yb-surface));
  padding: 0 8px;
  font-size: 13px;
  color: var(--ac-text);
  outline: none;
}

.ac-page-jump-input:focus {
  border-color: rgb(var(--yb-apple-blue) / 0.55);
  box-shadow: 0 0 0 2px rgb(var(--yb-apple-blue) / 0.12);
}

.ac-page-size {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: var(--ac-sec);
}

.ac-page-size-select {
  height: 32px;
  border-radius: 8px;
  border: 1px solid rgb(var(--yb-black) / 0.1);
  padding: 0 8px;
  font-size: 13px;
  background: rgb(var(--yb-surface));
  color: var(--ac-text);
}

.cell-mono {
  font-family: var(--yb-font-data);
}

.detail-grid,
.version-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(16rem, 1fr));
  gap: 0.5rem 1rem;
  margin: 0;
}

.detail-row {
  margin: 0;
}

.detail-row-full {
  grid-column: 1 / -1;
}

.detail-row dt {
  font-size: 0.75rem;
  color: rgb(var(--yb-text-muted-strong));
  margin-bottom: 0.2rem;
}

.detail-row dd {
  margin: 0;
  font-size: 0.8125rem;
  color: rgb(var(--yb-text-navy));
  word-break: break-word;
}

.versions-section {
  margin-top: 1rem;
  padding-top: 1rem;
  border-top: 1px solid rgb(var(--yb-border-slate));
}

.subsection-title {
  margin: 0 0 0.75rem;
  font-size: 0.875rem;
  font-weight: 700;
  color: rgb(var(--yb-text-slate));
}

.version-list {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(20rem, 1fr));
  gap: 0.75rem;
}

.version-card {
  border: 1px solid rgb(var(--yb-border-slate));
  border-radius: 0.875rem;
  padding: 0.875rem;
  background: rgb(var(--yb-surface-subtle));
}

.version-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  margin-bottom: 0.75rem;
}

.version-title {
  font-size: 0.8125rem;
  font-weight: 700;
  color: rgb(var(--yb-text-navy));
}

.version-pill {
  border-radius: 9999px;
  background: rgb(var(--yb-indigo-soft));
  color: rgb(var(--yb-indigo-text));
  padding: 0.15rem 0.55rem;
  font-size: 0.6875rem;
  font-weight: 600;
}

.preview-panel {
  margin-bottom: 1rem;
}

.preview-media-shell {
  width: 100%;
  min-height: 12rem;
  border: 1px solid rgb(var(--yb-border-slate));
  border-radius: 0.75rem;
  background: rgb(var(--yb-surface-subtle));
  padding: 0.5rem;
  display: flex;
  align-items: center;
  justify-content: center;
}

.preview-media-shell :deep(.apm) {
  width: 100%;
  min-height: 10rem;
}

.preview-media-shell :deep(.apm-img),
.preview-media-img {
  width: 100%;
  max-height: min(44vh, 360px);
  object-fit: contain;
}

.preview-actions {
  margin-top: 0.65rem;
  display: flex;
  align-items: center;
  gap: 0.6rem;
  flex-wrap: wrap;
}

.preview-state-hint {
  font-size: 0.75rem;
  color: rgb(var(--yb-text-muted-strong));
}

.state-text {
  font-size: 0.875rem;
  color: rgb(var(--yb-text-soft));
}

.state-error {
  color: rgb(var(--yb-danger-text));
}

.ac-selected-list {
  display: grid;
  gap: 10px;
}

.ac-selected-item {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  border: 1px solid rgb(var(--yb-border-slate));
  border-radius: 12px;
  padding: 10px 12px;
  background: rgb(var(--yb-surface-subtle));
}

.ac-selected-main {
  min-width: 0;
}

.ac-selected-title {
  margin: 0;
  font-size: 14px;
  color: rgb(var(--yb-text-navy));
  line-height: 1.4;
}

.ac-selected-meta {
  margin: 6px 0 0;
  font-size: 12px;
  color: rgb(var(--yb-text-muted-strong));
}

.ac-selected-divider {
  margin: 0 6px;
}

.ac-selected-remove {
  flex-shrink: 0;
  border: 1px solid rgb(var(--yb-black) / 0.12);
  border-radius: 8px;
  background: rgb(var(--yb-surface));
  color: rgb(var(--yb-text-slate));
  font-size: 12px;
  padding: 6px 10px;
  cursor: pointer;
}

.bulk-search-panel {
  display: grid;
  gap: 1rem;
  color: rgb(var(--yb-text));
}

.bulk-search-input-card {
  border: 1px solid rgb(var(--yb-border));
  border-radius: 16px;
  background: rgb(var(--yb-surface));
  padding: 1rem;
  box-shadow: 0 1px 2px rgb(var(--yb-shadow) / 0.06);
}

.bulk-search-label {
  display: block;
  margin-bottom: 0.5rem;
  color: var(--ac-text, rgb(var(--yb-text-apple)));
  font-size: 0.86rem;
  font-weight: 600;
}

.bulk-search-textarea {
  width: 100%;
  min-height: 11rem;
  resize: vertical;
  border: 1px solid rgb(var(--yb-border));
  border-radius: 14px;
  background: rgb(var(--yb-surface));
  color: var(--ac-text, rgb(var(--yb-text-apple)));
  padding: 0.85rem 0.95rem;
  font-family: var(--yb-font-data);
  font-size: 0.88rem;
  line-height: 1.7;
  outline: none;
  transition: border-color 0.2s, box-shadow 0.2s;
}

.bulk-search-textarea::placeholder {
  color: rgb(var(--yb-text-faint));
}

.bulk-search-textarea:hover {
  border-color: rgb(var(--yb-border-strong));
}

.bulk-search-textarea:focus {
  border-color: rgb(var(--yb-brand) / 0.45);
  box-shadow: 0 0 0 3px rgb(var(--yb-brand) / 0.1);
}

.bulk-search-filter-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.75rem;
  margin-top: 0.85rem;
}

.bulk-search-actions,
.bulk-search-summary {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.65rem;
  margin-top: 0.85rem;
}

.bulk-search-hint {
  margin: 0.75rem 0 0;
  color: var(--ac-sec, rgb(var(--yb-text-apple-muted)));
  font-size: 0.78rem;
  line-height: 1.6;
}

.bulk-search-summary {
  margin-top: 0;
}

.bulk-search-summary span {
  border: 1px solid rgb(var(--yb-border));
  border-radius: 999px;
  background: rgb(var(--yb-surface-muted));
  color: rgb(var(--yb-text-body));
  padding: 0.35rem 0.7rem;
  font-size: 0.78rem;
  font-weight: 600;
}

.bulk-result-list {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(min(100%, 28rem), 1fr));
  gap: 0.8rem;
  max-height: min(58vh, 42rem);
  overflow: auto;
  padding-right: 0.15rem;
}

.bulk-result-card {
  display: grid;
  grid-template-columns: 8rem minmax(0, 1fr);
  gap: 0.85rem;
  border: 1px solid rgb(var(--yb-border));
  border-radius: 18px;
  background: rgb(var(--yb-surface));
  padding: 0.8rem;
  box-shadow: 0 1px 2px rgb(var(--yb-shadow) / 0.06);
}

.bulk-result-card--failed {
  grid-template-columns: 5.8rem minmax(0, 1fr);
  background: rgb(var(--yb-danger-soft));
  border-color: rgb(var(--yb-danger-border));
}

.bulk-result-preview {
  min-height: 7.2rem;
  border-radius: 14px;
  overflow: hidden;
  background: rgb(var(--yb-surface-soft));
  border: 1px solid rgb(var(--yb-border));
  display: flex;
  align-items: center;
  justify-content: center;
}

.bulk-result-apm {
  width: 100%;
  height: 100%;
}

.bulk-result-img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.bulk-result-empty {
  color: rgb(var(--yb-danger-text));
  font-size: 0.8rem;
  font-weight: 700;
}

.bulk-result-main {
  min-width: 0;
}

.bulk-result-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.6rem;
}

.bulk-result-term {
  color: rgb(var(--yb-text));
  font-size: 0.86rem;
  font-weight: 700;
}

.bulk-result-pill {
  border-radius: 999px;
  background: rgb(var(--yb-surface-muted));
  color: rgb(var(--yb-text-body));
  padding: 0.25rem 0.55rem;
  font-size: 0.72rem;
  font-weight: 600;
  white-space: nowrap;
}

.bulk-result-card--failed .bulk-result-pill {
  background: rgb(var(--yb-danger-border));
  color: rgb(var(--yb-danger-text));
}

.bulk-result-title {
  margin: 0.45rem 0 0;
  color: rgb(var(--yb-text));
  font-size: 0.96rem;
  font-weight: 700;
  line-height: 1.35;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.bulk-result-meta {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.45rem;
  margin: 0.7rem 0 0;
}

.bulk-result-meta div {
  border: 1px solid rgb(var(--yb-border));
  border-radius: 10px;
  background: rgb(var(--yb-surface-soft));
  padding: 0.45rem 0.55rem;
}

.bulk-result-meta dt {
  color: rgb(var(--yb-text-muted));
  font-size: 0.7rem;
  font-weight: 600;
}

.bulk-result-meta dd {
  margin: 0.15rem 0 0;
  color: rgb(var(--yb-text));
  font-size: 0.78rem;
  font-weight: 600;
}

.bulk-result-message {
  margin: 0.55rem 0 0;
  color: var(--ac-sec, rgb(var(--yb-text-apple-muted)));
  font-size: 0.78rem;
  line-height: 1.55;
}

@media (max-width: 720px) {
  .bulk-result-card,
  .bulk-result-card--failed {
    grid-template-columns: minmax(0, 1fr);
  }

  .bulk-result-preview {
    min-height: 12rem;
  }

  .bulk-result-meta {
    grid-template-columns: minmax(0, 1fr);
  }

  .bulk-search-filter-grid {
    grid-template-columns: minmax(0, 1fr);
  }
}

/* Phase 5: light admin asset center — final override wins over dark glass skin. Style-only. */
.assets-index-view {
  margin: 0;
  min-height: auto;
  padding: 0 0 3.75rem;
  --ac-bg: transparent;
  --ac-card: rgb(var(--yb-surface));
  --ac-text: rgb(var(--yb-text));
  --ac-sec: rgb(var(--yb-text-muted));
  --ac-accent: rgb(var(--yb-brand));
  --ac-card-border: rgb(var(--yb-border));
  --ac-card-border-hover: rgb(var(--yb-border-strong));
  --ac-card-subpanel: rgb(var(--yb-surface-soft));
  --ac-field-label: rgb(var(--yb-text-muted));
  --ac-field-value: rgb(var(--yb-text));
  background: transparent ;
  color: rgb(var(--yb-text)) ;
}

.assets-index-view {
  --ac-page-pad: clamp(1rem, 2vw, 1.5rem);
}

.assets-index-view .ac-header {
  background: transparent ;
  backdrop-filter: none ;
  -webkit-backdrop-filter: none ;
  border-bottom: 0;
  box-shadow: none ;
  padding-top: 0.65rem;
  padding-bottom: 0.35rem;
}

.ac-batch-bar,
.ac-excel-package-bar,
.ac-pagination,
.ac-selected-item {
  border-color: rgb(var(--yb-border)) ;
  background: rgb(var(--yb-surface)) ;
  color: rgb(var(--yb-text)) ;
  box-shadow: 0 1px 2px rgb(var(--yb-shadow) / 0.06) ;
}

.ac-filters-panel {
  background: transparent ;
  border: 0 ;
  box-shadow: none ;
}

.ac-search-input {
  border: 1px solid rgb(var(--yb-border)) ;
  background: rgb(var(--yb-surface)) ;
  box-shadow: none ;
}

.ac-search-input:focus {
  border-color: rgb(var(--yb-brand) / 0.45) ;
  box-shadow: 0 0 0 3px rgb(var(--yb-brand) / 0.08) ;
}

.ac-status-bar {
  background: transparent ;
  border: 0 ;
  box-shadow: none ;
  color: rgb(var(--yb-text-faint)) ;
  font-size: 11px ;
  margin-top: 0.65rem;
  padding-top: 0;
  padding-bottom: 0;
}

.ac-status-bar b {
  color: rgb(var(--yb-text-muted)) ;
  font-weight: 500 ;
}

.ac-brand,
.ac-card-title,
.ac-footer-stat,
.ac-selected-title,
.ac-title-row .ac-card-title {
  color: rgb(var(--yb-text)) ;
}

.ac-card {
  border-color: rgb(var(--yb-border)) ;
  background: rgb(var(--yb-surface)) ;
  box-shadow: 0 1px 2px rgb(var(--yb-shadow) / 0.06) ;
}

.ac-header,
.ac-status-bar,
.ac-batch-bar,
.ac-excel-package-bar,
.ac-pagination {
  max-width: none;
  margin-left: 0;
  margin-right: 0;
}

.ac-grid {
  max-width: none;
  margin: 0.85rem 0 0;
  padding: 0;
  grid-template-columns: repeat(auto-fill, minmax(min(100%, 29rem), 1fr));
  gap: 0.85rem;
}

.ac-card {
  min-height: 12.25rem;
  display: grid;
  grid-template-columns: 9.25rem minmax(0, 1fr);
  grid-template-rows: auto minmax(2.6rem, auto);
  grid-template-areas:
    "preview info"
    "preview actions";
  gap: 0.78rem 1.05rem;
  padding: 0.9rem;
  border-radius: 1.05rem ;
}

.ac-card--external {
  grid-template-rows: auto auto minmax(2.6rem, auto);
  grid-template-areas:
    "preview info"
    "preview footer"
    "preview actions";
}

.ac-card:hover,
.ac-card--active,
.ac-card--selected {
  border-color: rgb(var(--yb-brand-border-strong)) ;
  background: rgb(var(--yb-surface)) ;
  box-shadow: 0 0 0 1px rgb(var(--yb-brand) / 0.12), 0 2px 8px rgb(var(--yb-shadow) / 0.06) ;
}

.ac-card--selected {
  border-color: rgb(var(--yb-brand)) ;
}

.ac-card-check {
  background: rgb(var(--yb-surface)) ;
  border-color: rgb(var(--yb-border-strong)) ;
  box-shadow: none ;
}

.ac-card--selected .ac-card-check {
  background: rgb(var(--yb-brand)) ;
  border-color: rgb(var(--yb-brand-border-strong)) ;
}

.ac-card-img-box {
  grid-area: preview;
  width: 9.25rem;
  min-height: 9.25rem;
  margin: 0;
  align-self: stretch;
  border-radius: 0.82rem ;
  background: rgb(var(--yb-surface-preview)) ;
  border: 1px solid rgb(var(--yb-border-preview)) ;
  position: relative;
}

.ac-card-img-box :deep(.ac-card-apm),
.ac-card-img-box :deep(.apm) {
  width: 100%;
  height: 100%;
  min-height: 0;
  border: 0 ;
  border-radius: inherit ;
  background: transparent ;
  box-shadow: none ;
}

.ac-card-img-box :deep(.apm-placeholder),
.ac-card-img-box :deep(.apm-empty) {
  width: 100%;
  height: 100%;
  min-height: 0;
  border: 0 ;
  border-radius: 0 ;
  background: transparent ;
  box-shadow: none ;
  outline: 0;
}

.ac-card-img-box :deep(.apm-placeholder-img) {
  max-width: 82%;
  max-height: 82%;
}

:global(#app .assets-index-view .ac-card-img-box .ac-card-apm),
:global(#app .assets-index-view .ac-card-img-box .apm) {
  width: 100%;
  height: 100%;
}

:global(#app .assets-index-view .ac-card-img-box .apm-placeholder),
:global(#app .assets-index-view .ac-card-img-box .apm-empty) {
  width: 100%;
  height: 100%;
}

:global(#app .assets-index-view .ac-card-img-box .apm-placeholder-img) {
  max-width: 82%;
  max-height: 82%;
}

.ac-card-download-fab {
  position: absolute;
  right: 0.5rem;
  bottom: 0.5rem;
  z-index: 3;
  min-height: 1.95rem;
  padding: 0.38rem 0.66rem;
  border-radius: 999px ;
  border-color: rgb(var(--yb-brand) / 0.58) ;
  background: rgb(var(--yb-brand) / 0.92) ;
  color: rgb(var(--yb-surface)) ;
  font-size: 0.74rem ;
  font-weight: 850 ;
  line-height: 1 ;
  text-decoration: none ;
  box-shadow: 0 0.45rem 1.05rem rgb(var(--yb-brand) / 0.24) ;
  opacity: 0;
  transform: translateY(0.2rem);
  transition:
    opacity 0.16s ease,
    transform 0.16s ease,
    background 0.16s ease,
    border-color 0.16s ease,
    box-shadow 0.16s ease;
}

.ac-card:hover .ac-card-download-fab,
.ac-card:focus-within .ac-card-download-fab,
.ac-card-download-fab:focus-visible {
  opacity: 1;
  transform: translateY(0);
}

.ac-card-download-fab:hover {
  border-color: rgb(var(--yb-brand-border) / 0.95) ;
  background: rgb(var(--yb-brand-strong)) ;
  box-shadow: 0 0.55rem 1.25rem rgb(var(--yb-brand) / 0.34) ;
}

.ac-card-download-fab :deep(.asset-dl-icon) {
  width: 0.82rem;
  height: 0.82rem;
}

.ac-card-download-fab :deep(.asset-dl-text) {
  line-height: 1 ;
}

.ac-card-info {
  grid-area: info;
  display: flex;
  flex-direction: column;
  gap: 0.52rem;
  min-width: 0;
}

.ac-title-row {
  display: flex;
  flex-wrap: wrap;
  align-items: start;
  gap: 0.6rem;
  min-width: 0;
}

.ac-title-row .ac-card-title {
  flex: 1 1 9rem;
  min-width: 0;
  margin: 0;
  color: rgb(var(--yb-text-navy)) ;
  font-size: 1rem ;
  font-weight: 850 ;
  line-height: 1.3 ;
  letter-spacing: 0 ;
  max-height: 2.6rem;
  overflow: hidden;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
}

.ac-card-footer {
  grid-area: footer;
  display: grid;
  grid-template-columns: minmax(0, 0.78fr) minmax(0, 1.22fr);
  align-items: stretch;
  gap: 0.5rem;
  margin: 0;
  padding: 0;
  border: 0 ;
}

.ac-card-actions {
  grid-area: actions;
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(5.35rem, 1fr));
  align-items: flex-end;
  justify-content: flex-start;
  margin: 0;
  gap: 0.5rem;
}

.ac-footer-right {
  max-width: none;
  min-width: 0;
  text-align: left ;
}

.ac-mono,
.ac-card-spec,
.ac-card-footer > div,
.ac-footer-right {
  border-color: rgb(var(--yb-border)) ;
  background: rgb(var(--yb-surface-soft)) ;
  color: rgb(var(--yb-text)) ;
}

.ac-mono::before,
.ac-card-spec::before {
  color: rgb(var(--yb-text-muted)) ;
}

.ac-copy-tag,
.ac-card-link-btn,
.ac-icon-btn,
.ac-batch-btn,
.ac-pg-btn,
.ac-selected-remove,
.ac-page-size-select,
.ac-page-jump-input {
  border-color: rgb(var(--yb-border-strong)) ;
  background: rgb(var(--yb-surface)) ;
  color: rgb(var(--yb-text-body)) ;
  box-shadow: none ;
}

.ac-copy-tag:hover,
.ac-card-link-btn:hover {
  border-color: rgb(var(--yb-brand-border-strong)) ;
  background: rgb(var(--yb-brand-soft)) ;
  color: rgb(var(--yb-brand-strong)) ;
}

.ac-icon-btn--primary,
.ac-batch-btn--primary {
  background: rgb(var(--yb-brand)) ;
  border-color: rgb(var(--yb-brand)) ;
  color: rgb(var(--yb-surface)) ;
}

.ac-footer-label {
  color: rgb(var(--yb-text-muted)) ;
}

.ac-footer-tag,
.ac-format-pill {
  border: 1px solid rgb(var(--yb-brand-subtle)) ;
  background: rgb(var(--yb-surface-brand-preview)) ;
  color: rgb(var(--yb-brand-strong)) ;
}

.ac-card-footer .ac-footer-tag {
  display: -webkit-box;
  width: 100%;
  overflow: hidden;
  color: rgb(var(--yb-text-slate)) ;
  font-size: 0.78rem ;
  font-weight: 650 ;
  line-height: 1.35 ;
  white-space: normal;
  text-overflow: clip ;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
}

.ac-format-pill {
  align-self: start;
  flex: 0 1 auto;
  max-width: min(8rem, 100%);
  min-width: 0;
  overflow: hidden;
  padding: 0.24rem 0.48rem;
  border-radius: 999px ;
  font-size: 0.72rem ;
  font-weight: 800 ;
  line-height: 1.1 ;
  text-align: center;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ac-format-pill--delivery {
  border-color: rgb(var(--yb-brand-border)) ;
  background: rgb(var(--yb-brand-soft)) ;
  color: rgb(var(--yb-brand-strong)) ;
}

.ac-format-pill--source {
  border-color: rgb(var(--yb-purple-border)) ;
  background: rgb(var(--yb-purple-soft)) ;
  color: rgb(var(--yb-purple-text)) ;
}

.ac-format-pill--reference {
  border-color: rgb(var(--yb-warning-border-warm)) ;
  background: rgb(var(--yb-warning-orange-soft)) ;
  color: rgb(var(--yb-warning-orange-text)) ;
}

.ac-format-pill--preview {
  border-color: rgb(var(--yb-info-cyan-border)) ;
  background: rgb(var(--yb-info-cyan-soft)) ;
  color: rgb(var(--yb-info-cyan-text)) ;
}

.ac-format-pill--other {
  border-color: rgb(var(--yb-text-disabled)) ;
  background: rgb(var(--yb-surface-subtle)) ;
  color: rgb(var(--yb-text-soft)) ;
}

.ac-format-pill--module {
  border-color: rgb(var(--yb-border-strong)) ;
  background: rgb(var(--yb-surface-muted)) ;
  color: rgb(var(--yb-text-muted-strong)) ;
}

.ac-card-meta,
.ac-card-spec,
.state-text,
.preview-state-hint,
.ac-selected-meta {
  color: rgb(var(--yb-text-muted)) ;
}

.ac-business-row {
  display: grid;
  grid-template-columns: 2.45rem minmax(0, 1fr);
  align-items: center;
  gap: 0.45rem;
  min-height: 1.62rem;
  padding: 0.22rem 0;
  border: 0;
  border-radius: 0;
  background: transparent;
}

.ac-business-key {
  color: rgb(var(--yb-text-muted-strong));
  font-family: var(--yb-font-text);
  font-size: 0.72rem;
  font-weight: 700;
}

.ac-business-value {
  min-width: 0;
  overflow: hidden;
  color: rgb(var(--yb-text-navy));
  font-size: 0.8rem;
  font-weight: 650;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ac-footer-stat--operator {
  max-width: 100%;
  overflow: hidden;
  color: rgb(var(--yb-text)) ;
  font-size: 0.9rem ;
  font-weight: 800 ;
  letter-spacing: 0 ;
  line-height: 1.25 ;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ac-footer-stat--external {
  max-width: 10rem;
  font-size: 0.82rem ;
  line-height: 1.25 ;
  white-space: normal;
}

.ac-footer-stat.ac-usability--ready,
.ac-footer-stat.ac-usability--pending,
.ac-footer-stat.ac-usability--rejected,
.ac-footer-stat.ac-usability--history,
.ac-footer-stat.ac-usability--cleaned,
.ac-footer-stat.ac-usability--neutral {
  border: 0 ;
  background: transparent ;
}

.ac-footer-stat.ac-usability--ready {
  color: rgb(var(--yb-success-strong)) ;
}

.ac-footer-stat.ac-usability--pending {
  color: rgb(var(--yb-warning-text)) ;
}

.ac-footer-stat.ac-usability--rejected {
  color: rgb(var(--yb-danger-text)) ;
}

.ac-footer-stat.ac-usability--history,
.ac-footer-stat.ac-usability--cleaned,
.ac-footer-stat.ac-usability--neutral {
  color: rgb(var(--yb-text-muted-strong)) ;
}

.ac-card-link-btn {
  background: rgb(var(--yb-brand-soft)) ;
  border-color: rgb(var(--yb-brand-border)) ;
  color: rgb(var(--yb-brand)) ;
  min-width: 0;
  min-height: 2.3rem;
  padding-inline: 0.58rem;
  border-radius: 0.78rem ;
  font-weight: 800 ;
  white-space: nowrap;
}

.detail-business-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 0.9rem;
}

.ac-card-link-btn--task {
  background: rgb(var(--yb-brand)) ;
  border-color: rgb(var(--yb-brand)) ;
  color: rgb(var(--yb-surface)) ;
}

.ac-card-link-btn--edit {
  background: rgb(var(--yb-teal)) ;
  border-color: rgb(var(--yb-teal)) ;
  color: rgb(var(--yb-surface)) ;
}

.ac-card-link-btn:disabled {
  cursor: not-allowed ;
  opacity: 0.5 ;
}

@media (max-width: 1280px) {
  .ac-card {
    grid-template-columns: 8.25rem minmax(0, 1fr);
  }

  .ac-card-img-box {
    width: 8.25rem;
    min-height: 8.25rem;
  }
}

@media (max-width: 980px) {
  .ac-grid {
    grid-template-columns: repeat(auto-fill, minmax(min(100%, 22rem), 1fr));
  }

  .ac-card {
    min-height: 0;
    grid-template-columns: minmax(0, 1fr);
    grid-template-areas:
      "preview"
      "info"
      "actions";
  }

  .ac-card--external {
    grid-template-areas:
      "preview"
      "info"
      "footer"
      "actions";
  }

  .ac-card-img-box {
    width: 100%;
    min-height: 9.5rem;
    aspect-ratio: 16 / 10;
  }

  .ac-card-footer {
    grid-template-columns: minmax(0, 1fr);
  }

  .ac-card-download-fab {
    opacity: 1;
    transform: none;
  }
}

@media (max-width: 420px) {
  .ac-title-row {
    display: flex;
  }

  .ac-format-pill {
    justify-self: start;
  }

  .ac-card-actions {
    grid-template-columns: minmax(0, 1fr);
    align-items: stretch;
  }
}

</style>
