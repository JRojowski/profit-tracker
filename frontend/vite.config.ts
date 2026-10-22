import { resolve } from "node:path"
import tailwindcss from "@tailwindcss/vite"
import react from "@vitejs/plugin-react"
import { defineConfig } from "vite"

// The Go program reads the same variable. Keep the default equal to its default.
const backendAddr = process.env.PROFIT_TRACKER_ADDR ?? "127.0.0.1:8080"

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": resolve(import.meta.dirname, "./src"),
    },
  },
  server: {
    // Send API requests to the Go program. Start it with `make backend-dev`.
    proxy: {
      "/api": `http://${backendAddr}`,
    },
  },
})
