package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/gitstatus"
	"github.com/Beamfall/corvint/internal/gokernel"
)

// affectedAbbrevMin is the shortest commit prefix `--base` resolves (AFP-V0-230).
const affectedAbbrevMin = 7

// validAbbreviatedObjectID reports a lowercase-hex prefix shorter than a full object id.
func validAbbreviatedObjectID(value string) bool {
	if len(value) < affectedAbbrevMin || len(value) >= 40 {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

// affectedBaseRevisionLimit bounds the length of a --base revision (ACX-V0-854).
const affectedBaseRevisionLimit = 256

// validAffectedBaseRevision reports a --base value read as a git revision such as HEAD~1 or
// origin/main: printable non-space ASCII, not all hex (that shape keeps the AFP-V0-230 rules),
// never an option, a `:` search or a `..` range (ACX-V0-854).
func validAffectedBaseRevision(value string) bool {
	if value == "" || len(value) > affectedBaseRevisionLimit || value[0] == '-' || value[0] == ':' || strings.Contains(value, "..") {
		return false
	}
	hex := true
	for index := 0; index < len(value); index++ {
		character := value[index]
		if character <= ' ' || character > '~' {
			return false
		}
		if !strings.ContainsRune("0123456789abcdefABCDEF", rune(character)) {
			hex = false
		}
	}
	return !hex
}

// affectedBaseHint is the one recovery every unresolved --base refusal carries (ACX-V0-854).
func affectedBaseHint(value string) string {
	return "; pass the full commit id (git rev-parse --verify '" + value + "^{commit}')"
}

// affectedAmbiguousBaseCode names a --base prefix that matches more than one commit (AFP-V0-232).
const affectedAmbiguousBaseCode = "ambiguous-affected-base"

// affectedAmbiguousCandidateLimit bounds the commits an ambiguous-prefix refusal names.
const affectedAmbiguousCandidateLimit = 5

// affectedAmbiguousObjectLimit bounds the objects inspected for one ambiguous prefix.
const affectedAmbiguousObjectLimit = 64

// resolveAffectedAbbrevBase replaces an abbreviated or revision --base with the one commit it
// names in the root repository. An unknown or non-commit value is refused with the full-id hint,
// and a prefix naming several commits with ambiguous-affected-base, never guessed (AFP-V0-230,
// AFP-V0-232, ACX-V0-854).
func resolveAffectedAbbrevBase(invocation *affectedInvocation) error {
	if invocation.Base == "" || validGitObjectID(invocation.Base) { // a prefix or a revision is the only other admitted shape
		return nil
	}
	prefix := invocation.Base
	revision := !validAbbreviatedObjectID(prefix)
	refuse := func() error {
		return argumentError("--base " + prefix + " is not a commit in this repository" + affectedBaseHint(prefix))
	}
	stdout, err := affectedAbbrevGit(invocation.Root, "rev-parse", "--verify", "--quiet", "--end-of-options", prefix+"^{commit}")
	if err != nil {
		if revision {
			return refuse()
		}
		if candidates := affectedAmbiguousCommits(invocation.Root, prefix); len(candidates) > 1 {
			return &gokernel.Error{Code: affectedAmbiguousBaseCode, Message: "--base " + prefix + " matches more than one commit (" + strings.Join(candidates, ", ") + "); pass the full commit id"}
		}
		return refuse()
	}
	full := strings.TrimSpace(stdout)
	if !validGitObjectID(full) || !revision && !strings.HasPrefix(full, prefix) {
		return refuse()
	}
	invocation.Base = full
	return nil
}

// affectedAbbrevGit runs one read-only git command in root with a scrubbed configuration.
func affectedAbbrevGit(root string, arguments ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, gitstatus.Executable(), append([]string{"--no-optional-locks", "-C", root}, arguments...)...)
	command.Env = []string{"PATH=" + os.Getenv("PATH"), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_TERMINAL_PROMPT=0", "LC_ALL=C"}
	var stdout bytes.Buffer
	command.Stdout = &stdout
	err := command.Run()
	return stdout.String(), err
}

// affectedAmbiguousCommits lists, sorted and bounded, the commit ids that prefix abbreviates.
// A git failure reads as no candidates, so the caller keeps the generic refusal.
func affectedAmbiguousCommits(root, prefix string) []string {
	listing, err := affectedAbbrevGit(root, "rev-parse", "--disambiguate="+prefix)
	if err != nil {
		return nil
	}
	objects := strings.Fields(listing)
	if len(objects) < 2 {
		return nil
	}
	if len(objects) > affectedAmbiguousObjectLimit {
		objects = objects[:affectedAmbiguousObjectLimit]
	}
	var commits []string
	for _, object := range objects {
		kind, err := affectedAbbrevGit(root, "cat-file", "-t", object)
		if err != nil || strings.TrimSpace(kind) != "commit" || !validGitObjectID(object) {
			continue
		}
		commits = append(commits, object)
	}
	sort.Strings(commits)
	if len(commits) > affectedAmbiguousCandidateLimit {
		commits = append(commits[:affectedAmbiguousCandidateLimit], "...")
	}
	return commits
}

// extractAffectedTrailingRoot removes one `--root VALUE` or `--root=VALUE` that follows the
// `affected` verb (AFP-V0-230). Sub-verbs keep their own parsers, and the value of another
// option is never read as --root.
func extractAffectedTrailingRoot(rest []string) ([]string, string, error) {
	if len(rest) != 0 && (rest[0] == "discovery" || rest[0] == "cache" || rest[0] == "audit") {
		return rest, "", nil
	}
	kept, root, seen := make([]string, 0, len(rest)), "", false
	for index := 0; index < len(rest); index++ {
		token := rest[index]
		switch {
		case token == "--root" || strings.HasPrefix(token, "--root="):
			if seen {
				return nil, "", argumentError("--root may be given once, before or after affected")
			}
			seen = true
			if token == "--root" {
				if index+1 >= len(rest) || rest[index+1] == "" || strings.HasPrefix(rest[index+1], "--") {
					return nil, "", argumentError("--root requires a nonempty value")
				}
				root, index = rest[index+1], index+1
			} else if root = strings.TrimPrefix(token, "--root="); root == "" {
				return nil, "", argumentError("--root requires a nonempty value")
			}
		case affectedOptionNames[token] && index+1 < len(rest):
			kept = append(kept, token, rest[index+1])
			index++
		default:
			kept = append(kept, token)
		}
	}
	return kept, root, nil
}
