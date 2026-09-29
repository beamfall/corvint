import { createHash } from "node:crypto"
import { accessSync, constants, createReadStream, lstatSync, readFileSync, readdirSync } from "node:fs"
import path from "node:path"
import { fileURLToPath } from "node:url"
import { ADAPTER_VERSION } from "./runtime.js"

const packageRoot = fileURLToPath(new URL("../", import.meta.url))
const reportPath = fileURLToPath(new URL("../../opencode-qualification.json", import.meta.url))
const CASES = ["install", "snapshot", "context", "expansion", "observations", "frontier", "degradation", "privacy", "normalization", "recursion", "compaction", "latency", "recall", "cleanup"]
const SUPPORTED_HOST_RANGE = ">=2.0.18 <2.1.0"
const sha = bytes => createHash("sha256").update(bytes).digest("hex")

function compatibleHostVersion(version) {
  const match = /^2\.0\.(0|[1-9]\d{0,17})$/.exec(version ?? "")
  return match !== null && BigInt(match[1]) >= 18n
}

function resolveBinary(binary, environment, root) {
  if (binary.includes(path.sep)) return path.resolve(root, binary)
  for (const directory of (environment.PATH ?? "").split(path.delimiter)) {
    const candidate = path.resolve(root, directory, binary)
    try {
      accessSync(candidate, constants.X_OK)
      return candidate
    } catch {}
  }
  throw new Error("binary unavailable")
}

async function executableDigest(file, signal) {
  const hash = createHash("sha256")
  let bytes = 0
  for await (const chunk of createReadStream(file, { signal })) {
    bytes += chunk.length
    if (bytes > 1024 * 1024 * 1024) throw new Error("executable over qualification bound")
    hash.update(chunk)
  }
  return hash.digest("hex")
}

// Qualification is maintainer test evidence, never execution attestation or authority.
// Read/hash only on an explicit status request, outside automatic lifecycle latency.
export async function qualificationStatus({ hostVersion, corvintBinary, environment, root, signal }) {
  const result = {
    profile: "opencode-native-integration/1",
    integrationSupport: "UNQUALIFIED",
    hostVersion: hostVersion ?? "unknown",
    adapterVersion: ADAPTER_VERSION,
    operatingSystem: process.platform,
    architecture: process.arch,
    executionAuthority: "NONE",
    frontier: "UNAVAILABLE",
    legacyReceiptSupport: "FALLBACK",
    supportedHostRange: SUPPORTED_HOST_RANGE,
    qualificationAction: "from a clean Corvint checkout, run GOTOOLCHAIN=local go run ./tools/qualify-opencode --host /absolute/path/to/opencode --corvint /absolute/path/to/corvint --output /absolute/evidence; see integrations/opencode/README.md#qualify-this-installation",
    reason: compatibleHostVersion(hostVersion) ? "qualification-record-unavailable" : "unsupported-host-version",
  }
  if (!compatibleHostVersion(hostVersion)) return result
  try {
    if (lstatSync(reportPath).size > 131072) return result
    const report = JSON.parse(readFileSync(reportPath, "utf8"))
    if (report.profile !== result.profile || report.result !== "PASS" ||
        report.adapterVersion !== ADAPTER_VERSION || report.hostVersion !== hostVersion ||
        report.os !== process.platform || report.architecture !== process.arch ||
        report.executionAuthority !== "NONE" ||
        !CASES.every(name => report.conformance?.[name] === "PASS")) {
      return { ...result, reason: "qualification-tuple-or-gate-mismatch" }
    }
    const files = readdirSync(packageRoot, { recursive: true }).filter(name => lstatSync(path.join(packageRoot, name)).isFile()).sort()
    const expected = Object.keys(report.sourceFiles ?? {}).sort()
    if (JSON.stringify(files.map(name => "integrations/opencode/" + name)) !== JSON.stringify(expected) ||
        files.some(name => sha(readFileSync(path.join(packageRoot, name))) !== report.sourceFiles["integrations/opencode/" + name])) {
      return { ...result, reason: "qualification-package-changed" }
    }
    if (await executableDigest(process.execPath, signal) !== report.hostSHA256 ||
        await executableDigest(resolveBinary(corvintBinary, environment, root), signal) !== report.corvintSHA256) {
      return { ...result, reason: "qualification-executable-changed" }
    }
    return { ...result, integrationSupport: "FULL", reason: "exact-tuple-conformance-passed" }
  } catch {
    return result
  }
}
