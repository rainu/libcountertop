// Isolated browser contract tests. All configuration and RPC results are synthetic.
import assert from "node:assert/strict";
import { createServer } from "node:http";
import { readFile } from "node:fs/promises";
import { resolve, extname } from "node:path";
import { chromium } from "playwright";
import { viewFixture } from "./openclaw-fixture.mjs";

const dist = resolve(import.meta.dirname, "../../pkg/runtime/webui/dist");
const server = createServer(async (req, res) => {
  try {
    const path = new URL(req.url, "http://localhost").pathname;
    const f = await readFile(
      resolve(dist, "." + (path === "/" ? "/index.html" : path)),
    );
    res.setHeader(
      "Content-Type",
      extname(path) === ".js"
        ? "text/javascript"
        : extname(path) === ".css"
          ? "text/css"
          : "text/html",
    );
    res.end(f);
  } catch {
    res.writeHead(404);
    res.end();
  }
});
await new Promise((r) => server.listen(0, "127.0.0.1", r));
const browser = await chromium.launch({
  headless: true,
  args: ["--no-sandbox"],
});
const page = await browser.newPage({ viewport: { width: 1200, height: 950 } });
const wav = Buffer.alloc(44 + 4800 * 2);
wav.write("RIFF"); wav.writeUInt32LE(wav.length - 8, 4); wav.write("WAVEfmt ", 8);
wav.writeUInt32LE(16, 16); wav.writeUInt16LE(1, 20); wav.writeUInt16LE(1, 22);
wav.writeUInt32LE(24000, 24); wav.writeUInt32LE(48000, 28); wav.writeUInt16LE(2, 32);
wav.writeUInt16LE(16, 34); wav.write("data", 36); wav.writeUInt32LE(wav.length - 44, 40);
for(let i=0;i<4800;i++) wav.writeInt16LE(Math.round(4000*Math.sin(2*Math.PI*440*i/24000)),44+i*2);
let v = viewFixture(),
  writes = [],
  errors = [],
  deny = false;
page.on("pageerror", (e) => errors.push(e.message));
await page.route("**/api/v1/**", async (route) => {
  const req = route.request(),
    u = new URL(req.url()),
    path = u.pathname.slice("/api/v1".length);
  const body = req.postDataJSON(),
    method = req.method();
  let data = {},
    status = 200;
  if (path === "/device") {
    data = {
      name: "Synthetic test device",
      pages: ["openclaw"],
      screen: false,
      wifi: false,
    };
    if (deny) status = 401;
  } else if (path === "/settings" || path === "/entities") data = [];
  else if (path === "/events") {
    await route.fulfill({
      status: 200,
      contentType: "text/event-stream",
      body: 'data: {"type":"states","states":[]}\n\n',
    });
    return;
  } else if (path === "/openclaw") data = v;
  else if (path === "/openclaw/connection") {
    writes.push({ path, body });
    Object.assign(v.connection, body);
    delete v.connection.token;
    data = v;
  } else if (path.startsWith("/openclaw/slots/")) {
    const n = Number(path.slice(-1)) - 1;
    if (method === "PUT") {
      writes.push({ path, body });
      if (body.deviceContextTemplate === "{{unknown}}") {
        status = 422;
        data = { error: "talk_context_placeholder" };
      } else {
        v.slots[n] = body;
        data = body;
      }
    } else data = v.slots[n];
  } else if (path === "/openclaw/choices/agents")
    data = [
      { value: "test", label: "Test agent" },
      { value: "other", label: "Other agent" },
    ];
  else if (path === "/openclaw/choices/models")
    data = [
      { value: "", label: "Gateway default" },
      {
        value: JSON.stringify({ Provider: "openai", Model: "native-test" }),
        label: "native-test · OpenAI",
      },
    ];
  else if (path === "/openclaw/choices/sessions")
    data = [
      { value: "", label: "New session" },
      { value: "agent:test:existing", label: "Existing test session" },
    ];
  else if (path.startsWith("/openclaw/wake/")) {
    v.wake.ids[Number(path.slice(-1)) - 1] = body.value;
    writes.push({ path, body });
    data = v;
  } else if (path === "/openclaw/permissions/operator") {
    v.requestedScopes = body.scopes;
    data = v;
    writes.push({ path, body });
  } else if (path === "/openclaw/permissions/node") {
    Object.assign(v.grants, body.grants);
    data = v;
    writes.push({ path, body });
  } else if (path === "/openclaw/preview")
    {
    writes.push({path, body});
    data = body.cue === "context" ? { text: "Use synthetic node", bytes: 18, maxBytes: 1536 } : {audio: wav.toString("base64")};
  }
  else if (path === "/openclaw/history") data = [];
  else if (path === "/openclaw/actions") {
    writes.push({ path, body });
    data = v;
  }
  await route.fulfill({
    status,
    contentType: "application/json",
    body: JSON.stringify(data),
  });
});
try {
  await page.goto("http://127.0.0.1:" + server.address().port + "/#openclaw");
  await page.getByRole("heading", { name: "OpenClaw", exact: true }).waitFor();
  const field = (name) =>
    page
      .locator("label")
      .filter({
        has: page.locator("span", { hasText: new RegExp("^" + name + "$") }),
      })
      .locator("input,textarea,select")
      .first();
  assert.equal(await page.locator("input[type=password]").inputValue(), "");
  await field("Controller UI device name").fill("Renamed UI");
  await page.getByRole("button", { name: "Save", exact: true }).click();
  await page.getByRole("status").filter({ hasText: "Saved" }).waitFor();
  assert.ok(!("token" in writes.at(-1).body));
  await page.locator("input[type=password]").fill("SYNTHETIC_REPLACEMENT");
  await page.getByRole("button", { name: "Save", exact: true }).click();
  await page.waitForFunction(
    () => document.querySelector("input[type=password]")?.value === "",
  );
  assert.equal(writes.at(-1).body.token, "SYNTHETIC_REPLACEMENT");
  await page.getByRole("checkbox", { name: /Clear stored/ }).check();
  await page.getByRole("button", { name: "Save", exact: true }).click();
  await page.getByRole("status").filter({ hasText: "Saved" }).waitFor();
  assert.equal(writes.at(-1).body.token, "");

  await page.getByRole("button", { name: "Assistant 1", exact: true }).click();
  assert.ok(await page.locator('nav img[aria-hidden="true"]').count() > 0);
  const aligned = async () => {
    const rows = await page.locator(".claw-control-row").evaluateAll(rows => rows.map(row => {
      const a=row.querySelector("select").getBoundingClientRect(), b=row.querySelector("button").getBoundingClientRect();
      return Math.abs((a.top+a.height/2)-(b.top+b.height/2));
    }));
    assert.ok(rows.length >= 5);
    assert.ok(rows.every(delta=>delta <= 1), JSON.stringify(rows));
  };
  await aligned();
  for (let i=1;i<=3;i++) {
    await page.getByRole("button", {name:"Preview",exact:true}).nth(i).click();
    await page.locator("audio[controls]").waitFor();
    await page.waitForFunction(()=>document.querySelector("audio").readyState>=2);
    assert.equal(writes.at(-1).path,"/openclaw/preview");
    assert.equal(writes.at(-1).body.cue,["ready","progress","end"][i-1]);
    assert.ok(await page.locator("audio").evaluate(a=>a.src.startsWith("data:audio/wav;base64,")));
    await page.locator("audio").evaluate(a=>a.play());
    await page.waitForFunction(()=>document.querySelector("audio").ended);
  }
  await page.locator("audio").evaluate(a=>{a.loop=true;return a.play();});
  await page.evaluate(()=>{window.previousAudition=document.querySelector("audio");});
  await page.getByRole("button",{name:"Assistant 2",exact:true}).click();
  await aligned();
  assert.equal(await page.locator("audio").count(),0);
  assert.ok(await page.evaluate(()=>window.previousAudition.paused));
  await page.getByRole("button",{name:"Assistant 1",exact:true}).click();
  await field("Local wake word").selectOption("wake-b");
  await page
    .getByRole("button", { name: "Save · Local wake word", exact: true })
    .click();
  await page.getByRole("status").filter({ hasText: "Saved" }).waitFor();
  assert.equal(v.wake.ids[0], "wake-b");
  await field("Greeting prompt").fill("Synthetic greeting");
  await field("Device context prompt").fill("Use {{node_id}}");
  await page
    .getByRole("checkbox", { name: "Resume session", exact: true })
    .check();
  await page
    .getByRole("button", { name: "Load choices", exact: true })
    .nth(2)
    .click();
  await field("Session").selectOption("agent:test:existing");
  await page
    .getByRole("button", { name: "Load choices", exact: true })
    .nth(1)
    .click();
  await field("Model").selectOption(
    JSON.stringify({ Provider: "openai", Model: "native-test" }),
  );
  await field("Match position").selectOption("end");
  await field("Evaluate").selectOption("immediate");
  await field("Connection tone").selectOption("none");
  await page.getByRole("button", { name: "Save", exact: true }).click();
  await page.getByRole("status").filter({ hasText: "Saved" }).waitFor();
  assert.equal(v.slots[0].greetingPrompt, "Synthetic greeting");
  assert.equal(v.slots[0].model, "native-test");
  assert.equal(v.slots[0].context, "agent:test:existing");
  await page.getByRole("button", { name: "Assistant 2", exact: true }).click();
  assert.equal(await field("Timeout · Seconds").inputValue(), "600");
  assert.ok(await field("Device context prompt").isDisabled());
  await page.getByRole("button", { name: "Assistant 1", exact: true }).click();
  await field("Device context prompt").fill("{{unknown}}");
  await page.getByRole("button", { name: "Save", exact: true }).click();
  await page
    .getByRole("alert")
    .filter({ hasText: "talk_context_placeholder" })
    .waitFor();
  assert.equal(
    await field("Device context prompt").inputValue(),
    "{{unknown}}",
  );
  assert.equal(v.slots[0].deviceContextTemplate, "Use {{node_id}}");

  await page.getByRole("button", { name: "Permissions", exact: true }).click();
  await page
    .getByRole("checkbox", { name: "operator.write", exact: true })
    .check();
  await page.getByRole("button", { name: "Save", exact: true }).first().click();
  await page.getByRole("status").filter({ hasText: "Saved" }).waitFor();
  assert.ok(v.requestedScopes.includes("operator.write"));
  await page.getByRole("checkbox", { name: "audio", exact: true }).check();
  await page.getByRole("button", { name: "Save", exact: true }).nth(1).click();
  await page.getByRole("status").filter({ hasText: "Saved" }).waitFor();
  assert.ok(v.grants.audio);

  await page.setViewportSize({ width: 390, height: 844 });
  await page.getByRole("button", { name: "Assistant 1", exact: true }).click();
  assert.ok(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  );
  await page.screenshot({
    path: resolve(dist, "../issue37-mobile.png"),
    fullPage: true,
  });
  await page
    .getByRole("button", { name: "Local history", exact: true })
    .click();
  await page.getByText("No local history", { exact: true }).waitFor();
  assert.deepEqual(errors, []);
  deny = true;
  await page.reload();
  await page.locator("input[type=password]").waitFor();
  assert.ok(
    !(await page
      .getByRole("heading", { name: "OpenClaw", exact: true })
      .count()),
  );
  console.log(
    JSON.stringify({
      passed: [
        "OpenClaw navigation icon",
        "aligned controls in both assistant slots",
        "browser audio decode/play/end for all three cues",
        "audition cleanup on tab change",
        "write-only keep/replace/clear",
        "local wake selection",
        "per-slot persistence contract",
        "dynamic model/session choices",
        "conditional controls",
        "invalid input preserves draft",
        "operator and Node permission forms",
        "mobile layout",
        "local history",
        "unauthorized sign-out",
      ],
      pageErrors: errors.length,
    }),
  );
} finally {
  await browser.close();
  await new Promise((r) => server.close(r));
}
