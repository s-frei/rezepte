<script lang="ts">
	import { afterNavigate, goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import type { ResolvedPathname } from '$app/types';
	import { Command, Dialog } from 'bits-ui';
	import Plus from '@lucide/svelte/icons/plus';
	import Search from '@lucide/svelte/icons/search';
	import Settings from '@lucide/svelte/icons/settings';
	import { prefersReducedMotion } from 'svelte/motion';
	import { MediaQuery } from 'svelte/reactivity';
	import { fade, fly, scale } from 'svelte/transition';
	import { listRecipes, type RecipeCard } from '$lib/api/recipes';
	import { m } from '$lib/paraglide/messages';
	import { palette } from '$lib/palette.svelte';

	let query = $state('');
	let results = $state<RecipeCard[]>([]);
	let searching = $state(true);

	const actions = [
		{
			id: 'new',
			label: m.overview_new_recipe(),
			icon: Plus,
			href: resolve('/recipes/new')
		},
		{
			id: 'settings',
			label: m.nav_settings(),
			icon: Settings,
			href: resolve('/settings')
		}
	];

	// 150ms after the last keystroke; an in-flight request for an older
	// query is aborted so a slow response can never overwrite a newer one.
	// With an empty query the eight most recently updated recipes show.
	$effect(() => {
		if (!palette.open) {
			return;
		}
		const q = query.trim();
		const controller = new AbortController();
		// Set before the debounce, not inside it: between a keystroke and the
		// request that answers it the old results are stale, and the empty
		// state would otherwise flash for 150ms on every open.
		searching = true;
		const timer = setTimeout(async () => {
			try {
				const page = await listRecipes({ q, limit: 8 }, { signal: controller.signal });
				results = page.items;
			} catch (error) {
				if (!(error instanceof DOMException && error.name === 'AbortError')) {
					results = [];
				}
			} finally {
				if (!controller.signal.aborted) {
					searching = false;
				}
			}
		}, 150);
		return () => {
			clearTimeout(timer);
			controller.abort();
		};
	});

	// Closing clears the search so the next ⌘K starts fresh.
	$effect(() => {
		if (!palette.open) {
			query = '';
			results = [];
		}
	});

	afterNavigate(() => {
		palette.open = false;
	});

	// Below `md` the palette is a sheet: the phone's bottom nav opens it as
	// its search, and a dialog floating at 12vh would sit under the software
	// keyboard. The sheet starts near the top so the field stays above it.
	const phone = new MediaQuery('max-width: 767.98px');
	let input = $state<HTMLInputElement | null>(null);

	// Opening focuses the field, not the first focusable element - on a phone
	// that is the sheet's grip, and a field without focus brings up no
	// keyboard.
	function focusField(event: Event) {
		event.preventDefault();
		input?.focus();
	}

	// The software keyboard shrinks the visual viewport, not the layout one a
	// `fixed` sheet measures from, so the sheet's bottom follows the keyboard's
	// top: the last results and "all results" stay reachable above it.
	//
	// Where `bottom: 0` lands is measured, not assumed: Chrome for Android
	// can anchor fixed elements to a box that differs from `innerHeight`, and
	// an inset computed from `innerHeight` left the sheet floating a few
	// millimeters above the keyboard. The probe is a zero-height fixed line at
	// `bottom: 0` with no transition, so the sheet's own fly-in cannot skew
	// the reading; both it and the visual viewport report layout coordinates.
	//
	// Lifted off the screen edge, the sheet is a card resting on the keyboard,
	// so it rounds its bottom corners too; its list fades out over its last
	// 1.5rem either way, since a list cut off at a straight edge reads as
	// broken rather than as scrollable.
	let keyboardInset = $state(0);
	let floorProbe = $state<HTMLElement>();
	$effect(() => {
		const viewport = window.visualViewport;
		const probe = floorProbe;
		if (!palette.open || !phone.current || !viewport || !probe) return;
		const follow = () => {
			const floor = probe.getBoundingClientRect().top;
			keyboardInset = Math.max(0, Math.round(floor - viewport.offsetTop - viewport.height));
		};
		follow();
		viewport.addEventListener('resize', follow);
		viewport.addEventListener('scroll', follow);
		// The toolbar moving resizes the window without always resizing the
		// visual viewport, and it moves the floor the probe measures.
		window.addEventListener('resize', follow);
		return () => {
			viewport.removeEventListener('resize', follow);
			viewport.removeEventListener('scroll', follow);
			window.removeEventListener('resize', follow);
			keyboardInset = 0;
		};
	});
	const motion = $derived(prefersReducedMotion.current ? 0 : phone.current ? 250 : 200);

	// Hands the query to the overview, which has the filters, the tags and
	// every result rather than eight.
	const allResultsHref = $derived(resolve(`/?${new URLSearchParams({ q: query.trim() })}`));

	function go(href: ResolvedPathname) {
		palette.open = false;
		void goto(href);
	}

	function paletteIn(node: Element, params: { duration: number }) {
		return phone.current
			? fly(node, { duration: params.duration, y: 200 })
			: scale(node, { duration: params.duration, start: 0.95 });
	}

	// The highlight is the other of the two warm tones: the dialog is surface,
	// so its rows light up in background, and the sheet is background, where
	// the same tone would vanish, so its rows light up in surface.
	const item = $derived(
		`flex h-11 cursor-default items-center gap-3 rounded-md px-3 text-body text-text outline-none ${phone.current ? 'data-selected:bg-surface' : 'data-selected:bg-background'}`
	);
	const heading = 'px-3 pt-3 pb-1 text-caption font-semibold text-text-muted';
</script>

<Dialog.Root bind:open={palette.open}>
	<Dialog.Portal>
		{#if palette.open && phone.current}
			<span
				bind:this={floorProbe}
				aria-hidden="true"
				class="pointer-events-none fixed inset-x-0 bottom-0 h-0"
			></span>
		{/if}
		<Dialog.Overlay forceMount>
			{#snippet child({ props, open: isOpen })}
				{#if isOpen}
					<div
						{...props}
						class="fixed inset-0 z-40 bg-overlay"
						transition:fade={{ duration: 200 }}
					></div>
				{/if}
			{/snippet}
		</Dialog.Overlay>
		<Dialog.Content forceMount preventScroll={false} onOpenAutoFocus={focusField}>
			{#snippet child({ props, open: isOpen })}
				{#if isOpen}
					<div
						{...props}
						class={phone.current
							? `fixed inset-x-0 top-10 bottom-0 z-50 flex flex-col bg-background px-3 shadow-sheet ${keyboardInset > 0 ? 'rounded-3xl' : 'rounded-t-3xl'}`
							: 'fixed top-[12vh] left-1/2 z-50 w-[calc(100%-2.5rem)] max-w-[600px] -translate-x-1/2 rounded-3xl bg-surface p-3 shadow-dialog'}
						style:bottom={phone.current ? `${keyboardInset}px` : undefined}
						transition:paletteIn={{ duration: motion }}
					>
						<Dialog.Title class="sr-only">{m.palette_title()}</Dialog.Title>
						<Dialog.Description class="sr-only">{m.palette_description()}</Dialog.Description>
						{#if phone.current}
							<Dialog.Close
								aria-label={m.common_close()}
								class="flex w-full shrink-0 justify-center pt-2 pb-3 focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-primary"
							>
								<span class="h-1 w-9 rounded-pill bg-handle" aria-hidden="true"></span>
							</Dialog.Close>
						{/if}
						<Command.Root
							shouldFilter={false}
							loop
							label={m.palette_title()}
							class={phone.current ? 'flex min-h-0 flex-1 flex-col' : ''}
						>
							<div class="flex items-center gap-3 border-b border-dashed border-border px-3 pb-3">
								<Search class="size-5 shrink-0 text-text-muted" aria-hidden="true" />
								<Command.Input
									bind:ref={input}
									bind:value={query}
									placeholder={m.palette_placeholder()}
									class="h-10 min-w-0 flex-1 bg-transparent text-body-lg outline-none placeholder:text-text-muted"
								/>
							</div>
							<Command.List
								class="mt-2 overflow-y-auto {phone.current
									? 'min-h-0 flex-1 mask-b-from-[calc(100%-1.5rem)] pb-6'
									: 'max-h-[60vh]'}"
							>
								<Command.Viewport>
									<Command.Group>
										<Command.GroupHeading class={heading}
											>{m.palette_group_recipes()}</Command.GroupHeading
										>
										<Command.GroupItems>
											{#each results as recipe (recipe.id)}
												<Command.Item
													value="recipe:{recipe.id}"
													onSelect={() => go(resolve('/recipes/[slug]', { slug: recipe.slug }))}
													class={item}
												>
													{#if recipe.coverImageId}
														<img
															src="/images/{recipe.id}/{recipe.coverImageId}/thumb.jpg"
															alt=""
															class="size-8 shrink-0 rounded-sm object-cover"
														/>
													{:else}
														<span
															aria-hidden="true"
															class="flex size-8 shrink-0 items-center justify-center rounded-sm bg-accent font-display text-accent-foreground italic"
														>
															{recipe.title.charAt(0).toUpperCase()}
														</span>
													{/if}
													<span class="truncate">{recipe.title}</span>
												</Command.Item>
											{/each}
											<!-- Offered only while there is something to hand over: next
												to "No recipes found" it would promise an overview that
												turns out empty. -->
											{#if query.trim() && (results.length > 0 || searching)}
												<Command.Item
													value="all-results"
													onSelect={() => go(allResultsHref)}
													class={item}
												>
													<span
														aria-hidden="true"
														class="flex size-8 shrink-0 items-center justify-center rounded-sm bg-accent text-accent-foreground"
													>
														<Search class="size-4" />
													</span>
													<span class="truncate"
														>{m.palette_all_results({ query: query.trim() })}</span
													>
												</Command.Item>
											{/if}
											{#if results.length === 0 && !searching}
												<p class="px-3 py-2 text-body-sm text-text-muted">
													{query.trim() ? m.palette_no_recipes() : m.overview_empty_title()}
												</p>
											{/if}
										</Command.GroupItems>
									</Command.Group>
									<Command.Group>
										<Command.GroupHeading class={heading}
											>{m.palette_group_actions()}</Command.GroupHeading
										>
										<Command.GroupItems>
											{#each actions as action (action.id)}
												{@const Icon = action.icon}
												<Command.Item
													value="action:{action.id}"
													onSelect={() => go(action.href)}
													class={item}
												>
													<span
														aria-hidden="true"
														class="flex size-8 shrink-0 items-center justify-center rounded-sm bg-accent text-accent-foreground"
													>
														<Icon class="size-4" />
													</span>
													<span class="truncate">{action.label}</span>
												</Command.Item>
											{/each}
										</Command.GroupItems>
									</Command.Group>
								</Command.Viewport>
							</Command.List>
						</Command.Root>
					</div>
				{/if}
			{/snippet}
		</Dialog.Content>
	</Dialog.Portal>
</Dialog.Root>
