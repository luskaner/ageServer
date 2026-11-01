import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import wails from "@wailsio/runtime/plugins/vite";

// https://vitejs.dev/config/
export default defineConfig({
  server: {
    host: "127.0.0.1",
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
    // The project can live on a network drive, where native file watchers do
    // not deliver events: polling keeps HMR and full reloads working there.
    watch: {
      usePolling: true,
      interval: 300,
    },
  },
  plugins: [react(), wails("./bindings")],
});
