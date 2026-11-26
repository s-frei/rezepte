<script lang="ts">
	import Download from '@lucide/svelte/icons/download';
	import LoaderCircle from '@lucide/svelte/icons/loader-circle';
	import Printer from '@lucide/svelte/icons/printer';
	import Share from '@lucide/svelte/icons/share';
	import type { Snippet } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { isSignedOut } from '#lib/api/client.js';
	import type { Recipe } from '#lib/api/recipes.js';
	import { saveBlob } from '#lib/api/transfer.js';
	import Button from '#lib/components/ui/Button.svelte';
	import SegmentedControl from '#lib/components/ui/SegmentedControl.svelte';
	import Switch from '#lib/components/ui/Switch.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import {
		printOptions,
		setPrintOption,
		type PrintOptions
	} from '#lib/recipe/print-options.svelte.js';
	import { requestPdf, requestPreview } from '#lib/api/pdf.js';
	import { printRecipe, recipeRequest } from '#lib/recipe/print.js';
	import { canShareFiles } from '#lib/recipe/share.svelte.js';

	// The pass-on sheet's PDF tab: page 1 of the PDF as a preview, the
	// options every print shares, and Download, Print and Share. It mounts with the sheet, so a PDF held
	// for Safari's second tap goes when the sheet closes. Laid out like
	// `ShareLinkPanel`: one column in the phone sheet's tab cell, the
	// dialog's two columns from 768px.
	let {
		recipe,
		servings,
		phone,
		shown,
		switcher,
		heading
	}: {
		recipe: Recipe;
		/** The servings chosen on the page; the PDF is scaled to them. */
		servings: number;
		phone: boolean;
		/** The PDF tab is the one showing. */
		shown: boolean;
		/** The dialog's tab switch, at the top of its left column. */
		switcher: Snippet;
		/** The dialog's label and title, at the top of its right column. */
		heading: Snippet;
	} = $props();

	const shareable = canShareFiles('application/pdf');

	// The request out, by the button that sent it.
	let pending = $state<'download' | 'share' | 'print' | null>(null);
	// A share sheet is up: a second tap would only be refused.
	let sending = $state(false);
	// Safari refused the share because the request spent the tap's
	// activation: the PDF waits here for a second tap, which shares at once.
	let held = $state<File | null>(null);
	// Counts option changes: a PDF rendered before one is stale.
	let changes = 0;

	// A held PDF shows the old options, so a change drops it: Share now
	// never shares what the preview no longer shows.
	function pick<K extends 'format' | 'photo' | 'footer'>(key: K, value: PrintOptions[K]) {
		setPrintOption(key, value);
		changes++;
		held = null;
	}

	const pdfBody = () => recipeRequest(recipe, servings, printOptions);
	const render = () => requestPdf(pdfBody());

	// Page 1 as Typst renders it, the custom template's too: asked for when
	// the tab shows and again, debounced, after an option changes. A change
	// while one renders aborts it, which frees its render on the server.
	let previewUrl = $state<string | null>(null);
	let previewing = $state(false);
	$effect(() => {
		if (!shown) return;
		const body = recipeRequest(recipe, servings, printOptions);
		const abort = new AbortController();
		const timer = setTimeout(async () => {
			previewing = true;
			try {
				const blob = await requestPreview(body, abort.signal);
				if (previewUrl) URL.revokeObjectURL(previewUrl);
				previewUrl = URL.createObjectURL(blob);
			} catch (error) {
				if (!abort.signal.aborted) console.error(error);
			} finally {
				if (!abort.signal.aborted) previewing = false;
			}
		}, 250);
		return () => {
			abort.abort();
			clearTimeout(timer);
		};
	});
	$effect(() => () => {
		if (previewUrl) URL.revokeObjectURL(previewUrl);
	});

	async function print() {
		if (pending) return;
		pending = 'print';
		await printRecipe(pdfBody());
		pending = null;
	}

	function failed(error: unknown) {
		if (!isSignedOut(error)) toast.error(m.share_pdf_failed());
	}

	async function download() {
		if (pending) return;
		pending = 'download';
		try {
			const { blob, filename } = await render();
			saveBlob(blob, filename);
		} catch (error) {
			failed(error);
		} finally {
			pending = null;
		}
	}

	// Not `shareImage`: that one downloads on `NotAllowedError`, this one
	// keeps the file and turns the button into Share now.
	async function share() {
		if (pending || sending) return;
		let file = held;
		const asked = changes;
		if (!file) {
			pending = 'share';
			try {
				const { blob, filename } = await render();
				file = new File([blob], filename, { type: 'application/pdf' });
			} catch (error) {
				failed(error);
				return;
			} finally {
				pending = null;
			}
			// An option changed while it rendered: this PDF is stale.
			if (asked !== changes) return;
		}
		sending = true;
		try {
			await navigator.share({ files: [file], title: recipe.title });
			held = null;
		} catch (error) {
			// A canceled sheet (`AbortError`) leaves everything as it was.
			const name = error instanceof DOMException ? error.name : '';
			if (name === 'NotAllowedError') {
				if (asked === changes) held = file;
			} else if (name !== 'AbortError') {
				console.error(error);
				toast.error(m.share_pdf_share_failed());
			}
		} finally {
			sending = false;
		}
	}
</script>

{#snippet preview(frame: string, width: number)}
	<!-- The rendered page, tilted like the Image tab's picture; a paper-shaped
	     placeholder until it arrives. Decorative: the options say it all. -->
	<div
		aria-hidden="true"
		class="flex justify-center overflow-y-auto overscroll-contain mask-b-from-80% px-8 pt-3 pb-8 {frame}"
	>
		<div
			data-preview
			class="relative h-max shrink-0 -rotate-[1.4deg] overflow-hidden rounded-tape bg-surface-elevated shadow-lift"
			style:width="{width}px"
		>
			{#if previewUrl}
				<img src={previewUrl} alt="" class="block w-full" />
			{:else}
				<div class={printOptions.format === 'a5' ? 'aspect-[148/210]' : 'aspect-[210/297]'}></div>
			{/if}
			{#if previewing}
				<div class="absolute inset-0 grid place-items-center bg-surface-elevated/60">
					<LoaderCircle
						class="size-6 animate-spin text-text-muted motion-reduce:animate-none"
						aria-hidden="true"
					/>
				</div>
			{/if}
		</div>
	</div>
{/snippet}

{#snippet options()}
	<div class="divide-y divide-border rounded-lg border border-border bg-surface">
		<div class="flex items-center justify-between gap-3 py-1.5 pr-1.5 pl-3.5">
			<span class="text-body-sm">{m.share_pdf_format()}</span>
			<div class={phone ? 'w-[168px]' : 'w-[230px]'}>
				<SegmentedControl
					value={printOptions.format}
					options={[
						{ value: 'a4', label: phone ? m.share_pdf_a4() : m.share_pdf_cookbook() },
						{ value: 'a5', label: phone ? m.share_pdf_a5() : m.share_pdf_card() }
					]}
					label={m.share_pdf_format()}
					size="sm"
					onchange={(format) => pick('format', format)}
				/>
			</div>
		</div>
		{#each [['photo', m.share_pdf_photo()], ['footer', m.share_pdf_footer()]] as const as [key, name] (key)}
			<div class="flex min-h-12 items-center justify-between gap-3 px-3.5">
				<span class="text-body-sm">{name}</span>
				<Switch checked={printOptions[key]} label={name} onchange={(on) => pick(key, on)} />
			</div>
		{/each}
	</div>
{/snippet}

{#snippet shareLabel()}
	{#if pending === 'share'}
		<LoaderCircle class="size-4 animate-spin motion-reduce:animate-none" aria-hidden="true" />
		{m.share_pdf_creating()}
	{:else}
		<Share class="size-4" aria-hidden="true" />
		{held ? m.share_pdf_share_now() : phone ? m.share_pdf_share_pdf() : m.share_pdf_share()}
	{/if}
{/snippet}

{#snippet downloadLabel()}
	{#if pending === 'download'}
		<LoaderCircle class="size-4 animate-spin motion-reduce:animate-none" aria-hidden="true" />
		{m.share_pdf_creating()}
	{:else}
		<Download class="size-4" aria-hidden="true" />
		{m.share_pdf_download()}
	{/if}
{/snippet}

{#if phone}
	<!-- Shares the sheet's tab cell with the other tabs; the preview takes
	     what their height leaves. -->
	<div class="col-start-1 row-start-1 flex min-h-0 flex-col {shown ? '' : 'invisible'}">
		{@render preview('min-h-0 flex-[1_1_0]', 190)}
		<div class="px-5">{@render options()}</div>
		<div class="flex gap-2.5 px-5 pt-4 pb-[max(1.25rem,env(safe-area-inset-bottom))]">
			<!-- Download or print, always: the primary shares where the browser
			     can share a file, else it downloads; Print sits round beside it
			     like the Image tab's Save. -->
			{#if shareable}
				<Button
					size="lg"
					class="flex-1 px-3! whitespace-nowrap shadow-cta"
					busy={pending === 'share'}
					disabled={sending}
					onclick={share}
				>
					{@render shareLabel()}
				</Button>
				<Button
					variant="secondary"
					size="lg"
					class="aspect-square shrink-0 px-0!"
					label={m.share_pdf_download()}
					busy={pending === 'download'}
					onclick={download}
				>
					{#if pending === 'download'}
						<LoaderCircle
							class="size-5 animate-spin motion-reduce:animate-none"
							aria-hidden="true"
						/>
					{:else}
						<Download class="size-5" aria-hidden="true" />
					{/if}
				</Button>
			{:else}
				<Button
					size="lg"
					class="flex-1 px-3! whitespace-nowrap shadow-cta"
					busy={pending === 'download'}
					onclick={download}
				>
					{@render downloadLabel()}
				</Button>
			{/if}
			<Button
				variant="secondary"
				size="lg"
				class="aspect-square shrink-0 px-0!"
				label={m.detail_print()}
				busy={pending === 'print'}
				onclick={print}
			>
				{#if pending === 'print'}
					<LoaderCircle class="size-5 animate-spin motion-reduce:animate-none" aria-hidden="true" />
				{:else}
					<Printer class="size-5" aria-hidden="true" />
				{/if}
			</Button>
		</div>
	</div>
{:else if shown}
	<div class="-my-7 -ml-7 flex min-h-[480px] flex-col bg-background">
		<div class="px-5.5 pt-7 pb-1.5">{@render switcher()}</div>
		<div class="relative flex-1">{@render preview('absolute inset-0', 240)}</div>
	</div>
	<!-- Heading at the top, the options in the room between, the actions on
	     the stage's bottom edge, as on the other tabs. -->
	<div class="flex flex-col py-1">
		{@render heading()}
		<p class="mt-1 text-body-sm text-text-muted">
			{servings === 1
				? m.share_pdf_dialog_hint_one()
				: m.share_pdf_dialog_hint({ servings: String(servings) })}
		</p>
		<div class="flex flex-1 flex-col justify-center py-5">{@render options()}</div>
		<div class="flex flex-col gap-1.5">
			<Button busy={pending === 'download'} onclick={download}>
				{@render downloadLabel()}
			</Button>
			<div class="flex gap-1.5">
				<Button variant="secondary" class="flex-1" busy={pending === 'print'} onclick={print}>
					{#if pending === 'print'}
						<LoaderCircle
							class="size-4 animate-spin motion-reduce:animate-none"
							aria-hidden="true"
						/>
						{m.share_pdf_creating()}
					{:else}
						<Printer class="size-4" aria-hidden="true" />
						{m.detail_print()}
					{/if}
				</Button>
				{#if shareable}
					<Button
						variant="secondary"
						class="flex-1"
						busy={pending === 'share'}
						disabled={sending}
						onclick={share}
					>
						{@render shareLabel()}
					</Button>
				{/if}
			</div>
		</div>
	</div>
{/if}
