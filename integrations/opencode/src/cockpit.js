import { createHash, randomUUID } from "node:crypto"
import { constants, lstatSync, realpathSync, openSync, fstatSync, readSync, closeSync } from "node:fs"
import path from "node:path"
import { visibleText } from "./display.js"
import { changeLimitations } from "./ui-presentation.js"

const object = properties => ({ type: "object", properties, required: Object.keys(properties), additionalProperties: false })
const string = { type: "string" }, strings = { type: "array", items: string }
const rows = properties => ({ type: "array", items: object(properties) })
export const cockpitSchema = object({ state: string, reason: string, receiptId: string, base: string, target: string, tree: string, observed: string, scope: string, workflow: string, files: strings, impacts: rows({ id: string, path: string, unit: string, tests: strings, via: strings, kind: string }), checks: rows({ id: string, status: string, argv: strings, testedCommit: string, currentTarget: string, reason: string }), intents: strings, declarations: rows({ command: string, kind: string, source: string, reason: string }), gaps: strings })
export const emptyCockpit = (state = "empty", reason = "Refresh to inspect this change.") => ({ state, reason, receiptId: "", base: "", target: "", tree: "", observed: "", scope: "unknown", workflow: "Not observed", files: [], impacts: [], checks: [], intents: [], declarations: [], gaps: [] })
const list = value => Array.isArray(value) ? value : []
const hash = value => createHash("sha256").update(JSON.stringify(value)).digest("hex")
const text = value => visibleText(typeof value === "string" ? value : "", 1600)
const oid = value => /^(?:[0-9a-f]{40}|[0-9a-f]{64})$/.test(value ?? "")
const fail = code => { throw new Error(code) }

// The Git directory is the trusted local anchor. Every component below it must be real,
// and the opened regular file must match the inspected inode. Same-user writes are not attested.
export function readPrivateFile(anchor, relative, maximum, truncate = false) {
  const root = realpathSync(anchor)
  if (typeof relative !== "string" || path.isAbsolute(relative) || relative.split(path.sep).some(x => !x || x === "." || x === "..")) fail("unsafe-private-path")
  const parts = relative.split(path.sep)
  let current = root, before
  for (let i = 0; i < parts.length; i++) {
    current = path.join(current, parts[i]); before = lstatSync(current)
    if (before.isSymbolicLink() || (i < parts.length - 1 ? !before.isDirectory() : !before.isFile() || before.nlink !== 1)) fail("unsafe-private-file")
  }
  const fd = openSync(current, constants.O_RDONLY | constants.O_NOFOLLOW | constants.O_NONBLOCK)
  try {
    const opened = fstatSync(fd)
    if (!opened.isFile() || opened.nlink !== 1 || opened.dev !== before.dev || opened.ino !== before.ino) fail("private-file-changed")
    const bytes = Buffer.alloc(maximum + 1)
    let count = 0, size
    do { size = readSync(fd, bytes, count, bytes.length - count, count); count += size } while (size && count < bytes.length)
    const after = fstatSync(fd), final = lstatSync(current)
    if (after.size !== opened.size || after.mtimeMs !== opened.mtimeMs || final.isSymbolicLink() || final.ino !== opened.ino || final.dev !== opened.dev) fail("private-file-changed")
    if (count > maximum && !truncate) fail("private-file-too-large")
    return { text: bytes.subarray(0, maximum).subarray(0, Math.min(count, maximum)).toString("utf8"), truncated: count > maximum }
  } finally { closeSync(fd) }
}

function owner(repo) {
  try {
    const value = readPrivateFile(repo.gitdir, path.join("corvint", "local-completion", "owner"), 128).text.trim()
    if (!/^[0-9a-f]{64}$/.test(value)) fail("invalid-completion-owner")
    return value
  } catch (error) { if (error.code === "ENOENT") return ""; throw error }
}
async function checked(run, read) {
  const value = await run(read)
  if (!value?.ok) fail(value?.code || "read-unavailable")
  return value
}
async function completion(repo, run) {
  const key = owner(repo)
  if (!key) return { key, policy: null, fingerprint: "" }
  const result = await checked(run, { kind: "completion", key })
  if (owner(repo) !== key) fail("completion-owner-changed")
  return { key, policy: result.policy, fingerprint: hash(result.policy) }
}
const sameRepo = (a, b) => a.root === b.root && a.gitdir === b.gitdir && a.target === b.target && a.tree === b.tree

export function checkStatus(check, dirty = false) {
  if (dirty && check.testedCommit) return "STALE"
  if (check.qualified === true) return "PASS"
  if (check.testedCommit && check.currentTarget && check.testedCommit !== check.currentTarget) return "STALE"
  if (check.cancelled) return "CANCELLED"
  if (check.timedOut) return "TIMEOUT"
  if (check.secretScreened) return "WITHHELD"
  if (Number.isInteger(check.exit) && check.exit > 0) return "FAIL"
  return check.testedCommit ? "UNVERIFIED" : "NOT RUN"
}

export function projectCockpit(repo, base, affected, policy, gaps = []) {
  const view = { ...emptyCockpit("ready", "Observed snapshot · refresh after external edits"), receiptId: randomUUID(), base, target: repo.target, tree: repo.tree, observed: new Date().toISOString(), scope: text(affected.plan.scope), workflow: policy?.satisfied === true ? "Recorded workflow satisfied" : policy ? policy.lifecycle === "satisfied" ? "Recorded completion is stale · obligations open" : `Workflow ${text(policy.lifecycle)} · obligations open` : "No recorded verification workflow", gaps: [...gaps] }
  const cap = (values, name, maximum = 64) => { if (values.length > maximum) view.gaps.push(`${values.length - maximum} ${name} omitted by the display limit.`); return values.slice(0, maximum) }
  view.files = cap(list(affected.plan.dirty), "changed paths").map(text)
  view.impacts = cap(list(affected.plan.selected), "impact witnesses").map((row, i) => ({ id: String(i), path: text(row.witness?.dirtyPath), unit: text(row.unitId), tests: cap(list(row.tests), "test paths", 32).map(text), via: cap(list(row.witness?.via), "dependency steps", 32).map(text), kind: text(row.witness?.kind) }))
  view.checks = cap(list(policy?.checks), "check observations").map(c => ({ id: text(c.id), status: checkStatus(c, list(policy?.unmet).includes("uncommitted-work")), argv: cap(list(c.argv), "command arguments", 64).map(text), testedCommit: text(c.testedCommit), currentTarget: text(c.currentTarget), reason: `Recorded exit: ${Number.isInteger(c.exit) ? c.exit : "unknown"}; Core qualified for committed target: ${c.qualified === true}.${list(policy?.unmet).includes("uncommitted-work") ? " Working-tree edits are unverified." : ""}` }))
  view.intents = cap(list(policy?.intents), "intent paths").map(text)
  view.declarations = cap(list(affected.advice?.checks), "declared checks").map(c => ({ command: text(c.command), kind: text(c.kind), source: text(c.source), reason: text(c.reason) }))
  view.gaps.push(...list(affected.plan.unknown).map(x => `${text(x.reason)}: ${text(x.detail)}`), ...list(affected.advice?.unknown).map(x => typeof x === "string" ? text(x) : text(JSON.stringify(x))), ...list(policy?.unmet).map(x => `Workflow: ${text(x)}`))
  if (!view.checks.length) view.gaps.push("No test execution results supplied. Suggested tests have not been run by this UI.")
  view.gaps.push(...changeLimitations)
  if (view.gaps.length > 128) view.gaps = [...view.gaps.slice(0, 127), `${view.gaps.length - 127} further gaps omitted by the display limit.`]
  return view
}

export async function collectCockpit(run, requestedBase = "") {
  const repo = await checked(run, { kind: "repository" })
  const readCompletion = async () => {
    try { return await completion(repo, run) }
    catch (error) { return { key: "", policy: null, fingerprint: `unavailable:${text(error.message)}`, error: text(error.message) } }
  }
  const record = await readCompletion(), gaps = record.error ? [`Verification unavailable: ${record.error}`] : []
  const base = requestedBase ? (await checked(run, { kind: "resolve", ref: requestedBase })).commit : oid(record.policy?.base) ? record.policy.base : repo.target
  const affected = await checked(run, { kind: "affected", base })
  const after = await checked(run, { kind: "repository" })
  if (!sameRepo(repo, after) || affected.revision !== repo.target) fail("repository-changed-during-refresh")
  const current = await readCompletion()
  if (current.key !== record.key || current.fingerprint !== record.fingerprint) fail("verification-changed-during-refresh")
  return { view: projectCockpit(repo, base, affected, record.policy, gaps), binding: { repo, ...record } }
}

export async function inspectProof(run, binding, checkID) {
  if (!binding?.key || !binding.policy) fail("verification-unavailable")
  const before = await checked(run, { kind: "repository" })
  const current = await completion(before, run)
  if (!sameRepo(binding.repo, before) || current.key !== binding.key || current.fingerprint !== binding.fingerprint) fail("verification-changed-refresh-required")
  const check = list(current.policy.checks).find(c => c.id === checkID)
  if (!check) fail("check-not-in-current-receipt")
  if (check.secretScreened) fail("verification-output-withheld")
  const output = [`${text(check.id)} · ${checkStatus(check, list(current.policy.unmet).includes("uncommitted-work"))}`, `Command (not executed): ${list(check.argv).map(x => text(x)).join(" ")}`, `Tested commit: ${text(check.testedCommit) || "not run"}`, `Current target: ${text(check.currentTarget)}`, `Core qualified for committed target: ${check.qualified === true} · recorded exit: ${check.exit}`, ...(list(current.policy.unmet).includes("uncommitted-work") ? ["Working-tree edits are unverified; this result predates those edits."] : []), "Output is a caller-owned local log; the status API supplies no content attestation."]
  for (const field of ["stdout", "stderr"]) {
    if (!check[field]) { output.push(`\n${field}: not recorded`); continue }
    const relative = path.relative(before.gitdir, check[field])
    const expected = new RegExp(`^corvint/local-completion/${current.key}/${current.policy.planDigest}-[0-9]+/check-[0-9]+-[01]\\.log$`)
    if (!/^[0-9a-f]{64}$/.test(current.policy.planDigest ?? "") || !expected.test(relative.split(path.sep).join("/"))) fail("unsafe-verification-log")
    const log = readPrivateFile(before.gitdir, relative, 65_536, true)
    output.push(`\n${field}:`, log.text.replace(/\r\n/g, "\n").split("\n").map(line => visibleText(line, 65_536)).join("\n"), ...(log.truncated ? ["[Output truncated at 64 KiB]"] : []))
  }
  const again = await completion(before, run), after = await checked(run, { kind: "repository" })
  if (!sameRepo(before, after) || again.key !== current.key || again.fingerprint !== current.fingerprint) fail("verification-changed-during-read")
  return output.join("\n")
}
