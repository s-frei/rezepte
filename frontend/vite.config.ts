import { paraglideVitePlugin } from '@inlang/paraglide-js';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vitest/config';
import adapter from '@sveltejs/adapter-static';
import { sveltekit } from '@sveltejs/kit/vite';
import { inlangProject, writeInlangSettings } from './scripts/inlang-settings';

// Before Paraglide reads it: the settings are generated from the Go module's
// list of languages (see scripts/inlang-settings.ts).
writeInlangSettings();

const apiTarget = `http://localhost:${process.env.RZP_BACKEND_PORT ?? 8060}`;

export default defineConfig({
	plugins: [
		tailwindcss(),
		sveltekit({
			compilerOptions: {
				// Force runes mode for the project, except for libraries. Can be removed in svelte 6.
				runes: ({ filename }) =>
					filename.split(/[/\\]/).includes('node_modules') ? undefined : true
			},
			// The logo's sources live outside the app (assets/brand), next to
			// the icon masters; $brand is how components import them.
			alias: { $brand: '../assets/brand' },
			adapter: adapter({
				pages: '../service/internal/web/dist',
				assets: '../service/internal/web/dist',
				fallback: 'index.html',
				precompress: false,
				strict: true
			})
		}),

		paraglideVitePlugin({
			project: inlangProject,
			outdir: './src/lib/paraglide',
			emitTsDeclarations: true,
			// cookie: the account's language, written by the Go service from
			// users.locale (service/internal/auth/handler.go). Read before the
			// first render, so a signed-in person never sees the wrong language
			// flash past. preferredLanguage: the browser's setting, which is all
			// there is to go on at the login screen. baseLocale: English.
			strategy: ['cookie', 'preferredLanguage', 'baseLocale']
		})
	],
	server: {
		// Listen on all interfaces so the dev server is reachable from other
		// devices on the local network, not just localhost.
		host: true,
		// Both ports come from the worktree's offset (root mise.toml [env]),
		// so a second worktree's dev stack does not fight this one for them.
		port: Number(process.env.RZP_FRONTEND_PORT ?? 9060),
		strictPort: true,
		// $brand points outside the project root, which the dev server
		// refuses to serve unless the folder is allowed explicitly.
		fs: { allow: ['../assets/brand'] },
		// `changeOrigin: false` keeps the browser's Host header: the API's
		// Origin check compares it with the Origin, and Vite's string
		// shorthand would rewrite it to the target, turning every login
		// or save in dev mode into a 403.
		proxy: {
			'/api': { target: apiTarget, changeOrigin: false },
			'/images': { target: apiTarget, changeOrigin: false },
			// The API card reads the instance version from here. The built
			// binary serves it from the same origin as the SPA; only the dev
			// server needs to be told.
			'/healthz': { target: apiTarget, changeOrigin: false }
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
