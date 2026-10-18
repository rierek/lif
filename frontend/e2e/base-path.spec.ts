import { mkdirSync, writeFileSync } from 'node:fs';
import { createServer, request } from 'node:http';
import { connect, type AddressInfo, type Socket } from 'node:net';
import { join } from 'node:path';
import { test as lifTest, expect } from './fixtures';
import { agentSession } from './agent-session';
import { BacklogPage } from './pages/backlog-page';

const PREFIX = '/lif';

/**
 * A reverse proxy that mounts lif-serve under /lif and strips the prefix, as
 * `tailscale serve --set-path /lif`, nginx `proxy_pass http://…/;` and Caddy
 * `handle_path` do. Everything else goes through unchanged (Host included),
 * WebSocket upgrades too. It adds no X-Forwarded-* header.
 */
async function startStrippingProxy(targetPort: number) {
  const strip = (url?: string) => (url?.startsWith(PREFIX + '/') ? url.slice(PREFIX.length) : null);

  const server = createServer((req, res) => {
    const path = strip(req.url);
    if (path === null) {
      res.writeHead(404).end();
      return;
    }
    const upstream = request(
      { host: '127.0.0.1', port: targetPort, method: req.method, path, headers: req.headers },
      (up) => {
        res.writeHead(up.statusCode ?? 502, up.headers);
        up.pipe(res);
      }
    );
    upstream.on('error', () => res.writeHead(502).end());
    req.pipe(upstream);
  });

  // An upgraded connection leaves the HTTP server's own bookkeeping, so the
  // proxy keeps the sockets of its WebSocket tunnels to close them itself.
  const tunnels = new Set<Socket>();

  server.on('upgrade', (req, socket, head) => {
    const path = strip(req.url);
    if (path === null) {
      socket.destroy();
      return;
    }
    const upstream = connect(targetPort, '127.0.0.1', () => {
      let raw = `${req.method} ${path} HTTP/1.1\r\n`;
      for (let i = 0; i < req.rawHeaders.length; i += 2) {
        raw += `${req.rawHeaders[i]}: ${req.rawHeaders[i + 1]}\r\n`;
      }
      upstream.write(raw + '\r\n');
      upstream.write(head);
      socket.pipe(upstream);
      upstream.pipe(socket);
    });
    tunnels.add(socket).add(upstream);
    upstream.on('error', () => socket.destroy());
    socket.on('error', () => upstream.destroy());
  });

  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  return {
    port: (server.address() as AddressInfo).port,
    close: () =>
      new Promise<void>((resolve) => {
        tunnels.forEach((socket) => socket.destroy());
        server.closeAllConnections();
        server.close(() => resolve());
      })
  };
}

/** `base` is the address of lif-serve behind the proxy: http://127.0.0.1:<port>/lif */
const test = lifTest.extend<{ base: string }>({
  base: async ({ lif }, use) => {
    const proxy = await startStrippingProxy(Number(new URL(lif.baseURL).port));
    await use(`http://127.0.0.1:${proxy.port}${PREFIX}`);
    await proxy.close();
  }
});

test.describe('under a sub-path behind a prefix-stripping proxy', () => {
  test('the planning view, live updates, navigation and reloads work', async ({
    page,
    lif,
    base
  }) => {
    const backlog = new BacklogPage(page, base);
    const sockets: string[] = [];
    page.on('websocket', (ws) => sockets.push(ws.url()));

    lif.create('First item');
    await backlog.goto(1);
    // The index page sends the router to /planning under the app root.
    await expect(page).toHaveURL(`${base}/planning`);

    // The GraphQL subscription reaches the server through the proxy too.
    lif.create('Live item');
    await backlog.waitForItem('Live item');

    // The router keeps the prefix when it navigates.
    await page.getByRole('button', { name: 'Board' }).click();
    await expect(page).toHaveURL(`${base}/planning/board`);
    await page.waitForSelector('[data-status]', { timeout: 10_000 });

    // A prerendered page answers a reload of its route.
    await page.reload();
    await expect(page).toHaveURL(`${base}/planning/board`);
    await page.waitForSelector('[data-status]', { timeout: 10_000 });

    // The page of a route the build did not prerender is the rewritten fallback.
    const workspace = `${base}/workspace/__central__`;
    for (const load of [() => page.goto(workspace), () => page.reload()]) {
      await load();
      await expect(page).toHaveURL(workspace);
      // The router found the workspace id below the app root: its entry is the active one.
      await expect(page.locator('nav button.font-medium span.truncate')).toHaveText('main');
    }

    // The terminal and GraphQL sockets are opened under the prefix.
    const terminalToggle = page.getByTitle(/^(Show|Hide) terminal$/);
    if ((await terminalToggle.getAttribute('title')) === 'Show terminal')
      await terminalToggle.click();
    for (const path of ['/api/graphql', '/api/terminal']) {
      await expect
        .poll(() => sockets.some((url) => url.endsWith(PREFIX + path)), {
          message: `a WebSocket to ${PREFIX}${path}`
        })
        .toBe(true);
    }
  });

  test('a deep link opens its item', async ({ page, lif, base }) => {
    const id = lif.create('Deep Linked', { status: 'todo', type: 'task' });

    await page.goto(`${base}/?item=${id}`);

    await expect(page.locator('h1', { hasText: 'Deep Linked' })).toBeVisible({ timeout: 10_000 });
  });

  test('an agent attachment is fetched under the prefix', async ({ page, lif, base }) => {
    // A 1x1 PNG
    const png = Buffer.from(
      'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8/5+hHgAHggJ/PchI7wAAAABJRU5ErkJggg==',
      'base64'
    );
    const attachments = join(lif.lifPath, '.conversations', 'attachments', '__central__');
    mkdirSync(attachments, { recursive: true });
    writeFileSync(join(attachments, 'test-image.png'), png);

    await agentSession('__central__', { lifPath: lif.lifPath, baseURL: base })
      .withMessages([
        {
          role: 'user',
          content: 'Check this screenshot',
          images: [{ id: 'test-image.png', media_type: 'image/png' }]
        }
      ])
      .open(page);

    const image = page.locator('img[alt="Attachment"]');
    await expect(image).toHaveAttribute(
      'src',
      `${PREFIX}/api/attachments/__central__/test-image.png`
    );
    await expect.poll(() => image.evaluate((el: HTMLImageElement) => el.naturalWidth)).toBe(1);
  });
});
