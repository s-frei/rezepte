<script lang="ts">
	import { tick } from 'svelte';
	import { flip } from 'svelte/animate';
	import Plus from 'lucide-svelte/icons/plus';
	import { dragHandleZone, type DndEvent } from 'svelte-dnd-action';
	import { UNIT_SUGGESTIONS } from '$lib/api/recipes';
	import Button from '$lib/components/ui/Button.svelte';
	import { m } from '$lib/paraglide/messages';
	import {
		newGroup,
		newIngredient,
		type FieldErrors,
		type FormGroup,
		type FormIngredient
	} from '$lib/recipe/form';
	import IngredientRow from './IngredientRow.svelte';

	let {
		groups = $bindable([]),
		errors = {}
	}: {
		groups?: FormGroup[];
		errors?: FieldErrors;
	} = $props();

	const FLIP_DURATION = 150;

	/** Moves the caret into a freshly added row once it is in the DOM. */
	async function focusRow(id: string) {
		await tick();
		document.getElementById(`ingredient-quantity-${id}`)?.focus();
	}

	async function addRow(group: FormGroup, index = group.ingredients.length) {
		const row = newIngredient();
		group.ingredients.splice(index, 0, row);
		await focusRow(row.id);
	}

	/**
	 * Hands focus to the row that took the removed one's place, since the
	 * element that had focus (the x button, or the field Backspace was
	 * pressed in) is gone and focus would otherwise fall to `<body>`. That is
	 * the row above, or - when the first row went - the one that moved up into
	 * index 0. When the group was emptied and refilled below, index 0 is the
	 * fresh blank row.
	 */
	async function focusRowAfterRemoval(group: FormGroup, removedIndex: number) {
		const neighbour = group.ingredients[Math.max(0, removedIndex - 1)];
		await tick();
		document.getElementById(`ingredient-name-${neighbour.id}`)?.focus();
	}

	async function removeRow(group: FormGroup, index: number) {
		group.ingredients.splice(index, 1);
		// The API allows a group without rows, but an empty editor group has
		// nothing to type into, so the last row is replaced rather than dropped.
		if (group.ingredients.length === 0) {
			group.ingredients.push(newIngredient());
		}
		await focusRowAfterRemoval(group, index);
	}

	/** Backspace in an empty row deletes it and moves the caret to its neighbour. */
	async function collapseRow(group: FormGroup, index: number) {
		if (group.ingredients.length === 1) {
			return;
		}
		group.ingredients.splice(index, 1);
		await focusRowAfterRemoval(group, index);
	}

	function addGroup() {
		groups = [...groups, newGroup()];
	}

	function removeGroup(index: number) {
		groups = groups.filter((_, position) => position !== index);
		if (groups.length === 0) {
			groups = [newGroup()];
		}
	}

	function reorder(group: FormGroup, event: CustomEvent<DndEvent<FormIngredient>>) {
		group.ingredients = event.detail.items;
	}
</script>

<!-- One datalist for every unit field on the page. -->
<datalist id="unit-suggestions">
	{#each UNIT_SUGGESTIONS as unit (unit)}
		<option value={unit}></option>
	{/each}
</datalist>

<div class="space-y-6">
	{#each groups as group, groupIndex (group.id)}
		<div class="space-y-3">
			<div class="flex items-center gap-3">
				<input
					bind:value={group.name}
					type="text"
					autocomplete="off"
					aria-label={m.editor_ingredient_group_rename()}
					placeholder={m.editor_group_name_placeholder()}
					class="min-w-0 flex-1 border-b border-transparent bg-transparent pb-1 font-display text-body-lg font-medium text-primary italic transition outline-none placeholder:text-text-muted placeholder:not-italic focus:border-border"
				/>
				<span class="shrink-0 text-caption text-text-muted">
					{m.editor_ingredient_group_summary({ count: group.ingredients.length })}
				</span>
				{#if groups.length > 1}
					<!-- Visible "Entfernen" is short enough for the header row; the
					     accessible name says which Entfernen this is. It contains
					     the visible label, so voice control still matches it. -->
					<button
						type="button"
						onclick={() => removeGroup(groupIndex)}
						aria-label={m.editor_group_remove()}
						class="shrink-0 text-caption font-medium text-text-muted transition hover:text-destructive focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
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
						animate:flip={{ duration: FLIP_DURATION }}
						aria-label={row.name.trim() === ''
							? m.editor_ingredient_row_label({ number: rowIndex + 1 })
							: row.name}
						class="list-none"
					>
						<IngredientRow
							{row}
							quantityError={errors[`quantity:${row.id}`] ?? null}
							nameError={errors[`name:${row.id}`] ?? null}
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
