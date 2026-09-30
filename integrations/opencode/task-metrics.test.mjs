import test from "node:test"
import assert from "node:assert/strict"
import { existsSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import path from "node:path"
import { collectTaskDetail, collectTaskMetrics, inspectTask } from "./src/task-metrics.js"
import { cockpitReadArguments, createCorvintRunner } from "./src/runtime.js"
import { INSPECTOR_RPC } from "./src/inspector.js"

const receipt = "a".repeat(64), changed = "b".repeat(64), ticketId = "ticket:corvint:main:V1-0042"
const status = (digest = receipt) => ({ profile: "taskman-command-result/0", outcome: "OK", command: ["queue", "status"], snapshot: { headReceiptSha256: digest }, items: [{ queueId: "queue:corvint:main", tickets: "5", byStatus: { DRAFT: "1", OPEN: "2", COMPLETED: "1", HELD: "0", ARCHIVED: "1" }, blocked: "2", intentChecksPassed: "1", liveAttempts: [] }] })
const open = (digest = receipt) => ({ profile: "taskman-command-result/0", outcome: "OK", command: ["ticket", "search"], snapshot: { headReceiptSha256: digest }, page: { offset: "0", total: "2", limit: "32" }, items: [{ ticketId, title: "Fix \u001b[31mqueue", status: "OPEN", milestone: "v1", priority: "P1", eligibility: "BLOCKED", nextAction: "set-effects", gateResults: "NOT_OBSERVED", blockers: [{ code: "COVERAGE_UNKNOWN", detail: "Coverage unknown" }] }] })
const detail = (digest = receipt) => ({ profile: "taskman-command-result/0", outcome: "OK", command: ["ticket", "show"], snapshot: { headReceiptSha256: digest }, items: [{ ticketId, title: "Fix queue", status: "OPEN", eligibility: "BLOCKED", priority: "P1", milestone: "v1", nextAction: "set-effects", gateResults: "NOT_OBSERVED", completion: null, blockers: [{ code: "COVERAGE_UNKNOWN", detail: "Coverage is not established" }], record: { acceptanceCriteria: ["Show truthful \u202e metrics"] } }] })
const source = (values = [status(), open(), status()]) => async () => ({ ok: true, envelope: values.shift() })

test("AHI-035 Task metrics preserve queue counts, open-page limits and unobserved gates", async () => {
  const snapshot = await collectTaskMetrics(source())
  assert.equal(snapshot.total, 5)
  assert.equal(snapshot.draft, 1)
  assert.equal(snapshot.completed, 1)
  assert.equal(snapshot.open, 2)
  assert.equal(snapshot.openTotal, 2)
  assert.equal(snapshot.tickets.length, 1)
  assert.equal(snapshot.tickets[0].gateResults, "NOT_OBSERVED")
  assert.match(snapshot.tickets[0].title, /\\u001b/)
  assert.match(snapshot.gaps.join("\n"), /outside this page|separate evidence/)
  const text = await inspectTask(source([detail(), status()]), snapshot, ticketId)
  assert.match(text, /Coverage is not established/)
  assert.match(text, /\\u202e/)
  assert.match(text, /Completion: NOT_OBSERVED/)
})

test("AHI-036 focused detail binds criteria, declared paths and live attempt to one queue receipt", async () => {
  const snapshot = await collectTaskMetrics(source())
  const selected = detail(); selected.items[0].record.requirementRefs = ["AHI-036"]
  selected.items[0].record.effects = { touchPaths: ["integrations/opencode/src/workbench.js"] }
  const live = status(); live.items[0].liveAttempts = [{ ticketId, attemptId: "attempt-1", holder: "agent-1", phase: "CHECKING", expiresAt: "2026-09-30T00:00:00Z" }]
  const view = await collectTaskDetail(source([selected, live]), snapshot, ticketId)
  assert.equal(view.queueDigest, receipt)
  assert.deepEqual(view.requirementRefs, ["AHI-036"])
  assert.deepEqual(view.touchPaths, ["integrations/opencode/src/workbench.js"])
  assert.equal(view.attempt.holder, "agent-1")
  await assert.rejects(collectTaskDetail(source([selected, status(changed)]), snapshot, ticketId), /task-queue-changed/)
})

test("AHI-037 focused proof detail escapes hostile criteria and refuses malformed declared evidence", async () => {
  const snapshot = await collectTaskMetrics(source())
  const hostile = detail(); hostile.items[0].record.acceptanceCriteria = ["Review \u001b[31m and \u202e proof"]
  const view = await collectTaskDetail(source([hostile, status()]), snapshot, ticketId)
  assert.match(view.criteria[0], /\\u001b/)
  assert.match(view.criteria[0], /\\u202e/)
  const invalid = detail(); invalid.items[0].record.requirementRefs = "AHI-037"
  await assert.rejects(collectTaskDetail(source([invalid, status()]), snapshot, ticketId), /task-detail-unavailable/)
  const over = detail(); over.items[0].record.acceptanceCriteria = Array(17).fill("one criterion")
  const bounded = await collectTaskDetail(source([over, status()]), snapshot, ticketId)
  assert.equal(bounded.criteria.length, 16)
  assert.match(bounded.gaps.join("\n"), /omitted/)
})

test("AHI-035 changed or inconsistent queue snapshots refuse joined metrics and details", async () => {
  await assert.rejects(collectTaskMetrics(source([status(), open(changed), status()])), /task-queue-changed/)
  const invalid = status(); invalid.items[0].tickets = "6"
  await assert.rejects(collectTaskMetrics(source([invalid, open(), invalid])), /inconsistent-task-summary/)
  const snapshot = await collectTaskMetrics(source())
  await assert.rejects(inspectTask(source([detail(changed), status(changed)]), snapshot, ticketId), /task-queue-changed/)
  await assert.rejects(inspectTask(source(), snapshot, "ticket:corvint:main:V1-9999"), /ticket-not-in-current-page/)
  const malformedPage = open(); malformedPage.items[0].blockers = "corrupt"
  await assert.rejects(collectTaskMetrics(source([status(), malformedPage, status()])), /invalid-task-row/)
  const malformedDetail = detail(); malformedDetail.items[0].record = "corrupt"
  await assert.rejects(inspectTask(source([malformedDetail, status()]), snapshot, ticketId), /task-detail-unavailable/)
  const missingBlockers = detail(); delete missingBlockers.items[0].blockers
  await assert.rejects(inspectTask(source([missingBlockers, status()]), snapshot, ticketId), /task-detail-unavailable/)
  const beyond = open(); beyond.page.offset = "32"
  await assert.rejects(collectTaskMetrics(source([status(), beyond, status()]), 32), /inconsistent-open-page/)
})

test("AHI-035 open ticket pages disclose preceding and later omissions", async () => {
  const first = status(); first.items[0].tickets = "36"; first.items[0].byStatus.OPEN = "33"
  const page = open(); page.page.offset = "32"; page.page.total = "33"
  const after = structuredClone(first)
  const value = await collectTaskMetrics(source([first, page, after]), 32)
  assert.equal(value.offset, 32)
  assert.equal(value.openTotal, 33)
  assert.match(value.gaps.join("\n"), /32 earlier open tickets/)
})

test("AHI-035 Tasks runner permits only fixed read verbs and preserves native refusal", async t => {
  assert.deepEqual(cockpitReadArguments("/repo", { kind: "tasks-status" }).args, ["queue", "status"])
  assert.equal(cockpitReadArguments("/repo", { kind: "tasks-detail", ticketId: "--help" }), undefined)
  assert.equal(cockpitReadArguments("/repo", { kind: "tasks-open", offset: -1 }), undefined)
  assert.equal(cockpitReadArguments("/repo", { kind: "tasks-complete", ticketId }), undefined)
  for (const name of ["tasksSnapshot", "tasksRefresh", "tasksDetail"]) assert.equal(INSPECTOR_RPC.methods[name].input.additionalProperties, false)
  const dir = mkdtempSync(path.join(tmpdir(), "corvint-tasks-read-")); t.after(() => rmSync(dir, { recursive: true, force: true }))
  const script = path.join(dir, "tasks")
  const write = value => writeFileSync(script, `#!${process.execPath}\nprocess.stdout.write(${JSON.stringify(JSON.stringify(value))}); process.exit(${value.outcome === "OK" ? 0 : 1})\n`, { mode: 0o755 })
  const runner = createCorvintRunner({ tasksBinary: script })
  const invoke = read => runner({ root: dir, event: "tasks", input: {}, read })
  write(status()); assert.equal((await invoke({ kind: "tasks-status" })).ok, true)
  write({ profile: "taskman-command-result/0", outcome: "REFUSED", codes: ["UNINITIALIZED"], command: ["queue", "status"], items: [] })
  assert.equal((await invoke({ kind: "tasks-status" })).code, "tasks-uninitialized")
  write({ ...status(), command: ["ticket", "complete"] })
  assert.equal((await invoke({ kind: "tasks-status" })).code, "malformed-cockpit-output")
})

test("AHI-035 cancelling a Tasks read retires its child process group", async t => {
  const dir = mkdtempSync(path.join(tmpdir(), "corvint-tasks-abort-")); t.after(() => rmSync(dir, { recursive: true, force: true }))
  const script = path.join(dir, "tasks"), pidFile = path.join(dir, "descendant.pid")
  writeFileSync(script, `#!${process.execPath}\nconst {spawn}=require('node:child_process'); const {writeFileSync}=require('node:fs'); const child=spawn(process.execPath,['-e','setInterval(()=>{},1000)'],{stdio:'ignore'}); writeFileSync(${JSON.stringify(pidFile)},String(child.pid)); setInterval(()=>{},1000)\n`, { mode: 0o755 })
  const runner = createCorvintRunner({ tasksBinary: script }), controller = new AbortController()
  const pending = runner({ root: dir, event: "tasks", input: {}, read: { kind: "tasks-status" }, signal: controller.signal })
  let pid
  try {
    for (let i = 0; i < 300 && !existsSync(pidFile); i++) await new Promise(resolve => setTimeout(resolve, 10))
    assert.ok(existsSync(pidFile), "descendant started")
    pid = Number(readFileSync(pidFile, "utf8"))
    controller.abort()
    assert.equal((await pending).code, "host-aborted")
    let alive = true
    for (let i = 0; i < 50 && alive; i++) {
      await new Promise(resolve => setTimeout(resolve, 10))
      try { process.kill(pid, 0) } catch (error) { if (error.code === "ESRCH") alive = false; else throw error }
    }
    assert.equal(alive, false, "owned descendant retired")
  } finally {
    controller.abort()
    if (Number.isInteger(pid)) { try { process.kill(pid, "SIGKILL") } catch {} }
    await pending
  }
})

test("AHI-042 gate declarations distinguish absent, empty and observed escaped IDs", async () => {
  const snapshot = await collectTaskMetrics(source())
  const absent = await collectTaskDetail(source([detail(), status()]), snapshot, ticketId)
  assert.deepEqual(absent.requiredGates, { state: "NOT_OBSERVED", ids: [] })
  const selected = detail(); selected.items[0].record.requiredGates = []
  assert.deepEqual((await collectTaskDetail(source([selected, status()]), snapshot, ticketId)).requiredGates, { state: "OBSERVED", ids: [] })
  selected.items[0].record.requiredGates = ["gate; touch INERT", "\u001b[31m\u202e gate"]
  const observed = await collectTaskDetail(source([selected, status()]), snapshot, ticketId)
  assert.equal(observed.requiredGates.state, "OBSERVED")
  assert.equal(observed.requiredGates.ids[0], "gate; touch INERT")
  assert.match(observed.requiredGates.ids[1], /\\u001b/)
  assert.match(observed.requiredGates.ids[1], /\\u202e/)
  assert.doesNotMatch(observed.requiredGates.ids[1], /\u001b|\u202e/u)
  await assert.rejects(collectTaskDetail(source([selected, status(changed)]), snapshot, ticketId), /task-queue-changed/)
})

test("AHI-042 gate declarations validate the entire bounded array before truncation", async () => {
  const snapshot = await collectTaskMetrics(source())
  for (const gates of [null, "gate", {}, [1], [null], [""], [" \t"], ["x".repeat(16_385)], Array(257).fill("gate"), [...Array(32).fill("gate"), false]]) {
    const selected = detail(); selected.items[0].record.requiredGates = gates
    await assert.rejects(collectTaskDetail(source([selected, status()]), snapshot, ticketId), /task-detail-unavailable/)
  }
  const selected = detail(); selected.items[0].record.requiredGates = Array.from({ length: 256 }, (_, i) => `gate-${i}`)
  selected.items[0].record.requiredGates[0] = "x".repeat(1601)
  selected.items[0].record.requiredGates[1] = "\u001b".repeat(300)
  const observed = await collectTaskDetail(source([selected, status()]), snapshot, ticketId)
  assert.equal(observed.requiredGates.ids.length, 32)
  assert.match(observed.gaps.join("\n"), /224 declared gate IDs omitted/)
  assert.match(observed.gaps.join("\n"), /not complete identities/)
  assert.match(observed.requiredGates.ids[0], /display shortened/)
  assert.match(observed.requiredGates.ids[1], /display shortened/)
})
