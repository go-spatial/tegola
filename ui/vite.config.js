/* eslint-disable */

//import * as path from "path";
// import vuePlugin from '@vitejs/plugin-vue';
// import legacyPlugin from '@vitejs/plugin-legacy';
// const { createVuePlugin } = require("vite-plugin-vue2");


// import { createVuePlugin } from 'vite-plugin-vue2';
import vue from '@vitejs/plugin-vue'
import path from 'path';
import { defineConfig } from 'vite';
// const path = require('path')

// let alias = {
//   "@": path.resolve(__dirname, "src"),
//   //vue$: "vue/dist/vue.runtime.esm.js"
// };

export default defineConfig({

  plugins: [vue()],

  resolve: {
    alias: {
      "@": path.resolve(__dirname, "src")
    },
     },
})
