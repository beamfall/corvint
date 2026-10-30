package workqueue

import "testing"

const capacityCPU = "capacity:corvint:worklist:cpu"
const capacityGPU = "capacity:corvint:worklist:gpu"

func TestCapacityRepair(t *testing.T) {
	t.Run("WQO-V0-019 invalid capacity conflicts", func(t *testing.T) {
		tests := []struct {
			name   string
			change func(*Snapshot, *CapacityEnvelope)
		}{
			{"missing own class", func(s *Snapshot, e *CapacityEnvelope) { e.Available = nil }},
			{"duplicate caller class", func(s *Snapshot, e *CapacityEnvelope) { e.Available = append(e.Available, e.Available[0]) }},
			{"conflicting caller class", func(s *Snapshot, e *CapacityEnvelope) {
				e.Available = append(e.Available, CapacityClass{ID: capacityCPU, AvailableUnits: 3})
			}},
			{"unknown own class", func(s *Snapshot, e *CapacityEnvelope) {
				e.Available = append(e.Available, CapacityClass{ID: capacityGPU, AvailableUnits: 1})
			}},
			{"duplicate repository class", func(s *Snapshot, e *CapacityEnvelope) {
				s.CapacityClasses = append(s.CapacityClasses, s.CapacityClasses[0])
			}},
			{"caller Count overflow", func(s *Snapshot, e *CapacityEnvelope) { e.Available[0].AvailableUnits = Count(maxCount + 1) }},
			{"repository Count overflow", func(s *Snapshot, e *CapacityEnvelope) { s.CapacityClasses[0].AvailableUnits = Count(maxCount + 1) }},
			{"duplicate ticket use", func(s *Snapshot, e *CapacityEnvelope) {
				s.Tickets[0].CapacityUses = append(s.Tickets[0].CapacityUses, s.Tickets[0].CapacityUses[0])
			}},
			{"zero ticket use", func(s *Snapshot, e *CapacityEnvelope) { s.Tickets[0].CapacityUses[0].Units = 0 }},
			{"use Count overflow", func(s *Snapshot, e *CapacityEnvelope) { s.Tickets[0].CapacityUses[0].Units = Count(maxCount + 1) }},
			{"missing declared use reference", func(s *Snapshot, e *CapacityEnvelope) { s.Tickets[0].CapacityUses[0].ClassID = capacityGPU }},
			{"arithmetic overflow", func(s *Snapshot, e *CapacityEnvelope) {
				s.CapacityClasses = append(s.CapacityClasses, CapacityClass{ID: capacityGPU, AvailableUnits: Count(maxCount)})
				e.Available = append(e.Available, CapacityClass{ID: capacityGPU, AvailableUnits: Count(maxCount)})
				s.Tickets[0].CapacityUses = []CapacityUse{{capacityCPU, Count(maxCount)}, {capacityGPU, 1}}
			}},
			{"zero lease use", func(s *Snapshot, e *CapacityEnvelope) {
				s.Leases = []LeaseSummary{capacityLease(s, []CapacityUse{{capacityCPU, 0}})}
			}},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				s, e := capacityInputs()
				test.change(s, e)
				refreshEnvelopeID(e)
				p, err := ProposeWave(s, e, CollisionClosure{Complete: true}, 2)
				if ErrorCode(err) != CodeConflicted || p != nil {
					t.Fatalf("invalid local capacity must conflict without proposal: proposal=%#v error=%v", p, err)
				}
			})
		}
	})
	t.Run("WQO-V0-018 nonvalidated state abstention", func(t *testing.T) {
		for _, state := range []string{StateUnknown, StatePartial, StateStale, StateConflicted} {
			t.Run(state, func(t *testing.T) {
				s, e := capacityInputs()
				s.ObservationState = state
				e.Available = nil
				refreshEnvelopeID(e)
				t.Run("WQO-V0-045 no mutation authority", func(t *testing.T) {
					assertCapacityEmpty(t, s, e)
				})
			})
		}
	})
	t.Run("WQO-V0-024 foreign authority abstention", func(t *testing.T) {
		for _, field := range []string{"repository", "caller class", "capability", "repository class", "ticket use", "lease use"} {
			t.Run(field, func(t *testing.T) {
				s, e := capacityInputs()
				switch field {
				case "repository":
					e.RepositoryAuthorityID = "repo:foreign"
				case "caller class":
					e.Available[0].ID = "capacity:foreign:worklist:cpu"
				case "capability":
					e.Capabilities = []string{"capability:foreign:worklist:go"}
				case "repository class":
					s.CapacityClasses[0].ID = "capacity:foreign:worklist:cpu"
				case "ticket use":
					s.Tickets[0].CapacityUses[0].ClassID = "capacity:foreign:worklist:cpu"
				case "lease use":
					s.Leases = []LeaseSummary{capacityLease(s, []CapacityUse{{"capacity:foreign:worklist:cpu", 1}})}
				}
				refreshEnvelopeID(e)
				assertCapacityEmpty(t, s, e)
			})
		}
	})
}

func capacityInputs() (*Snapshot, *CapacityEnvelope) {
	s := testSnapshot(testTicket("one", 1))
	s.Tickets[0].CapacityUses = []CapacityUse{{capacityCPU, 1}}
	s.CapacityClasses = []CapacityClass{{ID: capacityCPU, AvailableUnits: 2}}
	return s, testEnvelope(s, CapacityClass{ID: capacityCPU, AvailableUnits: 2})
}

func capacityLease(s *Snapshot, uses []CapacityUse) LeaseSummary {
	return LeaseSummary{BlocksSelection: true, CapacityUses: uses, HolderID: "holder:corvint:worklist:h", LeaseID: "lease:corvint:worklist:l", QueueAuthorityID: testQueue, RepositoryAuthorityID: testRepository, TicketID: s.Tickets[0].TicketID}
}

func assertCapacityEmpty(t *testing.T, s *Snapshot, e *CapacityEnvelope) {
	t.Helper()
	p, err := ProposeWave(s, e, CollisionClosure{Complete: true}, 2)
	if err != nil || p == nil || p.State != "EMPTY" || len(p.Entries) != 0 || p.MutationAuthority {
		t.Fatalf("expected ordinary EMPTY abstention: proposal=%#v error=%v", p, err)
	}
}

func TestCapacityRepairPoolControls(t *testing.T) {
	t.Run("WQO-V0-019", func(t *testing.T) {
		t.Run("asymmetric minima atomic failure and exact exhaustion", func(t *testing.T) {
			a, b, c := testTicket("a", 1), testTicket("b", 2), testTicket("c", 3)
			a.CapacityUses = []CapacityUse{{capacityCPU, 1}, {capacityGPU, 1}}
			b.CapacityUses = []CapacityUse{{capacityCPU, 1}, {capacityGPU, 1}}
			c.CapacityUses = []CapacityUse{{capacityCPU, 1}}
			s := testSnapshot(a, b, c)
			s.CapacityClasses = []CapacityClass{{ID: capacityCPU, AvailableUnits: 2}, {ID: capacityGPU, AvailableUnits: 3}}
			e := testEnvelope(s, CapacityClass{ID: capacityCPU, AvailableUnits: 3}, CapacityClass{ID: capacityGPU, AvailableUnits: 1})
			p, err := ProposeWave(s, e, CollisionClosure{Complete: true}, 3)
			if err != nil || p == nil {
				t.Fatalf("proposal: %v", err)
			}
			want := []string{reasonEligibleAtCheckpoint, reasonCapacityExhausted, reasonEligibleAtCheckpoint}
			if len(p.Entries) != len(want) {
				t.Fatalf("entries: %#v", p.Entries)
			}
			for i, reason := range want {
				if p.Entries[i].Reason != reason {
					t.Fatalf("entry %d: %#v, want %s", i, p.Entries[i], reason)
				}
			}
		})
		t.Run("active lease capacity already net", func(t *testing.T) {
			s, e := capacityInputs()
			s.CapacityClasses[0].AvailableUnits = 1
			s.Leases = []LeaseSummary{capacityLease(s, []CapacityUse{{capacityCPU, 2}})}
			p, err := ProposeWave(s, e, CollisionClosure{Complete: true}, 1)
			if err != nil || p == nil || len(p.Entries) != 1 || p.Entries[0].State != "SELECTED" {
				t.Fatalf("net capacity double-subtracted: proposal=%#v error=%v", p, err)
			}
		})
		t.Run("zero available is valid exhaustion", func(t *testing.T) {
			s, e := capacityInputs()
			e.Available[0].AvailableUnits = 0
			refreshEnvelopeID(e)
			p, err := ProposeWave(s, e, CollisionClosure{Complete: true}, 1)
			if err != nil || p == nil || len(p.Entries) != 1 || p.Entries[0].Reason != reasonCapacityExhausted {
				t.Fatalf("zero available: proposal=%#v error=%v", p, err)
			}
		})
	})
}
