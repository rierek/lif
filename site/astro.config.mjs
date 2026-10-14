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
		// D38/D53/D55: ```mermaid fences become diagrams in the browser (theme follows data-theme, colours: custom.css).
		// Must come before starlight(): its remark step runs before Expressive Code turns the fences into code blocks.
		mermaid({ enableLog: false }),
		starlight({
			title: 'Lif',
			description: 'Lightweight, Markdown-native work management for humans, AI agents and automation.',
			components: { Header: './src/components/Header.astro', Hero: './src/components/Hero.astro' },
			customCss: ['./src/styles/custom.css'],
			// D36/D51: sidebar mode (all chapters or current chapter only) is one switch in src/routeData.ts.
			routeMiddleware: './src/routeData.ts',
			// custom.css defines the --ctp-* tones for these two flavours (captured lif output); change both together.
			plugins: [catppuccin({ dark: { flavor: 'mocha', accent: 'teal' }, light: { flavor: 'latte', accent: 'teal' } })],
			social: [{ icon: 'github', label: 'GitHub', href: 'https://github.com/xRiErOS/lif' }],
			// Each top-level group is a chapter. Header chapters: Overview, Features, Use Cases, Reference, Resources
			// (= Blog, Releases, Contribute, About); see src/components/nav.ts. D57: Blog is its own sidebar chapter;
			// Releases, Contribute and About form the sidebar chapter "Resources".
			sidebar: [
				{
					label: 'Overview',
					items: [
						'overview',
						'overview/installation',
						'overview/first-steps',
						'overview/concepts',
						'overview/agent-onboarding',
						'overview/store-management',
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
	],
});
