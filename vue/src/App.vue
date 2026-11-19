<template>
  <NConfigProvider :theme-overrides="themeOverrides">
    <NMessageProvider>
      <main v-if="oaCookieSessionEnabled && oaSessionRecovery" class="oa-session-recovery" aria-labelledby="oa-session-recovery-title">
        <section>
          <h1 id="oa-session-recovery-title">{{ oaSessionRecovery.error.blocked ? '当前账号无法访问 OA' : '暂时无法确认登录状态' }}</h1>
          <p role="status">
            {{ oaSessionRecovery.error.blocked ? '账号可能已停用或缺少访问权限。请切换账号，或联系管理员核对。' : '请重试以继续访问当前页面。' }}
          </p>
          <p v-if="oaSessionRecovery.error.traceId">追踪号 {{ oaSessionRecovery.error.traceId }}</p>
          <button v-if="oaSessionRecovery.error.blocked" type="button" :disabled="retryingSession" @click="switchAccount">
            {{ retryingSession ? '正在退出当前账号' : '切换账号' }}
          </button>
          <button v-else type="button" :disabled="retryingSession" @click="retrySession">
            {{ retryingSession ? '正在确认登录状态' : '重试' }}
          </button>
        </section>
      </main>
      <router-view v-else />
      <RequestErrorNotice v-if="RequestErrorNotice" />
    </NMessageProvider>
  </NConfigProvider>
</template>

<script setup lang="ts">
import { defineAsyncComponent, ref } from 'vue'
import { isNavigationFailure, NavigationFailureType, useRouter } from 'vue-router'
import { usePermissionsStore } from '@/stores/permissions'
import { resolvePostLoginLandingRoute } from '@/router/home-fallback'
import { oaCookieSessionEnabled } from '@/services/oaSession'
import { OASessionRecoveryError, oaSessionRecovery, showOASessionRecovery } from '@/oa/sessionRecovery'
import { NConfigProvider, NMessageProvider, type GlobalThemeOverrides } from 'naive-ui'
import { resolveCssRgbToken } from '@/utils/color-tokens'

const router = useRouter()
const retryingSession = ref(false)

async function retrySession(): Promise<void> {
  const recovery = oaSessionRecovery.value
  if (!recovery || retryingSession.value) return
  retryingSession.value = true
  try {
    const failure = await router.replace(recovery.target)
    if (!isNavigationFailure(failure, NavigationFailureType.duplicated)) return

    // A failed check immediately after signing in leaves /login mounted in the router.
    // Retry the same check before deciding whether the sign-in form is needed.
    const permissions = usePermissionsStore()
    try {
      await permissions.restoreSession()
    } catch (error) {
      if (error instanceof OASessionRecoveryError) {
        showOASessionRecovery(recovery.target, error)
        return
      }
      oaSessionRecovery.value = undefined
      return
    }
    const route = router.currentRoute.value
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : undefined
    await router.replace(resolvePostLoginLandingRoute(router, permissions, redirect))
  } finally {
    retryingSession.value = false
  }
}

async function switchAccount(): Promise<void> {
  const recovery = oaSessionRecovery.value
  if (!recovery || retryingSession.value) return
  retryingSession.value = true
  try {
    await usePermissionsStore().logout()
    oaSessionRecovery.value = undefined
    const target = router.resolve(recovery.target)
    const redirect = target.name === 'Login' ? target.query.redirect : recovery.target
    await router.replace({ name: 'Login', query: typeof redirect === 'string' ? { redirect } : {} })
  } catch {
    // OA's request error notice presents any failed logout. Keep the switch action available.
  } finally {
    retryingSession.value = false
  }
}

const RequestErrorNotice = import.meta.env.VITE_OA_FEATURES === 'true'
  ? defineAsyncComponent(() => import('@/oa/components/RequestErrorNotice.vue'))
  : undefined

// Lock naive-ui's accent to the design-system brand blue so any naive surface
// (messages, future data tables) matches the rest of the app instead of
// naive's default green. Components that set their own theme-overrides
// (e.g. WorkflowProgress NSteps) keep their explicit colors.
const themeOverrides: GlobalThemeOverrides = {
  common: {
    primaryColor: resolveCssRgbToken('--yb-brand', '37 99 235'),
    primaryColorHover: resolveCssRgbToken('--yb-brand-strong', '29 78 216'),
    primaryColorPressed: resolveCssRgbToken('--yb-brand-deep', '30 64 175'),
    primaryColorSuppl: resolveCssRgbToken('--yb-brand', '37 99 235'),
  },
}
</script>

<style scoped>
.oa-session-recovery {
  min-height: 100dvh;
  display: grid;
  place-items: center;
  padding: 24px;
  color: rgb(var(--yb-text));
  background: rgb(var(--yb-surface-soft));
}

.oa-session-recovery section {
  width: min(100%, 440px);
  padding: 28px;
  border: 1px solid rgb(var(--yb-border));
  border-radius: 12px;
  background: rgb(var(--yb-surface));
}

.oa-session-recovery h1 {
  font-size: 1.25rem;
  font-weight: 650;
}

.oa-session-recovery p {
  margin: 12px 0;
}

.oa-session-recovery button {
  background: rgb(var(--yb-brand));
  color: white;
  padding: 8px 18px;
  border-radius: 7px;
}
</style>
