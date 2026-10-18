/**
 * Item colours come from .lif.yml through configStore. Each helper returns an
 * inline `--tone: <colour>` declaration for `style={…}`; the `.badge-tone`,
 * `.border-l-tone`, `.text-tone` and `.border-tone` classes in layout.css read it.
 */
import { configStore } from '#lib/config.svelte.js';

/** CSS colour for a configured colour: a Catppuccin tone name or "#rrggbb". Unset gives `fallback`. */
export function toneValue(color: string, fallback = 'var(--th-text-faint)'): string {
  if (!color) return fallback;
  return color.startsWith('#') ? color : `var(--ctp-${color})`;
}

function tone(list: { name: string; color: string }[], name: string, fallback?: string): string {
  return `--tone: ${toneValue(list.find((v) => v.name === name)?.color ?? '', fallback)}`;
}

export const statusTone = (name: string) => tone(configStore.statuses, name);
export const typeTone = (name: string) => tone(configStore.types, name);
export const priorityTone = (name: string) =>
  tone(configStore.priorities, name, 'var(--th-text-muted)');
