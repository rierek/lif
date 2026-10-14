---
title: "Home lab"
description: "Track servers, services and chores."
---

# Home lab

## Zweck
Zeigt, wie ein Heimserver-Betrieb (Server, Dienste, Wartung, Ausbaupläne) mit einer schlanken eigenen Lif-Konfiguration verwaltet wird.

## Zielgruppe
Home-Lab-Betreiber und Selbsthoster, die Wartung und Ausbau nachvollziehbar in Dateien führen wollen; Agenten, die so ein Setup vorschlagen.

## Pflichtabschnitte

### Problem
- Beschreibt die Situation: Viele Systeme, verstreute Notizen, vergessene Wartung, unklare Reihenfolge (erst Backup, dann Update).
- Nennt Anforderungen: pro System filtern, Abhängigkeiten sichtbar, schnell ein Item finden.
- Bleibt Szenario-Text; keine Behauptungen über Lif-Verhalten.
- Quelle: CONCEPT.md §5.3 (Content)

### Solution
- Erklärt: Eine kleine eigene `types`-Tabelle (Typ-Vorschläge, z. B. Projekt und Aufgabe) statt eines fertigen Presets; Tags benennen die Systeme.
- Erklärt `blocked_by` für Reihenfolgen: ein Item wartet, bis sein Blocker `completed` oder `scrapped` ist.
- Erklärt `lif next` und `lif list --ready` als Antwort auf die Frage was heute dran ist.
- Quelle: FACTS.md §4.3 (`types`, `types_exclusive`), §5.1, §5.2; Typnamen sind ein Vorschlag → `TODO: verify`

### Structure
- Zeigt eine Beispiel-`.lif.yml` mit `types_exclusive: true` und zwei bis drei eigenen Typen; Ränge so wählen, dass Eltern einen kleineren Rang haben als Kinder.
- Erklärt: Ein neuer Typ ohne `rank` erhält Rang 4; Ränge dürfen Lücken haben.
- Hierarchie-Skizze (Mermaid, D38): Projekt (z. B. Netzwerk-Umbau) > Aufgabe; Tags je System (`nas`, `router`).
- Beispiel-Konfiguration und Ausgabe von `lif check`: in Phase 5 mit `lif init` bauen und prüfen. (TODO: verify)
- Optional: `require_fields_on` verlangt für einen Status einen Extra-Schlüssel (z. B. Notiz zum Wartungsfenster). (TODO: verify)
- Quelle: FACTS.md §4.3, §3, §2.3; konkrete Beispiel-Config `TODO: verify` — in Phase 5 mit `lif init` und `lif check` bauen

### Workflow
- Schleife: `lif create "…" -t <type> --tag <system>` → `lif create … --blocked-by <id>` → `lif next --tag <system>` → `lif start` → `lif complete --summary "…"`.
- Zeigt `lif list --tag <system>` und `lif list --is-blocked` für Wartestau.
- Zeigt `lif pick` und das zsh-Widget (Alt+i), um Item-IDs per Fuzzy-Auswahl in die Befehlszeile zu setzen.
- Zeigt `lif graph --format mermaid` für die Abhängigkeitsübersicht.
- Quelle: FACTS.md §1.1 (create, next, list, pick, graph, complete), §8.3, §5.5

### Benefits
- Wartungswissen liegt als Text im Repository oder Verzeichnis und lässt sich mit Git versionieren.
- Reihenfolgen werden über `blocked_by` sichtbar; `lif next` überspringt Blockiertes.
- Systeme lassen sich per Tag filtern, ohne feste Hierarchie.
- Quelle: FACTS.md §5.1, §5.2, §2.2 (Tags)

### Limits
- Wiederkehrende Pflichten (monatliche Updates, Zertifikats-Erneuerung): kein Feature belegt. (TODO: verify)
- Lif überwacht keine Systeme und löst keine Erinnerungen aus. (TODO: verify)
- Ein Link wird nur auf einer Seite gespeichert; `lif check` meldet keine Asymmetrie zwischen `blocking` und `blocked_by`.
- Quelle: FACTS.md §5.1, §5.6; Wiederholung/Überwachung `TODO: verify` — `lif --help` und Config-Schlüssel in Phase 5 prüfen

### References
- Feature: Dependencies (`features/dependencies`), Flexible planning (`features/project-planning`).
- Reference: Configuration (`reference/configuration`), `lif next`, `lif list`, `lif graph`, `lif pick` (`reference/commands`).
- Quelle: DESIGN.md D32, D33
