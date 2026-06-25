import { useMessage, type MessageOptions, type MessageReactive } from 'naive-ui'

type ToastContent = string

interface AppToast {
  success: (content: ToastContent, options?: MessageOptions) => MessageReactive
  error: (content: ToastContent, options?: MessageOptions) => MessageReactive
  warning: (content: ToastContent, options?: MessageOptions) => MessageReactive
  info: (content: ToastContent, options?: MessageOptions) => MessageReactive
  loading: (content: ToastContent, options?: MessageOptions) => MessageReactive
}

/**
 * Thin, app-wide wrapper over naive-ui's message API. Use ONLY for transient,
 * non-blocking feedback: operation/copy/save success and short, ignorable
 * failures. Form validation, page-load errors, and batch results that the user
 * must review or correct should stay inline (AsyncStateWrapper / field errors),
 * not be funneled into ephemeral toasts.
 *
 * Must be called from component setup() under the App-level NMessageProvider.
 */
export function useAppToast(): AppToast {
  const message = useMessage()
  return {
    success: (content, options) => message.success(content, options),
    error: (content, options) => message.error(content, options),
    warning: (content, options) => message.warning(content, options),
    info: (content, options) => message.info(content, options),
    loading: (content, options) => message.loading(content, options),
  }
}
