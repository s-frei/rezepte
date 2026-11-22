<script lang="ts">
	import House from '@lucide/svelte/icons/house';
	import Plus from '@lucide/svelte/icons/plus';
	import Search from '@lucide/svelte/icons/search';
	import ChevronUp from '@lucide/svelte/icons/chevron-up';
	import X from '@lucide/svelte/icons/x';
	import { Dialog } from 'bits-ui';
	import { tick, untrack } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { isSignedOut } from '#lib/api/client.js';
	import { listIngredients, type IngredientSuggestion } from '#lib/api/ingredients.js';
	import { getStaples, setStaple, type Staple } from '#lib/api/mealplan.js';
	import Button from '#lib/components/ui/Button.svelte';
	import ResponsiveDialog from '#lib/components/ui/ResponsiveDialog.svelte';
	import TypingFold from '#lib/components/ui/TypingFold.svelte';
	import { narrow } from '#lib/ui/narrow.js';
	import { phone } from '#lib/ui/phone.svelte.js';
	import { settleHeight, STALE_ROW } from '#lib/ui/settle-height.js';
	import { m } from '#lib/paraglide/messages.js';
	import { ingredientKey } from '#lib/recipe/key.js';

	/**
	 * "Haben wir immer" from a plan's ⋯: the plan's staples, each with a remove
	 * button, and "Zutat hinzufügen", which unfolds an ingredient search in
	 * place. The house in "Auf die Liste" and on a list item from this plan
	 * mark them too; nothing here touches another plan.
	 */
	let { open = $bindable(false), planId }: { open?: boolean; planId: string } = $props();

	type State = { kind: 'loading' } | { kind: 'failed' } | { kind: 'ready'; staples: Staple[] };
	let view = $state<State>({ kind: 'loading' });
	let busy = $state('');

	// Read afresh on every opening, search folded: the house elsewhere changes them.
	$effect(() => {
		if (!open) return;
		untrack(() => {
			searching = false;
			text = '';
			void load();
		});
	});

	let req = 0;
	async function load() {
		const mine = ++req;
		view = { kind: 'loading' };
		try {
			const { staples } = await getStaples(planId);
			if (mine === req) view = { kind: 'ready', staples };
		} catch (error) {
			if (mine !== req) return;
			view = { kind: 'failed' };
			if (!isSignedOut(error)) toast.error(m.plan_staples_error());
		}
	}

	/** The plan's staples by key, for "Schon dabei" with the name as stored. */
	const stored = $derived(
		new Map(view.kind === 'ready' ? view.staples.map((s) => [s.nameKey, s.name]) : [])
	);

	/** Takes `staple` off; the toast brings it back. */
	async function remove(staple: Staple, index: number) {
		if (busy) return;
		busy = staple.nameKey;
		try {
			const { staples } = await setStaple(planId, staple.name, false);
			req++;
			view = { kind: 'ready', staples };
			toast.success(m.plan_staples_removed({ name: staple.name }), {
				action: { label: m.common_undo(), onClick: () => void restore(staple.name) }
			});
			// The keyboard stays where it was: in the search while it is open (a
			// phone keeps its keyboard), else in the list - the row that moved up,
			// else the one above, else Done. The buttons are enabled again first;
			// a disabled one takes no focus.
			busy = '';
			await tick();
			if (searching) {
				input?.focus();
				return;
			}
			const buttons = document.querySelectorAll<HTMLElement>('[data-staple-remove]');
			(
				buttons[Math.min(index, buttons.length - 1)] ??
				document.querySelector<HTMLElement>('[data-staples-done]')
			)?.focus();
		} catch (error) {
			if (!isSignedOut(error)) toast.error(m.item_sheet_staple_error());
		} finally {
			busy = '';
		}
	}

	async function restore(name: string) {
		try {
			const { staples } = await setStaple(planId, name, true);
			if (open) {
				req++;
				view = { kind: 'ready', staples };
			}
		} catch (error) {
			if (!isSignedOut(error)) toast.error(m.item_sheet_staple_error());
		}
	}

	// The search: folded until "Zutat hinzufügen" asks for it.
	let searching = $state(false);
	let text = $state('');
	let input = $state<HTMLInputElement>();
	let addButton = $state<HTMLButtonElement>();
	/** On a phone the footer gives way while the field has focus, so the names are never behind "Fertig". */
	let typing = $state(false);

	/**
	 * A phone's search with its soft keyboard up: the sheet takes its full
	 * height and the title and its text fold away (`TypingFold`), so the
	 * field heads the sheet and stays put while the names change under it.
	 */
	const onPhone = $derived(typing && narrow.current && phone.value);
	/** The sheet at its full height: while searching and until the top has unfolded. */
	let held = $state(false);
	/** What a pick or a fold says to a screen reader. */
	let said = $state('');
	/** The server's last answer with the text it answers; `fresh` tells whether it answers the field's. */
	let suggested = $state<{ q: string; items: IngredientSuggestion[] }>({ q: '', items: [] });
	const query = $derived(text.trim());

	$effect(() => {
		const q = query.toLocaleLowerCase();
		if (!searching || !q) return;
		const controller = new AbortController();
		const timer = setTimeout(() => {
			listIngredients(q, 6, { signal: controller.signal }).then(
				(items) => (suggested = { q, items }),
				() => {
					if (!controller.signal.aborted) suggested = { q, items: [] };
				}
			);
		}, 150);
		return () => {
			clearTimeout(timer);
			controller.abort();
		};
	});

	type Option = { key: string; name: string; typed: boolean };
	const typed = $derived<Option | null>(
		ingredientKey(query) ? { key: ingredientKey(query), name: query, typed: true } : null
	);
	/** Whether the names shown answer the text in the field. */
	const fresh = $derived(suggested.q === query.toLocaleLowerCase());
	/** A keystroke whose answer is not in; nothing is, without text. */
	const pending = $derived(!!typed && !fresh);
	/**
	 * The names in use, then what was typed unless one of them is it already
	 * (by key). Until the answer for this text is in, the last answer's names
	 * that still hold the text stay, dimmed and out of reach, above the typed
	 * row: the list keeps its size between keystrokes instead of emptying and
	 * filling on each. A name that cannot match any more goes.
	 */
	const rows = $derived.by((): Option[] => {
		if (!typed) return [];
		const q = query.toLocaleLowerCase();
		const out: Option[] = suggested.items
			.filter((i) => fresh || i.name.toLocaleLowerCase().includes(q))
			.map((i) => ({ key: i.key, name: i.name, typed: false }));
		if (!out.some((o) => o.key === typed.key)) out.push(typed);
		return out;
	});
	/** What the keys and Enter reach: every row once fresh, else only the typed one, so the rows never shift under a highlight. */
	const options = $derived(fresh ? rows : rows.filter((o) => o.typed));
	let highlighted = $state<string | null>(null);
	/**
	 * What Enter takes: the highlighted option, else the one with the typed
	 * key (in already, it says so), else the first one not in yet. "Salt"
	 * with salt in must never take "salt to taste".
	 */
	const current = $derived(
		options.find((o) => o.key === highlighted) ??
			options.find((o) => o.key === typed?.key) ??
			options.find((o) => !stored.has(o.key))
	);

	async function unfold() {
		searching = true;
		said = '';
		await tick();
		input?.focus();
	}

	async function fold() {
		searching = false;
		text = '';
		highlighted = null;
		said = m.plan_staples_search_closed();
		await tick();
		addButton?.focus();
	}

	async function add(option: Option) {
		const have = stored.get(option.key);
		if (have !== undefined) {
			said = m.plan_staples_already({ name: have });
			return;
		}
		if (busy) return;
		busy = option.key;
		try {
			const { staples } = await setStaple(planId, option.name, true);
			req++;
			view = { kind: 'ready', staples };
			said = m.plan_staples_added({ name: option.name });
			// Room for the next one: the household's first visit adds several.
			text = '';
			highlighted = null;
		} catch (error) {
			if (!isSignedOut(error)) toast.error(m.item_sheet_staple_error());
		} finally {
			busy = '';
			input?.focus();
		}
	}

	/**
	 * A press on a button while the field has focus keeps it there: the
	 * keyboard stays up, and the footer it hides does not come back under the
	 * finger between press and release, which moved the button away.
	 */
	function keepField(event: PointerEvent) {
		if (typing) event.preventDefault();
	}

	/** Escape anywhere in the search, its close button too, folds it; the next Escape closes the sheet. */
	function escape(event: KeyboardEvent) {
		if (event.key !== 'Escape') return;
		event.preventDefault();
		event.stopPropagation();
		void fold();
	}

	function keydown(event: KeyboardEvent) {
		if (event.key === 'Enter') {
			event.preventDefault();
			// Before the names are in, Enter takes the text as typed.
			// With every row in already, the first says so.
			const pick = options.length > 0 ? (current ?? options[0]) : typed;
			if (pick) void add(pick);
		} else if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
			event.preventDefault();
			if (options.length === 0) return;
			const at = current ? options.indexOf(current) : -1;
			const step = event.key === 'ArrowDown' ? 1 : -1;
			highlighted = options[(at + step + options.length) % options.length].key;
		}
	}

	const optionId = (key: string) => `staple-option-${encodeURIComponent(key)}`;
</script>

{#snippet body()}
	<TypingFold shut={onPhone} field={input} bind:held>
		<Dialog.Title class="font-display text-heading font-medium">{m.plan_staple()}</Dialog.Title>
		<Dialog.Description class="mt-2 pb-5 text-body-sm text-text-muted">
			{m.plan_staples_text()}
		</Dialog.Description>
	</TypingFold>

	<div aria-busy={view.kind === 'loading'}>
		{#if view.kind === 'loading'}
			<div class="h-14 animate-pulse rounded-2xl bg-surface-elevated"></div>
		{:else if view.kind === 'failed'}
			<button
				type="button"
				onclick={load}
				class="flex min-h-11 w-full items-center justify-between rounded-md border border-border bg-surface-elevated px-4 py-1.5 text-left text-body-sm text-text-muted focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
			>
				<span class="min-w-0 text-caption leading-tight">{m.plan_staples_error()}</span>
				<span class="shrink-0 pl-3 font-semibold whitespace-nowrap text-primary"
					>{m.add_to_list_retry()}</span
				>
			</button>
		{:else}
			<!-- One frame: the way to add one first, which unfolds into the search
			     with its names hanging under it, then the staples. First, so the
			     field stays near the top however long the list, and a phone's
			     keyboard leaves its names in view. -->
			<div class="rounded-2xl border border-border bg-surface-elevated px-1.5 py-1">
				{#if !searching}
					<button
						bind:this={addButton}
						type="button"
						onclick={unfold}
						class="flex min-h-11 w-full items-center gap-3 rounded-lg hover-tint pl-2.5 text-left text-body-sm font-semibold text-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
					>
						<Plus class="size-4 shrink-0" aria-hidden="true" />
						{m.plan_staples_add()}
					</button>
				{:else}
					<!-- Escape anywhere in here, the close button too, folds the search. -->
					<!-- svelte-ignore a11y_no_static_element_interactions -->
					<div onkeydown={escape}>
						<div class="flex min-h-11 items-center gap-3 pl-2.5">
							<Search class="size-4 shrink-0 text-text-muted" aria-hidden="true" />
							<input
								bind:this={input}
								bind:value={text}
								onkeydown={keydown}
								onfocus={() => (typing = true)}
								onblur={() => (typing = false)}
								type="text"
								role="combobox"
								aria-label={m.plan_staples_search()}
								aria-expanded={options.length > 0}
								aria-controls="staple-options"
								aria-autocomplete="list"
								aria-activedescendant={current ? optionId(current.key) : undefined}
								placeholder={m.plan_staples_search_placeholder()}
								autocomplete="off"
								enterkeyhint="done"
								maxlength="200"
								class="h-11 min-w-0 flex-1 scroll-mt-4 bg-transparent text-input text-text outline-none placeholder:text-text-muted"
							/>
							<button
								type="button"
								aria-label={m.plan_staples_search_close()}
								onpointerdown={keepField}
								onclick={() => void fold()}
								class="flex size-11 shrink-0 items-center justify-center rounded-full hover-tint text-text-muted transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary md:size-10"
							>
								<ChevronUp class="size-4" aria-hidden="true" />
							</button>
						</div>
						<!-- A press keeps the focus in the field, so the keyboard stays up for the next name.
						     The box settles: it grows with the names at once and shrinks only once they rest. -->
						<div
							{@attach settleHeight(
								() => pending,
								() => text
							)}
							class="overflow-y-clip transition-[height] duration-200 ease-out motion-reduce:transition-none"
						>
							<div
								id="staple-options"
								role="listbox"
								tabindex="-1"
								aria-label={m.plan_staples_search()}
								aria-busy={pending}
								onpointerdown={(event) => event.preventDefault()}
								class={rows.length > 0 ? 'border-t border-border pt-1' : ''}
							>
								<!-- The typed row keyed by '' (no name's key): it changes its text in place. -->
								{#each rows as option (option.typed ? '' : option.key)}
									{@const have = stored.get(option.key)}
									{@const stale = !options.includes(option)}
									<!-- svelte-ignore a11y_click_events_have_key_events -->
									<div
										id={optionId(option.key)}
										role="option"
										tabindex="-1"
										aria-selected={current === option}
										aria-disabled={stale || have !== undefined || busy !== ''}
										onclick={() => void add(option)}
										style:animation-fill-mode="backwards"
										class="flex min-h-11 items-center gap-3 rounded-lg pr-3 pl-2.5 text-body transition-opacity {option.typed
											? ''
											: 'animate-fade-in'} {stale ? STALE_ROW : ''} {have !== undefined
											? 'text-text-muted'
											: 'cursor-pointer hover:bg-popover-highlight'} {current === option &&
										have === undefined
											? 'bg-popover-highlight'
											: ''}"
									>
										{#if option.typed && have === undefined}
											<Plus class="size-4 shrink-0 text-primary" aria-hidden="true" />
										{:else}
											<House
												class="size-4 shrink-0 {have !== undefined
													? 'text-primary'
													: 'text-text-muted'}"
												fill={have !== undefined ? 'currentColor' : 'none'}
												fill-opacity={have !== undefined ? 0.18 : 0}
												aria-hidden="true"
											/>
										{/if}
										<!-- One already in reads as stored: "Zwiebel" for a typed "Zwiebeln". -->
										<span class="min-w-0 flex-1 py-2 [overflow-wrap:anywhere]">
											{have ??
												(option.typed
													? m.plan_staples_add_typed({ name: option.name })
													: option.name)}
										</span>
										{#if have !== undefined}
											<span class="shrink-0 text-caption">{m.plan_staples_have()}</span>
										{/if}
									</div>
								{/each}
							</div>
						</div>
					</div>
				{/if}
				{#if view.staples.length > 0}
					<ul class="border-t border-dashed border-border">
						{#each view.staples as staple, i (staple.nameKey)}
							<li
								class="flex min-h-11 items-center gap-3 border-b border-dashed border-border pl-2.5 last:border-b-0"
							>
								<House
									class="size-4 shrink-0 text-primary"
									fill="currentColor"
									fill-opacity={0.18}
									aria-hidden="true"
								/>
								<span class="min-w-0 flex-1 py-2 text-body break-words">{staple.name}</span>
								<button
									type="button"
									data-staple-remove
									aria-label={m.plan_staples_remove({ name: staple.name })}
									aria-busy={busy === staple.nameKey}
									disabled={busy !== ''}
									onpointerdown={keepField}
									onclick={() => void remove(staple, i)}
									class="flex size-11 shrink-0 items-center justify-center rounded-full hover-tint text-text-muted transition hover:text-destructive focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary disabled:opacity-60 md:size-10"
								>
									<X class="size-4" aria-hidden="true" />
								</button>
							</li>
						{/each}
					</ul>
				{/if}
			</div>
			<!-- How else they get here, also once there are some: the sheet is where people tend the list. -->
			<p class="mt-3 flex items-start gap-2 px-1 text-caption text-text-muted">
				<House class="mt-px size-3.5 shrink-0" aria-hidden="true" />
				<span class="min-w-0">{m.plan_staples_more()}</span>
			</p>
		{/if}
		<p class="sr-only" aria-live="polite">{said}</p>
	</div>
{/snippet}

{#snippet footer()}
	<Button class="grow max-md:h-11 md:grow-0" data-staples-done onclick={() => (open = false)}
		>{m.item_sheet_done()}</Button
	>
{/snippet}

<ResponsiveDialog
	bind:open
	closeLabel={m.common_close()}
	fill={held}
	top
	{body}
	footer={narrow.current && searching && typing ? undefined : footer}
/>
