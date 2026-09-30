import { emptyTaskDetail } from "./task-metrics.js"
import { visibleText } from "./display.js"

const string = { type: "string" }
const strings = { type: "array", items: string }
const object = properties => ({ type: "object", properties, required: Object.keys(properties), additionalProperties: false })
const action = object({ kind: string, text: string })
const criterion = object({ text: string, status: string, requirements: strings, reason: string })
const check = object({ id: string, status: string, testedCommit: string })
const attempt = object({ holder: string, phase: string, expiresAt: string, attemptId: string })
export const workbenchSchema = object({
  state: string, reason: string, ticketId: string, title: string, queueDigest: string, taskRevision: string,
  eligibility: string, nextAction: string, gateResults: string, completion: string, attempt,
  criteria: { type: "array", items: criterion }, requirements: strings, declaredPaths: strings,
  changedPaths: strings, plannedChecks: strings, observedChecks: { type: "array", items: check },
  contextReceipt: string, changeReceipt: string, workflow: string, blockers: strings, gaps: strings,
  actions: { type: "array", items: action }, doctor: object({ support: string, reason: string,
    hostVersion: string, adapterVersion: string, platform: string, action: string, failedCases: strings }),
})
const emptyDoctor = () => ({ support: "UNQUALIFIED", reason: "Qualification has not been read.", hostVersion: "unknown", adapterVersion: "unknown", platform: "unknown", action: "Open the workbench and refresh qualification.", failedCases: [] })
export const emptyWorkbench = (state = "empty", reason = "Select an open ticket in Tasks and press s to focus it.") => ({
  state, reason, ticketId: "", title: "", queueDigest: "", taskRevision: "", eligibility: "", nextAction: "",
  gateResults: "", completion: "", attempt: emptyTaskDetail().attempt, criteria: [], requirements: [],
  declaredPaths: [], changedPaths: [], plannedChecks: [], observedChecks: [], contextReceipt: "",
  changeReceipt: "", workflow: "NOT_OBSERVED", blockers: [], gaps: [], actions: [], doctor: emptyDoctor(),
})

const safe = value => visibleText(typeof value === "string" ? value : "", 1600)
const distinct = values => [...new Set(values)].slice(0, 32)

export function projectWorkbench(detail, tasks, cockpit, context, qualification) {
  if (!detail || detail.state !== "ready") return emptyWorkbench()
  if (tasks?.state !== "ready" || tasks.queueDigest !== detail.queueDigest) return {
    ...emptyWorkbench("stale", "Task queue changed. Refresh Tasks and select the ticket again."),
    ticketId: detail.ticketId, title: detail.title,
  }
  const changeReady = cockpit?.state === "ready"
  const declaredPaths = detail.touchPaths.slice(0, 32)
  const changedPaths = changeReady ? cockpit.files.filter(file => declaredPaths.includes(file)).slice(0, 32) : []
  const requirements = detail.requirementRefs.slice(0, 32)
  const criteria = detail.criteria.map(value => ({
    text: value,
    status: "UNOBSERVED",
    requirements: requirements.filter(ref => value.includes(ref)),
    reason: "No criterion-specific proof or accepted completion is linked to this queue receipt.",
  }))
  const observedChecks = changeReady ? cockpit.checks.slice(0, 32).map(row => ({ id: safe(row.id), status: safe(row.status), testedCommit: safe(row.testedCommit) })) : []
  const plannedChecks = changeReady ? distinct(cockpit.declarations.map(row => safe(row.command))) : []
  const blockers = detail.blockers.slice(0, 32).map(row => `${row.code}: ${row.detail}`)
  const gaps = [...detail.gaps]
  if (!changeReady) gaps.push(`Change evidence ${safe(cockpit?.state || "NOT_OBSERVED")}; refresh the Change view.`)
  else {
    if (declaredPaths.length && !changedPaths.length) gaps.push("No declared ticket path matches the observed changed-file list.")
    if (cockpit.checks.length > 32) gaps.push(`${cockpit.checks.length - 32} recorded checks omitted from this workbench.`)
    if (cockpit.declarations.length > 32) gaps.push(`${cockpit.declarations.length - 32} suggested checks omitted from this workbench.`)
    gaps.push(...(cockpit.gaps || []).slice(0, 16).map(safe))
    gaps.push("Changed paths, context citations, and checks are task-level leads; no criterion is marked satisfied without an explicit proof link.")
  }
  if (context?.state !== "ready") gaps.push("Governing context is not observed for this session.")
  else gaps.push(...(context.gaps || []).slice(0, 8).map(safe))
  if (!requirements.length) gaps.push("The ticket declares no requirement references.")
  const actions = []
  if (detail.nextAction && detail.nextAction !== "NOT_OBSERVED") actions.push({ kind: "tasks", text: `Task manager next action: ${detail.nextAction}` })
  for (const blocker of blockers.slice(0, 2)) actions.push({ kind: "tasks", text: `Resolve ${blocker}` })
  for (const check of observedChecks.filter(row => ["FAIL", "STALE", "TIMEOUT", "CANCELLED", "WITHHELD"].includes(row.status)).slice(0, 2)) actions.push({ kind: "change", text: `Inspect ${check.status.toLowerCase()} check ${check.id}` })
  if (criteria.length) actions.push({ kind: "proof", text: `Link proof to ${criteria.length} unobserved acceptance criteria.` })
  if (qualification?.integrationSupport !== "FULL") actions.push({ kind: "doctor", text: `Inspect native qualification: ${safe(qualification?.reason || "NOT_OBSERVED")}` })
  const doctor = qualification ? {
    support: safe(qualification.integrationSupport), reason: safe(qualification.reason),
    hostVersion: safe(qualification.hostVersion), adapterVersion: safe(qualification.adapterVersion),
    platform: `${safe(qualification.operatingSystem)}/${safe(qualification.architecture)}`,
    action: safe(qualification.qualificationAction),
    failedCases: Array.isArray(qualification.failedCases) ? qualification.failedCases.slice(0, 14).map(safe) : [],
  } : emptyDoctor()
  return { ...emptyWorkbench("ready", "Read-only, receipt-bound workbench"), ticketId: detail.ticketId,
    title: detail.title, queueDigest: detail.queueDigest, taskRevision: detail.revision,
    eligibility: detail.eligibility, nextAction: detail.nextAction, gateResults: detail.gateResults,
    completion: detail.completion, attempt: detail.attempt, criteria, requirements, declaredPaths,
    changedPaths, plannedChecks, observedChecks, contextReceipt: context?.state === "ready" ? safe(context.receiptId) : "",
    changeReceipt: changeReady ? safe(cockpit.receiptId) : "", workflow: changeReady ? safe(cockpit.workflow) : "NOT_OBSERVED",
    blockers, gaps: gaps.slice(0, 32), actions: actions.slice(0, 5), doctor,
  }
}
