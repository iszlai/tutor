import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// Dev server proxies /api to the Go backend. `host: true` exposes it on the
// LAN so you can open it from a phone on the same wifi.
export default defineConfig({
  plugins: [react()],
  server: {
    host: true,
    proxy: {
      "/api": "http://localhost:8787",
    },
  },
});
