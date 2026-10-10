package typescript

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	json "encoding/json/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const discoveryFixtureRevision = "1111111111111111111111111111111111111111"

// Input membership is provided by an independent fixture oracle, never selector output.
func discoveryFixtureBytes(t *testing.T, root string, units []PlaywrightDiscoveryUnit) []byte {
	return discoveryFixtureBytesForConfig(t, root, "playwright.config.ts", units)
}

func discoveryFixtureBytesForConfig(t *testing.T, root, configPath string, units []PlaywrightDiscoveryUnit) []byte {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, configPath))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	digest, err := ObservePlaywrightSources(root, configPath)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(PlaywrightDiscovery{Profile: "playwright-discovery/0", Revision: discoveryFixtureRevision, Config: PlaywrightConfigIdentity{Path: configPath, SHA256: hex.EncodeToString(sum[:])}, SourceDigest: digest, Units: units}, json.Deterministic(true))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func smallDiscoveryUnits() []PlaywrightDiscoveryUnit {
	return []PlaywrightDiscoveryUnit{
		{Project: "angular", Test: "tests/login.spec.ts"}, {Project: "angular", Test: "tests/other.spec.ts"},
		{Project: "chromium", Test: "tests/login.spec.ts"}, {Project: "chromium", Test: "tests/other.spec.ts"},
		{Project: "cleanup", Test: "tests/global.teardown.ts"},
		{Project: "react", Test: "tests/login.spec.ts"}, {Project: "react", Test: "tests/other.spec.ts"},
		{Project: "setup", Test: "tests/global.setup.ts"},
	}
}

func TestPlaywrightDiscoveryReconciliation(t *testing.T) {
	t.Run("TJAA-V0-013 custom config global setup helper reaches complete matched suite", func(t *testing.T) {
		root := t.TempDir()
		write(t, root, "package.json", `{"devDependencies":{"@playwright/test":"1.63.0"}}`)
		write(t, root, "e2e.config.ts", `export default {globalSetup:"./support/setup.ts",projects:[{name:"chromium",testDir:"tests"}]}`)
		write(t, root, "support/setup.ts", `import { setup } from "./helper"; export default setup`)
		write(t, root, "support/helper.ts", `export const setup = () => {}`)
		write(t, root, "tests/a.spec.ts", `import {test} from "@playwright/test";test("a",()=>{})`)
		write(t, root, "tests/b.spec.ts", `import {test} from "@playwright/test";test("b",()=>{})`)
		units := []PlaywrightDiscoveryUnit{{Project: "chromium", Test: "tests/a.spec.ts"}, {Project: "chromium", Test: "tests/b.spec.ts"}}
		raw := discoveryFixtureBytesForConfig(t, root, "e2e.config.ts", units)
		plan, err := SelectPlaywright(root, "e2e.config.ts", discoveryFixtureRevision, []string{"support/helper.ts"}, raw)
		if err != nil {
			t.Fatal(err)
		}
		if plan.Scope != "BOUNDED" || plan.Discovery.State != "MATCHED" || len(plan.Selected) != 2 || len(plan.Excluded) != 0 {
			t.Fatal(plan)
		}
		for _, unit := range plan.Selected {
			if !strings.HasSuffix(unit.Test, ".spec.ts") {
				t.Fatal("helper argv", unit)
			}
		}
	})
	t.Run("TJAA-V0-012 discovered membership excludes helper argv", func(t *testing.T) {
		root := playwrightFixture(t)
		write(t, root, "tests/api.ts", `export const api = 1`)
		raw := discoveryFixtureBytes(t, root, smallDiscoveryUnits())
		plan, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, []string{"tests/pages/login.ts"}, raw)
		if err != nil {
			t.Fatal(err)
		}
		if plan.Discovery.State != "MATCHED" || len(plan.Selected) != 5 || len(plan.Excluded) != 3 || len(plan.FallbackArgv) != 0 {
			t.Fatalf("%+v", plan)
		}
		for _, unit := range plan.Selected {
			if !strings.Contains(unit.Test, ".spec.") && !strings.Contains(unit.Test, ".setup.") && !strings.Contains(unit.Test, ".teardown.") {
				t.Fatal("helper became runnable", unit)
			}
		}
		unrelated, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, []string{"tests/api.ts"}, raw)
		if err != nil || len(unrelated.Selected) != 0 || unrelated.Discovery.State != "MATCHED" {
			t.Fatal(unrelated, err)
		}
	})
	t.Run("TJAA-V0-015 unproven universe uses one suite command", func(t *testing.T) {
		root := playwrightFixture(t)
		valid := discoveryFixtureBytes(t, root, smallDiscoveryUnits())
		for _, row := range []struct {
			name  string
			edit  func([]byte) []byte
			state string
		}{
			{"absent", func([]byte) []byte { return nil }, "MISSING"},
			{"truncated", func(b []byte) []byte { return b[:len(b)-2] }, "MALFORMED"},
			{"duplicate-key", func(b []byte) []byte {
				return bytes.Replace(b, []byte(`"profile":`), []byte(`"profile":"playwright-discovery/0","profile":`), 1)
			}, "MALFORMED"},
			{"unknown-field", func(b []byte) []byte { return append([]byte(`{"extra":true,`), b[1:]...) }, "MALFORMED"},
			{"stale-revision", func(b []byte) []byte {
				return bytes.ReplaceAll(b, []byte(discoveryFixtureRevision), []byte(strings.Repeat("2", 40)))
			}, "BINDING_MISMATCH"},
			{"noncanonical-path", func(b []byte) []byte {
				return bytes.ReplaceAll(b, []byte("tests/login.spec.ts"), []byte("tests/../tests/login.spec.ts"))
			}, "MALFORMED"},
			{"profile", func(b []byte) []byte {
				return bytes.ReplaceAll(b, []byte("playwright-discovery/0"), []byte("playwright-discovery/1"))
			}, "MALFORMED"},
			{"bound", func([]byte) []byte { return bytes.Repeat([]byte("x"), PlaywrightDiscoveryMaxBytes+1) }, "MALFORMED"},
		} {
			t.Run(row.name, func(t *testing.T) {
				plan, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, []string{"tests/pages/login.ts"}, row.edit(valid))
				if err != nil {
					t.Fatal(err)
				}
				assertDiscoveryFallback(t, plan, row.state)
			})
		}
		for _, mutate := range []func(*PlaywrightDiscovery){
			func(r *PlaywrightDiscovery) { r.Config.SHA256 = strings.Repeat("0", 64) },
			func(r *PlaywrightDiscovery) { r.SourceDigest = "playwright-sources:sha256:" + strings.Repeat("0", 64) },
		} {
			var r PlaywrightDiscovery
			_ = json.Unmarshal(valid, &r)
			mutate(&r)
			raw, _ := json.Marshal(r, json.Deterministic(true))
			plan, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, nil, raw)
			if err != nil {
				t.Fatal(err)
			}
			assertDiscoveryFallback(t, plan, "BINDING_MISMATCH")
		}
	})
	t.Run("TJAA-V0-017 exact discovery differences and canonical repeated bytes", func(t *testing.T) {
		root := playwrightFixture(t)
		for _, units := range [][]PlaywrightDiscoveryUnit{
			smallDiscoveryUnits()[1:],
			append([]PlaywrightDiscoveryUnit{{Project: "angular", Test: "tests/api.ts"}}, smallDiscoveryUnits()...),
		} {
			raw := discoveryFixtureBytes(t, root, units)
			plan, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, nil, raw)
			if err != nil {
				t.Fatal(err)
			}
			assertDiscoveryFallback(t, plan, "UNIVERSE_MISMATCH")
			if len(plan.Discovery.OnlyInReceipt)+len(plan.Discovery.OnlyInStatic) != 1 {
				t.Fatal(plan.Discovery)
			}
			second, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, nil, raw)
			if err != nil {
				t.Fatal(err)
			}
			a, _ := plan.Canonical()
			b, _ := second.Canonical()
			if !bytes.Equal(a, b) {
				t.Fatal("unstable bytes")
			}
		}
		for _, units := range [][]PlaywrightDiscoveryUnit{append(smallDiscoveryUnits(), smallDiscoveryUnits()[0]), {smallDiscoveryUnits()[1], smallDiscoveryUnits()[0]}} {
			plan, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, nil, discoveryFixtureBytes(t, root, units))
			if err != nil {
				t.Fatal(err)
			}
			assertDiscoveryFallback(t, plan, "MALFORMED")
		}
	})
	t.Run("TJAA-V0-015 matched dynamic closure widens only to discovered files", func(t *testing.T) {
		root := playwrightFixture(t)
		write(t, root, "tests/pages/login.ts", `export const page = import(process.env.PAGE)`)
		plan, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, []string{"tests/pages/login.ts"}, discoveryFixtureBytes(t, root, smallDiscoveryUnits()))
		if err != nil {
			t.Fatal(err)
		}
		if plan.Scope != "UNKNOWN" || plan.Discovery.State != "MATCHED" || len(plan.Selected) != 8 || len(plan.Excluded) != 0 || len(plan.FallbackArgv) != 0 {
			t.Fatal(plan)
		}
	})
}

func assertDiscoveryFallback(t *testing.T, plan PlaywrightPlan, state string) {
	t.Helper()
	if plan.Discovery.State != state || plan.Scope != "UNKNOWN" || plan.Fallback != PlaywrightFallbackFullSuite || len(plan.Selected) != 0 || len(plan.Excluded) != 0 || !slices.Equal(plan.FallbackArgv, []string{"npx", "playwright", "test", "--config=playwright.config.ts"}) {
		t.Fatalf("%+v", plan)
	}
}

func TestPlaywrightDiscoveryUnnamedProject(t *testing.T) {
	t.Run("AFU-V1-019 explicit default project identity", func(t *testing.T) {
		root := t.TempDir()
		write(t, root, "playwright.config.ts", `export default {testDir:"tests"}`)
		write(t, root, "tests/a.spec.ts", `test("a",()=>{})`)
		raw := discoveryFixtureBytes(t, root, []PlaywrightDiscoveryUnit{{Project: "", Test: "tests/a.spec.ts"}})
		units, state := VerifyPlaywrightDiscovery(root, "playwright.config.ts", discoveryFixtureRevision, raw)
		if state != "MATCHED" || len(units) != 1 || units[0].Project != "" {
			t.Fatalf("default project: %s %+v", state, units)
		}
		for _, bad := range [][]byte{bytes.Replace(raw, []byte(`"project":"",`), nil, 1), bytes.Replace(raw, []byte(`"project":""`), []byte(`"project":null`), 1)} {
			if _, state := VerifyPlaywrightDiscovery(root, "playwright.config.ts", discoveryFixtureRevision, bad); state != "MALFORMED" {
				t.Fatalf("missing/null project: %s", state)
			}
		}
	})
}

// GitHub #709 part 3 (V1-1067): a MALFORMED receipt names one stable reason and a one-line
// detail (TJAA-V0-019); every other state carries neither.
func TestPlaywrightDiscoveryMalformedReason_V1_1067(t *testing.T) {
	root := playwrightFixture(t)
	valid := discoveryFixtureBytes(t, root, smallDiscoveryUnits())
	indented := func(b []byte) []byte {
		var out bytes.Buffer
		var value any
		if err := json.Unmarshal(b, &value); err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(value, json.Deterministic(true))
		if err != nil {
			t.Fatal(err)
		}
		out.Write(bytes.ReplaceAll(raw, []byte(`,"`), []byte(",\n  \"")))
		return out.Bytes()
	}
	for _, row := range []struct {
		name, reason, detail string
		edit                 func([]byte) []byte
	}{
		{"truncated", "DECODE_FAILED", "", func(b []byte) []byte { return b[:len(b)-2] }},
		{"unknown-field", "DECODE_FAILED", `"extra"`, func(b []byte) []byte { return append([]byte(`{"extra":true,`), b[1:]...) }},
		{"playwright-report", "DECODE_FAILED", "corvint affected discovery", func([]byte) []byte {
			return []byte(`{"config":{"rootDir":"/r"},"suites":[],"errors":[]}`)
		}},
		{"bound", "DECODE_FAILED", "4194304", func([]byte) []byte { return bytes.Repeat([]byte("x"), PlaywrightDiscoveryMaxBytes+1) }},
		{"indented", "NON_CANONICAL_BYTES", "byte", indented},
		{"missing-member", "NON_CANONICAL_BYTES", "byte", func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"profile":"playwright-discovery/0",`), nil, 1)
		}},
		{"profile", "INVALID_FIELD", "profile", func(b []byte) []byte {
			return bytes.ReplaceAll(b, []byte("playwright-discovery/0"), []byte("playwright-discovery/1"))
		}},
		{"path", "INVALID_FIELD", "units[0].test", func(b []byte) []byte {
			return bytes.Replace(b, []byte("tests/login.spec.ts"), []byte("tests/../tests/login.spec.ts"), 1)
		}},
		{"revision", "INVALID_FIELD", "revision", func(b []byte) []byte {
			return bytes.ReplaceAll(b, []byte(discoveryFixtureRevision), []byte(strings.Repeat("A", 40)))
		}},
	} {
		t.Run(row.name, func(t *testing.T) {
			plan, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, nil, row.edit(valid))
			if err != nil {
				t.Fatal(err)
			}
			assertDiscoveryFallback(t, plan, "MALFORMED")
			t.Logf("%s: %s", plan.Discovery.Reason, plan.Discovery.Detail)
			if plan.Discovery.Reason != row.reason || !strings.Contains(plan.Discovery.Detail, row.detail) || plan.Discovery.Detail == "" || strings.ContainsAny(plan.Discovery.Detail, "\r\n") {
				t.Fatalf("reason=%q detail=%q", plan.Discovery.Reason, plan.Discovery.Detail)
			}
			if _, state := VerifyPlaywrightDiscovery(root, "playwright.config.ts", discoveryFixtureRevision, row.edit(valid)); state != "MALFORMED" {
				t.Fatal(state)
			}
		})
	}
	unsorted := discoveryFixtureBytes(t, root, []PlaywrightDiscoveryUnit{smallDiscoveryUnits()[1], smallDiscoveryUnits()[0]})
	plan, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, nil, unsorted)
	if err != nil || plan.Discovery.Reason != "INVALID_FIELD" || !strings.Contains(plan.Discovery.Detail, "units[1]") {
		t.Fatalf("%+v %v", plan.Discovery, err)
	}
	for _, raw := range [][]byte{nil, valid} {
		plan, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, nil, raw)
		if err != nil || plan.Discovery.Reason != "" || plan.Discovery.Detail != "" {
			t.Fatalf("%+v %v", plan.Discovery, err)
		}
		encoded, err := json.Marshal(plan.Discovery, json.Deterministic(true))
		if err != nil || bytes.Contains(encoded, []byte(`"reason"`)) || bytes.Contains(encoded, []byte(`"detail"`)) {
			t.Fatalf("reason member emitted outside MALFORMED: %s %v", encoded, err)
		}
	}
}
