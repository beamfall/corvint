//go:build !(darwin || linux)

package main

// openNoFollow has no portable equivalent here; writeFile's Lstat check still refuses a
// final-component symlink that exists before the open.
const openNoFollow = 0
