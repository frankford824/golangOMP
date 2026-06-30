<script setup lang="ts">
import { RouterLink } from 'vue-router'
import { Boxes, HandCoins, ImagePlus } from 'lucide-vue-next'
</script>

<template>
  <section class="aw-simple-home">
    <div class="aw-simple-hero">
      <p class="aw-eyebrow">今天要做什么</p>
      <h1>选一个入口就能开始</h1>
      <p>交作品会自动套用你能选择的类型；看收入只显示你自己的金额。</p>
    </div>

    <div class="aw-simple-card-grid">
      <RouterLink class="aw-simple-card aw-simple-card--primary" to="/upload">
        <span class="aw-simple-card__icon">
          <ImagePlus :size="32" aria-hidden="true" />
        </span>
        <span>
          <strong>交作品</strong>
          <small>上传做好的文件</small>
        </span>
      </RouterLink>
      <RouterLink class="aw-simple-card aw-simple-card--money" to="/my-settlement">
        <span class="aw-simple-card__icon">
          <HandCoins :size="32" aria-hidden="true" />
        </span>
        <span>
          <strong>看收入</strong>
          <small>查看这个月能拿多少</small>
        </span>
      </RouterLink>
      <RouterLink class="aw-simple-card" to="/submissions">
        <span class="aw-simple-card__icon">
          <Boxes :size="32" aria-hidden="true" />
        </span>
        <span>
          <strong>上传记录</strong>
          <small>查看已提交的文件</small>
        </span>
      </RouterLink>
    </div>
  </section>
</template>
