---
title: "Blog"
description: "Experiments, integrations and field notes around Lif."
---

# Blog

## Zweck
Übersicht über Blog-Beiträge: Erfahrungen, Experimente, Integrationen und Erweiterungen rund um Lif, ohne feste Erzählstruktur.

## Zielgruppe
Leserinnen, die über den Kernumfang hinausschauen wollen, und Autoren, die entscheiden, ob ein Text ins Blog oder in Use cases gehört.

## Pflichtabschnitte

### What belongs on the blog
- Erklärt: Das Blog teilt eine konkrete Erfahrung, ein Experiment, eine Integration oder Erweiterung; Use cases zeigen dagegen allgemein anwendbare Lösungen.
- Beispiel der Abgrenzung: Use case "Managing a Software Project with Lif" gegenüber Blog "Wie ich Items mit Git-Worktrees verbinde".
- Regel: Blog-Beiträge verlinken die Reference, statt technische Definitionen zu wiederholen.
- Mögliche Themen laut Seed: Git-Worktree-Integration, Rückführbarkeit Item zu Umsetzung, Release-Workflow, eigene Skripte, Obsidian.
- Quelle: CONCEPT.md §6 (Purpose, Potential Articles, Editorial Distinction); DESIGN.md D34, D42

### Post list
- Zeigt die Beitragsliste chronologisch, neueste zuerst, mit Titel, Datum, Tags und Kurzbeschreibung.
- Aktueller Bestand: Managing Lif items in Obsidian (`blog/obsidian-experiment`).
- Listenmechanismus offen: Das Starlight-Blog-Plugin aus D34 ist nicht installiert. (TODO: verify)
- Quelle: DESIGN.md D34; Mechanismus `TODO: verify` — `site/` (Plugins in `astro.config.mjs`) in Phase 5 prüfen

### Tags and the experiment tag
- Erklärt: Beiträge tragen Tags; experimentelle Beiträge tragen den Tag `experiment`.
- Zeigt die Tag-Liste mit Anzahl, sobald mehr als ein Beitrag existiert.
- Erklärt, was der Tag `experiment` bedeutet: ausprobiert, nicht als unterstützte Integration zugesagt.
- Quelle: DESIGN.md D34, D42; Tag-Mechanik des Plugins `TODO: verify`

### Notice box for experimental posts
- Zeigt den Hinweistext, den jeder Beitrag mit Tag `experiment` am Anfang trägt.
- Erklärt Zweck: Leser erkennen, dass Verhalten nicht von Lif zugesichert ist und Referenzseiten Vorrang haben.
- Bestimmt, wie die Box umgesetzt wird (Plugin-Funktion oder Starlight-Aside). (TODO: verify)
- Quelle: DESIGN.md D34, D42; Umsetzung `TODO: verify` — in Phase 5 mit Seiten-Generator und Plugin abstimmen
