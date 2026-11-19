<script setup lang="ts">
import { RouterLink, useRouter, type RouteLocationRaw } from 'vue-router'
import { goBackTo } from '../returnNavigation'

const props = defineProps<{ to: RouteLocationRaw; source?: string }>()
const router = useRouter()
function follow(event: MouseEvent) {
  const anchor = event.currentTarget as HTMLAnchorElement
  if (event.defaultPrevented || event.button !== 0 || event.ctrlKey || event.metaKey ||
      event.altKey || event.shiftKey || anchor.target === '_blank') return
  event.preventDefault()
  void goBackTo(router, props.to, props.source)
}
</script>

<template>
  <RouterLink v-slot="{ href }" :to="to" custom><a :href="href" @click="follow"><slot /></a></RouterLink>
</template>
