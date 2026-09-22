package contextindex

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/secretscreen"
)

const (
	maxHistoryCommits = 200
	maxHistoryBytes   = 64 * 1024 * 1024
)

var pythonIgnoreCaseSpecials = strings.NewReplacer("İ", "i", "ı", "i", "ſ", "s", "K", "k")

type historyEntry struct {
	commit, tree, subject string
	paths                 []string
}

func authorityTraceState(index *Index) (string, error) {
	if len(index.DirtyPaths) != 0 {
		return "blocked-mixed-worktree", nil
	}
	base := filepath.Join(index.Root, ".context-corvint")
	info, err := os.Lstat(base)
	if os.IsNotExist(err) {
		return "absent", nil
	}
	if err != nil {
		return "", &Error{Code: "unsupported-query-trace-state", Message: "native Go authority-start query cannot inspect the local trace store"}
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", &Error{Code: "unsupported-query-trace-state", Message: "native Go authority-start query requires an absent clean-tree local trace store"}
	}
	traces := filepath.Join(base, "traces")
	_, err = os.Lstat(traces)
	if os.IsNotExist(err) {
		return "absent", nil
	}
	if err != nil {
		return "", &Error{Code: "unsupported-query-trace-state", Message: "native Go authority-start query cannot inspect the local trace store"}
	}
	return "", &Error{Code: "unsupported-query-trace-state", Message: "native Go authority-start query requires an absent clean-tree local trace store"}
}

func historyObject(ctx context.Context, index *Index, expression string) (string, error) {
	raw, err := git(ctx, index.Root, maxIdentityBytes, nil, "rev-parse", expression)
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(string(raw))
	if !validObjectID(value, index.ObjectFormat) {
		return "", &Error{Message: "Git returned an invalid history object identity"}
	}
	return value, nil
}

// historyObservation is one side of the history-learning stability bracket:
// the worktree status and a Git object identity, sampled as one instant.
type historyObservation struct {
	statusSHA256 string
	dirtyErr     error
	head, tree   string
	identityErr  error
}

// observeHistory opens the bracket. The status read and the identity read are
// two independent Git invocations of the same instant, so run in series each
// spent a whole process spawn and worktree scan waiting for the other. Both
// errors are carried out rather than returned, so each caller still reports
// whichever failure its sequential form reported first.
func observeHistory(ctx context.Context, index *Index) historyObservation {
	return observeHistoryWith(ctx, index, func(observation *historyObservation) {
		observation.head, observation.tree, observation.identityErr = historyIdentity(ctx, index)
	})
}

// historyProbe is one opening history observation still in flight.
type historyProbe struct {
	done        chan struct{}
	observation historyObservation
}

// startHistoryObservation opens the bracket without waiting for it. The caller
// must reach result before it reads the repository again; ranking does not read
// the repository at all, which is why the observation can be issued ahead of it.
func startHistoryObservation(ctx context.Context, index *Index) *historyProbe {
	probe := &historyProbe{done: make(chan struct{})}
	go func() {
		defer close(probe.done)
		probe.observation = observeHistory(ctx, index)
	}()
	return probe
}

func (probe *historyProbe) result() historyObservation {
	<-probe.done
	return probe.observation
}

// observeHistoryTree closes it, reading only the tree the learn stage compares.
func observeHistoryTree(ctx context.Context, index *Index) historyObservation {
	return observeHistoryWith(ctx, index, func(observation *historyObservation) {
		observation.tree, observation.identityErr = historyObject(ctx, index, "HEAD^{tree}")
	})
}

func observeHistoryWith(ctx context.Context, index *Index, identity func(*historyObservation)) historyObservation {
	var observation historyObservation
	var pending sync.WaitGroup
	pending.Add(1)
	go func() {
		defer pending.Done()
		_, observation.statusSHA256, observation.dirtyErr = readStatusSnapshot(ctx, index.Root)
	}()
	identity(&observation)
	pending.Wait()
	return observation
}

func historyIdentity(ctx context.Context, index *Index) (string, string, error) {
	raw, err := git(ctx, index.Root, maxIdentityBytes, nil, "rev-parse", "HEAD^{commit}", "HEAD^{tree}")
	if err != nil {
		return "", "", err
	}
	return parseHistoryIdentity(raw, index.ObjectFormat)
}

func parseHistoryIdentity(raw []byte, objectFormat string) (string, string, error) {
	lines := strings.Split(string(raw), "\n")
	if len(lines) != 3 || lines[2] != "" || !validObjectID(lines[0], objectFormat) || !validObjectID(lines[1], objectFormat) {
		return "", "", &Error{Message: "Git returned an invalid history object identity"}
	}
	return lines[0], lines[1], nil
}

func parseHistory(raw []byte, objectFormat string) ([]historyEntry, []any, error) {
	entries := make([]historyEntry, 0, maxHistoryCommits)
	canonical := make([]any, 0, maxHistoryCommits)
	for _, record := range bytes.Split(raw, []byte{0x1e}) {
		if len(record) == 0 {
			continue
		}
		fields := bytes.Split(record, []byte{0})
		header := bytes.Trim(fields[0], "\n")
		parts := bytes.SplitN(header, []byte{0x1f}, 3)
		if len(parts) != 3 || !validObjectID(string(parts[0]), objectFormat) || !validObjectID(string(parts[1]), objectFormat) {
			return nil, nil, &Error{Message: "invalid Git history record"}
		}
		paths := make([]string, 0, len(fields)-1)
		pathValues := make([]any, 0, len(fields)-1)
		for _, rawPath := range fields[1:] {
			rawPath = bytes.Trim(rawPath, "\n")
			if len(rawPath) == 0 {
				continue
			}
			if !utf8.Valid(rawPath) {
				return nil, nil, &Error{Code: "unsupported-query-history", Message: "native Go authority-start query requires UTF-8 Git history paths"}
			}
			value := string(rawPath)
			paths = append(paths, value)
			pathValues = append(pathValues, value)
		}
		subject := decodePythonUTF8(parts[2])
		entry := historyEntry{commit: string(parts[0]), tree: string(parts[1]), subject: subject, paths: paths}
		entries = append(entries, entry)
		canonical = append(canonical, []any{entry.commit, entry.tree, entry.subject, pathValues})
		if len(entries) > maxHistoryCommits {
			return nil, nil, &Error{Message: "Git history exceeds its commit limit"}
		}
	}
	return entries, canonical, nil
}

// decodePythonUTF8 matches bytes.decode("utf-8", "replace"): malformed
// prefixes are replaced separately, while a valid prefix of a truncated or
// interrupted multibyte sequence is replaced once as a unit.
func decodePythonUTF8(value []byte) string {
	if utf8.Valid(value) {
		return string(value)
	}
	var output strings.Builder
	output.Grow(len(value))
	for offset := 0; offset < len(value); {
		first := value[offset]
		if first < utf8.RuneSelf {
			output.WriteByte(first)
			offset++
			continue
		}
		width := 0
		switch {
		case first >= 0xc2 && first <= 0xdf:
			width = 2
		case first >= 0xe0 && first <= 0xef:
			width = 3
		case first >= 0xf0 && first <= 0xf4:
			width = 4
		default:
			output.WriteRune(utf8.RuneError)
			offset++
			continue
		}
		consumed := 1
		if offset+1 < len(value) && validPythonSecondByte(first, value[offset+1]) {
			consumed = 2
			if width >= 3 && offset+2 < len(value) && continuationByte(value[offset+2]) {
				consumed = 3
				if width == 4 && offset+3 < len(value) && continuationByte(value[offset+3]) {
					consumed = 4
				}
			}
		}
		if consumed == width {
			output.Write(value[offset : offset+width])
		} else {
			output.WriteRune(utf8.RuneError)
		}
		offset += consumed
	}
	return output.String()
}

// DecodePythonUTF8 exposes the oracle-compatible decoder to scoped CLI adapters.
func DecodePythonUTF8(value []byte) string { return decodePythonUTF8(value) }

// TrimPythonSpace matches str.strip() for Python's Unicode whitespace set.
func TrimPythonSpace(value string) string {
	return strings.TrimFunc(value, func(character rune) bool {
		return unicode.IsSpace(character) || character >= 0x1c && character <= 0x1f
	})
}

func validPythonSecondByte(first, second byte) bool {
	if !continuationByte(second) {
		return false
	}
	switch first {
	case 0xe0:
		return second >= 0xa0
	case 0xed:
		return second <= 0x9f
	case 0xf0:
		return second >= 0x90
	case 0xf4:
		return second <= 0x8f
	default:
		return true
	}
}

func continuationByte(value byte) bool { return value >= 0x80 && value <= 0xbf }

func containsSecret(value string) bool {
	return secretscreen.MatchString(pythonIgnoreCaseSpecials.Replace(value))
}
