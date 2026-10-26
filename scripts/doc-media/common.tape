# Gemeinsame Einstellungen und Start der TUI (beans-wy2o).
# Erwartet LIF_DOC_ROOT (Wegwerf-Store) und LIF_DOC_BIN (Verzeichnis mit lif)
# in der Umgebung; doc-media.sh setzt beides.
Set Shell bash
Set Width 1280
Set Height 800
Set FontSize 14
Set FontFamily "JetBrains Mono"
Set Theme "Catppuccin Mocha"
Set Padding 10
Set TypingSpeed 0

Hide
Type "export HOME=$LIF_DOC_ROOT/home PATH=$LIF_DOC_BIN:$PATH PS1=; cd $LIF_DOC_ROOT; clear" Enter
Type "lif tui" Enter
Sleep 2s
Show
