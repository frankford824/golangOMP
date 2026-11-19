<template>
  <div class="forbidden-page min-h-[100dvh] flex items-center justify-center bg-[rgb(var(--yb-surface-soft))]">
    <div class="forbidden-card">
      <div class="icon-wrap">
        <div class="icon-circle">
          <span class="icon-bar" />
        </div>
      </div>
      <h1 class="title">403 - 无权限访问</h1>
      <p class="desc">
        您当前角色无权访问此页面，如需访问请联系管理员或尝试切换账号。
      </p>
      <div class="mt-4 flex items-center justify-center gap-3">
        <OAPageReturn v-if="oaEnabled" :to="homeTarget || { name: 'Me' }">返回首页</OAPageReturn>
        <BaseButton v-else variant="secondary" size="sm" @click="goHome">
          返回首页
        </BaseButton>
        <BaseButton variant="primary" size="sm" @click="goLogin">
          {{ oaEnabled ? '前往登录' : '返回登录' }}
        </BaseButton>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { oaEnabled } from '@/oa/access'
import OAPageReturn from '@/oa/components/OAPageReturn.vue'
import { goBackTo } from '@/oa/returnNavigation'
import BaseButton from '@/components/base/BaseButton.vue'
import { usePermissionsStore } from '@/stores/permissions'
import { resolveFirstAccessibleHomeRoute } from '@/router/home-fallback'

const router = useRouter()
const permissionsStore = usePermissionsStore()

const homeTarget = computed(() => resolveFirstAccessibleHomeRoute(permissionsStore))

function goHome() {
  return router.push(homeTarget.value ?? { name: 'Forbidden' })
}

function goLogin() {
  if (oaEnabled) return goBackTo(router, { name: 'Login' })
  return router.push({ name: 'Login' })
}
</script>

<style scoped>
.forbidden-card {
  padding: 2rem;
  background: rgb(var(--yb-surface));
  border-radius: 0.75rem;
  border: 1px solid rgb(var(--yb-border-slate));
  text-align: center;
  max-width: 360px;
}
.title {
  margin: 0 0 0.75rem;
  font-size: 1.25rem;
  font-weight: 600;
  color: rgb(var(--yb-text-navy));
}
.desc {
  margin: 0;
  font-size: 0.875rem;
  color: rgb(var(--yb-text-muted-strong));
}

.icon-wrap {
  display: flex;
  justify-content: center;
  margin-bottom: 1.25rem;
}

.icon-circle {
  position: relative;
  width: 56px;
  height: 56px;
  border-radius: 9999px;
  border: 2px solid rgb(var(--yb-danger-coral));
  background: rgb(var(--yb-danger-soft));
  display: flex;
  align-items: center;
  justify-content: center;
}

.icon-bar {
  position: absolute;
  width: 32px;
  height: 2px;
  background-color: rgb(var(--yb-red));
  transform: rotate(-45deg);
}
</style>

