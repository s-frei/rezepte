<script lang="ts">
	import { Popover } from 'bits-ui';
	import type { Snippet } from 'svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { narrow } from '#lib/ui/narrow.js';
	import BottomSheet from './BottomSheet.svelte';

	// Who someone is, behind a tap on their mark or name: a bottom sheet
	// below `md`, the phone's one shape for "one person", and a popover
	// from `md`. A popover rather than a tooltip, because bits-ui's tooltip
	// returns early on `pointerType === "touch"` - a touch laptop would get
	// nothing; with `openOnHover` it still behaves like a tooltip under a mouse.
	let {
		open = $bindable(false),
		label,
		class: className,
		hover = true,
		anchor,
		panelClass,
		trigger,
		card
	}: {
		open?: boolean;
		/** The trigger's accessible name, when its content does not say it. */
		label?: string;
		/** The trigger's classes. */
		class: string;
		/** Opens on hover after 300ms as well as on a click. */
		hover?: boolean;
		/** What the popover lines up with, when not the trigger. */
		anchor?: HTMLElement | null;
		/** The popover's look and width. */
		panelClass: string;
		trigger: Snippet;
		/** What opens; `sheet` is true in the phone's sheet. */
		card: Snippet<[sheet: boolean]>;
	} = $props();
</script>

{#if narrow.current}
	<button type="button" aria-label={label} onclick={() => (open = true)} class={className}>
		{@render trigger()}
	</button>
	<BottomSheet bind:open closeLabel={m.common_close()}>
		<div data-sheet-body class="min-h-0 flex-1 overflow-y-auto px-5 pb-8">{@render card(true)}</div>
	</BottomSheet>
{:else}
	<Popover.Root bind:open>
		<Popover.Trigger openOnHover={hover} openDelay={300} aria-label={label} class={className}>
			{@render trigger()}
		</Popover.Trigger>
		<Popover.Portal>
			<!-- `collisionPadding` keeps it off the screen edge when Floating UI
			     flips it above a trigger at the very bottom of the viewport. -->
			<Popover.Content
				customAnchor={anchor}
				side="bottom"
				sideOffset={6}
				align="start"
				collisionPadding={12}
				class="z-50 rounded-2xl shadow-dialog {panelClass}"
			>
				{@render card(false)}
			</Popover.Content>
		</Popover.Portal>
	</Popover.Root>
{/if}
