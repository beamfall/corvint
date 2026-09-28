import { emptyCockpit } from "./cockpit.js"

// Refresh acknowledgments never publish data. One ordered snapshot path supplies the view,
// and a resolved comparison stays pinned until the operator explicitly chooses another base.
export function createCockpitClient({ rpc, signal, publish }) {
  let base = "", scope, epoch = 0, readSerial = 0, pending = false, request, reading
  const stopRead = () => { readSerial++; reading?.abort() }
  const observe = async () => {
    if (pending || !scope || signal.aborted) return
    stopRead(); reading = new AbortController()
    const version = readSerial, generation = epoch, captured = scope
    try {
      const value = await rpc.cockpitSnapshot({ sessionID: captured.sessionID }, { location: captured.location, signal: AbortSignal.any([signal, reading.signal]) })
      if (generation !== epoch || version !== readSerial || signal.aborted) return
      if (value.state === "ready") base = value.base
      publish(value)
    } catch {
      if (generation === epoch && version === readSerial && !signal.aborted) publish(emptyCockpit("unavailable", "Change inspection unavailable. Press r to retry."))
    }
  }
  return {
    observe,
    async refresh(nextScope, selectedBase) {
      if (scope?.sessionID !== nextScope.sessionID || scope?.location.directory !== nextScope.location.directory || scope?.location.workspace !== nextScope.location.workspace) base = ""
      selectedBase ??= base
      const generation = ++epoch
      scope = nextScope; request?.abort(); stopRead(); request = new AbortController(); pending = true
      publish(emptyCockpit("loading", "Reading change and verification evidence…"))
      try {
        await rpc.cockpitRefresh({ sessionID: scope.sessionID, base: selectedBase }, { location: scope.location, signal: AbortSignal.any([signal, request.signal]) })
        if (generation !== epoch || signal.aborted) return
        pending = false
        await observe()
      } catch {
        if (generation === epoch && !signal.aborted) { pending = false; stopRead(); publish(emptyCockpit("unavailable", "Change inspection unavailable. Press r to retry.")) }
      }
    },
    cancel() { epoch++; pending = false; request?.abort(); stopRead() },
  }
}
