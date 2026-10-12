#!/bin/sh
. "$(dirname "$0")/../lib/solo.sh"
test "$(solo_value)" = 7
