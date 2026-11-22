import assert from "node:assert/strict";
import { createServer } from "node:http";
import { readFile } from "node:fs/promises";
import { resolve, extname } from "node:path";
import { chromium } from "playwright";
const words=JSON.parse(await readFile(resolve(import.meta.dirname,"avatar-languages.json"),"utf8"));
const dist=resolve(import.meta.dirname,"../../pkg/runtime/webui/dist");
let value="",lang="en"; const writes=[],errors=[];
const field=()=>({id:"avatar_path",name:"path",title:words[lang].path,group:words[lang].title,help:words[lang].help,reset_label:words[lang].default,kind:"text",value});
const server=createServer(async(req,res)=>{
 const path=new URL(req.url,"http://localhost").pathname;
 if(path.startsWith("/api/v1/")){
  let data={};
  if(path==="/api/v1/name")data={name:"Synthetic device"};
  else if(path==="/api/v1/device")data={name:"Synthetic device",pages:["files"],screen:false,wifi:false};
  else if(path==="/api/v1/settings")data=[{domain:"avatar",section_key:"settings.display",section:"Display",fields:[field()]}];
  else if(path==="/api/v1/entities")data=[];
  else if(path==="/api/v1/events"){res.setHeader("Content-Type","text/event-stream");res.end('data: {"type":"states","states":[]}\n\n');return;}
  else if(path==="/api/v1/settings/avatar/path"){
   if(req.method==="PUT"){const chunks=[];for await(const b of req)chunks.push(b);value=JSON.parse(Buffer.concat(chunks).toString()).value;writes.push(value);}
   data=field();
  }
  res.setHeader("Content-Type","application/json");res.end(JSON.stringify(data));return;
 }
 try{const data=await readFile(resolve(dist,"."+(path==="/"?"/index.html":path)));res.setHeader("Content-Type",extname(path)===".js"?"text/javascript":extname(path)===".css"?"text/css":"text/html");res.end(data);}catch{res.writeHead(404);res.end();}
});
await new Promise(r=>server.listen(0,"127.0.0.1",r));
const browser=await chromium.launch({headless:true,args:["--no-sandbox"]});
try{
 for(lang of ["en","de","it","vi"]){
  const page=await browser.newPage({viewport:{width:1280,height:800}});page.on("pageerror",e=>errors.push(e.message));
  await page.goto("http://127.0.0.1:"+server.address().port+"/#display");
  const input=page.locator("#f-avatar_path input");await input.waitFor();
  assert(await page.getByText(words[lang].help,{exact:true}).isVisible());
  await input.fill("/data/local/tmp/custom.vrm");await input.press("Enter");
  await page.waitForFunction(()=>document.querySelector("#f-avatar_path input")?.value==="/data/local/tmp/custom.vrm");
  await page.reload();await input.waitFor();assert.equal(await input.inputValue(),"/data/local/tmp/custom.vrm");
  for(const width of [1280,640,390]){await page.setViewportSize({width,height:844});await page.waitForTimeout(80);assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),"horizontal overflow "+lang+" "+width);assert(await page.getByRole("button",{name:words[lang].default,exact:true}).isVisible());}
  await page.getByRole("button",{name:words[lang].default,exact:true}).click();
  await page.waitForFunction(()=>document.querySelector("#f-avatar_path input")?.value==="");
  assert.equal(value,"");await page.close();
 }
 assert.deepEqual(writes,Array(4).fill(["/data/local/tmp/custom.vrm",""]).flat());assert.deepEqual(errors,[]);
 console.log("PASS: custom VRM setting, API writes, reload, explicit reset and 4-language layouts at 1280/640/390 pixels.");
}finally{await browser.close();server.close();}
