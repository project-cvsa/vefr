import { cp, mkdir, rm } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const siteDir = dirname(fileURLToPath(import.meta.url));
const outputDir = resolve(siteDir, "dist");

await rm(outputDir, { recursive: true, force: true });
await mkdir(outputDir, { recursive: true });
await cp(resolve(siteDir, "index.html"), resolve(outputDir, "index.html"));
await cp(resolve(siteDir, "..", "scripts", "install.sh"), resolve(outputDir, "install.sh"));

console.log(`Built ${outputDir}`);
