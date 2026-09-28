import { visibleText } from "./display.js"
export { visibleText } from "./display.js"
import { cockpitSchema } from "./cockpit.js"
// The view keeps Core's evidence labels and exact handles separate from terminal presentation.
const object = properties => ({ type: "object", properties, required: Object.keys(properties), additionalProperties: false })
const string = { type: "string" }
const strings = { type: "array", items: string }
const row = object({ title: string, summary: string, kind: string, path: string, line: { type: "integer" }, reason: string, authority: string, confidence: string, blob: string, handle: string })
export const snapshotSchema = object({ state: string, reason: string, receiptId: string, revision: string, freshness: string, support: string, rows: { type: "array", items: row }, gaps: strings })
const session = { type: "string", minLength: 1, maxLength: 256 }
export const INSPECTOR_RPC = {
  id: "corvint.inspector",
  methods: {
    cockpitSnapshot: { input: object({ sessionID: session }), output: cockpitSchema },
    cockpitRefresh: { input: object({ sessionID: session, base: { type: "string", maxLength: 256 } }), output: cockpitSchema },
    cockpitProof: { input: object({ sessionID: session, receiptId: { type: "string", maxLength: 128 }, checkID: { type: "string", maxLength: 256 } }), output: object({ state: string, text: string }) },
    snapshot: { input: object({ sessionID: session }), output: snapshotSchema },
    query: { input: object({ sessionID: session, task: { type: "string", minLength: 1, maxLength: 16384 } }), output: snapshotSchema },
    expand: { input: object({ sessionID: session, receiptId: string, handle: { type: "string", maxLength: 1024 } }), output: object({ state: string, text: string }) },
  },
  events: { updated: { schema: object({ sessionIdSha256: string }) } },
}


export function emptySnapshot(state = "empty", reason = "Request context for this session to see its evidence.") {
  return { state, reason, receiptId: "", revision: "", freshness: "unknown", support: "FALLBACK", rows: [], gaps: [] }
}

export function projectReceipt(response, handles = []) {
  if (!response?.ok || !response.context) return emptySnapshot("unavailable", visibleText(response?.code || "context-unavailable"))
  const context = response.context
  const snapshot = { ...emptySnapshot("ready", ""), receiptId: response.receiptId || "", revision: response.repository?.treeRevision || context.revision || "", freshness: context.freshness?.state || response.repository?.worktreeState || "unknown" }
  const coverage = context.coverage || {}
  snapshot.gaps = [
    ...(Array.isArray(response.degradations) ? response.degradations : []),
    ...(Array.isArray(coverage.uncertainty) ? coverage.uncertainty : []),
    ...(Array.isArray(coverage.critical_missing) ? coverage.critical_missing.map(x => `Missing: ${typeof x === "string" ? x : JSON.stringify(x)}`) : []),
    ...(coverage.omitted_results > 0 ? [`${coverage.omitted_results} results omitted by Core's budget.`] : []),
    ...(context.abstention?.active ? [`Core abstained: ${context.abstention.reason}`] : []),
  ].map(x => visibleText(x))
  let evidenceCount = 0
  for (const result of Array.isArray(context.results) ? context.results : []) {
    const evidence = Array.isArray(result.evidence) ? result.evidence : []
    for (const e of evidence) {
      evidenceCount++
      if (snapshot.rows.length >= 32) continue
      const handle = `cv1:${snapshot.revision}:${e.blob_hash}:all:${e.path}`
      snapshot.rows.push({ title: visibleText(result.title || result.name || result.id), summary: visibleText(result.summary), kind: visibleText(result.kind), path: visibleText(e.path), line: Number.isSafeInteger(e.line) && e.line > 0 ? e.line : 0, reason: visibleText(e.reason), authority: visibleText(e.authority || "unknown"), confidence: visibleText(e.confidence || "unknown"), blob: visibleText(e.blob_hash), handle: handles.includes(handle) ? handle : "" })
    }
    if (!evidence.length) snapshot.gaps.push(`No evidence location: ${visibleText(result.title || result.id)}`)
  }
  if (evidenceCount > snapshot.rows.length) snapshot.gaps.push(`${evidenceCount - snapshot.rows.length} evidence locations omitted by the inspector's 32-row limit.`)
  if (snapshot.gaps.length > 32) snapshot.gaps = [...snapshot.gaps.slice(0, 31), `${snapshot.gaps.length - 31} further gaps omitted from this view.`]
  return snapshot
}

export function invalidateInspection(state, reason) {
  state.cockpitGeneration = (state.cockpitGeneration || 0) + 1
  state.cockpitRequest?.abort()
  state.cockpitBinding = undefined
  if (state.cockpit) state.cockpit = { ...state.cockpit, state: "stale", reason }
  state.inspectorGeneration = (state.inspectorGeneration || 0) + 1
  if (state.inspection) state.inspection = { ...state.inspection, state: "stale", reason }
}

export function beginInspection(state) {
  invalidateInspection(state, "A newer context request has started.")
  state.inspection = { ...(state.inspection || emptySnapshot()), state: "loading", reason: "Collecting context…" }
  return state.inspectorGeneration
}

export function completeInspection(state, generation, response, handles) {
  if (!state.active || state.inspectorGeneration !== generation) return false
  state.inspection = projectReceipt(response, handles)
  return true
}

export function inspectionMatches(state, receiptId, handle) {
  return state?.active && state.inspection?.state === "ready" && state.inspection.receiptId === receiptId && state.inspection.rows.some(row => row.handle !== "" && row.handle === handle)
}
