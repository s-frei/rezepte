import type { StepTime } from '$lib/api/recipes';
import type { FormStep } from './form';
import { firstWordMatch } from './references';
import { suggestTimes } from './times';

/**
 * A step's times that the text still carries. An edit that removes a phrase
 * hides its time rather than deleting it, so undo brings it back; `toInput`
 * sends only what this returns.
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

/** Whether `phrase` overlaps a stored time or a linked ingredient word. */
export function isMarked(step: FormStep, phrase: string): boolean {
	return overlapsAny(step.text, phrase, [
		...step.times.map((time) => time.phrase),
		...step.references.map((ref) => ref.word)
	]);
}

/**
 * The detector's proposals the author has neither stored nor dismissed. A
 * proposal overlapping a linked word or a stored time is skipped, unless it
 * wholly contains that time: "1 Stunde 20 Minuten" written around a stored
 * "20 Minuten" is proposed, and accepting it replaces the shorter one (see
 * `addTime`). Dismissed phrases share the list of dismissed reference words: a
 * phrase holds a number, a word practically never.
 */
export function pendingTimes(step: FormStep, dismissed: string[] = []): StepTime[] {
	const refs = spans(
		step.text,
		step.references.map((ref) => ref.word)
	);
	const stored = spans(
		step.text,
		step.times.map((time) => time.phrase)
	);
	return suggestTimes(step.text).filter((time) => {
		if (dismissed.includes(time.phrase)) return false;
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
