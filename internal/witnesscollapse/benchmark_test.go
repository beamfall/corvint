package witnesscollapse

import (
	"runtime"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

func BenchmarkCWCX0ParseMaxCEM(b *testing.B) {
	raw, _ := testCEM(b, 4096, false)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		document, err := wire.ParseMap(raw)
		if err != nil {
			b.Fatal(err)
		}
		runtime.KeepAlive(document)
	}
}

func BenchmarkCWCX0CompileFiveCopies(b *testing.B) {
	raw, evidence := testCEM(b, 5, true)
	rows := sharedAttributions(evidence, CauseGenerator, digestText("one-generator"))
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		report, err := Compile(raw, rows)
		if err != nil {
			b.Fatal(err)
		}
		runtime.KeepAlive(report)
	}
}

func BenchmarkCWCX0CompileMaxDisconnected(b *testing.B) {
	raw, rows := benchmarkFixture(b, false)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		report, err := Compile(raw, rows)
		if err != nil {
			b.Fatal(err)
		}
		runtime.KeepAlive(report)
	}
}

func BenchmarkCWCX0CompileMaxConnected(b *testing.B) {
	raw, rows := benchmarkFixture(b, true)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		report, err := Compile(raw, rows)
		if err != nil {
			b.Fatal(err)
		}
		runtime.KeepAlive(report)
	}
}

func BenchmarkCWCX0CanonicalAndVerifyMaxReport(b *testing.B) {
	raw, rows := benchmarkFixture(b, true)
	report, err := Compile(raw, rows)
	if err != nil {
		b.Fatal(err)
	}
	b.Run("canonical", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			document, digest, err := Canonical(report)
			if err != nil {
				b.Fatal(err)
			}
			runtime.KeepAlive(document)
			runtime.KeepAlive(digest)
		}
	})
	b.Run("verify", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if err := Verify(raw, report); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func benchmarkFixture(tb testing.TB, connected bool) ([]byte, []Attribution) {
	tb.Helper()
	raw, evidence := testCEM(tb, 4096, false)
	rows := make([]Attribution, 0, MaxAttributions)
	for causeIndex := 0; causeIndex < MaxCauses; causeIndex++ {
		left, right := causeIndex%(len(evidence)-1), causeIndex%(len(evidence)-1)+1
		if !connected {
			left = causeIndex % (len(evidence) / 2)
			left *= 2
			right = left + 1
		}
		subject := digestText(string(rune(causeIndex)))
		attestation := evidence[(right+1)%len(evidence)]
		rows = append(rows,
			attribution(evidence[left], attestation, CauseGenerator, subject),
			attribution(evidence[right], attestation, CauseGenerator, subject),
		)
	}
	return raw, rows
}
