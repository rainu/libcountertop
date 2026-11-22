// Synthetic settings-renderer coverage; not Windows/device interoperability.
import assert from "node:assert/strict";
import { createServer } from "node:http";
import { readFile } from "node:fs/promises";
import { resolve, extname } from "node:path";
import { chromium } from "playwright";
const dist=resolve(import.meta.dirname,"../../pkg/runtime/webui/dist");
let enabled=false,deny=false;const writes=[],errors=[];
const field=()=>({id:"frontend_webdav",name:"webdav",title:"Device files via WebDAV (HTTPS)",group:"Web access",kind:"toggle",value:enabled?"on":"off"});
const server=createServer(async(req,res)=>{
 const path=new URL(req.url,"http://localhost").pathname;
 if(path.startsWith("/api/v1/")){
  let status=200,data={};
  if(path==="/api/v1/name")data={name:"Synthetic device"};
  else if(path==="/api/v1/device")data={name:"Synthetic device",pages:["files","webdav"],screen:false,wifi:false};
  else if(path==="/api/v1/settings")data=[{domain:"access",section_key:"settings.network",section:"Network",fields:[field(),{id:"frontend_password",name:"password",title:"Admin password",group:"Web access",kind:"secret",value:"set"}]}];
  else if(path==="/api/v1/entities")data=[];
  else if(path==="/api/v1/events"){res.setHeader("Content-Type","text/event-stream");res.end('data: {"type":"states","states":[]}\n\n');return;}
  else if(path==="/api/v1/settings/access/webdav"&&req.method==="GET")data=field();
  else if(path==="/api/v1/settings/access/webdav"&&req.method==="PUT"){
   const chunks=[];for await(const b of req)chunks.push(b);
   const body=JSON.parse(Buffer.concat(chunks).toString());writes.push(body.value);
   if(deny){status=422;data={error:"change rejected"};}
   else{enabled=body.value==="on";data=field();}
  }
  res.writeHead(status,{"Content-Type":"application/json"});res.end(JSON.stringify(data));return;
 }
 try{const data=await readFile(resolve(dist,"."+(path==="/"?"/index.html":path)));res.setHeader("Content-Type",extname(path)===".js"?"text/javascript":extname(path)===".css"?"text/css":"text/html");res.end(data);}catch{res.writeHead(404);res.end();}
});
await new Promise(r=>server.listen(0,"127.0.0.1",r));
const browser=await chromium.launch({headless:true,args:["--no-sandbox"]});
try{
 const page=await browser.newPage({viewport:{width:1200,height:900}});
 page.on("pageerror",e=>errors.push(e.message));
 const url="http://127.0.0.1:"+server.address().port+"/#network";
 await page.goto(url);
 const toggle=page.locator("#f-frontend_webdav input[type=checkbox]");
 await toggle.waitFor({state:"attached"});
 assert(!(await toggle.isChecked()));
 assert.equal(await page.getByTestId("webdav-connection").count(),0);
 await page.locator("#f-frontend_webdav").locator("label").first().click();
 await page.waitForFunction(()=>document.querySelector("#f-frontend_webdav input[type=checkbox]")?.checked===true);
 assert(enabled);assert.deepEqual(writes,["on"]);
 const expectedURL="http://127.0.0.1:"+server.address().port+"/api/v1/webdav/";
 assert.equal(await page.getByLabel("WebDAV URL (HTTPS)").inputValue(),expectedURL);
 await page.context().grantPermissions(["clipboard-read","clipboard-write"]);
 await page.getByRole("button",{name:"Copy WebDAV URL"}).click();
 assert.equal(await page.evaluate(()=>navigator.clipboard.readText()),expectedURL);
 await page.reload();await toggle.waitFor({state:"attached"});assert(await toggle.isChecked());
 await page.setViewportSize({width:390,height:844});
 await page.waitForFunction(()=>document.documentElement.scrollWidth<=window.innerWidth);
 assert(await page.evaluate(()=>document.documentElement.scrollWidth<=window.innerWidth));
 await page.locator("#f-frontend_webdav").locator("label").first().click();
 await page.waitForFunction(()=>document.querySelector("#f-frontend_webdav input[type=checkbox]")?.checked===false);
 assert(!enabled);assert.deepEqual(writes,["on","off"]);
 assert.equal(await page.getByTestId("webdav-connection").count(),0);
 assert.equal(await page.locator("#f-frontend_password input").inputValue(),"");
 assert.deepEqual(errors,[]);
 console.log("PASS: WebDAV settings toggle, exact owner API writes, refresh, mobile layout and empty secret editor.");
}finally{await browser.close();server.close();}
