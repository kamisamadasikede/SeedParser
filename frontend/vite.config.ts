import { fileURLToPath, URL } from "node:url";
import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import vueJsx from "@vitejs/plugin-vue-jsx";

// https://vitejs.dev/config/
export default defineConfig(({ mode }) => ({
  // 修复核心1：生产环境设置相对路径（Wails 本地 file 协议必须用 ./）
  base: mode === "production" ? "./" : "/",

  plugins: [vue(), vueJsx()],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  build: {
    // 修复核心2：禁用 CSS 代码分割（避免部分样式加载失败导致侧边栏空白）
    cssCodeSplit: false,
    // 修复核心5：内联动态导入，确保没有单独的 chunk 文件需要加载
    inlineDynamicImports: true,
    rollupOptions: {
      output: {
        assetFileNames: (assetInfo) => {
          let extType = assetInfo.name.split('.').at(1);
          if (/png|jpe?g|svg|gif|tiff|bmp|ico/i.test(extType)) {
            extType = 'img';
          } else if (/woff2?|ttf|otf|eot/i.test(extType)) {
            extType = 'fonts';
          }
          // 修复核心3：asset 路径保持相对（不要加 /）
          return `assets/${extType}/[name]-[hash][extname]`;
        },
        chunkFileNames: 'assets/js/[name]-[hash].js',
        entryFileNames: 'assets/js/[name]-[hash].js',
        // 修复核心4：确保静态资源导入为相对路径
        manualChunks: undefined, // 禁用代码分割（可选，减少 chunk 加载问题）
      },
    },
  },
  // 确保字体/图片文件被正确识别
  assetsInclude: ['**/*.woff', '**/*.woff2', '**/*.ttf', '**/*.eot', '**/*.ico', '**/*.png'],
  server: {
    fs: {
      strict: false,
    },
  },
}));