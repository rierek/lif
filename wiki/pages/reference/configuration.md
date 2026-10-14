---
title: "Configuration"
description: "The .lif.yml file: discovery, keys, defaults, types, statuses, priorities and presets."
---

# Configuration

## Zweck
Kanonische Seite für die Konfigurationsdatei `.lif.yml`, für Typen, Ranks und Hierarchieregeln sowie für die ausgelieferten Presets. Sie sagt genau, was Lif beim Schreiben erzwingt und was `lif check` nicht prüft.

## Zielgruppe
Nutzer, die ein Projekt einrichten oder anpassen (eigene Typen, Status, Prioritäten, Store-Pfad), und Agenten, die wissen müssen, welche Regeln verlässlich gelten. Overview und Features verlinken hierher, statt die Regeln zu wiederholen.

## Pflichtabschnitte

### Config file and discovery
- Dateiname `.lif.yml`; ein altes `.beans.yml` wird an Ort und Stelle gelesen und nie umbenannt.
- Pro Verzeichnis darf nur eine der beiden Dateien existieren; sonst Fehler `both .lif.yml and .beans.yml exist in <dir>; keep one of them`.
- Auffindung: `--config <path>` lädt genau diese Datei; sonst Suche Verzeichnis für Verzeichnis aufwärts ab dem aktuellen Verzeichnis bis zur Dateisystemwurzel.
- Keine Datei gefunden: eingebaute Defaults, verankert im aktuellen Verzeichnis.
- Format: YAML; Item-Einstellungen unter dem Schlüssel `lif:` (alt `beans:`; beide in einer Datei sind ein Fehler); übrige Abschnitte stehen auf oberster Ebene.
- `lif init` schreibt eine Datei mit kommentierten Schlüsseln (`project.name`, `lif.path`, `lif.prefix`, `lif.id_length`, `lif.default_status`, `lif.default_type`, `worktree.base_ref`/`setup`/`run`/`integrate`, `agent.enabled`/`default_mode`, `server.port`).
- Quelle: FACTS.md §4.1, §1.1 (`lif init`)

### Store resolution
- Rangfolge: (1) `--lif-path` (2) Config-Datei (ihr `lif.path` oder das Standard-Store-Verzeichnis neben ihr) (3) `LIF_PATH` (4) Standard-Store-Verzeichnis im aktuellen Verzeichnis.
- Eine gefundene Config-Datei, und nur eine gefundene, stuft `LIF_PATH` herab; Grund im Code: per direnv exportierte Variablen sollen keine fremden Repositories umlenken.
- `LIF_PATH` und altes `BEANS_PATH` zugleich gesetzt: Fehler; ist nur das alte gesetzt, wird es benutzt.
- Relativer `lif.path` wird auf das Config-Verzeichnis bezogen, absoluter unverändert genutzt.
- `lif.anchor: repo-root`: alle Git-Worktrees eines Repositories teilen einen Store; ohne `anchor` nutzt ein Worktree seinen eigenen lokalen Store; jeder andere Wert ist ein Config-Ladefehler.
- Fehlermeldungen bei fehlendem Verzeichnis: `no .lif directory found at <path> (run 'lif init' to create one)`; mit explizitem Pfad `store path does not exist or is not a directory: <path>`. `lif path` gibt das Ergebnis aus.
- Quelle: FACTS.md §4.2, §1.1 (`lif path`)

### Keys and defaults
- Je Gruppe eine Tabelle: Schlüssel, Typ, Default, Wirkung; `lif check` meldet ungültige Werte bei `agent.default_effort` und `display.theme`. `project`: `name` (Default leer; `lif init` schreibt den Verzeichnisnamen).
- `lif`: `path` (leer = `.lif`), `anchor` (leer; nur `repo-root`), `prefix` (leer; `lif init` schreibt `<verzeichnisname>-`), `id_length` (4), `default_status` (`todo`), `default_type` (`task`; ohne `task` in der Typtabelle der erste Typ), `require_if_match` (false), `require_fields_on` (keine), `commit_field` (`commit`).
- `worktree`: `base_ref` (`main`), `path` (`~/.lif/worktrees/<project>/`), `setup`, `run`, `integrate` (`local` oder `pr`; Default `local`), `fetch_timeout` (10 Sekunden, 0 schaltet den Fetch ab).
- `agent`: `enabled` (true), `default_mode` (`act` oder `plan`; Default `act`), `default_effort` (`low`, `medium`, `high`, `max`; Default leer = keine Überschreibung).
- `server`: `port` (8080), `cors_origins` (Default `http://localhost:*`, `http://127.0.0.1:*`; `*` erlaubt alle).
- `display`: `theme` (`latte`, `frappe`, `macchiato`, `mocha`; Default `mocha`), `max_width` (0 = 110; -1 hebt die Begrenzung auf; `--max-width` überschreibt je Befehl).
- Quelle: FACTS.md §4.3

### Statuses, types and priorities
- Eingebaute Tabellen: Status (`in-progress`, `todo`, `draft`, `completed`, `scrapped`), Typen (`milestone`, `epic`, `feature`, `bug`, `task`), Prioritäten (`critical`, `high`, `normal`, `low`, `deferred`); Details auf `reference/item-file-format` (Allowed values and defaults).
- Eigene Einträge unter `statuses:`, `types:`, `priorities:` überschreiben eingebaute Einträge nach Name oder werden angehängt.
- Felder: `statuses`: `name`, `color`, `archive`, `short`, `description`; `types`: `name`, `color`, `rank`, `emphasis`, `roadmap`, `short`, `description`; `priorities`: `name`, `color`, `symbol`, `description`. `archive` weggelassen behält den Default des eingebauten Eintrags; `lif archive` verschiebt Items mit Archiv-Status nach `archive/`.
- `types_exclusive: true`: `types` ist die vollständige Tabelle, eingebaute Typen werden nicht ergänzt; `lif init --profile` setzt es.
- Achtung, Blockier-Logik: "aufgelöst" ist fest `completed` oder `scrapped`; das Flag `archive: true` zählt dort nicht, ein eigener Archiv-Status blockiert weiter. Bereit-Logik (`ready`) dagegen schließt jeden Status mit `archive: true` aus.
- Akzeptierte Farbwerte (TODO: verify)
- Quelle: FACTS.md §2.3, §4.3, §5.1 ("Resolved" status), §5.2; `TODO: verify` für das Farbformat: `pkg/config/config.go` und die Farbprüfung in `internal/commands/check.go` ansehen

### Types, ranks and hierarchy rules
- Rank-Skala: jeder Typ hat ein ganzzahliges `rank`; ein Parent ist gültig, wenn der Rank des Kindes strikt größer ist als der des Parents; Typen gleichen Ranks verschachteln sich nicht ("level" ist in Prosa nur ein Synonym für rank). Ranks 1 bis 3 sind Container-Ranks, Rank 4 ist der Leaf-Rank; ein Typ ohne `rank` bekommt 4, ein unbekannter Typ zählt als 4; Lücken sind erlaubt.
- Tiefe: Mit den eingebauten Typen und dem Preset `classic` sind es vier Ebenen (`milestone` > `epic` > `feature` > `bug`/`task`); eigene Typen können tiefer gehen, es gibt keine feste Obergrenze (Rank 5 bis 7 wurden akzeptiert). Die "vier Ebenen" sind Eigenschaft der eingebauten Skala, kein erzwungenes Maximum.
- Wo erzwungen: `Core.ValidateParent` prüft beim Anlegen mit `--parent`, bei `lif update --parent` und bei den GraphQL-Mutationen `createItem`, `updateItem`, `setParent`; alle CLI-Schreibbefehle, die GraphQL-Mutationen und der TUI-Parent-Picker laufen durch denselben Resolver. Fehler: `parent item not found: <id>`, `<type> items cannot have a parent`, `<type> items can only have <a, b, or c> as parent, not <type>`. Zusätzlich Zyklenprüfung für `parent` (`would create cycle: [...]`).
- Lücke beim Erzwingen: Wer nur den `type` ändert (`lif update -t`, `updateItem(type)`), löst die Rank-Prüfung nicht erneut aus; ein Item kann so unter einem Parent mit unpassendem Rank stehen bleiben und `lif check` bleibt grün. Als Fehler geführt (D43).
- Was nur validiert wird: `lif check` prüft Konfiguration (Default-Typ, Farben, Theme, Effort, Präfix-Konsistenz), den Typ jedes Items gegen die Typtabelle, kaputte Links, Selbstreferenzen, Zyklen in `parent`, `blocking` und `blocked_by`, verwaiste `attachments/<id>`-Verzeichnisse sowie Pflichtfelder und Commit-SHAs als Warnungen.
- Was `lif check` NICHT prüft (kanonische Aussage zu D40): Rank-Regel von Parent und Kind, Status- und Prioritätswerte von Items, Tag-Format, doppelte IDs, Abweichung zwischen Dateiname und ID, Front-Matter-Syntax über das Laden hinaus; Handbearbeitung (`parent:`, `type:`, `status:`, `blocking:`) wird beim Laden angenommen. Erweiterung von `lif check` ist geplant; die Seite folgt, sobald sie ausgeliefert ist.
- Quelle: FACTS.md §3, §3.1, §3.2, §5.3, §6.9, §9(a), §9(c), §9(d), §9(e); DESIGN.md D39, D40, D43, D45

### Presets
- Erklärt: ein Preset ist eine Typtabelle, die `lif init --profile <name>` einmal in `.lif.yml` ausschreibt; die Datei verweist danach nicht mehr auf den Preset-Namen. `lif init --profile` setzt `types_exclusive: true` und den Default-Typ `task`.
- Tabelle `classic`: `milestone` (1), `epic` (2), `feature` (3), `bug` (4), `task` (4); vier Ebenen. Tabelle `todo`: nur `task` (4); eine Ebene.
- Tabelle `simple`: `milestone` (1), `bucket` (1, `roadmap: false`), `epic` (2), `feature` (2), `bug` (4), `task` (4); drei Ebenen.
- Tabelle `complex`: `release` (1), `bucket` (1, `roadmap: false`), `feature` (2), `improvement` (2), `chore` (2), `story` (4), `bug` (4), `task` (4); drei Ebenen.
- Erklärt `roadmap: false` am Typ: blendet den Typ in `lif roadmap` und `lif milestones` aus; auf einem Container-Rank verschwindet der ganze Teilbaum. Mermaid-Diagramm je Preset (Hierarchie; D38); unbekanntes Profil gibt `unknown profile "<x>" (must be classic, complex, simple, todo)`.
- Offen: ob ein Preset auch Status oder Prioritäten ändert (TODO: verify)
- Quelle: FACTS.md §9(b), §3, §1.1 (`lif init`); DESIGN.md D38, D39; `TODO: verify` für Status und Prioritäten der Presets: `pkg/config/profiles.go` ansehen

### Status-dependent required fields
- `lif.require_fields_on`: Map Status zu Liste von Extra-Schlüsseln; unbekannter Status, leerer Feldname oder reservierter Feldname sind ein Config-Ladefehler.
- Schreiben eines Items in einen solchen Status ohne nicht-leeren Wert für jeden Schlüssel scheitert bei `create` und `update` mit einer Policy-Verletzung (Fehlercode `POLICY_VIOLATION`).
- `lif check` meldet es als Warnung (Abschnitt "Policy"); mit `--strict` zählt die Warnung als Befund (Exit-Code 1).
- "Fehlt" heißt: Schlüssel fehlt, ist null oder nur Leerraum.
- `lif.commit_field` (Default `commit`): Extra-Schlüssel mit einer Git-SHA; `lif complete --commit <ref>` schreibt sie, `lif check` prüft, ob das Repository sie kennt (Warnung sonst; ohne Git-Repository wird die Prüfung übersprungen).
- Quelle: FACTS.md §2.3, §4.3, §6.5, §1.0

### Annotated example and recipes
- Zeigt eine kommentierte `.lif.yml`, wie `lif init` sie erzeugt (TODO: verify)
- Rezept: einen Typ umbenennen oder einen eigenen Typ mit Rank anlegen (TODO: verify)
- Rezept: einen eigenen Status anlegen, inklusive `archive` (TODO: verify)
- Rezept: eigene Farben setzen (TODO: verify)
- Quelle: `TODO: verify` — Beispiele in Phase 5 mit `lif init` bauen und mit `lif check` prüfen; FACTS enthält keine Beispiel-Konfiguration
