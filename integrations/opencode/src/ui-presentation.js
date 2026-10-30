// These source-owned limitations also populate the existing RPC gaps field.
// Only exact matches are separated from actionable attention; unknown gaps stay visible.
export const changeLimitations = Object.freeze([
  "Affected selection is advisory. Missing connections do not establish that a file is unaffected or that tests can be skipped.",
  "Verification records are caller-owned local observations; no execution authority or complete coverage is asserted.",
])
export const changeSections = Object.freeze([
  { key: "files", label: "Files" }, { key: "impact", label: "Impact" },
  { key: "proof", label: "Checks" }, { key: "gaps", label: "Attention" },
])

export function changePresentation(view) {
  const attention = view.gaps.filter(gap => !changeLimitations.includes(gap))
  const counts = `${view.files.length} files · ${view.impacts.length} impacts · ${view.checks.length} checks · ${attention.length} attention`
  const states = { empty: "Inspect your change", loading: "Reading change…", stale: "Snapshot needs refresh", unavailable: "Change unavailable" }
  const heading = view.state === "ready" ? "Change snapshot" : states[view.state] || "Change state unknown"
  return { attention, limitations: changeLimitations, heading, summary: view.state === "ready" ? counts : view.reason, counts }
}

export function checkPresentation(check, state) {
  const labels = { PASS: "Passed · recorded commit", FAIL: "Failed", STALE: "Stale result", CANCELLED: "Cancelled", TIMEOUT: "Timed out", WITHHELD: "Output withheld", UNVERIFIED: "Unverified", "NOT RUN": "Not run" }
  return {
    label: `${labels[check.status] || check.status} · ${check.id}`,
    tone: state !== "ready" ? "warning" : check.status === "PASS" ? "info" : check.status === "FAIL" ? "error" : "warning",
    sub: state !== "ready" ? "Snapshot out of date · refresh to inspect" : check.testedCommit ? `Tested ${check.testedCommit.slice(0, 8)}` : "No execution recorded",
  }
}

export function changeEmpty(view, section, filtered = false) {
  if (view.state !== "ready") {
    if (view.state === "loading") return "Reading paths, impact and recorded checks…"
    if (view.state === "stale") return "This snapshot is out of date. Press r to refresh before inspecting evidence."
    if (view.state === "unavailable") return `${view.reason}\nPress r to retry or b to choose a comparison revision.`
    return `${view.reason}\nPress r to inspect this change.`
  }
  if (section === "files") return "No changed paths supplied for this comparison. Press b to choose another base or r after edits."
  if (section === "impact") return `No dependency witnesses supplied${filtered ? " for this file" : ""}. This does not establish that files are unaffected.\n${filtered ? "Press x to show all impact witnesses." : "Press e to inspect context and governing evidence."}`
  if (section === "proof") return "No recorded check results. Suggested checks have not been run by this UI. Press e to inspect governing evidence, then r after recording checks."
  return "No actionable gaps supplied. This does not establish complete coverage. Tab shows scope limitations."
}
