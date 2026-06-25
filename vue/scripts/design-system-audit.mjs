import { readdir, readFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const rootDir = path.resolve(__dirname, '..')
const srcDir = path.join(rootDir, 'src')
const mainCssPath = path.join(srcDir, 'assets', 'main.css')
const taskListPath = path.join(srcDir, 'views', 'TaskListView.vue')
const taskDetailPath = path.join(srcDir, 'views', 'TaskDetailView.vue')
const taskAssetsPath = path.join(srcDir, 'views', 'TaskAssetsView.vue')
const assetsIndexPath = path.join(srcDir, 'views', 'AssetsIndexView.vue')
const assetDetailPath = path.join(srcDir, 'views', 'AssetDetailView.vue')
const dashboardPath = path.join(srcDir, 'views', 'DashboardView.vue')
const appShellPath = path.join(srcDir, 'layouts', 'AppShell.vue')
const avatarDropdownPath = path.join(srcDir, 'components', 'layout', 'AvatarDropdown.vue')
const baseModalPath = path.join(srcDir, 'components', 'base', 'BaseModal.vue')
const baseSelectPath = path.join(srcDir, 'components', 'base', 'BaseSelect.vue')
const globalSearchOverlayPath = path.join(srcDir, 'components', 'global-search', 'GlobalSearchOverlay.vue')
const userManagementPath = path.join(srcDir, 'views', 'org-permission', 'UserManagementView.vue')
const closeDraftConfirmModalPath = path.join(srcDir, 'components', 'task-create', 'CloseDraftConfirmModal.vue')
const reassignDesignerDialogPath = path.join(srcDir, 'components', 'task', 'ReassignDesignerDialog.vue')
const designAssetBlockPath = path.join(srcDir, 'components', 'task-detail', 'DesignAssetBlock.vue')
const outsourceOrderTablePath = path.join(srcDir, 'components', 'outsource', 'OutsourceOrderTable.vue')
const workflowProgressPath = path.join(srcDir, 'components', 'task', 'WorkflowProgress.vue')

const importantBudget = Number(process.env.DESIGN_IMPORTANT_BUDGET ?? 0)
const mainCssImportantBudget = Number(process.env.DESIGN_MAIN_CSS_IMPORTANT_BUDGET ?? 0)
const taskListImportantBudget = Number(process.env.DESIGN_TASK_LIST_IMPORTANT_BUDGET ?? 0)
const taskDetailImportantBudget = Number(process.env.DESIGN_TASK_DETAIL_IMPORTANT_BUDGET ?? 0)
const taskAssetsImportantBudget = Number(process.env.DESIGN_TASK_ASSETS_IMPORTANT_BUDGET ?? 0)
const assetsIndexImportantBudget = Number(process.env.DESIGN_ASSETS_INDEX_IMPORTANT_BUDGET ?? 0)
const assetDetailImportantBudget = Number(process.env.DESIGN_ASSET_DETAIL_IMPORTANT_BUDGET ?? 0)
const dashboardImportantBudget = Number(process.env.DESIGN_DASHBOARD_IMPORTANT_BUDGET ?? 0)
const appShellImportantBudget = Number(process.env.DESIGN_APP_SHELL_IMPORTANT_BUDGET ?? 0)
const avatarDropdownImportantBudget = Number(process.env.DESIGN_AVATAR_DROPDOWN_IMPORTANT_BUDGET ?? 0)
const baseModalImportantBudget = Number(process.env.DESIGN_BASE_MODAL_IMPORTANT_BUDGET ?? 0)
const baseSelectImportantBudget = Number(process.env.DESIGN_BASE_SELECT_IMPORTANT_BUDGET ?? 0)
const globalSearchOverlayImportantBudget = Number(process.env.DESIGN_GLOBAL_SEARCH_OVERLAY_IMPORTANT_BUDGET ?? 0)
const taskCreateModalImportantBudget = Number(process.env.DESIGN_TASK_CREATE_MODAL_IMPORTANT_BUDGET ?? 0)
const userManagementImportantBudget = Number(process.env.DESIGN_USER_MANAGEMENT_IMPORTANT_BUDGET ?? 0)
const taskInfoEditModalImportantBudget = Number(process.env.DESIGN_TASK_INFO_EDIT_MODAL_IMPORTANT_BUDGET ?? 0)
const closeDraftConfirmModalImportantBudget = Number(process.env.DESIGN_CLOSE_DRAFT_CONFIRM_MODAL_IMPORTANT_BUDGET ?? 0)
const reassignDesignerDialogImportantBudget = Number(process.env.DESIGN_REASSIGN_DESIGNER_DIALOG_IMPORTANT_BUDGET ?? 0)
const designAssetBlockImportantBudget = Number(process.env.DESIGN_DESIGN_ASSET_BLOCK_IMPORTANT_BUDGET ?? 0)
const outsourceOrderTableImportantBudget = Number(process.env.DESIGN_OUTSOURCE_ORDER_TABLE_IMPORTANT_BUDGET ?? 0)
const workflowProgressImportantBudget = Number(process.env.DESIGN_WORKFLOW_PROGRESS_IMPORTANT_BUDGET ?? 0)
const hardcodedColorBudget = Number(process.env.DESIGN_HARDCODED_COLOR_BUDGET ?? 0)
const mainCssHardcodedColorBudget = Number(process.env.DESIGN_MAIN_CSS_COLOR_BUDGET ?? 0)
const taskListHardcodedColorBudget = Number(process.env.DESIGN_TASK_LIST_COLOR_BUDGET ?? 0)
const taskDetailHardcodedColorBudget = Number(process.env.DESIGN_TASK_DETAIL_COLOR_BUDGET ?? 0)
const assetsIndexHardcodedColorBudget = Number(process.env.DESIGN_ASSETS_INDEX_COLOR_BUDGET ?? 0)
const dashboardHardcodedColorBudget = Number(process.env.DESIGN_DASHBOARD_COLOR_BUDGET ?? 0)
const appShellHardcodedColorBudget = Number(process.env.DESIGN_APP_SHELL_COLOR_BUDGET ?? 0)
const avatarDropdownHardcodedColorBudget = Number(process.env.DESIGN_AVATAR_DROPDOWN_COLOR_BUDGET ?? 0)
const baseModalHardcodedColorBudget = Number(process.env.DESIGN_BASE_MODAL_COLOR_BUDGET ?? 0)
const baseSelectHardcodedColorBudget = Number(process.env.DESIGN_BASE_SELECT_COLOR_BUDGET ?? 0)
const globalSearchOverlayHardcodedColorBudget = Number(process.env.DESIGN_GLOBAL_SEARCH_OVERLAY_COLOR_BUDGET ?? 0)
const closeDraftConfirmModalHardcodedColorBudget = Number(process.env.DESIGN_CLOSE_DRAFT_CONFIRM_MODAL_COLOR_BUDGET ?? 0)
const reassignDesignerDialogHardcodedColorBudget = Number(process.env.DESIGN_REASSIGN_DESIGNER_DIALOG_COLOR_BUDGET ?? 0)
const designAssetBlockHardcodedColorBudget = Number(process.env.DESIGN_DESIGN_ASSET_BLOCK_COLOR_BUDGET ?? 0)
const outsourceOrderTableHardcodedColorBudget = Number(process.env.DESIGN_OUTSOURCE_ORDER_TABLE_COLOR_BUDGET ?? 0)
const workflowProgressHardcodedColorBudget = Number(process.env.DESIGN_WORKFLOW_PROGRESS_COLOR_BUDGET ?? 0)

const sourceExtensions = new Set(['.css', '.vue'])
const failures = []
const warnings = []

const files = await collectFiles(srcDir)
const mainCss = await readFile(mainCssPath, 'utf8')
const taskListSource = await readFile(taskListPath, 'utf8')
const taskDetailSource = await readFile(taskDetailPath, 'utf8')
await readFile(taskAssetsPath, 'utf8')
const assetsIndexSource = await readFile(assetsIndexPath, 'utf8')
await readFile(assetDetailPath, 'utf8')
const dashboardSource = await readFile(dashboardPath, 'utf8')
const appShellSource = await readFile(appShellPath, 'utf8')
const avatarDropdownSource = await readFile(avatarDropdownPath, 'utf8')
const baseModalSource = await readFile(baseModalPath, 'utf8')
const baseSelectSource = await readFile(baseSelectPath, 'utf8')
const globalSearchOverlaySource = await readFile(globalSearchOverlayPath, 'utf8')
const closeDraftConfirmModalSource = await readFile(closeDraftConfirmModalPath, 'utf8')
const reassignDesignerDialogSource = await readFile(reassignDesignerDialogPath, 'utf8')
const designAssetBlockSource = await readFile(designAssetBlockPath, 'utf8')
const outsourceOrderTableSource = await readFile(outsourceOrderTablePath, 'utf8')
const workflowProgressSource = await readFile(workflowProgressPath, 'utf8')
await readFile(userManagementPath, 'utf8')
const topLevelRootCount = mainCss.split(/\r?\n/).filter((line) => /^:root\s*\{/.test(line)).length
const mainCssWithoutRoot = stripTopLevelRoot(mainCss)
const mainCssHardcodedColors = findHardcodedColors(mainCssWithoutRoot)
const taskListHardcodedColors = findHardcodedColors(taskListSource)
const taskDetailHardcodedColors = findHardcodedColors(taskDetailSource)
const assetsIndexHardcodedColors = findHardcodedColors(assetsIndexSource)
const dashboardHardcodedColors = findHardcodedColors(dashboardSource)
const appShellHardcodedColors = findHardcodedColors(appShellSource)
const avatarDropdownHardcodedColors = findHardcodedColors(avatarDropdownSource)
const baseModalHardcodedColors = findHardcodedColors(baseModalSource)
const baseSelectHardcodedColors = findHardcodedColors(baseSelectSource)
const globalSearchOverlayHardcodedColors = findHardcodedColors(globalSearchOverlaySource)
const closeDraftConfirmModalHardcodedColors = findHardcodedColors(closeDraftConfirmModalSource)
const reassignDesignerDialogHardcodedColors = findHardcodedColors(reassignDesignerDialogSource)
const designAssetBlockHardcodedColors = findHardcodedColors(designAssetBlockSource)
const outsourceOrderTableHardcodedColors = findHardcodedColors(outsourceOrderTableSource)
const workflowProgressHardcodedColors = findHardcodedColors(workflowProgressSource)
const ybFontSansDefined = mainCss.includes('--yb-font-sans:')
const importantByFile = new Map()
const hardcodedColorsByFile = new Map()
const malformedTokenColorsByFile = new Map()
const ybFontSansRefs = []

for (const file of files) {
  const source = await readFile(file, 'utf8')
  const relative = path.relative(rootDir, file).replaceAll(path.sep, '/')
  const importantCount = countMatches(source, /!important/g)
  if (importantCount > 0) {
    importantByFile.set(relative, importantCount)
  }
  const colorSource = file === mainCssPath ? stripTopLevelRoot(source) : source
  const hardcodedColors = findHardcodedColors(colorSource)
  if (hardcodedColors.length > 0) {
    hardcodedColorsByFile.set(relative, hardcodedColors)
  }
  const malformedTokenColors = findMalformedTokenColors(source)
  if (malformedTokenColors.length > 0) {
    malformedTokenColorsByFile.set(relative, malformedTokenColors)
  }
  if (source.includes('var(--yb-font-sans)')) {
    ybFontSansRefs.push(relative)
  }
}

const importantTotal = [...importantByFile.values()].reduce((sum, value) => sum + value, 0)
const hardcodedColorTotal = [...hardcodedColorsByFile.values()].reduce((sum, value) => sum + value.length, 0)
const malformedTokenColorTotal = [...malformedTokenColorsByFile.values()].reduce(
  (sum, value) => sum + value.length,
  0,
)
const mainCssImportantCount = importantByFile.get('src/assets/main.css') ?? 0
const taskListImportantCount = importantByFile.get('src/views/TaskListView.vue') ?? 0
const taskDetailImportantCount = importantByFile.get('src/views/TaskDetailView.vue') ?? 0
const taskAssetsImportantCount = importantByFile.get('src/views/TaskAssetsView.vue') ?? 0
const assetsIndexImportantCount = importantByFile.get('src/views/AssetsIndexView.vue') ?? 0
const assetDetailImportantCount = importantByFile.get('src/views/AssetDetailView.vue') ?? 0
const dashboardImportantCount = importantByFile.get('src/views/DashboardView.vue') ?? 0
const appShellImportantCount = importantByFile.get('src/layouts/AppShell.vue') ?? 0
const avatarDropdownImportantCount = importantByFile.get('src/components/layout/AvatarDropdown.vue') ?? 0
const baseModalImportantCount = importantByFile.get('src/components/base/BaseModal.vue') ?? 0
const baseSelectImportantCount = importantByFile.get('src/components/base/BaseSelect.vue') ?? 0
const globalSearchOverlayImportantCount =
  importantByFile.get('src/components/global-search/GlobalSearchOverlay.vue') ?? 0
const taskCreateModalImportantCount = importantByFile.get('src/components/task/TaskCreateModal.vue') ?? 0
const userManagementImportantCount = importantByFile.get('src/views/org-permission/UserManagementView.vue') ?? 0
const taskInfoEditModalImportantCount = importantByFile.get('src/components/task-detail/TaskInfoEditModal.vue') ?? 0
const closeDraftConfirmModalImportantCount =
  importantByFile.get('src/components/task-create/CloseDraftConfirmModal.vue') ?? 0
const reassignDesignerDialogImportantCount =
  importantByFile.get('src/components/task/ReassignDesignerDialog.vue') ?? 0
const designAssetBlockImportantCount =
  importantByFile.get('src/components/task-detail/DesignAssetBlock.vue') ?? 0
const outsourceOrderTableImportantCount =
  importantByFile.get('src/components/outsource/OutsourceOrderTable.vue') ?? 0
const workflowProgressImportantCount = importantByFile.get('src/components/task/WorkflowProgress.vue') ?? 0

if (topLevelRootCount !== 1) {
  failures.push(`expected exactly one top-level :root in src/assets/main.css, found ${topLevelRootCount}`)
}

if (importantTotal > importantBudget) {
  failures.push(`src !important count ${importantTotal} exceeds budget ${importantBudget}`)
}

if (mainCssImportantCount > mainCssImportantBudget) {
  failures.push(`main.css !important count ${mainCssImportantCount} exceeds budget ${mainCssImportantBudget}`)
}

if (hardcodedColorTotal > hardcodedColorBudget) {
  failures.push(`src .vue/.css hardcoded colors ${hardcodedColorTotal} exceeds budget ${hardcodedColorBudget}`)
}

if (malformedTokenColorTotal > 0) {
  failures.push(`malformed token colors ${malformedTokenColorTotal} found`)
}

if (taskListImportantCount > taskListImportantBudget) {
  failures.push(`TaskList !important count ${taskListImportantCount} exceeds budget ${taskListImportantBudget}`)
}

if (taskDetailImportantCount > taskDetailImportantBudget) {
  failures.push(`TaskDetail !important count ${taskDetailImportantCount} exceeds budget ${taskDetailImportantBudget}`)
}

if (taskAssetsImportantCount > taskAssetsImportantBudget) {
  failures.push(`TaskAssets !important count ${taskAssetsImportantCount} exceeds budget ${taskAssetsImportantBudget}`)
}

if (assetsIndexImportantCount > assetsIndexImportantBudget) {
  failures.push(`AssetsIndex !important count ${assetsIndexImportantCount} exceeds budget ${assetsIndexImportantBudget}`)
}

if (assetDetailImportantCount > assetDetailImportantBudget) {
  failures.push(`AssetDetail !important count ${assetDetailImportantCount} exceeds budget ${assetDetailImportantBudget}`)
}

if (dashboardImportantCount > dashboardImportantBudget) {
  failures.push(`Dashboard !important count ${dashboardImportantCount} exceeds budget ${dashboardImportantBudget}`)
}

if (appShellImportantCount > appShellImportantBudget) {
  failures.push(`AppShell !important count ${appShellImportantCount} exceeds budget ${appShellImportantBudget}`)
}

if (avatarDropdownImportantCount > avatarDropdownImportantBudget) {
  failures.push(
    `AvatarDropdown !important count ${avatarDropdownImportantCount} exceeds budget ${avatarDropdownImportantBudget}`,
  )
}

if (baseModalImportantCount > baseModalImportantBudget) {
  failures.push(`BaseModal !important count ${baseModalImportantCount} exceeds budget ${baseModalImportantBudget}`)
}

if (baseSelectImportantCount > baseSelectImportantBudget) {
  failures.push(`BaseSelect !important count ${baseSelectImportantCount} exceeds budget ${baseSelectImportantBudget}`)
}

if (globalSearchOverlayImportantCount > globalSearchOverlayImportantBudget) {
  failures.push(
    `GlobalSearchOverlay !important count ${globalSearchOverlayImportantCount} exceeds budget ${globalSearchOverlayImportantBudget}`,
  )
}

if (taskCreateModalImportantCount > taskCreateModalImportantBudget) {
  failures.push(
    `TaskCreateModal !important count ${taskCreateModalImportantCount} exceeds budget ${taskCreateModalImportantBudget}`,
  )
}

if (userManagementImportantCount > userManagementImportantBudget) {
  failures.push(
    `UserManagement !important count ${userManagementImportantCount} exceeds budget ${userManagementImportantBudget}`,
  )
}

if (taskInfoEditModalImportantCount > taskInfoEditModalImportantBudget) {
  failures.push(
    `TaskInfoEditModal !important count ${taskInfoEditModalImportantCount} exceeds budget ${taskInfoEditModalImportantBudget}`,
  )
}

if (closeDraftConfirmModalImportantCount > closeDraftConfirmModalImportantBudget) {
  failures.push(
    `CloseDraftConfirmModal !important count ${closeDraftConfirmModalImportantCount} exceeds budget ${closeDraftConfirmModalImportantBudget}`,
  )
}

if (reassignDesignerDialogImportantCount > reassignDesignerDialogImportantBudget) {
  failures.push(
    `ReassignDesignerDialog !important count ${reassignDesignerDialogImportantCount} exceeds budget ${reassignDesignerDialogImportantBudget}`,
  )
}

if (designAssetBlockImportantCount > designAssetBlockImportantBudget) {
  failures.push(
    `DesignAssetBlock !important count ${designAssetBlockImportantCount} exceeds budget ${designAssetBlockImportantBudget}`,
  )
}

if (outsourceOrderTableImportantCount > outsourceOrderTableImportantBudget) {
  failures.push(
    `OutsourceOrderTable !important count ${outsourceOrderTableImportantCount} exceeds budget ${outsourceOrderTableImportantBudget}`,
  )
}

if (workflowProgressImportantCount > workflowProgressImportantBudget) {
  failures.push(
    `WorkflowProgress !important count ${workflowProgressImportantCount} exceeds budget ${workflowProgressImportantBudget}`,
  )
}

if (mainCssHardcodedColors.length > mainCssHardcodedColorBudget) {
  failures.push(
    `main.css hardcoded colors outside token root ${mainCssHardcodedColors.length} exceeds budget ${mainCssHardcodedColorBudget}`,
  )
}

if (taskListHardcodedColors.length > taskListHardcodedColorBudget) {
  failures.push(
    `TaskList hardcoded colors ${taskListHardcodedColors.length} exceeds budget ${taskListHardcodedColorBudget}`,
  )
}

if (taskDetailHardcodedColors.length > taskDetailHardcodedColorBudget) {
  failures.push(
    `TaskDetail hardcoded colors ${taskDetailHardcodedColors.length} exceeds budget ${taskDetailHardcodedColorBudget}`,
  )
}

if (assetsIndexHardcodedColors.length > assetsIndexHardcodedColorBudget) {
  failures.push(
    `AssetsIndex hardcoded colors ${assetsIndexHardcodedColors.length} exceeds budget ${assetsIndexHardcodedColorBudget}`,
  )
}

if (dashboardHardcodedColors.length > dashboardHardcodedColorBudget) {
  failures.push(
    `Dashboard hardcoded colors ${dashboardHardcodedColors.length} exceeds budget ${dashboardHardcodedColorBudget}`,
  )
}

if (appShellHardcodedColors.length > appShellHardcodedColorBudget) {
  failures.push(
    `AppShell hardcoded colors ${appShellHardcodedColors.length} exceeds budget ${appShellHardcodedColorBudget}`,
  )
}

if (avatarDropdownHardcodedColors.length > avatarDropdownHardcodedColorBudget) {
  failures.push(
    `AvatarDropdown hardcoded colors ${avatarDropdownHardcodedColors.length} exceeds budget ${avatarDropdownHardcodedColorBudget}`,
  )
}

if (baseModalHardcodedColors.length > baseModalHardcodedColorBudget) {
  failures.push(
    `BaseModal hardcoded colors ${baseModalHardcodedColors.length} exceeds budget ${baseModalHardcodedColorBudget}`,
  )
}

if (baseSelectHardcodedColors.length > baseSelectHardcodedColorBudget) {
  failures.push(
    `BaseSelect hardcoded colors ${baseSelectHardcodedColors.length} exceeds budget ${baseSelectHardcodedColorBudget}`,
  )
}

if (globalSearchOverlayHardcodedColors.length > globalSearchOverlayHardcodedColorBudget) {
  failures.push(
    `GlobalSearchOverlay hardcoded colors ${globalSearchOverlayHardcodedColors.length} exceeds budget ${globalSearchOverlayHardcodedColorBudget}`,
  )
}

if (closeDraftConfirmModalHardcodedColors.length > closeDraftConfirmModalHardcodedColorBudget) {
  failures.push(
    `CloseDraftConfirmModal hardcoded colors ${closeDraftConfirmModalHardcodedColors.length} exceeds budget ${closeDraftConfirmModalHardcodedColorBudget}`,
  )
}

if (reassignDesignerDialogHardcodedColors.length > reassignDesignerDialogHardcodedColorBudget) {
  failures.push(
    `ReassignDesignerDialog hardcoded colors ${reassignDesignerDialogHardcodedColors.length} exceeds budget ${reassignDesignerDialogHardcodedColorBudget}`,
  )
}

if (designAssetBlockHardcodedColors.length > designAssetBlockHardcodedColorBudget) {
  failures.push(
    `DesignAssetBlock hardcoded colors ${designAssetBlockHardcodedColors.length} exceeds budget ${designAssetBlockHardcodedColorBudget}`,
  )
}

if (outsourceOrderTableHardcodedColors.length > outsourceOrderTableHardcodedColorBudget) {
  failures.push(
    `OutsourceOrderTable hardcoded colors ${outsourceOrderTableHardcodedColors.length} exceeds budget ${outsourceOrderTableHardcodedColorBudget}`,
  )
}

if (workflowProgressHardcodedColors.length > workflowProgressHardcodedColorBudget) {
  failures.push(
    `WorkflowProgress hardcoded colors ${workflowProgressHardcodedColors.length} exceeds budget ${workflowProgressHardcodedColorBudget}`,
  )
}

if (ybFontSansRefs.length > 0 && !ybFontSansDefined) {
  warnings.push(
    `--yb-font-sans is referenced but intentionally not defined yet: ${ybFontSansRefs.join(', ')}`,
  )
}

const topImportantFiles = [...importantByFile.entries()]
  .sort((a, b) => b[1] - a[1])
  .slice(0, 8)
const topHardcodedColorFiles = [...hardcodedColorsByFile.entries()]
  .map(([file, colors]) => [file, colors.length])
  .sort((a, b) => b[1] - a[1])
  .slice(0, 8)

console.log('Design system audit')
console.log(`- top-level token roots: ${topLevelRootCount}`)
console.log(`- src !important count: ${importantTotal}/${importantBudget}`)
console.log(`- src .vue/.css hardcoded colors: ${hardcodedColorTotal}/${hardcodedColorBudget}`)
console.log(`- malformed token colors: ${malformedTokenColorTotal}/0`)
console.log(`- main.css !important count: ${mainCssImportantCount}/${mainCssImportantBudget}`)
console.log(`- TaskList !important count: ${taskListImportantCount}/${taskListImportantBudget}`)
console.log(`- TaskDetail !important count: ${taskDetailImportantCount}/${taskDetailImportantBudget}`)
console.log(`- TaskAssets !important count: ${taskAssetsImportantCount}/${taskAssetsImportantBudget}`)
console.log(`- AssetsIndex !important count: ${assetsIndexImportantCount}/${assetsIndexImportantBudget}`)
console.log(`- AssetDetail !important count: ${assetDetailImportantCount}/${assetDetailImportantBudget}`)
console.log(`- Dashboard !important count: ${dashboardImportantCount}/${dashboardImportantBudget}`)
console.log(`- AppShell !important count: ${appShellImportantCount}/${appShellImportantBudget}`)
console.log(`- AvatarDropdown !important count: ${avatarDropdownImportantCount}/${avatarDropdownImportantBudget}`)
console.log(`- BaseModal !important count: ${baseModalImportantCount}/${baseModalImportantBudget}`)
console.log(`- BaseSelect !important count: ${baseSelectImportantCount}/${baseSelectImportantBudget}`)
console.log(
  `- GlobalSearchOverlay !important count: ${globalSearchOverlayImportantCount}/${globalSearchOverlayImportantBudget}`,
)
console.log(
  `- TaskCreateModal !important count: ${taskCreateModalImportantCount}/${taskCreateModalImportantBudget}`,
)
console.log(`- UserManagement !important count: ${userManagementImportantCount}/${userManagementImportantBudget}`)
console.log(
  `- TaskInfoEditModal !important count: ${taskInfoEditModalImportantCount}/${taskInfoEditModalImportantBudget}`,
)
console.log(
  `- CloseDraftConfirmModal !important count: ${closeDraftConfirmModalImportantCount}/${closeDraftConfirmModalImportantBudget}`,
)
console.log(
  `- ReassignDesignerDialog !important count: ${reassignDesignerDialogImportantCount}/${reassignDesignerDialogImportantBudget}`,
)
console.log(`- DesignAssetBlock !important count: ${designAssetBlockImportantCount}/${designAssetBlockImportantBudget}`)
console.log(
  `- OutsourceOrderTable !important count: ${outsourceOrderTableImportantCount}/${outsourceOrderTableImportantBudget}`,
)
console.log(`- WorkflowProgress !important count: ${workflowProgressImportantCount}/${workflowProgressImportantBudget}`)
console.log(`- main.css hardcoded colors outside token root: ${mainCssHardcodedColors.length}/${mainCssHardcodedColorBudget}`)
console.log(`- TaskList hardcoded colors: ${taskListHardcodedColors.length}/${taskListHardcodedColorBudget}`)
console.log(`- TaskDetail hardcoded colors: ${taskDetailHardcodedColors.length}/${taskDetailHardcodedColorBudget}`)
console.log(`- AssetsIndex hardcoded colors: ${assetsIndexHardcodedColors.length}/${assetsIndexHardcodedColorBudget}`)
console.log(`- Dashboard hardcoded colors: ${dashboardHardcodedColors.length}/${dashboardHardcodedColorBudget}`)
console.log(`- AppShell hardcoded colors: ${appShellHardcodedColors.length}/${appShellHardcodedColorBudget}`)
console.log(
  `- AvatarDropdown hardcoded colors: ${avatarDropdownHardcodedColors.length}/${avatarDropdownHardcodedColorBudget}`,
)
console.log(`- BaseModal hardcoded colors: ${baseModalHardcodedColors.length}/${baseModalHardcodedColorBudget}`)
console.log(`- BaseSelect hardcoded colors: ${baseSelectHardcodedColors.length}/${baseSelectHardcodedColorBudget}`)
console.log(
  `- GlobalSearchOverlay hardcoded colors: ${globalSearchOverlayHardcodedColors.length}/${globalSearchOverlayHardcodedColorBudget}`,
)
console.log(
  `- CloseDraftConfirmModal hardcoded colors: ${closeDraftConfirmModalHardcodedColors.length}/${closeDraftConfirmModalHardcodedColorBudget}`,
)
console.log(
  `- ReassignDesignerDialog hardcoded colors: ${reassignDesignerDialogHardcodedColors.length}/${reassignDesignerDialogHardcodedColorBudget}`,
)
console.log(
  `- DesignAssetBlock hardcoded colors: ${designAssetBlockHardcodedColors.length}/${designAssetBlockHardcodedColorBudget}`,
)
console.log(
  `- OutsourceOrderTable hardcoded colors: ${outsourceOrderTableHardcodedColors.length}/${outsourceOrderTableHardcodedColorBudget}`,
)
console.log(
  `- WorkflowProgress hardcoded colors: ${workflowProgressHardcodedColors.length}/${workflowProgressHardcodedColorBudget}`,
)
console.log('- highest !important files:')
for (const [file, count] of topImportantFiles) {
  console.log(`  ${count.toString().padStart(4, ' ')} ${file}`)
}
console.log('- highest hardcoded color files:')
for (const [file, count] of topHardcodedColorFiles) {
  console.log(`  ${count.toString().padStart(4, ' ')} ${file}`)
}

if (warnings.length > 0) {
  console.log('- warnings:')
  for (const warning of warnings) {
    console.log(`  ${warning}`)
  }
}

if (failures.length > 0) {
  console.error('Design system audit failed:')
  for (const failure of failures) {
    console.error(`- ${failure}`)
  }
  if (mainCssHardcodedColors.length > 0) {
    console.error('- main.css hardcoded color samples:')
    for (const sample of mainCssHardcodedColors.slice(0, 12)) {
      console.error(`  ${sample.line}: ${sample.value}`)
    }
  }
  if (hardcodedColorTotal > 0) {
    console.error('- src .vue/.css hardcoded color samples:')
    for (const [file, colors] of [...hardcodedColorsByFile.entries()].slice(0, 8)) {
      for (const sample of colors.slice(0, 4)) {
        console.error(`  ${file}:${sample.line}: ${sample.value}`)
      }
    }
  }
  if (malformedTokenColorTotal > 0) {
    console.error('- malformed token color samples:')
    for (const [file, colors] of [...malformedTokenColorsByFile.entries()].slice(0, 8)) {
      for (const sample of colors.slice(0, 4)) {
        console.error(`  ${file}:${sample.line}: ${sample.value}`)
      }
    }
  }
  process.exit(1)
}

async function collectFiles(dir) {
  const entries = await readdir(dir, { withFileTypes: true })
  const result = []
  for (const entry of entries) {
    const fullPath = path.join(dir, entry.name)
    if (entry.isDirectory()) {
      result.push(...(await collectFiles(fullPath)))
      continue
    }
    if (sourceExtensions.has(path.extname(entry.name))) {
      result.push(fullPath)
    }
  }
  return result
}

function countMatches(source, pattern) {
  return [...source.matchAll(pattern)].length
}

function stripTopLevelRoot(css) {
  const lines = css.split(/\r?\n/)
  const rootStart = lines.findIndex((line) => /^:root\s*\{/.test(line))
  if (rootStart === -1) return css

  let depth = 0
  let rootEnd = rootStart
  for (let index = rootStart; index < lines.length; index += 1) {
    depth += countMatches(lines[index], /\{/g)
    depth -= countMatches(lines[index], /\}/g)
    if (depth === 0) {
      rootEnd = index
      break
    }
  }

  return [...lines.slice(0, rootStart), ...lines.slice(rootEnd + 1)].join('\n')
}

function findHardcodedColors(source) {
  const regex = /#[0-9a-fA-F]{3,8}\b|rgba?\(\s*\d/g
  const results = []
  const lines = source.split(/\r?\n/)
  for (const [index, line] of lines.entries()) {
    for (const match of line.matchAll(regex)) {
      results.push({ line: index + 1, value: match[0] })
    }
  }
  return results
}

function findMalformedTokenColors(source) {
  const regex = /rgba?\(var\(--yb-[^)]+\)\)[0-9a-fA-F]+\b/g
  const results = []
  const lines = source.split(/\r?\n/)
  for (const [index, line] of lines.entries()) {
    for (const match of line.matchAll(regex)) {
      results.push({ line: index + 1, value: match[0] })
    }
  }
  return results
}
