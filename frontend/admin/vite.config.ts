import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// The admin lives under /admin/ (in production the Go binary serves it there).
// In development open http://localhost:5174/admin/
export default defineConfig({
  base: "/admin/",
  plugins: [react()],
  server: {
    port: 5174,
    strictPort: true,
    proxy: {
      "/api": "http://localhost:8090",
    },
  },
});
