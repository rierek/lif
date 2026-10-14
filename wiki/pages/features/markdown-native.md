---
title: "Markdown-native"
description: "Items are plain Markdown files you can read, diff and edit anywhere."
---

# Markdown-native

## Zweck
Erklärt, dass jedes Item eine einfache Markdown-Datei ist und was das für Besitz, Lesbarkeit und Bearbeitung bedeutet. Enthält zwei eingeklappte Anleitungen (Attachments, Links aus READMEs) und benennt ehrlich, was bei Handbearbeitung ungeprüft bleibt.

## Zielgruppe
Menschen, die wissen wollen, ob ihre Daten offen und werkzeugunabhängig bleiben, und Agents, die Item-Dateien direkt lesen oder schreiben. Die Seite beantwortet "Was steht in der Datei?" und "Was darf ich von Hand ändern?".

## Pflichtabschnitte

### What
- Ein Item ist genau eine Markdown-Datei: `---`, YAML Front Matter, `---`, danach der Markdown-Body.
- Dateiname `<id>--<slug>.md`; ID und Slug stehen nicht im Front Matter, sondern werden aus dem Dateinamen abgeleitet.
- Von Lif geschriebene Felder in fester Reihenfolge: `title`, `status`, `type`, `priority`, `tags`, `created_at`, `updated_at`, `order`, `parent`, `blocking`, `blocked_by`; erste Front-Matter-Zeile ist ein YAML-Kommentar `# <id>`.
- Unbekannte Schlüssel bleiben erhalten (extra keys) und werden alphabetisch hinter den bekannten Feldern geschrieben.
- Erste Verwendung des Begriffs: Liff = die Markdown-Datei eines Items; Definition steht auf `reference/item-file-format` und `reference/glossary`, hier nur kurz nennen und verweisen.
- Quelle: FACTS.md §2.1, §2.2, §2.4

### Why
- Nutzer behalten Besitz und Kontrolle über ihre Informationen (Kernnutzen laut Konzept).
- Menschenlesbar, diffbar, in jedem Editor änderbar; kein Datenbank-Export nötig.
- Offenes Format: Skripte und Agents lesen die Dateien ohne Lif.
- Quelle: CONCEPT.md §4.1

### How it looks
- Annotierte Beispiel-Datei: Dateiname, Front Matter, Body; Pfeile oder Nummern erklären jedes Feld.
- Zeigt einen extra key (zum Beispiel `commit`) und seinen Platz hinter den bekannten Feldern.
- Beispiel in Phase 5 mit `lif create` erzeugen und die Datei unverändert einfügen (TODO: verify).
- Optional Verweis auf `lif show` für dieselbe Ansicht im Terminal.
- Quelle: FACTS.md §2.1, §2.2, §2.4; (TODO: verify) für das konkrete Beispiel: in Phase 5 mit `lif init` und `lif create` bauen

### Attachments
- Anleitung: Pro Item gibt es ein Verzeichnis `attachments/<item-id>/` im Store für Belege und Artefakte.
- Beim Laden überspringt Lif das Verzeichnis `attachments/`; es werden dort keine Items gesucht.
- `lif check` meldet Verzeichnisse ohne zugehöriges Item als Problem (`attachments/<id> has no item <id>`, Exit 1); Items im Archiv gelten weiter als vorhanden.
- Wann und wie man aus dem Body auf eine Anlage verweist (Design-Doku, Entscheidung, Review-Artefakt, Screenshot): Konvention aus dem Entwurf (TODO: verify)
- Wie Anlagen in die Item-Datei verlinkt werden und ob `lif` sie anlegt oder nur prüft (TODO: verify)
- Quelle: FACTS.md §2.4, §6.4; (TODO: verify) für Anlagen-Workflow: `lif prime` Template (`internal/commands/prompt.tmpl`) und `wiki/` im Checkout prüfen

### Link items from READMEs
- Anleitung: README als Einstieg, Items als Arbeitsstand; Status nicht in der README duplizieren.
- Roadmap als Markdown erzeugen: `lif roadmap --format markdown`, Optionen `--no-links`, `--link-prefix`, `--depth`, `--tags`, `--include-done`.
- Zeigt ein kurzes Roadmap-Snippet aus echter Ausgabe (in Phase 5 erzeugen).
- Beispielabschnitt für die README, Badge und Verweis auf Docs-Site oder `lif prime`: im Entwurf nur angedeutet (TODO: verify)
- Quelle: FACTS.md §1.1 (`lif roadmap`); (TODO: verify) für Beispielabschnitt und Badge: Trial-Seite und README im Checkout prüfen, Snippet mit `lif roadmap` real laufen lassen

### Limits
- Handbearbeitung wird beim Laden nicht geprüft: kein Check von Hierarchie, Status oder Priorität.
- Beim Schreiben über Lif gelten Grenzen: Body höchstens 1 MiB, Titel höchstens 1000 Zeichen, Tags nach Muster (klein, mit Buchstabe beginnend, Bindestriche nur zwischen Zeichen).
- Eine nicht parsbare `.md`-Datei bricht das gesamte Laden ab (`loading <path>: ...`); Lif liefert dann keine Items.
- `lif check` meldet weder abweichenden Dateinamen noch doppelte IDs; bei doppelter ID überschreibt die später geladene Datei still.
- Obsidian-Bases-Kompatibilität des Formats ist ungeklärt; nichts im Code oder in der Doku sagt etwas dazu.
- Quelle: FACTS.md §2.1, §2.2, §2.4, §3.2, §6.8, §6.9, §9(g)

### Reference links
- Item file format (`reference/item-file-format`): alle Felder, Werte, Dateinamen, Handbearbeitung.
- Configuration (`reference/configuration`): Store-Pfad, ID-Präfix, Typen, Status.
- `lif check` (`reference/commands/lif-check`): was geprüft wird und was nicht.
- `lif roadmap` (`reference/commands/lif-roadmap`): Markdown-Ausgabe und Link-Optionen.
- Glossary (`reference/glossary`): Begriff Liff.
- Quelle: DESIGN.md D26, D47 und Open (Page-Cut Phase 4)
