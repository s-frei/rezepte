<script lang="ts">
	import Check from '@lucide/svelte/icons/check';
	import ChevronDown from '@lucide/svelte/icons/chevron-down';
	import X from '@lucide/svelte/icons/x';
	import { onMount } from 'svelte';
	import { session } from '#lib/auth.svelte.js';
	import FaceStack from '#lib/components/ui/FaceStack.svelte';
	import PersonMark from '#lib/components/ui/PersonMark.svelte';
	import ListGroups from '#lib/components/shopping/ListGroups.svelte';
	import ShoppingRow from '#lib/components/shopping/ShoppingRow.svelte';
	import ViewSwitch from '#lib/components/shopping/ViewSwitch.svelte';
	import { ScreenWakeLock } from '#lib/recipe/wake-lock.svelte.js';
	import { listHref } from '#lib/shopping/href.js';
	import { failList } from '#lib/shopping/fail.js';
	import { ListState } from '#lib/shopping/list-state.svelte.js';
	import { listViews } from '#lib/shopping/rows.js';
	import { listView } from '#lib/shopping/view.svelte.js';
	import { m } from '#lib/paraglide/messages.js';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	// As on the list page: the server's answer replaces the list, live changes refetch it.
	// svelte-ignore state_referenced_locally
	const lst = new ListState(data.list, failList);
	lst.connect(() => data.list);
	// `shown`: what another device just ticked stays in place for a moment.
	const list = $derived(lst.shown);

	const { departments, recipes, done, openRows, doneRows } = $derived(
		listViews(list, listView.current)
	);
	// Everyone on the list but the viewer, whose own face stands apart.
	const others = $derived(
		[list.owner, ...list.members].filter((person) => person.id !== session.user?.id)
	);
	const progress = $derived((doneRows / Math.max(1, openRows + doneRows)) * 100);
	let doneOpen = $state(false);

	const wakeLock = new ScreenWakeLock();

	onMount(() => wakeLock.hold());

	const column = 'mx-auto w-full max-w-[720px] px-5';
</script>

<svelte:head><title>{list.name} · {m.list_shop_title()}</title></svelte:head>

<!-- S1: the list without the app's bars; the top stays, the rows scroll below it. -->
<main aria-label={m.list_shop_title()} class="flex h-dvh flex-col bg-surface text-text">
	<header class="shrink-0 pt-4">
		<div class={column}>
			<!-- Three columns, the outer two equal while they fit, so the name
			     stays centered between the close button and the faces; the button sits
			     on the title's first line however many lines the title takes. -->
			<div class="grid grid-cols-[1fr_minmax(0,auto)_1fr] items-start gap-3">
				<a
					href={listHref(list.id)}
					aria-label={m.list_shop_close()}
					class="relative mt-[calc((1lh_-_2.5rem)/2)] inline-flex size-10 shrink-0 items-center justify-center rounded-full border border-border bg-background hover-tint text-card text-text transition after:absolute after:-inset-[3px] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
				>
					<X class="size-5" aria-hidden="true" />
				</a>
				<div class="min-w-0 text-center">
					<h1 class="line-clamp-2 font-display text-card font-medium wrap-break-word hyphens-auto">
						{list.name}
					</h1>
					<p class="text-caption font-semibold text-text-muted">
						{m.list_counts({ open: openRows, done: doneRows })}
					</p>
				</div>
				<!-- Under whose account the ticks go: the viewer's own face at the
				     end, ringed like the top bar's, on a shared list the others'
				     before it. Faces only, no color on the items. -->
				<div class="mt-[calc((1lh_-_1.75rem)/2)] flex items-center justify-end gap-2.5 text-card">
					{#if others.length > 0}
						<FaceStack people={others} slots={3} size="sm" ring="ring-surface" />
						<span class="sr-only"
							>{m.lists_card_shared_with({
								names: others.map((p) => p.displayName).join(', ')
							})}</span
						>
					{/if}
					{#if session.user}
						<span role="img" aria-label={m.list_shop_as({ name: session.user.displayName })}>
							<PersonMark person={session.user} size="sm" ringed />
						</span>
					{/if}
				</div>
			</div>
			<div class="mt-3 h-1 overflow-hidden rounded-full bg-border" aria-hidden="true">
				<div class="h-full rounded-full bg-primary transition-all" style:width="{progress}%"></div>
			</div>
			<ViewSwitch class="mt-3.5" />
		</div>
	</header>

	<!--
		The rows fade out at both ends instead of meeting the top and the
		screen's foot at a hard edge; the padding keeps every row clear of the
		fades once scrolled to that end.
	-->
	<div
		class="min-h-0 flex-1 overflow-y-auto [mask-image:linear-gradient(to_bottom,transparent,black_1.5rem,black_calc(100%-2.5rem),transparent)] pt-3 pb-10"
	>
		<div class={column}>
			<ListGroups
				{departments}
				{recipes}
				checkers={list.members.length > 0}
				shop
				onToggle={(row) => lst.toggle(row)}
			/>

			{#if done.length > 0}
				<section class="mt-5">
					<button
						type="button"
						aria-expanded={doneOpen}
						onclick={() => (doneOpen = !doneOpen)}
						class="flex min-h-13 w-full items-center justify-between gap-2 rounded-xl bg-background hover-tint px-4 text-body-sm font-semibold text-text-muted transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
					>
						<span class="inline-flex items-center gap-2">
							<Check class="size-4" aria-hidden="true" />
							{m.list_shop_done({ count: done.length })}
						</span>
						<ChevronDown
							class="size-[18px] transition {doneOpen ? 'rotate-180' : ''}"
							aria-hidden="true"
						/>
					</button>
					{#if doneOpen}
						<ul class="mt-1">
							{#each done as row (row.key)}
								<ShoppingRow
									checkers={list.members.length > 0}
									{row}
									shop
									onToggle={(row) => lst.toggle(row)}
								/>
							{/each}
						</ul>
					{/if}
				</section>
			{/if}
		</div>
	</div>

	<!-- Nothing is added here: the list is kept before, shopping mode is for ticking. -->
	{#if wakeLock.active}
		<footer class="shrink-0 pt-2 pb-[max(0.75rem,env(safe-area-inset-bottom))]">
			<p class="text-center text-label tracking-normal text-text-muted">
				{m.list_shop_awake()}
			</p>
		</footer>
	{/if}
</main>
