import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vitest/config';
import adapter from '@sveltejs/adapter-static';
import { sveltekit } from '@sveltejs/kit/vite';

const devApi = process.env.WATCHT_DEV_API ?? 'http://127.0.0.1:8090';

export default defineConfig({
	plugins: [
		tailwindcss(),
		sveltekit({
			compilerOptions: {
				// Force runes mode for the project, except for libraries. Can be removed in svelte 6.
				runes: ({ filename }) =>
					filename.split(/[/\\]/).includes('node_modules') ? undefined : true
			},
			// SPA: Go serves build/ and falls back to index.html for client-side routes.
			adapter: adapter({ fallback: 'index.html' })
		})
	],
	server: {
		// In development, the Go server handles the API and health check.
		// `make dev` runs it on :8090 and sets WATCHT_DEV_API.
		proxy: {
			'/api': devApi,
			'/healthz': devApi
		}
	},
	test: {
		expect: { requireAssertions: true },
		projects: [
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
