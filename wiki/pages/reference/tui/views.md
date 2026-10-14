---
title: "Views"
description: "Tree, flat, backlog and detail views of the TUI."
---

# Views

## Zweck
Beschreibt die Ansichten der TUI, wie man zwischen ihnen wechselt und wie man sich darin bewegt, sucht und filtert.

## Zielgruppe
Nutzer der TUI, die wissen wollen, welche Ansicht für welche Aufgabe passt und wie die Bedienung grundsätzlich funktioniert.

## Pflichtabschnitte

### Overview of the views
- Tabelle: Repo-Baum (Browse), flache Liste, Backlog, Item-Detail, Vollbild, Lobby, Tag-Verwaltung; je Ansicht Zweck in einer Zeile und Taste zum Erreichen.
- Erklärt: `tab` wechselt den Fokus zwischen Baum und Detail. (Anordnung auf dem Bildschirm: TODO: verify)
- Verweist für Ansichten mit Taste auf `reference/tui/key-bindings`.
- Quelle: FACTS.md §8.2, §8.3; Bildschirmanordnung — `TODO: verify` (Phase 5: laufende TUI prüfen)

### Tree view
- Zeigt die Hierarchie der Items (Eltern/Kinder) im Repo-Baum; `→`/`l` geht hinein bzw. klappt auf, `←`/`j` geht zurück bzw. hinaus.
- Baumbreite: Einstellung `layout.tree_width` (Standard 36, Minimum 24, Maximum 60).
- Screenshot `browse.png` (Plan).
- Quelle: FACTS.md §8.2, §8.3

### Flat view
- `G` schaltet zwischen verschachtelt (nested) und flach um.
- Zeigt, wann die flache Liste hilfreich ist (Suche über alle Items, ohne Hierarchie).
- Quelle: FACTS.md §8.3, §8.2

### Backlog view
- `b` öffnet das Backlog; `S` wechselt die Sortierung (zyklisch).
- Erklärt, welche Items das Backlog zeigt und welche Sortierungen es gibt. (TODO: verify)
- Screenshot `backlog.png` (Plan).
- Quelle: FACTS.md §8.3, §8.2; Inhalt und Sortierarten — `TODO: verify` (Phase 5: `internal/tui/view_browse_backlog.go` und laufende TUI prüfen)

### Detail view
- Zeigt ein Item mit Feldern, Beziehungen und Body; `tab` setzt den Fokus in die Detailansicht, `shift+tab` zurück.
- `v` öffnet das Item im Vollbild (Screenshot `detail-fullscreen.png`, Plan).
- `1`..`9` springen zu einem Abschnitt (Akkordeon) der Detailansicht.
- Aufbau der Abschnitte im Detail beschreiben. (TODO: verify)
- Quelle: FACTS.md §8.3, §8.2; Abschnittsnamen — `TODO: verify` (Phase 5: `internal/tui/view_detail_bean.go` prüfen)

### Moving around
- Bewegung mit der `i/j/k/l`-Raute (hoch/links/runter/rechts), nicht mit vim `h/j/k/l`; Pfeiltasten gehen ebenfalls.
- `enter` öffnet, `esc` geht zurück; `[` / `ctrl+left` zurück in der Verlaufsliste, `]` / `ctrl+right` vor.
- `K` öffnet die Befehlspalette, `ctrl+r` lädt neu.
- Maus wird unterstützt. (Umfang der Mausbedienung: TODO: verify)
- Quelle: FACTS.md §8.3, §8.2; Mausumfang — `TODO: verify` (Phase 5: `internal/tui/mouse.go` prüfen)

### Search and filters
- `/` startet die Suche; `f` öffnet die Facetten-Filter (Typ, Status, Priorität, Tags); `space`/`x` schaltet eine Facette um; `X` löscht alle Filter.
- Im Item-Picker gibt es Facetten-Chips: `ctrl+t` Typ, `ctrl+n` Status, `ctrl+p` Priorität, `ctrl+g` Tags.
- Verweist für die Suchsyntax auf `reference/search-syntax`. (Gilt die Syntax auch in der TUI? TODO: verify)
- Quelle: FACTS.md §8.3, §8.2; Suchsyntax in der TUI — `TODO: verify` (Phase 5: `internal/tui/box_filter_bar.go` prüfen)

### Screenshots plan
- Vorhandene Bilder: `wiki/screenshots/{browse,backlog,detail-fullscreen,command-center,lobby}.png`; Zuordnung: Tree, Backlog, Detail im Vollbild, Befehlspalette, Lobby.
- Neue Aufnahmen als webp bzw. VHS-GIF (D38), wenn die Bilder nicht mehr zur Oberfläche passen.
- Quelle: DESIGN.md D38, D44; Aktualität — `TODO: verify` (Phase 5: mit laufender TUI vergleichen)
