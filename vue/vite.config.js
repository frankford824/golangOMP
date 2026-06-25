import { defineConfig, loadEnv } from 'vite';
import vue from '@vitejs/plugin-vue';
import path from 'path';
import { fileURLToPath } from 'node:url';
export default defineConfig(function (_a) {
    var _b;
    var mode = _a.mode;
    var env = loadEnv(mode, process.cwd(), 'VITE_');
    var devProxyTarget = (_b = env.VITE_DEV_API_PROXY_TARGET) === null || _b === void 0 ? void 0 : _b.trim();
    var proxy = devProxyTarget
        ? {
            // 开发机：将相对 `/v1/...` 转发到后端（仅影响 `vite dev`）
            '/v1': {
                target: devProxyTarget,
                changeOrigin: true,
                secure: false,
                rewrite: function (path) { return path; },
            },
        }
        : undefined;
    return {
        plugins: [vue()],
        resolve: {
            alias: {
                '@': path.join(path.dirname(fileURLToPath(import.meta.url)), 'src'),
            },
        },
        server: {
            host: '0.0.0.0',
            proxy: proxy,
        },
        build: {
            // `vite build --mode test` 输出到 `dist-test/`，避免覆盖生产 `dist/`
            outDir: mode === 'test' ? 'dist-test' : 'dist',
            chunkSizeWarningLimit: 1100,
            rollupOptions: {
                output: {
                    manualChunks: function (id) {
                        if (!id.includes('node_modules'))
                            return undefined;
                        if (id.includes('/echarts/'))
                            return 'charts';
                        if (id.includes('/exceljs/'))
                            return 'excel';
                        if (id.includes('/jszip/'))
                            return 'zip';
                        if (id.includes('/vue/') || id.includes('/vue-router/') || id.includes('/pinia/')) {
                            return 'vue-vendor';
                        }
                        return 'vendor';
                    },
                },
            },
        },
    };
});
