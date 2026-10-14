---
title: "Managing Lif items in Obsidian"
description: "An experiment: browsing and editing Lif items in an Obsidian vault."
---

# Managing Lif items in Obsidian

## Zweck
Platzhalter für einen Experiment-Beitrag (Tag `experiment`): Lif-Items in einem Obsidian-Vault ansehen und bearbeiten. Der Beitrag behauptet nichts über Obsidian-Verhalten, das nicht getestet wurde.

## Zielgruppe
Obsidian-Nutzer, die Lif-Dateien mit einem Markdown-Editor mit Properties verwalten wollen; Leser, die den Stand des Experiments einschätzen.

## Pflichtabschnitte

### Notice
- Hinweisbox am Anfang: experimentell, keine unterstützte Integration, Ergebnisse können sich ändern.
- Verweist auf die Reference als verbindliche Quelle für das Item-Format (`reference/item-file-format`).
- Tag `experiment`.
- Quelle: DESIGN.md D34, D42

### The idea
- Erklärt die Idee: Items sind Markdown-Dateien mit YAML-Front-Matter, also Dateien, die ein Obsidian-Vault grundsätzlich öffnen kann. (TODO: verify)
- Nennt das Ziel: Items über Obsidian lesen, ordnen und bearbeiten, Lif-Befehle für Validierung nutzen.
- Quelle: CONCEPT.md §5.6 (Content); Obsidian-Verhalten `TODO: verify` — FACTS §9(g): nicht gefunden, im Experiment prüfen

### Setup
- Beschreibt, wie der Store (`.lif/`) im Vault erscheint; `.lif/` ist ein Dot-Verzeichnis, Verhalten von Obsidian damit ist ungeprüft. (TODO: verify)
- Zeigt Varianten: Vault auf Repository-Wurzel, Symlink oder Vault im Store (Wahl wird im Experiment getestet).
- Nennt Voraussetzungen: installiertes Lif (`overview/installation`), Store vorhanden.
- Quelle: FACTS.md §9(g), §2.4; Setup-Schritte `TODO: verify` — im Experiment durchführen

### What the format offers an Obsidian-like tool
- Zeigt die Formatfakten: eine `.md`-Datei je Item; YAML-Front-Matter mit Skalarfeldern (`title`, `status`, `type`, `priority`, `created_at`, `updated_at`, `order`, `parent`) und Listenfeldern (`tags`, `blocking`, `blocked_by`).
- Unbekannte Front-Matter-Schlüssel bleiben erhalten.
- Erste Zeile im Front-Matter ist ein YAML-Kommentar `# <id>`; ID und Slug stammen aus dem Dateinamen, nicht aus dem Front-Matter.
- Dateiname: `<id>--<slug>.md`.
- Quelle: FACTS.md §2.1, §2.2, §2.4, §9(g)

### Obsidian Bases
- Erklärt: Ob Obsidian Bases mit dem Item-Format arbeiten, ist nicht verifiziert.
- Im Code und in der bisherigen Dokumentation gibt es keine Erwähnung von Obsidian oder Bases und keine `.base`-Datei.
- Zeigt nach dem Experiment: eine Base-Ansicht über Items, falls sie funktioniert. (TODO: verify)
- Quelle: FACTS.md §9(g); Kompatibilität `TODO: verify` — im Experiment prüfen, bis dahin keine Aussage

### Validating changes with lif check
- Erklärt: `lif check` nach Änderungen in Obsidian ausführen; Exit-Code 1 bei Problemen.
- Listet, was es erkennt: unbekannter Item-Typ, defekte Links, Selbstreferenzen, Zyklen (parent, blocking, blocked_by), fehlende Pflichtfelder (Warnung), unbekannte SHAs (Warnung), verwaiste Attachment-Verzeichnisse.
- Listet, was es nicht erkennt: Eltern-/Kind-Ränge, Statuswerte, Prioritätswerte, Tag-Format, doppelte IDs, Front-Matter-Syntax über das Laden hinaus.
- Hinweis: Ein Parse-Fehler in einer Datei bricht das Laden ab.
- Quelle: FACTS.md §3.2, §6.9, §9(e), §2.4

### Limits
- Handbearbeitung umgeht die Hierarchieregeln der CLI; `lif check` meldet Rangverstöße nicht.
- Dateiname trägt die ID; Umbenennen in Obsidian kann die ID ändern. (TODO: verify)
- Blocking-Links liegen nur auf einer Seite; `lif check` meldet keine Asymmetrie.
- Obsidian-Verhalten (Properties-Ansicht, Bases, Dot-Verzeichnis) ist nicht belegt.
- Quelle: FACTS.md §3.2, §5.6, §2.1, §9(d), §9(g); Umbenennen `TODO: verify` — im Experiment prüfen

### Reference links
- Item file format (`reference/item-file-format`), Configuration (`reference/configuration`), `lif check` (`reference/commands`).
- Features: Markdown-native (`features/markdown-native`), Surface independence (`features/surface-independence`).
- Blog-Übersicht (`blog`).
- Quelle: DESIGN.md D34, D42
