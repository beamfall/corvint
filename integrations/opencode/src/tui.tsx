import { Plugin } from "@opencode/plugin/tui"
import { createEffect, createMemo, createSignal, For, onCleanup, Show } from "solid-js"
import { createHash } from "node:crypto"
import { INSPECTOR_RPC, emptySnapshot, visibleText } from "./inspector.js"

const PANEL = "corvint.context"
const sessionKey = id => createHash("sha256").update(id).digest("hex")

export default Plugin.define({
  id: "corvint.inspector.ui",
  setup(ctx) {
    const rpc = ctx.client.rpc(INSPECTOR_RPC)
    const lifetime = new AbortController()
    const location = () => ctx.location ?? ctx.data.location.default()
    const options = signal => ({ location: location(), signal })

    function useSnapshot(sessionID) {
      const [snapshot, setSnapshot] = createSignal(emptySnapshot())
      let request
      let serial = 0
      const refresh = async () => {
        const id = sessionID()
        const generation = ++serial
        request?.abort()
        request = new AbortController()
        if (!id) { setSnapshot(emptySnapshot()); return }
        try {
          const value = await rpc.snapshot({ sessionID: id }, options(AbortSignal.any([request.signal, lifetime.signal])))
          if (generation === serial && !request.signal.aborted) setSnapshot(value)
        } catch {
          if (generation === serial && !request.signal.aborted) setSnapshot(emptySnapshot("unavailable", "Corvint is unavailable in this location. Check the server plugin configuration."))
        }
      }
      createEffect(() => { sessionID(); setSnapshot(emptySnapshot()); void refresh() })
      const unsubscribe = rpc.events.on("updated", event => {
        const here = location()
        if (sessionID() && event.location?.directory === here.directory && event.data.sessionIdSha256 === sessionKey(sessionID())) void refresh()
      }, { signal: lifetime.signal })
      onCleanup(() => { serial++; request?.abort(); unsubscribe() })
      return snapshot
    }

    async function query(sessionID, task, signal = lifetime.signal) {
      if (!task?.trim()) return
      try { await rpc.query({ sessionID, task }, options(signal)) }
      catch { ctx.ui.toast.show({ title: "Corvint", message: "Context request unavailable.", variant: "error" }) }
    }

    function Sidebar(props) {
      const snapshot = useSnapshot(() => props.sessionID)
      return <box flexDirection="column" marginTop={1} onMouseDown={() => ctx.ui.panel.open(PANEL)}>
        <text fg={ctx.theme.text.base}><b>Corvint context</b></text>
        <text fg={ctx.theme.text.muted}>{() => snapshot().state === "ready" ? `${snapshot().rows.length} evidence locations` : snapshot().state}</text>
        <Show when={snapshot().revision}><text fg={ctx.theme.text.muted}>{() => `${snapshot().revision.slice(0, 10)} · observed ${snapshot().freshness}`}</text></Show>
        <Show when={snapshot().gaps.length}><text fg={ctx.theme.text.muted}>{() => `${snapshot().gaps.length} gaps / limitations`}</text></Show>
        <text fg={ctx.theme.text.base}>Inspect · /corvint</text>
      </box>
    }

    function Inspector(props) {
      const snapshot = useSnapshot(() => props.panel.sessionID)
      const [selected, setSelected] = createSignal(0)
      const [source, setSource] = createSignal("")
      const [sourceState, setSourceState] = createSignal("idle")
      const [showGaps, setShowGaps] = createSignal(false)
      const evidence = createMemo(() => snapshot().rows[selected()])
      let request
      let generation = 0
      const resetSource = () => { generation++; request?.abort(); setSource(""); setSourceState("idle") }
      createEffect(() => { snapshot(); setSelected(0); resetSource() })
      onCleanup(resetSource)
      const select = index => { resetSource(); setSelected(Math.max(0, Math.min(snapshot().rows.length - 1, index))) }
      const expand = async () => {
        const row = evidence()
        if (!row?.handle || snapshot().state !== "ready") return
        resetSource()
        const version = generation
        request = new AbortController()
        setSourceState("loading")
        try {
          const result = await rpc.expand({ sessionID: props.panel.sessionID, receiptId: snapshot().receiptId, handle: row.handle }, options(AbortSignal.any([request.signal, lifetime.signal])))
          if (version !== generation || request.signal.aborted) return
          setSource(result.text); setSourceState(result.state)
        } catch {
          if (version === generation && !request.signal.aborted) { setSource("Evidence expansion is unavailable."); setSourceState("unavailable") }
        }
      }
      const ask = async () => {
        const task = await ctx.ui.dialog.prompt({ title: "Corvint context", description: "Describe the task or name a file or requirement.", placeholder: "Locate the parser's governing requirements" })
        await query(props.panel.sessionID, task)
      }
      ctx.keymap.layer(() => ({
        commands: [
          { bind: "up", run: () => select(selected() - 1) },
          { bind: "down", run: () => select(selected() + 1) },
          { bind: "return", run: expand },
          { bind: "g", run: () => setShowGaps(x => !x) },
          { bind: "r", run: ask },
          { bind: "f", run: props.panel.toggleFullscreen },
          { bind: "escape", run: props.panel.close },
        ],
      }))
      const windowRows = () => snapshot().rows.slice(Math.max(0, selected() - 2), Math.max(0, selected() - 2) + 5)
      return <box flexDirection="column" flexGrow={1} padding={1} gap={1}>
        <box flexDirection="row" justifyContent="space-between">
          <text fg={ctx.theme.text.base}><b>Corvint · Context</b></text>
          <text fg={ctx.theme.text.muted} onMouseDown={props.panel.close}>esc close</text>
        </box>
        <text fg={ctx.theme.text.muted}>{() => snapshot().revision ? `${snapshot().revision.slice(0, 12)} · observed ${snapshot().freshness} · FALLBACK` : "Session evidence · FALLBACK"}</text>
        <Show when={snapshot().reason}><text fg={ctx.theme.text.base}>{() => visibleText(snapshot().reason)}</text></Show>
        <Show when={snapshot().rows.length} fallback={<text fg={ctx.theme.text.muted}>Press r to request context. No evidence has been collected for this session.</text>}>
          <box flexDirection="column">
            <text fg={ctx.theme.text.muted}>{() => `Evidence ${selected() + 1}/${snapshot().rows.length} · ↑↓ select · enter open`}</text>
            <For each={windowRows()}>{row => <text fg={row === evidence() ? ctx.theme.text.base : ctx.theme.text.muted} onMouseDown={() => select(snapshot().rows.indexOf(row))}>{() => `${row === evidence() ? "›" : " "} ${row.path}${row.line ? `:${row.line}` : ""}`}</text>}</For>
          </box>
          <Show when={evidence()}>{row => <>
            <text fg={ctx.theme.text.base}><b>{() => row().title}</b></text>
            <text fg={ctx.theme.text.muted}>{() => `${row().kind} · ${row().authority} · ${row().confidence}`}</text>
            <text fg={ctx.theme.text.base}>{() => `Why included: ${row().reason || "No reason supplied."}`}</text>
            <Show when={!row().handle}><text fg={ctx.theme.text.muted}>Exact expansion was not supplied for this location.</text></Show>
          </>}</Show>
        </Show>
        <scrollbox flexGrow={1} minHeight={3}>
          <Show when={showGaps()} fallback={<>
            <Show when={sourceState() === "loading"}><text fg={ctx.theme.text.muted}>Opening pinned source…</text></Show>
            <Show when={source()} fallback={<text fg={ctx.theme.text.muted}>{() => evidence()?.summary || "Evidence details will appear here."}</text>}>
              <text fg={ctx.theme.text.muted}>{() => sourceState() === "ready" ? `Pinned source · blob ${evidence()?.blob}` : "Source unavailable"}</text>
              <text fg={ctx.theme.text.base}>{() => source().split("\n").map(line => visibleText(line, 24000)).join("\n")}</text>
            </Show>
          </>}>
            <text fg={ctx.theme.text.base}><b>Gaps and limitations</b></text>
            <text fg={ctx.theme.text.muted}>{() => `Tree: ${snapshot().revision || "unknown"}`}</text>
            <text fg={ctx.theme.text.muted}>{() => `Receipt: ${snapshot().receiptId || "unavailable"}`}</text>
            <For each={snapshot().gaps}>{gap => <text fg={ctx.theme.text.muted}>{() => `• ${gap}`}</text>}</For>
            <text fg={ctx.theme.text.muted}>Evidence inclusion does not establish correctness or completion.</text>
          </Show>
        </scrollbox>
        <text fg={ctx.theme.text.muted}>{() => `g gaps (${snapshot().gaps.length}) · r query · f expand view`}</text>
      </box>
    }

    const dispose = [
      ctx.ui.slot({ append: "sidebar.content", render: props => <Sidebar {...props} /> }),
      ctx.ui.slot({ append: "session.panel", render: panel => <Show when={panel.name === PANEL}><Inspector panel={panel} /></Show> }),
      ctx.ui.slot({ append: "app", render: () => {
        ctx.keymap.layer(() => ({ mode: "global", commands: [{ id: "corvint.context", title: "Corvint: inspect context", group: "Corvint", palette: true, slash: { name: "corvint", arguments: true }, run: async task => {
          if (!ctx.ui.panel.open(PANEL)) { ctx.ui.toast.show({ message: "Open a session to inspect its context.", variant: "info" }); return }
          const sessionID = ctx.ui.panel.current()?.sessionID
          if (sessionID && task?.trim()) await query(sessionID, task)
        } }] }))
        return null
      } }),
    ]
    return () => { lifetime.abort(); for (const stop of dispose) stop() }
  },
})
