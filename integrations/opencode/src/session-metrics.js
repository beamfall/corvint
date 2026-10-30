import { visibleText } from "./display.js"

const finite = value => typeof value === "number" && Number.isFinite(value) && value >= 0 ? value : null
const value = input => input === null ? "NOT_OBSERVED" : String(input)

export function sessionEconomics(session, baseline) {
  const cost = finite(session?.cost)
  const tokens = session?.tokens
  const result = {
    cost: cost === null ? "NOT_OBSERVED" : `$${cost.toFixed(4)}`,
    ticketCost: "NOT_OBSERVED",
    input: value(finite(tokens?.input)), output: value(finite(tokens?.output)),
    reasoning: value(finite(tokens?.reasoning)), cacheRead: value(finite(tokens?.cache?.read)),
    cacheWrite: value(finite(tokens?.cache?.write)),
    note: "OpenCode session totals; no task attribution or savings claim.",
  }
  if (cost !== null && baseline && finite(baseline.cost) !== null && baseline.cost <= cost && baseline.ticketId) {
    result.ticketCost = `$${(cost - baseline.cost).toFixed(4)}`
    result.note = `Observed session cost since ${visibleText(baseline.ticketId, 128)} was focused in this TUI. Other work after focus may be included; verified-criterion cost is NOT_OBSERVED.`
  }
  return result
}

export function focusedEconomics(session, baseline, ticketId, directory) {
  return sessionEconomics(session, baseline?.ticketId === ticketId && baseline?.directory === directory ? baseline : undefined)
}

export function sessionMap(currentID, familyIDs, sessions, focusRows) {
  const allIDs = [...new Set([currentID, ...(Array.isArray(familyIDs) ? familyIDs : [])].filter(id => typeof id === "string" && id.length <= 256))]
  const ids = allIDs.slice(0, 16)
  const focus = new Map((Array.isArray(focusRows) ? focusRows : []).map(row => [row.sessionID, row]))
  const rows = ids.map(id => {
    const session = sessions(id), row = focus.get(id)
    return {
      id: visibleText(id.slice(-12), 12), current: id === currentID,
      parent: visibleText(session?.parentID?.slice(-12) || "root", 12),
      directory: visibleText(session?.directory || "NOT_OBSERVED", 256),
      ticketId: visibleText(row ? row.ticketId || "UNBOUND" : "UNOBSERVED", 128),
      state: visibleText(row?.state || "UNOBSERVED", 40),
      holder: visibleText(row?.holder || "NOT_OBSERVED", 128),
      phase: visibleText(row?.phase || "NOT_OBSERVED", 80),
      expiresAt: visibleText(row?.expiresAt || "NOT_OBSERVED", 80),
    }
  })
  const duplicates = [...new Set(rows.filter(row => row.ticketId !== "UNBOUND" && row.ticketId !== "UNOBSERVED" && rows.filter(other => other.ticketId === row.ticketId).length > 1).map(row => row.ticketId))]
  return { rows, duplicates, omitted: allIDs.length - ids.length }
}
