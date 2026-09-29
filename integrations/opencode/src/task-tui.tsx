import { createEffect, createSignal, For, onCleanup, Show, untrack } from "solid-js"
import { createHash } from "node:crypto"
import { emptyTaskMetrics } from "./task-metrics.js"

export function createTaskPanel(ctx, { rpc, lifetime, location, accent, onChange }) {
  return function TaskPanel(props) {
    const [snapshot, setSnapshot] = createSignal(emptyTaskMetrics())
    const [selected, setSelected] = createSignal(0)
    const [detail, setDetail] = createSignal("")
    const [detailPane, setDetailPane] = createSignal(false)
    const controller = new AbortController()
    let request, detailRequest, serial = 0, detailSerial = 0, busy = false, disposed = false, list, reader
    const scope = () => `${props.panel.sessionID}\0${location().directory}`
    const wide = () => props.panel.width >= 96
    const current = () => snapshot().tickets[Math.min(selected(), Math.max(0, snapshot().tickets.length - 1))]
    const clearDetail = () => { detailSerial++; detailRequest?.abort(); setDetail(""); setDetailPane(false) }
    const apply = value => {
      if (value.receiptId !== snapshot().receiptId || value.state !== "ready") { clearDetail(); setSelected(0) }
      setSnapshot(value)
    }
    const observe = async (loadIfEmpty = false) => {
      const generation = ++serial, captured = scope(), here = location()
      request?.abort(); request = new AbortController()
      busy = true
      try {
        let value = await rpc.tasksSnapshot({ sessionID: props.panel.sessionID }, { location: here, signal: AbortSignal.any([controller.signal, request.signal, lifetime.signal]) })
        if (loadIfEmpty && value.state === "empty") value = await rpc.tasksRefresh({ sessionID: props.panel.sessionID, offset: 0 }, { location: here, signal: AbortSignal.any([controller.signal, request.signal, lifetime.signal]) })
        if (!disposed && generation === serial && captured === scope()) apply(value)
      } catch { if (!disposed && generation === serial) apply(emptyTaskMetrics("unavailable", "Tasks read unavailable. Press r to retry.")) }
      finally { if (generation === serial) busy = false }
    }
    const refresh = async offset => {
      const generation = ++serial, captured = scope(), here = location()
      request?.abort(); request = new AbortController(); clearDetail()
      busy = true
      apply(emptyTaskMetrics("loading", "Reading Corvint Tasks…"))
      try {
        const value = await rpc.tasksRefresh({ sessionID: props.panel.sessionID, offset }, { location: here, signal: AbortSignal.any([controller.signal, request.signal, lifetime.signal]) })
        if (!disposed && generation === serial && captured === scope()) apply(value)
      } catch { if (!disposed && generation === serial) apply(emptyTaskMetrics("unavailable", "Tasks read unavailable. Press r to retry.")) }
      finally { if (generation === serial) busy = false }
    }
    createEffect(() => { scope(); untrack(() => { clearDetail(); setSelected(0); void observe(true) }) })
    const unsubscribe = rpc.events.on("updated", event => {
      if (!busy && event.location?.directory === location().directory && event.data.sessionIdSha256 === createHash("sha256").update(props.panel.sessionID).digest("hex")) void observe()
    }, { signal: lifetime.signal })
    onCleanup(() => { disposed = true; controller.abort(); request?.abort(); clearDetail(); unsubscribe() })
    const open = async () => {
      const row = current(), value = snapshot(), captured = scope()
      if (!row || value.state !== "ready") return
      detailRequest?.abort(); detailRequest = new AbortController()
      const detailGeneration = ++detailSerial, readRequest = detailRequest
      setDetailPane(true); setDetail("Reading ticket detail…")
      try {
        const result = await rpc.tasksDetail({ sessionID: props.panel.sessionID, receiptId: value.receiptId, ticketId: row.ticketId }, { location: location(), signal: AbortSignal.any([readRequest.signal, controller.signal, lifetime.signal]) })
        if (!disposed && detailGeneration === detailSerial && !readRequest.signal.aborted && captured === scope() && snapshot().receiptId === value.receiptId && current()?.ticketId === row.ticketId) setDetail(result.text)
      } catch { if (!disposed && detailGeneration === detailSerial && !readRequest.signal.aborted && captured === scope() && snapshot().receiptId === value.receiptId) setDetail("Ticket detail unavailable. Refresh and retry.") }
    }
    const move = amount => {
      if (detailPane()) { reader?.scrollBy(amount); return }
      clearDetail(); setSelected(Math.max(0, Math.min(snapshot().tickets.length - 1, selected() + amount)))
      list?.scrollChildIntoView(`corvint-task-row-${selected()}`)
    }
    const nextPage = delta => {
      const next = snapshot().offset + 32 * delta
      if (snapshot().state === "ready" && next >= 0 && next < snapshot().openTotal) void refresh(next)
    }
    ctx.keymap.layer(() => ({ enabled: () => props.panel.focused, commands: [
      { bind: "up", run: () => move(-1) }, { bind: "down", run: () => move(1) },
      { bind: "pageup", run: () => detailPane() ? reader?.scrollBy(-1, "viewport") : move(-8) },
      { bind: "pagedown", run: () => detailPane() ? reader?.scrollBy(1, "viewport") : move(8) },
      { bind: "return", run: open }, { bind: "tab", run: () => setDetailPane(value => !value) },
      { bind: "left", run: () => nextPage(-1) }, { bind: "right", run: () => nextPage(1) },
      { bind: "r", run: () => refresh(snapshot().offset) }, { bind: "c", run: onChange },
      { bind: "f", run: props.panel.toggleFullscreen }, { bind: "escape", run: props.panel.close },
    ] }))
    const summary = () => { const v = snapshot(); return `${v.completed}/${v.total} completed · ${v.open} open · ${v.draft} draft · ${v.held} held · ${v.archived} archived` }
    const context = () => { const v = snapshot(); return `${v.blocked} blocked across queue · ${v.intentChecksPassed} intent checks passed · ${v.activeAttempts} active attempts` }
    const description = () => detail() || (current() ? `${current().ticketId}\n${current().title}\n\n${current().priority} · ${current().milestone || "No milestone"}\nEligibility: ${current().eligibility}\nNext action: ${current().nextAction}\nGate results: ${current().gateResults}\nBlockers reported: ${current().blockers}\n\nEnter to read blockers and acceptance criteria.` : "No open tickets on this page.")
    return <box flexDirection="column" flexGrow={1} minHeight={0} padding={1}>
      <box flexDirection="row" justifyContent="space-between" flexShrink={0}><text fg={ctx.theme.text.base}><b>Corvint · Tasks</b></text><text fg={accent()} onMouseDown={onChange}>c Change</text></box>
      <text height={1} flexShrink={0} fg={snapshot().state === "ready" ? ctx.theme.text.feedback.info.base : ctx.theme.text.feedback.warning.base}>{() => snapshot().state === "ready" ? summary() : `${snapshot().state}: ${snapshot().reason}`}</text>
      <Show when={snapshot().state === "ready"}><text height={1} flexShrink={0} fg={ctx.theme.text.muted}>{context}</text></Show>
      <text height={1} flexShrink={0} fg={ctx.theme.text.muted}>{() => snapshot().observed ? `Observed ${snapshot().observed.slice(11, 19)} UTC · receipt ${snapshot().queueDigest.slice(0, 8)} · ${snapshot().queueId}` : "Read-only queue observation"}</text>
      <text height={1} flexShrink={0} fg={accent()} onMouseDown={() => refresh(snapshot().offset)}>{() => `Open tickets ${snapshot().offset + (snapshot().tickets.length ? 1 : 0)}–${snapshot().offset + snapshot().tickets.length} of ${snapshot().openTotal} · r refresh · ←/→ pages`}</text>
      <box flexDirection="row" flexGrow={1} minHeight={0} gap={2} marginTop={1}>
        <Show when={wide() || !detailPane()}><scrollbox ref={value => { list = value }} width={wide() ? 42 : "100%"} flexGrow={wide() ? 0 : 1} minHeight={0}>
          <Show when={snapshot().tickets.length} fallback={<text fg={ctx.theme.text.muted}>No ticket rows supplied. Refresh or inspect the queue state above.</text>}>
            <For each={snapshot().tickets}>{(row, index) => {
              const click = event => { event.stopPropagation(); props.panel.focus(); setSelected(index()); void open() }
              return <box id={`corvint-task-row-${index()}`} flexDirection="column" paddingX={1} paddingY={1} backgroundColor={index() === selected() ? ctx.theme.background.raised.high : undefined} onMouseDown={click}>
                <text fg={index() === selected() ? accent() : ctx.theme.text.base} onMouseDown={click}>{() => `${row.priority} ${row.ticketId.split(":").at(-1)} · ${row.title}`}</text>
                <text fg={ctx.theme.text.muted} onMouseDown={click}>{() => `${row.eligibility} · ${row.blockers} blockers · ${row.milestone || "No milestone"}`}</text>
              </box>
            }}</For>
          </Show>
        </scrollbox></Show>
        <Show when={wide() || detailPane()}><scrollbox ref={value => { reader = value }} flexGrow={1} minWidth={0} minHeight={0}>
          <text fg={ctx.theme.text.base}>{description}</text>
          <Show when={snapshot().gaps.length}><text fg={ctx.theme.text.feedback.warning.base}>{() => `\n\nLimits\n${snapshot().gaps.join("\n")}`}</text></Show>
        </scrollbox></Show>
      </box>
      <text fg={ctx.theme.text.muted}>↑↓ select/scroll · Enter detail · Tab list/details · r refresh · ←/→ pages · c Change · esc close</text>
    </box>
  }
}
