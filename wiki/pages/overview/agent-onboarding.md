---
title: "Agent onboarding"
description: "Teach a coding agent to use Lif: lif prime, hooks and a few rules."
---

# Agent onboarding

## Zweck
Bringt einen Coding-Agenten schnell zum produktiven Umgang mit Lif und hält ihn davon ab, vom Store abzudriften. Empfiehlt Hooks (Re-Priming und Pre-Commit), die Doku zeigt nur Snippets; es wird nichts mitgeliefert.

## Zielgruppe
Menschen, die einen Coding-Agenten (Claude Code, OpenCode, andere) mit Lif einrichten, und Agenten, die sich selbst orientieren. Grundkenntnisse zu Items und `lif` aus Concepts (`overview/concepts`) helfen.

## Pflichtabschnitte

### How an agent learns Lif
- Reihenfolge: `lif prime` zuerst, dann `lif --help`, dann Skills; keine Website nötig. (TODO: verify)
- `prime` und `--help` laufen ohne geladenen Store; alle anderen Befehle brauchen einen.
- Zeigt die Tagline "A file-based issue tracker for AI-first workflows" als Leitidee.
- Diagramm: prime → help → skills als drei Stufen. (TODO: verify)
- Quelle: FACTS.md §1.0 (Root-Texte, Bootstrap-Skip), §1.1 (`prime`). Die Stufenfolge prime → help → skills ist `TODO: verify` (Prüfung: `wiki/agent-integration.md`, Skill-Ort siehe Abschnitt "Skills").

### lif prime
- `lif prime` gibt einen Prompt aus, der Agenten in die Nutzung von Lif einweist; es hat keine Flags.
- Zeigt `lif prime` mit gekürzter Beispielausgabe (Phase 5 echt laufen lassen).
- Erklärt: Text kommt aus einer Vorlage im Code (`internal/commands/prompt.tmpl`) und enthält Befehlsfakten; er erwähnt auch `lif check --strict`.
- Inhalt der Ausgabe im Einzelnen (Abschnittsliste) (TODO: verify).
- Quelle: FACTS.md §1.1 (`prime`), §6.10 (Verweis auf `prompt.tmpl:203`). Die Inhaltsübersicht der Ausgabe ist `TODO: verify` (prüfen mit `lif prime` in Phase 5).

### Session hooks
- Idee: Der Agent liest `lif prime` automatisch, statt sich auf Erinnerung zu verlassen.
- Claude Code: Hooks auf `SessionStart` und `PreCompact` führen `lif prime` aus.
- Zeigt kurzes Snippet der Hook-Konfiguration (Phase 5 aus der vorhandenen Konfiguration übernehmen und prüfen).
- Agenten ohne Hooks: Prompt einmal einfügen oder in der Agent-Anweisungsdatei auf `lif prime` verweisen. (TODO: verify)
- Quelle: FACTS.md §6.10, §9(f) (Claude Code Hooks führen `lif prime` aus, `.claude/settings.json`, `extras/claude/plugins/lif-prime/hooks/hooks.json`). Hook-Snippet und Verhalten ohne Hooks sind `TODO: verify` (prüfen in `extras/claude/plugins/lif-prime`, `wiki/agent-integration.md`).

### Drift prevention hook
- Problem: Ein Agent vergisst nach Kontext-Kompaktierung die Regeln oder ändert Dateien im Store per Hand; der Store driftet vom Soll ab.
- Empfehlung 1, Re-Priming: Session-Hook, der `lif prime` bei Sitzungsbeginn und vor der Kontext-Kompaktierung (`PreCompact`) ausführt. Snippet zeigen.
- Empfehlung 2, Pre-Commit: Git-Hook, der `lif check` ausführt; Exit-Code 1 bei Fund bricht den Commit ab. Snippet zeigen (Skript mit `lif check`, ausführbar unter `.git/hooks/pre-commit` oder über den üblichen Hook-Manager des Projekts).
- Nichts wird mitgeliefert: Die Doku zeigt nur die Snippets, jedes Projekt richtet sie selbst ein (D41). Verweis auf Git-native (`features/git-native`).
- Ehrliche Grenze (D40): `lif check` prüft Config, unbekannten Typ, kaputte Links, Selbstverweise, Zyklen, Pflichtfelder (nur Warnung), unbekannte Commit-SHAs (Warnung), verwaiste Anhang-Ordner. Es prüft nicht: Parent-/Kind-Ranks, Status- und Prioritätswerte, Tag-Format, doppelte IDs, Dateiname/ID-Abweichung.
- Folge: Der Pre-Commit-Hook fängt Handänderungs-Drift nur teilweise; Warnungen nur mit `lif check --strict`; Doku folgt, sobald die Erweiterung der Prüfung geliefert ist.
- Quelle: FACTS.md §6.8, §6.9, §6.10, §6.11, §9(e), §9(f); DESIGN.md D40, D41. Die Hook-Snippets selbst sind in Phase 5 zu bauen und zu testen (Prüfung: Commit mit absichtlich kaputtem Link).

### Rules for agents
- Lesen über `--json` (`lif list --json`, `lif show --json`, `lif next --json`), nicht Text parsen.
- Schreiben über die CLI (`lif create`, `lif update`, `lif start`, `lif complete`), weil nur diese Wege Regeln erzwingen.
- IDs nie per Hand umbenennen, sondern `lif rename` nutzen (aktualisiert Verweise). (TODO: verify)
- Nach jeder Handänderung `lif check` ausführen und den Exit-Code prüfen.
- Kurz- oder Voll-ID sind beide gültig.
- Fehler als JSON: Codes wie `NOT_FOUND`, `VALIDATION_ERROR`, `CONFLICT`.
- Quelle: FACTS.md §1.0, §1.1 (`rename`, `check`, `list`, `show`, `next`), §3.1, §3.2, §6.8. Die Aussage "`lif rename` aktualisiert Verweise" ist `TODO: verify` (prüfen mit `lif rename --help` und `internal/commands/rename.go`).

### Skills
- Was ein Skill über `lif prime` hinaus bringt: stabile, projektspezifische Arbeitsanweisungen und Beispiele.
- Wo ein Skill liegt und wie er geladen wird, je Agent.
- Abgrenzung: `prime` liefert Fakten aus den Befehlen, ein Skill liefert Arbeitsweise.
- Quelle: `TODO: verify` — FACTS.md enthält nichts zu Skills; prüfen in `extras/claude/plugins/lif-prime`, `.opencode`, `wiki/agent-integration.md`.

### Setup per agent
- Claude Code: Plugin `lif-prime` mit Hooks für `SessionStart` und `PreCompact` (Installationsbefehle ergänzen).
- OpenCode: Plugin im Verzeichnis `.opencode` (Installationsschritte ergänzen).
- Andere Agenten: `lif prime` einmal einfügen oder als Befehl aufrufen lassen.
- Tabelle: Agent · Mechanismus · Befehle.
- Quelle: FACTS.md §6.10 (Claude Code Hooks nachgewiesen). OpenCode- und Plugin-Installationsschritte sind `TODO: verify` (prüfen in `extras/claude/plugins/lif-prime/README.md`, `.opencode/plugin`, `wiki/agent-integration.md`).

### Example first session
- Beispielablauf als Transkript: `lif prime` → `lif next` → `lif start` → Arbeit → `lif complete --summary` → `lif check`.
- Kommentiert jeden Schritt in einer Zeile.
- Zeigt `--json`-Ausgaben dort, wo ein Agent sie parst.
- Quelle: FACTS.md §1.1 (`prime`, `next`, `start`, `complete`, `check`), §6.8. Das Transkript ist in Phase 5 mit echtem Lauf zu erzeugen (`TODO: verify`).
