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
			// D36: the sidebar shows only the current chapter (top-level group below).
			routeMiddleware: './src/routeData.ts',
			// custom.css defines the --ctp-* tones for these two flavours (captured lif output); change both together.
			plugins: [catppuccin({ dark: { flavor: 'mocha', accent: 'teal' }, light: { flavor: 'latte', accent: 'teal' } })],
			social: [{ icon: 'github', label: 'GitHub', href: 'https://github.com/xRiErOS/lif' }],
			// Each top-level group is a chapter. Header chapters: Overview, Features, Use Cases, Reference, Resources
			// (= Blog, Releases, Contribute, About); see src/components/nav.ts.
			sidebar: [
				{
					label: 'Overview',
					items: ['overview', 'overview/installation', 'overview/first-steps', 'overview/concepts', 'overview/agent-onboarding'],
				},
				{
					label: 'Features',
					items: [
						'features',
						'features/markdown-native',
						'features/project-planning',
						'features/dependencies',
						'features/surface-independence',
						'features/automation',
						'features/git-native',
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
						'reference/json-schemas',
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
				{ label: 'Releases', items: [{ autogenerate: { directory: 'releases' } }] },
				{ label: 'Contribute', items: ['contribute'] },
				{ label: 'About', items: ['about'] },
			],
		}),
	],
});
