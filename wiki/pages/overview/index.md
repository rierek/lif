---
title: "Lif in brief"
description: "What Lif is, who it is for and where to go next."
---

# Lif in brief

## Zweck
Kurze, überzeugende Orientierung: Was ist Lif, für wen, wie funktioniert es, was ist es nicht. Die Seite ersetzt die Seed-Seite "Feature Overview" und dupliziert keine Feature-Details.

## Zielgruppe
Menschen, die Lif bewerten oder weiterempfehlen, und Agenten, die per Websuche herausfinden, was Lif ist. Kein Vorwissen nötig.

## Pflichtabschnitte

### What Lif is
- Ein Satz Definition: dateibasierter Issue-Tracker für KI-first-Workflows, Items als Markdown-Dateien neben dem Code.
- Zweiter Satz: gibt Menschen und Coding-Agenten die gleiche vollständige Sicht auf das Projekt.
- Zeigt die Tagline aus `lif --help` wörtlich als Zitatblock.
- Optional: Terminal-Capture `lif roadmap --tags` als Hero-Medium (Verweis auf Medienplan, kein Bild im Skelett).
- Quelle: FACTS.md §1.0; DESIGN.md D48

### Who it is for
- Menschen: CLI, TUI (`lif tui`), Web-UI (`lif-serve`) oder beliebiger Editor auf denselben Dateien.
- Agenten: `lif prime` und `lif --help` reichen zum Einstieg, keine Website nötig.
- Skripte: `--json`-Ausgabe der Befehle und GraphQL (`lif graphql`).
- Drei Zugriffswege als kleine Tabelle: Markdown-Dateien · CLI plus `lif prime` und `--help` · JSON plus GraphQL.
- Verweis auf Feature-Seite "Surface independence" (`features/surface-independence`) und "Automation and agents" (`features/automation`).
- Quelle: FACTS.md §1.0, §1.1 (`prime`, `graphql`, `tui`), §7.5, §8.1

### How it works
- Store: Verzeichnis `.lif/` neben `.lif.yml`; ein Item = eine Markdown-Datei (Liff) mit YAML-Front-Matter und Body.
- Zeigt kleines Beispiel einer Item-Datei (Front-Matter-Felder `title`, `status`, `type`, `priority`, `tags`, `parent`, `blocking`, `blocked_by`); Beispiel in Phase 5 mit `lif create` erzeugen.
- `.lif.yml`: Konfiguration, wird aufwärts vom aktuellen Verzeichnis gesucht; ohne Datei gelten eingebaute Defaults.
- Kein Server nötig: alle CLI-Befehle laden die Dateien direkt; `lif-serve` ist optional.
- Hinweis auf Legacy-Namen `.beans/` und `.beans.yml` nur als Randnotiz, die weiter gelesen werden.
- Quelle: FACTS.md §2.1, §2.4, §4.1, §1.0 (Store wird vor jedem Befehl geladen)

### What makes it different
- Sechs Features, je eine Zeile plus Slug in Backticks: Markdown-native (`features/markdown-native`), Flexible planning (`features/project-planning`), Dependencies (`features/dependencies`), Surface independence (`features/surface-independence`), Automation and agents (`features/automation`), Git-native (`features/git-native`).
- Pro Zeile ein Nutzenversprechen, keine Details; Details stehen auf den Feature-Seiten.
- Verweis auf Features-Index (`features/index`).
- Quelle: DESIGN.md D30, D32; CONCEPT.md §3.3 (Feature Overview darf Features nicht duplizieren)

### What Lif is not
- Keine Workflow-Engine: Status, Typen und Prioritäten sind konfigurierbare Namen, keine erzwungenen Übergänge. (TODO: verify: "keine Übergangsregeln" gegen Code prüfen; FACTS zeigt nur `require_fields_on` je Status.)
- Presets (`lif init --profile`: `classic`, `todo`, `simple`, `complex`) sind Beispiele, keine feste Vorgabe.
- Nicht "vier feste Ebenen": mitgelieferte Presets nutzen bis zu vier Ranks, eigene Typen können tiefer gehen, es gibt keine harte Obergrenze (D39).
- Kein Ersatz für Hosted-Tracker mit Benutzerverwaltung (TODO: verify: Abgrenzung gegen andere Tracker ist redaktionell, FACTS enthält dazu nichts).
- Quelle: FACTS.md §3, §9(a), §9(b); DESIGN.md D39. Die Bullets "keine Übergangsregeln" und "kein Hosted-Tracker" sind `TODO: verify` (Prüfung: `pkg/config/config.go`, `README.md`).

### Where to go next
- Installation (`overview/installation`) für Neueinsteiger.
- First steps (`overview/first-steps`) für die erste Stunde.
- Concepts (`overview/concepts`) für das mentale Modell.
- Agent onboarding (`overview/agent-onboarding`) für Coding-Agenten.
- Features (`features/index`) und Use cases (`use-cases/index`) zum Vertiefen.
- Quelle: DESIGN.md D35, D37; CONCEPT.md §3
