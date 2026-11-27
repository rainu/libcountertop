// Synthetic UI/API contract test. No room audio, device or provider access.
import assert from "node:assert/strict";
import { createServer } from "node:http";
import { readFile } from "node:fs/promises";
import { resolve, extname } from "node:path";
import { chromium } from "playwright";
const dist=resolve(import.meta.dirname,"../../pkg/runtime/webui/dist");
const actions=[],errors=[];let phase="idle",seconds=0,owned=false,problem="",settingsValue="10";
const field=()=>({id:"microphone_gain",name:"gain",title:"Microphone gain",group:"Microphone",kind:"number",value:settingsValue,min:0,max:59,unit:"dB"});
const server=createServer(async(req,res)=>{
 const path=new URL(req.url,"http://localhost").pathname;
 if(path.startsWith("/api/v1/")){
  let status=200,data={};
  if(path==="/api/v1/name")data={name:"Synthetic device"};
  else if(path==="/api/v1/device")data={name:"Synthetic device",pages:["microphone-test"],screen:false,wifi:false};
  else if(path==="/api/v1/settings")data=[{domain:"microphone",section_key:"settings.volume",section:"Sound",fields:[field()]},{domain:"other",section_key:"settings.system",section:"System",fields:[{id:"x",name:"x",title:"Other setting",group:"Other",kind:"toggle",value:"on"}]}];
  else if(path==="/api/v1/entities")data=[];
  else if(path==="/api/v1/events"){res.setHeader("Content-Type","text/event-stream");res.end('data: {"type":"states","states":[]}\n\n');return;}
  else if(path==="/api/v1/settings/microphone/gain"){
   if(req.method==="PUT"){let body="";for await(const b of req)body+=b;settingsValue=JSON.parse(body).value;}
   data=field();
  }
  else if(path==="/api/v1/microphone-test/"){
   assert.match(req.headers["x-microphone-test"],/^[a-f0-9]{32}$/);
   if(req.method==="POST"){
    let body="";for await(const b of req)body+=b;const a=JSON.parse(body).action;actions.push(a);
    if(a==="record"){if(problem){status=409;res.writeHead(status,{"Content-Type":"text/plain"});res.end(problem);return;}phase="recording";owned=true;seconds=0;}
    if(a==="stop"){phase="ready";seconds=2;}
    if(a==="play")phase="playing";
    if(a==="discard"){phase="idle";owned=false;seconds=0;}
   }
   data={phase,seconds,owned,processing:"synthetic processed take"};
  }
  res.writeHead(status,{"Content-Type":"application/json"});res.end(JSON.stringify(data));return;
 }
 try{const data=await readFile(resolve(dist,"."+(path==="/"?"/index.html":path)));res.setHeader("Content-Type",extname(path)===".js"?"text/javascript":extname(path)===".css"?"text/css":"text/html");res.end(data);}catch{res.writeHead(404);res.end();}
});
await new Promise(r=>server.listen(0,"127.0.0.1",r));
const browser=await chromium.launch({headless:true,args:["--no-sandbox"]});
try{
 const page=await browser.newPage({viewport:{width:1280,height:800}});page.on("pageerror",e=>errors.push(e.message));
 await page.goto("http://127.0.0.1:"+server.address().port+"/#volume");
 const card=page.getByTestId("microphone-test");await card.waitFor();
 const record=card.getByRole("button",{name:"Record / replace take (8 s)"});const stop=card.getByRole("button",{name:"Stop",exact:true});const play=card.getByRole("button",{name:"Play on Echo"});
 assert(await play.isDisabled());
 await record.click();await page.getByRole("status").filter({hasText:"RECORDING"}).waitFor();assert(await record.isDisabled());assert(await play.isDisabled());
 await stop.click();await page.getByRole("status").filter({hasText:"Ready to play"}).waitFor();assert(await play.isEnabled());
 await play.click();await page.getByRole("status").filter({hasText:"Playing on Echo"}).waitFor();assert(await record.isDisabled());
 await stop.click();await page.getByRole("status").filter({hasText:"Ready to play"}).waitFor();
 const gain=page.locator("#f-microphone_gain").getByRole("slider");await gain.focus();for(let i=0;i<4;i++) await gain.press("ArrowRight");await gain.press("Tab");await page.waitForTimeout(400);assert.equal(settingsValue,"14");
 await record.click();await page.getByRole("status").filter({hasText:"RECORDING"}).waitFor();
 await card.getByRole("button",{name:"Delete take"}).click();await page.getByRole("status").filter({hasText:"Ready to record"}).waitFor();
 problem="muted";await record.click();await card.getByRole("alert").filter({hasText:"muted"}).waitFor();problem="";
 await record.click();await page.getByRole("status").filter({hasText:"RECORDING"}).waitFor();
 const before=actions.filter(a=>a==="discard").length;await page.evaluate(()=>{window.location.hash="system"});await card.waitFor({state:"detached"});await page.waitForTimeout(400);assert.equal(phase,"idle");assert.equal(actions.filter(a=>a==="discard").length,before+1);
 await page.evaluate(()=>{window.location.hash="volume"});await card.waitFor();
 await page.setViewportSize({width:390,height:844});await page.waitForFunction(()=>document.documentElement.scrollWidth<=window.innerWidth);assert(await record.isEnabled());
 assert.deepEqual(errors,[]);
 console.log("PASS: discoverability, record/stop/play/delete, settings/repeat, mute error, page cleanup and mobile layout.");
}finally{await browser.close();server.close();}
