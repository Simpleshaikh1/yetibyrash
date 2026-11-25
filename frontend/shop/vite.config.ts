import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// The shop runs on :5173 and forwards /api to the Go API on :8090,
// so the browser sees one address and cookies work without CORS setup.
export default defineConfig({
  plugins: [react()],
  server: {
    // IPv4 loopback: Node alone picks IPv6 (::1), which some browsers on Windows can't reach via 127.0.0.1.
    host: "127.0.0.1",
    port: 5173,
    strictPort: true,
    hmr: { host: "127.0.0.1" }, // live reload connects over IPv4 even when the page is opened as localhost
    // Lets a Cloudflare quick tunnel (https://….trycloudflare.com) show the site to someone else. See README.
    allowedHosts: [".trycloudflare.com"],
    proxy: {
      "/api": "http://localhost:8090",
      "/media": "http://localhost:8090",
    },
  },
});
