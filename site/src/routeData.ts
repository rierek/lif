import { defineRouteMiddleware, type StarlightRouteData } from '@astrojs/starlight/route-data';

type Entry = StarlightRouteData['sidebar'][number];
type Link = Extract<Entry, { type: 'link' }>;

const holdsCurrent = (e: Entry): boolean => (e.type === 'link' ? e.isCurrent : e.entries.some(holdsCurrent));
const links = (entries: Entry[]): Link[] => entries.flatMap((e) => (e.type === 'link' ? [e] : links(e.entries)));

// D36: the sidebar shows only the chapter (top-level group in astro.config.mjs) that holds the current page,
// with its groups expanded. Previous/next links stay inside the chapter.
export const onRequest = defineRouteMiddleware((context) => {
	const route = context.locals.starlightRoute;
	const chapter = route.sidebar.find(holdsCurrent);
	if (chapter?.type !== 'group') return;
	route.sidebar = chapter.entries;
	const pages = links(chapter.entries);
	const at = pages.findIndex((p) => p.isCurrent);
	route.pagination = { prev: route.pagination.prev && pages[at - 1], next: route.pagination.next && pages[at + 1] };
});
