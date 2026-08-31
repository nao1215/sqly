#!/bin/sh
# shellcheck shell=sh
#
# Renders one screenshot per sqly theme into doc/img/themes/, which the theme
# gallery on the website shows. Run from the repository root:
#
#	make themes
#
# One vhs invocation per theme, rather than one tape stepping through all of
# them: vhs writes a screenshot from the frame stream it is recording, and over
# a long tape that stream falls behind the sleeps, so a shot lands mid-typing or
# on the line before. A run per theme keeps each one short enough that it does
# not.
#
# sqly colors the text and the terminal supplies the background, so a light
# theme is rendered on a light one. That is what it looks like where it belongs,
# and the gallery says so.
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if ! command -v vhs >/dev/null 2>&1; then
	echo "themes: vhs is not installed. Install it from https://github.com/charmbracelet/vhs" >&2
	exit 127
fi

make build

OUT="doc/img/themes"
mkdir -p "$OUT"

# The statement holds one of everything the highlighter names: a keyword, a
# string literal, a number, a name the user chose, and a comment.
STATEMENT="SELECT actor, 'star' AS role, 42 AS n FROM actor -- a comment"

# The themes to render, and the terminal background each is shown on.
THEMES="night-owl:dark dracula:dark monokai:dark nord:dark solarized:dark
gruvbox:dark tokyo-night:dark catppuccin:dark vscode:dark github-light:light
accessible:dark none:dark"

TAPE="$(mktemp -d)/theme.tape"
trap 'rm -rf "$(dirname "$TAPE")"' EXIT

for entry in $THEMES; do
	name="${entry%%:*}"
	background="${entry##*:}"

	# sqly colors the text; the terminal supplies the background. A light theme
	# is therefore rendered on a light terminal, which is where it belongs and
	# what the gallery page says about it.
	if [ "$background" = light ]; then
		theme='Set Theme "Github"'
	else
		theme=''
	fi

	cat > "$TAPE" <<-TAPEEOF
	Output "$(dirname "$TAPE")/unused.gif"
	Set Shell "bash"
	Set FontSize 16
	Set Width 1000
	Set Height 120
	Set Padding 14
	Set CursorBlink false
	Set TypingSpeed 8ms
	$theme
	Hide
	Type "WORK=\$(mktemp -d); mkdir -p \$WORK/data; cp testdata/actor.csv \$WORK/data/; export PATH=\$PWD:\$PATH HOME=\$WORK SQLY_SETTINGS_PATH=\$WORK/settings.json; cd \$WORK/data; clear"
	Enter
	Type "sqly actor.csv"
	Enter
	Sleep 2s
	Type ".theme $name"
	Enter
	Sleep 800ms
	Type ".clear"
	Enter
	Sleep 800ms
	Show
	Type "$STATEMENT"
	Sleep 2s
	Screenshot "$OUT/$name.png"
	Sleep 500ms
	TAPEEOF

	echo "themes: rendering $name"
	vhs "$TAPE" >/dev/null
done

echo "themes: wrote $(find "$OUT" -name '*.png' | wc -l) screenshots to $OUT"
