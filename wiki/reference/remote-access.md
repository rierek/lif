---
title: "Remote access"
description: "Reach lif-serve from another machine: Tailscale, SSH tunnel, Caddy, nginx, TLS, and running it as a service."
---

`lif-serve` listens on `127.0.0.1` and needs no token there. To use it from a phone, a laptop or another server, pick one recipe on this page. Each recipe keeps the access token, the terminal gate and the agent gate of [Network and CORS](../web-server/#network-and-cors). The options are described on [Web server](../web-server/#starting-the-server).

Measured on a 0.16 development build, 2026-10-18, on macOS with Tailscale 1.102.4 and Caddy 2.11.7. The numbers in the recipes are HTTP status codes seen with `curl`. Every recipe below that is not marked "by review" ran once. Three parts were not run: the SSH tunnel (the test machine has no passwordless SSH login), nginx (not installed) and the systemd unit (needs Linux). Each is marked.

Pick a recipe:

| You have | Use | Needs a flag |
|---|---|---|
| Tailscale | [Tailscale serve](#tailscale-serve-recommended): HTTPS, nothing else to install | `--public-url` |
| Tailscale, no HTTPS needed | [`--host tailscale`](#--host-tailscale) | none |
| SSH access to the machine | [SSH tunnel](#ssh-tunnel) | none |
| Caddy or nginx | [Caddy](#caddy), [nginx](#nginx) | `--public-url` |
| A certificate | [TLS](#tls) | none |

## Tailscale serve (recommended)

`tailscale serve` is a reverse proxy of your tailnet. It ends TLS with a certificate for the machine name and passes the request to `lif-serve` on loopback, so only devices of your tailnet reach it.

```sh
lif-serve --port 8080 --public-url https://<machine>.<tailnet>.ts.net/lif
tailscale serve --bg --set-path /lif http://127.0.0.1:8080
```

`<machine>.<tailnet>.ts.net` is the name `tailscale status --json` reports as `Self.DNSName`, without the trailing dot. The server prints the address to open:

```text
[lif] Listening on 127.0.0.1:8080 (loopback only: reachable from this machine only)
[lif] Access: token required (--require-token or --public-url)
[lif] Access token: <token>
[lif] Open the UI: https://<machine>.<tailnet>.ts.net/lif/?token=<token>
```

Open that address once, **with the trailing slash**: the server sets the cookie and redirects to the page without the token. Measured with `curl`:

| Request | Answer |
|---|---|
| `GET https://<machine>.<tailnet>.ts.net/lif/` | `401` |
| `GET https://<machine>.<tailnet>.ts.net/lif/?token=<token>` | `303`, `Location: ./`, `Set-Cookie: lif_token_<port>=…; HttpOnly; Secure; SameSite=Lax` |
| `POST …/lif/api/graphql` with `Authorization: Bearer <token>` (query `agentEnabled`) | `200`, `false` |
| `GET …/lif/api/terminal` | `403`, the terminal is disabled |

`tailscale serve` removes the `/lif` prefix before it calls the server, and the UI works under it: its files and API calls use paths relative to the page. The server needs no prefix setting. `--public-url` carries the prefix only to build the links in the log.

Without `--public-url` (or `--require-token`) the server refuses the request, because `tailscale serve` adds identity headers to it: `403` with `this request came through a proxy, but lif-serve runs without an access token. Start it with --public-url <url> or --require-token.`

`tailscale serve reset` removes the serve configuration again. Never use `tailscale funnel` for this: it puts the server on the public internet.

## `--host tailscale`

```sh
lif-serve --host tailscale --port 8080
```

The server binds the first IPv4 address of this machine on the tailnet (from `tailscale status --json`), not the LAN and not loopback, and prints links with the MagicDNS name:

```text
[lif] Listening on 100.x.y.z:8080 (non-loopback address: reachable from the network)
[lif] Open the UI: http://<machine>.<tailnet>.ts.net:8080/?token=<token>
```

Measured: without the token `401`, with `?token=` `303` to `./`, with `Authorization: Bearer` `200`. The server is a listener beyond loopback, so the token is required, and the terminal and the agent are off.

Tailscale encrypts the traffic between your devices (WireGuard), but the page itself is plain `http://`. The token cookie is not `Secure`, and features that browsers allow only on `https://` pages are off. Use [Tailscale serve](#tailscale-serve-recommended) or [TLS](#tls) when you want HTTPS.

The server fails at start when it cannot get the address: `--host tailscale: Tailscale is not running (state …)`, `--host tailscale: no tailnet IPv4 address`, or `--host tailscale: tailscale CLI not found`.

## SSH tunnel

```sh
# on the machine that runs lif-serve
lif-serve

# on your laptop
ssh -L 8080:127.0.0.1:8080 user@host
```

Then open `http://localhost:8080/` on the laptop. By review: this recipe was not run, because the test machine has no passwordless SSH login; a plain `curl` to loopback without proxy headers, which is what the tunnel end does, gets the loopback answers (no token). The tunnel makes a connection to loopback of the remote machine, which adds no proxy header, so the server stays in its loopback mode: no token, terminal and agent on. Only users who can log in to the machine by SSH reach it. The local port may differ (`-L 9090:127.0.0.1:8080`): the host check ignores the port.

This is the smallest setup, and the right one when you already use SSH. It does not help on a phone. Every other local user of the remote machine can still use the port, as on any loopback server.

## Caddy

Caddy ends TLS and removes the prefix with `handle_path`:

```caddyfile
box.example.org {
	handle_path /lif/* {
		reverse_proxy 127.0.0.1:8080
	}
}
```

```sh
lif-serve --port 8080 --public-url https://box.example.org/lif
```

Caddy passes WebSocket upgrades (GraphQL subscriptions, the terminal) without extra settings. Measured with Caddy 2.11.7 on `:8793` (plain HTTP) and `--public-url http://localhost:8793/lif`:

| Request | Answer |
|---|---|
| `GET localhost:8793/lif/` | `401` |
| `GET localhost:8793/lif/?token=<token>` | `303`, `Location: ./` |
| `GET localhost:8793/lif/planning` with `Authorization: Bearer` | `200` |
| WebSocket upgrade of `localhost:8793/lif/api/graphql` with `Authorization: Bearer` | `101` |
| `GET localhost:8793/lif` (no trailing slash) | `200` with an empty body: `handle_path /lif/*` does not match it |

Without `--public-url` and without `--require-token` every request through Caddy gets `403` with `this request came through a proxy…`: Caddy adds `X-Forwarded-For`. Always start the server with `--public-url`. Caddy writes the query string to its access log; the token is in it on the first request (see [Threat model](#threat-model-summary)).

## nginx

By review: nginx is not installed on the test machine, so this recipe was not run.

```nginx
location = /lif { return 301 /lif/$is_args$args; }

location /lif/ {
    proxy_pass http://127.0.0.1:8080/;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_read_timeout 3600s;
}
```

```sh
lif-serve --port 8080 --public-url https://box.example.org/lif
```

The slash at the end of `proxy_pass http://127.0.0.1:8080/;` is what removes the `/lif/` prefix; without it the server sees `/lif/…` and answers with the page for an unknown path. The `Upgrade` lines are for WebSockets. `proxy_read_timeout` keeps idle WebSockets open longer than the 60 s default.

**`--public-url` or `--require-token` is mandatory with nginx.** The proxy guard of the server recognises a proxy by the headers `X-Forwarded-For`, `X-Forwarded-Host`, `X-Forwarded-Proto`, `Forwarded` and `Tailscale-User-Login`. nginx adds none of them unless you write `proxy_set_header X-Forwarded-For …`. A loopback server without the flag would then serve every client of the proxy with no token and with a terminal, and nothing would warn you.

## TLS

`lif-serve` can serve HTTPS itself:

```sh
lif-serve --host 0.0.0.0 --port 8443 --tls-cert cert.pem --tls-key key.pem
```

Both files are PEM. With TLS the cookie is `Secure`, the own origin of the UI is `https://…`, and the log drops the clear-text warning. Plain HTTP to the port is refused with `400`.

For a real certificate on a tailnet, `tailscale cert <machine>.<tailnet>.ts.net` writes a certificate and a key for the MagicDNS name (HTTPS must be enabled for the tailnet first, see the Tailscale documentation). Use `--host tailscale --tls-cert <machine>.<tailnet>.ts.net.crt --tls-key <machine>.<tailnet>.ts.net.key` and open `https://<machine>.<tailnet>.ts.net:8080/`. Not run here.

For a test, a self-signed certificate does it. This recipe ran with both the LibreSSL 3.3.6 that macOS ships (`/usr/bin/openssl`) and OpenSSL 4.0.3, and `lif-serve` loaded both results:

```sh
openssl ecparam -name prime256v1 -genkey -noout -out key.pem
openssl req -x509 -new -key key.pem -out cert.pem -days 30 -subj /CN=localhost
lif-serve --host 0.0.0.0 --port 8443 --tls-cert cert.pem --tls-key key.pem
```

Measured with `--host 0.0.0.0` and `curl -k`: without the token `401`; with `?token=<token>` `303` and `Set-Cookie: lif_token_<port>=…; HttpOnly; Secure; SameSite=Lax`; with `Authorization: Bearer` `200`; `http://` to the same port `400`. The browser warns about a self-signed certificate, as it should.

## Run as a service

Two service files are in the repository under `extras/`. Both start `lif-serve` on port 8080 in the project directory with `--token-file`, which keeps the token across restarts and is created with mode `0600` on first start.

**systemd** (Linux, user unit), `extras/systemd/lif-serve.service`:

```sh
mkdir -p ~/.config/systemd/user
cp extras/systemd/lif-serve.service ~/.config/systemd/user/
# edit WorkingDirectory and ExecStart, then:
systemctl --user daemon-reload
systemctl --user enable --now lif-serve
loginctl enable-linger $USER   # keeps it running after you log out
```

By review: the unit was not started, because systemd needs Linux and the test machine runs macOS. It uses only `WorkingDirectory`, `ExecStart`, `Restart=on-failure` and `WantedBy=default.target`, with `%h` for the home directory.

**launchd** (macOS, user agent), `extras/launchd/org.lif.serve.plist`:

```sh
cp extras/launchd/org.lif.serve.plist ~/Library/LaunchAgents/
# replace CHANGE-ME and the path of lif-serve, then:
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/org.lif.serve.plist
launchctl kickstart -k gui/$(id -u)/org.lif.serve   # restart
launchctl bootout gui/$(id -u)/org.lif.serve        # stop
```

Measured with a copy of the file (label `org.lif.serve.test`, port 8794, `--require-token` added): `plutil -lint` passed; after `bootstrap` the server answered `401` without and `200` with the token from the token file (mode `-rw-------`); after `kickstart -k` the process had a new PID and the same token still gave `200`; after `kill` of the process launchd started it again (`KeepAlive`, after its 10 s throttle) and the token still gave `200`.

A service has no terminal to print the token to. Read it from the token file when you open the UI: `…/?token=$(cat ~/.config/lif/serve-token)`. The token file only applies when the server asks for a token, so add the flag of your recipe (`--public-url …`, `--require-token`, or `--host …`) to the command line of the service. On loopback without such a flag the file is ignored. If you also give `--allow-remote-terminal` or `--allow-remote-agent`, the token that opens the server is derived from the one in the file, and the log shows it: `Access token: <token> (derived from <path> for the remote opt-ins)`.

## Threat model summary

- **The token is the protection beyond loopback.** It has 32 random bytes, or the 32 characters or more you bring with `--token-file` or `LIF_SERVE_TOKEN`. Every request needs it, as `Authorization: Bearer` or as the cookie. Anyone who has it can read and change every item of the store. Treat the token file and the log like a password.
- **The token is in the first address.** The server removes it from its own access log, but a reverse proxy logs the query string of `/?token=<token>` unless you configure it not to. Open the link once, and rotate the token (delete the token file, restart) if a log leaked.
- **Terminal and agent are separate gates.** Beyond loopback, and behind a proxy with `--require-token` or `--public-url`, the terminal is off until `--allow-remote-terminal` and agent sessions are off until `--allow-remote-agent`. Both are flags and have no config key, so a committed `.lif.yml` cannot turn them on. A client that has the token and one of these flags runs commands as the user that runs `lif-serve`: the terminal is a shell, and an agent session in act mode runs commands without asking.
- **`createWorktree` runs commands too.** It runs the `worktree.setup` commands of `.lif.yml` in the new worktree. The client does not choose them, but a repository you cloned may contain them. Check `worktree.setup` before you serve a repository you do not trust, and know that a token holder can trigger them.
- **Loopback is open.** Without a token every local user and program of the machine can use the port. The host and origin checks keep web pages out; they do not keep programs out. A proxy makes that matter: forward only with `--public-url` or `--require-token`.
- **A forwarded request without the flag is refused.** The server refuses `X-Forwarded-*`, `Forwarded` and `Tailscale-User-Login` while it runs without a token. nginx sends none of them by default, so the flag is mandatory there.
- **Do not use `tailscale funnel`.** It publishes the server on the internet, with the token as the only protection.
