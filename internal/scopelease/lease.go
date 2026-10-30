// Package scopelease implements bounded local scope leases so several agents
// sharing one worktree cannot silently edit overlapping files or the same
// ticket. Lease state is private derived state under .corvint/leases; it is never
// an input to ranking, learning, evidence, or authority, and it enforces
// nothing about the edits an agent actually makes.
package scopelease

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// SchemaVersion is the on-disk lease document version.
const SchemaVersion = 1

const (
	// MaxTTL bounds one acquisition or renewal.
	MaxTTL = 24 * time.Hour
	// StaleLockAge is the age after which a lock file is treated as crashed.
	StaleLockAge = 30 * time.Second
	// LockWait bounds how long a writer waits for the single-writer lock.
	LockWait = 2 * time.Second

	lockPoll   = 5 * time.Millisecond
	timeLayout = time.RFC3339Nano

	lockName         = ".lock"
	unreadableReason = "unreadable-lease"
)

// Sentinel errors. Callers use errors.Is.
var (
	ErrConflict       = errors.New("scopelease: scope conflict")
	ErrNotFound       = errors.New("scopelease: lease not found")
	ErrHolderMismatch = errors.New("scopelease: holder does not hold this lease")
	ErrLockBusy       = errors.New("scopelease: lease lock is held by another writer")
	// ErrInvalidRequest marks a caller-supplied argument refused before any
	// lease state is read; the refusal keeps its own message.
	ErrInvalidRequest = errors.New("scopelease: invalid request")
)

type requestError string

func (e requestError) Error() string      { return string(e) }
func (requestError) Is(target error) bool { return target == ErrInvalidRequest }

func invalidRequest(format string, arguments ...any) error {
	return requestError(fmt.Sprintf(format, arguments...))
}

// Lease is one recorded local scope claim.
type Lease struct {
	SchemaVersion int      `json:"schema_version"`
	ID            string   `json:"lease_id"`
	Holder        string   `json:"holder"`
	Paths         []string `json:"paths"`
	Ticket        string   `json:"ticket"`
	Note          string   `json:"note"`
	AcquiredAt    string   `json:"acquired_at"`
	ExpiresAt     string   `json:"expires_at"`
	Revision      string   `json:"revision"`
}

// Conflict explains why a scope could not be claimed, or why a touched path is
// not covered by exactly one live lease.
type Conflict struct {
	LeaseID string `json:"lease_id"`
	Holder  string `json:"holder"`
	Reason  string `json:"reason"`
	Scope   string `json:"scope"`
}

// Report pairs a lease with its reported state: "live" or "expired".
type Report struct {
	Lease Lease  `json:"lease"`
	State string `json:"state"`
}

// Request is one acquisition input.
type Request struct {
	Holder string
	Paths  []string
	Ticket string
	Note   string
	TTL    time.Duration
}

// Directory is the private lease directory under root.
func Directory(root string) string {
	return filepath.Join(root, ".corvint", "leases")
}

// Acquire reaps expired leases, refuses on any overlap with a live lease, and
// otherwise writes one new lease document.
func Acquire(root string, request Request) (Lease, []Conflict, error) {
	normalized, err := normalizeRequest(request)
	if err != nil {
		return Lease{}, nil, err
	}
	directory := Directory(root)
	if err := refuseLinkedDirectory(root); err != nil {
		return Lease{}, nil, err
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return Lease{}, nil, fmt.Errorf("scopelease: create lease directory: %w", err)
	}
	if err := refuseLinkedDirectory(root); err != nil {
		return Lease{}, nil, err
	}
	unlock, err := lock(directory)
	if err != nil {
		return Lease{}, nil, err
	}
	defer unlock()
	active, unreadable, err := reap(directory, time.Now().UTC())
	if err != nil {
		return Lease{}, nil, err
	}
	conflicts := conflictsWith(active, unreadable, normalized)
	if len(conflicts) != 0 {
		return Lease{}, conflicts, ErrConflict
	}
	acquired := time.Now().UTC()
	lease := Lease{
		SchemaVersion: SchemaVersion,
		ID:            identifier(normalized, acquired),
		Holder:        normalized.Holder,
		Paths:         normalized.Paths,
		Ticket:        normalized.Ticket,
		Note:          normalized.Note,
		AcquiredAt:    acquired.Format(timeLayout),
		ExpiresAt:     acquired.Add(normalized.TTL).Format(timeLayout),
		Revision:      revision(root),
	}
	if err := write(directory, lease); err != nil {
		return Lease{}, nil, err
	}
	return lease, nil, nil
}

// Release reaps expired leases and removes one lease held by holder.
func Release(root, id, holder string) (Lease, error) {
	return mutateOne(root, id, holder, func(lease Lease, directory string) (Lease, error) {
		if err := os.Remove(filepath.Join(directory, lease.ID+".json")); err != nil {
			return Lease{}, fmt.Errorf("scopelease: remove lease: %w", err)
		}
		if err := syncDirectory(directory); err != nil {
			return Lease{}, err
		}
		return lease, nil
	})
}

// Renew reaps expired leases and extends one lease held by holder to now plus
// ttl, never earlier than its recorded expiry (decision 0254).
func Renew(root, id, holder string, ttl time.Duration) (Lease, error) {
	if err := checkTTL(ttl); err != nil {
		return Lease{}, err
	}
	return mutateOne(root, id, holder, func(lease Lease, directory string) (Lease, error) {
		lease.ExpiresAt = laterExpiry(lease.ExpiresAt, time.Now().UTC().Add(ttl)).Format(timeLayout)
		if err := write(directory, lease); err != nil {
			return Lease{}, err
		}
		return lease, nil
	})
}

// Status reports one lease without writing anything, including without reaping.
func Status(root, id string) (Report, error) {
	if err := checkIdentifier(id); err != nil {
		return Report{}, err
	}
	lease, err := read(filepath.Join(Directory(root), id+".json"))
	if errors.Is(err, fs.ErrNotExist) {
		return Report{}, ErrNotFound
	}
	if err != nil {
		return Report{}, err
	}
	return report(lease, time.Now().UTC()), nil
}

// List reports every lease without writing anything, including without reaping.
// An unreadable document is reported by lease id with state "unreadable".
func List(root string) ([]Report, error) {
	leases, unreadable, err := load(Directory(root))
	if err != nil {
		return nil, err
	}
	moment := time.Now().UTC()
	reports := make([]Report, 0, len(leases)+len(unreadable))
	for _, lease := range leases {
		reports = append(reports, report(lease, moment))
	}
	for _, id := range unreadable {
		reports = append(reports, Report{Lease: Lease{ID: id, Paths: []string{}}, State: "unreadable"})
	}
	sort.Slice(reports, func(first, second int) bool { return reports[first].Lease.ID < reports[second].Lease.ID })
	return reports, nil
}

// Check is a read-only helper for a future CEM or dogfood gate: it reports,
// for each touched path, whether exactly one live lease covers it. A path that
// no live lease covers, or that two or more live leases cover, is a conflict.
// It is wired to nothing else and writes nothing.
func Check(root string, paths []string) ([]Conflict, error) {
	leases, unreadable, err := load(Directory(root))
	if err != nil {
		return nil, err
	}
	moment := time.Now().UTC()
	conflicts := make([]Conflict, 0)
	for _, raw := range paths {
		path, err := normalizePath(raw)
		if err != nil {
			return nil, err
		}
		for _, id := range unreadable {
			conflicts = append(conflicts, Conflict{LeaseID: id, Reason: unreadableReason, Scope: path})
		}
		holders := make([]Lease, 0, 1)
		for _, lease := range leases {
			if !live(lease, moment) {
				continue
			}
			if covers(lease, path) {
				holders = append(holders, lease)
			}
		}
		switch len(holders) {
		case 1:
			continue
		case 0:
			conflicts = append(conflicts, Conflict{Reason: "uncovered", Scope: path})
		default:
			for _, lease := range holders {
				conflicts = append(conflicts, Conflict{
					LeaseID: lease.ID, Holder: lease.Holder, Reason: "multiple-leases", Scope: path,
				})
			}
		}
	}
	return conflicts, nil
}

func mutateOne(root, id, holder string, apply func(Lease, string) (Lease, error)) (Lease, error) {
	if err := checkIdentifier(id); err != nil {
		return Lease{}, err
	}
	if strings.TrimSpace(holder) == "" {
		return Lease{}, invalidRequest("scopelease: holder must not be empty")
	}
	directory := Directory(root)
	if err := refuseLinkedDirectory(root); err != nil {
		return Lease{}, err
	}
	if _, err := os.Stat(directory); errors.Is(err, fs.ErrNotExist) {
		return Lease{}, ErrNotFound
	}
	unlock, err := lock(directory)
	if err != nil {
		return Lease{}, err
	}
	defer unlock()
	if _, _, err := reap(directory, time.Now().UTC()); err != nil {
		return Lease{}, err
	}
	lease, err := read(filepath.Join(directory, id+".json"))
	if errors.Is(err, fs.ErrNotExist) {
		return Lease{}, ErrNotFound
	}
	if err != nil {
		return Lease{}, err
	}
	if lease.Holder != holder {
		return Lease{}, ErrHolderMismatch
	}
	return apply(lease, directory)
}

// refuseLinkedDirectory rejects a `.corvint` or `.corvint/leases` that exists as
// anything but a real directory. A repository can commit either as a symlink,
// and following it would lock, write, and reap outside the worktree
// (SCL-V0-008). A component that does not exist yet is fine.
func refuseLinkedDirectory(root string) error {
	for _, component := range []string{".corvint", filepath.Join(".corvint", "leases")} {
		info, err := os.Lstat(filepath.Join(root, component))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("scopelease: inspect lease directory: %w", err)
		}
		if !info.IsDir() {
			return fmt.Errorf("scopelease: %s must be a real directory, not a symlink or file", filepath.ToSlash(component))
		}
	}
	return nil
}

// conflictsWith treats every unreadable document as overlapping the request:
// its scope is undecidable, so the conservative rule refuses the claim.
func conflictsWith(live []Lease, unreadable []string, request Request) []Conflict {
	conflicts := make([]Conflict, 0)
	for _, id := range unreadable {
		conflicts = append(conflicts, Conflict{LeaseID: id, Reason: unreadableReason})
	}
	for _, lease := range live {
		if request.Ticket != "" && lease.Ticket == request.Ticket {
			conflicts = append(conflicts, Conflict{
				LeaseID: lease.ID, Holder: lease.Holder, Reason: "ticket", Scope: request.Ticket,
			})
		}
		for _, held := range lease.Paths {
			for _, wanted := range request.Paths {
				if !overlaps(held, wanted) {
					continue
				}
				conflicts = append(conflicts, Conflict{
					LeaseID: lease.ID, Holder: lease.Holder, Reason: "path-overlap", Scope: held,
				})
			}
		}
	}
	sort.Slice(conflicts, func(first, second int) bool {
		if conflicts[first].LeaseID != conflicts[second].LeaseID {
			return conflicts[first].LeaseID < conflicts[second].LeaseID
		}
		if conflicts[first].Reason != conflicts[second].Reason {
			return conflicts[first].Reason < conflicts[second].Reason
		}
		return conflicts[first].Scope < conflicts[second].Scope
	})
	return conflicts
}

// overlaps is conservative: two globs overlap when they are literal-equal, or
// when the literal path prefix of either is a path prefix of the other. A glob
// whose literal prefix is empty overlaps every scope.
func overlaps(first, second string) bool {
	// Decision 0168: a case-insensitive volume names one file for both
	// spellings, so case-fold-equal scopes overlap on every host.
	first, second = strings.ToLower(first), strings.ToLower(second)
	if first == second {
		return true
	}
	firstPrefix, secondPrefix := literalPrefix(first), literalPrefix(second)
	if prefixOverlap(firstPrefix, secondPrefix) {
		return true
	}
	// Decision 0168 addendum, 2026-09-12: APFS is also normalization-
	// insensitive, so an NFC and an NFD spelling of one non-ASCII path name
	// the same file but compare byte-unequal here. Go's standard library
	// carries no normalization table, and golang.org/x/text is not a module
	// dependency, so the two spellings cannot be proven equal or distinct.
	// SCL-V0-002 already treats an undecidable comparison as an overlap:
	// any non-ASCII byte in either compared literal prefix makes the
	// comparison undecidable, so it fails closed.
	return containsNonASCII(firstPrefix) || containsNonASCII(secondPrefix)
}

// containsNonASCII reports whether value has any byte outside the 7-bit ASCII
// range. It is a byte scan, not a rune decode: a truncated or invalid UTF-8
// sequence still contains a byte >= 0x80, so it is still flagged undecidable.
func containsNonASCII(value string) bool {
	for index := 0; index < len(value); index++ {
		if value[index] >= utf8.RuneSelf {
			return true
		}
	}
	return false
}

func prefixOverlap(first, second string) bool {
	if first == "" || second == "" {
		return true
	}
	if first == second {
		return true
	}
	return strings.HasPrefix(first, second+"/") || strings.HasPrefix(second, first+"/")
}

// literalPrefix is the longest leading run of whole path segments that contains
// no glob metacharacter.
func literalPrefix(pattern string) string {
	segments := strings.Split(pattern, "/")
	kept := make([]string, 0, len(segments))
	for _, segment := range segments {
		if strings.ContainsAny(segment, "*?[]{}\\") {
			break
		}
		kept = append(kept, segment)
	}
	return strings.Join(kept, "/")
}

// covers reports whether one concrete repository-relative path falls inside a
// lease scope. Coverage is glob matching only (SCL-V0-009): a literal
// directory scope covers that exact path, not the files beneath it, which
// need a `dir/**` scope. Unlike overlaps, which widens to prefixes because a
// false overlap only refuses a claim, covers stays narrow because an
// `uncovered` report is Check's conservative direction.
func covers(lease Lease, path string) bool {
	for _, scope := range lease.Paths {
		if scope == path || matchSegments(strings.Split(scope, "/"), strings.Split(path, "/")) {
			return true
		}
	}
	return false
}

// matchSegments matches scope against target one path segment at a time. A
// whole `**` segment matches zero or more segments (at least one when it is
// last); any other segment matches exactly one segment by path.Match, so `*`
// never crosses `/`. It runs in O(len(scope) * len(target)) time: reached[j]
// records whether the scope segments consumed so far match target[:j].
func matchSegments(scope, target []string) bool {
	reached := make([]bool, len(target)+1)
	reached[0] = true
	for index, segment := range scope {
		reached = advanceSegment(reached, segment, index == len(scope)-1, target)
	}
	return reached[len(target)]
}

// advanceSegment returns which target prefixes are matched after one more
// scope segment, given the prefixes matched before it.
func advanceSegment(reached []bool, segment string, last bool, target []string) []bool {
	next := make([]bool, len(reached))
	if segment == "**" {
		return advanceDoubleStar(reached, next, last)
	}
	for end := 1; end < len(reached); end++ {
		matched, err := path.Match(segment, target[end-1])
		next[end] = reached[end-1] && err == nil && matched
	}
	return next
}

// advanceDoubleStar lets `**` extend any matched prefix by zero or more
// segments, or by at least one when it is the last scope segment.
func advanceDoubleStar(reached, next []bool, last bool) []bool {
	seen := false
	for end := range reached {
		if !last {
			seen = seen || reached[end]
		}
		next[end] = seen
		seen = seen || reached[end]
	}
	return next
}

func normalizeRequest(request Request) (Request, error) {
	if strings.TrimSpace(request.Holder) == "" {
		return Request{}, invalidRequest("scopelease: holder must not be empty")
	}
	if err := checkTTL(request.TTL); err != nil {
		return Request{}, err
	}
	if len(request.Paths) == 0 && request.Ticket == "" {
		return Request{}, invalidRequest("scopelease: at least one --path or --ticket is required")
	}
	seen := make(map[string]struct{}, len(request.Paths))
	paths := make([]string, 0, len(request.Paths))
	for _, raw := range request.Paths {
		path, err := normalizePath(raw)
		if err != nil {
			return Request{}, err
		}
		if _, duplicate := seen[path]; duplicate {
			continue
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}
	sort.Strings(paths)
	request.Paths = paths
	if !fitsDocument(request) {
		return Request{}, invalidRequest("scopelease: lease document would exceed %d bytes", documentLimit)
	}
	return request, nil
}

func normalizePath(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || filepath.IsAbs(trimmed) {
		return "", invalidRequest("scopelease: path must be repository-relative: %q", value)
	}
	slashed := filepath.ToSlash(trimmed)
	for _, segment := range strings.Split(slashed, "/") {
		if segment == ".." {
			return "", invalidRequest("scopelease: path must be repository-relative: %q", value)
		}
	}
	if _, err := filepath.Match(slashed, "probe"); err != nil {
		return "", invalidRequest("scopelease: invalid path pattern: %q", value)
	}
	// Equivalent spellings ("./a", "a//b", "a/./b", "a/") must compare equal.
	cleaned := path.Clean(slashed)
	if cleaned == "" || cleaned == "." {
		return "", invalidRequest("scopelease: path must be repository-relative: %q", value)
	}
	return cleaned, nil
}

func checkTTL(ttl time.Duration) error {
	if ttl <= 0 || ttl > MaxTTL {
		return invalidRequest("scopelease: ttl must be positive and at most %s", MaxTTL)
	}
	return nil
}

func checkIdentifier(id string) error {
	if len(id) != 16 {
		return invalidRequest("scopelease: invalid lease id: %q", id)
	}
	if _, err := hex.DecodeString(id); err != nil {
		return invalidRequest("scopelease: invalid lease id: %q", id)
	}
	return nil
}

func report(lease Lease, moment time.Time) Report {
	if live(lease, moment) {
		return Report{Lease: lease, State: "live"}
	}
	return Report{Lease: lease, State: "expired"}
}

// laterExpiry is the later of a recorded expiry and a renewal's. Reaping has
// already removed a lease whose recorded expiry does not parse.
func laterExpiry(recorded string, renewal time.Time) time.Time {
	expiry, err := time.Parse(timeLayout, recorded)
	if err != nil {
		return renewal
	}
	if renewal.After(expiry) {
		return renewal
	}
	return expiry
}

// live treats an unparsable expiry as expired; a lease can never outlive the
// document that describes it.
func live(lease Lease, moment time.Time) bool {
	expiry, err := time.Parse(timeLayout, lease.ExpiresAt)
	if err != nil {
		return false
	}
	return moment.Before(expiry)
}
