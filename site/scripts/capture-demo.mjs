#!/usr/bin/env node
// Captures real `lif` output into site/src/data/demo.json. The site build only reads that file
// and never runs lif. Re-run when the demo store or the CLI/TUI output changes:
//
//   node site/scripts/capture-demo.mjs [--lif <lif binary>] [--tui "<lif binary> tui"]
//
// Everything runs in a throwaway store under $TMPDIR with an isolated HOME and a clean environment
// (no NO_COLOR, CI, LIF_PATH, BEANS_PATH), so the project's own store is never touched.
// Needs `tmux` (real PTY, 24-bit colour) and, without --lif, `go`.
// ANSI colours are mapped to Catppuccin tones; custom.css defines --ctp-<tone> per flavour.
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');
const arg = (name) => { const i = process.argv.indexOf(name); return i < 0 ? undefined : process.argv[i + 1]; };
const sleep = (s) => execFileSync('sleep', [String(s)]);
const q = (s) => `'${String(s).replaceAll("'", "'\\''")}'`;

// ---- throwaway store -------------------------------------------------------------------------
const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'lif-demo-'));
const store = path.join(tmp, 'app'); // dir name becomes project name and ID prefix ("app-")
fs.mkdirSync(store);
fs.mkdirSync(path.join(tmp, 'home'));
const lif = arg('--lif') ?? build();
const tui = arg('--tui') ?? `${lif} tui`;
const env = { PATH: process.env.PATH, HOME: path.join(tmp, 'home') };
const flags = ['--config', path.join(store, '.lif.yml'), '--lif-path', path.join(store, '.lif')];
const run = (...a) => execFileSync(lif, [...a, ...flags], { cwd: store, env, encoding: 'utf8' });

function build() {
	const bin = path.join(tmp, 'lif');
	execFileSync('go', ['build', '-o', bin, './cmd/lif'], { cwd: root, stdio: 'inherit' });
	return bin;
}

execFileSync(lif, ['init', '--profile', 'classic'], { cwd: store, env, stdio: 'ignore' });

const ids = {};
const create = (key, type, title, status, priority, tags, parent, body) => {
	const a = ['create', title, '--type', type, '--status', status, '--json'];
	if (priority) a.push('--priority', priority);
	for (const t of tags) a.push('--tag', t);
	if (parent) a.push('--parent', ids[parent]);
	if (body) a.push('--body', body);
	ids[key] = JSON.parse(run(...a)).id;
};
const body = `Let users reset a forgotten password with a one-time link.

## Acceptance criteria

- [x] The form sends a reset email
- [ ] The link expires after 30 minutes
- [ ] The link works only once
`;
create('v1', 'milestone', 'v1.0 launch', 'in-progress', 'high', ['release']);
create('acc', 'epic', 'Accounts and sign-in', 'in-progress', null, ['backend'], 'v1');
create('mail', 'feature', 'Sign in with email', 'completed', null, ['backend', 'api'], 'acc');
create('hash', 'task', 'Pick a password hashing library', 'completed', null, ['backend', 'security'], 'acc');
create('pw', 'feature', 'Password reset', 'in-progress', 'high', ['backend', 'security'], 'acc', body);
create('pw1', 'task', 'Send reset email', 'todo', null, ['backend'], 'pw');
create('pw2', 'bug', 'Reset link works twice', 'todo', 'critical', ['security'], 'pw');
create('2fa', 'feature', 'Two-factor login', 'draft', 'low', ['security'], 'acc');
create('dash', 'epic', 'Dashboard', 'todo', null, ['ui'], 'v1');
create('feed', 'feature', 'Activity feed', 'todo', null, ['ui', 'api'], 'dash');
create('empty', 'task', 'Write empty-state copy', 'draft', 'low', ['ui', 'docs'], 'dash');
create('v2', 'milestone', 'v1.1 polish', 'todo', null, ['release']);
create('dark', 'feature', 'Dark mode', 'todo', 'low', ['ui'], 'v2');

// ---- ANSI -> HTML ------------------------------------------------------------------------------
// `lif` CLI prints Catppuccin Mocha (internal/ui/theme.go), the TUI hard-codes Macchiato (internal/tui/theme/theme.go).
// Both are mapped to tone names; the site picks the real colours per flavour via --ctp-<tone>.
const PALETTES = [
	{ // mocha
		rosewater: '#f5e0dc', flamingo: '#f2cdcd', pink: '#f5c2e7', mauve: '#cba6f7', red: '#f38ba8', maroon: '#eba0ac',
		peach: '#fab387', yellow: '#f9e2af', green: '#a6e3a1', teal: '#94e2d5', sky: '#89dceb', sapphire: '#74c7ec',
		blue: '#89b4fa', lavender: '#b4befe', text: '#cdd6f4', subtext1: '#bac2de', subtext0: '#a6adc8', overlay2: '#9399b2',
		overlay1: '#7f849c', overlay0: '#6c7086', surface2: '#585b70', surface1: '#45475a', surface0: '#313244', base: '#1e1e2e',
	},
	{ // macchiato
		rosewater: '#f4dbd6', flamingo: '#f0c6c6', pink: '#f5bde6', mauve: '#c6a0f6', red: '#ed8796', maroon: '#ee99a0',
		peach: '#f5a97f', yellow: '#eed49f', green: '#a6da95', teal: '#8bd5ca', sky: '#91d7e3', sapphire: '#7dc4e4',
		blue: '#8aadf4', lavender: '#b7bdf8', text: '#cad3f5', subtext1: '#b8c0e0', subtext0: '#a5adcb', overlay2: '#939ab7',
		overlay1: '#8087a2', overlay0: '#6e738d', surface2: '#5b6078', surface1: '#494d64', surface0: '#363a4f', base: '#24273a',
	},
	{ overlay1: '#7c7c7c' }, // TUI hint grey, not a Catppuccin colour; shown as overlay1
];
const RGB = PALETTES.flatMap((p) => Object.entries(p).map(([name, hex]) => [name, [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16))]));
// 24-bit colours must match a palette (lipgloss rounds by +-1); xterm-256 colours (the TUI's Markdown preview) snap to the nearest tone.
const tone = (r, g, b, snap) => {
	const [name, d] = RGB.map(([n, c]) => [n, Math.max(Math.abs(c[0] - r), Math.abs(c[1] - g), Math.abs(c[2] - b))]).sort((x, y) => x[1] - y[1])[0];
	if (d > 2 && !snap) throw new Error(`colour ${r},${g},${b} is no known Catppuccin tone`);
	return name;
};
const xterm = (n) => {
	if (n < 16) throw new Error(`unsupported ANSI colour ${n}`);
	if (n >= 232) return Array(3).fill(8 + 10 * (n - 232));
	const lv = [0, 95, 135, 175, 215, 255];
	return [lv[Math.floor((n - 16) / 36)], lv[Math.floor((n - 16) / 6) % 6], lv[(n - 16) % 6]];
};
const esc = (s) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
const span = (css, text) => (css ? `<span style="${css}">${esc(text)}</span>` : esc(text));
const cssOf = (s) => {
	let [fg, bg] = [s.fg, s.bg];
	if (s.reverse) [fg, bg] = [bg ?? 'base', fg ?? 'text'];
	return [
		fg && `color:var(--ctp-${fg})`, bg && `background:var(--ctp-${bg})`, s.bold && 'font-weight:700',
		s.italic && 'font-style:italic', s.underline && 'text-decoration:underline', s.dim && 'opacity:.6',
	].filter(Boolean).join(';');
};
function sgr(s, params) {
	const p = params === '' ? [0] : params.split(';').map(Number);
	for (let i = 0; i < p.length; i++) {
		const n = p[i];
		if (n === 0) Object.keys(s).forEach((k) => delete s[k]);
		else if (n === 1) s.bold = true;
		else if (n === 2) s.dim = true;
		else if (n === 3) s.italic = true;
		else if (n === 4) s.underline = true;
		else if (n === 7) s.reverse = true;
		else if (n === 22) { delete s.bold; delete s.dim; }
		else if (n === 23) delete s.italic;
		else if (n === 24) delete s.underline;
		else if (n === 27) delete s.reverse;
		else if (n === 39) delete s.fg;
		else if (n === 49) delete s.bg;
		else if ((n === 38 || n === 48) && p[i + 1] === 2) {
			s[n === 38 ? 'fg' : 'bg'] = tone(p[i + 2], p[i + 3], p[i + 4]);
			i += 4;
		} else if ((n === 38 || n === 48) && p[i + 1] === 5) {
			s[n === 38 ? 'fg' : 'bg'] = tone(...xterm(p[i + 2]), true);
			i += 2;
		} else throw new Error(`unsupported SGR ${p.slice(i).join(';')}`);
	}
}
function ansiToHtml(text) {
	const lines = text.replace(/\r/g, '').split('\n').map((line) => {
		const s = {};
		let out = '', css = '', buf = '';
		const flush = () => { if (buf) out += span(css, buf); buf = ''; };
		// split() alternates text, SGR params, text, ...
		line.split(/\x1b\[([0-9;]*)m/).forEach((part, i) => {
			if (i % 2) return sgr(s, part);
			const next = cssOf(s);
			if (next !== css) { flush(); css = next; }
			buf += part;
		});
		flush();
		return out;
	});
	while (lines.length && lines[lines.length - 1] === '') lines.pop();
	return lines.join('\n');
}

// ---- capture through tmux (real PTY, 24-bit colour) ------------------------------------------
function capture(cols, rows, command, { until, keys = [], settle = until } = {}) {
	const sock = `lifdemo${process.pid}`;
	const t = (...a) => execFileSync('tmux', ['-L', sock, ...a], { encoding: 'utf8' });
	const strip = (s) => s.replace(/\x1b\[[0-9;]*m/g, '');
	const shell = `env -i PATH=${q(env.PATH)} HOME=${q(env.HOME)} TERM=tmux-256color COLORTERM=truecolor ${command}; sleep 60`;
	t('new-session', '-d', '-s', 'd', '-x', cols, '-y', rows, '-c', store, 'sh', '-c', shell);
	const read = () => t('capture-pane', '-p', '-e', '-t', 'd');
	const waitFor = (re) => {
		for (let i = 0; i < 80; i++) {
			const pane = read();
			if (re.test(strip(pane))) return pane;
			sleep(0.25);
		}
		throw new Error(`timeout waiting for ${re} in: ${command}`);
	};
	try {
		waitFor(until);
		for (const k of keys) { t('send-keys', '-t', 'd', k); sleep(0.4); }
		waitFor(settle);
		sleep(0.5);
		return read();
	} finally {
		try { t('kill-server'); } catch {}
	}
}
const lifCmd = (...a) => `${q(lif)} ${[...a, ...flags].map(q).join(' ')}`;

const roadmap = capture(80, 30, lifCmd('roadmap', '--tags'), { until: /priority/ });
const show = capture(80, 30, lifCmd('show', ids.pw), { until: /created/ });
// Narrow roadmap (NARROW columns, lif reflows its tree) for containers too small for 80 columns; the site swaps it in
// with a container query. `lif show` has no useful reflow (80-column rules), so it scrolls inside the window instead.
const NARROW = 42;
const roadmapNarrow = capture(NARROW, 30, lifCmd('roadmap', '--tags'), { until: /priority/ });
const [tuiBin, ...tuiArgs] = tui.split(' ');
const tuiShot = capture(110, 30, [q(tuiBin), ...tuiArgs.map(q), ...flags.map(q)].join(' '), {
	until: /Browse/,
	keys: ['Right', 'Down', 'Right', 'Down', 'Right'], // expand milestone, epic, feature; select "Password reset"
	settle: new RegExp(`Title · ${ids.pw}`),
});

// ---- Markdown file preview (front matter + body), highlighted with Catppuccin tones -----------
const file = fs.readdirSync(path.join(store, '.lif')).find((f) => f.startsWith(`${ids.pw}--`));
const c = (name, text, extra = '') => `<span style="color:var(--ctp-${name})${extra}">${esc(text)}</span>`;
let inFront = false;
const fileHtml = fs.readFileSync(path.join(store, '.lif', file), 'utf8').trimEnd().split('\n').map((line, i) => {
	if (line === '---') { inFront = i === 0; return c('overlay1', line); }
	if (inFront) {
		if (line.startsWith('#')) return c('overlay1', line, ';font-style:italic');
		const m = line.match(/^(\s*)(-\s+)?(?:([\w-]+):)?\s*(.*)$/);
		const [, ind, dash = '', key, val] = m;
		return esc(ind) + c('overlay2', dash) + (key ? c('blue', key) + c('overlay2', ':') + (val ? ' ' : '') : '') + (val ? c(/^\d{4}-/.test(val) ? 'peach' : 'green', val) : '');
	}
	if (line.startsWith('#')) return c('mauve', line, ';font-weight:700');
	const box = line.match(/^(- \[[ x]\])( .*)$/);
	return box ? c('overlay2', box[1]) + esc(box[2]) : esc(line);
}).join('\n');

// ---- board data (Web tab mock) ----------------------------------------------------------------
const board = JSON.parse(run('list', '--json')).map((b) => ({
	id: b.id, title: b.title, type: b.type, status: b.status, priority: b.priority ?? '', tags: b.tags ?? [],
}));

const git = (...a) => execFileSync('git', ['-C', root, ...a], { encoding: 'utf8' }).trim();
const out = {
	// --tui-from is free text for the TUI source when it comes from another tree, e.g. "milestone/0.13 5cf6d40".
	generated: { date: new Date().toISOString().slice(0, 10), cli: `${git('rev-parse', '--abbrev-ref', 'HEAD')} ${git('rev-parse', '--short', 'HEAD')}`, tui: arg('--tui-from') ?? 'same binary as cli' },
	roadmap: { command: 'lif roadmap --tags', html: ansiToHtml(roadmap), narrow: ansiToHtml(roadmapNarrow), narrowCols: NARROW },
	show: { command: `lif show ${ids.pw}`, html: ansiToHtml(show) },
	tui: { command: 'lif tui', html: ansiToHtml(tuiShot) },
	file: { name: file, html: fileHtml },
	board,
};
const target = path.join(root, 'site/src/data/demo.json');
fs.mkdirSync(path.dirname(target), { recursive: true });
fs.writeFileSync(target, `${JSON.stringify(out, null, 1)}\n`);
fs.rmSync(tmp, { recursive: true, force: true });
console.log(`wrote ${path.relative(root, target)}`);
