package update

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tasksBinary(build int) []byte {
	return []byte(fmt.Sprintf("#!/bin/sh\ncase \"$1\" in\nversion) echo '{\"items\":[{\"version\":\"0.0.0-tcp01-unverified+build.%d\"}],\"profile\":\"taskman-command-result/0\"}' ;;\n*) echo '{}' ;;\nesac\n", build))
}

// tasksRelease is a Tasks archive for platform at build and the report that
// declares it, with archiveSha256 bound to the archive bytes.
func tasksRelease(t *testing.T, platform string, build int) (archive, report []byte) {
	t.Helper()
	binary := tasksBinary(build)
	files := map[string][]byte{"bin/corvint-tasks": binary, "README.md": []byte("readme"), "source/corvint-tasks-src.tar.gz": []byte("source")}
	for _, n := range []string{"LICENSE", "LICENSING.md", "PROVENANCE.md", "THIRD-PARTY-NOTICES.txt"} {
		files["notices/corvint-tasks/"+n] = []byte("notice")
	}
	component := map[string]any{"name": "corvint-tasks", "binaryPath": "bin/corvint-tasks", "binarySha256": hash(binary), "binarySizeBytes": len(binary), "sourceArchivePath": "source/corvint-tasks-src.tar.gz", "sourceArchiveSha256": hash(files["source/corvint-tasks-src.tar.gz"])}
	manifest := map[string]any{"profile": "corvint-tasks-archive/0", "target": strings.ReplaceAll(platform, "_", "/"), "build": fmt.Sprint(build), "component": component, "qualification": "reproducible-build-and-native-version-help-smoke-only"}
	files["MANIFEST.json"], _ = json.Marshal(manifest)
	sums := ""
	for name, body := range files {
		sums += hash(body) + "  " + name + "\n"
	}
	files["SHA256SUMS"] = []byte(sums)
	archive = archiveBytes(t, files)
	manifest["archiveSha256"] = hash(archive)
	report, _ = json.Marshal(manifest)
	return archive, report
}

func tasksEngine(t *testing.T) (engine, string) {
	t.Helper()
	e := fixtureEngine(t)
	e.config.Component = "tasks"
	dest := filepath.Join(e.config.BinDir, "corvint-tasks")
	if err := os.WriteFile(dest, tasksBinary(400), 0o700); err != nil {
		t.Fatal(err)
	}
	return e, dest
}

// otherPlatform is the supported archive platform that is not this host's.
func otherPlatform(platform string) string {
	if platform == "linux_amd64" {
		return "darwin_arm64"
	}
	return "linux_amd64"
}

// V1-3013: apply refuses a report whose archiveSha256 is not the downloaded
// archive's digest, as check already did.
func TestUPDV0016ApplyBindsReportToArchiveDigest(t *testing.T) {
	e, dest := tasksEngine(t)
	name := "corvint-tasks_" + e.platform + ".tar.gz"
	archive, report := tasksRelease(t, e.platform, 401)
	var m map[string]any
	if err := json.Unmarshal(report, &m); err != nil {
		t.Fatal(err)
	}
	m["archiveSha256"] = hash([]byte("another archive"))
	forged, _ := json.Marshal(m)
	sums := []byte(hash(archive) + "  " + name + "\n")
	relationClient(&e, "tasks-dev-20261028.1", map[string][]byte{name: archive, "SHA256SUMS": sums, "build-verification.json": forged})
	r, err := e.run(context.Background(), "check")
	if err != nil || r.Relation != "UNKNOWN" || !strings.Contains(r.RelationReason, "does not match the release archive") {
		t.Fatalf("check: %+v %v", r, err)
	}
	if _, err = e.run(context.Background(), "apply"); err == nil || !strings.Contains(err.Error(), "does not match the release archive") {
		t.Fatalf("apply accepted a report for another archive: %v", err)
	}
	if d, _ := digest(dest); d != hash(tasksBinary(400)) {
		t.Fatal("destination changed")
	}
	// The bound report applies.
	relationClient(&e, "tasks-dev-20261028.1", map[string][]byte{name: archive, "SHA256SUMS": sums, "build-verification.json": report})
	if r, err = e.run(context.Background(), "apply"); err != nil || r.Action != "applied" || r.InstalledBuild != 401 {
		t.Fatalf("bound report: %+v %v", r, err)
	}
}

// V1-3014: a release carrying both platform archives qualifies each through
// its per-platform report; the single legacy report still qualifies only the
// platform it names.
func TestUPDV0017PerPlatformTasksReports(t *testing.T) {
	e, _ := tasksEngine(t)
	other := otherPlatform(e.platform)
	here, hereReport := tasksRelease(t, e.platform, 401)
	there, thereReport := tasksRelease(t, other, 401)
	hereName, thereName := "corvint-tasks_"+e.platform+".tar.gz", "corvint-tasks_"+other+".tar.gz"
	sums := []byte(hash(here) + "  " + hereName + "\n" + hash(there) + "  " + thereName + "\n")
	both := map[string][]byte{hereName: here, thereName: there, "SHA256SUMS": sums, "build-verification_" + e.platform + ".json": hereReport, "build-verification_" + other + ".json": thereReport}
	besideLegacy := map[string][]byte{"build-verification.json": thereReport}
	for k, v := range both {
		besideLegacy[k] = v
	}
	for name, assets := range map[string]map[string][]byte{
		"per-platform reports": both,
		// A legacy report naming the other platform is ignored when this
		// platform has its own.
		"per-platform beside legacy": besideLegacy,
		"legacy single platform":     {hereName: here, "SHA256SUMS": []byte(hash(here) + "  " + hereName + "\n"), "build-verification.json": hereReport},
	} {
		e, dest := tasksEngine(t)
		relationClient(&e, "tasks-dev-20261028.1", assets)
		r, err := e.run(context.Background(), "check")
		if err != nil || r.Relation != "UPDATE_AVAILABLE" || r.AvailableBuild != 401 {
			t.Fatalf("%s check: %+v %v", name, r, err)
		}
		if r, err = e.run(context.Background(), "apply"); err != nil || r.Action != "applied" || r.InstalledBuild != 401 {
			t.Fatalf("%s apply: %+v %v", name, r, err)
		}
		if d, _ := digest(dest); d != hash(tasksBinary(401)) {
			t.Fatalf("%s: candidate not installed", name)
		}
	}
	// A legacy report for the other platform does not qualify this one.
	e, dest := tasksEngine(t)
	relationClient(&e, "tasks-dev-20261028.1", map[string][]byte{hereName: here, thereName: there, "SHA256SUMS": sums, "build-verification.json": thereReport})
	if r, err := e.run(context.Background(), "check"); err != nil || r.Relation != "UNKNOWN" {
		t.Fatalf("other-platform legacy check: %+v %v", r, err)
	}
	if _, err := e.run(context.Background(), "apply"); err == nil {
		t.Fatal("other-platform legacy report qualified this platform")
	}
	if d, _ := digest(dest); d != hash(tasksBinary(400)) {
		t.Fatal("destination changed")
	}
}
