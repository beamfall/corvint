// Copyright 2026 Russell Lewis
// Licensed under the Apache License, Version 2.0.

package main

// The seed universe. Every verification fixture and every frozen producer
// vector is derived from ONE deterministic temporary Git repository: pinned
// author, committer, and dates make its commit and blob OIDs a function of its
// content alone, so the bytes `ocm prepare` and `ocm link` write for it are
// reproducible on any host and can be frozen as data.
//
// Nothing is stubbed. The CEM is produced by the real two-phase workflow and
// the OCM by the real `prepare`, `link`, and `mark` producers, exactly as
// `script/dogfood-change.sh` drives them through the CLI.

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cem/workflow"
	"github.com/Beamfall/corvint/internal/lrfrepo"
)

const (
	intentPath = "docs/intent.md"
	claimsPath = "tests/claims_test.go"
	sourcePath = "pkg/thing.go"
	// MapPath is the first dogfood scope map path `script/dogfood-change.sh`
	// derives for one intent scope.
	MapPath = ".corvint/change.ocm.001.json"
)

// DeclaredObligation is one obligation of the seed intent scope and the
// disposition the real producer must give it: `unknown` with a reason, or
// `linked` through `ocm link` against the universe's one supported hunk and the
// obligation's own committed test claim.
type DeclaredObligation struct {
	ID          string `json:"id"`
	Disposition string `json:"disposition"`
	Reason      string `json:"reason,omitempty"`
}

// Universe is one built seed repository.
type Universe struct {
	Root   string
	Base   string
	Target string
	// Successor is an empty commit on top of Target with the identical tree,
	// so a caller naming it resolves and derives the same patch but fails the
	// OCM target binding.
	Successor string
	CEMRaw    []byte
	OCMRaw    []byte
}

// BuildUniverse materializes the seed repository for the declared obligations
// and drives the real producers over it. The returned cleanup removes it.
func BuildUniverse(obligations []DeclaredObligation) (*Universe, func(), error) {
	workspace, err := os.MkdirTemp("", "ocm-v0-fixture-")
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() { os.RemoveAll(workspace) }
	u, err := buildUniverse(workspace, obligations)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	return u, cleanup, nil
}

func buildUniverse(workspace string, obligations []DeclaredObligation) (*Universe, error) {
	resolved, err := filepath.EvalSymlinks(workspace)
	if err != nil {
		return nil, err
	}
	root := filepath.Join(resolved, "repo")
	home := filepath.Join(resolved, "home")
	for _, dir := range []string{root, home} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	git := gitRunner{root: root, home: home}
	if _, err := git.run("init", "-q", "-b", "main"); err != nil {
		return nil, err
	}
	base, err := commitBaseTree(git, root, obligations)
	if err != nil {
		return nil, err
	}
	target, err := commitTargetTree(git, root)
	if err != nil {
		return nil, err
	}
	cemRaw, err := buildCEM(root, base, target)
	if err != nil {
		return nil, err
	}
	ocmRaw, err := buildOCM(root, base, target, obligations)
	if err != nil {
		return nil, err
	}
	successor, err := commitSuccessor(git)
	if err != nil {
		return nil, err
	}
	return &Universe{Root: root, Base: base, Target: target, Successor: successor, CEMRaw: cemRaw, OCMRaw: ocmRaw}, nil
}

func commitBaseTree(git gitRunner, root string, obligations []DeclaredObligation) (string, error) {
	files := map[string]string{
		intentPath: intentDocument(obligations),
		claimsPath: claimsDocument(obligations),
		sourcePath: "package pkg\n\nfunc Thing() int { return 1 }\n",
	}
	for path, body := range files {
		if err := writeFile(root, path, body); err != nil {
			return "", err
		}
	}
	return commitAll(git, "base")
}

func commitTargetTree(git gitRunner, root string) (string, error) {
	if err := writeFile(root, sourcePath, "package pkg\n\nfunc Thing() int { return 2 }\n"); err != nil {
		return "", err
	}
	return commitAll(git, "target")
}

func commitSuccessor(git gitRunner) (string, error) {
	if _, err := git.run("commit", "-q", "--allow-empty", "-m", "successor"); err != nil {
		return "", err
	}
	return git.run("rev-parse", "HEAD")
}

func commitAll(git gitRunner, message string) (string, error) {
	if _, err := git.run("add", "-A"); err != nil {
		return "", err
	}
	if _, err := git.run("commit", "-qm", message); err != nil {
		return "", err
	}
	return git.run("rev-parse", "HEAD")
}

// intentDocument is the pinned intent scope: one `## Requirements` section in
// the frozen inline-code requirement-line form, one line per obligation.
func intentDocument(obligations []DeclaredObligation) string {
	var out strings.Builder
	out.WriteString("# Intent\n\n## Requirements\n\n")
	for _, obligation := range obligations {
		out.WriteString("- `" + obligation.ID + "`: Preserve the recorded invariant under review.\n")
	}
	out.WriteString("\n## Next\n")
	return out.String()
}

// claimsDocument commits one Go table-case claim per obligation, anchored on
// the exact obligation ID, which is the anchor `OCM-V0-005` requires.
func claimsDocument(obligations []DeclaredObligation) string {
	var out strings.Builder
	out.WriteString("package tests\n\nimport \"testing\"\n")
	for index, obligation := range obligations {
		out.WriteString(fmt.Sprintf(
			"\nfunc TestClaim%s(t *testing.T) {\n\t_ = []struct{ name string }{{name: %q}}\n}\n",
			claimSuffix(index), obligation.ID))
	}
	return out.String()
}

// claimSuffix names one claim's test function with letters only.
func claimSuffix(index int) string {
	return string(rune('A' + index))
}

// claimSelector is the extractor-owned selector `ocm link --claim` resolves for
// one obligation's table case: the test name, then the case slug, which drops
// digit-only words.
func claimSelector(index int, obligationID string) string {
	words := []string{}
	for _, word := range strings.Split(strings.ToLower(obligationID), "-") {
		if strings.Trim(word, "0123456789") != "" {
			words = append(words, word)
		}
	}
	return "test:TestClaim" + claimSuffix(index) + "/case:" + strings.Join(words, "-")
}

// buildCEM runs the real two-phase workflow: prepare, then cite the one hunk
// against the intent scope so it becomes `supported`.
func buildCEM(root, base, target string) ([]byte, error) {
	session, err := workflow.Open(root)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	if _, err := session.Prepare(ctx, workflow.PrepareOptions{Base: base, Target: target}); err != nil {
		return nil, fmt.Errorf("cem prepare: %w", err)
	}
	if _, err := session.Cite(ctx, workflow.CiteOptions{
		MapPath: wire.ExcludedCEMPath, Hunk: "1",
		EvidencePath: intentPath, Lines: "5:5", Relation: "specification",
	}); err != nil {
		return nil, fmt.Errorf("cem cite: %w", err)
	}
	return os.ReadFile(filepath.Join(root, filepath.FromSlash(wire.ExcludedCEMPath)))
}

// buildOCM drives `ocm prepare` with the dogfood argv, then `link` or `mark`
// per declared obligation, and returns the map bytes the producers wrote.
func buildOCM(root, base, target string, obligations []DeclaredObligation) ([]byte, error) {
	ctx := context.Background()
	if _, err := lrfrepo.PrepareOCM(ctx, root, lrfrepo.PrepareOptions{
		MapPath: MapPath, CEMPath: wire.ExcludedCEMPath, IntentPath: intentPath,
		ExpectedBase: base, Target: target,
	}); err != nil {
		return nil, fmt.Errorf("ocm prepare: %w", err)
	}
	for index, obligation := range obligations {
		if err := applyDisposition(ctx, root, base, target, index, obligation); err != nil {
			return nil, err
		}
	}
	return os.ReadFile(filepath.Join(root, filepath.FromSlash(MapPath)))
}

func applyDisposition(ctx context.Context, root, base, target string, index int, obligation DeclaredObligation) error {
	switch obligation.Disposition {
	case "linked":
		_, err := lrfrepo.LinkOCM(ctx, root, lrfrepo.LinkOptions{
			MapPath: MapPath, CEMPath: wire.ExcludedCEMPath, Obligation: obligation.ID,
			Hunks: []string{"1"}, TestPath: claimsPath, Claims: []string{claimSelector(index, obligation.ID)},
			ExpectedBase: base, Target: target,
		})
		if err != nil {
			return fmt.Errorf("ocm link %s: %w", obligation.ID, err)
		}
	case "unknown":
		// `prepare` already leaves every row `unknown`/`unassessed`.
		if obligation.Reason == "unassessed" {
			return nil
		}
		_, err := lrfrepo.MarkOCM(ctx, root, lrfrepo.MarkOptions{
			MapPath: MapPath, Obligation: obligation.ID, Reason: obligation.Reason,
		})
		if err != nil {
			return fmt.Errorf("ocm mark %s: %w", obligation.ID, err)
		}
	default:
		return fmt.Errorf("obligation %s declares an unsupported disposition %q", obligation.ID, obligation.Disposition)
	}
	return nil
}

type gitRunner struct{ root, home string }

func (g gitRunner) run(args ...string) (string, error) {
	command := exec.Command("git", args...)
	command.Dir = g.root
	command.Env = gitEnvironment(g.home)
	out, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %v: %w\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out)), nil
}

// gitEnvironment pins identity and dates so the seed repository's OIDs are a
// function of its content alone.
func gitEnvironment(home string) []string {
	return append(os.Environ(),
		"GIT_CONFIG_NOSYSTEM=1", "HOME="+home, "XDG_CONFIG_HOME="+home,
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid",
		"GIT_AUTHOR_DATE=2000-01-01T00:00:00+0000", "GIT_COMMITTER_DATE=2000-01-01T00:00:00+0000")
}

func writeFile(root, path, content string) error {
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, []byte(content), 0o644)
}
