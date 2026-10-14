---
title: "Command overview"
description: "All lif commands, global flags and shared conventions."
---

# Command overview

## Zweck
Handgeschriebene Einleitung zu den generierten Befehlsseiten: alle Befehle auf einen Blick, gemeinsame Flags und Konventionen, die nicht auf jeder Befehlsseite wiederholt werden.

## Zielgruppe
Alle, die den passenden Befehl suchen oder wissen wollen, was für alle Befehle gleich gilt (IDs, `--json`, Exit-Codes). Agenten, die den Befehlsraum vollständig erfassen müssen.

## Pflichtabschnitte

### The lif command line
- Erklärt die zwei Binaries: `lif` (CLI, TUI, GraphQL im Prozess) und `lif-serve` (Web-Server, einziger Befehl `serve`).
- Nennt die Zahl: `lif --help` listet 28 Befehle; 26 davon sind im Repository definiert, `completion` und `help` kommen von Cobra.
- Nennt die Kurzbeschreibung des Wurzelbefehls ("A file-based issue tracker for AI-first workflows").
- Hinweis: Marker "plumbing" für `init`, `path`, `version`, `help`, `completion` (nicht zum interaktiven Tippen gedacht); alle anderen sind nutzerseitig.
- Quelle: FACTS.md §1.0, §1.1 (Command count)

### Command list
- Alphabetische Liste ohne Kategorien, eine Zeile je Befehl: `archive`, `check`, `complete`, `completion`, `create`, `delete`, `graph`, `graphql`, `help`, `init`, `list`, `milestones`, `next`, `order`, `path`, `pick`, `prime`, `progress`, `promote`, `rename`, `roadmap`, `scrap`, `show`, `start`, `tag`, `tui`, `update`, `version`.
- Je Zeile: Befehl, Aliase, ein Satz Zweck (aus den Purpose-Einträgen in FACTS), Slug der generierten Seite in Backticks.
- Erklärt: die vier `completion`-Shells (bash, zsh, fish, powershell) teilen sich eine Seite; versteckte Befehle (`lif serve` Platzhalter, `__complete`) sind nicht dokumentiert.
- Hinweis für Phase 5: Die Liste wird aus dem Cobra-Baum generiert, nicht von Hand gepflegt; Generator-Anbindung im Seitenbau prüfen.
- Quelle: DESIGN.md D24, D25; CONCEPT.md §7.1 (alphabetisch, keine Kategorien); FACTS.md §1.1 (Command count, Cobra built-ins, `lif serve` stub)

### Global flags
- Tabelle: `--config <path>` (Pfad zur Config-Datei; ohne Angabe Suche aufwärts nach `.lif.yml`; ein altes `.beans.yml` wird gelesen, wenn kein `.lif.yml` existiert).
- Tabelle: `--lif-path <path>` (Pfad zum Store; schlägt Config und `LIF_PATH`; ein altes `BEANS_PATH` wird gelesen, wenn `LIF_PATH` nicht gesetzt ist).
- Tabelle: `-h, --help` (Cobra-Standard, an jedem Befehl).
- Verweis auf `reference/configuration` (Store resolution) für die Rangfolge Flag, Config, `LIF_PATH`, Standardverzeichnis.
- Quelle: FACTS.md §1.0, §4.2

### Shared conventions
- Item-ID: vollständige ID oder Kurz-ID ohne das konfigurierte Präfix.
- `--json`: Flag der meisten Befehle; keines haben `graph` (nutzt `--format json`), `path`, `pick`, `prime`, `tui` sowie die Cobra-Befehle; Details auf `reference/json-schemas`.
- Exit-Codes: 0 bei Erfolg, 1 bei Fehler; Fehler werden einmal ausgegeben, Cobra-Usage ist unterdrückt; `lif check` liefert 1 bei mindestens einem Befund; kein Exit-Code 2.
- Befehle ohne Store: `init`, `prime`, `version`, `tui`; alle anderen laden vor der Ausführung Config und Items, ein fehlender Store ist ein Fehler.
- Aliase: `ls` (`list`), `rm` (`delete`), `new` und `c` (`create`), `query` (`graphql`), `u` (`update`).
- Mehrere IDs: viele Schreibbefehle (`start`, `complete`, `scrap`, `update`, `tag`, `delete`) nehmen mehrere IDs; Prüfungen laufen für alle IDs vor dem ersten Schreiben (belegt für `update`, `complete`).
- Quelle: FACTS.md §1.0, §1.1 (je Befehl: Aliase, Flags, Purpose), §6.11

### How these pages are generated
- Erklärt: Befehlsseiten entstehen durch `cmd/lif-docgen` aus dem Cobra-Baum; sie nie von Hand bearbeiten.
- Erklärt das optionale handgeschriebene Supplement je Befehl (Erklärungen, Beispiele, Links), das der Generator in die Seite mischt.
- Erklärt: Hilfetexte der CLI enthalten keine Web-Links.
- Hinweis: Die Aussage, dass `--help` und Shell-Completion aus derselben Quelle stammen, ist durch FACTS nicht belegt (TODO: verify)
- Quelle: DESIGN.md D24; `TODO: verify` für die Aussage zur gemeinsamen Quelle mit `--help`/Completion: in `cmd/lif-docgen` und `internal/commands/completion.go` prüfen

### Related reference
- Verweis auf `reference/configuration` (Store resolution, Config-Schlüssel).
- Verweis auf `reference/json-schemas` (Payloads, Fehlercodes, Exit-Codes).
- Verweis auf `reference/search-syntax` (für `lif list --search`).
- Verweis auf `reference/graphql` und `reference/web-server` für `lif graphql` und `lif-serve`.
- Quelle: DESIGN.md D26, D27, D28
