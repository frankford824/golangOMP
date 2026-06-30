<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { RouterLink, RouterView, useRoute, useRouter } from 'vue-router'

import { useAssetWorkbenchBootstrap } from '@aw/app/useAssetWorkbenchBootstrap'
import { visibleSettingsNavItems } from '@aw/app/navigation'

const route = useRoute()
const router = useRouter()
const tablistRef = ref<HTMLElement | null>(null)
const { bootstrap } = useAssetWorkbenchBootstrap()

const tabs = computed(() => visibleSettingsNavItems(bootstrap.value))
const peopleTabs = computed(() => tabs.value.filter((tab) => tab.to === '/settings/people' || tab.to === '/settings/members'))
const otherTabs = computed(() => tabs.value.filter((tab) => tab.to !== '/settings/people' && tab.to !== '/settings/members'))

function isActive(path: string) {
  return route.path === path
}

async function goTab(path: string) {
  await router.push(path)
}

function onTabKeydown(event: KeyboardEvent, index: number, source: typeof tabs.value) {
  const total = source.length
  if (!total) return
  if (event.key === 'ArrowRight') {
    event.preventDefault()
    void goTab(source[(index + 1) % total].to)
    return
  }
  if (event.key === 'ArrowLeft') {
    event.preventDefault()
    void goTab(source[(index - 1 + total) % total].to)
    return
  }
  if (event.key === 'Home') {
    event.preventDefault()
    void goTab(source[0].to)
    return
  }
  if (event.key === 'End') {
    event.preventDefault()
    void goTab(source[total - 1].to)
  }
}

async function scrollActiveIntoView() {
  await nextTick()
  const activeEl = tablistRef.value?.querySelector<HTMLElement>('[aria-selected="true"]')
  activeEl?.scrollIntoView({ block: 'nearest', inline: 'nearest' })
}

watch(
  () => route.path,
  () => {
    void scrollActiveIntoView()
  },
)

onMounted(() => {
  void scrollActiveIntoView()
})
</script>

<template>
  <section class="aw-page-stack aw-settings-shell">
    <div class="aw-page-bar">
      <div class="aw-page-bar__copy">
        <h2>工作台设置</h2>
        <p class="aw-copy">计价、分配与人事配置集中在这里，日常作业请返回左侧主菜单。</p>
      </div>
      <div class="aw-page-bar__actions">
        <RouterLink class="aw-secondary-button" to="/">返回工作台</RouterLink>
      </div>
    </div>

    <nav ref="tablistRef" class="aw-hub-tabs aw-settings-tabs" role="tablist" aria-label="工作台设置">
      <template v-for="(tab, index) in otherTabs" :key="tab.to">
        <button
          class="aw-hub-tabs__tab"
          :class="{ 'aw-hub-tabs__tab--active': isActive(tab.to) }"
          type="button"
          role="tab"
          :aria-selected="isActive(tab.to)"
          :tabindex="isActive(tab.to) ? 0 : -1"
          @click="goTab(tab.to)"
          @keydown="onTabKeydown($event, index, otherTabs)"
        >
          <span>{{ tab.label }}</span>
          <small>{{ tab.subtitle }}</small>
        </button>
      </template>
      <div v-if="peopleTabs.length" class="aw-settings-tabs__group" role="presentation">
        <span class="aw-settings-tabs__group-label">人事</span>
        <button
          v-for="(tab, index) in peopleTabs"
          :key="tab.to"
          class="aw-hub-tabs__tab"
          :class="{ 'aw-hub-tabs__tab--active': isActive(tab.to) }"
          type="button"
          role="tab"
          :aria-selected="isActive(tab.to)"
          :tabindex="isActive(tab.to) ? 0 : -1"
          @click="goTab(tab.to)"
          @keydown="onTabKeydown($event, index, peopleTabs)"
        >
          <span>{{ tab.label }}</span>
          <small>{{ tab.subtitle }}</small>
        </button>
      </div>
    </nav>

    <RouterView />
  </section>
</template>
