import { goto } from '$app/navigation';
import type { Item } from '#lib/items.svelte.js';
import { itemsStore } from '#lib/items.svelte.js';

class UIState {
  // Active view: 'planning' or an itemId for workspace view (derived from URL)
  activeView = $state<'planning' | string>('planning');

  // Planning sub-view (derived from URL)
  planningView = $state<'backlog' | 'board'>('backlog');

  get isPlanning(): boolean {
    return this.activeView === 'planning';
  }

  /** Sync UIState from URL path. Called reactively from layout on every navigation. */
  syncFromUrl(pathname: string) {
    const workspaceMatch = pathname.match(/^\/workspace\/([^/]+)/);
    const newView = workspaceMatch ? workspaceMatch[1] : 'planning';
    const viewChanged = newView !== this.activeView;

    if (workspaceMatch) {
      this.activeView = workspaceMatch[1];
    } else {
      this.activeView = 'planning';
      this.planningView = pathname === '/planning/board' ? 'board' : 'backlog';
    }

    // Restore the remembered item selection in the URL only when switching views
    if (viewChanged) {
      this.syncSelectedItemToUrl();
    }
  }

  /** Navigate to a view via URL routing. */
  navigateTo(view: 'planning' | string) {
    if (view === 'planning') {
      goto(`/planning${this.planningView === 'board' ? '/board' : ''}`);
    } else {
      goto(`/workspace/${view}`);
    }
  }

  /** Navigate to a planning sub-view. */
  navigateToPlanningView(view: 'backlog' | 'board') {
    goto(view === 'board' ? '/planning/board' : '/planning');
  }

  // Per-view selected item ID (keyed by activeView: 'planning' or worktree ID)
  private selectedItemByView = $state<Record<string, string | null>>({});

  // Selected item ID for the current view
  get selectedItemId(): string | null {
    return this.selectedItemByView[this.activeView] ?? null;
  }

  set selectedItemId(id: string | null) {
    this.selectedItemByView[this.activeView] = id;
  }

  // Resolved item from store
  get currentItem(): Item | null {
    return this.selectedItemId ? (itemsStore.get(this.selectedItemId) ?? null) : null;
  }

  selectItem(item: Item) {
    this.selectedItemId = item.id;
    this.syncSelectedItemToUrl();
  }

  selectItemById(id: string) {
    this.selectedItemId = id;
    this.syncSelectedItemToUrl();
  }

  /** Pre-select an item for a specific view (e.g. before navigating to that view). */
  selectItemForView(itemId: string, view: string) {
    this.selectedItemByView[view] = itemId;
    // If already on this view, sync to URL immediately
    if (view === this.activeView) {
      this.syncSelectedItemToUrl();
    }
    // Otherwise, syncFromUrl will pick it up when the navigation completes
  }

  clearSelection() {
    this.selectedItemId = null;
    this.syncSelectedItemToUrl();
  }

  /** Update the URL query param without navigation */
  private syncSelectedItemToUrl() {
    const url = new URL(window.location.href);
    if (this.selectedItemId) {
      url.searchParams.set('item', this.selectedItemId);
    } else {
      url.searchParams.delete('item');
    }
    window.history.replaceState(window.history.state, '', url);
  }

  // Planning chat pane (persisted to localStorage)
  showPlanningChat = $state(false);

  togglePlanningChat() {
    this.showPlanningChat = !this.showPlanningChat;
    localStorage.setItem('lif-planning-chat', this.showPlanningChat ? 'true' : 'false');
  }

  // Changes pane (persisted to localStorage)
  showChanges = $state(false);

  toggleChanges() {
    this.showChanges = !this.showChanges;
    localStorage.setItem('lif-changes-pane', this.showChanges ? 'true' : 'false');
  }

  // Terminal pane (always hidden by default, not persisted)
  showTerminal = $state(false);
  terminalInitialized = $state(false);

  toggleTerminal() {
    this.showTerminal = !this.showTerminal;
    if (this.showTerminal) {
      this.terminalInitialized = true;
    }
  }

  // Filter text (persisted to localStorage)
  filterText = $state('');

  setFilterText(text: string) {
    this.filterText = text;
    if (text) {
      localStorage.setItem('lif-filter-text', text);
    } else {
      localStorage.removeItem('lif-filter-text');
    }
  }

  // Form modal
  showForm = $state(false);
  editingItem = $state<Item | null>(null);

  openCreateForm() {
    this.editingItem = null;
    this.showForm = true;
  }

  openEditForm(item: Item) {
    this.editingItem = item;
    this.showForm = true;
  }

  closeForm() {
    this.showForm = false;
    this.editingItem = null;
  }
}

export const ui = new UIState();
