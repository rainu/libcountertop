// Synthetic UI checks against a temporary real directory tree, not a device.
import assert from "node:assert/strict";
import { createServer } from "node:http";
import { mkdtemp, mkdir, writeFile, symlink, readdir, lstat, readFile, rm } from "node:fs/promises";
import { resolve, extname, join } from "node:path";
import { tmpdir } from "node:os";
import { chromium } from "playwright";
const root = await mkdtemp(join(tmpdir(), "devicefs-ui-"));
await mkdir(join(root, "assets"));
await mkdir(join(root, "many"));
await mkdir(join(root, "uploads"));
await writeFile(join(root, "assets", "actual.txt"), "fixture only");
await symlink("assets", join(root, "alias"));
for (let i = 0; i < 270; i++) await writeFile(join(root, "many", "item-" + String(i).padStart(3, "0")), "");
const dist = resolve(import.meta.dirname, "../../pkg/runtime/webui/dist");
const errors = [];
let deny = false;
const server = createServer(async (req, res) => {
 const url = new URL(req.url, "http://localhost"), route = url.pathname;
 const send = (status, data) => { res.writeHead(status, { "Content-Type": "application/json" }); res.end(JSON.stringify(data)); };
 if (route.startsWith("/api/v1/")) {
  if (route === "/api/v1/device") return send(200, { name: "Synthetic device", pages: ["files", "filesystem"], screen: false, wifi: false });
  if (route === "/api/v1/settings" || route === "/api/v1/entities") return send(200, []);
  if (route === "/api/v1/events") { res.setHeader("Content-Type", "text/event-stream"); return res.end('data: {"type":"states","states":[]}\n\n'); }
  if (deny) return send(401, {});
  if (route === "/api/v1/files") return send(200, { directory: "/uploads", files: [], usedBytes: 0, availableBytes: 536870912, budgetBytes: 536870912, maxFileBytes: 134217728, maxFiles: 256 });
  if (route === "/api/v1/filesystem") {
   const path = url.searchParams.get("path") || "/", offset = Number(url.searchParams.get("offset") || 0);
   if (path.includes("..")) return send(400, { error: "invalid_path" });
   if (path === "/denied") return send(403, { error: "permission_denied" });
   try {
    const absolute = join(root, path);
    if (!(await lstat(absolute)).isDirectory()) return send(409, { error: "not_directory" });
    const names = (await readdir(absolute)).sort(), entries = [];
    for (const name of names.slice(offset, offset + 256)) {
     const info = await lstat(join(absolute, name));
     entries.push({ name, kind: info.isSymbolicLink() ? "symlink" : info.isDirectory() ? "directory" : "file", size: info.size, modified: info.mtime.toISOString(), mode: info.isDirectory() ? "drwx------" : "-rw-------" });
    }
    return send(200, { path, parent: path === "/" ? "" : "/", entries, ...(names.length > offset + 256 ? { nextOffset: offset + 256 } : {}) });
   } catch { return send(404, { error: "not_found" }); }
  }
  return send(404, {});
 }
 try { const data = await readFile(resolve(dist, "." + (route === "/" ? "/index.html" : route))); res.setHeader("Content-Type", extname(route) === ".js" ? "text/javascript" : extname(route) === ".css" ? "text/css" : "text/html"); res.end(data); } catch { res.writeHead(404); res.end(); }
});
await new Promise((r) => server.listen(0, "127.0.0.1", r));
const browser = await chromium.launch({ headless: true, args: ["--no-sandbox"] });
try {
 const page = await browser.newPage({ viewport: { width: 1200, height: 950 } });
 page.on("pageerror", (e) => errors.push(e.message));
 await page.goto("http://127.0.0.1:" + server.address().port + "/#files");
 await page.getByRole("button", { name: "assets/", exact: true }).click();
 await page.getByRole("cell", { name: "actual.txt", exact: true }).waitFor();
 assert.equal(await page.getByLabel("Directory path", { exact: true }).inputValue(), "/assets");
 await writeFile(join(root, "assets", "external.txt"), "outside UI");
 await page.getByRole("button", { name: "Refresh directories", exact: true }).click();
 await page.getByRole("cell", { name: "external.txt", exact: true }).waitFor();
 await page.getByRole("button", { name: "Up", exact: true }).click();
 await page.getByRole("cell", { name: "alias", exact: true }).waitFor();
 assert.equal(await page.getByRole("button", { name: "alias/" }).count(), 0);
 const go = async (path) => { await page.getByLabel("Directory path", { exact: true }).fill(path); await page.getByRole("button", { name: "Browse", exact: true }).click(); };
 await go("/many");
 await page.getByRole("cell", { name: "item-000", exact: true }).waitFor();
 await page.getByRole("button", { name: "Next page", exact: true }).click();
 await page.getByRole("cell", { name: "item-256", exact: true }).waitFor();
 await page.getByRole("button", { name: "Previous page", exact: true }).click();
 await page.getByRole("cell", { name: "item-000", exact: true }).waitFor();
 await go("/denied"); await page.getByRole("alert").filter({ hasText: "does not permit" }).waitFor();
 await go("/missing"); await page.getByRole("alert").filter({ hasText: "no longer exists" }).waitFor();
 await go("/alias"); await page.getByRole("alert").filter({ hasText: "not a real directory" }).waitFor();
 await page.getByRole("button", { name: "Show upload directory", exact: true }).click();
 await page.getByText("This directory is empty.", { exact: true }).waitFor();
 assert.equal(await page.getByLabel("Directory path", { exact: true }).inputValue(), "/uploads");
 await page.getByRole("button", { name: "/", exact: true }).click();
 await page.getByRole("button", { name: "assets/", exact: true }).waitFor();
 await page.setViewportSize({ width: 390, height: 844 });
 await page.screenshot({ path: "/root/issue38/filesystem-mobile.png", fullPage: true });
 assert(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth));
 deny = true;
 await page.getByRole("button", { name: "Refresh directories", exact: true }).click();
 await page.getByText("Sign in", { exact: true }).first().waitFor();
 assert.deepEqual(errors, []);
 console.log("PASS: real fixture tree, directory/path/up/breadcrumb navigation, external-file refresh, paging, symlink metadata, permission/missing errors, actual upload-directory shortcut, mobile layout and sign-out.");
} finally { await browser.close(); server.close(); await rm(root, { recursive: true, force: true }); }
