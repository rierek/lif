---
title: "Glossary"
description: "Short definitions of the terms Lif uses."
---

# Glossary

## Zweck
Kurze Definitionen der Begriffe, die Lif in Befehlen, Dateien, JSON und Dokumentation benutzt. Jeder Eintrag nennt die Seite, die das Detail besitzt. Hier wird neben `reference/item-file-format` der Begriff Liff eingeführt.

## Zielgruppe
Neue Nutzer, die einen Begriff nachschlagen, und Agenten, die dieselben Wörter wie der Code benutzen sollen. Wer von früheren Versionen (`beans`) kommt, findet die Zuordnung alter zu neuen Namen.

## Pflichtabschnitte

### How to read the glossary
- Erklärt: Die Begriffe folgen dem Code (item, type, rank, status, priority, tag, parent, blocking / blocked_by, preset, store).
- Erklärt: "level" ist in Fließtext nur ein Synonym für rank, nie ein Feldname.
- Erklärt Liff: Name für die Markdown-Datei, die ein Item speichert; Feldnamen, Befehle, JSON und GraphQL behalten `item`.
- Erklärt das Format der Einträge: Begriff, eine Zeile Definition, Seite mit Details in Backticks; Gruppen: Core terms, Store and files, Planning terms, Interface terms, Legacy terms.
- Quelle: DESIGN.md D45, D47

### Core terms
- item, Liff: Item = Einheit der Arbeit (Aufgabe, Feature, Epic, Meilenstein …); Liff = die Markdown-Datei dazu. Details: `reference/item-file-format`.
- type, rank: Typ benennt die Art des Items; `rank` ist eine ganze Zahl, die bestimmt, welcher Typ unter welchem stehen darf (Kind-Rank strikt größer als Parent-Rank). Details: `reference/configuration` (Types, ranks and hierarchy rules).
- status, priority: Status = Stand der Arbeit (`in-progress`, `todo`, `draft`, `completed`, `scrapped`); Priorität = Dringlichkeit (`critical` bis `deferred`). Details: `reference/item-file-format` (Allowed values and defaults), `reference/configuration`.
- tag, parent: Tag = Etikett in Kleinbuchstaben mit Bindestrichen; `parent` = die ID des einen übergeordneten Items. Details: `reference/item-file-format`.
- blocking, blocked_by: zwei Listen von IDs; `blocking` nennt, was dieses Item blockiert, `blocked_by` nennt, was es blockiert; Speicherung nur auf einer Seite. Details: `features/dependencies`, `reference/item-file-format` (Relationships).
- preset: eine mit `lif init --profile` einmal in `.lif.yml` ausgeschriebene Typtabelle (`classic`, `todo`, `simple`, `complex`). Details: `reference/configuration` (Presets).
- Quelle: FACTS.md §2.1, §2.2, §2.3, §3, §5.1, §9(b); DESIGN.md D45, D47

### Store and files
- store, archive: Store = das Verzeichnis mit den Liffs (Standard `.lif/`); `archive/` = Unterverzeichnis für Items mit Archiv-Status, sie bleiben geladen und ansprechbar. Details: `reference/item-file-format` (Where files live).
- attachments: `attachments/<item-id>/` mit Belegen je Item. Details: `reference/item-file-format` (Where files live).
- ID, prefix, slug: ID = Präfix plus Zufallssuffix; Slug = Titelkurzform im Dateinamen `<id>--<slug>.md`; Kurz-ID = ID ohne Präfix. Details: `reference/item-file-format` (File name and ID).
- anchor: `lif.anchor: repo-root` lässt alle Git-Worktrees eines Repositories einen Store teilen. Details: `reference/configuration` (Store resolution).
- ETag: Hash der Dateibytes für `--if-match` und GraphQL `ifMatch`. Details: `reference/item-file-format` (Body).
- Quelle: FACTS.md §2.1, §2.4, §4.2, §5.4

### Planning terms
- container, leaf: Container = Item auf Rank 1 bis 3, Leaf = Item auf Rank 4 (eingebaute Skala); eigene Typen können andere Ranks haben. Details: `reference/configuration` (Types, ranks and hierarchy rules).
- ready: nicht blockiert (direkt oder über einen Vorfahren), Status nicht `in-progress`, `draft` oder Archiv-Status, kein Vorfahr `completed` oder `scrapped`. Details: `features/dependencies`, `reference/commands/index` (generierte Seiten `lif next`, `lif list`).
- blocked, explizit und implizit: explizit = hat selbst aktive Blocker; implizit = ein Vorfahr hat aktive Blocker; aktiv heißt Status nicht `completed` oder `scrapped`. Details: `features/dependencies`.
- roadmap: Baumansicht aus Meilensteinen, Epics und Kindern; Typen mit `roadmap: false` erscheinen nicht. Details: `lif roadmap` (generierte Seite), `reference/configuration` (Presets).
- Quelle: FACTS.md §3, §5.1, §5.2

### Interface terms
- CLI, `lif prime`: CLI = die Binary `lif`; `lif prime` gibt einen Prompt aus, der einen Coding-Agenten in die Nutzung von Lif einführt. Details: `reference/commands/index`, `overview/agent-onboarding`.
- TUI: Terminal-Oberfläche, gestartet mit `lif tui`. Details: `reference/tui/*`.
- `lif-serve`: eigene Binary für den Web-Server. Details: `reference/web-server`.
- GraphQL: Abfrage- und Änderungsschnittstelle; im Prozess mit `lif graphql` (Alias `lif query`) oder per HTTP über `lif-serve`. Details: `reference/graphql`.
- Quelle: FACTS.md §1.0, §1.1 (`lif prime`, `lif tui`, `lif-serve`, `lif graphql`), §7.5

### Legacy terms
- bean: früherer Name für item; heute Item oder Liff (D47). JSON-Felder `bean_id` und `bean_issues` heißen jetzt `item_id` und `item_issues`. Details: `reference/json-schemas`.
- `.beans/`, `.beans.yml`, `beans:`: alter Store-Ordner, alte Config-Datei, alter Config-Schlüssel; werden an Ort und Stelle gelesen, nie umbenannt; `lif check` meldet sie unter "Legacy names". Details: `reference/configuration`.
- `BEANS_PATH`: alte Umgebungsvariable; wird gelesen, wenn `LIF_PATH` nicht gesetzt ist; beide zugleich gesetzt ist ein Fehler. Details: `reference/configuration` (Store resolution).
- `beans-`-Präfix: alter ID-Präfix; Zuordnung zu `lif rename --prefix` (TODO: verify)
- Quelle: FACTS.md §2.4, §4.1, §4.2, §6.6, §6.8; `TODO: verify` für den `beans-`-Präfix: in FACTS nicht belegt, `lif rename --help` und Migrationshinweise im Repository prüfen
