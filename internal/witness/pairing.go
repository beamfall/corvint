package witness

import (
	"context"
	"fmt"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/gitrun"
)

// PairingTrailerKey is the Git trailer key of the merged-history pairing
// convention (AGW-V0-015).
const PairingTrailerKey = "Paired-Range"

// maxPairingMessage bounds the commit message --paired-from reads.
const maxPairingMessage = 64 << 10

// Pairing records where a paired report's ranges came from when the
// application range was read from a Paired-Range trailer (AGW-V0-036): the
// tests commit carrying the trailer, the trailer's repository identity as
// written, and the trailer value. The identity is the caller's label; it is
// never checked against the mapped checkout.
type Pairing struct {
	Source     string `json:"source"`
	Commit     string `json:"commit"`
	Repository string `json:"repository"`
	Trailer    string `json:"trailer"`
}

// PairingFromTrailer is the only Pairing source.
const PairingFromTrailer = "trailer"

// TrailerPairing is the application range one Paired-Range trailer names.
type TrailerPairing struct {
	Pairing
	Base, Head string
	// Parent is the tests commit's only parent: the tests range is
	// Parent..Commit.
	Parent string
}

// ReadPairingTrailer reads the one Paired-Range trailer of the tests commit
// revision in root (AGW-V0-036). It refuses, never ignores, every trailer it
// cannot use: a commit without exactly one parent, no trailer, a
// Paired-Range line Git does not parse as a trailer, more than one trailer,
// a name other than app, a value that is not REPO@BASE..HEAD with full
// lowercase-hex object IDs, or a repository identity with whitespace. A
// revision that does not resolve is refused naming the trailer it was to
// carry (AGW-V0-037). It only reads.
func ReadPairingTrailer(ctx context.Context, root, revision string) (*TrailerPairing, error) {
	commit, err := ResolveCommit(ctx, root, revision)
	if err != nil {
		return nil, fmt.Errorf("cannot read a %s trailer: %w", PairingTrailerKey, err)
	}
	budget := gitrun.NewDefaultBudget()
	parents, err := run(ctx, budget, root, 4096, "rev-list", "--parents", "-n", "1", commit)
	if err != nil {
		return nil, fmt.Errorf("cannot read the parents of %s", commit)
	}
	fields := strings.Fields(string(parents))
	if len(fields) != 2 {
		return nil, fmt.Errorf("commit %s has %d parents; a Paired-Range trailer pairs a tests commit with exactly one parent", commit, len(fields)-1)
	}
	parsed, err := run(ctx, budget, root, maxPairingMessage, "log", "-1", "--no-show-signature", "--format=%(trailers:key="+PairingTrailerKey+",valueonly,unfold)", commit)
	if err != nil {
		return nil, fmt.Errorf("cannot read the trailers of %s", commit)
	}
	message, err := run(ctx, budget, root, maxPairingMessage, "log", "-1", "--no-show-signature", "--format=%B", commit)
	if err != nil {
		return nil, fmt.Errorf("cannot read the message of %s", commit)
	}
	values := nonEmptyLines(string(parsed))
	written := 0
	for _, line := range strings.Split(string(message), "\n") {
		if key, _, ok := strings.Cut(line, ":"); ok && strings.EqualFold(strings.TrimSpace(key), PairingTrailerKey) {
			written++
		}
	}
	switch {
	case written != len(values):
		return nil, fmt.Errorf("commit %s has %d %s line(s) but Git parses %d as trailers; a pairing line outside the closing trailer block is refused, not ignored", commit, written, PairingTrailerKey, len(values))
	case len(values) == 0:
		return nil, fmt.Errorf("commit %s carries no %s trailer", commit, PairingTrailerKey)
	case len(values) > 1:
		return nil, fmt.Errorf("commit %s carries %d %s trailers; --paired-from reads exactly one, naming %s", commit, len(values), PairingTrailerKey, AppRange)
	}
	value := values[0]
	pairing, err := parsePairingValue(value)
	if err != nil {
		return nil, fmt.Errorf("commit %s: %s: %s: %w", commit, PairingTrailerKey, value, err)
	}
	pairing.Commit, pairing.Parent, pairing.Trailer, pairing.Source = commit, fields[1], value, PairingFromTrailer
	return pairing, nil
}

// parsePairingValue splits <name>=<repo>@<base>..<head> at the first = and the
// last @ (AGW-V0-015).
func parsePairingValue(value string) (*TrailerPairing, error) {
	name, rest, ok := strings.Cut(value, "=")
	if !ok {
		return nil, fmt.Errorf("takes <name>=<repo>@<base>..<head>")
	}
	if name != AppRange {
		return nil, fmt.Errorf("names range %q; a tests commit pairs only with %s", name, AppRange)
	}
	at := strings.LastIndex(rest, "@")
	if at <= 0 {
		return nil, fmt.Errorf("takes <name>=<repo>@<base>..<head> with a repository identity")
	}
	repository, span := rest[:at], rest[at+1:]
	if strings.ContainsAny(repository, " \t\r\n") {
		return nil, fmt.Errorf("repository identity %q holds whitespace", repository)
	}
	base, head, ok := strings.Cut(span, "..")
	if !ok || !fullObjectID(base) || !fullObjectID(head) || len(base) != len(head) {
		return nil, fmt.Errorf("takes <base>..<head> as full lowercase-hex commit object IDs")
	}
	return &TrailerPairing{Pairing: Pairing{Repository: repository}, Base: base, Head: head}, nil
}

func fullObjectID(value string) bool {
	return (len(value) == 40 || len(value) == 64) && strings.TrimLeft(value, "0123456789abcdef") == ""
}

// CheckPairedApplication refuses a trailer whose application range does not
// exist in the caller's application checkout root (AGW-V0-036): base or head
// that is not a commit there is unresolvable, and a base that is not an
// ancestor of head is a stale pairing. It only reads.
func CheckPairedApplication(ctx context.Context, root string, pairing *TrailerPairing) error {
	for _, side := range []struct{ label, id string }{{"base", pairing.Base}, {"head", pairing.Head}} {
		resolved, err := ResolveCommit(ctx, root, side.id)
		if err != nil || resolved != side.id {
			return fmt.Errorf("%s %s is not a commit in the mapped checkout", side.label, side.id)
		}
	}
	if _, err := run(ctx, gitrun.NewDefaultBudget(), root, 64, "merge-base", "--is-ancestor", pairing.Base, pairing.Head); err != nil {
		return fmt.Errorf("stale pairing: base %s is not an ancestor of head %s in the mapped checkout", pairing.Base, pairing.Head)
	}
	return nil
}

func nonEmptyLines(text string) []string {
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
