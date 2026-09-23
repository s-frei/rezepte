/**
 * Writes Paraglide's inlang project settings from the one list of languages.
 *
 * The languages live in `service/internal/i18n/locales.json`, which the Go
 * service embeds - go:embed cannot reach into frontend/, so the list has to
 * sit in the Go module. Everything else in the settings is frontend build
 * configuration and is written down here. `vite.config.ts` calls this before
 * it hands the project to Paraglide, so every vite run - dev, build, vitest -
 * compiles against the current list, and `project.inlang/` is gitignored.
 *
 * Runs inside the vite config, which Node loads, so this uses node:fs rather
 * than Bun's file API.
 */
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const frontend = join(dirname(fileURLToPath(import.meta.url)), '..');
const localesFile = join(frontend, '..', 'service', 'internal', 'i18n', 'locales.json');

/** Where the generated project lives; the path `paraglideVitePlugin` gets. */
export const inlangProject = './project.inlang';

export function writeInlangSettings(): void {
	const { baseLocale, locales } = JSON.parse(readFileSync(localesFile, 'utf8')) as {
		baseLocale: string;
		locales: string[];
	};
	// The Go side refuses the same file at startup; failing here too keeps a
	// broken list from ever compiling into a frontend.
	if (!Array.isArray(locales) || !locales.includes(baseLocale)) {
		throw new Error(`${localesFile}: baseLocale ${baseLocale} is not in locales ${locales}`);
	}
	const settings = {
		$schema: 'https://inlang.com/schema/project-settings',
		modules: [
			'https://cdn.jsdelivr.net/npm/@inlang/plugin-message-format@4/dist/index.js',
			'https://cdn.jsdelivr.net/npm/@inlang/plugin-m-function-matcher@2/dist/index.js'
		],
		'plugin.inlang.messageFormat': { pathPattern: './messages/{locale}.json' },
		baseLocale,
		locales
	};
	const file = join(frontend, inlangProject, 'settings.json');
	const text = `${JSON.stringify(settings, null, '\t')}\n`;
	// Unchanged content is left alone, so a vite restart does not touch the
	// file's mtime and wake Paraglide's watcher for nothing.
	let current = '';
	try {
		current = readFileSync(file, 'utf8');
	} catch {
		// first run: nothing there yet
	}
	if (current !== text) {
		mkdirSync(dirname(file), { recursive: true });
		writeFileSync(file, text);
	}
}
