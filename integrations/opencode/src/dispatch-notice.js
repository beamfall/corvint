// DSG-V0-008 (GitHub #1821): a Corvint Tasks dispatcher worker that runs OpenCode gets its WARN
// and HANDOFF context notices in-session. The dispatcher hands the worker its context record as
// CORVINT_DISPATCH_CONTEXT_FILE; when that record changes, this module runs the same
// `corvint-tasks dispatch context-hook` the Claude Code PostToolUse hook runs (ATR-V0-101), which
// marks each delivered level exclusively, and adds every notice it returned to the system context
// of each later model request of the process.
import { spawn } from "node:child_process"
import { stat } from "node:fs/promises"

const MAX_NOTICES = 4
const MAX_NOTICE_BYTES = 2_000
const MAX_HOOK_OUTPUT = 64 << 10
const HOOK_TIMEOUT_MS = 5_000

export function runContextHook(binary, environment, timeoutMs = HOOK_TIMEOUT_MS) {
  return new Promise((resolve) => {
    let out = ""
    let done = false
    const finish = (value) => {
      if (done) return
      done = true
      clearTimeout(timer)
      resolve(value)
    }
    let child
    try {
      child = spawn(binary, ["dispatch", "context-hook"], { env: environment, stdio: ["pipe", "pipe", "ignore"] })
    } catch {
      return resolve("")
    }
    const timer = setTimeout(() => {
      child.kill("SIGKILL")
      finish("")
    }, timeoutMs)
    child.on("error", () => finish(""))
    child.stdout.on("data", (chunk) => {
      out += chunk
      if (out.length > MAX_HOOK_OUTPUT) {
        child.kill("SIGKILL")
        finish("")
      }
    })
    child.on("close", () => finish(out))
    child.stdin.on("error", () => {})
    child.stdin.end(JSON.stringify({ hook_event_name: "opencode.session.context" }))
  })
}

// noticeOf reads the additionalContext of the hook's one JSON line, or "".
export function noticeOf(output) {
  try {
    const text = JSON.parse(output)?.hookSpecificOutput?.additionalContext
    if (typeof text !== "string" || text.length === 0 || Buffer.byteLength(text, "utf8") > MAX_NOTICE_BYTES) return ""
    return text
  } catch {
    return ""
  }
}

// createDispatchNotices returns undefined outside a dispatcher worker.
export function createDispatchNotices({ environment, tasksBinary, run = runContextHook } = {}) {
  const record = environment?.CORVINT_DISPATCH_CONTEXT_FILE
  if (typeof record !== "string" || record.length === 0 || record.length > 4_096) return undefined
  const notices = []
  let seen = ""
  let pending
  const refresh = async () => {
    let info
    try {
      info = await stat(record)
    } catch {
      return
    }
    const stamp = `${info.mtimeMs}:${info.size}`
    if (stamp === seen) return
    seen = stamp
    for (const text of noticeOf(await run(tasksBinary, environment)).split("\n")) {
      if (text && !notices.includes(text)) notices.push(text)
    }
    if (notices.length > MAX_NOTICES) notices.splice(0, notices.length - MAX_NOTICES)
  }
  return {
    notices: () => [...notices],
    async inject(request) {
      if (!Array.isArray(request?.system)) return
      pending ??= refresh().finally(() => { pending = undefined })
      await pending
      for (const text of notices) {
        if (!request.system.some(item => item?.text === text)) request.system.push({ type: "text", text })
      }
    },
  }
}
