#!/usr/bin/env bash
# Screenshots (PNG) und GIFs der TUI per VHS erzeugen (beans-wy2o).
#
# Aufruf: doc-media.sh <root> [png|gif|<tapename>]
# <root> kommt vom Aufrufer (justfile: doc_media_root); der Pfad steht hinter
# einem `rm -rf`, deshalb hier kein eigener Default. Idempotent: jeder Lauf
# baut Binary und Store neu und überschreibt wiki/screenshots/tui-*.png und
# wiki/assets/tui-*.gif. Die Medien sind nicht eingecheckt; der docs-Workflow
# erzeugt sie bei jedem Deploy.
set -euo pipefail

root="${1:?Wurzel des Wegwerf-Verzeichnisses fehlt}"
sel="${2:-}"
repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tapes="$repo/scripts/doc-media"

for c in vhs ttyd ffmpeg; do
	command -v "$c" >/dev/null || { echo "$c fehlt" >&2; exit 1; }
done

work="$(mktemp -d)"
trap 'rm -rf "$root" "$work"' EXIT
rm -rf "$root"
mkdir -p "$root/bin"

# Vor der HOME-Umlenkung bauen, sonst wird $root/home zum GOPATH und das
# Aufräumen scheitert an Nur-Lese-Dateien des Modulcaches.
echo "==> Binary bauen"
go build -o "$root/bin/lif" "$repo/cmd/lif"

echo "==> Store seeden"
bash "$tapes/seed.sh" "$root/acme-app" "$root/bin/lif"

# Tapes nach $work kopieren: Screenshot-/Output-Pfade sind dort relativ.
# Ein Screenshot als letzter Befehl geht oft verloren: vhs beendet sich vor
# dem nächsten Frame (gemessen: 7 von 16 Läufen ohne PNG, mit Nachlauf 20/20).
# Deshalb bekommt jedes Screenshot-Tape einen kurzen Nachlauf.
cp "$tapes"/*.tape "$work"/
for t in "$work"/tui-*.tape; do grep -q '^Output' "$t" || echo 'Sleep 500ms' >>"$t"; done
export LIF_DOC_ROOT="$root/acme-app" LIF_DOC_BIN="$root/bin"

for tape in "$work"/tui-*.tape; do
	name="$(basename "$tape" .tape)"
	case "$sel" in
	"" | "$name") ;;
	png) grep -q '^Output' "$tape" && continue ;;
	gif) grep -q '^Output' "$tape" || continue ;;
	*) continue ;;
	esac
	echo "==> $name"
	# vhs verliert gelegentlich einen Screenshot (beobachtet: ganzer Lauf, ein
	# Tape ohne Ausgabe, kein Fehler). Ergebnis prüfen, bis zu dreimal wiederholen.
	out="$name.png"
	grep -q '^Output' "$tape" && out="$name.gif"
	for _ in 1 2 3; do
		(cd "$work" && env -u NO_COLOR TERM=xterm-256color vhs "$name.tape" >/dev/null)
		[ -s "$work/$out" ] && continue 2
		echo "    $out fehlt, neuer Versuch" >&2
	done
	echo "FEHLER: $out nicht erzeugt" >&2
	exit 1
done

mkdir -p "$repo/wiki/screenshots" "$repo/wiki/assets"
shopt -s nullglob
for f in "$work"/tui-*.png; do mv "$f" "$repo/wiki/screenshots/"; done
for f in "$work"/tui-*.gif; do mv "$f" "$repo/wiki/assets/"; done
# README bettet dieses Bild über die Pages-URL ein (beans-mfkq).
if [ -f "$repo/wiki/screenshots/tui-roadmap-detail.png" ]; then mkdir -p "$repo/site/public/media" && cp "$repo/wiki/screenshots/tui-roadmap-detail.png" "$repo/site/public/media/"; fi
echo "==> fertig"
