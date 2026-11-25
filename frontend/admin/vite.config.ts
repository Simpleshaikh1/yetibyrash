import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// The admin lives under /admin/ (in production the Go binary serves it there).
// In development open http://localhost:5174/admin/
export default defineConfig({
  base: "/admin/",
  plugins: [react()],
  server: {
    // IPv4 loopback: Node alone picks IPv6 (::1), which some browsers on Windows can't reach via 127.0.0.1.
    host: "127.0.0.1",
    port: 5174,
    strictPort: true,
    proxy: {
      "/api": "http://localhost:8090",
    },
  },
});
