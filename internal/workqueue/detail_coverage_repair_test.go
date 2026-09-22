package workqueue

import "testing"

func coverageFixture() (*Snapshot, *DetailsDocument) {
	ticket := testTicket("one", 1)
	detail := Detail{RepositoryAuthorityID: testRepository, TicketID: ticket.TicketID, TicketVersionID: ticket.TicketVersionID, Payload: DetailPayload{AcceptanceCriteria: []string{}, EvidenceHandles: []string{}}}
	RefreshDetail(&detail)
	ticket.DetailPayloadSHA256 = &detail.PayloadSHA256
	snapshot := testSnapshot(ticket)
	snapshot.DetailRequestTicketVersionIDs = []string{ticket.TicketVersionID}
	RefreshSnapshot(snapshot)
	document := &DetailsDocument{SnapshotID: snapshot.ID, Details: []Detail{detail}}
	RefreshDetails(document)
	return snapshot, document
}

func TestDetailCoverageRepair(t *testing.T) {
	t.Run("WQO-V0-012", func(t *testing.T) {
		cases := []struct {
			name   string
			mutate func(*Snapshot, *DetailsDocument)
			state  string
			codes  []string
		}{
			{"complete", func(*Snapshot, *DetailsDocument) {}, StateValidated, nil},
			{"missing", func(s *Snapshot, d *DetailsDocument) { d.Details = []Detail{} }, StatePartial, []string{UnknownDetailMissing}},
			{"wrong stable ticket", func(s *Snapshot, d *DetailsDocument) { d.Details[0].TicketID = "ticket:corvint:worklist:other" }, StateConflicted, nil},
			{"foreign stable ticket", func(s *Snapshot, d *DetailsDocument) { d.Details[0].TicketID = "ticket:foreign:worklist:other" }, StateConflicted, []string{UnknownMultiRepoUnsupported}},
			{"foreign repository field only", func(s *Snapshot, d *DetailsDocument) { d.Details[0].RepositoryAuthorityID = "repo:foreign" }, StateUnknown, []string{UnknownMultiRepoUnsupported}},
			{"extra version", func(s *Snapshot, d *DetailsDocument) {
				other := d.Details[0]
				other.TicketVersionID = "ticket-version:sha256:" + testZeroDigest
				d.Details = append(d.Details, other)
			}, StateConflicted, nil},
			{"duplicate version", func(s *Snapshot, d *DetailsDocument) {
				other := d.Details[0]
				body := "other"
				other.Payload.Body = &body
				d.Details = append(d.Details, other)
			}, StateConflicted, nil},
			{"changed payload digest", func(s *Snapshot, d *DetailsDocument) { body := "changed"; d.Details[0].Payload.Body = &body }, StateConflicted, nil},
			{"no requests", func(s *Snapshot, d *DetailsDocument) {
				s.DetailRequestTicketVersionIDs = []string{}
				d.Details = []Detail{}
			}, StateValidated, nil},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				s, d := coverageFixture()
				c.mutate(s, d)
				RefreshSnapshot(s)
				d.SnapshotID = s.ID
				for i := range d.Details {
					RefreshDetail(&d.Details[i])
				}
				RefreshDetails(d)
				parsed, err := ParseDetails(d.Canonical())
				if err != nil {
					t.Fatal(err)
				}
				got := ValidateDetailCoverage(s, parsed)
				if got.State != c.state || !equalStrings(got.Unknowns, c.codes) {
					t.Fatalf("got %+v; want %s %v", got, c.state, c.codes)
				}
			})
		}
	})
}

func TestDetailRequestMustResolveExactlyOnce(t *testing.T) {
	t.Run("WQO-V0-012", func(t *testing.T) {
		s, d := coverageFixture()
		s.Tickets = append(s.Tickets, s.Tickets[0])
		s.Scope.TicketCount++
		RefreshSnapshot(s)
		d.SnapshotID = s.ID
		RefreshDetails(d)
		if got := ValidateDetailCoverage(s, d); got.State != StateConflicted {
			t.Fatalf("duplicate summary tuple accepted: %+v", got)
		}
	})
}

func TestForeignAtomicAndMissingOwnAuthority(t *testing.T) {
	t.Run("WQO-V0-024", func(t *testing.T) {
		s := testSnapshot(testTicket("one", 1))
		s.Tickets[0].AtomicRepositoryAuthorityIDs = []string{"repo:foreign"}
		RefreshSnapshot(s)
		got := ValidateSnapshot(s)
		if got.State != StateConflicted || !equalStrings(got.Unknowns, []string{UnknownMultiRepoUnsupported}) {
			t.Fatalf("got %+v", got)
		}
	})
}

func TestMissingDetailSuppressesWave(t *testing.T) {
	t.Run("WQO-V0-031", func(t *testing.T) {
		snapshot, details := coverageFixture()
		details.Details = []Detail{}
		RefreshDetails(details)
		result := ValidateDetailCoverage(snapshot, details)
		if result.State != StatePartial || !equalStrings(result.Unknowns, []string{UnknownDetailMissing}) {
			t.Fatalf("missing detail: %+v", result)
		}
		snapshot.ObservationState = result.State
		snapshot.ObservationUnknowns = result.Unknowns
		proposal, err := ProposeWave(snapshot, testEnvelope(snapshot), CollisionClosure{Complete: true}, 1)
		if err != nil {
			t.Fatal(err)
		}
		if proposal.State != "EMPTY" || len(proposal.Entries) != 0 || proposal.MutationAuthority || !equalStrings(proposal.Unknowns, result.Unknowns) {
			t.Fatalf("missing detail produced wave: %+v", proposal)
		}
	})
}
