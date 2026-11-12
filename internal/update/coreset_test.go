package update

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
	"github.com/Beamfall/corvint/internal/releasecandidate"
)

// setEngine is a fixture engine on linux/amd64, a platform the release ships
// app archives for, whatever the host is.
func setEngine(t *testing.T) engine {
	t.Helper()
	e := fixtureEngine(t)
	e.platform = "linux_amd64"
	return e
}

// setDigests is the digest of corvint and of each core-set app in e's bin
// directory, "" for an absent one.
func setDigests(t *testing.T, e engine) map[string]string {
	t.Helper()
	got := map[string]string{}
	for _, name := range append([]string{"corvint"}, coreSetApps...) {
		d, err := presentDigest(filepath.Join(e.config.BinDir, name))
		if err != nil {
			t.Fatal(err)
		}
		got[name] = d
	}
	return got
}

// wantSet is the digests a whole-set install of build leaves.
func wantSet(build int) map[string]string {
	want := map[string]string{"corvint": hash(scriptBinary(build, 0))}
	for _, app := range coreSetApps {
		want[app] = hash(appBinary(app, build))
	}
	return want
}

func sameDigests(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func TestUPDV0920SetListsMatchTheReleaseCatalog(t *testing.T) {
	if !slices.Equal(coreSetApps, releasecandidate.CoreSetApps) {
		t.Fatalf("core-set apps %q, release catalog %q", coreSetApps, releasecandidate.CoreSetApps)
	}
	if !slices.Equal(appPlatforms, releasecandidate.AppPlatforms) {
		t.Fatalf("app platforms %q, release catalog %q", appPlatforms, releasecandidate.AppPlatforms)
	}
}

func TestUPDV0920ApplyInstallsTheWholeCoreSet(t *testing.T) {
	e := setEngine(t)
	writeScript(t, filepath.Join(e.config.BinDir, "corvint"), 162)
	releaseClient(t, &e, 163)
	r, err := e.run(context.Background(), "apply")
	if err != nil || r.Action != "applied" {
		t.Fatalf("apply: %+v %v", r, err)
	}
	if got := setDigests(t, e); !sameDigests(got, wantSet(163)) {
		t.Fatalf("installed set %v, want %v", got, wantSet(163))
	}
	wantResult := []SetMember{{"corvint-docs", "installed"}, {"corvint-docs-mcp", "installed"}, {"corvint-mcp", "installed"}, {"corvint", "replaced"}}
	if !slices.Equal(r.Set, wantResult) {
		t.Fatalf("set result %+v", r.Set)
	}
	b, err := os.ReadFile(r.Receipt)
	if err != nil {
		t.Fatal(err)
	}
	var rec receipt
	if err = json.Unmarshal(b, &rec); err != nil || len(rec.Members) != 3 {
		t.Fatalf("receipt members: %s %v", b, err)
	}
	for i, m := range rec.Members {
		if m.Name != coreSetApps[i] || m.Destination != filepath.Join(e.config.BinDir, m.Name) || m.PreviousDigest != "" || m.CandidateDigest != hash(appBinary(m.Name, 163)) {
			t.Fatalf("receipt member %+v", m)
		}
	}

	// An app at another version, or with a notice the corvint archive does
	// not carry, refuses before any destination changes.
	for name, archive := range map[string][]byte{
		"version": appArchiveOf(t, "corvint-docs", "linux_amd64", []byte("#!/bin/sh\necho 'corvint-docs 1.0.0-rc.2 (build 163)'\n")),
		"name":    appArchiveOf(t, "corvint-docs", "linux_amd64", appBinary("corvint-mcp", 163)),
		"notice": archiveBytes(t, map[string][]byte{
			"corvint-docs_linux_amd64/corvint-docs": appBinary("corvint-docs", 163),
			"corvint-docs_linux_amd64/SHA256SUMS":   []byte(hash(appBinary("corvint-docs", 163)) + "  corvint-docs\n"),
			"corvint-docs_linux_amd64/LICENSE":      []byte("another notice"),
		}),
		"inner checksum": archiveBytes(t, map[string][]byte{
			"corvint-docs_linux_amd64/corvint-docs": appBinary("corvint-docs", 163),
			"corvint-docs_linux_amd64/SHA256SUMS":   []byte(hash([]byte("other")) + "  corvint-docs\n"),
		}),
	} {
		e := setEngine(t)
		writeScript(t, filepath.Join(e.config.BinDir, "corvint"), 162)
		before := setDigests(t, e)
		apps := appArchives(t, e.platform, 163)
		apps["corvint-docs"] = archive
		releaseFixture(t, &e, scriptBinary(163, 0), apps)
		if _, err := e.run(context.Background(), "apply"); err == nil || !strings.Contains(err.Error(), "corvint-docs") {
			t.Fatalf("%s: disagreeing app accepted or not named: %v", name, err)
		}
		if got := setDigests(t, e); !sameDigests(got, before) {
			t.Fatalf("%s: refusal changed the set: %v", name, got)
		}
	}
}

func TestUPDV0921RollbackAndInterruptedSetRestore(t *testing.T) {
	e := setEngine(t)
	writeScript(t, filepath.Join(e.config.BinDir, "corvint"), 162)
	if err := execfile.WriteFile(filepath.Join(e.config.BinDir, "corvint-docs"), appBinary("corvint-docs", 162), 0700); err != nil {
		t.Fatal(err)
	}
	before := setDigests(t, e)
	releaseClient(t, &e, 163)
	if _, err := e.run(context.Background(), "apply"); err != nil {
		t.Fatal(err)
	}
	e.config.AllowNetwork = false
	r, err := e.run(context.Background(), "rollback")
	if err != nil {
		t.Fatal(err)
	}
	if got := setDigests(t, e); !sameDigests(got, before) {
		t.Fatalf("rollback left %v, want %v", got, before)
	}
	if want := []SetMember{{"corvint-docs", "restored"}, {"corvint-docs-mcp", "removed"}, {"corvint-mcp", "removed"}, {"corvint", "restored"}}; !slices.Equal(r.Set, want) {
		t.Fatalf("rollback set %+v", r.Set)
	}
	if _, err = e.run(context.Background(), "rollback"); err == nil {
		t.Fatal("second rollback matched a receipt")
	}

	// A member changed after the apply refuses the rollback with it named and
	// nothing changed; a member already restored by an interrupted rollback
	// lets the rollback run again.
	e.config.AllowNetwork = true
	if _, err = e.run(context.Background(), "apply"); err != nil {
		t.Fatal(err)
	}
	e.config.AllowNetwork = false
	mcp := filepath.Join(e.config.BinDir, "corvint-mcp")
	if err = execfile.WriteFile(mcp, []byte("#!/bin/sh\necho changed\n"), 0700); err != nil {
		t.Fatal(err)
	}
	changed := setDigests(t, e)
	if _, err = e.run(context.Background(), "rollback"); err == nil || !strings.Contains(err.Error(), "corvint-mcp") {
		t.Fatalf("changed member accepted or not named: %v", err)
	}
	if got := setDigests(t, e); !sameDigests(got, changed) {
		t.Fatalf("refused rollback changed the set: %v", got)
	}
	if err = os.Remove(mcp); err != nil {
		t.Fatal(err)
	}
	if _, err = e.run(context.Background(), "rollback"); err != nil {
		t.Fatalf("rerun after a partly restored set: %v", err)
	}
	if got := setDigests(t, e); !sameDigests(got, before) {
		t.Fatalf("rerun rollback left %v", got)
	}

	// A run cancelled after the first app switched restores it and returns an error.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var seen []string
	e.beforeMember = func(name string) {
		seen = append(seen, name)
		if name == "corvint-docs-mcp" {
			cancel()
		}
	}
	e.config.AllowNetwork = true
	if _, err = e.run(ctx, "apply"); err == nil {
		t.Fatal("cancelled set switch reported success")
	}
	if got := setDigests(t, e); !sameDigests(got, before) {
		t.Fatalf("cancelled run left %v, want %v", got, before)
	}
	if !slices.Equal(seen, []string{"corvint-docs", "corvint-docs-mcp"}) {
		t.Fatalf("member order %q", seen)
	}
}

func TestUPDV0922MissingAppRefusesAndEqualBuildCompletesTheSet(t *testing.T) {
	for name, omit := range map[string]string{"archive": "corvint-mcp", "row": "corvint-docs"} {
		e := setEngine(t)
		writeScript(t, filepath.Join(e.config.BinDir, "corvint"), 162)
		before := setDigests(t, e)
		apps := appArchives(t, e.platform, 163)
		var rows []string
		if name == "archive" {
			delete(apps, omit)
		} else {
			rows = append(rows, omit+"_linux_amd64.tar.gz")
		}
		releaseFixture(t, &e, scriptBinary(163, 0), apps, rows...)
		_, err := e.run(context.Background(), "apply")
		if err == nil || !strings.Contains(err.Error(), "core-set app "+omit) {
			t.Fatalf("missing %s for %s accepted or not named: %v", name, omit, err)
		}
		if got := setDigests(t, e); !sameDigests(got, before) {
			t.Fatalf("missing %s changed the set: %v", name, got)
		}
		if entries, _ := os.ReadDir(e.config.StateDir); len(entries) != 0 {
			t.Fatalf("missing %s retained staging: %v", name, entries)
		}
	}

	// corvint already at the release build with the apps absent: the apply
	// installs them, leaves corvint unrewritten and is not unchanged.
	e := setEngine(t)
	dest := filepath.Join(e.config.BinDir, "corvint")
	writeScript(t, dest, 163)
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatal(err)
	}
	releaseClient(t, &e, 163)
	r, err := e.run(context.Background(), "apply")
	if err != nil || r.Action != "applied" {
		t.Fatalf("equal build with apps absent: %+v %v", r, err)
	}
	if got := setDigests(t, e); !sameDigests(got, wantSet(163)) {
		t.Fatalf("completed set %v", got)
	}
	if now, err := os.Stat(dest); err != nil || !os.SameFile(info, now) {
		t.Fatal("equal corvint was rewritten")
	}
	if want := []SetMember{{"corvint-docs", "installed"}, {"corvint-docs-mcp", "installed"}, {"corvint-mcp", "installed"}, {"corvint", "unchanged"}}; !slices.Equal(r.Set, want) {
		t.Fatalf("completion set %+v", r.Set)
	}
	if r, err = e.run(context.Background(), "apply"); err != nil || r.Action != "unchanged" {
		t.Fatalf("complete set not unchanged: %+v %v", r, err)
	}

	// A core platform the release ships no app archive for installs corvint alone.
	e = fixtureEngine(t)
	e.platform = "darwin_amd64"
	writeScript(t, filepath.Join(e.config.BinDir, "corvint"), 162)
	releaseClient(t, &e, 163)
	if r, err = e.run(context.Background(), "apply"); err != nil || r.Action != "applied" || r.Set != nil || !strings.Contains(r.Qualification, "ships none for darwin_amd64") {
		t.Fatalf("corvint-only platform: %+v %v", r, err)
	}
	if got := setDigests(t, e); got["corvint"] != hash(scriptBinary(163, 0)) || got["corvint-docs"] != "" {
		t.Fatalf("corvint-only platform set %v", got)
	}
}
