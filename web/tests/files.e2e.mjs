// Synthetic browser coverage; no device credentials or private addresses.
import assert from "node:assert/strict";
import { createServer } from "node:http";
import { readFile } from "node:fs/promises";
import { resolve, extname } from "node:path";
import { chromium } from "playwright";
const dist=resolve(import.meta.dirname,"../../pkg/runtime/webui/dist");
const files=new Map(),errors=[];let failure="",deny=false,slow=false;
const server=createServer(async(req,res)=>{
 const path=new URL(req.url,"http://localhost").pathname;
 if(path.startsWith("/api/v1/")){
  const at=path.slice(7),method=req.method;
  let data={},status=200;
  if(at==="/device")data={name:"Synthetic device",pages:["files"],screen:false,wifi:false};
  else if(at==="/settings"||at==="/entities")data=[];
  else if(at==="/events"){res.setHeader("Content-Type","text/event-stream");res.end('data: {"type":"states","states":[]}\n\n');return;}
  else if(deny){status=401;}
  else if(at==="/filesystem") { const dir=new URL(req.url,"http://localhost").searchParams.get("path"); data={path:dir,parent:dir==="/"?"":"/",entries:[],availableBytes:536870912,maxUploadBytes:134217728,readOnly:false,capacityKnown:true}; }
  else if(at==="/files")data={directory:"/data/misc/echolocal/user-files",files:[...files].map(([name,b])=>({name,size:b.length,modified:"2026-01-01T00:00:00Z"})),usedBytes:[...files.values()].reduce((n,b)=>n+b.length,0),availableBytes:536870912,budgetBytes:536870912,maxFileBytes:134217728,maxFiles:256};
  else if(at.startsWith("/files/")){
   const name=decodeURIComponent(at.slice(7));
   if(method==="PUT"){
    if(slow){setTimeout(()=>{res.writeHead(408);res.end();},1500);req.resume();return;}
    if(failure){status=507;data={error:failure};req.resume();}
    else if(files.has(name)){status=409;data={error:"already_exists"};req.resume();}
    else{const chunks=[];for await(const b of req)chunks.push(b);const b=Buffer.concat(chunks);files.set(name,b);status=201;data={name,size:b.length,modified:"2026-01-01T00:00:00Z"};}
   }else if(method==="DELETE"){files.delete(name);status=204;}
   else{const b=files.get(name);res.writeHead(200,{"Content-Type":"application/octet-stream","Content-Disposition":'attachment; filename="'+name+'"',"Content-Length":b.length});res.end(b);return;}
  }
  res.writeHead(status,{"Content-Type":"application/json"});res.end(status===204?undefined:JSON.stringify(data));return;
 }
 try{const data=await readFile(resolve(dist,"."+(path==="/"?"/index.html":path)));res.setHeader("Content-Type",extname(path)===".js"?"text/javascript":extname(path)===".css"?"text/css":"text/html");res.end(data);}catch{res.writeHead(404);res.end();}
});
await new Promise(r=>server.listen(0,"127.0.0.1",r));
const browser=await chromium.launch({headless:true,args:["--no-sandbox"]});
try {
 const page=await browser.newPage({viewport:{width:1200,height:950},acceptDownloads:true});
 page.on("pageerror",e=>errors.push(e.message));
 const origin="http://127.0.0.1:"+server.address().port;
 await page.goto(origin+"/#files");await page.getByRole("tab",{name:"Managed assets",exact:true}).click();await page.getByText("No files yet.").waitFor();
 const payload=Buffer.from([0,255,1,2,3,10,0,42]);
 await page.getByLabel("Choose file",{exact:true}).setInputFiles({name:"asset.txt",mimeType:"application/octet-stream",buffer:payload});
 await page.getByRole("button",{name:"Upload",exact:true}).click();
 await page.getByRole("cell",{name:"asset.txt",exact:true}).waitFor();assert.deepEqual(files.get("asset.txt"),payload);
 assert.match(await page.getByRole("status").innerText(),/Upload complete/);
 const dlEvent=page.waitForEvent("download");await page.getByRole("link",{name:"Download",exact:true}).click();const dl=await dlEvent;assert.deepEqual(await readFile(await dl.path()),payload);
 await page.reload();await page.getByRole("tab",{name:"Managed assets",exact:true}).click();await page.getByRole("cell",{name:"asset.txt",exact:true}).waitFor();
 await page.getByLabel("Choose file",{exact:true}).setInputFiles({name:"asset.txt",mimeType:"application/octet-stream",buffer:Buffer.from("replacement")});
 await page.getByRole("button",{name:"Upload",exact:true}).click();await page.getByRole("alert").filter({hasText:"already exists"}).waitFor();assert.deepEqual(files.get("asset.txt"),payload);
 failure="storage_full";
 await page.getByLabel("Choose file",{exact:true}).setInputFiles({name:"new.bin",mimeType:"application/octet-stream",buffer:payload});
 await page.getByRole("button",{name:"Upload",exact:true}).click();await page.getByRole("alert").filter({hasText:"Not enough capacity"}).waitFor();assert(!files.has("new.bin"));failure="";
 slow=true;await page.getByRole("button",{name:"Upload",exact:true}).click();await page.getByRole("button",{name:"Cancel upload"}).click();await page.getByRole("status").filter({hasText:"cancelled"}).waitFor();assert(!files.has("new.bin"));slow=false;
 await page.setViewportSize({width:390,height:844});await page.screenshot({path:"/root/issue38/files-mobile.png",fullPage:true});
 assert(await page.evaluate(()=>document.documentElement.scrollWidth<=window.innerWidth));
 page.once("dialog",d=>d.accept());await page.getByRole("button",{name:"Delete",exact:true}).click();await page.getByText("No files yet.").waitFor();assert.equal(files.size,0);
 deny=true;await page.getByRole("button",{name:"Refresh",exact:true}).click();await page.getByText("Sign in",{exact:true}).first().waitFor();
 assert.deepEqual(errors,[]);console.log("PASS: Files browser upload, identical download, refresh persistence, duplicate protection, quota error, cancellation, mobile layout, deletion, signed-out flow; no page errors.");
} finally {await browser.close();server.close();}
