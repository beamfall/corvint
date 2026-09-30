import test from "node:test"
import assert from "node:assert/strict"
import { emptyCockpit, projectCockpit } from "./src/cockpit.js"
import { changeLimitations, changePresentation, checkPresentation, changeEmpty } from "./src/ui-presentation.js"

test("AHI-042 Change attention excludes only shared exact limitations and preserves future gaps", () => {
  const unknown = "Verification records are caller-owned local observations; a new verification failure occurred."
  const view = { ...emptyCockpit("ready"), gaps: [...changeLimitations, unknown, "No test execution results supplied.", `${changeLimitations[0]} Additional missing evidence.`] }
  const result = changePresentation(view)
  assert.deepEqual(result.attention, view.gaps.slice(2))
  assert.deepEqual(result.limitations, changeLimitations)
  assert.match(result.summary, /3 attention/)
  assert.equal(view.gaps.length, 5)
})

test("AHI-042 Projection and presentation share limitations without changing the RPC shape", () => {
  const view = projectCockpit({ target: "a".repeat(40), tree: "b".repeat(40) }, "c".repeat(40), { plan: { scope: "UNKNOWN", dirty: [], selected: [], unknown: [] } }, null)
  const result = changePresentation(view)
  assert.ok(changeLimitations.every(value => view.gaps.includes(value)))
  assert.equal(result.attention.length, 1)
  assert.match(result.attention[0], /No test execution results/)
  assert.deepEqual(Object.keys(view).sort(), Object.keys(emptyCockpit()).sort())
  const saturated = projectCockpit({ target: "a".repeat(40), tree: "b".repeat(40) }, "c".repeat(40), { plan: { scope: "UNKNOWN", unknown: Array.from({ length: 140 }, (_, i) => ({ reason: "FUTURE", detail: String(i) })) } }, null)
  const truncated = changePresentation(saturated)
  assert.equal(truncated.attention.length, 128)
  assert.match(truncated.attention.at(-1), /omitted/)
  assert.deepEqual(truncated.limitations, changeLimitations)
})

test("AHI-042 Check presentation preserves outcomes and never gives stale snapshots a passing tone", () => {
  const check = { id: "unit", status: "PASS", testedCommit: "a".repeat(40) }
  assert.equal(checkPresentation(check, "ready").tone, "info")
  assert.match(checkPresentation(check, "ready").label, /recorded commit/)
  for (const state of ["stale", "loading", "unavailable", "future-state"]) {
    const row = checkPresentation(check, state)
    assert.equal(row.tone, "warning")
    assert.match(row.sub, /refresh/)
  }
  assert.equal(checkPresentation({ ...check, status: "FAIL" }, "ready").tone, "error")
  for (const status of ["STALE", "CANCELLED", "TIMEOUT", "WITHHELD", "UNVERIFIED", "NOT RUN", "FUTURE_STATUS"]) {
    assert.equal(checkPresentation({ ...check, status }, "ready").tone, "warning")
  }
  assert.match(checkPresentation({ ...check, status: "FUTURE_STATUS" }, "ready").label, /FUTURE_STATUS/)
})

test("AHI-042 Empty sections distinguish missing evidence from unavailable and filtered snapshots", () => {
  const ready = emptyCockpit("ready")
  assert.notEqual(changeEmpty(ready, "impact", true), changeEmpty(ready, "impact", false))
  assert.equal(new Set(["files", "impact", "proof", "gaps"].map(section => changeEmpty(ready, section))).size, 4)
  for (const state of ["empty", "loading", "stale", "unavailable", "future-state"]) {
    const view = emptyCockpit(state, "Detailed reason")
    assert.equal(new Set(["files", "impact", "proof", "gaps"].map(section => changeEmpty(view, section))).size, 1)
    assert.notEqual(changePresentation(view).heading, "Change snapshot")
    assert.equal(changePresentation(view).summary, view.reason)
  }
  assert.match(changeEmpty(emptyCockpit("unavailable", "Detailed failure"), "proof"), /Detailed failure/)
})
