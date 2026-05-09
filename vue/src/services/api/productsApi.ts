import type { Product } from '@/domain/types/product'
import { mockProducts } from '@/mock/products'

export async function fetchProductList(): Promise<Product[]> {
  return Promise.resolve([...mockProducts])
}

export async function fetchProductById(id: string): Promise<Product | null> {
  const list = await fetchProductList()
  return list.find((p) => p.id === id) ?? null
}
