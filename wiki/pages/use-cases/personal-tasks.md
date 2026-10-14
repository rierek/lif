---
title: "Personal tasks"
description: "Run your own to-do list as plain files."
---

# Personal tasks

## Zweck
Zeigt, wie eine einzelne Person ihre To-do-Liste mit Lif führt: flach mit dem Preset `todo` oder mit kleiner Hierarchie über `simple`.

## Zielgruppe
Einzelpersonen, die eine lokale, textbasierte Aufgabenliste ohne Dienst wollen, und Agenten, die so ein Setup für Nutzer aufsetzen.

## Pflichtabschnitte

### Problem
- Beschreibt die Situation: Aufgaben verteilen sich auf Notiz-Apps, Zettel und E-Mails; Prioritäten und der nächste Schritt sind unklar.
- Nennt die Anforderungen: lokal, lesbar, ohne Konto, später erweiterbar.
- Bleibt Szenario-Text; keine Behauptungen über Lif-Verhalten.
- Quelle: CONCEPT.md §5.2 (Content)

### Solution
- Erklärt: Jede Aufgabe ist ein Item in einer Markdown-Datei im Store; Status, Priorität und Tags ordnen sie.
- Nennt die zwei Wege: Preset `todo` (ein Typ `task`, keine Hierarchie) oder `simple` (kleine Hierarchie).
- Erklärt, dass `lif next` die wichtigste bereite Aufgabe nennt.
- Quelle: FACTS.md §9(b), §2.1, §5.2

### Structure
- Zeigt Preset `todo`: einziger Typ `task` auf Rang 4; Gruppierung über Tags (z. B. Bereiche wie `home`, `work`).
- Zeigt Preset `simple`: milestone (Rang 1), bucket (Rang 1, `roadmap: false`), epic und feature (Rang 2), bug und task (Rang 4); drei Ebenen.
- Hierarchie-Diagramm (Mermaid, D38) für `simple`; für `todo` genügt eine Zeile.
- Erklärt Tag-Format: kleingeschrieben, beginnt mit Buchstaben, Bindestriche nur zwischen Alphanumerischem.
- Beispielausgabe von `lif init --profile todo` und `lif check`: in Phase 5 bauen und prüfen. (TODO: verify)
- Quelle: FACTS.md §9(b), §3, §2.2 (Tags); Beispielausgabe `TODO: verify` — in Phase 5 mit `lif init --profile todo` erzeugen

### Workflow
- Befehlsschleife: `lif create "…" --tag <tag> -p <priority>` → `lif next` → `lif start <id>` → `lif complete <id>`.
- Zeigt `lif list --ready` und `lif list --tag <tag>` für Übersichten.
- Zeigt Prioritäten (`critical`, `high`, `normal`, `low`, `deferred`) und Reihenfolge mit `lif order`.
- Zeigt `lif scrap <id> --reason "…"` für verworfene Aufgaben und `lif archive` zum Aufräumen.
- Zeigt kurze IDs: Item-ID-Argumente akzeptieren auch die ID ohne Präfix.
- Quelle: FACTS.md §1.1 (create, next, start, complete, list, order, scrap, archive), §2.3, §1.0

### Benefits
- Aufgaben sind einfache Dateien: lesbar, durchsuchbar, versionierbar.
- Eine Priorität und ein Status genügen für `lif next`, kein Zusatzwerkzeug.
- Wächst mit: von `todo` zu `simple` oder eigenen `types`, ohne Datenformat-Wechsel. (TODO: verify)
- Quelle: FACTS.md §2.1, §5.2; Wachstumsaussage `TODO: verify` — Wechsel des Presets mit bestehenden Items in Phase 5 ausprobieren

### Limits
- Kein typisiertes Fälligkeitsdatum: `created_at` und `updated_at` sind verwaltet; eigene Schlüssel wie `due` sind nur Extra-Front-Matter-Schlüssel (`--set`, `lif list --where`).
- Wiederkehrende Aufgaben und Erinnerungen: Unterstützung nicht belegt. (TODO: verify)
- `lif next` kann auch Container-Items liefern (kein Typ-Ausschluss).
- Quelle: FACTS.md §2.1, §2.2, §5.2; Wiederholung/Erinnerung `TODO: verify` — `lif prime`/Hilfe und Config-Schlüssel in Phase 5 prüfen

### References
- Feature: Flexible planning (`features/project-planning`), Dependencies (`features/dependencies`).
- Reference: Configuration (`reference/configuration`) für Presets; Befehle `lif create`, `lif next`, `lif start`, `lif complete`, `lif archive` (`reference/commands`).
- Overview: First steps (`overview/first-steps`).
- Quelle: DESIGN.md D32, D33, D39
