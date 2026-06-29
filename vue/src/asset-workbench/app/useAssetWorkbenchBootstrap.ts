import { storeToRefs } from 'pinia'

import { useAssetWorkbenchSessionStore } from './session.store'

export function useAssetWorkbenchBootstrap() {
  const session = useAssetWorkbenchSessionStore()
  const { bootstrap, entry, loading, entryLoading, error, entryError } = storeToRefs(session)
  const { refresh, loadEntry, ensureReady, setBootstrap, setEntry, reset, hasAnyCapability } = session

  return {
    bootstrap,
    entry,
    loading,
    entryLoading,
    error,
    entryError,
    refresh,
    loadEntry,
    ensureReady,
    setBootstrap,
    setEntry,
    reset,
    hasAnyCapability,
  }
}
