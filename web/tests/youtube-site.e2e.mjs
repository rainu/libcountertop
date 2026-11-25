import assert from "node:assert/strict";
import {readFile} from "node:fs/promises";
import {resolve} from "node:path";
import {chromium} from "playwright";
const bridge=await readFile(resolve(import.meta.dirname,"../../pkg/media/cast/protocols/webreceiver/youtube-site.js"),"utf8");
// A real browser media element with deterministic synthetic audio, no YouTube/network.
const rate=8000,seconds=12,data=Buffer.alloc(44+rate*seconds*2);
data.write("RIFF");data.writeUInt32LE(data.length-8,4);data.write("WAVEfmt ",8);
data.writeUInt32LE(16,16);data.writeUInt16LE(1,20);data.writeUInt16LE(1,22);
data.writeUInt32LE(rate,24);data.writeUInt32LE(rate*2,28);data.writeUInt16LE(2,32);
data.writeUInt16LE(16,34);data.write("data",36);data.writeUInt32LE(data.length-44,40);
const html='<div id="movie_player"><video src="data:audio/wav;base64,'+data.toString("base64")+'"></video></div>';
const browser=await chromium.launch({headless:true,args:["--no-sandbox","--autoplay-policy=no-user-gesture-required"]});
try {
 for(const autoplay of ["1","0"]) {
  const page=await browser.newPage();
  await page.route("https://www.youtube.com/watch*",route=>route.fulfill({contentType:"text/html",body:html}));
  await page.addInitScript(bridge);
  await page.goto("https://www.youtube.com/watch?v=fixture&autoplay="+autoplay);
  await page.waitForFunction(()=>window.youtubeStatus?.ready && document.querySelector("video").readyState>=2);
  assert.equal(await page.evaluate(()=>window.youtubeStatus.site),true);
  if(autoplay==="0") assert.equal(await page.evaluate(()=>document.querySelector("video").paused),true);
  await page.evaluate(()=>youtubePlayer.playVideo());
  await page.waitForFunction(()=>youtubePlayer.getCurrentTime()>0.3);
  await page.evaluate(()=>youtubePlayer.pauseVideo());
  const at=await page.evaluate(()=>youtubePlayer.getCurrentTime());
  await page.waitForTimeout(350);
  assert.equal(await page.evaluate(()=>youtubePlayer.getCurrentTime()),at);
  assert.equal(await page.evaluate(()=>youtubeStatus.state),2);
  await page.evaluate(()=>youtubePlayer.seekTo(7));
  await page.waitForFunction(()=>Math.abs(youtubePlayer.getCurrentTime()-7)<0.1);
  await page.evaluate(()=>document.getElementById("movie_player").classList.add("ad-showing"));
  assert.equal(await page.evaluate(()=>youtubePlayer.getDuration()),0);
  await page.evaluate(()=>youtubePlayer.seekTo(9));
  assert.ok(Math.abs(await page.evaluate(()=>youtubePlayer.getCurrentTime())-7)<0.1);
  await page.evaluate(()=>document.getElementById("movie_player").classList.remove("ad-showing"));
  assert.equal(await page.evaluate(()=>youtubePlayer.getDuration()),12);
  await page.evaluate(()=>{youtubePlayer.seekTo(11.9);youtubePlayer.playVideo();});
  await page.waitForFunction(()=>youtubeStatus.state===0);
  await page.close();
 }
 console.log("PASS: full-site bridge uses real media play/pause/seek/end, initial pause and ad-safe seeking without network/cookies");
} finally {await browser.close();}
