---
title: "Item file format"
description: "Structure of an item file: Markdown with YAML front matter."
---

# Item file format

## Zweck
Kanonische Beschreibung des Datenmodells: wie ein Item als Markdown-Datei gespeichert ist, welche Felder und Werte es gibt und was beim Bearbeiten von Hand gilt. Hier wird der Begriff Liff eingeführt.

## Zielgruppe
Nutzer und Agenten, die Items-Dateien lesen, von Hand ändern oder mit eigenen Skripten verarbeiten. Wer Hierarchie- und Statusregeln sucht, wird zu `reference/configuration` verwiesen.

## Pflichtabschnitte

### What an item file is
- Führt **Liff** (Mehrzahl **Liffs**) ein: Kurzform für Lif-Datei, die Markdown-Datei, die ein Item speichert; Feldnamen, Befehle, JSON und GraphQL behalten das Wort `item`.
- Erklärt: ein Item = eine Markdown-Datei; Aufbau `---`, YAML-Front-Matter, `---`, Markdown-Body.
- Erklärt: die erste Zeile im Front Matter ist ein YAML-Kommentar `# <id>`, den Lif schreibt, wenn die ID bekannt ist.
- Erklärt: ID und Slug stehen nicht im Front Matter, sie werden aus dem Dateinamen abgeleitet.
- Quelle: DESIGN.md D47; FACTS.md §2.1

### File name and ID
- Dateiname `<id>--<slug>.md` (doppelter Bindestrich); ohne Slug `<id>.md`; enthält die ID `-` oder `.` und es gibt keinen Slug, heißt die Datei `<id>--.md`.
- ID = konfiguriertes `prefix` plus Zufallssuffix aus `id_length` Zeichen (Standard 4) aus dem Alphabet `0-9a-z`; IDs mit gesperrten Wörtern werden neu erzeugt.
- Slug: Titel kleingeschrieben, Leerzeichen und Unterstriche zu `-`, nur Buchstaben, Ziffern und `-`, zusammengezogen, gekürzt auf 50 Bytes.
- Eigenes Präfix beim Anlegen (`lif create --prefix`): beginnt mit Buchstaben, enthält nur Buchstaben, Ziffern und einzelne Bindestriche.
- Beim Lesen akzeptiert: `id--slug.md`, `id.slug.md`, alt `id-slug.md` und `id.md`.
- Verweis: Umbenennen von Slug, ID oder Präfix mit `lif rename` (generierte Befehlsseite).
- Quelle: FACTS.md §2.4, §1.1 (`lif rename`)

### Front matter fields
- Tabelle: Feld, Typ, Default, Prüfung. Reihenfolge, in der Lif schreibt: `title`, `status`, `type`, `priority`, `tags`, `created_at`, `updated_at`, `order`, `parent`, `blocking`, `blocked_by`.
- `title`: Pflicht, nicht leer, höchstens 1000 Zeichen beim Schreiben; `lif create` ohne Titel nutzt "Untitled".
- `status`, `type`, `priority`: Strings aus den konfigurierten Tabellen; leerer `type` wird beim Laden zu `task`, leere `priority` zu `normal`; `priority` wird beim Anlegen nicht vorbelegt.
- `tags`: Liste; jeder Tag klein, getrimmt, Muster `^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`.
- `created_at`, `updated_at`: RFC 3339, UTC, Sekunden; fehlen sie beim Laden, springt Lif auf das jeweils andere Feld, sonst auf die Änderungszeit der Datei.
- `order`: String, Fractional Index in base62, letzte Stelle nie `0`; geschrieben von `lif order`, `lif create --order` und GraphQL `updateItem(order)`. `parent`: höchstens eine ID; `blocking` und `blocked_by`: Listen von IDs.
- Quelle: FACTS.md §2.1, §2.2

### Allowed values and defaults
- Tabelle Status (Sortierreihenfolge): `in-progress`, `todo`, `draft`, `completed`, `scrapped`; Archiv-Status sind `completed` und `scrapped`.
- Tabelle Typen: `milestone` (Rank 1), `epic` (2), `feature` (3), `bug` (4), `task` (4).
- Tabelle Prioritäten, höchste Dringlichkeit zuerst: `critical`, `high`, `normal`, `low`, `deferred`.
- Defaults: Status `todo`, Typ `task`, keine Priorität geschrieben.
- Verweis auf `reference/configuration` für eigene Status, Typen und Prioritäten und für die Hierarchieregeln.
- Hinweis: Werte werden beim Schreiben über Lif (CLI, GraphQL) gegen die Tabellen geprüft; `update` prüft nur Felder, die es setzt.
- Quelle: FACTS.md §2.3

### Relationships
- Erklärt `parent` (Ein-Eltern-Beziehung, Rank-Regel beim Schreiben, Details auf `reference/configuration`).
- Erklärt `blocking` und `blocked_by`: zwei getrennte Listen; ein Link wird nur in der Datei des geänderten Items gespeichert, die Gegenseite wird nicht geschrieben.
- Erklärt: beim Lesen werden beide Richtungen vereinigt; `lif check` meldet keine Asymmetrie zwischen `blocking` und `blocked_by`.
- Erklärt: `lif delete` räumt eingehende Verweise (parent, blocking, blocked_by) in anderen Dateien auf.
- Verweis auf `features/dependencies` für die Semantik (blockiert, bereit) und auf die generierten Seiten zu `lif next`, `lif list`, `lif graph`.
- Quelle: FACTS.md §2.2, §5.1, §5.4, §5.6

### Extra keys
- Erklärt: jeder weitere Front-Matter-Schlüssel bleibt erhalten und wird nach den typisierten Feldern alphabetisch geschrieben.
- Reservierte Schlüssel: `title`, `status`, `type`, `priority`, `tags`, `created_at`, `updated_at`, `order`, `parent`, `blocking`, `blocked_by`; `--set` und `--unset` lehnen sie ab.
- Beispiel: `summary:` ist ein gewöhnlicher Extra-Schlüssel ohne Sonderbehandlung; `lif list --where key=value` filtert darauf (mehrfach, UND).
- Erklärt Zusammenspiel mit `require_fields_on` und `commit_field` (Verweis auf `reference/configuration`).
- Quelle: FACTS.md §2.1, §2.3, §1.1 (`lif list`)

### Body
- Body = Markdown nach dem Front Matter; ein abschließender Zeilenumbruch gehört nicht zum Inhalt.
- Grenze: höchstens 1 MiB beim Schreiben.
- ETag: Hash der Dateibytes; für optimistisches Sperren mit `--if-match` und GraphQL `ifMatch`; `require_if_match: true` macht es verpflichtend.
- Hinweis: `lif complete --summary` hängt einen Abschnitt "## Summary of Changes" an, `lif scrap --reason` einen Abschnitt "## Reason for Scrapping".
- Quelle: FACTS.md §2.1, §1.1 (`lif complete`, `lif scrap`), §4.3

### Where files live
- Store-Verzeichnis `.lif/` neben `.lif.yml`; ein altes `.beans/` wird genutzt, wenn es existiert; `lif.path` überschreibt.
- Neue Items liegen flach im Store-Wurzelverzeichnis.
- Laden: rekursiv alle `*.md`; Verzeichnisse mit Punkt am Namensanfang und `attachments/` werden übersprungen; `archive/` wird geladen.
- `archive/`: Unterverzeichnis; archivierte Items bleiben geladen und ansprechbar.
- `attachments/<item-id>/`: Belege je Item; `lif init` schreibt `<store>/.gitignore` mit `.conversations/`.
- Eine nicht lesbare `.md`-Datei bricht das Laden ab (`loading <path>: ...`).
- Quelle: FACTS.md §2.4, §5.4

### Editing by hand
- Erklärt: Handbearbeitung ist erlaubt; Lif lädt und lädt neu, ohne Hierarchie, Status oder Priorität zu prüfen.
- Tabelle: Was `lif check` in Hand-Edits findet (unbekannter Typ, kaputte Links, Selbstreferenzen, Zyklen, fehlende Pflichtfelder als Warnung, unbekannte Commit-SHAs als Warnung, verwaiste `attachments/<id>`).
- Tabelle: Was `lif check` nicht findet (Rank-Regel, unbekannte Status- oder Prioritätswerte, Tag-Format, doppelte IDs, Abweichung Dateiname/ID, Front-Matter-Syntax über das Laden hinaus).
- Warnt: doppelte IDs überschreiben sich beim Laden stillschweigend; ein Parse-Fehler bricht das Laden ab.
- Verweis auf `reference/configuration` (Types, ranks and hierarchy rules) als kanonische Aussage zu D40.
- Quelle: FACTS.md §3.2, §6.9, §9(d), §9(e); DESIGN.md D40

### Legacy names
- Tabelle: alt zu neu: `.beans/` zu `.lif/`, `.beans.yml` zu `.lif.yml`, Config-Schlüssel `beans:` zu `lif:`, Dateiname `id-slug.md` wird weiterhin gelesen.
- Erklärt: Lif liest alte Namen an Ort und Stelle und benennt nie selbst um; `lif check` meldet sie unter "Legacy names", sie zählen nie als Befund.
- Erklärt: JSON-Felder `bean_id` und `bean_issues` heißen jetzt `item_id` und `item_issues`.
- Quelle: FACTS.md §2.4, §4.1, §6.6, §6.8

### Annotated example
- Zeigt eine echte Liff-Datei mit Kommentaren zu jedem Feld (TODO: verify)
- Zeigt die JSON-Entsprechung desselben Items aus `lif show --json` (TODO: verify)
- Quelle: `TODO: verify` — Beispiel in Phase 5 mit `lif init` und `lif create` bauen und die Ausgabe von `lif show --json` übernehmen; FACTS enthält keine Beispieldatei
