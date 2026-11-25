import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// The shop runs on :5173 and forwards /api to the Go API on :8090,
// so the browser sees one address and cookies work without CORS setup.
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    strictPort: true,
    proxy: {
      "/api": "http://localhost:8090",
    },
  },
});
