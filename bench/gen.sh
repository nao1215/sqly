#!/bin/sh
# Writes the inputs of the himorime suites in bench/. Deterministic: the same
# arguments always write the same bytes, so a base and a head revision read
# the same input.
#
#   sh gen.sh csv N FILE       N rows of id,amount,code,name
#   sh gen.sh tables N DIR     N CSV files table0.csv ... with four columns
#   sh gen.sh tree N DIR       N directories of N one-line CSV files
#   sh gen.sh statement N FILE one SELECT of N columns with multi-byte names
set -eu

csv() {
	mkdir -p "$(dirname "$2")"
	awk -v n="$1" 'BEGIN {
		print "id,amount,code,name"
		for (i = 0; i < n; i++) printf "%d,%.2f,%08d,name-%d\n", i, i + 0.5, i, i
	}' > "$2"
}

case "$1" in
csv)
	csv "$2" "$3"
	;;
tables)
	mkdir -p "$3"
	i=0
	while [ "$i" -lt "$2" ]; do
		printf 'col_a,col_b,col_c,col_d\n1,2,3,4\n' > "$3/table$i.csv"
		i=$((i + 1))
	done
	;;
tree)
	d=0
	while [ "$d" -lt "$2" ]; do
		mkdir -p "$3/dir$d"
		f=0
		while [ "$f" -lt "$2" ]; do
			printf 'a\n' > "$3/dir$d/file$f.csv"
			f=$((f + 1))
		done
		d=$((d + 1))
	done
	;;
statement)
	awk -v n="$2" 'BEGIN {
		printf "SELECT "
		for (i = 0; i < n; i++) printf "%st.列%d AS '"'"'名前%d'"'"'", (i ? ", " : ""), i, i
		printf " FROM 売上 t WHERE t.id = 1 -- コメント"
	}' > "$3"
	;;
*)
	echo "gen.sh: unknown kind $1" >&2
	exit 2
	;;
esac
