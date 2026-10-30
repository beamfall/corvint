// DSG-V0-008 (GitHub #1821): a dispatcher worker's WARN/HANDOFF notices reach OpenCode's system context.
import test from "node:test"
import assert from "node:assert/strict"
import { mkdtempSync, writeFileSync, rmSync, chmodSync, utimesSync } from "node:fs"
import { tmpdir } from "node:os"
import path from "node:path"
import { createDispatchNotices, noticeOf, runContextHook } from "./src/dispatch-notice.js"

const hookLine = text => JSON.stringify({ hookSpecificOutput: { hookEventName: "opencode.session.context", additionalContext: text } })

test("outside a dispatcher worker there is no notice source", () => {
  assert.equal(createDispatchNotices({ environment: {}, tasksBinary: "corvint-tasks" }), undefined)
})

test("noticeOf reads only a bounded additionalContext", () => {
  assert.equal(noticeOf(hookLine("HANDOFF now")), "HANDOFF now")
  assert.equal(noticeOf("not json"), "")
  assert.equal(noticeOf(hookLine("x".repeat(2001))), "")
  assert.equal(noticeOf(""), "")
})

test("a changed record runs the hook once and every later request keeps the notice", async t => {
  const root = mkdtempSync(path.join(tmpdir(), "corvint-dispatch-notice-")); t.after(() => rmSync(root, { recursive: true, force: true }))
  const record = path.join(root, "context.json"); writeFileSync(record, "{}\n")
  const outputs = [hookLine("WARN: 150000 tokens"), ""]
  let runs = 0
  const notices = createDispatchNotices({ environment: { CORVINT_DISPATCH_CONTEXT_FILE: record }, tasksBinary: "x", run: async () => outputs[runs++] ?? "" })
  const first = { system: [] }
  await notices.inject(first)
  assert.deepEqual(first.system, [{ type: "text", text: "WARN: 150000 tokens" }])
  const second = { system: [] }
  await notices.inject(second)
  assert.equal(runs, 1, "an unchanged record does not rerun the hook")
  assert.deepEqual(second.system, first.system)
  writeFileSync(record, "{\"level\":\"HANDOFF\"}\n"); utimesSync(record, new Date(), new Date(Date.now() + 5000))
  outputs[1] = hookLine("HANDOFF: write your note")
  const third = { system: [{ type: "text", text: "WARN: 150000 tokens" }] }
  await notices.inject(third)
  assert.equal(runs, 2)
  assert.deepEqual(third.system.map(s => s.text), ["WARN: 150000 tokens", "HANDOFF: write your note"])
  await notices.inject(undefined)
})

test("a missing record or failing binary delivers nothing and never throws", async t => {
  const root = mkdtempSync(path.join(tmpdir(), "corvint-dispatch-notice-")); t.after(() => rmSync(root, { recursive: true, force: true }))
  const missing = createDispatchNotices({ environment: { CORVINT_DISPATCH_CONTEXT_FILE: path.join(root, "absent") }, tasksBinary: path.join(root, "absent-bin") })
  const request = { system: [] }
  await missing.inject(request)
  assert.deepEqual(request.system, [])
  assert.equal(await runContextHook(path.join(root, "absent-bin"), {}), "")
})

test("runContextHook runs `dispatch context-hook` with the event on stdin", { skip: process.platform === "win32" }, async t => {
  const root = mkdtempSync(path.join(tmpdir(), "corvint-dispatch-notice-")); t.after(() => rmSync(root, { recursive: true, force: true }))
  const bin = path.join(root, "fake-tasks")
  writeFileSync(bin, "#!/bin/sh\n[ \"$1 $2\" = \"dispatch context-hook\" ] || exit 2\nread -r line\nprintf '{\"hookSpecificOutput\":{\"additionalContext\":\"%s|%s\"}}\\n' \"$CORVINT_DISPATCH_CONTEXT_FILE\" \"$(printf %s \"$line\" | tr -d '\"{}:')\"\n")
  chmodSync(bin, 0o755)
  const out = await runContextHook(bin, { PATH: process.env.PATH, CORVINT_DISPATCH_CONTEXT_FILE: "/r" })
  assert.equal(noticeOf(out), "/r|hook_event_nameopencode.session.context")
})
