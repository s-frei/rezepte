import { afterEach, describe, expect, test } from 'vitest';
import type { RecipeInput } from '$lib/api/recipes';
import { getLocale, overwriteGetLocale } from '$lib/paraglide/runtime';
import de from '../../../../service/internal/recipe/testdata/recipes.de.json';
import en from '../../../../service/internal/recipe/testdata/recipes.en.json';
import { catalogUnits, formatDuration, manualTime, suggestTimes, type TimeUnits } from './times';

const units: TimeUnits = {
	seconds: ['Sekunden', 'Sekunde', 'Sek.', 'Sek', 'seconds', 'second', 'secs', 'sec'],
	minutes: ['Minuten', 'Minute', 'Min.', 'Min', 'minutes', 'minute', 'mins', 'min'],
	hours: ['Stunden', 'Stunde', 'Std.', 'Std', 'h', 'hours', 'hour', 'hrs', 'hr'],
	range: ['bis', 'to'],
	and: ['und', 'and']
};
const s = (text: string) => suggestTimes(text, units);

describe('suggestTimes', () => {
	test.each([
		['etwa 90 Minuten schmoren', { phrase: '90 Minuten', seconds: 5400 }],
		['etwa 2,5 Stunden schmoren', { phrase: '2,5 Stunden', seconds: 9000 }],
		['simmer for 1.5 hours', { phrase: '1.5 hours', seconds: 5400 }],
		['½ Stunde ziehen lassen', { phrase: '½ Stunde', seconds: 1800 }],
		['1 ½ Stunden backen', { phrase: '1 ½ Stunden', seconds: 5400 }],
		['30 Sekunden mixen', { phrase: '30 Sekunden', seconds: 30 }],
		['ca. 10 Min. backen', { phrase: '10 Min.', seconds: 600 }],
		// A dot that ends the sentence is not the abbreviation's: it stays out of the phrase.
		['bake 10 min.', { phrase: '10 min', seconds: 600 }],
		['Bake for 10 min. Then serve.', { phrase: '10 min', seconds: 600 }],
		['Etwa 5 Min.', { phrase: '5 Min', seconds: 300 }],
		['ca. 10 Min. ruhen lassen', { phrase: '10 Min.', seconds: 600 }],
		['20–25 Minuten ruhen', { phrase: '20–25 Minuten', seconds: 1200, maxSeconds: 1500 }],
		['20-25 min ruhen', { phrase: '20-25 min', seconds: 1200, maxSeconds: 1500 }],
		['20 bis 25 Minuten ruhen', { phrase: '20 bis 25 Minuten', seconds: 1200, maxSeconds: 1500 }],
		['rest for 20 to 25 minutes', { phrase: '20 to 25 minutes', seconds: 1200, maxSeconds: 1500 }],
		['1 Std. 30 Min. garen', { phrase: '1 Std. 30 Min.', seconds: 5400 }],
		['cook 1 hour and 30 minutes', { phrase: '1 hour and 30 minutes', seconds: 5400 }],
		['nach 5,5 Minuten wenden', { phrase: '5,5 Minuten', seconds: 330 }],
		['ETWA 15 MINUTEN', { phrase: '15 MINUTEN', seconds: 900 }]
	])('%s', (text, want) => {
		expect(s(text)).toEqual([want]);
	});

	test('finds several in order', () => {
		expect(
			s('Den Teig 20 Minuten ruhen lassen, dann 1 Stunde kühlen.').map((t) => t.phrase)
		).toEqual(['20 Minuten', '1 Stunde']);
	});

	test.each([
		'2 Eier verquirlen',
		'bei 180 °C backen',
		'3 EL Öl',
		'über Nacht kalt stellen',
		'einige Minuten rühren',
		'190 Minutenbrot', // unit glued into a longer word
		'Dauer: 0 Minuten'
	])('ignores %s', (text) => {
		expect(s(text)).toEqual([]);
	});

	test('proposes a repeated duration once, where it anchors', () => {
		expect(s('20 Minuten backen, wenden, weitere 20 Minuten backen.')).toEqual([
			{ phrase: '20 Minuten', seconds: 1200 }
		]);
	});

	test('a duration first found inside a compound is no extra proposal', () => {
		expect(s('1 Stunde 30 Minuten ruhen lassen, dann 30 Minuten backen.')).toEqual([
			{ phrase: '1 Stunde 30 Minuten', seconds: 5400 }
		]);
	});

	test('a sentence comma is not a decimal comma', () => {
		expect(s('5, 10 Minuten später').map((t) => t.phrase)).toEqual(['10 Minuten']);
	});

	test('drops what the service would refuse', () => {
		expect(s('200 Stunden')).toEqual([]); // over a week
		expect(s('25–20 Minuten')).toEqual([]); // range running down
	});

	test('the catalogs feed every language at once', () => {
		const merged = catalogUnits();
		expect(merged.minutes).toEqual(expect.arrayContaining(['Minuten', 'minutes']));
	});
});

describe('manualTime', () => {
	test('reads the number from a selection the detector missed', () => {
		expect(manualTime('15 perc', 'minutes')).toEqual({ phrase: '15 perc', seconds: 900 });
		expect(manualTime('20-25 perc', 'minutes')).toEqual({
			phrase: '20-25 perc',
			seconds: 1200,
			maxSeconds: 1500
		});
		expect(manualTime('über Nacht', 'hours')).toBeNull();
	});
});

describe('formatDuration', () => {
	const english = getLocale;
	afterEach(() => overwriteGetLocale(english));
	const t = (seconds: number, maxSeconds?: number) => ({ phrase: '', seconds, maxSeconds });

	test.each([
		[t(45), '45 sec', '45 Sek'],
		[t(900), '15 min', '15 Min'],
		[t(5400), '1 hr 30 min', '1 Std 30 Min'],
		[t(7200), '2 hr', '2 Std'],
		[t(90), '1 min 30 sec', '1 Min 30 Sek'],
		[t(330), '5 min 30 sec', '5 Min 30 Sek'],
		[t(1200, 1500), '20–25 min', '20–25 Min'],
		[t(3600, 7200), '1–2 hr', '1–2 Std'],
		[t(30, 45), '30–45 sec', '30–45 Sek'],
		[t(50, 70), '50 sec–1 min 10 sec', '50 Sek–1 Min 10 Sek'],
		[t(2700, 5400), '45 min–1 hr 30 min', '45 Min–1 Std 30 Min']
	])('%o', (time, english, german) => {
		expect(formatDuration(time)).toBe(english);
		overwriteGetLocale(() => 'de');
		expect(formatDuration(time)).toBe(german);
	});
});

// The sample recipes mark every duration the detector finds, and nothing else.
describe.each([
	['de', de],
	['en', en]
])('the %s sample recipes', (_, recipes) => {
	const steps = (recipes as RecipeInput[]).flatMap((recipe) =>
		recipe.steps.map((step) => [`${recipe.title}: ${step.text}`, step] as const)
	);
	test.each(steps)('%s', (_, step) => {
		expect(suggestTimes(step.text)).toEqual(step.times);
	});
});

describe('phrase length', () => {
	// Leading zeros keep the duration at 5 minutes, so only the length decides.
	test('suggestTimes keeps a phrase of exactly 60 characters', () => {
		const phrase = `${'0'.repeat(51)}5 Minuten`;
		expect(s(`dann ${phrase} ruhen`)).toEqual([{ phrase, seconds: 300 }]);
	});
	test('suggestTimes drops a phrase over 60 characters', () => {
		expect(s(`dann ${'0'.repeat(52)}5 Minuten ruhen`)).toEqual([]);
	});
	test('manualTime refuses a phrase over 60 characters', () => {
		expect(manualTime(`5 ${'x'.repeat(60)}`, 'minutes')).toBeNull();
	});
});
