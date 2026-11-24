import { useEffect, useState } from "react";
import { Alert, Button, Group, NumberInput, Paper, Stack, Text, TextInput, Title } from "@mantine/core";
import { api, type VideoState } from "./api";

export function Video() {
 const [url, setURL] = useState("");
 const [state,setState]=useState<VideoState>({state:"idle",position:0,duration:0,width:0,height:0,frames:0,seekable:false});
 const [position,setPosition]=useState<number|string>(0);
 const [busy,setBusy]=useState(false);
 const [error,setError]=useState("");
 useEffect(()=>{
  let stopped=false;
  const refresh=()=>api.video().then(s=>{if(!stopped)setState(s)},()=>{if(!stopped)setError("Could not read playback status")});
  refresh(); const timer=window.setInterval(refresh,1000);
  return ()=>{stopped=true;window.clearInterval(timer)};
 },[]);
 const command=async(action:"load"|"play"|"pause"|"stop"|"seek")=>{
  setBusy(true);setError("");
  try {await api.videoCommand({action,...(action==="load"?{url}:{}),...(action==="seek"?{position:Number(position)}:{})});setState(await api.video())}
  catch(e){setError(e instanceof Error?e.message:"Playback failed")}
  finally{setBusy(false)}
 };
 const active=state.state!=="idle"&&state.state!=="error";
 return <Stack maw={760}>
  <Title order={2}>Video</Title>
  <Text c="dimmed">Play a direct video, stream, or public YouTube / YouTube Music video on this device. No Chromecast connection is needed.</Text>
  <Paper withBorder p="md"><Stack>
   <form onSubmit={e=>{e.preventDefault();void command("load")}}>
    <Stack>
     <TextInput label="Video or stream URL" placeholder="https://example.com/video.mp4" value={url} onChange={e=>setURL(e.currentTarget.value)} required type="url" autoComplete="off" />
     <Button type="submit" loading={busy} disabled={!url.trim()}>Play on device</Button>
    </Stack>
   </form>
   <Text size="sm" c="dimmed">Paste a YouTube or YouTube Music link to a video, song, podcast episode, Short or live stream, or a direct MP4 (H.264/AAC). YouTube uses its official player: sign-in, restricted videos and playlists are not supported; ads may play. The device must be able to reach the URL.</Text>
  </Stack></Paper>
  {(error||state.error)&&<Alert color="red" title="Playback error">{error||state.error}</Alert>}
  <Paper withBorder p="md"><Stack>
   <Text role="status">Status: {state.state} · {state.position.toFixed(1)} / {state.duration.toFixed(1)} s</Text>
   {state.width>0&&<Text size="sm">{state.width} × {state.height} · {state.frames} video frames</Text>}
   <Group>
    <Button disabled={busy||!active||state.state==="loading"} onClick={()=>void command(state.state==="paused"?"play":"pause")}>{state.state==="paused"?"Resume":"Pause"}</Button>
    <Button variant="default" disabled={busy||state.state==="idle"} onClick={()=>void command("stop")}>Stop</Button>
   </Group>
   <Group align="end">
    <NumberInput label="Jump to (seconds)" min={0} max={state.duration||undefined} value={position} onChange={setPosition} disabled={!state.seekable} />
    <Button variant="default" disabled={busy||!active||!state.seekable||position===""} onClick={()=>void command("seek")}>Seek</Button>
   </Group>
  </Stack></Paper>
 </Stack>;
}
