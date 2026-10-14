---
title: "Surface independence"
description: "Use the CLI, TUI, web UI or any editor on the same files."
---

# Surface independence

## Zweck
Zeigt, dass CLI, TUI, Web UI und direktes Bearbeiten der Markdown-Dateien denselben Store nutzen. Klärt, welche Wege Regeln erzwingen und was `lif check` danach abdeckt.

## Zielgruppe
Menschen, die ein Werkzeug nach Geschmack wählen und wechseln wollen, und Teams mit gemischter Nutzung (Terminal, Browser, Editor, Agent). Sie wollen verstehen, ob die Daten bei jedem Weg konsistent bleiben.

## Pflichtabschnitte

### What
- Vier Wege auf einen Store: CLI (`lif`), TUI (`lif tui`), Web UI (eingebettet in das separate Programm `lif-serve`) und direktes Bearbeiten der Markdown-Dateien.
- Alle nutzen denselben Store; Store-Pfad wird nach fester Rangfolge bestimmt (`--lif-path`, Konfiguration, `LIF_PATH`, aktuelles Verzeichnis).
- Der Web-Server beobachtet das Store-Verzeichnis und schickt Änderungen an Abonnenten; das TUI ist Begleiter der CLI.
- Der Server hat weitere Fähigkeiten (Worktrees, Agent-Sitzungen, Terminal), die nur über `lif-serve` verfügbar sind.
- Quelle: FACTS.md §4.2, §8.1, §8.2

### Why
- Kein Lock-in: dieselben Informationen sind über verschiedene Werkzeuge erreichbar.
- Jeder arbeitet in seinem Werkzeug; Agents nutzen CLI oder GraphQL, Menschen Editor, TUI oder Browser.
- Quelle: CONCEPT.md §4.4

### How it looks
- Vierfach-Gegenüberstellung desselben Items: Datei im Editor, `lif show`, TUI-Detailansicht, Web-UI-Ansicht.
- Medienplan: VHS-GIFs für CLI und TUI, Screenshots (webp) für die Web UI.
- Zeigt dieselbe Änderung (zum Beispiel Status) über zwei Wege und die Auswirkung auf die Datei.
- TUI: Taste `e` öffnet den Body im `$EDITOR`.
- Inhalte der Web-UI-Ansicht (Spalten, Board): FACTS enthält dazu nichts (TODO: verify)
- Quelle: DESIGN.md D38; FACTS.md §8.2, §8.3; (TODO: verify) für Web-UI-Ansicht: Screenshot in Phase 5 aufnehmen, `frontend/` im Checkout prüfen

### Consistency and validation
- Schreibende Wege (CLI-Befehle, GraphQL-Mutationen, TUI, Web UI) laufen über dieselbe Item-Logik und erzwingen dieselben konfigurierten Regeln (Rangregel für Parents, gültige Werte für Typ, Status, Priorität).
- Direktes Bearbeiten bleibt uneingeschränkt; Lif lädt die Änderung ohne Prüfung und beobachtet Dateien nur zum Neuladen.
- Danach `lif check` ausführen. Es prüft: Konfiguration, unbekannte Typen, defekte Links, Selbstbezüge, Zyklen, fehlende Pflichtfelder (Warnung), unbekannte Commit-SHAs (Warnung), verwaiste Anhang-Verzeichnisse.
- Es prüft nicht: Rang von Parent und Kind, Statuswerte, Prioritätswerte, Tags, doppelte IDs, Dateiname gegen ID, Syntaxfehler des Front Matters (die brechen schon das Laden ab).
- Gleichzeitige Änderungen: `--if-match` und GraphQL `ifMatch` sichern per ETag; Konfiguration `require_if_match` macht es Pflicht.
- Erweiterung der Prüfung ist als Verbesserung geplant; die Seite folgt, sobald sie ausgeliefert ist (D40).
- Quelle: FACTS.md §2.1, §3.1, §3.2, §6.8, §6.9, §8.2; DESIGN.md D40

### Editors such as Obsidian
- Jeder Markdown-fähige Editor kann Item-Dateien öffnen; das Format ist Front Matter plus Body.
- Obsidian ist ein Experiment und steht im Blog (`blog/obsidian-experiment`), nicht mehr als Use Case.
- Bases-Kompatibilität ist nicht belegt: im Code, in der Doku und im README taucht Obsidian nicht auf.
- Zu beachten: erste Front-Matter-Zeile ist ein YAML-Kommentar `# <id>`; der Store liegt in einem Punkt-Verzeichnis (`.lif/`) (Auswirkung auf Obsidian: TODO: verify)
- Quelle: FACTS.md §2.1, §2.4, §9(g); DESIGN.md D42; (TODO: verify) für Obsidian-Verhalten: im Experiment (Blog) prüfen

### Limits
- Handbearbeitung wird nicht beim Bearbeiten, sondern höchstens nachträglich mit `lif check` geprüft; dessen Lücken sind oben genannt.
- Das Web UI braucht das separate Programm `lif-serve`; `lif serve` im Hauptprogramm ist ein versteckter Stub.
- Ein Store gilt nur für einen Pfad: Ohne `lif.anchor: repo-root` nutzt jeder Git-Worktree seinen eigenen lokalen Store.
- Gleichzeitiges Editieren ohne `--if-match` ist ohne Schutz; ein überschriebener Stand wird nicht gemeldet (TODO: verify)
- GraphQL `createItem` ohne `status` lässt den Status leer (bekannter Fehler); `lif create` setzt `todo`.
- Quelle: FACTS.md §4.2, §7.5, §8.1; DESIGN.md D43; (TODO: verify) für Schreibkonflikte: Verhalten von `Core.Update` ohne ETag prüfen

### Reference links
- TUI: Start and repositories (`reference/tui/start-and-repositories`), Views (`reference/tui/views`), Editing items (`reference/tui/editing-items`).
- Web server (`reference/web-server`): `lif-serve`, Port, CORS.
- `lif check` (`reference/commands/lif-check`): Prüfumfang.
- `lif path` (`reference/commands/lif-path`) und Configuration (`reference/configuration`): Store-Auflösung, `require_if_match`.
- Quelle: DESIGN.md D28, D29, D44
