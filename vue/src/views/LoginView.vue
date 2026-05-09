<!--
  @deprecated 此文件已废弃
  登录和注册功能已迁移至 AuthView.vue
  该文件保留仅作为备份，请勿继续使用
  路由 /login 现已指向 AuthView.vue
-->

<template>
  <div class="flex h-screen w-full items-center justify-center bg-slate-50">
    <div class="text-center">
      <h1 class="text-lg font-medium text-slate-900 mb-2">页面已迁移</h1>
      <p class="text-sm text-slate-500 mb-4">
        登录功能已整合至新的 AuthView.vue
      </p>
      <router-link
        to="/login"
        class="inline-block px-4 py-2 bg-slate-900 text-white text-sm rounded-lg"
      >
        前往登录页
      </router-link>
    </div>
  </div>
</template>

<script setup lang="ts">
// 此组件已废弃，所有登录逻辑已迁移至 AuthView.vue
</script>
