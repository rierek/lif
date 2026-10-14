---
title: "Contribute"
description: "Repository layout, where to change what and how to build the docs."
---

# Contribute

## Zweck
Hilft neuen Mitwirkenden, sich im Repository zurechtzufinden: Aufbau, Änderungsorte, Docs-Build und Pflegeregeln.

## Zielgruppe
Entwickler und Dokumentationsautoren, die Lif oder die Docs ändern; Agenten, die Änderungen am Repository vornehmen.

## Pflichtabschnitte

### Repository layout
- Zeigt eine Baumskizze: `cmd/lif` (CLI), `cmd/lif-serve` (Webserver), `cmd/lif-docgen` (erzeugt Befehls-Referenzseiten), `internal/commands`, `pkg/item`, `wiki/pages`, `wiki/assets`, `site`, `.lif`-Store.
- Erklärt jede Zeile in einem Halbsatz.
- Hinweis: `lif-serve` ist ein eigenes Binary; `lif serve` im Hauptbinary ist ein versteckter Stub.
- Quelle: FACTS.md §8.1 (Binaries); Verzeichnisbaum `TODO: verify` — Checkout-Baum, `justfile` und `README.md` in Phase 5 prüfen

### Where to change what
- Befehl ändern: Definition unter `internal/commands`; Hilfe, Completion und Reference-Seiten folgen beim Build. (TODO: verify)
- Docs ändern: Seiten unter `wiki/pages`, Medien unter `wiki/assets`.
- TUI-Tastenbelegung: Single Source ist die `keyMap` in `internal/tui/keymap.go`; die Hilfe im TUI und die Seite Key bindings (`reference/tui/key-bindings`) folgen daraus.
- Item-Dateiformat: `pkg/item`.
- Quelle: FACTS.md §8.3 (keyMap), §2.1 (`pkg/item`); Befehl→Reference-Kette `TODO: verify` — `cmd/lif-docgen` und `justfile` prüfen

### Build and preview the docs
- Zeigt `just docs-gen`, `just docs-build`, `just docs-dev` und was jedes tut.
- Erklärt Voraussetzungen (Go, Node) und wohin die Ausgabe geht.
- Quelle: `TODO: verify` — Rezeptnamen und Voraussetzungen im `justfile` und in `site/package.json` prüfen

### Report issues and propose changes
- Erklärt, wo Fehler gemeldet und Änderungen vorgeschlagen werden (Issue-Tracker, Branch, Pull Request).
- Verweist auf Commit- und Branch-Konventionen des Repositories.
- Quelle: `TODO: verify` — `README.md`, `.github/` und Prozessdokumente im Checkout prüfen

### License
- Nennt die Lizenz und verweist auf die Datei `LICENSE`.
- Verweist auf die Attribution zum Upstream-Projekt (`about`).
- Quelle: `TODO: verify` — `LICENSE` im Checkout prüfen (Trial-Draft nennt Apache-2.0)

### Maintenance rules
- Checkliste je Befehlsänderung: Definition, Hilfetext, Completion, generierte Reference, `lif prime`, Tests, Docs.
- Regel: Verhaltensaussagen in Docs stammen aus Code oder `lif --help`, nie aus Annahmen.
- Regel: Dokumentation folgt der Terminologie des Codes (item, type, rank, status, priority, tag, parent, blocking / blocked_by, preset).
- Quelle: DESIGN.md D45, D47; Checkliste und `lif prime`-Verhältnis `TODO: verify` — Prozess mit Maintainern klären
