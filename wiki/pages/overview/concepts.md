---
title: "Concepts"
description: "Items, types, ranks, status, priority, tags, parents, blocking and presets in plain words."
---

# Concepts

## Zweck
Vermittelt das mentale Modell von Lif in einfachen Worten, bevor die Reference gelesen wird. Kanonische Regeln und Tabellen bleiben in der Reference; die Seite erklärt und verweist, sie wiederholt keine Regeln.

## Zielgruppe
Neue Nutzer und Agenten, die nach der Installation verstehen wollen, wie Items, Hierarchie und Abhängigkeiten zusammenhängen. Kein Vorwissen über Lif nötig.

## Pflichtabschnitte

### Item and Liff
- Ein Item ist eine Arbeitseinheit; es liegt als eine Markdown-Datei im Store (YAML-Front-Matter plus Markdown-Body).
- Nennt den Begriff Liff als Name der Datei, mit Verweis auf Reference: Item file format (`reference/item-file-format`) als Definition (D47).
- Erklärt: ID und Slug stammen aus dem Dateinamen `<id>--<slug>.md`, nicht aus dem Front-Matter.
- Erklärt: Zusätzliche Front-Matter-Schlüssel bleiben erhalten.
- Zeigt kleines Beispiel eines Liffs (Phase 5 mit `lif create` erzeugen).
- Quelle: FACTS.md §2.1, §2.2, §2.4; DESIGN.md D47

### Type and rank
- Typ = Art des Items (z. B. `milestone`, `epic`, `feature`, `bug`, `task`); jeder Typ hat einen Rank (ganze Zahl).
- Eingebaute Zuordnung als Tabelle: milestone 1, epic 2, feature 3, bug 4, task 4.
- Ranks 1 bis 3 sind Container, Rank 4 ist Blatt; in Prosa ist "Ebene" ein Synonym für Rank, nie ein Feldname.
- Eigene Typen sind möglich und können tiefere Ranks haben; es gibt keine harte Obergrenze (D39).
- Ein Item ohne Typ wird als `task` geladen.
- Verweis auf Reference: Configuration (`reference/configuration`) für Typen und Ranks.
- Quelle: FACTS.md §2.2, §2.3, §3, §9(a); DESIGN.md D39, D45

### Status
- Status beschreibt den Fortschritt; eingebaute Werte: `in-progress`, `todo`, `draft`, `completed`, `scrapped`.
- `completed` und `scrapped` sind Archiv-Status; `lif archive` verschiebt solche Items nach `archive/`.
- Neue Items erhalten `todo`, wenn nichts anderes gesetzt ist.
- Status lassen sich über `statuses:` in `.lif.yml` ergänzen oder überschreiben.
- Befehle zum Wechseln: `lif start`, `lif complete`, `lif scrap`, `lif update --status`.
- Quelle: FACTS.md §2.2, §2.3, §1.1 (`start`, `complete`, `scrap`, `archive`, `update`)

### Priority
- Priorität von dringend bis zurückgestellt: `critical`, `high`, `normal`, `low`, `deferred`.
- Neue Items bekommen keine gespeicherte Priorität; ein Item ohne Priorität wird als `normal` geladen.
- Priorität steuert die Standard-Sortierung (Status, dann Priorität, dann Typ, dann Titel) und damit `lif next`.
- Zeigt `lif update <id> --priority high`.
- Quelle: FACTS.md §2.2, §2.3, §1.1 (`next`, `list` mit `--sort`)

### Tags
- Tags sind freie Stichwörter ohne Hierarchie; sie dienen zum Filtern (`lif list --tag`).
- Format: klein geschrieben, beginnt mit Buchstaben, Bindestriche nur zwischen Buchstaben/Ziffern.
- Zeigt `lif tag <id> --tag x` und `--remove-tag`.
- Ehrliche Grenze: Das Tag-Format wird bei Schreibzugriffen über lif geprüft, bei Handänderungen nicht und auch nicht von `lif check`.
- Quelle: FACTS.md §2.2, §1.1 (`tag`, `list`), §6.9, §9(e)

### Parent and hierarchy
- Ein Item hat höchstens einen Parent; die Regel lautet: Der Rank des Kindes muss größer sein als der des Parents.
- Erklärt Folgen: feature unter epic oder milestone; task unter task wird abgelehnt (Beispiel-Fehlermeldung).
- Erklärt: lif erzwingt die Regel bei Schreibzugriffen (`lif create --parent`, `lif update --parent`, GraphQL, TUI); Handänderungen in der Datei werden beim Laden nicht geprüft (D39).
- Ehrliche Grenze: `lif check` prüft Ranks nicht; zudem prüft ein reiner Typwechsel per `lif update -t` den Parent nicht erneut (bekannte Lücke).
- Zeigt `lif roadmap` und `lif list --view tree` als Hierarchie-Ansichten.
- Quelle: FACTS.md §3, §3.1, §3.2, §9(c), §9(d), §1.1 (`roadmap`, `list`); DESIGN.md D39, D40, D43

### Blocking and blocked_by
- Zwei Listen im Front-Matter: `blocking` (IDs, die dieses Item blockiert) und `blocked_by` (IDs, die dieses Item blockieren).
- Ein Link wird nur auf einer Seite gespeichert (am geänderten Item); beim Lesen werden beide Richtungen vereint.
- Erklärt in einem Absatz "blocked" und "ready": blockiert = ein nicht abgeschlossener Blocker am Item oder an einem Vorfahren; bereit = nicht blockiert, nicht `in-progress`, nicht `draft`, kein Archiv-Status, kein abgeschlossener Vorfahr.
- Zeigt `lif next`, `lif list --ready`, `lif list --is-blocked`.
- Verweis auf Features: Dependencies (`features/dependencies`) als kanonischen Ort der Semantik.
- Quelle: FACTS.md §5.1, §5.2, §1.1 (`next`, `list`)

### Presets
- Preset = vorgefertigte Typtabelle, gewählt mit `lif init --profile`: `classic`, `todo`, `simple`, `complex`.
- Beispiel-Tabelle: Preset · Typen · Ranks · Zweck (Mermaid-Diagramm je Preset kommt auf Reference: Configuration, D38).
- Erklärt: Das Preset wird beim Init in `.lif.yml` ausgeschrieben, nicht als Name referenziert; danach frei änderbar.
- Betont: Presets sind Beispiele, keine feste Vorgabe; eigene Typen jederzeit möglich (D39).
- Quelle: FACTS.md §9(b), §3, §1.1 (`init`); DESIGN.md D38, D39

### How the concepts fit together
- Mermaid-Skizze: Store (`.lif.yml` + `.lif/`) → Liff → Felder (Typ/Rank, Status, Priorität, Tags) und Beziehungen (Parent, blocking/blocked_by).
- Zweites Mini-Diagramm: Hierarchie milestone > epic > feature > task, daneben ein blockierender Link quer über die Hierarchie.
- Ein Absatz, wie `lif next` die Felder verbindet: bereit, höchste Priorität.
- Verweise auf Reference: Glossary (`reference/glossary`) und Features (`features/index`).
- Quelle: FACTS.md §2, §3, §5.2; DESIGN.md D38
