import type {
  ResourceFile,
  ResourceGroup,
  ResourceGroupDownloadItem,
  ResourceGroupListResult,
  ResourceRevision,
  ResourceRevisionItem,
} from '@/services/api/resourceGroupsApi'

// The workbench intentionally aliases the main project's canonical resource
// contracts instead of maintaining a second, partial copy.
export type WorkbenchResourceFile = ResourceFile
export type WorkbenchResourceRevisionItem = ResourceRevisionItem
export type WorkbenchResourceRevision = ResourceRevision
export type WorkbenchResourceGroup = ResourceGroup
export type WorkbenchResourceGroupList = ResourceGroupListResult
export interface WorkbenchResourceDownloadManifest {
  items: ResourceGroupDownloadItem[]
}
