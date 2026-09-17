import { paraglideVitePlugin } from '@inlang/paraglide-js';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vitest/config';
import adapter from '@sveltejs/adapter-static';
import { sveltekit } from '@sveltejs/kit/vite';

export default defineConfig({
	plugins: [
		tailwindcss(),
		sveltekit({
			compilerOptions: {
				// Force runes mode for the project, except for libraries. Can be removed in svelte 6.
				runes: ({ filename }) =>
					filename.split(/[/\\]/).includes('node_modules') ? undefined : true
			},
			adapter: adapter({
				pages: '../service/internal/web/dist',
				assets: '../service/internal/web/dist',
				fallback: 'index.html',
				precompress: false,
				strict: true
			})
		}),

		paraglideVitePlugin({
			project: './project.inlang',
			outdir: './src/lib/paraglide',
			emitTsDeclarations: true,
			strategy: ['cookie', 'baseLocale']
		})
	],
	server: {
		// Listen on all interfaces so the dev server is reachable from other
		// devices on the local network, not just localhost.
		host: true,
		port: 9060,
		strictPort: true,
		// `changeOrigin: false` keeps the browser's Host header: the API's
		// Origin check compares it with the Origin, and Vite's string
		// shorthand would rewrite it to localhost:8060, turning every login
		// or save in dev mode into a 403.
		proxy: {
			'/api': { target: 'http://localhost:8060', changeOrigin: false },
			'/images': { target: 'http://localhost:8060', changeOrigin: false }
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
