// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fixture is a temporary Git repository with one OpenSpec root, a REQUIREMENTS.tsv and one spec.
type fixture struct {
	t   *testing.T
	dir string
	n   int
}

const (
	fixtureDate = "2026-11-11T12:00:00Z"
	tsvHeader   = "id\tfile\ttitle\tacceptance\tacceptedBy\n"
)

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{t: t, dir: t.TempDir()}
	f.git("init", "-q")
	f.write("docs/specs/openspec/config.yaml", "schema: spec-driven\n")
	f.write("docs/specs/REQUIREMENTS.tsv", tsvHeader+
		"OLD-V1-001\tdocs/specs/base-v1.md\t(accepted under the owner's standing auto-accept (2026-10-11); V1-1) Old\tstanding\t2026-10-11\n")
	f.write("docs/specs/base-v1.md", "# Base\n\nIntent status: accepted\nDelivery status: experimental\nHealth: KEEP\n\n## Requirements\n")
	f.write("README.md", "fixture\n")
	f.commit("base")
	return f
}

func (f *fixture) git(args ...string) string {
	f.t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid",
		"-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main"}, args...)...)
	cmd.Dir = f.dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE="+fixtureDate, "GIT_COMMITTER_DATE="+fixtureDate)
	out, err := cmd.CombinedOutput()
	if err != nil {
		f.t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func (f *fixture) write(path, content string) {
	f.t.Helper()
	full := filepath.Join(f.dir, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) remove(path string) {
	f.t.Helper()
	if err := os.RemoveAll(filepath.Join(f.dir, path)); err != nil {
		f.t.Fatal(err)
	}
}

// commit stages everything and commits with message; it returns the commit name.
func (f *fixture) commit(message string) string {
	f.t.Helper()
	f.git("add", "-A")
	f.git("commit", "-q", "--allow-empty", "-m", message)
	return f.git("rev-parse", "HEAD")
}

func (f *fixture) check(args ...string) (int, string) {
	f.t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(append([]string{"--root", f.dir}, args...), &stdout, &stderr)
	return code, stdout.String() + stderr.String()
}

func (f *fixture) mustPass(args ...string) string {
	f.t.Helper()
	code, out := f.check(args...)
	if code != 0 {
		f.t.Fatalf("want exit 0, got %d:\n%s", code, out)
	}
	return out
}

func (f *fixture) mustFail(want int, args []string, fragments ...string) string {
	f.t.Helper()
	code, out := f.check(args...)
	if code != want {
		f.t.Fatalf("want exit %d, got %d:\n%s", want, code, out)
	}
	for _, fragment := range fragments {
		if !strings.Contains(out, fragment) {
			f.t.Fatalf("output lacks %q:\n%s", fragment, out)
		}
	}
	return out
}

func proposal(name, intent, delivery, owning string) string {
	return "# Change proposal: " + name + "\n\nChange name: `" + name + "`\nTicket: V1-9001\n" +
		"Owning capability spec: " + owning + "\nIntent status: " + intent + "\nDelivery status: " + delivery + "\n\n" +
		"## Why\n\nBecause.\n\n## What Changes\n\nThings.\n\n## Impact\n\nSome.\n\n## Non-goals\n\nNone.\n"
}

const design = "# Design\n\n## Decisions\n\nD.\n\n## Failure modes\n\nF.\n\n## Acceptance evidence\n\nA.\n\n## Rollback\n\nR.\n"

const delta = "# Spec delta\n\n## ADDED Requirements\n\n### Requirement: XYZ-V1-001 Thing\n\nThe tool MUST do it.\n\n" +
	"#### Scenario: Done\n\n- **WHEN** it runs\n- **THEN** it is done\n"

func tasks(ticks ...bool) string {
	text := "# Tasks\n\n## Tasks\n\n"
	for i, ticked := range ticks {
		box := " "
		if ticked {
			box = "x"
		}
		text += "- [" + box + "] 1." + string(rune('1'+i)) + " Step. Requirements: XYZ-V1-001. Proof: a Go test.\n"
	}
	return text
}

// addChange writes a valid change folder x with one ADDED requirement and two open tasks.
func (f *fixture) addChange(name string) {
	dir := changesDir + "/" + name + "/"
	f.write(dir+"proposal.md", proposal(name, "proposed", "not-started", "`docs/specs/x-v1.md`, a new spec"))
	f.write(dir+"design.md", design)
	f.write(dir+"tasks.md", tasks(false, false))
	f.write(dir+"specs/"+name+"/spec.md", delta)
}

func TestOSCV1001Layout(t *testing.T) {
	f := newFixture(t)
	f.addChange("x")
	f.commit("propose x")
	out := f.mustPass()
	if !strings.Contains(out, "openspec-check: PASS") || !strings.Contains(out, "NOT_RUN commit rules") {
		t.Fatalf("want PASS with the commit rules NOT_RUN:\n%s", out)
	}
	f.write(changesDir+"/x/.openspec.yaml", "schema: spec-driven\n")
	f.mustPass()

	f.remove(changesDir + "/x/design.md")
	f.mustFail(1, nil, "FAIL OSC-V1-001 change x: docs/specs/openspec/changes/x/design.md is missing")
	f.write(changesDir+"/x/design.md", design)

	f.write(changesDir+"/x/specs/x/spec.md", "## ADDED Requirements\n\n### Requirement: XYZ-V1-001 Thing\n\nIt MUST.\n")
	f.mustFail(1, nil, "ADDED requirement XYZ-V1-001 has no `#### Scenario:` block")
	f.write(changesDir+"/x/specs/x/spec.md", "## ADDED Requirements\n\n### Requirement: A title with no ID\n\nIt MUST.\n")
	f.mustFail(1, nil, "names no requirement ID")
	f.write(changesDir+"/x/specs/x/spec.md", delta)

	f.write(changesDir+"/x/tasks.md", "# Tasks\n\n- [ ] Write it. Requirements: XYZ-V1-001. Proof: test.\n")
	f.mustFail(1, nil, "tasks.md:3:", "is not `- [ ] N text` with a task number")
	f.write(changesDir+"/x/tasks.md", "# Tasks\n\n- [ ] 1 Write it. Proof: test.\n- [ ] 2 Ship it for V1-9001\n  (XYZ-V1-001).\n")
	out = f.mustFail(1, nil, "task 1 names no requirement ID", "task 2 names no `Proof:`")
	if strings.Contains(out, "task 2 names no requirement ID") {
		t.Fatalf("a continuation line's ID was not read:\n%s", out)
	}
	f.write(changesDir+"/x/tasks.md", "- [ ] 2.3a Close. Requirements: all. Proof: test.\n")
	f.mustPass()

	f.write(changesDir+"/x/proposal.md", "# P\n\nIntent status: proposed\n\n## Why\n\n## What changes\n\n## Impact\n")
	f.mustFail(1, nil, "proposal.md has no `## Non-goals` heading", "no `Delivery status:` header line")
}

func TestOSCV1002RequirementIDs(t *testing.T) {
	f := newFixture(t)
	f.write("docs/specs/REQUIREMENTS.tsv", tsvHeader+"XYZ-V1-001\tdocs/specs/other-v1.md\tTaken\tunrecorded\t-\n")
	f.commit("an existing requirement")
	f.addChange("x")
	f.commit("propose x")
	f.mustFail(1, nil, "FAIL OSC-V1-002 change x: ADDED XYZ-V1-001 already has a REQUIREMENTS.tsv row in docs/specs/other-v1.md")

	g := newFixture(t)
	g.addChange("x")
	g.commit("propose x")
	// The change's own apply-stage definition is allowed.
	g.write("docs/specs/REQUIREMENTS.tsv", tsvHeader+"OLD-V1-001\tdocs/specs/base-v1.md\t(retired) Old\tstanding\t-\n"+
		"XYZ-V1-001\tdocs/specs/x-v1.md\tThing\tstanding\t2026-10-11\n")
	g.commit("apply x")
	g.mustPass()
	g.addChange("y")
	g.mustFail(1, nil, "ADDED XYZ-V1-001 is also ADDED by docs/specs/openspec/changes/x")
	g.remove(changesDir + "/y")

	g.write(changesDir+"/x/specs/x/spec.md", delta+"\n## MODIFIED Requirements\n\n### Requirement: NOPE-V1-001 Gone\n\nIt MUST.\n\n#### Scenario: S\n\n- x\n")
	g.mustFail(1, nil, "MODIFIED NOPE-V1-001 has no REQUIREMENTS.tsv row")
	g.write(changesDir+"/x/specs/x/spec.md", delta+"\n## REMOVED Requirements\n\n### Requirement: OLD-V1-001 Old\n\n**Reason**: superseded.\n")
	g.mustPass()
}

func TestOSCV1003CommitBinding(t *testing.T) {
	f := newFixture(t)
	f.addChange("x")
	base := f.commit("propose x")
	f.write("main.go", "package main\n")
	bad := f.commit("code with no trailer")
	f.mustFail(1, []string{"--base", base}, "FAIL OSC-V1-003 commit "+bad[:12]+" changes main.go outside docs/ but binds to no change")
	f.mustPass() // without a range the commit rules are NOT_RUN

	for _, trailer := range []string{"Change: x", "Corvint-Ticket: V1-9001", "Requirement: OLD-V1-001"} {
		g := newFixture(t)
		g.addChange("x")
		base := g.commit("propose x")
		g.write("main.go", "package main\n")
		g.commit("code\n\n" + trailer)
		g.write("docs/notes.md", "docs only, no trailer\n")
		g.commit("docs")
		g.mustPass("--base", base)
	}

	h := newFixture(t)
	h.addChange("x")
	base = h.commit("propose x")
	h.write("main.go", "package main\n")
	h.commit("code\n\nRequirement: XYZ-V1-001\nChange: nope")
	h.mustFail(1, []string{"--base", base}, "`Requirement: XYZ-V1-001` names no requirement accepted", "`Change: nope` names no active or archived change folder")

	// A merge commit is exempt; the commits it brings in are checked.
	m := newFixture(t)
	m.addChange("x")
	base = m.commit("propose x")
	m.git("checkout", "-q", "-b", "side")
	m.write("side.go", "package main\n")
	m.commit("side\n\nChange: x")
	m.git("checkout", "-q", "-")
	m.write("docs/a.md", "a\n")
	m.commit("docs")
	m.git("-c", "core.editor=true", "merge", "-q", "--no-ff", "side", "-m", "merge side")
	m.mustPass("--base", base)
}

func TestOSCV1004ProposeStage(t *testing.T) {
	f := newFixture(t)
	base := f.git("rev-parse", "HEAD")
	f.addChange("x")
	f.write("main.go", "package main\n")
	together := f.commit("folder and code together\n\nChange: x")
	f.mustFail(1, []string{"--base", base}, "FAIL OSC-V1-004 commit "+together[:12],
		"its first parent holds no active docs/specs/openspec/changes/x/proposal.md: commit the proposal before the code")

	g := newFixture(t)
	base = g.git("rev-parse", "HEAD")
	g.addChange("x")
	g.commit("propose x")
	g.write("main.go", "package main\n")
	g.commit("code\n\nChange: x")
	g.mustPass("--base", base)
}

func TestOSCV1005ApplyStage(t *testing.T) {
	f := newFixture(t)
	f.addChange("x")
	f.write(changesDir+"/x/tasks.md", "- [ ] 2.1 A. Requirements: XYZ-V1-001. Proof: t.\n- [x] 2.2 B. Requirements: XYZ-V1-001. Proof: t.\n")
	f.commit("propose x")
	f.mustFail(1, nil, "FAIL OSC-V1-005 change x: tasks.md:2: task 2.2 is ticked after unticked task 2.1",
		"task 2.2 is ticked but ADDED XYZ-V1-001 has no definition in a canonical spec")

	f.write(changesDir+"/x/tasks.md", tasks(true, false))
	f.mustFail(1, nil, "task 1.1 is ticked but ADDED XYZ-V1-001 has no definition")
	f.write("docs/specs/REQUIREMENTS.tsv", tsvHeader+"XYZ-V1-001\tdocs/specs/x-v1.md\tThing\tstanding\t2026-10-11\n")
	f.mustPass()

	e := newFixture(t)
	e.addChange("t")
	e.write(changesDir+"/t/proposal.md", proposal("t", "proposed", "not-started",
		"`docs/specs/tasks-hardening-v1.md` in the\n`beamfall/corvint-tasks` repository"))
	e.write(changesDir+"/t/tasks.md", "- [x] 1.1 A. Requirements: XYZ-V1-001. Proof: t; defined at beamfall/corvint-tasks@0a1b2c3d.\n- [ ] 1.2 B. Requirements: XYZ-V1-001. Proof: t.\n")
	e.commit("external")
	out := e.mustPass()
	if !strings.Contains(out, "REPORT external-spec-not-verified change t: task 1.1: XYZ-V1-001 defined by beamfall/corvint-tasks commit 0a1b2c3d") {
		t.Fatalf("want the external report:\n%s", out)
	}
	e.write(changesDir+"/t/tasks.md", "- [x] 1.1 A. Requirements: XYZ-V1-001. Proof: t.\n")
	e.mustFail(1, nil, "task 1.1 is ticked for a spec in beamfall/corvint-tasks but names no commit there")
}

// archive moves change x to archive/DIR as openspec archive --skip-specs does.
func (f *fixture) archive(dir string) {
	f.t.Helper()
	if err := os.MkdirAll(filepath.Join(f.dir, changesDir, "archive"), 0o755); err != nil {
		f.t.Fatal(err)
	}
	f.git("mv", changesDir+"/x", changesDir+"/archive/"+dir)
}

// finishedChange is a fixture whose change x has every task ticked and its ID in REQUIREMENTS.tsv.
func finishedChange(t *testing.T) *fixture {
	f := newFixture(t)
	f.addChange("x")
	f.write(changesDir+"/x/tasks.md", tasks(true, true))
	f.write("docs/specs/REQUIREMENTS.tsv", tsvHeader+"XYZ-V1-001\tdocs/specs/x-v1.md\tThing\tstanding\t2026-10-11\n")
	f.commit("finished x")
	return f
}

func TestOSCV1006ArchiveStage(t *testing.T) {
	f := finishedChange(t)
	f.mustFail(1, nil, "FAIL OSC-V1-006 change x: every task is ticked", "`openspec archive x --skip-specs`",
		"docs/specs/openspec/changes/archive/")

	f.archive("2026-11-11-x")
	f.commit("archive x")
	f.mustPass()
	f.write(changesDir+"/archive/2026-11-11-x/tasks.md", tasks(true, true)+"- [x] 1.3 More. Requirements: XYZ-V1-001. Proof: t.\n")
	f.mustFail(1, nil, "the work tree edits archived folder")
	edit := f.commit("edit the archive")
	f.mustFail(1, nil, "commit "+edit[:12]+" edits archived folder docs/specs/openspec/changes/archive/2026-11-11-x")

	g := finishedChange(t)
	g.archive("2026-11-11-x")
	g.write(changesDir+"/archive/2026-11-11-x/design.md", design+"\nEdited while moving.\n")
	g.commit("archive and edit")
	g.mustFail(1, nil, "is not a pure rename of docs/specs/openspec/changes/x")

	h := finishedChange(t)
	h.archive("2026-10-01-x")
	h.commit("archive with a wrong date")
	h.mustFail(1, nil, "archive date 2026-10-01 is more than one day from archive commit")

	o := newFixture(t)
	o.addChange("x")
	o.write(changesDir+"/x/tasks.md", tasks(true, false))
	o.write("docs/specs/REQUIREMENTS.tsv", tsvHeader+"XYZ-V1-001\tdocs/specs/x-v1.md\tThing\tstanding\t2026-10-11\n")
	o.commit("x with an open task")
	o.archive("2026-11-11-x")
	o.commit("archive with --yes")
	o.mustFail(1, nil, "archived folder docs/specs/openspec/changes/archive/2026-11-11-x holds unticked task 1.2")

	d := newFixture(t)
	d.addChange("x")
	d.write(changesDir+"/archive/2026-11-11-x/tasks.md", tasks(true))
	d.write(changesDir+"/archive/Bad_Name/tasks.md", tasks(true))
	d.commit("duplicate and bad names")
	d.mustFail(1, nil, "the name is used by two folders", "archive/YYYY-MM-DD-<name>/")
}

func TestOSCV1007Boundaries(t *testing.T) {
	f := newFixture(t)
	f.write("openspec/changes/x/proposal.md", "# x\n")
	f.mustFail(1, nil, "FAIL OSC-V1-007 the repository root holds an openspec/ directory", "docs/specs/openspec/changes/")
	f.remove("openspec")
	f.write(".openspec.yaml", "schema: spec-driven\n")
	f.mustFail(1, nil, "the repository root holds .openspec.yaml")
	f.remove(".openspec.yaml")
	f.write("docs/specs/openspec/specs/watch/spec.md", "# watch\n")
	f.mustFail(1, nil, "docs/specs/openspec/specs/ exists: the canonical spec belongs directly under docs/specs/")
	f.remove("docs/specs/openspec/specs")
	f.write("interop/spec-toolchains/openspec/specs/x/spec.md", "other repositories\n")
	f.mustPass()
	f.write("docs/specs/openspec/config.yaml", "schema: other\n")
	f.mustFail(1, nil, "names schema \"other\", want `schema: spec-driven`")
	f.remove("docs/specs/openspec/config.yaml")
	f.mustFail(1, nil, "docs/specs/openspec/config.yaml is missing")
}

func TestOSCV1008HealthClass(t *testing.T) {
	f := newFixture(t)
	f.write("docs/specs/new-v1.md", "# New\n\nDelivery status: experimental\n\n## Requirements\n\nHealth: KEEP\n")
	f.mustFail(1, nil, "FAIL OSC-V1-008 spec docs/specs/new-v1.md: its header has no `Health:` line")
	f.write("docs/specs/new-v1.md", "# New\n\nHealth: ALIVE\n")
	f.mustFail(1, nil, "`Health: ALIVE` is not one of KEEP, MERGE, RETIRE or OUT-OF-1.0")
	f.write("docs/specs/new-v1.md", "# New\n\nDelivery status: experimental\nHealth: OUT-OF-1.0 (kept command)\n")
	f.mustFail(1, nil, "`Health: OUT-OF-1.0` needs `Delivery status: retired`")
	f.write("docs/specs/new-v1.md", "# New\n\nDelivery status: retired\nHealth: RETIRE\n")
	f.write("docs/specs/README.md", "# Specs\n\nNo Health line needed here.\n")
	f.mustPass()
}

func TestOSCV1009AcceptanceLink(t *testing.T) {
	f := newFixture(t)
	f.addChange("x")
	f.write(changesDir+"/x/proposal.md", proposal("x", "accepted (self-declared)", "experimental", "`docs/specs/x-v1.md`"))
	f.commit("x")
	f.mustFail(1, nil, "FAIL OSC-V1-009 change x: proposal.md says the intent is accepted but "+acceptanceLog+" has no `change` row for x")
	f.write(acceptanceLog, "id\tkind\tmode\tdate\townerRef\trationale\nx\tchange\tpending-owner\t2026-11-11\tr\tr\n")
	f.mustFail(1, nil, "has no `change` row for x with mode owner or auto")
	f.write(acceptanceLog, "id\tkind\tmode\tdate\townerRef\trationale\nx\tchange\tauto\t2026-11-11\tstanding auto-accept\tr\n")
	f.mustPass()
	f.remove(acceptanceLog)
	f.write(changesDir+"/x/proposal.md", proposal("x", "proposed", "implemented", "`docs/specs/x-v1.md`"))
	f.mustFail(1, nil, "with no acceptance log row the delivery must read not-started or experimental, got \"implemented\"")
}

func TestOSCV1011Baseline(t *testing.T) {
	f := newFixture(t)
	f.addChange("x")
	f.remove(changesDir + "/x/design.md")
	f.write(defaultBaseline, "# Report-only subjects.\nchange x\n")
	base := f.commit("x without design, baselined")
	out := f.mustPass("--base", base)
	if !strings.Contains(out, "BASELINE OSC-V1-001 change x: docs/specs/openspec/changes/x/design.md is missing") {
		t.Fatalf("want the baselined finding listed:\n%s", out)
	}
	f.write(defaultBaseline, "change x\nspec docs/specs/base-v1.md\n")
	f.commit("grow the baseline")
	f.mustFail(1, []string{"--base", base}, "baseline entry \"spec docs/specs/base-v1.md\" was added since",
		"baseline entry \"spec docs/specs/base-v1.md\" no longer fails")
	f.write(defaultBaseline, "change x\nchange Not_A_Name\n")
	f.mustFail(1, nil, "is not `change <name>` or `spec docs/specs/<file>.md`")
	f.write(changesDir+"/x/design.md", design)
	f.write(defaultBaseline, "change x\n")
	f.mustFail(1, nil, "baseline entry \"change x\" no longer fails: delete its line")
	f.write(defaultBaseline, "")
	f.mustPass()
	// A subject with no baseline entry still fails, and boundary findings are never baselined.
	f.write(defaultBaseline, "change x\n")
	f.remove("docs/specs/openspec/config.yaml")
	f.remove(changesDir + "/x/design.md")
	f.mustFail(1, nil, "FAIL OSC-V1-007 docs/specs/openspec/config.yaml is missing")
}

func TestOSCV1011HistoryUnavailable(t *testing.T) {
	f := newFixture(t)
	f.addChange("x")
	first := f.commit("propose x")
	for i := 0; i < 10; i++ {
		f.write("docs/log.md", strings.Repeat("x", i+1)+"\n")
		f.commit("step")
	}
	clone := t.TempDir()
	f.git("clone", "-q", "--depth", "1", "file://"+f.dir, clone)
	shallow := &fixture{t: t, dir: clone}
	shallow.mustFail(2, []string{"--base", first}, "REFUSE history-unavailable")
	shallow.mustFail(2, nil, "REFUSE history-unavailable", "the clone is shallow")

	f.mustFail(2, []string{"--base", "0000000000000000000000000000000000000001"}, "REFUSE history-unavailable", "is not a commit")
	bare := &fixture{t: t, dir: t.TempDir()}
	bare.mustFail(2, nil, "REFUSE history-unavailable", "is not a Git work tree")
	code, _ := f.check("--base")
	if code != 2 {
		t.Fatalf("a missing flag value: want exit 2, got %d", code)
	}
}

func TestOSCV1011StaleRequirements(t *testing.T) {
	f := newFixture(t)
	f.write(generator, "#!/bin/sh\nprintf 'id\\tfile\\ttitle\\tacceptance\\tacceptedBy\\n'\n")
	f.commit("generator")
	f.mustFail(2, nil, "REFUSE requirements-stale", "make spec-requirements")
	f.write("docs/specs/REQUIREMENTS.tsv", tsvHeader)
	out := f.mustPass()
	if strings.Contains(out, "NOT_RUN requirements-stale") {
		t.Fatalf("the staleness check did not run:\n%s", out)
	}
	f.remove(generator)
	if out := f.mustPass(); !strings.Contains(out, "NOT_RUN requirements-stale check") {
		t.Fatalf("want the staleness check reported NOT_RUN:\n%s", out)
	}
}

// TestOSCV1001TaskIDForms covers the ways a task names requirement IDs: plain IDs, `X-NNN to NNN`,
// the reservation form `PREFIX` NNN..NNN, and `Requirements: all`; ticket IDs are not IDs.
func TestOSCV1001TaskIDForms(t *testing.T) {
	for _, tc := range []struct {
		text string
		want string
		all  bool
	}{
		{"Requirements: OSC-V1-001, PROD-INT-011. Proof: t.", "OSC-V1-001 PROD-INT-011", false},
		{"Requirements: OSC-V1-001 to 003. Proof: t.", "OSC-V1-001 OSC-V1-002 OSC-V1-003", false},
		{"Requirements: OSC-V1-009 to OSC-V1-010.", "OSC-V1-009 OSC-V1-010", false},
		{"Reserve `RNC-V1` 001..003; set requirementRefs.", "RNC-V1-001 RNC-V1-002 RNC-V1-003", false},
		{"reserve prefix `TKF-V1` (001 to 002)", "TKF-V1-001 TKF-V1-002", false},
		{"reserve `GHA-V1` 001-002, 020-021", "GHA-V1-001 GHA-V1-002", false},
		{"File ticket V1-5399. Requirements: all. Proof: t.", "", true},
		{"Build-log entry; Tasks gate. Proof: receipt audit.", "", false},
	} {
		ids, all := namedIDs(tc.text)
		if got := strings.Join(ids, " "); got != tc.want || all != tc.all {
			t.Errorf("namedIDs(%q) = %q, %v; want %q, %v", tc.text, got, all, tc.want, tc.all)
		}
	}
}
