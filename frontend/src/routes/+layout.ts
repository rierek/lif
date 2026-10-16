import { browser } from '$app/env';

export const prerender = true;
export const ssr = false;

export function load() {
  let selectedItemId: string | null = null;
  let showPlanningChat = false;
  let showChanges = false;
  let filterText = '';

  if (browser) {
    const params = new URLSearchParams(window.location.search);
    selectedItemId = params.get('item');

    showPlanningChat = localStorage.getItem('lif-planning-chat') === 'true';
    showChanges = localStorage.getItem('lif-changes-pane') === 'true';
    filterText = localStorage.getItem('lif-filter-text') ?? '';
  }

  return { selectedItemId, showPlanningChat, showChanges, filterText };
}
