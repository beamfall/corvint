import assert from "node:assert/strict"
import { existsSync, readdirSync, readFileSync } from "node:fs"
import test from "node:test"

const json = (url) => JSON.parse(readFileSync(url, "utf8"))
const pkg = json(new URL("./package.json", import.meta.url))
const compatibility = json(new URL("../compatibility.json", import.meta.url)).entries.find((entry) => entry.host === "opencode")
const results = new URL("../../conformance/opencode-native-v1/results/", import.meta.url)

test("AHI-177 every tested OpenCode host version names committed native and MCP evidence", () => {
  const tested = pkg.corvintIntegration.testedHostVersions
  assert.ok(tested.length > 0, "testedHostVersions must name retained evidence (V1-0453)")
  assert.ok(tested.includes(compatibility.hostVersion), "compatibility.json hostVersion must be a tested version")
  assert.equal(compatibility.adapterVersion, pkg.version)
  for (const version of tested) {
    const dirs = readdirSync(results).filter((name) => name.startsWith(`${version}-`))
    assert.ok(dirs.length > 0, `no retained evidence directory for OpenCode ${version}`)
    for (const dir of dirs) {
      const report = json(new URL(`${dir}/native-report.json`, results))
      assert.equal(report.profile, "opencode-native-integration/1")
      assert.equal(report.result, "PASS")
      assert.equal(report.hostVersion, version)
      assert.match(report.hostSHA256, /^[0-9a-f]{64}$/)
      assert.equal(`${report.os}-${report.architecture}`, dir.slice(version.length + 1))
      assert.ok(Object.values(report.checks).every((value) => value === true), "every native check passed")
      assert.equal(report.checks["native-bundled-skill"], true)
      if (version === compatibility.hostVersion) assert.equal(report.adapterVersion, compatibility.hostVersionEvidenceAdapterVersion)
      const probe = new URL(`${dir}/mcp-probe.json`, results)
      assert.ok(existsSync(probe), `no retained MCP probe for OpenCode ${version}`)
      const mcp = json(probe)
      assert.equal(mcp.profile, "opencode-mcp-live-probe/0")
      assert.equal(mcp.result, "PASS")
      assert.equal(mcp.hostSHA256, report.hostSHA256, "the MCP probe ran the same host image")
      assert.match(mcp.statusCallResult, /"tool":"corvint\.corvint_status"/)
    }
  }
})

test("AHI-177 the qualification range admits every tested version and the manifest range agrees", async () => {
  const { compatibleHostVersion, SUPPORTED_HOST_RANGE } = await import("./src/qualification.js")
  assert.equal(pkg.corvintIntegration.compatibleHostRange, SUPPORTED_HOST_RANGE)
  for (const version of pkg.corvintIntegration.testedHostVersions) assert.equal(compatibleHostVersion(version), true, version)
})
