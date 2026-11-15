// SPDX-License-Identifier: Apache-2.0
package update

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/execfile"
)

// TestUPDV0920ApplyInstallsATagReleaseOutput is the live qualification of ARTIFACT-BIN-V0-526
// and UPD-V0-920 (V1-5473): CORVINT_UPDATE_RELEASE_DIR names the output directory of a real
// `release-binary release --tag core/vX.Y.Z --core-gate GATE_DIR` run, whose files are the tag
// release's assets unchanged. Served as one release, one `apply --component core` from the
// previous build must install corvint and every core-set app at the binary digests the release
// manifest records, through the real platform inspection, version probes and smoke.
func TestUPDV0920ApplyInstallsATagReleaseOutput(t *testing.T) {
	directory := os.Getenv("CORVINT_UPDATE_RELEASE_DIR")
	if directory == "" {
		t.Skip("CORVINT_UPDATE_RELEASE_DIR names no release-binary release output")
	}
	e := fixtureEngine(t)
	e.inspect = inspectPlatform
	if setApps(e.platform) == nil {
		t.Skipf("the release ships no core-set app archives for %s", e.platform)
	}
	var manifest struct {
		Binaries map[string]struct {
			Version, Build string
			Archives       []struct{ Name, BinarySHA256 string }
		}
	}
	raw, err := os.ReadFile(filepath.Join(directory, "release-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	core := manifest.Binaries["corvint"]
	build, err := strconv.Atoi(core.Build)
	if err != nil || build < 2 {
		t.Fatalf("release manifest corvint build %q: %v", core.Build, err)
	}
	want := map[string]string{}
	for _, name := range append([]string{"corvint"}, coreSetApps...) {
		for _, archive := range manifest.Binaries[name].Archives {
			if archive.Name == name+"_"+e.platform+".tar.gz" {
				want[name] = archive.BinarySHA256
			}
		}
		if want[name] == "" {
			t.Fatalf("release manifest records no %s archive for %s", name, e.platform)
		}
	}

	// The release: every regular file of the directory is an asset of one published tag.
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	r := release{Tag: "v" + core.Version, Published: time.Now(), Prerelease: strings.Contains(core.Version, "-")}
	for _, entry := range entries {
		if entry.Type().IsRegular() {
			r.Assets = append(r.Assets, asset{entry.Name(), "https://github.com/assets/" + entry.Name()})
		}
	}
	rows, err := json.Marshal([]release{r})
	if err != nil {
		t.Fatal(err)
	}
	e.client = &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
		b := rows
		if name, ok := strings.CutPrefix(req.URL.Path, "/assets/"); ok {
			var readErr error
			if b, readErr = os.ReadFile(filepath.Join(directory, filepath.Base(name))); readErr != nil {
				return &http.Response{StatusCode: 404, Body: io.NopCloser(bytes.NewReader(nil))}, nil
			}
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(b))}, nil
	})}

	// The installed corvint is the previous build of the same line, with no app beside it.
	installed := fmt.Sprintf("#!/bin/sh\necho 'Corvint %s (build %d)'\n", core.Version, build-1)
	if err = execfile.WriteFile(filepath.Join(e.config.BinDir, "corvint"), []byte(installed), 0o700); err != nil {
		t.Fatal(err)
	}
	result, err := e.run(context.Background(), "apply")
	if err != nil || result.Action != "applied" {
		t.Fatalf("apply of the tag release output: %+v %v", result, err)
	}
	if got := setDigests(t, e); !sameDigests(got, want) {
		t.Fatalf("installed set %v, want the release's binaries %v", got, want)
	}
	wantSet := []SetMember{}
	for _, app := range coreSetApps {
		wantSet = append(wantSet, SetMember{app, "installed"})
	}
	wantSet = append(wantSet, SetMember{"corvint", "replaced"})
	if !slices.Equal(result.Set, wantSet) {
		t.Fatalf("set result %+v, want %+v", result.Set, wantSet)
	}
	t.Logf("applied %s: %+v", r.Tag, result.Set)
}
