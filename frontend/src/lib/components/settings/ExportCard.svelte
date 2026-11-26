<script lang="ts">
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { toast } from 'svelte-sonner';
	import FileText from '@lucide/svelte/icons/file-text';
	import {
		getRecipe,
		imageUrl,
		listRecipes,
		type Recipe,
		type RecipeCard
	} from '#lib/api/recipes.js';
	import { requestPdf } from '#lib/api/pdf.js';
	import type { Settings } from '#lib/api/settings.js';
	import { exportRecipes, saveBlob } from '#lib/api/transfer.js';
	import { isSignedOut } from '#lib/api/client.js';
	import { bulkPdfRequest } from '#lib/recipe/bulk-pdf.js';
	import { printOptions } from '#lib/recipe/print-options.svelte.js';
	import ExportPdfDialog from './ExportPdfDialog.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import Select from '#lib/components/ui/Select.svelte';
	import Skeleton from '#lib/components/ui/Skeleton.svelte';
	import TransferRow from './TransferRow.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';

	/** Every recipe here; bindable so the import card can spot titles already here. */
	let {
		recipes = $bindable([]),
		settings = null,
		onready
	}: {
		recipes?: RecipeCard[];
		/** The instance's settings: "As PDF" shows only where the service can render PDFs. */
		settings?: Settings | null;
		/** Called once every recipe has loaded; the import card waits for it. */
		onready?: () => void;
	} = $props();

	let loading = $state(true);
	let loadFailed = $state(false);
	let query = $state('');
	let sort = $state('title');
	let ticked = $state<Record<string, boolean>>({});
	// One export at a time; `kind` names the button that shows its progress.
	let progress = $state<null | { kind: 'zip' | 'pdf'; loaded: number; total: number | null }>(null);
	let pdfOpen = $state(false);
	// A text action in running text: an `::after` grows the target to 44px
	// tall without moving the line (RecipeColophon's trigger).
	const textAction =
		'relative rounded-sm after:absolute after:inset-x-0 after:top-1/2 after:h-11 after:-translate-y-1/2 underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary';

	const sortOptions = [
		{ value: 'title', label: m.settings_transfer_sort_title() },
		{ value: 'updated', label: m.settings_transfer_sort_updated() }
	];

	async function load() {
		loading = true;
		loadFailed = false;
		try {
			// By id: a recipe saved while the pages load pushes the last row of
			// one page onto the next, and a row twice breaks the keyed list.
			const byId: Record<string, RecipeCard> = {};
			for (let page = 1; ; page++) {
				const res = await listRecipes({ page, limit: 100 });
				for (const r of res.items) byId[r.id] = r;
				if (Object.keys(byId).length >= res.total || res.items.length === 0) break;
			}
			const all = Object.values(byId);
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
		return list.sort(compare);
	});
	/** The list's order, which the PDF follows too. */
	function compare(a: RecipeCard, b: RecipeCard) {
		return sort === 'title'
			? a.title.localeCompare(b.title, getLocale())
			: b.updatedAt.localeCompare(a.updatedAt);
	}
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
		progress = { kind: 'zip', loaded: 0, total: null };
		try {
			const { blob, filename } = await exportRecipes(
				selected.map((r) => r.id),
				(loaded, total) => (progress = { kind: 'zip', loaded, total })
			);
			saveBlob(blob, filename);
		} catch (error) {
			if (!isSignedOut(error)) toast.error(m.settings_transfer_export_failed());
		} finally {
			progress = null;
		}
	}

	// The PDF needs each recipe in full; ten requests at a time keep a large
	// pick from opening hundreds at once. The order is the list's, whatever
	// the search hides.
	async function runPdf() {
		progress = { kind: 'pdf', loaded: 0, total: null };
		try {
			const ids = [...selected].sort(compare).map((r) => r.id);
			const full: Recipe[] = [];
			for (let i = 0; i < ids.length; i += 10) {
				full.push(...(await Promise.all(ids.slice(i, i + 10).map(getRecipe))));
			}
			const { blob, filename } = await requestPdf(
				bulkPdfRequest(full, printOptions, location.origin),
				(loaded, total) => (progress = { kind: 'pdf', loaded, total })
			);
			saveBlob(blob, filename);
		} catch (error) {
			if (!isSignedOut(error)) toast.error(m.settings_transfer_pdf_failed());
		} finally {
			progress = null;
		}
	}
</script>

{#snippet state(kind: 'zip' | 'pdf', idle: string)}
	{#if progress?.kind !== kind}
		{idle}
	{:else if percent === null}
		{m.settings_transfer_preparing()}
	{:else}
		{m.settings_transfer_downloading({ percent })}
	{/if}
{/snippet}

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
					crop={{ focus: r.coverFocus, width: r.coverWidth, height: r.coverHeight }}
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
				<button type="button" class="{textAction} hover:text-text" onclick={() => tickAll(true)}>
					{m.settings_transfer_tick_all()}
				</button>
				·
				<button type="button" class="{textAction} hover:text-text" onclick={() => tickAll(false)}>
					{m.settings_transfer_clear()}
				</button>
			</span>
			<!-- ml-auto: wrapped below the summary, the buttons stay on the right. -->
			<div class="ml-auto flex flex-wrap justify-end gap-2">
				{#if settings?.pdf.available}
					<Button
						variant="secondary"
						disabled={selected.length === 0 || progress !== null}
						onclick={() => (pdfOpen = true)}
					>
						<FileText class="size-4" aria-hidden="true" />
						{@render state('pdf', m.settings_transfer_pdf_button())}
					</Button>
				{/if}
				<Button
					variant="primary"
					disabled={selected.length === 0 || progress !== null}
					onclick={run}
				>
					{@render state('zip', m.settings_transfer_export_button())}
				</Button>
			</div>
		</div>
		<ExportPdfDialog bind:open={pdfOpen} count={selected.length} oncreate={() => void runPdf()} />
	{/if}
</section>
