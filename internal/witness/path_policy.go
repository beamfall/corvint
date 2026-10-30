package witness

import (
	"context"
	"fmt"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/witnesspaths"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/witnesspaths/datareaders"
)

// PathPolicyPath is where the declared witness path policy is committed. It is
// read from the range's base tree only, so a range cannot exempt its own
// paths (AGW-V0-041). The decoder is shared with cem's cem-path-policy/1
// profile (internal/cem/witnesspaths).
const PathPolicyPath = witnesspaths.PolicyPath

// PathPolicyProfile is the closed profile of the policy document.
const PathPolicyProfile = witnesspaths.Profile

// Path classes a policy rule may declare (AGW-V0-041).
const (
	ClassInert = witnesspaths.ClassInert
	ClassData  = witnesspaths.ClassData
)

// Exempt is the verdict of a path the policy declares inert: determined,
// never closed (AGW-V0-043).
const Exempt Verdict = "EXEMPT"

// PathPolicy is the report's account of the base tree's policy (AGW-V0-045).
type PathPolicy = witnesspaths.Policy

// Declaration is the policy rule that classified one obligation's path, with
// the file and line that declare it (AGW-V0-042).
type Declaration = witnesspaths.Declaration

// Reader is one reader of a data path and where it came from (AGW-V0-044).
type Reader = witnesspaths.Reader

// readPathPolicy reads the policy blob from the base tree. It returns nil when
// none is committed there, so a repository without one keeps its report bytes.
// A defective policy is reported INVALID and contributes no rule.
func readPathPolicy(ctx context.Context, budget *gitrun.Budget, root, base string) (*PathPolicy, witnesspaths.Rules) {
	listing, err := run(ctx, budget, root, 4096, "--literal-pathspecs", "ls-tree", "-z", "--full-tree", base, "--", PathPolicyPath)
	if err != nil {
		return witnesspaths.Invalid("the base tree listing failed: " + err.Error()), witnesspaths.Rules{}
	}
	if len(listing) == 0 {
		return nil, witnesspaths.Rules{}
	}
	meta, _, _ := strings.Cut(string(listing), "\t")
	fields := strings.Fields(meta)
	if len(fields) != 3 || fields[1] != "blob" || fields[0] == "120000" {
		return witnesspaths.Invalid("the base tree holds a non-file object at this path"), witnesspaths.Rules{}
	}
	raw, err := run(ctx, budget, root, witnesspaths.MaxBytes, "cat-file", "blob", fields[2])
	if err != nil {
		return witnesspaths.Invalid(fmt.Sprintf("the policy blob is unreadable or over %d bytes", witnesspaths.MaxBytes)), witnesspaths.Rules{}
	}
	return witnesspaths.FromBlob(raw)
}

// applyPathPolicy classifies each CHANGE obligation left NOT_RUN only because
// no extractor reads its path, or because such a path was deleted (AGW-V0-042): inert paths become EXEMPT, and data
// paths are admitted text-only when a reader is declared or found, else they
// stay NOT_RUN as DATA_READER_MISSING (AGW-V0-043, AGW-V0-044). It returns the
// IMPACT obligations it opens for readers the range does not change.
func applyPathPolicy(obligations []Obligation, rules witnesspaths.Rules, deletedFiles map[string]bool, index *contextindex.Index, unparsed map[string]string) []Obligation {
	if rules.Len() == 0 {
		return nil
	}
	changed := map[string]bool{}
	for _, item := range obligations {
		changed[item.Path] = true
	}
	opened := map[string]bool{}
	var readers []Obligation
	for position := range obligations {
		item := &obligations[position]
		deleted := item.Reason == "ADMISSION_REFUSED_DELETED" || item.Reason == "PATH_DELETED_OUT_OF_PROFILE"
		if item.Kind != KindChange || item.Verdict != NotRun || item.Reason != "LANGUAGE_COVERAGE_NONE" && !deleted {
			continue
		}
		// The policy never classifies itself, and a deleted symbolic link or
		// gitlink keeps its reason as a changed one does.
		if item.Path == PathPolicyPath || deleted && !deletedFiles[item.Path] {
			continue
		}
		if deleted {
			// Only a deletion no extractor would read can be inert; a deleted
			// data path keeps its reason, since a reader may still need it.
			if _, reason := coverage(change{status: "M", path: item.Path}, nil); reason != "LANGUAGE_COVERAGE_NONE" {
				continue
			}
		}
		declaration := rules.Rule(item.Path)
		if declaration == nil || deleted && declaration.Class != ClassInert {
			continue
		}
		item.Declaration = declaration
		if declaration.Class == ClassInert {
			item.Verdict, item.Reason = Exempt, "DECLARED_INERT"
			continue
		}
		item.Declaration.Readers = datareaders.Find(item.Path, rules.ReaderNames(declaration), index, unparsed)
		live := 0
		for _, reader := range item.Declaration.Readers {
			if reader.Origin != "declared" && reader.Origin != "found" {
				continue
			}
			live++
			if changed[reader.Path] || opened[reader.Path] {
				continue
			}
			opened[reader.Path] = true
			tier, _ := coverage(change{status: "M", path: reader.Path}, unparsed)
			readers = append(readers, Obligation{
				ID: "impact:data-reader:" + reader.Path, Kind: KindImpact, Path: reader.Path,
				Relation: "data-reader", Coverage: tier, Witnesses: []Witness{},
			})
		}
		if live == 0 {
			item.Reason = "DATA_READER_MISSING"
			continue
		}
		item.Verdict, item.Reason, item.Coverage = "", "", "GENERIC"
	}
	return readers
}

// deletedFiles names the regular files the range deletes.
func deletedFiles(changes []change) map[string]bool {
	result := map[string]bool{}
	for _, item := range changes {
		if item.status == "D" && (item.oldMode == "100644" || item.oldMode == "100755") {
			result[item.path] = true
		}
	}
	return result
}
