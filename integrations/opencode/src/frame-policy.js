// AHI-215 (V1-2755): the prompt-time candidate frame abstains unless it adds signal. The
// 2026-10-26 value audit found the frame cost-neutral at best: agents reached a gold file in their
// first step or two without it, every row was a syntax match, and most of its bytes were envelope
// and disclosure. These pure helpers decide when the frame is withheld and which rows it shows.
import { lstatSync } from "node:fs"
import path from "node:path"

// Row authorities that corroborate a task beyond matching its words. Anything else, including an
// authority this adapter does not know, is syntax-class and cannot make the frame emit.
export const CORROBORATING_AUTHORITIES = Object.freeze([
  "project-instructions", "instruction-reference", "repository-spec", "git-history",
])
export const FRAME_ROWS = 5
// The whole injected frame text, prefix and disclosure included, stays within this many UTF-8
// bytes: at most 200 tokens even at a conservative 3 bytes per token.
export const FRAME_BYTES = 600
// AHI-219 (V1-2870): the frame names its query by a 28-byte link, not the 87-byte receipt id, so
// five candidates fit the bound. corvint_expand resolves the link within the session that saw it.
export const RECEIPT_LINK = /^cvr:[0-9a-f]{24}$/
export function receiptLink(receiptId) {
  const match = typeof receiptId === "string" ? /^harness-receipt:sha256:([0-9a-f]{64})$/.exec(receiptId) : null
  return match ? `cvr:${match[1].slice(0, 24)}` : undefined
}
// Records the packet behind a delivered frame's link, keeping the newest `cap` links. A link two
// different receipts share is kept as ambiguous (null), never resolved to either.
export function rememberFrameLink(links, link, entry, cap) {
  const prior = links.get(link)
  links.delete(link)
  links.set(link, prior === null || (prior && prior.response.receiptId !== entry.response.receiptId) ? null : entry)
  if (links.size > cap) links.delete(links.keys().next().value)
  return links
}

const PATH_TOKEN = /`[^`\n]{1,256}`|[A-Za-z0-9_.][A-Za-z0-9_./-]*[A-Za-z0-9_]/g
const PATH_SHAPE = /^(?:[A-Za-z0-9_.-]+\/)+[A-Za-z0-9_.-]+\/?$|^[A-Za-z0-9_.-]*[A-Za-z0-9_-]\.[A-Za-z][A-Za-z0-9]{0,7}$/
const MAX_PATH_CHECKS = 256

// The repository-relative paths the prompt names that exist under `root` (files or directories).
// Absolute paths, `..` segments and over-long tokens are never checked; at most 256 are.
export function namedExistingPaths(prompt, root) {
  if (typeof prompt !== "string" || typeof root !== "string") return []
  const seen = new Set(), found = []
  for (const match of prompt.matchAll(PATH_TOKEN)) {
    let token = match[0].startsWith("`") ? match[0].slice(1, -1).trim() : match[0]
    if (token.startsWith("./")) token = token.slice(2)
    if (token.length > 512 || !PATH_SHAPE.test(token) || seen.has(token)) continue
    if (token.split("/").some(part => part === ".." || part === ".")) continue
    seen.add(token)
    if (seen.size > MAX_PATH_CHECKS) break
    try {
      const stat = lstatSync(path.join(root, token))
      if (stat.isFile() || stat.isDirectory()) found.push(token)
    } catch {
      // A missing or unreadable path is not named.
    }
  }
  return found
}

// A derived over-bound query word is junk when it carries no letter or digit (`*`, `^`), is an
// absolute path from another checkout or container (`/app`), or occurs in the prompt only as a
// markup tag name (`<pr_description>`...`</pr_description>`). Junk never strips the query to nothing.
export function stripJunkAnchors(task, prompt) {
  const words = typeof task === "string" ? task.split(/\s+/).filter(Boolean) : []
  // A tag name counts only when the prompt also closes it, so `List<FooBar>` keeps `FooBar`.
  const tagNames = new Set(Array.from(String(prompt ?? "").matchAll(/<\/([A-Za-z_][A-Za-z0-9_.-]*)>/g), m => m[1]))
  const outsideTags = String(prompt ?? "").replace(/<\/?[A-Za-z_][A-Za-z0-9_.-]*[^<>]{0,256}>/g, " ")
  const junk = word =>
    !/[A-Za-z0-9]/.test(word) || word.startsWith("/") ||
    (tagNames.has(word) && !new RegExp(`(?:^|[^A-Za-z0-9_])${word.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}(?:$|[^A-Za-z0-9_])`).test(outsideTags))
  const kept = words.filter(word => !junk(word))
  if (kept.length === 0) return { task, kept: words.length, dropped: 0 }
  return { task: kept.join(" "), kept: kept.length, dropped: words.length - kept.length }
}

const TEST_PATH = /(?:^|\/)(?:tests?|__tests__|testdata|spec)\/|_test\.[A-Za-z]+$|\.(?:test|spec)\.[A-Za-z]+$|(?:^|\/)test_[^/]*\.py$/
export const isTestPath = file => TEST_PATH.test(file)

// The packet's rows as frame candidates: score order (stable; unscored rows keep their place
// after scored ones), a name declared in two or more non-test files demoted behind the rest,
// then one row per file.
export function frameCandidates(results) {
  const rows = []
  for (const [index, row] of (Array.isArray(results) ? results : []).entries()) {
    const evidence = (Array.isArray(row?.evidence) ? row.evidence : []).find(item => typeof item?.path === "string" && item.path !== "")
    if (!evidence) continue
    const candidate = { path: evidence.path }
    if (Number.isInteger(evidence.line) && evidence.line > 0) candidate.line = evidence.line
    if (typeof row.name === "string" && row.name !== "") candidate.symbol = row.name
    if (typeof evidence.authority === "string" && evidence.authority !== "") candidate.authority = evidence.authority
    rows.push({ candidate, index, score: Number.isFinite(row.score) ? row.score : undefined })
  }
  rows.sort((a, b) => (a.score === undefined) - (b.score === undefined) || (b.score ?? 0) - (a.score ?? 0) || a.index - b.index)
  const declaredIn = new Map()
  for (const { candidate } of rows) {
    if (candidate.symbol === undefined || isTestPath(candidate.path)) continue
    if (!declaredIn.has(candidate.symbol)) declaredIn.set(candidate.symbol, new Set())
    declaredIn.get(candidate.symbol).add(candidate.path)
  }
  const ambiguous = row => row.candidate.symbol !== undefined && (declaredIn.get(row.candidate.symbol)?.size ?? 0) >= 2
  const ordered = [...rows.filter(row => !ambiguous(row)), ...rows.filter(ambiguous)]
  const files = new Set()
  return ordered.map(row => row.candidate).filter(candidate => !files.has(candidate.path) && files.add(candidate.path))
}

export const corroborated = candidates => candidates.some(candidate => CORROBORATING_AUTHORITIES.includes(candidate.authority))
