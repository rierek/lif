---
title: "Start and repositories"
description: "Start lif tui and choose the repository to work in."
---

# Start and repositories

## Zweck
Erklärt, wie die TUI gestartet wird, welches Repository sie öffnet und wie man zwischen Repositories wechselt. Einstieg in den TUI-Unterbereich der Reference.

## Zielgruppe
Nutzer, die `lif tui` zum ersten Mal starten oder mehrere Projekte in einer Oberfläche verwalten wollen.

## Pflichtabschnitte

### Starting the TUI
- Zeigt `lif tui [repo-path]`: höchstens ein Argument (Repo-Pfad, Verzeichnis-Vervollständigung); keine eigenen Flags außer den globalen.
- Erklärt: `lif tui` ist die TUI im `lif`-Binary (das frühere eigene Binary `lif-tui` gibt es nicht mehr); sie läuft auch ohne geladenen Store.
- Nennt `--lif-path` als globales Flag, das den Store des geöffneten Repositories bestimmt.
- Beschreibt die TUI als Begleiter der CLI (kurzer Satz, Verweis auf `reference/commands/lif-tui`).
- Quelle: FACTS.md §1.0, §1.1 (`lif tui`), §8.1, §8.2

### Which repository opens
- Reihenfolge als nummerierte Liste: 1) ausdrücklicher Pfad-Argument, 2) Repository, das aufwärts vom aktuellen Verzeichnis gefunden wird, 3) ohne Repository und mit mindestens zwei Einträgen in `repos` der Settings-Datei der Repository-Picker (Lobby), 4) sonst die Fehlermeldung "nicht gefunden".
- Zeigt Beispielaufrufe für jeden Fall.
- Hinweis: Mit genau einem Repository in den Settings und außerhalb eines Repositories erscheint keine Lobby (Fehler).
- Quelle: FACTS.md §8.1, §1.1 (`lif tui`)

### The repository picker
- Erklärt die Lobby als Liste der Repositories aus der Settings-Datei; Auswahl öffnet das Repository.
- Taste `p` öffnet den Picker auch aus einem geöffneten Repository (Tastenkürzel siehe `reference/tui/key-bindings`).
- Beschreibt Voraussetzung: mindestens zwei Repositories in `repos`.
- Screenshot `lobby.png` (siehe Abschnitt "Screenshots").
- Quelle: FACTS.md §8.1, §8.2, §8.3

### Settings that matter at start
- Nennt `repos` (Liste von Repository-Pfaden) und `repo_store_paths` (Abbildung Repository auf Store-Pfad) in `~/.config/lif/tui/config.yaml`.
- Nennt `--lif-path` als Weg, den Store einmalig vorzugeben.
- Verweist auf `reference/tui/settings-and-themes` für alle Schlüssel.
- Zeigt, wie ein Repository in `repos` aufgenommen wird. (TODO: verify)
- Quelle: FACTS.md §8.2, §1.1 (`lif tui`); Aufnahme neuer Repositories — `TODO: verify` (Phase 5: prüfen, ob die TUI `repos` selbst schreibt, `internal/tui/config/settings.go`)

### Quit and help
- `q` und `ctrl+c` beenden nach Bestätigung; `esc` geht zurück.
- `?` öffnet die Hilfe; sie wird aus derselben Tastenbelegung (`keyMap`) erzeugt wie die Seite `reference/tui/key-bindings`.
- `K` öffnet die Befehlspalette (Command Palette).
- Quelle: FACTS.md §8.3, §8.2

### Screenshots
- Plan: vorhandenes Bild `wiki/screenshots/lobby.png` (Lobby) einbinden; weitere Bilder auf den Seiten `reference/tui/views`.
- Medienart laut Design: VHS-GIF für den Start (`lif tui`), Screenshots als webp.
- Bilder prüfen, ob sie zur aktuellen Oberfläche passen (neu aufnehmen, falls nicht). (TODO: verify)
- Quelle: DESIGN.md D38, D44; Aktualität der Bilder — `TODO: verify` (Phase 5: mit laufender TUI vergleichen)
