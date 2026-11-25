import { useEffect, useState } from "react";
import { Alert, Button, Checkbox, Group, NumberInput, Paper, PasswordInput, Stack, Text, TextInput, Title } from "@mantine/core";
import { api, type SMBSettings, type SMBView } from "./api";

const empty:SMBView={enabled:false,host:"",port:445,share:"",username:"",domain:"",password_set:false,labels:{}};
export function SMBSource(){
 const [value,setValue]=useState<SMBView>(empty);
 const [password,setPassword]=useState("");
 const [busy,setBusy]=useState(false);
 const [ready,setReady]=useState(false);
 const [notice,setNotice]=useState("");
 const [error,setError]=useState("");
 const [testResult,setTestResult]=useState<{ok:boolean;message:string}>();
 const [testing,setTesting]=useState(false);
 useEffect(()=>{let alive=true;api.smb().then(v=>{if(alive){setValue(v);setReady(true)}},()=>{if(alive)setError("Could not read SMB settings")});return()=>{alive=false}},[]);
 const t=(key:string)=>value.labels[key]||key;
 const change=<K extends keyof SMBSettings>(key:K,v:SMBSettings[K])=>{setTestResult(undefined);setNotice("");setValue(s=>({...s,[key]:v}));};
 const save=async(clear=false)=>{
  setBusy(true);setError("");setNotice("");setTestResult(undefined);
  const {enabled,host,port,share,username,domain}=value;
  try{
   await api.smbSave({enabled:clear?false:enabled,host,port,share,username,domain,...(clear?{clear_password:true}:password?{password}:{})});
   setPassword("");setValue(await api.smb());setNotice(t("saved"));
  }catch(e){setError(e instanceof Error?e.message:t("error"))}
  finally{setBusy(false)}
 };
 const test=async()=>{
  if(busy)return;
  setBusy(true);setTesting(true);setError("");setNotice("");setTestResult(undefined);
  const {enabled,host,port,share,username,domain}=value;
  try{setTestResult(await api.smbTest({enabled,host,port,share,username,domain,...(password?{password}:{})}));}
  catch(e){setError(e instanceof Error?e.message:t("error"))}
  finally{setBusy(false);setTesting(false)}
 };
 return <Paper withBorder p="md" data-testid="smb-source"><Stack>
  <Title order={3}>{ready?t("title"):"SMB"}</Title>
  {ready&&<><Text size="sm" c="dimmed">{t("help")}</Text>
  <form onSubmit={e=>{e.preventDefault();void save()}}><Stack>
   <Checkbox disabled={busy} label={t("enabled")} checked={value.enabled} onChange={e=>change("enabled",e.currentTarget.checked)} />
   <TextInput disabled={busy} label={t("host")} value={value.host} autoComplete="off" onChange={e=>change("host",e.currentTarget.value)} maxLength={255} required={value.enabled} />
   <NumberInput disabled={busy} label={t("port")} value={value.port} min={1} max={65535} onChange={v=>change("port",Number(v))} required />
   {(["share","username","domain"] as const).map(k=><TextInput disabled={busy} key={k} label={t(k)} value={value[k]} autoComplete="off" maxLength={255} required={value.enabled&&k!=="domain"} onChange={e=>change(k,e.currentTarget.value)} />)}
   <PasswordInput disabled={busy} label={t("password")} value={password} autoComplete="new-password" onChange={e=>{setTestResult(undefined);setNotice("");setPassword(e.currentTarget.value)}} maxLength={1024} />
   <Text size="sm">{t(value.password_set?"password_set":"password_unset")}</Text>
   <Group><Button style={{maxWidth:"100%",height:"auto",minHeight:36}} styles={{label:{whiteSpace:"normal",overflowWrap:"anywhere"}}} type="submit" loading={busy&&!testing} disabled={busy}>{t("save")}</Button><Button type="button" variant="default" loading={testing} disabled={busy} onClick={()=>void test()}>{t("test")}</Button><Button style={{maxWidth:"100%",height:"auto",minHeight:36}} styles={{label:{whiteSpace:"normal",overflowWrap:"anywhere"}}} variant="light" color="red" disabled={busy} onClick={()=>void save(true)}>{t("clear")}</Button></Group>
  </Stack></form></>}
  {error&&<Alert color="red" title={t("error")}>{error}</Alert>}
  {testResult&&<Alert color={testResult.ok?"green":"red"} role="status">{testResult.message}</Alert>}
  {notice&&<Text role="status">{notice}</Text>}
 </Stack></Paper>;
}
