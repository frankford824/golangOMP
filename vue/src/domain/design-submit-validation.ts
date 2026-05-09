import { isBitmapDeliveryFile } from '@/domain/constants/upload-types'

export function shouldWarnForMissingBitmapDelivery(fileNames: string[]): boolean {
  return !fileNames.some((name) => isBitmapDeliveryFile(name))
}
