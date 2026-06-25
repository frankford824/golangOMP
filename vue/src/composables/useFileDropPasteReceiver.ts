import { onBeforeUnmount, onMounted, unref } from 'vue'
import type { Ref } from 'vue'

type MaybeRefOrGetter<T> = T | Ref<T> | (() => T)
type FileReceiverSource = 'drop' | 'paste'

interface FileReceiver {
  id: symbol
  enabled: MaybeRefOrGetter<boolean>
  acceptDrop: MaybeRefOrGetter<boolean>
  acceptPaste: MaybeRefOrGetter<boolean>
  onFiles: (files: File[], source: FileReceiverSource) => void | Promise<void>
}

export interface FileDropPasteReceiverOptions {
  enabled?: MaybeRefOrGetter<boolean>
  acceptDrop?: MaybeRefOrGetter<boolean>
  acceptPaste?: MaybeRefOrGetter<boolean>
  onFiles: (files: File[], source: FileReceiverSource) => void | Promise<void>
}

const receivers = new Map<symbol, FileReceiver>()
let activeReceiverId: symbol | null = null
let listenersInstalled = false

function resolveMaybeRef<T>(value: MaybeRefOrGetter<T> | undefined, fallback: T): T {
  if (typeof value === 'function') return (value as () => T)()
  if (value === undefined) return fallback
  return unref(value)
}

export function getFilesFromDataTransfer(dataTransfer: DataTransfer | null | undefined): File[] {
  if (!dataTransfer) return []
  const itemFiles = Array.from(dataTransfer.items ?? [])
    .filter((item) => item.kind === 'file')
    .map((item) => item.getAsFile())
    .filter((file): file is File => file instanceof File)
  if (itemFiles.length > 0) return itemFiles
  return Array.from(dataTransfer.files ?? [])
}

export function getFilesFromClipboardEvent(event: ClipboardEvent): File[] {
  return getFilesFromDataTransfer(event.clipboardData)
}

export function hasFileDataTransfer(dataTransfer: DataTransfer | null | undefined): boolean {
  if (!dataTransfer) return false
  if (Array.from(dataTransfer.items ?? []).some((item) => item.kind === 'file')) return true
  return Array.from(dataTransfer.types ?? []).includes('Files') || (dataTransfer.files?.length ?? 0) > 0
}

function receiverEnabled(receiver: FileReceiver, source: FileReceiverSource): boolean {
  if (!resolveMaybeRef(receiver.enabled, true)) return false
  return source === 'drop'
    ? resolveMaybeRef(receiver.acceptDrop, true)
    : resolveMaybeRef(receiver.acceptPaste, true)
}

function pickReceiver(source: FileReceiverSource): FileReceiver | null {
  if (activeReceiverId) {
    const active = receivers.get(activeReceiverId)
    if (active && receiverEnabled(active, source)) return active
  }
  const enabled = Array.from(receivers.values()).filter((receiver) => receiverEnabled(receiver, source))
  return enabled.length === 1 ? enabled[0]! : null
}

function dispatchFiles(files: File[], source: FileReceiverSource) {
  if (!files.length) return false
  const receiver = pickReceiver(source)
  if (!receiver) return false
  void receiver.onFiles(files, source)
  return true
}

function onDocumentDragOver(event: DragEvent) {
  if (event.defaultPrevented || !hasFileDataTransfer(event.dataTransfer)) return
  if (!pickReceiver('drop')) return
  event.preventDefault()
  if (event.dataTransfer) event.dataTransfer.dropEffect = 'copy'
}

function onDocumentDrop(event: DragEvent) {
  if (event.defaultPrevented) return
  const files = getFilesFromDataTransfer(event.dataTransfer)
  if (!files.length) return
  if (dispatchFiles(files, 'drop')) event.preventDefault()
}

function onDocumentPaste(event: ClipboardEvent) {
  if (event.defaultPrevented) return
  const files = getFilesFromClipboardEvent(event)
  if (!files.length) return
  if (dispatchFiles(files, 'paste')) event.preventDefault()
}

function ensureDocumentListeners() {
  if (listenersInstalled || typeof document === 'undefined') return
  document.addEventListener('dragover', onDocumentDragOver)
  document.addEventListener('drop', onDocumentDrop)
  document.addEventListener('paste', onDocumentPaste)
  listenersInstalled = true
}

export function useFileDropPasteReceiver(options: FileDropPasteReceiverOptions) {
  const id = Symbol('file-drop-paste-receiver')
  const receiver: FileReceiver = {
    id,
    enabled: options.enabled ?? true,
    acceptDrop: options.acceptDrop ?? true,
    acceptPaste: options.acceptPaste ?? true,
    onFiles: options.onFiles,
  }

  function activateFileReceiver() {
    if (!resolveMaybeRef(receiver.enabled, true)) return
    activeReceiverId = id
  }

  function deactivateFileReceiver() {
    if (activeReceiverId === id) activeReceiverId = null
  }

  onMounted(() => {
    receivers.set(id, receiver)
    ensureDocumentListeners()
  })

  onBeforeUnmount(() => {
    receivers.delete(id)
    deactivateFileReceiver()
  })

  return {
    activateFileReceiver,
    deactivateFileReceiver,
  }
}
