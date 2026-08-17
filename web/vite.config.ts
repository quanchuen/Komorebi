import { defineConfig } from 'vite';
import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
  plugins: [tailwindcss(), sveltekit()],
  server: {
    proxy: {
      '/api': process.env.API_PROXY_TARGET ?? 'http://127.0.0.1:8080',
      '/tiles': process.env.TILES_PROXY_TARGET ?? 'http://127.0.0.1:3000',
      '/nominatim': {
        target: 'https://nominatim.openstreetmap.org',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/nominatim/, '')
      }
    }
  }
});
