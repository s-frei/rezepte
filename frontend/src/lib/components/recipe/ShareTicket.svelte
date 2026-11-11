<script lang="ts">
	import Globe from '@lucide/svelte/icons/globe';
	import Lock from '@lucide/svelte/icons/lock';
	import Pause from '@lucide/svelte/icons/pause';
	import { imageUrl, type Recipe } from '#lib/api/recipes.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import type { StubFace } from '#lib/recipe/share-stub.js';
	import PlaceholderTile from './PlaceholderTile.svelte';

	// The link drawn as a ticket, the way the image is a card: the recipe on
	// the left, the stub with the audience or the time left behind a
	// perforation. Decorative: the row text and the chip say all of it.
	let {
		recipe,
		address,
		face,
		photo = 'full'
	}: {
		recipe: Recipe;
		/** The address the ticket passes on, shown on one line. */
		address: string;
		face: StubFace;
		/** The preview's fit steps: 84px, 56px, or no photo at all. */
		photo?: 'full' | 'flat' | 'none';
	} = $props();

	const coverId = $derived(recipe.coverImageId ?? recipe.images[0]?.id ?? null);
	let coverFailed = $state(false);
	const off = $derived(face.kind === 'paused' || face.kind === 'limited');
	const everyone = $derived(!off && face.kind !== 'household');
	// The photo's box, and for the placeholder its initial in proportion.
	const box = $derived(
		photo === 'flat' ? 'h-14 [&>span]:text-placeholder-xs' : 'h-[84px] [&>span]:text-placeholder-sm'
	);
</script>

<!-- The shadow sits on a wrapper as a filter, so it follows the punched
     holes: two notches cut out of the ticket where the perforation meets its
     edges, showing whatever lies behind - not discs in the ground's color,
     which the ticket's own shadow around them gave away. -->
<div aria-hidden="true" class="w-67 max-w-full -rotate-[2.5deg] drop-shadow-lift">
	<div class="ticket relative flex rounded-lg bg-surface-elevated text-text">
		<div class="min-w-0 flex-1 pt-[9px] pr-2.5 pb-[11px] pl-[9px]">
			{#if photo !== 'none'}
				{#if coverId && !coverFailed}
					<img
						src={imageUrl(recipe.id, coverId, 'detail')}
						alt=""
						class="block w-full rounded-sm object-cover {box} {off ? 'opacity-55 grayscale' : ''}"
						onerror={() => (coverFailed = true)}
					/>
				{:else}
					<PlaceholderTile
						id={recipe.id}
						title={recipe.title}
						class="overflow-hidden rounded-sm {box} {off ? 'opacity-55 grayscale' : ''}"
					/>
				{/if}
			{/if}
			<p
				class="mt-2 line-clamp-2 font-display text-ticket-title/[1.2] font-medium hyphens-auto {off
					? 'text-text-muted'
					: ''}"
			>
				{recipe.title}
			</p>
			<p class="mt-[3px] truncate text-micro/[1.3] text-text-muted">{address}</p>
		</div>
		<div
			class="flex w-16 flex-none flex-col items-center justify-center gap-1 rounded-r-lg border-l-2 border-dashed text-center text-label/[1.25] font-semibold tracking-normal {everyone
				? 'border-handle bg-accent text-accent-foreground'
				: off
					? 'border-border bg-surface text-text-muted'
					: 'border-border text-text-muted'}"
		>
			{#if face.kind === 'household'}
				<Lock class="size-4.5" />
				<span>{m.share_stub_household()}</span>
			{:else if face.kind === 'paused' || face.kind === 'limited'}
				<Pause class="size-4.5" />
				<span>
					{face.kind === 'paused'
						? m.public_share_status_paused()
						: m.public_share_status_limited()}
				</span>
			{:else}
				<Globe class="size-4.5" />
				{#if face.kind === 'today'}
					<strong class="font-display text-body-lg/none font-semibold"
						>{m.share_stub_today()}</strong
					>
					<span>
						{m.share_stub_until({
							time: new Intl.DateTimeFormat(getLocale(), { hour: 'numeric' }).format(face.until)
						})}
					</span>
				{:else}
					<strong class="font-display text-ticket-figure/none font-semibold">
						{face.kind === 'open' ? '∞' : face.value}
					</strong>
					<span>
						{face.kind === 'open'
							? m.lifetime_open_end()
							: face.kind === 'months'
								? m.share_unit_months()
								: face.value === 1
									? m.share_unit_day()
									: m.share_unit_days()}
					</span>
				{/if}
			{/if}
		</div>
	</div>
</div>

<style>
	/* Two 8px notches centered on the perforation, at the top and bottom edge:
	   the stub is 64px wide and its dashed edge, 2px, runs from 64px to 62px
	   off the right, so its middle is 63px. */
	.ticket {
		--notch:
			radial-gradient(circle 8px at calc(100% - 63px) 0, transparent 7.5px, #000 8px),
			radial-gradient(circle 8px at calc(100% - 63px) 100%, transparent 7.5px, #000 8px);
		-webkit-mask-image: var(--notch);
		mask-image: var(--notch);
		-webkit-mask-composite: source-in;
		mask-composite: intersect;
	}
</style>
