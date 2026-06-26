<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RefreshCw, Send, UsersRound } from 'lucide-vue-next'

import {
  assetWorkbenchApi,
  type WorkbenchGroupRow,
  type WorkbenchTemplateAssignmentRow,
  type WorkbenchTemplateRow,
} from '@aw/shared/api/assetWorkbenchApi'

const loading = ref(false)
const saving = ref(false)
const notice = ref('')
const error = ref('')
const groups = ref<WorkbenchGroupRow[]>([])
const templates = ref<WorkbenchTemplateRow[]>([])
const assignments = ref<WorkbenchTemplateAssignmentRow[]>([])

const groupForm = ref({
  name: '',
  description: '',
})
const templateForm = ref({
  name: '',
  category: '',
  difficulty_class: 'A',
  worker_type: '',
  sort_order: 0,
})
const assignmentForm = ref({
  template_id: 0,
  user_ids: '',
  group_ids: '',
})
const memberForm = ref({
  group_id: 0,
  user_ids: '',
})

const enabledTemplates = computed(() => templates.value.filter((item) => item.enabled))
const enabledGroups = computed(() => groups.value.filter((item) => item.enabled))

function parseIDs(raw: string) {
  return raw
    .split(/[,，\s]+/)
    .map((item) => Number(item.trim()))
    .filter((item, index, source) => Number.isFinite(item) && item > 0 && source.indexOf(item) === index)
}

async function loadAll() {
  loading.value = true
  error.value = ''
  try {
    const [groupRes, templateRes, assignmentRes] = await Promise.all([
      assetWorkbenchApi.listGroups({ page_size: 200 }),
      assetWorkbenchApi.listTemplates({ page_size: 200 }),
      assetWorkbenchApi.listTemplateAssignments({ page_size: 200 }),
    ])
    groups.value = groupRes.items
    templates.value = templateRes.items
    assignments.value = assignmentRes.items
    if (!assignmentForm.value.template_id && enabledTemplates.value[0]) {
      assignmentForm.value.template_id = enabledTemplates.value[0].id
    }
    if (!memberForm.value.group_id && enabledGroups.value[0]) {
      memberForm.value.group_id = enabledGroups.value[0].id
    }
  } catch (err) {
    error.value = err instanceof Error ? err.message : '模板下发数据加载失败'
  } finally {
    loading.value = false
  }
}

async function createGroup() {
  if (!groupForm.value.name.trim()) return
  saving.value = true
  error.value = ''
  try {
    await assetWorkbenchApi.createGroup({
      name: groupForm.value.name,
      description: groupForm.value.description,
      enabled: true,
    })
    groupForm.value = { name: '', description: '' }
    notice.value = '分组已创建'
    await loadAll()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '分组创建失败'
  } finally {
    saving.value = false
  }
}

async function createTemplate() {
  if (!templateForm.value.name.trim()) return
  saving.value = true
  error.value = ''
  try {
    await assetWorkbenchApi.createTemplate({
      name: templateForm.value.name,
      category: templateForm.value.category,
      difficulty_class: templateForm.value.difficulty_class,
      worker_type: templateForm.value.worker_type,
      sort_order: templateForm.value.sort_order,
      enabled: true,
    })
    templateForm.value = { name: '', category: '', difficulty_class: 'A', worker_type: '', sort_order: 0 }
    notice.value = '作品类型已创建'
    await loadAll()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '作品类型创建失败'
  } finally {
    saving.value = false
  }
}

async function addMembers() {
  const userIds = parseIDs(memberForm.value.user_ids)
  if (!memberForm.value.group_id || userIds.length === 0) return
  saving.value = true
  error.value = ''
  try {
    await assetWorkbenchApi.addGroupMembers(memberForm.value.group_id, userIds)
    memberForm.value.user_ids = ''
    notice.value = '成员已加入分组'
    await loadAll()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '成员加入失败'
  } finally {
    saving.value = false
  }
}

async function assignTemplate() {
  const userIds = parseIDs(assignmentForm.value.user_ids)
  const groupIds = parseIDs(assignmentForm.value.group_ids)
  if (!assignmentForm.value.template_id || (userIds.length === 0 && groupIds.length === 0)) return
  saving.value = true
  error.value = ''
  try {
    await assetWorkbenchApi.assignTemplate({
      template_id: assignmentForm.value.template_id,
      user_ids: userIds,
      group_ids: groupIds,
    })
    assignmentForm.value.user_ids = ''
    assignmentForm.value.group_ids = ''
    notice.value = '下发完成'
    await loadAll()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '下发失败'
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  void loadAll()
})
</script>

<template>
  <section class="aw-page-stack">
    <div class="aw-page-bar">
      <div class="aw-page-bar__copy">
        <p class="aw-eyebrow">类型与分组</p>
        <h2>模板下发</h2>
        <p>管理员维护作品类型和人员分组，再按人或按组下发。普通用户上传时只会看到分配给自己的类型。</p>
      </div>
      <div class="aw-page-bar__actions">
        <button class="aw-secondary-button" type="button" :disabled="loading" @click="loadAll">
          <RefreshCw :size="16" aria-hidden="true" />
          刷新
        </button>
      </div>
    </div>

    <p v-if="error" class="aw-inline-alert">{{ error }}</p>
    <p v-else-if="notice" class="aw-inline-alert">{{ notice }}</p>

    <div class="aw-three-column">
      <div class="aw-panel">
        <div class="aw-panel__head">
          <div>
            <p class="aw-eyebrow">分组</p>
            <h3>人员分组</h3>
          </div>
        </div>
        <div class="aw-form-grid">
          <label>
            <span>分组名</span>
            <input v-model="groupForm.name" />
          </label>
          <label>
            <span>说明</span>
            <input v-model="groupForm.description" />
          </label>
          <button class="aw-primary-button aw-form-grid__full" type="button" :disabled="saving" @click="createGroup">新增分组</button>
        </div>
      </div>

      <div class="aw-panel">
        <div class="aw-panel__head">
          <div>
            <p class="aw-eyebrow">类型</p>
            <h3>作品类型</h3>
          </div>
        </div>
        <div class="aw-form-grid">
          <label>
            <span>名称</span>
            <input v-model="templateForm.name" />
          </label>
          <label>
            <span>分类</span>
            <input v-model="templateForm.category" />
          </label>
          <label>
            <span>难度类</span>
            <input v-model="templateForm.difficulty_class" />
          </label>
          <label>
            <span>人员类型</span>
            <select v-model="templateForm.worker_type">
              <option value="">不限</option>
              <option value="fulltime">全职</option>
              <option value="parttime">兼职</option>
            </select>
          </label>
          <button class="aw-primary-button aw-form-grid__full" type="button" :disabled="saving" @click="createTemplate">新增类型</button>
        </div>
      </div>

      <div class="aw-panel">
        <div class="aw-panel__head">
          <div>
            <p class="aw-eyebrow">下发</p>
            <h3>批量下发</h3>
          </div>
        </div>
        <div class="aw-form-grid">
          <label class="aw-form-grid__full">
            <span>作品类型</span>
            <select v-model.number="assignmentForm.template_id">
              <option v-for="item in enabledTemplates" :key="item.id" :value="item.id">{{ item.name }}</option>
            </select>
          </label>
          <label>
            <span>用户 ID</span>
            <input v-model="assignmentForm.user_ids" placeholder="用逗号分隔" />
          </label>
          <label>
            <span>分组 ID</span>
            <input v-model="assignmentForm.group_ids" placeholder="用逗号分隔" />
          </label>
          <button class="aw-primary-button aw-form-grid__full" type="button" :disabled="saving" @click="assignTemplate">
            <Send :size="16" aria-hidden="true" />
            下发
          </button>
        </div>
      </div>
    </div>

    <div class="aw-panel">
      <div class="aw-panel__head">
        <div>
          <p class="aw-eyebrow">成员</p>
          <h3>加入分组</h3>
        </div>
        <UsersRound :size="18" aria-hidden="true" />
      </div>
      <div class="aw-form-grid">
        <label>
          <span>分组</span>
          <select v-model.number="memberForm.group_id">
            <option v-for="item in enabledGroups" :key="item.id" :value="item.id">{{ item.name }}</option>
          </select>
        </label>
        <label>
          <span>用户 ID</span>
          <input v-model="memberForm.user_ids" placeholder="用逗号分隔" />
        </label>
        <button class="aw-secondary-button" type="button" :disabled="saving" @click="addMembers">加入</button>
      </div>
    </div>

    <div class="aw-two-column">
      <div class="aw-data-surface">
        <div class="aw-panel__head">
          <div>
            <p class="aw-eyebrow">已建</p>
            <h3>作品类型</h3>
          </div>
        </div>
        <div class="aw-compact-list">
          <article v-for="item in templates" :key="item.id" class="aw-compact-list__item">
            <div>
              <strong>{{ item.name }}</strong>
              <span>{{ item.category || '未分类' }} · {{ item.difficulty_class }}</span>
            </div>
            <span class="aw-chip" :class="item.enabled ? 'aw-chip--success' : 'aw-chip--neutral'">{{ item.enabled ? '启用' : '停用' }}</span>
          </article>
        </div>
      </div>
      <div class="aw-data-surface">
        <div class="aw-panel__head">
          <div>
            <p class="aw-eyebrow">记录</p>
            <h3>下发记录</h3>
          </div>
        </div>
        <div class="aw-compact-list">
          <article v-for="item in assignments" :key="item.id" class="aw-compact-list__item">
            <div>
              <strong>模板 {{ item.template_id }}</strong>
              <span>{{ item.target_type === 'group' ? '分组' : '用户' }} {{ item.target_id }}</span>
            </div>
            <span class="aw-chip" :class="item.enabled ? 'aw-chip--success' : 'aw-chip--neutral'">{{ item.enabled ? '已下发' : '已停用' }}</span>
          </article>
        </div>
      </div>
    </div>
  </section>
</template>
