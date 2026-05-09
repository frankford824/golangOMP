export interface MockBlueprintModule {
  module_key: string
  initial_state: string
}

export interface MockBlueprint {
  key: string
  task_type: string
  modules: MockBlueprintModule[]
}

export const mockBlueprints: MockBlueprint[] = [
  {
    key: 'bp_original',
    task_type: 'original_product_development',
    modules: [
      { module_key: 'basic_info', initial_state: 'completed' },
      { module_key: 'design', initial_state: 'pending_claim' },
      { module_key: 'audit', initial_state: 'pending_claim' },
      { module_key: 'warehouse', initial_state: 'pending' },
    ],
  },
  {
    key: 'bp_new_dev',
    task_type: 'new_product_development',
    modules: [
      { module_key: 'basic_info', initial_state: 'completed' },
      { module_key: 'design', initial_state: 'pending_claim' },
      { module_key: 'audit', initial_state: 'pending_claim' },
      { module_key: 'warehouse', initial_state: 'pending' },
    ],
  },
  {
    key: 'bp_new_single',
    task_type: 'new_product_single',
    modules: [
      { module_key: 'basic_info', initial_state: 'completed' },
      { module_key: 'design', initial_state: 'pending_claim' },
      { module_key: 'audit', initial_state: 'pending_claim' },
      { module_key: 'warehouse', initial_state: 'pending' },
    ],
  },
  {
    key: 'bp_new_batch',
    task_type: 'new_product_batch',
    modules: [
      { module_key: 'basic_info', initial_state: 'completed' },
      { module_key: 'design', initial_state: 'pending_claim' },
      { module_key: 'audit', initial_state: 'pending_claim' },
      { module_key: 'warehouse', initial_state: 'pending' },
    ],
  },
  {
    key: 'bp_purchase',
    task_type: 'purchase_task',
    modules: [
      { module_key: 'basic_info', initial_state: 'completed' },
      { module_key: 'procurement', initial_state: 'pending_claim' },
      { module_key: 'warehouse', initial_state: 'pending' },
    ],
  },
  {
    key: 'bp_purchase_single',
    task_type: 'purchase_single',
    modules: [
      { module_key: 'basic_info', initial_state: 'completed' },
      { module_key: 'procurement', initial_state: 'in_progress' },
      { module_key: 'warehouse', initial_state: 'pending' },
    ],
  },
  {
    key: 'bp_retouch',
    task_type: 'retouch_task',
    modules: [
      { module_key: 'basic_info', initial_state: 'completed' },
      { module_key: 'retouch', initial_state: 'pending_claim' },
      { module_key: 'warehouse', initial_state: 'pending' },
    ],
  },
  {
    key: 'bp_retouch_legacy',
    task_type: 'retouch',
    modules: [
      { module_key: 'basic_info', initial_state: 'completed' },
      { module_key: 'retouch', initial_state: 'pending_claim' },
      { module_key: 'audit', initial_state: 'pending_claim' },
      { module_key: 'warehouse', initial_state: 'pending' },
    ],
  },
  {
    key: 'bp_customization',
    task_type: 'regular_customization',
    modules: [
      { module_key: 'basic_info', initial_state: 'completed' },
      { module_key: 'customization', initial_state: 'pending_claim' },
      { module_key: 'audit', initial_state: 'pending_claim' },
      { module_key: 'warehouse', initial_state: 'pending' },
    ],
  },
  {
    key: 'bp_customer_cust',
    task_type: 'customer_customization',
    modules: [
      { module_key: 'basic_info', initial_state: 'completed' },
      { module_key: 'customization', initial_state: 'pending_claim' },
      { module_key: 'audit', initial_state: 'pending_claim' },
      { module_key: 'warehouse', initial_state: 'pending' },
    ],
  },
]
