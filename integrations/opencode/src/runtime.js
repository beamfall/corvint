import { createHash } from "node:crypto"
import { spawn } from "node:child_process"
import { realpathSync, statSync } from "node:fs"
import path from "node:path"
import { StringDecoder } from "node:string_decoder"
import { overQueryBound, trimSpace } from "./prompt-bound.js"

export const ADAPTER_VERSION = "0.7.6"
export const PROTOCOL = "corvint-harness-event/0"
export const SUPPORT = "FALLBACK"

const MAX_INPUT_BYTES = 131_072
const MAX_OUTPUT_BYTES = 8_000
const MAX_STDERR_BYTES = 4_096
const MAX_ITEMS = 256
const MAX_PATH_CHARS = 1_024
const MAX_VALUE_CHARS = 512
// Complete-command hang detector, distinct from AHI-012's 250 ms p95 target.
// The former 500 ms default killed valid file-change receipts; use the existing
// automatic-event ceiling without extending the host's bounded wait.
const AUTOMATIC_TIMEOUT_MS = 2_000
const MAX_AUTOMATIC_TIMEOUT_MS = 2_000
const QUERY_TIMEOUT_MS = 2_000
const MAX_QUERY_TIMEOUT_MS = 10_000
const MAX_ENVELOPE_NODES = 2_048
const RECEIPT_PREFIX = "harness-receipt:sha256:"
// Receipt degradation codes this adapter is validated to carry through. A receipt
// may name any subset, including none: the set shrinks whenever core closes a gap
// (the frontier authority being the standing example), and requiring an exact list
// would turn every such closure into an adapter break. An unrecognised code is a
// contract the adapter has not been validated against, so it refuses loudly rather
// than passing an unreviewed claim into the session. Declared centrally as
// receiptDegradationPolicy in integrations/compatibility.json.
export const RECOGNISED_DEGRADATIONS = new Set([
  "compaction-critical-evidence-overflow",
  "compaction-dirty-set-over-budget",
  "compaction-untracked-paths-not-rehydratable",
  "frontier-authority-unavailable",
  "host-version-unknown",
  "outcome-persistence-unavailable",
])
const ALLOWED_ENV = [
  "HOME",
  "LANG",
  "LC_ALL",
  "LC_CTYPE",
  "LOGNAME",
  "NO_COLOR",
  "PATH",
  "TEMP",
  "TMP",
  "TMPDIR",
  "USER",
  "SYSTEMROOT",
  "WINDIR",
]

function utf8Bytes(value) {
  return Buffer.byteLength(value, "utf8")
}

function boundedInteger(value, fallback, maximum) {
  const parsed = Number(value)
  if (!Number.isInteger(parsed) || parsed < 25 || parsed > maximum) return fallback
  return parsed
}

function boundedToken(value, fallback = "unknown") {
  if (typeof value !== "string" || !/^[A-Za-z0-9][A-Za-z0-9._+-]{0,127}$/.test(value)) {
    return fallback
  }
  return value
}

// Decision 0178: only a confirmed absence of .git at every level is outside a repository; an
// unreadable level counts as inside, so its failure keeps its fault notice.
export function insideGitRepository(directory) {
  let current
  try {
    current = realpathSync(directory)
  } catch {
    current = path.resolve(directory)
  }
  for (; ; current = path.dirname(current)) {
    try {
      statSync(path.join(current, ".git"))
      return true
    } catch (error) {
      if (error?.code !== "ENOENT") return true
    }
    if (path.dirname(current) === current) return false
  }
}

function childEnvironment(source) {
  const result = {}
  for (const key of ALLOWED_ENV) {
    if (typeof source[key] === "string") result[key] = source[key]
  }
  return result
}

function executable(value, fallback = "corvint") {
  if (typeof value !== "string" || value.length === 0 || value.length > 4_096 || value.includes("\0")) {
    return fallback
  }
  return value
}

export function hashSessionId(value) {
  if (typeof value !== "string" || value.length === 0) return undefined
  return createHash("sha256").update(value, "utf8").digest("hex")
}

export function hashTask(value) {
  if (typeof value !== "string" || value.length === 0) return undefined
  return createHash("sha256").update(value, "utf8").digest("hex")
}

export function normalizeRepositoryPath(root, value) {
  if (typeof value !== "string" || value.length === 0 || value.includes("\0")) return undefined
  const relative = path.isAbsolute(value) ? path.relative(root, value) : value
  const normalized = path.normalize(relative)
  if (
    path.isAbsolute(normalized) ||
    normalized === "." ||
    normalized === ".." ||
    normalized.startsWith(`..${path.sep}`)
  ) {
    return undefined
  }
  const posix = normalized.split(path.sep).join("/")
  if (posix.length > MAX_PATH_CHARS) return undefined
  return posix
}

export function boundedPaths(root, values) {
  if (!Array.isArray(values)) return []
  const result = []
  const seen = new Set()
  for (const value of values.slice(0, MAX_ITEMS)) {
    const normalized = normalizeRepositoryPath(root, value)
    if (normalized && !seen.has(normalized)) {
      seen.add(normalized)
      result.push(normalized)
    }
  }
  return result
}

export function explicitEvidenceHandles(metadata) {
  const values = metadata?.corvint?.observedEvidenceHandles
  if (!Array.isArray(values)) return []
  const result = []
  const seen = new Set()
  for (const value of values.slice(0, MAX_ITEMS)) {
    if (
      typeof value === "string" &&
      value.length > 0 &&
      value.length <= MAX_VALUE_CHARS &&
      !seen.has(value)
    ) {
      seen.add(value)
      result.push(value)
    }
  }
  return result
}

export function explicitVerification(metadata) {
  const values = metadata?.corvint?.verification
  if (!Array.isArray(values)) return []
  const result = []
  const seen = new Set()
  for (const value of values.slice(0, MAX_ITEMS)) {
    if (
      value &&
      typeof value === "object" &&
      Object.keys(value).sort().join(",") === "commandSha256,status" &&
      /^[0-9a-f]{64}$/.test(value.commandSha256) &&
      ["passed", "failed", "not-run", "unknown"].includes(value.status)
    ) {
      const key = `${value.commandSha256}:${value.status}`
      if (!seen.has(key)) {
        seen.add(key)
        result.push({ commandSha256: value.commandSha256, status: value.status })
      }
    }
  }
  return result
}

export function suppliedEvidenceHandles(envelope) {
  const result = []
  const seen = new Set()
  const rows = Array.isArray(envelope?.context?.results) ? envelope.context.results : []
  for (const row of rows) {
    const evidence = Array.isArray(row?.evidence) ? row.evidence : []
    for (const item of evidence) {
      const handle = item?.handle
      if (
        typeof handle === "string" &&
        handle.length > 0 &&
        handle.length <= MAX_VALUE_CHARS &&
        !seen.has(handle) &&
        result.length < MAX_ITEMS
      ) {
        seen.add(handle)
        result.push(handle)
      }
    }
  }
  return result
}

function safeCode(value) {
  return typeof value === "string" && /^[a-z0-9][a-z0-9-]{0,63}$/u.test(value) ? value : undefined
}

/**
 * Corvint's own nonzero-exit failure line is a bounded `{"code": ..., "error": ...}`
 * object on stderr (cmd/corvint emitError). Recovering that code lets a failure
 * surface as its real cause instead of the generic corvint-command-failed code.
 */
function stderrFailureCode(stderrText) {
  try {
    return safeCode(JSON.parse(stderrText)?.code)
  } catch {
    return undefined
  }
}

function degradation(event, code, deadlineMs) {
  return {
    code,
    event,
    ok: false,
    profile: PROTOCOL,
    support: SUPPORT,
    ...(code === "timeout" ? { deadlineMs } : {}),
  }
}

function canonicalJson(value) {
  if (value === null || typeof value === "boolean" || typeof value === "string") {
    return JSON.stringify(value)
  }
  if (typeof value === "number" && Number.isFinite(value)) return JSON.stringify(value)
  if (Array.isArray(value)) return `[${value.map(canonicalJson).join(",")}]`
  if (value && typeof value === "object") {
    const fields = Object.keys(value)
      .sort()
      .map((key) => `${JSON.stringify(key)}:${canonicalJson(value[key])}`)
    return `{${fields.join(",")}}`
  }
  throw new TypeError("value is not canonical JSON")
}

function receiptIdentity(value, event, input) {
  try {
    const basis = canonicalJson({ adapter: value.adapter, event, input, repository: value.repository })
    return RECEIPT_PREFIX + createHash("sha256").update(basis, "utf8").digest("hex")
  } catch {
    return undefined
  }
}

function containsTaskEcho(value, task) {
  if (typeof task !== "string" || task.length === 0) return false
  const pending = [value]
  let visited = 0
  while (pending.length > 0) {
    const current = pending.pop()
    visited += 1
    if (visited > MAX_ENVELOPE_NODES) return true
    if (typeof current === "string") {
      if (current === task) return true
      continue
    }
    if (!current || typeof current !== "object") continue
    if (Array.isArray(current)) {
      for (const item of current) pending.push(item)
      continue
    }
    for (const item of Object.values(current)) pending.push(item)
  }
  return false
}

function validEnvelope(value, event, hostVersion, input) {
  let contextBytes = 0
  try {
    contextBytes = value?.context === undefined ? 0 : utf8Bytes(JSON.stringify(value.context))
  } catch {
    return false
  }
  return Boolean(
    value &&
      typeof value === "object" &&
      value.profile === PROTOCOL &&
      value.ok === true &&
      value.support === SUPPORT &&
      value.event === event &&
      value.adapter?.host === "opencode" &&
      value.adapter?.surface === "plugin" &&
      value.adapter?.adapterVersion === ADAPTER_VERSION &&
      value.adapter?.hostVersion === hostVersion &&
      typeof value.receiptId === "string" &&
      /^harness-receipt:sha256:[0-9a-f]{64}$/.test(value.receiptId) &&
      value.repository &&
      typeof value.repository === "object" &&
      !Array.isArray(value.repository) &&
      contextBytes <= 8_000 &&
      value.receiptId === receiptIdentity(value, event, input) &&
      (event !== "user-prompt" || !containsTaskEcho(value, input?.task)) &&
      (event !== "stop" ||
        (value.frontier?.state === "UNAVAILABLE" && value.frontier?.shouldContinue === false))
  )
}

/** Accept any subset of the recognised codes; name the fault for an unreviewed one. */
function degradationsFault(value) {
  if (!Array.isArray(value) || value.length > MAX_ITEMS) return "corvint-degradations-invalid"
  if (!value.every((item) => typeof item === "string")) return "corvint-degradations-invalid"
  const named = new Set(value)
  if (named.size !== value.length) return "corvint-degradations-invalid"
  // The offending code is by definition unvalidated text, so it is named by the
  // corvint invocation rather than interpolated into the session message.
  for (const code of named) {
    if (!RECOGNISED_DEGRADATIONS.has(code)) return "corvint-degradations-unrecognised"
  }
  return undefined
}

function envelopeFault(value, event, hostVersion, input) {
  if (!validEnvelope(value, event, hostVersion, input)) return "incompatible-corvint-output"
  return degradationsFault(value.degradations)
}

// Internal read kinds have fixed argv; the RPC surface never accepts a command or executable.
export function cockpitReadArguments(root, read) {
  const oid = value => /^(?:[0-9a-f]{40}|[0-9a-f]{64})$/.test(value ?? "")
  if (read?.kind === "tasks-status") return { tasks: true, args: ["queue", "status"] }
  if (read?.kind === "tasks-open" && Number.isInteger(read.offset) && read.offset >= 0 && read.offset <= 100_000) return { tasks: true, args: ["ticket", "search", "--status", "OPEN", "--offset", String(read.offset), "--limit", "32"] }
  if (read?.kind === "tasks-detail" && typeof read.ticketId === "string" && read.ticketId.length <= 128 && /^ticket:[A-Za-z0-9_-]+:[A-Za-z0-9_-]+:[A-Za-z0-9_-]+$/.test(read.ticketId)) return { tasks: true, args: ["ticket", "show", read.ticketId] }
  if (read?.kind === "repository") return { git: true, args: ["-C", root, "rev-parse", "--show-toplevel", "--absolute-git-dir", "HEAD", "HEAD^{tree}"] }
  if (read?.kind === "resolve" && typeof read.ref === "string" && read.ref.length > 0 && read.ref.length <= 256 && !/[\x00-\x20\x7f]/u.test(read.ref)) return { git: true, args: ["-C", root, "rev-parse", "--verify", "--end-of-options", `${read.ref}^{commit}`] }
  if (read?.kind === "affected" && oid(read.base)) return { args: ["--root", root, "affected", "--base", read.base] }
  if (read?.kind === "completion" && /^[0-9a-f]{64}$/.test(read.key ?? "")) return { args: ["--root", root, "dogfood", "status", "--session-key", read.key] }
  return undefined
}

function cockpitReadResult(kind, output) {
  const oid = value => /^(?:[0-9a-f]{40}|[0-9a-f]{64})$/.test(value ?? "")
  if (kind === "repository") {
    const lines = output.trimEnd().split("\n")
    if (lines.length === 4 && lines.slice(0, 2).every(x => path.isAbsolute(x) && !/[\x00-\x1f]/u.test(x)) && lines.slice(2).every(oid)) return { ok: true, root: lines[0], gitdir: lines[1], target: lines[2], tree: lines[3] }
  } else if (kind === "resolve") {
    if (oid(output.trim())) return { ok: true, commit: output.trim() }
  } else {
    const value = JSON.parse(output)
    if (kind.startsWith("tasks-")) {
      const command = kind === "tasks-status" ? ["queue", "status"] : kind === "tasks-open" ? ["ticket", "search"] : ["ticket", "show"]
      if (value?.profile !== "taskman-command-result/0" || value.outcome !== "OK" || !Array.isArray(value.items) ||
          JSON.stringify(value.command) !== JSON.stringify(command) ||
          !/^[0-9a-f]{64}$/.test(value.snapshot?.headReceiptSha256 ?? "")) return undefined
      if (kind === "tasks-status" && value.items.length !== 1) return undefined
      if (kind === "tasks-detail" && value.items.length !== 1) return undefined
      if (kind === "tasks-open" && (value.items.length > 32 || !Number.isSafeInteger(Number(value.page?.total)) || !Number.isSafeInteger(Number(value.page?.offset)))) return undefined
      return { ok: true, envelope: value }
    }
    if (value?.ok !== true || value.mutates !== false) return undefined
    if (kind === "affected" && value.profile === "affected-plan/0" && oid(value.revision) && Array.isArray(value.plan?.dirty) && Array.isArray(value.plan?.selected) && Array.isArray(value.plan?.unknown) && Array.isArray(value.advice?.checks)) return value
    if (kind === "completion" && value.profile === "corvint-local-completion/0" && value.tool === "dogfood-status" && typeof value.policy?.lifecycle === "string" && Array.isArray(value.policy.unmet)) return value
  }
}

export function createCorvintRunner(options = {}) {
  const environment = options.environment ?? process.env
  const binary = executable(options.corvintBinary ?? environment.CORVINT_BIN)
  const tasksBinary = executable(options.tasksBinary ?? environment.CORVINT_TASKS_BIN ?? "corvint-tasks", "corvint-tasks")
  const hostVersion = boundedToken(options.hostVersion ?? environment.CORVINT_OPENCODE_HOST_VERSION)
  const automaticTimeoutMs = boundedInteger(
    options.automaticTimeoutMs ?? environment.CORVINT_OPENCODE_TIMEOUT_MS,
    AUTOMATIC_TIMEOUT_MS,
    MAX_AUTOMATIC_TIMEOUT_MS,
  )
  const queryTimeoutMs = boundedInteger(
    options.queryTimeoutMs ?? environment.CORVINT_OPENCODE_QUERY_TIMEOUT_MS,
    QUERY_TIMEOUT_MS,
    MAX_QUERY_TIMEOUT_MS,
  )
  const platform = options.platform ?? process.platform

  return async function runCorvint({ root, event, input, query = false, signal, expand, read, budgetBytes = 8000 }) {
    if (platform === "win32") return degradation(event, "unsupported-process-tree-cleanup")
    if (expand !== undefined && (typeof expand !== "string" || expand.length > 1024 || !/^cv1:[0-9a-f]{40,64}:[0-9a-f]{40,64}:(?:all|[1-9][0-9]*-[1-9][0-9]*):[^\0\r\n]+$/.test(expand))) {
      return degradation(event, "invalid-expansion-handle")
    }
    const reading = read === undefined ? undefined : cockpitReadArguments(root, read)
    if (read !== undefined && (!reading || expand !== undefined)) return degradation(event, "invalid-cockpit-read")
    if (signal?.aborted) return degradation(event, "host-aborted")
    let serialized
    try {
      serialized = JSON.stringify(input)
    } catch {
      return degradation(event, "input-not-serializable")
    }
    if (utf8Bytes(serialized) > MAX_INPUT_BYTES) {
      return degradation(event, "input-too-large")
    }

    const args = reading ? reading.args : expand !== undefined ? ["--root", root, "context", "--expand", expand, "--max-bytes", "6000"] : [
      "--root",
      root,
      "harness",
      "event",
      "--host",
      "opencode",
      "--host-version",
      hostVersion,
      "--surface",
      "plugin",
      "--adapter-version",
      ADAPTER_VERSION,
      "--event",
      event,
      "--input",
      "-",
      "--budget-bytes",
      String(budgetBytes === 7000 ? 7000 : 8000),
    ]
    const timeoutMs = reading ? MAX_QUERY_TIMEOUT_MS : query ? queryTimeoutMs : automaticTimeoutMs

    return await new Promise((resolve) => {
      let settled = false
      let output = ""
      const decoder = new StringDecoder("utf8")
      let outputBytes = 0
      let overflow = false
      let stderrOutput = ""
      let stderrBytes = 0
      let forceTimer
      let reapTimer
      let terminationCode
      let leaderExited = false
      const child = spawn(reading?.tasks ? tasksBinary : reading?.git ? executable(options.gitBinary ?? "git") : binary, args, {
        cwd: root,
        detached: true,
        env: childEnvironment(environment),
        shell: false,
        stdio: ["pipe", "pipe", "pipe"],
        windowsHide: true,
      })
      // The leader's close does not end its group: a same-group descendant that ignored SIGTERM and
      // closed its stdio still runs, so every completion first kills the owned group (V1-0371). A
      // delivered SIGKILL or ESRCH confirms cleanup; Darwin also answers EPERM for a group whose
      // members have all exited but are not yet reaped, which is benign only after the leader's exit.
      const groupCleaned = () => {
        if (!Number.isInteger(child.pid)) return true
        try {
          process.kill(-child.pid, "SIGKILL")
          return true
        } catch (error) {
          return error?.code === "ESRCH" || (leaderExited && error?.code === "EPERM")
        }
      }
      const finish = (value) => {
        if (settled) return
        settled = true
        clearTimeout(timer)
        clearTimeout(forceTimer)
        clearTimeout(reapTimer)
        if (signal) signal.removeEventListener("abort", abort)
        resolve(groupCleaned() ? value : degradation(event, "corvint-process-cleanup-unconfirmed"))
      }
      const killGroup = (signalName) => {
        if (!Number.isInteger(child.pid)) return
        try {
          process.kill(-child.pid, signalName)
        } catch {
          try {
            child.kill(signalName)
          } catch {
            // The process already exited and will be reaped by its close event.
          }
        }
      }
      const terminate = (code) => {
        if (terminationCode) return
        terminationCode = code
        killGroup("SIGTERM")
        forceTimer = setTimeout(() => killGroup("SIGKILL"), 25)
        forceTimer.unref?.()
        reapTimer = setTimeout(() => finish(degradation(event, code, timeoutMs)), 100)
        reapTimer.unref?.()
      }
      const abort = () => terminate("host-aborted")
      const timer = setTimeout(() => terminate("timeout"), timeoutMs)
      timer.unref?.()
      if (signal) {
        if (signal.aborted) abort()
        else signal.addEventListener("abort", abort, { once: true })
      }
      child.on("error", () => finish(degradation(event, reading?.tasks ? "task-manager-unavailable" : "corvint-unavailable")))
      child.once("exit", () => {
        leaderExited = true
      })
      child.stdout.on("data", (chunk) => {
        outputBytes += chunk.length
        if (outputBytes > (reading ? 1_048_576 : MAX_OUTPUT_BYTES)) {
          overflow = true
          terminate("output-too-large")
          return
        }
        output += decoder.write(chunk)
      })
      child.stderr.on("data", (chunk) => {
        // Bounded capture, drained past the bound so a noisy failure cannot apply
        // backpressure to the child; see stderrFailureCode above.
        if (stderrBytes < MAX_STDERR_BYTES) {
          stderrOutput += chunk.toString("utf8")
          stderrBytes += chunk.length
        }
      })
      child.on("close", (code) => {
        if (settled) return
        if (terminationCode) {
          finish(degradation(event, terminationCode, timeoutMs))
          return
        }
        if (code !== 0) {
          let tasksCode
          if (reading?.tasks) {
            try {
              const failure = JSON.parse(output)
              if (failure.profile === "taskman-command-result/0" && failure.outcome === "REFUSED" && /^[A-Z][A-Z0-9_]{0,63}$/.test(failure.codes?.[0] ?? "")) tasksCode = `tasks-${failure.codes[0].toLowerCase().replaceAll("_", "-")}`
            } catch {}
          }
          finish(degradation(event, tasksCode ?? stderrFailureCode(stderrOutput) ?? (reading?.tasks ? "task-manager-command-failed" : "corvint-command-failed")))
          return
        }
        output += decoder.end()
        if (reading) {
          let value
          try { value = cockpitReadResult(read.kind, output) } catch {}
          finish(value || degradation(event, "malformed-cockpit-output"))
          return
        }
        let envelope
        try {
          envelope = JSON.parse(output)
        } catch {
          finish(degradation(event, "malformed-corvint-output"))
          return
        }
        if (expand !== undefined) {
          const selection = envelope?.selection
          const valid = envelope?.ok === true && envelope.mutates === false && envelope.handle === expand &&
            envelope.schema_version === 1 && envelope.view === "experimental-evidence-expand/0" &&
            envelope.source?.blob_verified === true && typeof selection?.text === "string" &&
            selection.bytes === utf8Bytes(selection.text) &&
            selection.sha256 === createHash("sha256").update(selection.text, "utf8").digest("hex")
          finish(valid ? envelope : degradation(event, "malformed-expansion-output"))
          return
        }
        const fault = envelopeFault(envelope, event, hostVersion, input)
        finish(fault === undefined ? envelope : degradation(event, fault))
      })
      child.stdin.on("error", () => undefined)
      child.stdin.end(serialized)
    })
  }
}

// The session-end task is trimmed with Go strings.TrimSpace whitespace before the
// bound check, as cmd/corvint/prompt_bound.go does for a prompt, so the digest
// and the bound see the same text on every host.
export function boundedTask(value) {
  if (typeof value !== "string") return undefined
  const task = trimSpace(value)
  if (!task || overQueryBound(task)) return undefined
  return task
}
