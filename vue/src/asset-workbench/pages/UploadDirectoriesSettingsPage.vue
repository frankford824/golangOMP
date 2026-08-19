<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { FolderCog, Plus } from 'lucide-vue-next'

import {
  assetWorkbenchApi,
  type DifficultyClassRow,
  type UploadDirectoryRow,
  type UpsertUploadDirectoryPayload,
} from '@aw/shared/api/assetWorkbenchApi'

interface DirectoryForm {
  name: string
  oss_prefix: string
  description: string
  difficulty_class: string
  allowed_file_types: string
  enabled: boolean
  sort_order: number
}

const directories = ref<UploadDirectoryRow[]>([])
const difficultyRows = ref<DifficultyClassRow[]>([])
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const notice = ref('')
const editorOpen = ref(false)
const editingID = ref<number | null>(null)
const pendingToggle = ref<UploadDirectoryRow | null>(null)
const form = ref<DirectoryForm>(emptyForm())

const difficultyOptions = computed(() => {
  const values = difficultyRows.value.filter((row) => row.enabled).map((row) => row.code)
  if (form.value.difficulty_class && !values.includes(form.value.difficulty_class)) values.push(form.value.difficulty_class)
  return values
})
const editorTitle = computed(() => editingID.value ? '编辑上传目录' : '新建上传目录')

function emptyForm(): DirectoryForm {
  return {
    name: '',
    oss_prefix: '',
    description: '',
    difficulty_class: '',
    allowed_file_types: '',
    enabled: true,
    sort_order: directories.value.length + 1,
  }
}

function makeDirectoryPrefix(name: string) {
  const ascii = name.trim().toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '')
  return ascii || `dir-${Date.now().toString(36)}`
}

function parseFileTypes(raw: string) {
  return [...new Set(raw.split(/[,\s，、]+/).map((value) => value.trim().toLowerCase().replace(/^\.+/, '')).filter(Boolean))]
}

function allowedFileTypesLabel(row: UploadDirectoryRow) {
  return row.allowed_file_types?.length ? row.allowed_file_types.join('、') : '全部格式'
}

async function loadDirectories() {
  loading.value = true
  error.value = ''
  try {
    const [nextDirectories, nextDifficulties] = await Promise.all([
      assetWorkbenchApi.listUploadDirectoriesAdmin(),
      assetWorkbenchApi.listDifficultyClasses(),
    ])
    directories.value = nextDirectories
    difficultyRows.value = nextDifficulties
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : '上传目录加载失败'
  } finally {
    loading.value = false
  }
}

function startCreate() {
  editingID.value = null
  form.value = emptyForm()
  form.value.difficulty_class = difficultyOptions.value[0] || ''
  editorOpen.value = true
  error.value = ''
}

function startEdit(row: UploadDirectoryRow) {
  editingID.value = row.id
  form.value = {
    name: row.name,
    oss_prefix: row.oss_prefix,
    description: row.description || '',
    difficulty_class: row.difficulty_class,
    allowed_file_types: row.allowed_file_types?.join(', ') || '',
    enabled: row.enabled,
    sort_order: row.sort_order,
  }
  editorOpen.value = true
  error.value = ''
}

function closeEditor() {
  if (saving.value) return
  editorOpen.value = false
  editingID.value = null
}

async function saveDirectory() {
  if (saving.value) return
  const name = form.value.name.trim()
  const difficulty = form.value.difficulty_class.trim()
  if (!name) {
    error.value = '请填写目录名称'
    return
  }
  if (!difficulty) {
    error.value = '请选择计价分类'
    return
  }
  const payload: UpsertUploadDirectoryPayload = {
    name,
    oss_prefix: form.value.oss_prefix.trim() || makeDirectoryPrefix(name),
    description: form.value.description.trim(),
    difficulty_class: difficulty,
    allowed_file_types: parseFileTypes(form.value.allowed_file_types),
    enabled: form.value.enabled,
    sort_order: Math.max(0, Number(form.value.sort_order) || 0),
  }
  saving.value = true
  error.value = ''
  notice.value = ''
  try {
    if (editingID.value) {
      await assetWorkbenchApi.updateUploadDirectory(editingID.value, payload)
      notice.value = `已更新上传目录：${name}`
    } else {
      await assetWorkbenchApi.createUploadDirectory(payload)
      notice.value = `已创建上传目录：${name}`
    }
    editorOpen.value = false
    editingID.value = null
    await loadDirectories()
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : '上传目录保存失败'
  } finally {
    saving.value = false
  }
}

async function confirmToggleDirectory() {
  const row = pendingToggle.value
  if (!row || saving.value) return
  saving.value = true
  error.value = ''
  notice.value = ''
  try {
    const updated = await assetWorkbenchApi.updateUploadDirectory(row.id, { enabled: !row.enabled })
    notice.value = updated.enabled ? `已恢复上传目录：${updated.name}` : `已停用上传目录：${updated.name}`
    pendingToggle.value = null
    await loadDirectories()
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : '上传目录状态更新失败'
  } finally {
    saving.value = false
  }
}

onMounted(loadDirectories)
</script>

<template>
  <section class="aw-page-stack aw-upload-directory-settings">
    <div class="aw-page-bar">
      <div class="aw-page-bar__copy">
        <p class="aw-eyebrow">上传中心配置</p>
        <h2>上传目录</h2>
        <p>统一维护上传中心的目录名称、存储路径、计价分类和允许格式。历史目录不硬删除，停用后不再出现在新上传选择中。</p>
      </div>
      <div class="aw-page-bar__actions">
        <button class="aw-primary-button" type="button" @click="startCreate">
          <Plus :size="16" aria-hidden="true" />
          新建目录
        </button>
      </div>
    </div>

    <p v-if="notice" class="aw-inline-alert">{{ notice }}</p>
    <p v-if="error" class="aw-inline-alert aw-inline-alert--error" role="alert">{{ error }}</p>

    <section v-if="editorOpen" class="aw-panel aw-upload-directory-editor" role="dialog" :aria-label="editorTitle">
      <div class="aw-panel__head">
        <div>
          <p class="aw-eyebrow">{{ editingID ? '修改目录' : '添加目录' }}</p>
          <h3>{{ editorTitle }}</h3>
        </div>
        <span class="aw-chip aw-chip--info">{{ editingID ? `目录 ID ${editingID}` : '新目录' }}</span>
      </div>
      <div class="aw-form-grid">
        <label class="aw-field">
          <span>目录名称</span>
          <input v-model.trim="form.name" aria-label="目录名称" placeholder="例如 A类定稿" required />
        </label>
        <label class="aw-field">
          <span>存储路径</span>
          <input v-model.trim="form.oss_prefix" aria-label="存储路径" placeholder="不填则按名称自动生成" />
          <small>修改路径只影响之后的新上传，历史文件仍保留原路径快照。</small>
        </label>
        <label class="aw-field">
          <span>计价分类</span>
          <select v-model="form.difficulty_class" aria-label="计价分类" required>
            <option value="" disabled>请选择</option>
            <option v-for="difficulty in difficultyOptions" :key="difficulty" :value="difficulty">{{ difficulty }}</option>
          </select>
        </label>
        <label class="aw-field">
          <span>允许格式</span>
          <input v-model.trim="form.allowed_file_types" aria-label="允许格式" placeholder="jpg, png, psd；不填则不限" />
        </label>
        <label class="aw-field">
          <span>排序</span>
          <input v-model.number="form.sort_order" type="number" min="0" aria-label="目录排序" />
        </label>
        <label class="aw-inline-check aw-upload-directory-editor__enabled">
          <input v-model="form.enabled" type="checkbox" />
          <span>启用后在上传中心显示</span>
        </label>
      </div>
      <label class="aw-field">
        <span>说明</span>
        <textarea v-model.trim="form.description" rows="3" aria-label="目录说明" placeholder="用途、文件要求或交付说明" />
      </label>
      <div class="aw-inline-actions">
        <button class="aw-primary-button" type="button" :disabled="saving" @click="saveDirectory">{{ saving ? '保存中…' : '保存目录' }}</button>
        <button class="aw-secondary-button" type="button" :disabled="saving" @click="closeEditor">取消</button>
      </div>
    </section>

    <section class="aw-data-surface">
      <div class="aw-grid-toolbar">
        <span>已配置 {{ directories.length }} 个上传目录</span>
        <button class="aw-grid-button" type="button" :disabled="loading" @click="loadDirectories">{{ loading ? '刷新中…' : '刷新' }}</button>
      </div>
      <p v-if="loading && !directories.length" class="aw-empty-state">正在加载上传目录…</p>
      <div v-else-if="directories.length" class="aw-upload-directory-settings__grid">
        <article v-for="row in directories" :key="row.id" class="aw-upload-directory-card" :class="{ 'is-disabled': !row.enabled }">
          <div class="aw-upload-directory-card__head">
            <span class="aw-upload-directory-card__icon"><FolderCog :size="18" aria-hidden="true" /></span>
            <div>
              <strong>{{ row.name }}</strong>
              <small>ID {{ row.id }} · 排序 {{ row.sort_order }}</small>
            </div>
            <span :class="row.enabled ? 'aw-chip aw-chip--success' : 'aw-chip aw-chip--neutral'">{{ row.enabled ? '启用中' : '已停用' }}</span>
          </div>
          <dl class="aw-upload-directory-card__meta">
            <div><dt>存储路径</dt><dd>{{ row.oss_prefix }}</dd></div>
            <div><dt>计价分类</dt><dd>{{ row.difficulty_class }}</dd></div>
            <div><dt>允许格式</dt><dd>{{ allowedFileTypesLabel(row) }}</dd></div>
            <div><dt>说明</dt><dd>{{ row.description || '—' }}</dd></div>
          </dl>
          <div class="aw-inline-actions">
            <button class="aw-secondary-button" type="button" @click="startEdit(row)">编辑</button>
            <button
              class="aw-secondary-button"
              :class="{ 'aw-secondary-button--danger': row.enabled }"
              type="button"
              @click="pendingToggle = row"
            >
              {{ row.enabled ? '停用目录' : '恢复启用' }}
            </button>
          </div>
        </article>
      </div>
      <div v-else class="aw-empty-state">
        <h3>还没有上传目录</h3>
        <p>点击“新建目录”配置上传中心的第一个目录。</p>
      </div>
    </section>

    <div v-if="pendingToggle" class="aw-modal-backdrop" @click.self="pendingToggle = null">
      <section class="aw-confirm-dialog" role="dialog" aria-modal="true" aria-label="确认上传目录状态">
        <p class="aw-eyebrow">{{ pendingToggle.enabled ? '安全删除' : '恢复目录' }}</p>
        <h3>{{ pendingToggle.enabled ? '确认停用上传目录' : '确认恢复上传目录' }}</h3>
        <p>
          {{ pendingToggle.enabled
            ? `${pendingToggle.name} 停用后不再出现在新上传选择中，历史文件和计价记录不会删除。`
            : `${pendingToggle.name} 恢复后会重新出现在上传中心。`
          }}
        </p>
        <div class="aw-inline-actions">
          <button class="aw-secondary-button" :class="{ 'aw-secondary-button--danger': pendingToggle.enabled }" type="button" :disabled="saving" @click="confirmToggleDirectory">
            {{ saving ? '处理中…' : pendingToggle.enabled ? '确认停用' : '确认恢复' }}
          </button>
          <button class="aw-secondary-button" type="button" :disabled="saving" @click="pendingToggle = null">取消</button>
        </div>
      </section>
    </div>
  </section>
</template>
