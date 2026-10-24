#!/usr/bin/env bash
# Deterministischer Wegwerf-Store für die Doku-Medien (beans-wy2o).
#
# Aufruf: seed.sh <root> <lif-binary>
# Löscht <root> und legt es neu an. HOME wird auf <root>/home umgelenkt
# (Suchindex landet sonst im echten ~/.lif); das Binary muss VORHER gebaut
# sein, sonst wird <root>/home zum GOPATH und rm -rf scheitert an Nur-Lese-Dateien.
set -euo pipefail

root="${1:?Wurzel fehlt}"
bin="${2:?lif-Binary fehlt}"

rm -rf "$root"
mkdir -p "$root/home"
export HOME="$root/home"
unset NO_COLOR
cd "$root"

# Neues Repo in <dir> mit Präfix <prefix>; `init` läuft IM Verzeichnis ohne
# --lif-path, nur dann schreibt es eine eigene .lif.yml (siehe demo-stage.sh).
# Kurzer fester ID-Präfix: Screenshots bleiben lesbar und gleich lang.
init_repo() {
	mkdir -p "$1"
	(
		cd "$1"
		"$bin" init >/dev/null
		tmp="$(mktemp)"
		sed "s/^    prefix: .*/    prefix: \"$2\"/" .lif.yml >"$tmp"
		mv "$tmp" .lif.yml
	)
}
init_repo "$root" "lif-"
init_repo "$root/repos/shop-api" "api-"
init_repo "$root/repos/docs-site" "web-"
"$bin" --lif-path "$root/repos/shop-api/.lif" create "Rate limiting" --type task >/dev/null
"$bin" --lif-path "$root/repos/shop-api/.lif" create "Webhook retries" --type bug --priority high >/dev/null
"$bin" --lif-path "$root/repos/docs-site/.lif" create "Search page" --type feature >/dev/null
# Lobby liest die Repos aus der TUI-Konfiguration unter $HOME.
mkdir -p "$HOME/.config/lif/tui"
cat >"$HOME/.config/lif/tui/config.yaml" <<YML
repos:
  - $root
  - $root/repos/shop-api
  - $root/repos/docs-site
YML

cat >>.lif.yml <<YML
statuses:
  - name: to-review
    color: yellow
    short: R
    description: Wartet auf PO-Abnahme
tasks:
  po-accept:
    desc: Abnehmen
    args: [id]
    steps: ['$bin update \$id -s completed']
tui:
  actions:
    - task: po-accept
      key: A
      label: accept
      statuses: [to-review]
YML

lif() { "$bin" "$@"; }
# Feste --order pro Item (Zähler in Datei: mk läuft oft in $(…)-Subshells).
# Ohne sie sortieren gleichzeitig angelegte Geschwister nach Zufalls-ID.
seq_file="$(mktemp)"
echo 0 >"$seq_file"
mk() {
	local t=$1 title=$2 p=$3 n
	shift 3
	n=$(($(cat "$seq_file") + 1))
	echo "$n" >"$seq_file"
	n="$(printf 'a0%03d1' "$n")" # Fraktionalindex: Bruchteil darf nicht auf 0 enden
	if [ "$p" = - ]; then lif create "$title" --type "$t" --order "$n" "$@" | awk '{print $2}'
	else lif create "$title" --type "$t" --parent "$p" --order "$n" "$@" | awk '{print $2}'; fi
}

M1=$(mk milestone "Release 1.0" - --priority high)
E=$(mk epic "Login überarbeiten" "$M1" --priority high)
T1=$(mk task "Fehlertexte der Login-Maske prüfen" "$E" --tag auth)
mk task "Passwort-Reset per E-Mail mit einem sehr langen Titel, der in der Roadmap umbrechen muss und mehrere Zeilen belegt" "$E" --tag auth --tag mail >/dev/null
mk task "Zwei-Faktor-Anmeldung einbauen" "$E" --status in-progress --tag auth >/dev/null
mk task "Session-Timeout als Bug" "$E" --priority critical --tag auth --blocked-by "$T1" --body $'Sessions laufen nach 5 Minuten ab, obwohl Nutzer aktiv sind.\n\n- Timeout nur bei Inaktivität zählen\n- Wartet auf die Fehlertexte der Login-Maske' >/dev/null
E=$(mk epic "Export" "$M1" --priority low)
mk task "Export als CSV" "$E" >/dev/null
mk task "Export als JSON" "$E" --status draft >/dev/null
mk task "Export-Dialog mit Fortschritt" "$E" --tag ui >/dev/null
E=$(mk epic "Dokumentation" "$M1")
mk task "Dokumentation der Login-Seite" "$E" --status to-review --tag docs >/dev/null
mk task "Installationsanleitung" "$E" --tag docs >/dev/null
mk task "Changelog 1.0 schreiben" "$E" --status completed --tag docs >/dev/null
E=$(mk epic "Performance" "$M1" --priority normal)
mk task "Startzeit messen" "$E" --status completed >/dev/null
mk task "Suchindex cachen" "$E" --status in-progress >/dev/null
mk task "Große Listen virtualisieren" "$E" >/dev/null
mk task "Speicherprofil erstellen" "$E" --status draft >/dev/null

M2=$(mk milestone "Release 1.1" -)
E=$(mk epic "Benachrichtigungen" "$M2" --priority high)
mk task "E-Mail bei Zuweisung" "$E" --tag mail >/dev/null
mk task "Push im Browser" "$E" >/dev/null
mk task "Stille Stunden konfigurieren" "$E" --status to-review >/dev/null
E=$(mk epic "Teams und Rechte" "$M2")
mk task "Rollen anlegen" "$E" >/dev/null
mk task "Rechte pro Projekt" "$E" --status in-progress >/dev/null
mk task "Einladung per Link" "$E" >/dev/null
mk task "Audit-Log" "$E" --status draft >/dev/null
E=$(mk epic "Mobile Ansicht" "$M2" --priority low)
mk task "Navigation einklappen" "$E" --tag ui >/dev/null
mk task "Touch-Ziele vergrößern" "$E" --tag ui >/dev/null
mk task "Offline-Hinweis" "$E" >/dev/null

M3=$(mk milestone "Release 2.0" - --priority low)
E=$(mk epic "Plugin-System" "$M3" --priority high)
mk task "Plugin-Schnittstelle entwerfen" "$E" --status in-progress >/dev/null
mk task "Beispiel-Plugin" "$E" >/dev/null
mk task "Plugin-Sandbox" "$E" --priority critical >/dev/null
mk task "Marktplatz-Konzept" "$E" --status draft >/dev/null
E=$(mk epic "Neue Oberfläche" "$M3")
mk task "Design-System" "$E" --tag ui >/dev/null
mk task "Dunkles Thema" "$E" --tag ui >/dev/null
mk task "Tastaturkürzel vereinheitlichen" "$E" --tag ui >/dev/null
E=$(mk epic "Migration" "$M3" --priority normal)
mk task "Altdaten importieren" "$E" >/dev/null
mk task "Rückwärtskompatibilität prüfen" "$E" >/dev/null
mk task "Migrationsleitfaden" "$E" --tag docs >/dev/null

mk feature "Idee ohne Meilenstein" - --tag idee >/dev/null
mk milestone "Leerer Meilenstein" - >/dev/null
rm -f "$seq_file"
