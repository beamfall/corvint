// SPDX-License-Identifier: Apache-2.0
package mergedrivers

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestAppendDWCV07700CollisionFreeMarkers(t *testing.T) {
	for _, tc := range []struct {
		name, base, ours, theirs, want string
		size, width                    int
		clean                          bool
	}{
		{"heading", "old\n", "Ours\n=======\n", "Theirs\n", "", 7, 8, false},
		{"base-and-separator", "old\n", "||||||| example\n=======\nOurs\n", "Theirs\n", "", 7, 8, false},
		{"end", "old\n", "Ours\n", "Theirs\n>>>>>>> example\n", "", 7, 8, false},
		{"long-run", "old\n", strings.Repeat("|", 2048) + " example\nOurs\n", "Theirs\n", "", 7, 2049, false},
		{"minimum", "old\n", "Ours\n", "Theirs\n", "", 1, 7, false},
		{"literal-append", "", "||||||| example\n=======\nOurs\n", "Theirs\n", "||||||| example\n=======\nOurs\nTheirs\n", 7, 8, true},
		{"literal-before-append", "||||||| example\n=======\n>>>>>>> theirs\nanchor\n", "||||||| example\n=======\n>>>>>>> theirs\nanchor\nOurs\n", "||||||| example\n=======\n>>>>>>> theirs\nanchor\nTheirs\n", "||||||| example\n=======\n>>>>>>> theirs\nanchor\nOurs\nTheirs\n", 7, 8, true},
		{"clean-marker-block", "old\n", "||||||| example\n=======\n>>>>>>> theirs\n", "old\n", "||||||| example\n=======\n>>>>>>> theirs\n", 7, 8, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, clean, err := Append(context.Background(), t.TempDir(), []byte(tc.base), []byte(tc.ours), []byte(tc.theirs), tc.size)
			want := tc.want
			if !tc.clean {
				want = strings.Repeat("<", tc.width) + " ours\n" + tc.ours + strings.Repeat("|", tc.width) + " base\n" + tc.base + strings.Repeat("=", tc.width) + "\n" + tc.theirs + strings.Repeat(">", tc.width) + " theirs\n"
			}
			if err != nil || clean != tc.clean || string(got) != want {
				t.Fatalf("got %q clean=%v err=%v, want %q clean=%v", got, clean, err, want, tc.clean)
			}
		})
	}
}

// Every placement and marker character keeps a real edit conflicted, regardless
// of whether the literal resembles a labeled delimiter or a bare marker line.
func TestAppendDWCV07700DefaultMarkerProperty(t *testing.T) {
	for _, marker := range "<|=>" {
		for _, suffix := range []string{"", " example", " ours", " base", " theirs"} {
			for side := 0; side < 3; side++ {
				t.Run(fmt.Sprintf("%c/%q/%d", marker, suffix, side), func(t *testing.T) {
					inputs := [3]string{"old\n", "Ours\n", "Theirs\n"}
					literal := strings.Repeat(string(marker), 7) + suffix + "\n"
					inputs[side] = literal + inputs[side]
					got, clean, err := Append(context.Background(), t.TempDir(), []byte(inputs[0]), []byte(inputs[1]), []byte(inputs[2]), 7)
					want := "<<<<<<<< ours\n" + inputs[1] + "|||||||| base\n" + inputs[0] + "========\n" + inputs[2] + ">>>>>>>> theirs\n"
					if err != nil || clean || string(got) != want {
						t.Fatalf("real edit changed: got %q clean=%v err=%v, want %q", got, clean, err, want)
					}
				})
			}
		}
	}
}

func TestResolveAppendDWCV07700UnexpectedDelimiters(t *testing.T) {
	valid := "<<<<<<< ours\nA\n||||||| base\n=======\nB\n>>>>>>> theirs\n"
	for _, malformed := range []string{
		"||||||| base\n",
		"=======\n",
		">>>>>>> theirs\n",
		"<<<<<<< ours\n<<<<<<< ours\n||||||| base\n=======\n>>>>>>> theirs\n",
		"<<<<<<< ours\n||||||| base\n||||||| base\n=======\n>>>>>>> theirs\n",
		"<<<<<<< ours\n||||||| base\n=======\n=======\n>>>>>>> theirs\n",
		"<<<<<<< ours\n||||||| base\n>>>>>>> theirs\n=======\n>>>>>>> theirs\n",
		"<<<<<<<< ours\n|||||||| base\n========\n>>>>>>>> theirs\n",
		"<<<<<<< ours\n|||||||| base\n=======\n>>>>>>> theirs\n",
		"<<<<<<< ours\n||||||| base\n========\n>>>>>>> theirs\n",
		"<<<<<<< ours\n||||||| base\n=======\n>>>>>>>> theirs\n",
		"<<<<<<< ours\n||||||| unexpected\n=======\n>>>>>>> theirs\n",
		"<<<<<<< ours\n||||||| base\n=======\n>>>>>>> unexpected\n",
	} {
		for _, prefix := range []string{"", valid} {
			data := prefix + malformed
			got, clean := resolveAppend([]byte(data), 7)
			if clean || string(got) != data {
				t.Errorf("ambiguous conflict changed: got %q clean=%v, want %q", got, clean, data)
			}
		}
	}
}
