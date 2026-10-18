package obligation

import (
	_ "embed"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"

	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// FixtureName is the file name of the TOL-V0-060 qualification fixture; a
// report match is read as the fixture only from a file with this name.
const FixtureName = "obligation-qualification.spec.ts"

// FixturePrefix is the ledger prefix of the fixture's ids.
const FixturePrefix = "TOLQ"

// Fixture classes (TOL-V0-060).
const (
	ClassCredited        = "credited"
	ClassFailed          = "failed"
	ClassDefectConfirmed = "defectConfirmed"
)

//go:embed obligation-qualification.spec.ts
var fixture []byte

// fixtureExpected is each fixture id's expected class, in id order.
var fixtureExpected = []struct{ id, class string }{
	{"TOLQ-1", ClassCredited},
	{"TOLQ-2", ClassFailed},
	{"TOLQ-3", ClassCredited},
	{"TOLQ-4", ClassFailed},
	{"TOLQ-5", ClassFailed},
	{"TOLQ-6", ClassCredited},
	{"TOLQ-7", ClassFailed},
	{"TOLQ-8", ClassCredited},
	{"TOLQ-9", ClassDefectConfirmed},
}

// Fixture returns a copy of the embedded fixture text.
func Fixture() []byte { return append([]byte(nil), fixture...) }

// FixtureSha256 is the digest of the embedded fixture.
func FixtureSha256() wire.Digest { return wire.Sum(fixture) }

// FixtureExpected returns the sorted ids the fixture expects in each class.
func FixtureExpected() (credited, failed, defectConfirmed []string) {
	for _, e := range fixtureExpected {
		switch e.class {
		case ClassCredited:
			credited = append(credited, e.id)
		case ClassFailed:
			failed = append(failed, e.id)
		default:
			defectConfirmed = append(defectConfirmed, e.id)
		}
	}
	return credited, failed, defectConfirmed
}

// Qualification is the retained evidence of one passing TOL-V0-061 check.
type Qualification struct {
	Version         string
	ReportSha256    wire.Digest
	FixtureSha256   wire.Digest
	Credited        []string
	Failed          []string
	DefectConfirmed []string
	QualifiedAt     wire.Timestamp
}

// ReadQualificationReport reads a report of the fixture with the TOL-V0-009
// reader, admitting any version label (TOL-V0-061).
func ReadQualificationReport(file string) (*Report, error) {
	raw, err := readReportBytes(file)
	if err != nil {
		return nil, err
	}
	return parseReport(raw, nil, true)
}

func qualificationErr(f string, args ...any) error {
	return wire.Errorf(wire.CodeGateFailed, "--from-playwright-report", ticket.ObligationQualificationDetail+" "+f, args...)
}

// Qualify classifies a fixture report against a synthetic OPEN ledger of the
// fixture ids and requires each id's class to equal its expected class
// (TOL-V0-061). It reads no Tasks record and no Git object: a match is bound
// only in a file named FixtureName, read as the embedded fixture bytes.
func Qualify(r *Report) (*Qualification, error) {
	if r.errors > 0 {
		return nil, qualificationErr("the report has %d top-level errors", r.errors)
	}
	if !filepath.IsAbs(r.RootDir) {
		return nil, qualificationErr("config.rootDir is not an absolute path")
	}
	l := &ticket.ObligationLedger{Prefix: FixturePrefix}
	for _, e := range fixtureExpected {
		l.Entries = append(l.Entries, ticket.ObligationEntry{ID: e.id, Core: true, State: ticket.ObligationOpen})
	}
	sort.Slice(l.Entries, func(i, j int) bool { return l.Entries[i].ID < l.Entries[j].ID })
	root := filepath.Clean(r.RootDir)
	src := Source{Present: map[string]bool{}, Content: map[string][]byte{}}
	for _, p := range r.SourcePaths(NamesOf(l), root) {
		if path.Base(p) == FixtureName {
			src.Present[p], src.Content[p] = true, fixture
		}
	}
	res, err := Classify(r, l, root, src, nil)
	if err != nil {
		return nil, err
	}
	observed := map[string]string{}
	mark := func(class string, ids ...string) {
		for _, id := range ids {
			if _, ok := observed[id]; !ok {
				observed[id] = class
			}
		}
	}
	mark(ClassCredited, res.Credited...)
	mark(ClassFailed, res.Failed...)
	for _, d := range res.DefectConfirmed {
		mark(ClassDefectConfirmed, d.ID)
	}
	for _, m := range res.MixedExpectedFail {
		mark("mixedExpectedFail", m.ID)
	}
	for _, u := range res.Unbound {
		mark("unbound "+u.Reason, u.ID)
	}
	mark("conflicting", res.Conflicting...)
	mark("alreadyWitnessed", res.AlreadyWitnessed...)
	if len(res.Unknown) > 0 {
		return nil, qualificationErr("%s is not a fixture id", res.Unknown[0])
	}
	for _, e := range fixtureExpected {
		got, ok := observed[e.id]
		if !ok {
			got = "missing"
		}
		if got != e.class {
			return nil, qualificationErr("%s expected %s, observed %s", e.id, e.class, got)
		}
	}
	credited, failed, defects := FixtureExpected()
	return &Qualification{Version: r.Version, ReportSha256: r.Sha256, FixtureSha256: FixtureSha256(),
		Credited: credited, Failed: failed, DefectConfirmed: defects}, nil
}

// The store-local allow-list (TOL-V0-062).
const (
	ProfileQualifications = "taskman-obligation-qualifications/0"
	AllowListFile         = "taskman-obligation-qualifications.json"
	MaxQualifiedVersions  = 64
	MaxAllowListBytes     = 1 << 20
)

// AllowList is the store-local qualified version list, keyed by version.
type AllowList struct {
	Entries map[string]Qualification
}

// AllowListPath is the allow-list path beside the store's state directory.
func AllowListPath(commonDir string) string { return filepath.Join(commonDir, AllowListFile) }

func allowListErr(code, f string, args ...any) error {
	return wire.Errorf(code, AllowListFile, f, args...)
}

// ReadAllowList reads the allow-list at p through a MaxAllowListBytes bound.
// A missing file is an empty list; any other unreadable, oversize or
// non-regular file is MALFORMED, never an empty list.
func ReadAllowList(p string) (*AllowList, error) {
	st, err := os.Lstat(p)
	if errors.Is(err, fs.ErrNotExist) {
		return &AllowList{Entries: map[string]Qualification{}}, nil
	}
	if err != nil || !st.Mode().IsRegular() || st.Size() > MaxAllowListBytes {
		return nil, allowListErr(wire.CodeMalformed, "the qualification allow-list is not a readable regular file of at most %d bytes", MaxAllowListBytes)
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, allowListErr(wire.CodeMalformed, "the qualification allow-list is unreadable")
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, MaxAllowListBytes+1))
	if err != nil || len(raw) > MaxAllowListBytes {
		return nil, allowListErr(wire.CodeMalformed, "the qualification allow-list is unreadable or exceeds %d bytes", MaxAllowListBytes)
	}
	return DecodeAllowList(raw)
}

// DecodeAllowList decodes one allow-list document. A later profile refuses
// UNSUPPORTED_VERSION before any other read (CAL-V0-131).
func DecodeAllowList(raw []byte) (*AllowList, error) {
	v, err := wire.Parse(raw)
	if err != nil {
		return nil, err
	}
	r := wire.NewReader(v, "/")
	if err := r.Profile(ProfileQualifications); err != nil {
		return nil, err
	}
	r.Closed("profile", "versions")
	vs := r.Field("versions")
	if r.Err() != nil {
		return nil, r.Err()
	}
	if vs.Value().Kind != wire.KindObject || vs.Value().Obj == nil {
		return nil, allowListErr(wire.CodeMalformed, "versions is not an object")
	}
	keys := vs.Value().Obj.SortedKeys()
	if len(keys) > MaxQualifiedVersions {
		return nil, allowListErr(wire.CodeLimitExceeded, "the allow-list holds %d versions; at most %d", len(keys), MaxQualifiedVersions)
	}
	vs.Closed(keys...)
	a := &AllowList{Entries: map[string]Qualification{}}
	for _, k := range keys {
		if _, err := wire.ParseLabel("versions", k); err != nil {
			return nil, err
		}
		e := vs.Field(k)
		e.Closed("reportSha256", "fixtureSha256", "credited", "failed", "defectConfirmed", "qualifiedAt")
		q := Qualification{Version: k, ReportSha256: e.Field("reportSha256").Digest(), FixtureSha256: e.Field("fixtureSha256").Digest(),
			Credited:        e.Field("credited").Strings(len(fixtureExpected), false, (*wire.Reader).Label),
			Failed:          e.Field("failed").Strings(len(fixtureExpected), false, (*wire.Reader).Label),
			DefectConfirmed: e.Field("defectConfirmed").Strings(len(fixtureExpected), false, (*wire.Reader).Label),
			QualifiedAt:     e.Field("qualifiedAt").Timestamp()}
		if r.Err() != nil {
			return nil, r.Err()
		}
		a.Entries[k] = q
	}
	if r.Err() != nil {
		return nil, r.Err()
	}
	return a, nil
}

// Versions returns the sorted qualified versions.
func (a *AllowList) Versions() []string {
	out := make([]string, 0, len(a.Entries))
	for v := range a.Entries {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

// Put records q, replacing an earlier entry for its version. A new version
// beyond MaxQualifiedVersions refuses LIMIT_EXCEEDED.
func (a *AllowList) Put(q Qualification) error {
	if _, ok := a.Entries[q.Version]; !ok && len(a.Entries) >= MaxQualifiedVersions {
		return allowListErr(wire.CodeLimitExceeded, "the allow-list already holds %d versions; remove one first", MaxQualifiedVersions)
	}
	a.Entries[q.Version] = q
	return nil
}

func sortedStrings(ss []string) wire.Value {
	out := append([]string{}, ss...)
	sort.Strings(out)
	return wire.Strings(out)
}

// Value is the canonical allow-list document.
func (a *AllowList) Value() wire.Value {
	vs := wire.NewObject()
	for _, k := range a.Versions() {
		q := a.Entries[k]
		vs.Set(k, wire.ObjectValue(wire.NewObject().
			Set("reportSha256", wire.String(string(q.ReportSha256))).
			Set("fixtureSha256", wire.String(string(q.FixtureSha256))).
			Set("credited", sortedStrings(q.Credited)).
			Set("failed", sortedStrings(q.Failed)).
			Set("defectConfirmed", sortedStrings(q.DefectConfirmed)).
			Set("qualifiedAt", wire.String(string(q.QualifiedAt)))))
	}
	return wire.ObjectValue(wire.NewObject().Set("profile", wire.String(ProfileQualifications)).Set("versions", wire.ObjectValue(vs)))
}

// WriteAllowList writes a atomically to p: a temporary file in the same
// directory renamed over p.
func WriteAllowList(p string, a *AllowList) error {
	raw := wire.EncodeFile(a.Value())
	if len(raw) > MaxAllowListBytes {
		return allowListErr(wire.CodeLimitExceeded, "the allow-list would exceed %d bytes", MaxAllowListBytes)
	}
	f, err := os.CreateTemp(filepath.Dir(p), ".qualifications-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	_, werr := f.Write(raw)
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = os.Rename(tmp, p)
	}
	if werr != nil {
		_ = os.Remove(tmp)
		return werr
	}
	return nil
}
