import { tool } from "@opencode-ai/plugin"
import {
  ADAPTER_VERSION,
  boundedPaths,
  boundedTask,
  createCorvintRunner,
  explicitEvidenceHandles,
  explicitVerification,
  hashSessionId,
  hashTask,
  normalizeRepositoryPath,
  suppliedEvidenceHandles,
} from "./runtime.js"
import { ENVELOPE_COLLISION, frameRepositoryData } from "./envelope.js"
import { promptQuery, trimSpace } from "./prompt-bound.js"

const MAX_TRACKED_PATHS = 256
const MAX_SESSIONS = 128

function eventSessionId(event) {
  const properties = event?.properties
  return properties?.sessionID ?? properties?.info?.id
}

function eventFile(event) {
  const properties = event?.properties
  return properties?.file ?? properties?.path
}

function betaEnabled(options, environment) {
  if (options?.enableBetaContext === true) return true
  return environment.CORVINT_OPENCODE_BETA_CONTEXT === "1"
}

export const CorvintPlugin = async (host, options = {}) => {
  const environment = options.environment ?? process.env
  const root = host.worktree || host.directory
  const runCorvint = createCorvintRunner({ ...options, environment })
  const sessions = new Map()
  const startupContexts = new Map()
  const pendingFileChanges = new Map()
  let fileChangeDrain

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

  // AHI-022 (decision 0161 parity): a routine receipt's degradations and an expected guard
  // go to OpenCode's own log through the documented client.app.log API, not the terminal.
  // Without that client the warning stays, so the code is still recorded somewhere.
  const record = (code, event, receiptId) => {
    const message = notice(code, event, receiptId)
    if (typeof host.client?.app?.log !== "function") {
      console.warn(message)
      return
    }
    Promise.resolve()
      .then(() => host.client.app.log({ body: { service: "corvint-opencode", level: "info", message } }))
      .catch(() => console.warn(message))
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
      if (request.event === "file-change" && code === "unsupported-impact-path-suffix") {
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

  const stableHooks = {
    dispose: async () => {
      if (fileChangeDrain) await fileChangeDrain
      pendingFileChanges.clear()
      sessions.clear()
      startupContexts.clear()
    },

    event: async ({ event }) => {
      try {
        if (event?.type === "session.created") {
          const bound = stateFor(eventSessionId(event))
          const input = bound ? { sessionIdSha256: bound.key } : {}
          const response = await runVisible({ event: "session-start", input })
          if (bound && response.ok && response.context) startupContexts.set(bound.key, response)
          return
        }
        if (event?.type === "file.edited") {
          const rawSessionId = eventSessionId(event)
          const changed = rememberPath(eventFile(event), rawSessionId)
          const sessionIdSha256 = hashSessionId(rawSessionId)
          if (changed) {
            await queueFileChange(changed, sessionIdSha256)
          }
          return
        }
        if (event?.type === "session.idle") {
          const bound = stateFor(eventSessionId(event))
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
          return
        }
        if (event?.type === "session.deleted") {
          const key = hashSessionId(eventSessionId(event))
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
      } catch {
        report("stable-event-adapter-failed", event?.type ?? "unknown")
      }
    },

    "tool.execute.after": async (input, output) => {
      try {
        const bound = stateFor(input?.sessionID)
        if (bound) bound.state.stopArmed = true
        const metadata = output?.metadata
        const changedPaths = boundedPaths(root, metadata?.corvint?.changedPaths)
        for (const changed of changedPaths) rememberPath(changed, input?.sessionID)
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
    },

    tool: {
      corvint_context: tool({
        description:
          "Request bounded, revision-pinned Corvint context for one explicit task. Returns a FALLBACK receipt and evidence; it does not prove completion.",
        args: {
          task: tool.schema.string(),
        },
        async execute(args, context) {
          // AHI-016: an over-bound task is served by its disclosed anchor query or refused.
          const prompt = typeof args.task === "string" ? trimSpace(args.task) : ""
          const query = prompt === "" ? undefined : promptQuery(prompt)
          if (!query) {
            return (
              "Corvint degraded (prompt-over-query-bound): task must be non-empty and either at most " +
              "2,000 Unicode characters and 16,384 UTF-8 bytes or carry explicit anchors within that bound."
            )
          }
          const task = query.task
          const sessionIdSha256 = hashSessionId(context?.sessionID)
          const response = await runVisible({
            event: "user-prompt",
            input: { ...(sessionIdSha256 ? { sessionIdSha256 } : {}), task },
            query: true,
            signal: context?.abort,
          })
          if (!response.ok) {
            return `Corvint FALLBACK unavailable (${response.code}); unrelated coding may continue.`
          }
          const output = frameRepositoryData(response)
          if (!output) {
            report(ENVELOPE_COLLISION, "user-prompt", response.receiptId)
            return `Corvint FALLBACK unavailable (${ENVELOPE_COLLISION}); unrelated coding may continue.`
          }
          return {
            title: `Corvint context (${ADAPTER_VERSION}, FALLBACK)`,
            output: query.disclosure + output,
            metadata: {
              corvint: {
                observedEvidenceHandles: suppliedEvidenceHandles(response),
                receiptId: response.receiptId,
              },
            },
          }
        },
      }),
      corvint_record_outcome: tool({
        description:
          "Record a caller-reported explicit Corvint task outcome with bounded change and verification observations. This does not verify correctness.",
        args: {
          changedPaths: tool.schema.array(tool.schema.string()),
          outcome: tool.schema.enum(["passed", "failed", "blocked"]),
          task: tool.schema.string(),
          verification: tool.schema.array(
            tool.schema.object({
              commandSha256: tool.schema.string(),
              status: tool.schema.enum(["passed", "failed", "not-run", "unknown"]),
            }),
          ),
        },
        async execute(args, context) {
          const task = boundedTask(args.task)
          const changedPaths = boundedPaths(root, args.changedPaths)
          const verification = explicitVerification({ corvint: { verification: args.verification } })
          if (!task || changedPaths.length === 0 || verification.length === 0) {
            return "Corvint degraded: an explicit bounded task, changed path, and verification observation are required."
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
            signal: context?.abort,
          })
          if (!response.ok) {
            return `Corvint FALLBACK unavailable (${response.code}); unrelated coding may continue.`
          }
          const output = frameRepositoryData(response)
          if (!output) {
            report(ENVELOPE_COLLISION, "session-end", response.receiptId)
            return `Corvint FALLBACK unavailable (${ENVELOPE_COLLISION}); unrelated coding may continue.`
          }
          return {
            title: `Corvint explicit outcome (${ADAPTER_VERSION}, FALLBACK)`,
            output,
            metadata: { corvint: { receiptId: response.receiptId } },
          }
        },
      }),
    },
  }

  if (betaEnabled(options, environment)) {
    const { createBetaContextHooks } = await import("./beta-hooks.js")
    Object.assign(
      stableHooks,
      createBetaContextHooks({
        consumeStartupContext(sessionKey) {
          const value = startupContexts.get(sessionKey)
          startupContexts.delete(sessionKey)
          return value
        },
        hashSessionId,
        report,
      }),
    )
  }

  return stableHooks
}

export default CorvintPlugin
