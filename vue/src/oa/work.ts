import '../main'
import './workbench.css'
import { getOrderTaskContext } from './orderTask'

const contextBanner = document.getElementById('oa-order-context')

function refreshContext() {
  const context = getOrderTaskContext()
  if (!contextBanner) return
  contextBanner.hidden = !context
  contextBanner.textContent = context ? `经营来源 · 店铺 ${context.shop_id} · SKU ${context.sku} · ${context.from} 至 ${context.to}` : ''
}

window.addEventListener('hashchange', refreshContext)
refreshContext()
