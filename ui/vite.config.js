/* eslint-disable */

//import * as path from "path";
// import vuePlugin from '@vitejs/plugin-vue';
// import legacyPlugin from '@vitejs/plugin-legacy';
// const { createVuePlugin } = require("vite-plugin-vue2");

import { defineConfig } from 'vite';
import { createVuePlugin } from 'vite-plugin-vue2';
import path from 'path';

// https://cn.vitejs.dev/config/
export default ({ command, mode }) => {
  let rollupOptions = {};

  let optimizeDeps = {};

  let alias = {
    "@": path.resolve(__dirname, "src"),
    vue$: "vue/dist/vue.runtime.esm.js"
  };

  let proxy = {};

  let esbuild = {};

  return {
    base: "./", // index.html文件所在位置
    root: "./", // js导入的资源路径，src
    resolve: {
      alias
    },
    define: {
      "process.env.APP_IS_LOCAL": "'true'"
    },
    server: {
      // 代理
      proxy
    },
    build: {
      target: "es2015",
      minify: "terser", // 是否进行压缩,boolean | 'terser' | 'esbuild',默认使用terser
      manifest: false, // 是否产出maifest.json
      sourcemap: false, // 是否产出soucemap.json
      outDir: "build", // 产出目录
      rollupOptions
    },
    esbuild,
    optimizeDeps,
    plugins: [
      //vuePlugin(),
      createVuePlugin(),
      // legacyPlugin({
      //   targets: [
      //     "Android > 39",
      //     "Chrome >= 60",
      //     "Safari >= 10.1",
      //     "iOS >= 10.3",
      //     "Firefox >= 54",
      //     "Edge >= 15"
      //   ]
      // })
    ],
    // css: {
    //   preprocessorOptions: {
    //     less: {
    //       // 支持内联 JavaScript
    //       javascriptEnabled: true
    //     }
    //   }
    // }
  };
};
