package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cli"
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

// bindMap prepares base..target, cites every hunk ordinal to its own span of
// docs/spec.md and keeps the map as .corvint/changes/<target>.cem.json.
func bindMap(t *testing.T, root, base, target string, hunks int) {
	t.Helper()
	cem := func(args ...string) {
		t.Helper()
		var out, errb bytes.Buffer
		if exit := cli.Run(context.Background(), root, args, &out, &errb); exit != 0 {
			t.Fatalf("cem %v: exit %d\n%s%s", args, exit, out.String(), errb.String())
		}
	}
	// The baseline measures the committed cem/0.x maps (V1-1262); the stable
	// producer default (CEM-V1-140) is out of its scope until the migration.
	cem("prepare", "--spec", "cem/0.2", "--base", base, "--target", target)
	for ordinal := 1; ordinal <= hunks; ordinal++ {
		cem("cite", "--map", ".corvint/change.cem.json", "--hunk", strconv.Itoa(ordinal), "--evidence-path", "docs/spec.md", "--lines", "1:"+strconv.Itoa(ordinal), "--relation", "specification")
	}
	if err := os.Rename(filepath.Join(root, ".corvint", "change.cem.json"), filepath.Join(root, ".corvint", "changes", target+".cem.json")); err != nil {
		t.Fatal(err)
	}
}

// DCW-V0-049: the baseline tool reproduces the V1-1262 row and summary shape
// over a directory of maps and splits it by citation class: a change touching
// only CEM map paths is class chore, any other change is class change, and a
// map whose status carries no citation block is an error row.
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
	bindMap(t, root, base, change, 2)
	git(t, root, "add", ".corvint/changes")
	git(t, root, "commit", "-qm", "chore: bind the change map")
	chore := git(t, root, "rev-parse", "HEAD")
	bindMap(t, root, change, chore, 1)
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
	if len(rows) != 3 {
		t.Fatalf("rows = %d; want 3\n%s", len(rows), stdout.String())
	}
	if row := rows[change]; row["class"] != "change" || row["valid"] != true || row["hunksTotal"] != float64(2) || row["distinctEvidence"] != float64(2) {
		t.Errorf("change row = %v", row)
	}
	if row := rows[chore]; row["class"] != "chore" || row["valid"] != true || row["hunksTotal"] != float64(1) {
		t.Errorf("chore row = %v", row)
	}
	if row := rows[broken]; row["valid"] != false || row["hunksTotal"] != float64(0) || len(row["issues"].([]any)) == 0 {
		t.Errorf("error row = %v", row)
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
		{"all", "maps", 3}, {"all", "withCitation", 2}, {"all", "invalidOrError", 1}, {"all", "hunks", 3},
		{"change", "maps", 1}, {"change", "hunks", 2}, {"chore", "maps", 1}, {"chore", "hunks", 1},
	} {
		if got := summary[check.class][check.key]; got != check.want {
			t.Errorf("summary %s.%s = %v; want %v", check.class, check.key, got, check.want)
		}
	}
}

// The rows keep the baseline's Python json.dumps encoding, so a row for an
// unchanged map compares byte for byte apart from its class member.
func TestBaselineRowKeepsThePythonEncoding(t *testing.T) {
	r := row{stem: "02ed58f3402291aa678b6cc1741134ef7bd482ba", spec: "cem/0.2", class: "change", valid: true, citation: map[string]any{
		"hunks": []any{1, 2, 3, 4}, "annotationHunks": float64(0), "concentratedHunks": float64(4), "distinctEvidence": float64(2),
		"hunksByClass": map[string]any{"anchored": float64(0), "cited": float64(4), "held": float64(0), "mechanical-proved": float64(0), "unknown": float64(0)},
		"invariant":    false, "maxFanOut": float64(2), "specificity": float64(0),
	}}
	want := `{"annotationHunks": 0, "class": "change", "concentratedHunks": 4, "distinctEvidence": 2, "hunksByClass": {"anchored": 0, "cited": 4, "held": 0, "mechanical-proved": 0, "unknown": 0}, "hunksTotal": 4, "invariant": false, "map": "02ed58f3402291aa678b6cc1741134ef7bd482ba", "maxFanOut": 2, "spec": "cem/0.2", "specificity": 0, "valid": true}`
	if got := pyJSON(r.fields(), -1, 0); got != want {
		t.Errorf("row\n got %s\nwant %s", got, want)
	}
	if got := pyJSON(ordered{{"maps", 1}, {"tiers", ordered{{"cited", 4}}}}, 1, 0); got != "{\n \"maps\": 1,\n \"tiers\": {\n  \"cited\": 4\n }\n}" {
		t.Errorf("summary encoding = %q", got)
	}
}

// CEM-V1-201: a converted cem/1.0 map is measured with --artifacts at its
// target from --targets, and keeps its source's path class because the
// converter omits role. The rows match the cem/0.x rows apart from spec.
func TestBaselineCEMV1201MeasuresConvertedMaps(t *testing.T) {
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
	write(t, root, "a.txt", "a\nmore a\n")
	git(t, root, "commit", "-qam", "change")
	change := git(t, root, "rev-parse", "HEAD")
	if err := os.MkdirAll(filepath.Join(root, ".corvint", "changes"), 0o755); err != nil {
		t.Fatal(err)
	}
	bindMap(t, root, base, change, 1)
	git(t, root, "add", ".corvint/changes")
	git(t, root, "commit", "-qm", "chore: bind the change map")
	chore := git(t, root, "rev-parse", "HEAD")
	bindMap(t, root, change, chore, 1)
	measureAll := func(extra ...string) map[string]map[string]any {
		t.Helper()
		var stdout, stderr bytes.Buffer
		if exit := run(context.Background(), append([]string{"--root", root}, extra...), &stdout, &stderr); exit != 0 || stderr.Len() != 0 {
			t.Fatalf("exit %d\n%s", exit, stderr.String())
		}
		rows := map[string]map[string]any{}
		for _, line := range strings.Split(strings.TrimSpace(stdout.String()), "\n") {
			var row map[string]any
			if err := json.Unmarshal([]byte(line), &row); err != nil {
				t.Fatalf("row %q: %v", line, err)
			}
			delete(row, "spec")
			rows[row["map"].(string)] = row
		}
		return rows
	}
	before := measureAll()
	for _, bind := range []string{change, chore} {
		var out bytes.Buffer
		mapPath := filepath.Join(root, ".corvint", "changes", bind+".cem.json")
		if exit, ok := cli.RunStableInvocation(context.Background(), []string{"cem", "convert", "--in-place", "--repository", root, "--map", mapPath}, &out); !ok || exit != 0 {
			t.Fatalf("convert %s: exit %d\n%s", bind, exit, out.String())
		}
	}
	targets := filepath.Join(t.TempDir(), "targets.txt")
	if err := os.WriteFile(targets, []byte(change+" "+change+"\n"+chore+" "+chore+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	after := measureAll("--targets", targets)
	if len(after) != 2 || after[chore]["class"] != "chore" || after[change]["valid"] != true {
		t.Fatalf("converted rows = %v", after)
	}
	for bind, row := range before {
		if pyJSON(row, -1, 0) != pyJSON(after[bind], -1, 0) {
			t.Errorf("%s:\n before %v\n after  %v", bind, row, after[bind])
		}
	}
	if err := os.WriteFile(targets, []byte(change+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if exit := run(context.Background(), []string{"--root", root, "--targets", targets}, &stdout, &stderr); exit != 2 {
		t.Fatalf("malformed targets: exit %d", exit)
	}
}
