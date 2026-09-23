package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

type authorityParityCase struct {
	ID           string   `json:"id"`
	Repository   string   `json:"repository"`
	Commit       string   `json:"commitRevision"`
	Tree         string   `json:"treeRevision"`
	Argv         []string `json:"argv"`
	StdoutSHA256 string   `json:"stdoutSha256"`
	StderrSHA256 string   `json:"stderrSha256"`
	StdinSHA256  string   `json:"stdinSha256"`
	StatusBefore string   `json:"statusBeforeSha256"`
	StatusAfter  string   `json:"statusAfterSha256"`
	StdoutBytes  int      `json:"stdoutBytes"`
	Exit         int      `json:"exitStatus"`
	Mutation     string   `json:"mutation"`
}

type authorityParityManifest struct {
	Profile          string                `json:"profile"`
	Task             string                `json:"task"`
	CandidateCommand string                `json:"candidateCommand"`
	OracleCommand    string                `json:"oracleCommand"`
	Environment      map[string]string     `json:"selectedEnvironment"`
	Cases            []authorityParityCase `json:"cases"`
	RuntimeAuthority bool                  `json:"runtimeAuthority"`
}

func queryCLIRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, arguments := range [][]string{
		{"init", "-q"},
		{"config", "maintenance.auto", "false"}, {"config", "gc.auto", "0"},
		{"config", "user.email", "corvint@example.test"},
		{"config", "user.name", "Corvint Test"},
	} {
		command := exec.Command("git", arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
	files := map[string]string{
		".gitignore": ".context-corvint/\n",
		"AGENTS.md": "# Project instructions\n\nThe roadmap is the only active work queue.\n" +
			"Run `make orient`, then `script/context-packet.sh --ticket ID`.\n" +
			"Use `script/roadmap.sh` and run the required workflow gates.\n",
		"script/context-packet.sh": "#!/bin/sh\nexit 0\n",
		"script/roadmap.sh":        "#!/bin/sh\nexit 0\n",
		"internal/parser/token.go": "package parser\n\n// ParseToken validates token parser delimiters.\nfunc ParseToken(input string) string { return input }\n",
	}
	for relative, contents := range files {
		file := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, arguments := range [][]string{{"add", "."}, {"commit", "-qm", "initial authority fixture"}} {
		command := exec.Command("git", arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
	historyPath := filepath.Join(root, "docs", "history.md")
	if err := os.MkdirAll(filepath.Dir(historyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	for index, fixture := range []struct {
		contents string
		message  []byte
	}{
		{"unicode secret dotless i\n", []byte("roadmap prıvate_key\u2003=\u2003correct-horse-battery-staple ticket\n")},
		{"unicode secret dotted I\n", []byte("roadmap prİvate_key=value ticket\n")},
		{"unicode secret long s\n", []byte("roadmap ſecret=value ticket\n")},
		{"unicode secret kelvin\n", []byte("roadmap toKen=value ticket\n")},
		{"malformed subject\n", []byte{'r', 'o', 'a', 'd', 'm', 'a', 'p', ' ', 0xff, 0xff, ' ', 't', 'i', 'c', 'k', 'e', 't', '\n'}},
	} {
		if err := os.WriteFile(historyPath, []byte(fixture.contents), 0o644); err != nil {
			t.Fatal(err)
		}
		messagePath := filepath.Join(t.TempDir(), fmt.Sprintf("message-%d", index))
		if err := os.WriteFile(messagePath, fixture.message, 0o600); err != nil {
			t.Fatal(err)
		}
		for _, arguments := range [][]string{{"add", "docs/history.md"}, {"commit", "-q", "-F", messagePath}} {
			command := exec.Command("git", arguments...)
			command.Dir = root
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("git %v: %v\n%s", arguments, err, output)
			}
		}
	}
	return root
}

func runQueryProcess(t *testing.T, root string, extraArguments ...string) processResult {
	t.Helper()
	arguments := []string{"--root", root, "query", "--task", authorityStartPrompt, "--limit", "1"}
	arguments = append(arguments, extraArguments...)
	candidate := exec.Command(os.Args[0], append([]string{"-test.run=^TestCandidateHelperProcess$", "--"}, arguments...)...)
	candidate.Env = append(os.Environ(), "CORVINT_HELPER_PROCESS=1")
	return execute(t, candidate)
}

func TestFreshProcessAuthorityStartBudgetMatchesPython(t *testing.T) {
	t.Parallel()
	root := queryCLIRepository(t)
	candidate := runQueryProcess(t, root, "--budget-bytes", "1500")
	if candidate.exit != 0 || len(candidate.stderr) != 0 {
		t.Fatalf("candidate exit=%d stderr=%q", candidate.exit, candidate.stderr)
	}
}

func TestFreshProcessAuthorityStartQueryMatchesPythonCleanAndMixed(t *testing.T) {
	t.Parallel()
	root := queryCLIRepository(t)
	before := repositoryBytesDigest(t, root)
	candidate := runQueryProcess(t, root)
	if candidate.exit != 0 || len(candidate.stderr) != 0 {
		t.Fatalf("candidate exit=%d stderr=%q", candidate.exit, candidate.stderr)
	}
	if after := repositoryBytesDigest(t, root); before != after {
		t.Fatal("clean authority-start query mutated repository bytes")
	}

	file := filepath.Join(root, "AGENTS.md")
	if err := os.WriteFile(file, []byte("uncommitted replacement\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before = repositoryBytesDigest(t, root)
	candidate = runQueryProcess(t, root)
	if !bytes.Contains(candidate.stdout, []byte(`"state":"READY"`)) || !bytes.Contains(candidate.stdout, []byte(`"state":"mixed-worktree"`)) {
		t.Fatalf("mixed stdout=%s", candidate.stdout)
	}
	if after := repositoryBytesDigest(t, root); before != after {
		t.Fatal("mixed authority-start query mutated repository bytes")
	}
}

func TestAuthorityStartQueryAcceptsPacketBudget(t *testing.T) {
	t.Parallel()
	root := queryCLIRepository(t)
	arguments := []string{
		"--root", root, "query", "--task", authorityStartPrompt, "--limit", "1",
		"--budget-bytes", "1500",
	}
	reader := &forbiddenImpactReader{}
	var stdout, stderr bytes.Buffer
	if exit := run(arguments, reader, &stdout, &stderr); exit != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, &stdout, &stderr)
	}
	if stderr.Len() != 0 || reader.reads != 0 {
		t.Fatalf("stderr=%q stdin reads=%d", &stderr, reader.reads)
	}
	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	contextReceipt := payload["context"].(map[string]any)
	coverage := contextReceipt["coverage"].(map[string]any)
	if coverage["budget_bytes"] != float64(1500) || coverage["within_budget"] != true ||
		coverage["packet_bytes"].(float64) > 1500 {
		t.Fatalf("coverage=%v", coverage)
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"results":[]`)) {
		t.Fatalf("empty lists did not serialize as arrays: %s", &stdout)
	}
}

func TestAuthorityStartQueryValidatesPacketBudgetLikePython(t *testing.T) {
	t.Parallel()
	for _, budget := range []string{"1216", "1000000"} {
		parsed, err := parseQueryArgumentsForPlatform(options{queryLimit: 10}, []string{
			"--task", authorityStartPrompt, "--limit", "1", "--budget-bytes", budget,
		}, "darwin")
		if err != nil || parsed.queryBudget == nil {
			t.Fatalf("budget=%s parsed=%#v err=%v", budget, parsed.queryBudget, err)
		}
	}
	for _, test := range []struct {
		value, message string
	}{
		{"nope", "argument --budget-bytes: invalid literal for int() with base 10: 'nope'"},
		{"1023", "argument --budget-bytes: budget_bytes must be between 1216 and 1000000"},
		{"1000001", "argument --budget-bytes: budget_bytes must be between 1216 and 1000000"},
	} {
		_, err := parseQueryArgumentsForPlatform(options{queryLimit: 10}, []string{
			"--task", authorityStartPrompt, "--limit", "1", "--budget-bytes", test.value,
		}, "darwin")
		var stderr bytes.Buffer
		emitError(&stderr, err)
		if !bytes.Contains(stderr.Bytes(), []byte(test.message)) {
			t.Fatalf("value=%q stderr=%q", test.value, &stderr)
		}
	}
	parsed, err := parseQueryArgumentsForPlatform(options{queryLimit: 10}, []string{
		"--task", authorityStartPrompt, "--limit", "1",
	}, "darwin")
	if err != nil || parsed.queryBudget != nil {
		t.Fatalf("unbudgeted parsed=%#v err=%v", parsed.queryBudget, err)
	}
}

// TestQueryTaskValueFollowsArgparseOptionLikeClassification pins GPK-V0-064:
// a next token that looks like an option is refused as a missing value for
// --task, exactly like the retired argparse oracle, while a negative number
// or an inline `--task=VALUE` stays a value. docs/agent-memory/bugs.md
// "cmd/corvint: option values that start with `-` diverge from the argparse
// oracle" (decision 0173).
func TestQueryTaskValueFollowsArgparseOptionLikeClassification(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"short-option-like-value-refused", []string{"--task", "-x"}, "argument --task: expected one argument"},
		{"help-flag-as-value-refused", []string{"--task", "--help"}, "argument --task: expected one argument"},
		{"h-flag-as-value-refused", []string{"--task", "-h"}, "argument --task: expected one argument"},
		{"negative-number-is-a-value", []string{"--task", "-5"}, ""},
		{"inline-dash-value-is-a-value", []string{"--task=-x"}, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			parsed, err := parseQueryArgumentsForPlatform(options{queryLimit: 10}, test.args, "darwin")
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("args=%q unexpected error: %v", test.args, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("args=%q parsed=%#v: want error %q, got none", test.args, parsed, test.wantErr)
			}
			var stderr bytes.Buffer
			emitError(&stderr, err)
			if !bytes.Contains(stderr.Bytes(), []byte(test.wantErr)) {
				t.Fatalf("args=%q stderr=%q want=%q", test.args, &stderr, test.wantErr)
			}
		})
	}
}

func TestAuthorityStartQueryRejectsUnsupportedProfilesBeforeStdin(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, stderr string
		arguments    []string
	}{
		{"limit-51", "{\"error\": \"limit must be an integer from 1 to 50\", \"ok\": false}\n", []string{"query", "--task", authorityStartPrompt, "--limit", "51"}},
		{"limit-zero", "{\"error\": \"limit must be an integer from 1 to 50\", \"ok\": false}\n", []string{"query", "--task", "orient roadmap café workflow", "--limit", "0"}},
		{"empty-task", "{\"error\": \"query text must be non-empty\", \"ok\": false}\n", []string{"query", "--task", " ", "--limit", "1"}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			arguments := append([]string{"--root", queryRejectionCanaryRoot(t)}, test.arguments...)
			reader := &forbiddenImpactReader{}
			var stdout, stderr bytes.Buffer
			if exit := run(arguments, reader, &stdout, &stderr); exit != 2 || stdout.Len() != 0 {
				t.Fatalf("exit=%d stdout=%q stderr=%q", exit, &stdout, &stderr)
			}
			if stderr.String() != test.stderr {
				t.Fatalf("stderr=%q want=%q", &stderr, test.stderr)
			}
			if reader.reads != 0 {
				t.Fatalf("stdin reads=%d", reader.reads)
			}
		})
	}
}

func TestAuthorityStartQueryPlatformRejectsBeforeRepositoryWork(t *testing.T) {
	t.Parallel()
	_, err := parseQueryArgumentsForPlatform(options{queryLimit: 10}, []string{
		"--task", authorityStartPrompt, "--limit", "1",
	}, "windows")
	var stdout, stderr bytes.Buffer
	emitError(&stderr, err)
	if stdout.Len() != 0 || stderr.String() != "{\"code\": \"unsupported-query-platform\", \"error\": \"native Go authority-start query is qualified only on Darwin and Linux\", \"evidence\": [{\"name\": \"platform\", \"value\": \"windows\"}], \"ok\": false, \"subject\": {\"kind\": \"host-capability\", \"value\": \"native-platform\"}, \"supported_fixes\": [], \"terminal\": \"unsupported-platform\"}\n" {
		t.Fatalf("stdout=%q stderr=%q", &stdout, &stderr)
	}
}

func queryRejectionCanaryRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestAuthorityStartParityManifestIsClosed(t *testing.T) {
	t.Parallel()
	manifest := loadAuthorityParityManifest(t)
	if manifest.Profile != "corvint-authority-start-query-parity/0" || manifest.Task != authorityStartPrompt ||
		manifest.CandidateCommand != "corvint" || manifest.OracleCommand != "python3 -m corvint_cli" ||
		!reflect.DeepEqual(manifest.Environment, map[string]string{
			"CORVINT_CACHE_DIR": "external-private-temp", "PYTHONPATH": "candidate-src",
		}) || manifest.RuntimeAuthority || len(manifest.Cases) != 2 {
		t.Fatalf("manifest header=%#v", manifest)
	}
	for _, item := range manifest.Cases {
		if item.ID == "" || item.Repository == "" || len(item.Commit) != 40 || len(item.Tree) != 40 ||
			!lowerHex64(item.StdoutSHA256) || !lowerHex64(item.StderrSHA256) || !lowerHex64(item.StdinSHA256) ||
			!lowerHex64(item.StatusBefore) || item.StatusBefore != item.StatusAfter || item.StdinSHA256 != digestBytes(nil) || item.StdoutBytes < 1 ||
			item.Exit != 0 || item.Mutation != "none" || !reflect.DeepEqual(item.Argv, []string{
			"query", "--task", authorityStartPrompt, "--limit", "1",
		}) {
			t.Fatalf("invalid manifest case: %#v", item)
		}
	}
}

func loadAuthorityParityManifest(t *testing.T) authorityParityManifest {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(moduleRoot(t), "conformance", "go-query-start-v0", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest authorityParityManifest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		t.Fatal(err)
	}
	return manifest
}

func TestAuthorityStartParityManifestReplay(t *testing.T) {
	t.Parallel()
	roots := map[string]string{
		"corvint":  os.Getenv("CORVINT_QUERY_PARITY_CORVINT_ROOT"),
		"beamfall": os.Getenv("CORVINT_QUERY_PARITY_BEAMFALL_ROOT"),
	}
	if roots["corvint"] == "" || roots["beamfall"] == "" {
		t.Skip("set CORVINT_QUERY_PARITY_CORVINT_ROOT and CORVINT_QUERY_PARITY_BEAMFALL_ROOT to replay pinned dogfood")
	}
	manifest := loadAuthorityParityManifest(t)
	t.Run("GPK-V0-028 replay", func(t *testing.T) {
		for _, item := range manifest.Cases {
			t.Run(item.Repository, func(t *testing.T) {
				root := filepath.Join(t.TempDir(), "fixture")
				clone := exec.Command("git", "clone", "-q", "--no-hardlinks", roots[item.Repository], root)
				if output, err := clone.CombinedOutput(); err != nil {
					t.Fatalf("clone: %v\n%s", err, output)
				}
				checkout := exec.Command("git", "-C", root, "checkout", "-q", "--detach", item.Commit)
				if output, err := checkout.CombinedOutput(); err != nil {
					t.Fatalf("checkout: %v\n%s", err, output)
				}
				if tree := gitOutput(t, root, "rev-parse", "HEAD^{tree}"); tree != item.Tree {
					t.Fatalf("tree=%s want=%s", tree, item.Tree)
				}
				before := statusSHA256(t, root)
				arguments := append([]string{"--root", root}, item.Argv...)
				candidate := exec.Command(os.Args[0], append([]string{"-test.run=^TestCandidateHelperProcess$", "--"}, arguments...)...)
				candidate.Env = append(os.Environ(), "CORVINT_HELPER_PROCESS=1")
				candidate.Stdin = bytes.NewReader(nil)
				candidateResult := execute(t, candidate)

				after := statusSHA256(t, root)
				if candidateResult.exit != item.Exit || len(candidateResult.stdout) != item.StdoutBytes ||
					digestBytes(candidateResult.stdout) != item.StdoutSHA256 || digestBytes(candidateResult.stderr) != item.StderrSHA256 ||
					before != item.StatusBefore || after != item.StatusAfter {
					t.Fatalf("manifest replay mismatch: exit=%d bytes=%d stdout=%s stderr=%s before=%s after=%s",
						candidateResult.exit, len(candidateResult.stdout), digestBytes(candidateResult.stdout),
						digestBytes(candidateResult.stderr), before, after)
				}
			})
		}
	})
}

func gitOutput(t *testing.T, root string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, arguments...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", arguments, err, output)
	}
	return string(bytes.TrimSpace(output))
}

func statusSHA256(t *testing.T, root string) string {
	t.Helper()
	command := exec.Command("git", "-C", root, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	return digestBytes(output)
}

func digestBytes(value []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(value))
}

func lowerHex64(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}
