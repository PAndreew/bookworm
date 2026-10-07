import { defineConfig } from 'astro/config';
import svelte from '@astrojs/svelte';

// Static output: the Go API is the only server. Astro ships a prerendered PWA
// shell and the Svelte islands talk to the API directly with cookies.
export default defineConfig({
  integrations: [svelte()],
  output: 'static',
  vite: {
    server: {
      proxy: {
        // Dev only: same-origin API so the session cookie is sent without CORS.
        '/v1': { target: 'http://localhost:8080', changeOrigin: true },
      },
    },
  },
});
