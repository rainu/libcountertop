// Synthetic UI tests backed by a real temporary filesystem; no device secrets.
import assert from "node:assert/strict";
import { createServer } from "node:http";
import { mkdtemp, mkdir, writeFile, symlink, readdir, lstat, realpath, readFile, rm } from "node:fs/promises";
import { resolve, extname, join, relative } from "node:path";
import { tmpdir } from "node:os";
import { chromium } from "playwright";
const root = await mkdtemp(join(tmpdir(), "devicefs-ui-"));
for (const name of ["assets", "dest", "system", "many"]) await mkdir(join(root, name));
const systemBytes = Buffer.from([0, 255, 1, 42, 10]);
await writeFile(join(root, "system", "version.bin"), systemBytes);
await writeFile(join(root, "assets", "actual.txt"), Buffer.alloc(5120,42));
await symlink("assets", join(root, "alias"));
for (let i = 0; i < 270; i++) await writeFile(join(root, "many", "item-" + String(i).padStart(3, "0")), "");
const dist = resolve(import.meta.dirname, "../../pkg/runtime/webui/dist");
const errors = [];
let deny = false, failUpload = false, slow = false;
const server = createServer(async (req, res) => {
 const url = new URL(req.url, "http://localhost"), route = url.pathname;
 const send = (status, data) => { res.writeHead(status, { "Content-Type": "application/json" }); res.end(JSON.stringify(data)); };
 if (route.startsWith("/api/v1/")) {
  if (route === "/api/v1/device") return send(200, { name: "Synthetic device", pages: ["files", "filesystem"], screen: false, wifi: false });
  if (route === "/api/v1/settings" || route === "/api/v1/entities") return send(200, []);
  if (route === "/api/v1/events") { res.setHeader("Content-Type", "text/event-stream"); return res.end('data: {"type":"states","states":[]}\n\n'); }
  if (deny) return send(401, {});
  if (route === "/api/v1/files") return send(200, { directory: "/assets", files: [], usedBytes: 0, availableBytes: 536870912, budgetBytes: 536870912, maxFileBytes: 134217728, maxFiles: 256 });
  if (route.startsWith("/api/v1/filesystem")) {
   const path = url.searchParams.get("path") || "/", offset = Number(url.searchParams.get("offset") || 0);
   if (path.includes("..")) return send(400, { error: "invalid_path" });
   if (path === "/denied") return send(403, { error: "permission_denied" });
   if (route === "/api/v1/filesystem/file") {
    if (req.method === "PUT") {
     if (path.startsWith("/system/")) { req.resume(); return send(403, { error: "read_only_filesystem" }); }
     if (failUpload) { req.resume(); return send(507, { error: "storage_full" }); }
     if (slow) { req.resume(); setTimeout(() => send(408, {}), 1000); return; }
     const chunks = []; for await (const chunk of req) chunks.push(chunk);
     try { await writeFile(join(root, path), Buffer.concat(chunks), { flag: "wx" }); return send(201, { name: path.split("/").at(-1) }); }
     catch (e) { return send(e.code === "EEXIST" ? 409 : 500, { error: e.code === "EEXIST" ? "already_exists" : "failed" }); }
    }
    try { const b = await readFile(join(root, path)); res.writeHead(200, { "Content-Type": "application/octet-stream", "Content-Disposition": 'attachment; filename="version.bin"', "Content-Length": b.length }); return res.end(b); }
    catch { return send(404, { error: "not_found" }); }
   }
   try {
    const absolute = await realpath(join(root, path));
    if (!(await lstat(absolute)).isDirectory()) return send(409, { error: "not_directory" });
    const canonical = "/" + relative(root, absolute);
    const names = (await readdir(absolute)).sort(), allEntries = [];
    for (const name of names) {
     const info = await lstat(join(absolute, name));
     let target = {};
     if (info.isSymbolicLink()) {
      const actual = await realpath(join(absolute, name)), targetInfo = await lstat(actual);
      target = { targetKind: targetInfo.isDirectory() ? "directory" : "file", targetPath: "/" + relative(root, actual) };
     }
     allEntries.push({ name, kind: info.isSymbolicLink() ? "symlink" : info.isDirectory() ? "directory" : "file", size: info.size, modified: info.mtime.toISOString(), mode: info.isDirectory() ? "drwx------" : "-rw-------", ...target });
    }
    allEntries.sort((a,b) => { const af=a.kind==="directory"||a.targetKind==="directory",bf=b.kind==="directory"||b.targetKind==="directory"; if(af!==bf)return af?-1:1; const ak=a.name.toLowerCase(),bk=b.name.toLowerCase(); return ak<bk?-1:ak>bk?1:a.name<b.name?-1:a.name>b.name?1:0; });
    const entries=allEntries.slice(offset,offset+256);
    return send(200, { path: canonical, parent: canonical === "/" ? "" : "/", entries, availableBytes: 536870912, maxUploadBytes: 134217728, capacityKnown: true, readOnly: canonical === "/system", ...(names.length > offset + 256 ? { nextOffset: offset + 256 } : {}) });
   } catch { return send(404, { error: "not_found" }); }
  }
  return send(404, {});
 }
 try { const data = await readFile(resolve(dist, "." + (route === "/" ? "/index.html" : route))); res.setHeader("Content-Type", extname(route) === ".js" ? "text/javascript" : extname(route) === ".css" ? "text/css" : "text/html"); res.end(data); } catch { res.writeHead(404); res.end(); }
});
await new Promise((r) => server.listen(0, "127.0.0.1", r));
const browser = await chromium.launch({ headless: true, args: ["--no-sandbox"] });
try {
 const page = await browser.newPage({ viewport: { width: 1200, height: 950 }, acceptDownloads: true });
 page.on("pageerror", (e) => errors.push(e.message));
 await page.goto("http://127.0.0.1:" + server.address().port + "/#files");
 const go = async (path) => { await page.getByLabel("Directory path", { exact: true }).fill(path); await page.getByRole("button", { name: "Browse", exact: true }).click(); };
 await page.getByRole("button", { name: "alias/", exact: true }).click();
 await page.getByRole("cell", { name: "actual.txt", exact: true }).waitFor();
 assert.equal(await page.getByLabel("Directory path", { exact: true }).inputValue(), "/assets");
 await page.getByRole("row").filter({hasText:"actual.txt"}).getByRole("cell",{name:"5.0 KiB",exact:true}).waitFor();
 await writeFile(join(root, "assets", "external.txt"), "outside UI");
 await page.getByRole("button", { name: "Refresh directories", exact: true }).click();
 await page.getByRole("cell", { name: "external.txt", exact: true }).waitFor();
 await page.getByRole("button", { name: "Up", exact: true }).click();
 await page.getByRole("button", { name: "system/", exact: true }).click();
 await page.getByRole("cell", { name: "version.bin", exact: true }).waitFor();
 const dlEvent = page.waitForEvent("download");
 await page.getByRole("link", { name: "Download", exact: true }).click();
 assert.deepEqual(await readFile(await (await dlEvent).path()), systemBytes);
 const payload = Buffer.from([0, 254, 255, 42, 10, 0]);
 const choose = async (name) => page.getByLabel("Choose filesystem upload", { exact: true }).setInputFiles({ name, mimeType: "application/octet-stream", buffer: payload });
 await choose("blocked.bin");
 await page.getByRole("button", { name: "Upload to destination", exact: true }).click();
 await page.getByRole("alert").filter({ hasText: "read-only" }).waitFor();
 // Explicitly change the upload destination without navigating first.
 await page.getByLabel("Upload destination directory", { exact: true }).fill("/dest");
 await page.getByLabel("Destination filename", { exact: true }).fill("ä sample.bin");
 await page.getByRole("button", { name: "Upload to destination", exact: true }).click();
 await page.getByRole("cell", { name: "ä sample.bin", exact: true }).waitFor();
 assert.deepEqual(await readFile(join(root, "dest", "ä sample.bin")), payload);
 assert.equal(await page.getByLabel("Directory path", { exact: true }).inputValue(), "/dest");
 await choose("ä sample.bin");
 await page.getByRole("button", { name: "Upload to destination", exact: true }).click();
 await page.getByRole("alert").filter({ hasText: "already exists" }).waitFor();
 assert.deepEqual(await readFile(join(root, "dest", "ä sample.bin")), payload);
 failUpload = true;
 await choose("no-space.bin");
 await page.getByRole("button", { name: "Upload to destination", exact: true }).click();
 await page.getByRole("alert").filter({ hasText: "Not enough free space" }).waitFor();
 failUpload = false; slow = true;
 await page.getByRole("button", { name: "Upload to destination", exact: true }).click();
 await page.getByRole("button", { name: "Cancel filesystem upload", exact: true }).click();
 await page.getByRole("status").filter({ hasText: "cancelled" }).waitFor(); slow = false;
 await assert.rejects(readFile(join(root, "dest", "no-space.bin")));
 await go("/many");
 await page.getByRole("cell", { name: "item-000", exact: true }).waitFor();
 await page.getByRole("button", { name: "Next page", exact: true }).click();
 await page.getByRole("cell", { name: "item-256", exact: true }).waitFor();
 await page.getByRole("button", { name: "Previous page", exact: true }).click();
 await page.getByRole("cell", { name: "item-000", exact: true }).waitFor();
 await go("/denied"); await page.getByRole("alert").filter({ hasText: "does not permit" }).waitFor();
 await go("/missing"); await page.getByRole("alert").filter({ hasText: "no longer exists" }).waitFor();
 await page.getByRole("button", { name: "Show upload directory", exact: true }).click();
 await page.getByRole("cell", { name: "actual.txt", exact: true }).waitFor();
 await page.getByRole("button", { name: "/", exact: true }).click();
 await page.getByRole("button", { name: "assets/", exact: true }).waitFor();
 // Navigation must preserve the actual native file selection, not only state.
 assert.equal(await page.getByLabel("Choose filesystem upload", { exact: true }).evaluate((input) => input.files?.[0]?.name), "no-space.bin");
 await page.setViewportSize({ width: 390, height: 844 });
 await page.screenshot({ path: resolve(import.meta.dirname,"../../filesystem-mobile.png"), fullPage: true });
 assert(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth));
 deny = true;
 await page.getByRole("button", { name: "Refresh directories", exact: true }).click();
 await page.getByText("Sign in", { exact: true }).first().waitFor();
 assert.deepEqual(errors, []);
 console.log("PASS: real directories/symlinks/paging, external refresh, system-file byte download, explicit upload directory and filename, physical byte parity, no overwrite, read-only/full errors, cancellation, mobile layout and sign-out.");
} finally { await browser.close(); server.close(); await rm(root, { recursive: true, force: true }); }
