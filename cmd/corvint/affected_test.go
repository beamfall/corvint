package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
)

// treeDigest hashes every regular file under root except .git, ignored files
// included, so a write into ignored trace state is detected too.
func treeDigest(t *testing.T, root string) string {
	t.Helper()
	hash := sha256.New()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return fs.SkipDir
			}
			return nil
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		relative, _ := filepath.Rel(root, path)
		hash.Write([]byte(relative))
		hash.Write([]byte{0})
		hash.Write(body)
		hash.Write([]byte{0})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

// affectedFixtureRepository copies the selector's multi-language fixture into
// a fresh Git repository with one commit.
func affectedFixtureRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	source := filepath.Join("..", "..", "internal", "liveverify", "affected", "testdata", "fixture")
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, _ := filepath.Rel(source, path)
		target := filepath.Join(root, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		return os.WriteFile(target, body, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, arguments := range [][]string{
		{"init", "-q"}, {"config", "user.email", "corvint@example.test"}, {"config", "user.name", "Corvint Test"},
		// A commit must not spawn detached auto maintenance: it outlives the
		// fixture command and its .git writes race TempDir cleanup and digests.
		{"config", "maintenance.auto", "false"}, {"config", "gc.auto", "0"},
		{"add", "."}, {"commit", "-qm", "fixture"},
	} {
		affectedGit(t, root, arguments...)
	}
	return root
}

func affectedGit(t *testing.T, root string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", arguments...)
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", arguments, err, output)
	}
	return string(output)
}

func runAffectedCLI(t *testing.T, root string) (map[string]any, []byte, string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), []string{"--root", root, "affected"}, strings.NewReader(""), &stdout, &stderr)
	var receipt map[string]any
	if code == 0 {
		if err := json.Unmarshal(stdout.Bytes(), &receipt); err != nil {
			t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
		}
	}
	return receipt, stdout.Bytes(), stderr.String(), code
}

func TestAffectedCleanTreeSelectsNothingAndWritesNothing(t *testing.T) {
	t.Parallel()
	root := affectedFixtureRepository(t)
	if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".corvint", "self-observations.jsonl"), []byte("{\"kind\":\"event\"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".corvint/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	affectedGit(t, root, "add", ".gitignore")
	affectedGit(t, root, "commit", "-qm", "ignore")
	before := treeDigest(t, root)
	receipt, _, stderr, code := runAffectedCLI(t, root)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if receipt["tool"] != "affected" || receipt["profile"] != affectedProfile || receipt["mutates"] != false || receipt["ok"] != true {
		t.Fatalf("receipt envelope: %v", receipt)
	}
	plan := receipt["plan"].(map[string]any)
	if len(plan["selected"].([]any)) != 0 || len(plan["excluded"].([]any)) == 0 {
		t.Fatalf("clean tree must select nothing and exclude every unit: %v", plan)
	}
	goProvider := receipt["provider"].(map[string]any)["go"].(map[string]any)
	if len(goProvider["packages"].([]any)) != 0 || goProvider["state"] != providerStateEmpty {
		t.Fatalf("clean tree must yield an EMPTY_SELECTION projection: %v", goProvider)
	}
	if after := treeDigest(t, root); after != before {
		t.Fatal("affected wrote into the repository or its ignored state")
	}
}

func TestAffectedUnreadableSubtreeFailsClosed(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	root := affectedFixtureRepository(t)
	locked := filepath.Join(root, "leaf")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	_, _, stderr, code := runAffectedCLI(t, root)
	if code != 2 || !strings.Contains(stderr, "unsupported-affected-graph") {
		t.Fatalf("unreadable subtree must fail closed, got exit %d: %s", code, stderr)
	}
}

func TestAffectedDirtyGoSourceSelectsDependentsAsProviderPackages(t *testing.T) {
	t.Parallel()
	root := affectedFixtureRepository(t)
	corePath := filepath.Join(root, "core", "core.go")
	body, err := os.ReadFile(corePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(corePath, append(body, []byte("\n// edited\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	receipt, first, stderr, code := runAffectedCLI(t, root)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	plan := receipt["plan"].(map[string]any)
	selected := plan["selected"].([]any)
	if len(selected) == 0 {
		t.Fatalf("editing core/core.go must select at least the core unit: %v", plan)
	}
	witnessed := false
	for _, item := range selected {
		selection := item.(map[string]any)
		witness := selection["witness"].(map[string]any)
		if witness["dirtyPath"] == "core/core.go" {
			witnessed = true
		}
	}
	if !witnessed {
		t.Fatalf("every selection must carry a witness naming the dirty path: %v", selected)
	}
	goProvider := receipt["provider"].(map[string]any)["go"].(map[string]any)
	packages := goProvider["packages"].([]any)
	if len(packages) == 0 || goProvider["state"] != providerStateRunnable {
		t.Fatalf("Go selections must become a RUNNABLE provider projection: %v", goProvider)
	}
	for _, item := range packages {
		importPath := item.(string)
		if !strings.HasPrefix(importPath, "example.com/fixture") || strings.Contains(importPath, "...") {
			t.Fatalf("provider package is not an exact fixture import path: %q", importPath)
		}
	}
	_, second, _, _ := runAffectedCLI(t, root)
	if !bytes.Equal(first, second) {
		t.Fatalf("plan is not byte-identical across runs:\n%s\n%s", first, second)
	}
}

func TestAffectedUnownedDirtyPathIsUnknownScope(t *testing.T) {
	t.Parallel()
	root := affectedFixtureRepository(t)
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("scratch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	receipt, _, stderr, code := runAffectedCLI(t, root)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	plan := receipt["plan"].(map[string]any)
	if plan["scope"] != "UNKNOWN" {
		t.Fatalf("an unowned dirty path must leave scope UNKNOWN: %v", plan)
	}
	found := false
	for _, item := range plan["unknown"].([]any) {
		if item.(map[string]any)["reason"] == "UNOWNED_DIRTY_PATH" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected UNOWNED_DIRTY_PATH unknown: %v", plan["unknown"])
	}
}

// AFP-V0-021: a changed document selects the package whose test names it by
// literal, as PATH_LITERAL_READER, and stays UNOWNED_DIRTY_PATH; a document
// no literal names selects nothing and stays unknown too.
func TestAffectedDocumentSelectsThePackageThatNamesIt(t *testing.T) {
	t.Parallel()
	root := affectedFixtureRepository(t)
	for relative, body := range map[string]string{
		"core/guide_test.go": "package core\n\nconst guide = \"docs/guide.md\"\n",
		"docs/guide.md":      "guide\n",
	} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, relative)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, relative), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "document reader")
	for _, relative := range []string{"docs/guide.md", "docs/unread.md"} {
		if err := os.WriteFile(filepath.Join(root, relative), []byte("edited\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	receipt, first, stderr, code := runAffectedCLI(t, root)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	plan := receipt["plan"].(map[string]any)
	selected := fmt.Sprint(plan["selected"])
	if want := "[map[tests:[core/core_test.go core/guide_test.go] unitId:go:example.com/fixture/core witness:map[dirtyPath:docs/guide.md kind:PATH_LITERAL_READER via:[go:example.com/fixture/core]]]]"; selected != want {
		t.Fatalf("selected=%s\nwant    %s", selected, want)
	}
	unknown := fmt.Sprint(plan["unknown"])
	if plan["scope"] != "UNKNOWN" || !strings.Contains(unknown, "map[detail:docs/guide.md reason:UNOWNED_DIRTY_PATH] map[detail:docs/unread.md reason:UNOWNED_DIRTY_PATH]") {
		t.Fatalf("both documents must stay unknown: scope=%v unknown=%s", plan["scope"], unknown)
	}
	if packages := fmt.Sprint(receipt["provider"]); !strings.Contains(packages, "packages:[example.com/fixture/core] state:RUNNABLE") {
		t.Fatalf("provider=%s", packages)
	}
	if _, second, _, _ := runAffectedCLI(t, root); !bytes.Equal(first, second) {
		t.Fatalf("plan is not byte-identical across runs:\n%s\n%s", first, second)
	}
}

// AFP-V0-020: an edited Go package with no _test.go file is named in
// plan.unknown and leaves scope UNKNOWN instead of being omitted.
func TestAffectedUntestedGoPackageIsUnknownScope(t *testing.T) {
	t.Parallel()
	root := affectedFixtureRepository(t)
	source := filepath.Join(root, "untested", "untested.go")
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("package untested\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "untested package")
	if err := os.WriteFile(source, []byte("package untested\n\nconst Value = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	receipt, _, stderr, code := runAffectedCLI(t, root)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	plan := receipt["plan"].(map[string]any)
	if plan["scope"] != "UNKNOWN" {
		t.Fatalf("an edited package without tests must leave scope UNKNOWN: %v", plan)
	}
	found := false
	for _, item := range plan["unknown"].([]any) {
		entry := item.(map[string]any)
		if entry["reason"] == "NO_SELECTABLE_TEST" && entry["detail"] == "go:example.com/fixture/untested" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected NO_SELECTABLE_TEST for go:example.com/fixture/untested: %v", plan["unknown"])
	}
}

func TestAffectedRejectsNonRepositoryAndExtraArguments(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if code := runContext(context.Background(), []string{"--root", t.TempDir(), "affected"}, strings.NewReader(""), &stdout, &stderr); code != 2 {
		t.Fatalf("non-repository must exit 2, got %d: %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "not a Git repository") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
	stderr.Reset()
	root := affectedFixtureRepository(t)
	if code := runContext(context.Background(), []string{"--root", root, "affected", "--limit", "3"}, strings.NewReader(""), &stdout, &stderr); code != 2 {
		t.Fatalf("extra arguments must exit 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "unrecognized arguments") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestAffectedHelpTopicsAreWired(t *testing.T) {
	t.Parallel()
	for _, arguments := range [][]string{{"help", "affected"}, {"affected", "--help"}} {
		topic, requested, err := parseHelpInvocation(arguments)
		if err != nil || !requested || topic != "affected" {
			t.Fatalf("%v: topic=%q requested=%v err=%v", arguments, topic, requested, err)
		}
	}
	if !strings.Contains(rootHelp, "affected") || !strings.Contains(helpText("affected"), "affected-plan/0") {
		t.Fatal("affected help is not registered")
	}
}

// affectedChecks returns the advice checks of a receipt.
func affectedChecks(t *testing.T, receipt map[string]any) []map[string]any {
	t.Helper()
	advice, ok := receipt["advice"].(map[string]any)
	if !ok {
		t.Fatalf("receipt has no advice member: %v", receipt)
	}
	if advice["status"] != "PLAN_ONLY" || !strings.Contains(advice["note"].(string), "mandatory checks remain required") {
		t.Fatalf("advice envelope: %v", advice)
	}
	checks := []map[string]any{}
	for _, item := range advice["checks"].([]any) {
		checks = append(checks, item.(map[string]any))
	}
	return checks
}

func affectedAdviceUnknown(t *testing.T, receipt map[string]any) []string {
	t.Helper()
	lines := []string{}
	for _, item := range receipt["advice"].(map[string]any)["unknown"].([]any) {
		lines = append(lines, item.(string))
	}
	return lines
}

func writeAffectedGateDeclarations(t *testing.T, root string) {
	t.Helper()
	makefile := ".PHONY: gate\ngate: go-test\n\t@true\n\ngo-test:\n\t@true\n"
	if err := os.WriteFile(filepath.Join(root, "Makefile"), []byte(makefile), 0o644); err != nil {
		t.Fatal(err)
	}
	agents := "# Contract\n\nProse.\n\n## Verify\n\n```sh\nGOTOOLCHAIN=local go test -count=1 ./...\n$ GOTOOLCHAIN=local go vet ./...\n```\n"
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(agents), 0o644); err != nil {
		t.Fatal(err)
	}
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "declare the gate")
}

// AFP-V0-009: the mandatory gate the repository declares comes first, in source
// order, and the advisory Go command follows it.
func TestAffectedAdviceJoinsMandatoryGateAndAdvisoryPackages(t *testing.T) {
	t.Parallel()
	t.Run("AFP-V0-009 mandatory first then advisory", func(t *testing.T) {
		root := affectedFixtureRepository(t)
		writeAffectedGateDeclarations(t, root)
		corePath := filepath.Join(root, "core", "core.go")
		body, err := os.ReadFile(corePath)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(corePath, append(body, []byte("\n// edited\n")...), 0o644); err != nil {
			t.Fatal(err)
		}
		receipt, _, stderr, code := runAffectedCLI(t, root)
		if code != 0 {
			t.Fatalf("exit %d: %s", code, stderr)
		}
		checks := affectedChecks(t, receipt)
		if len(checks) != 4 {
			t.Fatalf("expected three mandatory checks and one advisory: %v", checks)
		}
		expected := []string{"make gate", "GOTOOLCHAIN=local go test -count=1 ./...", "GOTOOLCHAIN=local go vet ./..."}
		for index, command := range expected {
			if checks[index]["command"] != command || checks[index]["kind"] != adviceKindMandatory {
				t.Fatalf("mandatory check %d is %v, want %q", index, checks[index], command)
			}
		}
		if checks[0]["source"] != "Makefile" || checks[1]["source"] != "AGENTS.md" {
			t.Fatalf("mandatory checks must cite their declaring file: %v", checks)
		}
		advisory := checks[3]
		if advisory["kind"] != adviceKindAdvisory || advisory["source"] != adviceSourcePlan {
			t.Fatalf("last check must be the advisory plan command: %v", advisory)
		}
		if !strings.HasPrefix(advisory["command"].(string), "GOTOOLCHAIN=local go test -count=1 'example.com/fixture") {
			t.Fatalf("advisory command must run the selected packages: %v", advisory)
		}
		if !strings.Contains(advisory["reason"].(string), "dirty path") {
			t.Fatalf("advisory reason must name the selection and the dirty paths: %v", advisory)
		}
	})
}

// AFP-V0-009: a repository that declares no gate says so in the unknown
// frontier rather than inventing a mandatory check.
func TestAffectedAdviceReportsNoDeclaredGate(t *testing.T) {
	t.Parallel()
	t.Run("AFP-V0-009 no repository gate declared", func(t *testing.T) {
		root := affectedFixtureRepository(t)
		receipt, _, stderr, code := runAffectedCLI(t, root)
		if code != 0 {
			t.Fatalf("exit %d: %s", code, stderr)
		}
		for _, check := range affectedChecks(t, receipt) {
			if check["kind"] == adviceKindMandatory {
				t.Fatalf("no declaration exists, so no mandatory check may appear: %v", check)
			}
		}
		unknown := affectedAdviceUnknown(t, receipt)
		if !slices.Contains(unknown, adviceNoGateUnknown) {
			t.Fatalf("expected the NO_REPOSITORY_GATE_DECLARED unknown: %v", unknown)
		}
		if !slices.IsSorted(unknown) {
			t.Fatalf("advice unknown must be sorted: %v", unknown)
		}
	})
}

// AFP-V0-009: an unowned dirty path keeps the mandatory gate, states its
// unknown, and never proposes anything derived from an exclusion (AFP-V0-004).
func TestAffectedAdviceKeepsMandatoryGateAndNeverAdvisesExclusions(t *testing.T) {
	t.Parallel()
	t.Run("AFP-V0-009 unknown scope keeps the mandatory gate", func(t *testing.T) {
		root := affectedFixtureRepository(t)
		writeAffectedGateDeclarations(t, root)
		if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("scratch\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		receipt, _, stderr, code := runAffectedCLI(t, root)
		if code != 0 {
			t.Fatalf("exit %d: %s", code, stderr)
		}
		plan := receipt["plan"].(map[string]any)
		if plan["scope"] != "UNKNOWN" {
			t.Fatalf("an unowned dirty path must leave scope UNKNOWN: %v", plan)
		}
		checks := affectedChecks(t, receipt)
		mandatory := 0
		for _, check := range checks {
			if check["kind"] == adviceKindMandatory {
				mandatory++
			}
		}
		if mandatory != 3 {
			t.Fatalf("every declared mandatory check survives an UNKNOWN scope: %v", checks)
		}
		excluded := plan["excluded"].([]any)
		if len(excluded) == 0 {
			t.Fatal("fixture must exclude at least one unit for this assertion to bite")
		}
		for _, item := range excluded {
			unitID := item.(map[string]any)["unitId"].(string)
			for _, check := range checks {
				if strings.Contains(check["command"].(string), strings.TrimPrefix(unitID, "go:")) {
					t.Fatalf("check %v is derived from the exclusion %q", check, unitID)
				}
			}
		}
		unknown := affectedAdviceUnknown(t, receipt)
		if !slices.Contains(unknown, "NO_ADVISORY_GO_COMMAND: provider.go.state is "+providerStateEmpty) {
			t.Fatalf("a non-RUNNABLE projection must name its state: %v", unknown)
		}
		found := false
		for _, line := range unknown {
			found = found || strings.HasPrefix(line, "UNOWNED_DIRTY_PATH: ")
		}
		if !found {
			t.Fatalf("the plan frontier must be projected into advice.unknown: %v", unknown)
		}
	})
}

// AFP-V0-009: the receipt carries exactly the closed member list of AFP-V0-003
// and stays byte-identical across runs (AFP-V0-005).
func TestAffectedReceiptMembersAreClosedAndByteStable(t *testing.T) {
	t.Parallel()
	t.Run("AFP-V0-009 canonical members and byte identity", func(t *testing.T) {
		root := affectedFixtureRepository(t)
		writeAffectedGateDeclarations(t, root)
		receipt, first, stderr, code := runAffectedCLI(t, root)
		if code != 0 {
			t.Fatalf("exit %d: %s", code, stderr)
		}
		members := []string{}
		for member := range receipt {
			members = append(members, member)
		}
		sort.Strings(members)
		want := []string{"advice", "mutates", "ok", "plan", "profile", "provider", "range", "revision", "tool"}
		if !slices.Equal(members, want) {
			t.Fatalf("receipt members are %v, want %v", members, want)
		}
		adviceMembers := []string{}
		for member := range receipt["advice"].(map[string]any) {
			adviceMembers = append(adviceMembers, member)
		}
		sort.Strings(adviceMembers)
		if !slices.Equal(adviceMembers, []string{"checks", "note", "status", "unknown"}) {
			t.Fatalf("advice members are %v", adviceMembers)
		}
		if rangeMember := receipt["range"].(map[string]any); rangeMember["base"] != "" || len(rangeMember["paths"].([]any)) != 0 {
			t.Fatalf("worktree-only range must be empty, got %v", rangeMember)
		}
		adviceRaw := extractJSONObject(t, first, "advice")
		var adviceGeneric map[string]any
		if err := json.Unmarshal(adviceRaw, &adviceGeneric); err != nil {
			t.Fatalf("advice is not valid JSON: %v\n%s", err, adviceRaw)
		}
		var buf bytes.Buffer
		encoder := json.NewEncoder(&buf)
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(adviceGeneric); err != nil {
			t.Fatal(err)
		}
		resorted := bytes.TrimSuffix(buf.Bytes(), []byte{'\n'})
		if !bytes.Equal(adviceRaw, resorted) {
			t.Fatalf("advice key order is not alphabetical:\nraw:      %s\nresorted: %s", adviceRaw, resorted)
		}
		_, second, _, _ := runAffectedCLI(t, root)
		if !bytes.Equal(first, second) {
			t.Fatalf("advice is not byte-identical across runs:\n%s\n%s", first, second)
		}
	})
}

// extractJSONObject returns the raw bytes of the object value of key inside
// raw, by scanning brace depth rather than reformatting — so the result
// reflects the exact key order the producer emitted.
func extractJSONObject(t *testing.T, raw []byte, key string) []byte {
	t.Helper()
	marker := []byte(`"` + key + `":{`)
	start := bytes.Index(raw, marker)
	if start < 0 {
		t.Fatalf("key %q not found in %s", key, raw)
	}
	start += len(marker) - 1
	depth, inString, escaped := 0, false, false
	for index := start; index < len(raw); index++ {
		character := raw[index]
		if inString {
			switch {
			case escaped:
				escaped = false
			case character == '\\':
				escaped = true
			case character == '"':
				inString = false
			}
			continue
		}
		switch character {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return raw[start : index+1]
			}
		}
	}
	t.Fatalf("unbalanced object for key %q in %s", key, raw)
	return nil
}

// AFP-V0-009: the per-file read bound truncates rather than failing, and a
// declaration inside the bound is still found.
func TestAffectedAdviceBoundsTheDeclarationRead(t *testing.T) {
	t.Parallel()
	t.Run("AFP-V0-009 bounded declaration read", func(t *testing.T) {
		root := affectedFixtureRepository(t)
		padding := strings.Repeat("# padding padding padding padding padding padding\n", 6500)
		makefile := "gate:\n\t@true\n" + padding + "late:\n\t@true\n"
		if len(makefile) < 300<<10 {
			t.Fatalf("fixture Makefile is %d bytes, want more than 300 KiB", len(makefile))
		}
		if err := os.WriteFile(filepath.Join(root, "Makefile"), []byte(makefile), 0o644); err != nil {
			t.Fatal(err)
		}
		receipt, _, stderr, code := runAffectedCLI(t, root)
		if code != 0 {
			t.Fatalf("exit %d: %s", code, stderr)
		}
		checks := affectedChecks(t, receipt)
		if len(checks) == 0 || checks[0]["command"] != "make gate" {
			t.Fatalf("a gate target inside the read bound must still be found: %v", checks)
		}
		if body, truncated := readAdviceSource(filepath.Join(root, "Makefile"), adviceMaxSourceBytes); len(body) != adviceMaxSourceBytes || !truncated {
			t.Fatalf("the read is not bounded at %d bytes: %d (truncated=%v)", adviceMaxSourceBytes, len(body), truncated)
		}
	})
}

// AFP-V0-009: an advisory package path carrying a shell metacharacter must
// not break a paste of the advisory command into a shell.
func TestShellQuoteJoinEscapesMetacharacters(t *testing.T) {
	t.Parallel()
	t.Run("AFP-V0-009 shell quoting", func(t *testing.T) {
		got := shellQuoteJoin([]string{"pkg;rm -rf /", "pkg's/path"})
		want := `'pkg;rm -rf /' 'pkg'\''s/path'`
		if got != want {
			t.Fatalf("shellQuoteJoin = %s, want %s", got, want)
		}
	})
}

// AFP-V0-009: a Makefile whose only gate target sits past the 256 KiB read
// bound must not be found, and the truncated read must be named rather than
// silently reported as no gate declared.
func TestAffectedAdviceTruncatedMandatoryDeclarationSuppressesNoGate(t *testing.T) {
	t.Parallel()
	t.Run("AFP-V0-009 truncated mandatory declaration", func(t *testing.T) {
		root := affectedFixtureRepository(t)
		padding := strings.Repeat("# padding padding padding padding padding padding\n", 6500)
		makefile := padding + "gate:\n\t@true\n"
		if len(makefile) < 300<<10 {
			t.Fatalf("fixture Makefile is %d bytes, want more than 300 KiB", len(makefile))
		}
		if err := os.WriteFile(filepath.Join(root, "Makefile"), []byte(makefile), 0o644); err != nil {
			t.Fatal(err)
		}
		receipt, _, stderr, code := runAffectedCLI(t, root)
		if code != 0 {
			t.Fatalf("exit %d: %s", code, stderr)
		}
		for _, check := range affectedChecks(t, receipt) {
			if check["kind"] == adviceKindMandatory {
				t.Fatalf("a gate target past the read bound must not be found: %v", check)
			}
		}
		unknown := affectedAdviceUnknown(t, receipt)
		want := fmt.Sprintf("MANDATORY_DECLARATION_TRUNCATED: Makefile exceeded %d bytes", adviceMaxSourceBytes)
		if !slices.Contains(unknown, want) {
			t.Fatalf("expected the truncated unknown: %v", unknown)
		}
		if slices.Contains(unknown, adviceNoGateUnknown) {
			t.Fatalf("a truncated source must never coexist with NO_REPOSITORY_GATE_DECLARED: %v", unknown)
		}
	})
}

// AFP-V0-009: an AGENTS.md Verify fence declaring more than 16 commands is
// capped at 16 mandatory entries, and the drop is named in unknown.
func TestAffectedAdviceCapsMandatoryChecksAtSixteen(t *testing.T) {
	t.Parallel()
	t.Run("AFP-V0-009 mandatory checks capped at sixteen", func(t *testing.T) {
		root := affectedFixtureRepository(t)
		lines := make([]string, 20)
		for index := range lines {
			lines[index] = fmt.Sprintf("cmd%02d", index)
		}
		agents := "## Verify\n\n```sh\n" + strings.Join(lines, "\n") + "\n```\n"
		if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(agents), 0o644); err != nil {
			t.Fatal(err)
		}
		receipt, _, stderr, code := runAffectedCLI(t, root)
		if code != 0 {
			t.Fatalf("exit %d: %s", code, stderr)
		}
		mandatory := 0
		for _, check := range affectedChecks(t, receipt) {
			if check["kind"] == adviceKindMandatory {
				mandatory++
			}
		}
		if mandatory != adviceMaxMandatoryChecks {
			t.Fatalf("expected %d mandatory entries, got %d", adviceMaxMandatoryChecks, mandatory)
		}
		unknown := affectedAdviceUnknown(t, receipt)
		want := fmt.Sprintf("MANDATORY_DECLARATION_CAPPED: AGENTS.md declared more than %d commands", adviceMaxMandatoryChecks)
		if !slices.Contains(unknown, want) {
			t.Fatalf("expected the capped unknown: %v", unknown)
		}
	})
}

// AFP-V0-009: a comment line inside a Verify fence contributes no command.
func TestAffectedAdviceSkipsCommentsInVerifyFence(t *testing.T) {
	t.Parallel()
	t.Run("AFP-V0-009 verify fence comment skipped", func(t *testing.T) {
		root := affectedFixtureRepository(t)
		agents := "## Verify\n\n```sh\n# comment\nGOTOOLCHAIN=local go test -count=1 ./...\nGOTOOLCHAIN=local go vet ./...\n```\n"
		if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(agents), 0o644); err != nil {
			t.Fatal(err)
		}
		receipt, _, stderr, code := runAffectedCLI(t, root)
		if code != 0 {
			t.Fatalf("exit %d: %s", code, stderr)
		}
		mandatory := 0
		for _, check := range affectedChecks(t, receipt) {
			if check["kind"] == adviceKindMandatory {
				mandatory++
			}
		}
		if mandatory != 2 {
			t.Fatalf("a comment line inside the fence must not become a check, got %d mandatory entries", mandatory)
		}
	})
}

// AFP-V0-010: `--base FULL_COMMIT_ID` joins the committed tree diff
// base..HEAD to the worktree dirty set, records it under range, and fails
// closed on a base that is not a full commit id in this repository.
func TestAffectedBaseRangeJoinsCommittedPathsAndFailsClosed(t *testing.T) {
	t.Parallel()
	t.Run("AFP-V0-010 committed range selects with a clean worktree", func(t *testing.T) {
		root := affectedFixtureRepository(t)
		base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
		corePath := filepath.Join(root, "core", "core.go")
		body, err := os.ReadFile(corePath)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(corePath, append(body, []byte("\n// committed edit\n")...), 0o644); err != nil {
			t.Fatal(err)
		}
		affectedGit(t, root, "commit", "-qam", "edit core")
		var stdout, stderr bytes.Buffer
		if code := runContext(context.Background(), []string{"--root", root, "affected", "--base", base}, strings.NewReader(""), &stdout, &stderr); code != 0 {
			t.Fatalf("exit %d: %s", code, stderr.String())
		}
		var receipt map[string]any
		if err := json.Unmarshal(stdout.Bytes(), &receipt); err != nil {
			t.Fatal(err)
		}
		rangeMember := receipt["range"].(map[string]any)
		if rangeMember["base"] != base || !slices.Equal(anyStrings(rangeMember["paths"]), []string{"core/core.go"}) {
			t.Fatalf("range=%v", rangeMember)
		}
		plan := receipt["plan"].(map[string]any)
		if !slices.Equal(anyStrings(plan["dirty"]), []string{"core/core.go"}) {
			t.Fatalf("plan.dirty=%v must carry the committed path", plan["dirty"])
		}
		packages := anyStrings(receipt["provider"].(map[string]any)["go"].(map[string]any)["packages"])
		if !slices.Contains(packages, "example.com/fixture/core") || !slices.Contains(packages, "example.com/fixture/leaf") {
			t.Fatalf("packages=%v", packages)
		}
	})
	t.Run("AFP-V0-010 malformed or unknown base exits 2", func(t *testing.T) {
		root := affectedFixtureRepository(t)
		for _, tc := range []struct{ base, code string }{
			{"main", "invalid-arguments"},
			{strings.Repeat("0", 40), "unsupported-affected-revision"},
		} {
			var stdout, stderr bytes.Buffer
			if code := runContext(context.Background(), []string{"--root", root, "affected", "--base", tc.base}, strings.NewReader(""), &stdout, &stderr); code != 2 {
				t.Fatalf("base %q must exit 2, got %d", tc.base, code)
			}
			if !strings.Contains(stderr.String(), tc.code) || stdout.Len() != 0 {
				t.Fatalf("base %q: stderr=%s stdout=%s", tc.base, stderr.String(), stdout.String())
			}
		}
	})
}

func anyStrings(value any) []string {
	items, _ := value.([]any)
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.(string))
	}
	return out
}
