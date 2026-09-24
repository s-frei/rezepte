<script lang="ts">
	import { tick, untrack } from 'svelte';
	import { Editor } from '@tiptap/core';
	import { Plugin, PluginKey } from '@tiptap/pm/state';
	import type { Node as ProseMirrorNode } from '@tiptap/pm/model';
	import { Decoration, DecorationSet, type EditorView } from '@tiptap/pm/view';
	import Suggestion, { exitSuggestion, type SuggestionProps } from '@tiptap/suggestion';
	import { m } from '$lib/paraglide/messages';
	import { presentReferences, type FormGroup, type FormRef, type FormStep } from '$lib/recipe/form';
	import { isWordChar } from '$lib/recipe/references';
	import { referenceExtension, stepExtensions, stepText, textToDoc } from '$lib/recipe/step-doc';
	import {
		addReference,
		firstWordMatch,
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
		/** The step being edited; its `text` and `references` are written in place. */
		step: FormStep;
		/** Position in the list, for the field's label. Changes when steps are reordered. */
		index: number;
		/** The recipe's ingredient groups; the matcher and the picker read both
		 * their names and the client ids a link is anchored to. */
		groups: FormGroup[];
		/** Every ingredient the picker may offer, for the whole recipe. */
		entries: PickerEntry[];
		/** Words of this step whose proposal the author has turned down. */
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

	const pending = $derived(pendingFor(step, groups, dismissed));

	/**
	 * The links whose word stands in the text right now. A link whose word was
	 * edited away is kept on the step, so undo can bring it back, but nothing
	 * is drawn for it until it does.
	 */
	const shown = $derived(presentReferences(step.references, step.text));

	/** Links and proposals in the order their words appear, which is how the list reads them. */
	function byPosition(refs: FormRef[]): FormRef[] {
		const at = (ref: FormRef) => firstWordMatch(step.text, ref.word)?.from ?? 0;
		return [...refs].sort((a, b) => at(a) - at(b));
	}

	/**
	 * The marked word the caret stands in, which is what the card at the word
	 * is about. Nothing while a picker is open: that popup is the one the
	 * author is working in, and two at the same caret would cover each other.
	 */
	const cardWord = $derived(
		picker !== null || caret === null
			? null
			: wordAtCaret(
					step.text,
					[...shown, ...pending].map((ref) => ref.word),
					caret
				)
	);
	const cardLink = $derived(shown.find((ref) => ref.word === cardWord) ?? null);
	const cardProposal = $derived(
		cardLink === null ? (pending.find((ref) => ref.word === cardWord) ?? null) : null
	);

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
			...pending.map((ref) => `ref-suggestion:${ref.word}`)
		])
	);

	/**
	 * Marks the referenced and the proposed words.
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
			for (const ref of pending) {
				if (!marked.some((entry) => entry.word === ref.word)) {
					marked.push({ word: ref.word, cls: 'ref-suggestion' });
				}
			}
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
		openManualFor(word, () => caretRect(view, from));
	}

	/** Opens the manual picker for `word`, at `rect`; the card's "Change" comes in here. */
	function openManualFor(word: string, rect: () => DOMRect | null) {
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
				onSelectionUpdate: ({ editor: changed }) => trackCaret(changed),
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
		links={byPosition(shown)}
		pending={byPosition(pending)}
		{entries}
		{editing}
		onlinkword={openManual}
		onunlink={unlink}
		onaccept={accept}
		ondismiss={dismiss}
	/>

	<!--
		The picker's open state, and what a click on the link button found. It
		lives here rather than as `aria-expanded` on the field, which is a
		textbox and does not support that attribute.
	-->
	<p role="status" class="sr-only">{notice || pickerStatus}</p>
</div>

{#if cardAt !== null && (cardLink ?? cardProposal)}
	{@const ref = (cardLink ?? cardProposal) as FormRef}
	{@const view = refView(ref, entries)}
	<!--
		The card at a marked word: what it points at, and what can be done
		about it. It never takes focus - its buttons answer to `mousedown` with
		the default prevented, so the caret stays in the word and typing goes on
		where it was. That is also why it is not the way the keyboard gets
		there: the list under the step holds the same actions in the tab order.
	-->
	<div
		role="group"
		aria-label={m.editor_reference_card({ word: ref.word })}
		class="fixed z-40 w-64 rounded-md border bg-surface-elevated p-3 shadow-card {cardLink
			? 'border-border'
			: 'border-dashed border-primary'}"
		style="left: {cardAt.left}px; {cardAt.top === null
			? `bottom: ${cardAt.bottom}px`
			: `top: ${cardAt.top}px`}"
	>
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
				<button
					type="button"
					tabindex={-1}
					onmousedown={(event) => event.preventDefault()}
					onclick={() => changeLink(ref.word)}
					class="rounded-pill px-3 py-1.5 text-caption font-medium text-text-muted transition hover:text-primary"
				>
					{m.editor_reference_change()}
				</button>
				<button
					type="button"
					tabindex={-1}
					onmousedown={(event) => event.preventDefault()}
					onclick={() => unlink(ref.word)}
					class="rounded-pill px-3 py-1.5 text-caption font-medium text-text-muted transition hover:text-destructive"
				>
					{m.editor_reference_remove_action()}
				</button>
			{:else}
				<button
					type="button"
					tabindex={-1}
					onmousedown={(event) => event.preventDefault()}
					onclick={() => dismiss(ref.word)}
					class="rounded-pill px-3 py-1.5 text-caption font-medium text-text-muted transition hover:text-destructive"
				>
					{m.editor_reference_dismiss_action()}
				</button>
				<button
					type="button"
					tabindex={-1}
					onmousedown={(event) => event.preventDefault()}
					onclick={() => accept(ref)}
					class="rounded-pill bg-primary px-3 py-1.5 text-caption font-medium text-primary-foreground transition hover:opacity-90"
				>
					{m.editor_reference_accept_action()}
				</button>
			{/if}
		</div>
	</div>
{/if}

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
