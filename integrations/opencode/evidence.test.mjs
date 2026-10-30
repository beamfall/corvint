import assert from "node:assert/strict"
import { createHash } from "node:crypto"
import { cpSync, existsSync, mkdtempSync, readdirSync, readFileSync, rmSync, unlinkSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import path from "node:path"
import test from "node:test"
import { fileURLToPath } from "node:url"
import { ADAPTER_VERSION } from "./src/runtime.js"

const json = (file) => JSON.parse(readFileSync(file, "utf8"))
const pkg = json(new URL("./package.json", import.meta.url))
const compatibility = json(new URL("../compatibility.json", import.meta.url)).entries.find((entry) => entry.host === "opencode")
const results = fileURLToPath(new URL("../../conformance/opencode-native-v1/results/", import.meta.url))

// The complete opencode-native-integration/1 check set that native.go evaluates; a report that
// omits any of them is not committed evidence, whatever the values of the checks it does hold.
const MANDATORY_NATIVE_CHECKS = [
  "advisory-completion", "awaited-current-prompt", "bounded-prompt", "bounded-stops", "critical-recall-and-bytes",
  "direct-install", "dirty-path-recovery", "environment-filter", "explicit-outcome", "frozen-identities", "governance",
  "interruption-cleanup", "latency", "native-bundled-skill", "native-compaction", "native-discovery", "native-edit",
  "native-exact-expansion", "native-exit", "native-host-image", "native-query", "oversized-exclusion", "snapshot",
  "supplied-evidence", "timed-delivery", "uninstall", "verification-observation",
]

// Validates one retained native report and its AHI-179 interruption sidecar in dir.
function assertNativeEvidence(dir, report, version, tuple) {
  assert.equal(report.profile, "opencode-native-integration/1")
  assert.equal(report.result, "PASS")
  assert.equal(report.hostVersion, version)
  assert.match(report.hostSHA256, /^[0-9a-f]{64}$/)
  assert.equal(`${report.os}-${report.architecture}`, tuple)
  const missing = MANDATORY_NATIVE_CHECKS.filter((name) => !Object.hasOwn(report.checks ?? {}, name))
  assert.deepEqual(missing, [], "the native report holds every mandatory check")
  assert.ok(MANDATORY_NATIVE_CHECKS.every((name) => report.checks[name] === true), "every mandatory native check passed")
  assert.ok(Object.values(report.checks).every((value) => value === true), "every native check passed")
  const observation = report.cleanupEvidence?.observation
  assert.ok(observation, "AHI-179: the report summarizes the interruption observation")
  assert.equal(path.basename(observation.processesFile), observation.processesFile, "the sidecar is named inside the evidence directory")
  const sidecar = path.join(dir, observation.processesFile)
  assert.ok(existsSync(sidecar), `AHI-179: no retained interruption sidecar ${observation.processesFile}`)
  const bytes = readFileSync(sidecar)
  assert.equal(createHash("sha256").update(bytes).digest("hex"), observation.processesSHA256, "the sidecar digest matches the report")
  const retained = JSON.parse(bytes)
  assert.ok(Array.isArray(retained.processes))
  assert.equal(retained.processes.length, observation.processCount, "the summary process count matches the sidecar")
  for (const key of ["absent", "scope", "interval_ms"]) assert.deepEqual(retained[key], observation[key], `the summary ${key} matches the sidecar`)
  assert.deepEqual(retained.failures, observation.failures)
  assert.equal(observation.absent, true)
  assert.deepEqual(observation.failures, [])
}

test("AHI-177 the runtime ADAPTER_VERSION is the package and compatibility adapter version", () => {
  assert.equal(ADAPTER_VERSION, pkg.version)
  assert.equal(ADAPTER_VERSION, pkg.corvintIntegration.adapterVersion)
  assert.equal(ADAPTER_VERSION, compatibility.adapterVersion)
})

test("AHI-177 every tested OpenCode host version names committed native and MCP evidence", () => {
  const tested = pkg.corvintIntegration.testedHostVersions
  assert.ok(tested.length > 0, "testedHostVersions must name retained evidence (V1-0453)")
  assert.ok(tested.includes(compatibility.hostVersion), "compatibility.json hostVersion must be a tested version")
  assert.equal(compatibility.adapterVersion, pkg.version)
  for (const version of tested) {
    const dirs = readdirSync(results).filter((name) => name.startsWith(`${version}-`))
    assert.ok(dirs.length > 0, `no retained evidence directory for OpenCode ${version}`)
    for (const dir of dirs) {
      const where = path.join(results, dir)
      const report = json(path.join(where, "native-report.json"))
      assertNativeEvidence(where, report, version, dir.slice(version.length + 1))
      if (version === compatibility.hostVersion) {
        assert.equal(report.adapterVersion, compatibility.hostVersionEvidenceAdapterVersion)
        // A live native corvint_status call against the completed record, not the MCP tool.
        const status = json(path.join(where, "native-status-probe.json"))
        assert.equal(status.profile, "opencode-native-status-live-probe/0")
        assert.equal(status.result, "PASS")
        assert.equal(status.hostSHA256, report.hostSHA256, "the native status probe ran the same host image")
        const call = JSON.parse(status.statusCallResult)
        assert.equal(call.tool, "corvint_status")
        const qualification = JSON.parse(call.result)
        assert.equal(qualification.integrationSupport, "FULL")
        assert.equal(qualification.reason, "exact-tuple-conformance-passed")
        assert.equal(qualification.adapterVersion, report.adapterVersion)
      }
      const probe = path.join(where, "mcp-probe.json")
      assert.ok(existsSync(probe), `no retained MCP probe for OpenCode ${version}`)
      const mcp = json(probe)
      assert.equal(mcp.profile, "opencode-mcp-live-probe/0")
      assert.equal(mcp.result, "PASS")
      assert.equal(mcp.hostSHA256, report.hostSHA256, "the MCP probe ran the same host image")
      assert.match(mcp.statusCallResult, /"tool":"corvint\.corvint_status"/)
    }
  }
})

test("AHI-179 retained native evidence fails closed on a partial check set or a missing or corrupted sidecar", (t) => {
  const version = compatibility.hostVersion
  const dir = readdirSync(results).find((name) => name.startsWith(`${version}-`))
  const tuple = dir.slice(version.length + 1)
  const source = path.join(results, dir)
  const fresh = () => {
    const copy = mkdtempSync(path.join(tmpdir(), "opencode-evidence-"))
    t.after(() => rmSync(copy, { recursive: true, force: true }))
    cpSync(source, copy, { recursive: true })
    return { copy, report: json(path.join(copy, "native-report.json")) }
  }
  const sidecarName = (report) => report.cleanupEvidence.observation.processesFile

  const baseline = fresh()
  assertNativeEvidence(baseline.copy, baseline.report, version, tuple)

  const partial = fresh()
  partial.report.checks = { "native-bundled-skill": true }
  assert.throws(() => assertNativeEvidence(partial.copy, partial.report, version, tuple), /every mandatory check/)

  const missing = fresh()
  unlinkSync(path.join(missing.copy, sidecarName(missing.report)))
  assert.throws(() => assertNativeEvidence(missing.copy, missing.report, version, tuple), /no retained interruption sidecar/)

  const corrupted = fresh()
  const file = path.join(corrupted.copy, sidecarName(corrupted.report))
  writeFileSync(file, readFileSync(file, "utf8").replace(/"pid": \d+/, '"pid": 1'))
  assert.throws(() => assertNativeEvidence(corrupted.copy, corrupted.report, version, tuple), /sidecar digest matches/)

  const zeroDigest = fresh()
  zeroDigest.report.cleanupEvidence.observation.processesSHA256 = "0".repeat(64)
  assert.throws(() => assertNativeEvidence(zeroDigest.copy, zeroDigest.report, version, tuple), /sidecar digest matches/)

  const disagreeing = fresh()
  disagreeing.report.cleanupEvidence.observation.processCount += 1
  assert.throws(() => assertNativeEvidence(disagreeing.copy, disagreeing.report, version, tuple), /process count matches/)
})

test("AHI-177 the qualification range admits every tested version and the manifest range agrees", async () => {
  const { compatibleHostVersion, SUPPORTED_HOST_RANGE } = await import("./src/qualification.js")
  assert.equal(pkg.corvintIntegration.compatibleHostRange, SUPPORTED_HOST_RANGE)
  for (const version of pkg.corvintIntegration.testedHostVersions) assert.equal(compatibleHostVersion(version), true, version)
})

test("AHI-245 the qualification range admits 2.2.x and refuses 2.3.0, prereleases, leading zeros and pre-2.0.18", async () => {
  const { compatibleHostVersion, SUPPORTED_HOST_RANGE } = await import("./src/qualification.js")
  assert.equal(SUPPORTED_HOST_RANGE, ">=2.0.18 <2.3.0")
  for (const version of ["2.0.18", "2.1.3", "2.2.0", "2.2.1", "2.2.999"]) assert.equal(compatibleHostVersion(version), true, version)
  for (const version of ["2.3.0", "2.3.1", "2.10.0", "3.0.0", "2.2.0-beta", "2.2.01", "2.02.0", "02.2.0", "2.0.17", "2.2.9999999999999999999", undefined]) {
    assert.equal(compatibleHostVersion(version), false, String(version))
  }
})
