// Header navigation (usememos.com style). Groups with `items` become drop-downs with a one-line description per
// entry. Hrefs are site-relative and must match page slugs in wiki/pages (chapters: see sidebar in astro.config.mjs).
// "Resources" lists Blog, Releases, Contribute and About. In the sidebar (D57) Blog is its own chapter, while
// Releases, Known limitations, Contribute and About form the chapter "Resources".
export interface NavItem {
	title: string;
	description: string;
	href: string;
}
export type NavEntry = { label: string; href: string } | { label: string; items: NavItem[] };

export const nav: NavEntry[] = [
	{
		label: 'Overview',
		items: [
			{ title: 'Lif in brief', description: 'What Lif is and where to go next', href: '/overview/' },
			{ title: 'Installation', description: 'Install the lif binary', href: '/overview/installation/' },
			{ title: 'First steps', description: 'Store, preset and your first items', href: '/overview/first-steps/' },
			{ title: 'Concepts', description: 'Items, ranks, order and dependencies', href: '/overview/concepts/' },
			{ title: 'Agent onboarding', description: 'Teach a coding agent to use Lif', href: '/overview/agent-onboarding/' },
			{ title: 'Store management', description: 'Several projects, worktrees, shared stores', href: '/overview/store-management/' },
		],
	},
	{
		label: 'Features',
		items: [
			{ title: 'All features', description: 'Everything Lif does, at a glance', href: '/features/' },
			{ title: 'Files in your repo', description: 'Plain files that Git versions with your code', href: '/features/files-in-your-repo/' },
			{ title: 'Automation and agents', description: 'JSON, GraphQL and lif prime', href: '/features/automation/' },
			{ title: 'Dependencies', description: 'Blockers and what is ready next', href: '/features/dependencies/' },
			{ title: 'Flexible planning', description: 'Types, statuses and hierarchy presets', href: '/features/project-planning/' },
			{ title: 'Surface independence', description: 'CLI, TUI, web UI or any editor', href: '/features/surface-independence/' },
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
			{ title: 'Known limitations', description: 'Open defects, by short ID', href: '/known-limitations/' },
			{ title: 'Contribute', description: 'Report bugs, build from source, send pull requests', href: '/contribute/' },
			{ title: 'About', description: 'What Lif is and where it comes from', href: '/about/' },
		],
	},
];
