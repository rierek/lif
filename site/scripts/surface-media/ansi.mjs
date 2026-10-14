export const ansiToHtmlX = (()=>{
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
return ansiToHtml;})();
