// Owned replay fixture: its variant is fixed by the repository build helper.
package main

import (
	"fmt"
	"os"
)

const variant = "old"

func main() {
	if len(os.Args) != 4 || os.Args[1] != "--owned-fixture" || os.Args[2] != "stdout" {
		os.Exit(125)
	}
	fmt.Fprint(os.Stdout, variant+":"+os.Args[3])
}
