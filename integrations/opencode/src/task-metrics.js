import { randomUUID } from "node:crypto"
import { visibleText } from "./display.js"

const string = { type: "string" }
const integer = { type: "integer", minimum: 0 }
const object = properties => ({ type: "object", properties, required: Object.keys(properties), additionalProperties: false })
const strings = { type: "array", items: string }
export const taskMetricsSchema = object({
  state: string, reason: string, receiptId: string, observed: string, queueId: string, queueDigest: string,
  total: integer, draft: integer, open: integer, completed: integer, held: integer, archived: integer,
  blocked: integer, intentChecksPassed: integer, activeAttempts: integer,
  offset: integer, openTotal: integer, tickets: { type: "array", items: object({
    ticketId: string, title: string, milestone: string, priority: string,
    eligibility: string, nextAction: string, gateResults: string, blockers: integer,
  }) }, gaps: strings,
})

export const emptyTaskMetrics = (state = "empty", reason = "Open Tasks to read queue metrics.") => ({
  state, reason, receiptId: "", observed: "", queueId: "", queueDigest: "", total: 0, draft: 0, open: 0,
  completed: 0, held: 0, archived: 0, blocked: 0, intentChecksPassed: 0,
  activeAttempts: 0, offset: 0, openTotal: 0, tickets: [], gaps: [],
})

const bounded = value => visibleText(typeof value === "string" ? value : "", 1600)
const ticketText = value => typeof value === "string" && value.length <= 16_384
const blockersValid = value => Array.isArray(value) && value.length <= 128 && value.every(row => row && typeof row === "object" && ticketText(row.code) && ticketText(row.detail))
const count = value => {
  if (typeof value !== "string" || !/^(0|[1-9][0-9]{0,8})$/.test(value)) throw new Error("invalid-task-count")
  return Number(value)
}
const digest = result => result?.envelope?.snapshot?.headReceiptSha256
const read = async (run, kind, extra = {}) => {
  const result = await run({ kind, ...extra })
  if (!result?.ok) throw new Error(result?.code || "task-manager-unavailable")
  return result.envelope
}

export async function collectTaskMetrics(run, offset = 0) {
  const status = await read(run, "tasks-status")
  const queue = status.items[0]
  const byStatus = queue?.byStatus || {}
  const metrics = { ...emptyTaskMetrics("ready", "Observed read-only task-manager snapshot"),
    receiptId: randomUUID(), observed: new Date().toISOString(), queueId: bounded(queue.queueId), queueDigest: digest({ envelope: status }),
    total: count(queue.tickets), draft: count(byStatus.DRAFT), open: count(byStatus.OPEN), completed: count(byStatus.COMPLETED),
    held: count(byStatus.HELD), archived: count(byStatus.ARCHIVED), blocked: count(queue.blocked),
    intentChecksPassed: count(queue.intentChecksPassed),
    activeAttempts: Array.isArray(queue.liveAttempts) ? queue.liveAttempts.length : 0,
  }
  if (metrics.draft + metrics.open + metrics.completed + metrics.held + metrics.archived !== metrics.total || !metrics.queueId || !Array.isArray(queue.liveAttempts) || queue.liveAttempts.length > 128 || !queue.liveAttempts.every(row => row && typeof row.attemptId === "string")) throw new Error("inconsistent-task-summary")
  const page = await read(run, "tasks-open", { offset })
  const after = await read(run, "tasks-status")
  if (digest({ envelope: status }) !== digest({ envelope: page }) || digest({ envelope: status }) !== digest({ envelope: after })) throw new Error("task-queue-changed-refresh-required")
  metrics.offset = count(page.page.offset)
  metrics.openTotal = count(page.page.total)
  if (metrics.openTotal !== metrics.open || metrics.offset !== offset || metrics.offset > metrics.openTotal || page.items.length > metrics.openTotal - metrics.offset) throw new Error("inconsistent-open-page")
  metrics.tickets = page.items.map(row => {
    if (typeof row.ticketId !== "string" || row.ticketId.length > 128 || !/^ticket:[A-Za-z0-9_-]+:[A-Za-z0-9_-]+:[A-Za-z0-9_-]+$/.test(row.ticketId) || row.status !== "OPEN" ||
      !ticketText(row.title) || !(row.milestone === null || ticketText(row.milestone)) ||
      !ticketText(row.priority) || !ticketText(row.eligibility) || !ticketText(row.nextAction) ||
      !ticketText(row.gateResults) || !blockersValid(row.blockers)) throw new Error("invalid-task-row")
    return { ticketId: row.ticketId, title: bounded(row.title), milestone: bounded(row.milestone),
      priority: bounded(row.priority), eligibility: bounded(row.eligibility),
      nextAction: bounded(row.nextAction), gateResults: bounded(row.gateResults),
      blockers: row.blockers.length }
  })
  if (metrics.tickets.length > 32) throw new Error("task-page-too-large")
  if (metrics.offset + metrics.tickets.length < metrics.openTotal) metrics.gaps.push(`${metrics.openTotal - metrics.offset - metrics.tickets.length} later open tickets are outside this page.`)
  if (metrics.offset > 0) metrics.gaps.push(`${metrics.offset} earlier open tickets are outside this page.`)
  metrics.gaps.push("Queue counts and eligibility are task-manager observations; gate results and completion remain separate evidence.")
  return metrics
}

export async function inspectTask(run, snapshot, ticketId) {
  if (snapshot?.state !== "ready" || !snapshot.tickets.some(row => row.ticketId === ticketId)) throw new Error("ticket-not-in-current-page")
  const detail = await read(run, "tasks-detail", { ticketId })
  const status = await read(run, "tasks-status")
  if (digest({ envelope: detail }) !== digest({ envelope: status }) || digest({ envelope: detail }) !== snapshot.queueDigest) throw new Error("task-queue-changed-refresh-required")
  const item = detail.items[0], record = item?.record
  if (item?.ticketId !== ticketId || item.status !== "OPEN" || !record || typeof record !== "object" || Array.isArray(record) ||
    !ticketText(item.title) || !ticketText(item.status) || !ticketText(item.eligibility) ||
    !ticketText(item.priority) || !(item.milestone === null || ticketText(item.milestone)) ||
    !ticketText(item.nextAction) || !ticketText(item.gateResults) ||
    !(item.completion === null || ticketText(item.completion)) || !blockersValid(item.blockers) ||
    !Array.isArray(record.acceptanceCriteria) || record.acceptanceCriteria.length > 256 ||
    !record.acceptanceCriteria.every(ticketText)) throw new Error("task-detail-unavailable")
  const lines = [
    `${bounded(item.title)} (${ticketId})`,
    `${bounded(item.status)} · ${bounded(item.eligibility)} · ${bounded(item.priority)} · ${bounded(item.milestone)}`,
    `Next action: ${bounded(item.nextAction) || "NOT_OBSERVED"}`,
    `Gate results: ${bounded(item.gateResults) || "NOT_OBSERVED"}`,
    `Completion: ${bounded(item.completion) || "NOT_OBSERVED"}`,
    "", "Blockers",
    ...(item.blockers.length ? item.blockers.slice(0, 32).map(row => `• ${bounded(row.code)}: ${bounded(row.detail)}`) : ["None reported by the task-manager read."]),
    "", "Acceptance criteria",
    ...(record.acceptanceCriteria.length ? record.acceptanceCriteria.slice(0, 16).map((value, index) => `${index + 1}. ${bounded(value)}`) : ["No criteria supplied by the task-manager read."]),
  ]
  if ((item.blockers?.length || 0) > 32 || (record.acceptanceCriteria?.length || 0) > 16) lines.push("Further detail omitted by the display limit.")
  return lines.join("\n")
}
