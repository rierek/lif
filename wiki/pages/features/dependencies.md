---
title: "Dependencies"
description: "Model blockers between items and see what is ready next."
---

# Dependencies

## Zweck
Erklärt, wie Blocker zwischen Items modelliert werden, wie Lif "blockiert" und "bereit" bestimmt und wie man Abhängigkeiten ansieht. Beschreibt genau, wo das Modell Lücken hat (keine Symmetrieprüfung, feste Definition von "erledigt").

## Zielgruppe
Menschen und Agents, die entscheiden wollen, womit sie als Nächstes anfangen, und Planer, die Abhängigkeiten zwischen Epics und Aufgaben pflegen. Sie brauchen die genaue Semantik, bevor sie sich auf `lif next` verlassen.

## Pflichtabschnitte

### What
- Zwei Felder: `blocking` (Items, die dieses Item blockiert) und `blocked_by` (Items, die dieses blockieren); eine Kante wird nur auf einer Seite gespeichert, nämlich am gerade geänderten Item.
- Beim Lesen vereinigt Lif beide Richtungen; Löschen eines Items entfernt alle eingehenden Verweise.
- Blockiert heißt: ein direkter aktiver Blocker oder ein blockierter Vorfahr in der Parent-Kette (explizit gegenüber implizit).
- "Bereit" (ready) = nicht blockiert, Status nicht `in-progress`, `draft` oder ein Archiv-Status, und kein Vorfahr mit Status `completed` oder `scrapped`.
- Befehle: `lif next` (höchste Priorität unter den bereiten Items), `lif list --ready`, `--is-blocked`, `--unblocked`, `lif graph` (Formate dot, ascii, mermaid, json).
- Quelle: FACTS.md §5.1, §5.2, §5.4, §5.5

### Why
- Komplexe Beziehungen werden explizit und handhabbar; Blocker sichtbar, bevor jemand anfängt.
- `lif next` beantwortet "Was jetzt?" auch für Agents ohne eigene Planungslogik.
- Unterstützt strukturierte Abläufe, ohne eine Workflow-Engine zu sein.
- Quelle: CONCEPT.md §4.3; FACTS.md §5.2

### How it looks
- Befehlsschleife: `lif create --blocked-by <id>` oder `lif update A --blocking B`; danach liefert `lif next` den Blocker A, nicht B.
- Beobachtetes Verhalten: `lif update B --blocking A` wird mit Zyklus-Meldung abgelehnt (`adding blocking relationship would create cycle: [B A B]`).
- Gegenüberstellung `lif list --ready`, `--is-blocked`, `--unblocked`; `--ready` und `--is-blocked` schließen sich gegenseitig aus.
- Ein blockierter Parent macht alle Kinder implizit blockiert; Beispielausgabe in Phase 5 erzeugen (TODO: verify).
- `lif graph --format mermaid` mit Beispiel; Option `--depth` und `--relation parent|blocks`.
- Quelle: FACTS.md §5.2, §5.3, §5.5, §5.6; (TODO: verify) für konkrete Ausgaben: in Phase 5 mit `lif list --ready` und `lif graph --format mermaid` laufen lassen

### Limits
- Keine Symmetrieprüfung: `A.blocking=[B]` und `B.blocked_by=[A]` können beide oder nur eine Seite vorliegen; `lif check` meldet die Abweichung nicht.
- "Erledigt" ist fest eingebaut: nur `completed` und `scrapped` lösen einen Blocker auf; `draft`, `todo`, `in-progress` und eigene Archiv-Status blockieren weiter.
- Die Zykluserkennung folgt je einem Verknüpfungstyp; gemischte Zyklen über `blocking` und `blocked_by` erkennt sie möglicherweise nicht (TODO: verify)
- Handbearbeitete Zyklen meldet `lif check`; `lif check --fix` entfernt nur defekte Links und Selbstbezüge, nie Zyklen.
- `lif next` ignoriert die Abhängigkeitsreihenfolge, `lif roadmap` nutzt sie; Container können als bereit zurückgegeben werden.
- Blocker, die nicht im Store existieren, werden ignoriert; ein Blocker blockiert nur nach seinem eigenen Status, nicht nach seinen Eltern.
- Quelle: FACTS.md §5.1, §5.2, §5.3, §5.6, §6.7; (TODO: verify) für gemischte Zyklen: FACTS nennt dies nur als Schlussfolgerung (INFERENCE), in Phase 5 testen

### Reference links
- `lif next` (`reference/commands/lif-next`): Filter, Sortierung, Verhalten ohne Treffer.
- `lif list` (`reference/commands/lif-list`): `--ready`, `--is-blocked`, `--unblocked`, `--has-blocking`.
- `lif graph` (`reference/commands/lif-graph`): Formate und Tiefe.
- `lif check` (`reference/commands/lif-check`): Links und Zyklen prüfen.
- `lif create` und `lif update` (`reference/commands/lif-create`, `reference/commands/lif-update`): Optionen `--blocking`, `--blocked-by`.
- GraphQL (`reference/graphql`): `addBlocking`, `addBlockedBy`, `ItemFilter`.
- Quelle: DESIGN.md D24, D25, D27 und Open (Page-Cut Phase 4: Dependency-Semantik kanonisch hier plus generierte Befehlsseiten)
