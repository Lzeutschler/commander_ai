#!/usr/bin/env bash
# lockstep.sh <out.tsv> [table...]: play our tables on both engines at once and
# compare them decision by decision.
#
# For each table (default: adjusted frontier) the lists it deals are exported as
# a deck pool, and the engine's own lockstep tool (kfuzz) plays PODS four-seat
# pods of them per policy and payment surface: gorge leads, dirac follows, and
# every option list and every resulting state is compared. One row per run goes
# to <out.tsv>. Rows are a function of the engine revision, the decks and the
# seed, not of the machine.
#
# A divergent pod is one where the fast kernel and the reference engine
# disagreed. It is a census, not a gate: the script fails only when a run
# could not be played.
set -euo pipefail
out=${1:?usage: lockstep.sh <out.tsv> [table...]}
shift
tables=${*:-adjusted frontier}
root=$(cd "$(dirname "$0")/.." && pwd)
engine=$root/third_party/gorge
pods=${PODS:-1000}
workers=${WORKERS:-$(getconf _NPROCESSORS_ONLN)}
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

(cd "$engine" && go build -trimpath -o bin/kfuzz ./dirac/oracle/cmd/kfuzz)
mkdir -p "$(dirname "$out")"
printf 'table\tpolicy\tsurface\tpods\tclean\tdivergent\tcapacity\tunmodelled\thang\timplicated\n' >"$out"
for table in $tables; do
	"$root/bin/podbench" export -table "$table" -allow "$root/decks/allow.txt" -out "$tmp/$table" >/dev/null
	for policy in random heuristic; do
		for surface in full autopay; do
			log=$tmp/$table.$policy.$surface.log
			# kfuzz exits 1 on a divergence; the row records it
			(cd "$engine" && bin/kfuzz -kernel cmd5 -decks "$tmp/$table/decks" -pool "$tmp/$table/pool.tsv" \
				-seats 4 -pods "$pods" -seed 1 -keep-going -surface "$surface" -policy "$policy" \
				-mulligans 1 -shrink 0 -workers "$workers" -allow-unverified -implicates-all) >"$log" 2>&1 || true
			line=$(grep -E '^cmd5 pods \(4 seats\):' "$log") || { echo "lockstep: $table $policy $surface did not play:" >&2; tail -5 "$log" >&2; exit 1; }
			num() { echo "$line" | sed -E "s/.* ([0-9]+) $1.*/\1/"; }
			implicated=$(grep -E '^implicates:' "$log" | sed -E 's/^implicates: //' || true)
			printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "$table" "$policy" "$surface" \
				"$(num games)" "$(num clean)" "$(num divergent)" "$(num capacity)" "$(num unmodelled)" "$(num hang)" "$implicated" >>"$out"
			echo "$table $policy $surface: ${line#*: }"
		done
	done
done
