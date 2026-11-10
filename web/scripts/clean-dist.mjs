// Removes the previous SPA build from backend/internal/webui/dist while keeping
// the backend's tracked placeholder files (.gitkeep). Vite runs with
// emptyOutDir=false so nothing outside this directory is ever touched.
import { readdirSync, rmSync, existsSync } from 'node:fs';
import { join, dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const outDir = resolve(here, '../../backend/internal/webui/dist');
const KEEP = new Set(['.gitkeep']);

if (existsSync(outDir)) {
  for (const entry of readdirSync(outDir)) {
    if (KEEP.has(entry)) continue;
    rmSync(join(outDir, entry), { recursive: true, force: true });
  }
}
console.log(`cleaned ${outDir}`);
