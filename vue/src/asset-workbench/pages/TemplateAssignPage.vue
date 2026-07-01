<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Plus, RefreshCw, Search, Send, Trash2, UsersRound, X } from 'lucide-vue-next'

import {
  assetWorkbenchApi,
  type WorkbenchGroupMemberRow,
  type WorkbenchGroupRow,
  type WorkbenchMemberRow,
  type WorkbenchTemplateAssignmentRow,
  type WorkbenchTemplateRow,
} from '@aw/shared/api/assetWorkbenchApi'
import { useRoutePageCopy } from '@aw/app/useRoutePageCopy'
import { usePageRequest } from '@aw/shared/composables/usePageRequest'
import { chipClass, workerTypeMeta } from '@aw/shared/format/status'
import AsyncBoundary from '@aw/shared/ui/AsyncBoundary.vue'

const saving = ref(false)
const notice = ref('')
const groups = ref<WorkbenchGroupRow[]>([])
const templates = ref<WorkbenchTemplateRow[]>([])
const assignments = ref<WorkbenchTemplateAssignmentRow[]>([])
const people = ref<WorkbenchMemberRow[]>([])
const groupMembers = ref<WorkbenchGroupMemberRow[]>([])

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
const peopleQuery = ref('')
const activeTemplate = ref<WorkbenchTemplateRow | null>(null)
const activeGroup = ref<WorkbenchGroupRow | null>(null)
const selectedPeople = ref<WorkbenchMemberRow[]>([])
const selectedGroups = ref<WorkbenchGroupRow[]>([])
const selectedGroupPeople = ref<WorkbenchMemberRow[]>([])
const { label: pageLabel } = useRoutePageCopy('/settings/dispatch')
const templateRequest = usePageRequest(
  async () => {
    const [groupRes, templateRes, assignmentRes] = await Promise.all([
      assetWorkbenchApi.listGroups({ page_size: 200 }),
      assetWorkbenchApi.listTemplates({ page_size: 200 }),
      assetWorkbenchApi.listTemplateAssignments({ page_size: 200, enabled: true }),
    ])
    return { groups: groupRes.items, templates: templateRes.items, assignments: assignmentRes.items }
  },
  null,
  '模板下发数据加载失败',
)
const loading = templateRequest.loading
const error = templateRequest.error

const enabledGroups = computed(() => groups.value.filter((item) => item.enabled))
const enabledTemplates = computed(() => templates.value.filter((item) => item.enabled))
const selectedReachCount = computed(() => selectedPeople.value.length + selectedGroups.value.length)

function personName(person: WorkbenchMemberRow | WorkbenchGroupMemberRow) {
  return person.real_name || person.display_name || person.username || '未命名'
}

function resetMessage() {
  notice.value = ''
  error.value = ''
}

function isPersonSelected(person: WorkbenchMemberRow, source = selectedPeople.value) {
  return source.some((item) => item.user_id === person.user_id)
}

function isGroupSelected(group: WorkbenchGroupRow) {
  return selectedGroups.value.some((item) => item.id === group.id)
}

function togglePerson(person: WorkbenchMemberRow) {
  if (isPersonSelected(person)) {
    selectedPeople.value = selectedPeople.value.filter((item) => item.user_id !== person.user_id)
    return
  }
  selectedPeople.value = [...selectedPeople.value, person]
}

function toggleGroupPerson(person: WorkbenchMemberRow) {
  if (isPersonSelected(person, selectedGroupPeople.value)) {
    selectedGroupPeople.value = selectedGroupPeople.value.filter((item) => item.user_id !== person.user_id)
    return
  }
  selectedGroupPeople.value = [...selectedGroupPeople.value, person]
}

function toggleGroup(group: WorkbenchGroupRow) {
  if (isGroupSelected(group)) {
    selectedGroups.value = selectedGroups.value.filter((item) => item.id !== group.id)
    return
  }
  selectedGroups.value = [...selectedGroups.value, group]
}

function openAssign(template: WorkbenchTemplateRow) {
  activeTemplate.value = template
  selectedPeople.value = []
  selectedGroups.value = []
  resetMessage()
}

async function loadAll() {
  resetMessage()
  const data = await templateRequest.run()
  if (!data) return
  groups.value = data.groups
  templates.value = data.templates
  assignments.value = data.assignments
  if (!activeGroup.value && enabledGroups.value[0]) {
    activeGroup.value = enabledGroups.value[0]
    await loadGroupMembers(enabledGroups.value[0])
  }
}

async function searchPeople() {
  if (!peopleQuery.value.trim()) {
    people.value = []
    return
  }
  resetMessage()
  try {
    const result = await assetWorkbenchApi.searchPeople({ q: peopleQuery.value, page_size: 20 })
    people.value = result.items
  } catch (err) {
    error.value = err instanceof Error ? err.message : '人员搜索失败'
  }
}

async function createGroup() {
  if (!groupForm.value.name.trim()) return
  saving.value = true
  resetMessage()
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
  resetMessage()
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
    notice.value = '旧模板已创建'
    await loadAll()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '旧模板创建失败'
  } finally {
    saving.value = false
  }
}

async function loadGroupMembers(group: WorkbenchGroupRow) {
  activeGroup.value = group
  selectedGroupPeople.value = []
  resetMessage()
  try {
    groupMembers.value = await assetWorkbenchApi.listGroupMembers(group.id)
  } catch (err) {
    error.value = err instanceof Error ? err.message : '分组成员加载失败'
  }
}

async function addMembersToGroup() {
  if (!activeGroup.value || selectedGroupPeople.value.length === 0) return
  saving.value = true
  resetMessage()
  try {
    await assetWorkbenchApi.addGroupMembers(
      activeGroup.value.id,
      selectedGroupPeople.value.map((item) => item.user_id),
    )
    notice.value = '成员已加入分组'
    await loadGroupMembers(activeGroup.value)
  } catch (err) {
    error.value = err instanceof Error ? err.message : '成员加入失败'
  } finally {
    saving.value = false
  }
}

async function removeMemberFromGroup(member: WorkbenchGroupMemberRow) {
  if (!activeGroup.value) return
  saving.value = true
  resetMessage()
  try {
    await assetWorkbenchApi.removeGroupMembers(activeGroup.value.id, [member.user_id])
    notice.value = '成员已移出分组'
    await loadGroupMembers(activeGroup.value)
  } catch (err) {
    error.value = err instanceof Error ? err.message : '成员移出失败'
  } finally {
    saving.value = false
  }
}

async function assignTemplate() {
  if (!activeTemplate.value || selectedReachCount.value === 0) return
  saving.value = true
  resetMessage()
  try {
    await assetWorkbenchApi.assignTemplate({
      template_id: activeTemplate.value.id,
      user_ids: selectedPeople.value.map((item) => item.user_id),
      group_ids: selectedGroups.value.map((item) => item.id),
    })
    notice.value = `已下发给 ${selectedReachCount.value} 个对象`
    activeTemplate.value = null
    selectedPeople.value = []
    selectedGroups.value = []
    await loadAll()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '下发失败'
  } finally {
    saving.value = false
  }
}

async function removeAssignment(item: WorkbenchTemplateAssignmentRow) {
  saving.value = true
  resetMessage()
  try {
    await assetWorkbenchApi.deleteTemplateAssignment(item.id)
    notice.value = '下发记录已撤销'
    await loadAll()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '撤销失败'
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
        <p class="aw-eyebrow">设置</p>
        <h2>{{ pageLabel }}</h2>
        <p>新上传按上传目录计价；这里仅保留旧模板和历史下发关系，不作为客户端上传前置条件。</p>
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

    <div class="aw-two-column">
      <section class="aw-panel">
        <div class="aw-panel__head">
          <div>
            <p class="aw-eyebrow">兼容模板</p>
            <h3>新建旧模板</h3>
          </div>
          <Plus :size="18" aria-hidden="true" />
        </div>
        <div class="aw-form-grid">
          <label>
            <span>模板名称</span>
            <input v-model="templateForm.name" placeholder="如：海报 / 视频 / 小夜灯" />
          </label>
          <label>
            <span>分类</span>
            <input v-model="templateForm.category" placeholder="可不填" />
          </label>
          <label>
            <span>计价类</span>
            <input v-model="templateForm.difficulty_class" placeholder="A / B / C" />
          </label>
          <label>
            <span>人员类型</span>
            <select v-model="templateForm.worker_type">
              <option value="">不限</option>
              <option value="fulltime">全职</option>
              <option value="parttime">兼职</option>
            </select>
          </label>
          <button class="aw-primary-button aw-form-grid__full" type="button" :disabled="saving" @click="createTemplate">新增旧模板</button>
        </div>
      </section>

      <section class="aw-panel">
        <div class="aw-panel__head">
          <div>
            <p class="aw-eyebrow">人员分组</p>
            <h3>新建分组</h3>
          </div>
          <UsersRound :size="18" aria-hidden="true" />
        </div>
        <div class="aw-form-grid">
          <label>
            <span>分组名</span>
            <input v-model="groupForm.name" placeholder="如：兼职海报组" />
          </label>
          <label>
            <span>说明</span>
            <input v-model="groupForm.description" placeholder="可不填" />
          </label>
          <button class="aw-primary-button aw-form-grid__full" type="button" :disabled="saving" @click="createGroup">新增分组</button>
        </div>
      </section>
    </div>

    <section class="aw-data-surface">
      <div class="aw-panel__head">
        <div>
          <p class="aw-eyebrow">历史下发</p>
          <h3>旧模板</h3>
        </div>
        <span class="aw-chip aw-chip--neutral">{{ enabledTemplates.length }} 个启用</span>
      </div>
      <AsyncBoundary
        :loading="loading"
        :error="error"
        loading-label="正在加载旧模板"
        @retry="loadAll"
      >
        <div v-if="templates.length" class="aw-compact-list">
          <article v-for="item in templates" :key="item.id" class="aw-compact-list__item">
            <div>
              <strong>{{ item.name }}</strong>
              <span>{{ item.category || '未分类' }} · {{ item.difficulty_class }} · {{ workerTypeMeta(item.worker_type || 'all').label }}</span>
            </div>
            <div class="aw-page-bar__actions">
              <span :class="chipClass(item.enabled ? 'success' : 'neutral')">{{ item.enabled ? '启用' : '停用' }}</span>
              <button class="aw-secondary-button" type="button" :disabled="!item.enabled" @click="openAssign(item)">
                <Send :size="16" aria-hidden="true" />
                下发
              </button>
            </div>
          </article>
        </div>
        <div v-else class="aw-empty-state">
          <h3>还没有旧模板</h3>
          <p>旧模板仅用于兼容历史下发；新上传请在素材库里维护上传目录。</p>
        </div>
      </AsyncBoundary>
    </section>

    <section v-if="activeTemplate" class="aw-panel aw-panel--stage">
      <div class="aw-panel__head">
        <div>
          <p class="aw-eyebrow">选择对象</p>
          <h3>下发旧模板「{{ activeTemplate.name }}」</h3>
        </div>
        <button class="aw-secondary-button" type="button" @click="activeTemplate = null">
          <X :size="16" aria-hidden="true" />
          关闭
        </button>
      </div>
      <div class="aw-two-column">
        <div class="aw-panel">
          <div class="aw-panel__head">
            <div>
              <p class="aw-eyebrow">人员</p>
              <h3>按姓名搜索</h3>
            </div>
          </div>
          <div class="aw-form-grid">
            <label class="aw-form-grid__full">
              <span>搜索</span>
              <input v-model="peopleQuery" placeholder="输入姓名、账号或手机号" @keydown.enter="searchPeople" />
            </label>
            <button class="aw-secondary-button" type="button" @click="searchPeople">
              <Search :size="16" aria-hidden="true" />
              搜索
            </button>
          </div>
          <div class="aw-compact-list">
            <button
              v-for="person in people"
              :key="person.user_id"
              class="aw-secondary-button"
              type="button"
              :class="{ 'aw-template-option--active': isPersonSelected(person) }"
              @click="togglePerson(person)"
            >
              {{ personName(person) }} · {{ person.username }}
            </button>
          </div>
        </div>

        <div class="aw-panel">
          <div class="aw-panel__head">
            <div>
              <p class="aw-eyebrow">分组</p>
              <h3>选择分组</h3>
            </div>
          </div>
          <div class="aw-compact-list">
            <button
              v-for="group in enabledGroups"
              :key="group.id"
              class="aw-secondary-button"
              type="button"
              :class="{ 'aw-template-option--active': isGroupSelected(group) }"
              @click="toggleGroup(group)"
            >
              {{ group.name }}
            </button>
          </div>
        </div>
      </div>

      <div class="aw-panel">
        <div class="aw-panel__head">
          <div>
            <p class="aw-eyebrow">确认</p>
            <h3>将应用到 {{ selectedReachCount }} 个对象</h3>
          </div>
          <button class="aw-primary-button" type="button" :disabled="saving || selectedReachCount === 0" @click="assignTemplate">
            <Send :size="16" aria-hidden="true" />
            确认下发旧模板
          </button>
        </div>
        <div class="aw-compact-list">
          <article v-for="person in selectedPeople" :key="`p-${person.user_id}`" class="aw-compact-list__item">
            <div>
              <strong>{{ personName(person) }}</strong>
              <span>人员</span>
            </div>
            <button class="aw-secondary-button" type="button" @click="togglePerson(person)">移除</button>
          </article>
          <article v-for="group in selectedGroups" :key="`g-${group.id}`" class="aw-compact-list__item">
            <div>
              <strong>{{ group.name }}</strong>
              <span>分组</span>
            </div>
            <button class="aw-secondary-button" type="button" @click="toggleGroup(group)">移除</button>
          </article>
        </div>
      </div>
    </section>

    <div class="aw-two-column">
      <section class="aw-data-surface">
        <div class="aw-panel__head">
          <div>
            <p class="aw-eyebrow">分组成员</p>
            <h3>人员分组</h3>
          </div>
        </div>
        <div class="aw-compact-list">
          <button
            v-for="group in enabledGroups"
            :key="group.id"
            class="aw-secondary-button"
            type="button"
            :class="{ 'aw-template-option--active': activeGroup?.id === group.id }"
            @click="loadGroupMembers(group)"
          >
            {{ group.name }}
          </button>
        </div>
        <div v-if="activeGroup" class="aw-panel">
          <div class="aw-panel__head">
            <div>
              <p class="aw-eyebrow">加入</p>
              <h3>{{ activeGroup.name }}</h3>
            </div>
            <button class="aw-primary-button" type="button" :disabled="saving || selectedGroupPeople.length === 0" @click="addMembersToGroup">
              加入 {{ selectedGroupPeople.length }} 人
            </button>
          </div>
          <div class="aw-compact-list">
            <button
              v-for="person in people"
              :key="`gm-${person.user_id}`"
              class="aw-secondary-button"
              type="button"
              :class="{ 'aw-template-option--active': isPersonSelected(person, selectedGroupPeople) }"
              @click="toggleGroupPerson(person)"
            >
              {{ personName(person) }} · {{ person.username }}
            </button>
          </div>
          <div class="aw-compact-list">
            <article v-for="member in groupMembers" :key="member.user_id" class="aw-compact-list__item">
              <div>
                <strong>{{ personName(member) }}</strong>
                <span>{{ member.username }} · {{ workerTypeMeta(member.worker_type || 'all').label }} · {{ member.job_grade || '未定级' }}</span>
              </div>
              <button class="aw-secondary-button" type="button" :disabled="saving" @click="removeMemberFromGroup(member)">
                <Trash2 :size="16" aria-hidden="true" />
                移出
              </button>
            </article>
          </div>
        </div>
      </section>

      <section class="aw-data-surface">
        <div class="aw-panel__head">
          <div>
            <p class="aw-eyebrow">记录</p>
            <h3>旧模板下发记录</h3>
          </div>
        </div>
        <div class="aw-compact-list">
          <article v-for="item in assignments" :key="item.id" class="aw-compact-list__item">
            <div>
              <strong>{{ item.template_name || `旧模板 ${item.template_id}` }}</strong>
              <span>{{ item.target_type === 'group' ? '分组' : '人员' }} · {{ item.target_name || '未命名' }}</span>
            </div>
            <div class="aw-page-bar__actions">
              <span :class="chipClass(item.enabled ? 'success' : 'neutral')">{{ item.enabled ? '已下发' : '已撤销' }}</span>
              <button v-if="item.enabled" class="aw-secondary-button" type="button" :disabled="saving" @click="removeAssignment(item)">撤销</button>
            </div>
          </article>
        </div>
      </section>
    </div>
  </section>
</template>
