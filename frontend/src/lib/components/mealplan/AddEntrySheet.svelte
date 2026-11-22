<script lang="ts">
	import CalendarDays from '@lucide/svelte/icons/calendar-days';
	import NotebookPen from '@lucide/svelte/icons/notebook-pen';
	import { Dialog } from 'bits-ui';
	import { tick } from 'svelte';
	import { imageUrl, listRecipes, type RecipeCard } from '#lib/api/recipes.js';
	import Input from '#lib/components/ui/Input.svelte';
	import ResponsiveDialog from '#lib/components/ui/ResponsiveDialog.svelte';
	import TypingFold from '#lib/components/ui/TypingFold.svelte';
	import { addRows, TRAY } from '#lib/mealplan/entries.js';
	import { dayNames } from '#lib/mealplan/week.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import { initialFor, tintFor } from '#lib/recipe/placeholder.js';
	import { narrow } from '#lib/ui/narrow.js';
	import { phone } from '#lib/ui/phone.svelte.js';
	import { settleHeight, STALE_ROW } from '#lib/ui/settle-height.js';
	import { softKeyboard } from '#lib/visible-area.js';

	/**
	 * Plans a meal: the day as chips (the week's seven and "Someday"), then
	 * one field that searches the recipes and, for anything else, takes the
	 * text as a note. The field is a combobox: its list holds the recipes
	 * found for exactly the text typed (`addRows`) and, once something is
	 * typed, the note last; arrows move through it and Enter takes the
	 * highlighted row, the first one unless moved. An empty field lists the
	 * recipes changed last, to pick by hand. A recipe comes in at its own
	 * servings. On a phone, while the field has focus, title and chips fold
	 * away so the field heads the sheet and the rows fill the room above the
	 * keyboard; the day stays in sight as a chip on the field's label line,
	 * which brings the chips back.
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
	/** A keystroke whose answer is not in. */
	const pending = $derived(found?.query !== query.trim());
	/**
	 * What the list shows: `rows`, and until the answer for this text is in,
	 * the last answer's recipes that still hold the text above them, dimmed
	 * and out of reach, so the list keeps its size between keystrokes; a
	 * recipe that cannot match any more goes. Same ids, so a recipe in both
	 * answers stays where it is.
	 */
	const shown = $derived.by(() => {
		if (!found || !pending) return rows;
		const text = query.trim().toLocaleLowerCase();
		const held = addRows(found.query, found).filter((r) =>
			r.recipe?.title.toLocaleLowerCase().includes(text)
		);
		return [...held, ...rows];
	});
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

	// A phone's (or a touch screen's) sheet opens on the recipes: a focused field would raise the
	// keyboard over them. The sheet takes the focus; a tap on the field the rest.
	function focusField(event: Event) {
		if (softKeyboard.current) return;
		event.preventDefault();
		document.getElementById('plan-add-field')?.focus();
	}

	/** The field or its day chip has focus. */
	let typing = $state(false);
	/**
	 * A phone's search with its soft keyboard up: the sheet takes its full
	 * height, the title and the chips fold away, the day rides on a chip.
	 * By touch (`phone`), so a narrow desktop window keeps the chips.
	 */
	const searching = $derived(typing && narrow.current && phone.value);
	let fieldBlock = $state<HTMLElement>();
	let dayChip = $state<HTMLButtonElement>();
	/** The sheet at its full height: while searching and until the top has unfolded. */
	let held = $state(false);
	const pickedNames = $derived(picked === TRAY ? null : dayNames(picked, locale));

	/** Focus moving between the field and its day chip (Shift+Tab) keeps the fold. */
	function leave(event: FocusEvent) {
		const to = event.relatedTarget;
		if (to !== dayChip && to !== document.getElementById('plan-add-field')) typing = false;
	}

	/** The day chip: the keyboard goes, the chips come back on the picked day. */
	async function showDays() {
		typing = false;
		(document.activeElement as HTMLElement | null)?.blur();
		await tick();
		// Not scrolled into view: the chips are still unfolding, and their clip would scroll.
		document
			.querySelector<HTMLElement>('input[name="plan-add-day"]:checked')
			?.focus({ preventScroll: true });
	}

	const chip =
		'flex h-11 cursor-pointer items-center justify-center gap-1 rounded-pill border border-border bg-surface-elevated px-2 text-body-sm whitespace-nowrap text-text transition peer-checked:border-primary peer-checked:bg-primary peer-checked:text-primary-foreground peer-focus-visible:outline-2 peer-focus-visible:outline-offset-2 peer-focus-visible:outline-primary md:h-10';
	const row =
		'flex min-h-12 w-full cursor-pointer items-center gap-3 rounded-md px-2 py-1.5 text-left text-body-sm text-text';
</script>

{#snippet body()}
	<TypingFold shut={searching} field={fieldBlock} bind:held>
		<Dialog.Title class="font-display text-heading font-medium">{m.week_add()}</Dialog.Title>
		<fieldset class="mt-4 pb-5" inert={searching}>
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
	</TypingFold>
	<div bind:this={fieldBlock}>
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
			onfocus={() => (typing = true)}
			onblur={leave}
		>
			{#snippet aside()}
				{#if searching}
					<!-- The picked day, in the chips' picked style. A press keeps the
					     field's focus until the click moves it on: a blur at the press
					     would unfold the chips under the finger. Named by what it shows. -->
					<button
						bind:this={dayChip}
						type="button"
						onpointerdown={(event) => event.preventDefault()}
						onclick={showDays}
						onblur={leave}
						class="relative inline-flex h-8 items-center gap-1 rounded-pill border border-primary bg-primary px-3 text-caption text-primary-foreground transition after:absolute after:inset-x-0 after:-inset-y-2 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
					>
						<span class="sr-only">{m.plan_add_day_change()}</span>
						{#if pickedNames}
							<span class="font-semibold">{pickedNames.weekday}</span>
							<span class="tabular-nums opacity-80">{pickedNames.number}</span>
						{:else}
							<span class="font-semibold">{m.plan_tray()}</span>
						{/if}
						<CalendarDays class="ml-0.5 size-3.5" aria-hidden="true" />
					</button>
				{/if}
			{/snippet}
		</Input>
	</div>
	<!-- The box settles: it grows with the rows right away and shrinks only once they rest. -->
	<div
		{@attach settleHeight(
			() => pending,
			() => query
		)}
		class="overflow-y-clip transition-[height] duration-200 ease-out motion-reduce:transition-none"
	>
		<div>
			{#if shown.some((r) => r.recipe)}
				<p class="mt-4 mb-1 text-caption font-semibold text-text-muted" aria-hidden="true">
					{found?.query ? m.plan_add_results() : m.plan_add_recent()}
				</p>
			{/if}
			<!-- A press keeps the field's focus: a blur would unfold the sheet's
			     top under the finger and move the row away before the click. -->
			<ul
				id="plan-add-list"
				role="listbox"
				aria-label={m.plan_add_results()}
				aria-busy={pending}
				onpointerdown={(event) => event.preventDefault()}
				class="-mx-2 pb-1 {shown.some((r) => r.recipe) ? '' : 'mt-3'}"
			>
				{#each shown as option (option.id)}
					{@const i = rows.indexOf(option)}
					<!-- The keys are the field's (arrows, Enter): an option of a combobox takes no focus. -->
					<!-- svelte-ignore a11y_click_events_have_key_events -->
					<li
						id={option.id}
						role="option"
						aria-selected={i === current}
						aria-disabled={i < 0}
						onclick={() => take(option)}
						onpointermove={() => (active = i)}
						style:animation-fill-mode="backwards"
						class="{row} transition-opacity {option.recipe ? 'animate-fade-in' : ''} {i < 0
							? STALE_ROW
							: ''} {i === current ? 'bg-background' : ''}"
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
							<span class="min-w-0 break-words">{m.plan_add_note({ text: option.note ?? '' })}</span
							>
						{/if}
					</li>
				{/each}
			</ul>
			{#if query.trim() && found?.query === query.trim() && results.length === 0}
				<p class="pt-1 text-caption text-text-muted">{m.plan_add_none()}</p>
			{/if}
		</div>
	</div>
{/snippet}

<ResponsiveDialog
	bind:open
	closeLabel={m.common_cancel()}
	onOpenAutoFocus={focusField}
	bodyClass="pb-6"
	fill={held}
	top
	{body}
/>
