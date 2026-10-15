#!/usr/bin/env bash
# Ort-Guard für `just install` (beans-nm1j).
#
# Verhindert, dass ein Build aus einem Wegwerf- oder Container-Baum das
# systemweit installierte Binary still ersetzt (Anlass: beans-kjf3, ein
# eigenständiger Clone in /private/tmp überschrieb ein 30 Minuten zuvor aus
# repo/ installiertes Binary). Die Erkennung stützt sich nicht auf den
# Pfadnamen, sondern auf git-Plumbing:
#
#   1. `--git-dir` vs. `--git-common-dir`: ungleich heißt linked worktree
#      (z.B. worktrees/<id>) -- die teilen sich die lokale Git-Config mit
#      dem Hauptbaum, sind aber selbst nicht der Hauptbaum.
#   2. `lif.installHome`-Marker in der lokalen Git-Config: den setzt nur
#      der eine als Installationsquelle vorgesehene Checkout (einmalig,
#      siehe wiki/overview/upgrading.md, Abschnitt Contributor tooling). Ein frischer Clone -- Wegwerf- oder
#      Ad-hoc-Baum -- hat ihn nicht, selbst wenn er (wie im Anlass) sein
#      eigener Hauptbaum ist.
#
# Beide Prüfungen zusammen lassen nur den einen dafür markierten Hauptbaum
# durch. Bewusster Override ohne den Guard aufzuweichen: LIF_BIN_DIR auf
# ein eigenes Verzeichnis umleiten, oder LIF_INSTALL_FORCE=1 setzen.
set -euo pipefail

if [ "${LIF_INSTALL_FORCE:-}" = "1" ]; then
    exit 0
fi

fail() {
    echo "just install: $1" >&2
    echo "Für einen Container-, Wegwerf- oder Testbaum: just install-wip" >&2
    exit 1
}

git_dir=$(git rev-parse --git-dir 2>/dev/null) || fail "kein Git-Baum erkennbar."
common_dir=$(git rev-parse --git-common-dir 2>/dev/null) || fail "kein Git-Baum erkennbar."

if [ "$git_dir" != "$common_dir" ]; then
    fail "dies ist ein linked worktree, nicht der Hauptbaum."
fi

marker=$(git config --local --get lif.installHome 2>/dev/null || true)
if [ "$marker" != "true" ]; then
    fail "dieser Baum ist nicht als Installationsquelle markiert (git config lif.installHome true)."
fi

exit 0
