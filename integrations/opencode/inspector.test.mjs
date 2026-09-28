import assert from "node:assert/strict"
import test from "node:test"
import { INSPECTOR_RPC, visibleText, projectReceipt, emptySnapshot, beginInspection, completeInspection, invalidateInspection, inspectionMatches } from "./src/inspector.js"

const tree = "a".repeat(40), blob = "b".repeat(40)
const handle = `cv1:${tree}:${blob}:all:add.go`
const receipt = () => ({ ok: true, receiptId: "receipt-1", repository: { treeRevision: tree, worktreeState: "clean" }, degradations: ["frontier-authority-unavailable"], context: { results: [{ title: "Add", kind: "symbol", summary: "Adds integers", evidence: [{ path: "add.go", line: 3, blob_hash: blob, authority: "syntax", confidence: "medium", reason: "Matches Add" }] }], coverage: { critical_missing: ["REQ-2"], uncertainty: ["partial index"], omitted_results: 2 } } })

test("AHI-033 projection preserves evidence, pinned handles and independent gaps", () => {
  const view = projectReceipt(receipt(), [handle])
  assert.equal(view.rows[0].handle, handle)
  assert.equal(view.rows[0].reason, "Matches Add")
  assert.equal(view.rows[0].authority, "syntax")
  assert.equal(view.revision, tree)
  assert.equal(view.support, "FALLBACK")
  assert.deepEqual(view.gaps, ["frontier-authority-unavailable", "partial index", "Missing: REQ-2", "2 results omitted by Core's budget."])
  assert.equal(projectReceipt(receipt(), []).rows[0].handle, "")
})

test("AHI-033 terminal control text cannot forge rows or terminal commands", () => {
  for (const value of ["\x1b]52;c;attack\x07", "\nFAKE PASS\r", "\u202ereverse", "\u200bhidden", "\x9b31m", "\t", "\ufeff"]) {
    assert.doesNotMatch(visibleText(value), /[\x00-\x1f\x7f-\x9f\u202e\u200b\ufeff]/u)
  }
  assert.match(visibleText("x".repeat(2000)), /display shortened/)
  const packet = receipt();packet.context.results[0].evidence[0].path = "evil\x1b.go"
  const exact = `cv1:${tree}:${blob}:all:evil\x1b.go`
  const view = projectReceipt(packet, [exact])
  assert.equal(view.rows[0].handle, exact)
  assert.equal(view.rows[0].path, "evil\\u001b.go")
})

test("AHI-033 bounded view discloses omitted evidence and gaps", () => {
  const packet = receipt();packet.context.results = Array.from({ length: 40 }, () => packet.context.results[0])
  const view = projectReceipt(packet, [handle])
  assert.equal(view.rows.length, 32);assert.ok(view.gaps.some(x => x.includes("8 evidence locations omitted")))
  packet.degradations = Array.from({ length: 60 }, (_, i) => `gap-${i}`)
  assert.equal(projectReceipt(packet).gaps.length, 32)
  assert.match(projectReceipt(packet).gaps.at(-1), /further gaps omitted/)
  assert.deepEqual(projectReceipt({ ok: false, code: "timeout" }), emptySnapshot("unavailable", "timeout"))
})

test("AHI-033 newer queries, edits and retirement prevent stale publication or expansion", () => {
  const state = { active: true }
  const first = beginInspection(state), second = beginInspection(state)
  assert.equal(completeInspection(state, first, receipt(), [handle]), false)
  assert.equal(completeInspection(state, second, receipt(), [handle]), true)
  assert.equal(inspectionMatches(state, "receipt-1", handle), true)
  assert.equal(inspectionMatches(state, "other", handle), false)
  invalidateInspection(state, "edited")
  assert.equal(inspectionMatches(state, "receipt-1", handle), false)
  assert.equal(completeInspection(state, second, receipt(), [handle]), false)
  const next = beginInspection(state);state.active = false
  assert.equal(completeInspection(state, next, receipt(), [handle]), false)
})

test("AHI-033 RPC has a closed bounded request surface and no execution or persistence verb", () => {
  assert.deepEqual(Object.keys(INSPECTOR_RPC.methods), ["snapshot", "query", "expand"])
  for (const method of Object.values(INSPECTOR_RPC.methods)) {
    assert.equal(method.input.additionalProperties, false)
    assert.equal(method.input.properties.sessionID.maxLength, 256)
  }
  assert.equal(INSPECTOR_RPC.methods.expand.input.properties.handle.maxLength, 1024)
})
