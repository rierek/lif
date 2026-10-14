---
title: "Use cases"
description: "Real problems, solved with Lif."
---

# Use cases

## Zweck
Einstiegsseite des Kapitels Use cases. Sie grenzt Use case, Feature und Blog-Beitrag voneinander ab, stellt die fünf Szenarien vor und hilft bei der Auswahl.

## Zielgruppe
Menschen, die ihre Situation wiedererkennen und ein passendes Preset oder eine passende Struktur finden wollen. Auch Agenten, die ein Szenario für einen Nutzer empfehlen.

## Pflichtabschnitte

### Introduction
- Erklärt: Ein Use case zeigt eine allgemein anwendbare Lösung für ein wiedererkennbares Problem; ein Feature erklärt eine Fähigkeit; ein Blog-Beitrag teilt eine konkrete Erfahrung oder ein Experiment.
- Betont: Use cases erklären eine vollständige Anwendung, nicht jeden Befehl einzeln.
- Verweist für Details auf Features (`features`) und Reference (`reference`).
- Hinweis: Preset-Namen und Hierarchien sind Beispiele, keine feste Vorgabe (D39).
- Quelle: CONCEPT.md §5 (Purpose, Recommended Content Pattern), §6 (Editorial Distinction); DESIGN.md D33, D39

### The five use cases
- Je Szenario eine Zeile: Problem in einem Satz, Preset bzw. Struktur, Slug.
- Personal tasks (`use-cases/personal-tasks`): Preset `todo` (ein Leaf-Typ, Gruppierung über Tags) oder `simple`.
- Home lab (`use-cases/homelab`): schlanke eigene Konfiguration mit eigenen `types` (Typnamen als Vorschlag, `TODO: verify`).
- Software project (`use-cases/software-projects`): Preset `classic` (milestone > epic > feature > bug/task).
- Release management (`use-cases/release-management`): Preset `complex` (release, bucket, feature/improvement/chore, story/bug/task).
- Writing or research project (`use-cases/writing-research`): eigene `types` für ein nicht-technisches Beispiel.
- Quelle: FACTS.md §9(b), §3; DESIGN.md D33, D42; Typnamen für Home lab und Writing sind Vorschläge → `TODO: verify` (in Phase 5 mit `lif init`/`lif check` bauen)

### How to pick one
- Tabelle: Situation, Preset oder Struktur, Seite.
- Zeigt die vier mitgelieferten Presets `classic`, `todo`, `simple`, `complex` (Auswahl mit `lif init --profile`) und wann ein eigenes `types`-Set sinnvoller ist.
- Erklärt: Ein Preset wird beim Init in die Konfiguration ausgeschrieben, nicht per Name referenziert; später frei änderbar.
- Verweist auf Reference: Configuration (`reference/configuration`) als kanonische Heimat der Presets.
- Quelle: FACTS.md §9(b), §4.3; DESIGN.md D39

### The common pattern
- Erklärt die sieben Abschnitte jeder Use-case-Seite: Problem, Solution, Structure, Workflow, Benefits, Limits, References.
- Structure zeigt Preset oder `types`-Tabelle plus Hierarchie; Workflow ist eine konkrete Befehlsschleife; Limits nennt ehrlich, was Lif für das Szenario nicht tut.
- Hinweis: Beispiel-Konfigurationen werden vor Veröffentlichung mit `lif init` und `lif check` gebaut und geprüft.
- Quelle: CONCEPT.md §5 (Recommended Content Pattern); DESIGN.md D33; der Abschnitt Limits ergänzt das Seed-Pattern (Skeleton-Entscheidung)

### Experiments live on the Blog
- Erklärt: Experimentelle Szenarien stehen im Blog mit Tag `experiment`, nicht unter Use cases.
- Nennt als Beispiel Managing Lif items in Obsidian (`blog/obsidian-experiment`).
- Verweist auf die Blog-Übersicht (`blog`).
- Quelle: DESIGN.md D34, D42
