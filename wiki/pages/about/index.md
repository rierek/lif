---
title: "About"
description: "What Lif is, the philosophy behind it and where it comes from."
---

# About

## Zweck
Eine Seite, die erklärt, was Lif ist, welche Haltung dahintersteht und woher das Projekt kommt. Philosophy und Origin sind Abschnitte dieser Seite, keine eigenen Seiten.

## Zielgruppe
Leser, die verstehen wollen, warum Lif so gebaut ist und wie es zu Beans steht; Menschen, die Herkunft und Lizenz prüfen; Anwender mit Altbestand (`.beans`).

## Pflichtabschnitte

### What Lif is
- Kurzbeschreibung: leichtgewichtige, Markdown-native Arbeitsverwaltung für Menschen, KI-Agenten und Automatisierung.
- Drei Zugänge zu einer Wahrheit: Markdown-Dateien, CLI mit `lif prime` und Hilfe, JSON und GraphQL.
- Verweist für Details auf Lif in brief (`overview`) und Features (`features`).
- Quelle: FACTS.md §2.1, §1.1 (prime), §7; Selbstbeschreibung und Positionierung `TODO: verify` — `README.md` im Checkout prüfen

### Philosophy
- Passt sich bestehenden Arbeitsweisen an, statt eine vorzugeben; Terminologie ist konfigurierbar.
- Lokal, menschenlesbar, diff-fähig, LLM-freundlich.
- Presets sind Beispiele, keine Workflow-Engine; Presets nutzen bis zu vier Ränge, eigene Typen können tiefer gehen (D39). Der Trial-Satz "four fixed levels" ist überholt.
- Dateien sind die Quelle der Wahrheit; mehrere Oberflächen (CLI, TUI, Web) arbeiten auf denselben Dateien.
- Quelle: FACTS.md §9(a), §9(b), §3; DESIGN.md D39; Haltung und Designabsicht `TODO: verify` — keine FACTS; Texte aus `README.md` und `wiki/` im Checkout prüfen

### Origin
- Erklärt: Lif ist die Fortsetzung des Forks `xRiErOS/beans` von `hmans/beans` von Hendrik Mans. (TODO: verify)
- Nennt, was aus dem Upstream stammt: Idee, Markdown-plus-YAML-Modell, erste Befehle, erste GraphQL-Engine. (TODO: verify)
- Nennt Lizenz Apache-2.0 und Attribution; der Fork ist unabhängig und spricht nicht für das Upstream-Projekt. (TODO: verify)
- Erklärt, warum aus `bean` das `item` und aus `beans` Lif wurde. (TODO: verify)
- Quelle: `TODO: verify` — keine FACTS; `wiki/fork-lineage.md`, `LICENSE` und `README.md` im Checkout prüfen

### Legacy compatibility
- Zeigt, was aus Beans-Zeiten weiterläuft: Config `.beans.yml` (nur lesend, wird nie umbenannt), Store `.beans/`, Präfix `beans-`, Umgebungsvariable `BEANS_PATH`.
- Erklärt: `.lif.yml` und `.beans.yml` im selben Verzeichnis ist ein Fehler; `LIF_PATH` und `BEANS_PATH` gleichzeitig ist ein Fehler.
- Erklärt: `lif check` meldet Altnamen als Hinweis (`Legacy names`), sie zählen nie als Fehler.
- Verweist auf die Releases-Seiten für den Umstieg (`releases`).
- Quelle: FACTS.md §2.4, §4.1, §4.2, §6.6

### Where to go next
- Verweist auf Releases (`releases`), Contribute (`contribute`) und Lif in brief (`overview`).
- Quelle: DESIGN.md D35, D46, D48
