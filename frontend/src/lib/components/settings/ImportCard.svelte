<script lang="ts">
	import { onDestroy } from 'svelte';
	import { resolve } from '$app/paths';
	import { SvelteMap } from 'svelte/reactivity';
	import { ApiError } from '$lib/api/client';
	import type { RecipeCard } from '$lib/api/recipes';
	import { importZip } from '$lib/api/transfer';
	import { normalizeTitle, packFolder, readExport, type ImportRow } from '$lib/transfer/zip';
	import Button from '$lib/components/ui/Button.svelte';
	import Skeleton from '$lib/components/ui/Skeleton.svelte';
	import TransferRow from './TransferRow.svelte';
	import { m } from '$lib/paraglide/messages';

	/** The recipes already here, to mark a title that would arrive twice. */
	let {
		existing,
		ready
	}: {
		existing: RecipeCard[];
		/** False until `existing` holds every recipe; the card waits till then. */
		ready: boolean;
	} = $props();

	type RowState =
		| { kind: 'idle' }
		| { kind: 'waiting' }
		| { kind: 'uploading'; percent: number }
		| { kind: 'processing' }
		| { kind: 'done'; slug: string }
		| { kind: 'failed'; message: string };

	let fileName = $state('');
	let fileError = $state('');
	/** The opened zip, unread past each recipe.json and cover until import. */
	let bytes = $state.raw(new Uint8Array());
	let rows = $state.raw<ImportRow[]>([]);
	let thumbs = $state.raw<Record<string, string>>({});
	let ticked = $state<Record<string, boolean>>({});
	let status = $state<Record<string, RowState>>({});
	let running = $state(false);
	let input = $state<HTMLInputElement>();
	/** What a screen reader hears: each row's state change, never its percent. */
	let announcement = $state('');

	onDestroy(() => {
		for (const url of Object.values(thumbs)) URL.revokeObjectURL(url);
	});

	const byTitle = $derived(new Map(existing.map((r) => [normalizeTitle(r.title), r.slug])));
	/** Titles imported this session, which `existing` does not hold yet. */
	const imported = new SvelteMap<string, string>();
	/** The slug of a recipe here with the same title, or undefined. */
	const twinOf = (title: string) =>
		byTitle.get(normalizeTitle(title)) ?? imported.get(normalizeTitle(title));
	const pending = $derived(
		rows.filter((r) => ticked[r.folder] && status[r.folder]?.kind !== 'done')
	);
	const anyDone = $derived(rows.some((r) => status[r.folder]?.kind === 'done'));

	async function open(file: File) {
		for (const url of Object.values(thumbs)) URL.revokeObjectURL(url);
		thumbs = {};
		fileName = file.name;
		fileError = '';
		status = {};
		let found: ImportRow[] | null = null;
		try {
			const raw = new Uint8Array(await file.arrayBuffer());
			found = readExport(raw);
			bytes = raw;
		} catch {
			// not a zip; handled below
		}
		// A zip that is not an export at all - no folder with a recipe.json -
		// says so too, rather than leaving the drop zone as if nothing happened.
		if (found === null || found.length === 0) {
			bytes = new Uint8Array();
			rows = [];
			fileError =
				found === null
					? m.settings_transfer_error_not_zip()
					: m.settings_transfer_error_no_recipes();
			return;
		}
		rows = found;
		thumbs = Object.fromEntries(
			rows.flatMap((r) =>
				r.cover
					? [
							[
								r.folder,
								URL.createObjectURL(new Blob([r.cover as BlobPart], { type: 'image/jpeg' }))
							]
						]
					: []
			)
		);
		// A title already here stays unticked: importing it makes a second copy.
		ticked = Object.fromEntries(
			rows.map((r) => [r.folder, r.error === null && twinOf(r.title) === undefined])
		);
	}

	function pick(e: Event & { currentTarget: HTMLInputElement }) {
		const file = e.currentTarget.files?.[0];
		// Cleared so choosing the same file again still fires a change.
		e.currentTarget.value = '';
		if (file) void open(file);
	}

	// One request per recipe, in order: each lands or fails on its own, and the
	// first failure stops the run so its message can be read before carrying on.
	async function run() {
		running = true;
		const queue = pending;
		for (const r of queue) status[r.folder] = { kind: 'waiting' };
		for (const r of queue) {
			status[r.folder] = { kind: 'uploading', percent: 0 };
			try {
				const res = await importZip(packFolder(bytes, r.folder), (f) => {
					if (f < 1) {
						status[r.folder] = { kind: 'uploading', percent: Math.floor(f * 100) };
					} else if (status[r.folder]?.kind !== 'processing') {
						status[r.folder] = { kind: 'processing' };
						announcement = `${r.title}: ${m.settings_transfer_state_processing()}`;
					}
				});
				const slug = res.created[0].slug;
				status[r.folder] = { kind: 'done', slug };
				imported.set(normalizeTitle(r.title), slug);
				announcement = `${r.title}: ${m.settings_transfer_state_done()}`;
			} catch (error) {
				const message =
					error instanceof ApiError
						? error.errors[0]?.message || error.detail || error.title
						: String(error);
				status[r.folder] = { kind: 'failed', message };
				announcement = `${r.title}: ${m.settings_transfer_state_failed()} – ${message}`;
				for (const rest of queue)
					if (status[rest.folder]?.kind === 'waiting') status[rest.folder] = { kind: 'idle' };
				break;
			}
		}
		running = false;
	}

	function errorText(e: ImportRow['error']) {
		if (e === 'unknown-format') return m.settings_transfer_error_unknown_format();
		if (e === 'unsupported-version') return m.settings_transfer_error_unsupported_version();
		if (e === 'newer-version') return m.settings_transfer_error_newer_version();
		return m.settings_transfer_error_unreadable();
	}
</script>

<section id="import" class="rounded-2xl bg-surface p-6 md:p-7" aria-labelledby="transfer-import">
	<h2 id="transfer-import" class="font-display text-heading font-medium">
		{m.settings_transfer_import_title()}
	</h2>
	<p class="mt-1 mb-4 text-body-sm text-text-muted">{m.settings_transfer_import_hint()}</p>
	<p class="sr-only" aria-live="polite">{announcement}</p>
	{#if !ready}
		<!-- Until every recipe here is known, a file would tick its duplicates,
		     so not even the file field exists yet. -->
		<Skeleton class="h-14 rounded-md" />
		<Skeleton class="mt-2 h-14 rounded-md" />
	{:else}
		<input
			bind:this={input}
			type="file"
			tabindex="-1"
			accept=".zip,application/zip"
			class="sr-only"
			aria-label={m.settings_transfer_choose_file()}
			onchange={pick}
		/>
		{#if rows.length === 0}
			<div
				role="presentation"
				class="rounded-lg border-[1.5px] border-dashed border-border bg-surface-elevated p-6 text-center text-body-sm text-text-muted"
				ondragover={(e) => e.preventDefault()}
				ondrop={(e) => {
					e.preventDefault();
					const file = e.dataTransfer?.files[0];
					if (file) void open(file);
				}}
			>
				{m.settings_transfer_drop()}
				<button
					type="button"
					class="font-semibold text-text underline"
					onclick={() => input?.click()}
				>
					{m.settings_transfer_choose_file()}
				</button>
				{#if fileError}
					<p class="mt-2 text-destructive" role="alert">{fileError}</p>
				{/if}
			</div>
		{:else}
			<!-- The delivery slip: what arrived, on paper, before any of it is kept. -->
			<div
				class="rounded-word border border-border bg-surface-elevated px-3 pt-3 pb-1 shadow-cover md:px-4"
			>
				<!-- The stamp is the slip's header: ruled off from the list, so rows
				     scrolling up under it end at a line instead of at its letters. -->
				<p class="border-b border-border pb-2 text-label break-words text-text-muted uppercase">
					{rows.length === 1
						? m.settings_transfer_received_one({ file: fileName })
						: m.settings_transfer_received({ file: fileName, count: rows.length })}
				</p>
				<ul class="max-h-[60vh] scrollbar-themed overflow-y-auto overscroll-contain pr-3 md:pr-2">
					{#each rows as r (r.folder)}
						{@const st = status[r.folder] ?? { kind: 'idle' }}
						{@const twin = twinOf(r.title)}
						<TransferRow
							bind:checked={ticked[r.folder]}
							disabled={r.error !== null || running || st.kind === 'done'}
							title={r.title}
							tags={r.tags}
							thumb={thumbs[r.folder] ?? null}
							href={st.kind === 'done'
								? resolve('/recipes/[slug]', { slug: st.slug })
								: twin
									? resolve('/recipes/[slug]', { slug: twin })
									: null}
							note={r.error ? errorText(r.error) : st.kind === 'failed' ? st.message : null}
							hrefLabel={st.kind === 'done'
								? m.settings_transfer_open({ title: r.title })
								: m.settings_transfer_open_existing({ title: r.title })}
						>
							{#snippet trailing()}
								<span class="text-micro whitespace-nowrap text-text-muted">
									{#if r.error}
										<!-- the reason is the note below the title -->
										<span class="text-destructive">✗</span>
									{:else if st.kind === 'waiting'}
										{m.settings_transfer_state_waiting()}
									{:else if st.kind === 'uploading'}
										{m.settings_transfer_state_uploading({ percent: st.percent })}
									{:else if st.kind === 'processing'}
										{m.settings_transfer_state_processing()}
									{:else if st.kind === 'done'}
										<span class="text-text">✓ {m.settings_transfer_state_done()}</span>
									{:else if st.kind === 'failed'}
										<span class="text-destructive">✗ {m.settings_transfer_state_failed()}</span>
									{:else if twin}
										<span
											class="rounded-pill bg-accent px-2 py-0.5 text-label text-accent-foreground uppercase"
										>
											{m.settings_transfer_already_here()}
										</span>
									{:else if r.imageCount === 1}
										{m.settings_transfer_photos_one()}
									{:else}
										{m.settings_transfer_photos({ count: r.imageCount })}
									{/if}
								</span>
							{/snippet}
						</TransferRow>
					{/each}
				</ul>
			</div>
			<div
				class="mt-4 flex flex-wrap items-center justify-between gap-3 border-t border-border pt-3 text-caption text-text-muted"
			>
				<span>
					{m.settings_transfer_import_summary({ ticked: pending.length, total: rows.length })}
					·
					<button
						type="button"
						class="underline hover:text-text"
						disabled={running}
						onclick={() => input?.click()}
					>
						{m.settings_transfer_other_file()}
					</button>
				</span>
				<Button variant="primary" disabled={pending.length === 0 || running} onclick={run}>
					{#if anyDone}
						{m.settings_transfer_import_remaining()}
					{:else if pending.length === 1}
						{m.settings_transfer_import_button_one()}
					{:else}
						{m.settings_transfer_import_button({ count: pending.length })}
					{/if}
				</Button>
			</div>
		{/if}
	{/if}
</section>
