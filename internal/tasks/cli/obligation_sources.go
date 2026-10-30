package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"path"

	"github.com/Beamfall/corvint/internal/tasks/obligation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// sourceReviewValue is the TOL-V0-042 stale-source listing that show and
// plan add when some entry records review sources. legacy-case and
// manual-case refs resolve by case ID in the corvint-legacy-cases/1 record
// at casesFile; legacy-spec refs resolve as repository paths at HEAD, read
// from Git objects. It writes nothing and changes no entry state. ok is
// false, and the caller emits nothing, when no entry records sources.
func sourceReviewValue(env Env, l *ticket.ObligationLedger, casesFile string) (wire.Value, bool, error) {
	if l == nil {
		return wire.Value{}, false, nil
	}
	var specs []string
	has := false
	for _, e := range l.Entries {
		has = has || e.Sources != nil
		for _, s := range e.Sources {
			if s.Kind == ticket.ObligationReviewLegacySpec && legacySpecPath(s.Ref) {
				specs = append(specs, s.Ref)
			}
		}
	}
	if !has {
		return wire.Value{}, false, nil
	}
	var cases map[string]string
	if casesFile != "" {
		raw, err := readBoundedFile("--legacy-cases", casesFile, obligation.MaxLegacyCasesBytes)
		if err != nil {
			return wire.Value{}, false, err
		}
		if cases, err = obligation.ReadLegacyCaseDigests(raw); err != nil {
			return wire.Value{}, false, err
		}
	}
	commit := ""
	present, content := map[string]bool{}, map[string][]byte{}
	if len(specs) > 0 {
		root, err := store.WorktreeRoot(env.Cwd)
		if err != nil {
			return wire.Value{}, false, err
		}
		if commit, present, content, err = store.FilesAtCommit(root, "HEAD", specs); err != nil {
			return wire.Value{}, false, err
		}
	}
	stale, unresolved, _ := obligation.CheckSourceDigests(l, func(kind, ref string) (string, string) {
		if kind == ticket.ObligationReviewLegacySpec {
			switch {
			case !legacySpecPath(ref):
				return "", "the legacy-spec ref is not a repository-relative path"
			case commit == "":
				return "", "the repository has no HEAD commit"
			case content[ref] != nil:
				sum := sha256.Sum256(content[ref])
				return hex.EncodeToString(sum[:]), ""
			case present[ref]:
				return "", "the legacy spec exceeds the read bound at HEAD"
			}
			return "", "the legacy spec is not a file at HEAD"
		}
		if cases == nil {
			return "", "no --legacy-cases record was given"
		}
		if d, ok := cases[ref]; ok {
			return d, ""
		}
		return "", "the legacy case is not in the --legacy-cases record"
	})
	list := func(items []obligation.SourceReview, stale bool) wire.Value {
		vs := make([]wire.Value, 0, len(items))
		for _, it := range items {
			o := wire.NewObject().Set("id", wire.String(it.ID)).Set("state", wire.String(it.State)).
				Set("kind", wire.String(it.Kind)).Set("ref", wire.String(prose(it.Ref))).Set("reviewed", wire.String(it.Reviewed))
			if stale {
				o.Set("current", wire.String(it.Current))
			} else {
				o.Set("reason", wire.String(it.Reason))
			}
			vs = append(vs, wire.ObjectValue(o))
		}
		return wire.Array(vs...)
	}
	head := wire.Null()
	if commit != "" {
		head = wire.String(commit)
	}
	o := wire.NewObject().Set("specCommit", head).
		Set("staleCount", wire.String(string(wire.CountOf(int64(len(stale)))))).Set("stale", list(stale, true)).
		Set("unresolvedCount", wire.String(string(wire.CountOf(int64(len(unresolved)))))).Set("unresolved", list(unresolved, false))
	return wire.ObjectValue(o), true, nil
}

// legacySpecPath reports whether p is a clean repository-relative path that
// Git can be asked for.
func legacySpecPath(p string) bool { return repoRelative(p) && path.Clean(p) == p }

// readBoundedFile reads at most max bytes of the regular file a flag names,
// refusing a larger or unreadable one. A symlink, FIFO, device or directory
// is refused before it is opened, and the open never blocks (openRegular).
func readBoundedFile(flag, file string, max int) ([]byte, error) {
	f, _, err := openRegular(file)
	if errors.Is(err, errNotRegular) {
		return nil, wire.Errorf(wire.CodeMalformed, flag, "the %s file is not a regular file", flag)
	}
	if err != nil {
		return nil, wire.Errorf(wire.CodeMissingEvidence, flag, "the %s file is unreadable", flag)
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, int64(max)+1))
	if err != nil {
		return nil, wire.Errorf(wire.CodeMissingEvidence, flag, "the %s file is unreadable", flag)
	}
	if len(raw) > max {
		return nil, wire.Errorf(wire.CodeLimitExceeded, flag, "the %s file exceeds %d bytes", flag, max)
	}
	return raw, nil
}
