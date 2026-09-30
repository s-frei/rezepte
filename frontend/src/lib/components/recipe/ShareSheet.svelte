<script lang="ts">
	import { Dialog } from 'bits-ui';
	import ClipboardCopy from '@lucide/svelte/icons/clipboard-copy';
	import Download from '@lucide/svelte/icons/download';
	import ImageIcon from '@lucide/svelte/icons/image';
	import Link from '@lucide/svelte/icons/link';
	import Scissors from '@lucide/svelte/icons/scissors';
	import Share from '@lucide/svelte/icons/share';
	import X from '@lucide/svelte/icons/x';
	import { prefersReducedMotion } from 'svelte/motion';
	import { MediaQuery } from 'svelte/reactivity';
	import type { Recipe } from '$lib/api/recipes';
	import BaseDialog from '$lib/components/ui/BaseDialog.svelte';
	import BottomSheet from '$lib/components/ui/BottomSheet.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Skeleton from '$lib/components/ui/Skeleton.svelte';
	import { formatServings } from '$lib/recipe/format';
	import { createServings } from '$lib/recipe/servings.svelte';
	import {
		canCopyImage,
		canShareFiles,
		copyImage,
		copyLink,
		downloadImage,
		shareImage,
		shareLink
	} from '$lib/recipe/share.svelte';
	import { renderShareImage } from '$lib/recipe/share-image';
	import { m } from '$lib/paraglide/messages';
	import ShareCard from './ShareCard.svelte';

	// Passing a recipe on: as a link, or as the recipe card - a picture of
	// the whole recipe. The card is drawn as soon as the sheet opens and
	// every picture action waits for it, so a tap never spends Safari's user
	// activation on a picture that is not there yet. The preview is the
	// drawn picture itself, not a second rendering of it.
	let {
		open = $bindable(false),
		recipe,
		attribution,
		linkUrl
	}: {
		open?: boolean;
		recipe: Recipe;
		/** The household's "name Rezepte when sharing" setting. */
		attribution: boolean;
		/** The address "Link" hands out - the page's `ShareLink`, read at tap time. */
		linkUrl: () => string;
	} = $props();

	const phone = new MediaQuery('max-width: 767.98px');
	// Read afresh on every opening: the page's stepper stores its choice.
	const servings = $derived(
		open ? createServings(recipe.id, recipe.servings).value : recipe.servings
	);
	const fileName = $derived(`${recipe.slug}.png`);
	const label = 'text-label font-bold tracking-[0.08em] text-text-muted uppercase';

	let blob = $state<Blob | null>(null);
	let preview = $state<string | null>(null);
	let failed = $state(false);
	let lifting = $state(false);

	// Attached to the off-screen host, which mounts with the sheet and goes
	// with it: one drawing per opening, dropped on closing.
	function draw(node: HTMLDivElement) {
		let cancelled = false;
		let url: string | null = null;
		renderShareImage(node)
			.then((drawn) => {
				if (cancelled) return;
				url = URL.createObjectURL(drawn);
				blob = drawn;
				preview = url;
			})
			.catch((error: unknown) => {
				if (cancelled) return;
				console.error(error);
				failed = true;
			});
		return () => {
			cancelled = true;
			if (url) URL.revokeObjectURL(url);
			blob = null;
			preview = null;
			failed = false;
			lifting = false;
		};
	}

	async function sendCard() {
		if (!blob) return;
		const result = await shareImage(blob, fileName, recipe.title);
		if (result === 'aborted') return;
		lifting = true;
		setTimeout(() => (open = false), prefersReducedMotion.current ? 0 : 320);
	}

	function sendLink() {
		return shareLink(recipe.title, linkUrl());
	}
</script>

{#snippet card(frame: string)}
	<div
		class="flex justify-center overflow-y-auto overscroll-contain mask-b-from-80% px-8 pt-3 pb-8 {frame}"
	>
		{#if preview}
			<img
				src={preview}
				alt={m.share_preview_alt({ title: recipe.title })}
				class="h-max w-[220px] -rotate-[2.5deg] rounded-lg shadow-lift transition duration-300 ease-spring motion-reduce:transition-none {lifting
					? '-translate-y-24 opacity-0'
					: ''}"
			/>
		{:else if failed}
			<p class="self-center text-center text-body-sm text-text-muted">{m.share_render_error()}</p>
		{:else}
			<Skeleton class="h-[320px] w-[220px] -rotate-[2.5deg] rounded-lg" />
		{/if}
	</div>
{/snippet}

<!-- The card itself, drawn from here: laid out, never seen, out of reach of
     focus and screen readers. Not `display: none` - a hidden node has no
     layout to draw. Inherits the page's theme from <html>. -->
{#if open}
	<div
		{@attach draw}
		aria-hidden="true"
		inert
		class="pointer-events-none fixed top-0 left-[-10000px]"
	>
		<ShareCard {recipe} {servings} {attribution} />
	</div>
{/if}

{#if phone.current}
	<BottomSheet bind:open closeLabel={m.share_sheet_close()}>
		<Dialog.Title class="px-5 text-center {label}">{m.share_sheet_title()}</Dialog.Title>
		{@render card('h-[340px]')}
		<div class="relative border-t-2 border-dashed border-handle" aria-hidden="true">
			<Scissors
				class="absolute -top-2.5 left-5 size-5 -rotate-90 bg-background px-0.5 text-text-muted"
			/>
		</div>
		<div class="flex gap-2.5 px-5 pt-4 pb-[max(1.25rem,env(safe-area-inset-bottom))]">
			<Button variant="secondary" size="lg" class="flex-1" onclick={sendLink}>
				<Link class="size-4" aria-hidden="true" />
				{m.share_link()}
			</Button>
			<Button
				variant="primary"
				size="lg"
				class="flex-1 shadow-cta"
				disabled={!blob}
				onclick={sendCard}
			>
				<ImageIcon class="size-4" aria-hidden="true" />
				{m.share_card()}
			</Button>
		</div>
	</BottomSheet>
{:else}
	<BaseDialog bind:open wide>
		<Dialog.Close
			aria-label={m.share_sheet_close()}
			class="absolute top-4 right-4 flex size-9 items-center justify-center rounded-full text-text-muted transition hover:bg-background focus-visible:outline-2 focus-visible:outline-primary"
		>
			<X class="size-5" aria-hidden="true" />
		</Dialog.Close>
		<div class="grid grid-cols-[300px_1fr] gap-7">
			<div class="-my-7 -ml-7 bg-background">{@render card('h-[480px]')}</div>
			<div class="flex flex-col py-1">
				<Dialog.Title class={label}>{m.share_sheet_title()}</Dialog.Title>
				<p class="mt-1.5 font-display text-heading font-medium">{recipe.title}</p>
				<p class="mt-1 text-body-sm text-text-muted">
					{m.share_dialog_hint({ servings: formatServings(servings) })}
				</p>
				<p class="mt-5 mb-2 {label}">{m.share_card()}</p>
				<div class="flex flex-col gap-1.5">
					{#if canCopyImage()}
						<Button class="justify-start!" disabled={!blob} onclick={() => blob && copyImage(blob)}>
							<ClipboardCopy class="size-4" aria-hidden="true" />
							{m.share_copy_image()}
						</Button>
					{/if}
					<Button
						variant="secondary"
						class="justify-start!"
						disabled={!blob}
						onclick={() => blob && downloadImage(blob, fileName)}
					>
						<Download class="size-4" aria-hidden="true" />
						{m.share_save_image()}
					</Button>
					{#if canShareFiles()}
						<Button variant="secondary" class="justify-start!" disabled={!blob} onclick={sendCard}>
							<Share class="size-4" aria-hidden="true" />
							{m.share_share()}
						</Button>
					{/if}
				</div>
				<p class="mt-5 mb-2 {label}">{m.share_link()}</p>
				<Button variant="secondary" class="justify-start!" onclick={() => copyLink(linkUrl())}>
					<Link class="size-4" aria-hidden="true" />
					{m.detail_copy_link()}
				</Button>
			</div>
		</div>
	</BaseDialog>
{/if}
