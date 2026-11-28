import assert from "node:assert/strict";
import test from "node:test";
import { renderToStaticMarkup } from "react-dom/server";
import { MantineProvider } from "@mantine/core";
import { IconSettings } from "@tabler/icons-react";
import { Settings } from "../src/Settings.tsx";
const section={key:"settings.volume",slug:"volume",title:"Sound",icon:IconSettings,count:1,groups:[{title:"Microphone",entries:[{domain:"microphone",field:{id:"microphone_gain",name:"gain",title:"Gain",group:"Microphone",kind:"number",value:"10",min:0,max:59}}]}]};
const render=(available)=>renderToStaticMarkup(<MantineProvider><Settings section={section} focus="" change={async()=>{}} microphoneTest={available}/></MantineProvider>);
test("microphone test is capability-gated and adjacent to microphone settings",()=>{
 assert(!render(false).includes('data-testid="microphone-test"'));
 const html=render(true);assert(html.includes('data-testid="microphone-test"'));assert(html.indexOf('id="f-microphone_gain"')<html.indexOf('data-testid="microphone-test"'));assert(html.includes("Processed recognition signal"));assert(html.includes("Play on Echo"));assert(!html.includes("Delete take"));assert(html.includes("Start recording (8 s)"));assert(html.includes("Microphone gain")||html.includes("Gain"));assert(!html.includes('<audio'));assert(!html.includes('download='));assert(!html.includes("Audio stays"));assert(!html.includes("after 2 minutes"));
});
