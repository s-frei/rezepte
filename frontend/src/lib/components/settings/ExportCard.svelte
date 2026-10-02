<script lang="ts">
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { toast } from 'svelte-sonner';
	import { imageUrl, listRecipes, type RecipeCard } from '$lib/api/recipes';
	import { exportRecipes, saveBlob } from '$lib/api/transfer';
	import { isSignedOut } from '$lib/api/client';
	import Button from '$lib/components/ui/Button.svelte';
	import Select from '$lib/components/ui/Select.svelte';
	import Skeleton from '$lib/components/ui/Skeleton.svelte';
	import TransferRow from './TransferRow.svelte';
	import { m } from '$lib/paraglide/messages';
	import { getLocale } from '$lib/paraglide/runtime';

	/** Every recipe here; bindable so the import card can spot titles already here. */
	let {
		recipes = $bindable([]),
		onready
	}: {
		recipes?: RecipeCard[];
		/** Called once every recipe has loaded; the import card waits for it. */
		onready?: () => void;
	} = $props();

	let loading = $state(true);
	let loadFailed = $state(false);
	let query = $state('');
	let sort = $state('title');
	let ticked = $state<Record<string, boolean>>({});
	let progress = $state<null | { loaded: number; total: number | null }>(null);

	const sortOptions = [
		{ value: 'title', label: m.settings_transfer_sort_title() },
		{ value: 'updated', label: m.settings_transfer_sort_updated() }
	];

	async function load() {
		loading = true;
		loadFailed = false;
		try {
			const all: RecipeCard[] = [];
			for (let page = 1; ; page++) {
				const res = await listRecipes({ page, limit: 100 });
				all.push(...res.items);
				if (all.length >= res.total || res.items.length === 0) break;
			}
			// Every row gets its key before it renders: binding a checkbox to an
			// undefined entry would trip the prop's fallback.
			ticked = Object.fromEntries(all.map((r) => [r.id, ticked[r.id] ?? false]));
			recipes = all;
			onready?.();
		} catch (error) {
			loadFailed = !isSignedOut(error);
		} finally {
			loading = false;
		}
	}
	onMount(load);

	const shown = $derived.by(() => {
		const q = query.trim().toLocaleLowerCase();
		const list = q
			? recipes.filter(
					(r) =>
						r.title.toLocaleLowerCase().includes(q) ||
						r.tags.some((t) => t.toLocaleLowerCase().includes(q))
				)
			: [...recipes];
		return list.sort((a, b) =>
			sort === 'title'
				? a.title.localeCompare(b.title, getLocale())
				: b.updatedAt.localeCompare(a.updatedAt)
		);
	});
	const selected = $derived(recipes.filter((r) => ticked[r.id]));
	const bytes = $derived(selected.reduce((sum, r) => sum + r.imageBytes, 0));
	// Whole megabytes, and never "~0 MB" for a pick that carries photos.
	const size = $derived(
		new Intl.NumberFormat(getLocale(), {
			style: 'unit',
			unit: 'megabyte',
			maximumFractionDigits: 0
		}).format(bytes ? Math.max(1, Math.round(bytes / 1e6)) : 0)
	);
	const percent = $derived(
		progress?.total ? Math.min(99, Math.floor((progress.loaded / progress.total) * 100)) : null
	);

	function tickAll(on: boolean) {
		for (const r of shown) ticked[r.id] = on;
	}

	async function run() {
		progress = { loaded: 0, total: null };
		try {
			const { blob, filename } = await exportRecipes(
				selected.map((r) => r.id),
				(loaded, total) => (progress = { loaded, total })
			);
			saveBlob(blob, filename);
		} catch (error) {
			if (!isSignedOut(error)) toast.error(m.settings_transfer_export_failed());
		} finally {
			progress = null;
		}
	}
</script>

<section class="rounded-2xl bg-surface p-6 md:p-7" aria-labelledby="transfer-export">
	<h2 id="transfer-export" class="font-display text-heading font-medium">
		{m.settings_transfer_export_title()}
	</h2>
	<p class="mt-1 mb-4 text-body-sm text-text-muted">{m.settings_transfer_export_hint()}</p>
	{#if loading}
		<Skeleton class="h-14 rounded-md" />
		<Skeleton class="mt-2 h-14 rounded-md" />
	{:else if loadFailed}
		<Button variant="secondary" onclick={load}>{m.common_retry()}</Button>
	{:else if recipes.length === 0}
		<p class="py-6 text-body-sm text-text-muted">{m.settings_transfer_empty()}</p>
	{:else}
		{@const searchLabel =
			recipes.length === 1
				? m.settings_transfer_search_one()
				: m.settings_transfer_search({ count: recipes.length })}
		<div class="mb-2 flex flex-wrap gap-2">
			<input
				type="search"
				bind:value={query}
				placeholder={searchLabel}
				aria-label={searchLabel}
				autocomplete="off"
				class="h-11 min-w-48 flex-1 rounded-pill border border-border bg-surface-elevated px-4 text-body-sm outline-none placeholder:text-text-muted focus:border-primary"
			/>
			<Select
				bind:value={sort}
				options={sortOptions}
				label={m.overview_sort_label()}
				class="rounded-pill bg-surface-elevated"
			/>
		</div>
		<!-- Bounded, so the import card below stays in reach (and the top bar's
		     #import link lands on it) however long the collection grows. -->
		<ul
			aria-labelledby="transfer-export"
			class="max-h-[60vh] [scrollbar-gutter:stable] overflow-y-auto overscroll-contain pr-3 md:pr-2"
		>
			{#each shown as r (r.id)}
				<TransferRow
					bind:checked={ticked[r.id]}
					title={r.title}
					tags={r.tags}
					thumb={r.coverImageId ? imageUrl(r.id, r.coverImageId, 'thumb') : null}
					href={resolve('/recipes/[slug]', { slug: r.slug })}
					hrefLabel={m.settings_transfer_open({ title: r.title })}
				>
					{#snippet trailing()}
						<span class="text-micro whitespace-nowrap text-text-muted">
							{r.imageCount === 1
								? m.settings_transfer_photos_one()
								: m.settings_transfer_photos({ count: r.imageCount })}
						</span>
					{/snippet}
				</TransferRow>
			{:else}
				<li class="py-6 text-body-sm text-text-muted">{m.settings_transfer_no_match()}</li>
			{/each}
		</ul>
		<div
			class="mt-3 flex flex-wrap items-center justify-between gap-3 border-t border-border pt-3 text-caption text-text-muted"
		>
			<span>
				<!-- No size before anything is ticked: "~0 MB" reads as a fact. -->
				{selected.length === 0
					? m.settings_transfer_import_summary({ ticked: 0, total: recipes.length })
					: m.settings_transfer_export_summary({
							ticked: selected.length,
							total: recipes.length,
							size
						})}
				·
				<button type="button" class="underline hover:text-text" onclick={() => tickAll(true)}>
					{m.settings_transfer_tick_all()}
				</button>
				·
				<button type="button" class="underline hover:text-text" onclick={() => tickAll(false)}>
					{m.settings_transfer_clear()}
				</button>
			</span>
			<Button variant="primary" disabled={selected.length === 0 || progress !== null} onclick={run}>
				{#if progress === null}
					{m.settings_transfer_export_button()}
				{:else if percent === null}
					{m.settings_transfer_preparing()}
				{:else}
					{m.settings_transfer_downloading({ percent })}
				{/if}
			</Button>
		</div>
	{/if}
</section>
