<script lang="ts">
	import ArrowRight from '@lucide/svelte/icons/arrow-right';
	import ArrowUpRight from '@lucide/svelte/icons/arrow-up-right';
	import { DropdownMenu } from 'bits-ui';
	import { session } from '#lib/auth.svelte.js';
	import { m } from '#lib/paraglide/messages.js';
	import { update } from '#lib/update.svelte.js';

	/**
	 * `menu`: the desktop popover's foot, clipped by its rounding; its own
	 * matches the inside of the 1px border, so the inset focus outline follows
	 * the corners. In the You sheet only an update stays in view: `sheet-foot`
	 * sits below the sheet's scroller, outside it, so the list scrolls above it
	 * and the sheet's scroll thumb ends where it starts; it renders only for
	 * `available`. `sheet` is the last entry of the scroller, for every other
	 * state, scrolled with the list - a band that says nothing is to do would
	 * only take room if it stuck. It is a card inside the scroller's padding,
	 * like the person card at the top, so the scroll thumb passes beside it. `onopen` lets the sheet close once the
	 * band's link is taken.
	 */
	let { place, onopen }: { place: 'menu' | 'sheet' | 'sheet-foot'; onopen?: () => void } = $props();

	// Gated here as well: a status can outlive an account switch for a moment.
	const owned = $derived(session.user?.role === 'superadmin' ? update.status : undefined);
	const pinned = $derived(owned?.state === 'available' && owned.latest !== undefined);
	const status = $derived(
		place === 'menu' || (place === 'sheet-foot') === pinned ? owned : undefined
	);

	const phone = $derived(place !== 'menu');
	const sheetEdge =
		'border-t border-border px-5 pt-3 pb-[max(0.875rem,env(safe-area-inset-bottom))]';
	const frame = $derived(
		place === 'sheet-foot'
			? `shrink-0 ${sheetEdge}`
			: place === 'sheet'
				? '-mx-1 mt-4 rounded-xl p-4'
				: '-mx-2 -mb-2 mt-1.5 rounded-b-[calc(var(--radius-2xl)-1px)] px-3.5 py-2.5'
	);
	const main = $derived(phone ? 'text-body-sm' : 'text-caption');
	const band = 'flex items-center gap-2.5 lining-nums tabular-nums';
	const available = $derived(
		`${band} ${frame} bg-accent text-accent-foreground hover-tint outline-none focus-visible:outline-2 focus-visible:outline-solid focus-visible:-outline-offset-2 focus-visible:outline-primary`
	);
</script>

{#snippet icon()}
	<img
		src="/icon-192.png"
		alt=""
		class="{phone ? 'size-9' : 'size-8'} shrink-0 rounded-lg shadow-card"
	/>
{/snippet}

{#snippet newer(current: string, latest: string)}
	{@render icon()}
	<span class="min-w-0 flex-1 leading-tight">
		<span class="block {main} font-semibold">{m.update_band_available()}</span>
		<span class="mt-0.5 flex items-center gap-1 text-caption">
			<span class="opacity-70">{current}</span>
			<ArrowRight class="size-3 shrink-0" aria-hidden="true" />
			<span class="font-semibold">{latest}</span>
		</span>
	</span>
	<ArrowUpRight class="{phone ? 'size-4.5' : 'size-3.5'} shrink-0" aria-hidden="true" />
{/snippet}

<!--
	The instance's version at the very foot of the owner's user menu: neutral
	with an "Up to date" chip, or on accent as a link to the new version's
	release notes. Without a status (not loaded, failed) it is left out.
-->
{#if status?.state === 'available' && status.latest}
	{@const latest = status.latest}
	{@const label = m.update_band_available_label({ from: status.current, to: latest.version })}
	{#if place === 'menu'}
		<!-- An item, so the menu's arrow keys reach it, rendered as the link;
		     Enter clicks it. It leaves the SPA, so `resolve()` does not apply. -->
		<DropdownMenu.Item aria-label={label}>
			{#snippet child({ props })}
				<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
				<a
					{...props}
					href={latest.changelogUrl}
					target="_blank"
					rel="noopener external"
					class={available}
				>
					{@render newer(status.current, latest.version)}
				</a>
			{/snippet}
		</DropdownMenu.Item>
	{:else}
		<!-- Leaves the SPA, so `resolve()` does not apply. -->
		<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
		<a
			href={latest.changelogUrl}
			target="_blank"
			rel="noopener external"
			aria-label={label}
			onclick={onopen}
			class={available}
		>
			{@render newer(status.current, latest.version)}
		</a>
	{/if}
{:else if status}
	<div class="{band} {frame} {phone ? 'bg-surface' : 'bg-background'}">
		{@render icon()}
		<span class="min-w-0 flex-1 leading-tight">
			<span class="block font-display {main} font-medium">{m.update_band_name()}</span>
			<span class="mt-0.5 block text-caption text-text-muted">
				{m.update_band_version({ version: status.current })}
			</span>
		</span>
		{#if status.state === 'current'}
			<span
				class="ml-auto inline-flex items-center gap-1.5 rounded-pill bg-success-soft py-0.5 pr-2.5 pl-2 text-micro font-semibold whitespace-nowrap text-success-foreground"
			>
				<span class="size-1.75 rounded-pill bg-success" aria-hidden="true"></span>
				{m.update_band_current()}
			</span>
		{/if}
	</div>
{/if}
