import { useEffect, useRef, useState } from "react";
import { Alert, Button, Code, Group, Paper, Progress, ScrollArea, Stack, Table, Text, TextInput } from "@mantine/core";
import { SignedOut } from "./api";
import { bytes } from "./fileSize";

type Entry = { name: string; kind: "directory" | "file" | "symlink" | "special"; size: number; modified: string; mode: string; targetKind?: string; targetPath?: string };
type Listing = { path: string; parent: string; entries: Entry[]; nextOffset?: number; limitReached?: boolean; availableBytes: number; maxUploadBytes: number; readOnly: boolean; capacityKnown: boolean };
const errors: Record<string, string> = {
 permission_denied: "The device does not permit this operation at the selected path.",
 read_only_filesystem: "The destination filesystem is read-only. Choose a writable directory; no remount is performed.",
 not_found: "This file or directory no longer exists.",
 not_directory: "The path is not a directory, or changed while it was being opened. Refresh and retry.",
 unsupported_file: "Only regular files can be downloaded. Devices, pipes and sockets are not supported.",
 invalid_path: "Enter an absolute path without dot segments or control characters.",
 invalid_offset: "This directory exceeds the browser's paging limit. Open a more specific path.",
 already_exists: "That destination already exists. Choose another filename; existing files are never overwritten.",
 file_too_large: "The file exceeds the upload limit.",
 storage_full: "Not enough free space while preserving the device's free-space reserve.",
 incomplete_upload: "The upload was incomplete. No destination file was published.",
 transfer_busy: "Another filesystem upload is in progress. Retry when it finishes.",
 cross_origin: "Use the device's own HTTPS interface.",
 atomic_publication_unsupported: "This filesystem cannot publish a complete file without replacement. Choose another destination.",
 length_required: "The upload needs a known content length.",
};
const fileURL = (path: string) => "/api/v1/filesystem/file?" + new URLSearchParams({ path });
export function FileSystemBrowser({ onSignedOut, uploadsPath }: { onSignedOut: (e: unknown) => void; uploadsPath?: string }) {
 const [location, setLocation] = useState({ path: "/", offset: 0 });
 const [draft, setDraft] = useState("/");
 const [destination, setDestination] = useState("/");
 const [filename, setFilename] = useState("");
 const [selected, setSelected] = useState<File | null>(null);
 const [listing, setListing] = useState<Listing>();
 const [error, setError] = useState("");
 const [notice, setNotice] = useState("");
 const [loading, setLoading] = useState(false);
 const [busy, setBusy] = useState(false);
 const [progress, setProgress] = useState(0);
 const [refresh, setRefresh] = useState(0);
 const active = useRef<XMLHttpRequest | null>(null);
 const input = useRef<HTMLInputElement>(null);
 const alive = useRef(true);
 useEffect(() => { alive.current = true; return () => { alive.current = false; active.current?.abort(); }; }, []);
 useEffect(() => {
  const controller = new AbortController();
  setLoading(true); setError(""); setListing(undefined);
  fetch("/api/v1/filesystem?" + new URLSearchParams({ path: location.path, offset: String(location.offset) }), { credentials: "same-origin", signal: controller.signal })
   .then(async (r) => {
    if (r.status === 401) throw new SignedOut();
    if (!r.ok) { let code = ""; try { code = (await r.json()).error; } catch {} throw new Error(errors[code] || "Could not list this directory. Refresh and retry."); }
    return r.json() as Promise<Listing>;
   }).then((v) => { if (!controller.signal.aborted) { setListing(v); setDraft(v.path); setDestination(v.path); } })
   .catch((e: unknown) => { if (!controller.signal.aborted) { if (e instanceof SignedOut) onSignedOut(e); else setError(e instanceof Error ? e.message : "Directory listing failed."); } })
   .finally(() => { if (!controller.signal.aborted) setLoading(false); });
  return () => controller.abort();
 }, [location, refresh, onSignedOut]);
 const go = (path: string) => { if (busy) return; setDraft(path); setLocation({ path, offset: 0 }); setNotice(""); };
 const current = listing?.path ?? location.path;
 const segments = current.split("/").filter(Boolean);
 const upload = () => {
  if (!selected || !listing || busy) return;
  setError(""); setNotice("");
  if (!destination.startsWith("/") || !filename || filename === "." || filename === ".." || /[/\\\x00-\x1f\x7f]/.test(filename)) { setError(errors.invalid_path); return; }
  if (selected.size > listing.maxUploadBytes) { setError(errors.file_too_large); return; }
  if (destination === listing.path && listing.readOnly) { setError(errors.read_only_filesystem); return; }
  if (destination === listing.path && listing.capacityKnown && selected.size > listing.availableBytes) { setError(errors.storage_full); return; }
  const targetDir = destination.replace(/\/+$/, "") || "/";
  const target = (targetDir === "/" ? "" : targetDir) + "/" + filename;
  const xhr = new XMLHttpRequest();
  active.current = xhr; setBusy(true); setProgress(0);
  xhr.open("PUT", fileURL(target)); xhr.timeout = 305000;
  xhr.setRequestHeader("Content-Type", "application/octet-stream");
  xhr.upload.onprogress = (e) => { if (alive.current && e.lengthComputable) setProgress(Math.round(e.loaded / e.total * 100)); };
  xhr.onload = () => {
   if (!alive.current) return;
   if (xhr.status === 401) onSignedOut(new SignedOut());
   else if (xhr.status === 201) {
    setNotice("Uploaded to " + target); setSelected(null); setFilename("");
    if (input.current) input.current.value = "";
    setLocation({ path: targetDir, offset: 0 });
   } else {
    let code = ""; try { code = JSON.parse(xhr.responseText).error; } catch {}
    setError(errors[code] || "Upload failed. Refresh to check the destination before retrying.");
   }
  };
  xhr.onerror = () => { if (alive.current) setError("Connection lost. Refresh to check whether the upload completed."); };
  xhr.ontimeout = () => { if (alive.current) setError("Upload timed out. Refresh the destination before retrying."); };
  xhr.onabort = () => { if (alive.current) setNotice("Upload cancelled. Refresh to check the destination."); };
  xhr.onloadend = () => { active.current = null; if (alive.current) setBusy(false); };
  xhr.send(selected);
 };
 return <Stack gap="md">
  <Text>Browse the real device filesystem. Download any regular file the service can read, including system files. Upload into the chosen directory when it is writable. Normal symlinks resolve to their actual targets.</Text>
  <form onSubmit={(e) => { e.preventDefault(); go(draft); }}>
   <Group align="end">
    <TextInput label="Directory path" value={draft} disabled={busy} onChange={(e) => setDraft(e.currentTarget.value)} style={{ flex: 1, minWidth: 180 }} />
    <Button type="submit" disabled={busy}>Browse</Button>
   </Group>
  </form>
  <Group gap="xs" aria-label="Directory breadcrumbs">
   <Button size="compact-sm" variant="subtle" disabled={busy} onClick={() => go("/")}>/</Button>
   {segments.map((part, i) => <Button key={i} size="compact-sm" variant="subtle" disabled={busy} onClick={() => go("/" + segments.slice(0, i + 1).join("/"))}>{part}</Button>)}
  </Group>
  <Group>
   <Button variant="default" disabled={busy || current === "/"} onClick={() => go(current.substring(0, current.lastIndexOf("/")) || "/")}>Up</Button>
   <Button variant="default" disabled={busy} onClick={() => setRefresh((v) => v + 1)}>Refresh directories</Button>
   {uploadsPath && <Button variant="light" disabled={busy} onClick={() => go(uploadsPath)}>Show upload directory</Button>}
  </Group>
  <Text>Actual path: <Code style={{ overflowWrap: "anywhere" }}>{current}</Code></Text>
  {error && <Alert color="red" role="alert">{error}</Alert>}
  {notice && <Alert color="blue" role="status">{notice}</Alert>}
  {loading && <Text role="status">Reading directory…</Text>}
  {listing && <>
   <Paper withBorder p="md"><Stack gap="sm">
    <Text fw={600}>Upload to a directory</Text>
    <TextInput label="Upload destination directory" value={destination} disabled={busy} onChange={(e) => setDestination(e.currentTarget.value)} />
    <input ref={input} type="file" aria-label="Choose filesystem upload" disabled={busy} onChange={(e) => { const f = e.target.files?.[0] ?? null; setSelected(f); setFilename(f?.name ?? ""); }} />
    <TextInput label="Destination filename" value={filename} disabled={busy} onChange={(e) => setFilename(e.currentTarget.value)} />
    <Text size="sm" c="dimmed">Create-only; existing files are never overwritten. Up to {bytes(listing.maxUploadBytes)} per upload. System partitions are not remounted. Downloads have no upload-size limit.</Text>
    {listing.capacityKnown && <Text size="sm">{listing.readOnly ? "Current filesystem: read-only." : bytes(listing.availableBytes) + " available for uploads here after the free-space reserve."}</Text>}
    {busy && <><Progress value={progress} aria-label="Filesystem upload progress" /><Text role="status">{progress === 100 ? "Finalizing upload…" : "Uploading: " + progress + "%"}</Text></>}
    <Group><Button disabled={!selected || !filename || busy} onClick={upload}>Upload to destination</Button>
     {busy && <Button variant="default" onClick={() => active.current?.abort()}>Cancel filesystem upload</Button>}
    </Group>
   </Stack></Paper>
   <ScrollArea><Table miw={750}>
    <Table.Thead><Table.Tr><Table.Th>Name</Table.Th><Table.Th>Type</Table.Th><Table.Th>Size</Table.Th><Table.Th>Modified</Table.Th><Table.Th>Permissions</Table.Th><Table.Th>Actions</Table.Th></Table.Tr></Table.Thead>
    <Table.Tbody>{listing.entries.map((entry) => {
     const directory = entry.kind === "directory" || entry.targetKind === "directory";
     const regular = entry.kind === "file" || entry.targetKind === "file";
     const entryPath = (listing.path === "/" ? "" : listing.path) + "/" + entry.name;
     return <Table.Tr key={entry.name}>
      <Table.Td style={{ overflowWrap: "anywhere" }}>{directory ? <Button variant="subtle" size="compact-sm" disabled={busy} onClick={() => go(entry.targetPath ?? entryPath)}>{entry.name}/</Button> : entry.name}
       {entry.targetPath && <Text size="xs" c="dimmed" style={{ overflowWrap: "anywhere" }}>→ {entry.targetPath}</Text>}</Table.Td>
      <Table.Td>{entry.kind}</Table.Td><Table.Td>{entry.kind === "file" ? bytes(entry.size) : "—"}</Table.Td>
      <Table.Td>{new Date(entry.modified).toLocaleString()}</Table.Td><Table.Td><Code>{entry.mode}</Code></Table.Td>
      <Table.Td>{regular && <Button size="xs" variant="default" component="a" href={fileURL(entryPath)} download={entry.name}>Download</Button>}</Table.Td>
     </Table.Tr>;
    })}</Table.Tbody>
   </Table></ScrollArea>
   {listing.entries.length === 0 && <Text c="dimmed">This directory is empty.</Text>}
   <Group>
    {location.offset > 0 && <Button variant="default" disabled={busy} onClick={() => setLocation({ path: listing.path, offset: Math.max(0, location.offset - 256) })}>Previous page</Button>}
    {listing.nextOffset !== undefined && <Button variant="default" disabled={busy} onClick={() => setLocation({ path: listing.path, offset: listing.nextOffset! })}>Next page</Button>}
   </Group>
   {listing.limitReached && <Alert color="yellow">The 65,536-entry browsing limit has been reached. Enter a more specific directory path.</Alert>}
   <Text size="sm" c="dimmed">Up to 256 entries per page, sorted within each page. Refresh after external changes.</Text>
  </>}
 </Stack>;
}
