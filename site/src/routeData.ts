import { defineRouteMiddleware, type StarlightRouteData } from '@astrojs/starlight/route-data';

type Entry = StarlightRouteData['sidebar'][number];
type Link = Extract<Entry, { type: 'link' }>;

// Sidebar mode, the one switch.
//   'all'     D51 (temporary, while content is written): every chapter is listed. All groups are collapsible and
//             closed, except those holding the current page (Starlight opens them). Previous/next cross chapters.
//   'chapter' D36 (final unless decided otherwise after phase 5): only the chapter that holds the current page,
//             groups expanded. Previous/next stay inside the chapter.
const SIDEBAR_MODE: 'all' | 'chapter' = 'all';

const holdsCurrent = (e: Entry): boolean => (e.type === 'link' ? e.isCurrent : e.entries.some(holdsCurrent));
const links = (entries: Entry[]): Link[] => entries.flatMap((e) => (e.type === 'link' ? [e] : links(e.entries)));
const collapsible = (entries: Entry[]): Entry[] =>
	entries.map((e) => (e.type === 'group' ? { ...e, collapsed: true, entries: collapsible(e.entries) } : e));

export const onRequest = defineRouteMiddleware((context) => {
	const route = context.locals.starlightRoute;
	if (SIDEBAR_MODE === 'all') {
		route.sidebar = collapsible(route.sidebar);
		return;
	}
	const chapter = route.sidebar.find(holdsCurrent);
	if (chapter?.type !== 'group') return;
	route.sidebar = chapter.entries;
	const pages = links(chapter.entries);
	const at = pages.findIndex((p) => p.isCurrent);
	route.pagination = { prev: route.pagination.prev && pages[at - 1], next: route.pagination.next && pages[at + 1] };
});
