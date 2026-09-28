import { createCockpit } from "./cockpit-tui.tsx"
import { Plugin } from "@opencode/plugin/tui"
import { SyntaxStyle } from "@opentui/core"
import { createEffect, createMemo, createSignal, For, onCleanup, Show, untrack } from "solid-js"
import { createHash } from "node:crypto"
import { INSPECTOR_RPC, emptySnapshot, visibleText } from "./inspector.js"
import { evidenceKey, filterEvidence, selectedEvidence, evidenceStatus, sourceDocument } from "./inspector-view.js"

const PANEL = "corvint.context"
const sessionKey = id => createHash("sha256").update(id).digest("hex")
const filename = path => path.split("/").at(-1)
const directory = path => path.includes("/") ? path.slice(0, path.lastIndexOf("/")) : "Repository root"

export default Plugin.define({
  id: "corvint.inspector.ui",
  setup(ctx) {
    const [mode, setMode] = createSignal("change")
    const rpc = ctx.client.rpc(INSPECTOR_RPC)
    const lifetime = new AbortController()
    const location = () => ctx.location ?? ctx.data.location.default()
    const syntax = SyntaxStyle.fromStyles({ default: { fg: ctx.theme.text.base }, ...Object.fromEntries(Object.entries(ctx.theme.syntax).map(([key, fg]) => [key, { fg }])) })
    const accent = () => ctx.theme.text.action.primary.base
    const tone = snapshot => ctx.theme.text.feedback[evidenceStatus(snapshot).tone].base

    function useSnapshot(sessionID) {
      const [snapshot, setSnapshot] = createSignal(emptySnapshot())
      let request, serial = 0
      const refresh = async () => {
        const id = sessionID(), here = location(), generation = ++serial
        request?.abort(); request = new AbortController()
        if (!id) { setSnapshot(emptySnapshot()); return }
        try {
          const value = await rpc.snapshot({ sessionID: id }, { location: here, signal: AbortSignal.any([request.signal, lifetime.signal]) })
          if (generation === serial && !request.signal.aborted) setSnapshot(value)
        } catch {
          if (generation === serial && !request.signal.aborted) setSnapshot(emptySnapshot("unavailable", "Corvint is unavailable here. Check the server plugin configuration."))
        }
      }
      createEffect(() => { sessionID(); location().directory; setSnapshot(emptySnapshot()); void refresh() })
      const unsubscribe = rpc.events.on("updated", event => {
        if (sessionID() && event.location?.directory === location().directory && event.data.sessionIdSha256 === sessionKey(sessionID())) void refresh()
      }, { signal: lifetime.signal })
      onCleanup(() => { serial++; request?.abort(); unsubscribe() })
      return snapshot
    }

    async function query(sessionID, task, here, signal = lifetime.signal) {
      if (!task?.trim() || signal.aborted) return
      try { await rpc.query({ sessionID, task }, { location: here, signal }) }
      catch { if (!signal.aborted) ctx.ui.toast.show({ title: "Corvint", message: "Context request unavailable. Try again.", variant: "error" }) }
    }

    function Sidebar(props) {
      const snapshot = useSnapshot(() => props.sessionID)
      return <box flexDirection="column" marginTop={1} onMouseDown={() => { setMode("evidence"); ctx.ui.panel.open(PANEL) }}>
        <text fg={ctx.theme.text.base}><b>Corvint context</b></text>
        <text fg={tone(snapshot())}>{() => `${evidenceStatus(snapshot()).label} · ${snapshot().rows.length} locations`}</text>
        <Show when={snapshot().revision}><text fg={ctx.theme.text.muted}>{() => `Observed ${snapshot().freshness} · ${snapshot().revision.slice(0, 8)}`}</text></Show>
        <Show when={snapshot().gaps.length}><text fg={ctx.theme.text.feedback.warning.base}>{() => `${snapshot().gaps.length} gaps / limitations`}</text></Show>
        <text fg={accent()}>Browse evidence  /corvint</text>
      </box>
    }

    function EvidenceInspector(props) {
      const snapshot = useSnapshot(() => props.panel.sessionID)
      const [selected, setSelected] = createSignal("")
      const [filter, setFilter] = createSignal("")
      const [view, setView] = createSignal("evidence")
      const [source, setSource] = createSignal("")
      const [sourceState, setSourceState] = createSignal("idle")
      const [highlight, setHighlight] = createSignal(false)
      const [dialogOpen, setDialogOpen] = createSignal(false)
      const rows = createMemo(() => filterEvidence(snapshot().rows, filter()))
      const evidence = createMemo(() => selectedEvidence(rows(), selected()))
      const wide = () => props.panel.width >= 96
      const document = createMemo(() => sourceDocument(source(), evidence()?.path || "", evidence()?.line))
      const controller = new AbortController()
      let evidenceBinding = "", layoutCallback, request, list, reader, sourceHandle = "", generation = 0, previous = "", scope = "", disposed = false
      const resetSource = () => { if (layoutCallback) ctx.renderer.off("frame", layoutCallback); layoutCallback = undefined; generation++; request?.abort(); sourceHandle = ""; setSource(""); setSourceState("idle") }
      const currentScope = () => `${props.panel.sessionID}\0${location().directory}`
      createEffect(() => {
        const nextScope = currentScope(), value = snapshot()
        if (scope !== nextScope) { scope = nextScope; setSelected(""); setFilter(""); setView("evidence"); setHighlight(false) }
        const binding = `${scope}\0${value.receiptId}\0${value.state}`
        if (binding !== previous) { previous = binding; resetSource() }
        const next = selectedEvidence(value.rows, untrack(selected))
        if (next) setSelected(evidenceKey(next))
      })
      createEffect(() => {
        const row = evidence(), identity = row ? evidenceKey(row) : ""
        if (identity !== evidenceBinding) {
          evidenceBinding = identity
          if (row?.handle !== sourceHandle || untrack(sourceState) !== "ready") resetSource()
        }
      })
      createEffect(() => {
        syntax.registerStyle("default", { fg: ctx.theme.text.base })
        for (const [key, fg] of Object.entries(ctx.theme.syntax)) syntax.registerStyle(key, { fg })
      })
      onCleanup(() => { disposed = true; controller.abort(); resetSource() })
      const select = row => {
        if (!row) return
        if (row.handle !== evidence()?.handle) resetSource()
        setSelected(evidenceKey(row))
      }
      const move = delta => {
        const index = rows().indexOf(evidence())
        select(rows()[Math.max(0, Math.min(rows().length - 1, index + delta))])
      }
      createEffect(() => {
        const index = rows().indexOf(evidence())
        if (index >= 0 && (wide() || view() === "evidence")) queueMicrotask(() => { if (!disposed) list?.scrollChildIntoView(`corvint-row-${index}`) })
      })
      const cited = () => reader?.scrollTo({ x: 0, y: Math.max(0, (document().citation || 1) - 3) })
      const scheduleCitation = () => {
        if (layoutCallback) ctx.renderer.off("frame", layoutCallback)
        layoutCallback = () => { layoutCallback = undefined; if (!disposed) cited() }
        ctx.renderer.once("frame", layoutCallback)
        ctx.renderer.requestRender()
      }
      const expand = async () => {
        const row = evidence(), value = snapshot(), here = location(), sessionID = props.panel.sessionID
        if (!row?.handle || value.state !== "ready") return
        setView("source")
        if (sourceHandle === row.handle && sourceState() === "ready") { scheduleCitation(); return }
        resetSource()
        const version = generation
        request = new AbortController(); setSourceState("loading")
        try {
          const result = await rpc.expand({ sessionID, receiptId: value.receiptId, handle: row.handle }, { location: here, signal: AbortSignal.any([request.signal, controller.signal, lifetime.signal]) })
          if (version !== generation || request.signal.aborted || disposed) return
          sourceHandle = row.handle; setSource(result.text); setSourceState(result.state)
          scheduleCitation()
        } catch {
          if (version === generation && !disposed) { setSource("Evidence expansion is unavailable. Press Enter to retry."); setSourceState("unavailable") }
        }
      }
      const prompt = async searching => {
        const captured = currentScope(), sessionID = props.panel.sessionID, here = location()
        setDialogOpen(true)
        try {
          const value = await ctx.ui.dialog.prompt(searching
            ? { title: "Find evidence", description: "Match a filename, symbol, inclusion reason or authority. Space separates terms.", placeholder: "parser requirements" }
            : { title: "Request context", description: "Describe your task or name a file or requirement.", placeholder: "Locate the parser's governing requirements" })
          if (disposed || currentScope() !== captured || value == null) return
          if (searching) { setFilter(value.slice(0, 512)); setView("evidence") }
          else await query(sessionID, value, here, controller.signal)
        } finally { if (!disposed) setDialogOpen(false) }
      }
      const toggle = () => setView(view() === "evidence" ? "source" : "evidence")
      ctx.keymap.layer(() => ({ enabled: () => props.panel.focused && !dialogOpen(), commands: [
        { bind: "up", run: () => view() === "evidence" ? move(-1) : reader?.scrollBy(-1) },
        { bind: "down", run: () => view() === "evidence" ? move(1) : reader?.scrollBy(1) },
        { bind: "left", run: () => reader?.scrollBy({ x: -8, y: 0 }) },
        { bind: "right", run: () => reader?.scrollBy({ x: 8, y: 0 }) },
        { bind: "pageup", run: () => view() === "evidence" ? move(-8) : reader?.scrollBy(-1, "viewport") },
        { bind: "pagedown", run: () => view() === "evidence" ? move(8) : reader?.scrollBy(1, "viewport") },
        { bind: "return", run: expand }, { bind: "tab", run: toggle },
        { bind: "/", run: () => prompt(true) }, { bind: "x", run: () => { setFilter(""); setView("evidence") } },
        { bind: "g", run: () => setView(view() === "gaps" ? "evidence" : "gaps") },
        { bind: "i", run: () => setView(view() === "details" ? "evidence" : "details") },
        { bind: "h", run: () => setHighlight(value => !value) }, { bind: "l", run: cited }, { bind: "r", run: () => prompt(false) },
        { bind: "c", run: () => setMode("change") }, { bind: "f", run: props.panel.toggleFullscreen }, { bind: "escape", run: props.panel.close },
      ] }))

      const tab = (name, label) => <text fg={view() === name ? accent() : ctx.theme.text.muted} onMouseDown={() => { props.panel.focus(); setView(name) }}>{() => `${view() === name ? "● " : ""}${label}`}</text>
      function EvidenceList() {
        return <box flexDirection="column" width={wide() ? 34 : "100%"} flexGrow={wide() ? 0 : 1} flexShrink={0} minHeight={0}>
          <text fg={ctx.theme.text.muted} height={1} flexShrink={0}>{() => `${rows().length} of ${snapshot().rows.length} locations  / find`}</text>
          <Show when={filter()}><text height={1} flexShrink={0} fg={accent()} onMouseDown={() => setFilter("")}>{() => `Filter: ${visibleText(filter(), 80)}  × clear`}</text></Show>
          <scrollbox ref={value => { list = value }} flexGrow={1} minHeight={0}>
            <Show when={rows().length} fallback={<box paddingY={1} flexDirection="column"><text fg={ctx.theme.text.base}>{() => filter() ? "No matching evidence." : evidenceStatus(snapshot()).action}</text><text fg={accent()} onMouseDown={() => filter() ? setFilter("") : prompt(false)}>{() => filter() ? "x Clear filter" : "r Request context"}</text></box>}>
              <For each={rows()}>{(row, index) => {
                const open = event => { event?.stopPropagation(); props.panel.focus(); select(row); void expand() }
                return <box id={`corvint-row-${index()}`} flexDirection="column" paddingX={1} paddingY={1} backgroundColor={row === evidence() ? ctx.theme.background.raised.high : undefined} onMouseDown={open}>
                  <text onMouseDown={open} fg={row === evidence() ? accent() : ctx.theme.text.base}><b>{() => `${row === evidence() ? "› " : "  "}${filename(row.path)}${row.line ? `:${row.line}` : ""}`}</b></text>
                  <text onMouseDown={open} fg={ctx.theme.text.muted}>{() => directory(row.path)}</text>
                  <text onMouseDown={open} fg={ctx.theme.text.muted}>{() => row.title}</text>
                </box>
              }}</For>
            </Show>
          </scrollbox>
        </box>
      }
      function SourceCode() {
        const [gutter, setGutter] = createSignal()
        createEffect(() => {
          const target = gutter(), line = document().citation
          target?.setLineColors(new Map(line ? [[line - 1, { gutter: ctx.theme.background.feedback.info.base, content: ctx.theme.background.feedback.info.base }]] : []))
          target?.setLineSigns(new Map(line ? [[line - 1, { before: "›", beforeColor: accent() }]] : []))
        })
        return <line_number ref={setGutter} fg={ctx.theme.text.muted}><code onSizeChange={scheduleCitation} content={document().content} filetype={highlight() ? document().filetype : undefined} syntaxStyle={syntax} conceal={false} drawUnstyledText={true} wrapMode="none" /></line_number>
      }
      function Details() {
        return <box flexDirection="column" flexGrow={1} minWidth={0} minHeight={0}>
          <Show when={evidence()}>{row => <box flexDirection="column" flexShrink={0}>
            <text fg={ctx.theme.text.base}><b>{() => row().path}</b></text>
            <text fg={ctx.theme.text.muted}>{() => `${row().authority} · ${row().confidence}`}</text>
            <Show when={view() !== "gaps" && view() !== "details"}><text fg={ctx.theme.text.base}>{() => `Why included: ${visibleText(row().reason || "No reason supplied.", Math.max(30, props.panel.width - (wide() ? 48 : 12)))}`}</text></Show>
          </box>}</Show>
          <Show when={sourceState() === "ready" && view() !== "gaps" && view() !== "details"}><text fg={accent()} height={1} flexShrink={0}>{() => `Pinned source · ${document().lineCount} lines${document().citation ? ` · cited line ${document().citation}` : " · cited line unavailable"}`}</text></Show>
          <Show when={sourceState() === "ready" && view() !== "gaps" && view() !== "details"}><text height={1} flexShrink={0} fg={accent()} onMouseDown={() => setHighlight(value => !value)}>{() => !document().filetype ? "Plain text · syntax unavailable for this file" : highlight() ? "Syntax enabled · h plain text" : "h Enable syntax · host may download a parser"}</text></Show>
          <scrollbox ref={value => { reader = value }} flexGrow={1} minHeight={0} scrollX={true}>
            <Show when={view() === "gaps"} fallback={<Show when={view() === "details"} fallback={<>
              <Show when={sourceState() === "loading"}><text fg={accent()}>Opening pinned source…</text></Show>
              <Show when={sourceState() === "unavailable"}><text fg={ctx.theme.text.feedback.error.base}>{() => visibleText(source())}</text></Show>
              <Show when={sourceState() === "ready"} fallback={<Show when={sourceState() === "idle"}><text fg={ctx.theme.text.muted}>{() => snapshot().state !== "ready" ? "Request fresh context to open source." : evidence()?.handle ? "Enter Open pinned source" : "Exact expansion was not supplied for this location."}</text></Show>}>
                <SourceCode />
              </Show>
            </>}>
              <text fg={ctx.theme.text.base}><b>Evidence details</b></text>
              <text fg={ctx.theme.text.base}>{() => evidence()?.reason || "No inclusion reason supplied."}</text>
              <text fg={ctx.theme.text.muted}>{() => evidence()?.summary || ""}</text>
              <text fg={ctx.theme.text.muted}>{() => `Tree: ${snapshot().revision || "unknown"}\nBlob: ${evidence()?.blob || "unavailable"}\nReceipt: ${snapshot().receiptId || "unavailable"}\nObserved freshness: ${snapshot().freshness}\nIntegration receipt: FALLBACK · execution authority: NONE`}</text>
            </Show>}>
              <text fg={ctx.theme.text.base}><b>Gaps and limitations</b></text>
              <Show when={snapshot().gaps.length} fallback={<text fg={ctx.theme.text.muted}>No gaps reported in this receipt. This does not establish complete coverage.</text>}><For each={snapshot().gaps}>{gap => <box marginBottom={1}><text fg={ctx.theme.text.feedback.warning.base}>{() => `• ${gap}`}</text></box>}</For></Show>
              <text fg={ctx.theme.text.muted}>Evidence inclusion does not establish correctness or completion. Freshness describes the last observation.</text>
            </Show>
          </scrollbox>
        </box>
      }
      return <box flexDirection="column" flexGrow={1} minHeight={0} padding={1}>
        <box flexDirection="row" justifyContent="space-between" flexShrink={0}><text fg={ctx.theme.text.base}><b>Corvint · Evidence</b></text><text fg={accent()} onMouseDown={() => setMode("change")}>c Change</text></box>
        <box flexDirection="row" gap={2} flexShrink={0}><text fg={tone(snapshot())}>{() => evidenceStatus(snapshot()).label}</text><text fg={ctx.theme.text.muted}>{() => `Observed ${snapshot().freshness}`}</text><text fg={ctx.theme.text.feedback.warning.base} onMouseDown={() => setView("gaps")}>{() => `${snapshot().gaps.length} gaps`}</text></box>
        <Show when={snapshot().reason}><text fg={tone(snapshot())} maxHeight={2}>{() => visibleText(snapshot().reason)}</text></Show>
        <box flexDirection="row" gap={2} marginY={1} flexShrink={0}>{tab("evidence", "Evidence")}{tab("source", "Source")}{tab("gaps", "Gaps")}{tab("details", "Details")}</box>
        <box flexDirection="row" flexGrow={1} minHeight={0} gap={wide() ? 2 : 0}>
          <Show when={wide() || view() === "evidence"}><EvidenceList /></Show>
          <Show when={wide() || view() !== "evidence"}><Details /></Show>
        </box>
        <box flexDirection="column" marginTop={1} flexShrink={0}>
          <text fg={ctx.theme.text.muted}>{() => view() === "evidence" ? "↑↓ select · Enter open · / find · x clear" : "↑↓ scroll · PgUp/PgDn · l cited line"}</text>
          <box flexDirection="row" gap={1}><text fg={accent()} onMouseDown={toggle}>Tab list/source</text><text fg={accent()} onMouseDown={() => prompt(false)}>r query</text><text fg={accent()} onMouseDown={props.panel.toggleFullscreen}>f full</text><text fg={ctx.theme.text.muted}>g gaps · i details</text></box>
        </box>
      </box>
    }
    const Cockpit = createCockpit(ctx, { rpc, lifetime, location, accent, openEvidence: async (file, signal, current) => {
      if (file) await query(ctx.ui.panel.current()?.sessionID, `Locate governing requirements and context for ${file}`, location(), signal)
      if (current() && !signal.aborted) setMode("evidence")
    } })
    const Inspector = props => <Show when={mode() === "change"} fallback={<EvidenceInspector panel={props.panel} />}><Cockpit panel={props.panel} /></Show>

    const dispose = [
      ctx.ui.slot({ append: "sidebar.content", render: props => <Sidebar {...props} /> }),
      ctx.ui.slot({ append: "session.panel", render: panel => <Show when={panel.name === PANEL}><Inspector panel={panel} /></Show> }),
      ctx.ui.slot({ append: "app", render: () => {
        ctx.keymap.layer(() => ({ mode: "global", commands: [{ id: "corvint.context", title: "Corvint: inspect context", group: "Corvint", palette: true, slash: { name: "corvint", arguments: true }, run: async task => {
          setMode(task?.trim() ? "evidence" : "change")
          if (!ctx.ui.panel.open(PANEL)) { ctx.ui.toast.show({ message: "Open a session to inspect its context.", variant: "info" }); return }
          const sessionID = ctx.ui.panel.current()?.sessionID
          if (sessionID && task?.trim()) await query(sessionID, task, location())
        } }] }))
        return null
      } }),
    ]
    return () => { lifetime.abort(); for (const stop of dispose) stop(); syntax.destroy() }
  },
})
