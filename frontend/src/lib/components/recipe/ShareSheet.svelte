<script lang="ts">
	import { Dialog } from 'bits-ui';
	import ClipboardCopy from '@lucide/svelte/icons/clipboard-copy';
	import Download from '@lucide/svelte/icons/download';
	import ImageIcon from '@lucide/svelte/icons/image';
	import Link from '@lucide/svelte/icons/link';
	import Share from '@lucide/svelte/icons/share';
	import X from '@lucide/svelte/icons/x';
	import { tick, untrack, type ComponentProps } from 'svelte';
	import { prefersReducedMotion } from 'svelte/motion';
	import { MediaQuery } from 'svelte/reactivity';
	import { fade } from 'svelte/transition';
	import type { Recipe } from '#lib/api/recipes.js';
	import BaseDialog from '#lib/components/ui/BaseDialog.svelte';
	import BottomSheet from '#lib/components/ui/BottomSheet.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import SegmentedControl from '#lib/components/ui/SegmentedControl.svelte';
	import Skeleton from '#lib/components/ui/Skeleton.svelte';
	import { formatServings } from '#lib/recipe/format.js';
	import { createServings } from '#lib/recipe/servings.svelte.js';
	import {
		canCopyImage,
		canShareFiles,
		copyImage,
		downloadImage,
		shareImage
	} from '#lib/recipe/share.svelte.js';
	import { renderShareImage } from '#lib/recipe/share-image.js';
	import { m } from '#lib/paraglide/messages.js';
	import { load, save } from '#lib/storage.js';
	import { narrow as phone } from '#lib/ui/narrow.js';
	import Perforation from './Perforation.svelte';
	import ShareCard from './ShareCard.svelte';
	import ShareLinkPanel from './ShareLinkPanel.svelte';

	type Tab = 'image' | 'link';

	// Passing a recipe on, in two tabs: as the recipe card - a picture of the
	// whole recipe - or as a link, drawn as a ticket. The card is drawn as
	// soon as the sheet opens, whichever tab shows, and every picture action
	// waits for it, so a tap never spends Safari's user activation on a
	// picture that is not there yet. The preview is the drawn picture itself,
	// not a second rendering of it. The Link tab is `ShareLinkPanel`.
	let {
		open = $bindable(false),
		recipe,
		attribution,
		share = $bindable(null),
		initialTab,
		...link
	}: {
		open?: boolean;
		recipe: Recipe;
		/** The household's "name Rezepte when sharing" setting. */
		attribution: boolean;
		/** The tab to open on; without one, the tab this device used last. */
		initialTab?: Tab;
	} & Pick<
		ComponentProps<typeof ShareLinkPanel>,
		'linkUrl' | 'share' | 'canCreateShare' | 'defaultDays' | 'maxDays'
	> = $props();

	// Press and hold is a touch gesture: a narrow desktop window gets the
	// phone sheet, but nothing to hold.
	const touch = new MediaQuery('pointer: coarse');
	// Read afresh on every opening: the page's stepper stores its choice.
	const servings = $derived(
		open ? createServings(recipe.id, recipe.servings).value : recipe.servings
	);
	const fileName = $derived(`${recipe.slug}.png`);
	const label = 'text-label font-bold tracking-[0.08em] text-text-muted uppercase';

	const TAB_KEY = 'rezepte-share-tab';
	const tabs = [
		{ value: 'image' as const, label: m.share_tab_image(), icon: ImageIcon },
		{ value: 'link' as const, label: m.share_tab_link(), icon: Link }
	];

	// Decided as the sheet opens - the page's choice, else the tab this
	// device used last - then the switch's. Only `open` is tracked, and on
	// closing the derived gives back the tab on screen, so the sheet keeps
	// it while it slides away.
	let onScreen: Tab = 'image';
	let tab: Tab = $derived.by(() => {
		if (open) onScreen = untrack(() => initialTab ?? (load(TAB_KEY) === 'link' ? 'link' : 'image'));
		return onScreen;
	});

	// From 768px each tab draws its own switch, so the one a key or a click
	// just focused goes with its tab: focus follows to the new tab's switch,
	// or it fell to the dialog's Close and the next Enter closed the dialog.
	async function pickTab(next: Tab) {
		const group = `[aria-label="${m.share_tabs_label()}"]`;
		const held = Boolean(document.activeElement?.closest(group));
		onScreen = next;
		save(TAB_KEY, next);
		if (!held) return;
		await tick();
		document.querySelector<HTMLElement>(`[role=dialog] ${group} [aria-checked="true"]`)?.focus();
	}

	let blob = $state<Blob | null>(null);
	let preview = $state<string | null>(null);
	let failed = $state(false);
	let lifting = $state(false);
	// A share sheet is up: a second tap would only be refused by the browser.
	let sending = $state(false);

	// Attached to the off-screen host, which mounts with the sheet and goes
	// with it: one drawing per opening, dropped on closing.
	function draw(node: HTMLDivElement) {
		let canceled = false;
		let url: string | null = null;
		renderShareImage(node)
			.then((drawn) => {
				if (canceled) return;
				url = URL.createObjectURL(drawn);
				blob = drawn;
				preview = url;
			})
			.catch((error: unknown) => {
				if (canceled) return;
				console.error(error);
				failed = true;
			});
		return () => {
			canceled = true;
			if (url) URL.revokeObjectURL(url);
			blob = null;
			preview = null;
			failed = false;
			lifting = false;
		};
	}

	async function sendCard() {
		if (!blob || sending) return;
		sending = true;
		const result = await shareImage(blob, fileName, recipe.title).finally(() => (sending = false));
		if (result === 'aborted') return;
		lifting = true;
		setTimeout(() => (open = false), prefersReducedMotion.current ? 0 : 320);
	}

	function saveCard() {
		if (blob) downloadImage(blob, fileName);
	}
</script>

{#snippet switcher()}
	<SegmentedControl
		bind:value={tab}
		options={tabs}
		label={m.share_tabs_label()}
		track="surface"
		onchange={pickTab}
	/>
{/snippet}

{#snippet card(frame: string)}
	<div
		class="flex justify-center overflow-y-auto overscroll-contain mask-b-from-80% px-8 pt-3 pb-8 {frame}"
	>
		{#if preview}
			<img
				src={preview}
				alt={m.share_preview_alt({ title: recipe.title })}
				in:fade={{ duration: 150 }}
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
	<!-- Keyed on the recipe: a reload of the page's data while the sheet is
	     open draws the picture again instead of keeping the old one. -->
	{#key recipe}
		<div
			{@attach draw}
			aria-hidden="true"
			inert
			data-testid="share-card"
			class="pointer-events-none fixed top-0 left-[-10000px]"
		>
			<ShareCard {recipe} {servings} {attribution} />
		</div>
	{/key}
{/if}

{#snippet heading()}
	<Dialog.Title class={label}>{m.share_sheet_title()}</Dialog.Title>
	<p class="mt-1.5 font-display text-heading font-medium">{recipe.title}</p>
{/snippet}

{#snippet panel(onPhone: boolean)}
	<ShareLinkPanel
		{...link}
		{recipe}
		bind:share
		phone={onPhone}
		shown={tab === 'link'}
		{switcher}
		{heading}
	/>
{/snippet}

{#if phone.current}
	<BottomSheet bind:open tall closeLabel={m.share_sheet_close()}>
		<Dialog.Title class="sr-only">{m.share_sheet_title()}</Dialog.Title>
		<div class="px-5 pb-1">{@render switcher()}</div>
		<!-- Both tabs are laid out in one cell, the other one invisible, so the
		     sheet keeps the taller one's height and neither the switch nor the
		     buttons move on a switch. The cell gives way below the sheet's cap:
		     the card's frame scrolls, the Link tab's preview steps down. -->
		<div class="grid min-h-0 grid-cols-[minmax(0,1fr)] grid-rows-[minmax(0,1fr)]">
			<div
				class="col-start-1 row-start-1 flex min-h-0 flex-col {tab === 'image' ? '' : 'invisible'}"
			>
				{@render card('h-[340px] flex-auto')}
				{#if !canShareFiles() && touch.current}
					<!-- No share sheet for files (plain http is not a secure context):
					     the phone's own long-press menu on the preview still shares it. -->
					<p class="px-5 pb-3 text-center text-caption text-text-muted">{m.share_hold_hint()}</p>
				{/if}
				<Perforation />
				<!-- The primary says what this browser will do: share where there is a
				     share sheet for files, else copy where the clipboard takes an
				     image, else save - and Save stays beside it while it is not Save. -->
				<div class="flex gap-2.5 px-5 pt-4 pb-[max(1.25rem,env(safe-area-inset-bottom))]">
					{#if canShareFiles()}
						<Button
							size="lg"
							class="flex-1 px-3! whitespace-nowrap shadow-cta"
							disabled={!blob || sending}
							onclick={sendCard}
						>
							<Share class="size-4" aria-hidden="true" />
							{m.share_share_image()}
						</Button>
					{:else if canCopyImage()}
						<Button
							size="lg"
							class="flex-1 px-3! whitespace-nowrap shadow-cta"
							disabled={!blob}
							onclick={() => blob && copyImage(blob)}
						>
							<ClipboardCopy class="size-4" aria-hidden="true" />
							{m.share_copy_image()}
						</Button>
					{/if}
					{#if canShareFiles() || canCopyImage()}
						<Button
							variant="secondary"
							size="lg"
							class="aspect-square shrink-0 px-0!"
							label={m.share_save_image()}
							disabled={!blob}
							onclick={saveCard}
						>
							<Download class="size-5" aria-hidden="true" />
						</Button>
					{:else}
						<Button
							size="lg"
							class="flex-1 px-3! whitespace-nowrap shadow-cta"
							disabled={!blob}
							onclick={saveCard}
						>
							<Download class="size-4" aria-hidden="true" />
							{m.share_save_image()}
						</Button>
					{/if}
				</div>
			</div>
			{@render panel(true)}
		</div>
	</BottomSheet>
{:else}
	<BaseDialog bind:open wide bleed>
		<Dialog.Close
			aria-label={m.share_sheet_close()}
			class="absolute top-4 right-4 flex size-9 items-center justify-center rounded-full text-text-muted transition hover:bg-background focus-visible:outline-2 focus-visible:outline-primary"
		>
			<X class="size-5" aria-hidden="true" />
		</Dialog.Close>
		<div class="grid grid-cols-[300px_1fr] gap-7">
			{#if tab === 'image'}
				<!-- At least as tall as the card's frame was alone, so switching tabs
				     keeps the dialog's size under a one-line title (a longer one
				     grows the Link tab's column a little); the card scrolls in what
				     is left. -->
				<div class="-my-7 -ml-7 flex min-h-[480px] flex-col bg-background">
					<div class="px-5.5 pt-7 pb-1.5">{@render switcher()}</div>
					<div class="relative flex-1">{@render card('absolute inset-0')}</div>
				</div>
				<div class="flex flex-col py-1">
					{@render heading()}
					<p class="mt-1 text-body-sm text-text-muted">
						{m.share_dialog_hint({ servings: formatServings(servings) })}
					</p>
					<div class="mt-5 flex flex-col gap-1.5">
						{#if canCopyImage()}
							<Button
								class="justify-start!"
								disabled={!blob}
								onclick={() => blob && copyImage(blob)}
							>
								<ClipboardCopy class="size-4" aria-hidden="true" />
								{m.share_copy_image()}
							</Button>
						{/if}
						<!-- The primary while nothing copies an image (plain http, older
						     Firefox), so the dialog always leads with one action. -->
						<Button
							variant={canCopyImage() ? 'secondary' : 'primary'}
							class="justify-start!"
							disabled={!blob}
							onclick={saveCard}
						>
							<Download class="size-4" aria-hidden="true" />
							{m.share_save_image()}
						</Button>
						{#if canShareFiles()}
							<Button
								variant="secondary"
								class="justify-start!"
								disabled={!blob || sending}
								onclick={sendCard}
							>
								<Share class="size-4" aria-hidden="true" />
								{m.share_share()}
							</Button>
						{/if}
					</div>
					{#if !canShareFiles() && touch.current}
						<!-- A tablet over plain http gets the dialog, and its own long-press
						     menu on the preview still shares the image. -->
						<p class="mt-3 text-caption text-text-muted">{m.share_hold_hint()}</p>
					{/if}
				</div>
			{/if}
			<!-- Kept on both tabs, so a choice made on Link outlasts a look at the image. -->
			{@render panel(false)}
		</div>
	</BaseDialog>
{/if}
