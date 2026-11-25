import assert from "node:assert/strict";
import {createServer} from "node:http";
import {readFile} from "node:fs/promises";
import {resolve,extname} from "node:path";
import {chromium} from "playwright";
const labels=JSON.parse(await readFile(new URL("./smb-labels.json",import.meta.url),"utf8"));
const dist=resolve(import.meta.dirname,"../../pkg/runtime/webui/dist");
const server=createServer(async(req,res)=>{try{const p=new URL(req.url,"http://localhost").pathname;const f=await readFile(resolve(dist,"."+(p==="/"?"/index.html":p)));res.setHeader("Content-Type",extname(p)===".js"?"text/javascript":extname(p)===".css"?"text/css":"text/html");res.end(f)}catch{res.writeHead(404);res.end()}});
await new Promise(r=>server.listen(0,"127.0.0.1",r));
const browser=await chromium.launch({headless:true,args:["--no-sandbox"]});
try{
 for(const lang of ["en","de","it","vi"]){
  const page=await browser.newPage({viewport:{width:1100,height:1000}});
  const errors=[];page.on("pageerror",e=>errors.push(e.message));
  let value={enabled:false,host:"",port:445,share:"",username:"",domain:"",password_set:false,labels:labels[lang]};
  const writes=[];
  await page.route("**/api/v1/**",async route=>{
   const req=route.request(),p=new URL(req.url()).pathname;
   let data={};
   if(p.endsWith("/device"))data={name:"SMB fixture",pages:["video","smb"],screen:true,wifi:false};
   else if(p.endsWith("/settings")||p.endsWith("/entities"))data=[];
   else if(p.endsWith("/smb/")){
    if(req.method()==="PUT"){
     const v=req.postDataJSON();writes.push(v);
     value={...value,...v,password_set:v.clear_password?false:v.password?true:value.password_set};
     delete value.password;delete value.clear_password;
     await route.fulfill({status:204});return;
    }data=value;
   } else if(p.endsWith("/video/")){
    if(req.method()==="POST"){writes.push(req.postDataJSON());await route.fulfill({status:204});return;}
    data={state:"idle",position:0,duration:0,width:0,height:0,frames:0,seekable:false};
   }
   await route.fulfill({json:data});
  });
  await page.goto("http://127.0.0.1:"+server.address().port+"/#video");
  const box=page.getByTestId("smb-source"),l=labels[lang];
  await box.getByLabel(l.host,{exact:true}).fill("server.example");
  await box.getByLabel(l.share,{exact:true}).fill("videos");
  await box.getByLabel(l.username,{exact:true}).fill("reader");
  await box.getByLabel(l.password,{exact:true}).fill("synthetic-secret");
  await box.getByLabel(l.enabled,{exact:true}).check();
  await box.getByRole("button",{name:l.save,exact:true}).click();
  await box.getByText(l.saved,{exact:true}).waitFor();
  assert.equal(writes.at(-1).password,"synthetic-secret");
  assert.equal(await box.getByLabel(l.password,{exact:true}).inputValue(),"");
  await box.getByRole("button",{name:l.save,exact:true}).click();
  await page.waitForTimeout(150);
  assert.equal("password" in writes.at(-1),false);
  await page.reload();
  await box.getByText(l.password_set,{exact:true}).waitFor();
  assert.equal(await box.getByLabel(l.password,{exact:true}).inputValue(),"");
  await page.getByLabel("Video or stream URL").fill("smb://media/folder/video.mp4");
  await page.getByRole("button",{name:"Play on device",exact:true}).click();
  assert.equal(writes.at(-1).url,"smb://media/folder/video.mp4");
  await box.getByRole("button",{name:l.clear,exact:true}).click();
  await box.getByText(l.password_unset,{exact:true}).waitFor();
  assert.equal(writes.at(-1).clear_password,true);
  assert.equal(writes.at(-1).enabled,false);
  await page.setViewportSize({width:390,height:844});
await page.evaluate(()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r))));
  await page.waitForFunction(()=>document.documentElement.scrollWidth<=innerWidth);
assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true);
  assert.deepEqual(errors,[]);
  await page.close();
 }
 console.log("PASS: SMB source four languages; keep/replace/clear password, refresh, alias playback and mobile layout");
}finally{await browser.close();await new Promise(r=>server.close(r))}
