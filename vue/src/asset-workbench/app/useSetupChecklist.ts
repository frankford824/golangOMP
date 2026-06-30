import { computed, ref } from 'vue'

import { canAccessPath } from '@aw/app/access'
import { assetWorkbenchApi } from '@aw/shared/api/assetWorkbenchApi'
import type { AssetWorkbenchBootstrap } from '@aw/shared/api/assetWorkbenchApi'

export interface SetupStep {
  id: string
  label: string
  hint: string
  to: string
  done: boolean
  visible: boolean
}

const SETUP_COLLAPSED_KEY = 'aw-setup-collapsed'

function businessMonthNow() {
  return new Intl.DateTimeFormat('en-CA', {
    timeZone: 'Asia/Shanghai',
    year: 'numeric',
    month: '2-digit',
  }).format(new Date())
}

export function useSetupChecklist(bootstrap: () => AssetWorkbenchBootstrap | null) {
  const loading = ref(false)
  const collapsed = ref(typeof window !== 'undefined' && window.localStorage.getItem(SETUP_COLLAPSED_KEY) === '1')
  const pricingReady = ref(false)
  const membersReady = ref(false)
  const assignmentsReady = ref(false)
  const acceptanceReady = ref(false)
  const settlementReady = ref(false)

  const steps = computed<SetupStep[]>(() => {
    const current = bootstrap()
    return [
      {
        id: 'pricing',
        label: '计价设置',
        hint: '配置单价、扣款与补贴规则',
        to: '/settings/pricing',
        done: pricingReady.value,
        visible: canAccessPath(current, '/settings/pricing'),
      },
      {
        id: 'members',
        label: '账号权限',
        hint: '开通成员登录与角色',
        to: '/settings/members',
        done: membersReady.value,
        visible: canAccessPath(current, '/settings/members'),
      },
      {
        id: 'dispatch',
        label: '分配任务',
        hint: '指定谁能交哪类稿',
        to: '/settings/dispatch',
        done: assignmentsReady.value,
        visible: canAccessPath(current, '/settings/dispatch'),
      },
      {
        id: 'acceptance',
        label: '查改作品',
        hint: '完成质检并处理待修正批次',
        to: '/submissions',
        done: acceptanceReady.value,
        visible: canAccessPath(current, '/submissions'),
      },
      {
        id: 'settlement',
        label: '本月结算',
        hint: '生成并确认当月工资批次',
        to: '/settlement',
        done: settlementReady.value,
        visible: canAccessPath(current, '/settlement'),
      },
    ].filter((step) => step.visible)
  })

  const completedCount = computed(() => steps.value.filter((step) => step.done).length)
  const allDone = computed(() => steps.value.length > 0 && completedCount.value === steps.value.length)

  async function refresh() {
    loading.value = true
    try {
      const month = businessMonthNow()
      const tasks: Promise<void>[] = []

      if (canAccessPath(bootstrap(), '/settings/pricing')) {
        tasks.push(
          assetWorkbenchApi.listPriceMatrix({ page: 1, page_size: 1 }).then((result) => {
            pricingReady.value = result.total > 0
          }),
        )
      } else {
        pricingReady.value = false
      }

      if (canAccessPath(bootstrap(), '/settings/members')) {
        tasks.push(
          assetWorkbenchApi.listMembers({ page: 1, page_size: 1 }).then((result) => {
            membersReady.value = result.total > 0
          }),
        )
      } else {
        membersReady.value = false
      }

      if (canAccessPath(bootstrap(), '/settings/dispatch')) {
        tasks.push(
          assetWorkbenchApi.listTemplateAssignments({ page: 1, page_size: 1 }).then((result) => {
            assignmentsReady.value = result.total > 0
          }),
        )
      } else {
        assignmentsReady.value = false
      }

      if (canAccessPath(bootstrap(), '/submissions')) {
        tasks.push(
          Promise.all([
            assetWorkbenchApi.listSubmissions({ page: 1, page_size: 1 }),
            assetWorkbenchApi.listSubmissions({ page: 1, page_size: 1, status: 'submitted' }),
            assetWorkbenchApi.listSubmissions({ page: 1, page_size: 1, status: 'needs_fix' }),
          ]).then(([all, submitted, needsFix]) => {
            acceptanceReady.value = all.total > 0 && submitted.total + needsFix.total === 0
          }),
        )
      } else {
        acceptanceReady.value = false
      }

      if (canAccessPath(bootstrap(), '/settlement')) {
        tasks.push(
          assetWorkbenchApi.listSettlementBatches({ page: 1, page_size: 20, business_month: month }).then((result) => {
            settlementReady.value = result.items.some((batch) => batch.status === 'confirmed')
          }),
        )
      } else {
        settlementReady.value = false
      }

      await Promise.allSettled(tasks)
      if (steps.value.every((step) => step.done)) {
        collapsed.value = true
        if (typeof window !== 'undefined') {
          window.localStorage.setItem(SETUP_COLLAPSED_KEY, '1')
        }
      }
    } finally {
      loading.value = false
    }
  }

  function toggleCollapsed() {
    collapsed.value = !collapsed.value
    if (typeof window !== 'undefined') {
      window.localStorage.setItem(SETUP_COLLAPSED_KEY, collapsed.value ? '1' : '0')
    }
  }

  return {
    loading,
    collapsed,
    steps,
    completedCount,
    allDone,
    refresh,
    toggleCollapsed,
  }
}
