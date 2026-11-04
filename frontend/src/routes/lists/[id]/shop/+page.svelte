<script lang="ts">
	import Check from '@lucide/svelte/icons/check';
	import ChevronDown from '@lucide/svelte/icons/chevron-down';
	import Plus from '@lucide/svelte/icons/plus';
	import X from '@lucide/svelte/icons/x';
	import { onMount } from 'svelte';
	import EntryCover from '#lib/components/shopping/EntryCover.svelte';
	import ShoppingRow from '#lib/components/shopping/ShoppingRow.svelte';
	import ViewSwitch from '#lib/components/shopping/ViewSwitch.svelte';
	import { servingsUnit } from '#lib/recipe/format.js';
	import { ScreenWakeLock } from '#lib/recipe/wake-lock.svelte.js';
	import { departmentLabel } from '#lib/shopping/departments.js';
	import { listHref } from '#lib/shopping/href.js';
	import { failList } from '#lib/shopping/fail.js';
	import { ListState } from '#lib/shopping/list-state.svelte.js';
	import { followList } from '#lib/shopping/live.js';
	import { listViews } from '#lib/shopping/rows.js';
	import { listView } from '#lib/shopping/view.svelte.js';
	import { m } from '#lib/paraglide/messages.js';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	// As on the list page: the server's answer replaces the list, live changes refetch it.
	// svelte-ignore state_referenced_locally
	const lst = new ListState(data.list, failList);
	// `shown`: what another device just ticked stays in place for a moment.
	const list = $derived(lst.shown);
	$effect.pre(() => lst.load(data.list));
	const listId = $derived(data.list.id);
	$effect(() => followList(listId, (revision) => void lst.follow(revision)));

	const { departments, recipes, done, openRows, doneRows } = $derived(
		listViews(list, listView.current)
	);
	const progress = $derived((doneRows / Math.max(1, openRows + doneRows)) * 100);
	let doneOpen = $state(false);

	const wakeLock = new ScreenWakeLock();

	onMount(() => {
		void wakeLock.acquire();
		// The browser drops the lock when the tab is hidden; take it again on return.
		function onVisibilityChange() {
			if (document.visibilityState === 'visible') void wakeLock.acquire();
		}
		document.addEventListener('visibilitychange', onVisibilityChange);
		return () => {
			document.removeEventListener('visibilitychange', onVisibilityChange);
			void wakeLock.release();
		};
	});

	const column = 'mx-auto w-full max-w-[720px] px-5';
</script>

<svelte:head><title>{list.name} · {m.list_shop_title()}</title></svelte:head>

<!-- S1: the list without the app's bars; the top stays, the rows scroll below it. -->
<main aria-label={m.list_shop_title()} class="flex h-dvh flex-col bg-surface text-text">
	<header class="shrink-0 pt-4">
		<div class={column}>
			<div class="flex items-center gap-3">
				<a
					href={listHref(list.id)}
					aria-label={m.list_shop_close()}
					class="relative inline-flex size-10 shrink-0 items-center justify-center rounded-full border border-border bg-background hover-tint text-text transition after:absolute after:-inset-[3px] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
				>
					<X class="size-5" aria-hidden="true" />
				</a>
				<div class="min-w-0 flex-1 text-center">
					<h1 class="truncate font-display text-card font-medium">{list.name}</h1>
					<p class="text-caption font-semibold text-text-muted">
						{m.list_counts({ open: openRows, done: doneRows })}
					</p>
				</div>
				<span class="w-10 shrink-0" aria-hidden="true"></span>
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
			{#if listView.current === 'department'}
				{#each departments as group (group.department)}
					<section>
						<h2
							class="mt-4.5 mb-0.5 flex items-baseline justify-between gap-3 font-display text-heading font-medium text-primary italic"
						>
							{departmentLabel(group.department)}
							<span class="font-sans text-body-sm font-semibold text-text-muted not-italic"
								>{group.rows.length}</span
							>
						</h2>
						<ul>
							{#each group.rows as row (row.key)}
								<ShoppingRow
									checkers={list.members.length > 0}
									{row}
									shop
									onToggle={(row) => lst.toggle(row)}
								/>
							{/each}
						</ul>
					</section>
				{/each}
			{:else}
				{#each recipes as group (group.entry?.id ?? 'free')}
					{@const rows = group.rows.filter((r) => !r.checked)}
					{#if rows.length > 0}
						<section>
							<div class="mt-2 flex items-center gap-3 pt-4 pb-1">
								{#if group.entry}
									<EntryCover entry={group.entry} class="size-10 text-body-lg" />
									<div class="min-w-0 flex-1">
										<h2 class="font-display text-card font-medium text-pretty">
											{group.entry.title}
										</h2>
										<p class="text-caption text-text-muted">
											{group.entry.servings}
											{servingsUnit(group.entry.servings)}
										</p>
									</div>
								{:else}
									<span
										aria-hidden="true"
										class="flex size-10 shrink-0 items-center justify-center rounded-lg bg-accent text-accent-foreground"
									>
										<Plus class="size-[18px]" />
									</span>
									<h2 class="min-w-0 flex-1 font-display text-card font-medium">
										{m.list_free_items()}
									</h2>
								{/if}
							</div>
							<ul>
								{#each rows as row (row.key)}
									<ShoppingRow
										checkers={list.members.length > 0}
										{row}
										bySource
										shop
										onToggle={(row) => lst.toggle(row)}
									/>
								{/each}
							</ul>
						</section>
					{/if}
				{/each}
			{/if}

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
