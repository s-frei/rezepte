<script lang="ts">
	import { afterNavigate, goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import type { ResolvedPathname } from '$app/types';
	import { Command, Dialog } from 'bits-ui';
	import Plus from '@lucide/svelte/icons/plus';
	import Search from '@lucide/svelte/icons/search';
	import Settings from '@lucide/svelte/icons/settings';
	import { fade, scale } from 'svelte/transition';
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

	function go(href: ResolvedPathname) {
		palette.open = false;
		void goto(href);
	}

	const item =
		'flex h-11 cursor-default items-center gap-3 rounded-md px-3 text-body text-text outline-none data-selected:bg-background';
	const heading = 'px-3 pt-2 pb-1 text-label font-bold text-text-muted uppercase';
</script>

<Dialog.Root bind:open={palette.open}>
	<Dialog.Portal>
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
		<Dialog.Content forceMount preventScroll={false}>
			{#snippet child({ props, open: isOpen })}
				{#if isOpen}
					<div
						{...props}
						class="fixed top-[12vh] left-1/2 z-50 w-[calc(100%-2.5rem)] max-w-[600px] -translate-x-1/2 rounded-3xl bg-surface p-3 shadow-dialog"
						transition:scale={{ duration: 200, start: 0.95 }}
					>
						<Dialog.Title class="sr-only">{m.palette_title()}</Dialog.Title>
						<Dialog.Description class="sr-only">{m.palette_description()}</Dialog.Description>
						<Command.Root shouldFilter={false} loop label={m.palette_title()}>
							<div class="flex items-center gap-3 border-b border-dashed border-border px-3 pb-3">
								<Search class="size-5 shrink-0 text-text-muted" aria-hidden="true" />
								<Command.Input
									bind:value={query}
									placeholder={m.palette_placeholder()}
									class="h-10 min-w-0 flex-1 bg-transparent text-body-lg outline-none placeholder:text-text-muted"
								/>
							</div>
							<Command.List class="mt-2 max-h-[60vh] overflow-y-auto">
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
