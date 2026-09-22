// Package testevidence is the opt-in producer half of retained test evidence
// (LPCV-V0-055, decision 0218): a provider run with --retain writes its exact
// stdout document into the one location testvaliditydoc.Discover reads.
package testevidence

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

// MaxRetained bounds how many documents one producer keeps.
const MaxRetained = 32

// Retain atomically writes document as <producer>-<unix-nanos>-<hex>.json
// under testvaliditydoc.EvidenceDirectory of worktree, creating missing
// directories 0700 and the file 0600, refusing a symlinked or non-directory
// component, then prunes that directory to the producer's newest MaxRetained
// names. It returns the retained file name; a document its own prune removed is
// a failure, never a retained name.
func Retain(worktree, producer string, document []byte) (string, error) {
	root, err := os.OpenRoot(worktree)
	if err != nil {
		return "", fmt.Errorf("worktree cannot be opened: %w", err)
	}
	defer root.Close()
	evidence, err := openEvidence(root)
	if err != nil {
		return "", err
	}
	defer evidence.Close()
	name, err := write(evidence, producer, document)
	if err != nil {
		return "", err
	}
	return name, prune(evidence, producer, name)
}

func openEvidence(worktree *os.Root) (*os.Root, error) {
	parent := worktree
	for _, component := range strings.Split(testvaliditydoc.EvidenceDirectory, "/") {
		next, err := openDirectory(parent, component)
		if parent != worktree {
			parent.Close()
		}
		if err != nil {
			return nil, err
		}
		parent = next
	}
	return parent, nil
}

// openDirectory creates name if missing and opens it only when it is a real
// directory, the same file as the no-follow Lstat that admitted it.
func openDirectory(parent *os.Root, name string) (*os.Root, error) {
	if err := parent.Mkdir(name, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
		return nil, fmt.Errorf("%s cannot be created: %w", name, err)
	}
	admitted, err := parent.Lstat(name)
	if err != nil || !admitted.IsDir() {
		return nil, fmt.Errorf("%s is not a real directory", name)
	}
	opened, err := parent.OpenRoot(name)
	if err != nil {
		return nil, fmt.Errorf("%s cannot be opened: %w", name, err)
	}
	current, err := opened.Stat(".")
	if err != nil || !os.SameFile(admitted, current) {
		opened.Close()
		return nil, fmt.Errorf("%s changed while it was opened", name)
	}
	return opened, nil
}

func write(evidence *os.Root, producer string, document []byte) (string, error) {
	suffix := make([]byte, 8)
	_, _ = rand.Read(suffix)
	name := fmt.Sprintf("%s-%019d-%s.json", producer, time.Now().UnixNano(), hex.EncodeToString(suffix))
	temporary := "." + strings.TrimSuffix(name, ".json") + ".tmp"
	file, err := evidence.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", fmt.Errorf("retained document cannot be created: %w", err)
	}
	_, err = file.Write(document)
	if err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = evidence.Rename(temporary, name)
	}
	if err != nil {
		_ = evidence.Remove(temporary)
		return "", fmt.Errorf("retained document cannot be written: %w", err)
	}
	return name, nil
}

// prune removes the producer's regular files beyond the newest MaxRetained.
// The fixed-width nanosecond field makes name order write order; a file
// whose name does not match the producer's pattern is never removed. A clock
// behind an earlier write names written below the newest MaxRetained; its
// removal is reported, since nothing was retained. Once MaxRetained documents
// exist, the producer's temporaries named below the oldest kept one are
// removed too: a crash before rename leaves one, and a writer still holding it
// would have its document pruned on arrival.
func prune(evidence *os.Root, producer, written string) error {
	stem := `^` + regexp.QuoteMeta(producer) + `-[0-9]{19}-[0-9a-f]{16}`
	pattern := regexp.MustCompile(stem + `\.json$`)
	temporaryPattern := regexp.MustCompile(`^\.` + stem[1:] + `\.tmp$`)
	listing, err := evidence.Open(".")
	if err != nil {
		return fmt.Errorf("retained-evidence location cannot be listed: %w", err)
	}
	defer listing.Close()
	entries, err := listing.ReadDir(-1)
	if err != nil {
		return fmt.Errorf("retained-evidence location cannot be listed: %w", err)
	}
	owned, temporaries := []string{}, []string{}
	for _, entry := range entries {
		if entry.Type().IsRegular() && pattern.MatchString(entry.Name()) {
			owned = append(owned, entry.Name())
		}
		if entry.Type().IsRegular() && temporaryPattern.MatchString(entry.Name()) {
			temporaries = append(temporaries, entry.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(owned)))
	pruned := owned[min(len(owned), MaxRetained):]
	if len(owned) >= MaxRetained {
		pruned = append(pruned, staleTemporaries(temporaries, owned[MaxRetained-1])...)
	}
	for _, name := range pruned {
		if err := evidence.Remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("retained document cannot be pruned: %w", err)
		}
	}
	if slices.Contains(pruned, written) {
		return errors.New("retained document was pruned: newer-named documents exist")
	}
	return nil
}

// staleTemporaries returns the temporaries whose document name would sort
// below oldestKept.
func staleTemporaries(temporaries []string, oldestKept string) []string {
	stale := []string{}
	for _, name := range temporaries {
		if strings.TrimSuffix(name[1:], ".tmp")+".json" < oldestKept {
			stale = append(stale, name)
		}
	}
	return stale
}
