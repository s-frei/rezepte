<script lang="ts">
	import Plus from '@lucide/svelte/icons/plus';
	import UnitInput from '#lib/components/editor/UnitInput.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { parseNumber } from '#lib/recipe/form.js';

	let {
		onadd
	}: {
		/** Stores the item; resolves to whether it landed, which clears the fields. */
		onadd: (name: string, fields: { quantity?: number; unit?: string }) => Promise<boolean>;
	} = $props();

	let quantity = $state('');
	let unit = $state('');
	let name = $state('');
	let busy = $state(false);
	let quantityError = $state(false);
	let nameField: HTMLInputElement | undefined = $state();
	let quantityField: HTMLInputElement | undefined = $state();

	const fieldClasses =
		'h-11 w-full min-w-0 rounded-md border border-border bg-surface-elevated px-3 text-body outline-none transition placeholder:text-text-muted focus:border-primary';

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		const text = name.trim();
		const amount = quantity.trim() === '' ? undefined : parseNumber(quantity);
		quantityError = amount === null || (amount !== undefined && amount <= 0);
		if (quantityError) {
			quantityField?.focus();
			return;
		}
		if (!text || busy) {
			nameField?.focus();
			return;
		}
		busy = true;
		try {
			if (await onadd(text, { quantity: amount ?? undefined, unit: unit.trim() || undefined })) {
				quantity = unit = name = '';
			}
		} finally {
			busy = false;
			// The next item is typed right away, from the row's first field.
			(window.matchMedia('(min-width: 48rem)').matches ? quantityField : nameField)?.focus();
		}
	}
</script>

<!--
	Laid out like an ingredient row in the recipe editor: on a desktop one row,
	amount, unit, name and the + button; on a phone the name comes first on a
	line of its own (`order-first`), amount, unit and + below it.
-->
<form
	onsubmit={submit}
	novalidate
	class="rounded-2xl bg-surface p-3 shadow-card md:p-3.5"
	aria-label={m.list_add_title()}
>
	<div
		class="grid grid-cols-[minmax(0,1fr)_minmax(0,1.3fr)_44px] gap-2 md:grid-cols-[88px_120px_minmax(0,1fr)_44px]"
	>
		<input
			bind:this={quantityField}
			bind:value={quantity}
			oninput={() => (quantityError = false)}
			type="text"
			inputmode="decimal"
			autocomplete="off"
			enterkeyhint="done"
			aria-label={m.editor_ingredient_quantity()}
			aria-invalid={quantityError ? 'true' : undefined}
			aria-describedby={quantityError ? 'list-add-quantity-error' : undefined}
			placeholder={m.editor_ingredient_quantity()}
			class="{fieldClasses} font-semibold {quantityError
				? 'border-[1.5px] border-destructive'
				: ''}"
		/>
		<UnitInput id="list-add-unit" bind:value={unit} class={fieldClasses} />
		<input
			bind:this={nameField}
			bind:value={name}
			aria-label={m.list_add_name()}
			placeholder={m.list_add_name()}
			autocomplete="off"
			enterkeyhint="done"
			maxlength={200}
			class="{fieldClasses} order-first col-span-3 md:order-none md:col-span-1"
		/>
		<button
			type="submit"
			aria-label={m.list_add()}
			class="inline-flex size-11 items-center justify-center rounded-full bg-primary hover-tint text-primary-foreground transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary active:scale-[.98]"
		>
			<Plus class="size-5" aria-hidden="true" />
		</button>
	</div>
	{#if quantityError}
		<p
			id="list-add-quantity-error"
			class="mt-1.5 text-micro font-medium text-destructive motion-safe:animate-error-in"
		>
			{m.editor_validation_number()}
		</p>
	{/if}
</form>
