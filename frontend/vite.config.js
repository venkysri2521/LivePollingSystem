import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// In development the Go service runs on :8080 and Vite proxies to it, so the
// browser sees one origin and CORS never enters the picture locally.
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      "/api": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
    },
  },
});
