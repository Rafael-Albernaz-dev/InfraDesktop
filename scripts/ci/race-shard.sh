#!/usr/bin/env bash
# Prints the -run regex for one race shard of a package's top-level tests.
#
#   race-shard.sh PKG only "TestA TestB"        just the named heavy tests
#   race-shard.sh PKG rest I N "TestA TestB"    every other test, dealt round-robin, shard I of N
#
# Every named test must exist (a rename fails here instead of leaving a shard empty),
# and the "rest" shards are the complement of the named ones, so each test runs in
# exactly one shard. Subprocess re-execs (-test.run=^X$) are unaffected.
set -euo pipefail
pkg=$1 mode=$2
mapfile -t tests < <(go test -tags webkit2_41 -list '.*' "$pkg" | grep -E '^(Test|Example|Fuzz)' | sort)
(( ${#tests[@]} > 0 )) || { echo "no tests listed in $pkg" >&2; exit 2; }
case $mode in
only) read -ra named <<<"$3" ;;
rest) index=$3 total=$4; read -ra named <<<"${5:-}" ;;
*) echo "mode must be only or rest" >&2; exit 2 ;;
esac
for name in "${named[@]}"; do
	printf '%s\n' "${tests[@]}" | grep -qxF "$name" || { echo "$name not found in $pkg" >&2; exit 2; }
done
picked=()
if [[ $mode == only ]]; then
	picked=("${named[@]}")
else
	(( index >= 1 && index <= total )) || { echo "shard $index out of 1..$total" >&2; exit 2; }
	n=0
	for test in "${tests[@]}"; do
		[[ " ${named[*]} " == *" $test "* ]] && continue
		(( n % total == index - 1 )) && picked+=("$test")
		n=$((n + 1))
	done
fi
(( ${#picked[@]} > 0 )) || { echo "empty shard" >&2; exit 2; }
(IFS='|'; printf '^(%s)$\n' "${picked[*]}")
