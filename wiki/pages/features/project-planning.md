---
title: "Flexible planning"
description: "Types, statuses and hierarchy presets that adapt to how you work."
---

# Flexible planning

## Zweck
Zeigt, wie Typen, Ränge, Status, Prioritäten und Parents die Struktur eines Projekts abbilden und welche Regeln Lif beim Schreiben durchsetzt. Enthält die Anleitung für Acceptance criteria und beschreibt ehrlich, wo die Regeln enden.

## Zielgruppe
Menschen und Agents, die eine Planungsstruktur wählen oder anpassen (einfache To-do-Liste bis tiefe Hierarchie). Sie wollen wissen, welche Presets es gibt, was erzwungen wird und was nicht.

## Pflichtabschnitte

### What
- Ein Item hat `type` (mit Rang), `status`, `priority`, `tags` und höchstens einen `parent`; ein Parent ist gültig, wenn der Rang des Kindes strikt größer ist als der des Parents.
- Eingebaute Typen: `milestone` (Rang 1), `epic` (2), `feature` (3), `bug` und `task` (4); Status: `in-progress`, `todo`, `draft`, `completed`, `scrapped`; Prioritäten: `critical`, `high`, `normal`, `low`, `deferred`.
- Eigene Typen, Status und Prioritäten in `.lif.yml` (`types`, `statuses`, `priorities`) überschreiben eingebaute Einträge oder ergänzen sie; `types_exclusive` ersetzt die Typtabelle vollständig.
- Vier Presets über `lif init --profile`: `classic`, `todo`, `simple`, `complex`; das Preset wird beim Anlegen in die Konfiguration ausgeschrieben, nicht per Namen referenziert.
- Planungsbefehle: `lif roadmap`, `lif milestones`, `lif progress`, `lif order` (manuelle Reihenfolge über `order`).
- Quelle: FACTS.md §2.2, §2.3, §3, §4.3, §9(b)

### Why
- Lif passt sich verschiedenen Projektstrukturen an und erzwingt die konfigurierten Regeln beim Schreiben.
- Statt fester Vorgabe: Presets sind Beispiele, die Struktur gehört dem Projekt (D39).
- Beispiele für unterschiedliche Modelle: Ein-Ebenen-Liste, Meilenstein bis Task, Release-Modell.
- Quelle: CONCEPT.md §4.2; DESIGN.md D39; FACTS.md §3.1, §9(c)

### How it looks
- Mermaid-Diagramme der vier Presets aus den Definitionen: `classic` (milestone 1, epic 2, feature 3, bug/task 4), `todo` (nur `task`), `simple` (milestone, bucket, epic, feature, bug, task), `complex` (release, bucket, feature, improvement, chore, story, bug, task).
- Hinweis: `bucket` ist mit `roadmap: false` aus Roadmap und Meilensteinliste ausgeblendet.
- Ausgabe von `lif roadmap` (zum Beispiel mit `--tags`, `--depth`) als echter Auszug; in Phase 5 mit `lif init --profile` aufsetzen und festhalten (TODO: verify).
- Regelverstoß zeigen: Task unter Task wird abgelehnt mit `task items can only have milestone, epic, or feature as parent, not task`.
- Preset-Diagramme in Phase 5 mit `lif init --profile <name>` nachbauen und prüfen (TODO: verify)
- Quelle: FACTS.md §3, §3.1, §9(b); (TODO: verify) für Roadmap-Ausgabe und Diagramme: in Phase 5 gegen `lif init --profile` und `lif roadmap` laufen lassen

### Acceptance criteria
- Anleitung zum Hinzufügen: Kriterien als Checkliste im Body des Items, jedes Kriterium beobachtbar; Format und Vorlage je Typ (Blatt gegenüber Container) (TODO: verify)
- Anleitung zum Prüfen: Kriterien lesen und abhaken, bevor `lif complete` läuft; Beweise im Item oder unter `attachments/<item-id>/`.
- Was Lif beim Abschluss erzwingt: Statusabhängige Pflichtfelder über `lif.require_fields_on` (Status mit Liste extra keys); fehlt ein Wert, schlägt Anlegen und Aktualisieren mit Policy-Verletzung fehl.
- `lif check` meldet fehlende Pflichtfelder als Warnung; `lif check --strict` zählt sie als Problem (Exit 1).
- `lif complete --summary` hängt einen Abschnitt "Summary of Changes" an; `--commit` hält die SHA im konfigurierten Feld (Standard `commit`) fest.
- Klären: Checklisten im Body sind Konvention; FACTS zeigt keinen Code, der sie auswertet (TODO: verify)
- Quelle: FACTS.md §2.3, §6.5, §1.1 (`lif complete`); (TODO: verify) für Checklistenformat und "Konvention oder Feature": `internal/commands/prompt.tmpl`, `complete.go` und `lif prime` prüfen

### Limits
- Die Rangregel gilt beim Schreiben über Lif (CLI, GraphQL, TUI, Web UI), nicht bei Handbearbeitung der Datei.
- `lif check` prüft weder Rang von Parent und Kind noch Statuswerte, Prioritätswerte oder Tags; es meldet nur unbekannten Typ, defekte Links, Selbstbezüge, Zyklen, fehlende Pflichtfelder und verwaiste Anhang-Verzeichnisse.
- Bekannter Fehler: Wer nur `type` ändert (`lif update -t`, GraphQL `updateItem(type)`), umgeht die Prüfung des Parent-Rangs; `lif check` meldet es danach auch nicht.
- Keine feste Tiefengrenze: Eingebaute Typen und `classic` nutzen bis zu vier Ränge, eigene Typen mit Rang 5 oder höher werden angenommen.
- Typen mit gleichem Rang können nicht ineinander liegen; Ränge dürfen Lücken haben.
- `lif next` schließt Container nicht aus; der Hinweis am Typ `epic` ("nicht direkt bearbeiten") wird nicht erzwungen.
- Quelle: FACTS.md §3, §3.1, §3.2, §5.2, §6.9, §9(a), §9(c), §9(d), §9(e); DESIGN.md D39, D40, D43

### Reference links
- Configuration (`reference/configuration`): Typen, Ränge, Status, Prioritäten, Presets, `require_fields_on`.
- `lif init` (`reference/commands/lif-init`): Option `--profile`.
- `lif roadmap`, `lif milestones`, `lif progress`, `lif order` (`reference/commands/lif-roadmap`, `reference/commands/lif-milestones`, `reference/commands/lif-progress`, `reference/commands/lif-order`): Planungsansichten und Reihenfolge.
- `lif complete` (`reference/commands/lif-complete`) und `lif check` (`reference/commands/lif-check`): Abschluss und Prüfung.
- Quelle: DESIGN.md D24, D25 und Open (Page-Cut Phase 4)
