# lif dev CLI — just ist der Einstieg (dev-cli-canon), mise bleibt die
# Build-Implementierung des Upstream-Forks.

# Zielverzeichnis der Installation. NICHT ~/.local/bin wie `mise run install`:
# in Eriks PATH gewinnt /opt/homebrew/bin, und dort liegt das aktive Binary.
bin_dir := env_var_or_default("LIF_BIN_DIR", "/opt/homebrew/bin")

# Remote to push release tags to. This workspace keeps `origin` on the
# read-only hmans/beans upstream and `fork` on the writable rierek/lif;
# a plain clone of the fork itself has no separate `fork` remote, so this
# picks `fork` when present and falls back to `origin` otherwise.
release_remote := env_var_or_default("LIF_RELEASE_REMOTE", `git remote get-url fork >/dev/null 2>&1 && echo fork || echo origin`)

# GitHub repo (owner/name) the release recipes query via `gh -R`.
release_repo := "rierek/lif"

# Name-Suffix des WIP-Binaries für Testläufe. Bewusst ein eigener Name statt
# eines zweiten Zielverzeichnisses: in Eriks PATH gewinnt {{bin_dir}}, ein
# dort abgelegter Teststand würde also das reguläre Binary verdecken.
wip_suffix := env_var_or_default("LIF_WIP_SUFFIX", "wip")

# List available recipes
default:
    @just --list

# Build lif und lif-serve mit Version/Commit/Date-Stempel
build:
    mise run build

# Build und Installation nach {{bin_dir}} — macht den Stand systemweit wirksam.
# Guard (beans-nm1j, scripts/install-guard.sh): bricht ab, wenn dieser Baum
# nicht der als Installationsquelle markierte Hauptbaum ist, und nennt
# `just install-wip` als Alternative. Override: LIF_BIN_DIR umleiten oder
# LIF_INSTALL_FORCE=1.
install: build
    bash scripts/install-guard.sh
    install -m 755 lif lif-serve "{{bin_dir}}/"
    @"{{bin_dir}}/lif" version

# Teststand des ungemergten CLI unter eigenem Namen nach {{bin_dir}}
# installieren — aus jedem Baum erlaubt, auch aus Container-Worktrees
# (beans-nji1). Beide Binaries folgen derselben Namensregel, keines
# überschreibt je sein reguläres Gegenstück.
install-wip: build
    install -m 755 lif "{{bin_dir}}/lif-{{wip_suffix}}"
    install -m 755 lif-serve "{{bin_dir}}/lif-serve-{{wip_suffix}}"
    @"{{bin_dir}}/lif-{{wip_suffix}}" version

# Wegwerf-Store plus zsh mit geladener Vervollständigung in einer
# tmux-Sitzung. Für Hand-Tests der interaktiven Oberfläche; das `lif`
# darin kann den echten Store aus keinem Verzeichnis treffen.

# Wurzel der Bühne. Einzige Quelle dieses Werts: das Skript führt keinen
# eigenen Default, weil der Pfad hinter einem `rm -rf` steht.
demo_root := env_var_or_default("LIF_DEMO_ROOT", "/tmp/lif-demo")

# Interaktive tmux-Bühne mit Wegwerf-Store starten (beans-ejxf)
demo SESSION='lif-demo':
    bash scripts/demo-stage.sh "{{ SESSION }}" "{{ demo_root }}"

# Bühne beenden und ihren Wegwerf-Store restlos entfernen
demo-stop SESSION='lif-demo':
    -tmux kill-session -t "{{ SESSION }}"
    rm -rf "{{ demo_root }}"

# Teststand wieder aus {{bin_dir}} entfernen
uninstall-wip:
    rm -f "{{bin_dir}}/lif-{{wip_suffix}}" "{{bin_dir}}/lif-serve-{{wip_suffix}}"

# Paket mit der //go:generate-Zeile für gqlgen (einzige Quelle des Pfads)
gqlgen_pkg := "./internal/graph"

# Regenerate gqlgen output (internal/graph/generated.go, pkg/itemgraph/model/models_gen.go)
gqlgen:
    go generate {{gqlgen_pkg}}

# Run the Go test suite, e.g. `just test ./pkg/item/...`
test ARGS='./...':
    go test -count=1 {{ARGS}}

# Run the frontend unit tests, e.g. `just test-web --project server`
test-web ARGS='':
    cd frontend && mise exec -- pnpm test {{ARGS}}

# Type- and a11y-check the frontend (svelte-check)
check-web:
    cd frontend && mise exec -- pnpm check

# Run the frontend end-to-end suite, e.g. `just test-e2e e2e/filter.spec.ts`
test-e2e ARGS='':
    mise run build:embed
    cd frontend && mise exec -- pnpm test:e2e {{ARGS}}

# Run the Go test suite under the race detector
test-race ARGS='./...':
    go test -race -count=1 {{ARGS}}

# Prove lif-pick-widget's zsh contract against a stubbed lif (beans-fcj0)
test-zsh:
    bash scripts/test-zsh-widget.sh

# Validate .goreleaser.yaml (schema, templates) without building anything
release-check:
    mise exec goreleaser@latest -- goreleaser check

# Build every release platform locally without publishing (goreleaser --snapshot)
release-snapshot: release-check
    mise run build:embed
    mise exec goreleaser@latest -- goreleaser release --snapshot --clean --skip=publish

# Cut+push a release tag (patch|minor|major) via svu, then watch Actions
release LEVEL='patch': test release-snapshot
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -n "$(git status --porcelain)" ]; then
        echo "working tree not clean, aborting" >&2
        exit 1
    fi
    mise run "release:{{LEVEL}}"
    NEW_TAG=$(git describe --tags --abbrev=0)
    echo "About to push $(git branch --show-current) and tag ${NEW_TAG} to remote '{{release_remote}}'."
    read -p "Continue? [y/N] " confirm
    case "$confirm" in
        y|Y) ;;
        *)
            echo "Aborted (tag ${NEW_TAG} was created locally; delete with: git tag -d ${NEW_TAG})." >&2
            exit 1
            ;;
    esac
    # Runs for a re-pushed tag name keep the same branch label; remember the
    # highest existing run ID so only a run created by this push is watched.
    LAST_ID=$(gh run list -R {{release_repo}} -w release --branch "$NEW_TAG" -L 20 --json databaseId -q 'map(.databaseId) | max // 0')
    git push {{release_remote}} HEAD
    git push {{release_remote}} "$NEW_TAG"
    RUN_ID=""
    for i in $(seq 1 20); do
        RUN_ID=$(gh run list -R {{release_repo}} -w release --branch "$NEW_TAG" -L 20 --json databaseId -q "map(select(.databaseId > $LAST_ID)) | map(.databaseId) | max // empty" 2>/dev/null || true)
        [ -n "$RUN_ID" ] && break
        sleep 3
    done
    if [ -z "$RUN_ID" ]; then
        echo "no release run appeared yet, check https://github.com/{{release_repo}}/actions" >&2
        exit 1
    fi
    gh run watch -R {{release_repo}} "$RUN_ID" --exit-status

# Watch the most recent release Actions run without cutting a new release
release-watch:
    #!/usr/bin/env bash
    set -euo pipefail
    RUN_ID=$(gh run list -R {{release_repo}} -w release -L1 --json databaseId -q '.[0].databaseId')
    gh run watch -R {{release_repo}} "$RUN_ID"

# Remove local goreleaser snapshot output (dist/ is gitignored)
release-clean:
    rm -rf dist

# Generate command reference pages from the Cobra tree
docs-gen:
    go run ./cmd/lif-docgen wiki/reference/commands

# Build the public docs site into site/dist
docs-build: docs-gen
    cd site && npm ci && npm run build

# Serve the docs site with live reload
docs-dev: docs-gen
    cd site && npm run dev
