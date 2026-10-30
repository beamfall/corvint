package testsource

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/liveverify/affected/typescript"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

const (
	bunUnitTest = `import { describe, expect, test } from "bun:test";
import { add } from "../../src/math";

describe("math", () => {
  test("adds", () => {
    expect(add(1, 1)).toBe(2);
  });
});
`
	bunIntegrationTest = `import { expect, test } from "bun:test";

test("db ready", () => {
  expect((globalThis as any).DB_READY).toBe(true);
});
`
	bunJUnitReport = `<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="bun test" tests="2" assertions="2" failures="0" skipped="0" time="0.01">
  <testsuite name="test/unit/math.test.ts" file="test/unit/math.test.ts" tests="1">
    <testsuite name="math" file="test/unit/math.test.ts" line="4" tests="1">
      <testcase name="adds" classname="math" time="0" file="test/unit/math.test.ts" line="5" assertions="1" />
    </testsuite>
  </testsuite>
  <testsuite name="test/integration/db.test.ts" file="test/integration/db.test.ts" tests="1">
    <testcase name="db ready" classname="" time="0" file="test/integration/db.test.ts" line="3" assertions="1" />
  </testsuite>
</testsuites>
`
)

// TestLPCVV0261_BunFixtureSelectedRunAndJoined pins V1-2191 acceptance 1 (LPCV-V0-261): a fixture
// repository with bun:test unit and integration suites is selected by affected as bun-test units,
// run by the bun profile (a fake bun replaying a bun 1.3.11 JUnit report), and its receipt joins to
// test-validity verdicts the way a Vitest receipt does.
func TestLPCVV0261_BunFixtureSelectedRunAndJoined(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake bun is a POSIX shell script")
	}
	root := t.TempDir()
	commit := commitFiles(t, root, map[string]string{
		"package.json":                "{\"name\":\"x\"}\n",
		"bun.lock":                    "{\n  \"lockfileVersion\": 1,\n  \"workspaces\": {\n    \"\": {\n      \"name\": \"x\",\n    },\n  },\n}\n",
		"src/math.ts":                 "export function add(a: number, b: number): number {\n  return a + b;\n}\n",
		"test/unit/math.test.ts":      bunUnitTest,
		"test/integration/db.test.ts": bunIntegrationTest,
		"test/integration/preload.ts": "(globalThis as any).DB_READY = true;\n",
	})

	units, err := typescript.New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, u := range units.Units {
		ids = append(ids, u.ID)
	}
	for _, want := range []string{"typescript:bun-test:test/unit/math.test.ts", "typescript:bun-test:test/integration/db.test.ts"} {
		if !slices.Contains(ids, want) {
			t.Fatalf("affected units %q lack %s", ids, want)
		}
	}

	bin := t.TempDir()
	report := filepath.Join(bin, "report.xml")
	if err := os.WriteFile(report, []byte(bunJUnitReport), 0o644); err != nil {
		t.Fatal(err)
	}
	script := fmt.Sprintf("#!/bin/sh\nif [ \"$1\" = \"--version\" ]; then echo 1.3.11; exit 0; fi\nfor a in \"$@\"; do case \"$a\" in --reporter-outfile=*) out=\"${a#--reporter-outfile=}\";; esac; done\ncp %q \"$out\"\n", report)
	if err := os.WriteFile(filepath.Join(bin, "bun"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	receipt, err := jstestprovider.RunBun(context.Background(), jstestprovider.BunConfig{
		Config: jstestprovider.Config{Dir: root, Lockfile: filepath.Join(root, "bun.lock"), PackageJSON: filepath.Join(root, "package.json"),
			TestFiles: []string{filepath.Join(root, "test/unit/math.test.ts"), filepath.Join(root, "test/integration/db.test.ts")}},
		Executable: filepath.Join(bin, "bun"), Preload: []string{"./test/integration/preload.ts"}, MaxConcurrency: 1,
	})
	if err != nil || receipt.Infrastructure != nil {
		t.Fatalf("RunBun: %v %+v", err, receipt.Infrastructure)
	}

	joined := joinOne(t, root, ExampleJoinOptions{JS: &receipt})
	if joined.Language != "js" || joined.Revision != commit || len(joined.Examples) != 2 {
		t.Fatalf("join = %+v", joined)
	}
	// A receipt with no project keeps the empty project segment, as a Vitest receipt does.
	want := map[string]int{"test/unit/math.test.ts >  > math > adds": 5, "test/integration/db.test.ts >  > db ready": 3}
	for _, record := range joined.Examples {
		line, ok := want[record.Example]
		if !ok || record.Line != line || record.Outcome != "passed" || record.IdentityKind != ExampleIdentityJS {
			t.Fatalf("record = %+v", record)
		}
		if record.Verdict.Hygiene.State != testvalidity.HygieneEligible {
			t.Fatalf("%s verdict = %+v", record.Example, record.Verdict)
		}
	}
}
