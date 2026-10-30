package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/localcompletion"
)

// OCM-V0-022 (V1-1388): the installed binary's `dogfood finish` keeps a bound
// aggregate refusal on its stdout evidence. Each case builds per-scope maps that
// ocm prepare and ocm link accept on their own, so only the aggregate across the
// scopes exceeds a bound; the policy.evidence dogfood-ocm-status row names the
// code and the bounded detail, and stderr carries only the bare code. The
// obligation and claim bounds are 16 maps' worth (V1-2971), which accepted
// maps cannot exceed, so only the summed byte bound is reachable here; the
// other two are pinned by TestOCMV0021BoundRefusalsNameTotalsBoundAndScopes.
func TestOCMV0022DogfoodFinishKeepsAggregateBoundRefusal(t *testing.T) {
	t.Parallel()
	run := portableDogfoodRunner(t)
	run.env = append(run.env, "DOGFOOD_OUTCOME=failed", "DOGFOOD_VERIFY=false", "DOGFOOD_CITATIONS="+filepath.Join(t.TempDir(), "missing.tsv"), "DOGFOOD_INTENTS_FILE="+filepath.Join(t.TempDir(), "missing"))
	cases := []struct {
		code         string
		scopes       int
		requirements int // per scope
		claims       int // per scope, linked 64 per obligation
		detail       string
	}{
		// Five scopes of 470 claims each exceed the summed 1 MiB map bytes.
		{"map-too-large", 5, 8, 470, "aggregate OCM map bytes exceed the limit: observed "},
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			t.Parallel()
			root, key := aggregateFinishRepo(t, run, tc.scopes, tc.requirements, tc.claims)
			code, stdout, stderr := run.exec(t, root, nil, "dogfood", "finish", "--session-key", key)
			if code != 2 {
				t.Fatalf("finish exit=%d stderr=%s", code, stderr)
			}
			var refused struct {
				Code  string `json:"code"`
				Error struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal([]byte(stderr), &refused); err != nil || refused.Code != tc.code || refused.Error.Code != tc.code || refused.Error.Message != tc.code {
				t.Fatalf("stderr is not the bare %s code: %v %s", tc.code, err, stderr)
			}
			if strings.Contains(stderr, "observed") || strings.Contains(stderr, "per scope") {
				t.Fatalf("stderr carries the detail: %s", stderr)
			}
			var finished struct {
				Policy struct {
					Unmet    []string `json:"unmet"`
					Evidence []struct {
						Tool   string `json:"tool"`
						Map    string `json:"map"`
						Code   string `json:"code"`
						Detail string `json:"detail"`
					} `json:"evidence"`
				} `json:"policy"`
			}
			if err := json.Unmarshal([]byte(stdout), &finished); err != nil {
				t.Fatalf("stdout: %v %.400s", err, stdout)
			}
			var rows int
			for _, item := range finished.Policy.Evidence {
				if item.Tool != "dogfood-ocm-status" {
					continue
				}
				rows++
				if item.Map != ".corvint/change.ocm-intents" || item.Code != tc.code || len(item.Detail) > 512 || !strings.HasPrefix(item.Detail, tc.detail) {
					t.Fatalf("aggregate row %+v, want code %s and detail %q", item, tc.code, tc.detail)
				}
				var shares []string
				for scope := 1; scope <= tc.scopes; scope++ {
					shares = append(shares, fmt.Sprintf("%d=", scope))
				}
				if !strings.Contains(item.Detail, ", bound ") || !strings.Contains(item.Detail, "; per scope "+shares[0]) || !strings.Contains(item.Detail, " "+shares[len(shares)-1]) {
					t.Fatalf("detail lacks the total, bound or per-scope counts: %q", item.Detail)
				}
			}
			if rows != 1 {
				t.Fatalf("stdout has %d dogfood-ocm-status rows: %.400s", rows, stdout)
			}
		})
	}
}

// aggregateFinishRepo returns an enrolled, verified repository whose plan names
// one intent per scope. Each intent declares requirements FX<s>-REQ-001..; when
// claims is positive, each scope's own test file offers that many cases and
// ocm link binds them 64 per obligation, the per-link limit.
func aggregateFinishRepo(t *testing.T, run portableRun, scopes, requirements, claims int) (string, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cemWrite(t, root, "go.mod", "module example.com/portable\n\ngo 1.22\n")
	cemWrite(t, root, "AGENTS.md", "# Fixture authority\nUse the intents to govern the fixture.\n")
	cemWrite(t, root, "fixture/fixture.go", "package fixture\nfunc Answer() int { return 1 }\n")
	cemWrite(t, root, "fixture/fixture_test.go", "package fixture\nimport \"testing\"\nfunc TestAnswer(t *testing.T) { if Answer() != 2 { t.Fatal(\"answer\") } }\n")
	intents := make([]string, 0, scopes)
	const perLink = 64
	for scope := 1; scope <= scopes; scope++ {
		var intent strings.Builder
		fmt.Fprintf(&intent, "# Intent %d\n\n## Requirements\n\n", scope)
		for requirement := 1; requirement <= requirements; requirement++ {
			fmt.Fprintf(&intent, "- `FX%d-REQ-%03d`: Requirement %d of scope %d holds.\n", scope, requirement, requirement, scope)
		}
		name := fmt.Sprintf("intent%d.md", scope)
		cemWrite(t, root, name, intent.String())
		intents = append(intents, name)
		if claims > 0 {
			// One file per scope: a test blob offers at most 512 claims.
			var cases strings.Builder
			fmt.Fprintf(&cases, "package fixture\nimport \"testing\"\nfunc TestScope%d(t *testing.T) {\n", scope)
			for claim := 1; claim <= claims; claim++ {
				fmt.Fprintf(&cases, "\tt.Run(\"FX%d-REQ-%03d c%03d\", func(t *testing.T) {})\n", scope, (claim-1)/perLink+1, claim)
			}
			cases.WriteString("}\n")
			cemWrite(t, root, fmt.Sprintf("fixture/s%d_test.go", scope), cases.String())
		}
	}
	cemGit(t, root, "init", "-q", "-b", "main")
	exclude := ".corvint/dogfood-report.json\n.corvint/change.ocm-intents\n.corvint/change.ocm-status.json\n.corvint/change.ocm.*.json\n.corvint/self-observations.jsonl\n.context-corvint/\n"
	if err = os.WriteFile(filepath.Join(root, ".git/info/exclude"), []byte(exclude), 0644); err != nil {
		t.Fatal(err)
	}
	cemGit(t, root, "add", ".")
	cemGit(t, root, "commit", "-qm", "base")
	base := cemGit(t, root, "rev-parse", "HEAD")
	cemWrite(t, root, "fixture/fixture.go", "package fixture\nfunc Answer() int { return 2 }\n")
	cemGit(t, root, "commit", "-qam", "change")
	key := localcompletion.HashSession(t.Name())
	plan, _ := json.Marshal(localcompletion.Plan{Base: base, Intents: intents, Checks: []localcompletion.Check{{ID: "actual-test", Argv: []string{"go", "test", "./fixture"}, TimeoutSeconds: 60}}})
	planPath := filepath.Join(t.TempDir(), "plan.json")
	if err = os.WriteFile(planPath, plan, 0600); err != nil {
		t.Fatal(err)
	}
	run.ok(t, root, "dogfood", "begin", "--plan", planPath, "--session-key", key)
	run.ok(t, root, "cem", "prepare", "--base", base, "--target", "HEAD")
	run.ok(t, root, "cem", "cite", "--map", ".corvint/change.cem.json", "--hunk", "1", "--evidence-path", "intent1.md", "--lines", "1:3", "--relation", "specification")
	cemGit(t, root, "add", ".corvint/change.cem.json")
	cemGit(t, root, "commit", "-qm", "bind evidence")
	target := cemGit(t, root, "rev-parse", "HEAD")
	run.ok(t, root, "dogfood", "verify", "--check", "actual-test", "--session-key", key)
	for scope := 1; scope <= scopes && claims > 0; scope++ {
		mapPath := fmt.Sprintf(".corvint/change.ocm.%03d.json", scope)
		run.ok(t, root, "ocm", "prepare", "--map", mapPath, "--cem", ".corvint/change.cem.json", "--intent", intents[scope-1], "--expected-base", base, "--target", target)
		for first := 1; first <= claims; first += perLink {
			args := []string{"ocm", "link", "--map", mapPath, "--cem", ".corvint/change.cem.json", "--obligation", fmt.Sprintf("FX%d-REQ-%03d", scope, (first-1)/perLink+1), "--hunk", "1", "--test-path", fmt.Sprintf("fixture/s%d_test.go", scope), "--expected-base", base, "--target", target}
			for claim := first; claim < first+perLink && claim <= claims; claim++ {
				args = append(args, "--claim", fmt.Sprintf("test:TestScope%d/case:fx%d-req-c%03d", scope, scope, claim))
			}
			run.ok(t, root, args...)
		}
	}
	return root, key
}

// ocmScopeRepo returns a repository whose committed CEM is bound and whose
// manifest names one intent per count, each with its OCM map prepared at the
// target and declaring that many requirements. When unlisted is positive, the
// base also holds intent-unlisted.md with that many requirements, which the
// manifest omits and no map prepares.
func ocmScopeRepo(t *testing.T, run portableRun, counts []int, unlisted int) (root, base, target string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cemWrite(t, root, "fixture/fixture.go", "package fixture\nfunc Answer() int { return 1 }\n")
	var manifest strings.Builder
	for scope, count := range counts {
		var intent strings.Builder
		fmt.Fprintf(&intent, "# Intent %d\n\n## Requirements\n\n", scope+1)
		for requirement := 1; requirement <= count; requirement++ {
			fmt.Fprintf(&intent, "- `FX%d-REQ-%03d`: Requirement %d of scope %d holds.\n", scope+1, requirement, requirement, scope+1)
		}
		name := fmt.Sprintf("intent%02d.md", scope+1)
		cemWrite(t, root, name, intent.String())
		manifest.WriteString(name + "\n")
	}
	if unlisted > 0 {
		var intent strings.Builder
		intent.WriteString("# Unlisted intent\n\n## Requirements\n\n")
		for requirement := 1; requirement <= unlisted; requirement++ {
			fmt.Fprintf(&intent, "- `FXU-REQ-%03d`: Requirement %d holds.\n", requirement, requirement)
		}
		cemWrite(t, root, "intent-unlisted.md", intent.String())
	}
	cemGit(t, root, "init", "-q", "-b", "main")
	if err = os.WriteFile(filepath.Join(root, ".git/info/exclude"), []byte(".corvint/change.ocm-intents\n.corvint/change.ocm.*.json\n.corvint/self-observations.jsonl\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cemGit(t, root, "add", ".")
	cemGit(t, root, "commit", "-qm", "base")
	base = cemGit(t, root, "rev-parse", "HEAD")
	cemWrite(t, root, "fixture/fixture.go", "package fixture\nfunc Answer() int { return 2 }\n")
	cemGit(t, root, "commit", "-qam", "change")
	run.ok(t, root, "cem", "prepare", "--base", base, "--target", "HEAD")
	run.ok(t, root, "cem", "cite", "--map", ".corvint/change.cem.json", "--hunk", "1", "--evidence-path", "intent01.md", "--lines", "1:3", "--relation", "specification")
	cemGit(t, root, "add", ".corvint/change.cem.json")
	cemGit(t, root, "commit", "-qm", "bind evidence")
	target = cemGit(t, root, "rev-parse", "HEAD")
	cemWrite(t, root, ".corvint/change.ocm-intents", manifest.String())
	for scope := range counts {
		run.ok(t, root, "ocm", "prepare", "--map", fmt.Sprintf(".corvint/change.ocm.%03d.json", scope+1), "--cem", ".corvint/change.cem.json", "--intent", fmt.Sprintf("intent%02d.md", scope+1), "--expected-base", base, "--target", target)
	}
	return root, base, target
}

// V1-2971 (OCM-V0-013, OCM-V0-021): the installed binary binds three scopes
// whose obligations sum past one map's 256-obligation bound, because the
// aggregate bound is derived from the scope count; a single intent over the
// per-map bound still refuses at its own map.
func TestOCMV0021AggregateBoundIsDerivedFromScopeCount(t *testing.T) {
	t.Parallel()
	run := portableDogfoodRunner(t)
	root, base, target := ocmScopeRepo(t, run, []int{151, 105, 15}, 257)
	stdout := run.ok(t, root, "dogfood-ocm", "status", "--expected-base", base, "--target", target)
	var status struct {
		OK        bool `json:"ok"`
		Aggregate struct {
			Coverage struct {
				Total int `json:"total"`
			} `json:"coverage"`
		} `json:"aggregate"`
	}
	if err := json.Unmarshal([]byte(stdout), &status); err != nil || !status.OK || status.Aggregate.Coverage.Total != 271 {
		t.Fatalf("271 obligations over three scopes did not bind: %v %.400s", err, stdout)
	}

	code, _, stderr := run.exec(t, root, nil, "ocm", "prepare", "--map", ".corvint/change.ocm.004.json", "--cem", ".corvint/change.cem.json", "--intent", "intent-unlisted.md", "--expected-base", base, "--target", target)
	if code == 0 || !strings.Contains(stderr, "too-many-obligations") {
		t.Fatalf("a 257-obligation intent did not refuse at its map: exit=%d stderr=%s", code, stderr)
	}
}
