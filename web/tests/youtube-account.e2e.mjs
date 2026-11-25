import assert from "node:assert/strict";
import {createServer} from "node:http";
import {readFile} from "node:fs/promises";
import {resolve,extname} from "node:path";
import {chromium} from "playwright";
const root=resolve(import.meta.dirname,"../.."),dist=resolve(root,"pkg/runtime/webui/dist");
const server=createServer(async(req,res)=>{try{const p=new URL(req.url,"http://localhost").pathname;const f=await readFile(resolve(dist,"."+(p==="/" ? "/index.html":p)));res.setHeader("Content-Type",extname(p)===".js"?"text/javascript":extname(p)===".css"?"text/css":"text/html");res.end(f)}catch{res.writeHead(404);res.end()}});
await new Promise(r=>server.listen(0,"127.0.0.1",r));
const browser=await chromium.launch({headless:true,args:["--no-sandbox","--autoplay-policy=no-user-gesture-required"]});
try {
 const page=await browser.newPage({viewport:{width:1100,height:950}});
 let state={state:"disconnected",premium:false};const writes=[];
 await page.route("**/api/v1/**",async route=>{
  const req=route.request(),p=new URL(req.url()).pathname;let data={};
  if(p.endsWith("/device"))data={name:"Test",pages:["video","youtube"],screen:true,wifi:false};
  else if(p.endsWith("/settings"))data=[{domain:"sendspin",section_key:"settings.features",section:"Connected services",fields:[{id:"sendspin.enabled",name:"enabled",title:"Sendspin",group:"Sendspin",kind:"toggle",value:"off"}]}];
  else if(p.endsWith("/entities"))data=[];
  else if(p.endsWith("/youtube/")){
   if(req.method()==="POST"){const a=req.postDataJSON().action;writes.push(a);state=a==="connect"?{state:"linking",premium:false,code:"TEST-ONLY-1234",verification_url:"https://yt.be/activate",expires_at:new Date(Date.now()+60000).toISOString()}:a==="disconnect"?{state:"disconnected",premium:false}:a==="cancel"?{state:"disconnected",premium:false}:state;await route.fulfill({status:202,json:{}});return;}data=state;
  }
  await route.fulfill({json:data});
 });
 await page.goto("http://127.0.0.1:"+server.address().port+"/#features");
 const card=page.getByTestId("youtube-service");
 await card.getByRole("button",{name:"Sign in with Google",exact:true}).click();
 await card.getByText("TEST-ONLY-1234",{exact:true}).waitFor();
 const link=card.getByRole("link",{name:"yt.be/activate",exact:true});
 assert.equal(await link.getAttribute("href"),"https://yt.be/activate");
 assert.equal(await link.getAttribute("referrerpolicy"),"no-referrer");
 const qr=card.getByRole("img",{name:"YouTube activation website QR code"});
 await qr.waitFor();
 for(const viewport of [{width:1100,height:950},{width:390,height:844}]){
  await page.setViewportSize(viewport);
  await page.waitForFunction(()=>document.documentElement.scrollWidth<=innerWidth);
  const lb=await link.boundingBox(),qb=await qr.boundingBox();
  assert.ok(qb.x>=lb.x+lb.width,"QR stays right of the activation URL");
  assert.equal(qb.width,144);
  assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
 }
 await page.setViewportSize({width:1100,height:950});
 await card.getByRole("button",{name:"Cancel sign-in",exact:true}).click();
 await card.getByText("Not connected",{exact:true}).waitFor();
 assert.equal(await card.getByText("TEST-ONLY-1234",{exact:true}).count(),0);
 assert.equal(await qr.count(),0);
 state={state:"expired",premium:false};
 await card.getByRole("button",{name:"Try again",exact:true}).click();
 await card.getByText("TEST-ONLY-1234",{exact:true}).waitFor();
 state={state:"connected",premium:true};
 await card.getByText("Premium",{exact:true}).waitFor();
 assert.equal(await card.getByText("TEST-ONLY-1234",{exact:true}).count(),0);
 assert.equal(await qr.count(),0);
 await card.getByRole("button",{name:"Sign out on this device",exact:true}).click();
 const dialog=page.getByRole("dialog");
 await dialog.getByRole("button",{name:"Cancel",exact:true}).click();
 assert.ok(!writes.includes("disconnect"));
 state={state:"error",premium:false,error:"Local sign-out could not be completed. Try again."};
 await card.getByText("Connection failed",{exact:true}).waitFor();
 await card.getByRole("button",{name:"Sign out on this device",exact:true}).click();
 await dialog.getByRole("button",{name:"Sign out",exact:true}).click();
 await card.getByText("Not connected",{exact:true}).waitFor();
 assert.deepEqual(writes,["connect","cancel","connect","disconnect"]);
 await page.setViewportSize({width:390,height:844});
 assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
 await page.close();
 // First-party labels are localized; none of these may claim authentication.
 const query=(await readFile(resolve(root,"pkg/media/cast/protocols/webreceiver/youtube-account.go"),"utf8")).match(/const youtubeAccountJS = \`([\s\S]*?)\`/)[1];
 const tvBridge=await readFile(resolve(root,"pkg/media/cast/protocols/webreceiver/youtube-tv.js"),"utf8");
 const accountPage=await browser.newPage();
 await accountPage.addInitScript(tvBridge);
 await accountPage.route("https://www.youtube.com/tv",r=>r.fulfill({contentType:"text/html",body:"<ytlr-avatar-lockup></ytlr-avatar-lockup><ytlr-logo-entity premium></ytlr-logo-entity>"}));
 await accountPage.goto("https://www.youtube.com/tv");
 for(const title of ["Sign in","Anmelden","Accedi","Đăng nhập","Guest","Gast","Ospite","Khách","",null,17,{runs:[{text:"Anmelden"}]},{simpleText:"Sign in"},{runs:[]},{runs:[{unknown:true}]}]){
  await accountPage.evaluate(title=>document.querySelector("ytlr-avatar-lockup").__instance={props:{title}},title);
  const p=await accountPage.evaluate(script=>JSON.parse((0,eval)(script)),query);
  assert.equal(p.signedIn,false,title);assert.equal(p.premium,false,title);
  assert.equal(await accountPage.evaluate(()=>youtubeTVReady()),false);
 }
 for(const title of ["Synthetic member",{runs:[{text:"Synthetic "},{text:"member"}]},{simpleText:"Synthetic member"}]){
  await accountPage.evaluate(title=>document.querySelector("ytlr-avatar-lockup").__instance={props:{title}},title);
  const connected=await accountPage.evaluate(script=>JSON.parse((0,eval)(script)),query);
  assert.equal(connected.signedIn,true);assert.equal(connected.premium,true);
  assert.equal(await accountPage.evaluate(()=>youtubeTVReady()),true);
  assert.equal(await accountPage.evaluate(()=>!!document.querySelector("video")),false);
  assert.ok(!JSON.stringify(connected).includes("Synthetic"));
 }
 await accountPage.close();
 console.log("PASS YouTube service placement, activation, cancel, expiry/retry, Premium, sign-out confirmation and mobile width");
 // The TV bridge drives a real HTML media element, never external services.
 const bridge=tvBridge;
 // A route that creates no video must be retried, but never indefinitely.
 const missing=await browser.newPage();
 await missing.clock.install();
 await missing.route("https://www.youtube.com/tv",r=>r.fulfill({contentType:"text/html",body:"<ytlr-avatar-lockup></ytlr-avatar-lockup>"}));
 await missing.addInitScript(bridge);await missing.goto("https://www.youtube.com/tv");
 await missing.evaluate(()=>youtubeTVLoad("TEST0000000",0,true));
 assert.equal(await missing.evaluate(()=>__youtubeTVRequest.routes),1);
 for(const routes of [2,3]){
  await missing.clock.fastForward(8500);await missing.clock.fastForward(300);
  assert.equal(await missing.evaluate(()=>__youtubeTVRequest.routes),routes);
  assert.ok((await missing.evaluate(()=>location.hash)).includes("TEST0000000"));
 }
 await missing.clock.fastForward(9000);
 assert.equal(await missing.evaluate(()=>__youtubeTVRequest.routes),3);
 await missing.close();

 const data=Buffer.alloc(44+8000*12*2);data.write("RIFF");data.writeUInt32LE(data.length-8,4);data.write("WAVEfmt ",8);data.writeUInt32LE(16,16);data.writeUInt16LE(1,20);data.writeUInt16LE(1,22);data.writeUInt32LE(8000,24);data.writeUInt32LE(16000,28);data.writeUInt16LE(2,32);data.writeUInt16LE(16,34);data.write("data",36);data.writeUInt32LE(data.length-44,40);
 for(const play of [true,false]){
  const p=await browser.newPage();
  await p.route("https://www.youtube.com/tv",r=>r.fulfill({contentType:"text/html",body:'<ytlr-player></ytlr-player><video></video><script>document.querySelector("ytlr-player").__instance={J:{isAdActive:false,play(){const v=document.querySelector("video");if(!v.currentSrc)v.src="data:audio/wav;base64,'+data.toString("base64")+'";v.play().catch(()=>{})},pause(){document.querySelector("video").pause()}}};document.querySelector("video").addEventListener("seeking",()=>document.querySelector("video").play().catch(()=>{}));</script>'}));
  await p.addInitScript(bridge);await p.goto("https://www.youtube.com/tv");
  await p.evaluate(play=>youtubeTVLoad("TEST0000000",0,play),play);
  await p.waitForFunction(()=>document.querySelector("video").readyState>=2);
  if(play)await p.waitForFunction(()=>youtubePlayer.getCurrentTime()>0.3);
  else {await p.waitForTimeout(600);assert.equal(await p.evaluate(()=>document.querySelector("video").paused),true);}
  await p.evaluate(()=>youtubePlayer.pauseVideo());const at=await p.evaluate(()=>youtubePlayer.getCurrentTime());
  await p.waitForTimeout(350);assert.equal(await p.evaluate(()=>youtubePlayer.getCurrentTime()),at);
  await p.evaluate(()=>youtubePlayer.seekTo(7));await p.waitForFunction(()=>Math.abs(youtubePlayer.getCurrentTime()-7)<0.1);
  await p.waitForTimeout(500);assert.equal(await p.evaluate(()=>document.querySelector("video").paused),true);assert.ok(Math.abs(await p.evaluate(()=>youtubePlayer.getCurrentTime())-7)<0.1);
  await p.evaluate(()=>document.querySelector("ytlr-player").__instance.J.isAdActive=true);
  assert.equal(await p.evaluate(()=>youtubePlayer.getDuration()),0);
  await p.evaluate(()=>youtubePlayer.seekTo(9));assert.ok(Math.abs(await p.evaluate(()=>youtubePlayer.getCurrentTime())-7)<0.1);
  await p.evaluate(()=>document.querySelector("ytlr-player").__instance.J.isAdActive=false);
  await p.evaluate(()=>{youtubePlayer.seekTo(11.9);youtubePlayer.playVideo()});await p.waitForFunction(()=>youtubeStatus.state===0);
  await p.close();
 }
 console.log("PASS TV adapter real media play/pause/seek/ad-safe seek, initial pause and EOF latch");
} finally {await browser.close();server.close();}
