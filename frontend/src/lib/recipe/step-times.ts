import type { StepTime } from '#lib/api/recipes.js';
import type { FormStep } from './form';
import { firstWordMatch } from './references';
import { suggestTimes } from './times';

/**
 * A step's times that the text still carries. Editing a phrase away drops
 * its time (`dropAbsent` in `step-doc.ts`); this keeps out one loaded with a
 * phrase the text lacks, and `toInput` sends only what it returns.
 */
export function presentTimes(times: StepTime[], text: string): StepTime[] {
	return times.filter((time) => firstWordMatch(text, time.phrase) !== null);
}

type Span = { from: number; to: number };
const overlaps = (a: Span, b: Span) => a.from < b.to && b.from < a.to;
const within = (inner: Span, outer: Span) => outer.from <= inner.from && inner.to <= outer.to;

/** Where each of `words` first stands in `text`, skipping the ones that do not. */
function spans(text: string, words: string[]): Span[] {
	return words.map((word) => firstWordMatch(text, word)).filter((span) => span !== null);
}

/**
 * Whether `phrase` (its first occurrence, the one a mark anchors to) overlaps
 * one of `words`: one stretch of text never carries two meanings.
 */
export function overlapsAny(text: string, phrase: string, words: string[]): boolean {
	const hit = firstWordMatch(text, phrase);
	return hit !== null && spans(text, words).some((span) => overlaps(hit, span));
}

/**
 * Whether `phrase` overlaps a stored time, a linked ingredient word or a photo
 * link. The caller passes the step with the photo links that show: one to a
 * photo hidden for its undo is not sent, so it is not in the way.
 */
export function isMarked(step: FormStep, phrase: string): boolean {
	return overlapsAny(step.text, phrase, [
		...step.times.map((time) => time.phrase),
		...step.references.map((ref) => ref.word),
		...step.photos.map((photo) => photo.phrase)
	]);
}

/**
 * The detector's proposals that are not stored yet. A proposal overlapping a
 * linked word or a photo link (the server refuses a time inside a photo's
 * phrase; pass the links that show, as for `isMarked`) is skipped, and so is
 * one overlapping a stored time, unless it
 * wholly contains that time: "1 Stunde 20 Minuten" written around a stored
 * "20 Minuten" is proposed, and accepting it replaces the shorter one (see
 * `addTime`).
 */
export function pendingTimes(step: FormStep): StepTime[] {
	const refs = spans(step.text, [
		...step.references.map((ref) => ref.word),
		...step.photos.map((photo) => photo.phrase)
	]);
	const stored = spans(
		step.text,
		step.times.map((time) => time.phrase)
	);
	return suggestTimes(step.text).filter((time) => {
		const hit = firstWordMatch(step.text, time.phrase);
		if (hit === null || refs.some((span) => overlaps(hit, span))) return false;
		return stored.every(
			(span) =>
				!overlaps(hit, span) || (within(span, hit) && span.to - span.from < hit.to - hit.from)
		);
	});
}

/** Adds a time to a step of `text`, replacing the ones whose phrase stands within its own. */
export function addTime(times: StepTime[], time: StepTime, text: string): StepTime[] {
	const hit = firstWordMatch(text, time.phrase);
	return [
		...times.filter((existing) => {
			if (existing.phrase === time.phrase) return false;
			const span = firstWordMatch(text, existing.phrase);
			return hit === null || span === null || !within(span, hit);
		}),
		time
	];
}

export function removeTime(times: StepTime[], phrase: string): StepTime[] {
	return times.filter((time) => time.phrase !== phrase);
}

/**
 * The time phrases a caret is looked up in, first match winning: proposals
 * before stored times, because a proposal may wholly contain a stored time
 * ("1 Stunde 20 Minuten" around "20 Minuten"), and in the overlap the
 * proposal's card is the one with something to do.
 */
export function caretTimePhrases(stored: StepTime[], pending: StepTime[]): string[] {
	return [...pending, ...stored].map((time) => time.phrase);
}
