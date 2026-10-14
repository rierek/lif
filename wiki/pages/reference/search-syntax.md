---
title: "Search syntax"
description: "Query syntax for lif list --search and the GraphQL search filter."
---

# Search syntax

## Zweck
Nachschlageseite für die Volltext-Query-Syntax von `lif list --search` und dem GraphQL-Filter `ItemFilter.search`: Begriffe, Phrasen, unscharfe Suche, Präfixe, Operatoren und Feldbereiche.

## Zielgruppe
Nutzer und Agenten, die ein Backlog gezielt durchsuchen. Die Seite beschreibt nur, was durch FACTS belegt ist; alles weitere ist als offen markiert.

## Pflichtabschnitte

### Where search applies
- Zeigt `lif list --search <query>` und die Kurzform `-S`: Volltextsuche in Titel und Body.
- Zeigt GraphQL: `ItemFilter.search` (String) in `items(filter)`; Verweis auf `reference/graphql`.
- Erklärt: die Syntax ist die Query-String-Syntax von Bleve, wie sie der Hilfetext von `lif list` nennt; dass `ItemFilter.search` dieselbe Syntax nutzt, ist nicht belegt (TODO: verify)
- Hinweis: Im Flag-Verzeichnis von FACTS hat nur `lif list` ein `--search`; GraphQL nutzt `ItemFilter.search`.
- Quelle: FACTS.md §1.1 (`lif list`), §7.4; `TODO: verify` für die Syntax von `ItemFilter.search`: `pkg/itemgraph/filters.go` ansehen

### Terms and phrases
- Zeigt einen einzelnen Begriff und mehrere Begriffe (Verknüpfung ohne Operator → TODO: verify).
- Zeigt Phrasensuche in doppelten Anführungszeichen, z. B. `"exact phrase"`.
- Hinweis auf Shell-Quoting der Anführungszeichen in Beispielen.
- Quelle: FACTS.md §1.1 (`lif list`: term, "phrase"); `TODO: verify` für die Verknüpfung mehrerer Begriffe ohne Operator: Bleve-Dokumentation und Testlauf in Phase 5

### Fuzzy, prefix and wildcard
- Zeigt unscharfe Suche `term~` und mit Editierabstand `term~2`.
- Zeigt Präfix `prefix*`.
- Allgemeine Wildcards außerhalb des Präfixes (TODO: verify)
- Quelle: FACTS.md §1.1 (`lif list`: `term~`, `term~2`, `prefix*`); `TODO: verify` für weitere Wildcards: Bleve-Dokumentation und Testlauf in Phase 5

### Boolean operators
- Zeigt `AND` und `OR` in Großbuchstaben.
- Zeigt Gruppierung mit Klammern (TODO: verify)
- Quelle: FACTS.md §1.1 (`lif list`: AND/OR); `TODO: verify` für Klammern: Bleve-Dokumentation und Testlauf in Phase 5

### Field scopes
- Zeigt `slug:`, `title:` und `body:` als Feldbereiche, z. B. `title:release`.
- Erklärt den Unterschied: ohne Feldangabe Suche in Titel und Body (Flag-Hilfe: "Full-text search in title and body").
- Weitere durchsuchbare Felder (Tags, Extra-Schlüssel) sind nicht belegt (TODO: verify)
- Quelle: FACTS.md §1.1 (`lif list`); `TODO: verify` für weitere Felder: Index-Definition im Code von `lif list` ansehen

### Combining with filters
- Zeigt `--search` zusammen mit Filter-Flags von `lif list`: `--status`, `--type`, `--priority`, `--tag`, `--parent`, `--ready`, `--is-blocked`, `--unblocked`, `--where`.
- Erklärt die Verknüpfung der Filter untereinander nur dort, wo belegt: `--tag` ODER innerhalb des Flags, `--where` mehrfach UND.
- Erklärt GraphQL: `search` steht neben `status`, `type`, `tags` usw. im selben `ItemFilter`.
- Ob Suche und Filter streng UND-verknüpft sind (TODO: verify)
- Quelle: FACTS.md §1.1 (`lif list`), §7.4; `TODO: verify` für die Verknüpfung von Suche und Filtern: `internal/commands/list.go` und `pkg/itemgraph/filters.go` ansehen

### Examples
- Zeigt vier bis sechs echte Aufrufe mit Ausgabe (Begriff, Phrase, Fuzzy, Präfix, Feldbereich, Operator, Kombination mit `--status`) (TODO: verify)
- Zeigt dasselbe über `lif graphql` mit `ItemFilter.search` (TODO: verify)
- Quelle: `TODO: verify` — Beispiele in Phase 5 gegen einen echten Store laufen lassen; FACTS enthält keine Beispielaufrufe

### Limits
- Negation (`-term`, `NOT`) (TODO: verify)
- Gewichtung (`term^2`) (TODO: verify)
- Groß-/Kleinschreibung und Umgang mit Bindestrichen oder Umlauten (TODO: verify)
- Hinweis: Filter nach `status`, `type`, `priority` und `tag` laufen über eigene Flags von `lif list`, nicht über die Query.
- Quelle: `TODO: verify` — FACTS nennt nur die in "Fuzzy, prefix and wildcard", "Boolean operators" und "Field scopes" genannten Elemente; Bleve-Query-String-Dokumentation und Testlauf in Phase 5
