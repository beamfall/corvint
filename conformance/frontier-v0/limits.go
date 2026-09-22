// Copyright 2026 Russell Lewis
// Licensed under the Apache License, Version 2.0.

package main

// CF-V0-023 fixes the Frontier-owned limits exactly. They are transcribed here
// from the clause text, one constant per named bound, so a limit fixture at N
// and at N+1 can be generated rather than hand-counted.
//
//	"Frontier-owned limits are 2,048 hunk items, 256 intent-change items,
//	 256 intent-test items, 2,560 total items, 64 related IDs per item,
//	 32 reasons per item, and 4,194,304 complete output bytes. Checked counts
//	 occur before append and checked byte arithmetic occurs before output
//	 allocation. Exceeding a limit fails `frontier-resource-exhausted`, exit 2,
//	 with no partial Frontier result."
const (
	LimitHunkItems         = 2048
	LimitIntentChangeItems = 256
	LimitIntentTestItems   = 256
	LimitTotalItems        = 2560
	LimitRelatedIDsPerItem = 64
	LimitReasonsPerItem    = 32
	LimitOutputBytes       = 4194304
)

// Limit is one named CF-V0-023 bound.
type Limit struct {
	Name  string
	Value int
	// Clause note recording what the bound counts.
	Counts string
}

// Limits is the exhaustive CF-V0-023 set. The limit fixture generates an at-N
// and an at-N+1 case for every entry; a bound missing here would silently go
// untested, so the manifest test asserts the count.
var Limits = []Limit{
	{"hunkItems", LimitHunkItems, "HUNK_BASIS items"},
	{"intentChangeItems", LimitIntentChangeItems, "INTENT_CHANGE items"},
	{"intentTestItems", LimitIntentTestItems, "INTENT_TEST items"},
	{"totalItems", LimitTotalItems, "items of every kind"},
	{"relatedIdsPerItem", LimitRelatedIDsPerItem, "relatedIds on one item"},
	{"reasonsPerItem", LimitReasonsPerItem, "reasons on one item"},
	{"outputBytes", LimitOutputBytes, "bytes of the complete output document"},
}

// TotalItemsIsNotTheSumOfKindLimits records a real trap in CF-V0-023: the total
// bound is 2,560, while the three kind bounds sum to 2,048+256+256 = 2,560 as
// well. They coincide, so an implementation that checks only the total, or only
// the three kinds, passes a naive test. The limit fixtures therefore drive each
// kind bound past N with the other kinds empty, so exceeding one kind bound is
// visible while the total is still far below its own limit.
const TotalItemsIsNotTheSumOfKindLimits = LimitHunkItems+LimitIntentChangeItems+LimitIntentTestItems == LimitTotalItems
