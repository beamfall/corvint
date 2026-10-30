package workflow

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cem/witnesspaths"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/witnesspaths/datareaders"
)

// indexReaders is the data-reader finder the binary installs.
func indexReaders(ctx context.Context, root, target string) (func(string, []string) []witnesspaths.Reader, error) {
	index, err := contextindex.BuildRevisionContext(ctx, root, target)
	if err != nil {
		return nil, err
	}
	unparsed := index.UnparsedPaths()
	return func(dataPath string, declared []string) []witnesspaths.Reader {
		return datareaders.Find(dataPath, declared, index, unparsed)
	}, nil
}

// policySession is a session with the binary's data-reader finder.
func policySession(t *testing.T, root string) *Session {
	t.Helper()
	session := openSession(t, root)
	session.SetDataReaderFinder(indexReaders)
	return session
}

// pathPolicyDocument declares, one rule per line from line 4, a PR template
// and the CI configuration inert, the locale catalogues data, and the Go
// package inert, which must never classify a code hunk (CAX-V0-062).
const pathPolicyDocument = `{
  "profile": "corvint-witness-paths/0",
  "rules": [
    {"glob": ".github/*.md", "class": "inert"},
    {"glob": ".circleci/**", "class": "inert"},
    {"glob": "package.json", "class": "inert"},
    {"glob": "config/locales/**", "class": "data"},
    {"glob": "pkg/**", "class": "inert"}
  ]
}
`

// pathPolicyRepo commits, over the calc fixture, the witness path policy, a
// PR template, a locale catalogue a Go source quotes and one it does not, and
// CI configuration, and returns the root and that commit, the base of every
// range below.
func pathPolicyRepo(t *testing.T) (string, string) {
	t.Helper()
	root, _, _ := makeCalcRepo(t)
	writeFile(t, root, ".corvint/witness-paths.json", pathPolicyDocument)
	writeFile(t, root, ".github/PULL_REQUEST_TEMPLATE.md", "## Summary\n")
	writeFile(t, root, "config/locales/en.yml", "en:\n  hello: Hello\n")
	writeFile(t, root, "config/locales/fr.yml", "fr:\n  hello: Bonjour\n")
	writeFile(t, root, "pkg/calc/i18n.go", "package calc\n\n// Catalogue is the English locale.\nconst Catalogue = \"config/locales/en.yml\"\n")
	writeFile(t, root, ".circleci/config.yml", "version: 2.1\n")
	writeFile(t, root, "package.json", "{\"scripts\": {\"test\": \"jest\"}}\n")
	writeFile(t, root, "README.md", "calc\n")
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "policy")
	return root, gitCmd(t, root, "rev-parse", "HEAD")
}

// commitRange writes files, commits them and returns the new commit.
func commitRange(t *testing.T, root string, files map[string]string) string {
	t.Helper()
	for name, content := range files {
		writeFile(t, root, name, content)
	}
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "range")
	return gitCmd(t, root, "rev-parse", "HEAD")
}

// policyStatus prepares base..target and reads its status with
// --max-unknown 0 --max-mechanical 0, under --path-policy base when on.
func policyStatus(t *testing.T, root, base, target string, on bool) (prepared, status map[string]any) {
	t.Helper()
	policy := ""
	if on {
		policy = PathPolicyBase
	}
	result, err := policySession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target, Replace: true, PathPolicy: policy})
	if err != nil {
		t.Fatal(err)
	}
	zero := 0
	read, err := policySession(t, root).Read(ctx(), "status", ReadOptions{
		MapPath: wire.ExcludedCEMPath, ExpectedBase: base, Target: target,
		ArtifactRoot: root, ArtifactsGiven: true, PathPolicy: on,
		Limits: PolicyLimits{MaxUnknown: &zero, MaxMechanical: &zero},
	})
	if err != nil {
		t.Fatal(err)
	}
	return generic(t, result), generic(t, read)
}

func generic(t *testing.T, value any) map[string]any {
	t.Helper()
	var decoded map[string]any
	if err := json.Unmarshal(mustJSON(t, value), &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded
}

func requireAccepted(t *testing.T, status map[string]any) {
	t.Helper()
	if status["stableVerification"].(map[string]any)["outcome"] != "ACCEPT" {
		t.Fatalf("status did not accept: %s", mustJSON(t, status))
	}
}

// TestCAXV0063TemplateRangeIsDeclaredInert checks CAX-V0-060, CAX-V0-061 and
// CAX-V0-063 (GitHub #1981): a range touching only a PR template is
// declared-inert under --path-policy base, citing the policy file and the
// rule's line, counted apart from unknown so --max-unknown 0 holds, while the
// map bytes match a plain prepare and a plain status keeps today's counts.
func TestCAXV0063TemplateRangeIsDeclaredInert(t *testing.T) {
	root, base := pathPolicyRepo(t)
	target := commitRange(t, root, map[string]string{".github/PULL_REQUEST_TEMPLATE.md": "## Summary\n\n## Risk\n"})
	_, plain := policyStatus(t, root, base, target, false)
	plainMap, err := os.ReadFile(root + "/" + wire.ExcludedCEMPath)
	if err != nil {
		t.Fatal(err)
	}
	if plain["policyState"] != "ceiling-exceeded" || plain["pathPolicy"] != nil || plain["pathPolicyHunks"] != nil ||
		string(mustJSON(t, plain["counts"])) != `{"mechanical":0,"supported":0,"total":1,"unknown":1}` {
		t.Fatalf("plain status changed: %s", mustJSON(t, plain))
	}
	prepared, status := policyStatus(t, root, base, target, true)
	profiledMap, err := os.ReadFile(root + "/" + wire.ExcludedCEMPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(plainMap) != string(profiledMap) {
		t.Fatalf("the profile changed the map bytes")
	}
	requireAccepted(t, status)
	want := `[{"declaration":{"class":"inert","file":".corvint/witness-paths.json","glob":".github/*.md","line":4},"id":"` +
		status["stableVerification"].(map[string]any)["hunks"].([]any)[0].(map[string]any)["id"].(string) +
		`","path":".github/PULL_REQUEST_TEMPLATE.md","reason":"declared-inert","selector":1,"state":"declared-inert"}]`
	if got := string(mustJSON(t, status["pathPolicyHunks"])); got != want {
		t.Fatalf("rows %s, want %s", got, want)
	}
	if string(mustJSON(t, prepared["pathPolicyHunks"])) != want || prepared["pathPolicyProfile"] != PathPolicyProfile {
		t.Fatalf("prepare rows %s", mustJSON(t, prepared))
	}
	next := string(mustJSON(t, prepared["nextActions"]))
	if !strings.Contains(next, `"--path-policy","base"`) {
		t.Fatalf("prepare next action %s does not opt in", next)
	}
	if string(mustJSON(t, status["counts"])) != `{"declaredInert":1,"mechanical":0,"supported":0,"total":1,"unknown":0}` ||
		status["policyState"] != "within-ceilings" || status["ok"] != true ||
		string(mustJSON(t, status["pathPolicy"])) != `{"detail":"read from the base tree; a policy this range adds or edits applies only to a later range","path":".corvint/witness-paths.json","rules":5,"status":"BOUND"}` {
		t.Fatalf("profiled status %s", mustJSON(t, status))
	}
}

// TestCAXV0063CIConfigRangeIsDeclaredInert checks CAX-V0-063 (GitHub #1981):
// a CI-config range (.circleci/config.yml and package.json) is wholly
// declared-inert, each row citing its own rule line.
func TestCAXV0063CIConfigRangeIsDeclaredInert(t *testing.T) {
	root, base := pathPolicyRepo(t)
	target := commitRange(t, root, map[string]string{
		".circleci/config.yml": "version: 2.1\njobs: {}\n", "package.json": "{\"scripts\": {\"test\": \"jest --ci\"}}\n",
	})
	_, status := policyStatus(t, root, base, target, true)
	requireAccepted(t, status)
	lines := map[string]float64{}
	for _, row := range status["pathPolicyHunks"].([]any) {
		entry := row.(map[string]any)
		if entry["state"] != PathPolicyDeclaredInert {
			t.Fatalf("row %v", entry)
		}
		lines[entry["path"].(string)] = entry["declaration"].(map[string]any)["line"].(float64)
	}
	if lines[".circleci/config.yml"] != 5 || lines["package.json"] != 6 || len(lines) != 2 ||
		status["ok"] != true || status["counts"].(map[string]any)["declaredInert"] != 2.0 {
		t.Fatalf("CI-config status %s", mustJSON(t, status))
	}
}

// TestCAXV0064LocaleRangeStaysUnknownNamingItsReader checks CAX-V0-064 (GitHub
// #1981): a locale range stays unknown under the data rule; a catalogue a Go
// source quotes names that reader, one no source reads is
// data-reader-missing, and the ceiling still fails.
func TestCAXV0064LocaleRangeStaysUnknownNamingItsReader(t *testing.T) {
	root, base := pathPolicyRepo(t)
	target := commitRange(t, root, map[string]string{
		"config/locales/en.yml": "en:\n  hello: Hi\n", "config/locales/fr.yml": "fr:\n  hello: Salut\n",
	})
	_, status := policyStatus(t, root, base, target, true)
	requireAccepted(t, status)
	rows := map[string]map[string]any{}
	for _, row := range status["pathPolicyHunks"].([]any) {
		rows[row.(map[string]any)["path"].(string)] = row.(map[string]any)
	}
	en, fr := rows["config/locales/en.yml"], rows["config/locales/fr.yml"]
	if en == nil || en["state"] != "unknown" || en["reason"] != "data-reader" || en["detail"] != "read by pkg/calc/i18n.go (found)" ||
		string(mustJSON(t, en["declaration"])) != `{"class":"data","file":".corvint/witness-paths.json","glob":"config/locales/**","line":7,"readers":[{"origin":"found","path":"pkg/calc/i18n.go"}]}` {
		t.Fatalf("en row %s", mustJSON(t, en))
	}
	if fr == nil || fr["state"] != "unknown" || fr["reason"] != "data-reader-missing" {
		t.Fatalf("fr row %s", mustJSON(t, fr))
	}
	if string(mustJSON(t, status["counts"])) != `{"declaredInert":0,"mechanical":0,"supported":0,"total":2,"unknown":2}` ||
		status["policyState"] != "ceiling-exceeded" || status["ok"] != false {
		t.Fatalf("locale status %s", mustJSON(t, status))
	}
	// Without a finder the rows stay unknown and say no reader was sought.
	bare, err := openSession(t, root).Read(ctx(), "status", ReadOptions{
		MapPath: wire.ExcludedCEMPath, ExpectedBase: base, Target: target, ArtifactRoot: root, ArtifactsGiven: true, PathPolicy: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range generic(t, bare)["pathPolicyHunks"].([]any) {
		if entry := row.(map[string]any); entry["state"] != "unknown" || entry["reason"] != "data-reader-not-produced" {
			t.Fatalf("finderless row %v", entry)
		}
	}
}

// TestCAXV0062CodeMutantAndUndeclaredPathsStayUnknown checks CAX-V0-062:
// a behaviour mutant in a Go file under an inert glob is never classified, an
// undeclared path keeps today's unknown, and neither is counted inert.
func TestCAXV0062CodeMutantAndUndeclaredPathsStayUnknown(t *testing.T) {
	root, base := pathPolicyRepo(t)
	source, err := os.ReadFile(root + "/pkg/calc/calc.go")
	if err != nil {
		t.Fatal(err)
	}
	mutant := strings.Replace(string(source), "return -1", "return 1", 1)
	if mutant == string(source) {
		t.Fatal("the mutant changed nothing")
	}
	target := commitRange(t, root, map[string]string{"pkg/calc/calc.go": mutant, "README.md": "calc tools\n"})
	_, status := policyStatus(t, root, base, target, true)
	requireAccepted(t, status)
	if len(status["pathPolicyHunks"].([]any)) != 0 ||
		string(mustJSON(t, status["counts"])) != `{"declaredInert":0,"mechanical":0,"supported":0,"total":2,"unknown":2}` ||
		status["policyState"] != "ceiling-exceeded" {
		t.Fatalf("code mutant status %s", mustJSON(t, status))
	}
}

// TestCAXV0061PolicyIsReadFromTheBaseTreeOnly checks CAX-V0-061: an
// uncommitted worktree edit and a range that rewrites the policy itself have
// no effect, the policy file's own hunk is never classified, a base without a
// policy reports a null pathPolicy, and --path-policy takes only base.
func TestCAXV0061PolicyIsReadFromTheBaseTreeOnly(t *testing.T) {
	root, base := pathPolicyRepo(t)
	edited := strings.Replace(pathPolicyDocument, `"glob": ".github/*.md", "class": "inert"`, `"glob": "nothing", "class": "inert"`, 1)
	target := commitRange(t, root, map[string]string{
		".github/PULL_REQUEST_TEMPLATE.md": "## Summary\n\n## Risk\n", ".corvint/witness-paths.json": edited,
	})
	writeFile(t, root, ".corvint/witness-paths.json", `{"profile":"corvint-witness-paths/0","rules":[]}`)
	_, status := policyStatus(t, root, base, target, true)
	requireAccepted(t, status)
	rows := status["pathPolicyHunks"].([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["path"] != ".github/PULL_REQUEST_TEMPLATE.md" ||
		string(mustJSON(t, status["counts"])) != `{"declaredInert":1,"mechanical":0,"supported":0,"total":2,"unknown":1}` {
		t.Fatalf("base-only status %s", mustJSON(t, status))
	}
	gitCmd(t, root, "checkout", "-q", "--", ".corvint/witness-paths.json")

	// A base without the policy keeps every hunk unknown and says so.
	calcRoot, calcBase, calcTarget := makeCalcRepo(t)
	_, none := policyStatus(t, calcRoot, calcBase, calcTarget, true)
	requireAccepted(t, none)
	if none["pathPolicy"] != nil || len(none["pathPolicyHunks"].([]any)) != 0 || none["counts"].(map[string]any)["declaredInert"] != 0.0 {
		t.Fatalf("no-policy status %s", mustJSON(t, none))
	}
	if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target, Replace: true, PathPolicy: "worktree"}); cemcode.CodeOf(err) != cemcode.InvalidArguments {
		t.Fatalf("--path-policy worktree: %v", err)
	}
	if _, err := openSession(t, root).Read(ctx(), "verify", ReadOptions{
		MapPath: wire.ExcludedCEMPath, ExpectedBase: base, Target: target, ArtifactRoot: root, ArtifactsGiven: true, PathPolicy: true,
	}); cemcode.CodeOf(err) != cemcode.InvalidArguments {
		t.Fatalf("verify --path-policy: %v", err)
	}
}
