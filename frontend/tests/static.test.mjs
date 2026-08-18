import assert from "node:assert/strict";
import { access, readFile } from "node:fs/promises";
import { dirname, join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const frontendRoot = join(dirname(fileURLToPath(import.meta.url)), "..");
const read = (path) => readFile(join(frontendRoot, path), "utf8");

test("HTML references only present local entry assets", async () => {
  const html = await read("index.html");
  for (const asset of ["styles.css", "app.js", "bootstrap.js", "assets/alzette-mark.svg"]) {
    assert.match(html, new RegExp(asset.replace(".", "\\.")));
    await access(join(frontendRoot, asset));
  }
  assert.doesNotMatch(html, /https?:\/\//, "the UI must remain offline-renderable");
});

test("build lifecycle prepares every Wails embedded asset", async () => {
  const html = await read("dist/index.html");
  const references = [...html.matchAll(/(?:src|href)="([^"#]+)"/g)]
    .map((match) => match[1])
    .filter((path) => !/^(?:https?:|data:)/.test(path));
  await access(join(frontendRoot, "dist/index.html"));
  for (const reference of references) {
    await access(join(frontendRoot, "dist", reference.replace(/^\//, "")));
  }
  assert.ok(references.some((path) => path.endsWith(".css")), "built HTML must reference CSS");
  assert.ok(references.some((path) => path.endsWith(".js")), "built HTML must reference JavaScript");
  assert.ok(references.some((path) => path.endsWith(".svg")), "built HTML must reference the mark");
});

test("all required connection states are implemented", async () => {
  const script = await read("app.js");
  for (const state of ["connected", "signed-out", "connecting", "offline", "no-models", "setup-attention", "access-ended"]) {
    assert.match(script, new RegExp(`['\"]${state}['\"]`));
  }
});

test("runtime state boundary avoids credential persistence and HTML injection", async () => {
  const appScript = await read("app.js");
  const bootstrapScript = await read("bootstrap.js");
  const nativeScript = await read("native.js");
  const script = `${appScript}\n${bootstrapScript}\n${nativeScript}`;
  assert.match(script, /window\.AlzetteConnect/);
  assert.match(script, /connect:state/);
  assert.match(nativeScript, /CurrentState/);
  assert.match(nativeScript, /BeginSignIn/);
  assert.match(bootstrapScript, /wails:/);
  assert.doesNotMatch(script, /innerHTML|outerHTML|insertAdjacentHTML/);
  assert.doesNotMatch(script, /localStorage|sessionStorage|indexedDB/i);
});

test("document IDs are unique", async () => {
  const html = await read("index.html");
  const ids = [...html.matchAll(/\sid="([^"]+)"/g)].map((match) => match[1]);
  assert.equal(new Set(ids).size, ids.length);
});

test("production copy uses verified client versions and bounded setup claims", async () => {
  const html = await read("index.html");
  assert.match(html, /Version 0\.8\.4/);
  assert.match(html, /Version 1\.46\.0/);
  assert.doesNotMatch(html, /Automatic setup/i);
});
