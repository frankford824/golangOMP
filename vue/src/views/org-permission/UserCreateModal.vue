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
        <div class="form-grid">
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
        </div>
        <p class="role-readonly-hint">新用户默认获得基础成员身份。创建后可在用户详情中勾选工作角色与数据范围。</p>
        <p v-if="error" class="action-msg">{{ error }}</p>
      </div>
      <footer class="modal-footer">
        <div class="modal-footer-actions">
          <button type="button" class="um-btn um-btn--ghost" @click="emit('close')">取消</button>
          <button type="button" class="um-btn um-btn--primary" :disabled="submitting" @click="emit('submit')">
            {{ submitting ? '创建中...' : '创建用户' }}
          </button>
        </div>
      </footer>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { CreateUserForm, SelectOptionItem } from './userManagementTypes'

defineProps<{
  form: CreateUserForm
  departmentOptions: SelectOptionItem[]
  teamOptions: SelectOptionItem[]
  error: string
  submitting: boolean
}>()

const emit = defineEmits<{
  close: []
  submit: []
}>()
</script>

<style scoped src="./userManagementModal.css"></style>
