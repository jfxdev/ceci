import path from "node:path"
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  server: {
    port: 8111,
    strictPort: true,
    proxy: {
      "/api": "http://localhost:8110",
      "/ofrep": "http://localhost:8110",
    },
  },
})
