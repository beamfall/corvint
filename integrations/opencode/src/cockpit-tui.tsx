import { createCockpitClient } from "./cockpit-client.js"
import { createEffect, createMemo, createSignal, For, onCleanup, Show, untrack } from "solid-js"
import { createHash } from "node:crypto"
import { emptyCockpit } from "./cockpit.js"
import { changePresentation, checkPresentation, changeEmpty, changeSections } from "./ui-presentation.js"

export function createCockpit(ctx, { rpc, lifetime, location, accent, openEvidence, openTasks }) {
  return function Cockpit(props) {
    const [snapshot, setSnapshot] = createSignal(emptyCockpit())
    const [section, setSection] = createSignal("files")
    const [selected, setSelected] = createSignal(0)
    const [selectedFile, setSelectedFile] = createSignal("")
    const [detail, setDetail] = createSignal(false)
    const [dialog, setDialog] = createSignal(false)
    const [proof, setProof] = createSignal("")
    const controller = new AbortController()
    let proofRequest, reader, list, proofSerial = 0, disposed = false
    const scope = () => `${props.panel.sessionID}\0${location().directory}`
    const wide = () => props.panel.width >= 96
    const presentation = createMemo(() => changePresentation(snapshot()))
    const items = createMemo(() => {
      const value = snapshot()
      if (section() === "files") return value.files.map(path => ({ label: path, path, sub: "Enter → affected tests" }))
      if (section() === "impact") return value.impacts.filter(row => !selectedFile() || row.path === selectedFile()).map(row => ({ ...row, label: row.unit, sub: `${row.tests.length} tests · ${row.kind}` }))
      if (section() === "proof") return value.checks.map(row => ({ ...row, ...checkPresentation(row, value.state) }))
      return presentation().attention.map((gap, i) => ({ label: gap, sub: `Attention ${i + 1}` }))
    })
    const current = () => items()[Math.min(selected(), Math.max(0, items().length - 1))]
    const clearProof = () => { proofSerial++; proofRequest?.abort(); setProof("") }
    const apply = value => {
      if (value.receiptId !== snapshot().receiptId || value.state !== "ready") { clearProof(); setSelected(0); setDetail(false) }
      setSnapshot(value)
    }
    const client = createCockpitClient({ rpc, signal: AbortSignal.any([controller.signal, lifetime.signal]), publish: apply })
    const readScope = () => ({ sessionID: props.panel.sessionID, location: location() })
    const refresh = () => client.refresh(readScope())
    createEffect(() => { scope(); untrack(() => { setSelectedFile(""); setSection("files"); void client.refresh(readScope(), "") }) })
    const unsubscribe = rpc.events.on("updated", event => {
      if (event.location?.directory === location().directory && event.data.sessionIdSha256 === createHash("sha256").update(props.panel.sessionID).digest("hex")) void client.observe()
    }, { signal: lifetime.signal })
    onCleanup(() => { disposed = true; controller.abort(); client.cancel(); clearProof(); unsubscribe() })
    const choose = name => { clearProof(); setSection(name); setSelected(0); setDetail(false) }
    const open = async () => {
      const row = current()
      if (!row) return
      if (section() === "files") { setSelectedFile(row.path); choose("impact"); return }
      setDetail(true)
      if (section() !== "proof") return
      clearProof(); const generation = proofSerial, receiptId = snapshot().receiptId, captured = scope()
      proofRequest = new AbortController(); setProof("Reading recorded output…")
      try {
        const value = await rpc.cockpitProof({ sessionID: props.panel.sessionID, receiptId, checkID: row.id }, { location: location(), signal: AbortSignal.any([proofRequest.signal, controller.signal, lifetime.signal]) })
        if (!disposed && generation === proofSerial && captured === scope() && snapshot().receiptId === receiptId) setProof(value.text)
      } catch { if (!disposed && generation === proofSerial) setProof("Recorded output unavailable. Refresh and retry.") }
    }
    const chooseBase = async () => {
      const captured = scope(); setDialog(true)
      try {
        const value = await ctx.ui.dialog.prompt({ title: "Compare from revision", description: "Enter a branch, tag or commit. It will be pinned to one commit. HEAD shows working-tree changes.", placeholder: "origin/main" })
        if (!disposed && captured === scope() && value?.trim()) { await client.refresh(readScope(), value.trim()) }
      } finally { if (!disposed) setDialog(false) }
    }
    const context = async () => {
      const captured = scope(), file = current()?.path || selectedFile()
      setDialog(true)
      try { await openEvidence(file || "", controller.signal, () => !disposed && captured === scope()) }
      finally { if (!disposed) setDialog(false) }
    }
    const move = amount => {
      if (detail()) { reader?.scrollBy(amount); return }
      clearProof(); setSelected(Math.max(0, Math.min(items().length - 1, selected() + amount)))
      list?.scrollChildIntoView(`corvint-change-row-${selected()}`)
    }
    ctx.keymap.layer(() => ({ enabled: () => props.panel.focused && !dialog(), commands: [
      ...changeSections.map(({ key }, i) => ({ bind: String(i + 1), run: () => choose(key) })),
      { bind: "up", run: () => move(-1) }, { bind: "down", run: () => move(1) },
      { bind: "pageup", run: () => detail() ? reader?.scrollBy(-1, "viewport") : move(-8) },
      { bind: "pagedown", run: () => detail() ? reader?.scrollBy(1, "viewport") : move(8) },
      { bind: "return", run: open }, { bind: "tab", run: () => setDetail(value => !value) },
      { bind: "x", run: () => { setSelectedFile(""); choose("impact") } },
      { bind: "r", run: refresh }, { bind: "b", run: chooseBase }, { bind: "e", run: context }, { bind: "t", run: openTasks },
      { bind: "f", run: props.panel.toggleFullscreen }, { bind: "escape", run: props.panel.close },
    ] }))
    const description = () => {
      const row = current(), value = snapshot()
      if (section() === "gaps") return `${row?.label || changeEmpty(value, "gaps")}\n\nScope limitations\n\n${presentation().limitations.join("\n\n")}`
      if (!row && section() !== "files") return changeEmpty(value, section(), !!selectedFile())
      if (section() === "proof") return proof() || (row ? `${row.label}\n${row.reason}\n\nCommand (not executed):\n${row.argv.join(" ")}\n\nTested: ${row.testedCommit || "not run"}\nCurrent: ${row.currentTarget}\n\nEnter Read recorded output\n\n${value.workflow}` : "No recorded check results. Declared and suggested checks have not been executed by this UI.")
      if (section() === "impact") return row ? `${row.path}\n\nWhy selected: ${row.kind}\n\nDependency path\n${row.via.map((x, i) => `${i + 1}. ${x}`).join("\n")}\n\nAffected tests\n${row.tests.join("\n") || "No test paths supplied"}\n\nSelection is advisory; these tests have not been run by this view.\n\ne Inspect file context and governing evidence` : "No dependency witness supplied for this selection. This does not establish that the file is unaffected.\n\nx Show all impact witnesses"
      return `${row?.path || changeEmpty(value, "files")}\n\nEnter Follow affected-test witnesses\ne Inspect file context and governing evidence\n\nRecorded intent scopes\n${value.intents.join("\n") || "No intent scope supplied by a verification workflow."}\n\nDeclared / suggested checks (not executed)\n${value.declarations.map(x => `${x.kind}: ${x.command}\n  ${x.source}: ${x.reason}`).join("\n\n") || "None supplied"}`
    }
    return <box flexDirection="column" flexGrow={1} minHeight={0} padding={1}>
      <box flexDirection="row" justifyContent="space-between" flexShrink={0}><text fg={ctx.theme.text.base}><b>{() => `Corvint · ${presentation().heading}`}</b></text><box flexDirection="row" gap={2}><text fg={accent()} onMouseDown={openTasks}>t Work queue</text><text fg={accent()} onMouseDown={context}>e Evidence</text></box></box>
      <text height={1} flexShrink={0} fg={snapshot().state === "ready" ? ctx.theme.text.feedback.info.base : ctx.theme.text.feedback.warning.base}>{() => presentation().summary}</text>
      <text height={1} flexShrink={0} fg={ctx.theme.text.muted} onMouseDown={chooseBase}>{() => `Base ${snapshot().base.slice(0, 8) || "—"} → ${snapshot().target.slice(0, 8) || "HEAD"} + worktree   b compare`}</text>
      <text height={1} flexShrink={0} fg={ctx.theme.text.muted}>{() => snapshot().workflow}</text>
      <box flexDirection="row" gap={2} marginY={1} flexShrink={0}><For each={changeSections}>{(item, index) => <text fg={section() === item.key ? accent() : ctx.theme.text.muted} onMouseDown={() => { props.panel.focus(); choose(item.key) }}>{() => `${index() + 1} ${item.key === section() ? "● " : ""}${item.label}`}</text>}</For></box>
      <Show when={section() === "impact" && selectedFile()}><text height={1} flexShrink={0} fg={accent()} onMouseDown={() => { setSelectedFile(""); choose("impact") }}>{() => `${selectedFile()} · x all`}</text></Show>
      <box flexDirection="row" flexGrow={1} minHeight={0} gap={2}>
        <Show when={wide() || !detail()}><scrollbox ref={value => { list = value }} width={wide() ? 38 : "100%"} flexGrow={wide() ? 0 : 1} minHeight={0}>
          <Show when={items().length} fallback={<text fg={ctx.theme.text.muted}>{() => changeEmpty(snapshot(), section(), !!selectedFile())}</text>}><For each={items()}>{(row, index) => {
            const color = row.tone === "info" ? ctx.theme.text.feedback.info.base : row.tone === "error" ? ctx.theme.text.feedback.error.base : row.tone === "warning" ? ctx.theme.text.feedback.warning.base : index() === selected() ? accent() : ctx.theme.text.base
            const click = event => { event.stopPropagation(); props.panel.focus(); clearProof(); setSelected(index()); void open() }
            return <box id={`corvint-change-row-${index()}`} flexDirection="column" paddingX={1} paddingY={1} backgroundColor={index() === selected() ? ctx.theme.background.raised.high : undefined} onMouseDown={click}><text fg={color} onMouseDown={click}>{row.label}</text><text fg={ctx.theme.text.muted} onMouseDown={click}>{row.sub}</text></box>
          }}</For></Show>
        </scrollbox></Show>
        <Show when={wide() || detail()}><scrollbox ref={value => { reader = value }} flexGrow={1} minWidth={0} minHeight={0}><text fg={ctx.theme.text.base}>{description}</text></scrollbox></Show>
      </box>
      <text height={1} flexShrink={0} fg={ctx.theme.text.muted} onMouseDown={() => { props.panel.focus(); choose("gaps"); setDetail(true) }}>4 Attention · Tab details includes scope limitations</text>
      <text height={1} flexShrink={0} fg={ctx.theme.text.muted}>{() => snapshot().observed ? `Observed ${snapshot().observed.slice(11, 19)} UTC · explicit refresh · ${snapshot().scope}` : "Read-only · no test execution"}</text>
      <text height={1} flexShrink={0} fg={ctx.theme.text.base}>↑↓ select/scroll  Enter open  Tab list/details</text>
      <text height={1} flexShrink={0} fg={ctx.theme.text.muted}>r refresh  b base  t tasks  e evidence  f full  esc close</text>
    </box>
  }
}
