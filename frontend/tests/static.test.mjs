import assert from "node:assert/strict";
import { access, readFile, readdir } from "node:fs/promises";
import { dirname, join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const frontendRoot = join(dirname(fileURLToPath(import.meta.url)), "..");
const read = (path) => readFile(join(frontendRoot, path), "utf8");

test("HTML references only present local entry assets", async () => {
  const html = await read("index.html");
  for (const asset of ["styles.css", "app.js", "bootstrap.js", "assets/alzette-mark.svg"]) {
    assert.match(html, new RegExp(asset.replaceAll(".", "\\.")));
    await access(join(frontendRoot, asset));
  }
  assert.doesNotMatch(html, /https?:\/\//, "the UI must remain offline-renderable");
});

test("build lifecycle prepares every Wails embedded asset", async () => {
  const html = await read("dist/index.html");
  const references = [...html.matchAll(/(?:src|href)="([^"#]+)"/g)]
    .map((match) => match[1])
    .filter((path) => !/^(?:https?:|data:)/.test(path));
  for (const reference of references) await access(join(frontendRoot, "dist", reference.replace(/^\//, "")));
  assert.ok(references.some((path) => path.endsWith(".css")));
  assert.ok(references.some((path) => path.endsWith(".js")));
  const assets = await readdir(join(frontendRoot, "dist", "assets"));
  assert.ok(assets.some((name) => name.endsWith(".js")));
});

test("approved launcher lifecycle and context choice are present", async () => {
  const html = await read("index.html");
  for (const screen of ["signed-out", "contexts", "launcher", "preparing", "running", "recovery"]) {
    assert.match(html, new RegExp(`data-screen="${screen}"`));
  }
  assert.match(html, /data-application-list/);
  assert.match(html, /data-model-list/);
  assert.match(html, /data-disconnect/);
  assert.match(html, /data-cancel-launch/);
  assert.match(html, /Double-click or press Enter/);
});

test("runtime state boundary avoids fixture claims, credential persistence, and HTML injection", async () => {
  const appScript = await read("app.js");
  const bootstrapScript = await read("bootstrap.js");
  const nativeScript = await read("native.js");
  const html = await read("index.html");
  const script = `${appScript}\n${bootstrapScript}\n${nativeScript}`;
  assert.match(script, /window\.AlzetteConnect/);
  assert.match(script, /connect:state/);
  for (const action of ["SelectContext", "LaunchApplication", "CancelLaunch", "Disconnect", "RetryCleanup", "HideToTray"]) {
    assert.match(nativeScript, new RegExp(action));
  }
  assert.doesNotMatch(script, /innerHTML|outerHTML|insertAdjacentHTML/);
  assert.doesNotMatch(script, /localStorage|sessionStorage|indexedDB/i);
  assert.doesNotMatch(`${html}\n${appScript}`, /Northstar|Alex Morgan|Example Bank|approved-coder|reasoning-private/);
  assert.doesNotMatch(html, /CONCEPT|illustrative catalogue|prototype/i);
  assert.match(html, /data-retry-cleanup/);
  assert.doesNotMatch(html, /recovery guide is not bundled/i);
});

test("application readiness is qualified and future adapters remain disabled", async () => {
  const script = await read("app.js");
  for (const status of ["ready", "verification_required", "not_installed", "update_required", "protocol_unavailable", "blocked_by_company", "not_supported"]) {
    assert.match(script, new RegExp(`\\b${status}\\b`));
  }
  assert.match(script, /Not yet supported/);
  assert.match(script, /chatgpt:\s*"ChatGPT"/);
  assert.match(script, /first listed company model.*complete catalogue available in its model picker/);
  assert.doesNotMatch(script, /Claude Code/);
  assert.doesNotMatch(await read("index.html"), /Protocol unavailable/);
});

test("stale authority, terminal access, large catalogues, and tray fallback stay truthful", async () => {
  const html = await read("index.html");
  const script = await read("app.js");
  const styles = await read("styles.css");
  assert.match(html, /data-authority-warning/);
  assert.match(script, /Last known company access · reconnect required/);
  assert.match(script, /Your company access has ended/);
  assert.match(script, /trayAvailable/);
  assert.match(styles, /overflow-y:\s*auto/);
  assert.match(script, /event\.key !== "Escape"/);
  assert.doesNotMatch(html, /role="listbox"/);
});

test("document IDs are unique and live regions are bounded", async () => {
  const html = await read("index.html");
  const ids = [...html.matchAll(/\sid="([^"]+)"/g)].map((match) => match[1]);
  assert.equal(new Set(ids).size, ids.length);
  assert.match(html, /aria-live="polite"/);
  assert.doesNotMatch(html, /aria-live="assertive"/);
});

test("native update controls remain bounded without exposing release URLs", async () => {
  const html = await read("index.html");
  const appScript = await read("app.js");
  const nativeScript = await read("native.js");
  assert.match(html, /data-check-update/);
  assert.match(html, /data-install-update/);
  for (const state of ["checking", "current", "available", "downloading", "installing", "installer_opened", "error"]) {
    assert.match(appScript, new RegExp(`"${state}"`));
  }
  assert.match(nativeScript, /CheckForUpdates/);
  assert.match(nativeScript, /InstallUpdate/);
  assert.doesNotMatch(`${html}\n${appScript}`, /browser_download_url|release-assets\.githubusercontent\.com/);
});
