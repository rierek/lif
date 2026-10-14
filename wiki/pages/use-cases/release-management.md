---
title: "Release management"
description: "Plan, track and ship a release."
---

# Release management

## Zweck
Zeigt, wie ein Release mit dem Preset `complex` geplant, nachverfolgt und über Commit-SHAs rückführbar gemacht wird.

## Zielgruppe
Release-Verantwortliche, Teams mit Review-Befunden und Agenten, die Lieferumfang und Nachweise verwalten.

## Pflichtabschnitte

### Problem
- Beschreibt die Situation: Unklar, was in welches Release gehört, welche Items erledigt sind und mit welchem Commit.
- Nennt Anforderungen: Wertbeiträge (Feature, Improvement, Chore) trennen, Befunde aus Reviews aufnehmen, Nachweis je Item.
- Bleibt Szenario-Text; keine Behauptungen über Lif-Verhalten.
- Quelle: CONCEPT.md §5.5 (Content)

### Solution
- Erklärt: Preset `complex` trennt Release-Container, Wertbeiträge und Umsetzungs-Items.
- Erklärt: `lif promote` macht aus Review-Befunden Items; `lif complete --commit` speichert die SHA am Item.
- Erklärt: `lif check` prüft gespeicherte SHAs gegen das Repository.
- Quelle: FACTS.md §9(b), §1.1 (promote, complete), §6.5

### Structure
- Zeigt Preset `complex`: release (Rang 1), bucket (Rang 1, `roadmap: false`), feature, improvement und chore (Rang 2), story, bug und task (Rang 4).
- Erklärt: Rang 3 ist unbesetzt, die Regel erlaubt Lücken; das Preset hat drei Ebenen.
- Erklärt `bucket` mit `roadmap: false`: ausgeblendet in `lif roadmap` und `lif milestones`, auf Containerrang ganzer Teilbaum.
- Hierarchie-Diagramm (Mermaid, D38).
- Optional: `require_fields_on` verlangt `commit` für `completed`; Schlüssel heißt standardmäßig `commit` (`lif.commit_field`).
- Beispiel-Konfiguration: in Phase 5 mit `lif init --profile complex` bauen und mit `lif check` prüfen. (TODO: verify)
- Quelle: FACTS.md §9(b), §3, §4.3, §2.3; Beispiel `TODO: verify` — in Phase 5 erzeugen

### Workflow
- Planen: `lif create "…" -t release`, Items mit `--parent <release-id>` einhängen.
- Review-Befunde: `lif promote <artifact> <finding-id>…` (Artefaktformat siehe Befehlsseite).
- Umsetzen: `lif start <id>` → `lif complete <id> --commit HEAD --summary "…"`.
- Prüfen: `lif check --strict` zählt Policy-Warnungen (fehlende Pflichtfelder, unbekannte SHAs) als Fehler; Exit-Code 1 bei Problemen.
- Roadmap als Markdown: `lif roadmap --format markdown`; Fortschritt `lif progress <release-id>`.
- Release-Tag setzen und Release Notes schreiben: nicht Teil von Lif, Ablauf prüfen. (TODO: verify)
- Quelle: FACTS.md §1.1 (create, promote, start, complete, check, roadmap, progress), §6.5, §6.8; Release-Tag/Notes `TODO: verify` — kein FACTS-Abschnitt, in Phase 5 gegen Repo-Prozess (`justfile`, `README.md`) prüfen

### Benefits
- Jedes erledigte Item kann eine Commit-SHA tragen: Rückführung von Item zu Code.
- Befunde aus Reviews werden validiert übernommen; bereits übernommene Befunde werden übersprungen.
- Release-Umfang ist ein Teilbaum mit Fortschrittszahl.
- Quelle: FACTS.md §1.1 (promote, complete, progress), §6.5

### Limits
- Lif legt keine Git-Tags an und erzeugt keine Release Notes aus Items. (TODO: verify)
- `lif check` verifiziert nur nicht-leere SHA-Strings im Commit-Feld und warnt (kein Fehler ohne `--strict`); außerhalb eines Git-Repositories wird die Prüfung übersprungen.
- `lif check` prüft keine Eltern-/Kind-Ränge und keine Statuswerte.
- Kein Pre-Commit-Hook im Lieferumfang (D41).
- Quelle: FACTS.md §6.5, §6.9, §9(f); DESIGN.md D40, D41; Tagging/Notes `TODO: verify` — in Phase 5 gegen Repo-Prozess prüfen

### References
- Features: Dependencies (`features/dependencies`), Git-native (`features/git-native`), Automation and agents (`features/automation`).
- Reference: Configuration (`reference/configuration`), `lif promote`, `lif complete`, `lif check`, `lif roadmap` (`reference/commands`).
- Use case: Software project (`use-cases/software-projects`).
- Quelle: DESIGN.md D32, D39
