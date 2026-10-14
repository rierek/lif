---
title: "Software project"
description: "Plan milestones, epics and features beside the code."
---

# Software project

## Zweck
Zeigt, wie ein Softwareprojekt mit dem Preset `classic` geplant und verfolgt wird, mit Items im selben Repository wie der Code.

## Zielgruppe
Entwicklerinnen, Teams und Agenten, die Roadmap, Abhängigkeiten und Fortschritt neben dem Code führen wollen.

## Pflichtabschnitte

### Problem
- Beschreibt die Situation: Planung steckt in einem externen Tracker, Code und Plan driften auseinander, Agenten sehen den Plan nicht.
- Nennt Anforderungen: Hierarchie von Meilenstein bis Aufgabe, Abhängigkeiten, Fortschritt auf einen Blick.
- Bleibt Szenario-Text; keine Behauptungen über Lif-Verhalten.
- Quelle: CONCEPT.md §5.4 (Content)

### Solution
- Erklärt: Preset `classic` bildet milestone > epic > feature > bug/task ab; jedes Element ist ein Item im Store.
- Erklärt: `blocked_by`/`blocking` modellieren Reihenfolgen; `lif next` liefert das nächste bereite Item.
- Erklärt: `lif roadmap`, `lif milestones` und `lif progress` zeigen Plan und Stand.
- Quelle: FACTS.md §9(b), §5.1, §5.2, §1.1 (roadmap, milestones, progress)

### Structure
- Zeigt Preset `classic`: milestone (Rang 1), epic (2), feature (3), bug und task (4); Eltern müssen einen kleineren Rang haben als Kinder.
- Hierarchie-Diagramm (Mermaid, D38) mit gültigen Eltern: feature unter epic oder milestone, bug/task unter milestone, epic oder feature.
- Erklärt: vier Ebenen sind Eigenschaft der eingebauten Rangskala, kein erzwungenes Maximum; eigene Typen können tiefer gehen (D39).
- Zeigt mehrere Git-Worktrees mit `lif.anchor: repo-root`: alle teilen einen Store.
- Beispielausgabe `lif roadmap`: in Phase 5 mit `lif init --profile classic` bauen. (TODO: verify)
- Quelle: FACTS.md §3, §9(a), §9(b), §4.2; Beispielausgabe `TODO: verify` — in Phase 5 erzeugen

### Workflow
- Planen: `lif create "…" -t milestone`, `lif create "…" -t epic --parent <id>`, `lif create "…" -t feature --parent <id>`.
- Abhängigkeiten: `lif update <id> --blocked-by <id>`; Anzeige mit `lif graph --format mermaid`.
- Arbeiten: `lif next --type task` → `lif start <id>` → `lif complete <id> --summary "…"`.
- Überblick: `lif roadmap --depth 2`, `lif milestones`, `lif progress <milestone-id>`; Reihenfolge mit `lif order <id> --after <id>`.
- Akzeptanzkriterien als Checkliste im Item-Body: siehe Feature Flexible planning (`features/project-planning`). (TODO: verify)
- Quelle: FACTS.md §1.1 (create, update, graph, next, start, complete, roadmap, milestones, progress, order); Akzeptanzkriterien `TODO: verify` — kein FACTS-Abschnitt, Phase 5 gegen `wiki/` und `lif prime` prüfen

### Benefits
- Plan und Code liegen im selben Repository und wandern mit Branches und Commits.
- Fortschritt je Container: Prozent erledigt = completed / (total − scrapped).
- Reihenfolge wird aus Abhängigkeiten abgeleitet; blockierte Kinder gelten als blockiert, wenn der Elternteil blockiert ist.
- Agenten lesen denselben Plan (`lif prime`, `--json`).
- Quelle: FACTS.md §1.1 (progress, prime), §5.1, §2.4, §4.2

### Limits
- `lif check` prüft keine Eltern-/Kind-Ränge, keine Statuswerte und Prioritätswerte, keine Tags; handbearbeitete Dateien umgehen die Hierarchieprüfung (D40).
- Typwechsel allein löst keine Hierarchieprüfung aus (bekannter Fund).
- `lif next` kann auch Container (milestone, epic, feature) liefern.
- Kein Pre-Commit-Hook im Lieferumfang; die Feature-Seite Git-native zeigt nur ein Snippet (D41).
- Quelle: FACTS.md §3.1, §3.2, §5.2, §6.9, §9(f); DESIGN.md D40, D41, D43

### References
- Features: Flexible planning (`features/project-planning`), Dependencies (`features/dependencies`), Git-native (`features/git-native`).
- Reference: Configuration (`reference/configuration`), `lif roadmap`, `lif milestones`, `lif progress`, `lif order`, `lif graph` (`reference/commands`).
- Use case: Release management (`use-cases/release-management`).
- Quelle: DESIGN.md D32, D39
