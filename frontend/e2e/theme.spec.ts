import type { Page } from '@playwright/test';
import { test, expect } from './fixtures';

const stored = (page: Page) => page.evaluate(() => localStorage.getItem('lif-theme'));

/** Relative luminance (0..1) of the xterm background, which xterm paints on its scrollable element. */
async function terminalLuminance(page: Page): Promise<number> {
  const rgb = await page
    .locator('.xterm-scrollable-element')
    .evaluate((el) => getComputedStyle(el).backgroundColor);
  const [r, g, b] = (rgb.match(/[\d.]+/g) ?? []).slice(0, 3).map(Number);
  return (0.2126 * r + 0.7152 * g + 0.0722 * b) / 255;
}

test.describe('theme switch, dark OS', () => {
  test.use({ colorScheme: 'dark' });

  test('system follows the OS scheme', async ({ lif, page }) => {
    await page.goto(`${lif.baseURL}/`);

    await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
  });

  test('an explicit choice beats the OS and survives a reload', async ({ lif, page }) => {
    await page.goto(`${lif.baseURL}/`);
    const toggle = page.getByTestId('theme-toggle');
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');

    // system -> light
    await toggle.click();
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');
    await expect(toggle).toHaveText('Light');
    expect(await stored(page)).toBe('light');

    await page.reload();
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');
    await expect(page.getByTestId('theme-toggle')).toHaveText('Light');
    expect(await stored(page)).toBe('light');

    // light -> dark
    await page.getByTestId('theme-toggle').click();
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
    expect(await stored(page)).toBe('dark');

    // dark -> system, which follows the dark OS again
    await page.getByTestId('theme-toggle').click();
    await expect(page.getByTestId('theme-toggle')).toHaveText('Auto');
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
    expect(await stored(page)).toBe('system');
  });

  test('the theme is on <html> before the app runs', async ({ lif, page }) => {
    await page.goto(`${lif.baseURL}/`);
    await page.getByTestId('theme-toggle').click();
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');

    // Block the app bundle: only the inline script in app.html can set the attribute.
    await page.route('**/_app/**', (route) => route.abort());
    await page.reload();

    await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');
  });
});

test.describe('theme switch, light OS', () => {
  test.use({ colorScheme: 'light' });

  test('system follows the OS live, and dark overrides it', async ({ lif, page }) => {
    await page.goto(`${lif.baseURL}/`);
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');

    await page.emulateMedia({ colorScheme: 'dark' });
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
    await page.emulateMedia({ colorScheme: 'light' });
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');

    // system -> light -> dark
    await page.getByTestId('theme-toggle').click();
    await page.getByTestId('theme-toggle').click();
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');

    // An explicit choice ignores later OS changes.
    await page.emulateMedia({ colorScheme: 'light' });
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');

    await page.reload();
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
  });

  test('the terminal follows the UI theme', async ({ lif, page }) => {
    await page.goto(`${lif.baseURL}/`);
    await expect(page.getByText('Workspaces')).toBeVisible({ timeout: 10_000 });
    await page.locator('nav button:has(span.truncate)').first().click();
    await expect(page).toHaveURL(/\/workspace\//, { timeout: 10_000 });
    await page.getByTitle('Show terminal').click();

    await expect(page.locator('.xterm-scrollable-element')).toBeVisible({ timeout: 10_000 });
    await expect.poll(() => terminalLuminance(page)).toBeGreaterThan(0.6);

    // system -> light -> dark
    await page.getByTestId('theme-toggle').click();
    await page.getByTestId('theme-toggle').click();
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
    await expect.poll(() => terminalLuminance(page)).toBeLessThan(0.3);
  });
});
