---
title: "Reference"
description: "Look-up pages for commands, formats, configuration and interfaces."
---

# Reference

## Zweck
Einstiegsseite des Reference-Kapitels. Sie zeigt, welche Nachschlageseiten es gibt, wo welche Regel kanonisch steht und welche Konventionen für alle Seiten gelten.

## Zielgruppe
Menschen und Agenten, die eine exakte Antwort suchen (Flag, Feld, Konfigurationsschlüssel, Fehlercode) und wissen wollen, auf welcher Seite sie steht. Wer erklärende Texte sucht, wird zu Features und Use cases geschickt.

## Pflichtabschnitte

### How the Reference is organised
- Erklärt in zwei Sätzen: Reference = nachschlagen, nicht erzählen; jede Regel hat genau eine kanonische Seite.
- Zeigt die Gliederung als Liste: Commands, Data and formats, Configuration, Interfaces, Glossary.
- Nennt die Seitenschnitt-Entscheidung: kein eigener Data-model-, Hierarchy-, Presets- oder Dependencies-Reference; Datenmodell in `reference/item-file-format`, Hierarchie und Presets in `reference/configuration`.
- Nennt, wo Abhängigkeiten kanonisch beschrieben sind: `features/dependencies` plus die generierten Seiten zu `lif next`, `lif list`, `lif graph`.
- Quelle: DESIGN.md D26, D45 und Open (Page-Cut Phase 4)

### Commands
- Verweist auf `reference/commands/index` (Command overview) als handgeschriebene Einleitung.
- Erklärt: jeder sichtbare Befehl hat eine eigene, generierte Seite; die vier `completion`-Shells teilen sich eine Seite; versteckte Befehle sind nicht dokumentiert.
- Erklärt: Seiten sind alphabetisch geordnet, ohne Kategorien; Seiten werden aus dem Cobra-Baum generiert und nicht von Hand bearbeitet.
- Nennt die Anzahl der Befehle der `lif`-Binary (28 laut `lif --help`; Zahl in Phase 5 gegen die Generator-Ausgabe prüfen).
- Quelle: DESIGN.md D24, D25; FACTS.md §1.1 (Command count)

### Data and formats
- Verweist auf `reference/item-file-format` (Item file format): Aufbau der Markdown-Datei, Felder, Dateinamen, Handbearbeitung.
- Verweist auf `reference/json-schemas` (JSON schemas): Ausgabe der Befehle mit `--json`, Fehlerpayload, Exit-Codes.
- Verweist auf `reference/search-syntax` (Search syntax): Query-Syntax für `lif list --search` und GraphQL `ItemFilter.search`.
- Ein Satz je Seite, wann man sie braucht (Datei von Hand ändern, Ausgabe maschinell lesen, Volltextsuche).
- Quelle: DESIGN.md D26; FACTS.md §1.1 (`lif list`), §2.1, §6.8

### Configuration
- Verweist auf `reference/configuration` (Configuration): `.lif.yml`, Auffindung, Schlüssel und Defaults.
- Sagt ausdrücklich: dies ist auch die kanonische Seite für Typen, Ranks, Hierarchieregeln und die ausgelieferten Presets (`classic`, `todo`, `simple`, `complex`).
- Zeigt, welche Frage dort beantwortet wird (Welche Typen gibt es? Was prüft `lif check`? Was erzwingt Lif beim Schreiben?).
- Quelle: DESIGN.md D39, D40 und Open (Page-Cut Phase 4); FACTS.md §4.1, §9(b)

### Interfaces
- Verweist auf `reference/graphql` (GraphQL): Schema, Abfragen, Mutationen; Aufruf per `lif graphql` oder per HTTP.
- Verweist auf `reference/web-server` (Web server): separate Binary `lif-serve`; niedrige Priorität.
- Verweist auf den TUI-Unterbereich `reference/tui/*`: fünf Seiten, Start mit `lif tui`.
- Hinweis: Der Web-Server ist eine eigene Binary `lif-serve`; `lif serve` in `lif` ist ein versteckter Platzhalter und wird nicht dokumentiert.
- Quelle: DESIGN.md D27, D28, D29, D44; FACTS.md §1.1 (`lif serve` stub, `lif-serve`, `lif tui`)

### Glossary
- Verweist auf `reference/glossary` (Glossary): kurze Definitionen, je Begriff die Seite mit den Details.
- Erklärt, dass die Begriffe dem Code folgen (item, type, rank, status, priority, tag, parent, blocking / blocked_by, preset) und dass "level" nur Prosa-Synonym für rank ist.
- Erwähnt den Namen Liff als Bezeichnung der Markdown-Datei eines Items; Einführung steht auf `reference/item-file-format` und `reference/glossary`.
- Quelle: DESIGN.md D45, D47

### Conventions
- Item-IDs: vollständige ID oder Kurz-ID ohne konfiguriertes Präfix (Hinweis auf Beispiel in Phase 5).
- `--json`: viele Befehle haben das Flag; Details auf `reference/json-schemas`; `lif graph` nutzt stattdessen `--format json`.
- Exit-Codes: 0 bei Erfolg, 1 bei Fehler; `lif check` liefert 1 bei mindestens einem Befund; es gibt keinen Exit-Code 2.
- Globale Flags `--config` und `--lif-path`; Verweis auf `reference/commands/index` und `reference/configuration` (Store resolution).
- Beispiele im Reference-Kapitel: Befehle und Feldnamen stehen in Code-Formatierung und bleiben englisch.
- Quelle: FACTS.md §1.0, §1.1 (`lif graph`), §6.11

### Where narrative lives
- Erklärt die Arbeitsteilung: Features erklären Nutzen und Ablauf (Muster What, Why, How it looks, Limits, Reference links); Use cases zeigen Szenarien; Overview erklärt Begriffe in einfachen Worten.
- Verweist auf `overview/concepts` (Concepts), `features/index` (Features) und `use-cases/index` (Use cases).
- Sagt: Overview und Features verlinken hierher, sie wiederholen die Regeln nicht.
- Quelle: DESIGN.md D30, D32, D33 und Open (Page-Cut Phase 4)
