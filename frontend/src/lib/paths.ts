import { resolve } from '$app/paths';

// A proxy may mount the UI under a prefix and strip it before the request
// reaches lif-serve (tailscale serve --set-path /lif, nginx, Caddy). The server
// does not know the prefix: the page it serves carries it as SvelteKit's base,
// which the browser derives from the page location. Every path of the server
// (/api/…) and every path of the router goes through here, so none is written
// root-absolute.

/** The app root as a path with a trailing slash: `/` at the origin root, `/lif/` under `/lif`. */
export function appRoot(): string {
  const root = resolve('/');
  return root.endsWith('/') ? root : root + '/';
}

/** A path of the server, e.g. `/api/graphql`, under the app root. */
export function apiUrl(path: string): string {
  return appRoot() + path.replace(/^\//, '');
}

/** The WebSocket URL of a path of the server. */
export function wsUrl(path: string): string {
  const url = new URL(apiUrl(path), location.href);
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:';
  return url.href;
}

/** A pathname of the browser location without the app root: `/lif/workspace/x` is `/workspace/x`. */
export function stripRoot(pathname: string): string {
  const root = appRoot();
  return pathname.startsWith(root) ? '/' + pathname.slice(root.length) : pathname;
}
