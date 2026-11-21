import { useEffect, useRef, useState } from "react";
import {
  Alert,
  Button,
  Group,
  Loader,
  Stack,
  Text,
  Title,
} from "@mantine/core";
import { openclawRequest, SignedOut, HTTPError, type Choice } from "./api";

export type Slot = {
  openclaw: boolean;
  agent?: string;
  agentLabel?: string;
  provider?: string;
  model?: string;
  context?: string;
  contextLabel?: string;
  resume: boolean;
  timeout: number;
  deviceContextTemplate?: string;
  greetingPrompt?: string;
  startTone?: string;
  progressTone?: string;
  endTone?: string;
  ending: { phrase?: string; position?: string; timing?: string };
};
type Status = {
  State: string;
  Reason: string;
  Configured: boolean;
  Paired: boolean;
  Enabled: boolean;
};
export type Connection = {
  url: string;
  uiName: string;
  nodeName: string;
  tokenConfigured: boolean;
  ui: Status;
  node: Status;
};
export type View = {
  connection: Connection;
  slots: Slot[];
  requestedScopes: string[];
  grantedScopes: string[];
  supportedScopes: string[];
  grants: Record<string, boolean>;
  groups: string[];
  tones: Choice[];
  language: string;
  labels: Record<string, string>;
  defaults: {
    deviceContextTemplate: string;
    greetingPrompt: string;
    ending: Slot["ending"];
  };
  placeholders: string[];
  wake?: { ids: string[]; choices: Choice[] };
  events: Record<
    string,
    { At: string; Direction: string; Type: string; Result: string }[]
  >;
};
type Labels = Record<string, string>;
type Request = <T>(path: string, method?: string, body?: unknown) => Promise<T>;
const attempt = (f: () => Promise<unknown>) => {
  void f().catch(() => {});
};
const modelValue = (s: Slot) =>
  s.model ? JSON.stringify({ Provider: s.provider, Model: s.model }) : "";
const optionList = (list: Choice[], value: string, label = value) =>
  value && !list.some((x) => x.value === value)
    ? [{ value, label }, ...list]
    : list;
export function connectionPayload(
  c: Connection,
  token: string,
  clear: boolean,
) {
  return {
    url: c.url,
    uiName: c.uiName,
    nodeName: c.nodeName,
    ...(clear ? { token: "" } : token ? { token } : {}),
  };
}
function Field({
  label,
  children,
  hint,
}: {
  label: string;
  children: React.ReactNode;
  hint?: string;
}) {
  return (
    <label className="claw-field">
      <span>{label}</span>
      {children}
      {hint && <small>{hint}</small>}
    </label>
  );
}
function Select({
  label,
  value,
  choices,
  onChange,
  disabled,
}: {
  label: string;
  value: string;
  choices: Choice[];
  onChange: (v: string) => void;
  disabled?: boolean;
}) {
  return (
    <Field label={label}>
      <select
        value={value}
        disabled={disabled}
        onChange={(e) => onChange(e.target.value)}
      >
        {optionList(choices, value).map((c) => (
          <option key={c.value} value={c.value}>
            {c.label}
          </option>
        ))}
      </select>
    </Field>
  );
}
export function ConnectionForm({
  initial,
  l,
  run,
  update,
}: {
  initial: Connection;
  l: Labels;
  run: Request;
  update: (v: View) => void;
}) {
  const [c, setC] = useState(initial),
    [token, setToken] = useState(""),
    [clear, setClear] = useState(false);
  const action = async (action: string, role = "both") => {
    const confirm = action === "forget" || action === "pair_again";
    if (confirm && !window.confirm(l.confirm)) return;
    update(await run<View>("/actions", "POST", { action, role, confirm }));
  };
  return (
    <Stack>
      <Text>
        {l.connection}: {l["state." + initial.ui.State] ?? initial.ui.State}{" "}
        {initial.ui.Reason && "· " + initial.ui.Reason} / Node:{" "}
        {l["state." + initial.node.State] ?? initial.node.State}{" "}
        {initial.node.Reason && "· " + initial.node.Reason}
      </Text>
      <form
        onSubmit={(e) => {
          e.preventDefault();
          attempt(async () => {
            update(
              await run<View>(
                "/connection",
                "PUT",
                connectionPayload(c, token, clear),
              ),
            );
            setToken("");
            setClear(false);
          });
        }}
      >
        <Field label={l.endpoint}>
          <input
            type="url"
            required
            value={c.url}
            onChange={(e) => setC({ ...c, url: e.target.value })}
            placeholder="wss://gateway.example"
          />
        </Field>
        <Field label={l.token} hint={l.tokenHint}>
          <input
            aria-label={l.token}
            type="password"
            autoComplete="new-password"
            value={token}
            disabled={clear}
            onChange={(e) => setToken(e.target.value)}
          />
        </Field>
        <label>
          <input
            type="checkbox"
            checked={clear}
            onChange={(e) => setClear(e.target.checked)}
          />{" "}
          {l.clearToken} ({initial.tokenConfigured ? "✓" : "—"})
        </label>
        {(["uiName", "nodeName"] as const).map((k) => (
          <Field key={k} label={l[k]}>
            <input
              maxLength={80}
              value={c[k]}
              onChange={(e) => setC({ ...c, [k]: e.target.value })}
            />
          </Field>
        ))}
        <Button type="submit">{l.save}</Button>
      </form>
      <Group>
        {["connect", "disconnect", "pair_again", "forget"].map((a) => (
          <Button
            color={a === "forget" ? "red" : undefined}
            key={a}
            onClick={() => attempt(() => action(a))}
          >
            {l[a]}
          </Button>
        ))}
      </Group>
      <Group>
        {(["ui", "node"] as const).map((role) => (
          <Button
            key={role}
            variant="light"
            onClick={() => attempt(() => action("connect", role))}
          >
            {l.connect} · {role === "ui" ? "Controller UI" : "Node"}
          </Button>
        ))}
      </Group>
    </Stack>
  );
}
export function PermissionsForm({
  v,
  run,
  update,
}: {
  v: View;
  run: Request;
  update: (v: View) => void;
}) {
  const l = v.labels,
    [scopes, setScopes] = useState(v.requestedScopes),
    [grants, setGrants] = useState(v.grants);
  const saveScopes = (s: string[]) =>
    run<View>("/permissions/operator", "PUT", { scopes: s }).then(update);
  const saveGrants = (g: Record<string, boolean>) =>
    run<View>("/permissions/node", "PUT", { grants: g }).then(update);
  return (
    <Stack>
      <Alert>{l.permissionsHint}</Alert>
      <Title order={3}>{l.operator}</Title>
      <Text>
        {l.requested}: {v.requestedScopes.join(", ") || "—"}
      </Text>
      <Text>
        {l.granted}: {v.grantedScopes.join(", ") || "—"}
      </Text>
      {v.supportedScopes.map((s) => (
        <label key={s}>
          <input
            type="checkbox"
            checked={scopes.includes(s)}
            onChange={(e) =>
              setScopes(
                e.target.checked
                  ? [...scopes, s]
                  : scopes.filter((x) => x !== s),
              )
            }
          />{" "}
          {l["scope." + s] ? (
            <>
              {l["scope." + s]} <small>{s}</small>
            </>
          ) : (
            s
          )}
        </label>
      ))}
      <Group>
        <Button onClick={() => attempt(() => saveScopes(scopes))}>
          {l.save}
        </Button>
        <Button
          variant="light"
          onClick={() => attempt(() => saveScopes(v.supportedScopes))}
        >
          ✓ {l.operator}
        </Button>
        <Button
          variant="light"
          onClick={() =>
            attempt(() => saveScopes(["operator.read", "operator.talk"]))
          }
        >
          operator.read + operator.talk
        </Button>
        <Button
          color="red"
          variant="light"
          onClick={() => attempt(() => saveScopes([]))}
        >
          — {l.operator}
        </Button>
      </Group>
      <Title order={3}>{l.node}</Title>
      {v.groups.map((g) => (
        <label key={g}>
          <input
            type="checkbox"
            checked={grants[g]}
            onChange={(e) => setGrants({ ...grants, [g]: e.target.checked })}
          />{" "}
          {l["group." + g] ? (
            <>
              {l["group." + g]} <small>{g}</small>
            </>
          ) : (
            g
          )}
        </label>
      ))}
      <Group>
        <Button onClick={() => attempt(() => saveGrants(grants))}>
          {l.save}
        </Button>
        {[true, false].map((on) => (
          <Button
            key={String(on)}
            variant="light"
            color={on ? undefined : "red"}
            onClick={() =>
              attempt(() =>
                saveGrants(Object.fromEntries(v.groups.map((g) => [g, on]))),
              )
            }
          >
            {on ? "✓" : "—"} {l.node}
          </Button>
        ))}
      </Group>
    </Stack>
  );
}
export function SlotForm({
  v,
  index,
  run,
  reload,
}: {
  v: View;
  index: number;
  run: Request;
  reload: () => Promise<void>;
}) {
  const l = v.labels,
    [s, setS] = useState(v.slots[index]),
    [agents, setAgents] = useState<Choice[]>([]),
    [models, setModels] = useState<Choice[]>([]),
    [sessions, setSessions] = useState<Choice[]>([]),
    [preview, setPreview] = useState(""),
    [audio, setAudio] = useState<{data: string; id: number} | null>(null);
  const player = useRef<HTMLAudioElement>(null);
  const audition = useRef(0);
  useEffect(() => () => { audition.current++; }, []);
  useEffect(() => {
    const element = player.current;
    return () => element?.pause();
  }, [audio]);
  const choices = async (kind: string) => {
    const list = await run<Choice[]>(
      "/choices/" +
        kind +
        "?slot=" +
        (index + 1) +
        "&agent=" +
        encodeURIComponent(s.agent ?? ""),
    );
    if (kind === "agents") setAgents(list);
    if (kind === "models") setModels(list);
    if (kind === "sessions") setSessions(list);
  };
  const set = (key: keyof Slot, value: unknown) => setS({ ...s, [key]: value });
  const defaults = v.defaults;
  return (
    <Stack>
      <Title order={3}>
        {l.assistant} {index + 1}
      </Title>
      {v.wake && s.openclaw && (
        <WakeForm
          initial={v.wake.ids[index] ?? ""}
          choices={v.wake.choices}
          label={l.wakeWord}
          saveLabel={l.save}
          save={async (value) => {
            await run("/wake/" + (index + 1), "PUT", { value });
            await reload();
          }}
        />
      )}
      <form
        onSubmit={(e) => {
          e.preventDefault();
          attempt(async () => {
            await run<Slot>("/slots/" + (index + 1), "PUT", s);
            await reload();
          });
        }}
      >
        <Select
          label={l.target}
          value={s.openclaw ? "openclaw" : "ha"}
          choices={[
            { value: "ha", label: "Home Assistant" },
            { value: "openclaw", label: "OpenClaw" },
          ]}
          onChange={(value) => set("openclaw", value === "openclaw")}
        />
        <fieldset disabled={!s.openclaw} className="claw-options">
          <Group align="end" className="claw-control-row">
            <Select
              label={l.agent}
              value={s.agent ?? ""}
              choices={optionList(
                [{ value: "", label: "—" }, ...agents],
                s.agent ?? "",
                s.agentLabel,
              )}
              onChange={(agent) => {
                setS({
                  ...s,
                  agent,
                  agentLabel: "",
                  context: "",
                  contextLabel: "",
                });
                setSessions([]);
              }}
            />
            <Button
              variant="light"
              onClick={() => attempt(() => choices("agents"))}
            >
              {l.loadChoices}
            </Button>
          </Group>
          <Group align="end" className="claw-control-row">
            <Select
              label={l.model}
              value={modelValue(s)}
              choices={optionList(
                models.length ? models : [{ value: "", label: l.default }],
                modelValue(s),
                s.model,
              )}
              onChange={(value) => {
                const x = value ? JSON.parse(value) : {};
                setS({
                  ...s,
                  provider: x.Provider ?? "",
                  model: x.Model ?? "",
                });
              }}
            />
            <Button
              variant="light"
              onClick={() => attempt(() => choices("models"))}
            >
              {l.loadChoices}
            </Button>
          </Group>
          <Text size="sm" c="dimmed">
            {l.modelHint}
          </Text>
          <label>
            <input
              type="checkbox"
              checked={s.resume}
              onChange={(e) => set("resume", e.target.checked)}
            />{" "}
            {l.resume}
          </label>
          <Text size="sm">{l.resumeHint}</Text>
          {s.resume && (
            <Group align="end" className="claw-control-row">
              <Select
                label={l.session}
                value={s.context ?? ""}
                choices={optionList(
                  [
                    { value: "", label: l.newContext },
                    ...sessions.filter((x) => x.value !== ""),
                  ],
                  s.context ?? "",
                  s.contextLabel || l.deviceSession || l.session,
                )}
                onChange={(context) =>
                  setS({ ...s, context, contextLabel: "" })
                }
              />
              <Button
                variant="light"
                disabled={!s.agent}
                onClick={() => attempt(() => choices("sessions"))}
              >
                {l.loadChoices}
              </Button>
            </Group>
          )}
          <Text size="sm">{l.nextConversation}</Text>
          <Field label={l.greeting} hint={l.promptHint}>
            <textarea
              rows={4}
              value={s.greetingPrompt ?? ""}
              placeholder={defaults.greetingPrompt}
              onChange={(e) => set("greetingPrompt", e.target.value)}
            />
          </Field>
          <Field label={l.devicePrompt} hint={l.promptHint}>
            <textarea
              rows={5}
              value={s.deviceContextTemplate ?? ""}
              placeholder={defaults.deviceContextTemplate}
              onChange={(e) => set("deviceContextTemplate", e.target.value)}
            />
          </Field>
          <Text size="xs">
            {v.placeholders.map((p) => "{{" + p + "}}").join(" · ")}
          </Text>
          <Group>
            <Button
              variant="light"
              onClick={() =>
                attempt(async () => {
                  const p = await run<{
                    text: string;
                    bytes: number;
                    maxBytes: number;
                  }>("/preview", "POST", {
                    cue: "context",
                    template: s.deviceContextTemplate ?? "",
                  });
                  setPreview(p.bytes + " / " + p.maxBytes + "\n" + p.text);
                })
              }
            >
              {l.preview}
            </Button>
            <Button
              variant="light"
              onClick={() => {
                setS({ ...s, greetingPrompt: "", deviceContextTemplate: "" });
                setPreview("");
              }}
            >
              {l.restoreDefaults}
            </Button>
          </Group>
          {preview && <pre className="claw-preview">{preview}</pre>}
          {(["startTone", "progressTone", "endTone"] as const).map((k, i) => (
            <Group key={k} align="end" className="claw-control-row">
              <Select
                label={l[k]}
                value={s[k] ?? ""}
                choices={optionList(v.tones, s[k] ?? "")}
                onChange={(value) => set(k, value)}
              />
              <Button
                variant="light"
                onClick={() =>
                  attempt(async () => {
                    const id = ++audition.current;
                    player.current?.pause();
                    setAudio(null);
                    const result = await run<{audio: string}>("/preview", "POST", {
                      cue: ["ready", "progress", "end"][i],
                      choice: s[k] ?? "",
                    });
                    if (id === audition.current) setAudio({data: result.audio, id});
                  })
                }
              >
                {l.preview}
              </Button>
            </Group>
          ))}
          {audio && <audio key={audio.id} ref={player} controls autoPlay
            aria-label={l.preview} src={"data:audio/wav;base64," + audio.data} />}
          <Title order={4}>{l.ending}</Title>
          <Field label={l.phrase}>
            <input
              maxLength={256}
              value={s.ending.phrase ?? ""}
              placeholder={defaults.ending.phrase}
              onChange={(e) =>
                set("ending", { ...s.ending, phrase: e.target.value })
              }
            />
          </Field>
          <Select
            label={l.position}
            value={s.ending.position ?? "exact"}
            choices={["exact", "start", "end"].map((value) => ({
              value,
              label: l[value === "exact" ? "contains" : value],
            }))}
            onChange={(position) => set("ending", { ...s.ending, position })}
          />
          <Select
            label={l.timing}
            value={s.ending.timing ?? "turn_end"}
            choices={["turn_end", "immediate"].map((value) => ({
              value,
              label: l[value],
            }))}
            onChange={(timing) => set("ending", { ...s.ending, timing })}
          />
          <Select
            label={l.timeout + " · " + l.seconds}
            value={String(s.timeout)}
            choices={[15, 30, 60, 120, 300, 900].map((n) => ({
              value: String(n),
              label: String(n),
            }))}
            onChange={(value) => set("timeout", Number(value))}
          />
        </fieldset>
        <Button type="submit">{l.save}</Button>
      </form>
    </Stack>
  );
}
function WakeForm({
  initial,
  choices,
  label,
  saveLabel,
  save,
}: {
  initial: string;
  choices: Choice[];
  label: string;
  saveLabel: string;
  save: (value: string) => Promise<void>;
}) {
  const [value, setValue] = useState(initial);
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        attempt(() => save(value));
      }}
    >
      <Group align="end" className="claw-control-row">
        <Select
          label={label}
          value={value}
          choices={optionList(choices, value)}
          onChange={setValue}
        />
        <Button type="submit">
          {saveLabel} · {label}
        </Button>
      </Group>
    </form>
  );
}
type Conversation = {
  Generation: number;
  Started: string;
  Entries: { Role: string; Text: string; Timestamp: string }[];
};
function History({ run, l }: { run: Request; l: Labels }) {
  const [calls, setCalls] = useState<Conversation[]>([]);
  const load = async () => setCalls(await run<Conversation[]>("/history"));
  useEffect(() => {
    attempt(load);
  }, []);
  const remove = async (id: number) => {
    if (!window.confirm(l.confirmHistory ?? l.confirm)) return;
    await run("/history/" + id, "DELETE", { confirm: true });
    await load();
  };
  return (
    <Stack>
      <Text>{l.historyHint}</Text>
      <Button
        color="red"
        variant="light"
        onClick={() =>
          attempt(() => run("/actions", "POST", { action: "stop_talk" }))
        }
      >
        {l.stopTalk}
      </Button>
      <Group>
        <Button onClick={() => attempt(() => load())}>{l.refresh}</Button>
        <Button color="red" onClick={() => attempt(() => remove(0))}>
          {l.deleteAll}
        </Button>
      </Group>
      {calls.length === 0 && <Text>{l.empty}</Text>}
      {[...calls].reverse().map((c) => (
        <section key={c.Generation}>
          <Group>
            <Text>{new Date(c.Started).toLocaleString()}</Text>
            <Button
              variant="light"
              color="red"
              onClick={() => attempt(() => remove(c.Generation))}
            >
              {l.deleteOne}
            </Button>
          </Group>
          {c.Entries.map((e, i) => (
            <p key={i}>
              <strong>{e.Role}</strong>: {e.Text}
            </p>
          ))}
        </section>
      ))}
    </Stack>
  );
}

export function OpenClaw({
  onSignedOut,
}: {
  onSignedOut: (e: unknown) => void;
}) {
  const [v, setV] = useState<View>(),
    [rev, setRev] = useState(0),
    [tab, setTab] = useState("connection"),
    [busy, setBusy] = useState(false),
    [error, setError] = useState(""),
    [saved, setSaved] = useState(false);
  const update = (next: View) => {
    setV(next);
    setRev((n) => n + 1);
  };
  const run: Request = async <T,>(
    path: string,
    method = "GET",
    body?: unknown,
  ) => {
    setBusy(true);
    setError("");
    if (method !== "GET") setSaved(false);
    try {
      const out = await openclawRequest<T>(path, method, body);
      if (method !== "GET") setSaved(true);
      return out;
    } catch (e) {
      if (e instanceof SignedOut) onSignedOut(e);
      else {
        let code = "";
        if (e instanceof HTTPError) {
          try {
            code = JSON.parse(e.message).error;
          } catch {}
        }
        const description = code ? v?.labels["error." + code] : undefined;
        setError(
          description ??
            (v?.labels.error ??
              "Configuration unavailable. Refresh and check the connection.") +
              (code ? " (" + code + ")" : ""),
        );
      }
      throw e;
    } finally {
      setBusy(false);
    }
  };
  const reload = async () => {
    update(await run<View>(""));
  };
  useEffect(() => {
    void reload().catch(() => {});
  }, []);
  useEffect(() => {
    if (!["connection", "permissions", "diagnostics"].includes(tab)) return;
    let disposed = false;
    const tick = async () => {
      try {
        const live = await openclawRequest<View>("/status");
        if (!disposed && live.connection)
          setV((current) =>
            current
              ? {
                  ...current,
                  connection: {
                    ...current.connection,
                    ui: live.connection.ui,
                    node: live.connection.node,
                  },
                  grantedScopes: live.grantedScopes,
                  events: live.events,
                }
              : current,
          );
      } catch (e) {
        if (!disposed && e instanceof SignedOut) onSignedOut(e);
      }
    };
    const timer = window.setInterval(() => {
      void tick();
    }, 3000);
    return () => {
      disposed = true;
      window.clearInterval(timer);
    };
  }, [tab, onSignedOut]);
  if (!v)
    return (
      <Stack>
        <Title order={2}>OpenClaw</Title>
        {error ? <Alert color="red">{error}</Alert> : <Loader />}
        <Button onClick={() => void reload().catch(() => {})}>Refresh</Button>
      </Stack>
    );
  const l = v.labels;
  return (
    <Stack className="claw-config">
      <Group justify="space-between">
        <Title order={2}>{l.title}</Title>
        <Button
          disabled={busy}
          variant="light"
          onClick={() => void reload().catch(() => {})}
        >
          {l.refresh}
        </Button>
      </Group>
      {error && (
        <Alert color="red" role="alert">
          {error}
        </Alert>
      )}
      {saved && (
        <Alert color="green" role="status">
          {l.saved}
        </Alert>
      )}
      <Group>
        {[
          "connection",
          "permissions",
          "assistant1",
          "assistant2",
          "diagnostics",
          "history",
        ].map((t) => (
          <Button
            key={t}
            variant={tab === t ? "filled" : "light"}
            onClick={() => setTab(t)}
          >
            {t.startsWith("assistant") ? l.assistant + " " + t.slice(-1) : l[t]}
          </Button>
        ))}
      </Group>
      <fieldset disabled={busy} className="claw-options" key={rev}>
        {tab === "connection" && (
          <ConnectionForm
            initial={v.connection}
            l={l}
            run={run}
            update={update}
          />
        )}
        {tab === "permissions" && (
          <PermissionsForm v={v} run={run} update={update} />
        )}
        {tab.startsWith("assistant") && (
          <SlotForm
            key={tab}
            v={v}
            index={Number(tab.slice(-1)) - 1}
            run={run}
            reload={reload}
          />
        )}
        {tab === "history" && <History run={run} l={l} />}
        {tab === "diagnostics" && (
          <Stack>
            <Button
              onClick={() =>
                attempt(async () =>
                  update(
                    await run<View>("/actions", "POST", {
                      action: "clear_events",
                    }),
                  ),
                )
              }
            >
              {l.clearEvents}
            </Button>
            {Object.entries(v.events).map(([role, events]) => (
              <section key={role}>
                <Title order={3}>
                  {role === "ui" ? "Controller UI" : "Node"}
                </Title>
                <ul>
                  {[...(events ?? [])].reverse().map((e, i) => (
                    <li key={i}>
                      {new Date(e.At).toLocaleTimeString()} · {e.Direction} ·{" "}
                      {e.Type} · {e.Result}
                    </li>
                  ))}
                </ul>
              </section>
            ))}
          </Stack>
        )}
      </fieldset>
    </Stack>
  );
}
