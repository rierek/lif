---
title: "Writing or research project"
description: "Organise chapters, sources and open questions."
---

# Writing or research project

## Zweck
Nicht-technisches Beispiel: Ein Schreib- oder Forschungsprojekt (Buch, Abschlussarbeit, Studie) mit eigenen Typen strukturieren.

## Zielgruppe
Autorinnen, Forschende und Studierende, die Kapitel, Quellen und offene Fragen in Dateien führen; Agenten, die ein Nicht-Software-Setup aufbauen.

## Pflichtabschnitte

### Problem
- Beschreibt die Situation: Viele Kapitel und Quellen, offene Fragen gehen verloren, unklar, worauf ein Abschnitt wartet.
- Nennt Anforderungen: eigene Begriffe statt Software-Vokabular, Themen-Filter, Fortschritt je Kapitel.
- Zeigt: Lif ist nicht auf Software beschränkt (D33).
- Quelle: DESIGN.md D33

### Solution
- Erklärt: Eigene `types` mit `types_exclusive: true` ersetzen das Software-Vokabular; Status und Priorität bleiben nutzbar.
- Erklärt: Tags bilden Themen; `blocked_by` bildet "wartet auf eine Quelle"; der Item-Body trägt Arbeitsnotizen.
- Erklärt: Status `draft` ("Needs refinement before it can be worked on") passt zu unfertigen Abschnitten.
- Quelle: FACTS.md §4.3, §2.3, §2.2 (Tags), §5.1, §9(b)

### Structure
- Zeigt eine `types`-Tabelle (Vorschlag): `chapter` (Rang 1), `section` (Rang 2), `source` und `question` (Rang 4). (TODO: verify)
- Erklärt: Ein Typ ohne `rank` erhält Rang 4; Ränge bestimmen gültige Eltern (Kind hat größeren Rang).
- Hierarchie-Diagramm (Mermaid, D38): chapter > section > source/question.
- Beispiel-`.lif.yml` mit `types_exclusive: true`: in Phase 5 mit `lif init` bauen und mit `lif check` prüfen. (TODO: verify)
- Erklärt: Der Standardtyp (`lif.default_type`) fällt auf den ersten Typ zurück, wenn `task` fehlt.
- Quelle: FACTS.md §4.3, §3, §9(b); Typnamen und Beispiel-Config `TODO: verify` — Vorschlag, in Phase 5 bauen

### Workflow
- Anlegen: `lif create "…" -t chapter`, `lif create "…" -t section --parent <chapter-id> --tag <theme>`.
- Quelle erfassen: `lif create "…" -t source --blocked-by …` bzw. offene Frage als `question`; Abschnitt wartet über `blocked_by`.
- Weiterarbeiten: `lif next --tag <theme>`, `lif start`, `lif complete --summary "…"`.
- Überblick: `lif list --tag <theme>`, `lif progress <chapter-id>`, `lif list --view tree`.
- Reihenfolge der Abschnitte: `lif order <id> --after <id>`.
- Quelle: FACTS.md §1.1 (create, next, start, complete, list, progress, order), §5.1

### Benefits
- Texte, Notizen und Planung liegen als Dateien nebeneinander; Änderungen sind per Git nachvollziehbar.
- Fortschritt je Kapitel in Prozent; offene Quellen werden über `blocked_by` sichtbar.
- Gleiche Befehle wie in Softwareprojekten, eigenes Vokabular.
- Quelle: FACTS.md §1.1 (progress), §5.1, §4.3

### Limits
- Lif verwaltet keine Zitate oder Literaturdatenbank und setzt keinen Text zusammen. (TODO: verify)
- `lif roadmap` baut auf Meilenstein- und Container-Rängen auf; Verhalten mit eigenen Typen am Beispiel prüfen. (TODO: verify)
- `lif check` prüft Typnamen gegen die Konfiguration, aber keine Eltern-/Kind-Ränge, Status- und Prioritätswerte (D40).
- Quelle: FACTS.md §3, §3.2, §6.9; DESIGN.md D40; Zitate/Export und `lif roadmap` mit eigenen Typen `TODO: verify` — in Phase 5 am Beispiel prüfen

### References
- Features: Flexible planning (`features/project-planning`), Markdown-native (`features/markdown-native`).
- Reference: Configuration (`reference/configuration`), Item file format (`reference/item-file-format`).
- Overview: Concepts (`overview/concepts`).
- Quelle: DESIGN.md D32, D33
