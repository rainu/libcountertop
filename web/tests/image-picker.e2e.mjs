import assert from "node:assert/strict";
import {createServer} from "node:http";
import {readFile} from "node:fs/promises";
import {resolve,extname} from "node:path";
import {chromium} from "playwright";
const words=JSON.parse(await readFile(resolve(import.meta.dirname,"image-picker-languages.json"),"utf8"));
const dist=resolve(import.meta.dirname,"../../pkg/runtime/webui/dist");
let lang="en",value="/images/old.png";const writes=[],reads=[],errors=[];
const field=()=>({id:"avatar_background",name:"background",title:"Background",group:"Background",kind:"text",value,reset_label:"Reset",file_picker:{extensions:[".png",".jpg",".jpeg"],start_dir:"/images",text:words[lang]}});
const server=createServer(async(req,res)=>{
 const u=new URL(req.url,"http://localhost"),p=u.pathname;
 const reply=(data,status=200)=>{res.writeHead(status,{"Content-Type":"application/json"});res.end(JSON.stringify(data));};
 if(p.startsWith("/api/v1/")){
  if(p==="/api/v1/name")return reply({name:"Synthetic device"});
  if(p==="/api/v1/device")return reply({name:"Synthetic device",pages:["files"],screen:false,wifi:false});
  if(p==="/api/v1/settings")return reply([{domain:"avatar",section_key:"settings.display",section:"Display",fields:[field()]}]);
  if(p==="/api/v1/entities")return reply([]);
  if(p==="/api/v1/events"){res.setHeader("Content-Type","text/event-stream");res.end('data: {"type":"states","states":[]}\n\n');return;}
  if(p==="/api/v1/filesystem"){
   const path=u.searchParams.get("path"),offset=Number(u.searchParams.get("offset")||0);reads.push([path,offset]);
   if(path==="/images/denied")return reply({error:"permission_denied"},403);
   if(path==="/images/sub")return reply({path,parent:"/images",entries:[{name:"nested.PNG",kind:"file",size:100}]});
   if(offset===256)return reply({path:"/images",parent:"/",entries:[{name:"page2.jpeg",kind:"file",size:100}]});
   return reply({path:"/images",parent:"/",nextOffset:256,entries:[
    {name:"sub",kind:"directory"},{name:"denied",kind:"directory"},
    {name:"valid.png",kind:"file",size:100},{name:"broken.jpg",kind:"file",size:100},
    {name:"notes.txt",kind:"file",size:100},{name:"pipe.png",kind:"special",size:0}]});
  }
  if(p==="/api/v1/settings/avatar/background"){
   if(req.method==="PUT"){
    const chunks=[];for await(const b of req)chunks.push(b);const next=JSON.parse(Buffer.concat(chunks).toString()).value;writes.push(next);
    if(next.endsWith("broken.jpg"))return reply({error:"Synthetic invalid image"},400);
    value=next;
   }
   return reply(field());
  }
  return reply({});
 }
 try{const b=await readFile(resolve(dist,"."+(p==="/"?"/index.html":p)));res.setHeader("Content-Type",extname(p)===".js"?"text/javascript":extname(p)===".css"?"text/css":"text/html");res.end(b);}catch{res.writeHead(404);res.end();}
});
await new Promise(r=>server.listen(0,"127.0.0.1",r));
const browser=await chromium.launch({headless:true,args:["--no-sandbox"]});
try{
 for(lang of ["en","de","it","vi"]){
  value="/images/old.png";
  const page=await browser.newPage({viewport:{width:390,height:844}});page.on("pageerror",e=>errors.push(e.message));
  await page.goto("http://127.0.0.1:"+server.address().port+"/#display");
  const input=page.locator("#f-avatar_background input");await input.waitFor();
  const n=writes.length;
  await input.fill("/unsaved.png");
  await page.getByRole("button",{name:words[lang].choose_image,exact:true}).click();
  const dialog=page.getByRole("dialog");
  await dialog.getByRole("button",{name:"valid.png",exact:true}).waitFor();
  assert.equal(await dialog.getByRole("button",{name:"notes.txt",exact:true}).count(),0);
  assert.equal(await dialog.getByRole("button",{name:"pipe.png",exact:true}).count(),0);
  assert.equal(writes.length,n,"opening committed unsaved text");
  await dialog.getByRole("button",{name:words[lang].picker_cancel,exact:true}).click();
  assert.equal(writes.length,n,"cancel wrote a setting");
  await page.reload();await input.waitFor();
  await page.getByRole("button",{name:words[lang].choose_image,exact:true}).click();
  await dialog.getByRole("button",{name:"sub/",exact:true}).click();
  await dialog.getByRole("button",{name:"nested.PNG",exact:true}).waitFor();
  await dialog.getByRole("button",{name:words[lang].picker_up,exact:true}).click();
  await dialog.getByRole("button",{name:"broken.jpg",exact:true}).click();
  await dialog.getByRole("alert").waitFor();
  assert.equal(value,"/images/old.png");assert(await dialog.isVisible(),"invalid selection closed picker");
  await dialog.getByRole("button",{name:"denied/",exact:true}).click();
  await dialog.getByRole("alert").waitFor();
  await dialog.getByRole("button",{name:words[lang].picker_up,exact:true}).click();
  await dialog.getByRole("button",{name:words[lang].picker_next,exact:true}).click();
  await dialog.getByRole("button",{name:"page2.jpeg",exact:true}).waitFor();
  await dialog.getByRole("button",{name:words[lang].picker_previous,exact:true}).click();
  await dialog.getByRole("button",{name:"valid.png",exact:true}).waitFor();
  for(const width of [390,640,1280]){
   await page.setViewportSize({width,height:844});
   assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),"horizontal overflow");
   assert(await dialog.getByRole("button",{name:words[lang].picker_cancel,exact:true}).isVisible());
  }
  await dialog.getByRole("button",{name:"valid.png",exact:true}).click();
  await page.waitForFunction(()=>document.querySelector("#f-avatar_background input")?.value==="/images/valid.png");
  assert.equal(await dialog.count(),0);assert.equal(value,"/images/valid.png");
  await page.reload();await input.waitFor();assert.equal(await input.inputValue(),"/images/valid.png");
  await page.close();
 }
 assert.deepEqual(errors,[]);
 assert.equal(writes.filter(x=>x==="/images/valid.png").length,4);
 assert(reads.some(([p,o])=>p==="/images"&&o===256));
 console.log("PASS image picker: 4 languages, 3 widths, folders/up, paging, image filter, cancel/no draft save, rejected image retains value, permission recovery, select/persist.");
}finally{await browser.close();server.close();}
