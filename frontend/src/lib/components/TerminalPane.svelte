<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { ui } from '#lib/uiState.svelte.js';
  import { wsUrl } from '#lib/paths.js';
  import { theme } from '#lib/theme.svelte.js';
  import { Terminal } from '@xterm/xterm';
  import { FitAddon } from '@xterm/addon-fit';
  import { WebLinksAddon } from '@xterm/addon-web-links';
  import '@xterm/xterm/css/xterm.css';
  interface Props {
    sessionId: string;
    hideToolbar?: boolean;
    onSessionEnd?: () => void;
  }

  let { sessionId, hideToolbar = false, onSessionEnd }: Props = $props();

  let terminalEl: HTMLDivElement | undefined = $state();
  let terminal: Terminal | undefined;
  let fitAddon: FitAddon | undefined;
  let ws: WebSocket | undefined;
  let resizeObserver: ResizeObserver | undefined;
  let reconnectTimeout: ReturnType<typeof setTimeout> | undefined;

  /** xterm theme from the active flavour's tones (--ctp-*), mapped like Catppuccin's terminal themes. */
  function resolveTheme() {
    const style = getComputedStyle(document.documentElement);
    const ctp = (tone: string) => style.getPropertyValue(`--ctp-${tone}`).trim();
    return {
      background: style.getPropertyValue('--th-surface').trim(),
      foreground: ctp('text'),
      cursor: ctp('rosewater'),
      selectionBackground: ctp('surface2'),
      black: ctp('surface1'),
      red: ctp('red'),
      green: ctp('green'),
      yellow: ctp('yellow'),
      blue: ctp('blue'),
      magenta: ctp('pink'),
      cyan: ctp('teal'),
      white: ctp('subtext1'),
      brightBlack: ctp('surface2'),
      brightRed: ctp('red'),
      brightGreen: ctp('green'),
      brightYellow: ctp('yellow'),
      brightBlue: ctp('blue'),
      brightMagenta: ctp('pink'),
      brightCyan: ctp('teal'),
      brightWhite: ctp('subtext0')
    };
  }

  // Re-apply the terminal theme when the UI scheme or flavour changes
  $effect(() => {
    void theme.resolved; // the CSS tokens read above change with the scheme and the flavour
    void theme.darkFlavour;
    const next = resolveTheme();
    if (terminal) terminal.options.theme = next;
  });

  function connect(term: Terminal) {
    const socket = new WebSocket(wsUrl('/api/terminal'));
    socket.binaryType = 'arraybuffer';
    ws = socket;

    socket.addEventListener('open', () => {
      socket.send(
        JSON.stringify({
          type: 'init',
          sessionId,
          cols: term.cols,
          rows: term.rows
        })
      );
    });

    socket.addEventListener('message', (e) => {
      if (e.data instanceof ArrayBuffer) {
        term.write(new Uint8Array(e.data));
      }
    });

    socket.addEventListener('close', (e) => {
      if (e.code === 1000 && e.reason === 'shell exited') {
        term.write('\r\n\x1b[90m[session ended]\x1b[0m\r\n');
        onSessionEnd?.();
      } else {
        // Connection lost — reset terminal and reconnect
        // The server will replay scrollback to restore the display
        reconnectTimeout = setTimeout(() => {
          term.reset();
          connect(term);
        }, 500);
      }
    });
  }

  onMount(() => {
    if (!terminalEl) return;

    const term = new Terminal({
      cursorBlink: true,
      fontSize: 13,
      fontFamily: '"JetBrainsMono NF", ui-monospace, SFMono-Regular, "SF Mono", Menlo, monospace',
      theme: resolveTheme()
    });

    const fit = new FitAddon();
    fitAddon = fit;
    term.loadAddon(fit);
    term.loadAddon(
      new WebLinksAddon((_event, uri) => {
        window.open(uri, '_blank', 'noopener');
      })
    );
    term.open(terminalEl);

    // Initial fit after DOM layout settles
    requestAnimationFrame(() => fit.fit());

    // Connect WebSocket
    connect(term);

    // User input → WebSocket
    term.onData((data) => {
      if (ws?.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 'input', data }));
      }
    });

    // Terminal resize → WebSocket
    term.onResize(({ cols, rows }) => {
      if (ws?.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 'resize', cols, rows }));
      }
    });

    // Re-fit when container resizes (e.g., SplitPane drag)
    resizeObserver = new ResizeObserver(() => {
      requestAnimationFrame(() => fit.fit());
    });
    resizeObserver.observe(terminalEl);

    terminal = term;
  });

  onDestroy(() => {
    if (reconnectTimeout) clearTimeout(reconnectTimeout);
    resizeObserver?.disconnect();
    ws?.close();
    terminal?.dispose();
  });
</script>

{#if hideToolbar}
  <div class="flex h-full min-h-0 flex-col bg-surface">
    <div class="min-h-0 flex-1 p-2" bind:this={terminalEl}></div>
  </div>
{:else}
  <div class="flex h-full min-h-0 flex-col bg-surface">
    <div class="pane-toolbar">
      <span>Terminal</span>
      <div class="flex-1"></div>
      <button onclick={() => ui.toggleTerminal()} class="btn-icon" title="Close"> &#x2715; </button>
    </div>
    <div class="min-h-0 flex-1 p-2" bind:this={terminalEl}></div>
  </div>
{/if}
