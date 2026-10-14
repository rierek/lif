---
title: "JSON schemas"
description: "Machine-readable output of lif commands: payloads, errors and exit codes."
---

# JSON schemas

## Zweck
Nachschlageseite für alles, was Skripte und Agenten aus der JSON-Ausgabe der Befehle lesen: welche Befehle `--json` haben, wie Erfolgs- und Fehlerpayloads aussehen und welche Exit-Codes gelten. Was FACTS nicht zeigt, bleibt als offen markiert.

## Zielgruppe
Skript-Autoren und Agenten, die Lif maschinell auswerten. Die Seite beschränkt sich auf belegte Formen; fehlende Schemas holt Phase 5 aus dem Code nach.

## Pflichtabschnitte

### Using `--json`
- Zeigt die Befehle mit `--json`: `archive`, `check`, `complete`, `create`, `delete`, `graphql`, `init`, `list`, `milestones`, `next`, `order`, `progress`, `promote`, `rename`, `roadmap`, `scrap`, `show`, `start`, `tag`, `update`, `version`.
- Zeigt die Befehle ohne `--json`: `graph` (nutzt `--format json`), `path`, `pick`, `prime`, `tui`.
- Erklärt: mit `--json` entfallen die menschenlesbaren Zeilen; `lif delete --json` setzt `--force` voraus (keine Rückfrage); `lif graphql --json` schaltet Farben ab.
- Erklärt: `lif list --json` enthält den Body nur mit `--full`.
- Quelle: FACTS.md §1.0, §1.1 (Flags je Befehl), §6.8, §7.5

### Success payloads
- Erklärt die drei Familien: Item-Objekte (Domänenmodell), Antworten mit betroffenem Item, eigene Formen je Befehl (z. B. `check`, `graph`, `version`).
- Tabelle je Befehl: Wurzelform (Objekt oder Liste), Felder, Beispiel (TODO: verify)
- Erklärt das einfache Erfolgsobjekt mit Erfolgsflag und Meldung (z. B. `lif next` ohne bereites Item); die JSON-Schlüsselnamen sind aus FACTS nicht ablesbar (TODO: verify)
- Zeigt `lif version --json` (TODO: verify)
- Verweis auf `reference/item-file-format` (Front matter fields) für die Bedeutung der Item-Felder.
- Quelle: FACTS.md §5.2 (`next --json`), §1.1 (`lif version`); `TODO: verify` für Item- und Versions-Payload: `internal/output/output.go` und die `--json`-Pfade der Befehle ansehen, dann je Befehl einmal ausführen

### Error payload and codes
- Tabelle der Fehlercodes: `NOT_FOUND`, `NO_LIF_DIR`, `INVALID_STATUS`, `FILE_ERROR`, `VALIDATION_ERROR`, `CONFLICT`, `POLICY_VIOLATION` mit je einer Zeile, wann er auftritt (Zuordnung je Code → TODO: verify).
- Zeigt den fehlenden Store: mit `--json` Fehlercode `NO_LIF_DIR` (nicht `NO_BEANS_DIR`).
- Zeigt Beispiele für Fehler mit Code: unbekanntes Item (`NOT_FOUND`), `lif graph --format` ungültig oder `--depth` ohne ID (Validierungsfehler im JSON-Format).
- Zeigt die Form des Fehlerobjekts (Feldnamen) (TODO: verify)
- Quelle: FACTS.md §1.0, §5.5; `TODO: verify` für Fehlerobjekt und Zuordnung der Codes: `internal/output/output.go` ansehen

### Without `--json`
- Erklärt: ohne `--json` gibt Lif Text aus; Fehler erscheinen einmal als Text, der Prozess endet mit Exit-Code 1, die Cobra-Usage ist unterdrückt.
- Erklärt: `lif show` gibt auf einem Terminal gestylte Ausgabe, bei Pipe oder Datei den rohen Markdown der Quelldatei.
- Zeigt Exit-Codes: 0 bei Erfolg, 1 bei Fehler; `lif check` liefert 1 bei mindestens einem Befund (Policy-Warnungen zählen nur mit `--strict`, Legacy-Hinweise nie); kein Exit-Code 2; `lif next` ohne bereites Item endet mit 0.
- Quelle: FACTS.md §1.0, §1.1 (`lif show`), §5.2, §6.8, §6.11

### `lif check --json`
- Tabelle der Felder: `success` (immer; `true` genau dann, wenn keine Befunde, mit `--strict` inklusive Policy-Warnungen), `config_errors` (Liste, `null` wenn leer), `item_issues` mit `broken_links`, `self_links`, `cycles` (`link_type`, `path`), `fixed` (Anzahl; fehlt bei 0), `policy_warnings`, `attachment_orphans`, `legacy_names` (nur wenn belegt).
- Erklärt: Ausgabe ist eingerückt mit zwei Leerzeichen; Exit-Code 1 bei Befunden auch mit `--json`.
- Erklärt: alte Schlüssel `bean_id` und `bean_issues` heißen jetzt `item_id` und `item_issues`.
- Zeigt ein Beispiel mit einem kaputten Link (TODO: verify)
- Feldnamen der Einträge in `broken_links` und `self_links` sowie die Frage, ob `cycles`-Einträge auch `item_id` tragen (FACTS nennt beides) (TODO: verify)
- Quelle: FACTS.md §6.8, §5.3, §6.11; `TODO: verify` für Eintragsfelder und Beispiel: `internal/commands/check.go` und `pkg/itemcore/links.go` ansehen, `lif check --json` einmal ausführen

### `lif graph --json`
- Erklärt: Aufruf mit `--format json`, nicht `--json`.
- Zeigt die Form: `{"nodes":[{"id","title","status","type"}],"edges":[{"from","to","relation"}]}`; `type` fehlt bei leerem Typ; `edges` ist `[]` ohne Kanten; Einrückung zwei Leerzeichen.
- Erklärt `relation`: `parent` (Parent zu Kind) oder `blocks` (blockierendes zu blockiertem Item); `blocking` und `blocked_by` werden zu einer Kante zusammengefasst.
- Erklärt: Fehler (ungültiges `--format`, `--relation`, `--depth`) erscheinen im JSON-Format als JSON-Fehlerobjekt.
- Zeigt ein Beispiel (TODO: verify)
- Quelle: FACTS.md §5.5; `TODO: verify` für Beispielausgabe: in Phase 5 einmal ausführen

### `lif next --json` message
- Erklärt: ist kein Item bereit, gibt `lif next --json` ein Erfolgsobjekt mit Meldung `no ready items found` aus, bei aktivem Filter `no ready items found matching <flags>`; Exit-Code 0.
- Erklärt: ist ein Item bereit, entspricht die Ausgabe der von `lif show --json` (`next` delegiert an `show`).
- Erklärt: ein Abfragefehler erscheint als Validierungsfehler `querying items: ...`.
- Quelle: FACTS.md §5.2

### Findings schema for `lif promote`
- Erklärt: `lif promote <artifact> <finding-id>...` prüft die gewählten Finding-Datensätze gegen ein eingebettetes Findings-Schema und legt je Datensatz ein Item an (feste Zuordnung von Code zu Typ).
- Erklärt: ist ein gewählter Datensatz fehlerhaft, schreibt der Befehl nichts; bereits übernommene Findings werden übersprungen.
- Zeigt das Schema mit allen Feldern (TODO: verify)
- Zeigt die Zuordnung Code zu Typ (TODO: verify)
- Quelle: FACTS.md §1.1 (`lif promote`); `TODO: verify` für Schema und Zuordnung: `internal/commands/findings-schema.json` und `internal/commands/promote.go` ansehen

### Schema files and recipes
- Prüft, ob JSON-Schema-Dateien veröffentlicht werden sollen (TODO: verify)
- Rezept mit `jq`: IDs offener Items aus `lif list --json` ziehen (TODO: verify)
- Rezept: Exit-Code und `success` von `lif check --json` in CI auswerten (TODO: verify)
- Quelle: `TODO: verify` — Rezepte in Phase 5 bauen und ausführen; FACTS nennt keine veröffentlichten Schema-Dateien und keine Rezepte
