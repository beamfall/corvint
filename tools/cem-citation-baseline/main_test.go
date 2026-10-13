package main

import (
	"bytes"
	"context"
	"encoding/json"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cli"
	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

func git(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func write(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// bindMap prepares a cem/1.0 map of base..target, cites every hunk ordinal to
// its own span of docs/spec.md, marks role when it is not empty (DCW-V0-049)
// and keeps the map as .corvint/changes/<target>.cem.json.
func bindMap(t *testing.T, root, base, target, role string, hunks int) {
	t.Helper()
	cem := func(args ...string) {
		t.Helper()
		var out, errb bytes.Buffer
		if exit := cli.Run(context.Background(), root, args, &out, &errb); exit != 0 {
			t.Fatalf("cem %v: exit %d\n%s%s", args, exit, out.String(), errb.String())
		}
	}
	// The baseline reads cem/1.0 maps only (CEM-V1-210).
	cem("prepare", "--spec", wire.StableSpec, "--base", base, "--target", target)
	for ordinal := 1; ordinal <= hunks; ordinal++ {
		cem("cite", "--map", ".corvint/change.cem.json", "--hunk", strconv.Itoa(ordinal), "--evidence-path", "docs/spec.md", "--lines", "1:"+strconv.Itoa(ordinal), "--relation", "specification")
	}
	if role != "" {
		mapPath := filepath.Join(root, ".corvint", "change.cem.json")
		data, err := os.ReadFile(mapPath)
		if err != nil {
			t.Fatal(err)
		}
		document, err := wire.ParseStable(data)
		if err != nil {
			t.Fatal(err)
		}
		document.Role = role
		if data, err = wire.EncodeStable(document); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(mapPath, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Rename(filepath.Join(root, ".corvint", "change.cem.json"), filepath.Join(root, ".corvint", "changes", target+".cem.json")); err != nil {
		t.Fatal(err)
	}
}

// DCW-V0-049: the baseline tool reproduces the V1-1262 row and summary shape
// over a directory of maps and splits it by citation class: a bind-chore map
// is class chore, any other change is class change, and a map whose status
// carries no citation block is an error row. CEM-V1-210: a retired cem/0.x map
// is an unclassified error row carrying the unsupported-spec refusal and its
// fix line, and is never read.
func TestBaselineDCWV0049SplitsRowsByCitationClass(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	git(t, root, "init", "-q", "-b", "main")
	write(t, root, "docs/spec.md", "one\ntwo\nthree\n")
	write(t, root, "a.txt", "a\n")
	write(t, root, "b.txt", "b\n")
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "base")
	base := git(t, root, "rev-parse", "HEAD")
	write(t, root, "a.txt", "a\nmore a\n")
	write(t, root, "b.txt", "b\nmore b\n")
	git(t, root, "commit", "-qam", "change")
	change := git(t, root, "rev-parse", "HEAD")
	if err := os.MkdirAll(filepath.Join(root, ".corvint", "changes"), 0o755); err != nil {
		t.Fatal(err)
	}
	bindMap(t, root, base, change, "", 2)
	git(t, root, "add", ".corvint/changes")
	git(t, root, "commit", "-qm", "chore: bind the change map")
	chore := git(t, root, "rev-parse", "HEAD")
	bindMap(t, root, change, chore, wire.StableRoleBindChore, 1)
	git(t, root, "add", ".corvint/changes")
	git(t, root, "commit", "-qm", "chore: bind the chore map")
	choreBound := git(t, root, "rev-parse", "HEAD")
	write(t, root, "c.txt", "c\n")
	git(t, root, "add", "c.txt")
	git(t, root, "commit", "-qm", "stable change")
	stable := git(t, root, "rev-parse", "HEAD")
	// A cem/1.0 map reads its class from the status envelope, whose stable
	// verifier needs the artifact root the tool passes.
	bindMap(t, root, choreBound, stable, "", 1)
	broken := strings.Repeat("0", 40)
	write(t, root, ".corvint/changes/"+broken+".cem.json", `{"spec":"cem/0.2","baseRevision":"`+base+`"}`)

	summaryPath := filepath.Join(t.TempDir(), "summary.json")
	var stdout, stderr bytes.Buffer
	if exit := run(context.Background(), []string{"--root", root, "--summary", summaryPath}, &stdout, &stderr); exit != 0 {
		t.Fatalf("exit %d\n%s", exit, stderr.String())
	}
	rows := map[string]map[string]any{}
	for _, line := range strings.Split(strings.TrimSpace(stdout.String()), "\n") {
		var row map[string]any
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			t.Fatalf("row %q: %v", line, err)
		}
		rows[row["map"].(string)] = row
	}
	if len(rows) != 4 {
		t.Fatalf("rows = %d; want 4\n%s", len(rows), stdout.String())
	}
	if row := rows[stable]; row["spec"] != "cem/1.0" || row["class"] != "change" || row["valid"] != true || row["hunksTotal"] != float64(1) {
		t.Errorf("stable row = %v", row)
	}
	if row := rows[change]; row["class"] != "change" || row["valid"] != true || row["hunksTotal"] != float64(2) || row["distinctEvidence"] != float64(2) || len(row["armsIssues"].([]any)) != 0 || row["supportedHunks"] != float64(2) {
		t.Errorf("change row = %v", row)
	}
	if row := rows[chore]; row["class"] != "chore" || row["valid"] != true || row["hunksTotal"] != float64(1) {
		t.Errorf("chore row = %v", row)
	}
	if row := rows[broken]; row["valid"] != false || row["hunksTotal"] != float64(0) || row["class"] != "" || len(row["issues"].([]any)) != 1 {
		t.Errorf("error row = %v", row)
	} else if refusal := row["issues"].([]any)[0].(map[string]any); refusal["code"] != "unsupported-spec" ||
		!strings.HasSuffix(refusal["message"].(string), "fix: "+wire.RetiredSpecFix) {
		t.Errorf("retired map refusal = %v", refusal)
	}
	if !strings.Contains(stderr.String(), "ERR "+broken+" ") {
		t.Errorf("stderr lacks the ERR line:\n%s", stderr.String())
	}
	data, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatal(err)
	}
	var summary map[string]map[string]any
	if err := json.Unmarshal(data, &summary); err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct {
		class, key string
		want       float64
	}{
		{"all", "maps", 4}, {"all", "withCitation", 3}, {"all", "invalidOrError", 1}, {"all", "hunks", 4},
		{"change", "maps", 2}, {"change", "hunks", 3}, {"chore", "maps", 1}, {"chore", "hunks", 1},
		{"unclassified", "maps", 1}, {"unclassified", "withCitation", 0},
	} {
		if got := summary[check.class][check.key]; got != check.want {
			t.Errorf("summary %s.%s = %v; want %v", check.class, check.key, got, check.want)
		}
	}
}

// DCW-V0-059, CEM-V1-170: a map whose hunks change only evidence records is
// a record row, summarized apart from change, and a change touching both a
// build-log entry and code stays class change.
func TestBaselineDCWV0059SplitsRecordRows(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	git(t, root, "init", "-q", "-b", "main")
	write(t, root, "docs/spec.md", "one\ntwo\nthree\n")
	write(t, root, "a.txt", "a\n")
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "base")
	base := git(t, root, "rev-parse", "HEAD")
	write(t, root, "docs/build-log/2026-10-13-x.md", "entry\n")
	write(t, root, ".agent-evidence/x/run.txt", "run\n")
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "record")
	record := git(t, root, "rev-parse", "HEAD")
	if err := os.MkdirAll(filepath.Join(root, ".corvint", "changes"), 0o755); err != nil {
		t.Fatal(err)
	}
	bindMap(t, root, base, record, "", 2)
	write(t, root, "docs/build-log/2026-10-13-y.md", "entry\n")
	write(t, root, "a.txt", "a\nmore a\n")
	git(t, root, "add", "a.txt", "docs/build-log")
	git(t, root, "commit", "-qm", "mixed")
	mixed := git(t, root, "rev-parse", "HEAD")
	bindMap(t, root, record, mixed, "", 2)

	summaryPath := filepath.Join(t.TempDir(), "summary.json")
	var stdout, stderr bytes.Buffer
	if exit := run(context.Background(), []string{"--root", root, "--summary", summaryPath}, &stdout, &stderr); exit != 0 {
		t.Fatalf("exit %d\n%s", exit, stderr.String())
	}
	rows := map[string]map[string]any{}
	for _, line := range strings.Split(strings.TrimSpace(stdout.String()), "\n") {
		var row map[string]any
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			t.Fatalf("row %q: %v", line, err)
		}
		rows[row["map"].(string)] = row
	}
	if row := rows[record]; row["class"] != "record" || row["valid"] != true || row["hunksTotal"] != float64(2) {
		t.Errorf("record row = %v", row)
	}
	if row := rows[mixed]; row["class"] != "change" || row["valid"] != true {
		t.Errorf("mixed row = %v", row)
	}
	data, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatal(err)
	}
	var summary map[string]map[string]any
	if err := json.Unmarshal(data, &summary); err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct {
		class, key string
		want       float64
	}{
		{"all", "maps", 2}, {"change", "maps", 1}, {"chore", "maps", 0}, {"record", "maps", 1}, {"record", "hunks", 2},
	} {
		if got := summary[check.class][check.key]; got != check.want {
			t.Errorf("summary %s.%s = %v; want %v", check.class, check.key, got, check.want)
		}
	}
}

// The rows keep the baseline's Python json.dumps encoding, so a row for an
// unchanged map compares byte for byte apart from its class member.
func TestBaselineRowKeepsThePythonEncoding(t *testing.T) {
	r := row{stem: "02ed58f3402291aa678b6cc1741134ef7bd482ba", spec: "cem/1.0", class: "change", valid: true, citation: map[string]any{
		"hunks": []any{1, 2, 3, 4}, "annotationHunks": float64(0), "concentratedHunks": float64(4), "distinctEvidence": float64(2),
		"hunksByClass": map[string]any{"anchored": float64(0), "cited": float64(4), "held": float64(0), "mechanical-proved": float64(0), "unknown": float64(0)},
		"invariant":    false, "maxFanOut": float64(2), "specificity": float64(0), "samePathCitations": float64(0),
	}}
	want := `{"annotationHunks": 0, "arms": null, "armsIssues": [], "class": "change", "concentratedHunks": 4, "diffAdjacentBases": null, "diffAdjacentHunks": null, "distinctEvidence": 2, "hunksByClass": {"anchored": 0, "cited": 4, "held": 0, "mechanical-proved": 0, "unknown": 0}, "hunksTotal": 4, "invariant": false, "map": "02ed58f3402291aa678b6cc1741134ef7bd482ba", "maxFanOut": 2, "samePathBases": null, "samePathCitations": 0, "samePathHunks": null, "spec": "cem/1.0", "specificity": 0, "supportedHunks": null, "valid": true}`
	if got := pyJSON(r.fields(), -1, 0); got != want {
		t.Errorf("row\n got %s\nwant %s", got, want)
	}
	if got := pyJSON(ordered{{"maps", 1}, {"tiers", ordered{{"cited", 4}}}}, 1, 0); got != "{\n \"maps\": 1,\n \"tiers\": {\n  \"cited\": 4\n }\n}" {
		t.Errorf("summary encoding = %q", got)
	}
}

// fixtureUnits is the synthetic repository's unit count (the panel's
// adversary build_repo.py, ported).
const fixtureUnits = 24

// fixtureRepo builds the base tree: docs/spec.md defines UNI-V0-1NN on line
// 2+NN, docs/paths.md names src/unitNN.go on line 2+NN, and src/unitNN.go is
// a five-line function whose return line (line 4) every fixture rewrites.
func fixtureRepo(t *testing.T) (string, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	git(t, root, "init", "-q", "-b", "main")
	spec, paths := "# Requirements\n\n", "# Ownership\n\n"
	for unit := 1; unit <= fixtureUnits; unit++ {
		spec += "- `" + unitID(unit) + "`: unit " + unitNumber(unit) + " MUST return its input unchanged.\n"
		paths += "The handler in " + unitPath(unit) + " owns step " + strconv.Itoa(unit) + ".\n"
		write(t, root, unitPath(unit), unitSource(unit, "value + 1"))
	}
	write(t, root, "docs/spec.md", spec)
	write(t, root, "docs/paths.md", paths)
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "base")
	if err := os.MkdirAll(filepath.Join(root, ".corvint", "changes"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root, git(t, root, "rev-parse", "HEAD")
}

func unitNumber(unit int) string { return strconv.Itoa(100 + unit)[1:] }
func unitID(unit int) string     { return "UNI-V0-1" + unitNumber(unit) }
func unitPath(unit int) string   { return "src/unit" + unitNumber(unit) + ".go" }
func unitSource(unit int, body string) string {
	return "package units\n\nfunc unit" + unitNumber(unit) + "(value int) int {\n\treturn " + body + "\n}\n"
}

// fixtureChange commits body(unit) as every unit's return line on a child
// of base, prepares its cem/1.0 map, cites each hunk by ID with the
// (evidence path, lines) cite(unit) names (none when the path is empty) and
// keeps the map as .corvint/changes/<target>.cem.json.
func fixtureChange(t *testing.T, root, base string, body func(int) string, cite func(int) (string, string)) string {
	t.Helper()
	git(t, root, "checkout", "-q", "--detach", base)
	for unit := 1; unit <= fixtureUnits; unit++ {
		write(t, root, unitPath(unit), unitSource(unit, body(unit)))
	}
	git(t, root, "commit", "-qam", "fixture")
	target := git(t, root, "rev-parse", "HEAD")
	cem := func(args ...string) {
		t.Helper()
		var out, errb bytes.Buffer
		if exit := cli.Run(context.Background(), root, args, &out, &errb); exit != 0 {
			t.Fatalf("cem %v: exit %d\n%s%s", args, exit, out.String(), errb.String())
		}
	}
	cem("prepare", "--spec", wire.StableSpec, "--base", base, "--target", target)
	mapPath := filepath.Join(root, ".corvint", "change.cem.json")
	data, err := os.ReadFile(mapPath)
	if err != nil {
		t.Fatal(err)
	}
	document, err := wire.ParseStable(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, hunk := range document.Change.Hunks {
		unit, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(hunk.Path, "src/unit"), ".go"))
		if err != nil {
			t.Fatalf("hunk path %q", hunk.Path)
		}
		if evidence, lines := cite(unit); evidence != "" {
			cem("cite", "--map", ".corvint/change.cem.json", "--hunk", hunk.ID, "--evidence-path", evidence, "--lines", lines, "--relation", "specification")
		}
	}
	if err := os.Rename(mapPath, filepath.Join(root, ".corvint", "changes", target+".cem.json")); err != nil {
		t.Fatal(err)
	}
	return target
}

func specLine(unit int) (string, string) {
	line := strconv.Itoa(2 + unit)
	return "docs/spec.md", line + ":" + line
}

// DCW-V0-051..DCW-V0-055: on the ported adversary fixtures the tool reports
// the authored, grep-bot and preimage arms by the verifier's own tiers, the
// derangement null and the self-citation members.
//
// The pinned numbers are the defect decision 0527's kill criterion targets:
// the forged-id map (every hunk breaks the requirement it cites behind a
// "// satisfies ID" comment) and the path-inventory map (every hunk cites a
// line that only names its file) anchor all 24 hunks, more than the
// honest-careful map's 16, and the grep bot, which reads only the diff,
// anchors at least as many hunks as every authored map. A tier redesign that
// tells support from lexical coincidence must change these numbers
// deliberately, with this test.
func TestBaselineDCWV0052ArmsPinTheAnchoredTierDefect(t *testing.T) {
	root, base := fixtureRepo(t)
	honest := fixtureChange(t, root, base, func(unit int) string {
		if unit <= 16 {
			return "value // " + unitID(unit) + ": return the input unchanged"
		}
		return "value"
	}, specLine)
	forged := fixtureChange(t, root, base, func(unit int) string { return "value - 1 // satisfies " + unitID(unit) }, specLine)
	inventory := fixtureChange(t, root, base, func(int) string { return "value - 1" }, func(unit int) (string, string) {
		line := strconv.Itoa(2 + unit)
		return "docs/paths.md", line + ":" + line
	})
	unknown := fixtureChange(t, root, base, func(int) string { return "value - 2" }, func(int) (string, string) { return "", "" })
	self := fixtureChange(t, root, base, func(int) string { return "value - 3" }, func(unit int) (string, string) { return unitPath(unit), "1:3" })

	summaryPath := filepath.Join(t.TempDir(), "summary.json")
	var stdout, stderr bytes.Buffer
	if exit := run(context.Background(), []string{"--root", root, "--summary", summaryPath}, &stdout, &stderr); exit != 0 {
		t.Fatalf("exit %d\n%s", exit, stderr.String())
	}
	rows := map[string]map[string]any{}
	for _, line := range strings.Split(strings.TrimSpace(stdout.String()), "\n") {
		var row map[string]any
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			t.Fatalf("row %q: %v", line, err)
		}
		rows[row["map"].(string)] = row
	}
	type armWant struct{ anchored, excl, bases, adjacent, excess float64 }
	for _, want := range []struct {
		name, stem                          string
		supported, samePath, adjacent, cite float64
		arms                                map[string]armWant
	}{
		{"honest-careful", honest, 24, 0, 0, 0, map[string]armWant{armAuthored: {16, 16, 24, 0, 16}, armGrepbot: {24, 24, 24, 0, 24}, armPreimage: {0, 0, 24, 24, 0}}},
		{"forged-id", forged, 24, 0, 0, 0, map[string]armWant{armAuthored: {24, 24, 24, 0, 24}, armGrepbot: {24, 24, 24, 0, 24}, armPreimage: {0, 0, 24, 24, 0}}},
		{"path-inventory", inventory, 24, 0, 0, 0, map[string]armWant{armAuthored: {24, 24, 24, 0, 24}, armGrepbot: {24, 24, 24, 0, 24}, armPreimage: {0, 0, 24, 24, 0}}},
		{"self-cite", self, 24, 24, 24, 24, map[string]armWant{armAuthored: {0, 0, 24, 24, 0}, armGrepbot: {24, 24, 24, 0, 24}, armPreimage: {0, 0, 24, 24, 0}}},
	} {
		row := rows[want.stem]
		if row["valid"] != true || len(row["armsIssues"].([]any)) != 0 {
			t.Fatalf("%s row = %v", want.name, row)
		}
		if row["supportedHunks"] != want.supported || row["samePathBases"] != want.samePath || row["diffAdjacentBases"] != want.adjacent || row["samePathCitations"] != want.cite {
			t.Errorf("%s members: supported %v samePath %v adjacent %v samePathCitations %v", want.name, row["supportedHunks"], row["samePathBases"], row["diffAdjacentBases"], row["samePathCitations"])
		}
		arms := row["arms"].(map[string]any)
		for name, w := range want.arms {
			arm := arms[name].(map[string]any)
			if arm["anchored"] != w.anchored || arm["anchoredExclAdjacent"] != w.excl || arm["bases"] != w.bases || arm["diffAdjacentBases"] != w.adjacent ||
				arm["nullMean"] != float64(0) || arm["anchoredExcess"] != w.excess || arm["anchoredExcessExclAdjacent"] != w.excl {
				t.Errorf("%s %s arm = %v; want %+v", want.name, name, arm, w)
			}
		}
	}
	// The pinned defect: lexical anchors on forged and inventory maps.
	anchored := func(stem, arm string) float64 {
		return rows[stem]["arms"].(map[string]any)[arm].(map[string]any)["anchored"].(float64)
	}
	if anchored(forged, armAuthored) < anchored(honest, armAuthored) || anchored(inventory, armAuthored) < anchored(honest, armAuthored) {
		t.Errorf("forged/inventory no longer anchor at least as many hunks as honest-careful: update decision 0527's baseline")
	}
	if row := rows[unknown]; row["supportedHunks"] != float64(0) || row["arms"].(map[string]any)[armAuthored].(map[string]any)["nullMean"] != nil {
		t.Errorf("all-unknown row = %v", row)
	}

	data, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatal(err)
	}
	var summary map[string]map[string]any
	if err := json.Unmarshal(data, &summary); err != nil {
		t.Fatal(err)
	}
	change := summary["change"]
	kill := change["killStatistic"].(map[string]any)
	if kill["mapsCompared"] != float64(4) || kill["grepbotAtLeastAuthored"] != float64(4) || kill["share"] != float64(1) {
		t.Errorf("killStatistic = %v", kill)
	}
	self2 := change["selfCitation"].(map[string]any)
	if self2["supportedHunks"] != float64(96) || self2["diffAdjacentBases"] != float64(24) || self2["diffAdjacentBaseShare"] != 0.25 {
		t.Errorf("selfCitation = %v", self2)
	}
	authored := change["arms"].(map[string]any)[armAuthored].(map[string]any)
	excess := authored["anchoredExcess"].(map[string]any)
	// (16 + 24 + 24 + 0) / 96 over the four maps with a null.
	if excess["maps"] != float64(4) || excess["pooled"] != 0.666667 || excess["ci95"] == nil {
		t.Errorf("authored anchoredExcess = %v", excess)
	}
}

// DCW-V0-053: the derangement null is reproducible from patchSha256 alone,
// draws nullDraws permutations without a fixed point, and is undefined below
// two supported hunks.
func TestBaselineDCWV0053DerangementsAreSeededFromThePatch(t *testing.T) {
	sha := strings.Repeat("0123456789abcdef", 4)
	first, again := derangements(sha, 7), derangements(sha, 7)
	if len(first) != nullDraws {
		t.Fatalf("draws = %d; want %d", len(first), nullDraws)
	}
	for k, draw := range first {
		if fmtInts(draw) != fmtInts(again[k]) {
			t.Fatalf("draw %d differs between runs: %v %v", k, draw, again[k])
		}
		seen := map[int]bool{}
		for i, j := range draw {
			if i == j || seen[j] || j < 0 || j >= 7 {
				t.Fatalf("draw %d = %v is not a derangement", k, draw)
			}
			seen[j] = true
		}
	}
	// Independent derivation from the DCW-V0-053 text: the seed words parsed
	// straight from the hex digits, one Fisher-Yates per attempt.
	hi, _ := strconv.ParseUint(sha[0:16], 16, 64)
	lo, _ := strconv.ParseUint(sha[16:32], 16, 64)
	generator := rand.New(rand.NewPCG(hi, lo))
	for k := 0; k < nullDraws; {
		draw := []int{0, 1, 2, 3, 4, 5, 6}
		for i := len(draw) - 1; i > 0; i-- {
			j := int(generator.Uint64() % uint64(i+1))
			draw[i], draw[j] = draw[j], draw[i]
		}
		fixed := false
		for i, j := range draw {
			fixed = fixed || i == j
		}
		if fixed {
			continue
		}
		if fmtInts(draw) != fmtInts(first[k]) {
			t.Fatalf("draw %d = %v; the DCW-V0-053 derivation gives %v", k, first[k], draw)
		}
		k++
	}
	if got := fmtInts(first[0]); got != "3 2 1 6 5 0 4" {
		t.Errorf("first draw = %s; pinned 3 2 1 6 5 0 4", got)
	}
	if other := derangements(strings.Repeat("f", 64), 7); fmtInts(other[0]) == fmtInts(first[0]) && fmtInts(other[1]) == fmtInts(first[1]) {
		t.Errorf("a different patchSha256 drew the same derangements")
	}
	if derangements(sha, 1) != nil || derangements(sha, 0) != nil {
		t.Errorf("a map with fewer than two supported hunks has a null")
	}
}

func TestBaselineDCWV0052PreimageWithoutABlobIsAnError(t *testing.T) {
	path := "a.go"
	hunks := []*patch.Hunk{{OldPath: &path, OldRange: wire.Range{Start: 3, Count: 2}}}
	if _, err := preimageSources(strings.Repeat("a", 40), hunks, map[string][]byte{}); err == nil {
		t.Fatalf("a missing old-path blob left the preimage basis empty instead of failing")
	}
	created := []*patch.Hunk{{NewPath: &path, NewRange: wire.Range{Start: 1, Count: 2}}}
	if sets, err := preimageSources(strings.Repeat("a", 40), created, map[string][]byte{}); err != nil || len(sets[0]) != 0 {
		t.Fatalf("created file: sets %v err %v; want no base and no error", sets, err)
	}
}

func fmtInts(values []int) string {
	parts := make([]string, len(values))
	for i, value := range values {
		parts[i] = strconv.Itoa(value)
	}
	return strings.Join(parts, " ")
}

// DCW-V0-056: a map whose bind commit's sidecar no longer holds its bytes is
// refused at its bind commit and measured at the target --targets names; a
// malformed targets line refuses the run.
func TestBaselineDCWV0056TargetsVerifyAtTheNamedCommit(t *testing.T) {
	root, base := fixtureRepo(t)
	target := fixtureChange(t, root, base, func(unit int) string { return "value // " + unitID(unit) }, specLine)
	write(t, root, ".corvint/change.cem.json", "{}\n")
	git(t, root, "add", ".corvint/change.cem.json")
	git(t, root, "commit", "-qm", "bind with a stale sidecar")
	bind := git(t, root, "rev-parse", "HEAD")
	if err := os.Rename(filepath.Join(root, ".corvint", "changes", target+".cem.json"), filepath.Join(root, ".corvint", "changes", bind+".cem.json")); err != nil {
		t.Fatal(err)
	}
	measure := func(args ...string) map[string]any {
		t.Helper()
		var stdout, stderr bytes.Buffer
		if exit := run(context.Background(), append([]string{"--root", root}, args...), &stdout, &stderr); exit != 0 {
			t.Fatalf("exit %d\n%s", exit, stderr.String())
		}
		var row map[string]any
		if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &row); err != nil {
			t.Fatalf("row %q: %v", stdout.String(), err)
		}
		return row
	}
	if row := measure(); row["valid"] != false || row["arms"] != nil {
		t.Errorf("row at the bind commit = %v; want a refusal without arms", row)
	}
	targets := filepath.Join(t.TempDir(), "targets.txt")
	write(t, filepath.Dir(targets), "targets.txt", bind+" "+target+"\n")
	row := measure("--targets", targets)
	if row["valid"] != true || len(row["armsIssues"].([]any)) != 0 || row["supportedHunks"] != float64(fixtureUnits) {
		t.Fatalf("row at the named target = %v", row)
	}
	if got := row["arms"].(map[string]any)[armAuthored].(map[string]any)["anchored"]; got != float64(fixtureUnits) {
		t.Errorf("authored anchored = %v; want %d", got, fixtureUnits)
	}
	write(t, filepath.Dir(targets), "targets.txt", bind+"\n")
	var stdout, stderr bytes.Buffer
	if exit := run(context.Background(), []string{"--root", root, "--targets", targets}, &stdout, &stderr); exit != 2 || !strings.Contains(stderr.String(), `want "<bind> <target>"`) {
		t.Errorf("malformed targets: exit %d stderr %q", exit, stderr.String())
	}
}
