<script lang="ts">
	import { flushSync, tick } from 'svelte';
	import { flip } from 'svelte/animate';
	import { prefersReducedMotion } from 'svelte/motion';
	import Plus from '@lucide/svelte/icons/plus';
	import { dragHandleZone, type DndEvent } from 'svelte-dnd-action';
	import Button from '#lib/components/ui/Button.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import {
		newGroup,
		newIngredient,
		type FieldErrors,
		type FormGroup,
		type FormIngredient
	} from '#lib/recipe/form.js';
	import { findDuplicates } from '#lib/recipe/step-references.js';
	import IngredientRow from './IngredientRow.svelte';
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

	/** Moves the caret into a freshly added row once it is in the DOM. */
	async function focusRow(id: string) {
		await tick();
		document.getElementById(`ingredient-quantity-${id}`)?.focus();
	}

	async function addRow(group: FormGroup, index = group.ingredients.length) {
		const row = newIngredient();
		fresh.add(row.id);
		group.ingredients.splice(index, 0, row);
		await focusRow(row.id);
		fresh.delete(row.id);
	}

	/**
	 * Folds the row shut and takes it out. Focus goes first, to the row that
	 * will take the removed one's place, since the element that had it (the
	 * x button, or the field Backspace was pressed in) is about to go and
	 * focus would otherwise fall to `<body>`: the row above, or - when the
	 * first row goes - the one that moves up into index 0. Handed over before
	 * the fold, a second Backspace lands in that neighbor instead of the
	 * closing row. The row is looked up again afterwards, since the list can
	 * have changed while it closed.
	 */
	async function dropRow(group: FormGroup, index: number) {
		const row = group.ingredients[index];
		const neighbor = group.ingredients[index === 0 ? 1 : index - 1];
		document.getElementById(`ingredient-name-${neighbor.id}`)?.focus();
		await collapse(document.getElementById(`ingredient-name-${row.id}`)?.closest('li'));
		const current = group.ingredients.findIndex((r) => r.id === row.id);
		if (current !== -1) group.ingredients.splice(current, 1);
	}

	async function removeRow(group: FormGroup, index: number) {
		// The API allows a group without rows, but an empty editor group has
		// nothing to type into, so the last row is replaced rather than dropped.
		if (group.ingredients.length === 1) {
			const row = newIngredient();
			group.ingredients.splice(0, 1, row);
			await focusRow(row.id);
			return;
		}
		await dropRow(group, index);
	}

	/** Backspace in an empty row deletes it and moves the caret to its neighbor. */
	async function collapseRow(group: FormGroup, index: number) {
		if (group.ingredients.length === 1) {
			return;
		}
		await dropRow(group, index);
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
		await tick();
		fresh.delete(group.id);
	}

	async function removeGroup(index: number) {
		const { id } = groups[index];
		await collapse(document.getElementById(`ingredient-group-${id}`));
		groups = groups.filter((group) => group.id !== id);
		if (groups.length === 0) {
			groups = [newGroup()];
		}
	}

	function reorder(group: FormGroup, event: CustomEvent<DndEvent<FormIngredient>>) {
		group.ingredients = event.detail.items;
	}
</script>

<div class="space-y-6" onfocusin={startEditing} onfocusout={() => (editing = null)}>
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
		<div
			id="ingredient-group-{group.id}"
			in:reveal={{ fresh: fresh.has(group.id) }}
			class="space-y-3"
		>
			<!--
				The name field takes the whole line on phones: beside the count and
				"Entfernen" it was left with ~90px, not enough for its own
				placeholder ("Gruppe benennen …" needs 140px), so a new group
				opened with its prompt cut off. Desktop keeps the single row.
			-->
			<div class="flex flex-wrap items-center gap-x-3 gap-y-1">
				<input
					id="ingredient-group-name-{group.id}"
					bind:value={group.name}
					type="text"
					autocomplete="off"
					aria-label={m.editor_ingredient_group_rename()}
					aria-invalid={groupError ? 'true' : undefined}
					aria-describedby={groupError ? `ingredient-group-error-${group.id}` : undefined}
					data-name-of={group.id}
					data-error-key="group:{group.id}"
					placeholder={m.editor_group_name_placeholder()}
					class="w-full min-w-0 border-b {groupError
						? 'border-destructive focus:border-destructive'
						: 'border-transparent focus:border-border'} bg-transparent pb-1 font-display text-body-lg font-medium text-primary italic transition outline-none placeholder:text-text-muted placeholder:not-italic md:w-auto md:flex-1"
				/>
				<!-- Under the field on phones, where it has the line to itself;
				     after the count and "Entfernen" on desktop, so the row
				     above keeps its single line. -->
				<!-- Said as it appears: it shows once the field is left, where a
				     description of that field is no longer read. -->
				<p class="sr-only" aria-live="polite">{groupError ?? ''}</p>
				{#if groupError}
					<p
						id="ingredient-group-error-{group.id}"
						class="w-full text-micro font-medium text-destructive motion-safe:animate-error-in md:order-last"
					>
						{groupError}
					</p>
				{/if}
				<!-- The catalog has no plural variants, so the singular is its
				     own message rather than "1 ingredients". -->
				<span class="shrink-0 text-caption text-text-muted">
					{group.ingredients.length === 1
						? m.editor_ingredient_group_summary_one()
						: m.editor_ingredient_group_summary({ count: group.ingredients.length })}
				</span>
				{#if groups.length > 1}
					<!-- Visible "Entfernen" is short enough for the header row; the
					     accessible name says which Entfernen this is. It contains
					     the visible label, so voice control still matches it. -->
					<button
						type="button"
						onclick={() => removeGroup(groupIndex)}
						aria-label={m.editor_group_remove()}
						class="ml-auto inline-flex h-11 shrink-0 items-center rounded-pill px-3 text-caption font-medium text-text-muted transition hover:text-destructive focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary md:ml-0 md:h-8"
					>
						{m.common_remove()}
					</button>
				{/if}
			</div>

			<!-- The `aria-label`s on the zone and on every item are what
			     svelte-dnd-action reads out while dragging (see its README);
			     without them its announcements say "item" and "list". -->
			<ul
				use:dragHandleZone={{
					items: group.ingredients,
					type: 'ingredient',
					flipDurationMs: FLIP_DURATION,
					dropTargetStyle: {}
				}}
				onconsider={(event) => reorder(group, event)}
				onfinalize={(event) => reorder(group, event)}
				aria-label={group.name.trim() === '' ? m.recipe_ingredients() : group.name}
				class="space-y-1.5"
			>
				{#each group.ingredients as row, rowIndex (row.id)}
					<li
						animate:flip={{ duration: fresh.size > 0 ? 0 : FLIP_DURATION }}
						in:reveal={{ fresh: fresh.has(row.id) }}
						aria-label={row.name.trim() === ''
							? m.editor_ingredient_row_label({ number: rowIndex + 1 })
							: row.name}
						class="list-none"
					>
						<IngredientRow
							bind:row={group.ingredients[rowIndex]}
							quantityError={errors[`quantity:${row.id}`] ?? null}
							nameError={errors[`name:${row.id}`] ?? null}
							duplicate={duplicates.rows.has(row.id)}
							onenter={() => addRow(group, rowIndex + 1)}
							onbackspace={() => collapseRow(group, rowIndex)}
							onremove={() => removeRow(group, rowIndex)}
						/>
					</li>
				{/each}
			</ul>

			<Button variant="accent" onclick={() => addRow(group)} class="w-full md:w-auto">
				<Plus class="size-4" aria-hidden="true" />
				{m.editor_ingredient_add_row()}
			</Button>
		</div>
	{/each}

	<Button variant="secondary" onclick={addGroup} class="w-full md:w-auto">
		<Plus class="size-4" aria-hidden="true" />
		{m.editor_ingredient_add_group()}
	</Button>
</div>
