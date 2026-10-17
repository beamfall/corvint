// Package defects is the TDR-V0 defect register: an append-only JSON-lines
// file of suspected and confirmed product and environment faults found
// during test work. Each line holds one defect's full state; a later line
// for the same id overrides an earlier one. Writes serialize on a sibling
// lock file; reads take no lock and ignore an unterminated last line.
package defects

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/obligation"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Profile is the profile of every register line (TDR-V0-002).
const Profile = "taskman-defect/0"

// Register location and bounds (TDR-V0-005, TDR-V0-009).
const (
	PrivateDir       = "taskman-defects"
	RegisterFile     = "register.jsonl"
	MaxRegisterBytes = 8 << 20
	MaxTextBytes     = 2048
	MaxDuplicateHops = 32
)

// Line operations (TDR-V0-004).
const (
	OpAdd    = "ADD"
	OpUpdate = "UPDATE"
)

// Statuses, categories and severities (TDR-V0-003).
const (
	StatusSuspected = "SUSPECTED"
	StatusConfirmed = "CONFIRMED"
	StatusNotABug   = "NOT_A_BUG"
	StatusDuplicate = "DUPLICATE"

	CategoryProduct     = "PRODUCT"
	CategoryEnvironment = "ENVIRONMENT"

	SeverityLow      = "LOW"
	SeverityMedium   = "MEDIUM"
	SeverityHigh     = "HIGH"
	SeverityCritical = "CRITICAL"
)

var (
	Statuses   = []string{StatusConfirmed, StatusDuplicate, StatusNotABug, StatusSuspected}
	Categories = []string{CategoryEnvironment, CategoryProduct}
	Severities = []string{SeverityCritical, SeverityHigh, SeverityLow, SeverityMedium}

	idShape = regexp.MustCompile(`^([A-Z][A-Z0-9]{0,15})-([1-9][0-9]{0,8})$`)
)

// SecretDetail prefixes the refusal of a defect write whose text matches the
// shared secret screen (TDR-V0-006); the detail names the field only.
const SecretDetail = "DEFECT_SECRET_SCREENED"

// Defect is one defect's current state.
type Defect struct {
	ID          string
	Title       string
	Location    string
	Evidence    string
	Severity    string
	Status      string
	Category    string
	DuplicateOf *string
	RecordedAt  wire.Timestamp
	Revision    int
}

// Value renders the defect as a result item.
func (d *Defect) Value() wire.Value {
	return wire.ObjectValue(wire.NewObject().Set("id", wire.String(d.ID)).Set("title", wire.String(d.Title)).
		Set("location", wire.String(d.Location)).Set("evidence", wire.String(d.Evidence)).
		Set("severity", wire.String(d.Severity)).Set("status", wire.String(d.Status)).
		Set("category", wire.String(d.Category)).Set("duplicateOf", wire.StringOrNull(d.DuplicateOf)).
		Set("recordedAt", wire.String(string(d.RecordedAt))).Set("revision", wire.String(strconv.Itoa(d.Revision))))
}

func (d *Defect) line(op string) []byte {
	return wire.EncodeFile(wire.ObjectValue(wire.NewObject().Set("profile", wire.String(Profile)).Set("op", wire.String(op)).
		Set("id", wire.String(d.ID)).Set("title", wire.String(d.Title)).Set("location", wire.String(d.Location)).
		Set("evidence", wire.String(d.Evidence)).Set("severity", wire.String(d.Severity)).Set("status", wire.String(d.Status)).
		Set("category", wire.String(d.Category)).Set("duplicateOf", wire.StringOrNull(d.DuplicateOf)).
		Set("recordedAt", wire.String(string(d.RecordedAt)))))
}

// Register is the folded register: the latest state of every defect in
// first-added order.
type Register struct {
	Path    string
	Private bool
	Exists  bool
	Lines   int
	order   []string
	byID    map[string]*Defect
}

// Defects returns every defect in first-added order.
func (r *Register) Defects() []*Defect {
	out := make([]*Defect, 0, len(r.order))
	for _, id := range r.order {
		out = append(out, r.byID[id])
	}
	return out
}

// Get returns the defect with id, or nil.
func (r *Register) Get(id string) *Defect {
	if r == nil {
		return nil
	}
	return r.byID[id]
}

// Resolve follows DUPLICATE links from id to the defect they end at, or nil
// when id is unknown, a link is dangling, or the chain loops (TDR-V0-008).
func (r *Register) Resolve(id string) *Defect {
	d := r.Get(id)
	for hops := 0; d != nil && d.Status == StatusDuplicate; hops++ {
		if hops >= MaxDuplicateHops || d.DuplicateOf == nil {
			return nil
		}
		d = r.Get(*d.DuplicateOf)
	}
	return d
}

// Location returns the register path for repo and policy, and whether it is
// the private default under the Git common directory (TDR-V0-009). The
// private register is outside every worktree and the Tasks state directory,
// so it is never committed, pushed or archived.
func Location(repo *intent.Repository, p *intent.Policy) (string, bool) {
	if p != nil && p.Defects != nil && p.Defects.Path != nil {
		return filepath.Join(repo.PrimaryWorktree, filepath.FromSlash(*p.Defects.Path)), false
	}
	return filepath.Join(repo.CommonDir, PrivateDir, RegisterFile), true
}

// Read folds the register at path without taking the lock or creating
// anything (TDR-V0-005, TDR-V0-010). A missing register is empty; an
// unterminated last line is a write in progress and is ignored.
func Read(path string, private bool) (*Register, error) {
	reg := &Register{Path: path, Private: private, byID: map[string]*Defect{}}
	raw, err := intent.ReadFile(path, MaxRegisterBytes)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return reg, nil
		}
		if _, serr := os.Lstat(path); errors.Is(serr, fs.ErrNotExist) {
			return reg, nil
		}
		return nil, err
	}
	reg.Exists = true
	if i := strings.LastIndexByte(string(raw), '\n'); i >= 0 {
		raw = raw[:i+1]
	} else {
		raw = nil
	}
	for n, line := range strings.SplitAfter(string(raw), "\n") {
		if line == "" {
			continue
		}
		d, op, err := decodeLine([]byte(line), "/lines/"+strconv.Itoa(n))
		if err != nil {
			return nil, err
		}
		if err := reg.apply(d, op); err != nil {
			return nil, err
		}
	}
	return reg, nil
}

func (r *Register) apply(d *Defect, op string) error {
	prev := r.byID[d.ID]
	switch {
	case op == OpAdd && prev != nil:
		return wire.Errorf(wire.CodeMalformed, d.ID, "register line %d adds %s twice", r.Lines+1, d.ID)
	case op == OpUpdate && prev == nil:
		return wire.Errorf(wire.CodeMalformed, d.ID, "register line %d updates unknown %s", r.Lines+1, d.ID)
	case prev == nil:
		r.order = append(r.order, d.ID)
	default:
		d.Revision = prev.Revision
	}
	d.Revision++
	r.byID[d.ID] = d
	r.Lines++
	return nil
}

func decodeLine(raw []byte, where string) (*Defect, string, error) {
	v, err := wire.Parse(raw)
	if err != nil {
		return nil, "", wire.Errorf(wire.CodeMalformed, where, "register line is not one JSON object")
	}
	r := wire.NewReader(v, where)
	// The version is checked first, so a line from a later build refuses
	// UNSUPPORTED_VERSION rather than MALFORMED for its new members.
	if err := wire.ProfileVersion(where+"/profile", v, Profile); err != nil {
		return nil, "", err
	}
	r.Closed("profile", "op", "id", "title", "location", "evidence", "severity", "status", "category", "duplicateOf", "recordedAt")
	if err := wire.CheckProfile(where+"/profile", r.Field("profile").String(), Profile); err != nil {
		return nil, "", err
	}
	op := r.Field("op").Enum(OpAdd, OpUpdate)
	d := &Defect{ID: r.Field("id").String(), Title: r.Field("title").String(), Location: r.Field("location").String(),
		Evidence: r.Field("evidence").String(), Severity: r.Field("severity").Enum(Severities...),
		Status: r.Field("status").Enum(Statuses...), Category: r.Field("category").Enum(Categories...),
		DuplicateOf: r.Field("duplicateOf").StringOrNull(func(x *wire.Reader) string { return x.String() }),
		RecordedAt:  r.Field("recordedAt").Timestamp()}
	if err := r.Err(); err != nil {
		return nil, "", err
	}
	if !idShape.MatchString(d.ID) {
		return nil, "", wire.Errorf(wire.CodeMalformed, where+"/id", "defect id must be PREFIX-N")
	}
	return d, op, nil
}

// Fields are the caller-supplied fields of an add or update; nil leaves an
// update's field unchanged.
type Fields struct {
	Title, Location, Evidence, Severity, Status, Category, DuplicateOf *string
}

// Add appends a new defect with the next id of prefix (TDR-V0-002). It holds
// the register lock across read, validate and append (TDR-V0-005).
func Add(path string, private bool, prefix string, f Fields, now wire.Timestamp) (*Defect, error) {
	var out *Defect
	err := locked(path, private, func(reg *Register) ([]byte, error) {
		d := &Defect{Severity: SeverityMedium, Status: StatusSuspected, Category: CategoryProduct, RecordedAt: now}
		if f.Title == nil || f.Location == nil || f.Evidence == nil {
			return nil, wire.Errorf(wire.CodeMalformed, "--title", "add needs --title, --location and --evidence")
		}
		next := 0
		for _, id := range reg.order {
			if m := idShape.FindStringSubmatch(id); m != nil && m[1] == prefix {
				if n, _ := strconv.Atoi(m[2]); n > next {
					next = n
				}
			}
		}
		d.ID = prefix + "-" + strconv.Itoa(next+1)
		if err := set(reg, d, f); err != nil {
			return nil, err
		}
		if err := reg.apply(d, OpAdd); err != nil {
			return nil, err
		}
		out = d
		return d.line(OpAdd), nil
	})
	return out, err
}

// Update appends the new full state of an existing defect; the earlier lines
// stay (TDR-V0-004).
func Update(path string, private bool, id string, f Fields, now wire.Timestamp) (*Defect, error) {
	var out *Defect
	err := locked(path, private, func(reg *Register) ([]byte, error) {
		prev := reg.Get(id)
		if prev == nil {
			return nil, wire.Errorf(wire.CodeMissingEvidence, id, "the register holds no defect %s", id)
		}
		d := *prev
		d.RecordedAt = now
		if f.Status != nil && *f.Status != StatusDuplicate && f.DuplicateOf == nil {
			d.DuplicateOf = nil
		}
		if err := set(reg, &d, f); err != nil {
			return nil, err
		}
		if err := reg.apply(&d, OpUpdate); err != nil {
			return nil, err
		}
		out = &d
		return d.line(OpUpdate), nil
	})
	return out, err
}

// set applies f to d and validates the result against reg.
func set(reg *Register, d *Defect, f Fields) error {
	for _, t := range []struct {
		name string
		src  *string
		dst  *string
	}{{"--title", f.Title, &d.Title}, {"--location", f.Location, &d.Location}, {"--evidence", f.Evidence, &d.Evidence}} {
		if t.src == nil {
			continue
		}
		s := strings.TrimSpace(*t.src)
		if s == "" || len(s) > MaxTextBytes || strings.ContainsAny(s, "\x00\r\n") {
			return wire.Errorf(wire.CodeMalformed, t.name, "%s must be one line of 1..%d bytes", t.name, MaxTextBytes)
		}
		if obligation.MatchesSecretScreen(s) {
			return wire.Errorf(wire.CodeSecretDetected, t.name, "%s: %s matches the secret screen; remove the secret and retry", SecretDetail, t.name)
		}
		*t.dst = s
	}
	for _, e := range []struct {
		name    string
		src     *string
		dst     *string
		allowed []string
	}{{"--severity", f.Severity, &d.Severity, Severities}, {"--status", f.Status, &d.Status, Statuses}, {"--category", f.Category, &d.Category, Categories}} {
		if e.src == nil {
			continue
		}
		if !contains(e.allowed, *e.src) {
			return wire.Errorf(wire.CodeMalformed, e.name, "%s must be one of %s", e.name, strings.Join(e.allowed, ", "))
		}
		*e.dst = *e.src
	}
	if f.DuplicateOf != nil {
		dup := *f.DuplicateOf
		d.DuplicateOf = &dup
	}
	if d.Status != StatusDuplicate {
		if d.DuplicateOf != nil {
			return wire.Errorf(wire.CodeMalformed, "--duplicate-of", "--duplicate-of needs --status DUPLICATE")
		}
		return nil
	}
	if d.DuplicateOf == nil || *d.DuplicateOf == d.ID || reg.Get(*d.DuplicateOf) == nil {
		return wire.Errorf(wire.CodeMalformed, "--duplicate-of", "a DUPLICATE names another defect in the register with --duplicate-of")
	}
	for cur, hops := reg.Get(*d.DuplicateOf), 0; cur != nil && cur.Status == StatusDuplicate && cur.DuplicateOf != nil; hops++ {
		if *cur.DuplicateOf == d.ID || hops >= MaxDuplicateHops {
			return wire.Errorf(wire.CodeMalformed, "--duplicate-of", "the duplicate link would form a cycle")
		}
		cur = reg.Get(*cur.DuplicateOf)
	}
	return nil
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// locked runs fn over the register read under the exclusive lock and appends
// the line it returns, synced before the lock is released (TDR-V0-005).
func locked(path string, private bool, fn func(*Register) ([]byte, error)) error {
	mode := os.FileMode(0o755)
	if private {
		mode = 0o700
	}
	// Readers refuse a symlinked ancestor, so a writer refuses one too,
	// before and after creating the directory: a symlinked parent never
	// places the register outside its root (TDR-V0-009).
	dir := filepath.Dir(path)
	if err := noSymlinkedAncestor(dir); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, mode); err != nil {
		return wire.Errorf(wire.CodeUnsupportedFilesystem, "/defects/path", "cannot create the register directory: %v", err)
	}
	if err := noSymlinkedAncestor(dir); err != nil {
		return err
	}
	lock, err := os.OpenFile(path+".lock", os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return wire.Errorf(wire.CodeUnsupportedFilesystem, "/defects/path", "cannot open the register lock: %v", err)
	}
	defer lock.Close()
	if err := lockFile(lock); err != nil {
		return wire.Errorf(wire.CodeUnsupported, "/defects/path", "cannot lock the register: %v", err)
	}
	defer unlockFile(lock)
	reg, err := Read(path, private)
	if err != nil {
		return err
	}
	line, err := fn(reg)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return wire.Errorf(wire.CodeUnsupportedFilesystem, "/defects/path", "cannot open the register: %v", err)
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return wire.Errorf(wire.CodeUnsupportedFilesystem, "/defects/path", "cannot stat the register: %v", err)
	}
	if info.Size() > 0 && !endsWithLF(path, info.Size()) {
		// Seal an unterminated tail left by an interrupted writer so the
		// new line stays whole; Read already ignored that tail.
		line = append([]byte{'\n'}, line...)
	}
	if info.Size()+int64(len(line)) > MaxRegisterBytes {
		// Every reader refuses a register over the bound, so a write that
		// would cross it refuses instead and the register stays readable.
		f.Close()
		return wire.Errorf(wire.CodeLimitExceeded, "/defects/path", "the append would grow the register past %d bytes", MaxRegisterBytes)
	}
	if _, err := f.Write(line); err != nil {
		f.Close()
		return wire.Errorf(wire.CodeUnsupportedFilesystem, "/defects/path", "cannot append to the register: %v", err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return wire.Errorf(wire.CodeUnsupportedFilesystem, "/defects/path", "cannot sync the register: %v", err)
	}
	return f.Close()
}

func endsWithLF(path string, size int64) bool {
	r, err := os.Open(path)
	if err != nil {
		return true
	}
	defer r.Close()
	b := make([]byte, 1)
	n, err := r.ReadAt(b, size-1)
	return err != nil || n != 1 || b[0] == '\n'
}

// noSymlinkedAncestor refuses UNSUPPORTED_FILESYSTEM when the existing part
// of dir resolves through a symbolic link.
func noSymlinkedAncestor(dir string) error {
	cur := filepath.Clean(dir)
	for {
		if _, err := os.Lstat(cur); err == nil {
			break
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	if real, err := filepath.EvalSymlinks(cur); err != nil || real != cur {
		return wire.Errorf(wire.CodeUnsupportedFilesystem, "/defects/path", "the register directory %s is reached through a symbolic link", dir)
	}
	return nil
}
