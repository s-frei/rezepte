<script lang="ts">
	import NotebookPen from '@lucide/svelte/icons/notebook-pen';
	import { Dialog } from 'bits-ui';
	import { imageUrl, listRecipes, type RecipeCard } from '#lib/api/recipes.js';
	import Input from '#lib/components/ui/Input.svelte';
	import ResponsiveDialog from '#lib/components/ui/ResponsiveDialog.svelte';
	import { addRows, TRAY } from '#lib/mealplan/entries.js';
	import { dayNames } from '#lib/mealplan/week.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import { initialFor, tintFor } from '#lib/recipe/placeholder.js';

	/**
	 * Plans a meal: the day as chips (the week's seven and "Someday"), then
	 * one field that searches the recipes and, for anything else, takes the
	 * text as a note. The field is a combobox: its list holds the recipes
	 * found for exactly the text typed (`addRows`) and, once something is
	 * typed, the note last; arrows move through it and Enter takes the
	 * highlighted row, the first one unless moved. An empty field lists the
	 * recipes changed last, to pick by hand. A recipe comes in at its own
	 * servings.
	 */
	let {
		open = $bindable(false),
		days,
		day,
		today,
		onadd
	}: {
		open?: boolean;
		/** The shown week's days. */
		days: string[];
		/** The day the sheet opens on; null the tray. */
		day: string | null;
		today: string;
		/** Resolves false when the plan refused it; the sheet stays open then. */
		onadd: (input: { day: string | null; recipeId?: string; title?: string }) => Promise<boolean>;
	} = $props();

	const locale = getLocale();

	let picked = $state(TRAY);
	let query = $state('');
	/** The last answer, with the text it answers. */
	let found = $state<{ query: string; items: RecipeCard[] } | null>(null);
	const results = $derived(found?.query === query.trim() ? found.items : []);
	let active = $state(0);
	let sending = $state(false);

	// Each opening starts over on the asked day with an empty field.
	$effect(() => {
		if (!open) return;
		picked = day ?? TRAY;
		query = '';
		active = 0;
		found = null;
	});

	// As the palette searches: 150ms after the last key, an older request
	// aborted; the empty field lists the recipes changed last.
	$effect(() => {
		if (!open) return;
		const q = query.trim();
		const controller = new AbortController();
		const timer = setTimeout(async () => {
			try {
				const items = (await listRecipes({ q, limit: 8 }, { signal: controller.signal })).items;
				found = { query: q, items };
			} catch (error) {
				if (!(error instanceof DOMException && error.name === 'AbortError'))
					found = { query: q, items: [] };
			}
		}, 150);
		return () => {
			clearTimeout(timer);
			controller.abort();
		};
	});

	const rows = $derived(addRows(query, found));
	type Row = (typeof rows)[number];
	/** The highlighted row, back on the first once the list got shorter. */
	const current = $derived(active < rows.length ? active : 0);

	async function take(row: Row | undefined) {
		if (!row || sending) return;
		sending = true;
		const target = picked === TRAY ? null : picked;
		const done = await onadd(
			row.recipe ? { day: target, recipeId: row.recipe.id } : { day: target, title: row.note }
		);
		sending = false;
		if (done) open = false;
	}

	function keydown(event: KeyboardEvent) {
		if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
			event.preventDefault();
			if (rows.length === 0) return;
			active = (current + (event.key === 'ArrowDown' ? 1 : rows.length - 1)) % rows.length;
			document.getElementById(rows[active].id)?.scrollIntoView({ block: 'nearest' });
		} else if (event.key === 'Enter') {
			event.preventDefault();
			// An empty field lists recent recipes to pick by hand; Enter there plans nothing.
			if (query.trim()) void take(rows[current]);
		}
	}

	function focusField(event: Event) {
		event.preventDefault();
		document.getElementById('plan-add-field')?.focus();
	}

	const chip =
		'flex h-11 cursor-pointer items-center justify-center gap-1 rounded-pill border border-border bg-surface-elevated px-2 text-body-sm whitespace-nowrap text-text transition peer-checked:border-primary peer-checked:bg-primary peer-checked:text-primary-foreground peer-focus-visible:outline-2 peer-focus-visible:outline-offset-2 peer-focus-visible:outline-primary md:h-10';
	const row =
		'flex min-h-12 w-full cursor-pointer items-center gap-3 rounded-md px-2 py-1.5 text-left text-body-sm text-text';
</script>

{#snippet body()}
	<Dialog.Title class="font-display text-heading font-medium">{m.week_add()}</Dialog.Title>
	<fieldset class="mt-4">
		<legend class="sr-only">{m.plan_add_day()}</legend>
		<!-- Three across: the seven days, and "Someday" over two cells. -->
		<div class="grid grid-cols-3 gap-1.5">
			{#each [...days, TRAY] as option (option)}
				{@const names = option === TRAY ? null : dayNames(option, locale)}
				<label class="relative {option === TRAY ? 'col-span-2' : ''}">
					<input
						type="radio"
						name="plan-add-day"
						value={option}
						bind:group={picked}
						aria-label={names ? names.full : m.plan_tray()}
						class="peer absolute inset-0 z-10 cursor-pointer appearance-none rounded-pill opacity-0"
					/>
					<span class={chip} aria-hidden="true">
						{#if names}
							<span class="font-semibold">{names.weekday}</span>
							<span class="tabular-nums opacity-80">{names.number}</span>
							{#if option === today}<span class="ml-0.5 size-1.5 rounded-full bg-current"
								></span>{/if}
						{:else}
							<span class="font-semibold">{m.plan_tray()}</span>
						{/if}
					</span>
				</label>
			{/each}
		</div>
	</fieldset>
	<div class="mt-5">
		<Input
			id="plan-add-field"
			label={m.plan_add_field()}
			placeholder={m.plan_add_placeholder()}
			autocomplete="off"
			enterkeyhint="done"
			role="combobox"
			aria-expanded={rows.length > 0}
			aria-controls="plan-add-list"
			aria-autocomplete="list"
			aria-activedescendant={rows[current]?.id}
			maxlength={200}
			bind:value={query}
			onkeydown={keydown}
		/>
	</div>
	{#if results.length > 0}
		<p class="mt-4 mb-1 text-caption font-semibold text-text-muted" aria-hidden="true">
			{query.trim() ? m.plan_add_results() : m.plan_add_recent()}
		</p>
	{/if}
	<ul
		id="plan-add-list"
		role="listbox"
		aria-label={m.plan_add_results()}
		class="-mx-2 pb-1 {results.length > 0 ? '' : 'mt-3'}"
	>
		{#each rows as option, i (option.id)}
			<!-- The keys are the field's (arrows, Enter): an option of a combobox takes no focus. -->
			<!-- svelte-ignore a11y_click_events_have_key_events -->
			<li
				id={option.id}
				role="option"
				aria-selected={i === current}
				onclick={() => take(option)}
				onpointermove={() => (active = i)}
				class="{row} {i === current ? 'bg-background' : ''}"
			>
				{#if option.recipe}
					{@const recipe = option.recipe}
					{#if recipe.coverImageId}
						<img
							src={imageUrl(recipe.id, recipe.coverImageId, 'thumb')}
							alt=""
							class="size-10 shrink-0 rounded-md object-cover"
						/>
					{:else}
						<span
							aria-hidden="true"
							class="flex size-10 shrink-0 items-center justify-center rounded-md font-display text-card font-medium text-primary italic {tintFor(
								recipe.id
							)}">{initialFor(recipe.title)}</span
						>
					{/if}
					<span class="min-w-0 font-semibold [hyphens:none]">{recipe.title}</span>
				{:else}
					<span
						aria-hidden="true"
						class="flex size-10 shrink-0 items-center justify-center rounded-md border-[1.5px] border-dashed border-handle text-text-muted"
					>
						<NotebookPen class="size-4" />
					</span>
					<span class="min-w-0 break-words">{m.plan_add_note({ text: option.note ?? '' })}</span>
				{/if}
			</li>
		{/each}
	</ul>
	{#if query.trim() && found?.query === query.trim() && results.length === 0}
		<p class="pt-1 text-caption text-text-muted">{m.plan_add_none()}</p>
	{/if}
{/snippet}

<ResponsiveDialog
	bind:open
	closeLabel={m.common_cancel()}
	onOpenAutoFocus={focusField}
	bodyClass="pb-6"
	{body}
/>
