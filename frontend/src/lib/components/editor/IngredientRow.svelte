<script lang="ts" module>
	/**
	 * The one-line layout's columns: grip, name, amount, unit, note, ×. The
	 * name leads as on two lines, where it is line 1, so the focus order
	 * (and a phone keyboard's "Next") reads the same at every width. The
	 * group's column header (IngredientGroupEditor) lays itself out on the
	 * same string, so its names stay over the fields.
	 */
	export const wideColumns =
		'@xl:grid-cols-[1.25rem_minmax(0,1fr)_5rem_6.25rem_minmax(10rem,1.25fr)_1.75rem] @xl:gap-x-2';
</script>

<script lang="ts">
	import { tick } from 'svelte';
	import GripVertical from '@lucide/svelte/icons/grip-vertical';
	import MessageSquareText from '@lucide/svelte/icons/message-square-text';
	import TriangleAlert from '@lucide/svelte/icons/triangle-alert';
	import Unlink from '@lucide/svelte/icons/unlink';
	import X from '@lucide/svelte/icons/x';
	import { dragHandle } from 'svelte-dnd-action';
	import { pickUp } from '#lib/recipe/dnd.js';
	import { m } from '#lib/paraglide/messages.js';
	import { isBlank, type FormIngredient } from '#lib/recipe/form.js';
	import FieldError from '#lib/components/ui/FieldError.svelte';
	import UnitInput from './UnitInput.svelte';

	let {
		row = $bindable(),
		quantityError = null,
		nameError = null,
		missing = false,
		duplicate = false,
		onenter,
		onbackspace,
		onremove
	}: {
		/** Part of the form's `$state`, so the fields are mutated in place. */
		row: FormIngredient;
		quantityError?: string | null;
		nameError?: string | null;
		/** The recipe has no ingredient yet and this is where the first one goes. */
		missing?: boolean;
		/** The name occurs again in this group, so no step can link to the row. */
		duplicate?: boolean;
		/** Enter in the name or note field: add a row below and focus it. */
		onenter: () => void;
		/** Backspace in an otherwise empty row: remove it. */
		onbackspace: () => void;
		onremove: () => void;
	} = $props();

	const empty = $derived(
		isBlank(row.quantity) && isBlank(row.unit) && isBlank(row.name) && isBlank(row.note)
	);

	// On one line the note has its column whether filled or not, so the field
	// is always there. On two lines it would cost a line of its own, and most
	// rows have no note: the field waits behind its toggle until asked for, and
	// a row whose note has text always shows it. Open while focused whatever
	// it holds: emptying a note mid-typing must not take the field away, only
	// leaving it folds it (`foldNote`).
	let noteOpen = $state(false);
	const hasNote = $derived(row.note.trim() !== '');
	const showNote = $derived(noteOpen || hasNote);
	let noteInput = $state<HTMLInputElement>();
	/** A pointer is down somewhere on the page; see `foldNote`. */
	let pressed = false;

	/*
	 * Each field's box is a <fieldset> (role none) whose <legend> holds the
	 * <label>: the browser cuts the legend's gap into the top border, so the
	 * card or a dragged copy shows through with no background to match. The
	 * content moves up by the legend's lower 6px to keep the field 44px tall.
	 *
	 * Focused or in error the line doubles and tints with the label; the 1px
	 * of padding it takes keeps the value in place. A field in error stays
	 * destructive while focused, as `Input` does. No placeholders: an empty
	 * field shows only its floating label (see the style block). On one line
	 * the legend is `sr-only`, which drops it from the border and leaves the
	 * label to name the field.
	 */
	const box = (error: boolean) =>
		`field relative min-w-0 rounded-md border bg-surface-elevated transition motion-reduce:transition-none *:not-[legend]:-mt-1.5 @xl:*:not-[legend]:mt-0 ${error ? 'border-2 border-destructive text-destructive has-focus-visible:outline-2 has-focus-visible:outline-offset-2 has-focus-visible:outline-destructive' : 'group/field border-border p-px text-field-label focus-within:border-2 focus-within:border-primary focus-within:p-0 focus-within:text-primary'}`;
	const legend = (error: boolean) =>
		`ml-2.5 text-label font-medium tracking-[0.01em] whitespace-nowrap @xl:sr-only ${error ? 'h-3.5 leading-3.5' : 'h-3.25 leading-3.25 group-focus-within/field:h-3.5 group-focus-within/field:leading-3.5'}`;
	/**
	 * Values in the text color; a note's muted: a remark beside the ingredient,
	 * not part of it. A value longer than its field ends in an ellipsis while
	 * the field is not focused, rather than mid-glyph.
	 */
	const control = (tone = 'text-text') =>
		`h-10 w-full min-w-0 truncate bg-transparent px-3.25 text-input ${tone} outline-none @xl:h-9 @xl:px-2.75`;

	const describedBy = $derived(
		[
			nameError && `ingredient-name-error-${row.id}`,
			duplicate && `ingredient-name-duplicate-${row.id}`
		]
			.filter(Boolean)
			.join(' ') || undefined
	);

	/** An edit is the member's answer to an import's "please check". */
	function clearHint() {
		row.source = undefined;
	}

	/**
	 * Enter - a phone keyboard's "next" arrow - walks the row: name, amount,
	 * unit. From the unit or the note it starts the next ingredient, so a
	 * list is typed without leaving the keyboard. `next` is the field it
	 * moves to; none means the row is done.
	 */
	function handleKeydown(event: KeyboardEvent, next?: 'quantity' | 'unit') {
		if (event.key === 'Backspace' && empty) {
			event.preventDefault();
			onbackspace();
			return;
		}
		if (event.key !== 'Enter' || event.defaultPrevented || event.isComposing) {
			return;
		}
		event.preventDefault();
		if (next) {
			document.getElementById(`ingredient-${next}-${row.id}`)?.focus();
		} else {
			onenter();
		}
	}

	async function openNote() {
		noteOpen = true;
		await tick();
		noteInput?.focus();
	}

	/**
	 * An emptied note folds back behind its toggle once left. On a phone the
	 * fold moves everything below the row up, and a tap that took the focus
	 * blurs the field before its click: folding then would slide something
	 * else under the finger and the click would land on nothing. So the fold
	 * waits until the press is over and its click has been dispatched (a
	 * touch tap fires its mouse events and click in one task, a mouse its
	 * mouseup and click).
	 */
	function foldNote() {
		const fold = () =>
			setTimeout(() => {
				if (row.note.trim() === '' && document.activeElement !== noteInput) {
					noteOpen = false;
				}
			});
		if (pressed) {
			// Whichever ends the press; the other is dropped with it.
			const controller = new AbortController();
			const end = () => {
				controller.abort();
				fold();
			};
			window.addEventListener('pointerup', end, { signal: controller.signal });
			window.addEventListener('pointercancel', end, { signal: controller.signal });
		} else {
			fold();
		}
	}
</script>

<svelte:window
	onpointerdowncapture={() => (pressed = true)}
	onpointerupcapture={() => (pressed = false)}
	onpointercancelcapture={() => (pressed = false)}
/>

<!--
    The row is its own size container. svelte-dnd-action drags a copy of the
    list item appended to <body>, outside every container of the editor; a
    query against one of those would leave the copy on its two-line layout,
    so a one-line row turned into another-looking one under the pointer.
-->
<div class="@container">
	<!--
        On two lines the rows of a group are not tiles: a hairline above each
        sets them apart, 8px inside it on either side - but not above the
        first, nor right under the drop slot, which would draw the line onto
        the slot's edge. On one line they keep the plain 6px rhythm (3px each)
        without hairlines. A picked-up row (`pickUp`) stands on a card drawn
        behind it, 8px out to the sides and 8px above and below its content,
        and hides its hairline; the dragged copy is cloned with the card.
    -->
	<div
		class="relative border-t border-border py-2 before:pointer-events-none before:absolute before:inset-0 before:-z-10 before:rounded-xl before:bg-surface before:opacity-0 before:outline-2 before:outline-primary before:transition-[inset,opacity,box-shadow] before:duration-160 before:ease-out in-data-lift:isolate in-data-[lift=up]:border-t-transparent in-data-[lift=up]:before:-inset-x-2 in-data-[lift=up]:before:opacity-100 in-data-[lift=up]:before:shadow-lift in-[[data-slot]+li]:border-t-0 in-[li:first-child]:border-t-0 motion-reduce:before:transition-none @xl:border-t-0 @xl:py-0.75 @xl:in-data-[lift=up]:before:-inset-y-1.25"
	>
		<!--
            One grid, two layouts. Narrow: grip, fields and a 44px action
            column; name and × on line 1, amount, unit and the note toggle on
            line 2, then the amount error and the note field full width. Each
            field's label sits in its top border, so cells align to the bottom
            of their line. Wide: everything on one line, no labels (the group's
            column header names the fields). The switch follows the row's own
            width (container query), not the window's. DOM order is name,
            quantity, unit, note for both, so tab order reads like the row.
        -->
		<div
			class="grid grid-cols-[1.25rem_minmax(0,1fr)_2.75rem] items-end gap-1.5 @xl:items-center {wideColumns}"
		>
			<!-- A div with the button role, not a <button>: svelte-dnd-action
                 ignores Space and Enter on anything with a `disabled` property.
                 The first span widens the hit area to 44px. -->
			<div
				use:dragHandle
				onpointerdown={pickUp}
				role="button"
				tabindex="0"
				aria-label={m.editor_ingredient_reorder()}
				class="relative col-1 row-1 flex h-11 cursor-grab touch-none items-center justify-center focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary @xl:col-auto @xl:row-auto @xl:h-10"
			>
				<span
					aria-hidden="true"
					class="absolute inset-y-0 -right-1.5 -left-4.5 @xl:-right-2 @xl:-left-4"
				></span>
				<span
					class="flex h-8 w-5 items-center justify-center rounded-pill bg-accent text-accent-foreground/70 @xl:h-7"
				>
					<GripVertical class="size-4" aria-hidden="true" />
				</span>
			</div>

			<fieldset
				role="none"
				data-float={row.name !== '' || !!nameError || missing || undefined}
				class="{box(!!nameError || missing)} col-2 row-1 @xl:col-auto @xl:row-auto"
			>
				<legend class={legend(!!nameError || missing)}>
					<label for="ingredient-name-{row.id}">{m.editor_ingredient_name()}</label>
				</legend>
				<input
					id="ingredient-name-{row.id}"
					bind:value={row.name}
					oninput={clearHint}
					onkeydown={(event) => handleKeydown(event, 'quantity')}
					type="text"
					autocomplete="off"
					enterkeyhint="next"
					aria-invalid={nameError || missing ? 'true' : undefined}
					data-error-key="name:{row.id} ingredientGroups"
					aria-describedby={describedBy}
					data-name-of={row.id}
					class={control()}
				/>
			</fieldset>
			<!-- Amount fits a typical value and its label; the unit takes the rest. -->
			<div class="col-2 row-2 grid grid-cols-[4.375rem_minmax(0,1fr)] gap-1.5 @xl:contents">
				<fieldset
					role="none"
					data-float={row.quantity !== '' || !!quantityError || undefined}
					class="{box(!!quantityError)} @xl:col-auto @xl:row-auto"
				>
					<legend class={legend(!!quantityError)}>
						<label for="ingredient-quantity-{row.id}">{m.editor_ingredient_quantity()}</label>
					</legend>
					<input
						id="ingredient-quantity-{row.id}"
						bind:value={row.quantity}
						oninput={clearHint}
						onkeydown={(event) => handleKeydown(event, 'unit')}
						type="text"
						inputmode="decimal"
						enterkeyhint="next"
						autocomplete="off"
						aria-invalid={quantityError ? 'true' : undefined}
						data-error-key="quantity:{row.id}"
						aria-describedby={quantityError ? `ingredient-quantity-error-${row.id}` : undefined}
						class="{control()} font-semibold"
					/>
				</fieldset>
				<fieldset role="none" data-float={row.unit !== '' || undefined} class={box(false)}>
					<legend class={legend(false)}>
						<label for="ingredient-unit-{row.id}">{m.editor_ingredient_unit()}</label>
					</legend>
					<UnitInput
						id="ingredient-unit-{row.id}"
						bind:value={row.unit}
						oninput={clearHint}
						onkeydown={(event) => handleKeydown(event)}
						class={control()}
						labelled
						enterkeyhint="next"
					/>
				</fieldset>
			</div>
			<!-- The note toggle, two lines only: dashed without a note, a hairline
                 and a dot with one. A tap opens the field and focuses it. -->
			<button
				type="button"
				onclick={openNote}
				aria-label={hasNote ? m.editor_ingredient_note() : m.editor_note_add()}
				aria-expanded={showNote}
				aria-controls="ingredient-note-{row.id}"
				class="relative col-3 row-2 flex size-11 items-center justify-center rounded-md border transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary @xl:hidden {hasNote
					? 'border-border text-accent-foreground'
					: 'border-dashed border-handle text-text-muted hover:text-text'}"
			>
				<MessageSquareText class="size-4.5" aria-hidden="true" />
				{#if hasNote}
					<span
						aria-hidden="true"
						class="absolute top-1.5 right-1.5 size-1.5 rounded-pill bg-primary"
					></span>
				{/if}
			</button>
			<!-- Under line 2 on phones, only while shown; on one line always there. -->
			<fieldset
				role="none"
				data-float={row.note !== '' || undefined}
				class="{box(false)} col-[2/4] @xl:col-auto @xl:row-auto {showNote ? '' : '@max-xl:hidden'}"
			>
				<legend class={legend(false)}>
					<label for="ingredient-note-{row.id}">{m.editor_ingredient_note()}</label>
				</legend>
				<input
					id="ingredient-note-{row.id}"
					bind:this={noteInput}
					bind:value={row.note}
					oninput={clearHint}
					onkeydown={(event) => handleKeydown(event)}
					onfocus={() => (noteOpen = true)}
					onblur={foldNote}
					type="text"
					autocomplete="off"
					enterkeyhint="next"
					class={control('text-text-muted')}
				/>
			</fieldset>

			<button
				id="ingredient-remove-{row.id}"
				type="button"
				onclick={onremove}
				aria-label={m.editor_ingredient_remove()}
				class="relative col-3 row-1 flex size-11 items-center justify-center text-text-muted transition hover:text-destructive focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary @xl:col-auto @xl:row-auto @xl:h-10 @xl:w-auto @xl:before:absolute @xl:before:inset-y-0 @xl:before:-right-2 @xl:before:-left-2"
			>
				<X class="size-4" aria-hidden="true" />
			</button>

			<!-- Placed explicitly (never in the line's flow): under line 2 on phones, under the line from the amount's column on desktop. -->
			{#if quantityError}
				<FieldError
					id="ingredient-quantity-error-{row.id}"
					message={quantityError}
					class="col-[2/-1] row-3 @xl:col-[3/-1] @xl:row-2"
				/>
			{/if}
		</div>

		{#if nameError}
			<FieldError
				id="ingredient-name-error-{row.id}"
				message={nameError}
				class="mt-1 pl-6.5 @xl:pl-7"
			/>
		{/if}
		{#if duplicate}
			<p
				id="ingredient-name-duplicate-{row.id}"
				class="mt-1 flex items-center gap-1.5 pl-6.5 text-micro text-text-muted @xl:pl-7"
			>
				<Unlink class="size-3.5 shrink-0" aria-hidden="true" />
				{m.editor_ingredient_duplicate()}
			</p>
		{/if}
		{#if row.source !== undefined}
			<p class="mt-1 flex items-center gap-1.5 pl-6.5 text-caption text-primary @xl:pl-7">
				<TriangleAlert class="size-3.5 shrink-0" aria-hidden="true" />
				<span class="min-w-0 break-words">
					<span class="font-medium">{m.editor_ingredient_check()}</span> · {m.editor_ingredient_original(
						{ line: row.source }
					)}
				</span>
				<button
					type="button"
					onclick={clearHint}
					aria-label={m.editor_ingredient_check_dismiss()}
					title={m.editor_ingredient_check_dismiss()}
					class="-my-1.5 ml-auto flex size-8 shrink-0 items-center justify-center rounded-pill text-text-muted transition hover:text-text focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
				>
					<X class="size-3.5" aria-hidden="true" />
				</button>
			</p>
		{/if}
	</div>
</div>

<style>
	/*
	 * The floating label. Resting, it sits where the value goes and the legend
	 * is a `minmax(0, 0fr)` column, so the border runs unbroken; floated (focus,
	 * a value, an error, autofill) the column is `1fr`, exactly label wide. fr
	 * interpolates, so the notch opens from the corner, in the first half of
	 * the label's 150ms and, going back, after the word has left the line. Only
	 * the label moves, so the field never changes size. 1.34375rem is the
	 * distance from the line's middle to the value's middle. The state is in
	 * the first style a field gets, so a loaded value draws floated, no motion.
	 */
	.field > legend {
		display: grid;
		grid-template-columns: minmax(0, 0fr);
	}

	.field > legend > label {
		min-width: 0;
		padding-inline: 0.1875rem;
		transform-origin: 0.1875rem 50%;
		translate: 0 1.34375rem;
		scale: calc(16 / 11);
		color: var(--color-text-muted);
		cursor: text;
	}

	/* Mirrors `text-input` in app.css: 16px, 15px (`--text-body`) from md up. A scale needs a number, so it is not derived. */
	@media (width >= 48rem) {
		.field > legend > label {
			scale: calc(15 / 11);
		}
	}

	.field:is(:focus-within, [data-float]) > legend {
		grid-template-columns: minmax(0, 1fr);
	}

	.field:is(:focus-within, [data-float]) > legend > label {
		translate: none;
		scale: none;
		color: inherit;
		cursor: default;
	}

	/* On its own: one unknown pseudo-class would drop a whole selector list. */
	.field:has(:global(:-webkit-autofill)) > legend {
		grid-template-columns: minmax(0, 1fr);
	}

	.field:has(:global(:-webkit-autofill)) > legend > label {
		translate: none;
		scale: none;
		color: inherit;
	}

	/* Firefox's name for it, on its own for the same reason. */
	.field:has(:global(:autofill)) > legend {
		grid-template-columns: minmax(0, 1fr);
	}

	.field:has(:global(:autofill)) > legend > label {
		translate: none;
		scale: none;
		color: inherit;
	}

	/* Each state names how the notch gets to it; reduced motion switches. */
	@media (prefers-reduced-motion: no-preference) {
		.field > legend {
			transition: grid-template-columns 75ms ease-in 75ms;
		}

		.field:is(:focus-within, [data-float]) > legend {
			transition: grid-template-columns 75ms ease-out;
		}

		.field > legend > label {
			transition:
				translate 150ms ease-out,
				scale 150ms ease-out,
				color 150ms ease-out;
		}
	}
</style>
