<script lang="ts">
	import { m } from '$lib/paraglide/messages';
	import { formatDate } from '$lib/recipe/format';
	import { lifetime, type Lifespan } from '$lib/settings/token-access';

	// How much of something's life has passed - an API token's, a shared
	// link's: issued on the left, expiry on the right, "today" as a mark and
	// the last use, when there is one, as a dot. Without an expiry the bar
	// stays open past today. Purely visual: the caller states the same dates
	// in words for assistive technology.
	let { item, faded = false }: { item: Lifespan; faded?: boolean } = $props();

	const life = $derived(lifetime(item));
	const percent = (fraction: number) => `${fraction * 100}%`;
	// "today" sits above its mark, kept off the edges; below the bar it would
	// collide with the dates at either end on a phone.
	const todayLeft = $derived(`clamp(1.5rem, ${percent(life.spent)}, calc(100% - 1.5rem))`);
</script>

<div aria-hidden="true" class={faded ? 'opacity-60' : ''}>
	{#if life.kind !== 'spent'}
		<div class="relative h-5 text-micro text-text-muted">
			<span class="absolute bottom-1 -translate-x-1/2" style:left={todayLeft}>
				{m.lifetime_today()}
			</span>
		</div>
	{/if}
	<div
		class="relative h-2.5 rounded-pill border {life.kind === 'open'
			? 'border-dashed border-border'
			: life.kind === 'spent'
				? 'border-border bg-[repeating-linear-gradient(135deg,var(--color-destructive-soft)_0_4px,transparent_4px_8px)]'
				: 'border-border bg-[repeating-linear-gradient(90deg,var(--color-border)_0_2px,transparent_2px_6px)]'}"
	>
		{#if life.kind !== 'spent'}
			<span
				class="absolute inset-y-0 left-0 rounded-pill bg-accent"
				style:width={percent(life.spent)}
			></span>
			<span
				class="absolute -top-1.5 h-4.5 w-0.5 -translate-x-1/2 rounded-pill bg-text"
				style:left={percent(life.spent)}
			></span>
		{/if}
		{#if life.lastUsed !== undefined}
			<span
				class="absolute top-1/2 size-1.5 -translate-1/2 rounded-full bg-primary"
				style:left={percent(life.lastUsed)}
			></span>
		{/if}
	</div>
	<div
		class="mt-1.5 flex flex-wrap justify-between gap-x-3 text-micro text-text-muted tabular-nums"
	>
		<span>{formatDate(item.createdAt)}</span>
		{#if life.kind === 'open'}
			<span class="ml-auto">{m.lifetime_open_end()}</span>
		{:else if life.kind === 'running' && item.expiresAt != null}
			<span class="ml-auto">{formatDate(item.expiresAt)}</span>
		{/if}
	</div>
</div>
