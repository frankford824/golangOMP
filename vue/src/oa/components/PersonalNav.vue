<template>
  <nav class="personal-nav" aria-label="个人中心">
    <RouterLink v-for="link in links" :key="link.name" :to="{ name: link.name }">
      {{ link.label }}
    </RouterLink>
  </nav>
</template>

<script setup lang="ts">
const links = [
  { name: 'Me', label: '基本资料' },
  { name: 'MeSecurity', label: '安全设置' },
  { name: 'MeOrg', label: '我的组织' },
  { name: 'MeNotifications', label: '通知中心' },
  { name: 'MeTaskDrafts', label: '任务草稿' },
]
</script>

<style scoped>
.personal-nav {
  display: flex;
  align-items: center;
  gap: 8px;
  overflow-x: auto;
  padding: 0 8px;
  border-bottom: 1px solid rgb(var(--yb-border));
  background: rgb(var(--yb-surface));
  flex-shrink: 0;
}
.personal-nav a {
  display: inline-flex;
  align-items: center;
  min-height: 40px;
  padding: 0 10px;
  border-bottom: 2px solid transparent;
  color: rgb(var(--yb-text-muted));
  text-decoration: none;
  white-space: nowrap;
  font-size: 14px;
  font-weight: 500;
}
.personal-nav a.router-link-exact-active {
  color: var(--oa-primary, rgb(var(--yb-brand)));
  border-color: currentColor;
}
.personal-nav a:focus-visible { outline: 2px solid currentColor; outline-offset: -2px; }
</style>
