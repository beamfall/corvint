import test from "node:test"
import assert from "node:assert/strict"
import { emptyWorkbench, projectWorkbench } from "./src/workbench.js"
import { focusedEconomics, sessionEconomics, sessionMap } from "./src/session-metrics.js"
import { INSPECTOR_RPC } from "./src/inspector.js"

const ticketId = "ticket:corvint:main:V1-0042"
const detail = () => ({ state: "ready", ticketId, title: "Build proof map", revision: "3", queueDigest: "a".repeat(64),
  eligibility: "BLOCKED", nextAction: "resolve-blocker", gateResults: "NOT_OBSERVED", completion: "NOT_OBSERVED",
  criteria: ["AHI-036 links explicit proof", "No invented certainty"], requirementRefs: ["AHI-036"],
  touchPaths: ["integrations/opencode/src/workbench.js"], blockers: [{ code: "MISSING_PROOF", detail: "Criterion has no receipt" }],
  attempt: { holder: "agent-1", phase: "CHECKING", expiresAt: "2026-09-30T00:00:00Z", attemptId: "attempt-1" }, gaps: [] })
const tasks = () => ({ state: "ready", queueDigest: "a".repeat(64) })
const cockpit = () => ({ state: "ready", receiptId: "change-1", workflow: "Workflow active · obligations open",
  files: ["integrations/opencode/src/workbench.js"], checks: [{ id: "unit", status: "STALE", testedCommit: "b".repeat(40) }],
  declarations: [{ command: "node --test", kind: "suggested" }] })

test("AHI-037 AHI-038 AHI-039 proof, doctor and next actions retain their distinct evidence", () => {
  const view = projectWorkbench(detail(), tasks(), cockpit(), { state: "ready", receiptId: "context-1" }, {
    integrationSupport: "UNQUALIFIED", reason: "qualification-package-changed", hostVersion: "2.0.18",
    adapterVersion: "0.7.5", operatingSystem: "darwin", architecture: "arm64", qualificationAction: "run qualifier", failedCases: ["cleanup"],
  })
  assert.equal(view.state, "ready")
  assert.deepEqual(view.criteria.map(row => row.status), ["UNOBSERVED", "UNOBSERVED"])
  assert.deepEqual(view.criteria[0].requirements, ["AHI-036"])
  assert.deepEqual(view.changedPaths, ["integrations/opencode/src/workbench.js"])
  assert.equal(view.observedChecks[0].status, "STALE")
  assert.equal(view.doctor.failedCases[0], "cleanup")
  assert.match(view.actions.map(row => row.text).join("\n"), /MISSING_PROOF|stale check|qualification/)
  assert.equal(view.attempt.holder, "agent-1")
})

test("AHI-036 queue drift refuses the joined workbench and never shows zero as proof", () => {
  const value = tasks(); value.queueDigest = "b".repeat(64)
  const result = projectWorkbench(detail(), value, cockpit(), {}, null)
  assert.equal(result.state, "stale")
  assert.equal(result.criteria.length, 0)
  assert.match(result.reason, /Refresh Tasks/)
  assert.equal(emptyWorkbench().doctor.support, "UNQUALIFIED")
})

test("AHI-040 session cost attribution needs an explicit local baseline", () => {
  const session = { cost: 1.75, tokens: { input: 100, output: 20, reasoning: 5, cache: { read: 30, write: 0 } } }
  assert.equal(sessionEconomics(session).ticketCost, "NOT_OBSERVED")
  const focused = sessionEconomics(session, { ticketId, cost: 1.25 })
  assert.equal(focused.ticketCost, "$0.5000")
  assert.equal(focused.cacheRead, "30")
  assert.match(focused.note, /Other work after focus may be included/)
  assert.equal(sessionEconomics({ cost: Number.NaN }, { ticketId, cost: 1 }).cost, "NOT_OBSERVED")
  assert.equal(focusedEconomics(session, { ticketId, cost: 1, directory: "/repo" }, ticketId, "/other").ticketCost, "NOT_OBSERVED")
  assert.equal(focusedEconomics(session, { ticketId, cost: 1, directory: "/repo" }, "another", "/repo").ticketCost, "NOT_OBSERVED")
})

test("AHI-041 session family reports only explicit focus and detects overlaps", () => {
  const map = sessionMap("session-one", ["session-one", "session-two"], id => ({ directory: `/tmp/${id}`, parentID: id === "session-two" ? "session-one" : undefined }), [
    { sessionID: "session-one", ticketId, state: "ready" }, { sessionID: "session-two", ticketId, state: "ready" },
  ])
  assert.equal(map.rows.length, 2)
  assert.deepEqual(map.duplicates, [ticketId])
  assert.equal(sessionMap("session-one", ["session-two"], () => ({}), []).rows[1].ticketId, "UNOBSERVED")
  assert.deepEqual(sessionMap("session-one", ["session-two"], () => ({}), []).duplicates, [])
  assert.equal(sessionMap("session-one", ["session-one", "session-two"], () => ({}), []).omitted, 0)
})

test("AHI-036 RPC adds only bounded read/focus operations", () => {
  for (const key of ["workbenchSnapshot", "workbenchFocus", "workbenchRefresh", "workbenchPeers"]) assert.equal(INSPECTOR_RPC.methods[key].input.additionalProperties, false)
  assert.equal(INSPECTOR_RPC.methods.workbenchPeers.input.properties.peerIDs.maxItems, 16)
  assert.equal(INSPECTOR_RPC.methods.workbenchFocus.input.properties.ticketId.maxLength, 128)
  assert.equal(INSPECTOR_RPC.methods.workbenchComplete, undefined)
})

test("AHI-042 declared gate IDs survive only an admitted queue binding, separate from results", () => {
  const selected = { ...detail(), gateResults: "PASS", completion: "COMPLETED", requiredGates: { state: "OBSERVED", ids: ["review", "tests"] } }
  const change = cockpit(); change.checks[0].status = "PASS"
  const projected = projectWorkbench(selected, tasks(), change, {}, null)
  assert.deepEqual(projected.requiredGates, { state: "OBSERVED", ids: ["review", "tests"] })
  assert.equal(projected.gateResults, "PASS")
  assert.equal(projected.completion, "COMPLETED")
  assert.equal(projected.observedChecks[0].status, "PASS")
  assert.equal(Object.hasOwn(projected.requiredGates, "results"), false)
  assert.deepEqual(projectWorkbench(detail(), tasks(), change, {}, null).requiredGates, { state: "NOT_OBSERVED", ids: [] })
  assert.deepEqual(projectWorkbench({ ...selected, requiredGates: { state: "OBSERVED", ids: [] } }, tasks(), change, {}, null).requiredGates, { state: "OBSERVED", ids: [] })
  for (const state of ["empty", "stale", "unavailable"]) {
    assert.deepEqual(emptyWorkbench(state).requiredGates, { state: "NOT_OBSERVED", ids: [] })
    assert.deepEqual(projectWorkbench(selected, { ...tasks(), state }, change, {}, null).requiredGates, { state: "NOT_OBSERVED", ids: [] })
  }
  assert.deepEqual(projectWorkbench(selected, { ...tasks(), queueDigest: "b".repeat(64) }, change, {}, null).requiredGates, { state: "NOT_OBSERVED", ids: [] })
  const schema = INSPECTOR_RPC.methods.workbenchSnapshot.output
  assert.ok(schema.required.includes("requiredGates"))
  assert.deepEqual(schema.properties.requiredGates.required, ["state", "ids"])
  assert.equal(schema.properties.requiredGates.additionalProperties, false)
})
