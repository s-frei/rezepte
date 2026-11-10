<script lang="ts">
	import { flushSync, tick } from 'svelte';
	import { flip } from 'svelte/animate';
	import { SvelteSet } from 'svelte/reactivity';
	import { prefersReducedMotion } from 'svelte/motion';
	import Ellipsis from '@lucide/svelte/icons/ellipsis';
	import Pencil from '@lucide/svelte/icons/pencil';
	import Plus from '@lucide/svelte/icons/plus';
	import Trash2 from '@lucide/svelte/icons/trash-2';
	import { DropdownMenu } from 'bits-ui';
	import { dragHandleZone, type DndEvent } from 'svelte-dnd-action';
	import FieldError from '#lib/components/ui/FieldError.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { applyDrag, isShadow, liftDragged, slotHeight } from '#lib/recipe/dnd.js';
	import {
		newGroup,
		newIngredient,
		type FieldErrors,
		type FormGroup,
		type FormIngredient
	} from '#lib/recipe/form.js';
	import { findDuplicates } from '#lib/recipe/step-references.js';
	import AddButton from './AddButton.svelte';
	import IngredientRow, { wideColumns } from './IngredientRow.svelte';
	import { collapse, fresh, reveal } from './motion';

	let {
		groups = $bindable([]),
		errors = {}
	}: {
		groups?: FormGroup[];
		errors?: FieldErrors;
	} = $props();

	const FLIP_DURATION = $derived(prefersReducedMotion.current ? 0 : 150);

	/** Rows an import asked about that nobody has answered yet. */
	const toCheck = $derived(
		groups.flatMap((g) => g.ingredients).filter((r) => r.source !== undefined).length
	);

	/**
	 * The name field being typed in, with the name it had when it took focus.
	 * A clash is shown only when it holds both with that name and with the
	 * one typed so far: a second group on its way to "Klopse in Soße" passes
	 * through "Klopse" without an error flashing at it, while typing a clash
	 * away clears it at once. Leaving the field settles the name, and so does a
	 * save: its errors (`group:<id>`) show even with the caret still inside.
	 */
	let editing = $state<{ id: string; name: string } | null>(null);

	/** What clashes right now, the field being typed in included. */
	const now = $derived(findDuplicates(groups));

	const duplicates = $derived.by(() => {
		const held = editing;
		if (held === null) return now;
		const name = (item: { id: string; name: string }) =>
			item.id === held.id ? held.name : item.name;
		const before = findDuplicates(
			groups.map((group) => ({
				...group,
				name: name(group),
				ingredients: group.ingredients.map((row) => ({ ...row, name: name(row) }))
			}))
		);
		return {
			groups: new Map([...now.groups].filter(([id]) => before.groups.has(id))),
			rows: new Set([...now.rows].filter((id) => before.rows.has(id)))
		};
	});

	/** Name fields carry `data-name-of` with the id of their group or row. */
	function startEditing(event: FocusEvent) {
		const field = event.target;
		if (field instanceof HTMLInputElement && field.dataset.nameOf) {
			editing = { id: field.dataset.nameOf, name: field.value };
		}
	}

	/** Focuses a fresh row's first field, the name, at every width. */
	async function focusRow(id: string) {
		await tick();
		document.getElementById(`ingredient-name-${id}`)?.focus();
	}

	async function addRow(group: FormGroup, index = group.ingredients.length) {
		const row = newIngredient();
		fresh.add(row.id);
		group.ingredients.splice(index, 0, row);
		await focusRow(row.id);
		fresh.delete(row.id);
	}

	/** What the live region says. */
	let said = $state('');
	let toggle = false;
	/** A screen reader stays silent when the text repeats; a trailing nbsp every other time makes it new. */
	function announce(text: string) {
		toggle = !toggle;
		said = toggle ? text : `${text}\u00A0`;
	}

	/** Rows folding away; focus and neighbors skip them. */
	const removing = new SvelteSet<string>();

	/**
	 * Folds the row shut and takes it out. Focus goes first, to `next`, since
	 * the element that had it is about to go; handed over before the fold, a
	 * second Backspace or Enter lands there, not in the closing row.
	 */
	async function dropRow(group: FormGroup, index: number, next: HTMLElement | null) {
		const row = group.ingredients[index];
		removing.add(row.id);
		next?.focus();
		await collapse(document.getElementById(`ingredient-name-${row.id}`)?.closest('li'));
		removing.delete(row.id);
		const current = group.ingredients.findIndex((r) => r.id === row.id);
		if (current !== -1) group.ingredients.splice(current, 1);
	}

	/**
	 * The x button: focus goes to a neighboring row's x (never a field: a caret
	 * there opens a phone's keyboard under the wrong row). The last remaining
	 * row is replaced rather than dropped, and focus goes to the group's add button.
	 */
	async function removeRow(group: FormGroup, index: number) {
		const row = group.ingredients[index];
		if (removing.has(row.id)) return;
		announce(
			m.editor_announce_removed({
				label: row.name.trim() || m.editor_ingredient_row_label({ number: index + 1 })
			})
		);
		const staying = group.ingredients.filter((r) => !removing.has(r.id));
		if (staying.length === 1) {
			group.ingredients.splice(index, 1, newIngredient());
			await tick();
			document.getElementById(`ingredient-add-${group.id}`)?.focus();
			return;
		}
		const at = staying.indexOf(row);
		const neighbor = staying[at === staying.length - 1 ? at - 1 : at + 1];
		await dropRow(group, index, document.getElementById(`ingredient-remove-${neighbor.id}`));
	}

	/**
	 * Backspace in an empty row deletes it and keeps the caret typing: in
	 * the row above, or - when the first row goes - the one that moves up
	 * into its place.
	 */
	async function collapseRow(group: FormGroup, index: number) {
		const staying = group.ingredients.filter((r) => !removing.has(r.id));
		const row = group.ingredients[index];
		if (removing.has(row.id) || staying.length === 1) return;
		const at = staying.indexOf(row);
		const neighbor = staying[at === 0 ? 1 : at - 1];
		await dropRow(group, index, document.getElementById(`ingredient-name-${neighbor.id}`));
	}

	async function addGroup() {
		const group = newGroup();
		fresh.add(group.id);
		groups = [...groups, group];
		// Measured before the group's reveal starts at height 0, so the scroll
		// aims at where the whole group will end up: below the fold on a
		// long list, where the button that added it was.
		flushSync();
		document.getElementById(`ingredient-group-${group.id}`)?.scrollIntoView({
			behavior: prefersReducedMotion.current ? 'auto' : 'smooth',
			block: 'nearest'
		});
		// A new group opens being named. Still inside the tap's handler, so
		// a phone opens its keyboard for it.
		document.getElementById(`ingredient-group-name-${group.id}`)?.focus({ preventScroll: true });
		await tick();
		fresh.delete(group.id);
	}

	/**
	 * Focus goes to the next group's menu button, or to "add group" when
	 * the last group went or one group is left (which has no menu). Set
	 * once the group is gone: the menu hands focus back to its trigger as
	 * it closes, and that trigger is in the group being folded.
	 */
	async function removeGroup(index: number) {
		const { id, name } = groups[index];
		announce(m.editor_announce_removed({ label: name.trim() || m.recipe_ingredients() }));
		await collapse(document.getElementById(`ingredient-group-${id}`));
		groups = groups.filter((group) => group.id !== id);
		if (groups.length === 0) {
			groups = [newGroup()];
		}
		await tick();
		const next = groups.length > 1 ? groups[index] : undefined;
		(
			(next && document.getElementById(`ingredient-group-menu-${next.id}`)) ||
			document.getElementById('ingredient-group-add')
		)?.focus();
	}

	/** The drop slot's height, the dragged row's (see `slotHeight`). */
	let slot = $state(0);

	function reorder(group: FormGroup, event: CustomEvent<DndEvent<FormIngredient>>) {
		slot = slotHeight(event) ?? slot;
		applyDrag(event, group.ingredients, (items) => (group.ingredients = items));
	}
</script>

<div class="@container space-y-5" onfocusin={startEditing} onfocusout={() => (editing = null)}>
	{#if toCheck > 0}
		<p class="text-caption text-primary">
			{toCheck === 1
				? m.editor_ingredients_to_check_one()
				: m.editor_ingredients_to_check({ count: toCheck })}
		</p>
	{/if}
	{#each groups as group, groupIndex (group.id)}
		<!-- A save's error shows with the caret still in the field, but only
		     while the clash it reports is there: fixed anywhere, it goes. -->
		{@const clash =
			now.groups.has(group.id) &&
			(duplicates.groups.has(group.id) || errors[`group:${group.id}`] !== undefined)
				? now.groups.get(group.id)
				: undefined}
		{@const groupError =
			clash === undefined
				? null
				: clash === null
					? m.editor_validation_group_unnamed_twice()
					: m.editor_validation_group_duplicate({ name: clash })}
		{@const named = group.name.trim() !== ''}
		{@const filled = group.ingredients.filter((row) => row.name.trim() !== '').length}
		<!--
			Every group is a card, a single unnamed one too, with its name as a
			tab growing out of the card's top-left edge: dashed and quiet while
			unnamed (an empty label waiting to be written on), filled with the
			accent once named, and in the focus color while typed in - card
			included, like a focused field. The tab overlaps the card's top
			border by its own width and sits above it, so its open bottom edge
			merges into the card. Beside it, at the far end, a quiet count of
			the rows that name an ingredient, gone while none does: a new
			group's empty row is not "1 ingredient".
		-->
		<div
			id="ingredient-group-{group.id}"
			in:reveal={{ fresh: fresh.has(group.id) }}
			class="group/card"
		>
			<div class="flex items-end gap-2">
				<!-- The tab is the field: a tap anywhere on it lands in the name. -->
				<label
					class="group/tab relative z-10 -mb-[1.5px] flex h-11 max-w-full min-w-0 cursor-text items-center gap-1.5 rounded-t-sm border-[1.5px] border-b-0 px-3 transition md:h-9 {groupError
						? 'border-destructive bg-surface text-text focus-within:bg-surface-elevated'
						: named
							? 'border-accent bg-accent text-accent-foreground focus-within:border-primary focus-within:bg-surface-elevated focus-within:text-text'
							: 'border-dashed border-handle bg-surface text-text-muted focus-within:border-solid focus-within:border-primary focus-within:bg-surface-elevated focus-within:text-text'}"
				>
					{#if !named}
						<Plus class="size-4 shrink-0 group-focus-within/tab:hidden" aria-hidden="true" />
					{/if}
					<!-- Grows with the name: the invisible copy sizes the cell
					     the field fills, since inputs do not size to their text. The italic's
					     last letter leans 2px past its box; `pr-0.5` keeps it uncut. -->
					<span class="grid min-w-0">
						<span
							aria-hidden="true"
							class="invisible col-1 row-1 overflow-hidden whitespace-pre {named
								? 'pr-0.5 font-display text-body-lg font-medium italic'
								: 'text-body font-medium'}">{group.name || m.editor_group_name_tab()}</span
						>
						<input
							id="ingredient-group-name-{group.id}"
							bind:value={group.name}
							type="text"
							autocomplete="off"
							enterkeyhint="next"
							aria-label={m.editor_ingredient_group_rename()}
							aria-invalid={groupError ? 'true' : undefined}
							aria-describedby={groupError ? `ingredient-group-error-${group.id}` : undefined}
							data-name-of={group.id}
							data-error-key="group:{group.id}"
							placeholder={m.editor_group_name_tab()}
							class="col-1 row-1 w-0 min-w-full bg-transparent font-display text-body-lg font-medium italic outline-none placeholder:font-sans placeholder:text-body placeholder:font-medium placeholder:text-text-muted placeholder:not-italic"
						/>
					</span>
					{#if named}
						<Pencil
							class="size-3.5 shrink-0 opacity-60 group-focus-within/tab:hidden"
							aria-hidden="true"
						/>
					{/if}
				</label>
				<!-- A quiet count of named rows, text lowered to share the tab name's
				     baseline. Hidden from screen readers: the list says how many it holds. -->
				{#if filled > 0}
					<span
						aria-hidden="true"
						class="-mb-[1.5px] ml-auto flex h-11 shrink-0 items-center text-caption whitespace-nowrap text-text-muted tabular-nums md:h-9 {named
							? 'pt-1.5'
							: 'pt-1'} {groups.length > 1 ? '' : 'pr-1.5'}"
					>
						{filled === 1
							? m.editor_ingredient_group_summary_one()
							: m.editor_ingredient_group_summary({ count: filled })}
					</span>
				{/if}
				{#if groups.length > 1}
					<!-- The hit area is the tab's box; the smaller circle inside stays
					     clear of the card's top border in every state. The menu sits in
					     the card's corner, 6px inside both its top and its right border:
					     `alignOffset` moves it from the box's right edge to the circle's,
					     which is that 6px in, and `sideOffset` 4.5 adds to the box's 1.5px
					     overlap of the border for the same 6px down. -->
					<DropdownMenu.Root>
						<DropdownMenu.Trigger
							id="ingredient-group-menu-{group.id}"
							aria-label={m.editor_group_menu()}
							class="group/menu -mb-[1.5px] inline-flex size-11 shrink-0 items-center justify-center text-text-muted outline-none hover:text-text data-[state=open]:text-text md:size-9 {filled >
							0
								? ''
								: 'ml-auto'}"
						>
							<span
								class="flex size-8 items-center justify-center rounded-pill transition group-hover/menu:bg-current/14 group-focus-visible/menu:outline-2 group-focus-visible/menu:outline-offset-2 group-focus-visible/menu:outline-primary group-data-[state=open]/menu:bg-current/14 md:size-6"
							>
								<Ellipsis class="size-5 md:size-4" aria-hidden="true" />
							</span>
						</DropdownMenu.Trigger>
						<DropdownMenu.Portal>
							<DropdownMenu.Content
								preventScroll={false}
								sideOffset={4.5}
								align="end"
								alignOffset={6}
								collisionPadding={16}
								class="z-50 w-56 rounded-2xl border border-popover-border bg-popover p-2 shadow-dialog"
							>
								<DropdownMenu.Item
									onSelect={() => removeGroup(groupIndex)}
									class="flex h-11 items-center gap-2 rounded-sm px-3 text-body-sm text-destructive transition outline-none data-highlighted:bg-destructive-soft md:h-10"
								>
									<Trash2 class="size-4" aria-hidden="true" />
									{m.editor_group_remove()}
								</DropdownMenu.Item>
							</DropdownMenu.Content>
						</DropdownMenu.Portal>
					</DropdownMenu.Root>
				{/if}
			</div>

			<div
				class="rounded-lg rounded-tl-none border-[1.5px] px-1.5 pt-0.5 pb-2 transition @xl:pt-0.75 @xl:pr-2 {groupError
					? 'border-destructive'
					: named
						? 'border-accent'
						: 'border-dashed border-handle'} {groupError
					? ''
					: 'group-has-[[id^=ingredient-group-name]:focus]/card:border-solid group-has-[[id^=ingredient-group-name]:focus]/card:border-primary'}"
			>
				<!-- Said as it appears: it shows once the field is left, where a
				     description of that field is no longer read. -->
				<p class="sr-only" aria-live="polite">{groupError ?? ''}</p>
				{#if groupError}
					<FieldError
						id="ingredient-group-error-{group.id}"
						message={groupError}
						class="mb-1.5 pl-1.5"
					/>
				{/if}

				<!-- On one line the fields carry no labels: this header names the
				     columns on the rows' grid (`wideColumns`), in a container as wide as
				     a row so it switches with them. Outside the dnd zone, so never
				     dragged; hidden from screen readers, each label names its field. -->
				<div class="@container" aria-hidden="true">
					<div
						class="hidden text-micro font-medium text-field-label @xl:grid {wideColumns} @xl:py-0.75"
					>
						<span class="col-2 pl-3.25">{m.editor_ingredient_name()}</span>
						<span class="pl-3.25">{m.editor_ingredient_quantity()}</span>
						<span class="pl-3.25">{m.editor_ingredient_unit()}</span>
						<span class="pl-3.25">{m.editor_ingredient_note()}</span>
					</div>
				</div>

				<!-- The `aria-label`s are what svelte-dnd-action reads out while
				     dragging. A pressed handle picks its row up onto a card
				     (`pickUp`), in front of its neighbors; a placed row lights up
				     (`flashPlaced`) 1px off its edge. The list itself is no Tab stop
				     (`zoneTabIndex`): the handles start a keyboard drag, and while one
				     runs the library makes the other groups' lists tabbable to drop into. -->
				<ul
					use:dragHandleZone={{
						items: group.ingredients,
						type: 'ingredient',
						flipDurationMs: FLIP_DURATION,
						zoneTabIndex: -1,
						dropTargetStyle: {},
						morphDisabled: true,
						transformDraggedElement: liftDragged
					}}
					onconsider={(event) => reorder(group, event)}
					onfinalize={(event) => reorder(group, event)}
					aria-label={group.name.trim() === '' ? m.recipe_ingredients() : group.name}
				>
					{#each group.ingredients as row, rowIndex (row.id)}
						<li
							animate:flip={{ duration: fresh.size > 0 ? 0 : FLIP_DURATION }}
							in:reveal={{ fresh: fresh.has(row.id) }}
							aria-label={row.name.trim() === ''
								? m.editor_ingredient_row_label({ number: rowIndex + 1 })
								: row.name}
							class="list-none outline-none [--flash-offset:1px] data-lift:relative data-lift:z-10"
							data-slot={isShadow(row) ? '' : undefined}
							style:height={isShadow(row) ? `${slot}px` : undefined}
						>
							{#if isShadow(row)}
								<!-- Where the dragged row will land: a slot of its height in
								     place of a second copy of it. The library hides the
								     shadow element, so the slot shows itself; `data-slot`
								     tells the row after it to drop its hairline. -->
								<div
									aria-hidden="true"
									class="visible h-full rounded-lg border-2 border-dashed border-handle bg-accent/40"
								></div>
							{:else}
								<IngredientRow
									bind:row={group.ingredients[rowIndex]}
									quantityError={errors[`quantity:${row.id}`] ?? null}
									nameError={errors[`name:${row.id}`] ?? null}
									missing={errors.ingredientGroups !== undefined &&
										groupIndex === 0 &&
										rowIndex === 0}
									duplicate={duplicates.rows.has(row.id)}
									onenter={() => addRow(group, rowIndex + 1)}
									onbackspace={() => collapseRow(group, rowIndex)}
									onremove={() => removeRow(group, rowIndex)}
								/>
							{/if}
						</li>
					{/each}
				</ul>

				<!-- Aligned with the name field's left edge, past the grip column. Its
				     own container so it switches with the rows; outside the dnd zone. -->
				<div class="@container mt-1.5 @xl:mt-0.75">
					<div class="pl-6.5 @xl:pl-7">
						<AddButton
							id="ingredient-add-{group.id}"
							onclick={() => addRow(group)}
							class="h-11 @xl:h-10 @xl:pl-3.25"
						>
							{m.editor_ingredient_add_ghost()}
						</AddButton>
					</div>
				</div>
			</div>
		</div>
	{/each}

	<!-- As wide as a group's card, one step deeper in tint than the adds
	     inside a card: it adds a whole list, not a row. -->
	<AddButton id="ingredient-group-add" onclick={addGroup} deep class="h-11 md:h-10">
		{m.editor_ingredient_add_group()}
	</AddButton>
	<!-- Last, so the stack's spacing never counts it. -->
	<p class="sr-only" aria-live="polite">{said}</p>
</div>
