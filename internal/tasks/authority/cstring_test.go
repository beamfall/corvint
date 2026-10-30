//go:build darwin || linux

package authority

import (
	"runtime"
	"testing"
)

// TestCStringMatchesNULPrefix pins the fixed-array name decoding the
// fixture session's mount observation compares: the bytes before the first
// NUL, the whole array when none, and the empty string for a leading NUL.
func TestCStringMatchesNULPrefix(t *testing.T) {
	arr := func(s string, size int) []int8 {
		out := make([]int8, size)
		for i := 0; i < len(s) && i < size; i++ {
			out[i] = int8(s[i])
		}
		return out
	}
	for _, c := range []struct {
		in   []int8
		want string
	}{
		{arr("apfs", 16), "apfs"},
		{arr("/System/Volumes/Data", 1024), "/System/Volumes/Data"},
		{arr("", 16), ""},
		{arr("abcd", 4), "abcd"},
		{[]int8{'a', 0, 'b', 0}, "a"},
		{nil, ""},
	} {
		if got := cString(c.in); got != c.want {
			t.Fatalf("cString(%v) = %q, want %q", c.in, got, c.want)
		}
	}
	// The result is sized to the name, not to the 1024-byte mount array.
	big := arr("/Volumes/x", 1024)
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	for i := 0; i < 1000; i++ {
		sink = cString(big)
	}
	runtime.ReadMemStats(&after)
	if per := (after.TotalAlloc - before.TotalAlloc) / 1000; per > 64 {
		t.Fatalf("cString allocated %d bytes per call for a 10-byte name, want at most 64", per)
	}
}

var sink string

func BenchmarkCStringMountName(b *testing.B) {
	in := make([]int8, 1024)
	for i, c := range "/System/Volumes/Data" {
		in[i] = int8(c)
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = cString(in)
	}
}
