---
title: "Git-native"
description: "Items live in your repository and travel with your code."
---

# Git-native

## Zweck
Erklärt, warum Items im Repository liegen, wie sie mit Branches und Worktrees zusammenspielen und wie man sie mit Git nutzt. Zeigt ein kurzes Hook-Snippet für `lif check` und sagt klar, dass Lif keinen Hook mitliefert.

## Zielgruppe
Entwickler, die Backlog und Code gemeinsam versionieren wollen, und Teams mit mehreren Worktrees oder Branches. Sie wollen wissen, was in Konflikten und Commits passiert und wie man Drift im Store verhindert.

## Pflichtabschnitte

### What
- Der Store liegt als Verzeichnis `.lif/` neben der Konfiguration `.lif.yml`, also im Repository; Items sind flache Dateien (neue Items kommen in die Wurzel des Stores, Archiv in `archive/`).
- Eine Datei pro Item mit stabilem Dateinamen `<id>--<slug>.md`; Felder in fester Reihenfolge, extra keys alphabetisch.
- Manuelle Reihenfolge über das Feld `order` (Bruchindex): `lif order` schreibt nur das platzierte Item.
- `lif.anchor: repo-root` lässt alle Git-Worktrees eines Repositorys einen Store teilen; ohne Anker nutzt jeder Worktree seinen eigenen.
- Einstellungen `worktree.*` (`base_ref`, `path`, `setup`, `run`, `integrate`, `fetch_timeout`) konfigurieren Worktrees; welche Oberfläche sie nutzt, ist nur teilweise belegt (`worktree.run` erzeugt einen Run-Button in der Web UI) (TODO: verify)
- `lif.commit_field` (Standard `commit`) hält eine Git-SHA; `lif complete --commit` schreibt sie.
- Quelle: FACTS.md §1.1 (`lif order`, `lif complete`), §2.1, §2.2, §2.4, §4.2, §4.3; (TODO: verify) für den Geltungsbereich von `worktree.*`: `internal/worktree` und Konfigurationsdoku prüfen

### Why
- Items sind Dateien: sie werden mit dem Code diffbar, reviewbar und mergebar.
- Backlog reist mit dem Branch; kein separates Ticketsystem nötig.
- Gemeinsamer Store über Worktrees verhindert Auseinanderlaufen paralleler Arbeit.
- Quelle: DESIGN.md D30; CONCEPT.md §4.1; FACTS.md §4.2

### How it looks
- Diff eines Items nach einem Befehl: nur Front-Matter-Zeilen ändern sich (Beispiel in Phase 5 erzeugen) (TODO: verify)
- Ablauf: Branch anlegen, Item per `lif update` anpassen, Code und Item gemeinsam committen.
- Worktrees: Tabelle oder Diagramm, wie zwei Worktrees mit `lif.anchor: repo-root` denselben Store lesen.
- `lif complete --commit HEAD`: speichert die SHA im Feld `commit`; `lif check` warnt, wenn die SHA im Repository fehlt, und überspringt die Prüfung außerhalb eines Git-Repositorys.
- Quelle: FACTS.md §1.1 (`lif complete`), §4.2, §6.5; (TODO: verify) für Diff-Beispiel: in Phase 5 in einem Test-Repository erzeugen

### Use with Git
- Anleitung: Items wie Code behandeln: committen, reviewen, mergen.
- Commit-Konvention mit Item-ID und Branch pro Container: aus dem Entwurf, nicht in FACTS (TODO: verify)
- Worktrees: `lif.anchor: repo-root` setzen, damit alle Worktrees denselben Store sehen.
- Merge-Konflikte in Item-Dateien: Vorgehen bei Konflikt im Front Matter oder Body, danach `lif check` (TODO: verify)
- Feste Feldreihenfolge und alphabetische extra keys halten Diffs klein; ein Parse-Fehler bricht das Laden ab (`loading <path>: ...`).
- Quelle: FACTS.md §2.1, §4.2, §6.8; (TODO: verify) für Commit-Konvention und Konfliktauflösung: Ablauf in Phase 5 im Testrepository nachstellen

### Pre-commit hook snippet
- Snippet zeigen: ein Git-`pre-commit`-Hook, der `lif check` aufruft; Exit 1 meldet ein Problem und bricht den Commit ab.
- Optional `lif check --strict`, damit auch Warnungen den Commit stoppen.
- Wichtig: Es gibt keinen mitgelieferten Hook, keine Installer-Funktion und kein Rezept; die Seite zeigt nur ein Beispiel, kein Produktcode.
- Hinweis: `lif check` nutzt das aktuelle Verzeichnis zur Store-Auflösung; die Commit-Prüfung läuft ab dem aktuellen Verzeichnis.
- Snippet in Phase 5 in einem Test-Repository ausführen (TODO: verify)
- Quelle: FACTS.md §6.8, §6.10, §9(f), §4.2, §6.5; DESIGN.md D41; (TODO: verify) für das Snippet selbst: in Phase 5 ausführen

### Limits
- Kein Hook wird mitgeliefert; auch CI führt `lif check` nicht aus.
- `lif check` kennt Lücken: Rang von Parent und Kind, Statuswerte, Prioritätswerte, Tags, doppelte IDs und Dateiname gegen ID prüft es nicht.
- Bei doppelter ID gewinnt die später geladene Datei, ohne Meldung.
- `lif rename --prefix` weigert sich, solange `lif-serve` läuft oder aktive Worktrees existieren.
- Ohne `lif.anchor: repo-root` hat jeder Worktree einen eigenen Store; Änderungen laufen erst über Merge zusammen.
- Quelle: FACTS.md §4.2, §6.9, §6.10, §1.1 (`lif rename`); DESIGN.md D40, D41

### Reference links
- Configuration (`reference/configuration`): `lif.path`, `lif.anchor`, `worktree.*`, `lif.commit_field`.
- `lif check` (`reference/commands/lif-check`): Prüfumfang, Exit-Codes, `--strict`, `--fix`.
- `lif complete`, `lif order`, `lif rename`, `lif init` (`reference/commands/lif-complete`, `reference/commands/lif-order`, `reference/commands/lif-rename`, `reference/commands/lif-init`).
- Item file format (`reference/item-file-format`): Felder und Dateinamen.
- Quelle: DESIGN.md D24, D25
