import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

export default defineConfig({
  base: "./",
  plugins: [react()],
  build: {
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes("node_modules/echarts") || id.includes("node_modules/zrender")) {
            return "echarts";
          }
          if (id.includes("node_modules/react")) {
            return "react";
          }
        },
      },
    },
  },
  test: {
    exclude: ["e2e/**", "node_modules/**", "dist/**", "dist-electron/**"],
  },
  server: {
    host: "127.0.0.1",
    port: 5173,
  },
});
