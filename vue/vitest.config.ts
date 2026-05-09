import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'
import path from 'path'
import { fileURLToPath } from 'node:url'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': path.join(path.dirname(fileURLToPath(import.meta.url)), 'src'),
    },
  },
  test: {
    globals: true,
    environment: 'jsdom',
  },
})
