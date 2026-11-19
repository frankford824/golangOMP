<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useOAAccess } from '../access'
import { directoryQuery, peopleReturnSource, shopBindingReturnSource } from '../peopleNavigation'

const route = useRoute()
const router = useRouter()
const { can, currentProfile } = useOAAccess()
const rosterSource = computed(() => {
  const source = shopBindingReturnSource(route.query.peopleFrom)
  return source && router.resolve(source).path === '/users/roster' ? source : undefined
})
const directorySource = computed(() => {
  if (route.path === '/users') return route.fullPath
  return peopleReturnSource(route.query.peopleFrom)
    ?? (rosterSource.value && peopleReturnSource(router.resolve(rosterSource.value).query.peopleFrom))
})
const directory = computed(() => {
  const source = router.resolve(directorySource.value || '/users')
  return { path: '/users', query: directoryQuery(source.query) }
})
const roster = computed(() => route.path === '/users/roster' ? route.fullPath : rosterSource.value || {
  path: '/users/roster',
  query: { peopleFrom: directorySource.value },
})
const bindings = computed(() => route.path === '/users/shop-bindings' ? route.fullPath : {
  path: '/users/shop-bindings',
  query: { peopleFrom: shopBindingReturnSource(route.fullPath) },
})
</script>

<template>
  <nav class="people-nav" aria-label="人员管理">
    <RouterLink :to="directory" active-class="selected" exact-active-class="">成员与组织</RouterLink>
    <RouterLink v-if="can('people.manage')" :to="roster" active-class="selected" exact-active-class="">人员花名册</RouterLink>
    <RouterLink v-if="can('people.manage') && currentProfile?.super_admin" :to="bindings" active-class="selected" exact-active-class="">店铺人员绑定</RouterLink>
  </nav>
</template>

<style scoped>
.people-nav { display:flex; align-items:stretch; gap:24px; min-width:0; overflow-x:auto; scrollbar-width:thin; }
.people-nav > a { flex:none; display:inline-flex; align-items:center; min-height:40px; padding-inline:4px; border-bottom:2px solid transparent; color:var(--oa-secondary); font:400 14px/1.5 var(--oa-font); text-decoration:none; white-space:nowrap; }
.people-nav > a:hover, .people-nav > a.selected { color:var(--oa-accent); }
.people-nav > a.selected { border-bottom-color:currentColor; font-weight:600; }
.people-nav > a:focus-visible { outline:2px solid var(--oa-accent); outline-offset:-2px; border-radius:4px; }
@media (max-width:850px) { .people-nav { gap:12px; } }
</style>
