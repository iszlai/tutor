import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// Dev server proxies /api to the Go backend. `host: true` exposes it on the
// LAN so you can open it from a phone on the same wifi.
export default defineConfig({
  plugins: [react()],
  server: {
    host: true,
    proxy: {
      "/api": {
        target: "http://localhost:8787",
        // Remove Accept-Encoding so the Go server never compresses SSE responses —
        // compression buffers the entire stream before sending, killing token-by-token delivery.
        configure: (proxy) => {
          proxy.on("proxyReq", (proxyReq) => {
            proxyReq.removeHeader("accept-encoding");
          });
        },
      },
    },
  },
});
