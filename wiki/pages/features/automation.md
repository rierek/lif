---
title: "Automation and agents"
description: "JSON output, GraphQL and lif prime for scripts and coding agents."
---

# Automation and agents

## Zweck
Zeigt, wie Skripte und Coding-Agents Lif maschinenlesbar nutzen: `--json`, GraphQL, `lif prime` und `lif promote`. Enthält die Anleitungen für Agent-Hooks und für die Prüfung nach automatischen Änderungen.

## Zielgruppe
Skript-Autoren, Betreiber von Coding-Agents und Menschen, die einen Agenten an ihr Backlog anschließen. Sie brauchen konkrete Aufrufe, Exit-Codes und die bekannten Lücken.

## Pflichtabschnitte

### What
- `--json` an vielen Befehlen (zum Beispiel `list`, `next`, `create`, `complete`, `progress`, `roadmap`, `check`); `lif graph` nutzt `--format json`; im JSON-Modus entfallen die menschlichen Zeilen, Fehler tragen Codes wie `NOT_FOUND` oder `VALIDATION_ERROR`.
- GraphQL ohne Server: `lif graphql '<query>'` (Alias `lif query`), auch per stdin; mit Server über `lif-serve` unter `/api/graphql` mit Playground unter `/playground`.
- `lif prime` gibt einen Prompt aus, der einem Coding-Agent die Nutzung von Lif beibringt; läuft ohne geladenen Store.
- `lif promote <artifact> <finding-id>...` legt aus geprüften Review-Findings Items an; schreibt nichts, wenn ein gewählter Eintrag fehlerhaft ist, und überspringt bereits übernommene.
- Exit-Codes: 0 bei Erfolg, 1 bei Fehler; `lif check` gibt 1 bei mindestens einem Problem; es gibt keinen Exit-Code 2.
- Quelle: FACTS.md §1.0, §1.1 (`lif list`, `lif promote`, `lif prime`, `lif graph`), §6.8, §6.11, §7.5

### Why
- Lif kann an automatischen Abläufen teilnehmen statt nur manuell bedient zu werden.
- Der Store braucht keinen Server; Skripte und Agents arbeiten direkt auf den Dateien.
- Agents lernen Lif über `lif prime` und `--help`, nicht über Vorwissen.
- Quelle: CONCEPT.md §4.5; FACTS.md §7.5, §1.0

### How it looks
- `lif list --json` mit `jq` in einer Pipeline; Optionen `--full` (Body im JSON) und `--quiet` (nur IDs).
- GraphQL-Einzeiler mit `lif graphql '{ items { id title status } }'`, Variablen mit `-v`, Anfrage über stdin, `--schema`.
- Mutation per CLI: `createItem` mit `title` und `type`; Hinweis auf die Statuslücke aus Limits.
- Promote-Ablauf: Review-Artefakt, Finding-IDs, Ergebnis als Items; Format des Artefakts in Phase 5 prüfen (TODO: verify)
- Alle Beispiele in Phase 5 real ausführen und echte Ausgaben einfügen (TODO: verify)
- Quelle: FACTS.md §1.1 (`lif list`, `lif promote`), §7.5; (TODO: verify) für Beispielausgaben und Promote-Artefakt: Befehle in Phase 5 laufen lassen, Schema in `internal/commands/findings-schema.json` prüfen

### Agent hooks
- Anleitung (Agent-Seite): Ein Session-Hook führt `lif prime` bei Sitzungsbeginn und vor der Kontextverdichtung aus; so bleibt der Agent bei der Sache.
- Das Projekt nutzt dafür Claude-Code-Hooks, die bei `SessionStart` und `PreCompact` `lif prime` aufrufen.
- Konfigurations-Snippet für die Hooks aus der Vorlage übernehmen und gegen den aktuellen Stand prüfen (TODO: verify)
- Hinweis: Das Beispiel gilt für Claude Code; andere Agents brauchen eigene Hook-Mechanismen (TODO: verify)
- Commit-Nachrichten-Konvention mit Item-ID, damit Agent-Arbeit zurückverfolgbar bleibt: Konvention, nicht belegt (TODO: verify)
- Verweis auf Drift-Vermeidung in Agent onboarding (`overview/agent-onboarding`).
- Quelle: FACTS.md §6.10; (TODO: verify) für Snippet, andere Agents und Commit-Konvention: `.claude/settings.json` und `extras/claude/plugins/lif-prime/hooks/hooks.json` im Checkout lesen

### Validation after automated changes
- Nach automatischen Änderungen `lif check` laufen lassen; Exit 1 meldet mindestens ein Problem, Exit 0 keines.
- `--strict` zählt Policy-Warnungen (fehlende Pflichtfelder, unbekannte Commit-SHAs) als Probleme.
- `--fix` entfernt defekte Links und Selbstbezüge; Zyklen, Konfigurationsfehler, Anhang-Waisen und Policy-Warnungen bleiben.
- `--json` liefert `success`, `config_errors`, `item_issues` (`broken_links`, `self_links`, `cycles`) und weitere Felder.
- Aussage des Prompts: Konfiguration, Links, Front Matter und Policies mit `lif check` prüfen, bevor die Automatisierung weiterläuft.
- Grenze: `lif check` prüft weder Rang, Status, Priorität noch Tags; ein Agent, der Dateien direkt schreibt, bleibt dort ungeprüft.
- Quelle: FACTS.md §6.7, §6.8, §6.9, §6.10; DESIGN.md D40

### Limits
- Bekannter Fehler: GraphQL `createItem` ohne `status` lässt den Status leer; nur `lif create` setzt den Standard `todo`.
- Bei einem GraphQL-Fehler gibt die CLI eine Fehlermeldung aus (`graphql: <message>`); ob dabei Teildaten geliefert werden, ist nicht belegt (TODO: verify)
- Server-nur Teile des Schemas (Agent, Terminal, Worktrees) scheitern in der CLI mit Meldungen wie "agent manager not available".
- Subscriptions laufen nur über das WebSocket von `lif-serve`, nicht über `lif graphql`.
- Direkt geschriebene Dateien umgehen die Regeln; `lif check` deckt nur einen Teil ab.
- Quelle: FACTS.md §7, §7.3, §7.5, §6.9; DESIGN.md D43; (TODO: verify) für Teildaten bei Fehlern: `internal/commands/graphql.go` prüfen

### Reference links
- GraphQL (`reference/graphql`): Schema, Abfragen, Mutationen, Fehler.
- JSON schemas (`reference/json-schemas`): Nutzlasten, Fehlercodes, Exit-Codes.
- `lif graphql`, `lif prime`, `lif promote`, `lif check` (`reference/commands/lif-graphql`, `reference/commands/lif-prime`, `reference/commands/lif-promote`, `reference/commands/lif-check`).
- Web server (`reference/web-server`): HTTP-Endpunkte von `lif-serve`.
- Agent onboarding (`overview/agent-onboarding`): empfohlene Hooks.
- Quelle: DESIGN.md D24, D27, D28
