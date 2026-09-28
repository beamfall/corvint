import path from "node:path"
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
  let fileChangeDrain

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
      response = await runCorvint({ root, ...request })
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
    } else if (response.degradations.length > 0) {
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
        sessions.delete(oldest)
        startupContexts.delete(oldest)
        record("session-state-evicted", "session-start")
      }
      state = { changedPaths: new Set(), stopActive: false, stopArmed: true }
      sessions.set(key, state)
    }
    return { key, state }
  }

  const rememberPath = (value, rawSessionId) => {
    const normalized = normalizeRepositoryPath(root, value)
    if (!normalized) return undefined
    const bound = stateFor(rawSessionId)
    if (bound) {
      if (bound.state.changedPaths.size < MAX_TRACKED_PATHS) {
        bound.state.changedPaths.add(normalized)
      }
      bound.state.stopArmed = true
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
    const response = await runVisible({ event: "session-start", input })
    if (bound && response.ok && response.context) startupContexts.set(bound.key, response)
  }

  const sessionEnd = async (rawSessionId) => {
    const key = hashSessionId(rawSessionId)
    const state = key ? sessions.get(key) : undefined
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
    }
  }

  const EVENT_HANDLERS = {
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
      const changedPaths = boundedPaths(root, [...edited, ...boundedPaths(root, metadata?.corvint?.changedPaths)])
      for (const changed of changedPaths) rememberPath(changed, call.sessionID)
      let drained
      for (const changed of edited) drained = queueFileChange(changed, bound?.key)
      await drained
      await runVisible({
        event: "post-tool",
        input: {
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
        return {
          content:
            "Corvint degraded (prompt-over-query-bound): task must be non-empty and either at most " +
            "2,000 Unicode characters and 16,384 UTF-8 bytes or carry explicit anchors within that bound.",
        }
      }
      const task = query.task
      const sessionIdSha256 = hashSessionId(context?.sessionID)
      const response = await runVisible({
        event: "user-prompt",
        input: { ...(sessionIdSha256 ? { sessionIdSha256 } : {}), task },
        query: true,
        signal: context?.signal,
      })
      if (!response.ok) {
        return { content: `Corvint FALLBACK unavailable (${response.code}); unrelated coding may continue.` }
      }
      const output = frameRepositoryData(response)
      if (!output) {
        report(ENVELOPE_COLLISION, "user-prompt", response.receiptId)
        return { content: `Corvint FALLBACK unavailable (${ENVELOPE_COLLISION}); unrelated coding may continue.` }
      }
      return {
        content: query.disclosure + output,
        metadata: {
          corvint: {
            observedEvidenceHandles: suppliedEvidenceHandles(response),
            receiptId: response.receiptId,
          },
        },
      }
    },
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
        signal: context?.signal,
      })
      if (!response.ok) {
        return { content: `Corvint FALLBACK unavailable (${response.code}); unrelated coding may continue.` }
      }
      const output = frameRepositoryData(response)
      if (!output) {
        report(ENVELOPE_COLLISION, "session-end", response.receiptId)
        return { content: `Corvint FALLBACK unavailable (${ENVELOPE_COLLISION}); unrelated coding may continue.` }
      }
      return { content: output, metadata: { corvint: { receiptId: response.receiptId } } }
    },
  }

  const subscription = (async () => {
    try {
      for await (const event of ctx.event.subscribe({ signal: events.signal })) await onEvent(event)
    } catch {
      if (!events.signal.aborted) report("event-subscription-failed", "session-start")
    }
  })()
  await ctx.tool.hook("execute.after", afterTool)
  await ctx.tool.transform((editor) => {
    editor.add(corvintContext)
    editor.add(corvintRecordOutcome)
  })

  if (betaEnabled(options, environment)) {
    const { createBetaContextHook } = await import("./beta-hooks.js")
    await ctx.session.hook(
      "context",
      createBetaContextHook({
        startupContext: (sessionKey) => startupContexts.get(sessionKey),
        hashSessionId,
        report,
      }),
    )
  }

  return async () => {
    events.abort()
    await subscription
    if (fileChangeDrain) await fileChangeDrain
    pendingFileChanges.clear()
    sessions.clear()
    startupContexts.clear()
  }
}

export default { id: "corvint", setup }
