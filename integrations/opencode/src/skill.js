import { closeSync, fstatSync, openSync, readSync } from "node:fs"
import { fileURLToPath } from "node:url"

// AHI-176: the bundled skill ships inside this package; registering it through OpenCode's
// native skill registry keeps the guidance at the installed adapter's version without a copy step.
export const BUNDLED_SKILL_PATH = fileURLToPath(new URL("../skills/corvint/SKILL.md", import.meta.url))
const MAX_SKILL_BYTES = 16_384
const FRONTMATTER = /^---\n([\s\S]*?)\n---\n/

// parseSkill returns OpenCode's Skill.Info shape, or undefined for any text it cannot read exactly.
export function parseSkill(text, skillPath) {
  if (typeof text !== "string" || Buffer.byteLength(text) > MAX_SKILL_BYTES || text.includes("\r")) return undefined
  const match = FRONTMATTER.exec(text)
  if (!match) return undefined
  const fields = {}
  for (const line of match[1].split("\n")) {
    const field = /^(name|description): (\S.*)$/.exec(line)
    if (!field || fields[field[1]] !== undefined) return undefined
    fields[field[1]] = field[2]
  }
  const content = text.slice(match[0].length).trim()
  if (fields.name !== "corvint" || !fields.description || !content) return undefined
  return { id: fields.name, name: fields.name, description: fields.description, path: skillPath, content }
}

export function loadBundledSkill(skillPath = BUNDLED_SKILL_PATH) {
  let fd
  try {
    fd = openSync(skillPath, "r")
    const stat = fstatSync(fd)
    if (!stat.isFile() || stat.size > MAX_SKILL_BYTES) return undefined
    const bytes = Buffer.alloc(stat.size)
    let count = 0, size
    do { size = readSync(fd, bytes, count, bytes.length - count, count); count += size } while (size && count < bytes.length)
    return parseSkill(bytes.subarray(0, count).toString("utf8"), skillPath)
  } catch {
    return undefined
  } finally {
    if (fd !== undefined) closeSync(fd)
  }
}

// registerBundledSkill adds the skill only when no project, user or earlier plugin skill already
// owns the id: a configured skill is project authority and is never replaced (AHI-176).
export async function registerBundledSkill(registry, skill) {
  if (typeof registry?.transform !== "function") return "unavailable"
  if (!skill) return "unreadable"
  let outcome = "registered"
  await registry.transform((editor) => {
    if (editor.get(skill.id) !== undefined) {
      outcome = "project-owned"
      return
    }
    editor.add({ ...skill })
  })
  return outcome
}
