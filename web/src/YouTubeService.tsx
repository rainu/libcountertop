import { useEffect, useState } from "react";
import { Alert, Badge, Button, Code, Group, Modal, Paper, Stack, Text, Title } from "@mantine/core";
import { YouTubeActivationQR } from "./YouTubeActivationQR";
import { api, SignedOut, type YouTubeConnection } from "./api";

export function YouTubeService({ onSignedOut }: { onSignedOut: (error: unknown) => void }) {
 const [state,setState]=useState<YouTubeConnection>({state:"checking",premium:false});
 const [busy,setBusy]=useState(false);
 const [error,setError]=useState("");
 const [confirm,setConfirm]=useState(false);
 useEffect(()=>{
  let stopped=false;
  let timer: ReturnType<typeof setTimeout>;
  const refresh=async()=>{
   try {const s=await api.youtube();if(!stopped){setState(s);setError("");}}
   catch(e){if(!stopped){if(e instanceof SignedOut)onSignedOut(e);else setError("Could not read the YouTube connection.");}}
   finally{if(!stopped)timer=setTimeout(()=>void refresh(),2000);}
  };
  void refresh();
  return ()=>{stopped=true;clearTimeout(timer);};
 },[onSignedOut]);
 const command=async(action:"connect"|"refresh"|"cancel"|"disconnect")=>{
  setBusy(true);setError("");
  try {await api.youtubeCommand(action);setState({state:"checking",premium:false});}
  catch(e){if(e instanceof SignedOut)onSignedOut(e);else setError(e instanceof Error?e.message:"YouTube connection failed.");}
  finally {setBusy(false);}
 };
 const labels:Record<string,string>={checking:"Checking connection…",connected:"Connected",disconnected:"Not connected",linking:"Waiting for Google confirmation",expired:"Activation code expired — try again",error:"Connection failed"};
 return <Paper withBorder p="md" data-testid="youtube-service"><Stack>
  <Group><Title order={4}>YouTube</Title>{state.state==="connected"&&<Badge color="green">Connected</Badge>}{state.premium&&<Badge>Premium</Badge>}</Group>
  <Text role="status">{labels[state.state]||"Connection unavailable"}</Text>
  <Text size="sm" c="dimmed">Connect this device to your YouTube account. The player uses this connection for eligible YouTube Premium benefits.</Text>
  {state.code&&state.state==="linking"&&<Stack gap="xs">
   <Group wrap="nowrap" align="center" gap="md"><Text style={{flex:1,minWidth:0,overflowWrap:"anywhere"}}><a href="https://yt.be/activate" target="_blank" rel="noopener noreferrer" referrerPolicy="no-referrer">yt.be/activate</a></Text><YouTubeActivationQR /></Group>
   <Text>Scan the QR code or open the website on your phone or computer, enter this code and confirm with Google:</Text>
   <Code block style={{fontSize:"1.6rem",letterSpacing:2}}>{state.code}</Code>
   {state.expires_at&&<Text size="sm">Use before {new Date(state.expires_at).toLocaleTimeString()}. If Google rejects the code, try again.</Text>}
   <Text size="sm" c="dimmed">Never enter your Google password here. Cancel closes this local sign-in flow; it cannot undo a confirmation already completed at Google.</Text>
  </Stack>}
  {(error||state.error)&&<Alert color="red">{error||state.error}</Alert>}
  <Group>
   {state.state!=="connected"&&<Button disabled={busy||state.state==="checking"} onClick={()=>void command("connect")}>{state.state==="linking"||state.state==="expired"||state.state==="error"?"Try again":"Sign in with Google"}</Button>}
   <Button variant="default" disabled={busy} onClick={()=>void command("refresh")}>Check connection</Button>
   {state.state==="linking"&&<Button variant="default" disabled={busy} onClick={()=>void command("cancel")}>Cancel sign-in</Button>}
   {(state.state==="connected"||state.state==="error")&&<Button color="red" variant="light" disabled={busy} onClick={()=>setConfirm(true)}>Sign out on this device</Button>}
  </Group>
  <Modal opened={confirm} onClose={()=>setConfirm(false)} title="Sign out of YouTube">
   <Stack><Text>Remove the local YouTube session and consent data on this device? Playback stops. Your Google account and subscription are unchanged. This does not revoke the device’s authorization at Google.</Text>
    <Group><Button variant="default" onClick={()=>setConfirm(false)}>Cancel</Button><Button color="red" disabled={busy} onClick={()=>{setConfirm(false);void command("disconnect");}}>Sign out</Button></Group>
   </Stack>
  </Modal>
 </Stack></Paper>;
}
