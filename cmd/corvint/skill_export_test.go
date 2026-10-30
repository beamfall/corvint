package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runSkillExportForTest(t *testing.T, arguments ...string) (int, string, string) {
	t.Helper()
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	code := runSkillExport(t.Context(), arguments, stdout, stderr)
	return code, stdout.String(), stderr.String()
}

// LTA-V0-006: skill-export is an explicit verb that writes only under the
// operator-named --out directory, leaves repository and trace state
// byte-identical, and exports the same admitted rows to identical bytes on a
// second run; unadmitted (failed, blocked) rows produce no skill.
func TestSkillExportWritesOnlyOperatorDirectory_LTA006(t *testing.T) {
	t.Parallel()
	root := calibrateRepository(t, 3)
	before := calibrateTreeDigest(t, root)
	first, second := filepath.Join(t.TempDir(), "skills"), filepath.Join(t.TempDir(), "again")
	code, stdout, stderr := runSkillExportForTest(t, "--root", root, "skill-export", "--out", first)
	if code != 0 || stderr != "" {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	if after := calibrateTreeDigest(t, root); after != before {
		t.Fatal("skill-export changed repository or trace state")
	}
	var manifest struct {
		Exported   int    `json:"exported"`
		TraceState string `json:"trace_state"`
		Skills     []struct {
			Name       string `json:"name"`
			Digest     string `json:"admission_evidence_digest"`
			Evaluation string `json:"evaluation"`
		} `json:"skills"`
	}
	if err := json.Unmarshal([]byte(stdout), &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Exported != 1 || len(manifest.Skills) != 1 || manifest.Skills[0].Evaluation != "NOT_RECORDED" || !strings.HasPrefix(manifest.Skills[0].Digest, "sha256:") {
		t.Fatalf("manifest = %s", stdout)
	}
	skill := filepath.Join(first, manifest.Skills[0].Name)
	document, err := os.ReadFile(filepath.Join(skill, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(document), "- Admission evidence digest: "+manifest.Skills[0].Digest+"\n") {
		t.Fatalf("SKILL.md lacks the manifest digest:\n%s", document)
	}
	entries, err := os.ReadDir(first)
	if err != nil || len(entries) != 1 {
		t.Fatalf("out entries = %v, %v; want the one passed rule", entries, err)
	}
	code, again, stderr := runSkillExportForTest(t, "--root="+root, "skill-export", "--out="+second)
	if code != 0 || stderr != "" {
		t.Fatalf("second run exit=%d stderr=%q", code, stderr)
	}
	if strings.Replace(again, second, first, 1) != stdout {
		t.Fatalf("manifests differ:\n%s\n%s", stdout, again)
	}
	for _, name := range []string{"SKILL.md", filepath.Join("references", "trace.md")} {
		reread, err := os.ReadFile(filepath.Join(second, manifest.Skills[0].Name, name))
		if err != nil {
			t.Fatal(err)
		}
		original, _ := os.ReadFile(filepath.Join(skill, name))
		if !bytes.Equal(original, reread) {
			t.Fatalf("%s differs between runs", name)
		}
	}
}

// LTA-V0-006: --out is required, must not point inside .corvint, and no other
// flag is accepted.
func TestSkillExportInvocationFlags_LTA006(t *testing.T) {
	t.Parallel()
	root := calibrateRepository(t, 1)
	if !skillExportInvoked([]string{"--root", root, "skill-export"}) || skillExportInvoked([]string{"--root", root, "calibrate"}) {
		t.Fatal("verb selection")
	}
	for _, test := range []struct {
		arguments []string
		want      string
	}{
		{[]string{"--root", root, "skill-export"}, "argument --out is required"},
		{[]string{"--root", root, "skill-export", "--out"}, "argument --out: expected one argument"},
		{[]string{"--root", root, "skill-export", "--out", filepath.Join(root, ".corvint", "skills")}, "must not point inside the trace state directory"},
		{[]string{"--root", root, "skill-export", "--out", t.TempDir(), "--format", "json"}, "unrecognized arguments: --format"},
	} {
		code, stdout, stderr := runSkillExportForTest(t, test.arguments...)
		if code != 2 || stdout != "" || !strings.Contains(stderr, test.want) {
			t.Fatalf("%q: exit=%d stdout=%q stderr=%q", test.arguments, code, stdout, stderr)
		}
	}
}
