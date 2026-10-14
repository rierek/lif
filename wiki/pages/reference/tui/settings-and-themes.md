---
title: "Settings and themes"
description: "TUI settings file, editor, accent colour and themes."
---

# Settings and themes

## Zweck
Beschreibt die Einstellungen der TUI (Settings-Datei, Editor, Akzentfarbe, Layout) und die Themes, die TUI und CLI gemeinsam nutzen.

## Zielgruppe
Nutzer, die die TUI anpassen (Repositories, Editor, Farben), und Leser, die wissen wollen, welche Einstellungen in `.lif.yml` und welche in der TUI-Datei stehen.

## Pflichtabschnitte

### Settings file
- Pfade: `~/.config/lif/tui/config.yaml` (Einstellungen) und `state.json` im selben Verzeichnis (Zustand).
- Tabelle der Schlüssel: `repos` (Liste von Repository-Pfaden), `editor`, `theme.accent` (Hex `#rrggbb`, leer = eingebautes Mauve), `layout.tree_width` (Standard 36, Minimum 24, Maximum 60), `repo_store_paths` (Abbildung Repository auf Store-Pfad).
- Abgrenzung: Diese Datei gehört der TUI; Projekteinstellungen stehen in `.lif.yml` (`reference/configuration`).
- Es gibt ein Einstellungsformular in der TUI. (Zugang und Taste: TODO: verify)
- Quelle: FACTS.md §8.2; Einstellungsformular — `TODO: verify` (Phase 5: `internal/tui/box_form_settings.go` und Befehlspalette prüfen)

### Themes
- `display.theme` in `.lif.yml`: `latte`, `frappe`, `macchiato`, `mocha`; Standard `mocha`.
- Das Theme gilt auch für die Farben der CLI; ein unbekannter Name wird von `lif check` gemeldet.
- Zeigt die vier Themes nebeneinander (Screenshots). (TODO: verify)
- Abgrenzung: `display.theme` (global, alle Oberflächen) vs `theme.accent` (nur TUI).
- Quelle: FACTS.md §8.4, §4.3; Screenshots — `TODO: verify` (Phase 5: aufnehmen)

### Display width
- `display.max_width`: 0 = nicht gesetzt = 110, `-1` hebt die Begrenzung auf; das Flag `--max-width` überschreibt pro Befehl.
- Klärt, ob `display.max_width` die TUI beeinflusst. (TODO: verify)
- Quelle: FACTS.md §4.3; Wirkung auf die TUI — `TODO: verify` (Phase 5: Code prüfen, wo `max_width` gelesen wird)

### Choosing the editor
- Schlüssel `editor` in der Settings-Datei; Befehl `e`/`ctrl+e` öffnet den Body im Editor.
- Erklärt, ob `editor` Vorrang vor `$EDITOR` hat und was passiert, wenn keiner gesetzt ist. (TODO: verify)
- Zeigt Beispiel mit einem Terminal- und einem GUI-Editor. (TODO: verify)
- Quelle: FACTS.md §8.2, §8.3; Reihenfolge und Beispiele — `TODO: verify` (Phase 5: `internal/tui/editor.go` prüfen)

### Example settings file
- Zeigt eine vollständige `config.yaml` mit `repos`, `editor`, `theme.accent`, `layout.tree_width`.
- Das Beispiel wird in Phase 5 gebaut und mit `lif tui` geprüft, nicht erfunden.
- Quelle: `TODO: verify` — FACTS nennt Schlüssel, aber kein Beispiel; in Phase 5 eine Datei anlegen und die TUI starten

### Limits
- Settings gelten pro Benutzer, nicht pro Repository.
- Ungültige Werte (z. B. Breite außerhalb 24–60, ungültige Hex-Farbe): Verhalten nicht belegt. (TODO: verify)
- Die Themes sind auf die vier Werte begrenzt; eigene Themes gibt es nicht. (TODO: verify)
- Quelle: FACTS.md §8.2, §8.4; Fehlerverhalten und eigene Themes — `TODO: verify` (Phase 5: `internal/tui/config/settings.go` prüfen)
