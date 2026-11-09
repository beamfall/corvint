package workflow

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cem/witnesspaths"
)

// PathPolicyProfile names the additive declared path policy profile of cem
// prepare and cem status (CAX-V0-060). It is reported beside the cem/1.0
// map and verifier result and never changes either.
const PathPolicyProfile = "cem-path-policy/1"

// PathPolicyBase is the only value of --path-policy: the policy is read from
// the range's base tree (CAX-V0-061).
const PathPolicyBase = "base"

// PathPolicyDeclaredInert is the state of a hunk a base-tree rule declares
// inert. It is a profile row state, never a map disposition (CAX-V0-063).
const PathPolicyDeclaredInert = "declared-inert"

func pathPolicyOption(value string) (bool, error) {
	switch value {
	case "":
		return false, nil
	case PathPolicyBase:
		return true, nil
	}
	return false, invalidArguments("--path-policy must be %s", PathPolicyBase)
}

// DataReaderFinder opens, for the target revision of the repository at root,
// a lookup of the readers of a declared data path given its rule's declared
// readers, with the origins of AGW-V0-044 (CAX-V0-064). The binary installs
// one built on the head-revision index, so the CEM seams keep their
// dependency closure (the index's own tests import this package).
type DataReaderFinder func(ctx context.Context, root, target string) (func(dataPath string, declared []string) []witnesspaths.Reader, error)

// SetDataReaderFinder installs the data-reader lookup of the path-policy
// profile.
func (s *Session) SetDataReaderFinder(finder DataReaderFinder) { s.dataReaders = finder }

// readBasePathPolicy reads the witness path policy blob from base's tree
// through the repository's bounded object reads, never the worktree or
// another tree (CAX-V0-061). It returns nil when base holds no policy and an
// INVALID account with no rule for a defective one, as witness does
// (AGW-V0-041).
func readBasePathPolicy(ctx context.Context, repository *gitauth.Repository, base string) (*witnesspaths.Policy, witnesspaths.Rules) {
	entry, exists, err := repository.LookupTreeEntry(ctx, base, witnesspaths.PolicyPath)
	switch {
	case err != nil:
		return witnesspaths.Invalid("the base tree listing failed: " + cemcode.MessageOf(err)), witnesspaths.Rules{}
	case !exists:
		return nil, witnesspaths.Rules{}
	case entry.Type != "blob" || (entry.Mode != "100644" && entry.Mode != "100755"):
		return witnesspaths.Invalid("the base tree holds a non-file object at this path"), witnesspaths.Rules{}
	}
	raw, over, err := repository.BlobBytesWithin(ctx, entry.OID, witnesspaths.MaxBytes)
	if err != nil || over {
		return witnesspaths.Invalid(fmt.Sprintf("the policy blob is unreadable or over %d bytes", witnesspaths.MaxBytes)), witnesspaths.Rules{}
	}
	return witnesspaths.FromBlob(raw)
}

// pathPolicyMembers reads the witness path policy from base's tree in the
// session's repository and classifies the map's unknown hunks under it
// (CAX-V0-061 to CAX-V0-064). It returns the pathPolicy member (nil when base
// holds no policy), one row per declared hunk in map order, and the count of
// declared-inert rows. A supported or mechanical hunk and a path an extractor
// reads are never classified, so the policy never upgrades a code hunk.
func (s *Session) pathPolicyMembers(ctx context.Context, base, target string, document *wire.Map) (any, []any, int) {
	policy, rules := readBasePathPolicy(ctx, s.repository, base)
	rows := []any{}
	if policy == nil {
		return nil, rows, 0
	}
	inert := 0
	var lookup func(string, []string) []witnesspaths.Reader
	var lookupErr error
	opened := false
	for position, hunk := range document.Hunks {
		if hunk.Disposition != "unknown" {
			continue
		}
		declaration := rules.Declare(hunk.Path)
		if declaration == nil {
			continue
		}
		row := map[string]any{"selector": position + 1, "id": hunk.ID, "path": hunk.Path}
		if declaration.Class == witnesspaths.ClassInert {
			inert++
			row["state"], row["reason"] = PathPolicyDeclaredInert, "declared-inert"
			row["declaration"] = declarationValue(declaration, nil)
			rows = append(rows, row)
			continue
		}
		// A data path stays unknown whatever its readers: the profile only
		// names why (CAX-V0-064). The lookup is opened once, and only when a
		// data rule applies.
		if !opened {
			opened = true
			lookupErr = errors.New("no data-reader finder is installed")
			if s.dataReaders != nil {
				lookup, lookupErr = s.dataReaders(ctx, s.root, target)
			}
		}
		row["state"] = "unknown"
		if lookupErr != nil {
			row["reason"], row["detail"] = "data-reader-not-produced", "the target readers could not be found: "+lookupErr.Error()
			row["declaration"] = declarationValue(declaration, []witnesspaths.Reader{})
			rows = append(rows, row)
			continue
		}
		readers := lookup(hunk.Path, rules.ReaderNames(declaration))
		live := []string{}
		for _, reader := range readers {
			if reader.Origin == "declared" || reader.Origin == "found" {
				live = append(live, reader.Path+" ("+reader.Origin+")")
			}
		}
		row["reason"], row["detail"] = "data-reader-missing", "no tracked source with an extractor reads this path"
		if len(live) != 0 {
			row["reason"], row["detail"] = "data-reader", "read by "+strings.Join(live, ", ")
		}
		row["declaration"] = declarationValue(declaration, readers)
		rows = append(rows, row)
	}
	return map[string]any{"path": policy.Path, "status": policy.Status, "detail": policy.Detail, "rules": policy.Rules}, rows, inert
}

func declarationValue(declaration *witnesspaths.Declaration, readers []witnesspaths.Reader) map[string]any {
	value := map[string]any{"file": declaration.File, "line": declaration.Line, "glob": declaration.Glob, "class": declaration.Class}
	if readers != nil {
		list := make([]any, 0, len(readers))
		for _, reader := range readers {
			entry := map[string]any{"path": reader.Path, "origin": reader.Origin}
			if reader.Reason != "" {
				entry["reason"] = reader.Reason
			}
			list = append(list, entry)
		}
		value["readers"] = list
	}
	return value
}

// stablePathPolicy adds the cem-path-policy/1 members to a status envelope
// and returns the declared-inert count, or -1 when no classification was
// produced. The policy is read only after the verifier accepts, from the
// map's verified base, so an unadmitted repository is never read and a
// worktree edit has no effect (CAX-V0-061).
func (s *Session) stablePathPolicy(ctx context.Context, envelope map[string]any, raw []byte, accepted bool, target string) int {
	envelope["pathPolicyProfile"] = PathPolicyProfile
	envelope["pathPolicyHunks"] = []any{}
	notProduced := func(detail string) int {
		envelope["pathPolicy"] = map[string]any{"path": witnesspaths.PolicyPath, "status": "NOT_PRODUCED", "detail": detail, "rules": 0}
		return -1
	}
	document, err := wire.ParseStable(raw)
	if !accepted || err != nil {
		return notProduced("the stable verifier did not accept the map, so no policy was read")
	}
	if err := s.openRepository(); err != nil {
		return notProduced("the repository was not admitted: " + cemcode.MessageOf(err))
	}
	defer s.repository.BeginObjectSession()()
	change := wire.Map(document.Change)
	policy, rows, inert := s.pathPolicyMembers(ctx, change.BaseRevision, target, &change)
	envelope["pathPolicy"], envelope["pathPolicyHunks"] = policy, rows
	return inert
}

// addDeclaredInertCounts moves each declared-inert hunk from unknown to
// counts.declaredInert, so --max-unknown does not bound it; total is
// unchanged. A negative value means the profile is off (CAX-V0-063).
func addDeclaredInertCounts(counts map[string]any, inert int) {
	if inert < 0 {
		return
	}
	counts["unknown"] = counts["unknown"].(int) - inert
	counts["declaredInert"] = inert
}
