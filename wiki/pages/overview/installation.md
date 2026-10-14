---
title: "Installation"
description: "Install Lif and check that it works."
---

# Installation

## Zweck
Ziel des Hero-Buttons auf der Startseite: Lif installieren und mit einem ersten Befehl prüfen, dass es läuft. Zeigt nur den Installationsweg, keine Konfiguration.

## Zielgruppe
Neue Nutzer, die sich für Lif entschieden haben und möglichst schnell ein lauffähiges `lif` wollen, plus Agenten, die Installationsschritte nachlesen.

## Pflichtabschnitte

### Requirements
- Unterstützte Betriebssysteme und Architekturen nennen (Tabelle).
- Voraussetzungen: Git empfohlen, Go nur für Build aus Quellen, Node nur für das Frontend (TODO: verify).
- Hinweis: `lif` selbst braucht keinen laufenden Server (siehe `overview/index`).
- Quelle: `TODO: verify` — FACTS.md hat keinen Installationsabschnitt; prüfen in `.goreleaser.yaml` (Build-Matrix), `go.mod` (Go-Version), `README.md`, `wiki/installation.md`.

### Install methods
- Vier Wege als Tabs oder Unterabschnitt-Liste: Installationsskript · Paketmanager · Release-Download · Build aus Quellen.
- Je Weg ein kopierbarer Befehl und eine Zeile "wann nehmen" (Befehle erst in Phase 5 verifizieren).
- Hinweis: macOS-Quarantäne bei Homebrew-Installation wird per `xattr` behandelt (TODO: verify; FACTS erwähnt nur den Post-Install-Hook).
- Hinweis zu `just install` als Entwicklerweg und seinem Schutz (`LIF_INSTALL_FORCE`) gehört auf die Seite Contribute, nicht hierher.
- Quelle: `TODO: verify` — Skript-URL, Paketmanager-Namen und Release-Artefakte fehlen in FACTS; prüfen in `install.sh`, `.goreleaser.yaml` (Homebrew-Abschnitt), `README.md`, `wiki/installation.md`. FACTS.md §6.10 belegt nur: goreleaser-Homebrew-Hook mit `xattr`, `scripts/install-guard.sh` mit `LIF_INSTALL_FORCE=1`.

### The binaries
- `lif`: CLI und TUI (`lif tui`); die TUI ist kein eigenes Binary mehr.
- `lif-serve`: separates Binary für den Webserver; ohne Argument startet es `serve`.
- Der Befehl `lif serve` im Binary `lif` ist ein versteckter Stub und nicht dokumentiert; er verweist auf `lif-serve`.
- Tabelle: Binary · Zweck · Standard-Port (8080 für `lif-serve`).
- Quelle: FACTS.md §1.0, §1.1 (`lif tui`, `lif serve` stub, `lif-serve`), §4.3

### Verify the installation
- Zeigt `lif version` mit Beispielausgabe; `--json` für strukturierte Ausgabe.
- Erklärt: Ein Entwicklungs-Build meldet die Version `dev`.
- Zeigt `lif --help`: 28 Befehle sind aufgelistet (Zahl in Phase 5 neu zählen).
- Erster sinnvoller Befehl: `lif init` im Projektordner; Verweis auf First steps (`overview/first-steps`).
- Quelle: FACTS.md §1.1 (`version`, Command count), §1.0 (Bootstrap-Skip: `version` läuft ohne Store)

### Shell completion
- Zeigt `lif completion bash|zsh|fish|powershell` (eine gemeinsame Befehlsseite, D25).
- Nennt Item-ID-Vervollständigung als Nutzen.
- Optional: zsh-Widget `Alt+i` für `lif pick` (extras, TODO: verify Installationsweg).
- Quelle: FACTS.md §1.1 (Cobra built-ins, `lif pick`); DESIGN.md D25. Der Installationsweg des zsh-Widgets ist `TODO: verify` (prüfen in `extras/zsh/lif-pick.zsh`).

### Update and uninstall
- Update je Installationsweg (Skript erneut, Paketmanager-Upgrade, neuer Download).
- Deinstallation: Binaries entfernen; Stores im Projekt (`.lif/`, `.lif.yml`) bleiben als normale Dateien erhalten.
- Hinweis auf Upgrade-Kompatibilität alter `.beans`-Stores (Verweis nur als Titel, Quelle prüfen).
- Quelle: `TODO: verify` — FACTS.md enthält weder Update- noch Deinstallationswege; prüfen in `install.sh`, `.goreleaser.yaml`, `wiki/installation.md`, `wiki/compatibility-and-upgrading.md`. Legacy-Lesen von `.beans/`: FACTS.md §2.4, §4.1.

### Next step
- Verweis auf First steps (`overview/first-steps`): Store anlegen, Preset wählen, erste Items.
- Verweis auf Concepts (`overview/concepts`) und, für Agenten, Agent onboarding (`overview/agent-onboarding`).
- Quelle: DESIGN.md D37; CONCEPT.md §3.1
