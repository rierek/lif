// Header navigation (usememos.com style). Groups with `items` become drop-downs with a one-line description per
// entry. Hrefs are site-relative and must match page slugs in wiki/pages (chapters: see sidebar in astro.config.mjs).
export interface NavItem {
	title: string;
	description: string;
	href: string;
}
export type NavEntry = { label: string; href: string } | { label: string; items: NavItem[] };

export const nav: NavEntry[] = [
	{ label: 'Overview', href: '/overview/' },
	{
		label: 'Features',
		items: [
			{ title: 'All features', description: 'Everything Lif does, at a glance', href: '/features/' },
			{ title: 'Markdown-native', description: 'Plain files you read, diff and edit', href: '/features/markdown-native/' },
			{ title: 'Flexible planning', description: 'Types, statuses and hierarchy presets', href: '/features/project-planning/' },
			{ title: 'Dependencies', description: 'Blockers and what is ready next', href: '/features/dependencies/' },
			{ title: 'Surface independence', description: 'CLI, TUI, web UI or any editor', href: '/features/surface-independence/' },
			{ title: 'Automation and agents', description: 'JSON, GraphQL and lif prime', href: '/features/automation/' },
			{ title: 'Git-native', description: 'Items versioned with your code', href: '/features/git-native/' },
		],
	},
	{
		label: 'Use Cases',
		items: [
			{ title: 'All use cases', description: 'Real problems, solved with Lif', href: '/use-cases/' },
			{ title: 'Personal tasks', description: 'Your to-do list in plain files', href: '/use-cases/personal-tasks/' },
			{ title: 'Home lab', description: 'Servers, services and chores', href: '/use-cases/homelab/' },
			{ title: 'Software project', description: 'Milestones and epics beside the code', href: '/use-cases/software-projects/' },
			{ title: 'Release', description: 'Plan, track and ship a release', href: '/use-cases/release-management/' },
			{ title: 'Writing or research', description: 'Chapters, sources, open questions', href: '/use-cases/writing-research/' },
		],
	},
	{ label: 'Reference', href: '/reference/' },
	{
		label: 'Resources',
		items: [
			{ title: 'Blog', description: 'Experiments and field notes', href: '/blog/' },
			{ title: 'Releases', description: 'What changed in each version', href: '/releases/' },
			{ title: 'Contribute', description: 'Repo layout and where to change what', href: '/contribute/' },
			{ title: 'About', description: 'What Lif is and where it comes from', href: '/about/' },
		],
	},
];
