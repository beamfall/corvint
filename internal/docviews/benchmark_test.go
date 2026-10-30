package docviews

import (
	"fmt"
	"testing"

	"github.com/Beamfall/corvint/internal/doccompiler"
)

func BenchmarkCompileAndVerifyFiveViewsThousandClaims(b *testing.B) {
	claims := make([]doccompiler.Clause, 0, 1_000)
	ids := make([]string, 0, 1_000)
	for index := 0; index < 1_000; index++ {
		id := fmt.Sprintf("C-%04d", index)
		ids = append(ids, id)
		switch index % 3 {
		case 0:
			claims = append(claims, testSupportedClaim(id))
		case 1:
			claims = append(claims, testConflictedClaim(id, "Evidence sources disagree."))
		default:
			claims = append(claims, testUnknownClaim(id, "Behavior is not established."))
		}
	}
	plan := testSource(b, claims)
	options := testOptions(ids)
	bundle, err := Compile(plan, options)
	if err != nil {
		b.Fatal(err)
	}

	b.Run("compile", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, err := Compile(plan, options); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("verify", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if report := Verify(plan, bundle); report.Status != "PASS" {
				b.Fatal(report)
			}
		}
	})
}
