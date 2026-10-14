---
title: "Web server"
description: "lif-serve: the web UI and GraphQL HTTP server."
---

# Web server

## Zweck
Referenz für den Webserver `lif-serve`: Start, Optionen, Routen, Netzwerk-/CORS-Verhalten und was er zusätzlich zur CLI anbietet. Die Seite hat niedrige Priorität und folgt dem implementierten Umfang.

## Zielgruppe
Nutzer, die die Weboberfläche starten, und Betreiber, die die GraphQL-HTTP-Schnittstelle in Skripte oder andere Rechner einbinden.

## Pflichtabschnitte

### What lif-serve is
- Erklärt: `lif-serve` ist ein eigenes Binary, das Weboberfläche und GraphQL-HTTP-Schnittstelle liefert; `lif` selbst enthält keinen Server.
- `lif serve` im `lif`-Binary ist ein versteckter Platzhalter, der auf `lif-serve` verweist und mit Exit 1 endet (nicht dokumentieren, nur diesen Hinweis geben).
- `lif-serve` kennt nur den Befehl `serve`; ohne Argument oder bei erstem Argument mit `-` wird `serve` ergänzt.
- Verweist auf `overview/installation` (welche Binaries es gibt) und `reference/graphql`.
- Quelle: FACTS.md §1.0, §1.1 (`lif serve`, `lif-serve`), §8.1

### Starting the server
- Zeigt `lif-serve` und `lif-serve serve` (gleichwertig); Aliase `server` und `s`.
- Optionen: `--port`/`-p` (Standard 8080), `--cors-origin` (Liste, kommagetrennt oder wiederholt; Standard `http://localhost:*` und `http://127.0.0.1:*`; `*` erlaubt alle).
- Konfiguration: `server.port` und `server.cors_origins` in `.lif.yml`; Rangfolge Flag vor Konfiguration vor Standard.
- Startmeldung des Servers (Adresse, Playground, erlaubte Origins) als Beispielausgabe.
- Hinweis, dass der Server die Store-Konfiguration wie die CLI lädt (globale Flags `--config`, `--lif-path`). (TODO: verify)
- Quelle: FACTS.md §8.1, §4.3, §1.1 (`lif-serve`); `--config`/`--lif-path` auf `lif-serve` — `TODO: verify` (Phase 5: `lif-serve --help` ausführen)

### Network and CORS
- Der Server lauscht auf `:<port>`, also auf allen Schnittstellen; Warnhinweis für Netzwerke.
- CORS-Allow-List: Muster mit `:*` am Ende passt auf jeden Port; `*` erlaubt alles.
- Der WebSocket-Upgrade (GraphQL) prüft den Origin gegen dieselbe Liste; unerlaubter Origin → 403 `origin not allowed`.
- Beispiel für Zugriff von einem anderen Rechner mit angepasster `--cors-origin`. (TODO: verify)
- Quelle: FACTS.md §8.1, §4.3; Beispiel — `TODO: verify` (in Phase 5 mit zweitem Host/Tailnet prüfen)

### Routes
- Tabelle: `/api/graphql` (GraphQL; WebSocket, OPTIONS, GET, POST), `/playground` (GraphQL Playground), `/api/attachments/:beanId/:filename` (Bildanhänge des Agent-Chats), `/api/terminal` (WebSocket-Terminal), alle anderen Pfade liefern die eingebettete Single-Page-App (Rückfall auf `index.html`).
- Hinweis: Der Platzhalter `:beanId` stammt aus dem Code; Beschreibung so wählen, dass klar bleibt, dass er die Item-ID meint.
- Verweist auf `reference/graphql` für `/api/graphql` und `/playground`.
- Quelle: FACTS.md §8.1

### Web UI
- Erklärt: Das Frontend ist in das Binary eingebettet (`dist/*`), ein SvelteKit/Vite-Projekt in `frontend/`; kein separater Webserver nötig.
- Zeigt Screenshots der Oberfläche (webp, D38): Board, Detailansicht. (TODO: verify)
- Beschreibt Ansichten und Board-Spalten. (TODO: verify)
- Hinweis: Eine Toolbar-Schaltfläche "Run" erscheint, wenn `worktree.run` gesetzt ist.
- Quelle: FACTS.md §8.1, §4.3; DESIGN.md D38; Ansichten und Board-Spalten — `TODO: verify` (Phase 5: Code unter `frontend/` (BoardView) und laufende Oberfläche prüfen)

### Live updates
- Erklärt: Der Server beobachtet das Store-Verzeichnis und schickt Änderungen an Abonnenten (`itemChanged`-Subscription); externe Änderungen an Dateien erscheinen ohne Neuladen.
- Verzeichnisse von Worktree-Stores werden ebenfalls beobachtet und in den Laufzeitzustand eingemischt.
- Zeigt kurzes Beispiel: Subscription im Playground, Item per `lif update` ändern, Ereignis erscheint.
- Quelle: FACTS.md §8.1, §7.3; Beispiel — `TODO: verify` (in Phase 5 ausführen)

### Server-only capabilities
- Nennt die Funktionen, die es nur mit `lif-serve` gibt: Worktrees, Agent-Sitzungen (Claude), Terminal-Sitzungen, Dateiänderungen und Diffs, Start/Stopp des Projekts, Öffnen im Editor.
- Schalter in der Konfiguration: `agent.enabled` (Standard `true`), `agent.default_mode`, `agent.default_effort`, `worktree.base_ref`, `worktree.path`, `worktree.setup`, `worktree.run`, `worktree.integrate`, `worktree.fetch_timeout`.
- Umgebungsvariable `LIF_WORKSPACE_PORT` für Shells in Workspaces (alt: `BEANS_WORKSPACE_PORT`).
- Verweist auf `reference/configuration` für die Schlüssel.
- Quelle: FACTS.md §8.1, §4.3, §7

### Limits
- Keine Beschreibung von Authentifizierung in den Fakten; ehrlich sagen, was bekannt ist: Der Server lauscht auf allen Schnittstellen, Schutz nur über die Origin-Liste.
- Aussage zur Authentifizierung und zu Transportverschlüsselung (TLS) nicht erfinden.
- `lif serve` im `lif`-Binary funktioniert nicht (Platzhalter).
- Quelle: FACTS.md §8.1; Authentifizierung/TLS — `TODO: verify` (Phase 5: `internal/commands/serve.go` prüfen)
