package store

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/ticket"
)

// Hold condition states (SCC-V0-011): UNBLOCKED when the condition is met at
// its ref's current commit, BLOCKED when it is observably unmet, UNKNOWN
// when the ref, commit, path or blob cannot be observed. An UNKNOWN never
// reads as UNBLOCKED.
const (
	HoldConditionUnblocked = "UNBLOCKED"
	HoldConditionBlocked   = "BLOCKED"
	HoldConditionUnknown   = "UNKNOWN"
)

// HoldConditionMaxBlobBytes bounds the file a SYMBOL_DEFINED condition reads.
const HoldConditionMaxBlobBytes = 1 << 20

// HoldConditionResult is one evaluation: the state, the commit the ref
// resolved to ("" when it did not), the containment answer of a
// COMMIT_CONTAINED condition and a one-line detail.
type HoldConditionResult struct {
	State, RefCommit, Detail string
	Containment              *SourceContainment
}

// EvaluateHoldCondition re-evaluates c against the current commit of its
// ref in root. It is a pure Git read: it never fetches, writes a ref or
// releases the hold.
func EvaluateHoldCondition(root string, c *ticket.HoldCondition) HoldConditionResult {
	res := HoldConditionResult{State: HoldConditionUnknown}
	out, err := gitOutput(root, "rev-parse", "--verify", "--quiet", "--end-of-options", c.Ref+"^{commit}")
	if err != nil {
		res.Detail = "ref " + c.Ref + " does not resolve to a commit"
		return res
	}
	res.RefCommit = strings.TrimSpace(string(out))
	at := " at " + c.Ref + " " + res.RefCommit
	switch c.Kind {
	case ticket.HoldConditionCommitContained:
		sc := SourceContainmentOf(root, res.RefCommit, c.Commit)
		res.Containment = &sc
		switch sc.State {
		case SourceAncestor:
			res.State, res.Detail = HoldConditionUnblocked, "condition met"+at+": commit "+c.Commit+" is an ancestor"
		case SourceEquivalent:
			res.State, res.Detail = HoldConditionUnblocked, "condition met"+at+": commit "+c.Commit+" is present as equivalent "+sc.Equivalent
		case SourceAbsent:
			res.State, res.Detail = HoldConditionBlocked, "commit "+c.Commit+" is absent"+at+" ("+sc.Reason+")"
		default:
			res.Detail = "commit containment is unknown" + at + " (" + sc.Reason + ")"
		}
	case ticket.HoldConditionPathExists:
		exists, err := holdConditionPathExists(root, res.RefCommit, c.Path)
		switch {
		case err != nil:
			res.Detail = "path " + c.Path + " is unobservable" + at
		case exists:
			res.State, res.Detail = HoldConditionUnblocked, "condition met"+at+": path "+c.Path+" exists"
		default:
			res.State, res.Detail = HoldConditionBlocked, "path "+c.Path+" does not exist"+at
		}
	case ticket.HoldConditionSymbolDefined:
		res.State, res.Detail = holdConditionSymbolAt(root, res.RefCommit, c.Path, c.Symbol)
		if res.State == HoldConditionUnblocked {
			res.Detail = "condition met" + at + ": " + res.Detail
		} else {
			res.Detail += at
		}
	default:
		res.Detail = "unknown condition kind " + c.Kind
	}
	return res
}

// holdConditionPathExists looks path up in commit's tree: it exists when an
// entry of that exact name is listed, and a trailing '/' requires that entry
// to be a directory (a tree), so a file never satisfies "dir/".
func holdConditionPathExists(root, commit, path string) (bool, error) {
	name := strings.TrimSuffix(path, "/")
	out, err := gitOutput(root, "ls-tree", "-z", "--full-tree", "--end-of-options", commit, "--", name)
	if err != nil {
		return false, err
	}
	for _, rec := range strings.Split(string(out), "\x00") {
		meta, entry, ok := strings.Cut(rec, "\t")
		if f := strings.Fields(meta); ok && entry == name && len(f) == 3 {
			return name == path || f[1] == "tree", nil
		}
	}
	return false, nil
}

// holdConditionDeclaration matches a declaration-keyword line naming the
// symbol (SCC-V0-012): Go, Rust, Python, Ruby, JavaScript/TypeScript,
// Java/Kotlin/C#-style and shell-function forms. It is a line heuristic, so a
// miss keeps the hold BLOCKED; it never parses the language.
func holdConditionDeclaration(symbol string) *regexp.Regexp {
	q := regexp.QuoteMeta(symbol)
	return regexp.MustCompile(`(?m)^[ \t]*(?:(?:export|pub(?:\([^)]*\))?|public|private|protected|internal|static|async|abstract|final|default|open|override|sealed|data|declare)[ \t]+)*` +
		`(?:func(?:[ \t]*\([^)]*\))?|type|var|const|class|def|function|module|interface|struct|enum|trait|fn|let|object|record)[ \t]+` + q + `\b` +
		`|^[ \t]*` + q + `[ \t]*\(\)[ \t]*\{`)
}

// holdConditionSymbolAt reads path's blob at commit within the byte bound
// and matches the declaration heuristic.
func holdConditionSymbolAt(root, commit, path, symbol string) (string, string) {
	out, err := gitOutput(root, "ls-tree", "-z", "--full-tree", "-l", "--end-of-options", commit, "--", path)
	if err != nil {
		return HoldConditionUnknown, "path " + path + " is unobservable"
	}
	if len(out) == 0 {
		return HoldConditionBlocked, "file " + path + " does not exist, so symbol " + symbol + " is not defined"
	}
	meta, _, _ := strings.Cut(string(out), "\t")
	f := strings.Fields(meta)
	if len(f) != 4 || f[1] != "blob" {
		return HoldConditionBlocked, "path " + path + " is not a file, so symbol " + symbol + " is not defined"
	}
	size, err := strconv.ParseInt(f[3], 10, 64)
	if err != nil || size > HoldConditionMaxBlobBytes {
		return HoldConditionUnknown, "file " + path + " exceeds the " + strconv.Itoa(HoldConditionMaxBlobBytes) + "-byte symbol read bound"
	}
	blob, err := gitOutput(root, "cat-file", "blob", f[2])
	if err != nil {
		return HoldConditionUnknown, "file " + path + " is unreadable"
	}
	if bytes.IndexByte(blob, 0) >= 0 {
		return HoldConditionUnknown, "file " + path + " is binary"
	}
	if holdConditionDeclaration(symbol).Match(blob) {
		return HoldConditionUnblocked, "symbol " + symbol + " is defined in " + path
	}
	return HoldConditionBlocked, "symbol " + symbol + " is not declared in " + path
}
