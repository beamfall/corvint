package procgroup

import (
	"context"
	"fmt"
	"slices"
	"testing"
)

// NEA-V0-006/007: sequential exits must not consume resident capacity forever.
func TestV10689BoundedChurn(t *testing.T) {
	root := ObservedProcess{PID: 1, Start: "root", State: "S"}
	parent := ObservedProcess{PID: 2, ParentPID: 1, Start: "parent", State: "S"}
	done := make(chan struct{})
	close(done)
	o := &descendantObserver{root: 1, rootIdentity: root, known: map[int]ObservedProcess{}, stop: make(chan struct{}), done: done,
		snap: func(context.Context) (map[int]ObservedProcess, error) { return map[int]ObservedProcess{}, nil }}
	for n := 0; n < 4200; n++ {
		child := ObservedProcess{PID: n + 3, ParentPID: 2, Start: fmt.Sprint(n), State: "S"}
		o.expand(map[int]ObservedProcess{1: root, 2: parent, child.PID: child})
	}
	report, err := o.finish()
	if err != nil || !report.Absent || len(report.Failures) != 0 {
		t.Fatalf("bounded sequential churn must finish absent without cumulative-cap failure: err=%v absent=%v failures=%v", err, report.Absent, report.Failures)
	}
}

func v10689Observer() (*descendantObserver, map[int]ObservedProcess) {
	root := ObservedProcess{PID: 1, Start: "root", State: "S"}
	return &descendantObserver{root: 1, rootIdentity: root, known: map[int]ObservedProcess{}}, map[int]ObservedProcess{1: root}
}

func v10689Child(pid, parent int, start, state string) ObservedProcess {
	return ObservedProcess{PID: pid, ParentPID: parent, Start: start, State: state}
}

func TestV10689ResidentTransitions(t *testing.T) {
	t.Run("visible zombie retains continuity and ancestry after root loss", func(t *testing.T) {
		o, rows := v10689Observer()
		rows[2] = v10689Child(2, 1, "zombie", "Z+")
		o.expand(rows)
		delete(rows, 1)
		for n := 0; n < 4200; n++ {
			o.expand(rows)
		}
		rows[3] = v10689Child(3, 2, "child", "S")
		o.expand(rows)
		if o.failure != nil || len(o.known) != 2 || len(o.witnesses) != 2 {
			t.Fatalf("lost zombie continuity: %+v", o)
		}
		rows[2] = v10689Child(2, 99, "replacement", "Z")
		rows[4] = v10689Child(4, 2, "unrelated", "S")
		rows[3] = v10689Child(3, 99, "child", "S")
		o.expand(rows)
		if len(o.known) != 1 || o.known[3].ParentPID != 99 {
			t.Fatalf("reuse inherited ancestry or reparented child lost: %v", o.known)
		}
	})
	t.Run("immutable root reuse requires independent ancestry", func(t *testing.T) {
		o, rows := v10689Observer()
		rows[2] = v10689Child(2, 1, "owned", "S")
		o.expand(rows)
		rows[1] = v10689Child(1, 99, "newroot", "S")
		rows[3] = v10689Child(3, 1, "unrelated", "S")
		o.expand(rows)
		if len(o.known) != 1 {
			t.Fatalf("replacement inherited root: %v", o.known)
		}
		rows[1] = v10689Child(1, 2, "newroot", "S")
		o.expand(rows)
		if len(o.known) != 3 || o.known[1].Start != "newroot" || o.rootIdentity.Start != "root" {
			t.Fatalf("independently owned root PID not retained: %+v", o)
		}
	})
	for _, replacement := range []string{"same PID", "new PID"} {
		t.Run("at capacity replacement "+replacement, func(t *testing.T) {
			o, rows := v10689Observer()
			for pid := 2; pid <= 4096; pid++ {
				rows[pid] = v10689Child(pid, 1, "old", "Z")
			}
			o.expand(rows)
			delete(rows, 2)
			pid := 2
			if replacement == "new PID" {
				pid = 5000
			}
			rows[pid] = v10689Child(pid, 1, "new", "S")
			o.expand(rows)
			if o.failure != nil || len(o.known) != 4095 || o.known[pid].Start != "new" {
				t.Fatalf("valid replacement refused: %v residents=%d", o.failure, len(o.known))
			}
			rows[6000] = v10689Child(6000, 1, "overflow", "S")
			o.expand(rows)
			if o.failure == nil || len(o.known) != 4095 || !o.residentOverflow {
				t.Fatal("resident overflow passed or grew storage")
			}
			o.expand(map[int]ObservedProcess{})
			if o.failure == nil {
				t.Fatal("overflow failure disappeared after retirement")
			}
			report := &DescendantObservation{}
			o.displayProcesses(report)
			if !slices.Contains(report.Limitations, descendantUnresolvedOmission) {
				t.Fatal("overflow omission not disclosed")
			}
		})
	}
}

func TestV10689WitnessSelection(t *testing.T) {
	var first []ObservedProcess
	for reverse := 0; reverse < 2; reverse++ {
		o, rows := v10689Observer()
		for n := 0; n < 4095; n++ {
			pid := n + 2
			if reverse != 0 {
				pid = 4096 - n
			}
			rows[pid] = v10689Child(pid, 1, "old", "Z")
		}
		o.expand(rows)
		o.expand(map[int]ObservedProcess{1: rows[1], 9000: v10689Child(9000, 1, "late-live", "S")})
		report := &DescendantObservation{}
		o.displayProcesses(report)
		if len(o.known) != 1 || len(o.witnesses) != 4095 || len(report.Processes) != 4095 {
			t.Fatal("cardinality bound changed")
		}
		if !slices.Contains(report.Processes, o.known[9000]) || !slices.Contains(report.Limitations, descendantWitnessOmission) {
			t.Fatal("late live identity hidden by historical sample")
		}
		if reverse == 0 {
			first = report.Processes
		} else if !slices.Equal(first, report.Processes) {
			t.Fatal("map insertion order changed selection")
		}
	}
}
