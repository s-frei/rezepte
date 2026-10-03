<script lang="ts">
	import { tick, untrack, type Snippet } from 'svelte';
	import { Editor } from '@tiptap/core';
	import { Plugin, PluginKey } from '@tiptap/pm/state';
	import type { Node as ProseMirrorNode } from '@tiptap/pm/model';
	import { Decoration, DecorationSet, type EditorView } from '@tiptap/pm/view';
	import Suggestion, { exitSuggestion, type SuggestionProps } from '@tiptap/suggestion';
	import { Popover, RadioGroup } from 'bits-ui';
	import Timer from '@lucide/svelte/icons/timer';
	import type { StepTime } from '$lib/api/recipes';
	import { m } from '$lib/paraglide/messages';
	import { presentReferences, type FormGroup, type FormRef, type FormStep } from '$lib/recipe/form';
	import { firstWordMatch, isWordChar } from '$lib/recipe/references';
	import { referenceExtension, stepExtensions, stepText, textToDoc } from '$lib/recipe/step-doc';
	import {
		addReference,
		fitsWordLimit,
		isResolved,
		matchEntries,
		pendingFor,
		refView,
		removeReference,
		wordAt,
		wordAtCaret,
		type PickerEntry
	} from '$lib/recipe/step-references';
	import {
		addTime,
		caretTimePhrases,
		isMarked,
		overlapsAny,
		pendingTimes,
		presentTimes,
		removeTime
	} from '$lib/recipe/step-times';
	import { MAX_PHRASE_LENGTH, formatDuration, manualTime, type TimeUnit } from '$lib/recipe/times';
	import StepLinks from './StepLinks.svelte';

	let {
		step,
		index,
		groups,
		entries,
		dismissed,
		error,
		ondismiss
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
		/** Words and time phrases of this step whose proposal the author has turned down. */
		dismissed: string[];
		/** The server's message for this step, if it rejected it. */
		error?: string;
		/** Turns a proposal down, or keeps it down after a link is removed. */
		ondismiss: (word: string) => void;
	} = $props();

	/** At most this many rows in the picker; the query narrows it further. */
	const PICKER_ROWS = 8;

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
	const CARD_HEIGHT = 132;
	/** The looks of a card's buttons. */
	const MUTED_PRIMARY = 'text-text-muted hover:text-primary';
	const MUTED_DESTRUCTIVE = 'text-text-muted hover:text-destructive';
	const SOLID = 'bg-primary text-primary-foreground hover:opacity-90';

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
	let pickerAt = $state<{ left: number; top: number | null; bottom: number | null }>({
		left: 0,
		top: 0,
		bottom: null
	});
	/** What the manual picker's query field holds, and the field itself. */
	let manualQuery = $state('');
	let queryField = $state<HTMLInputElement | null>(null);
	/** Said in the live region when there was no word to link. */
	let notice = $state('');
	/** Whether focus is anywhere in this step; the link button is offered only then. */
	let editing = $state(false);
	/** The caret as an offset into the plain text, while the field has focus and nothing is selected. */
	let caret = $state<number | null>(null);
	let cardAt = $state<{ left: number; top: number | null; bottom: number | null } | null>(null);

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

	const pending = $derived(pendingFor(step, groups, dismissed));
	/** Stored times whose phrase is in the text, the same rule `shown` follows for links. */
	const shownTimes = $derived(presentTimes(step.times, step.text));
	const pendingTimesList = $derived(pendingTimes(step, dismissed));

	/**
	 * The links whose word stands in the text right now. A link whose word was
	 * edited away is kept on the step, so undo can bring it back, but nothing
	 * is drawn for it until it does.
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
	const cardWord = $derived(
		picker !== null || timePhrase !== null || caret === null
			? null
			: wordAtCaret(
					step.text,
					[
						...[...shown, ...pending].map((ref) => ref.word),
						...caretTimePhrases(shownTimes, pendingTimesList)
					],
					caret
				)
	);
	const cardLink = $derived(shown.find((ref) => ref.word === cardWord) ?? null);
	const cardProposal = $derived(
		cardLink === null ? (pending.find((ref) => ref.word === cardWord) ?? null) : null
	);
	/** The time or time proposal at the caret, and whether it is stored. */
	const cardAnyTime = $derived(
		[...shownTimes, ...pendingTimesList].find((time) => time.phrase === cardWord) ?? null
	);
	const cardTime = $derived(cardAnyTime !== null && shownTimes.includes(cardAnyTime));

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
			...pending.map((ref) => `ref-suggestion:${ref.word}`),
			...shownTimes.map((time) => `time-link:${time.phrase}`),
			...pendingTimesList.map((time) => `time-proposal:${time.phrase}`)
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
			for (const ref of pending) add(ref.word, 'ref-suggestion');
			for (const time of shownTimes) add(time.phrase, 'time-link');
			for (const time of pendingTimesList) add(time.phrase, 'time-proposal');
			return marked;
		});
		const seen: string[] = [];
		const decorations: Decoration[] = [];
		doc.descendants((node, pos) => {
			const text = node.text;
			if (!node.isText || text === undefined) return;
			for (const { word, cls } of words) {
				if (seen.includes(word)) continue;
				const hit = firstWordMatch(text, word);
				if (hit === null) continue;
				seen.push(word);
				decorations.push(
					Decoration.inline(pos + hit.from, pos + hit.to, { class: cls, 'data-ref-word': word })
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
		const below = window.innerHeight - at.bottom;
		const above = at.top;
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
			word: null
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

	/** The caret's rectangle, the same shape the `@` flow's `clientRect` hands over. */
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
			items: matchEntries(entries, '').slice(0, PICKER_ROWS),
			active: 0,
			command: (entry) => linkWord(word, entry),
			word
		};
		openAt(rect);
		void tick().then(() => queryField?.focus());
	}

	/** Narrows the manual list, leaving the popup where it sits. */
	function updateManual(query: string) {
		manualQuery = query;
		const open = picker;
		if (open === null) return;
		open.items = matchEntries(entries, query).slice(0, PICKER_ROWS);
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
			new Plugin({
				key: new PluginKey('ingredientDecorations'),
				props: { decorations: (state) => decorate(state.doc) }
			}),
			Suggestion<PickerEntry, PickerEntry>({
				editor: instance,
				pluginKey: PICKER_KEY,
				char: '@',
				items: ({ query }) => untrack(() => matchEntries(entries, query)).slice(0, PICKER_ROWS),
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
					chosen.chain().focus().insertContentAt(range, { type: 'text', text: inserted }).run();
					const text = stepText(chosen);
					step.text = text;
					if (firstWordMatch(text, props.name) === null) return;
					step.references = addReference(step.references, {
						word: props.name,
						ingredientId: props.ingredientId,
						groupName: props.group,
						ingredientName: props.name
					});
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
				content: textToDoc(step.text),
				editorProps: { attributes: { id: `step-${step.id}`, class: EDITABLE } },
				// Only the text: a link whose word this edit removed stays on
				// the step, out of sight, so undoing the edit brings it back
				// (see `presentReferences`).
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
		const left = Math.max(
			PICKER_MARGIN,
			Math.min(rect.left, window.innerWidth - PICKER_WIDTH - PICKER_MARGIN)
		);
		pickerAt =
			anchor?.side === 'above'
				? { left, top: null, bottom: window.innerHeight - rect.top + PICKER_GAP }
				: // Floored, for the moment a caret sits under the sticky bar and
					// the browser has not scrolled it into view yet: a popup with a
					// negative top would be a list nobody can see.
					{ left, top: Math.max(PICKER_MARGIN, rect.bottom + PICKER_GAP), bottom: null };
	}

	// Follows the caret when the page moves under it, and only then: the list
	// changing is deliberately not a reason to re-place.
	$effect(() => {
		if (anchor === null) return;
		window.addEventListener('scroll', place, true);
		window.addEventListener('resize', place);
		return () => {
			window.removeEventListener('scroll', place, true);
			window.removeEventListener('resize', place);
		};
	});

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
	 * Puts the card under the word, or over it where the room below is too
	 * small - which on a phone is the room above the on-screen keyboard, so the
	 * visual viewport is measured rather than the window.
	 */
	function placeCard() {
		const element = cardWord === null ? null : wordElement(cardWord);
		if (element === null) {
			cardAt = null;
			return;
		}
		const rect = element.getBoundingClientRect();
		const viewport = window.visualViewport;
		const visibleBottom = viewport ? viewport.offsetTop + viewport.height : window.innerHeight;
		const left = Math.max(
			PICKER_MARGIN,
			Math.min(rect.left, window.innerWidth - CARD_WIDTH - PICKER_MARGIN)
		);
		cardAt =
			visibleBottom - rect.bottom >= CARD_HEIGHT + PICKER_GAP + PICKER_MARGIN
				? { left, top: rect.bottom + PICKER_GAP, bottom: null }
				: { left, top: null, bottom: window.innerHeight - rect.top + PICKER_GAP };
	}

	// Places the card once the decoration it points at has been drawn.
	$effect(() => {
		void decoratedWords;
		if (cardWord === null) {
			cardAt = null;
			return;
		}
		void tick().then(() => untrack(placeCard));
	});

	$effect(() => {
		if (cardWord === null) return;
		const viewport = window.visualViewport;
		window.addEventListener('scroll', placeCard, true);
		window.addEventListener('resize', placeCard);
		viewport?.addEventListener('resize', placeCard);
		return () => {
			window.removeEventListener('scroll', placeCard, true);
			window.removeEventListener('resize', placeCard);
			viewport?.removeEventListener('resize', placeCard);
		};
	});

	/** "Change" on the card: the manual picker, for the word the card is about. */
	function changeLink(word: string) {
		openManualFor(word, () => wordElement(word)?.getBoundingClientRect() ?? null);
	}

	function accept(ref: FormRef) {
		step.references = addReference(step.references, ref);
	}

	function dismiss(word: string) {
		ondismiss(word);
	}

	/**
	 * Removes a link and turns the matcher's proposal for that word down with
	 * it - otherwise the word the author just unlinked would come straight back
	 * as a suggestion.
	 */
	function unlink(word: string) {
		step.references = removeReference(step.references, word);
		ondismiss(word);
	}

	function acceptTime(time: StepTime) {
		step.times = addTime(step.times, time, step.text);
	}

	/** Removes a time and turns its proposal down, for the same reason `unlink` does. */
	function unmarkTime(phrase: string) {
		step.times = removeTime(step.times, phrase);
		ondismiss(phrase);
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
		pending={byPosition(pending, (ref) => ref.word)}
		times={byPosition(shownTimes, (time) => time.phrase)}
		pendingTimes={byPosition(pendingTimesList, (time) => time.phrase)}
		{entries}
		{editing}
		onlinkword={openManual}
		onmarktime={openTimeMark}
		onunlink={unlink}
		onaccept={accept}
		ondismiss={dismiss}
		onaccepttime={acceptTime}
		onremovetime={unmarkTime}
	/>

	<!--
		Why "Link word" or "Mark as time" did nothing, for everyone who can see
		it. The status line below says the same to a screen reader, so this
		copy is hidden from it rather than read twice.
	-->
	{#if editing && notice}
		<p aria-hidden="true" class="mt-1 text-caption text-text-muted">{notice}</p>
	{/if}

	<!--
		The picker's open state, and what a click on the link button found. It
		lives here rather than as `aria-expanded` on the field, which is a
		textbox and does not support that attribute.
	-->
	<p role="status" class="sr-only">{notice || pickerStatus}</p>
</div>

{#snippet cardButton(label: string, onclick: () => void, look: string)}
	<button
		type="button"
		tabindex={-1}
		onmousedown={(event) => event.preventDefault()}
		{onclick}
		class="rounded-pill px-3 py-1.5 text-caption font-medium transition {look}"
	>
		{label}
	</button>
{/snippet}

<!--
	A card at a marked word or time: what it stands for, and what can be done
	about it. It never takes focus - its buttons answer to `mousedown` with
	the default prevented, so the caret stays in the word and typing goes on
	where it was. That is also why it is not the way the keyboard gets there:
	the list under the step holds the same actions in the tab order.
-->
{#snippet cardFrame(
	at: { left: number; top: number | null; bottom: number | null },
	label: string,
	border: string,
	body: Snippet
)}
	<div
		role="group"
		aria-label={label}
		class="fixed z-40 w-64 rounded-md border bg-surface-elevated p-3 shadow-card {border}"
		style="left: {at.left}px; {at.top === null ? `bottom: ${at.bottom}px` : `top: ${at.top}px`}"
	>
		{@render body()}
	</div>
{/snippet}

{#if cardAt !== null && (cardLink ?? cardProposal)}
	{@const ref = (cardLink ?? cardProposal) as FormRef}
	{@const view = refView(ref, entries)}
	{#snippet refCard()}
		<p class="flex items-baseline justify-between gap-3 text-body-sm">
			<span
				class="font-medium {!view.resolved
					? 'text-destructive'
					: cardProposal
						? 'text-primary'
						: ''}">{view.name}</span
			>
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
		<div class="mt-2.5 flex justify-end gap-1">
			{#if cardLink}
				{@render cardButton(m.editor_reference_change(), () => changeLink(ref.word), MUTED_PRIMARY)}
				{@render cardButton(
					m.editor_reference_remove_action(),
					() => unlink(ref.word),
					MUTED_DESTRUCTIVE
				)}
			{:else}
				{@render cardButton(
					m.editor_reference_dismiss_action(),
					() => dismiss(ref.word),
					MUTED_DESTRUCTIVE
				)}
				{@render cardButton(m.editor_reference_accept_action(), () => accept(ref), SOLID)}
			{/if}
		</div>
	{/snippet}
	{@render cardFrame(
		cardAt,
		m.editor_reference_card({ word: ref.word }),
		cardLink ? 'border-border' : 'border-dashed border-primary',
		refCard
	)}
{:else if cardAt !== null && cardAnyTime !== null}
	{@const time = cardAnyTime}
	<!-- A time's card has no "Change": a wrong time is removed and marked again. -->
	{#snippet timeCard()}
		<p class="flex items-center gap-1.5 text-body-sm font-medium text-time-foreground tabular-nums">
			<Timer class="size-3.5 shrink-0" aria-hidden="true" />
			{formatDuration(time)}
		</p>
		<div class="mt-2.5 flex justify-end gap-1">
			{#if cardTime}
				{@render cardButton(
					m.editor_reference_remove_action(),
					() => unmarkTime(time.phrase),
					MUTED_DESTRUCTIVE
				)}
			{:else}
				{@render cardButton(
					m.editor_reference_dismiss_action(),
					() => dismiss(time.phrase),
					MUTED_DESTRUCTIVE
				)}
				{@render cardButton(m.editor_reference_accept_action(), () => acceptTime(time), SOLID)}
			{/if}
		</div>
	{/snippet}
	{@render cardFrame(
		cardAt,
		m.editor_time_card({ phrase: time.phrase }),
		cardTime ? 'border-border' : 'border-dashed border-time-foreground',
		timeCard
	)}
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
		class="z-50 w-72 rounded-md border border-border bg-surface-elevated p-3 shadow-card"
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
			<div class="mt-2.5 flex items-center justify-between gap-3">
				<span
					class="flex items-center gap-1.5 text-body-sm font-medium text-time-foreground tabular-nums"
				>
					{#if preview}
						<Timer class="size-3.5 shrink-0" aria-hidden="true" />
						{formatDuration(preview)}
					{/if}
				</span>
				<button
					type="button"
					disabled={preview === null}
					onclick={setTime}
					class="rounded-pill bg-primary px-3 py-1.5 text-caption font-medium text-primary-foreground transition hover:opacity-90 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary disabled:opacity-50"
				>
					{m.editor_time_set()}
				</button>
			</div>
		{/if}
	</Popover.Content>
</Popover.Root>

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
		class="fixed z-50 w-72 rounded-md border border-border bg-surface-elevated shadow-card"
		style="left: {pickerAt.left}px; {pickerAt.top === null
			? `bottom: ${pickerAt.bottom}px`
			: `top: ${pickerAt.top}px`}"
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
			<div class="border-b border-border p-2">
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
			class="max-h-64 overflow-y-auto py-1"
		>
			{#each picker.items as item, at (item.ingredientId)}
				<button
					type="button"
					role="option"
					tabindex={-1}
					id="ref-picker-{step.id}-{at}"
					aria-selected={at === picker.active}
					onmousedown={(event) => {
						event.preventDefault();
						choose(at);
					}}
					class="flex w-full items-baseline gap-1.5 px-3 py-1.5 text-left text-body-sm {at ===
					picker.active
						? 'bg-accent text-accent-foreground'
						: ''}"
				>
					<span class="font-medium">{item.name}</span>
					{#if item.amount}<span class="text-caption text-text-muted">· {item.amount}</span>{/if}
					{#if item.group}<span class="text-caption text-text-muted">· {item.group}</span>{/if}
				</button>
			{:else}
				<p class="px-3 py-1.5 text-body-sm text-text-muted">{m.editor_reference_no_match()}</p>
			{/each}
		</div>
	</div>
{/if}
