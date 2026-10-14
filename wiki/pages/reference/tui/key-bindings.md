---
title: "Key bindings"
description: "All TUI key bindings, generated from the key map."
---

# Key bindings

## Zweck
Vollständige Liste aller Tastenbelegungen der TUI. Die finale Seite wird aus `internal/tui/keymap.go` erzeugt und ist keine handgepflegte Tabelle.

## Zielgruppe
Nutzer der TUI, die eine Taste nachschlagen, und Dokumentationsautoren, die die Erzeugung pflegen.

## Pflichtabschnitte

### How to read this page
- Erklärt: Quelle ist `keyMap` in `internal/tui/keymap.go`; die In-App-Hilfe (`?`) wird aus derselben Tabelle erzeugt, daher stimmen Seite und Hilfe überein.
- Erklärt die Notation (`ctrl+x`, `shift+tab`, mehrere Tasten pro Aktion).
- Hinweis: Navigation mit `i/j/k/l`-Raute, nicht vim `h/j/k/l`.
- Tabellenaufbau: Taste, Aktion, Kontext (wo sie wirkt).
- Quelle: FACTS.md §8.3; DESIGN.md D44

### Navigation
- Hoch `↑`/`i`, runter `↓`/`k`, links (zurück/hinaus) `←`/`j`, rechts (hinein/aufklappen) `→`/`l`.
- `tab` Fokus hinein (Baum zu Detail), `shift+tab` Fokus hinaus.
- `enter` öffnen/bestätigen, `esc` zurück, `[`/`ctrl+left` Verlauf zurück, `]`/`ctrl+right` Verlauf vor.
- Quelle: FACTS.md §8.3

### Global actions
- `K` Befehlspalette, `p` Repository-Picker, `b` Backlog, `/` Suche, `f` Facetten-Filter, `X` Filter löschen, `S` Backlog-Sortierung wechseln, `G` verschachtelt/flach, `ctrl+r` neu laden.
- `v` Vollbild, `1`..`9` zu Akkordeon-Abschnitt springen, `y` Kontext kopieren (OSC52 und nativ).
- `q`/`ctrl+c` beenden (mit Bestätigung), `?` Hilfe.
- Quelle: FACTS.md §8.3

### Item actions
- `s` Status-Menü, `o` Typ-Menü, `u` Prioritäts-Menü, `a` Parent zuweisen, `r` Relationen (Blocking-Picker), `c` anlegen, `d` löschen (Bestätigung, kein Kaskadieren), `t` Tag-Picker, `e`/`ctrl+e` Body im `$EDITOR`.
- Verweist auf `reference/tui/editing-items` für das Verhalten.
- Quelle: FACTS.md §8.3

### Pickers and facets
- `space`/`x` Facette umschalten; Chips im Item-Picker: `ctrl+t` Typ, `ctrl+n` Status, `ctrl+p` Priorität, `ctrl+g` Tags.
- Zeigt, in welchen Pickern die Chips verfügbar sind. (TODO: verify)
- Quelle: FACTS.md §8.3; Verfügbarkeit je Picker — `TODO: verify` (Phase 5: `internal/tui/keymap.go` und Picker-Dateien prüfen)

### Tag picker and tag management
- Tag-Picker: `n` neuer Tag, `space` Tag umschalten.
- Tag-Verwaltung: `e` Tag umbenennen.
- Quelle: FACTS.md §8.3

### Outside the TUI
- Das zsh-Widget `extras/zsh/lif-pick.zsh` bindet `Alt+i` (`^[i`) an `lif pick`; gehört nicht zur TUI.
- Zeigt, wie man das Widget einbindet. (TODO: verify)
- Verweist auf die Befehlsseite `lif pick` (`reference/commands/lif-pick`).
- Quelle: FACTS.md §8.3, §1.1 (`lif pick`); Einbinden des Widgets — `TODO: verify` (Phase 5: `extras/zsh/lif-pick.zsh` lesen)

### Generation
- Beschreibt: Der Generator (`cmd/lif-docgen` oder eine Erweiterung) liest `keyMap` und schreibt die Tabellen in diese Seite; keine von Hand kopierte Tabelle in der finalen Seite.
- Hinweis für Phase 5: Generator-Erweiterung ist noch zu bauen; bis dahin bleibt die Seite ein Vertrag.
- Beschreibt Prüfung: Ein Test oder CI-Schritt stellt sicher, dass die Seite nicht von `keymap.go` abweicht. (TODO: verify)
- Quelle: DESIGN.md D24, D44, FACTS.md §8.3; Generator und Prüfung — `TODO: verify` (Phase 5: Generator unter `cmd/lif-docgen` ansehen)
