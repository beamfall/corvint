//go:build linux && (amd64 || arm64)

// Copyright 2026 Corvint contributors.
// SPDX-License-Identifier: AGPL-3.0-or-later
package postmergehost

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/postmergeproof"
)

type memRetainV2 map[string][]byte

func (m memRetainV2) RetainProcessArtifactV2(id string, data []byte) (postmergeproof.ArtifactRefV2, error) {
	m[id] = bytes.Clone(data)
	sum := sha256.Sum256(data)
	return postmergeproof.ArtifactRefV2{ID: id, SHA256: hex.EncodeToString(sum[:]), Bytes: int64(len(data))}, nil
}

func TestProcessCollectorV2RefusesMisuse(t *testing.T) {
	ctx := context.Background()
	c, err := NewProcessCollectorV2(ctx, "misuse", "g", "p", ObserverCommandV2{Path: "/proc/self/exe"}, memRetainV2{})
	if err != nil {
		t.Fatal(err)
	}
	if len(c.proof.Captures) != 1 || len(c.proof.Launches) != 1 || c.proof.Launches[0].Purpose != "host-supervisor" ||
		c.proof.Launches[0].ParentCaptureIndex != nil || c.SupervisorCapture() != 0 {
		t.Fatalf("supervisor boundary: %+v", c.proof.Launches)
	}
	spec := OwnedLaunchSpecV2{Purpose: "workflow-root", Parent: c.SupervisorCapture(), Phase: "start-barrier"}
	inherit := exec.Command("/bin/sh", "-c", "exit 0")
	preset := exec.Command("/bin/sh", "-c", "exit 0")
	preset.Env, preset.Stdin = []string{}, strings.NewReader("x")
	secret := exec.Command("/bin/sh", "-c", "exit 0")
	secret.Env = []string{"GITHUB_TOKEN=x"}
	parentless := exec.Command("/bin/sh", "-c", "exit 0")
	parentless.Env = []string{}
	for name, cmd := range map[string]*exec.Cmd{"inherited env": inherit, "preset stdin": preset, "secret env": secret} {
		if _, err := c.Start(ctx, cmd, spec, nil); err == nil || cmd.Process != nil {
			t.Fatalf("%s: started", name)
		}
	}
	for _, bad := range []OwnedLaunchSpecV2{{Purpose: "runner", Parent: 7, Phase: "during-run"}, {Purpose: "runner", Phase: "final-sweep"},
		{Purpose: "runner", Phase: "retirement"}, {Phase: "during-run"}} {
		_, err := c.Start(ctx, parentless, bad, nil)
		expectProcessCode(t, err, "BLOCKED", "process-collection-refused")
	}
	if parentless.Process != nil {
		t.Fatal("refused launch started")
	}
	_, err = c.Proof()
	expectProcessCode(t, err, "BLOCKED", "process-collection-refused")
	err = c.FinalSweep(ctx)
	expectProcessCode(t, err, "BLOCKED", "process-collection-refused")
}

func TestProcessObserverV2Modes(t *testing.T) {
	ctx := context.Background()
	for _, args := range [][]string{nil, {"replay"}, {"absence-sweep"}, {"absence-sweep", ""}, {"trusted-start", "x"}} {
		var out bytes.Buffer
		err := RunProcessObserverV2(ctx, args, strings.NewReader(""), &out)
		expectProcessCode(t, err, "BLOCKED", "process-observer-refused")
		if out.Len() != 0 {
			t.Fatalf("%v wrote output", args)
		}
	}
	var out bytes.Buffer
	err := RunProcessObserverV2(ctx, []string{"absence-sweep", "x"}, strings.NewReader("early"), &out)
	expectProcessCode(t, err, "BLOCKED", "process-observer-refused")

	self, err := selfExecutableSHA256V2()
	if err != nil {
		t.Fatal(err)
	}
	start := postmergeproof.TrustedStartV2{Profile: "postmerge-trusted-start/2", ExecutionID: "x", PolicySHA256: strings.Repeat("1", 64),
		RequestSHA256: strings.Repeat("2", 64), SupervisorSHA256: self, ObserverSHA256: strings.Repeat("3", 64),
		HostTupleSHA256: strings.Repeat("4", 64), RootCaptureIndex: 2}
	data, _ := json.Marshal(start)
	err = RunProcessObserverV2(ctx, []string{"trusted-start"}, bytes.NewReader(data), &out)
	expectProcessCode(t, err, "BLOCKED", "process-observer-refused")
	err = RunProcessObserverV2(ctx, []string{"trusted-start"}, strings.NewReader(`{"profile":"postmerge-trusted-start/2","extra":1}`), &out)
	expectProcessCode(t, err, "REJECTED", "process-wire-invalid")
	start.ObserverSHA256 = self
	data, _ = json.Marshal(start)
	if err := RunProcessObserverV2(ctx, []string{"trusted-start"}, bytes.NewReader(data), &out); err != nil || !bytes.Equal(out.Bytes(), data) {
		t.Fatalf("trusted start: %v %s", err, out.Bytes())
	}
	out.Reset()
	if err := RunProcessObserverV2(ctx, []string{"absence-sweep", "x"}, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	var sweep postmergeproof.AbsenceSweepDocumentV2
	if err := json.Unmarshal(out.Bytes(), &sweep); err != nil || sweep.ExecutionID != "x" || len(sweep.Processes) == 0 {
		t.Fatalf("absence sweep: %v %+v", err, sweep.ExecutionID)
	}
}
