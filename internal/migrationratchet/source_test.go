package migrationratchet

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// withSource attaches a source span to one record on both sides and reseals.
func withSource(profile Profile, id string, span func(Snapshot) *SourceSpan) Profile {
	for _, snapshot := range []*Snapshot{&profile.Baseline, &profile.Candidate} {
		snapshot.Records = cloneRecords(snapshot.Records)
		for i := range snapshot.Records {
			if snapshot.Records[i].ID == id {
				snapshot.Records[i].Source = span(*snapshot)
				sealRecord(&snapshot.Records[i])
			}
		}
		sealSnapshot(snapshot)
	}
	return profile
}

func coverageSource(snapshot Snapshot) *SourceSpan {
	return &SourceSpan{Commit: snapshot.Repository.Revision, Path: "tests/checkout.spec.ts", Start: 3, End: 9}
}

func TestRecordSourceSpanIsReverifiedAgainstGit(t *testing.T) {
	plain := fixtureProfile()
	encoded, _ := json.Marshal(plain.Baseline.Records[0])
	if strings.Contains(string(encoded), "source") {
		t.Fatalf("absent source changed the canonical record encoding: %s", encoded)
	}
	profile := withSource(fixtureProfile(), "coverage:checkout", coverageSource)
	content := Digest("coverage-target:coverage:checkout:content")
	calls := []string{}
	match := func(commit, path string, start, end int) (string, error) {
		calls = append(calls, commit+":"+path)
		if path != "tests/checkout.spec.ts" || start != 3 || end != 9 {
			t.Fatalf("read %s %s %d %d", commit, path, start, end)
		}
		return content, nil
	}
	receipt, err := CompareVerified(profile, match)
	if err != nil || receipt.Verdict != "pass" || len(calls) != 2 || calls[0] == calls[1] {
		t.Fatalf("verified = %v %q %v", err, receipt.Verdict, calls)
	}
	if !strings.Contains(strings.Join(receipt.Limitations, "\n"), "2 record source spans were re-read from Git") {
		t.Fatalf("limitations = %v", receipt.Limitations)
	}

	cases := map[string]struct {
		read SpanReader
		want string
	}{
		"mismatch":   {func(string, string, int, int) (string, error) { return Digest("other"), nil }, "migration-ratchet baseline source refused: records[1].source (coverage-target coverage:checkout) content_sha256 does not match Git span digest"},
		"unreadable": {func(string, string, int, int) (string, error) { return "", errors.New("source missing") }, "migration-ratchet baseline source unreadable: records[1].source (coverage-target coverage:checkout): source missing"},
		"no reader":  {nil, "migration-ratchet baseline source unverified: records[1].source (coverage-target coverage:checkout) has no Git reader"},
	}
	for name, test := range cases {
		if _, err := CompareVerified(profile, test.read); err == nil || !strings.HasPrefix(err.Error(), test.want) {
			t.Errorf("%s = %v", name, err)
		}
	}
	if _, err := Compare(profile); err == nil || !strings.Contains(err.Error(), "has no Git reader") {
		t.Fatalf("Compare without reader = %v", err)
	}

	for name, span := range map[string]func(Snapshot) *SourceSpan{
		"foreign commit": func(Snapshot) *SourceSpan { return &SourceSpan{Commit: hexID('9'), Path: "a.ts", Start: 1, End: 1} },
		"escaping path": func(s Snapshot) *SourceSpan {
			return &SourceSpan{Commit: s.Repository.Revision, Path: "../a.ts", Start: 1, End: 1}
		},
		"absolute path": func(s Snapshot) *SourceSpan {
			return &SourceSpan{Commit: s.Repository.Revision, Path: "/a.ts", Start: 1, End: 1}
		},
		"inverted span": func(s Snapshot) *SourceSpan {
			return &SourceSpan{Commit: s.Repository.Revision, Path: "a.ts", Start: 4, End: 3}
		},
		"zero start": func(s Snapshot) *SourceSpan {
			return &SourceSpan{Commit: s.Repository.Revision, Path: "a.ts", Start: 0, End: 3}
		},
		"over bound": func(s Snapshot) *SourceSpan {
			return &SourceSpan{Commit: s.Repository.Revision, Path: "a.ts", Start: 1, End: MaxSourceLine + 1}
		},
	} {
		_, err := CompareVerified(withSource(fixtureProfile(), "coverage:checkout", span), match)
		if err == nil || err.Error() != "migration-ratchet baseline record source refused" {
			t.Errorf("%s = %v", name, err)
		}
	}
}
