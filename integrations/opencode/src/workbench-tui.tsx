import { createEffect, createSignal, For, onCleanup, Show, untrack } from "solid-js"
import { createHash } from "node:crypto"
import { emptyWorkbench } from "./workbench.js"
import { focusedEconomics, sessionMap } from "./session-metrics.js"
import { visibleText } from "./inspector.js"

export function createWorkbench(ctx, { rpc, lifetime, location, accent, baselines, onTasks, onChange, onEvidence }) {
  return function Workbench(props) {
    const [snapshot, setSnapshot] = createSignal(emptyWorkbench())
    const [peers, setPeers] = createSignal([])
    const [tab, setTab] = createSignal("overview")
    const [busy, setBusy] = createSignal(false)
    const controller = new AbortController()
    let request, serial = 0, disposed = false, reader
    const scope = () => `${props.panel.sessionID}\0${location().directory}`
    const family = () => {
      try { return ctx.data.session.family(props.panel.sessionID) || [] } catch { return [] }
    }
    const observe = async (refresh = false) => {
      const generation = ++serial, captured = scope(), here = location(), sessionID = props.panel.sessionID
      request?.abort(); request = new AbortController()
      const signal = AbortSignal.any([request.signal, controller.signal, lifetime.signal])
      setBusy(refresh)
      try {
        const value = refresh ? await rpc.workbenchRefresh({ sessionID }, { location: here, signal }) : await rpc.workbenchSnapshot({ sessionID }, { location: here, signal })
        const ids = [...new Set([sessionID, ...family().filter(id => typeof id === "string")])].slice(0, 16)
        const map = await rpc.workbenchPeers({ sessionID, peerIDs: ids }, { location: here, signal })
        if (!disposed && generation === serial && captured === scope() && !signal.aborted) { setSnapshot(value); setPeers(map.rows) }
      } catch { if (!disposed && generation === serial && !signal.aborted) setSnapshot(emptyWorkbench("unavailable", "Workbench read unavailable. Press r to retry.")) }
      finally { if (generation === serial) setBusy(false) }
    }
    createEffect(() => { scope(); untrack(() => { setTab("overview"); setSnapshot(emptyWorkbench()); setPeers([]); void observe() }) })
    const unsubscribe = rpc.events.on("updated", event => {
      const familyIDs = [props.panel.sessionID, ...family()]
      if (event.location?.directory === location().directory && familyIDs.some(id => typeof id === "string" && createHash("sha256").update(id).digest("hex") === event.data.sessionIdSha256)) void observe()
    }, { signal: lifetime.signal })
    onCleanup(() => { disposed = true; controller.abort(); request?.abort(); unsubscribe() })
    const tabs = ["overview", "proof", "doctor", "cost", "sessions"]
    ctx.keymap.layer(() => ({ enabled: () => props.panel.focused, commands: [
      ...tabs.map((name, i) => ({ bind: String(i + 1), run: () => { setTab(name); reader?.scrollBy(-1000) } })),
      { bind: "r", run: () => observe(true) }, { bind: "t", run: onTasks }, { bind: "c", run: onChange },
      { bind: "e", run: onEvidence }, { bind: "up", run: () => reader?.scrollBy(-1) },
      { bind: "down", run: () => reader?.scrollBy(1) }, { bind: "pageup", run: () => reader?.scrollBy(-1, "viewport") },
      { bind: "pagedown", run: () => reader?.scrollBy(1, "viewport") }, { bind: "escape", run: props.panel.close },
    ] }))
    const cost = () => {
      const baseline = baselines.get(props.panel.sessionID)
      return focusedEconomics(ctx.data.session.get(props.panel.sessionID), baseline, snapshot().ticketId, location().directory)
    }
    const members = () => sessionMap(props.panel.sessionID, family(), id => ctx.data.session.get(id), peers())
    const branch = () => {
      try { return visibleText(ctx.data.location.vcs.info(location())?.branch?.current || "NOT_OBSERVED", 128) }
      catch { return "NOT_OBSERVED" }
    }
    const openAction = kind => {
      if (kind === "tasks") onTasks()
      else if (kind === "change") onChange()
      else if (kind === "doctor") setTab("doctor")
      else setTab("proof")
    }
    const body = () => {
      const view = snapshot()
      if (view.state !== "ready") return `${view.state}: ${view.reason}\n\nOpen Tasks, select an open ticket, then press s to focus it.`
      if (tab() === "proof") return [
        `Acceptance → proof · queue ${view.queueDigest.slice(0, 12)} · revision ${view.taskRevision || "NOT_OBSERVED"}`,
        ...view.criteria.flatMap((row, i) => [`\n${i + 1}. ${row.status} · ${row.text}`, `   Requirements: ${row.requirements.join(", ") || "NOT_LINKED"}`, `   ${row.reason}`]),
        "\nTicket-level requirement references", view.requirements.join("\n") || "NOT_OBSERVED",
        "\nDeclared paths", view.declaredPaths.join("\n") || "NOT_OBSERVED",
        "\nObserved changed paths matching declarations", view.changedPaths.join("\n") || "NOT_OBSERVED",
        "\nSuggested checks (not executed here)", view.plannedChecks.join("\n") || "NOT_OBSERVED",
        "\nRecorded check observations", ...view.observedChecks.map(row => `${row.status} · ${row.id} · ${row.testedCommit || "no tested commit"}`),
        `\nContext receipt: ${view.contextReceipt || "NOT_OBSERVED"}`,
        `Change receipt: ${view.changeReceipt || "NOT_OBSERVED"}`,
        `Workflow: ${view.workflow}`,
      ].join("\n")
      if (tab() === "doctor") return [
        `Native integration: ${view.doctor.support}`,
        `Host ${view.doctor.hostVersion} · adapter ${view.doctor.adapterVersion} · ${view.doctor.platform}`,
        `Reason: ${view.doctor.reason}`,
        `Failed conformance cases: ${view.doctor.failedCases.join(", ") || "none named by available record"}`,
        `\nQualification action\n${view.doctor.action}`,
        "\nExecution authority remains NONE; a UI witness does not qualify the host.",
      ].join("\n")
      if (tab() === "cost") { const metric = cost(); return [
        `OpenCode session cost: ${metric.cost}`,
        `Cost since ticket focus: ${metric.ticketCost}`,
        `Tokens · input ${metric.input} · output ${metric.output} · reasoning ${metric.reasoning}`,
        `Cache tokens · read ${metric.cacheRead} · write ${metric.cacheWrite}`,
        `\n${metric.note}`,
        "Cost per verified criterion: NOT_OBSERVED. Savings and latency: NOT_OBSERVED.",
      ].join("\n") }
      if (tab() === "sessions") { const map = members(); return [
        `OpenCode session family · ${map.rows.length} shown${map.omitted ? ` · ${map.omitted} omitted` : ""}`,
        ...map.rows.flatMap(row => [`\n${row.current ? "Current" : "Peer"} ${row.id} · parent ${row.parent}`, `Ticket ${row.ticketId} · ${row.state}`, `Lease ${row.holder} · ${row.phase} · expires ${row.expiresAt}`, `Worktree ${row.directory}`]),
        `\nOverlapping focused tickets: ${map.duplicates.join(", ") || "none observed"}`,
        "Unbound sessions may still do work. Focus is explicit and volatile; another worktree's binding may be unavailable from this plugin location.",
      ].join("\n") }
      return [
        `${view.title} (${view.ticketId})`,
        `Queue ${view.queueDigest.slice(0, 12)} · ticket revision ${view.taskRevision || "NOT_OBSERVED"}`,
        `Branch ${branch()} · worktree ${visibleText(location().directory, 256)}`,
        `Eligibility ${view.eligibility} · gate ${view.gateResults || "NOT_OBSERVED"} · completion ${view.completion || "NOT_OBSERVED"}`,
        `Attempt ${view.attempt.attemptId || "NOT_OBSERVED"} · holder ${view.attempt.holder || "NOT_OBSERVED"}`,
        `Phase ${view.attempt.phase || "NOT_OBSERVED"} · lease expires ${view.attempt.expiresAt || "NOT_OBSERVED"}`,
        `Criteria ${view.criteria.length} · criterion-specific proof 0 observed`,
        `Session cost ${cost().cost} · since focus ${cost().ticketCost}`,
        "\nNext actions (select an action below)",
        "\nLimits", ...view.gaps,
      ].join("\n")
    }
    return <box flexDirection="column" flexGrow={1} minHeight={0} padding={1}>
      <box flexDirection="row" justifyContent="space-between"><text fg={ctx.theme.text.base}><b>Corvint · Workbench</b></text><text fg={accent()} onMouseDown={() => observe(true)}>{() => busy() ? "Refreshing…" : "r Refresh"}</text></box>
      <box flexDirection="row" gap={2} marginY={1}>{tabs.map((name, i) => <text fg={tab() === name ? accent() : ctx.theme.text.muted} onMouseDown={() => setTab(name)}>{`${i + 1} ${name}`}</text>)}</box>
      <scrollbox ref={value => { reader = value }} flexGrow={1} minHeight={0}>
        <text fg={snapshot().state === "ready" ? ctx.theme.text.base : ctx.theme.text.feedback.warning.base}>{body}</text>
        <Show when={snapshot().state === "ready" && tab() === "overview"}><For each={snapshot().actions}>{(row, i) => <text fg={accent()} onMouseDown={() => openAction(row.kind)}>{() => `${i() + 1}. ${row.text}`}</text>}</For></Show>
      </scrollbox>
      <text fg={ctx.theme.text.muted}>1–5 tabs · ↑↓ scroll · r refresh · t Tasks · c Change · e Evidence · esc close</text>
    </box>
  }
}
