<script lang="ts">
	import GripVertical from 'lucide-svelte/icons/grip-vertical';
	import X from 'lucide-svelte/icons/x';
	import { dragHandle } from 'svelte-dnd-action';
	import { m } from '$lib/paraglide/messages';
	import { isBlank, type FormIngredient } from '$lib/recipe/form';

	let {
		row = $bindable(),
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
		'h-11 w-full min-w-0 rounded-md border border-border bg-surface-elevated px-3 text-body outline-none transition placeholder:text-text-muted focus:border-primary md:h-10 md:text-body-sm';
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
		class="grid grid-cols-[18px_1fr_24px] items-start gap-2 md:grid-cols-[20px_80px_100px_1fr_160px_28px] md:items-center"
	>
		<button
			use:dragHandle
			type="button"
			aria-label={m.editor_ingredient_reorder()}
			class="mt-3.5 flex cursor-grab items-center justify-center text-handle focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary md:mt-0"
		>
			<GripVertical class="size-4" aria-hidden="true" />
		</button>

		<!-- `md:contents` lifts the fields into the outer grid on desktop. On
		     mobile they stack: the ingredient name on its own line first (it
		     is what a cook scans for), quantity and unit side by side below,
		     the optional note last. `order-first` reorders only on mobile; the
		     DOM order stays quantity, unit, name, note for the desktop grid. -->
		<div class="grid grid-cols-2 gap-2 md:contents">
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
				class="{fieldClasses} order-first col-span-2 md:order-none md:col-span-1 {nameError
					? errorClasses
					: ''}"
			/>
			<input
				id="ingredient-note-{row.id}"
				bind:value={row.note}
				onkeydown={(event) => handleKeydown(event, true)}
				type="text"
				autocomplete="off"
				aria-label={m.editor_ingredient_note()}
				placeholder={m.editor_ingredient_note()}
				class="{fieldClasses} col-span-2 text-text-muted md:col-span-1"
			/>
		</div>

		<button
			type="button"
			onclick={onremove}
			aria-label={m.editor_ingredient_remove()}
			class="mt-3.5 flex items-center justify-center text-text-muted transition hover:text-destructive focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary md:mt-0"
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
