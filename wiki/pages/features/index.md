---
title: "Features"
description: "What Lif can do, with a page for each capability."
---

# Features

## Zweck
Einstieg in das Kapitel Features: zeigt die sechs Fähigkeiten von Lif auf einen Blick und erklärt, wie sie zusammenhängen. Die Seite beschreibt Fähigkeiten, keine einzelnen Befehle; Details stehen auf den Feature-Seiten und in der Reference.

## Zielgruppe
Menschen, die Lif bewerten oder empfehlen, und Coding-Agents, die per Websuche herausfinden, was Lif kann. Sie wollen in wenigen Zeilen wissen, welche Fähigkeit ihre Frage beantwortet, und gezielt zur passenden Seite springen.

## Pflichtabschnitte

### What Lif can do
- Eine Zeile pro Feature mit Slug in Backticks: Markdown-native (`features/markdown-native`), Flexible planning (`features/project-planning`), Dependencies (`features/dependencies`), Surface independence (`features/surface-independence`), Automation and agents (`features/automation`), Git-native (`features/git-native`).
- Prinzip vorab: Ein Feature ist eine Fähigkeit, kein Befehl; Befehle zeigen Fähigkeiten nur (CONCEPT §4).
- Hinweis auf das feste Seitenmuster: What, Why, How it looks, Limits, Reference links; Anleitungen sind als eingeklappte Abschnitte in die passende Feature-Seite gewandert.
- Sechs Seiten, nicht fünf: Git-native kam als sechstes Feature hinzu.
- Quelle: DESIGN.md D30, D31, D32; CONCEPT.md §4

### How the features fit together
- Erklärt die gemeinsame Grundlage: Ein Item ist eine Markdown-Datei im Store (`.lif/` neben `.lif.yml`), alle anderen Features setzen darauf auf.
- Verknüpfung: Planung (Typen, Ränge, Parents) und Abhängigkeiten (`blocking` / `blocked_by`) beschreiben Beziehungen zwischen Dateien; `lif next` und `lif roadmap` lesen beides.
- Verknüpfung: Alle schreibenden Wege (CLI, GraphQL, TUI, Web UI) laufen durch dieselbe Item-Logik und erzwingen dieselben konfigurierten Regeln; direktes Editieren der Dateien ist frei und wird nachträglich mit `lif check` geprüft.
- Verknüpfung: Weil der Store im Repository liegt, reisen Items mit dem Code (Git-native); Automatisierung und Agents arbeiten auf denselben Dateien.
- Optional: Mermaid-Diagramm, das Store in der Mitte zeigt und die sechs Features darum gruppiert (D38).
- Quelle: FACTS.md §2.1, §2.4, §3.1, §4.2, §5.2, §7.5; DESIGN.md D38

### Which feature answers which question
- Tabellenskizze mit Spalten Frage, Feature-Seite, Einstiegsbefehl.
- Zeile: "Kann ich meine Daten ohne Lif lesen und ändern?" mit `features/markdown-native`.
- Zeile: "Wie bilde ich Meilensteine, Epics und Aufgaben ab?" mit `features/project-planning` und `lif init --profile`.
- Zeile: "Was kann ich jetzt anfangen, was blockiert mich?" mit `features/dependencies` und `lif next`.
- Zeile: "Kann ich CLI, TUI, Web UI und meinen Editor mischen?" mit `features/surface-independence`.
- Zeile: "Wie binde ich Skripte oder Coding-Agents an?" mit `features/automation`, `lif prime`, `--json`.
- Zeile: "Wie läuft das mit Branches, Worktrees und Commits?" mit `features/git-native`.
- Quelle: FACTS.md §1.1 (`lif init`, `lif next`, `lif prime`), §5.2; DESIGN.md D30

### Where to go next
- Use cases (`use-cases/index`): dieselben Features in fünf durchgespielten Szenarien.
- Reference (`reference/index`): Befehle, Item-Dateiformat, Konfiguration, GraphQL.
- Einstieg für Coding-Agents: Agent onboarding (`overview/agent-onboarding`).
- Hinweis auf die Grenzen: Jede Feature-Seite hat einen Abschnitt Limits; die Seite weist darauf hin, dass dort ehrlich steht, was Lif nicht prüft (D39 bis D41).
- Quelle: DESIGN.md D32, D39, D40, D41
