import { useEffect, useState } from "react";
import { Alert, Button, Group, Modal, ScrollArea, Stack, Text, TextInput } from "@mantine/core";
import type { DeviceFilePickerConfig } from "./api";

type Entry = { name: string; kind: string; size: number; targetKind?: string };
type Listing = { path: string; parent: string; entries: Entry[]; nextOffset?: number; limitReached?: boolean };
type Props = { config: DeviceFilePickerConfig; onClose: () => void; onSelect: (path: string) => Promise<void> };

export function DeviceFilePicker({ config, onClose, onSelect }: Props) {
 const t = (key: string) => config.text[key] || key;
 const [location,setLocation] = useState({path:config.start_dir || "/",offset:0});
 const [draft,setDraft] = useState(location.path);
 const [listing,setListing] = useState<Listing>();
 const [loading,setLoading] = useState(true);
 const [busy,setBusy] = useState(false);
 const [error,setError] = useState("");
 const [refresh,setRefresh] = useState(0);
 useEffect(()=>{
  const controller=new AbortController();
  setLoading(true);setError("");
  fetch("/api/v1/filesystem?"+new URLSearchParams({path:location.path,offset:String(location.offset)}),{credentials:"same-origin",signal:controller.signal})
   .then(async r=>{
    if(!r.ok)throw new Error(t("picker_error"));
    return r.json() as Promise<Listing>;
   }).then(v=>{if(!controller.signal.aborted){setListing(v);setDraft(v.path);}})
   .catch((e:unknown)=>{if(!controller.signal.aborted)setError(e instanceof Error?e.message:t("picker_error"));})
   .finally(()=>{if(!controller.signal.aborted)setLoading(false);});
  return ()=>controller.abort();
 },[location,refresh,config]);
 const go=(path:string,offset=0)=>{if(busy)return;setListing(undefined);setLoading(true);setDraft(path);setError("");setLocation({path,offset});};
 const choose=async(path:string)=>{
  if(busy||loading)return;
  setBusy(true);setError("");
  try{await onSelect(path);onClose();}
  catch(e){setError(e instanceof Error?e.message:t("picker_error"));setBusy(false);}
 };
 const entries=(listing?.entries ?? []).filter(e=>{
  if(e.kind==="directory"||e.targetKind==="directory")return true;
  const ext=e.name.slice(e.name.lastIndexOf(".")).toLowerCase();
  return (e.kind==="file"||e.targetKind==="file")&&config.extensions.some(x=>x.toLowerCase()===ext);
 });
 return <Modal opened onClose={()=>{if(!busy)onClose();}} closeOnEscape={!busy} closeOnClickOutside={!busy} withCloseButton={false} title={t("picker_title")} size="lg">
  <Stack gap="sm">
   <Button variant="default" disabled={busy} onClick={onClose}>{t("picker_cancel")}</Button>
   <form onSubmit={e=>{e.preventDefault();go(draft);}}>
    <Group align="end" wrap="wrap"><TextInput label={t("picker_folder")} value={draft} disabled={busy} onChange={e=>setDraft(e.currentTarget.value)} style={{flex:"1 1 220px",minWidth:0}} />
     <Button type="submit" disabled={busy}>{t("picker_go")}</Button>
    </Group>
   </form>
   <Group wrap="wrap">
    <Button variant="light" disabled={busy||location.path==="/"} onClick={()=>go(listing?.parent || location.path.substring(0,location.path.lastIndexOf("/")) || "/")}>{t("picker_up")}</Button>
    <Button variant="light" disabled={busy} onClick={()=>{setListing(undefined);setLoading(true);setRefresh(n=>n+1);}}>{t("picker_refresh")}</Button>
   </Group>
   {error&&<Alert color="red" role="alert">{error}</Alert>}
   {loading&&<Text role="status">{t("picker_loading")}</Text>}
   <ScrollArea.Autosize mah="48vh">
    <Stack gap="xs">
     {!loading&&entries.map(e=>{
      const directory=e.kind==="directory"||e.targetKind==="directory";
      const name=(listing!.path==="/"?"/":listing!.path+"/")+e.name;
      return <Button key={e.name} fullWidth justify="flex-start" variant="subtle" disabled={busy} styles={{label:{whiteSpace:"normal",overflowWrap:"anywhere"}}} style={{height:"auto",minHeight:44}} onClick={()=>{if(directory)go(name);else void choose(name);}}>{e.name}{directory?"/":""}</Button>;
     })}
     {!loading&&listing&&entries.length===0&&<Text>{t("picker_empty")}</Text>}
    </Stack>
   </ScrollArea.Autosize>
   <Group wrap="wrap">
    {location.offset>0&&<Button disabled={busy||loading} onClick={()=>go(location.path,Math.max(0,location.offset-256))}>{t("picker_previous")}</Button>}
    {listing?.nextOffset!==undefined&&<Button disabled={busy||loading} onClick={()=>go(listing.path,listing.nextOffset!)}>{t("picker_next")}</Button>}
   </Group>
   {listing?.limitReached&&<Text size="sm">{t("picker_limit")}</Text>}
  </Stack>
 </Modal>;
}
