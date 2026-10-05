import { collectCockpit, emptyCockpit, inspectProof } from "./cockpit.js"
import { collectTaskDetail, collectTaskMetrics, emptyTaskMetrics, inspectTask } from "./task-metrics.js"
import { emptyWorkbench, projectWorkbench } from "./workbench.js"
import path from "node:path"
import { INSPECTOR_RPC, emptySnapshot, beginInspection, completeInspection, invalidateInspection, inspectionMatches, visibleText } from "./inspector.js"
import { qualificationStatus } from "./qualification.js"
import {
  boundedPaths,
  boundedTask,
  createCorvintRunner,
  explicitEvidenceHandles,
  explicitVerification,
  hashSessionId,
  hashTask,
  insideGitRepository,
  normalizeRepositoryPath,
  suppliedEvidenceHandles,
} from "./runtime.js"
import { ENVELOPE_COLLISION, frameRepositoryData } from "./envelope.js"
import { promptQuery, trimSpace } from "./prompt-bound.js"

const MAX_TRACKED_PATHS = 256
const MAX_SESSIONS = 128
const MAX_ADDITION_BYTES = 8_000
const MAX_IN_FLIGHT = 16
// AHI-022: best-effort file-change impact that does not apply to this path or repository is
// expected, not a fault the user can act on (decision 0379).
const EXPECTED_FILE_CHANGE_REFUSALS = new Set(["unsupported-impact-path-suffix", "unsupported-impact-repository"])

function betaEnabled(options, environment) {
  if (options?.enableBetaContext === true) return true
  return environment.CORVINT_OPENCODE_BETA_CONTEXT === "1"
}

async function setup(ctx) {
  const options = ctx.options ?? {}
  const environment = options.environment ?? process.env
  const directory = ctx.location.directory
  const root = ctx.location.project?.directory || directory
  // Outside a repository the plugin registers nothing and spawns nothing (decision 0378).
  if (!insideGitRepository(directory)) return
  const runCorvint = createCorvintRunner({ hostVersion: ctx.app?.version, ...options, environment })
  const sessions = new Map()
  const startupContexts = new Map()
  const pendingFileChanges = new Map()
  const events = new AbortController()
  const lifetime = new AbortController()
  let fileChangeDrain
  let inFlight = 0
  let inspectorRPC
  const changed = key => {
    if (inspectorRPC && !events.signal.aborted) void inspectorRPC.events.emit("updated", { sessionIdSha256: key }).catch(() => {})
  }
  const promptEvents = new WeakSet()

  // OpenCode 2 reports a built-in edit tool's target in its completed result; `edit` returns only
  // diffs, so its input path is resolved the way the host resolves it, against the location.
  const CHANGED_TARGETS = {
    edit: (call) => [typeof call.input?.path === "string" ? path.resolve(directory, call.input.path) : undefined],
    patch: (call) => (Array.isArray(call.result?.output?.applied) ? call.result.output.applied.map((entry) => entry?.target) : []),
    write: (call) => [call.result?.output?.target],
  }

  const notice = (code, event, receiptId, deadlineMs) => {
    const payload = { code, event, support: "FALLBACK" }
    if (receiptId) payload.receiptId = receiptId
    if (code === "timeout") {
      payload.deadlineMs = deadlineMs
      payload.detail = "Deadline exceeded; this is a bound, not a diagnosed fault."
    }
    return `[corvint/opencode] ${JSON.stringify(payload)}`
  }

  // A fault the user must act on: a terminal warning.
  const report = (code, event, receiptId, deadlineMs) => {
    console.warn(notice(code, event, receiptId, deadlineMs))
  }

  // AHI-022 (decision 0161 parity): a routine receipt's degradations and an expected guard are
  // recorded at info level. OpenCode 2 gives a plugin no log API, so both levels reach the server log.
  const record = (code, event, receiptId) => {
    console.info(notice(code, event, receiptId))
  }

  const runVisible = async (request) => {
    let response
    try {
      response = await runCorvint({ root, ...request, signal: request.signal ? AbortSignal.any([lifetime.signal, request.signal]) : lifetime.signal })
    } catch {
      response = { code: "adapter-internal-error", event: request.event, ok: false }
    }
    if (!response.ok) {
      const code = response.code ?? "corvint-degraded"
      if (request.event === "file-change" && EXPECTED_FILE_CHANGE_REFUSALS.has(code)) {
        record(code, request.event)
      } else {
        report(code, request.event, undefined, response.deadlineMs)
      }
    } else if (response.degradations?.length > 0) {
      record(response.degradations.join(","), request.event, response.receiptId)
    }
    return response
  }

  const drainFileChanges = async () => {
    await Promise.resolve()
    while (pendingFileChanges.size > 0) {
      const [key, batch] = pendingFileChanges.entries().next().value
      pendingFileChanges.delete(key)
      await runVisible({
        event: "file-change",
        input: {
          // V1-0767: a truncated batch carries its SOL-V0-010 code to Core's ledger writer.
          ...(batch.truncated ? { adapterCodes: ["changed-paths-truncated"] } : {}),
          ...(batch.sessionIdSha256 ? { sessionIdSha256: batch.sessionIdSha256 } : {}),
          paths: [...batch.paths],
        },
      })
    }
  }

  const queueFileChange = (changed, sessionIdSha256) => {
    let key = sessionIdSha256 ?? ""
    if (!pendingFileChanges.has(key) && pendingFileChanges.size >= MAX_SESSIONS) key = ""
    let batch = pendingFileChanges.get(key)
    if (!batch) {
      batch = { paths: new Set(), sessionIdSha256: key || undefined }
      pendingFileChanges.set(key, batch)
    }
    if (batch.paths.size < MAX_TRACKED_PATHS) batch.paths.add(changed)
    else if (!batch.paths.has(changed) && !batch.truncated) {
      batch.truncated = true
      record("changed-paths-truncated", "file-change")
    }
    if (!fileChangeDrain) {
      fileChangeDrain = drainFileChanges().finally(() => {
        fileChangeDrain = undefined
      })
    }
    return fileChangeDrain
  }

  const stateFor = (rawSessionId) => {
    const key = hashSessionId(rawSessionId)
    if (!key) return undefined
    let state = sessions.get(key)
    if (!state) {
      if (sessions.size >= MAX_SESSIONS) {
        const oldest = sessions.keys().next().value
        retire(sessions.get(oldest))
        sessions.delete(oldest)
        startupContexts.delete(oldest)
        changed(oldest)
        record("session-state-evicted", "session-start")
      }
      state = { changedPaths: new Set(), stopActive: false, stopArmed: true, active: true, calls: new Set(), prompts: new Set(), pendingPrompts: new Set(), generation: 0 }
      sessions.set(key, state)
    }
    return { key, state }
  }

  const retire = (state) => {
    if (!state) return
    invalidateInspection(state, "Session ended or was evicted.")
    state.tasksRequest?.abort()
    state.workbenchRequest?.abort()
    state.active = false
    for (const call of state.calls) call.abort()
  }

  const runForSession = async (bound, request) => {
    if (!bound) return runVisible(request)
    if (!bound.state.active) return { ok: false, code: "host-aborted" }
    const call = new AbortController()
    bound.state.calls.add(call)
    try {
      return await runVisible({ ...request, signal: request.signal ? AbortSignal.any([request.signal, call.signal]) : call.signal })
    } finally {
      bound.state.calls.delete(call)
    }
  }

  const runLimited = async (bound, request) => {
    if (inFlight >= MAX_IN_FLIGHT || (bound && bound.state.calls.size >= 2)) return { ok: false, code: "context-busy" }
    inFlight++
    try { return await runForSession(bound, request) } finally { inFlight-- }
  }

  // Encode existing pinned evidence as the core's cv1 selector; only the core may expand it.
  const expansionHandles = (response) => {
    const tree = response.repository?.treeRevision
    if (!/^(?:[0-9a-f]{40}|[0-9a-f]{64})$/.test(tree ?? "")) return []
    const handles = new Set()
    for (const row of Array.isArray(response.context?.results) ? response.context.results : []) {
      for (const evidence of Array.isArray(row?.evidence) ? row.evidence : []) {
        if (typeof evidence.path !== "string" || !/^(?:[0-9a-f]{40}|[0-9a-f]{64})$/.test(evidence.blob_hash ?? "")) continue
        const handle = `cv1:${tree}:${evidence.blob_hash}:all:${evidence.path}`
        if (handle.length <= 1024 && handles.size < 32) handles.add(handle)
      }
    }
    return [...handles]
  }

  const boundedFrame = (response, event, disclosure = "") => {
    const output = frameRepositoryData(response)
    if (!output) {
      report(ENVELOPE_COLLISION, event, response.receiptId)
      return undefined
    }
    const content = disclosure + output
    if (Buffer.byteLength(content, "utf8") > MAX_ADDITION_BYTES) {
      report("context-too-large", event, response.receiptId)
      return undefined
    }
    return content
  }

  const outsideProject = (value) => typeof value === "string" && value !== "" && !normalizeRepositoryPath(root, value)

  const rememberPath = (value, rawSessionId, adapterCodes) => {
    const normalized = normalizeRepositoryPath(root, value)
    if (!normalized) return undefined
    const bound = stateFor(rawSessionId)
    if (bound) {
      if (bound.state.changedPaths.size < MAX_TRACKED_PATHS) {
        bound.state.changedPaths.add(normalized)
      } else if (!bound.state.changedPaths.has(normalized) && !bound.state.pathsTruncated) {
        // V1-0746: stop and session-end now carry an incomplete path set; say so once per session.
        bound.state.pathsTruncated = true
        record("changed-paths-truncated", "post-tool")
        adapterCodes.add("changed-paths-truncated")
      }
      bound.state.stopArmed = true
    }
    for (const [key, state] of sessions) {
      invalidateInspection(state, "Files changed after this context was collected. Request context again.")
      changed(key)
    }
    return normalized
  }

  const stop = async (rawSessionId) => {
    const bound = stateFor(rawSessionId)
    if (!bound) {
      report("missing-session-identity", "stop")
      return
    }
    if (bound.state.stopActive || !bound.state.stopArmed) {
      record("stop-recursion-protected", "stop")
      return
    }
    bound.state.stopActive = true
    bound.state.stopArmed = false
    try {
      await runVisible({
        event: "stop",
        input: {
          changedPaths: [...bound.state.changedPaths],
          sessionIdSha256: bound.key,
          stopHookActive: false,
        },
      })
    } finally {
      bound.state.stopActive = false
    }
  }

  const sessionStart = async (rawSessionId) => {
    const bound = stateFor(rawSessionId)
    const input = bound ? { sessionIdSha256: bound.key } : {}
    const generation = bound?.state.generation
    const viewGeneration = bound?.state.inspectorGeneration || 0
    const response = await runForSession(bound, { event: "session-start", input, budgetBytes: 7000 })
    if (bound?.state.active && bound.state.generation === generation && response.ok && response.context) {
      startupContexts.set(bound.key, response)
      if (!bound.state.inspection) {
        bound.state.inspectorGeneration ??= 0
        if (completeInspection(bound.state, viewGeneration, response, expansionHandles(response))) changed(bound.key)
      }
    }
  }

  const sessionEnd = async (rawSessionId) => {
    const key = hashSessionId(rawSessionId)
    const state = key ? sessions.get(key) : undefined
    retire(state)
    await runVisible({
      event: "session-end",
      input: {
        ...(key ? { sessionIdSha256: key } : {}),
        changedPaths: state ? [...state.changedPaths] : [],
        openedPaths: [],
        verification: [],
      },
    })
    if (key) {
      sessions.delete(key)
      startupContexts.delete(key)
      changed(key)
    }
  }

  const compacted = (rawSessionId) => {
    const bound = stateFor(rawSessionId)
    if (!bound) return
    bound.state.generation++
    invalidateInspection(bound.state, "Session compacted. Request context again.")
    changed(bound.key)
    delete bound.state.promptContext
    bound.state.promptContextGeneration = (bound.state.promptContextGeneration ?? 0) + 1
    bound.state.needsCompaction = true
    startupContexts.delete(bound.key)
  }

  const EVENT_HANDLERS = {
    "session.compaction.ended": compacted,
    "session.created": sessionStart,
    "session.deleted": sessionEnd,
    "session.execution.failed": stop,
    "session.execution.succeeded": stop,
    "session.idle": stop,
  }

  const onEvent = async (event) => {
    const handler = EVENT_HANDLERS[event?.type]
    if (!handler) return
    try {
      await handler(event.data?.sessionID)
    } catch {
      report("stable-event-adapter-failed", event.type)
    }
  }

  // The outer code-mode `execute` call only wraps inner tool calls, which fire their own hooks.
  const afterTool = async (call) => {
    if (call?.tool === "execute") return
    try {
      const bound = stateFor(call.sessionID)
      if (bound) bound.state.stopArmed = true
      const targets = call.status === "completed" ? (CHANGED_TARGETS[call.tool]?.(call) ?? []) : []
      const edited = boundedPaths(root, targets)
      const metadata = call.result?.metadata
      const reported = Array.isArray(metadata?.corvint?.changedPaths) ? metadata.corvint.changedPaths : []
      const reportedPaths = boundedPaths(root, reported)
      const changedPaths = boundedPaths(root, [...edited, ...reportedPaths])
      // V1-0746: dropped host-reported paths are named at info level. boundedPaths examines at
      // most MAX_TRACKED_PATHS entries per list, so only those entries are scanned for an outside path.
      // V1-0767: each named abstention also reaches Core's SOL-V0-010 writer on this call's post-tool.
      const adapterCodes = new Set()
      if ([...targets.slice(0, MAX_TRACKED_PATHS), ...reported.slice(0, MAX_TRACKED_PATHS)].some(outsideProject)) {
        record("post-tool-path-not-project-relative", "post-tool")
        adapterCodes.add("post-tool-path-not-project-relative")
      }
      if (targets.length > MAX_TRACKED_PATHS || reported.length > MAX_TRACKED_PATHS || new Set([...edited, ...reportedPaths]).size > changedPaths.length) {
        record("changed-paths-truncated", "post-tool")
        adapterCodes.add("changed-paths-truncated")
      }
      for (const changed of changedPaths) rememberPath(changed, call.sessionID, adapterCodes)
      let drained
      for (const changed of edited) drained = queueFileChange(changed, bound?.key)
      await drained
      await runVisible({
        event: "post-tool",
        input: {
          ...(adapterCodes.size > 0 ? { adapterCodes: [...adapterCodes].sort() } : {}),
          ...(bound ? { sessionIdSha256: bound.key } : {}),
          changedPaths,
          observedEvidenceHandles: explicitEvidenceHandles(metadata),
          verification: explicitVerification(metadata),
        },
      })
    } catch {
      report("post-tool-adapter-failed", "post-tool")
    }
  }

  const corvintContext = {
    name: "corvint_context",
    description:
      "Request bounded, revision-pinned Corvint context for one explicit task. Returns a FALLBACK receipt and evidence; it does not prove completion.",
    input: {
      type: "object",
      properties: { task: { type: "string" } },
      required: ["task"],
      additionalProperties: false,
    },
    async execute(args, context) {
      // AHI-016: an over-bound task is served by its disclosed anchor query or refused.
      const prompt = typeof args?.task === "string" ? trimSpace(args.task) : ""
      const query = prompt === "" ? undefined : promptQuery(prompt)
      if (!query) {
        const bound = stateFor(context?.sessionID)
        if (bound) {
          const generation = beginInspection(bound.state)
          completeInspection(bound.state, generation, { ok: false, code: "prompt-over-query-bound" }, [])
          changed(bound.key)
        }
        return {
          content:
            "Corvint degraded (prompt-over-query-bound): task must be non-empty and either at most " +
            "2,000 Unicode characters and 16,384 UTF-8 bytes or carry explicit anchors within that bound.",
        }
      }
      const task = query.task
      const bound = stateFor(context?.sessionID)
      const viewGeneration = bound ? beginInspection(bound.state) : undefined
      if (bound) changed(bound.key)
      const sessionIdSha256 = hashSessionId(context?.sessionID)
      const response = await runLimited(bound, {
        event: "user-prompt",
        input: { ...(sessionIdSha256 ? { sessionIdSha256 } : {}), task },
        query: true,
        budgetBytes: 7000,
        signal: context?.signal,
      })
      if (!response.ok) {
        if (bound && completeInspection(bound.state, viewGeneration, response, [])) changed(bound.key)
        return { content: `Corvint FALLBACK unavailable (${response.code}); unrelated coding may continue.` }
      }
      const handles = expansionHandles(response)
      const output = boundedFrame({ ...response, expansionHandles: handles }, "user-prompt", query.disclosure)
      if (bound && completeInspection(bound.state, viewGeneration, output ? response : { ok: false, code: "unsafe-or-oversized-envelope" }, handles)) changed(bound.key)
      if (!output) return { content: `Corvint context unavailable (${frameRepositoryData(response) ? "context-too-large" : ENVELOPE_COLLISION}).` }
      return {
        content: output,
        metadata: {
          corvint: {
            observedEvidenceHandles: [...suppliedEvidenceHandles(response), ...handles],
            receiptId: response.receiptId,
          },
        },
      }
    },
  }

  const corvintExpand = {
    name: "corvint_expand",
    description: "Expand an exact cv1 evidence handle from Corvint context. Reads its immutable Git blob; refuses invalid or oversized evidence.",
    input: { type: "object", properties: { handle: { type: "string" } }, required: ["handle"], additionalProperties: false },
    async execute(args, context) {
      const response = await runLimited(stateFor(context?.sessionID), { event: "expand", input: {}, expand: args?.handle ?? "", query: true, signal: context?.signal })
      if (!response.ok) return { content: `Corvint expansion unavailable (${response.code}).` }
      const content = boundedFrame(response, "expand")
      if (!content) return { content: "Corvint expansion unavailable (unsafe or oversized envelope)." }
      return { content, metadata: { corvint: { observedEvidenceHandles: [response.handle] } } }
    },
  }

  const onPrompt = async (event) => {
    if (events.signal.aborted || !event?.prompt || typeof event.prompt.text !== "string" || promptEvents.has(event)) return
    promptEvents.add(event)
    const bound = stateFor(event.sessionID)
    if (!bound || !bound.state.active) return
    const identity = hashSessionId(event.messageID)
    if (identity && (bound.state.prompts.has(identity) || bound.state.pendingPrompts.has(identity))) return
    const promptGeneration = (bound.state.promptContextGeneration ?? 0) + 1
    bound.state.promptContextGeneration = promptGeneration
    delete bound.state.promptContext
    if (inFlight >= MAX_IN_FLIGHT || bound.state.calls.size >= 2) {
      report("prompt-context-busy", "user-prompt")
      return
    }
    if (identity) bound.state.pendingPrompts.add(identity)
    const original = event.prompt.text
    const call = new AbortController()
    try {
      const result = await corvintContext.execute({ task: original }, { sessionID: event.sessionID, signal: call.signal })
      if (!bound.state.active || call.signal.aborted || events.signal.aborted || event.prompt.text !== original || bound.state.promptContextGeneration !== promptGeneration) return
      if (!result.metadata?.corvint?.receiptId) {
        report("prompt-context-unavailable", "user-prompt")
        return
      }
      const addition = "\n\n" + result.content
      if (Buffer.byteLength(addition, "utf8") > MAX_ADDITION_BYTES) {
        report("context-too-large", "user-prompt")
        return
      }
      // AHI-032: keep repository data in model context rather than persisted user input.
      bound.state.promptContext = result.content
      bound.state.stopArmed = true
      if (identity) {
        bound.state.prompts.add(identity)
        if (bound.state.prompts.size > 64) bound.state.prompts.delete(bound.state.prompts.values().next().value)
      }
      record("prompt-context-supplied", "user-prompt", result.metadata.corvint.receiptId)
    } catch {
      report("prompt-context-failed", "user-prompt")
    } finally {
      if (identity) bound.state.pendingPrompts.delete(identity)
    }
  }

  const onContext = async (request) => {
    if (!Array.isArray(request?.system)) return
    const bound = stateFor(request?.sessionID)
    if (!bound?.state.active || events.signal.aborted) return
    const { state, key } = bound
    if (state.promptContext && !request.system.some(item => item.text === state.promptContext)) {
      request.system.push({ type: "text", text: state.promptContext })
    }
    const generation = state.generation
    if (state.needsCompaction) {
      if (!state.recovery || state.recovery.generation !== generation) {
        state.recovery = {
          generation,
          promise: runForSession(bound, { event: "session-start", input: { sessionIdSha256: key, startSource: "compact" }, budgetBytes: 7000 }),
        }
      }
      const response = await state.recovery.promise
      if (!state.active || events.signal.aborted || generation !== state.generation) return
      state.recovery = undefined
      if (!response.ok || !response.context) return
      startupContexts.set(key, response)
    } else if (!state.rehydrated && !betaEnabled(options, environment)) return
    const response = startupContexts.get(key)
    if (!response) return
    const content = boundedFrame({ context: response.context, receiptId: response.receiptId, support: response.support }, "session-start")
    if (!content) return
    const text = `[Corvint FALLBACK context; receipt-linked, non-authoritative]\n${content}`
    if (Buffer.byteLength(text, "utf8") > MAX_ADDITION_BYTES) {
      report("context-too-large", "session-start", response.receiptId)
      return
    }
    if (!request.system.some(item => item.text === text)) request.system.push({ type: "text", text })
    if (state.needsCompaction) {
      state.needsCompaction = false
      state.rehydrated = true
    }
  }

  const corvintRecordOutcome = {
    name: "corvint_record_outcome",
    description:
      "Record a caller-reported explicit Corvint task outcome with bounded change and verification observations. This does not verify correctness.",
    input: {
      type: "object",
      properties: {
        changedPaths: { type: "array", items: { type: "string" } },
        outcome: { type: "string", enum: ["passed", "failed", "blocked"] },
        task: { type: "string" },
        verification: {
          type: "array",
          items: {
            type: "object",
            properties: {
              commandSha256: { type: "string" },
              status: { type: "string", enum: ["passed", "failed", "not-run", "unknown"] },
            },
            required: ["commandSha256", "status"],
            additionalProperties: false,
          },
        },
      },
      required: ["changedPaths", "outcome", "task", "verification"],
      additionalProperties: false,
    },
    async execute(args, context) {
      const task = boundedTask(args?.task)
      const changedPaths = boundedPaths(root, args?.changedPaths)
      const verification = explicitVerification({ corvint: { verification: args?.verification } })
      if (!task || changedPaths.length === 0 || verification.length === 0) {
        return {
          content: "Corvint degraded: an explicit bounded task, changed path, and verification observation are required.",
        }
      }
      const sessionIdSha256 = hashSessionId(context?.sessionID)
      const response = await runVisible({
        event: "session-end",
        input: {
          ...(sessionIdSha256 ? { sessionIdSha256 } : {}),
          changedPaths,
          openedPaths: [],
          outcome: args.outcome,
          taskSha256: hashTask(task),
          verification,
        },
        query: true,
        budgetBytes: 7000,
        signal: context?.signal,
      })
      if (!response.ok) {
        return { content: `Corvint FALLBACK unavailable (${response.code}); unrelated coding may continue.` }
      }
      const output = boundedFrame(response, "session-end")
      if (!output) return { content: `Corvint context unavailable (${frameRepositoryData(response) ? "context-too-large" : ENVELOPE_COLLISION}).` }
      return { content: output, metadata: { corvint: { receiptId: response.receiptId } } }
    },
  }

  const cockpitSession = async (sessionID, signal) => {
    if (events.signal.aborted || signal.aborted) return false
    try {
      const info = await ctx.session.get({ sessionID }, { signal })
      return info.projectID === ctx.location.project?.id && path.resolve(info.location?.directory || "/") === path.resolve(directory)
    } catch { return false }
  }
  const cockpitCall = async (bound, signal, action) => {
    if (!bound?.state.active || inFlight >= MAX_IN_FLIGHT || bound.state.calls.size >= 2) throw new Error("context-busy")
    const controller = new AbortController()
    const combined = AbortSignal.any([signal, controller.signal, lifetime.signal, AbortSignal.timeout(20_000)])
    inFlight++; bound.state.calls.add(controller)
    try { return await action(read => runCorvint({ root, event: "cockpit", input: {}, read, signal: combined })) }
    finally { controller.abort(); bound.state.calls.delete(controller); inFlight-- }
  }
  const readQualification = signal => qualificationStatus({
    hostVersion: options.hostVersion ?? ctx.app?.version,
    corvintBinary: options.corvintBinary ?? environment.CORVINT_BIN ?? "corvint",
    environment, root, signal,
  })
  const workbenchFor = state => !state ? emptyWorkbench()
    : !state.active ? emptyWorkbench("unavailable", "Session ended or was evicted.")
      : state.focusDetail ? projectWorkbench(state.focusDetail, state.tasks, state.cockpit, state.inspection, state.qualification)
        : emptyWorkbench()

  // RPC is a trusted OpenCode-client surface. Session/location checks prevent accidental cross-view reads.
  if (ctx.rpc?.register) inspectorRPC = await ctx.rpc.register(INSPECTOR_RPC, {
    tasksSnapshot: async ({ sessionID }, context) => await cockpitSession(sessionID, context.signal)
      ? sessions.get(hashSessionId(sessionID))?.tasks || emptyTaskMetrics() : emptyTaskMetrics("unavailable", "Session is unavailable at this location."),
    tasksRefresh: async ({ sessionID, offset }, context) => {
      if (!await cockpitSession(sessionID, context.signal)) return emptyTaskMetrics("unavailable", "Session is unavailable at this location.")
      const bound = stateFor(sessionID), state = bound.state
      state.tasksRequest?.abort(); state.tasksRequest = new AbortController()
      const signal = AbortSignal.any([context.signal, state.tasksRequest.signal])
      const generation = state.tasksGeneration = (state.tasksGeneration || 0) + 1
      state.tasks = emptyTaskMetrics("loading", "Reading Corvint Tasks…")
      try {
        const result = await cockpitCall(bound, signal, run => collectTaskMetrics(run, offset))
        if (state.active && generation === state.tasksGeneration) state.tasks = signal.aborted ? emptyTaskMetrics("unavailable", "Task read cancelled. Refresh to retry.") : result
      } catch (error) {
        if (state.active && generation === state.tasksGeneration) state.tasks = emptyTaskMetrics("unavailable", visibleText(error.message || "Corvint Tasks unavailable."))
      }
      changed(bound.key)
      return state.tasks || emptyTaskMetrics()
    },
    tasksDetail: async ({ sessionID, receiptId, ticketId }, context) => {
      if (!await cockpitSession(sessionID, context.signal)) return { state: "unavailable", text: "Session is unavailable at this location." }
      const key = hashSessionId(sessionID), state = sessions.get(key)
      const current = () => state?.active && state.tasks?.state === "ready" && state.tasks.receiptId === receiptId
      if (!current()) return { state: "unavailable", text: "Task view changed. Refresh and retry." }
      try {
        const text = await cockpitCall({ key, state }, context.signal, run => inspectTask(run, state.tasks, ticketId))
        return current() ? { state: "ready", text } : { state: "unavailable", text: "Task view changed during the read." }
      } catch (error) { return { state: "unavailable", text: visibleText(error.message || "Task detail unavailable.") } }
    },
    workbenchSnapshot: async ({ sessionID }, context) => await cockpitSession(sessionID, context.signal)
      ? workbenchFor(sessions.get(hashSessionId(sessionID))) : emptyWorkbench("unavailable", "Session is unavailable at this location."),
    workbenchFocus: async ({ sessionID, receiptId, ticketId }, context) => {
      if (!await cockpitSession(sessionID, context.signal)) return emptyWorkbench("unavailable", "Session is unavailable at this location.")
      const key = hashSessionId(sessionID), state = sessions.get(key)
      if (state?.tasks?.state !== "ready" || state.tasks.receiptId !== receiptId || !state.tasks.tickets.some(row => row.ticketId === ticketId)) return emptyWorkbench("stale", "Ticket is not in the current queue page. Refresh Tasks.")
      const generation = state.workbenchGeneration = (state.workbenchGeneration || 0) + 1
      state.workbenchRequest?.abort(); state.workbenchRequest = new AbortController()
      const signal = AbortSignal.any([context.signal, state.workbenchRequest.signal])
      try {
        const detail = await cockpitCall({ key, state }, signal, run => collectTaskDetail(run, state.tasks, ticketId))
        if (!state.active || signal.aborted || generation !== state.workbenchGeneration || state.tasks.receiptId !== receiptId) return emptyWorkbench("stale", "Task view changed during selection.")
        const qualification = await readQualification(signal)
        if (!state.active || signal.aborted || generation !== state.workbenchGeneration || state.tasks?.receiptId !== receiptId) return emptyWorkbench("stale", "Task view changed during selection.")
        state.focusDetail = detail; state.qualification = qualification
        changed(key)
        return workbenchFor(state)
      } catch (error) { return emptyWorkbench("unavailable", visibleText(error.message || "Task focus unavailable.")) }
    },
    workbenchRefresh: async ({ sessionID }, context) => {
      if (!await cockpitSession(sessionID, context.signal)) return emptyWorkbench("unavailable", "Session is unavailable at this location.")
      const key = hashSessionId(sessionID), state = sessions.get(key)
      if (!state?.focusDetail) return emptyWorkbench()
      const generation = state.workbenchGeneration = (state.workbenchGeneration || 0) + 1
      state.workbenchRequest?.abort(); state.workbenchRequest = new AbortController()
      const signal = AbortSignal.any([context.signal, state.workbenchRequest.signal])
      state.tasksRequest?.abort(); state.cockpitRequest?.abort()
      const tasksGeneration = state.tasksGeneration = (state.tasksGeneration || 0) + 1
      const cockpitGeneration = state.cockpitGeneration = (state.cockpitGeneration || 0) + 1
      const ticketId = state.focusDetail.ticketId, offset = state.tasks?.offset || 0
      try {
        const value = await cockpitCall({ key, state }, signal, async run => {
          const tasks = await collectTaskMetrics(run, offset)
          if (!tasks.tickets.some(row => row.ticketId === ticketId)) throw new Error("focused-ticket-left-page-refresh-tasks")
          const detail = await collectTaskDetail(run, tasks, ticketId)
          let cockpit
          try { cockpit = await collectCockpit(run) } catch (error) { cockpit = { view: emptyCockpit("unavailable", visibleText(error.message)), binding: undefined } }
          return { tasks, detail, cockpit }
        })
        const qualification = await readQualification(signal)
        if (!state.active || signal.aborted || generation !== state.workbenchGeneration || tasksGeneration !== state.tasksGeneration || cockpitGeneration !== state.cockpitGeneration) return emptyWorkbench("stale", "Workbench changed during refresh.")
        state.tasks = value.tasks; state.focusDetail = value.detail
        state.cockpit = value.cockpit.view; state.cockpitBinding = value.cockpit.binding; state.qualification = qualification
        changed(key)
        return workbenchFor(state)
      } catch (error) {
        if (state.active && generation === state.workbenchGeneration && tasksGeneration === state.tasksGeneration) {
          state.tasks = emptyTaskMetrics("unavailable", visibleText(error.message || "Workbench refresh unavailable."))
          changed(key)
        }
        return emptyWorkbench("unavailable", visibleText(error.message || "Workbench refresh unavailable."))
      }
    },
    workbenchPeers: async ({ sessionID, peerIDs }, context) => {
      if (!await cockpitSession(sessionID, context.signal)) return { rows: [] }
      const rows = []
      for (const peerID of [...new Set(peerIDs)].slice(0, 16)) {
        if (!await cockpitSession(peerID, context.signal)) continue
        const state = sessions.get(hashSessionId(peerID))
        const focus = state?.active ? state.focusDetail : undefined
        rows.push({ sessionID: peerID, ticketId: focus?.ticketId || "", state: state?.active ? workbenchFor(state).state : "unbound",
          holder: focus?.attempt?.holder || "", phase: focus?.attempt?.phase || "", expiresAt: focus?.attempt?.expiresAt || "" })
      }
      return { rows }
    },
    cockpitSnapshot: async ({ sessionID }, context) => await cockpitSession(sessionID, context.signal)
      ? sessions.get(hashSessionId(sessionID))?.cockpit || emptyCockpit() : emptyCockpit("unavailable", "Session is unavailable at this location."),
    cockpitRefresh: async ({ sessionID, base }, context) => {
      if (!await cockpitSession(sessionID, context.signal)) return emptyCockpit("unavailable", "Session is unavailable at this location.")
      const bound = stateFor(sessionID), state = bound.state
      state.cockpitRequest?.abort(); state.cockpitRequest = new AbortController()
      const signal = AbortSignal.any([context.signal, state.cockpitRequest.signal])
      const generation = state.cockpitGeneration = (state.cockpitGeneration || 0) + 1
      state.cockpitBinding = undefined; state.cockpit = emptyCockpit("loading", "Reading change and verification evidence…"); changed(bound.key)
      try {
        const result = await cockpitCall(bound, signal, run => collectCockpit(run, base))
        if (state.active && generation === state.cockpitGeneration && !signal.aborted) { state.cockpit = result.view; state.cockpitBinding = result.binding }
      } catch (error) {
        if (state.active && generation === state.cockpitGeneration) state.cockpit = emptyCockpit("unavailable", visibleText(error.message || "Change inspection unavailable."))
      }
      changed(bound.key)
      return state.cockpit || emptyCockpit()
    },
    cockpitProof: async ({ sessionID, receiptId, checkID }, context) => {
      if (!await cockpitSession(sessionID, context.signal)) return { state: "unavailable", text: "Session is unavailable at this location." }
      const key = hashSessionId(sessionID), state = sessions.get(key)
      const current = () => state?.active && state.cockpit?.state === "ready" && state.cockpit.receiptId === receiptId
      if (!current() || !state.cockpit.checks.some(c => c.id === checkID)) return { state: "unavailable", text: "Verification is no longer current. Refresh the change view." }
      try {
        const text = await cockpitCall({ key, state }, context.signal, run => inspectProof(run, state.cockpitBinding, checkID))
        return current() ? { state: "ready", text } : { state: "unavailable", text: "Change view changed while reading verification." }
      } catch (error) { return { state: "unavailable", text: visibleText(error.message || "Verification unavailable. Refresh the change view.") } }
    },
    snapshot: async ({ sessionID }) => sessions.get(hashSessionId(sessionID))?.inspection || emptySnapshot(),
    query: async ({ sessionID, task }, rpcContext) => {
      let info
      try { info = await ctx.session.get({ sessionID }, { signal: rpcContext.signal }) } catch { return emptySnapshot("unavailable", "Session is unavailable.") }
      if (info.projectID !== ctx.location.project?.id || path.resolve(info.location?.directory || "/") !== path.resolve(directory)) return emptySnapshot("unavailable", "Session belongs to another location.")
      if (events.signal.aborted || rpcContext.signal.aborted) return emptySnapshot("unavailable", "Request cancelled.")
      await corvintContext.execute({ task }, { sessionID, signal: rpcContext.signal })
      return sessions.get(hashSessionId(sessionID))?.inspection || emptySnapshot()
    },
    expand: async ({ sessionID, receiptId, handle }, rpcContext) => {
      const key = hashSessionId(sessionID)
      const state = sessions.get(key)
      if (!inspectionMatches(state, receiptId, handle)) return { state: "unavailable", text: "This evidence is no longer current in the inspector. Request context again." }
      const generation = state.inspectorGeneration
      const response = await runLimited({ key, state }, { event: "expand", input: {}, expand: handle, query: true, signal: rpcContext.signal })
      if (state.inspectorGeneration !== generation || !inspectionMatches(state, receiptId, handle)) return { state: "unavailable", text: "Context changed while opening this evidence." }
      if (!response.ok) return { state: "unavailable", text: `Core could not expand this evidence (${response.code}).` }
      return { state: "ready", text: response.selection.text }
    },
  })

  const subscription = (async () => {
    try {
      for await (const event of ctx.event.subscribe({ signal: events.signal })) await onEvent(event)
    } catch {
      if (!events.signal.aborted) report("event-subscription-failed", "session-start")
    }
  })()
  await ctx.tool.hook("execute.after", afterTool)
  await ctx.tool.transform((editor) => {
    editor.add({
      name: "corvint_status",
      description: "Check this installed OpenCode integration against its exact qualification evidence. Reports integration support separately from execution authority.",
      input: { type: "object", properties: {}, additionalProperties: false },
      async execute(_args, context) {
        return { content: JSON.stringify(await qualificationStatus({
          hostVersion: options.hostVersion ?? ctx.app?.version,
          corvintBinary: options.corvintBinary ?? environment.CORVINT_BIN ?? "corvint",
          environment,
          signal: context?.signal,
          root,
        })) }
      },
    })
    editor.add(corvintContext)
    editor.add(corvintExpand)
    editor.add(corvintRecordOutcome)
  })

  await ctx.session.hook("prompt", onPrompt)
  await ctx.session.hook("context", onContext)

  return async () => {
    events.abort()
    await inspectorRPC?.dispose()
    for (const state of sessions.values()) retire(state)
    // Finish an already observed advisory stop within its normal deadline; cancel task context now.
    await subscription
    pendingFileChanges.clear()
    lifetime.abort()
    if (fileChangeDrain) await fileChangeDrain
    pendingFileChanges.clear()
    sessions.clear()
    startupContexts.clear()
  }
}

export default { id: "corvint", setup }
