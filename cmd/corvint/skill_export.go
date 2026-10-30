package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/skillexport"
	"github.com/Beamfall/corvint/internal/trace"
	"github.com/Beamfall/corvint/internal/tracerecordrepo"
)

type skillExportInvocation struct {
	root string
	out  string
}

// skillExportInvoked reports whether the argument vector selects the
// skill-export verb, sharing calibrate's leading `--root` handling.
func skillExportInvoked(arguments []string) bool {
	_, index := calibrateRoot(arguments)
	return index < len(arguments) && arguments[index] == "skill-export"
}

func parseSkillExportInvocation(arguments []string) (skillExportInvocation, error) {
	root, index := calibrateRoot(arguments)
	invocation := skillExportInvocation{}
	if root == "" {
		resolved, err := normalizeRoot(".")
		if err != nil {
			return invocation, argumentError("cannot resolve current directory")
		}
		invocation.root = resolved
	} else {
		resolved, err := resolveExplicitRoot(root)
		if err != nil {
			return invocation, err
		}
		invocation.root = resolved
	}
	for index++; index < len(arguments); index++ {
		name, value, inline := strings.Cut(arguments[index], "=")
		if name != "--out" {
			return invocation, argumentError("unrecognized arguments: " + arguments[index])
		}
		if !inline {
			if index+1 >= len(arguments) || argparseOptionLike(arguments[index+1]) {
				return invocation, argumentError("argument --out: expected one argument")
			}
			value, index = arguments[index+1], index+1
		}
		invocation.out = value
	}
	if invocation.out == "" {
		return invocation, argumentError("argument --out is required")
	}
	out, err := filepath.Abs(invocation.out)
	if err != nil {
		return invocation, argumentError("argument --out: " + err.Error())
	}
	invocation.out = resolveExistingPrefix(out)
	state := resolveExistingPrefix(filepath.Join(invocation.root, ".corvint"))
	if invocation.out == state || strings.HasPrefix(invocation.out, state+string(filepath.Separator)) {
		return invocation, argumentError("argument --out must not point inside the trace state directory " + state)
	}
	return invocation, nil
}

// resolveExistingPrefix resolves symlinks through the nearest existing
// ancestor of path so the --out guard compares against the resolved root.
func resolveExistingPrefix(path string) string {
	for prefix := path; ; prefix = filepath.Dir(prefix) {
		if resolved, err := filepath.EvalSymlinks(prefix); err == nil {
			rest, _ := filepath.Rel(prefix, path)
			return filepath.Join(resolved, rest)
		}
		if prefix == filepath.Dir(prefix) {
			return path
		}
	}
}

type skillExportManifestEntry struct {
	Name       string `json:"name"`
	Digest     string `json:"admission_evidence_digest"`
	Evaluation string `json:"evaluation"`
}

type skillExportManifest struct {
	Exported   int                        `json:"exported"`
	Out        string                     `json:"out"`
	TraceState string                     `json:"trace_state"`
	Skills     []skillExportManifestEntry `json:"skills"`
}

// runSkillExport writes one Agent Skills directory per admitted learned rule
// under the operator-named --out directory and prints a manifest. It reads
// the pinned local trace store and the committed index only; repository and
// trace state are never written (LTA-V0-006).
func runSkillExport(ctx context.Context, arguments []string, stdout, stderr io.Writer) int {
	invocation, err := parseSkillExportInvocation(arguments)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	type recorded struct {
		records []trace.Record
		state   string
	}
	read, err := overSnapshot(
		func() *contextindex.Index { return deferredSnapshotIndex(ctx, invocation.root) },
		func() *contextindex.Index { return snapshotIndex(ctx, invocation.root) },
		func() (*contextindex.Index, error) { return contextindex.BuildEval(ctx, invocation.root) },
		func(index *contextindex.Index) (recorded, error) {
			records, state, err := tracerecordrepo.Read(ctx, invocation.root, index)
			if err != nil {
				return recorded{}, mapRepositoryQueryTraceError(err)
			}
			return recorded{records, state}, nil
		},
	)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	skills, err := skillexport.Export(read.records)
	if err != nil {
		emitError(stderr, &contextindex.Error{Code: "unsupported-query-trace-state", Message: err.Error(), Cause: err})
		return 2
	}
	if err := writeSkills(invocation.out, skills); err != nil {
		emitError(stderr, argumentError("argument --out: "+err.Error()))
		return 2
	}
	manifest := skillExportManifest{Exported: len(skills), Out: invocation.out, TraceState: read.state, Skills: make([]skillExportManifestEntry, 0, len(skills))}
	for _, skill := range skills {
		manifest.Skills = append(manifest.Skills, skillExportManifestEntry{Name: skill.Name, Digest: skill.Digest, Evaluation: skillexport.Evaluation})
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	fmt.Fprintln(stdout, string(encoded))
	return 0
}

func writeSkills(out string, skills []skillexport.Skill) error {
	for _, skill := range skills {
		for name, content := range skill.Files {
			path := filepath.Join(out, skill.Name, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(path, content, 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}

const skillExportHelp = `
Learned-rule skill export (experimental, LTA-V0-006 to LTA-V0-008), read-only:

  corvint [--root PATH] skill-export --out DIR

Writes one Agent Skills directory per admitted learned trace (a stored row
with outcome passed that the trace reader re-validated) under DIR:
DIR/<name>/SKILL.md carries YAML frontmatter (name, description) and a short
body; DIR/<name>/references/trace.md carries the full paths and commands.
Each document names the admission evidence digest (sha256 of the stored row
bytes) and the evaluation result verbatim; V0 records none, so it reads
NOT_RECORDED. The same admitted rows export identical bytes on every run.

  --out DIR  Operator-named output directory; created if missing. It must not
             point inside the repository's .corvint state directory.

It reads the pinned local trace store and the committed index only and never
writes repository or trace state. Standard output carries a JSON manifest of
the exported names and digests.
`
