<script lang="ts" module>
	import { MediaQuery } from 'svelte/reactivity';

	// Any finger at all, not the primary pointer: an iPad with a trackpad
	// reports `hover: hover` and would keep the `<a>` its long press previews.
	const touch = new MediaQuery('any-pointer: coarse');
</script>

<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { HTMLAttributes } from 'svelte/elements';
	import { on } from 'svelte/events';
	import { goto } from '$app/navigation';

	/**
	 * A link on a card a finger drags. Without a touch screen it is an
	 * `<a href>` (middle-click, a new tab, the URL in the status bar); with
	 * one it is a `role="link"` that opens `href` on a tap or Enter,
	 * since iOS shows its link preview on a long press over any `<a href>`,
	 * which Chrome there opens whatever `-webkit-touch-callout` says.
	 */
	let {
		href,
		onclick,
		children,
		class: cls = '',
		...rest
	}: Omit<HTMLAttributes<HTMLElement>, 'onclick'> & {
		/** Resolved by the caller. */
		href: string;
		/** Runs before the navigation; `preventDefault` stops it. */
		onclick?: (event: MouseEvent) => void;
		children: Snippet;
	} = $props();

	function open(event: MouseEvent) {
		onclick?.(event);
		if (!event.defaultPrevented) void goto(href);
	}
	// Directly on the element: the drag zone's own keydown on the card takes
	// Enter and Space for a keyboard drag of anything without an `href`, and
	// stops it there. Space does nothing on a link, so it only scrolls. No
	// synthetic click on Enter: the card's guard drops untrusted ones.
	function enter(node: HTMLElement) {
		return on(node, 'keydown', (event) => {
			if (event.key === ' ') return event.stopPropagation();
			if (event.key !== 'Enter') return;
			event.preventDefault();
			event.stopPropagation();
			void goto(href);
		});
	}
</script>

{#if !touch.current}
	<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -- resolved by the caller -->
	<a {href} draggable="false" {onclick} class={cls} {...rest}>{@render children()}</a>
{:else}
	<span
		role="link"
		tabindex="0"
		onclick={open}
		{@attach enter}
		class="cursor-pointer {cls}"
		{...rest}>{@render children()}</span
	>
{/if}
