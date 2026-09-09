// maplibre-gl v6 loads its tile/glyph worker from a separate module file at
// runtime, resolved against its own module URL -- a URL that doesn't
// survive bundling. The worker file also has a static `import` of a sibling
// "maplibre-gl-shared.mjs" chunk, so both must be copied, unbundled, into
// the same directory and served at a stable path (main.ts points
// maplibregl.setWorkerUrl() at it). Runs before `dev`/`build` (predev/
// prebuild); output is gitignored, regenerated from node_modules each time.
import { copyFileSync, mkdirSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const src = join(here, "..", "node_modules", "maplibre-gl", "dist");
const dest = join(here, "..", "public", "vendor", "maplibre");

mkdirSync(dest, { recursive: true });
for (const file of ["maplibre-gl-worker.mjs", "maplibre-gl-shared.mjs"]) {
  copyFileSync(join(src, file), join(dest, file));
}
