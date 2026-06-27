self.onmessage = async (event: MessageEvent<{ id: string; file: File }>) => {
  const { id, file } = event.data
  try {
    if (!self.crypto?.subtle) {
      throw new Error('当前浏览器不支持文件哈希计算')
    }
    const buffer = await file.arrayBuffer()
    const digest = await self.crypto.subtle.digest('SHA-256', buffer)
    const hash = Array.from(new Uint8Array(digest))
      .map((byte) => byte.toString(16).padStart(2, '0'))
      .join('')
    self.postMessage({ id, hash })
  } catch (err) {
    self.postMessage({ id, error: err instanceof Error ? err.message : '文件哈希计算失败' })
  }
}
