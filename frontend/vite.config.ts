import { defineConfig } from "vite";

// Dev server proxies the API and basemap to a `just serve` running on
// 127.0.0.1:8867 (docs/WALKMAP.md piece 3), so the SPA only ever talks to
// its own origin -- no third-party hosts, in dev or in the built app.
export default defineConfig({
  server: {
    proxy: {
      "/api": "http://127.0.0.1:8867",
      "/basemap.pmtiles": "http://127.0.0.1:8867",
    },
  },
  build: {
    outDir: "dist",
    // main.ts uses a top-level await to resolve the vendored style's
    // glyphs/sprite URLs before creating the map; es2022 covers every
    // evergreen mobile/desktop browser this app targets.
    target: "es2022",
  },
});
