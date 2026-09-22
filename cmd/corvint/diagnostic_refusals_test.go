package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// refusalRun is one forced refusal of a converted family: the process outcome plus the complete
// envelope the CLI must emit for it.
type refusalRun struct {
	exit           int
	stdout, stderr string
	want           map[string]any
}

func refusalEnvelope(code, message string, subject map[string]any, evidence []any, fixes []any, terminal string) map[string]any {
	envelope := map[string]any{
		"code": code, "error": message, "ok": false,
		"subject": subject, "evidence": evidence, "supported_fixes": fixes,
	}
	if terminal != "" {
		envelope["terminal"] = terminal
	}
	return envelope
}

func subjectOf(kind, value string) map[string]any {
	return map[string]any{"kind": kind, "value": value}
}

func evidenceOf(pairs ...string) []any {
	evidence := []any{}
	for index := 0; index < len(pairs); index += 2 {
		evidence = append(evidence, map[string]any{"name": pairs[index], "value": pairs[index+1]})
	}
	return evidence
}

func fixesOf(identifiers ...string) []any {
	fixes := []any{}
	for _, identifier := range identifiers {
		fixes = append(fixes, identifier)
	}
	return fixes
}

func cliRun(want map[string]any, arguments ...string) func(*testing.T) refusalRun {
	return func(t *testing.T) refusalRun {
		exit, stdout, stderr := runCLI(t, arguments...)
		return refusalRun{exit: exit, stdout: stdout, stderr: stderr, want: want}
	}
}

func platformEnvelope(code, message string) map[string]any {
	return refusalEnvelope(code, message, subjectOf("host-capability", "native-platform"), evidenceOf("platform", "windows"), fixesOf(), "unsupported-platform")
}

func impactPathSuffixEnvelope(path string) map[string]any {
	return refusalEnvelope("unsupported-impact-path-suffix", "native Go impact requires a suffix admitted by the repository index", subjectOf("value", path), evidenceOf(), fixesOf("impact.omit-unadmitted-path"), "")
}

var validProofDocument = `{"tool":"prove","profile":"` + proveProfile + `","proof":{"counts":{"history-consistent":{"PASS":1}},"rows":[{"falsifier":"history-consistent","falsified":"PASS"}]}}`

func observeRun(stdin, message string) func(*testing.T) refusalRun {
	return func(t *testing.T) refusalRun {
		var stdout, stderr bytes.Buffer
		exit := runContext(context.Background(), []string{"--root", cliRepository(t), "prove-observe"}, strings.NewReader(stdin), &stdout, &stderr)
		want := refusalEnvelope("invalid-proof-document", message, subjectOf("request", "stdin"), evidenceOf(), fixesOf("prove-observe.pipe-prove-packet"), "")
		return refusalRun{exit: exit, stdout: stdout.String(), stderr: stderr.String(), want: want}
	}
}

func resolvedTestRoot(t *testing.T, directory string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(directory)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

// TestConvertedRefusalDiagnostics forces one refusal per site converted after the working-tree
// impact family and asserts, through the CLI emission path, the complete DRC-V0-001..005 members,
// the DRC-V0-006 code and error bytes ahead of them, and the DRC-V0-010 exit, envelope and stdout.
func TestConvertedRefusalDiagnostics(t *testing.T) {
	t.Parallel()
	cases := map[string]func(*testing.T) refusalRun{
		"explicit root without .git": func(t *testing.T) refusalRun {
			directory := t.TempDir()
			root := resolvedTestRoot(t, directory)
			want := refusalEnvelope("invalid-arguments", "not a Git repository: "+root, subjectOf("value", directory), evidenceOf("root", root), fixesOf("cli.use-git-repository-root"), "")
			return cliRun(want, "--root", directory, "affected")(t)
		},
		"query root without .git": func(t *testing.T) refusalRun {
			root := resolvedTestRoot(t, t.TempDir())
			want := refusalEnvelope("invalid-arguments", "not a Git repository: "+root, subjectOf("value", root), evidenceOf(), fixesOf("cli.use-git-repository-root"), "")
			return cliRun(want, "--root", root, "query", "--task", "where is Split")(t)
		},
		"query platform": func(t *testing.T) refusalRun {
			_, err := parseQueryArgumentsForPlatform(options{}, []string{"--task", "where is Split"}, "windows")
			var stderr bytes.Buffer
			emitError(&stderr, err)
			return refusalRun{exit: 2, stderr: stderr.String(), want: platformEnvelope("unsupported-query-platform", "native Go authority-start query is qualified only on Darwin and Linux")}
		},
		"feature platform": func(t *testing.T) refusalRun {
			_, err := parseFeatureArgumentsForPlatform(options{featureLimit: 10}, []string{"a-first"}, "windows")
			var stderr bytes.Buffer
			emitError(&stderr, err)
			return refusalRun{exit: 2, stderr: stderr.String(), want: platformEnvelope("unsupported-feature-platform", "native Go feature is qualified only on Darwin and Linux")}
		},
		"genesis platform": func(t *testing.T) refusalRun {
			var stdout, stderr bytes.Buffer
			exit := runActivationForPlatform(context.Background(), filepath.Join(t.TempDir(), "missing"), "init", nil, "windows", &stdout, &stderr)
			return refusalRun{exit: exit, stdout: stdout.String(), stderr: stderr.String(), want: platformEnvelope("unsupported-genesis-platform", "native Go Genesis inventory is qualified only on Darwin and Linux")}
		},
		"impact budget option": func(t *testing.T) refusalRun {
			want := refusalEnvelope("unsupported-impact-option", "--budget-bytes is not implemented for native Go impact", subjectOf("argument", "--budget-bytes"), evidenceOf(), fixesOf("impact.omit-budget-bytes"), "")
			return cliRun(want, "--root", impactCLIRepository(t), "impact", "pkg/main.go", "--budget-bytes", "1024")(t)
		},
		"batch without snapshot": func(t *testing.T) refusalRun {
			exit, stdout, stderr := runBatchForTest(t, batchRepository(t), `{"operations":[{"id":"a","verb":"query","task":"does Split keep empty demux keys"}]}`)
			want := refusalEnvelope("unsupported-batch-snapshot", "no index snapshot matches this tree and this binary: run corvint index", subjectOf("repository-state", "index-snapshot"), evidenceOf(), fixesOf("index.write-snapshot"), "")
			return refusalRun{exit: exit, stdout: string(stdout), stderr: string(stderr), want: want}
		},
		"affected without git": func(t *testing.T) refusalRun {
			stdout, stderr, exit := runCandidateWithEnvironment(t, "", []string{"PATH=" + t.TempDir()}, "--root", affectedFixtureRepository(t), "affected")
			want := refusalEnvelope("unsupported-affected-status", "git executable is unavailable", subjectOf("host-capability", "git-executable"), evidenceOf(), fixesOf("host.put-git-on-path"), "")
			return refusalRun{exit: exit, stdout: string(stdout), stderr: stderr, want: want}
		},
		"affected unknown base": func(t *testing.T) refusalRun {
			base := strings.Repeat("0", 40)
			want := refusalEnvelope("unsupported-affected-revision", "base "+base+" is not a commit in this repository", subjectOf("value", base), evidenceOf(), fixesOf("affected.use-commit-base", "affected.omit-base"), "")
			return cliRun(want, "--root", affectedFixtureRepository(t), "affected", "--base", base)(t)
		},
		"affected without HEAD": func(t *testing.T) refusalRun {
			root := t.TempDir()
			affectedGit(t, root, "init", "-q")
			want := refusalEnvelope("unsupported-affected-revision", "repository has no resolvable HEAD commit", subjectOf("repository-state", "head-commit"), evidenceOf(), fixesOf("git.create-head-commit"), "")
			return cliRun(want, "--root", root, "affected")(t)
		},
		"prove without git": func(t *testing.T) refusalRun {
			root := checkpointRepository(t)
			_, stdout, stderr, exit := runProveCLIEnvironment(t, []string{"PATH=" + t.TempDir()}, root, "--checkpoint", writeCheckpoint(t, checkpointInput(t, root, "AGENTS.md")))
			want := refusalEnvelope("unsupported-prove-revision", "git executable is not available", subjectOf("host-capability", "git-executable"), evidenceOf(), fixesOf("host.put-git-on-path"), "")
			return refusalRun{exit: exit, stdout: string(stdout), stderr: stderr, want: want}
		},
		"checkpoint without HEAD": func(t *testing.T) refusalRun {
			root := t.TempDir()
			affectedGit(t, root, "init", "-q")
			_, stdout, stderr, exit := runProveCLIContext(t, context.Background(), root, "--checkpoint", filepath.Join(t.TempDir(), "absent"))
			want := refusalEnvelope("unsupported-prove-revision", "repository has no resolvable HEAD commit and tree", subjectOf("repository-state", "head-commit"), evidenceOf(), fixesOf("git.create-head-commit"), "")
			return refusalRun{exit: exit, stdout: string(stdout), stderr: stderr, want: want}
		},
		"prove range past its byte bound": func(t *testing.T) refusalRun {
			root, base := proveBoundRepository(t)
			ctx := proveBoundsContext(func(bounds *proveBounds) { bounds.rangeChangeBytes = 4 })
			_, stdout, stderr, exit := runProveCLIContext(t, ctx, root, "--base", base)
			want := refusalEnvelope("unsupported-prove-history", "the range's changed paths exceed the byte bound", subjectOf("value", base), evidenceOf(), fixesOf("prove.use-bounded-base"), "")
			return refusalRun{exit: exit, stdout: string(stdout), stderr: stderr, want: want}
		},
		"checkpoint object format": func(t *testing.T) refusalRun {
			root := checkpointRepository(t)
			document := checkpointInput(t, root, "AGENTS.md")
			repository := document["repository"].(map[string]any)
			repository["object_format"], repository["base_commit"], repository["base_tree"] = "sha256", strings.Repeat("0", 64), strings.Repeat("0", 64)
			document["handles"].([]any)[0].(map[string]any)["blob_hash"] = strings.Repeat("0", 64)
			file := writeCheckpoint(t, document)
			ctx := proveBoundsContext(func(bounds *proveBounds) { bounds.blobBytes = 0 })
			_, stdout, stderr, exit := runProveCLIContext(t, ctx, root, "--checkpoint", file)
			want := refusalEnvelope("object-format-mismatch", "checkpoint object format differs from the current repository", subjectOf("artifact", file),
				evidenceOf("repository-object-format", "sha1", "checkpoint-object-format", "sha256"), fixesOf("prove.use-repository-object-format-checkpoint"), "")
			return refusalRun{exit: exit, stdout: string(stdout), stderr: stderr, want: want}
		},
		"surprise over a dirty worktree": func(t *testing.T) refusalRun {
			root := cliRepository(t)
			if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# edited\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			want := refusalEnvelope("unsupported-surprise-dirty-worktree", "touch-set surprise compares committed revisions only; the worktree has uncommitted changes",
				subjectOf("repository-state", "tracked-worktree-changes"), evidenceOf("tracked-status-entries", "1"), fixesOf("git.clean-tracked-worktree"), "")
			return cliRun(want, "--root", root, "surprise", "--task", "t", "--base", "HEAD", "--target", "HEAD")(t)
		},
		"surprise unknown revision": func(t *testing.T) refusalRun {
			want := refusalEnvelope("unsupported-surprise-revision", "revision does not resolve to a commit: no-such-revision", subjectOf("value", "no-such-revision"), evidenceOf(), fixesOf("surprise.use-commit-revision"), "")
			return cliRun(want, "--root", cliRepository(t), "surprise", "--task", "t", "--base", "no-such-revision", "--target", "HEAD")(t)
		},
		"impact platform": func(t *testing.T) refusalRun {
			_, err := parseImpactArgumentsForPlatform(options{}, []string{"pkg/main.go"}, "windows")
			var stderr bytes.Buffer
			emitError(&stderr, err)
			return refusalRun{exit: 2, stderr: stderr.String(), want: platformEnvelope("unsupported-impact-platform", "native Go impact is qualified only on Darwin and Linux")}
		},
		"impact path suffix": func(t *testing.T) refusalRun {
			return cliRun(impactPathSuffixEnvelope("README.csv"), "--root", impactCLIRepository(t), "impact", "README.csv")(t)
		},
		"prove range past its diff bound": func(t *testing.T) refusalRun {
			root, base := proveBoundRepository(t)
			ctx := proveBoundsContext(func(bounds *proveBounds) { bounds.rangeDiffBytes = 16 })
			_, stdout, stderr, exit := runProveCLIContext(t, ctx, root, "--base", base)
			want := refusalEnvelope("unsupported-prove-history", "the range's diff exceeds the byte bound", subjectOf("value", base), evidenceOf(), fixesOf("prove.use-diff-bounded-base"), "")
			return refusalRun{exit: exit, stdout: string(stdout), stderr: stderr, want: want}
		},
		"attest key missing": func(t *testing.T) refusalRun {
			root, base := proveCEMRepository(t)
			key := filepath.Join(t.TempDir(), "missing.pem")
			_, stdout, stderr, exit := runProveCLI(t, root, "--cem", ".corvint/change.cem.json", "--expected-base", base, "--target", "HEAD", "--attest-key", key)
			want := refusalEnvelope("attest-key-unavailable", "cannot read an Ed25519 PKCS#8 private key from "+key, subjectOf("value", key), evidenceOf(), fixesOf("attest.use-ed25519-private-key"), "")
			return refusalRun{exit: exit, stdout: string(stdout), stderr: stderr, want: want}
		},
		"attest public key missing": func(t *testing.T) refusalRun {
			root, envelopePath, _ := emitCEMAttestation(t)
			key := filepath.Join(t.TempDir(), "missing.pub")
			_, stdout, stderr, exit := runProveCLI(t, root, "--verify-cem-attestation", envelopePath, "--attest-public-key", key, "--cem", ".corvint/change.cem.json")
			want := refusalEnvelope("attest-public-key-unavailable", "cannot read an Ed25519 PKIX public key from "+key, subjectOf("value", key), evidenceOf(), fixesOf("attest.use-ed25519-public-key"), "")
			return refusalRun{exit: exit, stdout: string(stdout), stderr: stderr, want: want}
		},
		"attest envelope missing": func(t *testing.T) refusalRun {
			root, _, publicKeyPath := emitCEMAttestation(t)
			envelope := filepath.Join(t.TempDir(), "missing.dsse.json")
			_, stdout, stderr, exit := runProveCLI(t, root, "--verify-cem-attestation", envelope, "--attest-public-key", publicKeyPath, "--cem", ".corvint/change.cem.json")
			want := refusalEnvelope("attest-envelope-unavailable", "cannot read the DSSE envelope from "+envelope, subjectOf("value", envelope), evidenceOf(), fixesOf("attest.use-readable-envelope"), "")
			return refusalRun{exit: exit, stdout: string(stdout), stderr: stderr, want: want}
		},
		"cem map outside the repository": func(t *testing.T) refusalRun {
			root, base := proveCEMRepository(t)
			_, stdout, stderr, exit := runProveCLI(t, root, "--cem", "../outside.cem.json", "--expected-base", base, "--target", "HEAD")
			want := refusalEnvelope("map-unavailable", "map path must be repository-relative", subjectOf("value", "../outside.cem.json"), evidenceOf(), fixesOf("cem.use-repository-relative-map-path"), "")
			return refusalRun{exit: exit, stdout: string(stdout), stderr: stderr, want: want}
		},
		"cem map missing": func(t *testing.T) refusalRun {
			root, base := proveCEMRepository(t)
			_, stdout, stderr, exit := runProveCLI(t, root, "--cem", ".corvint/missing.cem.json", "--expected-base", base, "--target", "HEAD")
			want := refusalEnvelope("map-unavailable", "cannot read CEM map", subjectOf("value", ".corvint/missing.cem.json"), evidenceOf(), fixesOf("cem.use-readable-map"), "")
			return refusalRun{exit: exit, stdout: string(stdout), stderr: stderr, want: want}
		},
		"proof document oversized":       observeRun(`{"tool":"prove","pad":"`+strings.Repeat("x", maxProofDocumentBytes)+`"}`, "proof document exceeds 8 MiB"),
		"proof document not JSON":        observeRun("{", "stdin is not a JSON object"),
		"proof document another tool":    observeRun(strings.Replace(validProofDocument, `"tool":"prove"`, `"tool":"query"`, 1), "stdin is not a "+proveProfile+" document"),
		"proof document without rows":    observeRun(`{"tool":"prove","profile":"`+proveProfile+`","proof":{"counts":{}}}`, "proof.rows or proof.counts is missing"),
		"proof document unknown verdict": observeRun(strings.ReplaceAll(validProofDocument, "PASS", "MAYBE"), "proof.rows carries an unknown falsifier or verdict"),
		"proof document inflated counts": observeRun(strings.Replace(validProofDocument, `"PASS":1`, `"PASS":40`, 1), "proof.counts does not match proof.rows"),
	}
	for name, force := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			outcome := force(t)
			if outcome.exit != 2 || outcome.stdout != "" {
				t.Fatalf("exit=%d stdout=%q stderr=%q", outcome.exit, outcome.stdout, outcome.stderr)
			}
			prefix := "{\"code\": " + pythonJSONString(outcome.want["code"].(string)) + ", \"error\": " + pythonJSONString(outcome.want["error"].(string)) + ", \"evidence\": ["
			if !strings.HasPrefix(outcome.stderr, prefix) {
				t.Fatalf("code and error bytes changed:\n got %q\nwant prefix %q", outcome.stderr, prefix)
			}
			var got map[string]any
			if err := json.Unmarshal([]byte(outcome.stderr), &got); err != nil {
				t.Fatalf("stderr is not one JSON envelope: %v: %q", err, outcome.stderr)
			}
			if !reflect.DeepEqual(got, outcome.want) {
				t.Fatalf("envelope:\n got %#v\nwant %#v", got, outcome.want)
			}
		})
	}
}

// TestImpactPathSuffixDiagnosticReachesEveryLayer checks DRC-V0-010 at the two layers that re-emit
// the impact admission refusal: a batch operation row carries the same members beside its
// SBQ-V0-004 message and code, and a harness file-change event emits the full envelope.
func TestImpactPathSuffixDiagnosticReachesEveryLayer(t *testing.T) {
	t.Parallel()
	root := batchRepository(t)
	runIndexForTest(t, root, false)
	code, stdout, stderr := runBatchForTest(t, root, `{"operations":[{"id":"a","verb":"impact","paths":["README.csv"]}]}`)
	if code != 0 {
		t.Fatalf("batch exit=%d stderr=%s", code, stderr)
	}
	document := decodeBatchDocument(t, stdout)
	if len(document.Operations) != 1 || document.Operations[0].OK {
		t.Fatalf("operations = %s", stdout)
	}
	var row map[string]any
	if err := json.Unmarshal(document.Operations[0].Error, &row); err != nil {
		t.Fatal(err)
	}
	want := impactPathSuffixEnvelope("README.csv")
	delete(want, "ok")
	want["message"] = want["error"]
	delete(want, "error")
	if !reflect.DeepEqual(row, want) {
		t.Fatalf("batch row:\n got %#v\nwant %#v", row, want)
	}
	var harnessOut, harnessErr bytes.Buffer
	exit := runContext(context.Background(), cliArguments(root, "file-change"), strings.NewReader(`{"paths":["README.csv"]}`), &harnessOut, &harnessErr)
	var envelope map[string]any
	if err := json.Unmarshal(harnessErr.Bytes(), &envelope); exit != 2 || harnessOut.Len() != 0 || err != nil {
		t.Fatalf("harness exit=%d stdout=%q stderr=%q err=%v", exit, &harnessOut, &harnessErr, err)
	}
	if want := impactPathSuffixEnvelope("README.csv"); !reflect.DeepEqual(envelope, want) {
		t.Fatalf("harness envelope:\n got %#v\nwant %#v", envelope, want)
	}
}
