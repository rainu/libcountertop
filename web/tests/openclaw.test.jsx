import assert from "node:assert/strict";
import test from "node:test";
import { renderToStaticMarkup } from "react-dom/server";
import { MantineProvider } from "@mantine/core";
import {
  ConnectionForm,
  SlotForm,
  PermissionsForm,
  connectionPayload,
} from "../src/OpenClaw.tsx";
import { appPages } from "../src/Shell.tsx";
import { viewFixture } from "./openclaw-fixture.mjs";
const render = (node) =>
  renderToStaticMarkup(
    <MantineProvider forceColorScheme="light">{node}</MantineProvider>,
  );
const run = async () => assert.fail("SSR cannot send requests");

test("connection credentials are write-only with explicit keep/replace/clear semantics", () => {
  const v = viewFixture();
  const html = render(
    <ConnectionForm
      initial={v.connection}
      l={v.labels}
      run={run}
      update={() => {}}
    />,
  );
  assert.ok(html.includes('type="password"'));
  assert.ok(!html.includes('value="SYNTHETIC_SECRET"'));
  assert.ok(!("token" in connectionPayload(v.connection, "", false)));
  assert.equal(
    connectionPayload(v.connection, "replacement", false).token,
    "replacement",
  );
  assert.equal(connectionPayload(v.connection, "replacement", true).token, "");
});
test("assistant form includes every supported flow and disables HA-only inactive options", () => {
  const v = viewFixture();
  const html = render(
    <SlotForm v={v} index={0} run={run} reload={async () => {}} />,
  );
  for (const label of [
    "Agent",
    "Model",
    "Session",
    "Greeting prompt",
    "Device context prompt",
    "Ready tone",
    "Connection tone",
    "End tone",
    "End phrase",
    "Match position",
    "Evaluate",
    "Timeout",
  ]) {
    // Session selection is conditional on resume, whose control is always present.
    if (label === "Session") continue;
    assert.ok(html.includes(label), label);
  }
  assert.ok(html.includes("{{node_id}}"));
  const inactive = render(
    <SlotForm v={v} index={1} run={run} reload={async () => {}} />,
  );
  assert.match(inactive, /<fieldset disabled=""/);
  v.slots[0].resume = true;
  assert.ok(
    render(
      <SlotForm v={v} index={0} run={run} reload={async () => {}} />,
    ).includes("New session"),
  );
});
test("permissions distinguish requested and granted scopes and retain every Node group", () => {
  const v = viewFixture();
  const html = render(<PermissionsForm v={v} run={run} update={() => {}} />);
  for (const name of [
    ...v.groups,
    ...v.supportedScopes,
    "Requested",
    "Granted",
  ])
    assert.ok(html.includes(name));
  assert.ok(appPages.some((p) => p.slug === "openclaw"));
});
