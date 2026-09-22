import { ENVELOPE_COLLISION, frameRepositoryData } from "./envelope.js"

const MAX_INJECTED_BYTES = 8_000

export function createBetaContextHooks({ consumeStartupContext, hashSessionId, report }) {
  return {
    "experimental.chat.system.transform": async (input, output) => {
      try {
        const sessionKey = hashSessionId(input?.sessionID)
        if (!sessionKey) return
        const envelope = consumeStartupContext(sessionKey)
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
        output.system.push(`[Corvint FALLBACK context; receipt-linked, non-authoritative]\n${context}`)
      } catch {
        report("beta-context-failed", "session-start")
      }
    },
  }
}
