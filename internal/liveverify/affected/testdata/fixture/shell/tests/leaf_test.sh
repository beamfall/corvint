#!/bin/sh
. "$(dirname "$0")/../lib/leaf.sh"
test "$(leaf_value)" = 3
