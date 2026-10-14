---
title: "First steps"
description: "Create a store, pick a preset and work your first items."
---

# First steps

## Zweck
Führt durch die erste Stunde: Store anlegen, Preset wählen, erste Items erzeugen, ansehen, bearbeiten und abschließen. Abgrenzung: Alles, was konfigurierbar ist, steht in Reference: Configuration (`reference/configuration`).

## Zielgruppe
Neue Nutzer direkt nach der Installation. Sie wollen ohne Vorwissen einen funktionierenden Arbeitsablauf im Terminal.

## Pflichtabschnitte

### Initialise a store
- Zeigt `lif init` im Projektordner; legt `.lif.yml` und das Store-Verzeichnis `.lif/` an.
- Erklärt, was `init` schreibt: `lif.prefix` = `<Ordnername>-`, `project.name` = Ordnername, `.gitignore` im Store mit `.conversations/`.
- Erklärt, dass `init` ohne vorhandenen Store läuft, alle anderen Befehle brauchen einen.
- Fehlermeldung ohne Store zeigen: `no .lif directory found at <path> (run 'lif init' to create one)`.
- Zeigt `lif path` zur Kontrolle des aufgelösten Store-Verzeichnisses.
- Quelle: FACTS.md §1.1 (`init`, `path`), §1.0 (Bootstrap-Skip), §2.4, §4.2

### Choose a preset
- Tabelle der vier Presets für `lif init --profile`: `classic`, `todo`, `simple`, `complex` mit Typen und Ranks.
- `classic`: milestone, epic, feature, bug, task (bis zu vier Ranks); `todo`: nur `task`.
- `simple` und `complex`: Ranks 1, 2 und 4 (drei Ebenen); `bucket`-Typ ist aus der Roadmap ausgeblendet (`roadmap: false`).
- Hinweis: `init --profile` setzt `types_exclusive` und schreibt die Typen in `.lif.yml`; spätere Änderung per Hand möglich.
- Hinweis: Ohne `--profile` gelten die eingebauten Typen. (TODO: verify: ob `init` ohne `--profile` eine Typtabelle schreibt.)
- Verweis auf Reference: Configuration (`reference/configuration`) für die vollständigen Preset-Definitionen.
- Quelle: FACTS.md §1.1 (`init`), §3, §9(a), §9(b). `init` ohne `--profile` ist `TODO: verify` (prüfen mit `lif init` in leerem Ordner in Phase 5).

### Create your first items
- Zeigt `lif create "Titel" -t epic` und `lif create "Titel" -t task --parent <id>` mit Beispielausgabe (Phase 5 echt laufen lassen).
- Erklärt Standardwerte: Status `todo`, Typ `task`, kein Standard für `priority`; Titel ohne Argument = "Untitled".
- Flags kurz: `--status`, `--type`, `--priority`, `--tag`, `--parent`, `--blocked-by`, `--body`/`--body-file`.
- Erklärt Item-IDs: Präfix plus 4 Zeichen aus `0-9a-z`; Kurz-ID ohne Präfix wird überall akzeptiert.
- Zeigt die Regelverletzung: Task unter Task wird abgelehnt (`task items can only have milestone, epic, or feature as parent, not task`).
- Quelle: FACTS.md §1.0 (Kurz-IDs), §1.1 (`create`), §2.3, §2.4, §3, §3.1

### Look at them
- `lif list` (Tabelle) und `lif list --view tree`; wichtige Filter: `--status`, `--type`, `--tag`, `--ready`, `--search`.
- `lif show <id>`: formatiert im Terminal, roher Markdown-Text bei Pipe oder `--raw`.
- `lif roadmap` und `lif milestones` für Überblick; `lif progress` für Zähler und Prozent.
- `lif next`: höchstpriorisiertes bereites Item (Definition "ready" steht in Concepts, `overview/concepts`).
- `--json` an `list`, `show`, `next` für Skripte; Verweis auf JSON schemas (`reference/json-schemas`).
- Quelle: FACTS.md §1.1 (`list`, `show`, `roadmap`, `milestones`, `progress`, `next`), §5.2

### Work an item
- Lebenszyklus als kleine Befehlsfolge: `lif start <id>` → `lif complete <id> --summary "..."` oder `lif scrap <id> --reason "..."`.
- `complete --commit HEAD` speichert die Commit-SHA; `--summary` hängt den Abschnitt "## Summary of Changes" an; `scrap --reason` ist Pflicht.
- Mehrere IDs in einem Aufruf möglich; alle werden vor dem ersten Schreiben geprüft.
- `lif archive` verschiebt abgeschlossene und verworfene Items nach `archive/`; sie bleiben adressierbar.
- Nützlich: `lif update <id> --priority high`, `lif tag <id> --tag x`.
- Quelle: FACTS.md §1.1 (`start`, `complete`, `scrap`, `archive`, `update`, `tag`), §2.3, §2.4

### Open the Liff in your editor
- Führt das Wort "Liff" ein Mal im Fließtext ein und verweist auf Reference: Item file format (`reference/item-file-format`) als Definition.
- Zeigt `lif path` und eine Item-Datei mit Front-Matter (Dateiname `<id>--<slug>.md`).
- Per Hand bearbeiten ist erlaubt; `lif check` danach ausführen.
- Ehrliche Grenze: Handänderungen an `parent:`, `type:`, `status:` werden beim Laden nicht geprüft; `lif check` meldet Rang-, Status-, Prioritäts- und Tag-Fehler nicht (D40).
- Was `lif check` fängt: unbekannter Typ, kaputte Links, Selbstverweise, Zyklen, verwaiste Anhang-Ordner; Exit-Code 1 bei Fund.
- Quelle: FACTS.md §2.1, §2.4, §3.2, §6.8, §6.9, §9(d), §9(e); DESIGN.md D40, D47

### Next steps
- Concepts (`overview/concepts`) für das mentale Modell.
- Features (`features/index`) und Use cases (`use-cases/index`) für Arbeitsabläufe.
- Reference: Commands (`reference/commands/index`) für alle Befehle, Reference: Configuration (`reference/configuration`) für Einstellungen.
- TUI: Start and repositories (`reference/tui/start-and-repositories`) als Alternative zur CLI.
- Quelle: DESIGN.md D29, D44
