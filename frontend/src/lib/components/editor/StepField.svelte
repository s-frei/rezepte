<script lang="ts">
	import { flushSync, tick, untrack, type Snippet } from 'svelte';
	import { Editor } from '@tiptap/core';
	import { Plugin, PluginKey } from '@tiptap/pm/state';
	import type { Node as ProseMirrorNode } from '@tiptap/pm/model';
	import { Decoration, DecorationSet, type EditorView } from '@tiptap/pm/view';
	import Suggestion, { exitSuggestion, type SuggestionProps } from '@tiptap/suggestion';
	import { Dialog, Popover, RadioGroup } from 'bits-ui';
	import { prefersReducedMotion } from 'svelte/motion';
	import { MediaQuery } from 'svelte/reactivity';
	import BottomSheet from '#lib/components/ui/BottomSheet.svelte';
	import Check from '@lucide/svelte/icons/check';
	import Info from '@lucide/svelte/icons/info';
	import Keyboard from '@lucide/svelte/icons/keyboard';
	import Pencil from '@lucide/svelte/icons/pencil';
	import Timer from '@lucide/svelte/icons/timer';
	import Unlink from '@lucide/svelte/icons/unlink';
	import X from '@lucide/svelte/icons/x';
	import type { StepTime } from '#lib/api/recipes.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import {
		presentReferences,
		type FormGroup,
		type FormRef,
		type FormStep
	} from '#lib/recipe/form.js';
	import { firstWordMatch, isWordChar, laterWordMatches } from '#lib/recipe/references.js';
	import {
		dropAbsent,
		referenceExtension,
		STEP_WORDS,
		stepExtensions,
		stepText,
		textToDoc,
		wordsOf,
		type StepWords
	} from '#lib/recipe/step-doc.js';
	import { closeHistory } from '@tiptap/pm/history';
	import {
		addReference,
		fitsWordLimit,
		guessFirst,
		isResolved,
		matchEntries,
		matchedPart,
		proposals,
		refLabel,
		refView,
		removeReference,
		repeatAtCaret,
		wordAt,
		wordAtCaret,
		type Choice,
		type PickerEntry
	} from '#lib/recipe/step-references.js';
	import {
		addTime,
		caretTimePhrases,
		isMarked,
		overlapsAny,
		pendingTimes,
		presentTimes,
		removeTime
	} from '#lib/recipe/step-times.js';
	import {
		MAX_PHRASE_LENGTH,
		formatDuration,
		manualTime,
		type TimeUnit
	} from '#lib/recipe/times.js';
	import { CELL } from './link-list';
	import StepLinks from './StepLinks.svelte';

	let {
		step,
		index,
		groups,
		entries,
		error
	}: {
		/** The step being edited; its `text`, `references` and `times` are written in place. */
		step: FormStep;
		/** Position in the list, for the field's label. Changes when steps are reordered. */
		index: number;
		/** The recipe's ingredient groups; the matcher and the picker read both
		 * their names and the client ids a link is anchored to. */
		groups: FormGroup[];
		/** Every ingredient the picker may offer, for the whole recipe. */
		entries: PickerEntry[];
		/** The server's message for this step, if it rejected it. */
		error?: string;
	} = $props();

	/**
	 * This step's suggestion plugin. One per component instance, so `Escape`
	 * can dismiss exactly this picker; every step has its own editor state, so
	 * two of them never meet.
	 */
	const PICKER_KEY = new PluginKey('ingredientPicker');

	/**
	 * Classes for the editable itself. ProseMirror owns this element, so they
	 * are handed to it through `editorProps` rather than written in the markup -
	 * which is also why they must be one literal string: Tailwind scans source
	 * text for class names.
	 */
	const EDITABLE =
		'step-editable min-h-16 w-full rounded-md border border-border bg-surface-elevated px-3.5 py-2.5 text-body transition outline-none focus:border-primary';

	/**
	 * The popup's own size, which has to be known before it exists: the side it
	 * opens on is decided when it opens, and measuring an element that has not
	 * been rendered yet is not possible. Keep these two in step with the `w-72`
	 * and `max-h-64` on the popup below.
	 *
	 * The manual picker's query field sits on top of that list, so its box is a
	 * row taller than this. The side is still decided from the list alone: the
	 * number only has to be close enough to keep the popup from opening into
	 * the smaller half of the screen, and one that changed with the mode would
	 * make the same caret open the popup on different sides.
	 */
	const PICKER_WIDTH = 288;
	const PICKER_MAX_HEIGHT = 256;

	/**
	 * The card at a marked word, sized the same way and for the same reason:
	 * keep these in step with its `w-64`, and with the height its tallest
	 * content - name, group, missing-ingredient line and actions - comes to.
	 */
	const CARD_WIDTH = 256;
	/**
	 * One action of a popup's action bar. `look` picks its color: `positive`
	 * for what the popup is for (Accept), in the primary color of the app's
	 * main actions and of the proposals' underline, `danger` for taking a link
	 * off, `plain` for everything else. Color is in the text only, on every
	 * cell alike: a filled cell beside a text one read as two kinds of bar.
	 * Not green: green says something succeeded, never what to do.
	 */
	type BarAction = {
		label: string;
		icon: typeof Check;
		look: 'positive' | 'danger' | 'plain';
		run: () => void;
		disabled?: boolean;
	};
	const BAR_LOOK = {
		positive: 'text-primary hover:bg-surface',
		danger: 'text-destructive hover:bg-destructive-soft',
		plain: 'text-text hover:bg-surface'
	} as const;

	/** How far a finger may move and still tap rather than scroll, in px. */
	const TAP_SLOP = 10;
	/** How long a touch may last and still be a tap rather than a press-and-hold, in ms. */
	const TAP_TIME = 500;
	/** A marked word and a later occurrence: what a phone's tap opens a sheet for. */
	const MARKS = '[data-ref-word], [data-repeat-word]';
	/** Gap between the caret and the popup, and between the popup and the window. */
	const PICKER_GAP = 4;
	const PICKER_MARGIN = 8;

	type PickerState = {
		items: PickerEntry[];
		active: number;
		/** Takes the chosen entry: back to the suggestion plugin, or into a link. */
		command: (entry: PickerEntry) => void;
		/**
		 * The word a manual link will attach to, and null while `@` drives the
		 * popup - that flow inserts the ingredient's own name, so it has no
		 * word until it has chosen one. It is also what the two modes are told
		 * apart by: only the manual one carries a query field of its own.
		 */
		word: string | null;
		/** What was typed to narrow the list, whose letters each row sets bold. */
		query: string;
	};

	/**
	 * Where the popup sits, decided when it opens and then left alone.
	 *
	 * `side` above all: recomputing it from the list's current height made the
	 * popup flip from above the caret to below it the moment a query narrowed
	 * the list, which is movement the author did not ask for while they are
	 * reading. It is decided once, from the room around the caret and the
	 * popup's maximum height, and holds for as long as this picker is open.
	 */
	type PickerAnchor = {
		/** The caret's rectangle, re-read when the window scrolls or resizes. */
		rect: () => DOMRect | null;
		side: 'above' | 'below';
	};

	let editor = $state<Editor | null>(null);
	let picker = $state<PickerState | null>(null);
	let anchor = $state<PickerAnchor | null>(null);
	/**
	 * One of `top` and `bottom` is set, never both. Opening upwards pins the
	 * popup's BOTTOM edge to the caret, so a shrinking list changes the box's
	 * height and nothing else - pinning the top would move the whole popup on
	 * every keystroke.
	 */
	let pickerAt = $state<{
		left: number;
		top: number | null;
		bottom: number | null;
		/** The room on the chosen side, which the popup scrolls within. */
		maxHeight: number;
	}>({
		left: 0,
		top: 0,
		bottom: null,
		maxHeight: PICKER_MAX_HEIGHT
	});
	/** What the manual picker's query field holds, and the field itself. */
	let manualQuery = $state('');
	let queryField = $state<HTMLInputElement | null>(null);
	let pickerBox = $state<HTMLElement | null>(null);
	/** The list scrolls, so the arrow keys have to bring the row they reach into view. */
	$effect(() => {
		if (!picker) return;
		pickerBox
			?.querySelector(`#ref-picker-${step.id}-${picker.active}`)
			?.scrollIntoView({ block: 'nearest' });
	});
	/** Said in the live region when there was no word to link. */
	let notice = $state('');
	/** Whether focus is anywhere in this step; the link button is offered only then. */
	let editing = $state(false);
	/** The caret as an offset into the plain text, while the field has focus and nothing is selected. */
	let caret = $state<number | null>(null);
	/**
	 * A touch screen with little room - a phone, upright below `md:` or on its
	 * side under 32rem tall - gets the sheet; everything else the card at the
	 * caret. By pointer as well as width: a narrow window with a mouse has no
	 * taps to open a sheet with, and a phone on its side is wider than `md:`
	 * but no roomier once the keyboard is up.
	 */
	const phone = new MediaQuery(
		'(pointer: coarse) and ((max-width: 47.99rem) or (max-height: 32rem))'
	);
	// Turned between the two while a sheet was up: the sheet goes, rather than
	// lingering as a framed word and coming back on the next turn.
	$effect(() => {
		if (!phone.current) sheet = null;
	});
	/**
	 * On a phone, the marked word a tap opened the sheet for, or its later
	 * occurrence (`repeat`). A phone shows no card at the caret: the keyboard
	 * leaves too little room, and nothing pins a popup to it reliably in
	 * every browser (iOS keeps fixed elements behind it, Android moves them
	 * with the address bar), so a tap closes the keyboard and the decision
	 * gets the app's bottom sheet, where every option has room.
	 */
	let sheet = $state<{ word: string; repeat: boolean; end: number } | null>(null);
	/**
	 * A touch on the step in progress: where and when it began, and whether
	 * it has moved too far to be a tap. Null for a second finger or a
	 * cancelled touch, neither of which is a tap.
	 */
	let touch: { x: number; y: number; at: number; moved: boolean } | null = null;
	/** Where the desktop card sits; a phone's sheet places itself. */
	let cardAt = $state<{
		left: number;
		top: number | null;
		bottom: number | null;
		/** Set when the card fits on neither side of the word: it scrolls within this. */
		maxHeight: number | null;
	} | null>(null);
	/** The caret at which Escape closed the card; it stays closed until the caret moves. */
	let escapedAt = $state<number | null>(null);
	$effect(() => {
		if (caret !== escapedAt) escapedAt = null;
	});
	let cardBox = $state<HTMLElement | null>(null);
	/** The card's scrolling body, whose full height is what placing it measures. */
	let cardScroll = $state<HTMLElement | null>(null);
	let sheetBody = $state<HTMLElement | null>(null);
	/** The option a choice card's radio rows were set to; until then the guess counts. */
	let picked = $state<{ word: string; id: string | null } | null>(null);

	const TIME_UNITS: { value: TimeUnit; label: () => string }[] = [
		{ value: 'seconds', label: m.editor_time_unit_seconds },
		{ value: 'minutes', label: m.editor_time_unit_minutes },
		{ value: 'hours', label: m.editor_time_unit_hours }
	];
	/** The phrase "Mark as time" is marking while its popover is open, and the unit chosen for it. */
	let timePhrase = $state<string | null>(null);
	let timeUnit = $state<TimeUnit>('minutes');
	/** Where the popover hangs: the caret where the selection started. */
	let timeAnchor = $state<{ getBoundingClientRect: () => DOMRect } | null>(null);
	let timeBox = $state<HTMLElement | null>(null);
	/** Set when a press outside closed the popover: focus goes where that press put it. */
	let timeClosedOutside = false;

	// The matcher, once per edit for both.
	const found = $derived(proposals(step, groups));
	const pending = $derived(found.pending);
	/** Words naming an ingredient in several groups; a guessed one is in `pending` too. */
	const choices = $derived(found.choices);
	/** Every word drawn as a proposal: the plain ones, and the choices with or without a guess. */
	const proposed = $derived([
		...pending.map((ref) => ref.word),
		...choices.filter((choice) => choice.guess === null).map((choice) => choice.word)
	]);
	/** Stored times whose phrase is in the text, the same rule `shown` follows for links. */
	const shownTimes = $derived(presentTimes(step.times, step.text));
	const pendingTimesList = $derived(pendingTimes(step));

	/**
	 * The links whose word stands in the text right now. Editing a word away
	 * drops its link (`dropAbsent`, undone with it); this only keeps out a
	 * link loaded with a word the text lacks.
	 */
	const shown = $derived(presentReferences(step.references, step.text));

	/** Links, times and proposals in the order their words appear, which is how the list reads them. */
	function byPosition<T>(items: T[], word: (item: T) => string): T[] {
		const at = (item: T) => firstWordMatch(step.text, word(item))?.from ?? 0;
		return [...items].sort((a, b) => at(a) - at(b));
	}

	/**
	 * The marked word the caret stands in, which is what the card at the word
	 * is about. Nothing while a picker or the time popover is open: that popup
	 * is the one the author is working in, and two at the same caret would
	 * cover each other.
	 */
	const markedWord = $derived(
		picker !== null || timePhrase !== null || caret === null
			? null
			: wordAtCaret(
					step.text,
					[
						...shown.map((ref) => ref.word),
						...proposed,
						...caretTimePhrases(shownTimes, pendingTimesList)
					],
					caret
				)
	);
	/**
	 * The word whose card "Dismiss" or an accept closed. Nothing else
	 * changes: a dismissed word stays proposed, as if the recipe were opened
	 * again, and the card comes back once the caret has left the word and
	 * returns.
	 */
	// Kept with the word's state, so an undo that turns a just accepted link
	// back into a proposal brings its card back.
	let closed = $state<{ word: string; taken: boolean } | null>(null);
	const taken = (word: string) =>
		step.references.some((ref) => ref.word === word) ||
		step.times.some((time) => time.phrase === word);
	$effect(() => {
		if (markedWord !== closed?.word) closed = null;
	});
	const cardWord = $derived(
		phone.current
			? sheet !== null && !sheet.repeat
				? sheet.word
				: null
			: markedWord !== null && markedWord === closed?.word && taken(markedWord) === closed.taken
				? null
				: markedWord
	);

	/**
	 * A linked or proposed word whose later occurrence holds the caret: its
	 * card says the link sits at the first occurrence, where the quantity is.
	 */
	const cardRepeat = $derived(
		phone.current
			? sheet?.repeat
				? sheet.word
				: null
			: cardWord !== null || picker !== null || timePhrase !== null || caret === null
				? null
				: repeatAtCaret(step.text, [...shown.map((ref) => ref.word), ...proposed], caret)
	);
	const cardLink = $derived(shown.find((ref) => ref.word === cardWord) ?? null);
	/**
	 * The decision at the word, a plain proposal included: one option,
	 * picked. Every card that asks looks the same, one option or four.
	 */
	const cardChoice = $derived.by<Choice | null>(() => {
		if (cardLink !== null) return null;
		const choice = choices.find((found) => found.word === cardWord);
		if (choice) return choice;
		const ref = pending.find((found) => found.word === cardWord);
		return ref ? { word: ref.word, options: [ref], guess: ref, because: [] } : null;
	});
	/** The time or time proposal at the caret, and whether it is stored. */
	const cardAnyTime = $derived(
		[...shownTimes, ...pendingTimesList].find((time) => time.phrase === cardWord) ?? null
	);
	const cardTime = $derived(cardAnyTime !== null && shownTimes.includes(cardAnyTime));
	/** The card's frame: its accessible name, the sheet's heading, the card's border. */
	const cardMeta = $derived.by(() => {
		const word = cardChoice?.word ?? cardLink?.word ?? cardRepeat;
		if (word != null) {
			return {
				label: m.editor_reference_card({ word }),
				heading: cardChoice
					? m.editor_sheet_link()
					: cardLink
						? m.editor_sheet_linked()
						: m.editor_sheet_repeat(),
				border: 'border-primary'
			};
		}
		if (cardAnyTime === null) return null;
		return {
			label: m.editor_time_card({ phrase: cardAnyTime.phrase }),
			heading: cardTime ? m.editor_sheet_time() : m.editor_sheet_take_time(),
			border: 'border-time-foreground'
		};
	});

	/** The class a confirmed reference's word is underlined with. */
	function confirmedClass(ref: FormRef): string {
		return isResolved(ref, entries) ? 'ref-confirmed' : 'ref-broken';
	}

	/**
	 * Everything the decorations draw: each word and the class it is drawn in,
	 * as one comparable string. The editor re-draws decorations on every
	 * transaction of its own, so the only thing that has to push an update into
	 * ProseMirror is this changing - which is why the effect below keys on it
	 * rather than on the text. The class belongs in here as much as the word
	 * does: renaming an ingredient breaks a link without touching the step, and
	 * the underline has to say so.
	 */
	const decoratedWords = $derived(
		JSON.stringify([
			...shown.map((ref) => `${confirmedClass(ref)}:${ref.word}`),
			...proposed.map((word) => `ref-suggestion:${word}`),
			...shownTimes.map((time) => `time-link:${time.phrase}`),
			...pendingTimesList.map((time) => `time-proposal:${time.phrase}`),
			`active:${sheet?.end ?? ''}`
		])
	);

	/**
	 * Marks the referenced and the proposed words, and the stored and proposed times.
	 *
	 * Decorations are ProseMirror's way of styling a document WITHOUT changing
	 * it: nothing here writes into the doc, so opening a recipe leaves the
	 * author's text byte for byte as they wrote it. Anything that inserted
	 * markup instead would be saved back as literal text.
	 *
	 * Only the first occurrence of each word is marked, the same one
	 * `segmentStep` annotates on the detail page, so the author sees exactly
	 * where the quantity will appear.
	 */
	function decorate(doc: ProseMirrorNode): DecorationSet {
		// Plain arrays rather than a Map and a Set: both are local to this call
		// and hold a handful of entries, and `svelte/prefer-svelte-reactivity`
		// rightly refuses a built-in collection inside a component.
		const words = untrack(() => {
			const marked = shown.map((ref) => ({
				word: ref.word,
				cls: confirmedClass(ref)
			}));
			const add = (word: string, cls: string) => {
				if (!marked.some((entry) => entry.word === word)) marked.push({ word, cls });
			};
			for (const word of proposed) add(word, 'ref-suggestion');
			for (const time of shownTimes) add(time.phrase, 'time-link');
			for (const time of pendingTimesList) add(time.phrase, 'time-proposal');
			return marked;
		});
		// The occurrence a phone's sheet is about, by where it ends.
		const active = untrack(() => sheet?.end ?? null);
		const lit = (end: number) => (end === active ? ' ref-active' : '');
		const seen: string[] = [];
		const decorations: Decoration[] = [];
		doc.descendants((node, pos) => {
			const text = node.text;
			if (!node.isText || text === undefined) return;
			for (const { word, cls } of words) {
				const hit = firstWordMatch(text, word);
				if (hit === null) continue;
				// A link's or proposal's later occurrences are drawn faintly:
				// known, but carrying no quantity of their own. Times are not.
				const repeats = cls.startsWith('ref-')
					? seen.includes(word)
						? [hit, ...laterWordMatches(text, word)]
						: laterWordMatches(text, word)
					: [];
				for (const later of repeats) {
					decorations.push(
						Decoration.inline(pos + later.from, pos + later.to, {
							class: `ref-repeat${lit(pos + later.to)}`,
							'data-repeat-word': word
						})
					);
				}
				if (seen.includes(word)) continue;
				seen.push(word);
				decorations.push(
					Decoration.inline(pos + hit.from, pos + hit.to, {
						class: `${cls}${lit(pos + hit.to)}`,
						'data-ref-word': word
					})
				);
			}
		});
		return DecorationSet.create(doc, decorations);
	}

	/**
	 * Opens the picker: fixes the side it will sit on and places it there.
	 *
	 * The side is chosen against `PICKER_MAX_HEIGHT` rather than against the
	 * list's height, so the answer does not depend on how many rows the first
	 * query happens to match - below the caret unless there is not enough room
	 * there and more room above.
	 */
	function openPicker(props: SuggestionProps<PickerEntry, PickerEntry>) {
		if (props.clientRect == null) {
			anchor = null;
			return;
		}
		openAt(props.clientRect);
	}

	/** Fixes the side and places the popup, for whichever flow opened it. */
	function openAt(rect: () => DOMRect | null) {
		const at = rect();
		if (at === null) {
			anchor = null;
			return;
		}
		const area = visibleArea();
		const below = area.bottom - at.bottom;
		const above = at.top - area.top;
		const fits = below >= PICKER_MAX_HEIGHT + PICKER_GAP + PICKER_MARGIN;
		anchor = { rect, side: fits || below >= above ? 'below' : 'above' };
		place();
	}

	/** Takes the narrowed list without touching where the popup sits. */
	function updatePicker(props: SuggestionProps<PickerEntry, PickerEntry>, reset: boolean) {
		const previous = reset ? null : picker;
		notice = '';
		picker = {
			items: props.items,
			active:
				previous === null ? 0 : Math.min(previous.active, Math.max(0, props.items.length - 1)),
			command: props.command,
			word: null,
			query: props.query
		};
	}

	function choose(at: number) {
		const item = picker?.items[at];
		if (picker && item) {
			picker.command(item);
		}
	}

	/**
	 * A ProseMirror position as an offset into the plain text the field stores.
	 * `textBetween` joins blocks with the same `\n` `stepText` does, which is
	 * what keeps the two in step in a field holding several paragraphs.
	 */
	function plainOffset(view: EditorView, pos: number): number {
		return view.state.doc.textBetween(0, pos, '\n').length;
	}

	/**
	 * From the selection's start down to the bottom of its last line, so a
	 * popover hung below it never covers a phrase that wrapped onto a second line.
	 */
	function selectionRect(view: EditorView, from: number, to: number): DOMRect | null {
		const start = caretRect(view, from);
		const end = caretRect(view, to);
		if (!start || !end) return start;
		return new DOMRect(start.left, start.top, 0, Math.max(start.bottom, end.bottom) - start.top);
	}

	/** The caret's rectangle, the same shape the `@` flow's `clientRect` hands over. */
	function caretRect(view: EditorView, pos: number): DOMRect | null {
		if (pos > view.state.doc.content.size) return null;
		const at = view.coordsAtPos(pos);
		return new DOMRect(at.left, at.top, 0, at.bottom - at.top);
	}

	/**
	 * The third way to link, and the only one that reaches a word differing
	 * from the ingredient's name: the author selects a word - or simply leaves
	 * the caret in one - and picks what it means. "Fleisch" points at
	 * `Rinderbraten` this way, which is the whole reason `word` and
	 * `ingredient` are separate fields.
	 *
	 * Nothing is written into the document. The word is already in the
	 * sentence, so only the reference is added, and the decoration effect
	 * redraws the underline from it.
	 */
	function openManual() {
		const view = editor?.view;
		if (!view) return;
		const { from, to } = view.state.selection;
		const word = wordAt(step.text, plainOffset(view, from), plainOffset(view, to));
		// `wordAt` already grows to whole words, so this only fails on text
		// the editor and `step.text` disagree about. Refusing is the safe half
		// of that disagreement: the API rejects a word it cannot find.
		if (word === null || firstWordMatch(step.text, word) === null) {
			notice = m.editor_reference_no_word();
			return;
		}
		// A later occurrence: the link would land on the first one, silently.
		if (plainOffset(view, from) >= (firstWordMatch(step.text, word)?.to ?? Infinity)) {
			notice = m.editor_reference_repeat({ word });
			return;
		}
		if (!fitsWordLimit(word)) {
			notice = m.editor_reference_too_long();
			return;
		}
		if (
			overlapsAny(
				step.text,
				word,
				[...shownTimes, ...pendingTimesList].map((time) => time.phrase)
			)
		) {
			notice = m.editor_reference_in_time();
			return;
		}
		openManualFor(word, () => caretRect(view, from));
	}

	/**
	 * "Mark as time", for a duration the detector does not know: the selection
	 * grows to whole words the way "Link word"'s does, and only one holding a
	 * number opens the popover. The number is read from the phrase; the unit
	 * is the author's choice, so a phrase like "10 Min" in a language the
	 * catalogs lack still gets the right length.
	 */
	function openTimeMark() {
		const view = editor?.view;
		if (!view) return;
		const { from, to } = view.state.selection;
		const phrase = wordAt(step.text, plainOffset(view, from), plainOffset(view, to));
		if (phrase !== null && [...phrase].length > MAX_PHRASE_LENGTH) {
			notice = m.editor_time_too_long();
			return;
		}
		// A number out of range for minutes may fit another unit: the
		// popover's disabled "Set" says so, not this refusal.
		if (phrase === null || firstWordMatch(step.text, phrase) === null || !/\p{N}/u.test(phrase)) {
			notice = m.editor_time_no_number();
			return;
		}
		if (isMarked(step, phrase)) {
			notice = m.editor_time_already_marked();
			return;
		}
		// One popup at a time: the time popover takes the picker's place.
		if (picker?.word === null) exitSuggestion(view, PICKER_KEY);
		picker = null;
		notice = '';
		timePhrase = phrase;
		timeUnit = 'minutes';
		timeClosedOutside = false;
		timeAnchor = { getBoundingClientRect: () => selectionRect(view, from, to) ?? new DOMRect() };
	}

	function setTime() {
		const time = timePhrase === null ? null : manualTime(timePhrase, timeUnit);
		if (time !== null) step.times = addTime(step.times, time, step.text);
		timePhrase = null;
	}

	/** Opens the manual picker for `word`, at `rect`; the card's "Change" comes in here. */
	function openManualFor(word: string, rect: () => DOMRect | null) {
		timePhrase = null;
		notice = '';
		manualQuery = '';
		picker = {
			items: matchEntries(entries, ''),
			active: 0,
			command: (entry) => linkWord(word, entry),
			word,
			query: ''
		};
		openAt(rect);
		void tick().then(() => queryField?.focus());
	}

	/** Narrows the manual list, leaving the popup where it sits. */
	function updateManual(query: string) {
		manualQuery = query;
		const open = picker;
		if (open === null) return;
		open.items = matchEntries(entries, query);
		open.query = query;
		open.active = Math.min(open.active, Math.max(0, open.items.length - 1));
	}

	function linkWord(word: string, entry: PickerEntry) {
		step.references = addReference(step.references, {
			word,
			ingredientId: entry.ingredientId,
			groupName: entry.group,
			ingredientName: entry.name
		});
		closeManual();
	}

	/** Closes the manual picker and hands the caret back to the step. */
	function closeManual() {
		picker = null;
		anchor = null;
		editor?.commands.focus();
	}

	/**
	 * The query field's keys, matching what the suggestion plugin does for the
	 * `@` flow: the arrows walk the list, Enter takes the active row, Escape
	 * gives up. Enter is prevented rather than passed on because the field
	 * sits inside the recipe form, where the default is a submit.
	 */
	function manualKeys(event: KeyboardEvent) {
		const open = picker;
		if (open === null) return;
		if (event.key === 'Escape') {
			event.preventDefault();
			closeManual();
			return;
		}
		if (event.key === 'Enter') {
			event.preventDefault();
			if (open.items.length > 0) choose(open.active);
			return;
		}
		const count = open.items.length;
		if (count === 0) return;
		if (event.key === 'ArrowDown') {
			event.preventDefault();
			open.active = (open.active + 1) % count;
		} else if (event.key === 'ArrowUp') {
			event.preventDefault();
			open.active = (open.active - 1 + count) % count;
		}
	}

	/**
	 * A press outside the `@` popup closes it, as the manual picker closes on
	 * blur. A press in the step itself is Tiptap's: the caret staying in the
	 * `@query` keeps the list, leaving it ends the suggestion there.
	 */
	function closePickerOutside(event: PointerEvent) {
		const view = editor?.view;
		if (picker?.word !== null || !view) return;
		const target = event.target as Node;
		if (pickerBox?.contains(target) || view.dom.contains(target)) return;
		exitSuggestion(view, PICKER_KEY);
	}

	/**
	 * Anything that takes focus out of the query field closes the popup -
	 * clicking elsewhere, or tabbing on. Choosing a row never gets here: the
	 * rows answer to `mousedown` with the default prevented, so the field
	 * keeps focus until the link is made.
	 */
	function manualBlur() {
		if (picker?.word != null) {
			picker = null;
			anchor = null;
		}
	}

	/**
	 * The two ProseMirror plugins this step runs: the decorations, and the `@`
	 * picker. Built per editor so the closures can see this step; a plugin is a
	 * descriptor rather than state, so that costs nothing.
	 */
	function referencePlugins(instance: Editor): Plugin[] {
		return [
			dropAbsent(),
			new Plugin({
				key: new PluginKey('ingredientDecorations'),
				props: { decorations: (state) => decorate(state.doc) }
			}),
			Suggestion<PickerEntry, PickerEntry>({
				editor: instance,
				pluginKey: PICKER_KEY,
				char: '@',
				items: ({ query }) => untrack(() => matchEntries(entries, query)),
				/**
				 * Replaces `@Zuc` with the plain word and records the link. No
				 * mention node and no mark: the field stores a plain string, so
				 * the only trace a choice may leave in the document is the word
				 * itself.
				 *
				 * An `@` typed straight in front of a word would glue the name
				 * onto it (`ZuckerMehl`), so a space goes in between. The link is
				 * still only made if the name then stands as a whole word: the
				 * API refuses one that does not.
				 */
				command: ({ editor: chosen, range, props }) => {
					const after = chosen.state.doc.resolve(range.to).nodeAfter;
					const glued = after?.isText === true && isWordChar(after.text?.[0] ?? '');
					const inserted = glued ? `${props.name} ` : props.name;
					// The word and its link in one transaction, so one undo takes
					// both; `onTransaction` hands the link to the form.
					chosen
						.chain()
						.focus()
						.insertContentAt(range, { type: 'text', text: inserted })
						.command(({ tr }) => {
							const text = tr.doc.textBetween(0, tr.doc.content.size, '\n');
							if (firstWordMatch(text, props.name) === null) return true;
							tr.setDocAttribute(
								'references',
								addReference(wordsOf(tr.doc).references, {
									word: props.name,
									ingredientId: props.ingredientId,
									groupName: props.group,
									ingredientName: props.name
								})
							);
							return true;
						})
						.run();
				},
				render: () => ({
					onStart: (props) => {
						openPicker(props);
						updatePicker(props, true);
					},
					onUpdate: (props) => updatePicker(props, false),
					onExit: () => {
						picker = null;
						anchor = null;
					},
					onKeyDown: ({ event, view }) => {
						const open = picker;
						if (open === null) return false;
						const count = open.items.length;
						if (event.key === 'Escape') {
							exitSuggestion(view, PICKER_KEY);
							return true;
						}
						if (count === 0) return false;
						if (event.key === 'ArrowDown') {
							open.active = (open.active + 1) % count;
							return true;
						}
						if (event.key === 'ArrowUp') {
							open.active = (open.active - 1 + count) % count;
							return true;
						}
						if (event.key === 'Enter' || event.key === 'Tab') {
							choose(open.active);
							return true;
						}
						return false;
					}
				})
			})
		];
	}

	/**
	 * Creates the editor once, for this step, and tears it down with the
	 * element.
	 *
	 * The whole body runs untracked: an attachment re-runs when the state it
	 * reads changes, and re-running this one would throw away the editor - and
	 * the caret - on every keystroke. Everything the editor needs afterwards
	 * reaches it through the effects below instead.
	 */
	function mountEditor(node: HTMLElement) {
		return untrack(() => {
			const instance = new Editor({
				element: node,
				extensions: stepExtensions(m.editor_step_placeholder(), [
					referenceExtension(referencePlugins)
				]),
				content: textToDoc(step.text, stateWords()),
				editorProps: {
					attributes: { id: `step-${step.id}`, class: EDITABLE },
					// Escape closes the card at the caret, which no Tab reaches: it
					// stays closed until the caret moves.
					handleKeyDown: (_view, event) => {
						if (event.key !== 'Escape' || phone.current || picker !== null) return false;
						if (cardMeta === null || caret === null || escapedAt !== null) return false;
						escapedAt = caret;
						return true;
					},
					handleDOMEvents: {
						// A mouse or pen in the step while its sheet is up writes
						// there. The sheet goes first and at once, or its focus trap
						// would take back the focus the press gives the step. A
						// finger is answered at `touchend`, once it is a tap and not
						// a scroll that began on the step.
						pointerdown: (_view, event) => {
							if (event.pointerType === 'touch' || sheet === null) return false;
							if (!(event.target as Element | null)?.closest?.(MARKS)) {
								flushSync(() => (sheet = null));
							}
							return false;
						},
						touchstart: (_view, event) => {
							const first = event.touches[0];
							touch =
								event.touches.length === 1 && first
									? { x: first.clientX, y: first.clientY, at: event.timeStamp, moved: false }
									: null;
							return false;
						},
						touchmove: (_view, event) => {
							const now = event.touches[0];
							if (
								touch &&
								now &&
								Math.hypot(now.clientX - touch.x, now.clientY - touch.y) > TAP_SLOP
							) {
								touch.moved = true;
							}
							return false;
						},
						touchcancel: () => {
							touch = null;
							return false;
						},
						touchend: (view, event) => tapped(view, event)
					}
				},
				// The document is where the links and times
				// change on an undo, a redo or a deleted word: they are written
				// back to the step here, the text in `onUpdate`.
				// Only a transaction that changed them: any other - a caret moved
				// back after a pick - would copy the document's words over a change
				// the form made a moment ago and the effect below has yet to push.
				// The editor's state, not `transaction.doc`: Tiptap reports the
				// root transaction, and `dropAbsent`'s appended one is only in
				// the state it ended in.
				onTransaction: ({ editor: changed, transaction }) => {
					const after = wordsOf(changed.state.doc);
					if (!same(wordsOf(transaction.before), after)) takeWords(after);
				},
				onUpdate: ({ editor: changed }) => {
					step.text = stepText(changed);
					trackCaret(changed);
				},
				onSelectionUpdate: ({ editor: changed }) => {
					trackCaret(changed);
					// A refusal answers the selection it was about; a new one starts clean.
					notice = '';
				},
				onFocus: ({ editor: changed }) => trackCaret(changed),
				onBlur: () => (caret = null)
			});
			editor = instance;
			return () => {
				instance.destroy();
				editor = null;
			};
		});
	}

	/** The step's words as the form holds them, in the shape the document carries. */
	function stateWords(): StepWords {
		return {
			references: $state.snapshot(step.references),
			times: $state.snapshot(step.times)
		};
	}

	const same = (a: unknown, b: unknown) => JSON.stringify(a) === JSON.stringify(b);

	/** The document's words into the form, where they differ. */
	function takeWords(words: StepWords) {
		const held = stateWords();
		if (!same(held.references, words.references)) step.references = [...words.references];
		if (!same(held.times, words.times)) step.times = [...words.times];
	}

	/**
	 * The form's words into the document: every change made outside the
	 * editor - a card's or the list's button, "Accept all" above the steps -
	 * becomes a transaction of its own, which is what lets Cmd+Z take it back.
	 * `closeHistory` keeps it out of the typing before it. What `takeWords`
	 * just wrote back is equal to the document already, so this cannot loop.
	 */
	$effect(() => {
		const wanted = stateWords();
		const view = editor?.view;
		if (!view) return;
		untrack(() => {
			const held = wordsOf(view.state.doc);
			let tr = null;
			for (const key of STEP_WORDS) {
				if (!same(held[key], wanted[key]))
					tr = (tr ?? view.state.tr).setDocAttribute(key, wanted[key]);
			}
			if (tr) view.dispatch(closeHistory(tr));
		});
	});

	function trackCaret(instance: Editor) {
		const { selection } = instance.state;
		caret =
			instance.isFocused && selection.empty ? plainOffset(instance.view, selection.from) : null;
	}

	/**
	 * Pushes a decoration refresh into ProseMirror when the word list changes.
	 *
	 * This is the Svelte-to-ProseMirror half of the bridge (the other half is
	 * `onUpdate` writing `step.text` back). It is needed because `onUpdate`
	 * runs AFTER the transaction that caused it, so the decorations drawn
	 * during that transaction were computed from the previous text. The
	 * transaction dispatched here carries only metadata - it changes no
	 * content, so it does not fire `onUpdate` and cannot loop.
	 */
	$effect(() => {
		void decoratedWords;
		const view = editor?.view;
		if (!view) return;
		untrack(() => view.dispatch(view.state.tr.setMeta('ingredientReferences', true)));
	});

	/**
	 * The attributes that change while the editor lives. They are set on the
	 * element rather than through `editorProps`, which ProseMirror only reads
	 * when the view is created; ProseMirror removes only the attributes it set
	 * itself, so these survive its updates.
	 */
	$effect(() => {
		const dom = editor?.view.dom;
		if (!dom) return;
		dom.setAttribute('aria-label', m.editor_step_number({ number: index + 1 }));
		dom.setAttribute('aria-multiline', 'true');
		if (error) {
			dom.setAttribute('aria-invalid', 'true');
			dom.setAttribute('aria-describedby', `step-error-${step.id}`);
		} else {
			dom.removeAttribute('aria-invalid');
			dom.removeAttribute('aria-describedby');
		}
	});

	/**
	 * Points the field at the row the arrow keys have walked to.
	 *
	 * `aria-controls` and `aria-activedescendant` are both supported on a
	 * textbox, which is what a contenteditable is. `aria-expanded` is NOT -
	 * that one belongs to a combobox - so the picker's open state is announced
	 * by the live region in the markup instead of by an attribute a screen
	 * reader is free to ignore. Making the field a real combobox was the other
	 * option and is the wrong one here: the field is a multi-line text area
	 * that happens to offer a completion, and every step would stop being a
	 * textbox to assistive tech and to the tests that locate it as one.
	 */
	$effect(() => {
		const dom = editor?.view.dom;
		if (!dom) return;
		const open = picker;
		// The manual picker owns its own query field, and that field is the
		// combobox pointing at the list; the step would then be a second
		// element claiming to control it.
		if (open === null || open.word !== null || open.items.length === 0) {
			dom.removeAttribute('aria-controls');
			dom.removeAttribute('aria-activedescendant');
			return;
		}
		dom.setAttribute('aria-controls', `ref-picker-${step.id}`);
		dom.setAttribute('aria-activedescendant', `ref-picker-${step.id}-${open.active}`);
	});

	/** What the live region says while the picker is open. */
	const pickerStatus = $derived.by(() => {
		if (picker === null) return '';
		if (picker.items.length === 0) return m.editor_reference_no_match();
		return picker.items.length === 1
			? m.editor_reference_picker_status_one()
			: m.editor_reference_picker_status({ count: picker.items.length });
	});

	/**
	 * Puts the popup at the caret, on the side `openPicker` chose.
	 *
	 * Nothing here measures the popup, which is what keeps it still: the maths
	 * is the caret's rectangle and two constants, so narrowing the list cannot
	 * move it. `position: fixed` takes the client rect straight from the
	 * plugin, so no offset parent is involved either.
	 */
	function place() {
		const rect = anchor?.rect() ?? null;
		if (rect === null) return;
		const area = visibleArea();
		const left = Math.max(
			PICKER_MARGIN,
			Math.min(rect.left, area.width - drawnWidth(pickerBox, PICKER_WIDTH) - PICKER_MARGIN)
		);
		const room = (space: number) => Math.max(0, space - PICKER_GAP - PICKER_MARGIN);
		pickerAt =
			anchor?.side === 'above'
				? {
						left,
						top: null,
						bottom: area.height - rect.top + PICKER_GAP,
						maxHeight: room(rect.top - area.top)
					}
				: // Floored, for the moment a caret sits under the sticky bar and
					// the browser has not scrolled it into view yet: a popup with a
					// negative top would be a list nobody can see.
					{
						left,
						top: Math.max(area.top + PICKER_MARGIN, rect.bottom + PICKER_GAP),
						bottom: null,
						maxHeight: room(area.bottom - Math.max(area.top, rect.bottom))
					};
	}

	/**
	 * Follows the caret when the page moves under it, and only then: the list
	 * changing is deliberately not a reason to re-place. The side is chosen
	 * again on the way, since a scroll or the on-screen keyboard coming up can
	 * take the room it was chosen for - the keyboard resizes only the visual
	 * viewport, so that is listened to as well.
	 */
	$effect(() => {
		if (anchor === null) return;
		const follow = () => {
			if (anchor) openAt(anchor.rect);
		};
		const viewport = window.visualViewport;
		window.addEventListener('scroll', follow, true);
		window.addEventListener('resize', follow);
		viewport?.addEventListener('resize', follow);
		// iOS pans only the visual viewport while the keyboard is up.
		viewport?.addEventListener('scroll', follow);
		return () => {
			window.removeEventListener('scroll', follow, true);
			window.removeEventListener('resize', follow);
			viewport?.removeEventListener('resize', follow);
			viewport?.removeEventListener('scroll', follow);
		};
	});

	/**
	 * A popup's width as drawn, or before its first draw the design width in
	 * the reader's text size: the widths are `rem`, so a larger default font
	 * widens the popup, and a 256px guess put a 384px card past the edge.
	 */
	function drawnWidth(box: HTMLElement | null, designed: number): number {
		if (box) return box.offsetWidth;
		return (designed * parseFloat(getComputedStyle(document.documentElement).fontSize)) / 16;
	}

	/**
	 * The part of the window a popup may use, in the coordinates of
	 * `position: fixed`: the client area without its scrollbars (`innerWidth`
	 * counts a classic scrollbar, and a card clamped against it slid under
	 * it), from below the pinned bars at the top down to the on-screen
	 * keyboard, which shrinks only the visual viewport.
	 */
	function visibleArea() {
		const width = document.documentElement.clientWidth;
		const height = document.documentElement.clientHeight;
		const viewport = window.visualViewport;
		return {
			width,
			height,
			top: Math.max(coveredTop(), viewport?.offsetTop ?? 0),
			bottom: viewport ? Math.min(height, viewport.offsetTop + viewport.height) : height
		};
	}

	/** The marked word's element, which the card sits against. */
	function wordElement(word: string): HTMLElement | null {
		const dom = editor?.view.dom;
		if (!dom) return null;
		for (const element of dom.querySelectorAll<HTMLElement>('[data-ref-word]')) {
			if (element.dataset.refWord === word) return element;
		}
		return null;
	}

	/**
	 * Puts the desktop card under the word, or over it where the room below
	 * is too small. A phone's sheet places itself.
	 */
	function placeCard() {
		if (phone.current) return;
		const view = editor?.view;
		const rect =
			cardWord !== null
				? (wordElement(cardWord)?.getBoundingClientRect() ?? null)
				: cardRepeat !== null && view
					? caretRect(view, view.state.selection.from)
					: null;
		if (rect === null) {
			cardAt = null;
			return;
		}
		const area = visibleArea();
		const left = Math.max(
			PICKER_MARGIN,
			Math.min(rect.left, area.width - drawnWidth(cardBox, CARD_WIDTH) - PICKER_MARGIN)
		);
		// The whole card, not what a cap left visible of it: scrollHeight plus the
		// border. Before it is drawn there is nothing to measure; the effect on
		// `cardBox` places it again, before it is painted.
		const height = cardScroll ? cardScroll.scrollHeight + 2 : 0;
		const below = area.bottom - rect.bottom - PICKER_GAP - PICKER_MARGIN;
		const above = rect.top - area.top - PICKER_GAP - PICKER_MARGIN;
		const fits = height <= below || height <= above;
		const down = height <= below || (!fits && below >= above);
		const maxHeight = fits ? null : Math.max(below, above);
		cardAt = down
			? { left, top: rect.bottom + PICKER_GAP, bottom: null, maxHeight }
			: { left, top: null, bottom: area.height - rect.top + PICKER_GAP, maxHeight };
	}

	/**
	 * Where the screen stops being visible at the top: the bottom edge of the
	 * bars pinned there (`data-covers-top`, the phone's save bar and running
	 * head), or 0 where none is.
	 */
	function coveredTop(): number {
		const bars = [...document.querySelectorAll<HTMLElement>('[data-covers-top]')]
			.map((bar) => bar.getBoundingClientRect())
			.filter((box) => box.height > 0)
			.sort((a, b) => a.top - b.top);
		let edge = 0;
		// Only a bar that starts where the one above it ends is pinned there.
		for (const box of bars) if (box.top <= edge + 1) edge = Math.max(edge, box.bottom);
		return edge;
	}

	// Once drawn, the card is placed again by its measured height.
	$effect(() => {
		if (cardBox) untrack(placeCard);
	});

	// Places the card once the decoration it points at has been drawn.
	$effect(() => {
		void decoratedWords;
		if (phone.current || (cardWord === null && cardRepeat === null)) {
			cardAt = null;
			return;
		}
		void tick().then(() => untrack(placeCard));
	});

	$effect(() => {
		if (phone.current || (cardWord === null && cardRepeat === null)) return;
		const viewport = window.visualViewport;
		window.addEventListener('scroll', placeCard, true);
		window.addEventListener('resize', placeCard);
		viewport?.addEventListener('resize', placeCard);
		viewport?.addEventListener('scroll', placeCard);
		return () => {
			window.removeEventListener('scroll', placeCard, true);
			window.removeEventListener('resize', placeCard);
			viewport?.removeEventListener('resize', placeCard);
			viewport?.removeEventListener('scroll', placeCard);
		};
	});

	/**
	 * "Change" on the card: the manual picker, for the word the card is about.
	 * The sheet goes and the picker comes within the tap, so its field takes
	 * the focus inside the gesture - which is what lets iOS raise the keyboard.
	 */
	function changeLink(word: string) {
		flushSync(() => {
			sheet = null;
			openManualFor(word, () => wordElement(word)?.getBoundingClientRect() ?? null);
		});
		queryField?.focus();
	}

	/**
	 * Closes a phone's sheet after one of its actions, or its handle, Escape
	 * or a tap on the dim, and hands focus to the step's list toggle: the
	 * button it held is gone, and the step itself would raise the keyboard.
	 */
	function closeSheet() {
		if (sheet === null) return;
		flushSync(() => (sheet = null));
		document.getElementById(`step-links-toggle-${step.id}`)?.focus({ preventScroll: true });
	}

	/** Why the guess: at most three words, so the card stays short. */
	function becauseLine(words: string[]): string {
		if (words.length > 3) {
			return m.editor_reference_choice_because_more({
				words: words.slice(0, 2).join(', '),
				count: words.length - 2
			});
		}
		return m.editor_reference_choice_because({
			words: new Intl.ListFormat(getLocale(), { type: 'conjunction' }).format(words)
		});
	}

	/** Takes a proposal. Its card closes with it, as after "Dismiss": the job is done. */
	function accept(ref: FormRef) {
		step.references = addReference(step.references, ref);
		closed = { word: ref.word, taken: true };
		closeSheet();
	}

	/**
	 * A phone's tap on the step, answered when the finger lifts. On a marked
	 * word it opens that word's sheet, or swaps an open sheet's content: the
	 * default is prevented, so the caret stays and no keyboard comes, and
	 * whatever had focus lets go of it, so an open keyboard goes. Elsewhere in
	 * the step it closes an open sheet and lets the tap write there. A touch
	 * that moved, lasted, was cancelled or had a second finger is no tap, and
	 * neither is a lift the browser no longer lets be prevented - it has
	 * scrolled.
	 */
	function tapped(view: EditorView, event: TouchEvent): boolean {
		const start = touch;
		touch = null;
		if (!phone.current || !start || start.moved || event.touches.length > 0) return false;
		if (!event.cancelable || event.timeStamp - start.at > TAP_TIME) return false;
		const mark = (event.target as Element | null)?.closest?.(MARKS);
		if (!(mark instanceof HTMLElement)) {
			if (sheet !== null) flushSync(() => (sheet = null));
			return false;
		}
		event.preventDefault();
		(document.activeElement as HTMLElement | null)?.blur();
		const swap = sheet !== null;
		const end = view.posAtDOM(mark, mark.childNodes.length);
		sheet = mark.dataset.refWord
			? { word: mark.dataset.refWord, repeat: false, end }
			: { word: mark.dataset.repeatWord ?? '', repeat: true, end };
		if (swap) void tick().then(revealWord);
		return true;
	}

	/**
	 * The sheet's keyboard button: back to writing, with the caret at the end
	 * of the word the sheet was about - the one place a tap cannot put it, a
	 * tap on the word opening the sheet. Synchronous, so the focus lands in
	 * the tap that asked for it, which is what lets iOS raise the keyboard,
	 * and after the sheet is gone, or its focus trap would take it back.
	 */
	function editWord() {
		const at = sheet?.end;
		flushSync(() => (sheet = null));
		if (at !== undefined) editor?.chain().focus().setTextSelection(at).run();
	}

	/**
	 * Scrolls the word the sheet is about clear of the sheet where the sheet
	 * covers it, so the step stays in sight above it. The occurrence is found
	 * by its position, the fourth "Salz" as much as the first. Run again once
	 * the visual viewport settles: a keyboard still closing from the tap that
	 * opened the sheet leaves the first measure short.
	 */
	function revealWord() {
		const view = editor?.view;
		const box = sheetBody?.closest<HTMLElement>('[role="dialog"]');
		if (!view || !box || sheet === null) return;
		const viewport = window.visualViewport;
		const bottom = viewport ? viewport.offsetTop + viewport.height : window.innerHeight;
		const top = bottom - box.offsetHeight - PICKER_MARGIN * 2;
		const below = view.coordsAtPos(sheet.end).bottom - top;
		if (below > 0) {
			window.scrollBy({ top: below, behavior: prefersReducedMotion.current ? 'auto' : 'smooth' });
		}
	}

	/** Reveals the word now, and once more when the keyboard has gone. */
	function revealSoon() {
		revealWord();
		const viewport = window.visualViewport;
		if (!viewport) return;
		const again = () => {
			viewport.removeEventListener('resize', again);
			revealWord();
		};
		viewport.addEventListener('resize', again);
		setTimeout(() => viewport.removeEventListener('resize', again), 1000);
	}

	/**
	 * The arrows in a sheet's radio rows: move the pick and the focus to the
	 * next or previous row, around the ends, as a radio group does.
	 */
	function pickByArrow(event: KeyboardEvent, word: string, options: FormRef[], at: number) {
		const delta = { ArrowDown: 1, ArrowRight: 1, ArrowUp: -1, ArrowLeft: -1 }[event.key];
		if (delta === undefined) return;
		event.preventDefault();
		const next = (at + delta + options.length) % options.length;
		picked = { word, id: options[next].ingredientId };
		const rows = (event.currentTarget as HTMLElement).parentElement?.querySelectorAll(
			'[role="radio"]'
		);
		void tick().then(() => (rows?.[next] as HTMLElement | undefined)?.focus());
	}

	/** "Dismiss" on a card: closes it, and nothing more. */
	function dismiss(word: string) {
		closed = { word, taken: taken(word) };
		closeSheet();
	}

	/**
	 * Removes a link. The word becomes a proposal again where the matcher has
	 * one, which is safe because a save takes nothing the author did not
	 * accept.
	 */
	function unlink(word: string) {
		step.references = removeReference(step.references, word);
		closeSheet();
	}

	function acceptTime(time: StepTime) {
		step.times = addTime(step.times, time, step.text);
		closed = { word: time.phrase, taken: true };
		closeSheet();
	}

	/** Removes a time; like `unlink`, it may come back as a proposal. */
	function unmarkTime(phrase: string) {
		step.times = removeTime(step.times, phrase);
		closeSheet();
	}
</script>

<!--
	Focus anywhere in the step - the field, the link button, the list - counts
	as editing it. Leaving for something outside, the picker's own query field
	included, does not.
-->
<div
	class="relative min-w-0"
	onfocusin={() => (editing = true)}
	onfocusout={(event) => {
		if (!event.currentTarget.contains(event.relatedTarget as Node | null)) editing = false;
	}}
>
	<div {@attach mountEditor}></div>

	<StepLinks
		stepId={step.id}
		links={byPosition(shown, (ref) => ref.word)}
		pending={byPosition(
			pending.filter((ref) => !choices.some((choice) => choice.word === ref.word)),
			(ref) => ref.word
		)}
		choices={byPosition(choices, (choice) => choice.word)}
		times={byPosition(shownTimes, (time) => time.phrase)}
		pendingTimes={byPosition(pendingTimesList, (time) => time.phrase)}
		{entries}
		{editing}
		onlinkword={openManual}
		onmarktime={openTimeMark}
		onunlink={unlink}
		onaccept={accept}
		onaccepttime={acceptTime}
		onremovetime={unmarkTime}
	/>

	<!--
		Why "Link word" or "Mark as time" did nothing, for everyone who can see
		it. The status line below says the same to a screen reader, so this
		copy is hidden from it rather than read twice.
	-->
	{#if editing && notice && cardRepeat === null}
		<p aria-hidden="true" class="mt-1 text-caption text-text-muted">{notice}</p>
	{/if}

	<!--
		The picker's open state, and what a click on the link button found. It
		lives here rather than as `aria-expanded` on the field, which is a
		textbox and does not support that attribute.
	-->
	<p role="status" class="sr-only">{notice || pickerStatus}</p>
</div>

<!--
	A popup's actions: one bar across its foot, the fields equally wide and at
	least 46px tall, so a thumb cannot miss them and the main action always
	sits on the right. A label wraps rather than clips when the text is large.
	Pinned to the foot, it stays in reach when the popup scrolls. A desktop
	card never takes focus, so its buttons answer to `mousedown` with the
	default prevented and stay out of the tab order; a phone's sheet (the
	default there) and "Mark as time" (which passes `true`) are dialogs, and
	their buttons take focus.
-->
{#snippet actionBar(actions: BarAction[], focusable = phone.current)}
	<div
		class="sticky -bottom-(--pad) -mx-(--pad) mt-3 -mb-(--pad) grid auto-cols-fr grid-flow-col divide-x divide-border border-t border-border bg-inherit pb-(--foot)"
	>
		{#each actions as action (action.label)}
			<button
				type="button"
				tabindex={focusable ? undefined : -1}
				disabled={action.disabled}
				onmousedown={(event) => {
					if (!focusable) event.preventDefault();
				}}
				onclick={action.run}
				class="flex min-h-11.5 items-center justify-center gap-1.5 px-2 py-2 text-center text-body-sm font-semibold transition focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-primary disabled:opacity-45 disabled:hover:bg-transparent {BAR_LOOK[
					action.look
				]}"
			>
				<action.icon class="size-4 shrink-0" aria-hidden="true" />
				{action.label}
			</button>
		{/each}
	</div>
{/snippet}

<!--
	A card at a marked word or time: what it stands for, and what can be done
	about it. On a desktop it is a card at the word that never takes focus -
	its buttons answer to `mousedown` with the default prevented, so the caret
	stays in the word and typing goes on where it was; the list under the step
	holds the same actions in the tab order, and Escape closes it. Every popup
	there - these cards, the ingredient picker, "Mark as time" - wears one
	frame: a solid border in the color of what it is about, primary for
	ingredients and the time color for durations.

	On a phone it is the app's bottom sheet instead, a dialog that takes
	focus, with a heading over a hairline in place of the colored frame.
	`--pad` is the action bar's bleed to the frame's edges, `--foot` the room
	it keeps below for a phone's home indicator.
-->
{#snippet cardFrame(label: string, heading: string, border: string, body: Snippet)}
	{#if phone.current}
		<BottomSheet
			bind:open={() => sheet !== null, (open) => !open && closeSheet()}
			closeLabel={m.common_close()}
			spotlight={editor?.view.dom ?? null}
			onOpenAutoFocus={(event) => {
				// The sheet itself, so its title is read and no ring lights up on the handle.
				event.preventDefault();
				sheetBody?.focus();
				revealSoon();
			}}
			onCloseAutoFocus={(event) => event.preventDefault()}
		>
			<!--
				What the sheet is for, since a tap on a word is all that opened it;
				shown only, the dialog's name is the label. Beside it the way back
				to writing.
			-->
			<div
				class="-mt-2 mb-3 flex min-h-11 items-center justify-between gap-3 border-b border-border pr-3 pb-1 pl-5"
			>
				<p aria-hidden="true" class="font-display text-card font-medium text-text">{heading}</p>
				<button
					type="button"
					onclick={editWord}
					aria-label={m.editor_reference_edit_word()}
					class="flex size-11 shrink-0 items-center justify-center rounded-full text-text-muted transition hover:bg-surface hover:text-text focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
				>
					<Keyboard class="size-5" aria-hidden="true" />
				</button>
			</div>
			<!--
				Primary text reads in the accent's ink here: on the sheet's
				`background` the primary falls short of 4.5:1.
			-->
			<div
				bind:this={sheetBody}
				tabindex="-1"
				class="min-h-0 flex-1 overflow-y-auto bg-background px-5 pb-5 outline-none [--foot:env(safe-area-inset-bottom)] [--pad:1.25rem] [&_.text-primary]:text-accent-foreground"
			>
				<Dialog.Title class="sr-only">{label}</Dialog.Title>
				{@render body()}
			</div>
		</BottomSheet>
	{:else if cardAt !== null && escapedAt === null}
		<!--
			The frame clips, the body inside it scrolls: a scrollbar that takes
			room (Windows, macOS set to show them) then ends in the frame's
			rounded corners instead of running square past them.
		-->
		<div
			bind:this={cardBox}
			role="group"
			aria-label={label}
			class="fixed z-40 flex w-64 max-w-[calc(100%-1rem)] flex-col overflow-hidden rounded-md border bg-surface-elevated shadow-card {border}"
			style="left: {cardAt.left}px; {cardAt.top === null
				? `bottom: ${cardAt.bottom}px`
				: `top: ${cardAt.top}px`}; {cardAt.maxHeight === null
				? ''
				: `max-height: ${cardAt.maxHeight}px`}"
		>
			<div
				bind:this={cardScroll}
				class="min-h-0 flex-1 overflow-y-auto overscroll-contain bg-surface-elevated p-3 [--foot:0px] [--pad:0.75rem]"
			>
				{@render body()}
			</div>
		</div>
	{/if}
{/snippet}

<!--
	The card's content, whichever it is, rendered in one frame below: on a
	phone one sheet stays up while a tap on another word swaps what is in it,
	where a frame per kind closed and opened again on every switch.
-->
{#snippet cardBody()}
	{#if cardChoice}
		{@const choice = cardChoice}
		<!--
			A word naming one ingredient in several groups: every option as a
			radio row, the one the sentence points at first, preselected and with
			why. "Accept" links the picked one, as on every other proposal, so a
			stray tap never links the wrong row; without a guess nothing is picked
			and "Accept" waits. "Dismiss" only closes the card. In a phone's
			sheet the rows are a radio group the keyboard walks with the arrows.

			The head noun is what matched, so the options may name different
			ingredients ("Salz", "grobes Salz") or share a group. What tells them
			apart is the row's title - the group, or the ingredient where they all
			share one - and the other name is the line under it when it differs.
		-->
		{@const named = new Set(choice.options.map((option) => option.ingredientName)).size > 1}
		{@const groups = new Set(choice.options.map((option) => option.groupName)).size}
		{@const sameGroup = groups === 1}
		<!-- A pick of a row that is gone counts for nothing: the guess stands. -->
		{@const chosen =
			choice.options.find(
				(option) => picked?.word === choice.word && option.ingredientId === picked.id
			) ?? choice.guess}
		{@const ordered = guessFirst(choice)}
		<!-- On a phone, where height is short: the word and the count share a line. -->
		<div
			class="flex flex-col max-md:flex-row max-md:items-baseline max-md:justify-between max-md:gap-3"
		>
			<p class="text-body-sm font-medium text-primary">{choice.word}</p>
			{#if choice.options.length > 1}
				<p class="text-caption text-text-muted">
					{groups === choice.options.length
						? m.editor_reference_choice_groups({ count: choice.options.length })
						: m.editor_reference_choice_options({ count: choice.options.length })}
				</p>
			{/if}
		</div>
		<div role="radiogroup" aria-label={choice.word} class="mt-2.5 grid gap-2 max-md:mt-2">
			{#each ordered as option, at (option.ingredientId)}
				{@const view = refView(option, entries)}
				{@const on = option === chosen}
				{@const group = option.groupName ?? m.editor_reference_choice_no_group()}
				{@const under =
					choice.options.length === 1 ? view.group : sameGroup ? group : named ? view.name : ''}
				<button
					type="button"
					role="radio"
					aria-checked={on}
					aria-label={refLabel(option, entries)}
					aria-describedby={option === choice.guess && choice.because.length > 0
						? `choice-because-${step.id}`
						: undefined}
					tabindex={phone.current && (on || (chosen === null && at === 0)) ? 0 : -1}
					onmousedown={(event) => {
						if (!phone.current) event.preventDefault();
					}}
					onclick={() => (picked = { word: choice.word, id: option.ingredientId })}
					onkeydown={(event) => pickByArrow(event, choice.word, ordered, at)}
					class="flex min-h-11 items-center gap-2.5 rounded-md border bg-surface-elevated px-2.5 py-2 text-left transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary max-md:py-1.5 {on
						? 'border-primary ring-1 ring-primary'
						: 'border-border hover:border-primary'}"
				>
					<span
						aria-hidden="true"
						class="grid size-4.5 shrink-0 place-items-center rounded-pill border-2 {on
							? 'border-primary'
							: 'border-text-muted'}"
					>
						{#if on}<span class="size-2 rounded-pill bg-primary"></span>{/if}
					</span>
					<span class="min-w-0 flex-1">
						<span class="flex items-baseline justify-between gap-3">
							<span class="line-clamp-2 text-body-sm font-semibold text-text">
								{sameGroup ? view.name : group}
							</span>
							<span class="shrink-0 text-caption text-text-muted tabular-nums">
								{view.amount}
							</span>
						</span>
						<!-- A lone option names its group only where the recipe has several. -->
						{#if under}
							<span class="block text-caption text-text-muted">{under}</span>
						{/if}
						<!-- Why this one was guessed: on a phone too, where the sheet has the room. -->
						{#if option === choice.guess && choice.because.length > 0}
							<span id="choice-because-{step.id}" class="block text-caption text-text-muted">
								{becauseLine(choice.because)}
							</span>
						{/if}
					</span>
				</button>
			{/each}
		</div>
		{@render actionBar([
			{
				label: m.editor_reference_dismiss_action(),
				icon: X,
				look: 'plain',
				run: () => dismiss(choice.word)
			},
			{
				label: m.editor_reference_accept_action(),
				icon: Check,
				look: 'positive',
				run: () => chosen && accept(chosen),
				disabled: chosen === null
			}
		])}
	{:else if cardLink}
		{@const ref = cardLink}
		{@const view = refView(ref, entries)}
		<p class="flex items-baseline justify-between gap-3 text-body-sm">
			<span class="font-medium {view.resolved ? '' : 'text-destructive'}">{view.name}</span>
			{#if view.amount}
				<span class="shrink-0 text-caption text-text-muted tabular-nums">{view.amount}</span>
			{/if}
		</p>
		{#if view.group}
			<p class="text-caption text-text-muted">{view.group}</p>
		{/if}
		{#if !view.resolved}
			<p class="text-caption text-destructive">{m.editor_reference_unresolved()}</p>
		{/if}
		{@render actionBar([
			{
				label: m.editor_reference_change(),
				icon: Pencil,
				look: 'plain',
				run: () => changeLink(ref.word)
			},
			{
				label: m.editor_reference_remove_action(),
				icon: Unlink,
				look: 'danger',
				run: () => unlink(ref.word)
			}
		])}
	{:else if cardRepeat !== null}
		{@const word = cardRepeat}
		<!-- Nothing to decide here, but noticed as much as the cards that ask something. -->
		<p class="flex items-center gap-1.5 text-body-sm font-medium text-primary">
			<Info class="size-3.5 shrink-0" aria-hidden="true" />
			{word}
		</p>
		<p class="mt-0.5 text-caption">{m.editor_reference_repeat({ word })}</p>
	{:else if cardAnyTime !== null}
		{@const time = cardAnyTime}
		<!-- A time's card has no "Change": a wrong time is removed and marked again. -->
		<p class="flex items-center gap-1.5 text-body-sm font-medium text-time-foreground tabular-nums">
			<Timer class="size-3.5 shrink-0" aria-hidden="true" />
			{formatDuration(time)}
		</p>
		{#if cardTime}
			{@render actionBar([
				{
					label: m.editor_reference_remove_action(),
					icon: X,
					look: 'danger',
					run: () => unmarkTime(time.phrase)
				}
			])}
		{:else}
			{@render actionBar([
				{
					label: m.editor_reference_dismiss_action(),
					icon: X,
					look: 'plain',
					run: () => dismiss(time.phrase)
				},
				{
					label: m.editor_reference_accept_action(),
					icon: Check,
					look: 'positive',
					run: () => acceptTime(time)
				}
			])}
		{/if}
	{/if}
{/snippet}

{#if cardMeta !== null}
	{@render cardFrame(cardMeta.label, cardMeta.heading, cardMeta.border, cardBody)}
{/if}

<!--
	"Mark as time": the phrase, the unit, and the length the two make. Focus
	starts on the checked unit and goes back to the step when it closes,
	unless a press or Tab outside closed it and took focus elsewhere. It
	stays in place rather than in a portal and does not trap focus, so Tab
	walks on to the page after the step, as from the manual picker.
-->
<Popover.Root
	bind:open={
		() => timePhrase !== null,
		(open) => {
			if (!open) timePhrase = null;
		}
	}
>
	<Popover.Content
		bind:ref={timeBox}
		role="dialog"
		customAnchor={timeAnchor}
		side="bottom"
		align="start"
		sideOffset={PICKER_GAP}
		collisionPadding={PICKER_MARGIN}
		aria-label={timePhrase === null ? undefined : m.editor_time_card({ phrase: timePhrase })}
		onOpenAutoFocus={(event) => {
			event.preventDefault();
			timeBox?.querySelector<HTMLElement>('[data-state="checked"]')?.focus();
		}}
		trapFocus={false}
		strategy="fixed"
		onInteractOutside={() => (timeClosedOutside = true)}
		onFocusOutside={() => {
			timeClosedOutside = true;
			timePhrase = null;
		}}
		onCloseAutoFocus={(event) => {
			event.preventDefault();
			// Bits UI also reports a close when a reopen replaces the last
			// focus scope; the popover is open then, and focus stays in it.
			if (!timeClosedOutside && timePhrase === null) editor?.commands.focus();
		}}
		class="z-50 w-72 overflow-hidden rounded-md border border-time-foreground bg-surface-elevated p-3 shadow-card"
	>
		{#if timePhrase !== null}
			{@const preview = manualTime(timePhrase, timeUnit)}
			<p class="mb-2 text-caption text-text-muted">
				{m.editor_time_card({ phrase: timePhrase })}
			</p>
			<RadioGroup.Root
				value={timeUnit}
				onValueChange={(value) => (timeUnit = value as TimeUnit)}
				orientation="horizontal"
				aria-label={m.editor_time_unit()}
				class="flex rounded-pill bg-background p-1"
			>
				{#each TIME_UNITS as unit (unit.value)}
					<RadioGroup.Item
						value={unit.value}
						class="h-8 min-w-0 flex-1 truncate rounded-pill px-2 text-caption font-semibold text-text-muted transition hover:text-text focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary data-[state=checked]:bg-surface data-[state=checked]:text-text data-[state=checked]:shadow-card"
					>
						{unit.label()}
					</RadioGroup.Item>
				{/each}
			</RadioGroup.Root>
			<p
				class="mt-2.5 flex min-h-5 items-center gap-1.5 text-body-sm font-medium text-time-foreground tabular-nums"
			>
				{#if preview}
					<Timer class="size-3.5 shrink-0" aria-hidden="true" />
					{formatDuration(preview)}
				{/if}
			</p>
			{@render actionBar(
				[
					{
						label: m.editor_time_set(),
						icon: Check,
						look: 'positive',
						run: setTime,
						disabled: preview === null
					}
				],
				true
			)}
		{/if}
	</Popover.Content>
</Popover.Root>

<svelte:document onpointerdown={closePickerOutside} />

{#if picker}
	<!--
		The rows never take focus: in the `@` flow the caret stays in the step
		while the arrow keys walk the list, and in the manual flow the query
		field above holds it. That is why they answer to `mousedown` with the
		default prevented rather than to a click, and why they carry
		`tabindex="-1"`. Leaving them tabbable would put a row in the tab order
		that only a mouse can operate - both flows intercept the arrows and
		Enter, so the keyboard never reaches them anyway.
	-->
	<div
		bind:this={pickerBox}
		class="fixed z-50 flex w-72 max-w-[calc(100%-1rem)] flex-col overflow-hidden rounded-md border border-primary bg-surface-elevated shadow-card"
		style="left: {pickerAt.left}px; {pickerAt.top === null
			? `bottom: ${pickerAt.bottom}px`
			: `top: ${pickerAt.top}px`}; max-height: {pickerAt.maxHeight}px"
	>
		{#if picker.word !== null}
			<!--
				The manual flow's query field. The `@` flow's query is the text
				typed after the `@` and needs no field; this one has no such
				text, and without a field the list would stop at the rows that
				fit. It names the word the link will attach to as well, so a
				caret that grew out to a word the author did not mean is
				visible before anything is linked.
			-->
			<div class="shrink-0 border-b border-border px-3.5 py-2">
				<p class="mb-1 text-caption text-text-muted">
					{m.editor_reference_link_for({ word: picker.word })}
				</p>
				<input
					bind:this={queryField}
					type="text"
					role="combobox"
					autocomplete="off"
					aria-expanded="true"
					aria-controls="ref-picker-{step.id}"
					aria-activedescendant={picker.items.length > 0
						? `ref-picker-${step.id}-${picker.active}`
						: undefined}
					aria-label={m.editor_reference_link_for({ word: picker.word })}
					placeholder={m.editor_reference_search()}
					value={manualQuery}
					oninput={(event) => updateManual(event.currentTarget.value)}
					onkeydown={manualKeys}
					onblur={manualBlur}
					class="h-8 w-full rounded-md border border-border bg-surface px-2 text-body-sm outline-none focus:border-primary"
				/>
			</div>
		{/if}
		<div
			role="listbox"
			id="ref-picker-{step.id}"
			aria-label={m.editor_reference_picker()}
			class="m-1 grid max-h-64 min-h-0 gap-0.5 overflow-y-auto overscroll-contain rounded-[calc(var(--radius-md)-5px)]"
		>
			<!--
				Each row as the list under a step reads (`CELL`): the name with
				its group beside it - what tells two "Salz" apart - and the
				amount at the right end, dropping under the name when both do
				not fit. The active row is an inset, rounded fill inside the
				popup's own rounding, with the text left in its colors. The
				list is inset and rounded too, so a scrollbar that takes room
				(Windows, or macOS set to show them) stays inside the frame
				rather than running square over its border and corners. Both
				radii are the frame's less its border and the inset (1px + 4px),
				so the curves run parallel.
			-->
			{#each picker.items as item, at (item.ingredientId)}
				{@const part = matchedPart(item.name, picker.query)}
				<button
					type="button"
					role="option"
					tabindex={-1}
					id="ref-picker-{step.id}-{at}"
					aria-selected={at === picker.active}
					aria-label={[item.name, item.amount, item.group].filter(Boolean).join(' · ')}
					onmousedown={(event) => {
						event.preventDefault();
						choose(at);
					}}
					class="flex min-h-11 w-full items-center rounded-[calc(var(--radius-md)-5px)] px-2.5 py-1 text-left text-body-sm transition {at ===
					picker.active
						? 'bg-accent'
						: 'hover:bg-surface'}"
				>
					<span class="{CELL} w-full">
						<span class="flex min-w-0 flex-wrap items-baseline gap-x-1.5">
							<!-- The letters typed, bold and in the primary color within the name: why
							     this row matched, the same orange on every row. On the active row's
							     fill it stays under 4.5:1 (3.8 light, 4.1 dark) by the owner's
							     choice: two letters in a row the fill already marks, where a second,
							     darker orange read as another color. -->
							<span class="text-text {part ? '' : 'font-medium'}"
								>{#if part}{part.before}<b class="font-bold text-primary">{part.hit}</b
									>{part.after}{:else}{item.name}{/if}</span
							>
							{#if item.group}
								<span class="text-caption text-text-muted">{item.group}</span>
							{/if}
						</span>
						{#if item.amount}
							<span class="text-caption whitespace-nowrap text-text-muted tabular-nums"
								>{item.amount}</span
							>
						{/if}
					</span>
				</button>
			{:else}
				<p class="px-2.5 py-2 text-body-sm text-text-muted">{m.editor_reference_no_match()}</p>
			{/each}
		</div>
	</div>
{/if}
