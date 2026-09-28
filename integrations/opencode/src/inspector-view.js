import { visibleText } from "./inspector.js"

export const evidenceKey = row => JSON.stringify([row.handle, row.path, row.line, row.title, row.reason])
export function filterEvidence(rows, query) {
  const terms = query.trim().toLocaleLowerCase().split(/\s+/u).filter(Boolean)
  return rows.filter(row => terms.every(term => [row.path, row.title, row.kind, row.reason, row.authority, row.confidence].some(value => value.toLocaleLowerCase().includes(term))))
}
export function selectedEvidence(rows, key) {
  return rows.find(row => evidenceKey(row) === key) || rows[0]
}
export function evidenceStatus(snapshot) {
  if (snapshot.state === "ready") return { label: "Ready", tone: "info", action: "Browse evidence" }
  if (snapshot.state === "loading") return { label: "Collecting", tone: "info", action: "Waiting for Core" }
  if (snapshot.state === "stale") return { label: "Stale", tone: "warning", action: "Request fresh context" }
  if (snapshot.state === "unavailable") return { label: "Unavailable", tone: "error", action: "Retry context request" }
  return { label: "No context yet", tone: "info", action: "Request context" }
}
export function sourceDocument(text, path, citedLine) {
  // CRLF is one source line. Other controls stay visible without creating terminal instructions.
  const lines = text.replace(/\r\n/g, "\n").split("\n")
  if (lines.at(-1) === "") lines.pop()
  const citation = Number.isSafeInteger(citedLine) && citedLine > 0 && citedLine <= lines.length ? citedLine : null
  const extension = path.split(".").at(-1)?.toLowerCase()
  const filetype = { js: "javascript", mjs: "javascript", cjs: "javascript", jsx: "javascript", ts: "typescript", tsx: "tsx", go: "go", py: "python", rs: "rust", json: "json", sh: "bash", md: "markdown" }[extension]
  return { content: lines.map(line => visibleText(line, 24000)).join("\n"), lineCount: lines.length, citation, filetype }
}
