import { defineStore } from 'pinia'
import { ref } from 'vue'

export type SyncStatus = 'idle' | 'syncing'

export const useSyncStore = defineStore('sync', () => {
  const sequenceGap = ref(false)
  const versionConflict = ref(false)
  const syncStatus = ref<SyncStatus>('idle')
  const clientVersion = ref<{ hash?: string; updatedAt?: string } | null>(null)
  const serverVersion = ref<{ hash?: string; updatedAt?: string } | null>(null)

  function setSequenceGap(value: boolean) {
    sequenceGap.value = value
    if (value) syncStatus.value = 'syncing'
  }

  function setVersionConflict(value: boolean, client?: { hash?: string; updatedAt?: string }, server?: { hash?: string; updatedAt?: string }) {
    versionConflict.value = value
    clientVersion.value = client ?? null
    serverVersion.value = server ?? null
  }

  function clearConflict() {
    versionConflict.value = false
    clientVersion.value = null
    serverVersion.value = null
  }

  function clearSequenceGap() {
    sequenceGap.value = false
    syncStatus.value = 'idle'
  }

  return {
    sequenceGap,
    versionConflict,
    syncStatus,
    clientVersion,
    serverVersion,
    setSequenceGap,
    setVersionConflict,
    clearConflict,
    clearSequenceGap,
  }
})
