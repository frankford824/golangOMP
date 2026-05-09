import type { InjectionKey, ComputedRef } from 'vue'
import type { Task } from '@/domain/types/task'

/**
 * provide/inject key for TaskDetailView → 子区块的 task 注入。
 * 独立文件避免从 .vue 文件导出非组件内容（<script setup> 不允许 export）。
 */
export const TASK_DETAIL_KEY: InjectionKey<ComputedRef<Task | null>> = Symbol('task-detail')
