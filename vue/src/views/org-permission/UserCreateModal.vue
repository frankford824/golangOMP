<template>
  <div class="modal-mask" @click.self="emit('close')">
    <div class="modal-panel um-modal um-modal--wide">
      <header class="modal-header">
        <div class="modal-heading">
          <h3 class="section-title">新增用户</h3>
          <p class="modal-subtitle">创建账号与组织归属；工作角色在用户详情中统一配置</p>
        </div>
        <button type="button" class="modal-close" aria-label="关闭新增用户" @click="emit('close')">
          ×
        </button>
      </header>
      <div class="modal-body">
        <fieldset class="form-grid" :disabled="submitting || Boolean(createdUserId)">
          <input v-model.trim="form.username" class="input" placeholder="请输入用户名" />
          <input v-model.trim="form.employee_no" class="input" inputmode="numeric" placeholder="工号（0-9999）" />
          <input v-model.trim="form.display_name" class="input" placeholder="请输入姓名" />
          <select v-model="form.department" class="input">
            <option value="">选择部门</option>
            <option v-for="d in departmentOptions" :key="d.value" :value="d.value">{{ d.label }}</option>
          </select>
          <select v-model="form.team" class="input">
            <option value="">选择小组</option>
            <option v-for="t in teamOptions" :key="`${t.department ?? ''}-${t.value}`" :value="t.value">{{ t.label }}</option>
          </select>
          <input v-model.trim="form.mobile" class="input" placeholder="手机号" />
          <input v-model.trim="form.email" class="input" placeholder="邮箱(可选)" />
          <input v-model="form.password" class="input" type="password" placeholder="初始密码" />
          <select v-model="form.status" class="input">
            <option value="active">启用</option>
            <option value="disabled">已禁用</option>
          </select>
        </fieldset>
        <div v-if="oaOnboarding && form.status === 'active'" class="oa-form">
          <ManagerPicker v-model="managerId" empty-label="选择直属负责人" :disabled="submitting" />
          <div class="form-grid">
            <label class="field-label">入职日期<input v-model="joined" type="date" min="1000-01-01" :max="dateInChina()" :disabled="submitting" class="input" /></label>
            <label class="field-label">用工类型
              <select v-model="employmentType" class="input" :disabled="submitting || Boolean(createdUserId)">
                <option value="">请选择</option><option value="full_time">全职</option><option value="part_time">兼职</option>
              </select>
            </label>
          </div>
        </div>
        <p class="role-readonly-hint">新用户默认获得基础成员身份。创建后可在用户详情中勾选工作角色与数据范围。</p>
        <p v-if="error" class="action-msg">{{ error }}</p>
      </div>
      <footer class="modal-footer">
        <div class="modal-footer-actions">
          <button type="button" class="um-btn um-btn--ghost" @click="emit('close')">取消</button>
          <button type="button" class="um-btn um-btn--primary" :disabled="submitting" @click="emit('submit')">
            {{ submitting ? '处理中…' : createdUserId ? '继续办理入职' : '创建用户' }}
          </button>
        </div>
      </footer>
    </div>
  </div>
</template>

<script setup lang="ts">
import { defineAsyncComponent } from 'vue'
import { dateInChina } from '@/oa/api'
import type { CreateUserForm, SelectOptionItem } from './userManagementTypes'

const ManagerPicker = defineAsyncComponent(() => import('@/oa/components/ManagerPicker.vue'))
const managerId = defineModel<number>('managerId', { default: 0 })
const joined = defineModel<string>('joined', { default: '' })
const employmentType = defineModel<string>('employmentType', { default: '' })
defineProps<{
  form: CreateUserForm
  departmentOptions: SelectOptionItem[]
  teamOptions: SelectOptionItem[]
  error: string
  submitting: boolean
  oaOnboarding?: boolean
  createdUserId?: string
}>()

const emit = defineEmits<{
  close: []
  submit: []
}>()
</script>

<style scoped src="./userManagementModal.css"></style>
<style scoped>
fieldset.form-grid { padding: 0; margin: 0; border: 0; min-inline-size: 0; }
</style>
