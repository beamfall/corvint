import test from "node:test"
import assert from "node:assert/strict"
import { mkdtempSync, rmSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import path from "node:path"
import { execFileSync } from "node:child_process"
import { BUNDLED_SKILL_PATH, loadBundledSkill, parseSkill, registerBundledSkill } from "./src/skill.js"
import openCodePlugin from "./src/index.js"

// An OpenCode 2.1 skill registry as an external plugin sees it: transform hands the editor over.
function registry(existing = []) {
  const skills = new Map(existing.map((skill) => [skill.id, skill]))
  return { skills, transform: async (edit) => { edit({ list: () => [...skills.values()], get: (id) => skills.get(id), add: (skill) => skills.set(skill.id, skill), update() {}, remove: (id) => skills.delete(id) }) } }
}

test("AHI-176 the bundled skill parses into the OpenCode Skill.Info shape", () => {
  const skill = loadBundledSkill()
  assert.equal(skill.id, "corvint")
  assert.equal(skill.name, "corvint")
  assert.equal(skill.path, BUNDLED_SKILL_PATH)
  assert.ok(skill.description.startsWith("Use Corvint"))
  assert.ok(skill.content.startsWith("# Corvint in OpenCode"))
  assert.ok(!skill.content.includes("description:"))
})

test("AHI-176 unreadable, oversized or ambiguous skill text is refused, not guessed", (t) => {
  const dir = mkdtempSync(path.join(tmpdir(), "corvint-skill-"));t.after(() => rmSync(dir, { recursive: true, force: true }))
  const body = "---\nname: corvint\ndescription: d\n---\n# Body\n"
  assert.ok(parseSkill(body, "/x"))
  for (const text of [undefined, "# no frontmatter\n", body.replace("\n", "\r\n"), "---\nname: other\ndescription: d\n---\nbody\n", "---\nname: corvint\nname: corvint\ndescription: d\n---\nbody\n", "---\nname: corvint\ndescription: d\nextra: x\n---\nbody\n", "---\nname: corvint\ndescription: d\n---\n\n", body + "x".repeat(16_384)])
    assert.equal(parseSkill(text, "/x"), undefined, JSON.stringify(text)?.slice(0, 80))
  assert.equal(loadBundledSkill(path.join(dir, "missing.md")), undefined)
  assert.equal(loadBundledSkill(dir), undefined)
  const big = path.join(dir, "big.md");writeFileSync(big, body + "x".repeat(20_000))
  assert.equal(loadBundledSkill(big), undefined)
})

test("AHI-176 registration never replaces a project or user skill and tolerates an older host", async () => {
  const skill = loadBundledSkill()
  const empty = registry()
  assert.equal(await registerBundledSkill(empty, skill), "registered")
  assert.deepEqual(empty.skills.get("corvint"), skill)
  const project = { id: "corvint", name: "corvint", description: "project-owned", path: "/repo/.opencode/skills/corvint/SKILL.md", content: "project" }
  const owned = registry([project])
  assert.equal(await registerBundledSkill(owned, skill), "project-owned")
  assert.equal(owned.skills.get("corvint"), project)
  assert.equal(await registerBundledSkill(undefined, skill), "unavailable")
  assert.equal(await registerBundledSkill(registry(), undefined), "unreadable")
})

test("AHI-176 plugin setup registers the skill and a failing registry leaves tools intact", async (t) => {
  const dir = mkdtempSync(path.join(tmpdir(), "corvint-skill-repo-"));t.after(() => rmSync(dir, { recursive: true, force: true }))
  execFileSync("git", ["init", "-q", dir])
  const registration = { dispose: async () => {} }
  const context = (skill) => {
    const tools = {}, edits = []
    const rebuild = () => { for (const name of Object.keys(tools)) delete tools[name]; for (const edit of edits) edit({ add: (tool) => { tools[tool.name] = tool } }) }
    const ctx = { app: { version: "2.1.3" }, location: { directory: dir, project: { id: "fixture", directory: dir } }, options: { corvintBinary: "/nonexistent/corvint" },
      event: { subscribe: async function* ({ signal }) { await new Promise((resolve) => signal.addEventListener("abort", resolve, { once: true })) } },
      tool: { hook: async () => registration, transform: async (edit) => { edits.push(edit); rebuild(); return registration }, reload: async () => rebuild() },
      session: { get: async () => ({}), hook: async () => registration }, skill }
    return { ctx, tools }
  }
  const ok = registry(), first = context(ok)
  const cleanup = await openCodePlugin.setup(first.ctx);t.after(cleanup)
  // AHI-212: the skill registers with the follow-up tools on the first corvint_context call.
  assert.equal(ok.skills.get("corvint"), undefined)
  const call = (tools) => tools.corvint_context.execute({ task: "inspect" }, { sessionID: "s", signal: new AbortController().signal })
  await call(first.tools)
  assert.equal(ok.skills.get("corvint")?.path, BUNDLED_SKILL_PATH)
  const warnings = [], original = console.warn;console.warn = (row) => warnings.push(row);t.after(() => { console.warn = original })
  const failing = context({ transform: async () => { throw new Error("registry closed") } })
  const cleanupFailing = await openCodePlugin.setup(failing.ctx);t.after(cleanupFailing)
  await call(failing.tools)
  assert.ok(failing.tools.corvint_context && failing.tools.corvint_status)
  assert.ok(warnings.some((row) => row.includes("bundled-skill-registration-failed")), warnings.join("\n"))
})
