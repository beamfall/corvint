// Copyright 2026 Corvint contributors.
// SPDX-License-Identifier: AGPL-3.0-or-later
package postmergehost

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"

	"github.com/Beamfall/corvint/internal/postmergeproof"
	"github.com/Beamfall/corvint/internal/postmergeproof/procfs"
)

func observerRefusedV2(detail string) error {
	return &postmergeproof.ProcessErrorV2{Outcome: postmergeproof.OutcomeBlockedV2, Code: "process-observer-refused", Detail: detail}
}

// RunProcessObserverV2 is the pinned observer's internal role (PMR-V2-006).
// args select one mode:
//
//   - `trusted-start`: read one strict TrustedStartV2 on stdin, check that its
//     observer identity is this process's own executable bytes and write it.
//   - `absence-sweep EXECUTION_ID`: wait for an empty stdin to close, sweep the
//     /proc PID inventory in this process and write an AbsenceSweepDocumentV2.
//
// The observer derives no role, owner or absence fact; the offline verifier
// does. On a host without the Linux procfs profile it reports NOT_OBSERVED.
func RunProcessObserverV2(ctx context.Context, args []string, stdin io.Reader, stdout io.Writer) error {
	if !procfs.Supported {
		return procfs.UnsupportedError{}
	}
	switch {
	case len(args) == 1 && args[0] == "trusted-start":
		data, err := io.ReadAll(io.LimitReader(stdin, processStdinLimitV2+1))
		if err != nil || len(data) > processStdinLimitV2 {
			return observerRefusedV2("trusted start input is unreadable or exceeds its bound")
		}
		start, err := postmergeproof.DecodeTrustedStartV2(data)
		if err != nil {
			return err
		}
		self, err := selfExecutableSHA256V2()
		if err != nil || self != start.ObserverSHA256 {
			return observerRefusedV2("trusted start names another observer executable")
		}
		return writeObserverV2(stdout, start)
	case len(args) == 2 && args[0] == "absence-sweep" && args[1] != "":
		// The parent closes stdin only after it captured this birth, so the
		// sweep cannot precede its own observer capture.
		if n, err := io.Copy(io.Discard, io.LimitReader(stdin, 1)); err != nil || n != 0 {
			return observerRefusedV2("absence sweep stdin must be empty")
		}
		sweep, err := procfs.SweepProcesses(ctx, processSweepLimitV2)
		if err != nil {
			return observerRefusedV2("absence sweep: " + err.Error())
		}
		rows := make([]postmergeproof.SweepProcessV2, 0, len(sweep.Processes))
		for _, row := range sweep.Processes {
			rows = append(rows, postmergeproof.SweepProcessV2{PID: row.PID, StatBytes: row.Stat})
		}
		return writeObserverV2(stdout, postmergeproof.AbsenceSweepDocumentV2{Profile: "postmerge-absence-sweep/2", ExecutionID: args[1],
			BootIDBytes: sweep.BootID, NamespaceLinkBytes: sweep.NamespaceLink, PIDDirectoryBytes: sweep.PIDDirectory,
			Processes: rows, ReadFailures: sweep.ReadFailures})
	}
	return observerRefusedV2("unknown observer mode")
}

func writeObserverV2(stdout io.Writer, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return observerRefusedV2("observer output encoding: " + err.Error())
	}
	if _, err := stdout.Write(data); err != nil {
		return observerRefusedV2("observer output: " + err.Error())
	}
	return nil
}

// selfExecutableSHA256V2 hashes this process's executable through procfs.
func selfExecutableSHA256V2() (string, error) {
	file, err := os.Open("/proc/self/exe")
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, io.LimitReader(file, processExecutableLimitV2)); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
