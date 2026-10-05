//go:build darwin || linux

package dispatch

import (
	"reflect"
	"testing"
)

// CAL-V0-097: a ticket that requires a pool goes only to a role whose
// workers claim that pool, with {pool} bound; a role naming a pool takes
// only that pool's tickets, and TicketPools lists the claimable pools.
func TestCALV0097_RosterRoutesPoolTicketsToMatchingRole(t *testing.T) {
	c := testConfig(t, "exit 0")
	c.GlobalCap = 4
	c.Roles = []Role{
		{Name: "impl", Host: "sh", Cap: 2, Match: &Match{PlanSelected: true}, Prompt: "p", IdleSeconds: 30, WallSeconds: 60},
		{Name: "laned", Host: "sh", Cap: 2, Match: &Match{PlanSelected: true, Pool: "lanes"}, Prompt: "p {pool}", IdleSeconds: 30, WallSeconds: 60},
	}
	pooled, other, free := ticket("p", "P0", 1), ticket("o", "P0", 2), ticket("f", "P2", 3)
	pooled.RequiresPool, other.RequiresPool = "lanes", "other"
	for _, x := range []*Ticket{&pooled, &other, &free} {
		x.Plan = "SELECTED"
	}
	got := Roster(c, &Observation{Tickets: []Ticket{pooled, other, free}}, nil, nil)
	want := []Assignment{
		{Role: "laned", Key: "ticket:a:q:p", Ticket: "ticket:a:q:p", Local: "p", State: StateNone, Pool: "lanes", Slot: 1},
		{Role: "impl", Key: "ticket:a:q:f", Ticket: "ticket:a:q:f", Local: "f", State: StateNone, Slot: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("roster\n got %+v\nwant %+v", got, want)
	}
	if pools := c.TicketPools(); !reflect.DeepEqual(pools, []string{"lanes"}) {
		t.Fatalf("TicketPools = %v", pools)
	}
	c.Roles = c.Roles[:1]
	if pools := c.TicketPools(); pools == nil || len(pools) != 0 {
		t.Fatalf("TicketPools without pool roles = %#v, want empty non-nil", pools)
	}
}
