<script lang="ts">
	import GripVertical from 'lucide-svelte/icons/grip-vertical';
	import X from 'lucide-svelte/icons/x';
	import { dragHandle } from 'svelte-dnd-action';
	import { m } from '$lib/paraglide/messages';
	import { isBlank, type FormIngredient } from '$lib/recipe/form';

	let {
		row,
		quantityError = null,
		nameError = null,
		onenter,
		onbackspace,
		onremove
	}: {
		/** Part of the form's `$state`, so the fields are mutated in place. */
		row: FormIngredient;
		quantityError?: string | null;
		nameError?: string | null;
		/** Enter in the name or note field: add a row below and focus it. */
		onenter: () => void;
		/** Backspace in an otherwise empty row: remove it. */
		onbackspace: () => void;
		onremove: () => void;
	} = $props();

	const empty = $derived(
		isBlank(row.quantity) && isBlank(row.unit) && isBlank(row.name) && isBlank(row.note)
	);

	const fieldClasses =
		'h-11 w-full rounded-sm border border-border bg-surface-elevated px-2.5 text-body-sm outline-none transition focus:border-primary md:h-10';
	const errorClasses = 'border-[1.5px] border-destructive';

	function handleKeydown(event: KeyboardEvent, submitsRow: boolean) {
		if (event.key === 'Backspace' && empty) {
			event.preventDefault();
			onbackspace();
			return;
		}
		if (event.key === 'Enter' && submitsRow) {
			event.preventDefault();
			onenter();
		}
	}
</script>

<div>
	<div
		class="grid grid-cols-[18px_1fr_24px] items-center gap-2 md:grid-cols-[20px_80px_100px_1fr_160px_28px]"
	>
		<button
			use:dragHandle
			type="button"
			aria-label={m.editor_ingredient_reorder()}
			class="flex cursor-grab items-center justify-center text-handle focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
		>
			<GripVertical class="size-4" aria-hidden="true" />
		</button>

		<!-- `md:contents` lifts the fields into the outer grid on desktop; on
		     mobile they keep their own compact three-column layout with the
		     note wrapping onto a second line. -->
		<div class="grid grid-cols-[64px_72px_1fr] gap-1.5 md:contents">
			<input
				id="ingredient-quantity-{row.id}"
				bind:value={row.quantity}
				onkeydown={(event) => handleKeydown(event, false)}
				type="text"
				inputmode="decimal"
				autocomplete="off"
				aria-label={m.editor_ingredient_quantity()}
				aria-invalid={quantityError ? 'true' : undefined}
				aria-describedby={quantityError ? `ingredient-quantity-error-${row.id}` : undefined}
				placeholder={m.editor_ingredient_quantity()}
				class="{fieldClasses} font-semibold {quantityError ? errorClasses : ''}"
			/>
			<input
				id="ingredient-unit-{row.id}"
				bind:value={row.unit}
				onkeydown={(event) => handleKeydown(event, false)}
				type="text"
				list="unit-suggestions"
				autocomplete="off"
				aria-label={m.editor_ingredient_unit()}
				placeholder={m.editor_ingredient_unit()}
				class={fieldClasses}
			/>
			<input
				id="ingredient-name-{row.id}"
				bind:value={row.name}
				onkeydown={(event) => handleKeydown(event, true)}
				type="text"
				autocomplete="off"
				aria-label={m.editor_ingredient_name()}
				aria-invalid={nameError ? 'true' : undefined}
				aria-describedby={nameError ? `ingredient-name-error-${row.id}` : undefined}
				placeholder={m.editor_ingredient_name()}
				class="{fieldClasses} {nameError ? errorClasses : ''}"
			/>
			<input
				id="ingredient-note-{row.id}"
				bind:value={row.note}
				onkeydown={(event) => handleKeydown(event, true)}
				type="text"
				autocomplete="off"
				aria-label={m.editor_ingredient_note()}
				placeholder={m.editor_ingredient_note()}
				class="{fieldClasses} col-span-3 text-text-muted md:col-span-1"
			/>
		</div>

		<button
			type="button"
			onclick={onremove}
			aria-label={m.editor_ingredient_remove()}
			class="flex items-center justify-center text-text-muted transition hover:text-destructive focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
		>
			<X class="size-4" aria-hidden="true" />
		</button>
	</div>

	<!-- One element per field: both inputs can be wrong at once, and each
	     needs its own id to point `aria-describedby` at. -->
	{#if quantityError}
		<p
			id="ingredient-quantity-error-{row.id}"
			class="mt-1 pl-5 text-micro font-medium text-destructive md:pl-7"
		>
			{quantityError}
		</p>
	{/if}
	{#if nameError}
		<p
			id="ingredient-name-error-{row.id}"
			class="mt-1 pl-5 text-micro font-medium text-destructive md:pl-7"
		>
			{nameError}
		</p>
	{/if}
</div>
