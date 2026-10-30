import { pathToFileURL } from "node:url"

const [modulePath, root, corvintBinary, event] = process.argv.slice(2)
const { CorvintPlugin } = await import(pathToFileURL(modulePath))
const sessionID = "shared-native-session"
const task = "inspect the parser"
const changedPath = "src/../src/parser.py"
console.warn = () => undefined

const plugin = await CorvintPlugin(
  { directory: root, worktree: root },
  { corvintBinary, hostVersion: "unknown", automaticTimeoutMs: 1_000, queryTimeoutMs: 1_000 },
)

if (event === "session-start") {
  await plugin.event({ event: { type: "session.created", properties: { info: { id: sessionID } } } })
} else if (event === "user-prompt") {
  await plugin.tool.corvint_context.execute({ task }, { sessionID, abort: new AbortController().signal })
} else if (event === "file-change") {
  await plugin.event({
    event: { type: "file.edited", properties: { file: changedPath, sessionID } },
  })
} else if (event === "post-tool") {
  await plugin["tool.execute.after"](
    { callID: "call-1", sessionID, tool: "edit" },
    { metadata: { corvint: { changedPaths: [changedPath] } }, output: "not-forwarded" },
  )
} else if (event === "stop") {
  await plugin.event({ event: { type: "session.idle", properties: { sessionID } } })
} else if (event === "session-end") {
  await plugin.event({ event: { type: "session.deleted", properties: { info: { id: sessionID } } } })
} else {
  throw new Error(`unsupported fixture event: ${event}`)
}
