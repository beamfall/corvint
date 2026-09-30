package opencodequalification

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
)

// This observer never signals a process. Its result is captured before the outer
// supervisor rescues survivors, so a broken target cleanup cannot earn a PASS.
func processSnapshot(ctx context.Context, dir string) (map[int]procgroup.ObservedProcess, error) {
	raw, e := capture(ctx, dir, []string{"LC_ALL=C", "TZ=UTC"}, []string{"/bin/ps", "-axo", "pid=,ppid=,lstart=,stat="})
	if e != nil {
		return nil, e
	}
	rows := map[int]procgroup.ObservedProcess{}
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		f := strings.Fields(line)
		if len(f) != 8 {
			return nil, errors.New("invalid passive process snapshot")
		}
		pid, e := strconv.Atoi(f[0])
		if e != nil {
			return nil, e
		}
		parent, e := strconv.Atoi(f[1])
		if e != nil {
			return nil, e
		}
		rows[pid] = procgroup.ObservedProcess{PID: pid, ParentPID: parent, Start: strings.Join(f[2:7], " "), State: f[7]}
	}
	return rows, nil
}
func witnessDescendants(ctx context.Context, dir string, pid int) (map[int]procgroup.ObservedProcess, error) {
	rows, e := processSnapshot(ctx, dir)
	if e != nil {
		return nil, e
	}
	leader, ok := rows[pid]
	if !ok {
		return nil, errors.New("passive witness leader missing")
	}
	known := map[int]procgroup.ObservedProcess{pid: leader}
	for pass := 0; pass < len(rows); pass++ {
		added := false
		for id, p := range rows {
			if _, ok := known[id]; ok {
				continue
			}
			if _, ok := known[p.ParentPID]; ok {
				known[id] = p
				added = true
			}
		}
		if !added {
			break
		}
	}
	delete(known, pid)
	if len(known) == 0 {
		return nil, errors.New("passive witness observed no descendants")
	}
	return known, nil
}
func witnessAbsent(ctx context.Context, dir string, known map[int]procgroup.ObservedProcess) error {
	if len(known) == 0 {
		return errors.New("passive cleanup witness unavailable")
	}
	// Observe natural exit before the outer supervisor may rescue a survivor.
	// This window does not signal processes or broaden the captured identities.
	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	for {
		rows, e := processSnapshot(ctx, dir)
		if e != nil {
			return fmt.Errorf("passive cleanup witness unavailable before supervisor rescue: %w", e)
		}
		survivor := 0
		for pid, p := range known {
			if now, ok := rows[pid]; ok && now.Start == p.Start && !strings.HasPrefix(now.State, "Z") {
				survivor = pid
				break
			}
		}
		if survivor == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("target left descendant %d alive before supervisor rescue: %w", survivor, ctx.Err())
		case <-time.After(20 * time.Millisecond):
		}
	}
}
