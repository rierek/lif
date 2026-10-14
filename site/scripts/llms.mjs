// Writes dist/llms.txt (index) and dist/llms-full.txt (all page text) from wiki/pages. No dependencies.
import { readFileSync, readdirSync, statSync, writeFileSync } from 'node:fs';
import { join, relative } from 'node:path';

const pages = new URL('../../wiki/pages/', import.meta.url).pathname;
const out = new URL('../dist/', import.meta.url).pathname;
const site = 'https://xrieros.github.io/lif/';
const chapters = ['overview', 'features', 'use-cases', 'reference', 'blog', 'releases', 'contribute', 'about'];

const walk = (d) => readdirSync(d).flatMap((f) => {
	const p = join(d, f);
	return statSync(p).isDirectory() ? walk(p) : /\.mdx?$/.test(f) && !f.startsWith('_') ? [p] : [];
});

const rows = walk(pages).map((file) => {
	const rel = relative(pages, file).replace(/\.mdx?$/, '');
	const m = readFileSync(file, 'utf8').match(/^---\n([\s\S]*?)\n---\n?([\s\S]*)$/);
	const fm = (k) => (m[1].match(new RegExp(`^${k}:\\s*(.*)$`, 'm')) || [])[1]?.replace(/^["']|["']$/g, '') ?? '';
	const body = m[2].split('\n').filter((l) => !/^import\s/.test(l) && !/^<\/?[A-Z][\w.]*[^>]*\/?>\s*$/.test(l)).join('\n').trim();
	const slug = rel.replace(/(^|\/)index$/, '');
	const top = slug.split('/')[0];
	return { title: fm('title'), description: fm('description'), url: site + (slug ? slug + '/' : ''), body,
		order: [chapters.indexOf(top) < 0 ? -1 : chapters.indexOf(top), rel.endsWith('index') ? 0 : 1, rel] };
}).sort((a, b) => a.order[0] - b.order[0] || a.order[1] - b.order[1] || a.order[2].localeCompare(b.order[2]));

const head = '# Lif\n\n> Lightweight, Markdown-native work management for humans, AI agents and automation.\n\n';
writeFileSync(out + 'llms.txt', head + rows.map((r) => `- [${r.title}](${r.url})${r.description ? ': ' + r.description : ''}`).join('\n') + '\n');
writeFileSync(out + 'llms-full.txt', head + rows.map((r) => `# ${r.title}\n\nSource: ${r.url}\n\n${r.body}\n`).join('\n---\n\n'));
console.log(`llms: ${rows.length} pages`);
