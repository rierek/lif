import { defineConfig } from 'vitest/config';
import { playwright } from '@vitest/browser-playwright';
import tailwindcss from '@tailwindcss/vite';
import adapter from '@sveltejs/adapter-static';
import { sveltekit } from '@sveltejs/kit/vite';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

const backendPort = process.env.LIF_PORT || '22880';

export default defineConfig({
  plugins: [
    tailwindcss(),
    sveltekit({
      // Consult https://svelte.dev/docs/kit/integrations
      // for more information about preprocessors
      preprocess: vitePreprocess(),

      adapter: adapter({ fallback: 'index.html' }),

      compilerOptions: {
        experimental: {
          async: true
        }
      }
    })
  ],

  build: {
    // Shiki wasm + grammars produce large chunks; this is an embedded app, not a public site
    chunkSizeWarningLimit: 2800
  },

  server: {
    // Proxy some URL routes to the Go backend process in development.
    proxy: {
      '/api': {
        target: `http://localhost:${backendPort}`,
        ws: true,
        changeOrigin: true
      }
    }
  },

  test: {
    expect: { requireAssertions: true },

    projects: [
      {
        extends: './vite.config.ts',

        test: {
          name: 'client',

          browser: {
            enabled: true,
            provider: playwright(),
            instances: [{ browser: 'chromium', headless: true }]
          },

          include: ['src/**/*.svelte.{test,spec}.{js,ts}'],
          exclude: ['src/lib/server/**']
        }
      },

      {
        extends: './vite.config.ts',

        test: {
          name: 'server',
          environment: 'node',
          include: ['src/**/*.{test,spec}.{js,ts}'],
          exclude: ['src/**/*.svelte.{test,spec}.{js,ts}']
        }
      }
    ]
  }
});
