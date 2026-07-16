import type { ActiveTaskStatus } from '@/domain/types/task'

/** v8 status filters are sent verbatim; the client no longer expands retired aliases. */
export function expandTaskListStatusFilter(statuses: ActiveTaskStatus[]): ActiveTaskStatus[] {
  return Array.from(new Set(statuses))
}
