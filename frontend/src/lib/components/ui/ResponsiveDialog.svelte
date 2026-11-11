<script lang="ts">
	import type { Snippet } from 'svelte';
	import { narrow } from '#lib/ui/narrow.js';
	import BaseDialog from './BaseDialog.svelte';
	import BottomSheet from './BottomSheet.svelte';
	import DialogActions from './DialogActions.svelte';

	// A dialog that is a bottom sheet below `md` (`narrow`) and a `BaseDialog`
	// from there. Given `body`, it lays the two out: on the phone the body
	// scrolls (`data-sheet-body`) above a bordered footer that stays put, on
	// the computer the footer is the `DialogActions` row under a hairline.
	// Given `children` instead, the caller lays out both itself and reads
	// `narrow` to tell which it is in.
	let {
		open = $bindable(false),
		closeLabel,
		onsubmit,
		onOpenAutoFocus,
		onCloseAutoFocus,
		bodyClass = 'pb-4',
		footerClass = 'flex flex-wrap gap-3 px-5 pt-3.5',
		body,
		footer,
		children
	}: {
		open?: boolean;
		/** Accessible name of the sheet's handle. */
		closeLabel: string;
		/** Makes body and footer one form. */
		onsubmit?: (event: SubmitEvent) => void;
		/** The sheet's only: where focus goes once it has opened. */
		onOpenAutoFocus?: (event: Event) => void;
		onCloseAutoFocus?: (event: Event) => void;
		/** The sheet body's bottom padding. */
		bodyClass?: string;
		/** The sheet footer's layout and padding. */
		footerClass?: string;
		body?: Snippet;
		footer?: Snippet;
		children?: Snippet;
	} = $props();
</script>

{#snippet sheet()}
	<div data-sheet-body class="min-h-0 flex-1 overflow-y-auto px-5 {bodyClass}">
		{@render body?.()}
	</div>
	{#if footer}
		<div class="border-t border-border pb-5 {footerClass}">{@render footer()}</div>
	{/if}
{/snippet}

{#snippet dialog()}
	{@render body?.()}
	{#if footer}
		<DialogActions class="mt-6 border-t border-border pt-4">{@render footer()}</DialogActions>
	{/if}
{/snippet}

{#snippet laidOut(parts: Snippet, formClass?: string)}
	{#if children}
		{@render children()}
	{:else if onsubmit}
		<form {onsubmit} class={formClass}>{@render parts()}</form>
	{:else}
		{@render parts()}
	{/if}
{/snippet}

{#if narrow.current}
	<BottomSheet bind:open {closeLabel} {onOpenAutoFocus} {onCloseAutoFocus}>
		{@render laidOut(sheet, 'flex min-h-0 flex-1 flex-col')}
	</BottomSheet>
{:else}
	<BaseDialog bind:open {onCloseAutoFocus}>
		{@render laidOut(dialog)}
	</BaseDialog>
{/if}
