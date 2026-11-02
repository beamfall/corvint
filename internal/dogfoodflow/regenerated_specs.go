// SPDX-License-Identifier: Apache-2.0
package dogfoodflow

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
)

// generatedSpecFiles are the files script/gen-spec-requirements.sh --all writes from the staged
// specs (SRG-V0-018). Two lanes that each add requirement rows conflict here, and only here, even
// when their specs merge cleanly (V1-1467).
var generatedSpecFiles = []string{"docs/specs/REQUIREMENTS.tsv", "docs/specs/INDEX.json", "docs/specs/README.md"}

// regeneratedConflict is the one generated file whose conflict a regeneration may resolve: it is
// generated whole from the specs. INDEX.json and README.md are projections onto a kept file, whose
// conflicting hand edits a regeneration from one side could drop, so their conflicts stay unbound.
const regeneratedConflict = "docs/specs/REQUIREMENTS.tsv"

// specRegenerationTimeout bounds one merge's generator replay (DCW-V0-1924).
const specRegenerationTimeout = 5 * time.Minute

// regeneratedSpecs reports whether merge is ort's merge of its parents with every conflict in a
// generated spec file, resolved by regenerating those files from the merged specs (DCW-V0-1924,
// V1-1467): conflicted is ort's own `merge-tree -z` result, every path ort conflicted on is
// regeneratedConflict, every path where the merge tree differs from ort's written tree is a
// generatedSpecFiles entry, and a
// private clone of ort's written tree, running its script/gen-spec-requirements.sh --all,
// stages a tree equal to the merge tree. Nothing in the repository, its index or its refs is
// written. Any doubt (a parse, a replay or a timeout failure) is false: the merge stays unbound.
func (b *batch) regeneratedSpecs(merge, tree, conflicted string) bool {
	fields := strings.Split(conflicted, "\x00")
	written := fields[0]
	if !fullRevision.MatchString(written) {
		return false
	}
	paths := 0
	for _, entry := range fields[1:] {
		if entry == "" {
			break
		}
		_, path, ok := strings.Cut(entry, "\t")
		if !ok || path != regeneratedConflict {
			return false
		}
		paths++
	}
	if paths == 0 {
		return false
	}
	differing, status := b.scratchGit("diff-tree", "-r", "--name-only", "--no-renames", written, tree)
	if status != 0 {
		return false
	}
	for _, path := range nonEmpty(strings.Split(chomp(differing), "\n")) {
		if !slices.Contains(generatedSpecFiles, path) {
			return false
		}
	}
	if !b.gitSucceeds("cat-file", "-e", merge+":script/gen-spec-requirements.sh") {
		return false
	}
	return b.replaySpecs(written, tree)
}

// replaySpecs loads ort's merged inputs alone in a private clone, including the preserved README
// prose and INDEX fields. Replaying from the candidate would let hand edits validate themselves.
// The complete staged tree comparison includes file modes (DCW-V0-1924).
func (b *batch) replaySpecs(written, tree string) bool {
	if b.scratch == "" {
		b.scratchGit("rev-parse", "--git-dir")
	}
	// Refuse non-regular candidate paths before checking out or writing anything. In particular,
	// a symlink must never turn a replay write into a write to the original checkout.
	for _, revision := range []string{written, tree} {
		entries, status := b.scratchGit(append([]string{"ls-tree", revision, "--"}, generatedSpecFiles...)...)
		rows := nonEmpty(strings.Split(chomp(entries), "\n"))
		if status != 0 || len(rows) != len(generatedSpecFiles) {
			return false
		}
		for _, row := range rows {
			if !strings.HasPrefix(row, "100644 blob ") && !strings.HasPrefix(row, "100755 blob ") {
				return false
			}
		}
	}
	clone := filepath.Join(b.scratch, "specs-replay")
	out := filepath.Join(b.scratch, "specs-replay-out")
	defer os.RemoveAll(clone)
	defer os.RemoveAll(out)
	if b.replayGit(b.scratch, "init", "-q", "--template=", clone) != nil ||
		os.WriteFile(filepath.Join(clone, ".git", "objects", "info", "alternates"), []byte(filepath.Join(b.scratch, "repository.git", "objects")+"\n"), 0o600) != nil ||
		os.Mkdir(out, 0o700) != nil {
		return false
	}
	if b.replayGit(clone, "read-tree", "--reset", "-u", written) != nil {
		return false
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		return false
	}
	run := procgroup.Run(b.ctx, procgroup.Spec{
		Argv: []string{bash, "script/gen-spec-requirements.sh", "--all", out}, Dir: clone,
		Env:     replayEnvironment(os.Environ(), clone, b.scratch),
		Timeout: specRegenerationTimeout, ShutdownTimeout: 5 * time.Second,
		OutputLimit: 64 << 10, OverflowPolicy: procgroup.OverflowTruncate,
	})
	b.checkpoint()
	if run.TimedOut || run.Err != nil || !run.ExitObserved || run.ExitStatus != 0 {
		return false
	}
	for _, path := range generatedSpecFiles {
		destination := filepath.Join(clone, filepath.FromSlash(path))
		source := filepath.Join(out, filepath.Base(path))
		// Lstat refuses links and other non-regular files on both sides of the write.
		for _, file := range []string{source, destination} {
			info, err := os.Lstat(file)
			if err != nil || !info.Mode().IsRegular() {
				return false
			}
		}
		data, err := os.ReadFile(source)
		if err != nil || os.WriteFile(destination, data, 0o644) != nil {
			return false
		}
	}
	if b.replayGit(clone, append([]string{"add", "--"}, generatedSpecFiles...)...) != nil {
		return false
	}
	staged, err := b.replayOutput(clone, "write-tree")
	return err == nil && staged == tree
}
