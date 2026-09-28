import { ENVELOPE_COLLISION, frameRepositoryData } from "./envelope.js"

const MAX_INJECTED_BYTES = 8_000

// The host calls the context hook for every model request, so the cached session-start envelope is
// pushed on each one; the hook never spawns Corvint.
export function createBetaContextHook({ startupContext, hashSessionId, report }) {
  return (request) => {
    try {
      const sessionKey = hashSessionId(request?.sessionID)
      if (!sessionKey) return
      const envelope = startupContext(sessionKey)
      if (!envelope) return
      const context = frameRepositoryData({
        context: envelope.context,
        receiptId: envelope.receiptId,
        support: envelope.support,
      })
      if (!context) {
        report(ENVELOPE_COLLISION, "session-start")
        return
      }
      if (Buffer.byteLength(context, "utf8") > MAX_INJECTED_BYTES) {
        report("beta-context-too-large", "session-start")
        return
      }
      request.system.push({ type: "text", text: `[Corvint FALLBACK context; receipt-linked, non-authoritative]\n${context}` })
    } catch {
      report("beta-context-failed", "session-start")
    }
  }
}
