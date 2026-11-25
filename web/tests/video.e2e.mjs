import assert from "node:assert/strict";
import { createServer } from "node:http";
import { readFile } from "node:fs/promises";
import { resolve, extname } from "node:path";
import { chromium } from "playwright";
const dist=resolve(import.meta.dirname,"../../pkg/runtime/webui/dist");
const server=createServer(async(req,res)=>{try{const p=new URL(req.url,"http://localhost").pathname;const f=await readFile(resolve(dist,"."+ (p==="/" ? "/index.html":p)));res.setHeader("Content-Type",extname(p)===".js"?"text/javascript":extname(p)===".css"?"text/css":"text/html");res.end(f)}catch{res.writeHead(404);res.end()}});
await new Promise(r=>server.listen(0,"127.0.0.1",r));
const browser=await chromium.launch({headless:true,args:["--no-sandbox"]});
try {
 const page=await browser.newPage({viewport:{width:1100,height:950}});
 let state={state:"idle",position:0,duration:30,width:640,height:360,frames:0,seekable:false};
 let writes=[];let errors=[];
 page.on("pageerror",e=>errors.push(e.message));
 await page.route("**/api/v1/**",async route=>{
  const request=route.request(),path=new URL(request.url()).pathname;
  let data={};
  if(path.endsWith("/device"))data={name:"Video test",pages:["video"],screen:true,wifi:false};
  else if(path.endsWith("/settings")||path.endsWith("/entities"))data=[];
  else if(path.endsWith("/smb/"))data={enabled:false,host:"",port:445,share:"",username:"",domain:"",password_set:false,labels:{}};
else if(path.endsWith("/video/")){
   if(request.method()==="POST"){
    const cmd=request.postDataJSON();writes.push(cmd);
    if(cmd.action==="load")state={...state,state:"playing",seekable:true};
    if(cmd.action==="pause")state.state="paused";
    if(cmd.action==="play")state.state="playing";
    if(cmd.action==="seek")state.position=cmd.position;
    if(cmd.action==="stop")state={...state,state:"idle",seekable:false};
    await route.fulfill({status:204});return;
   } data=state;
  }
  await route.fulfill({json:data});
 });
 await page.goto("http://127.0.0.1:"+server.address().port+"/#video");
 await page.getByRole("heading",{name:"Video",exact:true}).waitFor();
 await page.getByLabel("Video or stream URL").fill("https://example.com/test.mp4");
 await page.getByRole("button",{name:"Play on device",exact:true}).click();
 await page.getByRole("button",{name:"Pause",exact:true}).click();
 await page.getByRole("button",{name:"Resume",exact:true}).waitFor();
 await page.getByRole("button",{name:"Resume",exact:true}).click();
 await page.getByLabel("Jump to (seconds)").fill("12");
 await page.getByRole("button",{name:"Seek",exact:true}).click();
 await page.getByRole("button",{name:"Stop",exact:true}).click();
 assert.deepEqual(writes.map(x=>x.action),["load","pause","play","seek","stop"]);
 assert.equal(writes[0].url,"https://example.com/test.mp4");assert.equal(writes[3].position,12);
 for(const url of ["https://www.youtube.com/watch?v=M7lc1UVf-VE","https://youtu.be/M7lc1UVf-VE","https://music.youtube.com/watch?v=M7lc1UVf-VE","https://music.youtube.com/podcast/cJjMmDeOAZ4","https://youtube.com/live/M7lc1UVf-VE","https://youtu.be/M7lc1UVf-VE#t=30"]){
  await page.getByLabel("Video or stream URL").fill(url);
  await page.getByRole("button",{name:"Play on device",exact:true}).click();
  assert.equal(writes.at(-1).url,url);
  await page.getByRole("button",{name:"Stop",exact:true}).click();
 }
 state={...state,state:"error",error:"The owner does not allow this video in an embedded player"};
 await page.getByText(state.error,{exact:true}).waitFor();
 await page.getByRole("button",{name:"Stop",exact:true}).click();

 await page.setViewportSize({width:390,height:844});
await page.evaluate(()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r))));
 await page.waitForFunction(()=>document.documentElement.scrollWidth<=innerWidth);
assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true);
 await page.screenshot({path:resolve(import.meta.dirname,"../../video-mobile.png")});
 assert.deepEqual(errors,[]);
 console.log("PASS: video navigation, direct/YouTube/Music URL load, pause/resume, seek, stop, actionable errors, mobile layout");
} finally {await browser.close();await new Promise(r=>server.close(r))}
