import type { MockHandler } from './types'
import { getBeijingDateCompactString } from '@/utils/date'

export const batchSkuHandler: MockHandler = (request) => {
  if (request.method === 'GET' && request.path === '/v1/tasks/batch-create/template.xlsx') {
    return {
      status: 200,
      data: {
        file_name: `新款开发-批量SKU模板-${getBeijingDateCompactString()}.xlsx`,
        text_csv: 'product_name,design_requirement,product_i_id,参考图\n商品A,设计要求A,KT001,\n商品B,设计要求B,,',
        mock: true,
      },
    }
  }

  if (request.method === 'POST' && request.path === '/v1/tasks/batch-create/parse-excel') {
    const taskType = (request.body as Record<string, unknown>)?.task_type ?? 'new_product_development'
    return {
      status: 200,
      data: {
        data: {
          task_type: taskType,
          preview: [
            {
              product_name: '产品A',
              design_requirement: '设计要求A',
              product_i_id: 'KT001',
              reference_file_refs: [
                {
                  asset_id: 'mock-asset-1',
                  ref_id: 'mock-ref-1',
                  filename: 'ref-image-1.png',
                  mime_type: 'image/png',
                  file_size: 12345,
                  download_url: '/v1/assets/files/mock-asset-1',
                  storage_key: 'mock-storage-key-1',
                },
              ],
            },
            {
              product_name: '产品B',
              design_requirement: '设计要求B',
              product_i_id: '',
              reference_file_refs: [],
            },
            {
              product_name: '产品C',
              design_requirement: '设计要求C',
              product_i_id: 'INVALID_ID',
              reference_file_refs: [],
            },
          ],
          violations: [
            {
              row: 3,
              column: '产品i_id',
              code: 'invalid_i_id',
              message: 'batch_items[].product_i_id must be selected from ERP product i_id options',
            },
          ],
        },
      },
    }
  }

  return null
}
