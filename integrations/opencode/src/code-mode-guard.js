// AHI-214 (V1-2753): OpenCode 2.1's Code Mode `execute` tool runs model-written JavaScript with a
// global `fetch`, so it reaches the network even when the host denies `webfetch` and `websearch`
// (the V1-2717 p07 session fetched proxy.golang.org through it). The plugin cannot remove `fetch`
// from the sandbox, so it refuses the whole `execute` call unless the host's effective permission
// rules allow both network tools without exception. Anything it cannot determine is refused.

export const NETWORK_ACTIONS = Object.freeze(["webfetch", "websearch"])
const EFFECTS = new Set(["allow", "ask", "deny"])

export const CODE_MODE_NETWORK_REFUSAL =
  "Corvint refused Code Mode execute (code-mode-network-denied): this host's permissions deny or " +
  "restrict webfetch/websearch, or could not be read, and Code Mode JavaScript can reach the network. " +
  "Use the direct tools instead; to remove Code Mode set permission execute to deny."

// OpenCode 2.1.3 permission wildcard: `*` matches any run, `?` one character, and a trailing ` *`
// also matches the bare command. Backslashes compare as slashes.
export function permissionMatches(value, pattern) {
  if (typeof value !== "string" || typeof pattern !== "string") return false
  let source = pattern.replaceAll("\\", "/").replace(/[.+^${}()|[\]\\]/g, "\\$&").replace(/\*/g, ".*").replace(/\?/g, ".")
  if (source.endsWith(" .*")) source = source.slice(0, -3) + "( .*)?"
  return new RegExp("^" + source + "$", "s").test(value.replaceAll("\\", "/"))
}

const wellFormed = rule =>
  rule !== null && typeof rule === "object" &&
  typeof rule.action === "string" && typeof rule.resource === "string" && EFFECTS.has(rule.effect)

// The host evaluates ordered rules and the last match wins; with no match the effect is `ask`.
// Network is permitted only when, for each network action, the last rule covering every resource
// (`*`) allows it and no later rule for that action denies or asks for any resource: Code Mode
// cannot enforce a per-URL rule, so a narrower restriction refuses too.
export function networkPermitted(rules) {
  if (!Array.isArray(rules) || !rules.every(wellFormed)) return false
  return NETWORK_ACTIONS.every(action => {
    const matching = rules.filter(rule => permissionMatches(action, rule.action))
    const last = matching.findLastIndex(rule => rule.resource === "*")
    return last >= 0 && matching.slice(last).every(rule => rule.effect === "allow")
  })
}

// The stock OpenCode 2.1.3 `ctx.agent.get` answers `{ location, data: agent }`, while `ctx.session.get`
// answers the session itself (observed in the V1-2804 native qualification); both shapes are read.
const body = value =>
  value !== null && typeof value === "object" && value.data !== null && typeof value.data === "object" && !Array.isArray(value.data)
    ? value.data
    : value

// The effective rules are the agent's followed by the session's, as OpenCode 2.1.3 composes them
// for the tool snapshot. Any read failure or missing shape leaves the rules undetermined.
export async function effectiveRules({ ctx, call }) {
  if (typeof ctx?.agent?.get !== "function" || typeof ctx?.session?.get !== "function") return undefined
  if (typeof call?.sessionID !== "string" || call.sessionID === "") return undefined
  try {
    const session = body(await ctx.session.get({ sessionID: call.sessionID }))
    const agentID = typeof call.agent === "string" && call.agent !== "" ? call.agent : session?.agent
    if (typeof agentID !== "string" || agentID === "") return undefined
    const agent = body(await ctx.agent.get({ agentID }))
    if (!Array.isArray(agent?.permissions)) return undefined
    if (session?.permissions !== undefined && !Array.isArray(session.permissions)) return undefined
    return [...agent.permissions, ...(session?.permissions ?? [])]
  } catch {
    return undefined
  }
}

// The `tool.hook("execute.before")` callback. OpenCode turns a thrown error into the tool call's
// failure, so the model sees the refusal and no Code Mode program runs. Inner tool calls made from
// an allowed program carry their own names and are not affected.
export function codeModeNetworkGuard(ctx, onRefuse = () => {}) {
  return async call => {
    if (call?.tool !== "execute") return
    if (networkPermitted(await effectiveRules({ ctx, call }))) return
    onRefuse()
    throw new Error(CODE_MODE_NETWORK_REFUSAL)
  }
}
