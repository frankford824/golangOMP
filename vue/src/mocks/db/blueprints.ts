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
    ],
  },
  {
    key: 'bp_new_dev',
    task_type: 'new_product_development',
    modules: [
      { module_key: 'basic_info', initial_state: 'completed' },
      { module_key: 'design', initial_state: 'pending_claim' },
      { module_key: 'audit', initial_state: 'pending_claim' },
    ],
  },
  {
    key: 'bp_new_single',
    task_type: 'new_product_single',
    modules: [
      { module_key: 'basic_info', initial_state: 'completed' },
      { module_key: 'design', initial_state: 'pending_claim' },
      { module_key: 'audit', initial_state: 'pending_claim' },
    ],
  },
  {
    key: 'bp_new_batch',
    task_type: 'new_product_batch',
    modules: [
      { module_key: 'basic_info', initial_state: 'completed' },
      { module_key: 'design', initial_state: 'pending_claim' },
      { module_key: 'audit', initial_state: 'pending_claim' },
    ],
  },
  {
    key: 'bp_sku_planning',
    task_type: 'sku_planning',
    modules: [
      { module_key: 'basic_info', initial_state: 'completed' },
    ],
  },
  {
    key: 'bp_retouch',
    task_type: 'retouch_task',
    modules: [
      { module_key: 'basic_info', initial_state: 'completed' },
      { module_key: 'retouch', initial_state: 'pending_claim' },
    ],
  },
  {
    key: 'bp_customization',
    task_type: 'regular_customization',
    modules: [
      { module_key: 'basic_info', initial_state: 'completed' },
      { module_key: 'design', initial_state: 'pending_claim' },
      { module_key: 'customization', initial_state: 'pending_claim' },
      { module_key: 'audit', initial_state: 'pending_claim' },
    ],
  },
  {
    key: 'bp_customer_cust',
    task_type: 'customer_customization',
    modules: [
      { module_key: 'basic_info', initial_state: 'completed' },
      { module_key: 'design', initial_state: 'pending_claim' },
      { module_key: 'customization', initial_state: 'pending_claim' },
      { module_key: 'audit', initial_state: 'pending_claim' },
    ],
  },
]
