import { useCallback, useEffect, useRef, useState } from "react";
import { Alert, Button, Group, Paper, Progress, ScrollArea, Stack, Table, Tabs, Text, Title } from "@mantine/core";
import { FileSystemBrowser } from "./FileSystemBrowser";
import { SignedOut } from "./api";
import { bytes } from "./fileSize";

type FileInfo = { name: string; size: number; modified: string };
type View = { directory?: string; files: FileInfo[]; usedBytes: number; availableBytes: number; budgetBytes: number; maxFileBytes: number; maxFiles: number; entries?: number };
export { bytes } from "./fileSize";
const base = "/api/v1/files";
const messages: Record<string, string> = {
 incomplete_upload: "The transfer was incomplete. No partial file was published; retry the upload.",
 invalid_name: "Use valid UTF-8 names (up to 255 bytes per component). Traversal, control characters, backslashes and reserved internal names are not allowed.",
 already_exists: "That name already exists. Delete it explicitly or choose another name; files are never overwritten.",
 file_too_large: "The file exceeds the per-file limit.",
 storage_full: "Not enough capacity or too many files. Delete a file and retry.",
 transfer_busy: "Another managed-file write is in progress or a WebDAV lock protects this file. Retry after the write or lock ends.",
 unsafe_entry: "Storage contains an unsupported entry. Ask the owner to inspect it.",
 cross_origin: "Use the device's own HTTPS web interface.",
 length_required: "The upload needs a known content length.",
 not_found: "The file no longer exists.",
};
async function checked(r: Response) {
 if (r.status === 401) throw new SignedOut();
 if (!r.ok) {
  let code = ""; try { code = (await r.json()).error; } catch {}
  throw new Error(messages[code] || "Transfer failed. Check the connection, refresh the file list, and retry.");
 }
 return r;
}
export function Files({ onSignedOut }: { onSignedOut: (e: unknown) => void }) {
 const [view, setView] = useState<View>();
 const [selected, setSelected] = useState<File | null>(null);
 const [busy, setBusy] = useState(false);
 const [progress, setProgress] = useState(0);
 const [error, setError] = useState("");
 const [notice, setNotice] = useState("");
 const active = useRef<XMLHttpRequest | null>(null);
 const alive = useRef(true);
 const input = useRef<HTMLInputElement>(null);
 const failed = useCallback((e: unknown) => {
  if (!alive.current) return;
  if (e instanceof SignedOut) onSignedOut(e);
  else setError(e instanceof Error ? e.message : "Transfer failed.");
 }, [onSignedOut]);
 const load = useCallback(async () => {
  const r = await checked(await fetch(base, { credentials: "same-origin" }));
  const v = await r.json() as View;
  if (alive.current) setView(v);
 }, []);
 useEffect(() => {
  alive.current = true; load().catch(failed);
  return () => { alive.current = false; active.current?.abort(); };
 }, [load, failed]);
 const upload = () => {
  if (!selected || !view || busy) return;
  setError(""); setNotice("");
  if (selected.size > view.maxFileBytes) { setError(messages.file_too_large); return; }
  if (selected.size > view.availableBytes || (view.entries ?? view.files.length) >= view.maxFiles) { setError(messages.storage_full); return; }
  const xhr = new XMLHttpRequest();
  active.current = xhr;
  setBusy(true); setProgress(0);
  xhr.open("PUT", base + "/" + encodeURIComponent(selected.name));
  xhr.timeout = 305000;
  xhr.setRequestHeader("Content-Type", "application/octet-stream");
  xhr.upload.onprogress = (e) => { if (alive.current && e.lengthComputable) setProgress(Math.round(e.loaded / e.total * 100)); };
  xhr.onload = () => {
   if (!alive.current) return;
   if (xhr.status === 401) onSignedOut(new SignedOut());
   else if (xhr.status === 201) {
    setNotice("Upload complete."); setSelected(null);
    if (input.current) input.current.value = "";
   } else {
    let code = ""; try { code = JSON.parse(xhr.responseText).error; } catch {}
    setError(messages[code] || "Upload failed. Refresh and retry; no partial file is active.");
   }
  };
  xhr.onerror = () => { if (alive.current) setError("Connection lost. Refresh to check whether the upload completed, then retry."); };
  xhr.ontimeout = () => { if (alive.current) setError("Upload timed out. Refresh and retry."); };
  xhr.onabort = () => { if (alive.current) setNotice("Upload cancelled. Refreshing the file list."); };
  xhr.onloadend = () => {
   active.current = null;
   if (alive.current) { setBusy(false); load().catch(failed); }
  };
  // File/Blob is streamed by the browser, never converted to a JS byte array.
  xhr.send(selected);
 };
 const remove = async (name: string) => {
  if (!window.confirm(`Delete "${name}" permanently?`)) return;
  setBusy(true); setError(""); setNotice("");
  try {
   await checked(await fetch(base + "/" + encodeURIComponent(name), { method: "DELETE", credentials: "same-origin" }));
   setNotice("File deleted.");
  } catch (e) { failed(e); }
  finally { if (alive.current) setBusy(false); load().catch(failed); }
 };
 return <Stack gap="md">
  <Title order={2}>Files</Title>
  <Tabs defaultValue="filesystem">
   <Tabs.List><Tabs.Tab value="filesystem">Device filesystem</Tabs.Tab><Tabs.Tab value="uploads">Managed assets</Tabs.Tab></Tabs.List>
   <Tabs.Panel value="filesystem" pt="md"><FileSystemBrowser onSignedOut={onSignedOut} uploadsPath={view?.directory} /></Tabs.Panel>
   <Tabs.Panel value="uploads" pt="md"><Stack gap="md">
   {view?.directory && <Text size="sm">Actual upload directory: {view.directory}</Text>}
  <Text>Store and transfer local assets. Uploading does not display a picture, import an avatar, or install a tone. Only supported consumers can explicitly import files.</Text>
  {error && <Alert color="red" role="alert">{error}</Alert>}
  {notice && <Alert color="blue" role="status">{notice}</Alert>}
  {view && <Text>{bytes(view.usedBytes)} used of {bytes(view.budgetBytes)} · {bytes(view.availableBytes)} available · {view.entries ?? view.files.length}/{view.maxFiles} files and folders · {bytes(view.maxFileBytes)} per file</Text>}
  <Paper withBorder p="md"><Stack gap="sm">
   <input ref={input} type="file" aria-label="Choose file" disabled={busy} onChange={(e) => setSelected(e.target.files?.[0] ?? null)} />
   <Text size="sm" c="dimmed">Web uploads create new root-level files; existing names are not replaced. WebDAV clients can manage folders, rename, replace and lock files.</Text>
   {busy && active.current && <><Progress value={progress} aria-label="Upload progress" /><Text role="status">{progress === 100 ? "Finalizing upload…" : `Uploading: ${progress}%`}</Text></>}
   <Group><Button onClick={upload} disabled={!selected || !view || busy}>Upload</Button>
    {busy && active.current && <Button variant="default" onClick={() => active.current?.abort()}>Cancel upload</Button>}
    <Button variant="default" disabled={busy} onClick={() => { setError(""); load().catch(failed); }}>Refresh</Button>
   </Group>
  </Stack></Paper>
  <ScrollArea><Table miw={600}>
   <Table.Thead><Table.Tr><Table.Th>Name</Table.Th><Table.Th>Size</Table.Th><Table.Th>Modified</Table.Th><Table.Th>Actions</Table.Th></Table.Tr></Table.Thead>
   <Table.Tbody>{view?.files.map((f) => <Table.Tr key={f.name}>
    <Table.Td style={{ overflowWrap: "anywhere" }}>{f.name}</Table.Td><Table.Td>{bytes(f.size)}</Table.Td>
    <Table.Td>{new Date(f.modified).toLocaleString()}</Table.Td>
    <Table.Td><Group gap="xs" wrap="nowrap">
     <Button size="xs" variant="default" component="a" href={base + "/" + encodeURIComponent(f.name)} download={f.name}>Download</Button>
     <Button size="xs" color="red" variant="light" disabled={busy} onClick={() => remove(f.name)}>Delete</Button>
    </Group></Table.Td>
   </Table.Tr>)}</Table.Tbody>
  </Table></ScrollArea>
  {view?.files.length === 0 && <Text c="dimmed">No files yet.</Text>}
   </Stack></Tabs.Panel>
  </Tabs>
 </Stack>;
}
