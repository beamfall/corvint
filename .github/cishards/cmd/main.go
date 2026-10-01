// Command ci-shards emits one complete-universe shard of typed Go packages.
package main

import (
	"flag"
	"fmt"
	"github.com/Beamfall/corvint/.github/cishards"
	"io"
	"os"
)

func main() {
	shard := flag.Int("shard", 0, "zero-based shard index")
	total := flag.Int("shards", 4, "complete partition shard count")
	profile := flag.Bool("profile", false, "print protected partition digest")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected arguments")
		os.Exit(2)
	}
	if *profile {
		fmt.Println(cishards.ProfileDigest())
		return
	}
	b, err := io.ReadAll(io.LimitReader(os.Stdin, cishards.MaxInputBytes+1))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	packages, err := cishards.Packages(b)
	if err == nil {
		packages, err = cishards.Intersect(packages, []string{"./..."}, *shard, *total)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	for _, p := range packages {
		fmt.Println(p)
	}
}
