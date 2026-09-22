package sqlnative

import (
	"fmt"
	"strings"
	"testing"
)

func BenchmarkAnalyzeCausalSQL(b *testing.B) {
	request := Request{Profile: Profile, Family: Family, RequestID: "causal-request", ScopeID: "root", CompilationUnitID: "unit", Target: Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}}
	for i := 0; i < 4; i++ {
		request.Inputs = append(request.Inputs, RequestForInput(fmt.Sprintf("input-%d", i), "sqlite.query", fmt.Sprintf("internal/store/%03d.sql", i), []byte(strings.Repeat("SELECT 1;", 8))))
	}
	frame := EncodeRequest(request)
	analyzer := New()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = analyzer.AnalyzeFrame(frame)
	}
}
