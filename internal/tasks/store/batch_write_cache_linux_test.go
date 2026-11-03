// SPDX-License-Identifier: Apache-2.0

//go:build linux

package store

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TCX-V0-977, CAL-V0-070: a dirty MAP_SHARED write can leave every reuse-key
// field unchanged. Both writer binding and gate pre-read binding must read
// live content even when the command's modeling cache is eligible for reuse.
func TestTCXV0977_BindingRefusesMappedWriteWithoutTimestampChange(t *testing.T) {
	for _, gateRead := range []bool{false, true} {
		t.Run(map[bool]string{false: "writer", true: "gate-pre-read"}[gateRead], func(t *testing.T) {
			s := newGatedStore(t)
			ctx := WithBatchWrites(context.Background())
			if gateRead {
				if rep, err := s.submit("mapped-pre-read-submit")(ctx); err != nil || rep.Outcome.Outcome != "COMPLETED" {
					t.Fatalf("submit: %+v %v", rep, err)
				}
			}
			_, target := mutationBoundaryFiles(t, s.repo)
			selected, err := wire.ParseTicketID("test", s.ticket)
			if err != nil || filepath.Base(target) == selected.Local+".json" {
				t.Fatalf("mapped target must be an unselected ticket: %s vs %s, %v", target, s.ticket, err)
			}
			flip := mapShared(t, target)
			// Fault the mapping writable before aging: on tmpfs even the first
			// write changes no timestamps; on other Linux filesystems a later
			// write through an already dirty mapping has the same property.
			flip()
			flip()
			time.Sleep(2100 * time.Millisecond)
			info, err := os.Stat(target)
			if err != nil {
				t.Fatal(err)
			}
			stamp := info.Sys().(*syscall.Stat_t)
			if stamp.Ctim.Nsec == 0 {
				t.Skip("filesystem has coarse ctime; stat cache cannot reuse this entry")
			}
			if _, err := captureWriteTree(ctx, s.repo); err != nil {
				t.Fatal(err)
			}
			before := mutationPublished(t, s.repo)
			edited := false
			ctx = context.WithValue(ctx, mutationStageKey{}, func(stage string) {
				if edited || !(gateRead && stage == "batch.pre-read.audit" || !gateRead && strings.HasPrefix(stage, "fast.lease.model=")) {
					return
				}
				flip()
				edited = true
				changed, err := os.Stat(target)
				if err != nil {
					t.Fatal(err)
				}
				st := changed.Sys().(*syscall.Stat_t)
				if !os.SameFile(info, changed) || info.Mode() != changed.Mode() || info.Size() != changed.Size() || stamp.Mtim != st.Mtim || stamp.Ctim != st.Ctim {
					t.Fatal("mapped write changed a cache key; regression did not exercise the bypass")
				}
			})
			if gateRead {
				if _, err := checkpointedLeaseProof(ctx, s.repo); err == nil {
					t.Error("gate pre-read binding accepted divergent mapped bytes")
				}
			} else {
				rep, err := s.submit("mapped-before-binding")(ctx)
				if err == nil && rep != nil && rep.Outcome.Outcome == "COMPLETED" {
					t.Error("pre-effect binding accepted divergent mapped bytes")
				}
			}
			if !edited {
				t.Fatal("did not reach the binding boundary")
			}
			if after := mutationPublished(t, s.repo); after != before {
				t.Fatalf("refused operation published: %s vs %s", after, before)
			}
		})
	}
}
