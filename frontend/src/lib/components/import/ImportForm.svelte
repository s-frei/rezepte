<script lang="ts">
	import ChevronDown from '@lucide/svelte/icons/chevron-down';
	import LoaderCircle from '@lucide/svelte/icons/loader-circle';
	import { Dialog } from 'bits-ui';
	import { onDestroy, tick } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { isSignedOut } from '$lib/api/client';
	import {
		createDraft,
		draftFailure,
		fetchDraftPhoto,
		type DraftFailure,
		type RecipeDraft
	} from '$lib/api/drafts';
	import { TOGGLE } from '$lib/components/editor/link-list';
	import Button from '$lib/components/ui/Button.svelte';
	import { importDialog } from '$lib/import.svelte';
	import { m } from '$lib/paraglide/messages';
	import { putDraft } from '$lib/recipe/draft';
	import { detectInput } from '$lib/recipe/import-input';

	let { ondone, admin }: { ondone: () => void; admin: boolean } = $props();

	let raw = $state('');
	let reading = $state(false);
	let failure = $state<DraftFailure | null>(null);
	/** Set when a page had no recipe: the text pasted next keeps it as source. */
	let fallback = $state<{ url: string; host: string } | null>(null);
	let duplicate = $state<RecipeDraft | null>(null);
	let field = $state<HTMLTextAreaElement>();

	const input = $derived(detectInput(raw));
	const readingHost = $derived(input.kind === 'link' ? input.host : null);
	// The example sits behind its toggle only while text is in: the empty
	// field shows it as placeholder, and a link needs none. Writable, and
	// closed again whenever the field starts or stops holding text: an
	// assignment holds until isText changes, which re-runs the derivation.
	const isText = $derived(input.kind === 'text');
	let exampleOpen = $derived.by(() => {
		void isText;
		return false;
	});

	// Closing the dialog while it reads cancels the reading: no draft, no
	// navigation, no error toast. At once, not when the form unmounts after
	// the dialog's fade-out, so an answer in between is dropped too. One
	// controller per read: a dialog reopened during the fade-out keeps this
	// form, and its next read must not start out aborted.
	let reads = new AbortController();
	function startRead(): AbortSignal {
		reads.abort();
		reads = new AbortController();
		return reads.signal;
	}
	$effect(() => {
		if (!importDialog.open) reads.abort();
	});
	onDestroy(() => reads.abort());

	async function focusField() {
		// Re-enabled only now: a disabled field drops its focus.
		await tick();
		field?.focus();
	}

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		if (input.kind === 'empty' || reading) return;
		reading = true;
		failure = null;
		// A new link is a new source; the old one stays only for text.
		if (input.kind === 'link') fallback = null;
		const signal = startRead();
		try {
			const draft = await createDraft(
				input.kind === 'link' ? { url: input.url } : { text: input.text },
				signal
			);
			// The service drops a source over 500 characters; so does the import.
			if (input.kind === 'text' && fallback && fallback.url.length <= 500) {
				draft.recipe.sourceUrl = fallback.url;
			}
			if (draft.duplicate) {
				duplicate = draft;
				return;
			}
			await open(draft, signal);
			return;
		} catch (error) {
			if (signal.aborted) return;
			const reason = draftFailure(error);
			if (!reason) {
				// 401 already redirects to the login page (see $lib/api/client).
				if (!isSignedOut(error)) toast.error(m.import_error());
			} else {
				failure = reason;
				if (reason === 'no-recipe' && input.kind === 'link') {
					fallback = { url: input.url, host: input.host };
					raw = '';
				}
			}
		} finally {
			reading = false;
		}
		await focusField();
	}

	async function open(draft: RecipeDraft, signal: AbortSignal) {
		let photo: File | null = null;
		let photoFailed = false;
		if (draft.photo) {
			try {
				photo = await fetchDraftPhoto(draft.photo.href, signal);
			} catch {
				photoFailed = true;
			}
		}
		if (signal.aborted || !importDialog.open) return;
		// Also from /recipes/new itself: the navigation lets an editor with
		// unsaved changes ask first (RecipeForm's leave guard), and its
		// "Keep editing" drops the draft again.
		putDraft({ draft, photo, photoFailed });
		ondone();
		await goto(resolve('/recipes/new'));
	}

	async function importAnyway() {
		if (!duplicate || reading) return;
		reading = true;
		try {
			await open(duplicate, startRead());
		} finally {
			reading = false;
		}
	}

	async function backToField() {
		duplicate = null;
		await focusField();
	}

	const chip = 'rounded-pill bg-accent px-2.5 py-0.5 font-semibold text-accent-foreground';
</script>

{#snippet readingLine()}
	<LoaderCircle class="size-4 animate-spin motion-reduce:animate-none" aria-hidden="true" />
	{readingHost ? m.import_reading({ host: readingHost }) : m.import_reading_text()}
{/snippet}

<form onsubmit={submit}>
	<Dialog.Title class="font-display text-heading font-medium">{m.import_title()}</Dialog.Title>
	{#if failure}
		<p
			role="alert"
			class="mt-3 rounded-md bg-destructive-soft px-3 py-2.5 text-body-sm text-destructive"
		>
			{#if failure === 'no-recipe'}
				{m.import_failed_no_recipe({ host: fallback?.host ?? '' })}
			{:else if failure === 'unreachable'}
				{m.import_failed_unreachable()}
			{:else}
				{m.import_failed_no_recipe_in_text()}
			{/if}
		</p>
	{:else}
		<Dialog.Description class="mt-1 text-body-sm text-text-muted">
			{m.import_lead()}
		</Dialog.Description>
	{/if}

	{#if duplicate?.duplicate}
		{@const found = duplicate.duplicate}
		<div class="mt-4 rounded-md bg-accent px-3 py-2.5 text-body-sm text-accent-foreground">
			<p>
				{m.import_duplicate()}
				<strong>{found.title}</strong> · {m.import_duplicate_by({
					name: found.createdBy.displayName
				})}
			</p>
		</div>
		<p class="mt-2 flex min-h-6 items-center gap-1.5 text-caption text-text-muted">
			{#if reading}
				{@render readingLine()}
			{/if}
		</p>
		<!-- On a phone the buttons stack full width, Import anyway on top. -->
		<div class="mt-2 flex justify-end gap-2 max-md:flex-col-reverse">
			<Button
				variant="ghost"
				onclick={backToField}
				disabled={reading}
				class="max-md:h-12 max-md:w-full md:mr-auto"
			>
				{m.import_duplicate_back()}
			</Button>
			<Button
				variant="secondary"
				href={resolve('/recipes/[slug]', { slug: found.slug })}
				onclick={ondone}
				class="max-md:h-12 max-md:w-full"
			>
				{m.import_duplicate_view()}
			</Button>
			<Button
				variant="primary"
				onclick={importAnyway}
				disabled={reading}
				class="max-md:h-12 max-md:w-full"
			>
				{m.import_duplicate_anyway()}
			</Button>
		</div>
	{:else}
		<label for="import-field" class="sr-only">{m.import_field_label()}</label>
		<!-- Dashed like a drop area: a link dragged from the browser lands in it. -->
		<textarea
			id="import-field"
			bind:this={field}
			bind:value={raw}
			rows={input.kind === 'text' ? 7 : input.kind === 'link' ? 3 : 10}
			placeholder={`${m.import_field_placeholder()}\n\n${m.import_example()}`}
			disabled={reading}
			class="mt-4 block w-full resize-y rounded-md border-2 border-dashed border-handle bg-surface-elevated px-3.5 py-3 text-body-sm transition outline-none focus:border-primary disabled:opacity-60"
		></textarea>
		<div class="mt-2 flex flex-wrap items-center gap-x-3">
			<p class="flex min-h-6 flex-wrap items-center gap-1.5 text-caption text-text-muted">
				{#if reading}
					{@render readingLine()}
				{:else}
					{#if input.kind === 'link'}
						<span class={chip}>{m.import_detected_link()}</span>
						{m.import_detected_link_host({ host: input.host })}
					{:else if input.kind === 'text'}
						<span class={chip}>{m.import_detected_text()}</span>
					{/if}
					{#if fallback && input.kind !== 'link'}
						<span class={input.kind === 'empty' ? '' : 'ml-2'}>{m.import_source()}</span>
						<span class={chip}>{fallback.host}</span>
					{/if}
				{/if}
			</p>
			<!-- Opens like the editor's suggestion lines. -->
			{#if isText}
				<button
					type="button"
					aria-expanded={exampleOpen}
					aria-controls="import-example"
					onclick={() => (exampleOpen = !exampleOpen)}
					class="ml-auto {TOGGLE}"
				>
					{m.import_example_toggle()}
					<ChevronDown
						class="size-3.5 transition-transform motion-reduce:transition-none {exampleOpen
							? 'rotate-180'
							: ''}"
						aria-hidden="true"
					/>
				</button>
			{/if}
		</div>
		{#if exampleOpen}
			<div
				id="import-example"
				class="mt-1 border-t border-dashed border-border pt-2.5 text-caption text-text-muted"
			>
				<p class="mb-1.5 text-label font-bold uppercase">{m.import_example_label()}</p>
				<p
					class="rounded-md border border-border bg-surface-elevated px-3 py-2.5 whitespace-pre-line text-text"
				>
					{m.import_example()}
				</p>
				<p class="mt-1.5">{m.import_example_note()}</p>
			</div>
		{/if}
		<!-- On a phone the sheet's handle closes it, so Import stands alone
		     and spans the sheet. -->
		<div class="mt-4 flex justify-end gap-2">
			<Button variant="ghost" onclick={ondone} class="max-md:hidden">
				{m.import_cancel()}
			</Button>
			<Button
				variant="primary"
				type="submit"
				disabled={input.kind === 'empty' || reading}
				class="max-md:h-12 max-md:w-full"
			>
				{m.import_submit()}
			</Button>
		</div>
	{/if}

	{#if admin}
		<div
			class="mt-4 flex items-start justify-between gap-2 border-t border-border pt-3 text-caption text-text-muted"
		>
			<p>
				{m.import_zip_hint()}
				<!-- Resolved; the rule only loses track of it once `#import` follows. -->
				<!-- eslint-disable svelte/no-navigation-without-resolve -->
				<a
					href={`${resolve('/settings/transfer')}#import`}
					onclick={ondone}
					class="font-semibold whitespace-nowrap text-primary">{m.import_zip_link()}</a
				>
				<!-- eslint-enable svelte/no-navigation-without-resolve -->
			</p>
			<span
				class="shrink-0 rounded-pill bg-accent px-2 text-label font-bold text-accent-foreground uppercase"
				>{m.import_admin_tag()}</span
			>
		</div>
	{/if}
</form>
