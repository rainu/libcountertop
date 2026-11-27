import { useEffect, useRef, useState } from "react";
import { Alert, Button, Group, Paper, Stack, Text, Title } from "@mantine/core";
import { api, type MicrophoneTestState } from "./api";

export function MicrophoneTest() {
  const [state, setState] = useState<MicrophoneTestState>({ phase: "idle", seconds: 0, owned: false });
  const [error, setError] = useState("");
  const [pending, setPending] = useState(false);
  const owner = useRef("");
  if (!owner.current) owner.current = Array.from(crypto.getRandomValues(new Uint8Array(16)), n => n.toString(16).padStart(2, "0")).join("");
  useEffect(() => {
    let live = true, updating = false;
    const refresh = async () => {
      if (updating) return;
      updating = true;
      try {
        const s = await api.microphoneTest(owner.current);
        if (!live) return;
        setState(s);
        if (s.owned) await api.microphoneTestCommand(owner.current, "ping");
      } catch (e) { if (live) setError((e as Error).message); }
      finally { updating = false; }
    };
    const discard = () => { void api.microphoneTestCommand(owner.current, "discard", true).catch(() => {}); };
    void refresh();
    const timer = setInterval(() => void refresh(), 1000);
    window.addEventListener("pagehide", discard);
    return () => { live = false; clearInterval(timer); window.removeEventListener("pagehide", discard); discard(); };
  }, []);
  const command = async (action: string) => {
    setPending(true); setError("");
    try { setState(await api.microphoneTestCommand(owner.current, action)); }
    catch (e) { setError((e as Error).message); }
    finally { setPending(false); }
  };
  const active = state.phase === "recording" || state.phase === "playing";
  const labels: Record<string, string> = { idle: "Ready to record", recording: "● RECORDING — speak now (max 8 s)", ready: "Ready to play", playing: "Playing on Echo", error: "Test stopped" };
  return <Paper withBorder p="md" data-testid="microphone-test"><Stack gap="sm">
    <Title order={4}>Microphone test</Title>
    <Text size="sm">Processed recognition signal: microphone mixing, echo cancellation, suppression, low-cut and leveling. Change the settings below between takes. No conversation is started.</Text>
    <Text size="sm">Audio stays in this Echo’s memory, never uploaded or saved. It is deleted when you leave this section, after a lost connection, or after 2 minutes. Playback starts only after capture stops; wake words are paused while recording or playing.</Text>
    <Text role="status" fw={700} c={state.phase === "recording" ? "red" : undefined}>{labels[state.phase] ?? state.phase} · {state.seconds.toFixed(1)} s</Text>
    {state.processing && <Text size="xs">Take settings: {state.processing}</Text>}
    {(error || state.error) && <Alert color="red" role="alert">{error || state.error}</Alert>}
    {active && !state.owned && <Alert>Another microphone test is active. Stop it on the page or device that started it.</Alert>}
    <Group>
      <Button disabled={pending || active} onClick={() => void command("record")}>Record / replace take (8 s)</Button>
      <Button variant="light" disabled={pending || !state.owned || !active} onClick={() => void command("stop")}>Stop</Button>
      <Button variant="light" disabled={pending || !state.owned || state.phase !== "ready"} onClick={() => void command("play")}>Play on Echo</Button>
      <Button variant="subtle" disabled={pending || !state.owned} onClick={() => void command("discard")}>Delete take</Button>
    </Group>
  </Stack></Paper>;
}
