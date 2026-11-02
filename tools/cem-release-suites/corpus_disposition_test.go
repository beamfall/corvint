// SPDX-License-Identifier: Apache-2.0
package main

// The release-readiness declaration names the platforms a release must observe
// (CEM-V1-600). The structural corpus qualification (decision 0551, CEM-V1-369)
// records the hosts that ran the corpus. These tests bind the two (V1-0591):
// every declared platform has an explicit corpus disposition (CEM-V1-860),
// every recorded host agrees with the one qualified observation (CEM-V1-861),
// and the corpus manifest pins every packet file (CEM-V1-862).

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/companionrelease"
)

// corpusRoot is the sealed structural corpus packet, read from the worktree.
const corpusRoot = "../../protocol/cem-1.0/structural-corpus"

const (
	corpusQualificationProfile = "cem-structural-corpus-qualification/1"
	corpusManifestProfile      = "cem-structural-corpus-manifest/1"
)

type corpusQualification struct {
	Profile        string   `json:"profile"`
	QualifiedTuple string   `json:"qualifiedTuple"`
	AdmittedTuples []string `json:"admittedTuples"`
	Cases          struct {
		ObservationsSha256 string `json:"observationsSha256"`
		Observations       string `json:"observations"`
	} `json:"cases"`
	Result struct {
		Repository corpusCount `json:"repository"`
		Cases      corpusCount `json:"cases"`
	} `json:"result"`
	Hosts  map[string]corpusHost `json:"hosts"`
	NotRun []string              `json:"notRun"`
}

type corpusCount struct {
	Files             int `json:"files"`
	PerturbationPairs int `json:"perturbationPairs"`
}

type corpusHost struct {
	Uname      string                       `json:"uname"`
	Where      string                       `json:"where"`
	Toolchains map[string]string            `json:"toolchains"`
	Binaries   map[string]string            `json:"binaries"`
	Portable   map[string]corpusObservation `json:"portable"`
	Native     map[string]corpusParity      `json:"native"`
}

type corpusObservation struct {
	Repo  string `json:"repo"`
	Cases string `json:"cases"`
}

type corpusParity struct {
	Repo  corpusParityCount `json:"repo"`
	Cases corpusParityCount `json:"cases"`
}

type corpusParityCount struct {
	Files         int   `json:"files"`
	Pairs         int   `json:"pairs"`
	VerdictsEqual *bool `json:"verdictsEqualPortableGo1.27.1"`
}

type corpusManifest struct {
	Profile string `json:"profile"`
	Files   []struct {
		Path   string `json:"path"`
		Sha256 string `json:"sha256"`
		Bytes  int    `json:"bytes"`
	} `json:"files"`
}

func readCorpusQualification(t *testing.T, root string) corpusQualification {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "QUALIFICATION.json"))
	if err != nil {
		t.Fatal(err)
	}
	var q corpusQualification
	if err := json.Unmarshal(raw, &q); err != nil {
		t.Fatal(err)
	}
	if q.Profile != corpusQualificationProfile {
		t.Fatalf("QUALIFICATION.json profile %q, want %s", q.Profile, corpusQualificationProfile)
	}
	return q
}

// undisposedPlatforms (CEM-V1-860) names each declared platform that has
// neither a hosts entry nor a verbatim notRun entry, and each that has both.
// Prose that merely mentions a platform is not a disposition.
func undisposedPlatforms(declared []string, hosts map[string]corpusHost, notRun []string) []string {
	var out []string
	for _, platform := range declared {
		_, ran := hosts[platform]
		skipped := slices.Contains(notRun, platform)
		switch {
		case ran && skipped:
			out = append(out, platform+": both a hosts entry and a notRun entry")
		case !ran && !skipped:
			out = append(out, platform+": neither a hosts entry nor a notRun entry")
		}
	}
	return out
}

// TestCEMV1860DeclaredPlatformsHaveCorpusDisposition: every platform row of the
// committed readiness declaration is either a corpus host or named NOT_RUN.
func TestCEMV1860DeclaredPlatformsHaveCorpusDisposition(t *testing.T) {
	raw, err := os.ReadFile("READINESS.json")
	if err != nil {
		t.Fatal(err)
	}
	declaration, err := companionrelease.ReadStableReleaseDeclaration(raw)
	if err != nil {
		t.Fatal(err)
	}
	q := readCorpusQualification(t, corpusRoot)
	if missing := undisposedPlatforms(declaration.Platform, q.Hosts, q.NotRun); len(missing) != 0 {
		t.Fatalf("declared platforms without a structural-corpus disposition: %s", strings.Join(missing, "; "))
	}
	for _, platform := range declaration.Platform {
		if _, ok := q.Hosts[platform]; !ok {
			t.Logf("%s: NOT_RUN by disposition", platform)
		}
	}

	hosts := map[string]corpusHost{"linux/amd64": {}}
	if got := undisposedPlatforms([]string{"windows/amd64", "linux/amd64"}, hosts, []string{"windows hosts"}); !slices.Equal(got, []string{"windows/amd64: neither a hosts entry nor a notRun entry"}) {
		t.Fatalf("prose notRun counted as a disposition: %v", got)
	}
	if got := undisposedPlatforms([]string{"windows/amd64", "linux/amd64"}, hosts, []string{"windows/amd64"}); got != nil {
		t.Fatalf("verbatim notRun refused: %v", got)
	}
	if got := undisposedPlatforms([]string{"linux/amd64"}, hosts, []string{"linux/amd64"}); !slices.Equal(got, []string{"linux/amd64: both a hosts entry and a notRun entry"}) {
		t.Fatalf("contradictory disposition admitted: %v", got)
	}
}

// corpusHostProblems (CEM-V1-861) names every way a hosts entry disagrees with
// the qualified observation: no host identity, no toolchain or binary record,
// no admitted tuple observed, a cases digest other than the qualified one, a
// repository digest other than the one every admitted tuple shares, or a
// native parity record that is not true with the qualified counts.
func corpusHostProblems(q corpusQualification) []string {
	var problems []string
	admitted := map[string]bool{}
	for _, tuple := range q.AdmittedTuples {
		admitted[tuple] = true
	}
	names := make([]string, 0, len(q.Hosts))
	for name := range q.Hosts {
		names = append(names, name)
	}
	sort.Strings(names)
	repoDigest := ""
	for _, name := range names {
		h := q.Hosts[name]
		if h.Uname == "" && h.Where == "" {
			problems = append(problems, name+": no uname or where")
		}
		if len(h.Toolchains) == 0 && len(h.Binaries) == 0 {
			problems = append(problems, name+": no toolchains or binaries")
		}
		observed := 0
		tuples := make([]string, 0, len(h.Portable))
		for tuple := range h.Portable {
			tuples = append(tuples, tuple)
		}
		sort.Strings(tuples)
		for _, tuple := range tuples {
			if !admitted[tuple] {
				continue
			}
			observed++
			o := h.Portable[tuple]
			if o.Cases != q.Cases.ObservationsSha256 {
				problems = append(problems, fmt.Sprintf("%s %s: cases observation %s, qualified %s", name, tuple, o.Cases, q.Cases.ObservationsSha256))
			}
			if repoDigest == "" {
				repoDigest = o.Repo
			}
			if o.Repo == "" || o.Repo != repoDigest {
				problems = append(problems, fmt.Sprintf("%s %s: repository observation %q differs from %s", name, tuple, o.Repo, repoDigest))
			}
		}
		tuples = tuples[:0]
		for tuple := range h.Native {
			tuples = append(tuples, tuple)
		}
		sort.Strings(tuples)
		for _, tuple := range tuples {
			if !admitted[tuple] {
				problems = append(problems, name+" "+tuple+": native parity on a tuple that is not admitted")
				continue
			}
			observed++
			p := h.Native[tuple]
			for _, c := range []struct {
				what  string
				got   corpusParityCount
				count corpusCount
			}{{"repository", p.Repo, q.Result.Repository}, {"cases", p.Cases, q.Result.Cases}} {
				if c.got.VerdictsEqual == nil || !*c.got.VerdictsEqual {
					problems = append(problems, fmt.Sprintf("%s %s: %s native verdicts are not recorded equal to the portable verdicts", name, tuple, c.what))
				}
				if c.got.Files != c.count.Files || c.got.Pairs != c.count.PerturbationPairs {
					problems = append(problems, fmt.Sprintf("%s %s: %s parity over %d files and %d pairs, qualified %d and %d", name, tuple, c.what, c.got.Files, c.got.Pairs, c.count.Files, c.count.PerturbationPairs))
				}
			}
		}
		if observed == 0 {
			problems = append(problems, name+": no admitted tuple observed")
		}
	}
	return problems
}

// TestCEMV1861HostEntriesAgreeWithTheQualifiedObservation: every recorded host
// observed the one qualified corpus, and a host that did not cannot be listed.
func TestCEMV1861HostEntriesAgreeWithTheQualifiedObservation(t *testing.T) {
	q := readCorpusQualification(t, corpusRoot)
	if len(q.Hosts) == 0 {
		t.Fatal("QUALIFICATION.json records no host")
	}
	if problems := corpusHostProblems(q); len(problems) != 0 {
		t.Fatalf("hosts disagree with the qualified observation:\n%s", strings.Join(problems, "\n"))
	}

	yes := true
	synthetic := q
	synthetic.Hosts = map[string]corpusHost{}
	for name, h := range q.Hosts {
		synthetic.Hosts[name] = h
	}
	synthetic.Hosts["plan9/amd64"] = corpusHost{Uname: "Plan 9", Toolchains: map[string]string{q.QualifiedTuple: "test"},
		Portable: map[string]corpusObservation{q.QualifiedTuple: {Repo: "0000", Cases: q.Cases.ObservationsSha256}},
		Native:   map[string]corpusParity{q.QualifiedTuple: {Repo: corpusParityCount{Files: 1, Pairs: 1, VerdictsEqual: &yes}, Cases: corpusParityCount{Files: q.Result.Cases.Files, Pairs: q.Result.Cases.PerturbationPairs}}}}
	problems := corpusHostProblems(synthetic)
	for _, want := range []string{"plan9/amd64 " + q.QualifiedTuple + ": repository observation \"0000\" differs", "repository parity over 1 files and 1 pairs", "cases native verdicts are not recorded equal"} {
		if !slices.ContainsFunc(problems, func(p string) bool { return strings.Contains(p, want) }) {
			t.Fatalf("problems lack %q: %v", want, problems)
		}
	}
	synthetic.Hosts = map[string]corpusHost{"empty/host": {}}
	problems = corpusHostProblems(synthetic)
	for _, want := range []string{"no uname or where", "no toolchains or binaries", "no admitted tuple observed"} {
		if !slices.ContainsFunc(problems, func(p string) bool { return strings.Contains(p, want) }) {
			t.Fatalf("problems lack %q: %v", want, problems)
		}
	}
}

// corpusManifestProblems (CEM-V1-862) names every packet file the manifest
// lists with another digest or length, lists twice or does not list, every
// listed file that is absent, and a cases observation digest that differs
// from the qualification.
func corpusManifestProblems(root string) []string {
	var problems []string
	raw, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil {
		return []string{err.Error()}
	}
	var m corpusManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return []string{"manifest.json: " + err.Error()}
	}
	if m.Profile != corpusManifestProfile {
		problems = append(problems, fmt.Sprintf("manifest.json profile %q, want %s", m.Profile, corpusManifestProfile))
	}
	listed := map[string]bool{}
	for _, f := range m.Files {
		if listed[f.Path] {
			problems = append(problems, f.Path+": listed twice")
			continue
		}
		listed[f.Path] = true
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f.Path)))
		if err != nil {
			problems = append(problems, f.Path+": "+err.Error())
			continue
		}
		if sum := sha256.Sum256(content); hex.EncodeToString(sum[:]) != f.Sha256 || len(content) != f.Bytes {
			problems = append(problems, fmt.Sprintf("%s: manifest pins sha256 %s (%d bytes), file is %s (%d bytes)", f.Path, f.Sha256, f.Bytes, hex.EncodeToString(sum[:]), len(content)))
		}
	}
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel != "manifest.json" && !listed[rel] {
			problems = append(problems, rel+": not listed in manifest.json")
		}
		return nil
	})
	if err != nil {
		problems = append(problems, err.Error())
	}
	qraw, err := os.ReadFile(filepath.Join(root, "QUALIFICATION.json"))
	if err != nil {
		return append(problems, err.Error())
	}
	var q corpusQualification
	if err := json.Unmarshal(qraw, &q); err != nil {
		return append(problems, "QUALIFICATION.json: "+err.Error())
	}
	observations, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(q.Cases.Observations)))
	if err != nil {
		return append(problems, err.Error())
	}
	if sum := sha256.Sum256(observations); hex.EncodeToString(sum[:]) != q.Cases.ObservationsSha256 {
		problems = append(problems, fmt.Sprintf("%s: sha256 %s, QUALIFICATION.json records %s", q.Cases.Observations, hex.EncodeToString(sum[:]), q.Cases.ObservationsSha256))
	}
	sort.Strings(problems)
	return problems
}

// TestCEMV1862CorpusManifestIsComplete: the committed corpus manifest pins
// every packet file, and a stale pin, an unlisted file or a missing file fails.
func TestCEMV1862CorpusManifestIsComplete(t *testing.T) {
	if problems := corpusManifestProblems(corpusRoot); len(problems) != 0 {
		t.Fatalf("structural corpus manifest:\n%s", strings.Join(problems, "\n"))
	}

	root := t.TempDir()
	write := func(name, content string) {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	digest := func(s string) string {
		sum := sha256.Sum256([]byte(s))
		return hex.EncodeToString(sum[:])
	}
	observations := "{\"path\":\"a.go.txt\"}\n"
	qualification := fmt.Sprintf(`{"profile":%q,"cases":{"observationsSha256":%q,"observations":"CASES.observations.jsonl"},"hosts":{},"notRun":[]}`+"\n", corpusQualificationProfile, digest(observations))
	write("CASES.observations.jsonl", observations)
	write("QUALIFICATION.json", qualification)
	write("cases/a.go.txt", "package a\n")
	manifest := func(entries ...string) string {
		return fmt.Sprintf(`{"profile":%q,"files":[%s]}`+"\n", corpusManifestProfile, strings.Join(entries, ","))
	}
	entry := func(path, content string) string {
		return fmt.Sprintf(`{"path":%q,"sha256":%q,"bytes":%d}`, path, digest(content), len(content))
	}
	write("manifest.json", manifest(entry("CASES.observations.jsonl", observations), entry("QUALIFICATION.json", qualification), entry("cases/a.go.txt", "package a\n")))
	if problems := corpusManifestProblems(root); len(problems) != 0 {
		t.Fatalf("complete synthetic packet refused: %v", problems)
	}

	write("cases/b.go.txt", "package b\n")
	problems := corpusManifestProblems(root)
	if !slices.Contains(problems, "cases/b.go.txt: not listed in manifest.json") {
		t.Fatalf("unlisted file admitted: %v", problems)
	}
	write("manifest.json", manifest(entry("CASES.observations.jsonl", observations), entry("QUALIFICATION.json", qualification), entry("cases/a.go.txt", "package b\n"), entry("cases/b.go.txt", "package b\n"), entry("cases/c.go.txt", "")))
	problems = corpusManifestProblems(root)
	for _, want := range []string{"cases/a.go.txt: manifest pins sha256", "cases/c.go.txt: open"} {
		if !slices.ContainsFunc(problems, func(p string) bool { return strings.Contains(p, want) }) {
			t.Fatalf("problems lack %q: %v", want, problems)
		}
	}
	write("CASES.observations.jsonl", observations+"\n")
	problems = corpusManifestProblems(root)
	if !slices.ContainsFunc(problems, func(p string) bool { return strings.HasPrefix(p, "CASES.observations.jsonl: sha256") }) {
		t.Fatalf("stale cases observation digest admitted: %v", problems)
	}
}
