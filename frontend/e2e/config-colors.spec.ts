import type { Locator } from '@playwright/test';
import { test, expect } from './fixtures';

/** The `--tone` declaration a badge carries inline, as styles.ts writes it. */
const tone = (badge: Locator) =>
  badge.evaluate((el) => (el as HTMLElement).style.getPropertyValue('--tone'));

test.describe('statuses, types and colours from .lif.yml', () => {
  test.use({
    lifConfigExtra: [
      'statuses:',
      '  - name: to-review',
      '    color: sky',
      '  - name: parked',
      'types:',
      '  - name: bug',
      '    color: "#FF0000"',
      ''
    ].join('\n')
  });

  test('the new-item form lists the configured statuses in config order', async ({ lif, page }) => {
    await page.goto(`${lif.baseURL}/`);
    await page.getByRole('button', { name: '+ New Item' }).click();

    await expect(page.locator('#item-status option')).toHaveText([
      'in-progress',
      'todo',
      'draft',
      'completed',
      'scrapped',
      'to-review',
      'parked'
    ]);
  });

  test('a custom status badge takes the tone the config names', async ({ lif, page }) => {
    const id = lif.create('Needs review', { status: 'to-review' });
    await page.goto(`${lif.baseURL}/?item=${id}`);

    const badge = page.locator('.badge', { hasText: 'to-review' });
    await expect(badge).toBeVisible({ timeout: 10_000 });
    await expect.poll(() => tone(badge)).toBe('var(--ctp-sky)');
    // The tone is a real palette entry: it resolves to the colour of the active flavour.
    const sky = await page.evaluate(() =>
      getComputedStyle(document.documentElement).getPropertyValue('--ctp-sky').trim()
    );
    expect(sky).toMatch(/^#[0-9a-f]{6}$/);
    await expect
      .poll(() => badge.evaluate((el) => getComputedStyle(el).getPropertyValue('--tone').trim()))
      .toBe(sky);
  });

  test('a status without a colour gets the fallback, not no style', async ({ lif, page }) => {
    const id = lif.create('Shelved', { status: 'parked' });
    await page.goto(`${lif.baseURL}/?item=${id}`);

    const badge = page.locator('.badge', { hasText: 'parked' });
    await expect(badge).toBeVisible({ timeout: 10_000 });
    await expect.poll(() => tone(badge)).toBe('var(--th-text-faint)');
  });

  test('a hex type colour reaches the badge, lower-cased', async ({ lif, page }) => {
    const id = lif.create('Red bug', { type: 'bug' });
    await page.goto(`${lif.baseURL}/?item=${id}`);

    const badge = page.locator('.badge', { hasText: /^bug$/ });
    await expect(badge).toBeVisible({ timeout: 10_000 });
    await expect.poll(() => tone(badge)).toBe('#ff0000');
  });
});
