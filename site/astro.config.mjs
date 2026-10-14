// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import { fileURLToPath } from 'node:url';
import catppuccin from '@catppuccin/starlight';

export default defineConfig({
	vite: {
		preview: { allowedHosts: ['macm.banteng-cardassian.ts.net'] },
		plugins: [{
			// Content lives in ../wiki/pages: resolve its bare imports from site/node_modules.
			name: 'wiki-pages-resolve',
			enforce: 'pre',
			async resolveId(id, importer, opts) {
				if (!importer || !/[\\/]wiki[\\/]pages[\\/]/.test(importer) || !/^(@astrojs|@lucide)\//.test(id)) return null;
				return this.resolve(id, fileURLToPath(new URL('./package.json', import.meta.url)), { ...opts, skipSelf: true });
			},
		}],
	},
	site: 'https://xrieros.github.io',
	base: '/lif',
	integrations: [
		starlight({
			title: 'Lif',
			description: 'Lightweight, Markdown-native work management for humans, AI agents and automation.',
			components: { Header: './src/components/Header.astro', Hero: './src/components/Hero.astro' },
			customCss: ['./src/styles/custom.css'],
			// custom.css defines the --ctp-* tones for these two flavours (captured lif output); change both together.
			plugins: [catppuccin({ dark: { flavor: 'mocha', accent: 'teal' }, light: { flavor: 'latte', accent: 'teal' } })],
			social: [{ icon: 'github', label: 'GitHub', href: 'https://github.com/xRiErOS/lif' }],
			sidebar: [
				{ label: 'Getting Started', items: [{ autogenerate: { directory: 'getting-started' } }] },
				{ label: 'Guides', items: [{ autogenerate: { directory: 'guides' } }] },
				{ label: 'Reference', items: [{ autogenerate: { directory: 'reference', collapsed: true } }] },
				{ label: 'Releases', items: [{ autogenerate: { directory: 'releases' } }] },
				{ label: 'Contribute', items: [{ autogenerate: { directory: 'contribute' } }] },
				{ label: 'About', items: [{ autogenerate: { directory: 'about' } }] },
			],
		}),
	],
});
