import { useEffect, useState } from "react";
import { Alert, Button, Code, Group, ScrollArea, Stack, Table, Text, TextInput } from "@mantine/core";
import { SignedOut } from "./api";
import { bytes } from "./fileSize";

type Entry = { name: string; kind: "directory" | "file" | "symlink" | "special"; size: number; modified: string; mode: string };
type Listing = { path: string; parent: string; entries: Entry[]; nextOffset?: number; limitReached?: boolean };
const errors: Record<string, string> = {
 permission_denied: "The device does not permit listing this directory.",
 not_found: "This directory no longer exists.",
 not_directory: "This path is not a real directory. Symbolic links are listed but not followed.",
 invalid_path: "Enter an absolute directory path without dot segments or control characters.",
 invalid_offset: "This directory exceeds the browser's paging limit. Open a more specific path.",
};
export function FileSystemBrowser({ onSignedOut, uploadsPath }: { onSignedOut: (e: unknown) => void; uploadsPath?: string }) {
 const [location, setLocation] = useState({ path: "/", offset: 0 });
 const [draft, setDraft] = useState("/");
 const [listing, setListing] = useState<Listing>();
 const [error, setError] = useState("");
 const [loading, setLoading] = useState(false);
 const [refresh, setRefresh] = useState(0);
 useEffect(() => {
  const controller = new AbortController();
  setLoading(true); setError(""); setListing(undefined);
  fetch("/api/v1/filesystem?" + new URLSearchParams({ path: location.path, offset: String(location.offset) }), { credentials: "same-origin", signal: controller.signal })
   .then(async (r) => {
    if (r.status === 401) throw new SignedOut();
    if (!r.ok) { let code = ""; try { code = (await r.json()).error; } catch {} throw new Error(errors[code] || "Could not list this directory. Refresh and retry."); }
    return r.json() as Promise<Listing>;
   }).then((v) => { if (!controller.signal.aborted) { setListing(v); setDraft(v.path); } })
   .catch((e: unknown) => { if (!controller.signal.aborted) { if (e instanceof SignedOut) onSignedOut(e); else setError(e instanceof Error ? e.message : "Directory listing failed."); } })
   .finally(() => { if (!controller.signal.aborted) setLoading(false); });
  return () => controller.abort();
 }, [location, refresh, onSignedOut]);
 const go = (path: string) => { setDraft(path); setLocation({ path, offset: 0 }); };
 const current = listing?.path ?? location.path;
 const segments = current.split("/").filter(Boolean);
 return <Stack gap="md">
  <Text>Browse the device's real filesystem from /. This view reads directory metadata only; it never opens file contents or changes system files. Symbolic links and special files cannot be opened.</Text>
  <form onSubmit={(e) => { e.preventDefault(); go(draft); }}>
   <Group align="end">
    <TextInput label="Directory path" value={draft} onChange={(e) => setDraft(e.currentTarget.value)} style={{ flex: 1, minWidth: 180 }} />
    <Button type="submit">Browse</Button>
   </Group>
  </form>
  <Group gap="xs" aria-label="Directory breadcrumbs">
   <Button size="compact-sm" variant="subtle" onClick={() => go("/")}>/</Button>
   {segments.map((part, i) => <Button key={i} size="compact-sm" variant="subtle" onClick={() => go("/" + segments.slice(0, i + 1).join("/"))}>{part}</Button>)}
  </Group>
  <Group>
   <Button variant="default" disabled={current === "/"} onClick={() => go(current.substring(0, current.lastIndexOf("/")) || "/")}>Up</Button>
   <Button variant="default" onClick={() => setRefresh((v) => v + 1)}>Refresh directories</Button>
   {uploadsPath && <Button variant="light" onClick={() => go(uploadsPath)}>Show upload directory</Button>}
  </Group>
  <Text>Actual path: <Code style={{ overflowWrap: "anywhere" }}>{current}</Code></Text>
  {error && <Alert color="red" role="alert">{error}</Alert>}
  {loading && <Text role="status">Reading directory…</Text>}
  {listing && <>
   <ScrollArea><Table miw={650}>
    <Table.Thead><Table.Tr><Table.Th>Name</Table.Th><Table.Th>Type</Table.Th><Table.Th>Size</Table.Th><Table.Th>Modified</Table.Th><Table.Th>Permissions</Table.Th></Table.Tr></Table.Thead>
    <Table.Tbody>{listing.entries.map((entry) => <Table.Tr key={entry.name}>
     <Table.Td style={{ overflowWrap: "anywhere" }}>{entry.kind === "directory" ?
      <Button variant="subtle" size="compact-sm" onClick={() => go((listing.path === "/" ? "" : listing.path) + "/" + entry.name)}>{entry.name}/</Button> : entry.name}</Table.Td>
     <Table.Td>{entry.kind}</Table.Td><Table.Td>{entry.kind === "file" ? bytes(entry.size) : "—"}</Table.Td>
     <Table.Td>{new Date(entry.modified).toLocaleString()}</Table.Td><Table.Td><Code>{entry.mode}</Code></Table.Td>
    </Table.Tr>)}</Table.Tbody>
   </Table></ScrollArea>
   {listing.entries.length === 0 && <Text c="dimmed">This directory is empty.</Text>}
   <Group>
    {location.offset > 0 && <Button variant="default" onClick={() => setLocation({ path: listing.path, offset: Math.max(0, location.offset - 256) })}>Previous page</Button>}
    {listing.nextOffset !== undefined && <Button variant="default" onClick={() => setLocation({ path: listing.path, offset: listing.nextOffset! })}>Next page</Button>}
   </Group>
   {listing.limitReached && <Alert color="yellow">The 65,536-entry browsing limit has been reached. Enter a more specific directory path.</Alert>}
   <Text size="sm" c="dimmed">Up to 256 entries per page, sorted within each page. Listings are live snapshots; refresh after external changes.</Text>
  </>}
 </Stack>;
}
