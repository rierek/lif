import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import { fileURLToPath } from 'node:url';
import { ansiToHtmlX as toHtml } from './ansi.mjs';
// Captures the CLI, TUI and editor media for features/surface-independence into src/data/surface-media.json.
//   LIF_DEMO_APP=<demo store dir with .lif/ and .lif.yml> LIF_BIN=<lif 0.14 binary> node site/scripts/surface-media/capture.mjs [cliCols] [tuiCols]
// Needs tmux and nvim. The demo store is the "app" project that capture-demo.mjs builds (it deletes it afterwards,
// so keep it or recreate it by hand; scripting that store is a known gap). The web UI shots (public/media/web-board-*.webp) are not covered here.
const S = process.env.LIF_DEMO_APP, LIF = process.env.LIF_BIN;
if (!S || !LIF) throw new Error('set LIF_DEMO_APP and LIF_BIN');
const here = fileURLToPath(new URL('.', import.meta.url));
const sleep = (s) => execFileSync('sleep', [String(s)]);
const strip = (s) => s.replace(/\x1b\[[0-9;]*m/g, '');
const flags = `--config ${S}/.lif.yml --lif-path ${S}/.lif`;
function cap(cols, rows, cmd, until, keys = [], settle = until) {
  const t = (...a) => execFileSync('tmux', ['-L', 'mk', ...a], { encoding: 'utf8' });
  try { t('kill-server'); } catch {}
  t('new-session', '-d', '-s', 'd', '-x', cols, '-y', rows, '-c', S, 'sh', '-c', `env -i PATH=/opt/homebrew/bin:/usr/bin:/bin HOME=/tmp/vh TERM=tmux-256color COLORTERM=truecolor ${cmd}; sleep 60`);
  const read = () => t('capture-pane', '-p', '-e', '-t', 'd');
  const wait = (re) => { for (let i = 0; i < 60; i++) { const p = read(); if (re.test(strip(p))) return; sleep(0.25); } throw new Error('timeout ' + cmd); };
  wait(until);
  for (const k of keys) { t('send-keys', '-t', 'd', k); sleep(0.4); }
  wait(settle); sleep(0.5);
  const out = read(); t('kill-server'); return out;
}
const W = +process.argv[2] || 56, TW = +process.argv[3] || 72;
const list = cap(80, 34, `${LIF} list ${flags}`, /priority/);
const show = cap(80, 40, `${LIF} show app-0aqa ${flags}`, /created/);
const tui = cap(TW, 26, `${LIF} tui ${flags}`, /Browse/, ['Right', 'Down', 'Right', 'Down', 'Right'], /Title · app-0aqa/);
const ed = cap(64, 22, `nvim -u ${here}init.lua -n .lif/app-0aqa--password-reset.md`, /Acceptance/);
const out = { cliCols: W, tuiCols: TW, list: toHtml(list), show: toHtml(show), tui: toHtml(tui), editor: toHtml(ed) };
fs.writeFileSync(fileURLToPath(new URL('../../src/data/surface-media.json', import.meta.url)), JSON.stringify(out, null, 1) + '\n');
for (const k of ['list','show','tui','editor']) console.log('=== ' + k + '\n' + out[k].replace(/<[^>]+>/g, '').replace(/&gt;/g,'>').replace(/&lt;/g,'<').replace(/&amp;/g,'&'));
