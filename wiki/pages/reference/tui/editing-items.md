---
title: "Editing items"
description: "Menus, pickers and dialogs for changing items in the TUI."
---

# Editing items

## Zweck
Erklärt, wie man Items in der TUI anlegt, ändert und löscht, über welche Menüs, Picker und Dialoge das läuft und wo die Grenzen liegen.

## Zielgruppe
Nutzer der TUI, die Items ohne CLI pflegen, und Leser, die wissen wollen, ob die TUI dieselben Regeln wie die CLI anwendet.

## Pflichtabschnitte

### Principles
- Erklärt: Schreibzugriffe der TUI gehen durch dieselben Item-Operationen wie CLI und GraphQL; Beispiel: der Parent-Picker ruft `SetParent` mit dem ETag auf.
- Folge: dieselben Regeln (Rang-Regel, Zyklusprüfung) gelten wie bei `lif update --parent`.
- Alle Aktionen arbeiten auf dem ausgewählten Item; Tastenkürzel siehe `reference/tui/key-bindings`.
- Quelle: FACTS.md §8.2, §3.1

### Create an item
- `c` öffnet das Formular zum Anlegen und danach einen Bestätigungsdialog.
- Zeigt die Felder des Formulars. (TODO: verify)
- Erklärt, welcher Status und welche Priorität vorbelegt werden und ob ein Elternteil aus der Auswahl übernommen wird. (TODO: verify)
- Verweist auf die CLI-Entsprechung `lif create` (`reference/commands/lif-create`).
- Quelle: FACTS.md §8.2, §8.3; Formularfelder und Vorbelegung — `TODO: verify` (Phase 5: `internal/tui/form_create_bean.go` und laufende TUI prüfen)

### Quick changes
- `s` öffnet das Status-Menü, `o` das Typ-Menü, `u` das Prioritäts-Menü.
- Menüs zeigen die konfigurierten Werte (`reference/configuration`). (TODO: verify)
- Titel ändern über ein eigenes Formular (Taste: TODO: verify).
- Quelle: FACTS.md §8.3, §8.2; Werte der Menüs und Taste für Titel-Dialog — `TODO: verify` (Phase 5: `internal/tui/keymap.go` und Menü-Code prüfen)

### Pickers
- Parent-Picker `a`: wählt das Elternelement; wendet die Hierarchieregeln an.
- Relations-Picker `r`: Blocking-Beziehungen (blocking / blocked_by) setzen und entfernen.
- Tag-Picker `t`: Tags umschalten mit `space`; `n` legt einen neuen Tag an.
- Item-Picker bieten Facetten-Chips zum Eingrenzen (`ctrl+t`, `ctrl+n`, `ctrl+p`, `ctrl+g`).
- Quelle: FACTS.md §8.2, §8.3

### Edit the body
- `e` oder `ctrl+e` öffnet den Body im Editor.
- Erklärt, wie der Editor gewählt wird (`editor` in den Settings oder `$EDITOR`) mit Verweis auf `reference/tui/settings-and-themes`. (Reihenfolge: TODO: verify)
- Nach dem Schließen wird der Body gespeichert. (TODO: verify)
- Quelle: FACTS.md §8.2, §8.3; Editor-Reihenfolge und Speichern — `TODO: verify` (Phase 5: `internal/tui/editor.go` prüfen)

### Delete
- `d` löscht nach Bestätigung; kein Kaskadieren (Kinder werden nicht mitgelöscht).
- Löschen entfernt eingehende Links automatisch (beim GraphQL-`deleteItem` belegt; gilt für die TUI vermutlich gleich). (TODO: verify)
- Quelle: FACTS.md §8.3, §7.2; Verhalten in der TUI — `TODO: verify` (Phase 5: `internal/tui/box_confirm_delete.go` prüfen)

### Tag management
- Eigene Ansicht zur Tag-Verwaltung; `e` benennt einen Tag um.
- Wie die Ansicht geöffnet wird und was Umbenennen bei Items bewirkt. (TODO: verify)
- Quelle: FACTS.md §8.2, §8.3; Zugang und Wirkung — `TODO: verify` (Phase 5: `internal/tui/view_tag_management.go` prüfen)

### Limits
- Eine Änderung nur des `type` prüft die Eltern-/Kind-Ränge nicht erneut; gleiches Verhalten wie `lif update -t` (bekannter Fehler).
- Die TUI schreibt keine Mehrfachänderungen über mehrere Items. (TODO: verify)
- Quelle: FACTS.md §3.1; DESIGN.md D43; Mehrfachänderungen — `TODO: verify` (Phase 5: TUI prüfen)
