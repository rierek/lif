type Preference = 'system' | 'light' | 'dark';

const PREFERENCES: Preference[] = ['system', 'light', 'dark'];

/** Flavours display.theme can pick for dark mode; light mode is always latte. */
const DARK_FLAVOURS = ['frappe', 'macchiato', 'mocha'];

/**
 * Light/dark/system switch. The choice lives in localStorage (`lif-theme`) and is
 * mirrored onto <html> as `data-theme` (light|dark) and `data-flavour` (the Catppuccin
 * flavour); app.html sets both before first paint so a reload does not flash.
 */
class ThemeStore {
  preference = $state<Preference>('system');
  resolved = $state<'light' | 'dark'>('light');
  darkFlavour = $state('mocha');

  private query: MediaQueryList | undefined;

  init() {
    const stored = localStorage.getItem('lif-theme');
    this.preference = PREFERENCES.find((p) => p === stored) ?? 'system';
    this.darkFlavour = localStorage.getItem('lif-dark-flavour') || 'mocha';

    if (!this.query) {
      this.query = matchMedia('(prefers-color-scheme: dark)');
      this.query.addEventListener('change', () => {
        if (this.preference === 'system') this.apply();
      });
    }
    this.apply();
  }

  /** Step system -> light -> dark -> system. */
  cycle() {
    this.preference = PREFERENCES[(PREFERENCES.indexOf(this.preference) + 1) % PREFERENCES.length];
    localStorage.setItem('lif-theme', this.preference);
    this.apply();
  }

  /** Apply display.theme. latte or an unknown name leaves dark mode on mocha. */
  setConfigured(name: string) {
    const flavour = name.toLowerCase();
    this.darkFlavour = DARK_FLAVOURS.includes(flavour) ? flavour : 'mocha';
    localStorage.setItem('lif-dark-flavour', this.darkFlavour);
    this.apply();
  }

  private apply() {
    this.resolved =
      this.preference === 'system' ? (this.query?.matches ? 'dark' : 'light') : this.preference;
    const root = document.documentElement;
    root.dataset.theme = this.resolved;
    root.dataset.flavour = this.resolved === 'light' ? 'latte' : this.darkFlavour;
  }
}

export const theme = new ThemeStore();
