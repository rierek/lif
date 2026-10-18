// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import { fileURLToPath } from 'node:url';
import catppuccin from '@catppuccin/starlight';
import mermaid from 'astro-mermaid';

export default defineConfig({
	vite: {
		preview: { allowedHosts: ['macm.banteng-cardassian.ts.net'] },
		plugins: [{
			// Content lives in ../wiki: resolve its bare imports from site/node_modules.
			name: 'wiki-resolve',
			enforce: 'pre',
			async resolveId(id, importer, opts) {
				if (!importer || !/[\\/]wiki[\\/]/.test(importer) || !/^(@astrojs|@lucide)\//.test(id)) return null;
				return this.resolve(id, fileURLToPath(new URL('./package.json', import.meta.url)), { ...opts, skipSelf: true });
			},
		}],
	},
	site: 'https://rierek.github.io',
	base: '/lif',
	integrations: [
		starlight({
			title: 'Lif',
			description: 'Lightweight, Markdown-native work management for humans, AI agents and automation.',
			components: { Header: './src/components/Header.astro', Hero: './src/components/Hero.astro' },
			customCss: ['./src/styles/custom.css', './src/styles/elements.css'],
			// Code blocks use the same Catppuccin flavours as the site; VISUAL-SYSTEM.md §9. Expressive Code lifts token
			// contrast against the theme base; the panes sit on mantle (--lif-surface), so the floor is above 4.5:1 with
			// room to spare. Options must stay serializable (a function here breaks the dev stylesheet route).
			expressiveCode: {
				themes: ['catppuccin-mocha', 'catppuccin-latte'],
				minSyntaxHighlightingColorContrast: 6.5,
				styleOverrides: {
					borderRadius: '0.5rem',
					borderWidth: '1px',
					codeFontSize: '0.875rem',
					codeLineHeight: '1.7',
					textMarkers: { lineMarkerAccentWidth: '0px', backgroundOpacity: '35%' },
				},
			},
			// D36/D51: sidebar mode (all chapters or current chapter only) is one switch in src/routeData.ts.
			routeMiddleware: './src/routeData.ts',
			// custom.css defines the --ctp-* tones for these two flavours (captured lif output); change both together.
			plugins: [catppuccin({ dark: { flavor: 'mocha', accent: 'teal' }, light: { flavor: 'latte', accent: 'teal' } })],
			social: [{ icon: 'github', label: 'GitHub', href: 'https://github.com/rierek/lif' }],
			// Each top-level group is a chapter. Header chapters: Overview, Features, Use Cases, Reference, Resources
			// (= Blog, Releases, Contribute, About); see src/components/nav.ts. D57: Blog is its own sidebar chapter;
			// Releases, Contribute and About form the sidebar chapter "Resources".
			sidebar: [
				{
					label: 'Overview',
					items: [
						'overview',
						'overview/installation',
						'overview/upgrading',
						'overview/first-steps',
						'overview/concepts',
						'overview/agent-onboarding',
						'overview/store-management',
						'overview/troubleshooting',
					],
				},
				{
					label: 'Features',
					items: [
						'features',
						'features/files-in-your-repo',
						'features/automation',
						'features/dependencies',
						'features/project-planning',
						'features/surface-independence',
					],
				},
				{
					label: 'Use Cases',
					items: [
						'use-cases',
						'use-cases/personal-tasks',
						'use-cases/homelab',
						'use-cases/software-projects',
						'use-cases/release-management',
						'use-cases/writing-research',
					],
				},
				{
					label: 'Reference',
					items: [
						'reference',
						{ label: 'Commands', items: [{ autogenerate: { directory: 'reference/commands' } }] },
						'reference/item-file-format',
						'reference/configuration',
						'reference/search-syntax',
						'reference/json-output',
						'reference/graphql',
						'reference/web-server',
						'reference/remote-access',
						{
							label: 'TUI',
							items: [
								'reference/tui/start-and-repositories',
								'reference/tui/views',
								'reference/tui/editing-items',
								'reference/tui/key-bindings',
								'reference/tui/settings-and-themes',
							],
						},
						'reference/glossary',
					],
				},
				{ label: 'Blog', items: [{ autogenerate: { directory: 'blog' } }] },
				{
					label: 'Resources',
					items: [{ label: 'Releases', items: [{ autogenerate: { directory: 'releases' } }] }, 'known-limitations', 'contribute', 'about'],
				},
			],
		}),

		// D38/D53/D55: ```mermaid fences become diagrams in the browser (theme follows data-theme, colours: custom.css).
		// Must come after starlight(): astro-mermaid replaces markdown.processor with a copy, and Expressive Code hooks
		// into the processor that is current when starlight() runs. Before starlight(), .md pages (Reference, Contribute)
		// rendered code blocks with default Expressive Code options and a stylesheet the build never wrote.
		mermaid({ enableLog: false }),
	],
});
