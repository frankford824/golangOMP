export interface MockAsset {
  id: string
  task_id: string
  file_name: string
  file_role: 'source' | 'delivery' | 'reference'
  created_at: string
}

export const mockAssets: MockAsset[] = []
