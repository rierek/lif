---
title: "GraphQL"
description: "The GraphQL API for items: invoke it, read the schema, run queries and mutations."
---

# GraphQL

## Zweck
Referenz der GraphQL-Schnittstelle für Items: wie man sie aufruft, was das Schema enthält und wie Queries und Mutations aussehen. Die Seite besteht in der finalen Fassung aus handgeschriebener Einführung mit Beispielen plus dem zur Build-Zeit eingebetteten Schema (SDL).

## Zielgruppe
Skript- und Agent-Autoren sowie Entwickler, die Items per JSON statt per Befehlszeile lesen oder ändern wollen, und Betreiber des Webservers, die die HTTP-Schnittstelle nutzen.

## Pflichtabschnitte

### What the API is and when to use it
- Erklärt: GraphQL ist der strukturierte Zugriff auf den Item-Store neben Markdown und CLI (dritter Zugangsweg: JSON + GraphQL).
- Nennt Einsatzfälle: Skripte und Agenten, die genau die Felder brauchen (`items(filter)` statt mehrerer `lif list`-Aufrufe), Beziehungen (`parent`, `children`, `blocking`, `blockedBy`) in einer Anfrage.
- Zeigt die zwei Betriebsarten: in-process per CLI (kein Server nötig) und über HTTP mit `lif-serve`.
- Verweist auf `reference/web-server` und die generierte Befehlsseite `lif graphql` (`reference/commands/lif-graphql`).
- Quelle: FACTS.md §7, §7.5, §1.1 (`lif graphql`); DESIGN.md D27

### Invoking it
- Tabelle oder Liste der zwei Wege: CLI in-process und HTTP.
- CLI: `lif graphql '<query>'`, Alias `lif query`; Query aus dem Argument oder, wenn keines, von stdin (Pipe oder Datei); mehr als ein Argument ist ein Fehler; leere stdin: `no query provided (pass as argument or pipe to stdin)`.
- Flags: `--json` (ohne Farben, beide Modi geben eingerückt aus), `-v`/`--variables` (JSON-Zeichenkette), `-o`/`--operation` (Operationsname bei mehreren Operationen), `--schema` (Schema ausgeben und beenden).
- HTTP: `lif-serve` stellt das volle Schema unter `/api/graphql` bereit (WebSocket, OPTIONS, GET, POST) und den GraphQL Playground unter `/playground`; Standardport 8080.
- Zeigt Beispielaufrufe: Argument, stdin-Pipe, Variablen, `--schema` (die Hilfe des Befehls nennt: Items auflisten, Item holen, nach Status filtern, Beziehungen, Variablen, stdin, Schema).
- Quelle: FACTS.md §7.5, §1.1 (`lif graphql`), §8.1

### Schema overview
- Erklärt: Ein Schema (`internal/graph/schema.graphqls`, eine Datei) enthält die Kern-Item-Operationen und daneben Operationen nur für den Webserver.
- Kern-Item-Operationen: `item`, `items`, `createItem`, `updateItem`, `deleteItem`, `setParent`, `addBlocking`, `removeBlocking`, `addBlockedBy`, `removeBlockedBy`, `archiveItem`, `saveDirtyItems`, `saveItem`, Subscription `itemChanged`.
- Nur Webserver: Worktrees, Agent-Sitzungen, Terminal, Dateiänderungen/Diffs, Run/Stop, Öffnen im Editor; im CLI-Betrieb liefern Mutations dieser Gruppe Fehler wie "agent manager not available".
- Hinweis: Einzelne reine Query-Felder der Server-Gruppe können im CLI trotzdem antworten, wenn sie keinen Manager brauchen (beobachtet: `{ agentEnabled }`).
- Betont: `status`, `type` und `priority` sind einfache `String`-Felder, keine Enums; gültige Werte kommen aus der Konfiguration (`reference/configuration`). Skalare: `Time`, `Map`.
- Zeigt die Typen `Item` (Felder inkl. abgeleitete `etag`, `isDirty`, `implicitStatus`, `implicitStatusFrom`, `extra`) in kompakter Tabelle.
- Quelle: FACTS.md §7, §7.1, §7.2, §7.3, §7.4

### Queries
- `item(id: ID!)`: akzeptiert volle oder kurze ID ohne Präfix, liefert `null`, wenn nichts gefunden wird.
- `items(filter: ItemFilter)`: Liste aller passenden Items.
- Tabelle `ItemFilter`: `search`, `status`/`excludeStatus`, `type`/`excludeType`, `priority`/`excludePriority`, `tags`/`excludeTags`, Eltern-Filter (`hasParent`, `noParent`, `parentId`), Blocking-Filter (`hasBlocking`, `noBlocking`, `blockingId`, `hasBlockedBy`, `noBlockedBy`, `blockedById`), Blockiert-Zustand (`isBlocked`, `isExplicitlyBlocked`, `isImplicitlyBlocked`), `excludeImplicitTerminal`.
- Weitere Felder: `projectName`, `mainBranch`; Beziehungsfelder am Item (`parent`, `children`, `blocking`, `blockedBy` mit eigenem `filter`).
- Zeigt je ein Beispiel: Item per ID, Filter nach Status, Kinder eines Containers.
- Quelle: FACTS.md §7.1, §7.4

### Mutations
- Erklärt die Kern-Mutations: `createItem(input: CreateItemInput!)`, `updateItem(id, input: UpdateItemInput!)`, `deleteItem(id)` (entfernt eingehende Links automatisch), `setParent(id, parentId, ifMatch)` (prüft die Typ-Hierarchie), `addBlocking`/`removeBlocking`/`addBlockedBy`/`removeBlockedBy` (je mit `ifMatch`), `archiveItem`.
- Tabelle `CreateItemInput` (`title`, `type`, `status`, `priority`, `tags`, `body`, `parent`, `blocking`, `blockedBy`, `prefix`) und `UpdateItemInput` (u. a. `addTags`/`removeTags`, `bodyMod`, `addBlocking` usw., `order`, `ifMatch`).
- Erklärt `bodyMod` (`replace: [{old, new}]`, `append`) als Alternative zu `body`; beides zusammen ist verboten.
- Erklärt `ifMatch` (ETag-Vergleich gegen gleichzeitige Änderungen) und das abgeleitete Feld `etag` am Item.
- Zeigt Beispiel: Item anlegen, Status ändern, Eltern setzen, Blocking hinzufügen.
- Quelle: FACTS.md §7.2, §7.4

### Subscriptions
- Erklärt `itemChanged(includeInitial: Boolean = false)` mit `ItemChangeEvent { type, item, items, itemId }`; `ChangeType` = `INITIAL_SNAPSHOT`, `CREATED`, `UPDATED`, `DELETED`; ein langsamer Abonnent bekommt zusätzlich `INITIAL_SNAPSHOT`.
- Stellt klar: Subscriptions laufen nur über den WebSocket von `lif-serve`, nicht über `lif graphql`.
- Weitere Subscriptions der Server-Gruppe nur nennen: `worktreesChanged`, `agentSessionChanged`, `activeAgentStatuses`, `workspaceStatuses`.
- Verweist auf den Abschnitt "Live updates" in `reference/web-server`.
- Quelle: FACTS.md §7.3, §8.1

### Examples
- Drei echte Läufe mit Ausgabe: Items filtern (Query), Item anlegen (Mutation), Item mit Variablen ändern (`-v`).
- Jedes Beispiel zeigt Befehl, Ausgabe und kurze Erklärung der Felder.
- Die Beispiele sind in Phase 5 in einem frischen Store mit `lif init` auszuführen; keine erfundene Ausgabe.
- Quelle: `TODO: verify` — FACTS zeigt keine vollständigen Läufe außer dem beobachteten `createItem`; in Phase 5 mit `lif graphql` im Test-Store ausführen

### Errors and validation
- Fehlerformat im CLI: ein Fehler als `graphql: <message>`, mehrere als `graphql errors:` mit eingerückter Liste.
- Eingabe-Validierung beim Schreiben: Titel nicht leer und höchstens 1000 Zeichen, Body höchstens 1 MiB, Tags müssen dem Tag-Muster entsprechen, Präfix-Muster; `body` und `bodyMod` schließen sich aus, ebenso `tags` und `addTags`/`removeTags`.
- Typ-, Status- und Prioritätsnamen werden gegen die konfigurierten Tabellen geprüft (nur bei Anlegen und bei gelieferten Feldern).
- Elternprüfung bei `createItem` (mit Parent), `setParent` und `updateItem` mit `parent`: Rang-Regel und Zyklusprüfung; wirkt wie im CLI.
- HTTP-WebSocket: nicht erlaubter Origin liefert 403 `origin not allowed` (Querverweis `reference/web-server`).
- Quelle: FACTS.md §7.4, §7.5, §3.1, §8.1

### Known gaps
- `createItem` ohne `status` lässt den Status leer (`"status": ""`); der Standardstatus `todo` wird nur von `lif create` gesetzt. Empfehlung: `status` immer angeben.
- Eine Änderung nur des `type` über `updateItem(type)` prüft die Eltern-/Kind-Ränge nicht erneut (gleich wie bei `lif update -t`).
- Beide Lücken sind als Produktfehler erfasst; Seite aktualisieren, sobald sie behoben sind.
- Quelle: FACTS.md §7.5, §3.1; DESIGN.md D43

### Schema (SDL)
- Bindet die vollständige SDL ein, zur Build-Zeit erzeugt aus `lif graphql --schema`; nicht von Hand kopieren.
- Hinweis für die Umsetzung: der Generator-Schritt (Einbettung) gehört in Phase 5.
- Optional kurzer Satz, dass das Schema 845 Zeilen lang ist und mit `just gqlgen` neu erzeugt wird (nur Entwickler).
- Quelle: FACTS.md §7, §7.5; DESIGN.md D27
