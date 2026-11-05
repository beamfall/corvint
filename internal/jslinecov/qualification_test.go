// SPDX-License-Identifier: Apache-2.0
package jslinecov

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/linecov"
	"github.com/Beamfall/corvint/internal/linecov/v8"
	tr "github.com/Beamfall/corvint/internal/testrunner"
)

// LPCV-V0-7606 / LPCV-V0-963: each native runner executes a fault on a
// covered original-source line. The selector must retain every failing test,
// with an unrelated test as the negative control. Missing runtimes skip visibly.
func TestRunnerFaultInjection(t *testing.T) {
	for _, c := range []struct{ runner, bin, file, config string }{
		{"node-test", "node", "test/node.test.js", ""}, {"mocha", "mocha", "test/mocha.spec.js", ""}, {"jasmine", "jasmine", "test/jasmine.spec.js", ""}, {"jest", "jest", "test/jest.test.js", ""}, {"vitest", "vitest", "test/vitest.test.js", "vitest.config.mjs"}, {"ava", "ava", "test/ava.test.mjs", ""}, {"bun-test", "bun", "test/bun.test.js", ""}, {"deno-test", "deno", "test/deno.test.ts", ""},
	} {
		t.Run(c.runner, func(t *testing.T) {
			root := fixture(t)
			o := Options{Module: "m", Runner: c.runner, Executable: executable(t, root, c.bin), Files: []string{c.file}, Config: c.config, Work: t.TempDir(), Parallel: 1, Timeout: time.Minute, SuiteTimeout: time.Minute}
			out, err := Collect(context.Background(), root, o)
			if err != nil {
				t.Fatal(err)
			}
			var recs []linecov.Record
			for _, r := range out {
				if r.Record == nil {
					t.Fatalf("baseline divergent %+v", r)
				}
				recs = append(recs, *r.Record)
			}
			oids := map[string]string{}
			err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() {
					return nil
				}
				rel, _ := filepath.Rel(root, p)
				oids[filepath.ToSlash(rel)] = "blob"
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			ix, err := linecov.Build("m", "base", "runner", oids, recs)
			if err != nil {
				t.Fatal(err)
			}
			path := "lib.js"
			if c.runner == "deno-test" {
				path = "src/calc.ts"
			}
			src, err := os.ReadFile(filepath.Join(root, path))
			if err != nil {
				t.Fatal(err)
			}
			mutant := strings.Replace(string(src), "a + b", "a - b", 1)
			if c.runner == "deno-test" {
				mutant = strings.Replace(string(src), "n * n", "n + n", 1)
			}
			if mutant == string(src) {
				t.Fatal("fault not applied")
			}
			if err = os.WriteFile(filepath.Join(root, path), []byte(mutant), 0600); err != nil {
				t.Fatal(err)
			}
			e := Analyze(ix, []linecov.FileDiff{{Path: path, OldOID: "blob", NewOID: "fault", Hunks: []linecov.Hunk{{Start: 2, End: 2}}}}, func(string) ([]byte, error) { return src, nil })
			heads := map[string][]string{}
			for _, test := range ix.Tests {
				if test.Name != linecov.InOrder {
					heads[test.Package] = append(heads[test.Package], test.Name)
				}
			}
			plans, why := ix.Narrow([]string{c.file}, heads, linecov.Effect{}, e)
			if why != "" {
				t.Fatalf("unexpected fallback %s", why)
			}
			res, err := o.run(context.Background(), root, v8.RepoScope(root, nil), t.TempDir(), c.file, nil, time.Minute)
			if err != nil {
				t.Fatal(err)
			}
			failed, misses := 0, 0
			pp := plans[c.file]
			for _, test := range res.observation.Tests {
				if test.State != tr.Failed {
					continue
				}
				failed++
				name := fullName(c.runner, test)
				keep := pp.Mode == linecov.Whole
				for _, n := range pp.Tests {
					keep = keep || n == name
				}
				if !keep {
					misses++
				}
			}
			if failed == 0 || misses != 0 {
				t.Fatalf("fault failed=%d misses=%d plan=%+v observation=%+v", failed, misses, pp, res.observation)
			}
			t.Logf("runner=%s fault=%s:2 failing=%d misses=%d selected=%d total=%d", c.runner, path, failed, misses, len(pp.Tests), len(heads[c.file]))
		})
	}
}
