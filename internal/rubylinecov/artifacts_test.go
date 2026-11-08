// SPDX-License-Identifier: Apache-2.0

package rubylinecov_test

import (
	"crypto/sha1"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/rubylinecov"
	"github.com/Beamfall/corvint/internal/testevidence"
)

const selectionDump = `{"kind":"start","ruby":"3.2"}
{"kind":"framework","name":"minitest","version":"5"}
{"kind":"load","lines":{"sample.rb":[1]}}
{"kind":"test","seq":1,"package":"sample.rb","name":"Sample#test_one","status":"passed","lines":{"sample.rb":[2]}}
{"kind":"files","files":{"sample.rb":{"lines":[1,2,3]}}}
{"kind":"done"}
`

func selectionArtifacts(t *testing.T, data string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "suite.jsonl"), []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestRSPV0005DecodeSelectionArtifacts(t *testing.T) {
	suite := selectionArtifacts(t, selectionDump)
	one := selectionArtifacts(t, selectionDump)
	source := "def helper\n 1\nend\n"
	oid := fmt.Sprintf("%x", sha1.Sum([]byte(fmt.Sprintf("blob %d\x00%s", len(source), source))))
	sources := map[string]rubylinecov.SelectionSource{"sample.rb": {OID: oid, Bytes: []byte(source)}}
	for _, dirs := range [][]string{nil, {one}} {
		run, err := rubylinecov.ReadSelectionArtifacts(suite, dirs)
		if err != nil {
			t.Fatal(err)
		}
		evidence, err := rubylinecov.SelectionEvidence(run, "repo", sources)
		if err != nil {
			t.Fatal(err)
		}
		if len(evidence) != 1 {
			t.Fatalf("%+v", evidence)
		}
		if (evidence[0].Native.Executed.State == testevidence.Present) != (len(dirs) == 1) {
			t.Fatalf("isolation admission: %+v", evidence)
		}
	}
	divergent := selectionArtifacts(t, strings.Replace(selectionDump, `"status":"passed"`, `"status":"failed"`, 1))
	run, err := rubylinecov.ReadSelectionArtifacts(suite, []string{divergent})
	if err != nil {
		t.Fatal(err)
	}
	if run.Outcomes[0].Recorded {
		t.Fatal("divergent isolation admitted")
	}
	if _, err := rubylinecov.ReadSelectionArtifacts(suite, []string{one, one}); err == nil {
		t.Fatal("duplicate isolated evidence admitted")
	}
}

func TestRSPV0005ArtifactFormsFailClosed(t *testing.T) {
	for name, dump := range map[string]string{
		"unknown-event":  selectionDump + "{\"kind\":\"future-coverage\"}\n",
		"unknown-status": strings.Replace(selectionDump, `"status":"passed"`, `"status":"future-status"`, 1),
		"incomplete":     strings.Replace(selectionDump, "{\"kind\":\"done\"}\n", "", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := rubylinecov.ReadSelectionArtifacts(selectionArtifacts(t, dump), nil); err == nil {
				t.Fatal("unsupported capture certified")
			}
		})
	}
	dir := selectionArtifacts(t, selectionDump)
	if err := os.WriteFile(filepath.Join(dir, "future.bin"), []byte("unknown"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := rubylinecov.ReadSelectionArtifacts(dir, nil); err == nil {
		t.Fatal("unknown artifact discarded")
	}
}
